package chain

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

func TestCodeOverridePreservesFullCallAndNumericBlock(t *testing.T) {
	target, caller := common.HexToAddress("0x01"), common.HexToAddress("0x02")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		result := "0x1"
		if q.Method == "eth_call" {
			if len(q.Params) != 3 {
				t.Error("override arguments changed")
				return
			}
			if string(q.Params[1]) != `"0xc"` {
				t.Error("override block changed")
			}
			var call map[string]any
			_ = json.Unmarshal(q.Params[0], &call)
			if call["from"] != caller.Hex() || call["to"] != target.Hex() || call["input"] != "0x0304" || call["value"] != "0x5" || call["gas"] != "0x6" || call["maxFeePerGas"] != "0x7" {
				t.Error("override call changed")
			}
			var overrides map[string]map[string]string
			_ = json.Unmarshal(q.Params[2], &overrides)
			if len(overrides) != 1 || overrides[target.Hex()]["code"] != "0x0809" {
				t.Error("override code changed")
			}
			result = "0x0a"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result})
	}))
	defer server.Close()
	client, err := Dial(t.Context(), server.URL, "", big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	result, err := client.CallContractWithCode(t.Context(), ethereum.CallMsg{From: caller, To: &target, Data: []byte{3, 4}, Value: big.NewInt(5), Gas: 6, GasFeeCap: big.NewInt(7)}, big.NewInt(12), []byte{8, 9})
	if err != nil || len(result) != 1 || result[0] != 10 {
		t.Fatal("override failed", err)
	}
}
