package recipes

import (
	"bytes"
	"context"
	"math/big"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/accessmanager"
)

type authorityReader struct {
	Reader
	snapshot Snapshot
	sender   common.Address
	data     []byte
	result   []byte
	err      error
	reads    int
}

func (r *authorityReader) CallContract(_ context.Context, call ethereum.CallMsg, block *big.Int) ([]byte, error) {
	r.reads++
	if call.To == nil || *call.To != r.snapshot.Caller || block == nil || block.Uint64() != r.snapshot.BlockNumber || !bytes.Equal(call.Data, accessmanager.NewAccessManager().PackCanCall(r.sender, r.snapshot.Executor, [4]byte(r.data[:4]))) {
		return nil, errors.New("wrong pinned authority read")
	}
	return r.result, r.err
}

func TestRouteExecution(t *testing.T) {
	manager := accessmanager.NewAccessManager()
	for _, tc := range []struct {
		name                        string
		immediate                   bool
		delay                       uint32
		direct, malformed, rpcError bool
		want                        string
	}{
		{name: "manager", immediate: true},
		{name: "direct", direct: true},
		{name: "denied", want: "immediate"},
		{name: "delayed", delay: 60, want: "immediate"},
		{name: "malformed", malformed: true, want: "decode"},
		{name: "rpc", rpcError: true, want: "read"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &authorityReader{snapshot: Snapshot{Executor: common.Address{1}, Caller: common.Address{2}, BlockNumber: 123}, sender: common.Address{3}, data: []byte{1, 2, 3, 4, 5}}
			if tc.direct {
				r.sender = r.snapshot.Caller
			}
			var err error
			r.result, err = manager.GetABI().Methods["canCall"].Outputs.Pack(tc.immediate, tc.delay)
			if err != nil {
				t.Fatal(err)
			}
			if tc.malformed {
				r.result = nil
			}
			if tc.rpcError {
				r.err = errors.New("unavailable")
			}
			call, err := RouteExecution(t.Context(), r, r.snapshot, r.sender, r.data)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.direct {
				if r.reads != 0 || call.Executor != r.snapshot.Executor || !bytes.Equal(call.Data, r.data) {
					t.Fatal("direct route changed")
				}
			} else {
				if r.reads != 1 || call.Executor != r.snapshot.Caller || !bytes.Equal(call.Data, manager.PackExecute(r.snapshot.Executor, r.data)) {
					t.Fatal("manager route changed")
				}
			}
		})
	}
}

func TestRouteExecutionRejectsInvalidIdentity(t *testing.T) {
	for _, field := range []string{"executor", "caller", "sender", "calldata"} {
		t.Run(field, func(t *testing.T) {
			snapshot := Snapshot{Executor: common.Address{1}, Caller: common.Address{2}}
			sender, data := common.Address{3}, []byte{1, 2, 3, 4}
			switch field {
			case "executor":
				snapshot.Executor = common.Address{}
			case "caller":
				snapshot.Caller = common.Address{}
			case "sender":
				sender = common.Address{}
			case "calldata":
				data = data[:3]
			}
			if _, err := RouteExecution(t.Context(), nil, snapshot, sender, data); err == nil {
				t.Fatal("invalid route accepted")
			}
		})
	}
}
