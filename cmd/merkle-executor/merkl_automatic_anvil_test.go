package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	distributor "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAnvilAutomaticMerklClaimFeedsRecipeBalance(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	owner, recipient := owners[0], owners[1]
	aa, ta, ea := loadServiceArtifact(t, "FixtureAccount"), loadServiceArtifact(t, "FixtureToken"), loadServiceArtifact(t, "RecipeExecutor")
	account := serviceSeedTransaction(t, ctx, client, owner, nil, aa.Bytecode).ContractAddress
	token := serviceSeedTransaction(t, ctx, client, owner, nil, ta.Bytecode).ContractAddress
	target := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(ea.Bytecode), servicePack(t, ea, "", owner, account)...)).ContractAddress
	artifactRaw, err := os.ReadFile(filepath.Join("..", "..", "sdk", "recipes", "lens", "FixtureAutomaticRewards.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		ABI      json.RawMessage
		Bytecode hexutil.Bytes
	}
	if err = json.Unmarshal(artifactRaw, &raw); err != nil {
		t.Fatal(err)
	}
	rewardABI, err := abi.JSON(bytes.NewReader(raw.ABI))
	if err != nil {
		t.Fatal(err)
	}
	ra := serviceArtifact{ABI: rewardABI, Bytecode: raw.Bytecode}
	rewardTarget := serviceSeedTransaction(t, ctx, client, owner, nil, ra.Bytecode).ContractAddress
	rewardRoot := crypto.Keccak256Hash(common.LeftPadBytes(account.Bytes(), 32), common.LeftPadBytes(token.Bytes(), 32), common.LeftPadBytes(big.NewInt(100).Bytes(), 32))
	serviceSeedTransaction(t, ctx, client, owner, &rewardTarget, servicePack(t, ra, "setRoot", rewardRoot))
	claim := distributor.NewMerklDistributor().PackClaim([]common.Address{account}, []common.Address{token}, []*big.Int{new(big.Int)}, [][][32]byte{{}})
	amountOffset := 4 + int(new(big.Int).SetBytes(claim[68:100]).Uint64()) + 32
	claimMask := make([]byte, len(claim))
	for i := amountOffset; i < amountOffset+32; i++ {
		claimMask[i] = 0xff
	}
	tokenABI := erc20.NewERC20()
	transfer := tokenABI.PackTransfer(recipient, new(big.Int))
	transferMask := make([]byte, len(transfer))
	for i := 36; i < len(transferMask); i++ {
		transferMask[i] = 0xff
	}
	leaves := []recipes.Leaf{{Target: rewardTarget, Value: new(big.Int), Data: claim, DataMask: claimMask}, {Target: token, Value: new(big.Int), Data: transfer, DataMask: transferMask}}
	for i := range leaves {
		leaves[i].Hash, err = recipes.HashLeaf(leaves[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	a, b := leaves[0].Hash, leaves[1].Hash
	if bytes.Compare(a[:], b[:]) > 0 {
		a, b = b, a
	}
	root := crypto.Keccak256Hash(a[:], b[:])
	serviceSeedTransaction(t, ctx, client, owner, &account, servicePack(t, aa, "configure", root, [][32]byte{leaves[0].Hash, leaves[1].Hash}, target))
	eABI := executor.NewRecipeExecutor()
	serviceSeedTransaction(t, ctx, client, owner, &target, eABI.PackSetCaller(owner))
	value := func(source, index int64) *big.Int {
		return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(source), 216), big.NewInt(index))
	}
	binding := func(offset int, v *big.Int) *big.Int {
		return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(int64(offset)), 224), v)
	}
	query := issuerStep(t, 0, []string{"address", "bytes", "uint256[]", "uint256"}, token, tokenABI.PackBalanceOf(account), []*big.Int{}, new(big.Int))[1:]
	recipe := recipes.Recipe{Queries: [][]byte{query}, Steps: [][]byte{
		issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaves[0].Hash}, [][]*big.Int{{binding(amountOffset, value(1, 0))}}),
		issuerStep(t, recipes.Read, []string{"uint256"}, new(big.Int)),
		issuerStep(t, recipes.Min, []string{"uint256", "uint256"}, value(2, 1), big.NewInt(17)),
		issuerStep(t, recipes.Equal, []string{"uint256", "uint256"}, value(1, 1), value(2, 2)),
		issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaves[1].Hash}, [][]*big.Int{{binding(36, value(1, 1))}}),
	}}
	serviceSeedTransaction(t, ctx, client, owner, &target, eABI.PackSetRecipe(recipe.Queries, recipe.Steps))
	entries := make([]any, len(leaves))
	for i, l := range leaves {
		entries[i] = map[string]any{"hash": l.Hash.Hex(), "target": l.Target.Hex(), "value": "zero", "data": hexutil.Encode(l.Data), "dataMask": hexutil.Encode(l.DataMask), "proof": []string{leaves[1-i].Hash.Hex()}}
	}
	permission, _ := json.Marshal(map[string]any{"root": root.Hex(), "chainId": 31337, "account": account.Hex(), "leaves": entries})
	permission = servicePermissionCertificate(t, permission, account, account)
	curator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(permission) }))
	defer curator.Close()
	var rewardsHTTP atomic.Int32
	rewardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rewardsHTTP.Add(1)
		_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 31337}, Rewards: []wire.MerklReward{{Token: wire.MerklToken{Address: token.Hex(), Decimals: 18}, Amount: "100", Claimed: "0", Pending: "0", Proofs: []string{}}}}})
	}))
	defer rewardServer.Close()
	rewrite, _ := url.Parse(rewardServer.URL)
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
	cfg.Mode = "simulate"
	cfg.Chain.RPCURL = rpc.URL
	cfg.Curator.URL = curator.URL
	cfg.Strategies[0] = strings.ToLower(target.Hex())
	cfg.Signer.Address = ""
	path := writeCLIConfig(t, t.TempDir(), cfg)
	httpClient := &http.Client{Transport: serviceRewriteTransport{host: "api.merkl.xyz", target: rewrite}}
	var output bytes.Buffer
	if err = runBotWithHTTP(ctx, path, true, true, &output, httpClient); err != nil {
		t.Fatal(err)
	}
	var plans map[string]keeper.Plan
	if err = json.Unmarshal(output.Bytes(), &plans); err != nil {
		t.Fatal(err)
	}
	plan := plans[cfg.Strategies[0]]
	if len(plan.Outputs) != 5 || plan.Results[1] != "100" || plan.Results[2] != "17" || plan.Outputs[1].Inputs[0] != "100" || len(plan.Outputs[1].Actions) != 1 || lenses.Load() != 1 || rewardsHTTP.Load() != 1 {
		t.Fatalf("incorrect staged preparation: results=%v lens=%d rewardHTTP=%d", plan.Results, lenses.Load(), rewardsHTTP.Load())
	}
	balance, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: tokenABI.PackBalanceOf(account)}, nil)
	if err != nil || new(big.Int).SetBytes(balance).Sign() != 0 {
		t.Fatal("lens changed local chain state")
	}
	// An unbound masked amount can be a fixed curator-supplied preimage.
	// Permission hashing excludes this amount; the original root/proof stays valid.
	entries[1].(map[string]any)["data"] = hexutil.Encode(tokenABI.PackTransfer(recipient, big.NewInt(17)))
	permission, err = json.Marshal(map[string]any{"root": root.Hex(), "chainId": 31337, "account": account.Hex(), "leaves": entries})
	if err != nil {
		t.Fatal(err)
	}
	permission = servicePermissionCertificate(t, permission, account, account)
	recipe.Steps[4] = issuerStep(t, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaves[1].Hash}, [][]*big.Int{{}})
	serviceSeedTransaction(t, ctx, client, owner, &target, eABI.PackSetRecipe(recipe.Queries, recipe.Steps))
	output.Reset()
	if err = runBotWithHTTP(ctx, path, true, true, &output, httpClient); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(output.Bytes(), &plans); err != nil {
		t.Fatal(err)
	}
	plan = plans[cfg.Strategies[0]]
	serviceSeedTransaction(t, ctx, client, owner, &target, plan.Calldata)
	for address, want := range map[common.Address]int64{account: 83, recipient: 17} {
		raw, e := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: tokenABI.PackBalanceOf(address)}, nil)
		if e != nil || new(big.Int).SetBytes(raw).Int64() != want {
			t.Fatalf("balance differs from prepared effects: %v", e)
		}
	}
}
