package cowswap

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	sb "github.com/symbioticfi/merkle-executor/api/bindings/cowsettlement"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	ab "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type cowBatchReader struct {
	negativeChain
	t         *testing.T
	calls     int
	filled    *big.Int
	badDomain bool
}

func (r *cowBatchReader) BatchRead(ctx context.Context, req []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	r.calls++
	if len(req) != 3 {
		r.t.Fatalf("group size %d", len(req))
	}
	out := make([]protocolio.ReadResult, 3)
	for i, q := range req {
		if !q.MulticallSafe || q.Block.Int64() < 13 {
			r.t.Fatal("unsafe or unpinned batch")
		}
		out[i].Data, out[i].Err = r.CallContract(ctx, q.Call, q.Block)
	}
	out[1].Data, _ = sb.NewCowSettlement().GetABI().Methods["filledAmount"].Outputs.Pack(r.filled)
	out[2].Err = errors.New("unused approval error")
	if r.badDomain {
		out[0].Data = make([]byte, 32)
	}
	return out, nil
}
func TestAdvanceBatchesButPreservesTerminalPrecedence(t *testing.T) {
	for _, mode := range []string{"filled", "expired", "partial", "invalidated", "domain", "local identity"} {
		t.Run(mode, func(t *testing.T) {
			r := &cowBatchReader{t: t, filled: big.NewInt(990)}
			p, err := fixtureFactory(node(t, negativeConfig("http://127.0.0.1:1")), registry.Deps{Chain: r})
			if err != nil {
				t.Fatal(err)
			}
			s := negativeSnapshot()
			s.BlockNumber = big.NewInt(13)
			s.BlockHash = testHeader(s.BlockNumber).Hash()
			if mode == "expired" {
				r.filled.SetInt64(0)
				s.BlockNumber = big.NewInt(14)
				s.BlockHash = testHeader(s.BlockNumber).Hash()
				s.Timestamp = 2000
			}
			if mode == "partial" {
				r.filled.SetInt64(1)
			}
			if mode == "invalidated" {
				r.filled.Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
			}
			r.badDomain = mode == "domain"
			o := sampleOrder()
			buf, _ := EncodeOrder(o)
			domain := protocolio.Domain("Gnosis Protocol", "v2", s.ChainID, addr(4))
			digest := protocolio.Digest(domain, crypto.Keccak256Hash(buf))
			payload, err := ab.NewMerkleAccount().TryPackSign(domain, buf, []ab.IMerkleAccountHash{{Offset: big.NewInt(0), Length: big.NewInt(int64(len(buf))), Destination: big.NewInt(0)}})
			if err != nil {
				t.Fatal(err)
			}
			d := prepared{Version: 1, ChainID: "1", Account: s.Account, PreparedBlock: "12", Order: o, Digest: digest, UID: UID(digest, s.Account, uint32(o.ValidTo))}
			if mode == "local identity" {
				d.UID = "wrong"
			}
			raw, _ := json.Marshal(d)
			out := provider.Output{Data: raw, Actions: map[string]hexutil.Bytes{"1:0": payload}}
			progress, err := p.Advance(context.Background(), s, out, nil)
			if mode == "local identity" {
				if err == nil || r.calls != 0 {
					t.Fatalf("local failure used RPC: %v %d", err, r.calls)
				}
				return
			}
			if r.calls != 1 {
				t.Fatalf("batches=%d", r.calls)
			}
			if mode == "filled" || mode == "expired" {
				if err != nil || !progress.Done {
					t.Fatalf("terminal branch masked: %+v %v", progress, err)
				}
			} else if err == nil || strings.Contains(err.Error(), "approval") {
				t.Fatalf("wrong error precedence: %v", err)
			}
		})
	}
}
