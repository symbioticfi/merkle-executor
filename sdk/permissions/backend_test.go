package permissions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestSourceReadsCurrentBackendCertificate(t *testing.T) {
	body, err := os.ReadFile("testdata/permission-set-full.json")
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Bundle struct {
			Adapter, Account, Root string
			Metadata               json.RawMessage
		}
	}
	if err = json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	bundle := response.Bundle
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/merkle/"+bundle.Adapter+"/"+bundle.Root || r.URL.RawQuery != "" {
			t.Errorf("wrong current backend request: %s %s", r.Method, r.URL.RequestURI())
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()
	reads := 0
	source, err := New(protocolio.HTTPConfig{URL: server.URL + "/api/v1"}, server.Client(), readerFunc(func(_ context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
		reads++
		if call.To == nil || *call.To != common.HexToAddress(bundle.Account) || block == nil || block.Cmp(big.NewInt(123)) != 0 || string(call.Data) != string(merkleaccount.NewMerkleAccount().PackAdapter()) {
			t.Fatal("account adapter was not read at the authenticated pin")
		}
		return common.LeftPadBytes(common.HexToAddress(bundle.Adapter).Bytes(), 32), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := source.PermissionTree(context.Background(), big.NewInt(31337), recipes.Snapshot{Account: common.HexToAddress(bundle.Account), Root: common.HexToHash(bundle.Root), BlockNumber: 123})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, bundle.Metadata) {
		t.Fatal("permission metadata bytes changed")
	}
	tree, err := recipes.DecodeTree(raw)
	if err != nil {
		t.Fatalf("current backend certificate is not executable: %v", err)
	}
	if tree.Root != common.HexToHash(bundle.Root) || tree.Account == nil || *tree.Account != common.HexToAddress(bundle.Account) || tree.ChainID.Cmp(big.NewInt(31337)) != 0 || len(tree.Leaves) != 78 {
		t.Fatal("current backend certificate scope/calls changed")
	}
	if reads != 1 {
		t.Fatalf("permission acquisition used %d adapter reads, want one", reads)
	}
}

type readerFunc func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)

func (f readerFunc) CallContract(ctx context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	return f(ctx, call, block)
}

func TestSourceRejectsCertificateScopeAndLeafChanges(t *testing.T) {
	body, err := os.ReadFile("testdata/permission-set-full.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want string
		mutate     func(map[string]any, map[string]any)
	}{
		{"bundle chain", "permission bundle: chain mismatch", func(b, _ map[string]any) { b["chainId"] = 1 }},
		{"bundle adapter", "permission bundle: adapter mismatch", func(b, _ map[string]any) { b["adapter"] = common.HexToAddress("0x1").Hex() }},
		{"bundle account", "permission bundle: account mismatch", func(b, _ map[string]any) { b["account"] = common.HexToAddress("0x1").Hex() }},
		{"bundle root", "permission bundle: root mismatch", func(b, _ map[string]any) { b["root"] = common.HexToHash("0x1").Hex() }},
		{"metadata chain", "permission metadata: chain mismatch", func(_, m map[string]any) { m["chainId"] = 1 }},
		{"metadata adapter", "permission metadata: adapter mismatch", func(_, m map[string]any) { m["adapter"] = common.HexToAddress("0x1").Hex() }},
		{"metadata account", "permission metadata: account mismatch", func(_, m map[string]any) { m["account"] = common.HexToAddress("0x1").Hex() }},
		{"metadata root", "permission metadata: root mismatch", func(_, m map[string]any) { m["root"] = common.HexToHash("0x1").Hex() }},
		{"bundle fractional chain", "permission bundle: chain mismatch", func(b, _ map[string]any) { b["chainId"] = json.Number("31337.0") }},
		{"metadata exponent chain", "permission metadata: chain mismatch", func(_, m map[string]any) { m["chainId"] = json.Number("3.1337e4") }},
		{"metadata overflow chain", "permission metadata: chain mismatch", func(_, m map[string]any) { m["chainId"] = new(big.Int).Lsh(big.NewInt(1), 256).String() }},
		{"metadata zero account", "permission metadata: account mismatch", func(_, m map[string]any) { m["account"] = common.Address{}.Hex() }},
		{"metadata short adapter", "permission metadata: adapter mismatch", func(_, m map[string]any) { m["adapter"] = "0x01" }},
		{"metadata null", "permission metadata: chain mismatch", func(b, _ map[string]any) { b["metadata"] = nil }},
		{"metadata scalar", "invalid permission metadata", func(b, _ map[string]any) { b["metadata"] = "private malformed metadata" }},
		{"missing leaves", "permission bundle requires 1..4096 leaves", func(b, _ map[string]any) { delete(b, "leaves") }},
		{"too many leaves", "permission bundle requires 1..4096 leaves", func(b, _ map[string]any) { b["leaves"] = make([]common.Hash, 4097) }},
		{"leaf omitted", "permission bundle leaves do not match root", func(b, _ map[string]any) { b["leaves"] = b["leaves"].([]any)[1:] }},
		{"leaf repeated", "permission bundle leaves must be sorted and unique", func(b, _ map[string]any) { b["leaves"].([]any)[1] = b["leaves"].([]any)[0] }},
		{"leaf unsorted", "permission bundle leaves must be sorted and unique", func(b, _ map[string]any) {
			leaves := b["leaves"].([]any)
			leaves[0], leaves[1] = leaves[1], leaves[0]
		}},
		{"missing calls", "permission metadata tree: tree requires exactly one of leaves or calls", func(_, m map[string]any) { delete(m, "calls") }},
		{"ambiguous calls and leaves", "permission metadata tree: tree requires exactly one of leaves or calls", func(_, m map[string]any) { m["leaves"] = m["calls"] }},
		{"bad proof", "permission metadata tree: leaf 0 proof does not match root", func(_, m map[string]any) {
			m["calls"].([]any)[0].(map[string]any)["proof"] = []string{common.HexToHash("0x1").Hex()}
		}},
		{"malformed proof", "permission metadata tree: leaf 0: proof: expected a 32-byte hexadecimal hash", func(_, m map[string]any) {
			m["calls"].([]any)[0].(map[string]any)["proof"] = []string{"0x01"}
		}},
		{"unused corrupt call", "permission metadata tree: leaf 0 hash mismatch", func(_, m map[string]any) {
			m["calls"].([]any)[0].(map[string]any)["target"] = common.HexToAddress("0x1").Hex()
		}},
		{"duplicate corrupt call", "permission metadata tree: leaf 78 hash mismatch", func(_, m map[string]any) {
			calls := m["calls"].([]any)
			duplicate := make(map[string]any)
			for key, value := range calls[0].(map[string]any) {
				duplicate[key] = value
			}
			duplicate["target"] = common.HexToAddress("0x1").Hex()
			m["calls"] = append(calls, duplicate)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var document map[string]any
			decoder := json.NewDecoder(strings.NewReader(string(body)))
			decoder.UseNumber()
			if err := decoder.Decode(&document); err != nil {
				t.Fatal(err)
			}
			bundle := document["bundle"].(map[string]any)
			adapter, account, root := common.HexToAddress(bundle["adapter"].(string)), common.HexToAddress(bundle["account"].(string)), common.HexToHash(bundle["root"].(string))
			tc.mutate(bundle, bundle["metadata"].(map[string]any))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(document) }))
			defer server.Close()
			source, err := New(protocolio.HTTPConfig{URL: server.URL}, server.Client(), readerFunc(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
				return common.LeftPadBytes(adapter.Bytes(), 32), nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := source.PermissionTree(t.Context(), big.NewInt(31337), recipes.Snapshot{Account: account, Root: root, BlockNumber: 123})
			if err == nil || !strings.Contains(err.Error(), tc.want) || raw != nil {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
			if strings.Contains(err.Error(), "private") {
				t.Fatal("malformed metadata leaked through error")
			}
		})
	}
}

func TestSourceRejectsFailedAccountAdapterBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		err  error
		want string
	}{
		{"empty", nil, nil, "invalid permission account adapter"},
		{"zero", make([]byte, 32), nil, "invalid permission account adapter"},
		{"noncanonical", append([]byte{1}, make([]byte, 31)...), nil, "invalid permission account adapter"},
		{"trailing word", make([]byte, 64), nil, "invalid permission account adapter"},
		{"read failure", nil, errors.New("private chain provider details"), "read permission account adapter: protocol chain read failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
			defer server.Close()
			source, err := New(protocolio.HTTPConfig{URL: server.URL}, server.Client(), readerFunc(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) { return tc.data, tc.err }))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := source.PermissionTree(t.Context(), big.NewInt(1), recipes.Snapshot{Account: common.HexToAddress("0x1")})
			if err == nil || !strings.Contains(err.Error(), tc.want) || calls != 0 || raw != nil {
				t.Fatalf("invalid adapter reached HTTP: %d %v", calls, err)
			}
			if strings.Contains(err.Error(), "private") {
				t.Fatal("chain provider error leaked")
			}
		})
	}
}

func TestSourcePreservesUint256ChainIDPrecision(t *testing.T) {
	body, err := os.ReadFile("testdata/permission-set-full.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []*big.Int{
		new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 53), big.NewInt(1)),
		new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)),
	} {
		for _, quoted := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/quoted=%t", id, quoted), func(t *testing.T) {
				var document map[string]any
				decoder := json.NewDecoder(bytes.NewReader(body))
				decoder.UseNumber()
				if err := decoder.Decode(&document); err != nil {
					t.Fatal(err)
				}
				bundle := document["bundle"].(map[string]any)
				metadata := bundle["metadata"].(map[string]any)
				var wireID any = json.Number(id.String())
				if quoted {
					wireID = id.String()
				}
				bundle["chainId"], metadata["chainId"] = wireID, wireID
				adapter := common.HexToAddress(bundle["adapter"].(string))
				account := common.HexToAddress(bundle["account"].(string))
				root := common.HexToHash(bundle["root"].(string))
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(document) }))
				defer server.Close()
				source, err := New(protocolio.HTTPConfig{URL: server.URL}, server.Client(), readerFunc(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
					return common.LeftPadBytes(adapter.Bytes(), 32), nil
				}))
				if err != nil {
					t.Fatal(err)
				}
				snapshot := recipes.Snapshot{Account: account, Root: root, BlockNumber: 123}
				raw, err := source.PermissionTree(t.Context(), id, snapshot)
				if err != nil {
					t.Fatal(err)
				}
				tree, err := recipes.DecodeTree(raw)
				if err != nil || tree.ChainID.Cmp(id) != 0 {
					t.Fatalf("chain ID precision changed: %v %v", tree, err)
				}
				neighbor := new(big.Int).Sub(id, big.NewInt(1))
				if raw, err := source.PermissionTree(t.Context(), neighbor, snapshot); err == nil || !strings.Contains(err.Error(), "permission bundle: chain mismatch") || raw != nil {
					t.Fatalf("adjacent uint256 chain ID accepted: %v", err)
				}
			})
		}
	}
}

func TestSourceHonorsExactHTTPResponseBound(t *testing.T) {
	body, err := os.ReadFile("testdata/permission-set-full.json")
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Bundle struct {
			Adapter, Account, Root string
			Metadata               json.RawMessage
		}
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	bundle := response.Bundle
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	for _, extra := range []int64{0, 1} {
		t.Run(fmt.Sprintf("responseOverBound=%d", extra), func(t *testing.T) {
			source, err := New(protocolio.HTTPConfig{URL: server.URL, MaxResponseBytes: int64(len(body)) - extra}, server.Client(), readerFunc(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
				return common.LeftPadBytes(common.HexToAddress(bundle.Adapter).Bytes(), 32), nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := source.PermissionTree(t.Context(), big.NewInt(31337), recipes.Snapshot{Account: common.HexToAddress(bundle.Account), Root: common.HexToHash(bundle.Root), BlockNumber: 123})
			if extra == 0 {
				if err != nil || !bytes.Equal(raw, bundle.Metadata) {
					t.Fatalf("exact-bound response rejected: %v", err)
				}
			} else if err == nil || err.Error() != "protocol response exceeds configured bound" || raw != nil {
				t.Fatalf("one-byte oversized response accepted: %v", err)
			}
		})
	}
}
