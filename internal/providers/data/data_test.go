package data

import (
	"context"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"

	"github.com/symbioticfi/merkle-executor/internal/provider"
)

func node(t *testing.T, text string) yaml.Node {
	t.Helper()
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(text), &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHTTPExactIntegerAndPointer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/0x0000000000000000000000000000000000000001" {
			t.Error(r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"rows":[{"amount":9007199254740993,"call":"0x12345678"}]}`))
	}))
	defer server.Close()
	p, err := httpFactory(node(t, "url: "+server.URL+"/{{account}}\ninputs: {0: /rows/0/amount}\nactions: {'9:0': /rows/0/call}"), provider.Deps{HTTP: server.Client(), Log: logr.Discard()})
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.Prepare(context.Background(), provider.Snapshot{Account: common.HexToAddress("0x1")})
	if err != nil {
		t.Fatal(err)
	}
	if out.Inputs[0] != "9007199254740993" || out.Actions["9:0"].String() != "0x12345678" {
		t.Fatal(out)
	}
}

func TestHTTPRejectsBadResponses(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"amount":1.2}`, `{"amount":"-1"}`, `{"amount":null}`, `{} {}`, `{"amount":"` + new(big.Int).Lsh(big.NewInt(1), 256).String() + `"}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			p, err := httpFactory(node(t, "url: "+server.URL+"\ninputs: {0: /amount}"), provider.Deps{HTTP: server.Client()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = p.Prepare(context.Background(), provider.Snapshot{}); err == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
}

type reader struct{ block *big.Int }

func (r *reader) CallContract(_ context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	r.block = block
	return common.LeftPadBytes([]byte{42}, 32), nil
}

func TestRPCPinsReadsAndBindsWords(t *testing.T) {
	t.Parallel()
	r := &reader{}
	p, err := rpcFactory(node(t, "reads:\n  - {target: '{{account}}', data: '0x12345678', offset: 0, input: 0}\nactions:\n  '9:0':\n    data: '0x123456780000000000000000000000000000000000000000000000000000000000000000'\n    words: [{offset: 4, input: 0}]"), provider.Deps{Chain: r})
	if err != nil {
		t.Fatal(err)
	}
	out, err := p.Prepare(context.Background(), provider.Snapshot{Account: common.HexToAddress("0x1"), BlockNumber: big.NewInt(77)})
	if err != nil {
		t.Fatal(err)
	}
	if r.block.Int64() != 77 || out.Inputs[0] != "42" || out.Actions["9:0"][35] != 42 {
		t.Fatal(out)
	}
}
