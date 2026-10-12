package merkl

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func rewardBatch(t *testing.T) ([]wire.MerklReward, common.Hash, [][][32]byte) {
	t.Helper()
	levels := [][]common.Hash{make([]common.Hash, 32)}
	for i := range 32 {
		levels[0][i] = crypto.Keccak256Hash(protocolio.AddressWord(addr(3)), protocolio.AddressWord(addr(int64(i+100))), protocolio.Word(big.NewInt(100)))
	}
	for len(levels[len(levels)-1]) > 1 {
		last := levels[len(levels)-1]
		next := make([]common.Hash, len(last)/2)
		for i := range next {
			a, b := last[2*i], last[2*i+1]
			if bytes.Compare(a[:], b[:]) > 0 {
				a, b = b, a
			}
			next[i] = crypto.Keccak256Hash(a[:], b[:])
		}
		levels = append(levels, next)
	}
	rows, proofs := make([]wire.MerklReward, 32), make([][][32]byte, 32)
	for i := range rows {
		rows[i] = wire.MerklReward{Token: wire.MerklToken{Address: addr(int64(i + 100)).Hex(), Decimals: 6}, Amount: "100", Claimed: "0", Pending: "0"}
		for level := 0; level < len(levels)-1; level++ {
			h := levels[level][(i>>level)^1]
			proofs[i] = append(proofs[i], h)
			rows[i].Proofs = append(rows[i].Proofs, h.Hex())
		}
	}
	return rows, levels[len(levels)-1][0], proofs
}
func TestDefaultRecipientReadOncePerPinnedPreparation(t *testing.T) {
	for _, mode := range []string{"missing default", "account default", "overridden", "unused malformed default", "mixed", "redirected", "malformed default"} {
		t.Run(mode, func(t *testing.T) {
			rows, root, proofs := rewardBatch(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: rows}})
			}))
			defer srv.Close()
			a := binding.NewMerklDistributor()
			contractABI := a.GetABI()
			calls, defaults := 0, 0
			pinned := big.NewInt(12)
			chain := reader(func(_ context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
				if b.Cmp(pinned) != 0 {
					t.Fatal("read escaped pinned snapshot")
				}
				method, err := contractABI.MethodById(m.Data[:4])
				if err != nil {
					return nil, err
				}
				switch method.Name {
				case "getMerkleRoot":
					return method.Outputs.Pack(root)
				case "claimed":
					return method.Outputs.Pack(new(big.Int), new(big.Int), root)
				case "claimRecipient":
					args, err := method.Inputs.Unpack(m.Data[4:])
					if err != nil {
						return nil, err
					}
					token := args[1].(common.Address)
					if token == (common.Address{}) {
						defaults++
						if mode == "malformed default" || mode == "unused malformed default" {
							return []byte{1}, nil
						}
						if pinned.Int64() == 13 || mode == "redirected" {
							return method.Outputs.Pack(addr(9))
						}
						if mode == "account default" {
							return method.Outputs.Pack(addr(3))
						}
					} else if mode == "overridden" || mode == "unused malformed default" || mode == "mixed" && token.Big().Int64()%2 == 0 {
						return method.Outputs.Pack(addr(3))
					}
					return method.Outputs.Pack(common.Address{})
				}
				t.Fatalf("unexpected read %s", method.Name)
				return nil, nil
			})
			tokens, users, amounts := make([]common.Address, 32), make([]common.Address, 32), make([]*big.Int, 32)
			cfg := Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: addr(4).Hex(), Multicall3: addr(5).Hex(), Action: "0:0"}
			for i, row := range rows {
				tokens[i] = common.HexToAddress(row.Token.Address)
				users[i] = addr(3)
				amounts[i] = big.NewInt(100)
				cfg.Tokens = append(cfg.Tokens, row.Token.Address)
				cfg.ProofLengths = append(cfg.ProofLengths, 5)
			}
			payload := a.PackClaim(users, tokens, amounts, proofs)
			cfg.CalldataBytes = len(payload)
			batch := multicallReader(t, tokens, chain)
			outer := reader(func(ctx context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
				calls++
				return batch(ctx, m, b)
			})
			p, err := Factory(node(t, cfg), providerRegistry.Deps{Chain: outer, HTTP: srv.Client()})
			if err != nil {
				t.Fatal(err)
			}
			snap := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: new(big.Int).Set(pinned), Account: addr(3), Timestamp: 1000}
			out, err := p.Prepare(context.Background(), snap)
			if mode == "redirected" || mode == "malformed default" {
				if err == nil {
					t.Fatal("invalid default recipient accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out.Actions["0:0"], payload) {
				t.Fatal("batch changed cumulative claim payload")
			}
			want, wantDefault := 1, 1

			if calls != want || defaults != wantDefault {
				t.Fatalf("RPC reads=%d default=%d, want %d/%d", calls, defaults, want, wantDefault)
			}
			pinned.SetInt64(13)
			snap.BlockNumber = new(big.Int).Set(pinned)
			calls, defaults = 0, 0
			_, err = p.Prepare(context.Background(), snap)
			if mode == "overridden" || mode == "unused malformed default" {
				if err != nil || defaults != 1 || calls != 1 {
					t.Fatal("explicit recipients must bypass default")
				}
			} else if err == nil || defaults != 1 || calls != 1 {
				t.Fatal("default recipient cached across blocks")
			}
		})
	}
}
