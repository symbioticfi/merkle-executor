package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	tokenbinding "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	executorbinding "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/chain"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/threef"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAnvilSingleThreeFClaimBatchesReadyRequests(t *testing.T) {
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
	raw, err := os.ReadFile(filepath.Join("..", "..", "sdk", "providers", "threef", "testdata", "frontend-claim.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Account, Factory common.Address
		Recipe           hexutil.Bytes
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	recipe, err := recipes.DecodeRecipe(fixture.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	aa := loadGuardedMerklArtifact(t, "FixtureGuardedAccount")
	ca := loadGuardedMerklArtifact(t, "RecipeCapability")
	fa := loadGuardedMerklArtifact(t, "FixtureThreeFFactory")
	ra := loadGuardedMerklArtifact(t, "FixtureThreeFRequest")
	ta := loadServiceArtifact(t, "FixtureToken")
	ea := loadServiceArtifact(t, "RecipeExecutor")
	install := func(address common.Address, a serviceArtifact) {
		t.Helper()
		deployed := serviceSeedTransaction(t, ctx, client, owner, nil, a.Bytecode).ContractAddress
		code, err := client.CodeAt(ctx, deployed, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err = client.Client().CallContext(ctx, nil, "anvil_setCode", address, hexutil.Bytes(code)); err != nil {
			t.Fatal(err)
		}
	}
	call := func(address common.Address, a serviceArtifact, name string, args ...any) {
		t.Helper()
		serviceSeedTransaction(t, ctx, client, owner, &address, servicePack(t, a, name, args...))
	}
	install(fixture.Account, aa)
	install(fixture.Factory, fa)
	capability := serviceSeedTransaction(t, ctx, client, owner, nil, ca.Bytecode).ContractAddress
	token := serviceSeedTransaction(t, ctx, client, owner, nil, ta.Bytecode).ContractAddress
	call(fixture.Account, aa, "setCapability", capability)
	requests := make([]common.Address, 4)
	for i := range requests {
		requests[i] = crypto.CreateAddress(fixture.Factory, 7+3*uint64(i))
		install(requests[i], ra)
		call(fixture.Factory, fa, "setRequest", requests[i])
		account := fixture.Account
		if i == 3 {
			account = owner
		}
		call(requests[i], ra, "configure", account, token, big.NewInt(int64(i+1)), i != 1)
	}
	target := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(ea.Bytecode), servicePack(t, ea, "", owner, fixture.Account)...)).ContractAddress
	call(fixture.Account, aa, "configure", tree.Root, [][32]byte{tree.Leaves[0].Hash}, target)
	executor := executorbinding.NewRecipeExecutor()
	serviceSeedTransaction(t, ctx, client, owner, &target, executor.PackSetCaller(owner))
	serviceSeedTransaction(t, ctx, client, owner, &target, executor.PackSetRecipe(recipe.Queries, recipe.Steps))
	// Keep the fixture's exact permission bytes, with only its test-chain identity changed.
	var document map[string]any
	_ = json.Unmarshal(raw, &document)
	document["chainId"] = 31337
	raw, _ = json.Marshal(document)
	path := filepath.Join(t.TempDir(), "tree.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	claimProvider, err := threef.NewAutomatic(provider.Deps{Chain: client})
	if err != nil {
		t.Fatal(err)
	}
	strategy := keeper.Strategy{Config: keeper.Config{Account: fixture.Account.Hex(), Executor: target.Hex(), Tree: path}, Providers: []keeper.Provider{claimProvider}}
	backend, err := chain.Dial(ctx, endpoint, "", big.NewInt(31337))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	planner := keeper.Planner{Chain: backend, ChainID: big.NewInt(31337), Caller: owner}
	plan, err := planner.Prepare(ctx, strategy)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Outputs[0]) != 2 {
		t.Fatal("ready requests were not batched")
	}
	// A request becoming unavailable invalidates the complete batch. Its first
	// claim must not leak state when a later claim reverts.
	call(requests[2], ra, "configure", fixture.Account, token, big.NewInt(3), false)
	if _, err := client.CallContract(ctx, ethereum.CallMsg{From: owner, To: &target, Data: plan.Calldata}, nil); err == nil {
		t.Fatal("pending request accepted")
	}
	before, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: tokenbinding.NewERC20().PackBalanceOf(fixture.Account)}, nil)
	if err != nil || new(big.Int).SetBytes(before).Sign() != 0 {
		t.Fatal("failed batch changed account balance")
	}
	call(requests[2], ra, "configure", fixture.Account, token, big.NewInt(3), true)
	receipt := serviceSeedTransaction(t, ctx, client, owner, &target, plan.Calldata)
	tb := tokenbinding.NewERC20()
	balanceData := tb.PackBalanceOf(fixture.Account)
	balance := func() *big.Int {
		t.Helper()
		raw, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: balanceData}, nil)
		if err != nil {
			t.Fatal(err)
		}
		n, err := tb.UnpackBalanceOf(raw)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	if balance().Cmp(big.NewInt(4)) != 0 {
		t.Fatal("ready request payouts not received exactly once")
	}
	// The native request prevents a duplicate burn; the failed multicall cannot change cash.
	if _, err := client.CallContract(ctx, ethereum.CallMsg{From: owner, To: &target, Data: plan.Calldata}, nil); err == nil {
		t.Fatal("duplicate claims succeeded")
	}
	if balance().Cmp(big.NewInt(4)) != 0 {
		t.Fatal("failed batch changed cash")
	}
	header, err := client.HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := plan.Snapshot
	snapshot.Execution = &provider.Execution{TxHash: receipt.TxHash, BlockNumber: receipt.BlockNumber, BlockHash: header.Hash(), Logs: nil}
	for _, output := range plan.Outputs[0] {
		if _, err = claimProvider.Advance(ctx, snapshot, output, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = planner.Prepare(ctx, strategy); err == nil {
		t.Fatal("already claimed and pending requests were resubmitted")
	}
}
