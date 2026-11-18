package recipes

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
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type lensAnvilReader struct {
	*ethclient.Client
	calls int
}

func (r *lensAnvilReader) CallContractWithCode(ctx context.Context, msg ethereum.CallMsg, block *big.Int, code []byte) ([]byte, error) {
	return r.CallContractWithCodeOverrides(ctx, msg, block, map[common.Address][]byte{*msg.To: code})
}

func (r *lensAnvilReader) CallContractWithCodeOverrides(ctx context.Context, msg ethereum.CallMsg, block *big.Int, code map[common.Address][]byte) ([]byte, error) {
	r.calls++
	overrides := make(map[common.Address]any, len(code))
	for address, runtime := range code {
		overrides[address] = map[string]any{"code": hexutil.Bytes(runtime)}
	}
	var raw hexutil.Bytes
	err := r.Client.Client().CallContext(ctx, &raw, "eth_call", protocolio.CallArgument(msg), hexutil.EncodeBig(block), overrides)
	return raw, err
}

func loadLensFixture(t *testing.T, name string) anvilFixture {
	t.Helper()
	raw, err := os.ReadFile("lens/" + name + ".json")
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
	a, err := abi.JSON(bytes.NewReader(document.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return anvilFixture{ABI: a, Bytecode: document.Bytecode}
}
func TestAnvilLensContextAnchorsAndDivergence(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	client := startAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	sender := owners[0]
	e := loadAnvilFixture(t, "RecipeExecutor")
	executor := localTransaction(t, ctx, client, sender, nil, append(bytes.Clone(e.Bytecode), fixtureCalldata(t, e, "", sender, sender)...)).ContractAddress
	probeFixture := loadLensFixture(t, "LensProbe")
	probe := localTransaction(t, ctx, client, sender, nil, append(bytes.Clone(probeFixture.Bytecode), fixtureCalldata(t, probeFixture, "", executor, sender)...)).ContractAddress
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetCaller(sender))
	query := func(method string, bindings []*big.Int, args ...any) []byte {
		return pack(t, queryArgs, probe, fixtureCalldata(t, probeFixture, method, args...), bindings, big.NewInt(0))
	}
	r := Recipe{Queries: [][]byte{query("contextValue", []*big.Int{}), query("preview", []*big.Int{binding(4, input(0))}, big.NewInt(0))}, Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0))), step(Equal, pack(t, binaryArgs, input(0), result(0))), step(Read, pack(t, readArgs, big.NewInt(1)))}}
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{Executor: executor, Account: sender, Caller: sender, BlockNumber: header.Number.Uint64()}
	reader := &lensAnvilReader{Client: client}
	resolved, err := ResolveValues(ctx, reader, snapshot, r, []Value{{Source: Result, Value: big.NewInt(2)}})
	if err != nil {
		t.Fatal(err)
	}
	if reader.calls != 1 || resolved[0].Int64() != 42 || resolved[2].Int64() != 84 {
		t.Fatalf("calls=%d values=%v", reader.calls, resolved)
	}
	encoded, err := executorABI.TryPackExecute(r.Queries, r.Steps, []*big.Int{big.NewInt(42)}, [][]byte{})
	if err != nil {
		t.Fatal(err)
	}
	original, err := client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executor, Data: encoded}, header.Number)
	if err != nil {
		t.Fatal(err)
	}
	if err = VerifyResults(encoded, original, ResultsWitness(resolved)); err != nil {
		t.Fatal(err)
	}
	r.Steps[1] = step(LessOrEqual, pack(t, binaryArgs, input(0), result(0)))
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
	header, _ = client.HeaderByNumber(ctx, nil)
	snapshot.BlockNumber = header.Number.Uint64()
	if _, err = ResolveValues(ctx, reader, snapshot, r, []Value{{Source: Result, Value: big.NewInt(2)}}); err == nil {
		t.Fatal("lens chose an input from an upper bound")
	}

	for _, zero := range []bool{false, true} {
		maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 216), big.NewInt(1))
		bad := Recipe{Queries: r.Queries[:1], Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0)))}}
		if zero {
			bad.Steps = append(bad.Steps, step(MulDiv, pack(t, mulDivArgs, result(0), literal(2), literal(0))))
		} else {
			bad.Steps = append(bad.Steps, step(MulDiv, pack(t, mulDivArgs, result(0), maximum, literal(1))), step(MulDiv, pack(t, mulDivArgs, result(1), maximum, maximum)))
		}
		localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(bad.Queries, bad.Steps))
		h, _ := client.HeaderByNumber(ctx, nil)
		snapshot.BlockNumber = h.Number.Uint64()
		if _, err = ResolveValues(ctx, reader, snapshot, bad, []Value{{Source: Result, Value: big.NewInt(int64(len(bad.Steps) - 1))}}); err == nil {
			t.Fatal("lens accepted overflow/division by zero")
		}
		data, e := executorABI.TryPackExecute(bad.Queries, bad.Steps, []*big.Int{}, [][]byte{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executor, Data: data}, h.Number); e == nil {
			t.Fatal("original accepted arithmetic error")
		}
	}

	r = Recipe{Queries: [][]byte{query("callerCodeSize", []*big.Int{})}, Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0)))}}
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
	header, _ = client.HeaderByNumber(ctx, nil)
	snapshot.BlockNumber = header.Number.Uint64()
	resolved, err = ResolveValues(ctx, reader, snapshot, r, []Value{{Source: Result, Value: big.NewInt(0)}})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = executorABI.TryPackExecute(r.Queries, r.Steps, []*big.Int{}, [][]byte{})
	if err != nil {
		t.Fatal(err)
	}
	original, err = client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executor, Data: encoded}, header.Number)
	if err != nil {
		t.Fatal(err)
	}
	if err = VerifyResults(encoded, original, ResultsWitness(resolved)); err == nil {
		t.Fatal("code introspection divergence was accepted")
	}
}

func TestAnvilLensStatefulPrefix(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	client := startAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	sender, recipient := owners[0], owners[1]
	aa, ta, ea := loadAnvilFixture(t, "FixtureAccount"), loadAnvilFixture(t, "FixtureToken"), loadAnvilFixture(t, "RecipeExecutor")
	account := localTransaction(t, ctx, client, sender, nil, aa.Bytecode).ContractAddress
	token := localTransaction(t, ctx, client, sender, nil, ta.Bytecode).ContractAddress
	executor := localTransaction(t, ctx, client, sender, nil, append(bytes.Clone(ea.Bytecode), fixtureCalldata(t, ea, "", sender, account)...)).ContractAddress
	leaf := func(method string) Leaf {
		data := fixtureCalldata(t, ta, method, recipient, new(big.Int))
		mask := make([]byte, len(data))
		for i := 36; i < len(mask); i++ {
			mask[i] = 0xff
		}
		l := Leaf{Target: token, Value: new(big.Int), Data: data, DataMask: mask}
		var err error
		l.Hash, err = HashLeaf(l)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	transfer, approval := leaf("transfer"), leaf("approve")
	transfer.Proof = []common.Hash{approval.Hash}
	approval.Proof = []common.Hash{transfer.Hash}
	tree := &Tree{Root: hashPair(transfer.Hash, approval.Hash), Leaves: []Leaf{transfer, approval}}
	localTransaction(t, ctx, client, sender, &account, fixtureCalldata(t, aa, "configure", tree.Root, [][32]byte{transfer.Hash, approval.Hash}, executor))
	localTransaction(t, ctx, client, sender, &token, fixtureCalldata(t, ta, "mint", account, big.NewInt(1000)))
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetCaller(sender))
	query := pack(t, queryArgs, token, fixtureCalldata(t, ta, "balanceOf", account), []*big.Int{}, big.NewInt(0))
	reader := &lensAnvilReader{Client: client}
	for _, externalInput := range []bool{false, true} {
		amount := result(1)
		check := result(1)
		inputs := map[uint]string{}
		if externalInput {
			amount, check = input(0), input(0)
		}
		r := Recipe{Queries: [][]byte{query}, Steps: [][]byte{
			step(Read, pack(t, readArgs, big.NewInt(0))),
			step(Min, pack(t, binaryArgs, result(0), literal(17))),
			step(Equal, pack(t, binaryArgs, check, result(1))),
			step(Approve, pack(t, approveArgs, amount, token, recipient, approval.Hash)),
			manageStep(t, transfer.Hash, literal(1), []*big.Int{binding(36, amount)}),
			step(Call, pack(t, callArgs, token, fixtureCalldata(t, ta, "touch", new(big.Int)), []*big.Int{binding(4, amount)})),
			step(Read, pack(t, readArgs, big.NewInt(0))),
		}}
		localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
		header, _ := client.HeaderByNumber(ctx, nil)
		s := Snapshot{Executor: executor, Account: account, Caller: sender, BlockNumber: header.Number.Uint64()}
		resolved, err := ResolveValuesWithRuntime(ctx, reader, s, r, []Value{{Source: Result, Value: big.NewInt(6)}}, PrefixRuntime{Tree: tree})
		if err != nil {
			t.Fatal(err)
		}
		if resolved[1].Int64() != 17 || resolved[6].Int64() != 983 {
			t.Fatalf("external input=%v: values=%v", externalInput, resolved)
		}
		if externalInput {
			anchors, e := AnchoredInputs(r, nil)
			if e != nil {
				t.Fatal(e)
			}
			selected, e := EconomicResult(anchors[0], resolved)
			if e != nil {
				t.Fatal(e)
			}
			inputs[0] = selected.String()
		}
		encoded, _, err := Assemble(r, tree, inputs, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		original, err := client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executor, Data: encoded}, header.Number)
		if err != nil {
			t.Fatal(err)
		}
		if err = VerifyResults(encoded, original, ResultsWitness(resolved)); err != nil {
			t.Fatal(err)
		}
		if got := fixtureRead(t, ctx, client, ta, token, "balanceOf", account)[0].(*big.Int); got.Int64() != 1000 {
			t.Fatal("lens wrote live state")
		}
		if externalInput {
			// A different amount cannot satisfy the same recipe and state.
			wrong, _, e := Assemble(r, tree, map[uint]string{0: "16"}, nil, nil)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executor, Data: wrong}, header.Number); e == nil {
				t.Fatal("original executor accepted a different selected amount")
			}
			localTransaction(t, ctx, client, sender, &executor, encoded)
			if got := fixtureRead(t, ctx, client, ta, token, "balanceOf", account)[0].(*big.Int); got.Int64() != 983 {
				t.Fatalf("executed balance=%s", got)
			}
		}
	}
	if reader.calls != 2 {
		t.Fatalf("lens RPCs=%d", reader.calls)
	}
	// Zero amount must short-circuit allowance, as the original executor does.
	za := loadLensFixture(t, "ZeroApprovalToken")
	zeroToken := localTransaction(t, ctx, client, sender, nil, za.Bytecode).ContractAddress
	data := fixtureCalldata(t, za, "approve", recipient, big.NewInt(0))
	zeroLeaf := Leaf{Target: zeroToken, Value: new(big.Int), Data: data, DataMask: make([]byte, len(data))}
	zeroLeaf.Hash, _ = HashLeaf(zeroLeaf)
	tree = &Tree{Root: zeroLeaf.Hash, Leaves: []Leaf{zeroLeaf}}
	localTransaction(t, ctx, client, sender, &account, fixtureCalldata(t, aa, "configure", tree.Root, [][32]byte{zeroLeaf.Hash}, executor))
	r := Recipe{Queries: [][]byte{query}, Steps: [][]byte{step(Approve, pack(t, approveArgs, literal(0), zeroToken, recipient, zeroLeaf.Hash)), step(Read, pack(t, readArgs, big.NewInt(0)))}}
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
	header, _ := client.HeaderByNumber(ctx, nil)
	s := Snapshot{Executor: executor, Account: account, Caller: sender, BlockNumber: header.Number.Uint64()}
	resolved, err := ResolveValuesWithRuntime(ctx, reader, s, r, []Value{{Source: Result, Value: big.NewInt(1)}}, PrefixRuntime{Tree: tree})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _, err := Assemble(r, tree, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	original, err := client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executor, Data: encoded}, header.Number)
	if err != nil {
		t.Fatal(err)
	}
	if err = VerifyResults(encoded, original, ResultsWitness(resolved)); err != nil {
		t.Fatal(err)
	}
}

func TestAnvilAccessManagerUsesSignerOriginForPrefixAndAdmission(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	client := startAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	sender := owners[0]
	deploy := func(artifact anvilFixture, args ...any) common.Address {
		t.Helper()
		bytecode := append(bytes.Clone(artifact.Bytecode), fixtureCalldata(t, artifact, "", args...)...)
		return localTransaction(t, ctx, client, sender, nil, bytecode).ContractAddress
	}
	ma, aa, ea := loadAnvilFixture(t, "AccessManager"), loadAnvilFixture(t, "FixtureAccount"), loadAnvilFixture(t, "RecipeExecutor")
	manager, account := deploy(ma, sender), deploy(aa)
	executor := deploy(ea, sender, account)
	probeFixture := loadLensFixture(t, "LensProbe")
	probe := deploy(probeFixture, executor, sender)
	selector := [4]byte(executorABI.GetABI().Methods["execute"].ID)
	localTransaction(t, ctx, client, sender, &manager, fixtureCalldata(t, ma, "grantRole", uint64(3), sender, uint32(0)))
	localTransaction(t, ctx, client, sender, &manager, fixtureCalldata(t, ma, "setTargetFunctionRole", executor, [][4]byte{selector}, uint64(3)))
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetCaller(manager))
	root := common.Hash{1}
	localTransaction(t, ctx, client, sender, &account, fixtureCalldata(t, aa, "configure", root, [][32]byte{}, executor))

	recipe := Recipe{
		Queries: [][]byte{pack(t, queryArgs, probe, fixtureCalldata(t, probeFixture, "contextValue"), []*big.Int{}, big.NewInt(0))},
		Steps: [][]byte{
			step(Read, pack(t, readArgs, big.NewInt(0))),
			step(Call, pack(t, callArgs, probe, fixtureCalldata(t, probeFixture, "contextValue"), []*big.Int{})),
		},
	}
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(recipe.Queries, recipe.Steps))
	reader := &lensAnvilReader{Client: client}
	prepared, err := DiscoverBatch(ctx, reader, []common.Address{executor}, []common.Address{account}, manager, PreparationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := prepared[0].Snapshot
	snapshot.Sender = sender
	resolved, err := ResolveValues(ctx, reader, snapshot, recipe, []Value{{Source: Result, Value: new(big.Int)}})
	if err != nil || resolved[0].Int64() != 42 {
		t.Fatalf("forwarded prefix results %v, error %v", resolved, err)
	}
	payload, err := executorABI.TryPackExecute(recipe.Queries, recipe.Steps, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := Multicall([][]byte{payload, payload})
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{payload, batch} {
		calls := []ExecutorCall{{Executor: executor, Data: data}}
		if err = ValidatePreparedBatchForSender(ctx, reader, prepared, calls, [][]common.Hash{nil}, []map[uint]string{{0: "41"}}, manager, sender); err == nil {
			t.Fatal("forwarded execution accepted a different witness")
		}
		if err = ValidatePreparedBatchForSender(ctx, reader, prepared, calls, [][]common.Hash{nil}, []map[uint]string{ResultsWitness(resolved)}, manager, sender); err != nil {
			t.Fatal("forwarded execution rejected correct witness", err)
		}
	}
}
