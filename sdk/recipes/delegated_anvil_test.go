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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
)

func TestAnvilDelegatedExecutors(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	client := startAnvil(t, ctx)
	var wallets []common.Address
	if err := client.Client().CallContext(ctx, &wallets, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	caller, recipient := wallets[0], wallets[1]
	reader := &lensAnvilReader{Client: client}
	if supported, err := SupportsDelegatedBatch(ctx, reader, caller, nil); err != nil || supported {
		t.Fatalf("undelegated EOA: supported=%v err=%v", supported, err)
	}
	var runtime struct {
		Address     common.Address
		Runtime     hexutil.Bytes
		RuntimeHash common.Hash
	}
	raw, err := os.ReadFile("testdata/anvil/MetaMaskDelegator.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &runtime); err != nil {
		t.Fatal(err)
	}
	if crypto.Keccak256Hash(runtime.Runtime) != runtime.RuntimeHash {
		t.Fatal("delegation fixture runtime hash mismatch")
	}
	for address, code := range map[common.Address][]byte{runtime.Address: runtime.Runtime, caller: types.AddressToDelegation(runtime.Address)} {
		if err := client.Client().CallContext(ctx, nil, "anvil_setCode", address, hexutil.Bytes(code)); err != nil {
			t.Fatal(err)
		}
	}
	if supported, err := SupportsDelegatedBatch(ctx, reader, caller, nil); err != nil || !supported {
		t.Fatalf("MetaMask delegation: supported=%v err=%v", supported, err)
	}
	aa, ta, ea := loadAnvilFixture(t, "FixtureAccount"), loadAnvilFixture(t, "FixtureToken"), loadAnvilFixture(t, "RecipeExecutor")
	accountA := localTransaction(t, ctx, client, caller, nil, aa.Bytecode).ContractAddress
	accountB := localTransaction(t, ctx, client, caller, nil, aa.Bytecode).ContractAddress
	token := localTransaction(t, ctx, client, caller, nil, ta.Bytecode).ContractAddress
	configure := func(account, destination common.Address) ExecutorCall {
		executor := localTransaction(t, ctx, client, caller, nil, append(bytes.Clone(ea.Bytecode), fixtureCalldata(t, ea, "", caller, account)...)).ContractAddress
		data := fixtureCalldata(t, ta, "transfer", destination, new(big.Int))
		mask := make([]byte, len(data))
		for i := 36; i < len(mask); i++ {
			mask[i] = 0xff
		}
		leaf := Leaf{Target: token, Value: new(big.Int), Data: data, DataMask: mask}
		leaf.Hash, err = HashLeaf(leaf)
		if err != nil {
			t.Fatal(err)
		}
		localTransaction(t, ctx, client, caller, &account, fixtureCalldata(t, aa, "configure", leaf.Hash, [][32]byte{leaf.Hash}, executor))
		query := pack(t, queryArgs, token, fixtureCalldata(t, ta, "balanceOf", account), []*big.Int{}, new(big.Int))
		r := Recipe{Queries: [][]byte{query}, Steps: [][]byte{step(Read, pack(t, readArgs, new(big.Int))), manageStep(t, leaf.Hash, result(0), []*big.Int{binding(36, result(0))})}}
		localTransaction(t, ctx, client, caller, &executor, executorABI.PackSetRecipe(r.Queries, r.Steps))
		localTransaction(t, ctx, client, caller, &executor, executorABI.PackSetCaller(caller))
		call, _, err := Assemble(r, &Tree{Root: leaf.Hash, Leaves: []Leaf{leaf}}, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		return ExecutorCall{Executor: executor, Data: call}
	}
	a, b := configure(accountA, accountB), configure(accountB, recipient)
	localTransaction(t, ctx, client, caller, &token, fixtureCalldata(t, ta, "mint", accountA, big.NewInt(25)))
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, calls := range [][]ExecutorCall{{a, b}, {b, a}} {
		results, err := SimulateExecutors(ctx, reader, caller, calls, header.Number, header.GasLimit)
		if err != nil {
			t.Fatal(err)
		}
		work, err := HasWork(calls[0].Data, results[0])
		if err != nil || work != (calls[0].Executor == a.Executor) {
			t.Fatalf("order did not affect available funds: work=%v err=%v", work, err)
		}
		if calls[0].Executor == a.Executor {
			if work, err = HasWork(b.Data, results[1]); err != nil || !work {
				t.Fatalf("second executor did not use preceding funds: %v %v", work, err)
			}
		}
	}
	readBalance := func(account common.Address) int64 {
		return fixtureRead(t, ctx, client, ta, token, "balanceOf", account)[0].(*big.Int).Int64()
	}
	if readBalance(accountA) != 25 || readBalance(accountB) != 0 {
		t.Fatal("simulation changed chain state")
	}
	data, err := DelegatedMulticall([]ExecutorCall{a, b})
	if err != nil {
		t.Fatal(err)
	}
	localTransaction(t, ctx, client, caller, &caller, data)
	if readBalance(accountA) != 0 || readBalance(accountB) != 0 || readBalance(recipient) != 25 {
		t.Fatal("ordered batch did not transfer through both executors")
	}
	localTransaction(t, ctx, client, caller, &token, fixtureCalldata(t, ta, "mint", accountA, big.NewInt(25)))
	// Invalidating the second committed recipe must revert the first transfer too.
	localTransaction(t, ctx, client, caller, &b.Executor, executorABI.PackSetRecipe(nil, nil))
	header, err = client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = SimulateExecutors(ctx, reader, caller, []ExecutorCall{a, b}, header.Number, header.GasLimit); !BatchRejected(err) {
		t.Fatalf("invalid second recipe accepted: %v", err)
	}
	var hash common.Hash
	if err := client.Client().CallContext(ctx, &hash, "eth_sendTransaction", map[string]any{"from": caller, "to": caller, "data": hexutil.Bytes(data), "gas": hexutil.Uint64(12_000_000)}); err != nil {
		t.Fatal(err)
	}
	var receipt *types.Receipt
	for {
		receipt, err = client.TransactionReceipt(ctx, hash)
		if !errors.Is(err, ethereum.NotFound) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err != nil || receipt.Status != types.ReceiptStatusFailed {
		t.Fatalf("batch should revert atomically: %v %v", receipt, err)
	}
	if readBalance(accountA) != 25 || readBalance(accountB) != 0 || readBalance(recipient) != 25 {
		t.Fatal("reverted batch left partial effects")
	}
	if _, err := client.CallContract(ctx, ethereum.CallMsg{From: recipient, To: &caller, Data: data}, nil); err == nil {
		t.Fatal("unrelated sender could use the keeper delegation")
	}
}
