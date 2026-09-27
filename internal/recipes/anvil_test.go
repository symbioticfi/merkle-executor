package recipes

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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-errors/errors"
)

type anvilFixture struct {
	ABI      abi.ABI
	Bytecode []byte
}

func loadAnvilFixture(t *testing.T, name string) anvilFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/anvil/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode hexutil.Bytes   `json:"bytecode"`
	}
	if err = json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	contractABI, err := abi.JSON(bytes.NewReader(raw.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return anvilFixture{ABI: contractABI, Bytecode: raw.Bytecode}
}

func startAnvil(t *testing.T, ctx context.Context) *ethclient.Client {
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
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	client, err := ethclient.DialContext(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ready := time.NewTimer(10 * time.Second)
	defer ready.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if id, err := client.ChainID(ctx); err == nil {
			if id.Cmp(big.NewInt(31337)) != 0 {
				t.Fatal("unexpected local chain ID")
			}
			return client
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ready.C:
			t.Fatal("local anvil did not start")
		case <-ticker.C:
		}
	}
}

func localTransaction(t *testing.T, ctx context.Context, client *ethclient.Client, from common.Address, to *common.Address, data []byte) *types.Receipt {
	t.Helper()
	// The endpoint is always the ephemeral loopback Anvil started by this test.
	args := map[string]any{"from": from, "data": hexutil.Bytes(data), "gas": hexutil.Uint64(12_000_000)}
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
				t.Fatalf("local transaction %s reverted", hash)
			}
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

func fixtureCalldata(t *testing.T, contract anvilFixture, method string, args ...any) []byte {
	t.Helper()
	data, err := contract.ABI.Pack(method, args...)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fixtureRead(t *testing.T, ctx context.Context, client *ethclient.Client, contract anvilFixture, target common.Address, method string, args ...any) []any {
	t.Helper()
	data, err := client.CallContract(ctx, ethereum.CallMsg{To: &target, Data: fixtureCalldata(t, contract, method, args...)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	values, err := contract.ABI.Unpack(method, data)
	if err != nil {
		t.Fatal(err)
	}
	return values
}

func TestAnvilFullExecute(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("set MERKLE_ANVIL_TEST=1 for the local EVM integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := startAnvil(t, ctx)
	var accounts []common.Address
	if err := client.Client().CallContext(ctx, &accounts, "eth_accounts"); err != nil || len(accounts) < 4 {
		t.Fatalf("local test accounts: %v", err)
	}
	sender, recipient := accounts[0], accounts[1]
	accountContract := loadAnvilFixture(t, "FixtureAccount")
	tokenContract := loadAnvilFixture(t, "FixtureToken")
	executorContract := loadAnvilFixture(t, "RecipesExecutor")
	accountAddress := localTransaction(t, ctx, client, sender, nil, accountContract.Bytecode).ContractAddress
	tokenAddress := localTransaction(t, ctx, client, sender, nil, tokenContract.Bytecode).ContractAddress
	constructor := fixtureCalldata(t, executorContract, "", sender, accountAddress)
	executorAddress := localTransaction(t, ctx, client, sender, nil, append(bytes.Clone(executorContract.Bytecode), constructor...)).ContractAddress

	makeLeaf := func(method string) Leaf {
		data := fixtureCalldata(t, tokenContract, method, recipient, new(big.Int))
		mask := make([]byte, len(data))
		for i := 36; i < len(mask); i++ {
			mask[i] = 0xff
		}
		leaf := Leaf{Target: tokenAddress, Value: new(big.Int), Data: data, DataMask: mask}
		var err error
		leaf.Hash, err = HashLeaf(leaf)
		if err != nil {
			t.Fatal(err)
		}
		return leaf
	}
	transfer, approval := makeLeaf("transfer"), makeLeaf("approve")
	transfer.Proof = []common.Hash{approval.Hash}
	approval.Proof = []common.Hash{transfer.Hash}
	tree := &Tree{Root: hashPair(transfer.Hash, approval.Hash), Leaves: []Leaf{transfer, approval}}
	localTransaction(t, ctx, client, sender, &accountAddress, fixtureCalldata(t, accountContract, "configure", tree.Root, [][32]byte{transfer.Hash, approval.Hash}, executorAddress))
	localTransaction(t, ctx, client, sender, &tokenAddress, fixtureCalldata(t, tokenContract, "mint", accountAddress, big.NewInt(1000)))
	localTransaction(t, ctx, client, sender, &executorAddress, executorABI.PackSetCaller(sender))

	query := pack(t, queryArgs, tokenAddress, fixtureCalldata(t, tokenContract, "balanceOf", accountAddress), []Binding{}, new(big.Int))
	recipe := Recipe{Queries: [][]byte{query}, Steps: []Step{
		{Operation: Read, Data: pack(t, readArgs, big.NewInt(0))},
		{Operation: Min, Data: pack(t, binaryArgs, result(0), input(0))},
		{Operation: Sub, Data: pack(t, binaryArgs, result(0), result(1))},
		{Operation: Approve, Data: pack(t, approveArgs, result(1), tokenAddress, recipient, approval.Hash)},
		manageStep(t, transfer.Hash, result(1), []Binding{{Offset: big.NewInt(36), Value: result(1)}}),
		manageStep(t, transfer.Hash, input(1), []Binding{{Offset: big.NewInt(36), Value: input(2)}}),
		{Operation: Call, Data: pack(t, callArgs, tokenAddress, fixtureCalldata(t, tokenContract, "touch", new(big.Int)), []Binding{{Offset: big.NewInt(4), Value: result(1)}})},
		{Operation: Read, Data: pack(t, readArgs, big.NewInt(0))},
		{Operation: Equal, Data: pack(t, binaryArgs, result(7), result(2))},
	}}
	localTransaction(t, ctx, client, sender, &executorAddress, executorABI.PackSetRecipe(recipe.Queries, recipe.Steps))
	transferred := int64(0)
	for _, amount := range []int64{17, 23} {
		snapshot, recovered, err := Discover(ctx, client, executorAddress, nil)
		if err != nil {
			t.Fatal(err)
		}
		expectedHash, err := HashRecipe(recipe)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Root != tree.Root || snapshot.Account != accountAddress || snapshot.RecipeHash != expectedHash {
			t.Fatal("on-chain commitment mismatch")
		}
		encoded, leaves, err := Assemble(recovered, tree, map[uint]string{0: strconv.FormatInt(amount, 10), 1: "0"}, nil, []uint{5})
		if err != nil {
			t.Fatal(err)
		}
		if err = VerifyRoles(ctx, client, snapshot, leaves); err != nil {
			t.Fatal(err)
		}
		simulated, err := client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executorAddress, Data: encoded}, new(big.Int).SetUint64(snapshot.BlockNumber))
		if err != nil {
			t.Fatalf("full execute simulation: %v", err)
		}
		decoded, err := executorABI.UnpackExecute(simulated)
		if err != nil {
			t.Fatal(err)
		}
		if decoded[1].Int64() != amount || decoded[7].Int64() != 1000-transferred-amount {
			t.Fatal("on-chain arithmetic/binding result mismatch")
		}
		if err = ValidateSnapshot(ctx, client, snapshot, sender); err != nil {
			t.Fatal(err)
		}
		localTransaction(t, ctx, client, sender, &executorAddress, encoded)
		transferred += amount
		if balance := fixtureRead(t, ctx, client, tokenContract, tokenAddress, "balanceOf", accountAddress)[0].(*big.Int); balance.Int64() != 1000-transferred {
			t.Fatalf("account balance %s", balance)
		}
		if balance := fixtureRead(t, ctx, client, tokenContract, tokenAddress, "balanceOf", recipient)[0].(*big.Int); balance.Int64() != transferred {
			t.Fatalf("recipient balance %s", balance)
		}
		if allowance := fixtureRead(t, ctx, client, tokenContract, tokenAddress, "allowance", accountAddress, recipient)[0].(*big.Int); allowance.Int64() != amount {
			t.Fatalf("allowance %s", allowance)
		}
		if marker := fixtureRead(t, ctx, client, tokenContract, tokenAddress, "marker")[0].(*big.Int); marker.Int64() != amount {
			t.Fatal("Call operation binding not executed")
		}
		if caller := fixtureRead(t, ctx, client, tokenContract, tokenAddress, "lastCaller")[0].(common.Address); caller != executorAddress {
			t.Fatal("Call did not originate from executor")
		}
		invalid, _, err := Assemble(recovered, tree, map[uint]string{0: "1", 1: "1"}, nil, []uint{5})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executorAddress, Data: invalid}, nil); err == nil {
			t.Fatal("nonzero gate accepted empty runtime")
		}
	}
	// Pinned and partially masked amount permissions are valid when the allowance
	// is zero. Their state-dependent reset path is checked by full EVM simulation.
	for index, maskByte := range []byte{0, 0x0f} {
		spender := accounts[index+2]
		data := fixtureCalldata(t, tokenContract, "approve", spender, big.NewInt(17))
		mask := make([]byte, len(data))
		mask[len(mask)-1] = maskByte
		leaf := Leaf{Target: tokenAddress, Value: new(big.Int), Data: data, DataMask: mask}
		var err error
		leaf.Hash, err = HashLeaf(leaf)
		if err != nil {
			t.Fatal(err)
		}
		tree := &Tree{Root: leaf.Hash, Leaves: []Leaf{leaf}}
		localTransaction(t, ctx, client, sender, &accountAddress, fixtureCalldata(t, accountContract, "configure", tree.Root, [][32]byte{leaf.Hash}, executorAddress))
		recipe := Recipe{Steps: []Step{{Operation: Approve, Data: pack(t, approveArgs, input(0), tokenAddress, spender, leaf.Hash)}}}
		localTransaction(t, ctx, client, sender, &executorAddress, executorABI.PackSetRecipe(recipe.Queries, recipe.Steps))
		snapshot, recovered, err := Discover(ctx, client, executorAddress, nil)
		if err != nil {
			t.Fatal(err)
		}
		encoded, leaves, err := Assemble(recovered, tree, map[uint]string{0: "17"}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err = VerifyRoles(ctx, client, snapshot, leaves); err != nil {
			t.Fatal(err)
		}
		if _, err = client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executorAddress, Data: encoded}, nil); err != nil {
			t.Fatalf("amount mask %x: %v", maskByte, err)
		}
		invalid, _, err := Assemble(recovered, tree, map[uint]string{0: "32"}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executorAddress, Data: invalid}, nil); err == nil {
			t.Fatal("amount outside mask accepted")
		}
		localTransaction(t, ctx, client, sender, &executorAddress, encoded)
		if _, err = client.CallContract(ctx, ethereum.CallMsg{From: sender, To: &executorAddress, Data: encoded}, nil); err == nil {
			t.Fatal("pinned permission incorrectly accepted allowance reset to zero")
		}
	}
}
