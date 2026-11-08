package cowswap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	accountbinding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executorbinding "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type cowAnvilReader struct {
	*ethclient.Client
	settlement common.Address
}

func (r *cowAnvilReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if *msg.To == r.settlement {
		return (&boundedCowReader{settlement: r.settlement}).CallContract(ctx, msg, block)
	}
	return r.Client.CallContract(ctx, msg, block)
}
func (r *cowAnvilReader) CallContractWithCode(ctx context.Context, msg ethereum.CallMsg, block *big.Int, code []byte) ([]byte, error) {
	var raw hexutil.Bytes
	err := r.Client.Client().CallContext(ctx, &raw, "eth_call", map[string]any{"from": msg.From, "to": *msg.To, "input": hexutil.Bytes(msg.Data), "gas": hexutil.Uint64(msg.Gas)}, hexutil.EncodeBig(block), map[string]any{msg.To.Hex(): map[string]any{"code": hexutil.Bytes(code)}})
	return raw, err
}
func TestAnvilAutomaticBoundedCowRecipe(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	command := exec.CommandContext(ctx, "anvil", "--host", "127.0.0.1", "--port", strconv.Itoa(port), "--chain-id", "1", "--hardfork", "osaka", "--silent")
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
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
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for until := time.Now().Add(10 * time.Second); ; {
		if _, err = client.ChainID(ctx); err == nil {
			break
		}
		if time.Now().After(until) {
			t.Fatal(err)
		}
		time.Sleep(25 * time.Millisecond)
	}
	var owners []common.Address
	if err = client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	owner := owners[0]
	type artifact struct {
		ABI      json.RawMessage
		Bytecode hexutil.Bytes
	}
	load := func(path string) (artifact, abi.ABI) {
		t.Helper()
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var a artifact
		if e = json.Unmarshal(raw, &a); e != nil {
			t.Fatal(e)
		}
		ab, e := abi.JSON(bytes.NewReader(a.ABI))
		if e != nil {
			t.Fatal(e)
		}
		return a, ab
	}
	send := func(to *common.Address, data []byte) common.Address {
		t.Helper()
		args := map[string]any{"from": owner, "data": hexutil.Bytes(data), "gas": hexutil.Uint64(12_000_000)}
		if to != nil {
			args["to"] = *to
		}
		var hash common.Hash
		if e := client.Client().CallContext(ctx, &hash, "eth_sendTransaction", args); e != nil {
			t.Fatal(e)
		}
		for {
			receipt, e := client.TransactionReceipt(ctx, hash)
			if e == nil {
				if receipt.Status != 1 {
					t.Fatalf("local transaction reverted: %s", hash)
				}
				return receipt.ContractAddress
			}
			if ctx.Err() != nil {
				t.Fatal(ctx.Err())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	pack := func(a abi.ABI, method string, args ...any) []byte {
		t.Helper()
		data, e := a.Pack(method, args...)
		if e != nil {
			t.Fatal(e)
		}
		return data
	}
	f, recipe, tree := boundedCow(t)
	install := func(path string, address common.Address) abi.ABI {
		t.Helper()
		a, ab := load(path)
		deployed := send(nil, a.Bytecode)
		code, e := client.CodeAt(ctx, deployed, nil)
		if e != nil {
			t.Fatal(e)
		}
		if e = client.Client().CallContext(ctx, nil, "anvil_setCode", address, hexutil.Bytes(code)); e != nil {
			t.Fatal(e)
		}
		return ab
	}
	accountABI := install("../liquidlanerfq/testdata/FixtureRFQAccount.json", f.Account)
	tokenABI := install("../../recipes/testdata/anvil/FixtureToken.json", f.Token)
	// Local price/clock double: price() returns 2; timestamp() returns TIMESTAMP.
	readsCode := hexutil.MustDecode("0x5f3560e01c63a035b1fe14601557425f5260205ff35b60025f5260205ff3")
	if err = client.Client().CallContext(ctx, nil, "anvil_setCode", f.Reads, hexutil.Bytes(readsCode)); err != nil {
		t.Fatal(err)
	}
	executorArtifact, executorABI := load("../../recipes/testdata/anvil/RecipeExecutor.json")
	executor := send(nil, append(bytes.Clone(executorArtifact.Bytecode), pack(executorABI, "", owner, f.Account)...))
	e := executorbinding.NewRecipeExecutor()
	send(&executor, e.PackSetCaller(owner))
	send(&executor, e.PackSetRecipe(recipe.Queries, recipe.Steps))
	hashes := make([][32]byte, len(tree.Leaves))
	for i, leaf := range tree.Leaves {
		hashes[i] = leaf.Hash
	}
	send(&f.Account, pack(accountABI, "configure", tree.Root, hashes, executor))
	send(&f.Token, pack(tokenABI, "mint", f.Account, big.NewInt(1000)))
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: header.Number, BlockHash: header.Hash(), Timestamp: header.Time, Account: f.Account, Executor: executor, Caller: owner, Root: tree.Root}
	chain := &cowAnvilReader{Client: client, settlement: f.Settlement}
	posts := 0
	var saved prepared
	p := &Automatic{deps: provider.Deps{Chain: chain, HTTP: boundedHTTP(t, &posts, &saved, new(common.Hash))}}
	req, err := p.RecipeRequirements(s, recipe, tree)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := recipes.ResolveValues(ctx, chain, recipes.Snapshot{Executor: executor, Caller: owner, Account: f.Account, BlockNumber: header.Number.Uint64()}, recipe, req.Values)
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := p.PrepareRecipe(ctx, s, recipe, tree, resolved)
	if err != nil {
		t.Fatal(err)
	}
	out := outputs[0]
	if err = json.Unmarshal(out.Data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Order.ValidTo != int64(header.Time+300) || saved.Order.AppData != (common.Hash{}).Hex() {
		t.Fatal("committed expiry/appData changed", saved.Order)
	}
	calldata, _, err := recipes.Assemble(recipe, tree, out.Inputs, out.Actions, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.CallContract(ctx, ethereum.CallMsg{From: owner, To: &executor, Data: calldata}, header.Number)
	if err != nil {
		t.Fatal("original bounded recipe rejected automatic provider", err)
	}
	if err = recipes.VerifyResults(calldata, result, recipes.ResultsWitness(resolved)); err != nil {
		t.Fatal(err)
	}
	badInputs := map[uint]string{}
	for index, value := range out.Inputs {
		badInputs[index] = value
	}
	for _, tc := range []struct {
		index uint
		value string
	}{{0, "999"}, {2, strconv.FormatUint(header.Time+301, 10)}} {
		badInputs[tc.index] = tc.value
		bad, _, e := recipes.Assemble(recipe, tree, badInputs, out.Actions, nil)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = client.CallContract(ctx, ethereum.CallMsg{From: owner, To: &executor, Data: bad}, header.Number); e == nil {
			t.Fatal("original recipe accepted changed amount/expiry", tc)
		}
		badInputs[tc.index] = out.Inputs[tc.index]
	}
	if posts != 0 {
		t.Fatal("provider published before authorization")
	}
	send(&executor, calldata)
	approved, err := client.CallContract(ctx, ethereum.CallMsg{To: &f.Account, Data: accountbinding.NewMerkleAccount().PackSignatures(saved.Digest)}, nil)
	if err != nil || new(big.Int).SetBytes(approved).Cmp(big.NewInt(1)) != 0 {
		t.Fatal("prepared digest was not authorized", err)
	}
}
