package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/merkl"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type merklAnvilReader func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)

func (f merklAnvilReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	return f(ctx, msg, block)
}

func loadMerklArtifact(t *testing.T, name string) serviceArtifact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "providers", "merkl", "testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		ABI      json.RawMessage
		Bytecode hexutil.Bytes
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(document.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return serviceArtifact{ABI: parsed, Bytecode: document.Bytecode}
}

func TestAnvilMerklMulticallUsesPinnedDistributorState(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("set MERKLE_ANVIL_TEST=1 for local Multicall3 integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client, _ := startServiceAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil || len(owners) == 0 {
		t.Fatalf("local fixture account unavailable: %v", err)
	}
	owner := owners[0]
	aggregateArtifact := loadMerklArtifact(t, "Multicall3")
	distributorArtifact := loadMerklArtifact(t, "FixtureMerklDistributor")
	aggregate := serviceSeedTransaction(t, ctx, client, owner, nil, aggregateArtifact.Bytecode).ContractAddress
	distributor := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(distributorArtifact.Bytecode), servicePack(t, distributorArtifact, "", aggregate)...)).ContractAddress
	account, token := common.HexToAddress("0x03"), common.HexToAddress("0x01")
	// Independent viem vector also used by the provider's golden test.
	sibling := common.HexToHash("0x" + strings.Repeat("11", 32))
	root := common.HexToHash("0xd98400418a73f31e4b681498ddc089b9c9b92f409942f76789590c1cae24f0ab")
	configure := func(claimed int64, recipient common.Address) {
		t.Helper()
		serviceSeedTransaction(t, ctx, client, owner, &distributor, servicePack(t, distributorArtifact, "configure", root, account, token, big.NewInt(claimed), recipient))
	}
	configure(80, common.Address{})
	currentSnapshot := func() provider.Snapshot {
		t.Helper()
		header, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		return provider.Snapshot{ChainID: big.NewInt(31337), BlockNumber: header.Number, BlockHash: header.Hash(), Account: account, Timestamp: header.Time}
	}
	original := currentSnapshot()
	reward := wire.MerklReward{Token: wire.MerklToken{Address: token.Hex(), Decimals: 6}, Amount: "100", Claimed: "70", Pending: "500", Proofs: []string{sibling.Hex()}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 31337}, Rewards: []wire.MerklReward{reward}}})
	}))
	defer srv.Close()
	calls := 0
	var expectedBlock *big.Int
	reader := merklAnvilReader(func(received context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
		calls++
		if received != ctx || msg.To == nil || *msg.To != aggregate || block == nil || block.Cmp(expectedBlock) != 0 {
			t.Fatal("Merkl read escaped caller context, configured aggregate or pinned block")
		}
		return client.CallContract(received, msg, block)
	})
	cfg := merkl.Config{HTTP: protocolio.HTTPConfig{URL: srv.URL}, Distributor: distributor.Hex(), Multicall3: aggregate.Hex(), Tokens: []string{token.Hex()}, ProofLengths: []int{1}, Action: "0:0", CalldataBytes: 452, MinClaimable: "10", AmountInputs: []uint{0}, ClaimableInputs: []uint{1}, SkipIfEmpty: true, DedicatedClaimGroup: true}
	p, err := merkl.New(cfg, providerRegistry.Deps{Chain: reader, HTTP: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	payload := binding.NewMerklDistributor().PackClaim([]common.Address{account}, []common.Address{token}, []*big.Int{big.NewInt(100)}, [][][32]byte{{sibling}})
	prepare := func(snapshot provider.Snapshot) (provider.Output, error) {
		t.Helper()
		calls, expectedBlock = 0, snapshot.BlockNumber
		out, err := p.Prepare(ctx, snapshot)
		if calls != 1 {
			t.Fatalf("Prepare used %d outer RPC calls; want one aggregate and no fallback", calls)
		}
		return out, err
	}
	claim := func(snapshot provider.Snapshot, delta string) {
		t.Helper()
		out, err := prepare(snapshot)
		if err != nil || out.Inputs[0] != "100" || out.Inputs[1] != delta || !bytes.Equal(out.Actions["0:0"], payload) || len(out.SkipSteps) != 0 {
			t.Fatalf("claim changed cumulative amount/current delta/layout: output=%+v error=%v", out, err)
		}
	}
	claim(original, "20")
	configure(90, common.Address{})
	claim(currentSnapshot(), "10")
	claim(original, "20") // eth_call must read historical state, not latest.
	serviceSeedTransaction(t, ctx, client, owner, &distributor, servicePack(t, distributorArtifact, "setRecipient", account, common.Address{}, common.HexToAddress("0x09")))
	if out, err := prepare(currentSnapshot()); err == nil || len(out.Actions) != 0 || len(out.SkipSteps) != 0 {
		t.Fatalf("redirected default became claim or skip: %+v %v", out, err)
	}
	claim(original, "20")
	serviceSeedTransaction(t, ctx, client, owner, &distributor, servicePack(t, distributorArtifact, "setRecipient", account, token, account))
	claim(currentSnapshot(), "10") // Explicit recipient overrides redirected default.
	configure(95, account)
	if out, err := prepare(currentSnapshot()); err != nil || len(out.Actions) != 0 || len(out.SkipSteps) != 1 || out.Inputs[0] != "0" || out.Inputs[1] != "0" {
		t.Fatalf("valid below-minimum claim did not skip: %+v %v", out, err)
	}
	serviceSeedTransaction(t, ctx, client, owner, &distributor, servicePack(t, distributorArtifact, "setFailure", true))
	if out, err := prepare(currentSnapshot()); err == nil || len(out.Actions) != 0 || len(out.SkipSteps) != 0 {
		t.Fatalf("reverting aggregate became claim or skip: %+v %v", out, err)
	}
	claim(original, "20")
}
