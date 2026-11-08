package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	executorABI "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/providers/aave"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type capacityReader struct {
	*ethclient.Client
	calls int
}

func TestAnvilFrontendNativeAaveLending(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	client, _ := startServiceAnvil(t, ctx)
	reader := &capacityReader{Client: client}
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	owner := owners[0]
	raw, err := os.ReadFile("../../sdk/providers/aave/testdata/frontend-lending.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Account, Pool, Asset, AToken, DebtToken, Rwa, Issuer common.Address
		Recipe                                               hexutil.Bytes
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	r, err := recipes.DecodeRecipe(fixture.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	load := func(name string) serviceArtifact {
		raw, err := os.ReadFile("../../sdk/providers/aave/lens/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var a struct {
			ABI      json.RawMessage
			Bytecode hexutil.Bytes
		}
		if err = json.Unmarshal(raw, &a); err != nil {
			t.Fatal(err)
		}
		parsed, err := abi.JSON(bytes.NewReader(a.ABI))
		if err != nil {
			t.Fatal(err)
		}
		return serviceArtifact{ABI: parsed, Bytecode: a.Bytecode}
	}
	deploy := func(a serviceArtifact, args ...any) common.Address {
		data := bytes.Clone(a.Bytecode)
		if len(args) > 0 {
			data = append(data, servicePack(t, a, "", args...)...)
		}
		return serviceSeedTransaction(t, ctx, client, owner, nil, data).ContractAddress
	}
	install := func(target common.Address, a serviceArtifact, args ...any) {
		source := deploy(a, args...)
		code, err := client.CodeAt(ctx, source, nil)
		if err != nil {
			t.Fatal(err)
		}
		var ignored any
		if err = client.Client().CallContext(ctx, &ignored, "anvil_setCode", target, hexutil.Bytes(code)); err != nil {
			t.Fatal(err)
		}
	}
	call := func(target common.Address, a serviceArtifact, method string, args ...any) {
		serviceSeedTransaction(t, ctx, client, owner, &target, servicePack(t, a, method, args...))
	}
	aa, ca, ea := loadGuardedMerklArtifact(t, "FixtureGuardedAccount"), loadGuardedMerklArtifact(t, "RecipeCapability"), loadServiceArtifact(t, "RecipeExecutor")
	pa, ta, ia := load("FixtureAave"), load("FixtureAaveToken"), load("FixtureAavePurchase")
	install(fixture.Account, aa)
	install(fixture.Pool, pa)
	for _, target := range []common.Address{fixture.Asset, fixture.AToken, fixture.DebtToken, fixture.Rwa} {
		install(target, ta)
	}
	install(fixture.Issuer, ia, fixture.Asset, fixture.Rwa)
	capability, executor := deploy(ca), deploy(ea, owner, fixture.Account)
	call(fixture.Account, aa, "setCapability", capability)
	hashes := make([][32]byte, len(tree.Leaves))
	for i, leaf := range tree.Leaves {
		hashes[i] = leaf.Hash
	}
	call(fixture.Account, aa, "configure", tree.Root, hashes, executor)
	call(fixture.Pool, pa, "setAsset", fixture.Asset)
	call(fixture.Pool, pa, "setDebtToken", fixture.DebtToken)
	call(fixture.Pool, pa, "setAToken", fixture.AToken)
	call(fixture.Pool, pa, "setState", new(big.Int), big.NewInt(100e8), new(big.Int), big.NewInt(8000))
	configuration := new(big.Int).Or(big.NewInt(7500|(8000<<16)|(6<<48)|(1<<56)), new(big.Int).Lsh(big.NewInt(1), 58))
	call(fixture.Pool, pa, "setConfiguration", configuration)
	call(fixture.Pool, pa, "setLiquidity", big.NewInt(1000e6), big.NewInt(900e6), new(big.Int))
	call(fixture.Pool, pa, "setPrice", big.NewInt(1e8), false)
	call(fixture.Asset, ta, "mint", fixture.Account, big.NewInt(1000e6))
	call(fixture.Asset, ta, "mint", fixture.AToken, big.NewInt(1000e6))
	call(fixture.DebtToken, ta, "mint", fixture.Account, big.NewInt(100e6))
	e := executorABI.NewRecipeExecutor()
	serviceSeedTransaction(t, ctx, client, owner, &executor, e.PackSetCaller(owner))
	serviceSeedTransaction(t, ctx, client, owner, &executor, e.PackSetRecipe(r.Queries, r.Steps))
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := provider.Snapshot{Account: fixture.Account, Executor: executor, Caller: owner, BlockNumber: header.Number}
	native, err := aave.New(reader)
	if err != nil {
		t.Fatal(err)
	}
	requirements, err := native.RecipeRequirements(snapshot, r, tree)
	if err != nil || len(requirements.Inputs) != 1 || requirements.Inputs[0] != 0 {
		t.Fatal(requirements, err)
	}
	outputs, err := native.PrepareRecipe(ctx, snapshot, r, tree, nil)
	if err != nil || len(outputs) != 1 {
		t.Fatal(outputs, err)
	}
	out := outputs[0]
	if reader.calls != 1 || out.Inputs[0] != "539999999" || len(out.Results) == 0 {
		t.Fatalf("reads=%d output=%+v", reader.calls, out)
	}
	encoded, _, err := recipes.Assemble(r, tree, out.Inputs, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	returned, err := client.CallContract(ctx, ethereum.CallMsg{From: owner, To: &executor, Data: encoded}, header.Number)
	if err != nil {
		t.Fatal("original frontend recipe execution", err)
	}
	if err = recipes.VerifyResults(encoded, returned, out.Results); err != nil {
		t.Fatal(err)
	}
	// The full native recipe rejects a stale capacity even after the prefix supplied collateral.
	excessive, _, err := recipes.Assemble(r, tree, map[uint]string{0: "900000000"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.CallContract(ctx, ethereum.CallMsg{From: owner, To: &executor, Data: excessive}, header.Number); err == nil {
		t.Fatal("capacity bypassed committed native health floor")
	}
	// A foreign runtime account cannot replay the same authenticated supply prefix.
	wrong := snapshot
	wrong.Account = owners[1]
	if _, err = native.PrepareRecipe(ctx, wrong, r, tree, nil); err == nil {
		t.Fatal("capacity accepted foreign account")
	}
	serviceSeedTransaction(t, ctx, client, owner, &executor, encoded)
	got, err := client.CallContract(ctx, ethereum.CallMsg{To: &fixture.Rwa, Data: servicePack(t, ta, "balanceOf", fixture.Account)}, nil)
	if err != nil || new(big.Int).SetBytes(got).Cmp(new(big.Int).Mul(big.NewInt(539999999), big.NewInt(1e12))) != 0 {
		t.Fatal("purchase did not consume the selected native borrow", err)
	}
}

func (r *capacityReader) CallContractWithCode(ctx context.Context, msg ethereum.CallMsg, block *big.Int, code []byte) ([]byte, error) {
	r.calls++
	var out hexutil.Bytes
	err := r.Client.Client().CallContext(ctx, &out, "eth_call", map[string]any{"from": msg.From, "to": msg.To, "data": hexutil.Bytes(msg.Data), "gas": hexutil.Uint64(msg.Gas)}, hexutil.EncodeBig(block), map[string]any{msg.To.Hex(): map[string]any{"code": hexutil.Bytes(code)}})
	return out, err
}
func TestAnvilAavePostSupplyCapacity(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	client, _ := startServiceAnvil(t, ctx)
	reader := &capacityReader{Client: client}
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	owner := owners[0]
	raw, err := os.ReadFile("../../sdk/providers/aave/lens/FixtureAave.json")
	if err != nil {
		t.Fatal(err)
	}
	var fa struct {
		ABI      json.RawMessage
		Bytecode hexutil.Bytes
	}
	if err = json.Unmarshal(raw, &fa); err != nil {
		t.Fatal(err)
	}
	parsed, err := abi.JSON(bytes.NewReader(fa.ABI))
	if err != nil {
		t.Fatal(err)
	}
	poolArtifact := serviceArtifact{ABI: parsed, Bytecode: fa.Bytecode}
	deploy := func(a serviceArtifact, args ...any) common.Address {
		data := bytes.Clone(a.Bytecode)
		if len(args) > 0 {
			data = append(data, servicePack(t, a, "", args...)...)
		}
		return serviceSeedTransaction(t, ctx, client, owner, nil, data).ContractAddress
	}
	aa, ea := loadGuardedMerklArtifact(t, "FixtureGuardedAccount"), loadServiceArtifact(t, "RecipeExecutor")
	account, pool := deploy(aa), deploy(poolArtifact)
	executor := deploy(ea, owner, account)
	asset := pool
	call := func(target common.Address, a serviceArtifact, method string, args ...any) {
		serviceSeedTransaction(t, ctx, client, owner, &target, servicePack(t, a, method, args...))
	}
	call(pool, poolArtifact, "setAsset", asset)
	e := executorABI.NewRecipeExecutor()
	serviceSeedTransaction(t, ctx, client, owner, &executor, e.PackSetCaller(owner))
	result := func(n int64) *big.Int { return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(n)) }
	input := new(big.Int).Lsh(big.NewInt(1), 216)
	step := func(op uint8, names []string, args ...any) []byte { return issuerStep(t, op, names, args...) }
	query := func(data []byte, offset int64) []byte {
		return step(0, []string{"address", "bytes", "uint256[]", "uint256"}, pool, data, []*big.Int{}, big.NewInt(offset))[1:]
	}
	borrowData := servicePack(t, poolArtifact, "borrow", asset, new(big.Int), big.NewInt(2), uint16(0), account)
	const offset = 36
	leaf := func(target common.Address, data []byte, offset int) recipes.Leaf {
		mask := make([]byte, len(data))
		if offset > 0 {
			for i := offset; i < offset+32; i++ {
				mask[i] = 255
			}
		}
		l := recipes.Leaf{Target: target, Value: new(big.Int), Data: data, DataMask: mask}
		l.Hash, _ = recipes.HashLeaf(l)
		return l
	}
	supply := leaf(pool, servicePack(t, poolArtifact, "supply", asset, big.NewInt(1000e6), account, uint16(0)), 0)
	borrow := leaf(pool, borrowData, offset)
	supply.Proof = []common.Hash{borrow.Hash}
	borrow.Proof = []common.Hash{supply.Hash}
	first, second := supply.Hash, borrow.Hash
	if first.Cmp(second) > 0 {
		first, second = second, first
	}
	root := crypto.Keccak256Hash(first[:], second[:])
	tree := &recipes.Tree{Root: root, Leaves: []recipes.Leaf{supply, borrow}}
	call(account, aa, "configure", root, [][32]byte{supply.Hash, borrow.Hash}, executor)
	binding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(int64(offset)), 224), result(4))
	r := recipes.Recipe{Queries: [][]byte{query(servicePack(t, poolArtifact, "balanceOf", account), 0), query(servicePack(t, poolArtifact, "getUserAccountData", account), 160)}, Steps: [][]byte{
		step(recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{supply.Hash}, [][]*big.Int{{}}),
		step(recipes.Read, []string{"uint256"}, new(big.Int)),
		step(recipes.SaturatingSub, []string{"uint256", "uint256"}, big.NewInt(1000e6), result(1)),
		step(recipes.SaturatingSub, []string{"uint256", "uint256"}, result(2), big.NewInt(1)),
		step(recipes.Min, []string{"uint256", "uint256"}, input, result(3)),
		step(recipes.LessOrEqual, []string{"uint256", "uint256"}, big.NewInt(1), result(4)),
		step(recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, result(4), [][32]byte{borrow.Hash}, [][]*big.Int{{binding}}),
		step(recipes.Read, []string{"uint256"}, big.NewInt(1)),
		step(recipes.LessOrEqual, []string{"uint256", "uint256"}, big.NewInt(1_250_000_000_000_000_000), result(7)),
	}}
	serviceSeedTransaction(t, ctx, client, owner, &executor, e.PackSetRecipe(r.Queries, r.Steps))
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := provider.Snapshot{Account: account, Executor: executor, Caller: owner, BlockNumber: header.Number}
	native, err := aave.New(reader)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := native.PrepareRecipe(ctx, snapshot, r, tree, nil)
	var out provider.Output
	if len(outputs) == 1 {
		out = outputs[0]
	}
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 {
		t.Fatalf("capacity physical reads=%d", reader.calls)
	}
	if out.Inputs[0] != "539999999" || out.Results[1] != "100000000" {
		t.Fatalf("capacity=%v witness=%v", out.Inputs, out.Results)
	}
	if got, err := client.CallContract(ctx, ethereum.CallMsg{To: &pool, Data: servicePack(t, poolArtifact, "collateral")}, nil); err != nil || new(big.Int).SetBytes(got).Sign() != 0 {
		t.Fatal("prefix leaked state", err)
	}
	encoded, _, err := recipes.Assemble(r, tree, out.Inputs, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	returned, err := client.CallContract(ctx, ethereum.CallMsg{From: owner, To: &executor, Data: encoded}, header.Number)
	if err != nil {
		t.Fatal("full original execute", err)
	}
	if err = recipes.VerifyResults(encoded, returned, out.Results); err != nil {
		t.Fatal(err)
	}
	// Runtime identity and account roles remain real during the prefix replay.
	wrong := snapshot
	wrong.Executor = owners[1]
	if _, err = native.PrepareRecipe(ctx, wrong, r, tree, nil); err == nil {
		t.Fatal("prefix ignored account permissions")
	}
	wrongWitness := map[uint]string{1: "1"}
	if recipes.VerifyResults(encoded, returned, wrongWitness) == nil {
		t.Fatal("wrong prefix witness accepted")
	}
	// Stale/excessive capacity cannot bypass the committed health floor.
	excessive, _, err := recipes.Assemble(r, tree, map[uint]string{0: "900000000"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.CallContract(ctx, ethereum.CallMsg{From: owner, To: &executor, Data: excessive}, header.Number); err == nil {
		t.Fatal("excess capacity escaped health check")
	}
	receipt := serviceSeedTransaction(t, ctx, client, owner, &executor, encoded)
	t.Logf("capacity physical RPCs=1, prefix capacity=%s, full execution gas=%d", out.Inputs[0], receipt.GasUsed)
	if out.Task != nil {
		t.Fatal("native capacity output created external settlement work")
	}

	// Probe the bounded native edge cases without a supply prefix.
	artifactRaw, err := os.ReadFile("../../sdk/providers/aave/lens/AaveBorrowProbe.json")
	if err != nil {
		t.Fatal(err)
	}
	var probe struct{ Runtime hexutil.Bytes }
	if err = json.Unmarshal(artifactRaw, &probe); err != nil {
		t.Fatal(err)
	}
	capacityArgs := step(0, []string{"address", "address", "address", "uint256"}, pool, asset, account, big.NewInt(1_250_000_000_000_000_000))[1:]
	probeData := append(crypto.Keccak256([]byte("maxBorrow(address,address,address,uint256)"))[:4], capacityArgs...)
	configuration := new(big.Int).Or(big.NewInt(7500|(8000<<16)|(6<<48)|(1<<56)), new(big.Int).Lsh(big.NewInt(1), 58))
	baseProbeData := bytes.Clone(probeData)
	reset := func() {
		probeData = bytes.Clone(baseProbeData)
		call(pool, poolArtifact, "setFailBitmap", false)
		call(pool, poolArtifact, "setState", big.NewInt(1000e8), big.NewInt(100e8), big.NewInt(600e8), big.NewInt(8000))
		call(pool, poolArtifact, "setConfiguration", configuration)
		call(pool, poolArtifact, "setMode", new(big.Int), new(big.Int))
		call(pool, poolArtifact, "setUser", new(big.Int), common.Address{}, new(big.Int))
		call(pool, poolArtifact, "setLiquidity", big.NewInt(1000e6), big.NewInt(900e6), new(big.Int))
		call(pool, poolArtifact, "setPrice", big.NewInt(1e8), false)
	}
	for _, tc := range []struct {
		name  string
		setup func()
		want  string
		fail  bool
	}{
		{"health debt", func() {}, "539999999", false},
		{"native LTV", func() {
			call(pool, poolArtifact, "setState", big.NewInt(1000e8), big.NewInt(100e8), big.NewInt(123e8), big.NewInt(8000))
		}, "122999999", false},
		{"cash", func() { call(pool, poolArtifact, "setLiquidity", big.NewInt(7e6), big.NewInt(12e6), new(big.Int)) }, "7000000", false},
		{"cap", func() {
			capConfig := new(big.Int).Or(configuration, new(big.Int).Lsh(big.NewInt(50), 80))
			call(pool, poolArtifact, "setConfiguration", capConfig)
			call(pool, poolArtifact, "setLiquidity", big.NewInt(1000e6), big.NewInt(900e6), big.NewInt(47e6+1))
		}, "2999998", false},
		{"emode", func() {
			call(pool, poolArtifact, "setMode", big.NewInt(2), new(big.Int).Lsh(big.NewInt(1), 127))
			call(pool, poolArtifact, "setUser", big.NewInt(2), owners[2], new(big.Int))
			call(pool, poolArtifact, "setState", big.NewInt(1000e8), big.NewInt(100e8), big.NewInt(800e8), big.NewInt(9000))
		}, "619999999", false},
		{"unborrowable emode", func() { call(pool, poolArtifact, "setMode", big.NewInt(1), big.NewInt(1)) }, "0", false},
		{"frozen", func() {
			call(pool, poolArtifact, "setConfiguration", new(big.Int).Or(configuration, new(big.Int).Lsh(big.NewInt(1), 57)))
		}, "0", false},
		{"siloed", func() {
			call(pool, poolArtifact, "setConfiguration", new(big.Int).Or(configuration, new(big.Int).Lsh(big.NewInt(1), 62)))
		}, "0", false},
		{"isolated", func() {
			call(pool, poolArtifact, "setUser", big.NewInt(2), owners[2], new(big.Int).Lsh(big.NewInt(1), 212))
		}, "0", false},
		{"existing siloed debt", func() {
			call(pool, poolArtifact, "setUser", big.NewInt(1), owners[2], new(big.Int).Lsh(big.NewInt(1), 62))
		}, "0", false},

		{"zero price", func() { call(pool, poolArtifact, "setPrice", new(big.Int), false) }, "0", false},
		{"inactive", func() {
			call(pool, poolArtifact, "setConfiguration", new(big.Int).Xor(configuration, new(big.Int).Lsh(big.NewInt(1), 56)))
		}, "0", false},
		{"borrow disabled", func() {
			call(pool, poolArtifact, "setConfiguration", new(big.Int).Xor(configuration, new(big.Int).Lsh(big.NewInt(1), 58)))
		}, "0", false},
		{"paused", func() {
			call(pool, poolArtifact, "setConfiguration", new(big.Int).Or(configuration, new(big.Int).Lsh(big.NewInt(1), 60)))
		}, "0", false},
		{"emode overflow", func() { call(pool, poolArtifact, "setMode", big.NewInt(256), new(big.Int)) }, "0", false},
		{"failed emode bitmap", func() {
			call(pool, poolArtifact, "setMode", big.NewInt(1), big.NewInt(1))
			call(pool, poolArtifact, "setFailBitmap", true)
		}, "", true},
		{"exhausted cap", func() {
			call(pool, poolArtifact, "setConfiguration", new(big.Int).Or(configuration, new(big.Int).Lsh(big.NewInt(50), 80)))
			call(pool, poolArtifact, "setLiquidity", big.NewInt(1000e6), big.NewInt(900e6), big.NewInt(51e6))
		}, "0", false},
		{"invalid health floor", func() { big.NewInt(999999999999999999).FillBytes(probeData[len(probeData)-32:]) }, "", true},
		{"unsupported decimals", func() {
			config := new(big.Int).Xor(configuration, new(big.Int).Lsh(big.NewInt(6), 48))
			call(pool, poolArtifact, "setConfiguration", new(big.Int).Or(config, new(big.Int).Lsh(big.NewInt(37), 48)))
		}, "", true},
		{"fractional conversion", func() {
			call(pool, poolArtifact, "setPrice", big.NewInt(3e8), false)
			call(pool, poolArtifact, "setState", big.NewInt(1000e8), big.NewInt(100e8), big.NewInt(1e8+1), big.NewInt(8000))
		}, "333332", false},
		{"full 512 health multiplication", func() {
			collateral := new(big.Int).Div(new(big.Int).Lsh(big.NewInt(1), 240), big.NewInt(10000))
			call(pool, poolArtifact, "setState", collateral, new(big.Int), new(big.Int).Lsh(big.NewInt(1), 250), big.NewInt(8000))
		}, "900000000", false},
		{"checked weighted collateral overflow", func() {
			call(pool, poolArtifact, "setState", new(big.Int).Lsh(big.NewInt(1), 255), new(big.Int), big.NewInt(1), big.NewInt(8000))
		}, "", true},

		{"failed price", func() { call(pool, poolArtifact, "setPrice", big.NewInt(1e8), true) }, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reset()
			tc.setup()
			h, err := client.HeaderByNumber(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			result, _, err := recipes.ProbePrefix(ctx, reader, recipes.Snapshot{BlockNumber: h.Number.Uint64(), Executor: executor, Account: account, Caller: owner}, recipes.Recipe{}, 0, recipes.PrefixRuntime{}, probe.Runtime, probeData)
			if tc.fail {
				if err == nil {
					t.Fatal("failed required read accepted")
				}
				return
			}
			if err != nil || new(big.Int).SetBytes(result).String() != tc.want {
				t.Fatalf("got %s want %s: %v", new(big.Int).SetBytes(result), tc.want, err)
			}
			amount := new(big.Int).SetBytes(result)
			rounded := new(big.Int).Add(amount, big.NewInt(1))
			switch tc.name {
			case "health debt":
				debtAfter := new(big.Int).Add(big.NewInt(100e8), new(big.Int).Mul(rounded, big.NewInt(100)))
				health := new(big.Int).Div(new(big.Int).Mul(big.NewInt(800e8), big.NewInt(1e18)), debtAfter)
				if health.Cmp(big.NewInt(1_250_000_000_000_000_000)) < 0 {
					t.Fatal("rounded debt breaches health floor")
				}
			case "native LTV":
				if rounded.Cmp(big.NewInt(123e6)) > 0 {
					t.Fatal("rounded debt breaches native LTV headroom")
				}
			case "cap":
				if new(big.Int).Add(big.NewInt(47e6+1), rounded).Cmp(big.NewInt(50e6)) > 0 {
					t.Fatal("rounded debt breaches reserve cap")
				}
			}

		})
	}
}
