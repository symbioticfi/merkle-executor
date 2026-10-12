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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	native "github.com/symbioticfi/merkle-executor/api/bindings/noonredeemhandlerv2"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/redemptions"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAnvilNoonNativeBurnAndUSDCPayment(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("set MERKLE_ANVIL_TEST=1 for source-pinned Noon execution")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	client, _ := startServiceAnvil(t, ctx)
	// This owned ephemeral process has no fork or external RPC. Chain 1 is needed
	// for the provider's canonical Ethereum route allowlist, not a live deployment.
	if err := client.Client().CallContext(ctx, nil, "anvil_setChainId", 1); err != nil {
		t.Fatal(err)
	}
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil || len(owners) < 2 {
		t.Fatalf("local accounts unavailable: %v", err)
	}
	owner, treasury := owners[0], owners[1]
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
	tokens, redeem, executors := erc20.NewERC20(), native.NewNoonRedeemHandlerV2(), recipeexecutor.NewRecipeExecutor()
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
	action := leaf(handler, redeem.PackRedeemOnchain(usdc, new(big.Int)))
	approval.Proof, action.Proof = []common.Hash{action.Hash}, []common.Hash{approval.Hash}
	a, b := approval.Hash, action.Hash
	if a.Cmp(b) > 0 {
		a, b = b, a
	}
	root := crypto.Keccak256Hash(a.Bytes(), b.Bytes())
	call(owner, account, accountArtifact, "configure", root, [][32]byte{approval.Hash, action.Hash}, executor)
	serviceSeedTransaction(t, ctx, client, owner, &executor, executors.PackSetCaller(owner))
	input := new(big.Int).Lsh(big.NewInt(1), 216)
	result := func(index int64) *big.Int {
		return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(index))
	}
	query := issuerStep(t, 0, []string{"address", "bytes", "uint256[]", "uint256"}, usdc, tokens.PackBalanceOf(account), []*big.Int{}, new(big.Int))[1:]
	recipe := recipes.Recipe{Queries: [][]byte{query}, Steps: [][]byte{
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(0)),
		issuerStep(t, recipes.Approve, []string{"uint256", "address", "address", "bytes32"}, input, usn, handler, approval.Hash),
		issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{action.Hash}, [][]*big.Int{{}}),
		issuerStep(t, recipes.Approve, []string{"uint256", "address", "address", "bytes32"}, big.NewInt(0), usn, handler, approval.Hash),
		issuerStep(t, recipes.Read, []string{"uint256"}, big.NewInt(0)),
		issuerStep(t, recipes.Sub, []string{"uint256", "uint256"}, result(4), result(0)),
		issuerStep(t, recipes.LessOrEqual, []string{"uint256", "uint256"}, big.NewInt(1_233_332), result(5)),
	}}
	serviceSeedTransaction(t, ctx, client, owner, &executor, executors.PackSetRecipe(recipe.Queries, recipe.Steps))
	entries := make([]any, 2)
	for i, v := range []recipes.Leaf{approval, action} {
		entries[i] = map[string]any{"hash": v.Hash.Hex(), "target": v.Target.Hex(), "value": "zero", "data": hexutil.Encode(v.Data), "dataMask": hexutil.Encode(v.DataMask), "proof": []string{v.Proof[0].Hex()}}
	}
	rawTree, _ := json.Marshal(map[string]any{"root": root.Hex(), "chainId": 1, "account": account.Hex(), "leaves": entries})
	treePath := filepath.Join(t.TempDir(), "tree.json")
	if err := os.WriteFile(treePath, rawTree, 0600); err != nil {
		t.Fatal(err)
	}
	index := uint(0)
	cfg := redemptions.Config{Family: "noon", Mode: "redeem-usn", ChainID: 1, Token: usn.Hex(), OutputAsset: usdc.Hex(), Target: handler.Hex(), Amount: amount.String(), Action: "2:0", AmountInput: &index}
	p, err := redemptions.Factory(serviceNode(t, cfg), registry.Deps{Chain: client})
	if err != nil {
		t.Fatal(err)
	}
	strategy := keeper.Strategy{Config: keeper.Config{Account: account.Hex(), Executor: executor.Hex(), Tree: treePath}, Providers: []provider.Provider{p}}
	planner := keeper.Planner{Chain: client, ChainID: big.NewInt(1), Caller: owner}
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
	plan, err := planner.Prepare(ctx, strategy)
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
			if _, err := client.CallContract(ctx, ethereum.CallMsg{From: owner, To: &executor, Data: plan.Calldata}, nil); err == nil {
				t.Fatal("native failure was accepted by real handler/recipe execution")
			}
			if failure == "treasury-allowance" || failure == "minimum-output" {
				// These failures happen after burnFrom or after the payout. A mined
				// reverting execution proves the earlier effects roll back atomically.
				var hash common.Hash
				args := map[string]any{"from": owner, "to": executor, "gas": hexutil.Uint64(12_000_000), "data": hexutil.Bytes(plan.Calldata)}
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
			if failure != "minimum-output" {
				_, prepareErr := planner.Prepare(ctx, strategy)
				if prepareErr == nil || (failure == "oracle-stale" && errors.Is(prepareErr, provider.ErrNotReady)) {
					t.Fatal("invalid native readiness passed preparation or stale quote became idle")
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
	plan, err = planner.Prepare(ctx, strategy)
	if err != nil {
		t.Fatal(err)
	}
	receipt := serviceSeedTransaction(t, ctx, client, owner, &executor, plan.Calldata)
	if read(usn, tokenArtifact, "balanceOf", account).Sign() != 0 || read(usn, tokenArtifact, "totalSupply").Sign() != 0 || read(usn, tokenArtifact, "allowance", account, handler).Sign() != 0 {
		t.Fatal("USN burn or exact-allowance cleanup failed")
	}
	if read(usdc, tokenArtifact, "balanceOf", account).Int64() != 1_233_332 || read(usdc, tokenArtifact, "balanceOf", treasury).Int64() != 8_766_668 {
		t.Fatal("18-to-6 decimal redemption payment or treasury debit is incorrect")
	}
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := plan.Snapshot
	snapshot.BlockNumber, snapshot.BlockHash, snapshot.Timestamp = head.Number, head.Hash(), head.Time
	snapshot.Execution = &provider.Execution{TxHash: receipt.TxHash, BlockNumber: receipt.BlockNumber, BlockHash: receipt.BlockHash}
	for _, log := range receipt.Logs {
		snapshot.Execution.Logs = append(snapshot.Execution.Logs, *log)
	}
	progress, err := p.Advance(ctx, snapshot, plan.Outputs[0], nil)
	if err != nil || !progress.Done || progress.Outcome != "redeemed" {
		t.Fatalf("actual native receipt did not reconcile: %+v %v", progress, err)
	}
}
