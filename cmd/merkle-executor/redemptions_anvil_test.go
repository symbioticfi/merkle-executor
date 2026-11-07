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
	"path/filepath"
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/internal/chain"
	"github.com/symbioticfi/merkle-executor/internal/curator"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/signer"
	"github.com/symbioticfi/merkle-executor/internal/txmanager"
	"github.com/symbioticfi/merkle-executor/sdk/provider"

	async "github.com/symbioticfi/merkle-executor/api/bindings/asyncredeemvault"
	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/providers/redemptions"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func loadIssuerArtifact(t *testing.T, name string) serviceArtifact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "providers", "redemptions", "testdata", name+".json"))
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

func issuerStep(t *testing.T, operation uint8, names []string, values ...any) []byte {
	t.Helper()
	args := make(abi.Arguments, len(names))
	for i, name := range names {
		value, err := abi.NewType(name, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		args[i] = abi.Argument{Type: value}
	}
	payload, err := args.Pack(values...)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte{operation}, payload...)
}

func TestAnvilRedemptionRequestThenClaim(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("set MERKLE_ANVIL_TEST=1 for full local issuer lifecycle integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil || len(owners) == 0 {
		t.Fatalf("local fixture account unavailable: %v", err)
	}
	owner := owners[0]
	accountArtifact := loadServiceArtifact(t, "FixtureAccount")
	outputArtifact := loadServiceArtifact(t, "FixtureToken")
	executorArtifact := loadServiceArtifact(t, "RecipeExecutor")
	shareArtifact := loadIssuerArtifact(t, "FixtureShare")
	issuerArtifact := loadIssuerArtifact(t, "FixtureAsyncIssuer")
	account := serviceSeedTransaction(t, ctx, client, owner, nil, accountArtifact.Bytecode).ContractAddress
	asset := serviceSeedTransaction(t, ctx, client, owner, nil, outputArtifact.Bytecode).ContractAddress
	share := serviceSeedTransaction(t, ctx, client, owner, nil, shareArtifact.Bytecode).ContractAddress
	issuer := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(issuerArtifact.Bytecode), servicePack(t, issuerArtifact, "", asset, share)...)).ContractAddress
	deployExecutor := func() common.Address {
		return serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(executorArtifact.Bytecode), servicePack(t, executorArtifact, "", owner, account)...)).ContractAddress
	}
	requestExecutor, claimExecutor := deployExecutor(), deployExecutor()
	serviceSeedTransaction(t, ctx, client, owner, &share, servicePack(t, shareArtifact, "setVault", asset, issuer))
	serviceSeedTransaction(t, ctx, client, owner, &share, servicePack(t, shareArtifact, "mint", account, big.NewInt(100)))
	serviceSeedTransaction(t, ctx, client, owner, &asset, servicePack(t, outputArtifact, "mint", issuer, big.NewInt(200)))

	// Only a new random key funded on this ephemeral local chain signs bot calls.
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	caller := crypto.PubkeyToAddress(key.PublicKey)
	if err := client.Client().CallContext(ctx, nil, "anvil_setBalance", caller, "0xde0b6b3a7640000"); err != nil {
		t.Fatal(err)
	}
	tokens, vaults, executors := erc20.NewERC20(), async.NewAsyncRedeemVault(), executor.NewRecipeExecutor()
	newLeaf := func(target common.Address, data []byte, offset int) recipes.Leaf {
		mask := make([]byte, len(data))
		for i := offset; i < offset+32; i++ {
			mask[i] = 0xff
		}
		leaf := recipes.Leaf{Target: target, Value: new(big.Int), Data: data, DataMask: mask}
		leaf.Hash, err = recipes.HashLeaf(leaf)
		if err != nil {
			t.Fatal(err)
		}
		return leaf
	}
	leaves := []recipes.Leaf{
		newLeaf(share, tokens.PackApprove(issuer, new(big.Int)), 36),
		newLeaf(issuer, vaults.PackRequestRedeem(new(big.Int), account, account), 4),
		newLeaf(issuer, vaults.PackRedeem(new(big.Int), account, account), 4),
	}
	pair := func(a, b common.Hash) common.Hash {
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		return crypto.Keccak256Hash(a[:], b[:])
	}
	branch := pair(leaves[1].Hash, leaves[2].Hash)
	root := pair(leaves[0].Hash, branch)
	leaves[0].Proof = []common.Hash{branch}
	leaves[1].Proof = []common.Hash{leaves[2].Hash, leaves[0].Hash}
	leaves[2].Proof = []common.Hash{leaves[1].Hash, leaves[0].Hash}
	for _, address := range []common.Address{requestExecutor, claimExecutor} {
		serviceSeedTransaction(t, ctx, client, owner, &account, servicePack(t, accountArtifact, "configure", root, [][32]byte{leaves[0].Hash, leaves[1].Hash, leaves[2].Hash}, address))
		serviceSeedTransaction(t, ctx, client, owner, &address, executors.PackSetCaller(caller))
	}
	manage := func(leaf common.Hash) []byte {
		return issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaf}, [][]*big.Int{{}})
	}
	requestRecipe := recipes.Recipe{Steps: [][]byte{
		issuerStep(t, recipes.Approve, []string{"uint256", "address", "address", "bytes32"}, big.NewInt(100), share, issuer, leaves[0].Hash),
		manage(leaves[1].Hash),
	}}
	claimRecipe := recipes.Recipe{Steps: [][]byte{manage(leaves[2].Hash)}}
	serviceSeedTransaction(t, ctx, client, owner, &requestExecutor, executors.PackSetRecipe(requestRecipe.Queries, requestRecipe.Steps))
	serviceSeedTransaction(t, ctx, client, owner, &claimExecutor, executors.PackSetRecipe(claimRecipe.Queries, claimRecipe.Steps))

	directory := t.TempDir()
	cfg := testCLIConfig(directory)
	cfg.Mode, cfg.Chain.RPCURL, cfg.Signer.Address = "send", endpoint, caller.Hex()
	permissionLeaves := make([]any, len(leaves))
	for i, leaf := range leaves {
		proof := make([]string, len(leaf.Proof))
		for j, sibling := range leaf.Proof {
			proof[j] = sibling.Hex()
		}
		permissionLeaves[i] = map[string]any{"hash": leaf.Hash.Hex(), "target": leaf.Target.Hex(), "value": "zero", "data": hexutil.Encode(leaf.Data), "dataMask": hexutil.Encode(leaf.DataMask), "proof": proof}
	}
	permission, err := json.Marshal(map[string]any{"root": root.Hex(), "chainId": 31337, "adapter": issuer.Hex(), "vault": account.Hex(), "actions": []any{}, "leaves": permissionLeaves})
	if err != nil {
		t.Fatal(err)
	}
	permissionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/merkle-trees/"+root.Hex() || r.URL.Query().Get("chainId") != "31337" {
			t.Error("wrong active-root permission request")
		}
		_, _ = w.Write(permission)
	}))
	defer permissionServer.Close()
	cfg.Curator.URL = permissionServer.URL + "/api/v1"
	requestConfig := redemptions.Config{Family: "centrifuge", Mode: "request", ChainID: 31337, Token: share.Hex(), OutputAsset: asset.Hex(), Target: issuer.Hex(), Amount: "100", Action: "1:0"}
	claimConfig := requestConfig
	claimConfig.Mode, claimConfig.Action, claimConfig.Amount = "claim", "0:0", "100"
	cfg.Strategies = []string{strings.ToLower(requestExecutor.Hex()), strings.ToLower(claimExecutor.Hex())}
	path := writeCLIConfig(t, directory, cfg)
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(key)))

	startServiceConfirmationMining(t, ctx, client, caller)
	run := func() {
		if err := runIssuerFixture(ctx, path, []redemptions.Config{requestConfig, claimConfig}); err != nil {
			t.Fatal(err)
		}
	}
	read := func(target common.Address, contract serviceArtifact, method string, args ...any) *big.Int {
		raw, err := client.CallContract(ctx, ethereum.CallMsg{To: &target, Data: servicePack(t, contract, method, args...)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		values, err := contract.ABI.Unpack(method, raw)
		if err != nil || len(values) != 1 {
			t.Fatalf("decode fixture %s: %v", method, err)
		}
		return values[0].(*big.Int)
	}
	load := func(name string) keeper.Job {
		var job keeper.Job
		if found, err := state.Read(cfg.StateDir, "strategy."+name+".v1", &job); err != nil || !found {
			t.Fatalf("load %s lifecycle job: %v", name, err)
		}
		return job
	}
	run()
	requested := load(cfg.Strategies[0])
	if requested.Phase != "settled" || len(requested.Progress) != 1 || !requested.Progress[0].Done || requested.Progress[0].Outcome != "requested" || requested.Result.Execution == nil {
		t.Fatal("confirmed request did not release account ownership with correlated execution evidence")
	}
	if read(issuer, issuerArtifact, "pendingShares", account).Int64() != 100 || read(issuer, issuerArtifact, "maxRedeem", account).Sign() != 0 || read(share, shareArtifact, "allowance", account, issuer).Sign() != 0 || read(asset, outputArtifact, "balanceOf", account).Sign() != 0 {
		t.Fatal("request did not consume exact allowance and retain issuer-owned pending shares")
	}
	if found, err := state.Read(cfg.StateDir, "strategy."+cfg.Strategies[1]+".v1", new(keeper.Job)); err != nil || found {
		t.Fatalf("unready claim became durable work: %v", err)
	}
	run() // Restart while the issuer is still pending must not submit another request.
	if read(issuer, issuerArtifact, "requestCount").Int64() != 1 {
		t.Fatal("restart duplicated redemption request")
	}
	serviceSeedTransaction(t, ctx, client, owner, &issuer, servicePack(t, issuerArtifact, "finalize", account))
	run() // Separate claim strategy may now own the same account and nonce lane.
	claimed := load(cfg.Strategies[1])
	if claimed.Phase != "settled" || !claimed.Progress[0].Done || claimed.Progress[0].Outcome != "claimed" || claimed.Result.Execution == nil || claimed.Result.Hash == requested.Result.Hash {
		t.Fatal("matured issuer claim did not finish as its own confirmed execution")
	}
	if read(issuer, issuerArtifact, "maxRedeem", account).Sign() != 0 || read(asset, outputArtifact, "balanceOf", account).Int64() != 200 || read(issuer, issuerArtifact, "claimCount").Int64() != 1 {
		t.Fatal("claim failed to redeem the exact committed 100 shares")
	}
	run()
	if nonce, err := client.NonceAt(ctx, caller, nil); err != nil || nonce != 2 {
		t.Fatalf("completed lifecycle repeated across restart: nonce=%d error=%v", nonce, err)
	}
	if found, err := state.Read(cfg.StateDir, "txmanager.active.v1", new(any)); err != nil || found {
		t.Fatalf("completed claim retained nonce ownership: %v", err)
	}
}

// The catalogue intentionally contains no local-chain deployments. This fixture
// constructs reviewed providers directly at the SDK boundary; production YAML
// continues to reject manual runtime providers and uses authenticated discovery.
func runIssuerFixture(ctx context.Context, path string, policies []redemptions.Config) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	chainID := new(big.Int).SetUint64(cfg.Chain.ChainID)
	client, err := chain.Dial(ctx, cfg.Chain.RPCURL, cfg.Chain.WriteRPCURL, chainID, common.HexToAddress(config.StandardMulticall3))
	if err != nil {
		return err
	}
	defer client.Close()
	source, err := curator.New(cfg.Curator.HTTPConfig(), &http.Client{Timeout: config.DefaultHTTPTimeout})
	if err != nil {
		return err
	}
	targets := make([]common.Address, len(cfg.Strategies))
	for i, strategy := range cfg.Strategies {
		targets[i] = common.HexToAddress(strategy)
	}
	accounts, caller, err := discoverIdentity(ctx, client, targets)
	if err != nil {
		return err
	}
	strategies := make([]keeper.Strategy, len(cfg.Strategies))
	for i, strategy := range cfg.Strategies {
		runtime, err := redemptions.New(policies[i], providerRegistry.Deps{Chain: client})
		if err != nil {
			return err
		}
		revision, _ := json.Marshal(policies[i])
		strategies[i] = keeper.Strategy{Config: keeper.Config{Name: strategy, Executor: strategy, Account: accounts[targets[i]].Hex(), Interval: config.DefaultStrategyInterval, Revision: string(revision)}, Providers: []provider.Provider{runtime}}
	}
	store, err := state.Open(cfg.StateDir)
	if err != nil {
		return err
	}
	defer store.Close()
	if err = bindState(store, cfg.Chain.ChainID, caller.Hex()); err != nil {
		return err
	}
	txSigner, err := signer.FromEnv(cfg.Signer.KeyEnv)
	if err != nil {
		return err
	}
	sender, err := txmanager.New(client, txSigner, chainID, store, cfg.TxManager.EngineConfig(), logr.Discard())
	if err != nil {
		return err
	}
	if err = sender.Start(ctx); err != nil {
		return err
	}
	defer sender.Close()
	planner := &keeper.Planner{PermissionSource: source, Chain: client, ChainID: chainID, Caller: caller, Confirmations: config.DefaultConfirmations}
	worker := keeper.Keeper{Store: store, Planner: planner, Sender: sender, Strategies: strategies, Mode: "send", OperationTimeout: config.DefaultOperationTimeout, Log: logr.Discard()}
	return worker.Tick(ctx)
}
