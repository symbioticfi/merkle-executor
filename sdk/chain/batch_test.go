package chain

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
	multicall "github.com/symbioticfi/merkle-executor/sdk/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

func TestBatchReadsPreserveCallsAndAssociateResponsesByID(t *testing.T) {
	from, to := common.HexToAddress("0x01"), common.HexToAddress("0x02")
	requests := 0
	read := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if body[0] != '[' {
			var req rpcRequest
			_ = json.Unmarshal(body, &req)
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
			return
		}
		requests++
		var batch []rpcRequest
		if err := json.Unmarshal(body, &batch); err != nil || len(batch) != 4 {
			t.Fatalf("expected four-member array: %v", err)
		}
		var call map[string]any
		if err := json.Unmarshal(batch[0].Params[0], &call); err != nil {
			t.Fatal(err)
		}
		expected := map[string]any{"from": from.Hex(), "to": to.Hex(), "input": "0x1234", "value": "0x3", "gas": "0x4d", "gasPrice": "0x7", "maxFeePerGas": "0x5", "maxPriorityFeePerGas": "0x2", "maxFeePerBlobGas": "0x9"}
		for key, value := range expected {
			if call[key] != value {
				t.Errorf("call %s=%v want %v", key, call[key], value)
			}
		}
		for _, key := range []string{"accessList", "blobVersionedHashes", "authorizationList"} {
			if _, ok := call[key]; !ok {
				t.Errorf("missing call field %s", key)
			}
		}
		if string(batch[0].Params[1]) != `"0xc"` {
			t.Error("block was not pinned")
		}
		_ = json.NewEncoder(w).Encode([]any{
			map[string]any{"jsonrpc": "2.0", "id": batch[3].ID, "result": nil},
			map[string]any{"jsonrpc": "2.0", "id": batch[2].ID, "error": map[string]any{"code": -32000, "message": "secret-provider-body execution reverted", "data": "0x1234"}},
			map[string]any{"jsonrpc": "2.0", "id": batch[1].ID, "result": nil},
			map[string]any{"jsonrpc": "2.0", "id": batch[0].ID, "result": "0xabcd"},
		})
	}))
	defer read.Close()
	write := newRPCFixture(t, "0x1")
	client, err := Dial(context.Background(), read.URL, write.server.URL, big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	batcher, ok := any(client).(protocolio.BatchReader)
	if !ok {
		t.Fatal("production chain lacks read batching")
	}
	message := ethereum.CallMsg{From: from, To: &to, Data: []byte{0x12, 0x34}, Value: big.NewInt(3), Gas: 77, GasPrice: big.NewInt(7), GasFeeCap: big.NewInt(5), GasTipCap: big.NewInt(2), BlobGasFeeCap: big.NewInt(9), BlobHashes: []common.Hash{{1}}, AccessList: types.AccessList{{Address: to}}, AuthorizationList: []types.SetCodeAuthorization{}}
	results, err := batcher.BatchRead(context.Background(), []protocolio.ReadRequest{
		{Kind: protocolio.ReadCall, Call: message, Block: big.NewInt(12)},
		{Kind: protocolio.ReadHeader, Block: big.NewInt(13)},
		protocolio.ContractRead(to, []byte{1, 2, 3, 4}, big.NewInt(12), false),
		{Kind: protocolio.ReadReceipt, Hash: common.HexToHash("0x05")},
	})
	if err != nil || len(results) != 4 || requests != 1 {
		t.Fatalf("batch response: %+v %v requests=%d", results, err, requests)
	}
	if string(results[0].Data) != string([]byte{0xab, 0xcd}) || results[0].Err != nil {
		t.Fatal("shuffled result lost association")
	}
	if !errors.Is(results[1].Err, ethereum.NotFound) || !errors.Is(results[3].Err, ethereum.NotFound) {
		t.Fatal("null header/receipt lost NotFound")
	}
	var cause rpc.Error
	if !errors.As(results[2].Err, &cause) || cause.ErrorCode() != -32000 || strings.Contains(results[2].Err.Error(), "secret-provider-body") {
		t.Fatalf("RPC identity or redaction changed: %v", results[2].Err)
	}
	write.mu.Lock()
	defer write.mu.Unlock()
	if len(write.requests) != 1 || write.requests[0].Method != "eth_chainId" {
		t.Fatal("batch escaped read endpoint")
	}
}
func TestBatchReadsRejectMissingDuplicateAndMalformedResponses(t *testing.T) {
	for _, mode := range []string{"missing", "duplicate", "unknown", "malformed", "transport", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			arrays := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body json.RawMessage
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body[0] != '[' {
					var req rpcRequest
					_ = json.Unmarshal(body, &req)
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
					return
				}
				arrays++
				var reqs []rpcRequest
				_ = json.Unmarshal(body, &reqs)
				if mode == "transport" {
					http.Error(w, "secret-url-token", http.StatusBadGateway)
					return
				}
				if mode == "malformed" {
					_, _ = w.Write([]byte("invalid secret payload"))
					return
				}
				response := []map[string]any{{"jsonrpc": "2.0", "id": reqs[0].ID, "result": "0x01"}, {"jsonrpc": "2.0", "id": reqs[1].ID, "result": "0x02"}}
				if mode == "missing" {
					response = response[:1]
				}
				if mode == "duplicate" {
					response[1]["id"] = reqs[0].ID
				}
				if mode == "unknown" {
					response[1]["id"] = 999
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			client, err := Dial(context.Background(), server.URL, "", big.NewInt(1))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			batcher, ok := any(client).(protocolio.BatchReader)
			if !ok {
				t.Fatal("production chain lacks read batching")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			_, err = batcher.BatchRead(ctx, []protocolio.ReadRequest{{Kind: protocolio.ReadHeader}, {Kind: protocolio.ReadHeader}})
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("invalid response accepted or leaked: %v", err)
			}
			if mode == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation identity lost")
			}
			if arrays > 1 {
				t.Fatal("failed batch retried")
			}
		})
	}
}

func TestMixedBatchAggregatesOnlyCompatibleVerifiedCalls(t *testing.T) {
	for _, mode := range []string{"member failure", "malformed", "wrong count", "outer error"} {
		t.Run(mode, func(t *testing.T) {
			aggregate, target := common.HexToAddress("0x05"), common.HexToAddress("0x06")
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body json.RawMessage
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body[0] != '[' {
					var req rpcRequest
					_ = json.Unmarshal(body, &req)
					_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": "0x1"})
					return
				}
				requests++
				var reqs []rpcRequest
				_ = json.Unmarshal(body, &reqs)
				if len(reqs) != 2 {
					t.Fatalf("mixed stage should use aggregate plus isolated call, got%d", len(reqs))
				}
				var call struct {
					To    common.Address
					Input hexutil.Bytes
					From  common.Address
				}
				_ = json.Unmarshal(reqs[0].Params[0], &call)
				if call.To != aggregate {
					t.Fatal("safe calls did not target configured aggregate")
				}
				a := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
				values, err := a.Inputs.Unpack(call.Input[4:])
				if err != nil {
					t.Fatal(err)
				}
				calls := *abi.ConvertType(values[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
				if len(calls) != 2 || calls[0].Target != target || !calls[0].AllowFailure || !calls[1].AllowFailure || calls[0].CallData[0] != 1 || calls[1].CallData[0] != 3 {
					t.Fatal("aggregate subcall order, target or lazy failure policy changed")
				}
				_ = json.Unmarshal(reqs[1].Params[0], &call)
				if call.To != target || call.From != common.HexToAddress("0x09") || call.Input[0] != 2 {
					t.Fatal("caller-sensitive call was aggregated or changed")
				}
				results := []multicall.Multicall3Result{{Success: true, ReturnData: []byte{10}}, {Success: false, ReturnData: []byte{20}}}
				if mode == "wrong count" {
					results = results[:1]
				}
				raw, err := a.Outputs.Pack(results)
				if err != nil {
					t.Fatal(err)
				}
				if mode == "malformed" {
					raw = []byte{1}
				}
				response := []map[string]any{{"jsonrpc": "2.0", "id": reqs[0].ID, "result": hexutil.Encode(raw)}, {"jsonrpc": "2.0", "id": reqs[1].ID, "result": "0x0b"}}
				if mode == "outer error" {
					delete(response[0], "result")
					response[0]["error"] = map[string]any{"code": -32000, "message": "secret aggregate failure"}
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			client, err := Dial(context.Background(), server.URL, "", big.NewInt(1), aggregate)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			reqs := []protocolio.ReadRequest{protocolio.ContractRead(target, []byte{1, 0, 0, 0}, big.NewInt(12), true), protocolio.ContractRead(target, []byte{2, 0, 0, 0}, big.NewInt(12), true), protocolio.ContractRead(target, []byte{3, 0, 0, 0}, big.NewInt(12), true)}
			reqs[1].Call.From = common.HexToAddress("0x09")
			results, err := client.BatchRead(context.Background(), reqs)
			if requests != 1 {
				t.Fatalf("mixed stage used%d requests", requests)
			}
			if mode == "malformed" || mode == "wrong count" {
				if err == nil {
					t.Fatal("invalid aggregate accepted")
				}
				return
			}
			if err != nil || len(results) != 3 || results[1].Err != nil || results[1].Data[0] != 11 || results[2].Err == nil {
				t.Fatalf("aggregate association/lazy error lost: %+v %v", results, err)
			}
			if mode == "outer error" {
				if results[0].Err == nil || strings.Contains(results[0].Err.Error(), "secret") {
					t.Fatal("aggregate error lost or leaked")
				}
			} else if results[0].Err != nil || results[0].Data[0] != 10 {
				t.Fatal("successful aggregate member lost")
			}
		})
	}
}
