package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/internal/config"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

type serviceArtifact struct {
	ABI      abi.ABI
	Bytecode hexutil.Bytes
}

func loadServiceArtifact(t *testing.T, name string) serviceArtifact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "sdk", "recipes", "testdata", "anvil", name+".json"))
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

func servicePack(t *testing.T, contract serviceArtifact, method string, values ...any) []byte {
	t.Helper()
	encoded, err := contract.ABI.Pack(method, values...)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func startServiceAnvil(t *testing.T, ctx context.Context) (*ethclient.Client, string) {
	t.Helper()
	binary, err := exec.LookPath("anvil")
	if err != nil {
		t.Fatal("MERKLE_ANVIL_TEST requires anvil in PATH")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, binary, "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--chain-id", "31337", "--hardfork", "osaka", "--silent")
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
		if t.Failed() {
			t.Log(output.String())
		}
	})
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if chainID, err := client.ChainID(ctx); err == nil {
			if chainID.Cmp(big.NewInt(31337)) != 0 {
				t.Fatal("unexpected local chain identity")
			}
			// Install the vendored fixture runtime at the official address so daemon
			// tests exercise the same fixed aggregate destination as production.
			runtime, err := client.CallContract(ctx, ethereum.CallMsg{Data: loadMerklArtifact(t, "Multicall3").Bytecode}, nil)
			if err != nil || len(runtime) == 0 {
				t.Fatal("Multicall3 fixture runtime unavailable", err)
			}
			if err := client.Client().CallContext(ctx, nil, "anvil_setCode", common.HexToAddress(config.StandardMulticall3), hexutil.Bytes(runtime)); err != nil {
				t.Fatal(err)
			}
			return client, endpoint
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-timer.C:
			t.Fatal("local Anvil startup timed out")
		case <-ticker.C:
		}
	}
}

// The fixed polling profile needs a stable head while checking ancestry. Mine
// exactly the required descendants after each included caller transaction.
func startServiceConfirmationMining(t *testing.T, ctx context.Context, client *ethclient.Client, caller common.Address) {
	t.Helper()
	mining, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		var confirmedNonce uint64
		for {
			select {
			case <-mining.Done():
				return
			case <-ticker.C:
				nonce, err := client.NonceAt(mining, caller, nil)
				if err != nil {
					if mining.Err() == nil {
						t.Error(err)
					}
					return
				}
				if nonce <= confirmedNonce {
					continue
				}
				for range config.DefaultConfirmations {
					if err := client.Client().CallContext(mining, nil, "evm_mine"); err != nil {
						if mining.Err() == nil {
							t.Error(err)
						}
						return
					}
				}
				confirmedNonce = nonce
			}
		}
	}()
	t.Cleanup(func() { stop(); <-done })
}

func serviceSeedTransaction(t *testing.T, ctx context.Context, client *ethclient.Client, from common.Address, to *common.Address, calldata []byte) *types.Receipt {
	t.Helper()
	args := map[string]any{"from": from, "gas": hexutil.Uint64(12_000_000), "data": hexutil.Bytes(calldata)}
	if to != nil {
		args["to"] = *to
	}
	var hash common.Hash
	if err := client.Client().CallContext(ctx, &hash, "eth_sendTransaction", args); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if err == nil {
			if receipt.Status != types.ReceiptStatusSuccessful {
				t.Fatal("local fixture setup reverted")
			}
			rememberSeededEvents(client, receipt)
			return receipt
		}
		if !errors.Is(err, ethereum.NotFound) {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}

func serviceRecipe(t *testing.T, leaf common.Hash, token, account common.Address) recipes.Recipe {
	t.Helper()
	result := func(index int64) *big.Int {
		return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(index))
	}
	input := new(big.Int).Lsh(big.NewInt(1), 216)
	query := issuerStep(t, 0, []string{"address", "bytes", "uint256[]", "uint256"}, token, erc20.NewERC20().PackBalanceOf(account), []*big.Int{}, new(big.Int))[1:]
	return recipes.Recipe{Queries: [][]byte{query}, Steps: [][]byte{
		issuerStep(t, recipes.Read, []string{"uint256"}, new(big.Int)),
		issuerStep(t, recipes.Min, []string{"uint256", "uint256"}, result(0), big.NewInt(17)),
		issuerStep(t, recipes.Equal, []string{"uint256", "uint256"}, input, result(1)),
		issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaf}, [][]*big.Int{{new(big.Int).Or(new(big.Int).Lsh(big.NewInt(36), 224), input)}}),
	}}
}

func TestAnvilServicePlanAndSend(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("set MERKLE_ANVIL_TEST=1 for full local service integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	var localAccounts []common.Address
	if err := client.Client().CallContext(ctx, &localAccounts, "eth_accounts"); err != nil || len(localAccounts) < 2 {
		t.Fatalf("local accounts unavailable: %v", err)
	}
	owner, recipient := localAccounts[0], localAccounts[1]
	accountArtifact := loadServiceArtifact(t, "FixtureAccount")
	tokenArtifact := loadServiceArtifact(t, "FixtureToken")
	executorArtifact := loadServiceArtifact(t, "RecipeExecutor")
	accountAddress := serviceSeedTransaction(t, ctx, client, owner, nil, accountArtifact.Bytecode).ContractAddress
	tokenAddress := serviceSeedTransaction(t, ctx, client, owner, nil, tokenArtifact.Bytecode).ContractAddress
	constructor := servicePack(t, executorArtifact, "", owner, accountAddress)
	executorAddress := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(executorArtifact.Bytecode), constructor...)).ContractAddress

	// Generate an ephemeral test-only signer and fund it exclusively in this local
	// node. No existing user key or configured external RPC is read by the test.
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	caller := crypto.PubkeyToAddress(key.PublicKey)
	if err = client.Client().CallContext(ctx, nil, "anvil_setBalance", caller, "0xde0b6b3a7640000"); err != nil {
		t.Fatal(err)
	}
	tokenABI, executorABI := erc20.NewERC20(), executor.NewRecipeExecutor()
	transfer := tokenABI.PackTransfer(recipient, new(big.Int))
	mask := make([]byte, len(transfer))
	for index := 36; index < len(mask); index++ {
		mask[index] = 0xff
	}
	leaf := recipes.Leaf{Target: tokenAddress, Value: new(big.Int), Data: transfer, DataMask: mask}
	leaf.Hash, err = recipes.HashLeaf(leaf)
	if err != nil {
		t.Fatal(err)
	}
	serviceSeedTransaction(t, ctx, client, owner, &accountAddress, servicePack(t, accountArtifact, "configure", leaf.Hash, [][32]byte{leaf.Hash}, executorAddress))
	serviceSeedTransaction(t, ctx, client, owner, &tokenAddress, servicePack(t, tokenArtifact, "mint", accountAddress, big.NewInt(1000)))
	serviceSeedTransaction(t, ctx, client, owner, &executorAddress, executorABI.PackSetCaller(caller))
	recipe := serviceRecipe(t, leaf.Hash, tokenAddress, accountAddress)
	serviceSeedTransaction(t, ctx, client, owner, &executorAddress, executorABI.PackSetRecipe(recipe.Queries, recipe.Steps))

	directory := t.TempDir()
	cfg := testCLIConfig(directory)
	cfg.IndexerURL = fixtureIndexer(t, client)
	cfg.Mode, cfg.Chain.RPCURL, cfg.Signer.Address = "send", endpoint, caller.Hex()
	cfg.Strategies[0] = strings.ToLower(executorAddress.Hex())
	permission, err := json.Marshal(map[string]any{"root": leaf.Hash.Hex(), "chainId": 31337, "adapter": recipient.Hex(), "vault": owner.Hex(), "actions": []any{}, "leaves": []any{map[string]any{"hash": leaf.Hash.Hex(), "target": tokenAddress.Hex(), "value": "zero", "data": hexutil.Encode(transfer), "dataMask": hexutil.Encode(mask), "proof": []string{}}}})
	if err != nil {
		t.Fatal(err)
	}
	permission = servicePermissionCertificate(t, permission, accountAddress, accountAddress)
	var curatorCalls atomic.Int32
	var curatorUnavailable atomic.Bool
	var wrongPermissionChain atomic.Bool
	var changeRootDuringHTTP atomic.Bool
	permissionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curatorCalls.Add(1)
		if r.URL.Path != "/api/v1/merkle/"+strings.ToLower(accountAddress.Hex())+"/"+leaf.Hash.Hex() || r.URL.RawQuery != "" {
			t.Error("wrong active-root request")
		}
		if curatorUnavailable.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		if changeRootDuringHTTP.Swap(false) {
			serviceSeedTransaction(t, ctx, client, owner, &accountAddress, servicePack(t, accountArtifact, "configure", common.HexToHash("0x999"), [][32]byte{leaf.Hash}, executorAddress))
		}
		if wrongPermissionChain.Load() {
			_, _ = w.Write(bytes.Replace(permission, []byte(`"chainId":31337`), []byte(`"chainId":1`), 1))
			return
		}
		_, _ = w.Write(permission)
	}))
	defer permissionServer.Close()
	cfg.Curator.URL = permissionServer.URL + "/api/v1"
	for name, mutate := range map[string]func(*config.Config){
		"chain":  func(c *config.Config) { c.Chain.ChainID++ },
		"caller": func(c *config.Config) { c.Signer.Address = owner.Hex() },
	} {
		invalid := cfg
		invalid.Strategies = append([]string(nil), cfg.Strategies...)
		mutate(&invalid)
		invalidPath := writeCLIConfig(t, directory, invalid)
		if err = runBot(ctx, invalidPath, true, true, new(bytes.Buffer)); err == nil {
			t.Fatalf("plan accepted wrong %s identity", name)
		}
	}
	cfg.Signer.Address = ""
	path := writeCLIConfig(t, directory, cfg)
	for _, invalidKey := range []string{"invalid-test-key-that-plan-must-ignore", ""} {
		t.Setenv(cfg.Signer.KeyEnv, invalidKey)
		var output bytes.Buffer
		if err = runBot(ctx, path, true, true, &output); err != nil {
			t.Fatalf("plan read signer or failed: %v", err)
		}
		var plans map[string]keeper.Plan
		if err = json.Unmarshal(output.Bytes(), &plans); err != nil {
			t.Fatal(err)
		}
		plan, exists := plans[cfg.Strategies[0]]
		if !exists || plan.Snapshot.Account != accountAddress || plan.Snapshot.Caller != caller || plan.Snapshot.Root != leaf.Hash || len(plan.Calldata) < 4 || len(plan.Outputs) != 5 || plan.Results[0] != "1000" || plan.Results[1] != "17" {
			t.Fatal("plan lost configuration, provider output or transaction")
		}
	}
	if _, err = os.Stat(cfg.StateDir); !os.IsNotExist(err) {
		t.Fatalf("plan created signing state: %v", err)
	}
	if nonce, err := client.NonceAt(ctx, caller, nil); err != nil || nonce != 0 {
		t.Fatalf("plan sent a transaction: nonce=%d / %v", nonce, err)
	}

	mismatchedKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(mismatchedKey)))
	if err := runBot(ctx, path, true, false, new(bytes.Buffer)); err == nil {
		t.Fatal("send accepted a key that is not executor caller")
	}
	if _, err := os.Stat(cfg.StateDir); !os.IsNotExist(err) {
		t.Fatal("caller mismatch created durable state", err)
	}

	// Failed curator acquisition/validation must stop the real send path before
	// signing or journal ownership. A changed root during HTTP must also fail the
	// existing fresh metadata guard; restore only the ephemeral fixture afterward.
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(key)))
	for _, failure := range []string{"HTTP503", "wrong chain", "root during HTTP"} {
		curatorUnavailable.Store(failure == "HTTP503")
		wrongPermissionChain.Store(failure == "wrong chain")
		changeRootDuringHTTP.Store(failure == "root during HTTP")
		if err = runBot(ctx, path, true, false, new(bytes.Buffer)); err == nil {
			t.Fatalf("send accepted curator failure %s", failure)
		}
		if nonce, e := client.NonceAt(ctx, caller, nil); e != nil || nonce != 0 {
			t.Fatalf("source failure submitted nonce%d: %v", nonce, e)
		}
		for _, key := range []string{"strategy." + cfg.Strategies[0] + ".v1", "txmanager.active.v1"} {
			if found, e := state.Read(cfg.StateDir, key, new(any)); e != nil || found {
				t.Fatalf("source failure persisted executable work %s: %v", key, e)
			}
		}
		raw, e := client.CallContract(ctx, ethereum.CallMsg{To: &tokenAddress, Data: tokenABI.PackBalanceOf(recipient)}, nil)
		if e != nil || new(big.Int).SetBytes(raw).Sign() != 0 {
			t.Fatal("source failure transferred funds")
		}
		if failure == "root during HTTP" {
			serviceSeedTransaction(t, ctx, client, owner, &accountAddress, servicePack(t, accountArtifact, "configure", leaf.Hash, [][32]byte{leaf.Hash}, executorAddress))
		}
	}
	curatorUnavailable.Store(false)
	wrongPermissionChain.Store(false)

	// Finality requires descendants after inclusion; mine local empty blocks while
	// the real transaction manager confirms and acknowledges the signed transaction.
	startServiceConfirmationMining(t, ctx, client, caller)
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(key)))
	if err = runBot(ctx, path, true, false, new(bytes.Buffer)); err != nil {
		t.Fatalf("send-once failed: %v", err)
	}
	var job keeper.Job
	if found, err := state.Read(cfg.StateDir, "strategy."+cfg.Strategies[0]+".v1", &job); err != nil || !found {
		t.Fatalf("durable job missing: %v", err)
	}
	if job.Phase != "settled" || !job.Result.Confirmed || job.Result.Reverted || job.Result.Hash == (common.Hash{}) || len(job.Progress) != 5 || job.FinishedAt.IsZero() {
		t.Fatalf("job did not settle: %+v", job.Result)
	}
	for _, progress := range job.Progress {
		if !progress.Done {
			t.Fatal("provider progress was not persisted")
		}
	}
	if found, err := state.Read(cfg.StateDir, "txmanager.active.v1", new(any)); err != nil || found {
		t.Fatalf("terminal journal was not acknowledged: %v", err)
	}
	transaction, pending, err := client.TransactionByHash(ctx, job.Result.Hash)
	if err != nil || pending {
		t.Fatalf("canonical local transaction missing: %v", err)
	}
	signedBy, err := types.Sender(types.LatestSignerForChainID(big.NewInt(31337)), transaction)
	if err != nil || signedBy != caller || transaction.To() == nil || *transaction.To() != executorAddress || !bytes.Equal(transaction.Data(), job.Plan.Calldata) || transaction.Nonce() != 0 {
		t.Fatalf("sender/transaction differs from durable plan: %v", err)
	}
	for address, expected := range map[common.Address]int64{accountAddress: 983, recipient: 17} {
		raw, err := client.CallContract(ctx, ethereum.CallMsg{To: &tokenAddress, Data: tokenABI.PackBalanceOf(address)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		balance, err := tokenABI.UnpackBalanceOf(raw)
		if err != nil || balance.Int64() != expected {
			t.Fatalf("post-execution balance expected %d: %v / %v", expected, balance, err)
		}
	}
	permissionRequests := curatorCalls.Load()
	curatorUnavailable.Store(true)
	if err = runBot(ctx, path, true, false, new(bytes.Buffer)); err != nil {
		t.Fatalf("restart failed: %v", err)
	}
	if nonce, err := client.NonceAt(ctx, caller, nil); err != nil || nonce != 1 || curatorCalls.Load() != permissionRequests {
		t.Fatalf("restart repeated completed work: nonce=%d / %v", nonce, err)
	}
}

// Match the generic backend certificate wire shape; call proofs remain independently verified.
func servicePermissionCertificate(t *testing.T, raw []byte, adapter, account common.Address) []byte {
	t.Helper()
	var metadata map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&metadata); err != nil {
		t.Fatal(err)
	}
	if leaves, ok := metadata["leaves"]; ok {
		metadata["calls"] = leaves
		delete(metadata, "leaves")
	}
	metadata["adapter"], metadata["account"] = strings.ToLower(adapter.Hex()), strings.ToLower(account.Hex())
	hashes := []string{}
	for _, call := range metadata["calls"].([]any) {
		hashes = append(hashes, call.(map[string]any)["hash"].(string))
	}
	sort.Strings(hashes)
	body, err := json.Marshal(map[string]any{"bundle": map[string]any{"chainId": metadata["chainId"], "adapter": metadata["adapter"], "account": metadata["account"], "root": metadata["root"], "leaves": hashes, "metadata": metadata}, "bundleHash": common.Hash{}.Hex(), "signer": account.Hex(), "signature": "0x", "deadline": "1800000000", "createdAt": "2026-10-20T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
