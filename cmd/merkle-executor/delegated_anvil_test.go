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
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAnvilServiceDelegatedBatch(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	var wallets []common.Address
	if err := client.Client().CallContext(ctx, &wallets, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	owner, recipient := wallets[0], wallets[1]
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	caller := crypto.PubkeyToAddress(key.PublicKey)
	if err := client.Client().CallContext(ctx, nil, "anvil_setBalance", caller, "0xde0b6b3a7640000"); err != nil {
		t.Fatal(err)
	}
	var runtime struct {
		Address     common.Address
		Runtime     hexutil.Bytes
		RuntimeHash common.Hash
	}
	raw, err := os.ReadFile("../../sdk/recipes/testdata/anvil/MetaMaskDelegator.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &runtime); err != nil {
		t.Fatal(err)
	}
	if crypto.Keccak256Hash(runtime.Runtime) != runtime.RuntimeHash {
		t.Fatal("invalid delegation fixture")
	}
	for address, code := range map[common.Address][]byte{runtime.Address: runtime.Runtime, caller: types.AddressToDelegation(runtime.Address)} {
		if err := client.Client().CallContext(ctx, nil, "anvil_setCode", address, hexutil.Bytes(code)); err != nil {
			t.Fatal(err)
		}
	}
	aa, ta, ea := loadServiceArtifact(t, "FixtureAccount"), loadServiceArtifact(t, "FixtureToken"), loadServiceArtifact(t, "RecipeExecutor")
	accounts := []common.Address{
		serviceSeedTransaction(t, ctx, client, owner, nil, aa.Bytecode).ContractAddress,
		serviceSeedTransaction(t, ctx, client, owner, nil, aa.Bytecode).ContractAddress,
	}
	token := serviceSeedTransaction(t, ctx, client, owner, nil, ta.Bytecode).ContractAddress
	cfg := testCLIConfig(t.TempDir())
	cfg.Mode, cfg.Chain.RPCURL, cfg.Strategies = "send", endpoint, nil
	cfg.Signer.Address = caller.Hex()
	permissions := map[string][]byte{}
	for i, account := range accounts {
		address := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(ea.Bytecode), servicePack(t, ea, "", owner, account)...)).ContractAddress
		cfg.Strategies = append(cfg.Strategies, strings.ToLower(address.Hex()))
		destination := recipient
		if i == 0 {
			destination = accounts[1]
		}
		data := erc20.NewERC20().PackTransfer(destination, new(big.Int))
		mask := make([]byte, len(data))
		for j := 36; j < len(mask); j++ {
			mask[j] = 0xff
		}
		leaf := recipes.Leaf{Target: token, Value: new(big.Int), Data: data, DataMask: mask}
		leaf.Hash, err = recipes.HashLeaf(leaf)
		if err != nil {
			t.Fatal(err)
		}
		serviceSeedTransaction(t, ctx, client, owner, &account, servicePack(t, aa, "configure", leaf.Hash, [][32]byte{leaf.Hash}, address))
		result := new(big.Int).Lsh(big.NewInt(2), 216)
		query := issuerStep(t, 0, []string{"address", "bytes", "uint256[]", "uint256"}, token, erc20.NewERC20().PackBalanceOf(account), []*big.Int{}, new(big.Int))[1:]
		r := recipes.Recipe{Queries: [][]byte{query}, Steps: [][]byte{
			issuerStep(t, recipes.Read, []string{"uint256"}, new(big.Int)),
			issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, result, [][32]byte{leaf.Hash}, [][]*big.Int{{new(big.Int).Or(new(big.Int).Lsh(big.NewInt(36), 224), result)}}),
		}}
		serviceSeedTransaction(t, ctx, client, owner, &address, executor.NewRecipeExecutor().PackSetCaller(caller))
		serviceSeedTransaction(t, ctx, client, owner, &address, executor.NewRecipeExecutor().PackSetRecipe(r.Queries, r.Steps))
		permission, err := json.Marshal(map[string]any{"root": leaf.Hash.Hex(), "chainId": 31337, "adapter": recipient.Hex(), "vault": owner.Hex(), "actions": []any{}, "leaves": []any{map[string]any{"hash": leaf.Hash.Hex(), "target": token.Hex(), "value": "zero", "data": hexutil.Encode(data), "dataMask": hexutil.Encode(mask), "proof": []string{}}}})
		if err != nil {
			t.Fatal(err)
		}
		permissions["/api/v1/merkle/"+strings.ToLower(account.Hex())+"/"+leaf.Hash.Hex()] = servicePermissionCertificate(t, permission, account, account)
	}
	serviceSeedTransaction(t, ctx, client, owner, &token, servicePack(t, ta, "mint", accounts[0], big.NewInt(25)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if data, ok := permissions[r.URL.Path]; ok {
			_, _ = w.Write(data)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	cfg.Curator.URL = server.URL + "/api/v1"
	path := writeCLIConfig(t, t.TempDir(), cfg)
	var output bytes.Buffer
	if err := planBot(ctx, path, &output); err != nil {
		t.Fatal(err)
	}
	var plan struct {
		To   common.Address
		Data hexutil.Bytes
		Plan keeper.ExecutionPlan
	}
	if err := json.Unmarshal(output.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.To != caller || len(plan.Plan.Members) != 2 {
		t.Fatal("plan omitted idle executor activated by predecessor")
	}
	if nonce, err := client.NonceAt(ctx, caller, nil); err != nil || nonce != 0 {
		t.Fatal("plan submitted a transaction")
	}
	t.Setenv(cfg.Signer.KeyEnv, hex.EncodeToString(crypto.FromECDSA(key)))
	startServiceConfirmationMining(t, ctx, client, caller)
	if err := runBot(ctx, path, true); err != nil {
		t.Fatal(err)
	}
	transaction := serviceTransaction(t, ctx, client, caller, 0)
	if transaction.To() == nil || *transaction.To() != caller || !bytes.Equal(transaction.Data(), plan.Data) {
		t.Fatal("not the planned self-transaction")
	}
	if nonce, err := client.NonceAt(ctx, caller, nil); err != nil || nonce != 1 {
		t.Fatalf("expected one transaction: nonce %d, %v", nonce, err)
	}
	amount, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: erc20.NewERC20().PackBalanceOf(recipient)}, nil)
	if err != nil || new(big.Int).SetBytes(amount).Cmp(big.NewInt(25)) != 0 {
		t.Fatalf("later executor did not consume earlier proceeds: %v", err)
	}

}
