package chainio

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
)

type testAggregateCodec struct{}

func (testAggregateCodec) Encode(calls []AggregateCall) ([]byte, error) { return json.Marshal(calls) }
func (testAggregateCodec) Decode(raw []byte) ([]AggregateResult, error) {
	var results []AggregateResult
	err := json.Unmarshal(raw, &results)
	return results, err
}

func TestRPCBatchReaderPreservesOrderAndSeparatesCallerAndBlock(t *testing.T) {
	a, b, aggregate := common.HexToAddress("0x01"), common.HexToAddress("0x02"), common.HexToAddress("0xcA11bde05977b3631167028862bE2a173976CA11")
	requests := []ReadRequest{ContractRead(a, []byte{1}, big.NewInt(12), true), ContractRead(b, []byte{2}, big.NewInt(12), true), ContractRead(a, []byte{3}, big.NewInt(12), true), ContractRead(a, []byte{4}, big.NewInt(13), true)}
	requests[2].Call.From = b
	calls := 0
	reader := RPCBatchReader{Multicall: aggregate, Codec: testAggregateCodec{}, Call: func(_ context.Context, batch []rpc.BatchElem) error {
		calls++
		if len(batch) != 3 {
			t.Fatalf("wrong stage shape: %d", len(batch))
		}
		for i := range batch {
			args := batch[i].Args[0].(map[string]any)
			raw := batch[i].Result.(*hexutil.Bytes)
			switch i {
			case 0:
				if *args["to"].(*common.Address) != aggregate || batch[i].Args[1] != "0xc" {
					t.Fatal("aggregate lost target/block")
				}
				var members []AggregateCall
				if err := json.Unmarshal(args["input"].(hexutil.Bytes), &members); err != nil || len(members) != 2 || members[0].Target != a || members[1].Target != b {
					t.Fatal("aggregate members changed", err)
				}
				encoded, err := json.Marshal([]AggregateResult{{Success: true, Data: []byte{1}}, {Success: false}})
				if err != nil {
					t.Fatal(err)
				}
				*raw = encoded
			case 1:
				if args["from"] != b || batch[i].Args[1] != "0xc" {
					t.Fatal("isolated caller lost")
				}
				*raw = []byte{3}
			case 2:
				if batch[i].Args[1] != "0xd" {
					t.Fatal("different block merged")
				}
				*raw = []byte{4}
			}
		}
		return nil
	}}
	result, err := reader.BatchRead(t.Context(), requests)
	if err != nil || calls != 1 || len(result) != 4 || result[0].Data[0] != 1 || result[1].Err == nil || result[2].Data[0] != 3 || result[3].Data[0] != 4 {
		t.Fatalf("wrong member association: %+v %v", result, err)
	}
}

func TestHTTPBatchRejectsAmbiguousWireResponsesWithoutRetry(t *testing.T) {
	for _, mode := range []string{"valid", "missing", "duplicate", "unknown", "version", "both", "malformed", "oversized", "http", "canceled", "member", "null"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var requests []batchMessage
				if err := json.NewDecoder(r.Body).Decode(&requests); err != nil || len(requests) != 2 {
					t.Error("wrong batch request", err)
					return
				}
				results := []map[string]any{{"jsonrpc": "2.0", "id": 2, "result": "0x02"}, {"jsonrpc": "2.0", "id": 1, "result": "0x01"}}
				switch mode {
				case "missing":
					results = results[:1]
				case "duplicate":
					results[1]["id"] = 2
				case "unknown":
					results[1]["id"] = 3
				case "version":
					results[1]["jsonrpc"] = "1.0"
				case "both":
					results[1]["error"] = map[string]any{"code": -32000, "message": "private body"}
				case "malformed":
					_, _ = w.Write([]byte("private malformed"))
					return
				case "oversized":
					_, _ = w.Write([]byte(strings.Repeat("x", maxBatchResponseBytes+1)))
					return
				case "http":
					http.Error(w, "private body", http.StatusBadGateway)
					return
				case "member":
					delete(results[1], "result")
					results[1]["error"] = map[string]any{"code": -32000, "message": "private body"}
				case "null":
					results[1]["result"] = nil
				}
				_ = json.NewEncoder(w).Encode(results)
			}))
			defer server.Close()
			var first, second hexutil.Bytes
			method := "eth_call"
			if mode == "null" {
				method = "eth_getBlockByNumber"
			}
			batch := []rpc.BatchElem{{Method: method, Args: []any{}, Result: &first}, {Method: "eth_call", Args: []any{}, Result: &second}}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			err := CallHTTPBatch(ctx, server.Client(), server.URL, batch)
			if mode == "valid" || mode == "member" || mode == "null" {
				if err != nil || len(second) != 1 || second[0] != 2 {
					t.Fatal("valid/member group rejected", err)
				}
				if mode == "valid" && (len(first) != 1 || first[0] != 1) {
					t.Fatal("out-of-order IDs misassociated")
				}
				if mode == "member" && batch[0].Error == nil {
					t.Fatal("missing member failure")
				}
				if mode == "null" && !errors.Is(batch[0].Error, ethereum.NotFound) {
					t.Fatal("null header accepted")
				}
			} else if err == nil {
				t.Fatal("invalid group accepted")
			}
			if calls > 1 {
				t.Fatal("failed batch retried")
			}
			if err != nil && strings.Contains(err.Error(), "private") {
				t.Fatal("private body exposed")
			}
		})
	}
}

func TestRPCBatchCodeOverridePreservesCompleteContext(t *testing.T) {
	to, from := common.HexToAddress("0x11"), common.HexToAddress("0x22")
	msg := ethereum.CallMsg{From: from, To: &to, Data: []byte{1, 2}, Value: big.NewInt(3), Gas: 4, GasPrice: big.NewInt(5), GasFeeCap: big.NewInt(6), GasTipCap: big.NewInt(7), BlobGasFeeCap: big.NewInt(8), BlobHashes: []common.Hash{common.HexToHash("0x09")}}
	reader := RPCBatchReader{Multicall: common.HexToAddress("0x33"), Codec: testAggregateCodec{}, Call: func(_ context.Context, b []rpc.BatchElem) error {
		if len(b) != 2 {
			t.Fatal("override aggregated")
		}
		for i := range b {
			if len(b[i].Args) != 3 {
				t.Fatal("missing code override")
			}
			raw, _ := json.Marshal(b[i].Args[0])
			expected, _ := json.Marshal(CallArgument(msg))
			if string(raw) != string(expected) {
				t.Fatalf("context changed: %s", raw)
			}
			over := b[i].Args[2].(map[common.Address]any)
			if _, ok := over[to]; !ok {
				t.Fatal("wrong override target")
			}
			*b[i].Result.(*hexutil.Bytes) = []byte{1}
		}
		return nil
	}}
	q := ReadRequest{Kind: ReadCall, Call: msg, Block: big.NewInt(10), CodeOverride: []byte{0x60}, MulticallSafe: true}
	if _, e := reader.BatchRead(t.Context(), []ReadRequest{q, q}); e != nil {
		t.Fatal(e)
	}
}
