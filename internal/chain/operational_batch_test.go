package chain

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestOperationalReadsBatchAndRejectNullScalars(t *testing.T) {
	for _, mode := range []string{"valid", "null balance", "null nonce", "history error"} {
		t.Run(mode, func(t *testing.T) {
			arrays, singles := 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var raw json.RawMessage
				_ = json.NewDecoder(r.Body).Decode(&raw)
				var reqs []rpcRequest
				if raw[0] == '[' {
					arrays++
					_ = json.Unmarshal(raw, &reqs)
				} else {
					singles++
					var q rpcRequest
					_ = json.Unmarshal(raw, &q)
					reqs = []rpcRequest{q}
				}
				out := make([]map[string]any, len(reqs))
				for i, q := range reqs {
					var value any = "0x1"
					switch q.Method {
					case "eth_getBalance":
						value = "0x7"
						if mode == "null balance" {
							value = nil
						}
					case "eth_getTransactionCount":
						value = "0x2"
						if string(q.Params[1]) == `"pending"` {
							value = "0x3"
						}
						if mode == "null nonce" {
							value = nil
						}
					case "eth_getBlockByNumber":
						value = &types.Header{Number: big.NewInt(12), Difficulty: big.NewInt(0), GasLimit: 30000000}
					case "eth_feeHistory":
						value = map[string]any{"oldestBlock": "0xa", "baseFeePerGas": []string{"0x1", "0x2"}, "gasUsedRatio": []float64{0.5}, "reward": [][]string{{"0x3"}}}
					}
					out[i] = map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": value}
					if q.Method == "eth_feeHistory" && mode == "history error" {
						delete(out[i], "result")
						out[i]["error"] = map[string]any{"code": -32000, "message": "secret"}
					}
				}
				if raw[0] == '[' {
					_ = json.NewEncoder(w).Encode(out)
				} else {
					_ = json.NewEncoder(w).Encode(out[0])
				}
			}))
			defer srv.Close()
			client, err := Dial(context.Background(), srv.URL, "", big.NewInt(1))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			account, ok := any(client).(interface {
				ReadAccountTelemetry(context.Context, common.Address) (*big.Int, uint64, uint64, error)
			})
			if !ok {
				t.Fatal("missing complete account batching")
			}
			balance, mined, pending, err := account.ReadAccountTelemetry(context.Background(), common.Address{1})
			if mode == "null balance" || mode == "null nonce" {
				if err == nil || balance != nil || mined != 0 || pending != 0 {
					t.Fatal("partial/null telemetry accepted")
				}
			} else if err != nil || balance.Int64() != 7 || mined != 2 || pending != 3 {
				t.Fatalf("telemetry %v %d %d %v", balance, mined, pending, err)
			}
			_, _, err = client.ReadNonces(context.Background(), common.Address{1})
			if (err != nil) != (mode == "null nonce") {
				t.Fatal(err)
			}
			fee, ok := any(client).(interface {
				ReadFeeContext(context.Context, uint64, []float64) (*types.Header, *ethereum.FeeHistory, error, error)
			})
			if !ok {
				t.Fatal("missing fee batching")
			}
			header, history, he, fe := fee.ReadFeeContext(context.Background(), 1, []float64{50})
			if he != nil || header.Number.Int64() != 12 {
				t.Fatalf("fee head: %v", he)
			}
			if mode == "history error" {
				if fe == nil {
					t.Fatal("lost fee member error")
				}
			} else if fe != nil || history.OldestBlock.Int64() != 10 || history.BaseFee[1].Int64() != 2 || history.Reward[0][0].Int64() != 3 {
				t.Fatalf("fee history: %+v %v", history, fe)
			}
			if arrays != 3 || singles != 1 {
				t.Fatalf("requests arrays=%d singles=%d", arrays, singles)
			}
		})
	}
}
