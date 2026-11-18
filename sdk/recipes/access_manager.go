package recipes

import (
	"bytes"
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/accessmanager"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// RouteExecution preserves the authenticated recipe payload and routes it through
// its on-chain caller when that caller is an AccessManager. Delayed grants cannot
// execute an atomic fill and are rejected; this never schedules an operation.
func RouteExecution(ctx context.Context, reader Reader, snapshot Snapshot, sender common.Address, data []byte) (ExecutorCall, error) {
	if snapshot.Executor == (common.Address{}) || snapshot.Caller == (common.Address{}) || sender == (common.Address{}) || len(data) < 4 {
		return ExecutorCall{}, errors.New("invalid executor routing identity or calldata")
	}
	call := ExecutorCall{Executor: snapshot.Executor, Data: bytes.Clone(data)}
	if sender == snapshot.Caller {
		return call, nil
	}
	dataToCheck := data
	if isMulticall(data) {
		entries, err := Executions(data)
		if err != nil {
			return ExecutorCall{}, err
		}
		dataToCheck = entries[0]
	}
	manager := accessmanager.NewAccessManager()
	raw, err := reader.CallContract(ctx, ethereum.CallMsg{
		To: &snapshot.Caller, Data: manager.PackCanCall(sender, snapshot.Executor, [4]byte(dataToCheck[:4])),
	}, new(big.Int).SetUint64(snapshot.BlockNumber))
	if err != nil {
		return ExecutorCall{}, errors.Errorf("read AccessManager caller permission: %w", err)
	}
	permission, err := manager.UnpackCanCall(raw)
	if err != nil {
		return ExecutorCall{}, errors.Errorf("decode AccessManager caller permission: %w", err)
	}
	if !permission.Immediate || permission.Delay != 0 {
		return ExecutorCall{}, errors.New("AccessManager caller lacks an immediate executor grant")
	}
	call.Executor = snapshot.Caller
	call.Data, err = AccessManagerCalldata(snapshot.Executor, data)
	return call, err
}

// AccessManagerCalldata preserves atomic same-executor batches using the
// manager's multicall of execute calls. Only the inner execute selector needs
// authorization; the executor's own multicall need not be granted separately.
func AccessManagerCalldata(executor common.Address, data []byte) ([]byte, error) {
	manager := accessmanager.NewAccessManager()
	if !isMulticall(data) {
		return manager.PackExecute(executor, data), nil
	}
	entries, err := Executions(data)
	if err != nil {
		return nil, err
	}
	forwarded := make([][]byte, len(entries))
	for i, entry := range entries {
		forwarded[i] = manager.PackExecute(executor, entry)
	}
	return manager.PackMulticall(forwarded), nil
}

func (s Snapshot) forwarded() bool {
	return s.Sender != (common.Address{}) && s.Sender != s.Caller
}

// callRouted simulates the exact envelope against unchanged code.
func callRouted(ctx context.Context, reader Reader, snapshot Snapshot, data []byte, gas uint64) error {
	from := snapshot.Caller
	to := snapshot.Executor
	if snapshot.forwarded() {
		route, err := RouteExecution(ctx, reader, snapshot, snapshot.Sender, data)
		if err != nil {
			return &MemberError{Index: 0, Err: err}
		}
		from, to, data = snapshot.Sender, route.Executor, route.Data
	}
	if _, err := reader.CallContract(ctx, ethereum.CallMsg{From: from, To: &to, Data: data, Gas: gas}, new(big.Int).SetUint64(snapshot.BlockNumber)); err != nil {
		return simulationFailure(ctx, err)
	}
	return nil
}

var simulateCallsSelector = crypto.Keccak256([]byte("simulateCalls((address,bytes)[])"))[:4]

func simulateCallerCalls(ctx context.Context, reader provider.Reader, sender, caller common.Address, calls []ExecutorCall, block *big.Int, gas uint64, overrides map[common.Address][]byte) ([][]byte, error) {
	type simulationCall struct {
		Executor common.Address
		Data     []byte
	}
	encoded := make([]simulationCall, len(calls))
	for i, call := range calls {
		encoded[i] = simulationCall{call.Executor, call.Data}
	}
	args, err := executorCalls.Pack(encoded)
	if err != nil {
		return nil, err
	}
	msg := ethereum.CallMsg{From: sender, To: &caller, Data: append(bytes.Clone(simulateCallsSelector), args...), Gas: gas}
	var raw []byte
	if len(overrides) == 0 {
		override, ok := reader.(provider.CodeOverrideReader)
		if !ok {
			return nil, errors.New("caller simulation requires RPC code-override support")
		}
		raw, err = override.CallContractWithCode(ctx, msg, block, batchRuntime)
	} else {
		override, ok := reader.(provider.CodeOverridesReader)
		if !ok {
			return nil, errors.New("forwarded prefix requires RPC code overrides at multiple addresses")
		}
		overrides[caller] = batchRuntime
		raw, err = override.CallContractWithCodeOverrides(ctx, msg, block, overrides)
	}
	if err != nil {
		return nil, simulationFailure(ctx, err)
	}
	values, err := batchArguments.Unpack(raw)
	if err != nil {
		return nil, errors.New("invalid caller simulation result")
	}
	results := values[0].([][]byte)
	if len(results) != len(calls) {
		return nil, errors.New("caller simulation result count mismatch")
	}
	return results, nil
}

func callPrefix(ctx context.Context, reader provider.Reader, snapshot Snapshot, data, code []byte) ([]byte, error) {
	const gas = 30_000_000
	block := new(big.Int).SetUint64(snapshot.BlockNumber)
	if snapshot.forwarded() {
		if snapshot.Executor == snapshot.Caller {
			return nil, errors.New("forwarded prefix requires distinct executor and caller")
		}
		results, err := simulateCallerCalls(ctx, reader, snapshot.Sender, snapshot.Caller, []ExecutorCall{{snapshot.Executor, data}}, block, gas, map[common.Address][]byte{snapshot.Executor: code})
		if err != nil {
			return nil, err
		}
		return results[0], nil
	}
	override, ok := reader.(provider.CodeOverrideReader)
	if !ok {
		return nil, errors.New("recipe prefix requires RPC code-override support")
	}
	raw, err := override.CallContractWithCode(ctx, ethereum.CallMsg{From: snapshot.Caller, To: &snapshot.Executor, Data: data, Gas: gas}, block, code)
	if err != nil {
		return nil, simulationFailure(ctx, err)
	}
	return raw, nil
}

// simulateForwarded checks the real manager first, then collects the results its
// execute ABI omits. The probe uses the original executor runtime and call order.
func simulateForwarded(ctx context.Context, reader Reader, snapshot Snapshot, data []byte, entries [][]byte, gas uint64) ([][]byte, error) {
	if err := callRouted(ctx, reader, snapshot, data, gas); err != nil {
		return nil, err
	}
	calls := make([]ExecutorCall, len(entries))
	for i, entry := range entries {
		calls[i] = ExecutorCall{snapshot.Executor, entry}
	}
	return simulateCallerCalls(ctx, reader, snapshot.Sender, snapshot.Caller, calls, new(big.Int).SetUint64(snapshot.BlockNumber), gas, nil)
}
