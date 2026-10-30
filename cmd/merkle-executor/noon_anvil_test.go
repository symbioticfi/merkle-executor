package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"github.com/symbioticfi/merkle-executor/sdk/state"
)

func TestAnvilNoonNativeBurnAndUSDCPayment(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("set MERKLE_ANVIL_TEST=1 for source-pinned Noon execution")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	// This owned ephemeral process has no fork or external RPC. Chain 1 only
	// reproduces the permission certificate's deployment identity.
	if err := client.Client().CallContext(ctx, nil, "anvil_setChainId", 1); err != nil {
		t.Fatal(err)
	}
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil || len(owners) < 2 {
		t.Fatalf("local accounts unavailable: %v", err)
	}
	owner, treasury := owners[0], owners[1]
	// This signer is generated and funded only in the owned local process.
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	caller := crypto.PubkeyToAddress(key.PublicKey)
	if err = client.Client().CallContext(ctx, nil, "anvil_setBalance", caller, "0xde0b6b3a7640000"); err != nil {
		t.Fatal(err)
	}
	if err = client.Client().CallContext(ctx, nil, "anvil_impersonateAccount", caller); err != nil {
		t.Fatal(err)
	}
	usn := common.HexToAddress("0xdA67B4284609d2d48e5d10cfAc411572727dc1eD")
	usdc := common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	handler := common.HexToAddress("0x9F8D0a683871125083170F65430F8A15aFfa0086")
	accountArtifact, executorArtifact := loadServiceArtifact(t, "FixtureAccount"), loadServiceArtifact(t, "RecipeExecutor")
	tokenArtifact, oracleArtifact := loadIssuerArtifact(t, "noon/FixtureNoonToken"), loadIssuerArtifact(t, "noon/FixtureNoonOracle")
	handlerArtifact := loadIssuerArtifact(t, "noon/RedeemHandlerV2")
	deploy := func(artifact serviceArtifact, args ...any) common.Address {
		return serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(artifact.Bytecode), servicePack(t, artifact, "", args...)...)).ContractAddress
	}
	install := func(target, deployed common.Address) {
		code, err := client.CodeAt(ctx, deployed, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Client().CallContext(ctx, nil, "anvil_setCode", target, hexutil.Bytes(code)); err != nil {
			t.Fatal(err)
		}
	}
	// Copy unmodified compiled runtime at the canonical route addresses. Tokens and
	// oracle are local fixtures; the handler is the complete verified source.
	install(usn, deploy(tokenArtifact, uint8(18)))
	install(usdc, deploy(tokenArtifact, uint8(6)))
	install(handler, deploy(handlerArtifact, usn))
	// Recreate the constructor's local DEFAULT_ADMIN_ROLE membership at the copied
	// address. All remaining handler state is set via its real administrative API.
	roleData := crypto.Keccak256Hash(make([]byte, 64)) // mapping(bytes32 => RoleData) at storage slot 0.
	member := crypto.Keccak256Hash(common.LeftPadBytes(owner.Bytes(), 32), roleData.Bytes())
	if err := client.Client().CallContext(ctx, nil, "anvil_setStorageAt", handler, member, common.BigToHash(big.NewInt(1))); err != nil {
		t.Fatal(err)
	}
	call := func(from common.Address, target common.Address, artifact serviceArtifact, method string, args ...any) {
		serviceSeedTransaction(t, ctx, client, from, &target, servicePack(t, artifact, method, args...))
	}
	oracle := deploy(oracleArtifact)
	call(owner, handler, handlerArtifact, "setTreasury", treasury)
	call(owner, handler, handlerArtifact, "setRedeemLimitPerBlock", new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)))
	accountant := crypto.Keccak256Hash([]byte("ACCOUNTANT_ROLE"))
	call(owner, handler, handlerArtifact, "grantRole", accountant, owner)
	call(owner, handler, handlerArtifact, "setPegPercentage", big.NewInt(9990))
	call(owner, handler, handlerArtifact, "addRedeemableCollateral", usdc, oracle, big.NewInt(1e8))
	account := deploy(accountArtifact)
	executor := deploy(executorArtifact, owner, account)
	call(owner, handler, handlerArtifact, "addWhitelistedUser", account)
	amount, _ := new(big.Int).SetString("1234567890123456789", 10)
	call(owner, usn, tokenArtifact, "mint", account, amount)
	call(owner, usdc, tokenArtifact, "mint", treasury, big.NewInt(10_000_000))
	call(treasury, usdc, tokenArtifact, "approve", handler, big.NewInt(10_000_000))
	tokens, executors := erc20.NewERC20(), recipeexecutor.NewRecipeExecutor()
	leaf := func(target common.Address, data []byte) recipes.Leaf {
		mask := make([]byte, len(data))
		for i := 36; i < 68; i++ {
			mask[i] = 0xff
		}
		value := recipes.Leaf{Target: target, Value: new(big.Int), Data: data, DataMask: mask}
		var err error
		value.Hash, err = recipes.HashLeaf(value)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	approval := leaf(usn, tokens.PackApprove(handler, new(big.Int)))
	action := leaf(handler, servicePack(t, handlerArtifact, "redeemOnchain", usdc, new(big.Int)))
	approval.Proof, action.Proof = []common.Hash{action.Hash}, []common.Hash{approval.Hash}
	a, b := approval.Hash, action.Hash
	if a.Cmp(b) > 0 {
		a, b = b, a
	}
	root := crypto.Keccak256Hash(a.Bytes(), b.Bytes())
	call(owner, account, accountArtifact, "configure", root, [][32]byte{approval.Hash, action.Hash}, executor)
	serviceSeedTransaction(t, ctx, client, owner, &executor, executors.PackSetCaller(caller))
	input := new(big.Int).Lsh(big.NewInt(1), 216)
	result := func(index int64) *big.Int {
		return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(index))
	}
	query := func(target common.Address, data []byte) []byte {
		return issuerStep(t, 0, []string{"address", "bytes", "uint256[]", "uint256"}, target, data, []*big.Int{}, new(big.Int))[1:]
	}
	binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(36), 224), input)
	recipe := recipes.Recipe{Queries: [][]byte{
		query(usn, tokens.PackBalanceOf(account)),
		query(handler, servicePack(t, handlerArtifact, "redeemLimitPerBlock")),
		query(usdc, tokens.PackBalanceOf(account)),
	}, Steps: [][]byte{
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(0)),
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(1)),
		issuerStep(t, recipes.Min, []string{"uint256", "uint256"}, result(0), result(1)),
		issuerStep(t, recipes.Equal, []string{"uint256", "uint256"}, input, result(2)),
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(2)),
		issuerStep(t, recipes.Approve, []string{"uint256", "address", "address", "bytes32"}, input, usn, handler, approval.Hash),
		issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{action.Hash}, [][]*big.Int{{binding}}),
		issuerStep(t, recipes.Approve, []string{"uint256", "address", "address", "bytes32"}, big.NewInt(0), usn, handler, approval.Hash),
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(2)),
		issuerStep(t, recipes.Sub, []string{"uint256", "uint256"}, result(8), result(4)),
		issuerStep(t, recipes.LessOrEqual, []string{"uint256", "uint256"}, big.NewInt(1_233_332), result(9)),
	}}
	serviceSeedTransaction(t, ctx, client, owner, &executor, executors.PackSetRecipe(recipe.Queries, recipe.Steps))
	entries := make([]any, 2)
	for i, v := range []recipes.Leaf{approval, action} {
		entries[i] = map[string]any{"hash": v.Hash.Hex(), "target": v.Target.Hex(), "value": "zero", "data": hexutil.Encode(v.Data), "dataMask": hexutil.Encode(v.DataMask), "proof": []string{v.Proof[0].Hex()}}
	}
	rawTree, _ := json.Marshal(map[string]any{"root": root.Hex(), "chainId": 1, "account": account.Hex(), "leaves": entries})
	rawTree = servicePermissionCertificate(t, rawTree, account, account)
	curator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/merkle/"+strings.ToLower(account.Hex())+"/"+root.Hex() || r.URL.RawQuery != "" {
			t.Error("wrong active-root curator request")
		}
		_, _ = w.Write(rawTree)
	}))
	defer curator.Close()
	endpointURL, _ := url.Parse(endpoint)
	proxy := httputil.NewSingleHostReverseProxy(endpointURL)
	var lenses atomic.Int32
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, e := io.ReadAll(r.Body)
		if e != nil {
			t.Error(e)
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		var request struct {
			Method string
			Params []json.RawMessage
		}
		if json.Unmarshal(body, &request) == nil && request.Method == "eth_call" && len(request.Params) == 3 {
			lenses.Add(1)
		}
		proxy.ServeHTTP(w, r)
	}))
	defer rpc.Close()
	cfg := testCLIConfig(t.TempDir())
	cfg.Mode = "send"
	cfg.Chain.ChainID = 1
	cfg.Chain.RPCURL = rpc.URL
	cfg.Curator.URL = curator.URL + "/api/v1"
	cfg.Strategies = []string{strings.ToLower(executor.Hex())}
	cfg.Signer.Address = ""
	path := writeCLIConfig(t, t.TempDir(), cfg)
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(key)))
	prepare := func() (keeper.Plan, error) {
		before := lenses.Load()
		var output bytes.Buffer
		if e := runBot(ctx, path, true, true, &output); e != nil {
			return keeper.Plan{}, e
		}
		var plans map[string]keeper.Plan
		if e := json.Unmarshal(output.Bytes(), &plans); e != nil {
			return keeper.Plan{}, e
		}
		plan, exists := plans[cfg.Strategies[0]]
		if !exists || len(plans) != 1 || plan.Snapshot.Account != account || plan.Snapshot.Caller != caller || len(plan.Outputs) != 6 || plan.Results[0] != amount.String() || plan.Results[2] != amount.String() || lenses.Load()-before != 1 {
			t.Fatalf("recipe host lost address identity/input derivation/lens budget: %+v", plan.Results)
		}
		for _, output := range plan.Outputs {
			if len(output.Inputs) != 0 || len(output.Actions) != 0 || len(output.SkipSteps) != 0 {
				t.Fatal("on-chain issuer recipe acquired provider-owned runtime values")
			}
		}
		return plan, nil
	}
	read := func(target common.Address, artifact serviceArtifact, method string, args ...any) *big.Int {
		data, err := client.CallContract(ctx, ethereum.CallMsg{To: &target, Data: servicePack(t, artifact, method, args...)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		values, err := artifact.ABI.Unpack(method, data)
		if err != nil {
			t.Fatal(err)
		}
		return values[0].(*big.Int)
	}
	t.Run("full-amount-rounding-and-collateral-depeg", func(t *testing.T) {
		call(owner, handler, handlerArtifact, "setPegPercentage", big.NewInt(9999))
		getRate := ""
		for name, method := range handlerArtifact.ABI.Methods {
			if method.RawName == "getRate" && len(method.Inputs) == 1 {
				getRate = name
			}
		}
		if getRate == "" {
			t.Fatal("source-pinned handler has no collateral rate getter")
		}
		tiny := big.NewInt(2_999_999_999_999)
		for _, price := range []int64{100_000_000, 90_000_000} {
			head, e := client.HeaderByNumber(ctx, nil)
			if e != nil {
				t.Fatal(e)
			}
			call(owner, oracle, oracleArtifact, "set", big.NewInt(price), new(big.Int).SetUint64(head.Time))
			quote := read(handler, handlerArtifact, "calculateCollateralAmount", usdc, tiny)
			if quote.Int64() != 1 {
				t.Fatalf("full-amount quote lost floor rounding or the $1 collateral clamp: price=%d quote=%s", price, quote)
			}
			rate := read(handler, handlerArtifact, getRate, usdc)
			scaled := new(big.Int).Div(new(big.Int).Mul(tiny, rate), big.NewInt(1e18))
			if scaled.Cmp(quote) <= 0 || (price == 100_000_000 && scaled.Int64() != 2) || (price == 90_000_000 && rate.Int64() != 1_110_999) {
				t.Fatalf("one-token rate did not expose the native full-amount/depeg distinction: price=%d rate=%s scaled=%s quote=%s", price, rate, scaled, quote)
			}
		}
		call(owner, handler, handlerArtifact, "setPegPercentage", big.NewInt(9990))
		head, e := client.HeaderByNumber(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		call(owner, oracle, oracleArtifact, "set", big.NewInt(1e8), new(big.Int).SetUint64(head.Time))
	})
	plan, err := prepare()
	if err != nil {
		t.Fatal(err)
	}
	// Removing readiness after preparing an executable plan must still make the
	// complete execute simulation fail; a quote cannot authorize a USN burn.
	for _, failure := range []string{"whitelist", "treasury-balance", "treasury-allowance", "block-limit", "oracle-stale", "minimum-output"} {
		t.Run(failure, func(t *testing.T) {
			switch failure {
			case "whitelist":
				call(owner, handler, handlerArtifact, "removeWhitelistedUser", account)
			case "treasury-balance":
				call(treasury, usdc, tokenArtifact, "transfer", owner, big.NewInt(10_000_000))
			case "treasury-allowance":
				call(treasury, usdc, tokenArtifact, "approve", handler, big.NewInt(0))
			case "block-limit":
				call(owner, handler, handlerArtifact, "setRedeemLimitPerBlock", big.NewInt(0))
			case "oracle-stale":
				head, e := client.HeaderByNumber(ctx, nil)
				if e != nil {
					t.Fatal(e)
				}
				call(owner, oracle, oracleArtifact, "set", big.NewInt(1e8), new(big.Int).SetUint64(head.Time-86401))
			case "minimum-output":
				call(owner, handler, handlerArtifact, "setPegPercentage", big.NewInt(9980))
			}
			if _, err := client.CallContract(ctx, ethereum.CallMsg{From: caller, To: &executor, Data: plan.Calldata}, nil); err == nil {
				t.Fatal("native failure was accepted by real handler/recipe execution")
			}
			if failure == "treasury-allowance" || failure == "minimum-output" {
				// These failures happen after burnFrom or after the payout. A mined
				// reverting execution proves the earlier effects roll back atomically.
				var hash common.Hash
				args := map[string]any{"from": caller, "to": executor, "gas": hexutil.Uint64(12_000_000), "data": hexutil.Bytes(plan.Calldata)}
				if err := client.Client().CallContext(ctx, &hash, "eth_sendTransaction", args); err != nil {
					t.Fatal(err)
				}
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					receipt, err := client.TransactionReceipt(ctx, hash)
					if err == nil {
						if receipt.Status != types.ReceiptStatusFailed || len(receipt.Logs) != 0 {
							t.Fatal("failed native execution was not reverted with all logs rolled back")
						}
						break
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
			if read(usn, tokenArtifact, "balanceOf", account).Cmp(amount) != 0 || read(usn, tokenArtifact, "allowance", account, handler).Sign() != 0 || read(usdc, tokenArtifact, "balanceOf", account).Sign() != 0 {
				t.Fatal("failed native execution changed account funds or allowance")
			}
			if failure != "treasury-balance" && read(usdc, tokenArtifact, "balanceOf", treasury).Int64() != 10_000_000 {
				t.Fatal("failed native execution changed treasury USDC")
			}
			_, prepareErr := prepare()
			if prepareErr == nil || !strings.Contains(prepareErr.Error(), "full recipe simulation failed") {
				t.Fatalf("issuer failure did not reach full recipe simulation: %v", prepareErr)
			}
			beforeNonce, err := client.NonceAt(ctx, caller, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := runBot(ctx, path, true, false, new(bytes.Buffer)); err == nil || !strings.Contains(err.Error(), "full recipe simulation failed") {
				t.Fatalf("send admission accepted failing issuer recipe: %v", err)
			}
			if afterNonce, err := client.NonceAt(ctx, caller, nil); err != nil || afterNonce != beforeNonce {
				t.Fatalf("failed preparation consumed a nonce: before=%d after=%d error=%v", beforeNonce, afterNonce, err)
			}
			for _, name := range []string{"strategy." + cfg.Strategies[0] + ".v1", "txmanager.active.v1"} {
				if found, err := state.Read(cfg.StateDir, name, new(any)); err != nil || found {
					t.Fatalf("failed preparation became durable work: %s / %v", name, err)
				}
			}
			switch failure {
			case "whitelist":
				call(owner, handler, handlerArtifact, "addWhitelistedUser", account)
			case "treasury-balance":
				call(owner, usdc, tokenArtifact, "transfer", treasury, big.NewInt(10_000_000))
			case "treasury-allowance":
				call(treasury, usdc, tokenArtifact, "approve", handler, big.NewInt(10_000_000))
			case "block-limit":
				call(owner, handler, handlerArtifact, "setRedeemLimitPerBlock", new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)))
			case "oracle-stale":
				head, e := client.HeaderByNumber(ctx, nil)
				if e != nil {
					t.Fatal(e)
				}
				call(owner, oracle, oracleArtifact, "set", big.NewInt(1e8), new(big.Int).SetUint64(head.Time))
			case "minimum-output":
				call(owner, handler, handlerArtifact, "setPegPercentage", big.NewInt(9990))
			}
		})
	}
	plan, err = prepare()
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Client().CallContext(ctx, nil, "anvil_stopImpersonatingAccount", caller); err != nil {
		t.Fatal(err)
	}
	cfg.Mode = "send"
	path = writeCLIConfig(t, t.TempDir(), cfg)
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(key)))
	startServiceConfirmationMining(t, ctx, client, caller)
	if err = runBot(ctx, path, true, false, new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	var job keeper.Job
	if found, e := state.Read(cfg.StateDir, "strategy."+cfg.Strategies[0]+".v1", &job); e != nil || !found || job.Phase != "settled" || !job.Result.Confirmed || job.Result.Reverted || len(job.Progress) != 6 || !job.Progress[4].Done {
		t.Fatalf("native canonical address job did not settle: %+v %v", job.Result, e)
	}
	if found, e := state.Read(cfg.StateDir, "txmanager.active.v1", new(any)); e != nil || found {
		t.Fatalf("native completed sender journal retained: %v", e)
	}
	if read(usn, tokenArtifact, "balanceOf", account).Sign() != 0 || read(usn, tokenArtifact, "totalSupply").Sign() != 0 || read(usn, tokenArtifact, "allowance", account, handler).Sign() != 0 {
		t.Fatal("USN burn or exact-allowance cleanup failed")
	}
	if read(usdc, tokenArtifact, "balanceOf", account).Int64() != 1_233_332 || read(usdc, tokenArtifact, "balanceOf", treasury).Int64() != 8_766_668 {
		t.Fatal("18-to-6 decimal redemption payment or treasury debit is incorrect")
	}
	nonce, e := client.NonceAt(ctx, caller, nil)
	if e != nil {
		t.Fatal(e)
	}
	if err = runBot(ctx, path, true, false, new(bytes.Buffer)); err != nil {
		t.Fatal(err)
	}
	if after, e := client.NonceAt(ctx, caller, nil); e != nil || after != nonce {
		t.Fatal("native restart repeated completed redemption", e)
	}
}
