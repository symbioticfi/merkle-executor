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
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	reactor "github.com/symbioticfi/merkle-executor/api/bindings/liquidlanereactor"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/providers/liquidlanerfq"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	accountbinding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func loadRFQArtifact(t *testing.T, name string) serviceArtifact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "providers", "liquidlanerfq", "testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		ABI      json.RawMessage
		Bytecode hexutil.Bytes
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(v.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return serviceArtifact{ABI: parsed, Bytecode: v.Bytecode}
}

func TestAnvilLiquidLaneRFQConsumerRestartAndFill(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("set MERKLE_ANVIL_TEST=1 for local RFQ consumer lifecycle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil || len(owners) == 0 {
		t.Fatal("local owner unavailable", err)
	}
	owner := owners[0]
	accountArtifact, tokenArtifact, reactorArtifact := loadRFQArtifact(t, "FixtureRFQAccount"), loadRFQArtifact(t, "FixtureRFQToken"), loadRFQArtifact(t, "FixtureRFQReactor")
	account := serviceSeedTransaction(t, ctx, client, owner, nil, accountArtifact.Bytecode).ContractAddress
	tokenIn := serviceSeedTransaction(t, ctx, client, owner, nil, tokenArtifact.Bytecode).ContractAddress
	tokenOut := serviceSeedTransaction(t, ctx, client, owner, nil, tokenArtifact.Bytecode).ContractAddress
	reactorAddress := serviceSeedTransaction(t, ctx, client, owner, nil, reactorArtifact.Bytecode).ContractAddress
	multicall := serviceSeedTransaction(t, ctx, client, owner, nil, loadMerklArtifact(t, "Multicall3").Bytecode).ContractAddress
	executorArtifact := loadServiceArtifact(t, "RecipeExecutor")
	executorAddress := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(executorArtifact.Bytecode), servicePack(t, executorArtifact, "", owner, account)...)).ContractAddress
	serviceSeedTransaction(t, ctx, client, owner, &tokenIn, servicePack(t, tokenArtifact, "mint", account, big.NewInt(1000)))
	serviceSeedTransaction(t, ctx, client, owner, &tokenOut, servicePack(t, tokenArtifact, "mint", owner, big.NewInt(950)))
	serviceSeedTransaction(t, ctx, client, owner, &tokenOut, servicePack(t, tokenArtifact, "approve", reactorAddress, big.NewInt(950)))
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	caller := crypto.PubkeyToAddress(key.PublicKey)
	if err = client.Client().CallContext(ctx, nil, "anvil_setBalance", caller, "0xde0b6b3a7640000"); err != nil {
		t.Fatal(err)
	}
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := head.Time + 3600
	request := reactor.IReactorRequest{TokenIn: tokenIn, AmountIn: big.NewInt(1000), Outputs: []reactor.IReactorOutput{{Token: tokenOut, Amount: big.NewInt(900), Recipient: account}}, Deadline: new(big.Int).SetUint64(deadline), Nonce: big.NewInt(7), Protocol: owner}
	// Leaf-fixed buffer independently mirrors the reviewed Request hash program.
	words := []common.Hash{crypto.Keccak256Hash([]byte("Request(address tokenIn,uint256 amountIn,Output[] outputs,uint256 deadline,uint256 nonce,address protocol)Output(address token,uint256 amount,address recipient)")), common.BytesToHash(tokenIn.Bytes()), common.BigToHash(request.AmountIn), {}, common.BigToHash(request.Deadline), common.BigToHash(request.Nonce), common.BytesToHash(owner.Bytes()), crypto.Keccak256Hash([]byte("Output(address token,uint256 amount,address recipient)")), common.BytesToHash(tokenOut.Bytes()), common.BigToHash(big.NewInt(900)), common.BytesToHash(account.Bytes())}
	var buffer []byte
	for _, word := range words {
		buffer = append(buffer, word[:]...)
	}
	hashes := []accountbinding.IMerkleAccountHash{{Offset: big.NewInt(224), Length: big.NewInt(128), Destination: big.NewInt(224)}, {Offset: big.NewInt(224), Length: big.NewInt(32), Destination: big.NewInt(96)}, {Offset: big.NewInt(0), Length: big.NewInt(224), Destination: big.NewInt(0)}}
	signData, err := accountbinding.NewMerkleAccount().TryPackSign(protocolio.Domain("Reactor", "1", big.NewInt(31337), reactorAddress), buffer, hashes)
	if err != nil {
		t.Fatal(err)
	}
	approval := erc20.NewERC20().PackApprove(reactorAddress, new(big.Int))
	leaves := []recipes.Leaf{{Target: tokenIn, Value: new(big.Int), Data: approval, DataMask: make([]byte, len(approval))}, {Target: account, Value: new(big.Int), Data: signData, DataMask: make([]byte, len(signData))}}
	for i := 36; i < 68; i++ {
		leaves[0].DataMask[i] = 0xff
	}
	for i := range leaves {
		leaves[i].Hash, err = recipes.HashLeaf(leaves[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	a, b := leaves[0].Hash, leaves[1].Hash
	if a.Cmp(b) > 0 {
		a, b = b, a
	}
	root := crypto.Keccak256Hash(a[:], b[:])
	leaves[0].Proof = []common.Hash{leaves[1].Hash}
	leaves[1].Proof = []common.Hash{leaves[0].Hash}
	serviceSeedTransaction(t, ctx, client, owner, &account, servicePack(t, accountArtifact, "configure", root, [][32]byte{leaves[0].Hash, leaves[1].Hash}, executorAddress))
	binding := executor.NewRecipeExecutor()
	serviceSeedTransaction(t, ctx, client, owner, &executorAddress, binding.PackSetCaller(caller))
	recipe := recipes.Recipe{Steps: [][]byte{issuerStep(t, recipes.Approve, []string{"uint256", "address", "address", "bytes32"}, big.NewInt(1000), tokenIn, reactorAddress, leaves[0].Hash), issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaves[1].Hash}, [][]*big.Int{{}})}}
	serviceSeedTransaction(t, ctx, client, owner, &executorAddress, binding.PackSetRecipe(recipe.Queries, recipe.Steps))
	var curatorCalls, quotes, gets, posts atomic.Int32
	var accepted atomic.Bool
	permission := []any{}
	for _, leaf := range leaves {
		permission = append(permission, map[string]any{"hash": leaf.Hash.Hex(), "target": leaf.Target.Hex(), "value": "zero", "data": hexutil.Encode(leaf.Data), "dataMask": hexutil.Encode(leaf.DataMask), "proof": []string{leaf.Proof[0].Hex()}})
	}
	curator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		curatorCalls.Add(1)
		if r.URL.Path != "/api/v1/merkle-trees/"+root.Hex() || r.URL.Query().Get("chainId") != "31337" {
			t.Error("wrong curator identity")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"root": root.Hex(), "chainId": 31337, "adapter": owner.Hex(), "leaves": permission})
	}))
	defer curator.Close()
	const quoteID = "11111111-1111-4111-8111-111111111111"
	const orderID = "22222222-2222-4222-8222-222222222222"
	output := wire.RfqOutput{Token: tokenOut.Hex(), Amount: "900", Recipient: account.Hex()}
	quote := wire.RfqQuote{QuoteID: quoteID, AggregatedOutputs: []wire.RfqAmount{{Token: tokenOut.Hex(), Amount: "900"}}, OrderInfo: wire.RfqQuoteInfo{TokenIn: tokenIn.Hex(), AmountIn: "1000", Outputs: []wire.RfqOutput{output}, Deadline: int64(deadline), Nonce: "7"}}
	quoteRaw, err := json.Marshal(quote)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]any{"Request": []map[string]string{{"name": "tokenIn", "type": "address"}, {"name": "amountIn", "type": "uint256"}, {"name": "outputs", "type": "Output[]"}, {"name": "deadline", "type": "uint256"}, {"name": "nonce", "type": "uint256"}, {"name": "protocol", "type": "address"}}, "Output": []map[string]string{{"name": "token", "type": "address"}, {"name": "amount", "type": "uint256"}, {"name": "recipient", "type": "address"}}}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/quote":
			quotes.Add(1)
			var q wire.RfqQuoteRequest
			if json.NewDecoder(r.Body).Decode(&q) != nil || q.Swapper != account.Hex() || q.Amount != "1000" || q.Type != "EXACT_INPUT" {
				t.Error("quote lost derived account or fixed economics")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"quotes": []any{map[string]any{"solver": map[string]any{"id": "local", "filler": owner.Hex()}, "quote": quote, "signatureData": map[string]any{"domain": map[string]any{"name": "Reactor", "version": "1", "chainId": 31337, "verifyingContract": reactorAddress.Hex()}, "primaryType": "Request", "types": fields, "value": map[string]any{"tokenIn": tokenIn.Hex(), "amountIn": "1000", "outputs": []wire.RfqOutput{output}, "deadline": strconv.FormatUint(deadline, 10), "nonce": "7", "protocol": owner.Hex()}}}}})
		case "/api/v1/orders":
			gets.Add(1)
			if r.URL.Query().Get("swapper") != account.Hex() || r.URL.Query().Has("orderStatus") {
				t.Error("lookup lost account or filtered terminal statuses")
			}
			orders := []wire.RfqOrder{}
			if accepted.Load() {
				orders = append(orders, wire.RfqOrder{Type: "Priority", OrderID: orderID, OrderStatus: "filled", QuoteID: quoteID, Swapper: account.Hex(), Nonce: "7", Input: wire.RfqAmount{Token: tokenIn.Hex(), Amount: "1000"}, Outputs: []wire.RfqOutput{{Token: tokenOut.Hex(), Amount: "950", Recipient: account.Hex()}}})
			}
			_ = json.NewEncoder(w).Encode(wire.RfqOrdersResponse{Orders: orders, Cursor: json.RawMessage("null")})
		case "/api/v1/order":
			posts.Add(1)
			var body wire.RfqOrderRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil || body.Signature != "0x00" || !bytes.Equal(body.Quote, quoteRaw) {
				t.Error("publication changed exact quote/signature")
			}
			accepted.Store(true)
			w.WriteHeader(http.StatusBadGateway) // Accepted by fixture, response lost.
		default:
			t.Error("unexpected RFQ route")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer api.Close()
	directory := t.TempDir()
	cfg := testCLIConfig(directory)
	cfg.Mode = "send"
	cfg.Chain.RPCURL = endpoint
	cfg.Chain.Multicall3 = multicall.Hex()
	cfg.Signer.Address = caller.Hex()
	cfg.Curator.URL = curator.URL + "/api/v1"
	cfg.Strategies[0].Executor = executorAddress.Hex()
	cfg.Strategies[0].Providers = []config.Provider{{Name: "liquidlane-rfq", Config: serviceNode(t, liquidlanerfq.Config{HTTP: protocolio.HTTPConfig{URL: api.URL + "/api/v1"}, Reactor: reactorAddress.Hex(), Protocol: owner.Hex(), TokenIn: tokenIn.Hex(), TokenOut: tokenOut.Hex(), AmountIn: "1000", MinAmountOut: "900", MinValiditySeconds: 60, MaxValiditySeconds: 7200, Action: "1:0", MaxLookupPages: 10, EventRangeBlocks: 2000})}}
	path := writeCLIConfig(t, directory, cfg)
	var planOutput bytes.Buffer
	if err = runBot(ctx, path, true, true, &planOutput); err != nil {
		t.Fatal(err)
	}
	var plans map[string]keeper.Plan
	if err = json.Unmarshal(planOutput.Bytes(), &plans); err != nil {
		t.Fatal(err)
	}
	plan := plans["funding"]
	if len(plan.Outputs) != 1 || !bytes.Equal(plan.Outputs[0].Actions["1:0"], signData) || len(signData) != 804 || posts.Load() != 0 || quotes.Load() != 1 {
		t.Fatal("plan did not preserve exact sign or published early")
	}
	if nonce, e := client.NonceAt(ctx, caller, nil); e != nil || nonce != 0 {
		t.Fatal("plan signed", nonce, e)
	}
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(key)))
	mining, stopMining := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-mining.Done():
				return
			case <-ticker.C:
				if e := client.Client().CallContext(mining, nil, "evm_mine"); e != nil && mining.Err() == nil {
					t.Error(e)
					return
				}
			}
		}
	}()
	t.Cleanup(func() { stopMining(); <-done })
	run := func() {
		if e := runBot(ctx, path, true, false, new(bytes.Buffer)); e != nil {
			t.Fatal(e)
		}
	}
	load := func() keeper.Job {
		var job keeper.Job
		if found, e := state.Read(cfg.StateDir, "strategy.funding.v1", &job); e != nil || !found {
			t.Fatal("job absent", e)
		}
		return job
	}
	run()
	job := load()
	if job.Phase != "authorized" || len(job.Progress) != 1 || job.Progress[0].Outcome != "publication-uncertain" || job.Result.Execution == nil || posts.Load() != 1 || quotes.Load() != 2 {
		t.Fatalf("ambiguous publication lost ownership: phase=%s progress=%+v posts=%d quotes=%d", job.Phase, job.Progress, posts.Load(), quotes.Load())
	}
	originalData := bytes.Clone(job.Plan.Outputs[0].Data)
	treeCalls := curatorCalls.Load()
	run()
	job = load()
	if job.Phase != "authorized" || job.Progress[0].Done || job.Progress[0].Reference != orderID || posts.Load() != 1 || quotes.Load() != 2 || curatorCalls.Load() != treeCalls || !bytes.Equal(originalData, job.Plan.Outputs[0].Data) {
		t.Fatal("API-only fill released work or restart changed identity")
	}
	if nonce, e := client.NonceAt(ctx, caller, nil); e != nil || nonce != 1 {
		t.Fatal("restart duplicated authorization", nonce, e)
	}
	for token, want := range map[common.Address]int64{tokenIn: 1000, tokenOut: 0} {
		raw, e := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: erc20.NewERC20().PackBalanceOf(account)}, nil)
		if e != nil || new(big.Int).SetBytes(raw).Int64() != want {
			t.Fatal("publication moved tokens before fill", token, want, e)
		}
	}
	rawAllowance, e := client.CallContract(ctx, ethereum.CallMsg{To: &tokenIn, Data: erc20.NewERC20().PackAllowance(account, reactorAddress)}, nil)
	if e != nil || new(big.Int).SetBytes(rawAllowance).Int64() != 1000 {
		t.Fatal("recipe did not authorize exact Reactor allowance", e)
	}
	order := reactor.IReactorOrder{Request: request, Swapper: account, SwapperSignature: []byte{0}, Filler: owner, Outputs: []reactor.IReactorOutput{{Token: tokenOut, Amount: big.NewInt(950), Recipient: account}}}
	serviceSeedTransaction(t, ctx, client, owner, &reactorAddress, servicePack(t, reactorArtifact, "fill", order))
	for range 3 {
		if err = client.Client().CallContext(ctx, nil, "evm_mine"); err != nil {
			t.Fatal(err)
		}
	}
	run()
	job = load()
	if job.Phase != "settled" || !job.Progress[0].Done || job.Progress[0].Outcome != "settled" {
		t.Fatal("canonical fill did not settle", job.Phase, job.Progress)
	}
	for token, want := range map[common.Address]int64{tokenIn: 0, tokenOut: 950} {
		raw, e := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: erc20.NewERC20().PackBalanceOf(account)}, nil)
		if e != nil || new(big.Int).SetBytes(raw).Int64() != want {
			t.Fatal("wrong settled balance", token, want, e)
		}
	}
	lookupCalls := gets.Load()
	run()
	if nonce, e := client.NonceAt(ctx, caller, nil); e != nil || nonce != 1 || posts.Load() != 1 || quotes.Load() != 2 || gets.Load() != lookupCalls || curatorCalls.Load() != treeCalls {
		t.Fatal("settled restart repeated network mutation", nonce, e)
	}
	if found, e := state.Read(cfg.StateDir, "txmanager.active.v1", new(any)); e != nil || found {
		t.Fatal("settled transaction retained nonce journal", e)
	}
}
