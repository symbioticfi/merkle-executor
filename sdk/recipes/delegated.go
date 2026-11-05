package recipes

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	delegated "github.com/symbioticfi/merkle-executor/sdk/bindings/delegatedaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// ExecutorCall keeps the committed execution attached to its own executor.
type ExecutorCall struct {
	Executor common.Address `json:"executor"`
	Data     hexutil.Bytes  `json:"data"`
}

type delegatedCall struct {
	Target   common.Address
	Value    *big.Int
	CallData []byte
}

var delegatedABI = delegated.NewDelegatedAccount()
var atomicBatchMode = [32]byte{1}
var delegatedExecutions = abi.Arguments{{Type: mustType("tuple[]", []abi.ArgumentMarshaling{{Name: "target", Type: "address"}, {Name: "value", Type: "uint256"}, {Name: "callData", Type: "bytes"}})}}
var executorCalls = abi.Arguments{{Type: mustType("tuple[]", []abi.ArgumentMarshaling{{Name: "executor", Type: "address"}, {Name: "data", Type: "bytes"}})}}

// DelegatedMulticall uses ERC-7579's atomic batch mode, supported by MetaMask's
// EIP-7702 account. No allowance, recipe management or arbitrary call is accepted.
func DelegatedMulticall(calls []ExecutorCall) ([]byte, error) {
	if len(calls) == 0 {
		return nil, errors.New("delegated batch requires at least one executor")
	}
	encoded := make([]delegatedCall, len(calls))
	for i, call := range calls {
		if call.Executor == (common.Address{}) {
			return nil, errors.New("delegated batch executor is zero")
		}
		if _, err := Executions(call.Data); err != nil {
			return nil, err
		}
		encoded[i] = delegatedCall{Target: call.Executor, Value: new(big.Int), CallData: bytes.Clone(call.Data)}
	}
	payload, err := delegatedExecutions.Pack(encoded)
	if err != nil {
		return nil, err
	}
	return delegatedABI.TryPackExecute(atomicBatchMode, payload)
}

// Executions validates and unwraps an executor's execute or homogeneous multicall.
func Executions(data []byte) ([][]byte, error) {
	if isMulticall(data) {
		return batchCalls(data)
	}
	if _, _, err := decodeExecution(data); err != nil {
		return nil, err
	}
	return [][]byte{bytes.Clone(data)}, nil
}

// SupportsDelegatedBatch detects the installed EIP-7702 account without changing
// its delegation. A delegated account that lacks atomic batches is an error.
func SupportsDelegatedBatch(ctx context.Context, reader BatchReader, caller common.Address, block *big.Int) (bool, error) {
	code, err := reader.CodeAt(ctx, caller, block)
	if err != nil {
		return false, errors.Errorf("read keeper delegation: %w", err)
	}
	implementation, ok := types.ParseDelegation(code)
	if !ok {
		return false, nil
	}
	if implementation == (common.Address{}) {
		return false, errors.New("keeper delegation has no implementation")
	}
	result, err := reader.CallContract(ctx, ethereum.CallMsg{From: caller, To: &caller, Data: delegatedABI.PackSupportsExecutionMode(atomicBatchMode)}, block)
	if err != nil {
		return false, errors.Errorf("read keeper atomic batch mode: %w", err)
	}
	supported, err := delegatedABI.UnpackSupportsExecutionMode(result)
	if err != nil || !supported {
		return false, errors.New("keeper does not support atomic delegated batches")
	}
	return true, nil
}

// SimulateExecutors returns every execution's values after its predecessors.
// The final check executes the exact envelope through the installed delegation;
// replacement code is used only to inspect values omitted by ERC-7579 execute.
func SimulateExecutors(ctx context.Context, reader BatchReader, caller common.Address, calls []ExecutorCall, block *big.Int, gas uint64) ([][]byte, error) {
	calldata, err := DelegatedMulticall(calls)
	if err != nil {
		return nil, err
	}
	if block == nil || block.Sign() < 0 || gas == 0 || caller == (common.Address{}) {
		return nil, errors.New("delegated simulation requires caller, pinned block and gas budget")
	}
	supported, err := SupportsDelegatedBatch(ctx, reader, caller, block)
	if err != nil {
		return nil, err
	}
	if !supported {
		return nil, errors.New("keeper has no EIP-7702 delegation")
	}
	if _, err = reader.CallContract(ctx, ethereum.CallMsg{From: caller, To: &caller, Data: calldata, Gas: gas}, block); err != nil {
		return nil, simulationFailure(ctx, err)
	}
	var flat []ExecutorCall
	lengths := make([]int, len(calls))
	for i, call := range calls {
		entries, _ := Executions(call.Data)
		lengths[i] = len(entries)
		for _, data := range entries {
			flat = append(flat, ExecutorCall{Executor: call.Executor, Data: data})
		}
	}
	var artifact struct {
		Runtime hexutil.Bytes `json:"runtime"`
	}
	if err := json.Unmarshal(batchArtifact, &artifact); err != nil {
		return nil, err
	}
	probeCalls := make([]struct {
		Executor common.Address
		Data     []byte
	}, len(flat))
	for i, call := range flat {
		probeCalls[i].Executor, probeCalls[i].Data = call.Executor, bytes.Clone(call.Data)
	}
	args, err := executorCalls.Pack(probeCalls)
	if err != nil {
		return nil, err
	}
	data := append(crypto.Keccak256([]byte("simulateCalls((address,bytes)[])"))[:4], args...)
	raw, err := reader.CallContractWithCode(ctx, ethereum.CallMsg{From: caller, To: &caller, Data: data, Gas: gas}, block, artifact.Runtime)
	if err != nil {
		return nil, simulationFailure(ctx, err)
	}
	values, err := batchArguments.Unpack(raw)
	if err != nil {
		return nil, errors.New("invalid delegated simulation result")
	}
	results := values[0].([][]byte)
	if len(results) != len(flat) {
		return nil, errors.New("delegated simulation result count mismatch")
	}
	grouped := make([][]byte, len(calls))
	index := 0
	for i, count := range lengths {
		for j := 0; j < count; j++ {
			if _, err := HasWork(flat[index+j].Data, results[index+j]); err != nil {
				return nil, err
			}
		}
		grouped[i] = bytes.Clone(results[index])
		if isMulticall(calls[i].Data) {
			grouped[i], err = batchArguments.Pack(results[index : index+count])
			if err != nil {
				return nil, err
			}
		}
		index += count
	}
	return grouped, nil
}

// BatchRejected distinguishes a contract/gas rejection from transport failure.
func BatchRejected(err error) bool {
	var boundary *batchBoundary
	return errors.As(err, &boundary)
}

// ValidateExecutorBatch authenticates every member against the same current
// block, then verifies the complete execution and closes all canonical pins.
func ValidateExecutorBatch(ctx context.Context, reader BatchReader, caller common.Address, snapshots []Snapshot, calls []ExecutorCall, witnesses []map[uint]string) error {
	if len(snapshots) != len(calls) || len(witnesses) != len(calls) || len(calls) == 0 {
		return errors.New("executor batch member count mismatch")
	}
	current, err := reader.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	if err = validateHeader(current, nil); err != nil {
		return err
	}
	for i, snapshot := range snapshots {
		if snapshot.Executor != calls[i].Executor || snapshot.Caller != caller || current.Number.Uint64() < snapshot.BlockNumber {
			return errors.New("executor batch identity mismatch")
		}
		if err = closeExecution(ctx, reader, snapshot, current); err != nil {
			return err
		}
		reads, e := protocolio.ReadGroup(ctx, reader, snapshotRequests(snapshot.Executor, snapshot.Account, current.Number)...)
		if e != nil {
			return e
		}
		if err = validateCurrent(snapshot, current, reads, caller); err != nil {
			return err
		}
	}
	results, err := SimulateExecutors(ctx, reader, caller, calls, current.Number, current.GasLimit)
	if err != nil {
		return err
	}
	for i, call := range calls {
		if len(witnesses[i]) != 0 {
			if err = VerifyResults(call.Data, results[i], witnesses[i]); err != nil {
				return err
			}
		}
		work, e := HasWork(call.Data, results[i])
		if e != nil {
			return e
		}
		if !work {
			return provider.ErrNotReady
		}
	}
	for _, snapshot := range snapshots {
		if err = closeExecution(ctx, reader, snapshot, current); err != nil {
			return err
		}
	}
	return nil
}
