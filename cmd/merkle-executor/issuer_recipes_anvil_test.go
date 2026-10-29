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

	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func loadIssuerArtifact(t *testing.T, name string) serviceArtifact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "issuers", name+".json"))
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

func TestAnvilIssuerRecipeRequestThenClaim(t *testing.T) {
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
	tokens, executors := erc20.NewERC20(), executor.NewRecipeExecutor()
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
		newLeaf(issuer, servicePack(t, issuerArtifact, "requestRedeem", new(big.Int), account, account), 4),
		newLeaf(issuer, servicePack(t, issuerArtifact, "redeem", new(big.Int), account, account), 4),
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
	result := new(big.Int).Lsh(big.NewInt(2), 216)
	query := func(target common.Address, data []byte) []byte {
		return issuerStep(t, 0, []string{"address", "bytes", "uint256[]", "uint256"}, target, data, []*big.Int{}, new(big.Int))[1:]
	}
	bind := func(offset int64) *big.Int {
		return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(offset), 224), result)
	}
	requestRecipe := recipes.Recipe{Queries: [][]byte{query(share, tokens.PackBalanceOf(account))}, Steps: [][]byte{
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(0)),
		issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, result,
			[][32]byte{leaves[0].Hash, leaves[1].Hash, leaves[0].Hash}, [][]*big.Int{{bind(36)}, {bind(4)}, {new(big.Int).Lsh(big.NewInt(36), 224)}}),
	}}
	claimRecipe := recipes.Recipe{Queries: [][]byte{query(issuer, servicePack(t, issuerArtifact, "maxRedeem", account))}, Steps: [][]byte{
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(0)),
		issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, result, [][32]byte{leaves[2].Hash}, [][]*big.Int{{bind(4)}}),
	}}
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
	permission = servicePermissionCertificate(t, permission, account, account)
	permissionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/merkle/"+strings.ToLower(account.Hex())+"/"+root.Hex() || r.URL.RawQuery != "" {
			t.Error("wrong active-root permission request")
		}
		_, _ = w.Write(permission)
	}))
	defer permissionServer.Close()
	cfg.Curator.URL = permissionServer.URL + "/api/v1"
	cfg.Strategies = []string{strings.ToLower(requestExecutor.Hex()), strings.ToLower(claimExecutor.Hex())}
	path := writeCLIConfig(t, directory, cfg)
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(key)))

	startServiceConfirmationMining(t, ctx, client, caller)
	run := func() {
		if err := runBot(ctx, path, true, false, new(bytes.Buffer)); err != nil {
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
	assertIdle := func() {
		t.Helper()
		for _, strategy := range cfg.Strategies {
			if found, err := state.Read(cfg.StateDir, "strategy."+strategy+".v1", new(keeper.Job)); err != nil || found {
				t.Fatalf("zero-gated issuer recipe became durable work: %v", err)
			}
		}
		if nonce, err := client.NonceAt(ctx, caller, nil); err != nil || nonce != 0 {
			t.Fatalf("idle recipes consumed a nonce: nonce=%d error=%v", nonce, err)
		}
	}
	run()
	assertIdle()
	serviceSeedTransaction(t, ctx, client, owner, &share, servicePack(t, shareArtifact, "mint", account, big.NewInt(100)))
	run()
	requested := load(cfg.Strategies[0])
	if requested.Phase != "settled" || len(requested.Progress) != 5 || !requested.Progress[4].Done || requested.Result.Execution == nil {
		t.Fatal("confirmed recipe request did not release account ownership with correlated execution evidence")
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
	if claimed.Phase != "settled" || len(claimed.Progress) != 5 || !claimed.Progress[4].Done || claimed.Result.Execution == nil || claimed.Result.Hash == requested.Result.Hash {
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
