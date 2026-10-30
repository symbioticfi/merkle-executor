package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	distributor "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func loadGuardedMerklArtifact(t *testing.T, name string) serviceArtifact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "merkl", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		ABI      json.RawMessage
		Bytecode hexutil.Bytes
	}
	if err = json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(document.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return serviceArtifact{ABI: parsed, Bytecode: document.Bytecode}
}

func TestAnvilFrontendGuardedMerklMixedRewards(t *testing.T)      { runFrontendGuardedMerkl(t, false) }
func TestAnvilIdleMerklDoesNotBlockReadyNativeClaim(t *testing.T) { runFrontendGuardedMerkl(t, true) }
func runFrontendGuardedMerkl(t *testing.T, native bool) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	owner := owners[0]
	raw, err := os.ReadFile(filepath.Join("..", "..", "sdk", "providers", "merkl", "testdata", "frontend-guarded.json"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Account, Capability, Distributor common.Address
		RewardRoot                       common.Hash
		Recipe                           hexutil.Bytes
		Rewards                          []struct {
			Token      common.Address
			Cumulative string
			Proof      []string
		}
		Calls []struct {
			Hash  common.Hash
			Slots []struct {
				Name   string
				Offset int
			}
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	aa, da, ca := loadGuardedMerklArtifact(t, "FixtureGuardedAccount"), loadGuardedMerklArtifact(t, "FixtureGuardedDistributor"), loadGuardedMerklArtifact(t, "RecipeCapability")
	ta, ea := loadServiceArtifact(t, "FixtureToken"), loadServiceArtifact(t, "RecipeExecutor")
	install := func(target common.Address, artifact serviceArtifact) {
		deployed := serviceSeedTransaction(t, ctx, client, owner, nil, artifact.Bytecode).ContractAddress
		code, err := client.CodeAt(ctx, deployed, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err = client.Client().CallContext(ctx, nil, "anvil_setCode", target, hexutil.Bytes(code)); err != nil {
			t.Fatal(err)
		}
	}
	install(fixture.Account, aa)
	install(fixture.Capability, ca)
	install(fixture.Distributor, da)
	for _, reward := range fixture.Rewards {
		install(reward.Token, ta)
	}
	call := func(target common.Address, artifact serviceArtifact, method string, args ...any) {
		serviceSeedTransaction(t, ctx, client, owner, &target, servicePack(t, artifact, method, args...))
	}
	call(fixture.Account, aa, "setCapability", fixture.Capability)
	call(fixture.Distributor, da, "setRoot", fixture.RewardRoot)
	for _, reward := range fixture.Rewards {
		call(fixture.Distributor, da, "setRecipient", fixture.Account, reward.Token, fixture.Account)
	}
	call(fixture.Distributor, da, "setPayoutBps", big.NewInt(10000))
	call(fixture.Distributor, da, "setClaimed", fixture.Account, fixture.Rewards[0].Token, big.NewInt(100))
	target := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(ea.Bytecode), servicePack(t, ea, "", owner, fixture.Account)...)).ContractAddress
	var nativeLeaf recipes.Leaf
	if native {
		call(fixture.Distributor, da, "setClaimed", fixture.Account, fixture.Rewards[1].Token, big.NewInt(200))
		shares, issuer := loadIssuerArtifact(t, "FixtureShare"), loadIssuerArtifact(t, "FixtureAsyncIssuer")
		share := serviceSeedTransaction(t, ctx, client, owner, nil, shares.Bytecode).ContractAddress
		venue := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(issuer.Bytecode), servicePack(t, issuer, "", fixture.Rewards[1].Token, share)...)).ContractAddress
		call(share, shares, "mint", fixture.Account, big.NewInt(10))
		if err = client.Client().CallContext(ctx, nil, "anvil_setBalance", fixture.Account, "0xde0b6b3a7640000"); err != nil {
			t.Fatal(err)
		}
		if err = client.Client().CallContext(ctx, nil, "anvil_impersonateAccount", fixture.Account); err != nil {
			t.Fatal(err)
		}
		serviceSeedTransaction(t, ctx, client, fixture.Account, &share, servicePack(t, shares, "approve", venue, big.NewInt(10)))
		serviceSeedTransaction(t, ctx, client, fixture.Account, &venue, servicePack(t, issuer, "requestRedeem", big.NewInt(10), fixture.Account, fixture.Account))
		if err = client.Client().CallContext(ctx, nil, "anvil_stopImpersonatingAccount", fixture.Account); err != nil {
			t.Fatal(err)
		}
		call(venue, issuer, "finalize", fixture.Account)
		call(fixture.Rewards[1].Token, ta, "mint", venue, big.NewInt(20))
		data := servicePack(t, issuer, "redeem", big.NewInt(10), fixture.Account, fixture.Account)
		nativeLeaf = recipes.Leaf{Target: venue, Value: new(big.Int), Data: data, DataMask: make([]byte, len(data))}
		nativeLeaf.Hash, err = recipes.HashLeaf(nativeLeaf)
		if err != nil {
			t.Fatal(err)
		}
		tree.Leaves = append(tree.Leaves, nativeLeaf)
		// Recommit unchanged frontend claim leaves together with the independent native
		// claim. Only permission-tree proofs/root change, never frontend call bytes.
		hashes := make([]common.Hash, len(tree.Leaves))
		for i, leaf := range tree.Leaves {
			hashes[i] = leaf.Hash
		}
		sort.Slice(hashes, func(i, j int) bool { return hashes[i].Cmp(hashes[j]) < 0 })
		heap := make([]common.Hash, 2*len(hashes)-1)
		positions := map[common.Hash]int{}
		for i, hash := range hashes {
			at := len(heap) - 1 - i
			heap[at] = hash
			positions[hash] = at
		}
		for i := len(hashes) - 2; i >= 0; i-- {
			a, b := heap[2*i+1], heap[2*i+2]
			if a.Cmp(b) > 0 {
				a, b = b, a
			}
			heap[i] = crypto.Keccak256Hash(a[:], b[:])
		}
		tree.Root = heap[0]
		for i := range tree.Leaves {
			tree.Leaves[i].Proof = nil
			for at := positions[tree.Leaves[i].Hash]; at > 0; at = (at - 1) / 2 {
				sibling := at + 1
				if at%2 == 0 {
					sibling = at - 1
				}
				tree.Leaves[i].Proof = append(tree.Leaves[i].Proof, heap[sibling])
			}
		}
	}
	roles := make([][32]byte, len(tree.Leaves))
	for i, leaf := range tree.Leaves {
		roles[i] = leaf.Hash
	}
	call(fixture.Account, aa, "configure", tree.Root, roles, target)
	eABI := executor.NewRecipeExecutor()
	serviceSeedTransaction(t, ctx, client, owner, &target, eABI.PackSetCaller(owner))
	recipe, err := recipes.DecodeRecipe(fixture.Recipe)
	if err != nil {
		t.Fatal(err)
	}

	if native {
		recipe.Steps = append(recipe.Steps, issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{nativeLeaf.Hash}, [][]*big.Int{{}}))
	}
	serviceSeedTransaction(t, ctx, client, owner, &target, eABI.PackSetRecipe(recipe.Queries, recipe.Steps))
	var permission map[string]any
	if err = json.Unmarshal(raw, &permission); err != nil {
		t.Fatal(err)
	}
	permission["chainId"] = 31337
	if native {
		entries := make([]any, len(tree.Leaves))
		for i, leaf := range tree.Leaves {
			proof := make([]string, len(leaf.Proof))
			for j, hash := range leaf.Proof {
				proof[j] = hash.Hex()
			}
			entries[i] = map[string]any{"hash": leaf.Hash.Hex(), "target": leaf.Target.Hex(), "value": "zero", "data": hexutil.Encode(leaf.Data), "dataMask": hexutil.Encode(leaf.DataMask), "proof": proof}
		}
		permission["root"], permission["calls"] = tree.Root.Hex(), entries
	}
	permissionRaw, _ := json.Marshal(permission)
	permissionRaw = servicePermissionCertificate(t, permissionRaw, fixture.Account, fixture.Account)
	curator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(permissionRaw) }))
	defer curator.Close()
	var fetches atomic.Int32
	var rewardRows atomic.Value
	rows := make([]wire.MerklReward, len(fixture.Rewards))
	for i, reward := range fixture.Rewards {
		rows[i] = wire.MerklReward{Token: wire.MerklToken{Address: reward.Token.Hex(), Decimals: 18}, Amount: reward.Cumulative, Claimed: "0", Pending: "0", Proofs: reward.Proof}
	}
	rewardRows.Store(rows)
	rewardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches.Add(1)
		_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 31337}, Rewards: rewardRows.Load().([]wire.MerklReward)}})
	}))
	defer rewardServer.Close()
	rewrite, _ := url.Parse(rewardServer.URL)
	httpClient := &http.Client{Transport: serviceRewriteTransport{host: "api.merkl.xyz", target: rewrite}}
	cfg := testCLIConfig(t.TempDir())
	cfg.Mode = "simulate"
	cfg.Chain.RPCURL = endpoint
	cfg.Curator.URL = curator.URL
	cfg.Strategies[0] = strings.ToLower(target.Hex())
	cfg.Signer.Address = ""
	path := writeCLIConfig(t, t.TempDir(), cfg)
	// The root/custody/ledger guards are the exact frontend-generated bytes. A
	// distributor paying half its ledger delta must fail original-code simulation.
	if !native {
		call(fixture.Distributor, da, "setPayoutBps", big.NewInt(5000))
		if err = runBotWithHTTP(ctx, path, true, true, new(bytes.Buffer), httpClient); err == nil || !strings.Contains(err.Error(), "full recipe simulation failed") {
			t.Fatal("frontend exact-cash-delta guard did not reject underpayment", err)
		}
		call(fixture.Distributor, da, "setPayoutBps", big.NewInt(10000))
	}
	before := fetches.Load()
	var output bytes.Buffer
	if err = runBotWithHTTP(ctx, path, true, true, &output, httpClient); err != nil {
		t.Fatal(err)
	}
	var plans map[string]keeper.Plan
	if err = json.Unmarshal(output.Bytes(), &plans); err != nil {
		t.Fatal(err)
	}
	plan := plans[cfg.Strategies[0]]
	wantDeltaB := "200"
	if native {
		wantDeltaB = "0"
	}
	if len(plan.Outputs) != 6 || len(plan.Outputs[1].Actions) != 2 || plan.Outputs[1].Inputs[0] != "100" || plan.Outputs[1].Inputs[1] != "0" || plan.Outputs[1].Inputs[2] != "200" || plan.Outputs[1].Inputs[3] != wantDeltaB || fetches.Load()-before != 1 {
		t.Fatalf("partial guarded reward plan: %+v HTTP=%d", plan.Outputs, fetches.Load()-before)
	}
	tokens := erc20.NewERC20()
	dist := distributor.NewMerklDistributor()
	for _, reward := range fixture.Rewards {
		balance, err := client.CallContract(ctx, ethereum.CallMsg{To: &reward.Token, Data: tokens.PackBalanceOf(fixture.Account)}, nil)
		if err != nil || new(big.Int).SetBytes(balance).Sign() != 0 {
			t.Fatal("simulation changed token state", err)
		}
	}
	serviceSeedTransaction(t, ctx, client, owner, &target, plan.Calldata)
	for i, reward := range fixture.Rewards {
		balance, err := client.CallContract(ctx, ethereum.CallMsg{To: &reward.Token, Data: tokens.PackBalanceOf(fixture.Account)}, nil)
		want := int64(0)
		if i == 1 {
			want = 200
			if native {
				want = 20
			}
		}
		if err != nil || new(big.Int).SetBytes(balance).Int64() != want {
			t.Fatalf("token %s balance differs: %x %v", reward.Token, balance, err)
		}
		raw, err := client.CallContract(ctx, ethereum.CallMsg{To: &fixture.Distributor, Data: dist.PackClaimed(fixture.Account, reward.Token)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		claimed, err := dist.UnpackClaimed(raw)
		if err != nil || claimed.Amount.String() != reward.Cumulative {
			t.Fatalf("ledger differs from cumulative: %+v %v", claimed, err)
		}
	}
	if native {
		return
	}
	// A later valid distributor root must work with the identical scheduled
	// recipe/permission artifact when its token and proof layout still fit.
	amounts := []*big.Int{big.NewInt(150), big.NewInt(300)}
	leaves := make([]common.Hash, len(fixture.Rewards))
	for i, reward := range fixture.Rewards {
		leaves[i] = crypto.Keccak256Hash(common.LeftPadBytes(fixture.Account.Bytes(), 32), common.LeftPadBytes(reward.Token.Bytes(), 32), common.LeftPadBytes(amounts[i].Bytes(), 32))
	}
	a, b := leaves[0], leaves[1]
	if a.Cmp(b) > 0 {
		a, b = b, a
	}
	newRoot := crypto.Keccak256Hash(a[:], b[:])
	call(fixture.Distributor, da, "setRoot", newRoot)
	newRows := make([]wire.MerklReward, len(rows))
	for i, reward := range rows {
		newRows[i] = reward
		newRows[i].Amount = amounts[i].String()
		newRows[i].Proofs = []string{leaves[1-i].Hex()}
	}
	badRows := append([]wire.MerklReward(nil), newRows...)
	badRows[0].Proofs = rows[0].Proofs
	rewardRows.Store(badRows)
	if err = runBotWithHTTP(ctx, path, true, true, new(bytes.Buffer), httpClient); err == nil || !strings.Contains(err.Error(), "proof does not match effective on-chain root") {
		t.Fatal("rollover accepted a proof for another root", err)
	}
	rewardRows.Store(newRows)
	call(fixture.Distributor, da, "setRootAfterClaim", fixture.RewardRoot)
	if err = runBotWithHTTP(ctx, path, true, true, new(bytes.Buffer), httpClient); err == nil || !strings.Contains(err.Error(), "full recipe simulation failed") {
		t.Fatal("frontend guard accepted a root change during claim", err)
	}
	call(fixture.Distributor, da, "setRootAfterClaim", common.Hash{})
	output.Reset()
	if err = runBotWithHTTP(ctx, path, true, true, &output, httpClient); err != nil {
		t.Fatal("scheduled recipe rejected a valid later root", err)
	}
	if err = json.Unmarshal(output.Bytes(), &plans); err != nil {
		t.Fatal(err)
	}
	plan = plans[cfg.Strategies[0]]
	for i, want := range []string{"150", "50", "300", "100"} {
		if plan.Outputs[1].Inputs[uint(i)] != want {
			t.Fatalf("later-root cumulative/delta differs: %v", plan.Outputs[1].Inputs)
		}
	}
	serviceSeedTransaction(t, ctx, client, owner, &target, plan.Calldata)
	for i, reward := range fixture.Rewards {
		balance, err := client.CallContract(ctx, ethereum.CallMsg{To: &reward.Token, Data: tokens.PackBalanceOf(fixture.Account)}, nil)
		if err != nil || new(big.Int).SetBytes(balance).Cmp([]*big.Int{big.NewInt(50), big.NewInt(300)}[i]) != 0 {
			t.Fatalf("later-root token balance differs: %x %v", balance, err)
		}
		raw, err := client.CallContract(ctx, ethereum.CallMsg{To: &fixture.Distributor, Data: dist.PackClaimed(fixture.Account, reward.Token)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		claimed, err := dist.UnpackClaimed(raw)
		if err != nil || claimed.Amount.Cmp(amounts[i]) != 0 {
			t.Fatalf("later-root ledger differs: %+v %v", claimed, err)
		}
	}
}
