package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func loadDiscountArtifact(t *testing.T, name string) serviceArtifact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "liquidlane-discount", name+".json"))
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
	parsed, err := abi.JSON(bytes.NewReader(document.ABI))
	if err != nil {
		t.Fatal(err)
	}
	return serviceArtifact{ABI: parsed, Bytecode: document.Bytecode}
}
func TestAnvilLLConnectorDiscountFrontendFunding(t *testing.T) {
	if os.Getenv("MERKLE_ANVIL_TEST") != "1" {
		t.Skip("local Anvil only")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	client, endpoint := startServiceAnvil(t, ctx)
	var owners []common.Address
	if err := client.Client().CallContext(ctx, &owners, "eth_accounts"); err != nil {
		t.Fatal(err)
	}
	owner := owners[0]
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "providers", "liquidlanediscount", "testdata", "frontend-ll.json"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Account, Adapter, Connector, Asset, RWA, Signer, Protocol, Recipient, Oracle common.Address
		Recipe                                                                       hexutil.Bytes
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	recipe, err := recipes.DecodeRecipe(f.Recipe)
	if err != nil {
		t.Fatal(err)
	}
	aa := loadServiceArtifact(t, "FixtureAccount")
	ea := loadServiceArtifact(t, "RecipeExecutor")
	da := loadDiscountArtifact(t, "FixtureDiscountConnector")
	ta := loadDiscountArtifact(t, "FixtureDiscountToken")
	fa := loadDiscountArtifact(t, "FixtureDiscountFunding")
	oa := loadDiscountArtifact(t, "FixtureDiscountOracle")
	install := func(target common.Address, a serviceArtifact) {
		deployed := serviceSeedTransaction(t, ctx, client, owner, nil, a.Bytecode).ContractAddress
		code, e := client.CodeAt(ctx, deployed, nil)
		if e != nil {
			t.Fatal(e)
		}
		if e = client.Client().CallContext(ctx, nil, "anvil_setCode", target, hexutil.Bytes(code)); e != nil {
			t.Fatal(e)
		}
	}
	for _, item := range []struct {
		address  common.Address
		artifact serviceArtifact
	}{{f.Account, aa}, {f.Adapter, fa}, {f.Connector, da}, {f.Asset, ta}, {f.RWA, ta}, {f.Oracle, oa}} {
		install(item.address, item.artifact)
	}
	call := func(target common.Address, a serviceArtifact, method string, args ...any) {
		serviceSeedTransaction(t, ctx, client, owner, &target, servicePack(t, a, method, args...))
	}
	call(f.Connector, da, "seed", f.Account, f.Asset, f.RWA, f.Oracle, f.Signer)
	call(f.Adapter, fa, "seed", f.Asset, f.Account)
	call(f.Asset, ta, "mint", f.Adapter, big.NewInt(200))
	call(f.RWA, ta, "mint", f.Connector, big.NewInt(100))
	call(f.Asset, ta, "setPayoutBps", big.NewInt(10000))
	call(f.RWA, ta, "setPayoutBps", big.NewInt(10000))
	target := serviceSeedTransaction(t, ctx, client, owner, nil, append(bytes.Clone(ea.Bytecode), servicePack(t, ea, "", owner, f.Account)...)).ContractAddress
	roles := make([][32]byte, len(tree.Leaves))
	for i, leaf := range tree.Leaves {
		roles[i] = leaf.Hash
	}
	call(f.Account, aa, "configure", tree.Root, roles, target)
	eABI := executor.NewRecipeExecutor()
	serviceSeedTransaction(t, ctx, client, owner, &target, eABI.PackSetCaller(owner))
	serviceSeedTransaction(t, ctx, client, owner, &target, eABI.PackSetRecipe(recipe.Queries, recipe.Steps))
	var permission map[string]any
	if err = json.Unmarshal(raw, &permission); err != nil {
		t.Fatal(err)
	}
	permission["chainId"] = 31337
	permissionRaw, _ := json.Marshal(permission)
	permissionRaw = servicePermissionCertificate(t, permissionRaw, f.Account, f.Account)
	curator := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, _ *http.Request) { _, _ = out.Write(permissionRaw) }))
	defer curator.Close()
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := crypto.HexToECDSA(strings.Repeat("1", 64))
	protocol, _ := crypto.HexToECDSA(strings.Repeat("2", 64))
	deadline := header.Time + 1000
	domainName := "LiquidLaneAdapter"
	server := httptest.NewServer(http.HandlerFunc(func(out http.ResponseWriter, in *http.Request) {
		if in.Method != "POST" || in.URL.Path != "/api-internal/v1/discounts" {
			t.Error("unexpected discount route")
		}
		var selector map[string]string
		if e := json.NewDecoder(in.Body).Decode(&selector); e != nil || selector["adapter"] != f.Connector.Hex() || selector["tokenToRedeem"] != f.RWA.Hex() {
			t.Error("wrong documented pair selector", e)
		}
		domain := protocolio.Domain(domainName, "1", big.NewInt(31337), f.Connector)
		structure := crypto.Keccak256Hash(crypto.Keccak256([]byte("Discount(address tokenToRedeem,uint256 discount,address signer,address protocol,uint256 nonce,uint48 deadline)")), protocolio.AddressWord(f.RWA), protocolio.Word(big.NewInt(1250000)), protocolio.AddressWord(f.Signer), protocolio.AddressWord(f.Protocol), protocolio.Word(big.NewInt(7)), protocolio.Word(new(big.Int).SetUint64(deadline)))
		digest := protocolio.Digest(domain, structure)
		sig, e := crypto.Sign(digest.Bytes(), signer)
		if e != nil {
			t.Error(e)
		}
		sig[64] += 27
		swap := crypto.Keccak256Hash(crypto.Keccak256([]byte("DiscountSwap(Discount discount,bytes signerSignature,uint48 protocolDeadline)Discount(address tokenToRedeem,uint256 discount,address signer,address protocol,uint256 nonce,uint48 deadline)")), structure.Bytes(), crypto.Keccak256(sig), protocolio.Word(new(big.Int).SetUint64(deadline)))
		psig, e := crypto.Sign(protocolio.Digest(domain, swap).Bytes(), protocol)
		if e != nil {
			t.Error(e)
		}
		psig[64] += 27
		_ = json.NewEncoder(out).Encode(wire.LiquidLaneResolvedDiscount{DiscountID: digest.Hex(), Discount: wire.LiquidLaneDiscountTerms{Adapter: f.Connector.Hex(), TokenToRedeem: f.RWA.Hex(), Discount: "1250000", Signer: f.Signer.Hex(), Protocol: f.Protocol.Hex(), Nonce: "7", Deadline: int64(deadline)}, SignerSignature: hexutil.Encode(sig), ProtocolDeadline: int64(deadline), ProtocolSignature: hexutil.Encode(psig)})
	}))
	defer server.Close()
	rewrite, _ := url.Parse(server.URL)
	httpClient := &http.Client{Transport: serviceRewriteTransport{host: "swap.symbiotic.fi", target: rewrite}}
	cfg := testCLIConfig(t.TempDir())
	cfg.Mode = "simulate"
	cfg.Chain.RPCURL = endpoint
	cfg.Curator.URL = curator.URL
	cfg.Strategies[0] = strings.ToLower(target.Hex())
	cfg.Signer.Address = ""
	path := writeCLIConfig(t, t.TempDir(), cfg)
	if err = runBotWithHTTP(ctx, path, true, true, new(bytes.Buffer), httpClient); err == nil || !strings.Contains(err.Error(), "signer signature") {
		t.Fatalf("adapter-domain response accepted: %v", err)
	}
	domainName = "LiquidLaneConnector"
	call(f.RWA, ta, "setPayoutBps", big.NewInt(5000))
	if err = runBotWithHTTP(ctx, path, true, true, new(bytes.Buffer), httpClient); err == nil || !strings.Contains(err.Error(), "full recipe simulation failed") {
		t.Fatalf("changed RWA payout escaped original exact guard: %v", err)
	}
	call(f.RWA, ta, "setPayoutBps", big.NewInt(10000))
	var output bytes.Buffer
	if err = runBotWithHTTP(ctx, path, true, true, &output, httpClient); err != nil {
		t.Fatal(err)
	}
	var plans map[string]keeper.Plan
	if err = json.Unmarshal(output.Bytes(), &plans); err != nil {
		t.Fatal(err)
	}
	plan := plans[cfg.Strategies[0]]
	if len(plan.Outputs) != 5 || plan.Outputs[3].Inputs[0] != "76" || plan.Outputs[3].Inputs[1] != "150" {
		t.Fatalf("wrong discount plan %+v", plan.Outputs)
	}
	balance := func(token, account common.Address) *big.Int {
		data := erc20.NewERC20().PackBalanceOf(account)
		raw, e := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
		if e != nil {
			t.Fatal(e)
		}
		n, e := erc20.NewERC20().UnpackBalanceOf(raw)
		if e != nil {
			t.Fatal(e)
		}
		return n
	}
	if balance(f.Asset, f.Account).Sign() != 0 || balance(f.RWA, f.Account).Sign() != 0 {
		t.Fatal("prefix/simulation mutated account")
	}
	serviceSeedTransaction(t, ctx, client, owner, &target, plan.Calldata)
	if balance(f.Asset, f.Account).Sign() != 0 || balance(f.RWA, f.Account).Int64() != 76 || balance(f.Asset, f.Recipient).Int64() != 150 || balance(f.Asset, f.Adapter).Int64() != 50 {
		t.Fatal("funding, purchase or exact settlement differs")
	}
	data := erc20.NewERC20().PackAllowance(f.Account, f.Connector)
	raw, err = client.CallContract(ctx, ethereum.CallMsg{To: &f.Asset, Data: data}, nil)
	if err != nil || new(big.Int).SetBytes(raw).Sign() != 0 {
		t.Fatal("payment approval not reset", err)
	}
}
