package recipes

import (
	"bytes"
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/accessmanager"
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
