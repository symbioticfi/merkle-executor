package permissions

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
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
		{"missing leaves", "permission bundle requires 1..4096 leaves", func(b, _ map[string]any) { delete(b, "leaves") }},
		{"leaf omitted", "permission bundle leaves do not match root", func(b, _ map[string]any) { b["leaves"] = b["leaves"].([]any)[1:] }},
		{"leaf repeated", "permission bundle leaves must be sorted and unique", func(b, _ map[string]any) { b["leaves"].([]any)[1] = b["leaves"].([]any)[0] }},
		{"unused corrupt call", "permission metadata tree: leaf 0 hash mismatch", func(_, m map[string]any) {
			m["calls"].([]any)[0].(map[string]any)["target"] = common.HexToAddress("0x1").Hex()
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
			_, err = source.PermissionTree(t.Context(), big.NewInt(31337), recipes.Snapshot{Account: account, Root: root, BlockNumber: 123})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, err)
			}
		})
	}
}

func TestSourceRejectsFailedAccountAdapterBeforeHTTP(t *testing.T) {
	for _, data := range [][]byte{nil, make([]byte, 32), append([]byte{1}, make([]byte, 31)...), make([]byte, 64)} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
		source, err := New(protocolio.HTTPConfig{URL: server.URL}, server.Client(), readerFunc(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) { return data, nil }))
		if err != nil {
			t.Fatal(err)
		}
		_, err = source.PermissionTree(t.Context(), big.NewInt(1), recipes.Snapshot{Account: common.HexToAddress("0x1")})
		server.Close()
		if err == nil || !strings.Contains(err.Error(), "invalid permission account adapter") || calls != 0 {
			t.Fatalf("invalid adapter reached HTTP: %d %v", calls, err)
		}
	}
}
