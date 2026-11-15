package recipes

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	delegated "github.com/symbioticfi/merkle-executor/sdk/bindings/delegatedaccount"
)

func TestDelegatedMulticallPreservesTargetsOrderAndAtomicMode(t *testing.T) {
	first, _ := executorABI.TryPackExecute(nil, nil, nil, nil)
	second, _ := executorABI.TryPackExecute(nil, [][]byte{step(Add, pack(t, binaryArgs, literal(2), literal(3)))}, nil, nil)
	calls := []ExecutorCall{{Executor: common.Address{1}, Data: first}, {Executor: common.Address{2}, Data: second}}
	data, err := DelegatedMulticall(calls)
	if err != nil {
		t.Fatal(err)
	}
	method := delegated.NewDelegatedAccount().GetABI().Methods["execute"]
	if !bytes.Equal(data[:4], method.ID) {
		t.Fatal("wrong delegated entry point")
	}
	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		t.Fatal(err)
	}
	mode := args[0].([32]byte)
	if mode[0] != 1 || mode != ([32]byte{1}) {
		t.Fatal("batch must use atomic CALL execution mode")
	}
	executions, err := delegatedExecutions.Unpack(args[1].([]byte))
	if err != nil {
		t.Fatal(err)
	}
	decoded := *abi.ConvertType(executions[0], new([]delegatedCall)).(*[]delegatedCall)
	for i, call := range calls {
		if decoded[i].Target != call.Executor || decoded[i].Value.Cmp(new(big.Int)) != 0 || !bytes.Equal(decoded[i].CallData, call.Data) {
			t.Fatalf("execution %d changed", i)
		}
	}
	for _, bad := range [][]ExecutorCall{nil, {{Data: first}}, {{Executor: calls[0].Executor, Data: executorABI.PackSetCaller(common.Address{9})}}} {
		if _, err := DelegatedMulticall(bad); err == nil {
			t.Fatal("invalid executor batch admitted")
		}
	}
}
