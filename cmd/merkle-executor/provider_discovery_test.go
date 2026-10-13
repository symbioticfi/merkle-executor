package main

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	settlement "github.com/symbioticfi/merkle-executor/api/bindings/cowsettlement"
	"github.com/symbioticfi/merkle-executor/internal/config"
	"github.com/symbioticfi/merkle-executor/internal/providers/cowswap"
	"github.com/symbioticfi/merkle-executor/internal/providers/merkl"
	accountbinding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestDaemonDirectProvidersUseDiscoveredAccount(t *testing.T) {
	for _, name := range []string{"merkl", "cowswap", "liquidlane", "liquidlane-rfq"} {
		t.Run(name, func(t *testing.T) {
			cfg := testCLIConfig(t.TempDir())
			target := common.HexToAddress(cfg.Strategies[0].Executor)
			account := common.HexToAddress("0x1234")
			caller := common.HexToAddress(cfg.Signer.Address)
			root := common.HexToHash("0x99")
			header := &types.Header{Number: big.NewInt(12), Difficulty: new(big.Int), Time: 1000}
			recipe := recipes.Recipe{}
			hash, _ := recipes.HashRecipe(recipe)
			abi := executor.NewRecipeExecutor()
			event := abi.GetABI().Events["SetRecipe"]
			eventData, _ := event.Inputs.NonIndexed().Pack(recipe.Queries, recipe.Steps)
			var apiCalls atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiCalls.Add(1)
				if name == "merkl" {
					if r.Method != "GET" || r.URL.Path != "/v4/users/"+strings.ToLower(account.Hex())+"/rewards" || r.URL.Query().Get("chainId") != "31337" {
						t.Error("Merkl used configured/manual account")
					}
				} else {
					var quote struct{ From, Receiver string }
					if json.NewDecoder(r.Body).Decode(&quote) != nil || quote.From != account.Hex() || quote.Receiver != account.Hex() || r.Method != "POST" || r.URL.Path != "/api/v1/quote" {
						t.Errorf("CoW used wrong owner/receiver: %+v %s", quote, r.URL.Path)
					}
				}
				http.Error(w, "fixture stops before authorization", http.StatusServiceUnavailable)
			}))
			defer api.Close()
			curatorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"chainId": 31337, "root": root.Hex(), "adapter": common.HexToAddress("0x888").Hex(), "leaves": []any{}})
			}))
			defer curatorServer.Close()
			cfg.Curator.URL = curatorServer.URL
			rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var raw json.RawMessage
				_ = json.NewDecoder(r.Body).Decode(&raw)
				type request struct {
					ID     json.RawMessage
					Method string
					Params []json.RawMessage
				}
				var qs []request
				batch := raw[0] == '['
				if batch {
					_ = json.Unmarshal(raw, &qs)
				} else {
					qs = make([]request, 1)
					_ = json.Unmarshal(raw, &qs[0])
				}
				responses := make([]any, len(qs))
				for i, q := range qs {
					var result any
					switch q.Method {
					case "eth_chainId":
						result = "0x7a69"
					case "eth_getBlockByNumber":
						result = header
					case "eth_getLogs":
						result = []types.Log{{Address: target, BlockNumber: 12, BlockHash: header.Hash(), Topics: []common.Hash{event.ID, hash}, Data: eventData}}
					case "eth_call":
						var call struct {
							To    common.Address
							Input hexutil.Bytes
						}
						_ = json.Unmarshal(q.Params[0], &call)
						if call.To == target {
							contractABI := abi.GetABI()
							m, e := contractABI.MethodById(call.Input)
							if e != nil {
								t.Error(e)
								return
							}
							var v any
							switch m.Name {
							case "account":
								v = account
							case "caller":
								v = caller
							case "recipeHash":
								v = hash
							case "recipeBlock":
								v = big.NewInt(12)
							}
							b, _ := m.Outputs.Pack(v)
							result = hexutil.Bytes(b)
						} else if call.To == account {
							b, _ := accountbinding.NewMerkleAccount().GetABI().Methods["root"].Outputs.Pack(root)
							result = hexutil.Bytes(b)
						} else if call.To == common.HexToAddress("0x44") {
							b, _ := settlement.NewCowSettlement().GetABI().Methods["domainSeparator"].Outputs.Pack(protocolio.Domain("Gnosis Protocol", "v2", big.NewInt(31337), call.To))
							result = hexutil.Bytes(b)
						} else {
							result = hexutil.Bytes{1}
						} // Unused staged Merkl failure stays lazy until API classification.
					default:
						t.Errorf("unexpected method %s", q.Method)
					}
					responses[i] = map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result}
				}
				if batch {
					_ = json.NewEncoder(w).Encode(responses)
				} else {
					_ = json.NewEncoder(w).Encode(responses[0])
				}
			}))
			defer rpc.Close()
			cfg.Chain.RPCURL = rpc.URL
			var providerConfig any
			switch name {
			case "merkl":
				providerConfig = merkl.Config{HTTP: protocolio.HTTPConfig{URL: api.URL}, Distributor: common.HexToAddress("0x55").Hex(), Multicall3: common.HexToAddress("0x66").Hex(), Tokens: []string{common.HexToAddress("0x77").Hex()}, ProofLengths: []int{0}, Action: "0:0", CalldataBytes: 420}
			case "cowswap":
				providerConfig = cowswap.Config{HTTP: protocolio.HTTPConfig{URL: api.URL}, Settlement: common.HexToAddress("0x44").Hex(), SellToken: common.HexToAddress("0x55").Hex(), BuyToken: common.HexToAddress("0x66").Hex(), SellAmount: "1000", MinBuyAmount: "900", ValiditySeconds: 600, AppData: common.HexToHash("0x77").Hex(), Action: "0:0"}
			default:
				providerConfig = map[string]any{}
			}
			cfg.Strategies[0].Providers = []config.Provider{{Name: name, Config: serviceNode(t, providerConfig)}}
			err := runBot(t.Context(), writeCLIConfig(t, t.TempDir(), cfg), true, true, new(bytes.Buffer))
			if err == nil {
				t.Fatal("fixture unexpected success")
			}
			if name == "liquidlane-rfq" {
				if strings.Contains(err.Error(), "unknown provider") {
					t.Fatalf("consumer redemption is unavailable: %v", err)
				}
			} else if name == "liquidlane" {
				if apiCalls.Load() != 0 || !strings.Contains(err.Error(), "unknown provider") {
					t.Fatalf("signed Liquid Lane remained registered: %v", err)
				}
			} else if apiCalls.Load() != 1 {
				t.Fatalf("daemon did not fetch %s for derived account: calls%d %v", name, apiCalls.Load(), err)
			}
		})
	}
}
