package recipes

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

//go:embed lens/BatchSimulation.json
var batchArtifact []byte

var batchArguments = abiArguments("bytes[]")

func abiArguments(types ...string) (args abi.Arguments) {
	for _, name := range types {
		args = append(args, abi.Argument{Type: mustType(name, nil)})
	}
	return args
}

// BatchReader adds original bytecode and eth_call-only code overrides to the
// ordinary pinned recipe reader. The chain client implements it directly.
type BatchReader interface {
	Reader
	provider.CodeOverrideReader
	CodeAt(context.Context, common.Address, *big.Int) ([]byte, error)
}

// A rejected extension does not invalidate its already simulated prefix.
// Transport and decoding failures must never be treated as an idle recipe.
type batchBoundary struct{ cause error }

func (e *batchBoundary) Error() string { return "candidate batch is not executable" }
func (e *batchBoundary) Unwrap() error { return e.cause }

func simulationFailure(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var rpcError interface{ ErrorCode() int }
	if errors.As(err, &rpcError) && rpcError.ErrorCode() == 3 {
		return &batchBoundary{err}
	}
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		message := strings.ToLower(cause.Error())
		for _, limit := range []string{"out of gas", "gas required exceeds allowance", "exceeds block gas limit"} {
			if strings.Contains(message, limit) {
				return &batchBoundary{err}
			}
		}
	}
	return errors.New("batch simulation RPC failed")
}

// Multicall preserves the order and original execute bytes. All entries execute
// on the same executor with the same caller; failure reverts the whole batch.
// It cannot encode management calls, nested batches or different recipes.
func Multicall(calls [][]byte) ([]byte, error) {
	if len(calls) == 0 {
		return nil, errors.New("executor batch requires at least one execution")
	}
	var hash common.Hash
	for i, call := range calls {
		recipe, _, err := decodeExecution(call)
		if err != nil {
			return nil, err
		}
		h, err := HashRecipe(recipe)
		if err != nil {
			return nil, err
		}
		if i > 0 && h != hash {
			return nil, errors.New("executor batch contains different recipes")
		}
		hash = h
	}
	return executorABI.GetABI().Pack("multicall", calls)
}

func decodeExecution(call []byte) (Recipe, []*big.Int, error) {
	method := executorABI.GetABI().Methods["execute"]
	if len(call) < 4 || !bytes.Equal(call[:4], method.ID) {
		return Recipe{}, nil, errors.New("expected executor execute calldata")
	}
	values, err := method.Inputs.Unpack(call[4:])
	if err != nil {
		return Recipe{}, nil, errors.New("invalid executor execute calldata")
	}
	r := Recipe{Queries: values[0].([][]byte), Steps: values[1].([][]byte)}
	inspection, err := Inspect(r)
	if err != nil {
		return Recipe{}, nil, err
	}
	if len(values[3].([][]byte)) != len(inspection.Slots) {
		return Recipe{}, nil, errors.New("execution runtime slot count mismatch")
	}
	return r, values[2].([]*big.Int), nil
}

func isMulticall(call []byte) bool {
	return len(call) >= 4 && bytes.Equal(call[:4], executorABI.GetABI().Methods["multicall"].ID)
}

func batchCalls(call []byte) ([][]byte, error) {
	decoded, err := executorABI.GetABI().Methods["multicall"].Inputs.Unpack(call[4:])
	if err != nil {
		return nil, errors.New("invalid executor multicall calldata")
	}
	calls := decoded[0].([][]byte)
	if _, err = Multicall(calls); err != nil {
		return nil, err
	}
	return calls, nil
}

// SimulateBatch obtains each execute result in one temporary EVM state. It also
// runs the exact multicall against the unchanged contract; only that calldata
// may be submitted. No helper contract is deployed on-chain.
func SimulateBatch(ctx context.Context, reader BatchReader, snapshot Snapshot, calls [][]byte, gas uint64) ([][]byte, error) {
	return simulateBatch(ctx, reader, snapshot, calls, gas, nil)
}

func originalExecutorRuntime(ctx context.Context, reader BatchReader, snapshot Snapshot) ([]byte, error) {
	code, err := reader.CodeAt(ctx, snapshot.Executor, new(big.Int).SetUint64(snapshot.BlockNumber))
	if err != nil {
		return nil, errors.New("read original executor runtime failed")
	}
	if len(code) == 0 || len(code) > 24576 {
		return nil, errors.New("invalid executor runtime size")
	}
	return code, nil
}

func simulateBatch(ctx context.Context, reader BatchReader, snapshot Snapshot, calls [][]byte, gas uint64, code []byte) ([][]byte, error) {
	calldata, err := Multicall(calls)
	if err != nil {
		return nil, err
	}
	if gas == 0 {
		return nil, errors.New("batch simulation requires a transaction gas budget")
	}
	for _, call := range calls {
		recipe, _, _ := decodeExecution(call)
		hash, _ := HashRecipe(recipe)
		if hash != snapshot.RecipeHash {
			return nil, errors.New("batch recipe differs from authenticated snapshot")
		}
	}
	block := new(big.Int).SetUint64(snapshot.BlockNumber)
	if code == nil {
		code, err = originalExecutorRuntime(ctx, reader, snapshot)
		if err != nil {
			return nil, err
		}
	}
	var artifact struct {
		Runtime hexutil.Bytes `json:"runtime"`
	}
	if err := json.Unmarshal(batchArtifact, &artifact); err != nil {
		return nil, err
	}
	args, err := abiArguments("bytes", "bytes[]").Pack(code, calls)
	if err != nil {
		return nil, err
	}
	data := append(crypto.Keccak256([]byte("simulate(bytes,bytes[])"))[:4], args...)
	// The probe creates one runtime copy. Its overhead is not part of the real
	// transaction budget, which the unchanged multicall enforces separately.
	const overhead = uint64(8_000_000)
	if gas > ^uint64(0)-overhead {
		return nil, errors.New("batch gas budget overflows probe allowance")
	}
	raw, err := reader.CallContractWithCode(ctx, ethereum.CallMsg{From: snapshot.Caller, To: &snapshot.Executor, Data: data, Gas: gas + overhead}, block, artifact.Runtime)
	if err != nil {
		return nil, simulationFailure(ctx, err)
	}
	values, err := batchArguments.Unpack(raw)
	if err != nil {
		return nil, errors.New("invalid stateful batch result")
	}
	results := values[0].([][]byte)
	if len(results) != len(calls) {
		return nil, errors.New("stateful batch result count mismatch")
	}
	for i, call := range calls {
		if _, err := HasWork(call, results[i]); err != nil {
			return nil, err
		}
	}
	if _, err := reader.CallContract(ctx, ethereum.CallMsg{From: snapshot.Caller, To: &snapshot.Executor, Data: calldata, Gas: gas}, block); err != nil {
		return nil, simulationFailure(ctx, err)
	}
	return results, nil
}

// BatchExecution extends a prepared execution while its live actions make
// progress and the complete transaction simulates. Runtime-input recipes remain
// single executions: external values cannot be reused after state changes.
// Exponential growth and bounded prefix search avoid replaying lengths 1..N.
// A reverted extension, gas limit or idle gate ends the batch; RPC failures fail
// preparation. The final selected bytes always receive an exact simulation.
func BatchExecution(ctx context.Context, reader BatchReader, snapshot Snapshot, call []byte, gas uint64) ([]byte, error) {
	recipe, inputs, err := decodeExecution(call)
	if err != nil {
		return nil, err
	}
	inspection, _ := Inspect(recipe)
	if len(inputs) != 0 || len(inspection.Inputs) != 0 {
		return bytes.Clone(call), nil
	}
	code, err := originalExecutorRuntime(ctx, reader, snapshot)
	if err != nil {
		return nil, err
	}
	finish := func(count int) ([]byte, error) {
		calls := repeatedCalls(call, count)
		data, err := compactBatch(calls)
		if err != nil {
			return nil, err
		}
		if _, err = reader.CallContract(ctx, ethereum.CallMsg{From: snapshot.Caller, To: &snapshot.Executor, Data: data, Gas: gas}, new(big.Int).SetUint64(snapshot.BlockNumber)); err != nil {
			return nil, simulationFailure(ctx, err)
		}
		return data, nil
	}
	count, good, upper := 1, 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		results, err := simulateBatch(ctx, reader, snapshot, repeatedCalls(call, count), gas, code)
		if err != nil {
			if !BatchRejected(err) || good == 0 {
				return nil, err
			}
			upper = count
		} else {
			selected, stopped, err := repetitionBoundary(call, results)
			if err != nil {
				return nil, err
			}
			if stopped {
				return finish(selected)
			}
			good = count
		}
		if upper != 0 {
			if upper-good == 1 {
				return finish(good)
			}
			count = good + (upper-good)/2
		} else {
			if good > int(^uint(0)>>1)/2 {
				return finish(good)
			}
			count = 2 * good
		}
	}
}

func repeatedCalls(call []byte, count int) [][]byte {
	calls := make([][]byte, count)
	for i := range calls {
		calls[i] = call
	}
	return calls
}

func repetitionBoundary(call []byte, results [][]byte) (int, bool, error) {
	seen := make(map[string]bool, len(results))
	for i, raw := range results {
		// Approval maintenance alone cannot justify another financial cycle.
		work, err := hasWork(call, raw, false)
		if err != nil {
			return 0, false, err
		}
		if !work {
			if i != 0 {
				return i, true, nil
			}
			if ordinary, err := HasWork(call, raw); err != nil {
				return 0, false, err
			} else if ordinary {
				return 1, true, nil
			}
			return 0, false, provider.ErrNotReady
		}
		if seen[string(raw)] {
			return i, true, nil
		}
		seen[string(raw)] = true
	}
	return len(results), false, nil
}

func compactBatch(calls [][]byte) ([]byte, error) {
	if len(calls) == 1 {
		return bytes.Clone(calls[0]), nil
	}
	return Multicall(calls)
}
