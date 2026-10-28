package protocolio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
)

// Batch responses are bounded independently of request count. Oversized or
// structurally incomplete responses invalidate the group, never trigger retries.
const maxBatchResponseBytes = 32 << 20

type batchMessage struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}
type batchResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *batchRPCError  `json:"error"`
}
type batchRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func (e *batchRPCError) Error() string  { return e.Message }
func (e *batchRPCError) ErrorCode() int { return e.Code }
func (e *batchRPCError) ErrorData() any { return e.Data }

// CallHTTPBatch sends one bounded read array using the supplied host transport.
// It rejects duplicate, missing and unknown response IDs before decoding results.
func CallHTTPBatch(ctx context.Context, client *http.Client, endpoint string, batch []rpc.BatchElem) error {
	if len(batch) == 0 {
		return nil
	}
	if client == nil {
		return errors.New("RPC HTTP client required")
	}
	if len(batch) > MaxReadBatch {
		return errors.New("RPC batch exceeds request limit")
	}
	messages := make([]batchMessage, len(batch))
	for i, element := range batch {
		if element.Method == "eth_getLogs" {
			return ErrLogScan
		}
		messages[i] = batchMessage{JSONRPC: "2.0", ID: uint64(i + 1), Method: element.Method, Params: element.Args}
	}
	body, err := json.Marshal(messages)
	if err != nil {
		return rpcFailure("encode read batch", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return rpcFailure("create read batch", err)
	}
	request.Header.Set("Content-Type", "application/json")
	bounded := *client
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := bounded.Do(request)
	if err != nil {
		return rpcFailure("read RPC batch", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return rpcFailure("read RPC batch HTTP status", rpc.HTTPError{StatusCode: response.StatusCode, Status: response.Status})
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxBatchResponseBytes+1))
	if err != nil {
		return rpcFailure("read RPC batch response", err)
	}
	if len(raw) > maxBatchResponseBytes {
		return errors.New("read RPC batch response exceeds limit")
	}
	var responses []batchResponse
	if err := json.Unmarshal(raw, &responses); err != nil {
		return errors.New("invalid RPC batch response")
	}
	if len(responses) != len(batch) {
		return errors.New("RPC batch response count mismatch")
	}
	seen := make([]bool, len(batch))
	for _, response := range responses {
		if response.JSONRPC != "2.0" || response.ID == 0 || response.ID > uint64(len(batch)) || seen[response.ID-1] {
			return errors.New("invalid or duplicate RPC batch response ID")
		}
		i := int(response.ID - 1)
		seen[i] = true
		if response.Error != nil {
			if len(response.Result) > 0 && !bytes.Equal(response.Result, []byte("null")) {
				return errors.New("RPC batch response has both result and error")
			}
			batch[i].Error = rpcFailure("read batch member", response.Error)
		} else if len(response.Result) == 0 {
			return errors.New("RPC batch member result missing")
		} else if bytes.Equal(response.Result, []byte("null")) {
			if batch[i].Method == "eth_getBlockByNumber" || batch[i].Method == "eth_getTransactionReceipt" {
				batch[i].Error = rpcFailure("read batch member", ethereum.NotFound)
			} else {
				batch[i].Error = errors.New("RPC batch member returned null")
			}
		} else {
			batch[i].Error = rpcFailure("decode read batch member", json.Unmarshal(response.Result, batch[i].Result))
		}
	}
	return nil
}

func (r RPCBatchReader) BatchRead(ctx context.Context, requests []ReadRequest) ([]ReadResult, error) {
	for _, request := range requests {
		if request.Kind == ReadLogs {
			return nil, ErrLogScan
		}
	}
	if r.Call == nil {
		return nil, errors.New("RPC batch transport required")
	}
	if r.Multicall != (common.Address{}) && r.Codec == nil {
		return nil, errors.New("configured aggregate requires a codec")
	}
	results := make([]ReadResult, 0, len(requests))
	for start := 0; start < len(requests); start += MaxReadBatch {
		end := min(start+MaxReadBatch, len(requests))
		chunk, err := r.readChunk(ctx, requests[start:end])
		if err != nil {
			return nil, err
		}
		results = append(results, chunk...)
	}
	return results, nil
}

// A compatible safe subset becomes one aggregate eth_call inside the same HTTP
// array as any isolated calls. Grouping never crosses a pinned block boundary.
func (r RPCBatchReader) readChunk(ctx context.Context, requests []ReadRequest) ([]ReadResult, error) {
	results := make([]ReadResult, len(requests))
	safe := map[string][]int{}
	if r.Multicall != (common.Address{}) && r.Codec != nil {
		for i, request := range requests {
			if canAggregate(request) {
				key := BlockArgument(request.Block)
				safe[key] = append(safe[key], i)
			}
		}
	}
	type operation struct {
		indices   []int
		aggregate bool
		data      *hexutil.Bytes
	}
	operations := make([]operation, 0, len(requests))
	calls := make([]rpc.BatchElem, 0, len(requests))
	aggregated := make([]bool, len(requests))
	for i, request := range requests {
		if aggregated[i] {
			continue
		}
		members := safe[BlockArgument(request.Block)]
		if canAggregate(request) && len(members) > 1 {
			subcalls := make([]AggregateCall, len(members))
			for j, index := range members {
				subcalls[j] = AggregateCall{Target: *requests[index].Call.To, Data: requests[index].Call.Data}
				aggregated[index] = true
			}
			payload, err := r.Codec.Encode(subcalls)
			if err != nil {
				return nil, errors.New("encode safe read aggregate")
			}
			raw := new(hexutil.Bytes)
			calls = append(calls, rpc.BatchElem{Method: "eth_call", Args: []any{CallArgument(ethereum.CallMsg{To: &r.Multicall, Data: payload}), BlockArgument(request.Block)}, Result: raw})
			operations = append(operations, operation{indices: members, aggregate: true, data: raw})
			continue
		}
		op := operation{indices: []int{i}}
		var call rpc.BatchElem
		switch request.Kind {
		case ReadCall:
			op.data = new(hexutil.Bytes)
			call = rpc.BatchElem{Method: "eth_call", Args: []any{CallArgument(request.Call), BlockArgument(request.Block)}, Result: op.data}
		case ReadHeader:
			call = rpc.BatchElem{Method: "eth_getBlockByNumber", Args: []any{BlockArgument(request.Block), false}, Result: &results[i].Header}
		case ReadLogs:
			return nil, ErrLogScan
		case ReadReceipt:
			call = rpc.BatchElem{Method: "eth_getTransactionReceipt", Args: []any{request.Hash}, Result: &results[i].Receipt}
		default:
			return nil, errors.New("invalid read batch request kind")
		}
		calls = append(calls, call)
		operations = append(operations, op)
	}
	if err := r.Call(ctx, calls); err != nil {
		return nil, err
	}
	for i, op := range operations {
		if calls[i].Error != nil {
			for _, index := range op.indices {
				results[index].Err = calls[i].Error
			}
			continue
		}
		if op.aggregate {
			decoded, err := r.Codec.Decode(*op.data)
			if err != nil || len(decoded) != len(op.indices) {
				return nil, errors.New("invalid safe read aggregate result")
			}
			for j, index := range op.indices {
				if !decoded[j].Success {
					results[index].Err = errors.New("safe read aggregate getter reverted")
				} else {
					results[index].Data = decoded[j].Data
				}
			}
		} else if op.data != nil {
			results[op.indices[0]].Data = *op.data
		}
	}
	return results, nil
}
func canAggregate(request ReadRequest) bool {
	call := request.Call
	return request.Kind == ReadCall && request.MulticallSafe && request.Block != nil && request.Block.Sign() >= 0 && call.To != nil && call.From == (common.Address{}) && call.Gas == 0 && call.Value == nil && call.GasPrice == nil && call.GasFeeCap == nil && call.GasTipCap == nil && call.AccessList == nil && call.BlobGasFeeCap == nil && call.BlobHashes == nil && call.AuthorizationList == nil
}

// BlockArgument preserves exact pinned heights and special block tags.
func BlockArgument(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	if number.IsInt64() && number.Sign() < 0 {
		return rpc.BlockNumber(number.Int64()).String()
	}
	return hexutil.EncodeBig(number)
}

// CallArgument matches ethclient's complete CallMsg encoding, including zero From, typed
// transactions and authorizations. Unknown contracts are isolated eth_calls.
func CallArgument(msg ethereum.CallMsg) map[string]any {
	arg := map[string]any{"from": msg.From, "to": msg.To}
	if len(msg.Data) > 0 {
		arg["input"] = hexutil.Bytes(msg.Data)
	}
	if msg.Value != nil {
		arg["value"] = (*hexutil.Big)(msg.Value)
	}
	if msg.Gas != 0 {
		arg["gas"] = hexutil.Uint64(msg.Gas)
	}
	if msg.GasPrice != nil {
		arg["gasPrice"] = (*hexutil.Big)(msg.GasPrice)
	}
	if msg.GasFeeCap != nil {
		arg["maxFeePerGas"] = (*hexutil.Big)(msg.GasFeeCap)
	}
	if msg.GasTipCap != nil {
		arg["maxPriorityFeePerGas"] = (*hexutil.Big)(msg.GasTipCap)
	}
	if msg.AccessList != nil {
		arg["accessList"] = msg.AccessList
	}
	if msg.BlobGasFeeCap != nil {
		arg["maxFeePerBlobGas"] = (*hexutil.Big)(msg.BlobGasFeeCap)
	}
	if msg.BlobHashes != nil {
		arg["blobVersionedHashes"] = msg.BlobHashes
	}
	if msg.AuthorizationList != nil {
		arg["authorizationList"] = msg.AuthorizationList
	}
	return arg
}

// RPCBatchCall sends one bounded array and reports each member in its own Result.
type RPCBatchCall func(context.Context, []rpc.BatchElem) error

// AggregateCall and AggregateResult keep ABI generation in the host codec.
type AggregateCall struct {
	Target common.Address
	Data   []byte
}
type AggregateResult struct {
	Success bool
	Data    []byte
}
type AggregateCodec interface {
	Encode([]AggregateCall) ([]byte, error)
	Decode([]byte) ([]AggregateResult, error)
}

// RPCBatchReader groups only source-reviewed, caller-independent getters. The
// host supplies its existing read transport and generated aggregate binding.
type RPCBatchReader struct {
	Call      RPCBatchCall
	Multicall common.Address
	Codec     AggregateCodec
}

type rpcReadError struct {
	operation string
	cause     error
}

func (e *rpcReadError) Error() string { return e.operation + " failed" }
func (e *rpcReadError) Unwrap() error { return e.cause }
func rpcFailure(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &rpcReadError{operation: operation, cause: err}
}
