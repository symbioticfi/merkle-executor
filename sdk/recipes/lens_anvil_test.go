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
)

type lensAnvilReader struct {
	*ethclient.Client
	calls int
}

func (r *lensAnvilReader) CallContractWithCode(ctx context.Context, msg ethereum.CallMsg, block *big.Int, code []byte) ([]byte, error) {
	r.calls++
	var raw hexutil.Bytes
	err := r.Client.Client().CallContext(ctx, &raw, "eth_call", map[string]any{"from": msg.From, "to": *msg.To, "input": hexutil.Bytes(msg.Data), "gas": hexutil.Uint64(msg.Gas)}, hexutil.EncodeBig(block), map[string]any{msg.To.Hex(): map[string]any{"code": hexutil.Bytes(code)}})
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
	bounded, err := ResolveValues(ctx, reader, snapshot, r, []Value{{Source: Result, Value: big.NewInt(2)}})
	if err != nil || bounded[2].Int64() != 84 {
		t.Fatalf("bounded preview=%v error=%v", bounded, err)
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
	r := Recipe{Queries: [][]byte{query}, Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0))), step(LessOrEqual, pack(t, binaryArgs, input(0), result(0))), step(LessOrEqual, pack(t, binaryArgs, input(0), literal(17))), step(Approve, pack(t, approveArgs, input(0), token, recipient, approval.Hash)), manageStep(t, transfer.Hash, literal(1), []*big.Int{binding(36, input(0))}), step(Call, pack(t, callArgs, token, fixtureCalldata(t, ta, "touch", new(big.Int)), []*big.Int{binding(4, input(0))})), step(Read, pack(t, readArgs, big.NewInt(0)))}}
	localTransaction(t, ctx, client, sender, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
	reader := &lensAnvilReader{Client: client}
	for i := int64(0); i < 2; i++ {
		header, _ := client.HeaderByNumber(ctx, nil)
		s := Snapshot{Executor: executor, Account: account, Caller: sender, BlockNumber: header.Number.Uint64()}
		resolved, err := ResolveValuesWithRuntime(ctx, reader, s, r, []Value{{Source: Result, Value: big.NewInt(6)}}, PrefixRuntime{Tree: tree})
		if err != nil {
			t.Fatal(err)
		}
		if resolved[6].Int64() != 983-i*17 {
			t.Fatalf("post-effect balance=%s", resolved[6])
		}
		encoded, _, err := Assemble(r, tree, map[uint]string{0: "17"}, nil, nil)
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
		if got := fixtureRead(t, ctx, client, ta, token, "balanceOf", account)[0].(*big.Int); got.Int64() != 1000-i*17 {
			t.Fatal("lens wrote live state")
		}
		localTransaction(t, ctx, client, sender, &executor, encoded)
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
	r = Recipe{Queries: [][]byte{query}, Steps: [][]byte{step(Approve, pack(t, approveArgs, literal(0), zeroToken, recipient, zeroLeaf.Hash)), step(Read, pack(t, readArgs, big.NewInt(0)))}}
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
