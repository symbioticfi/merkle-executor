package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAnvilNativeRedemptionsAtomicityAndInProcessReorg(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("set MERKLE_ANVIL_TEST=1 for native mixed issuer execution")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	// Only this owned, unforked local process uses the canonical route addresses.
	if err := client.Client().CallContext(ctx, nil, "anvil_setChainId", 1); err != nil {
		t.Fatal(err)
	}
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil || len(owners) == 0 {
		t.Fatal("local account unavailable", err)
	}
	owner := owners[0]
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	caller := crypto.PubkeyToAddress(key.PublicKey)
	if err = client.Client().CallContext(ctx, nil, "anvil_setBalance", caller, "0xde0b6b3a7640000"); err != nil {
		t.Fatal(err)
	}
	accountArtifact, executorArtifact := loadServiceArtifact(t, "FixtureAccount"), loadServiceArtifact(t, "RecipeExecutor")
	assetArtifact, shareArtifact := loadServiceArtifact(t, "FixtureToken"), loadIssuerArtifact(t, "FixtureShare")
	issuerArtifact := loadIssuerArtifact(t, "FixtureAsyncIssuer")
	deploy := func(artifact serviceArtifact, args ...any) common.Address {
		return serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(artifact.Bytecode), servicePack(t, artifact, "", args...)...)).ContractAddress
	}
	install := func(target, deployed common.Address) {
		code, err := client.CodeAt(ctx, deployed, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err = client.Client().CallContext(ctx, nil, "anvil_setCode", target, hexutil.Bytes(code)); err != nil {
			t.Fatal(err)
		}
	}
	call := func(from, target common.Address, artifact serviceArtifact, method string, args ...any) {
		serviceSeedTransaction(t, ctx, client, from, &target, servicePack(t, artifact, method, args...))
	}
	read := func(target common.Address, artifact serviceArtifact, method string, args ...any) *big.Int {
		raw, err := client.CallContract(ctx, ethereum.CallMsg{To: &target, Data: servicePack(t, artifact, method, args...)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		values, err := artifact.ABI.Unpack(method, raw)
		if err != nil || len(values) != 1 {
			t.Fatal("decode issuer fixture", err)
		}
		return values[0].(*big.Int)
	}
	waitReceipt := func(hash common.Hash) *types.Receipt {
		t.Helper()
		poll := time.NewTicker(10 * time.Millisecond)
		defer poll.Stop()
		for {
			receipt, err := client.TransactionReceipt(ctx, hash)
			if err == nil {
				return receipt
			}
			if !errors.Is(err, ethereum.NotFound) {
				t.Fatal(err)
			}
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-poll.C:
			}
		}
	}
	asset := common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	share := common.HexToAddress("0x5a0F93D040De44e78F251b03c43be9CF317Dcf64")
	issuer := common.HexToAddress("0x4880799eE5200fC58DA299e965df644fBf46780B")
	mglobal := common.HexToAddress("0x7433806912Eae67919e66aea853d46Fa0aef98A8")
	midasVault := common.HexToAddress("0x1e0fd66753198c7b8bA64edEe8d41D8628Bf20D7")
	install(asset, deploy(assetArtifact))
	install(share, deploy(shareArtifact))
	install(mglobal, deploy(shareArtifact))
	deployedIssuer := deploy(issuerArtifact, asset, share)
	install(issuer, deployedIssuer)
	// This runtime rejects redeemRequest: an accidentally active zero fallback
	// must fail execution instead of silently succeeding against an empty address.
	install(midasVault, deployedIssuer)
	account := deploy(accountArtifact)
	executor := deploy(executorArtifact, owner, account)
	call(owner, share, shareArtifact, "mint", account, big.NewInt(50))
	call(owner, asset, assetArtifact, "mint", issuer, big.NewInt(80))
	if err = client.Client().CallContext(ctx, nil, "anvil_setBalance", account, "0xde0b6b3a7640000"); err != nil {
		t.Fatal(err)
	}
	if err = client.Client().CallContext(ctx, nil, "anvil_impersonateAccount", account); err != nil {
		t.Fatal(err)
	}
	call(account, share, shareArtifact, "approve", issuer, big.NewInt(40))
	call(account, issuer, issuerArtifact, "requestRedeem", big.NewInt(40), account, account)
	if err = client.Client().CallContext(ctx, nil, "anvil_stopImpersonatingAccount", account); err != nil {
		t.Fatal(err)
	}
	call(owner, issuer, issuerArtifact, "finalize", account)
	tokens, executors := erc20.NewERC20(), recipeexecutor.NewRecipeExecutor()
	midasArtifact := serviceArtifact{}
	midasArtifact.ABI, err = abi.JSON(strings.NewReader(`[{"type":"function","name":"redeemRequest","inputs":[{"name":"tokenOut","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[]}]`))
	if err != nil {
		t.Fatal(err)
	}
	leaf := func(target common.Address, data []byte, offset int) recipes.Leaf {
		mask := make([]byte, len(data))
		for at := offset; at < offset+32; at++ {
			mask[at] = 255
		}
		value := recipes.Leaf{Target: target, Value: new(big.Int), Data: data, DataMask: mask}
		value.Hash, err = recipes.HashLeaf(value)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	leaves := []recipes.Leaf{
		leaf(issuer, servicePack(t, issuerArtifact, "redeem", new(big.Int), account, account), 4),
		leaf(issuer, servicePack(t, issuerArtifact, "requestRedeem", new(big.Int), account, account), 4),
		leaf(share, tokens.PackApprove(issuer, new(big.Int)), 36),
		leaf(midasVault, servicePack(t, midasArtifact, "redeemRequest", asset, new(big.Int)), 36),
	}
	pair := func(a, b common.Hash) common.Hash {
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		return crypto.Keccak256Hash(a[:], b[:])
	}
	// The backend certificate commits the complete sorted, reverse-leaf tree.
	for i := range leaves {
		for j := i + 1; j < len(leaves); j++ {
			if leaves[i].Hash.Cmp(leaves[j].Hash) > 0 {
				leaves[i], leaves[j] = leaves[j], leaves[i]
			}
		}
	}
	branches := []common.Hash{pair(leaves[0].Hash, leaves[1].Hash), pair(leaves[2].Hash, leaves[3].Hash)}
	root := pair(branches[0], branches[1])
	entries := make([]any, len(leaves))
	hashes := make([][32]byte, len(leaves))
	for i := range leaves {
		leaves[i].Proof = []common.Hash{leaves[i^1].Hash, branches[1-i/2]}
		v := leaves[i]
		entries[i] = map[string]any{"hash": v.Hash.Hex(), "target": v.Target.Hex(), "value": "zero", "data": hexutil.Encode(v.Data), "dataMask": hexutil.Encode(v.DataMask), "proof": []string{v.Proof[0].Hex(), v.Proof[1].Hex()}}
		hashes[i] = v.Hash
	}
	findLeaf := func(target common.Address, selector []byte) common.Hash {
		for _, value := range leaves {
			if value.Target == target && bytes.Equal(value.Data[:4], selector[:4]) {
				return value.Hash
			}
		}
		t.Fatal("missing recipe leaf")
		return common.Hash{}
	}
	call(owner, account, accountArtifact, "configure", root, hashes, executor)
	serviceSeedTransaction(t, ctx, client, owner, &executor, executors.PackSetCaller(caller))
	result := func(index int64) *big.Int {
		return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(index))
	}
	query := func(target common.Address, data []byte) []byte {
		return issuerStep(t, 0, []string{"address", "bytes", "uint256[]", "uint256"}, target, data, []*big.Int{}, new(big.Int))[1:]
	}
	manage := func(target common.Address, data []byte, offset, source int64) []byte {
		binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(offset), 224), result(source))
		return issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, result(source), [][32]byte{findLeaf(target, data)}, [][]*big.Int{{binding}})
	}
	recipe := recipes.Recipe{Queries: [][]byte{
		query(issuer, servicePack(t, issuerArtifact, "maxRedeem", account)),
		query(share, tokens.PackBalanceOf(account)),
		query(mglobal, tokens.PackBalanceOf(account)),
		query(asset, tokens.PackBalanceOf(account)),
	}, Steps: [][]byte{
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(0)),
		manage(issuer, servicePack(t, issuerArtifact, "redeem", new(big.Int), account, account), 4, 0),
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(0)),
		manage(issuer, servicePack(t, issuerArtifact, "redeem", new(big.Int), account, account), 4, 2),
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(1)),
		issuerStep(t, recipes.Approve, []string{"uint256", "address", "address", "bytes32"}, result(4), share, issuer, findLeaf(share, tokens.PackApprove(issuer, new(big.Int)))),
		manage(issuer, servicePack(t, issuerArtifact, "requestRedeem", new(big.Int), account, account), 4, 4),
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(2)),
		manage(midasVault, servicePack(t, midasArtifact, "redeemRequest", asset, new(big.Int)), 36, 7),
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(3)),
		issuerStep(t, recipes.LessOrEqual, []string{"uint256", "uint256"}, big.NewInt(80), result(9)),
	}}
	serviceSeedTransaction(t, ctx, client, owner, &executor, executors.PackSetRecipe(recipe.Queries, recipe.Steps))
	raw, err := json.Marshal(map[string]any{"root": root.Hex(), "chainId": 1, "account": account.Hex(), "leaves": entries})
	if err != nil {
		t.Fatal(err)
	}
	certificate := servicePermissionCertificate(t, raw, account, account)
	var curatorUnavailable atomic.Bool
	var curatorCalls atomic.Int32
	curator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curatorCalls.Add(1)
		if r.URL.Path != "/api/v1/merkle/"+strings.ToLower(account.Hex())+"/"+root.Hex() || r.URL.RawQuery != "" {
			t.Error("wrong native permission request")
		}
		if curatorUnavailable.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(certificate)
	}))
	defer curator.Close()
	cfg := testCLIConfig(t.TempDir())
	cfg.Mode, cfg.Chain.ChainID, cfg.Chain.RPCURL = "send", 1, endpoint
	cfg.Signer.Address = caller.Hex()
	cfg.Curator.URL = curator.URL + "/api/v1"
	cfg.Strategies = []string{strings.ToLower(executor.Hex())}
	path := writeCLIConfig(t, t.TempDir(), cfg)
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(key)))
	assertBalances := func(claimed bool) {
		t.Helper()
		wantShares, wantRedeem, wantCash, wantPending, wantRequests, wantClaims := int64(10), int64(40), int64(0), int64(0), int64(1), int64(0)
		if claimed {
			wantShares, wantRedeem, wantCash, wantPending, wantRequests, wantClaims = 0, 0, 80, 10, 2, 1
		}
		for _, check := range []struct {
			target   common.Address
			artifact serviceArtifact
			method   string
			args     []any
			want     int64
		}{
			{share, shareArtifact, "balanceOf", []any{account}, wantShares},
			{share, shareArtifact, "allowance", []any{account, issuer}, 0},
			{issuer, issuerArtifact, "maxRedeem", []any{account}, wantRedeem},
			{issuer, issuerArtifact, "pendingShares", []any{account}, wantPending},
			{issuer, issuerArtifact, "requestCount", nil, wantRequests},
			{issuer, issuerArtifact, "claimCount", nil, wantClaims},
			{asset, assetArtifact, "balanceOf", []any{account}, wantCash},
		} {
			if got := read(check.target, check.artifact, check.method, check.args...); got.Cmp(big.NewInt(check.want)) != 0 {
				t.Fatalf("%s = %s, want %d", check.method, got, check.want)
			}
		}
	}
	// The last assertion fails after the claim, allowance and request. Simulation
	// must stop daemon admission; a deliberately mined revert must undo them all.
	failedRecipe := recipes.CloneRecipe(recipe)
	failedRecipe.Steps[10] = issuerStep(t, recipes.LessOrEqual, []string{"uint256", "uint256"}, big.NewInt(81), result(9))
	serviceSeedTransaction(t, ctx, client, owner, &executor, executors.PackSetRecipe(failedRecipe.Queries, failedRecipe.Steps))
	if err = runBot(ctx, path, true); !recipes.BatchRejected(err) {
		t.Fatal("daemon did not reject the final output guard during simulation", err)
	}

	failedCalldata, _, err := recipes.Assemble(failedRecipe, &recipes.Tree{Root: root, Leaves: leaves}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Client().CallContext(ctx, nil, "anvil_impersonateAccount", caller); err != nil {
		t.Fatal(err)
	}
	var failedHash common.Hash
	if err = client.Client().CallContext(ctx, &failedHash, "eth_sendTransaction", map[string]any{"from": caller, "to": executor, "gas": hexutil.Uint64(12_000_000), "data": hexutil.Bytes(failedCalldata)}); err != nil {
		t.Fatal(err)
	}
	reverted := waitReceipt(failedHash)
	if reverted.Status != types.ReceiptStatusFailed || len(reverted.Logs) != 0 {
		t.Fatal("late output guard did not revert all effects and logs")
	}
	assertBalances(false)
	if err = client.Client().CallContext(ctx, nil, "anvil_stopImpersonatingAccount", caller); err != nil {
		t.Fatal(err)
	}
	serviceSeedTransaction(t, ctx, client, owner, &executor, executors.PackSetRecipe(recipe.Queries, recipe.Steps))
	var output bytes.Buffer
	if err = planBot(ctx, path, &output); err != nil {
		t.Fatal("mixed native recipe failed preparation", err)
	}
	var response struct {
		Plan keeper.ExecutionPlan `json:"plan"`
	}
	if err = json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	plan := response.Plan.Members[0].Plan
	if len(plan.Outputs) != 5 || len(plan.Calldata) == 0 {
		t.Fatal("mixed native plan missing automatic providers or execution")
	}
	for _, group := range plan.Outputs {
		for _, out := range group {
			if len(out.Actions)+len(out.Inputs)+len(out.SkipSteps) != 0 {
				t.Fatal("native issuer action acquired provider mutation ownership")
			}
		}
	}
	simulated, err := client.CallContract(ctx, ethereum.CallMsg{From: caller, To: &executor, Data: plan.Calldata}, nil)
	if err != nil {
		t.Fatal(err)
	}
	values, err := executors.UnpackExecute(simulated)
	if err != nil || len(values) != len(recipe.Steps) || values[0].Int64() != 40 || values[2].Sign() != 0 || values[4].Int64() != 10 || values[7].Sign() != 0 || values[9].Int64() != 80 {
		t.Fatal("mixed native execution did not read its preceding claim effects", err)
	}
	assertBalances(false)

	// Retain one RAM-owned attempt while an inclusion is orphaned and reincluded.
	if err = client.Client().CallContext(ctx, nil, "evm_setAutomine", false); err != nil {
		t.Fatal(err)
	}
	var checkpoint string
	if err = client.Client().CallContext(ctx, &checkpoint, "evm_snapshot"); err != nil {
		t.Fatal(err)
	}
	worker := serviceKeeper(t, ctx, path, nil)
	if err = worker.Tick(ctx); err != nil || !worker.Pending() {
		t.Fatal("native submission did not retain RAM ownership", err)
	}
	signed := servicePendingTransaction(t, ctx, client, caller)
	if !bytes.Equal(signed.Data(), plan.Calldata) {
		t.Fatal("submission changed prepared calldata")
	}
	if err = client.Client().CallContext(ctx, nil, "evm_mine"); err != nil {
		t.Fatal(err)
	}
	oldReceipt := waitReceipt(signed.Hash())
	if oldReceipt.Status != types.ReceiptStatusSuccessful {
		t.Fatal("native transaction did not mine")
	}
	assertBalances(true)
	if err = worker.Tick(ctx); err != nil || !worker.Pending() {
		t.Fatal("unconfirmed inclusion released ownership", err)
	}
	var restored bool
	if err = client.Client().CallContext(ctx, &restored, "evm_revert", checkpoint); err != nil || !restored {
		t.Fatal("local receipt reorg failed", err)
	}
	if _, err = client.TransactionReceipt(ctx, signed.Hash()); !errors.Is(err, ethereum.NotFound) {
		t.Fatal("orphaned receipt remained visible", err)
	}
	assertBalances(false)
	if err = worker.Tick(ctx); err != nil || !worker.Pending() {
		t.Fatal("reorg lost RAM ownership", err)
	}
	if servicePendingTransaction(t, ctx, client, caller).Hash() != signed.Hash() {
		t.Fatal("reorg changed signed attempt")
	}
	if err = client.Client().CallContext(ctx, nil, "evm_setNextBlockTimestamp", oldReceipt.BlockNumber.Uint64()+plan.Snapshot.Timestamp+10); err != nil {
		t.Fatal(err)
	}
	if err = client.Client().CallContext(ctx, nil, "evm_mine"); err != nil {
		t.Fatal(err)
	}
	canonical := waitReceipt(signed.Hash())
	if canonical.BlockHash == oldReceipt.BlockHash || canonical.Status != types.ReceiptStatusSuccessful {
		t.Fatal("exact transaction was not reincluded in a different canonical block")
	}
	for range 3 {
		if err = client.Client().CallContext(ctx, nil, "evm_mine"); err != nil {
			t.Fatal(err)
		}
	}
	curatorUnavailable.Store(true)
	beforeConfirmation := curatorCalls.Load()
	if err = worker.Tick(ctx); err != nil || worker.Pending() {
		t.Fatal("canonical confirmation did not finish native work", err)
	}
	if curatorCalls.Load() != beforeConfirmation {
		t.Fatal("active attempt fetched new permission data")
	}
	assertBalances(true)
	if nonce, e := client.NonceAt(ctx, caller, nil); e != nil || nonce != signed.Nonce()+1 {
		t.Fatal("reconciliation duplicated native execution", e)
	}
	// A native-only recipe with zero gates is idle even though the previous claim
	// left settlement cash and a new request remains outstanding.
	zeroRecipe := recipes.Recipe{Queries: recipe.Queries, Steps: [][]byte{
		recipe.Steps[0], recipe.Steps[1],
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(1)),
		manage(issuer, servicePack(t, issuerArtifact, "requestRedeem", new(big.Int), account, account), 4, 2),
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(2)),
		manage(midasVault, servicePack(t, midasArtifact, "redeemRequest", asset, new(big.Int)), 36, 4),
	}}
	if err = client.Client().CallContext(ctx, nil, "evm_setAutomine", true); err != nil {
		t.Fatal(err)
	}
	serviceSeedTransaction(t, ctx, client, owner, &executor, executors.PackSetRecipe(zeroRecipe.Queries, zeroRecipe.Steps))
	curatorUnavailable.Store(false)
	if err = planBot(ctx, path, new(bytes.Buffer)); !errors.Is(err, provider.ErrNotReady) {
		t.Fatal("zero native actions did not become idle", err)
	}
}
