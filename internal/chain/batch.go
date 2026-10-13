package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
	multicall "github.com/symbioticfi/merkle-executor/api/bindings/multicall3"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
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

// batchRPC uses the same read HTTP client/redirect policy as ordinary reads.
// Unlike rpc.BatchCallContext it rejects duplicate and unknown response IDs.
func (c *Client) batchRPC(ctx context.Context, batch []rpc.BatchElem) error {
	if len(batch) == 0 {
		return nil
	}
	if len(batch) > protocolio.MaxReadBatch {
		return errors.New("RPC batch exceeds request limit")
	}
	messages := make([]batchMessage, len(batch))
	for i, element := range batch {
		messages[i] = batchMessage{JSONRPC: "2.0", ID: uint64(i + 1), Method: element.Method, Params: element.Args}
	}
	body, err := json.Marshal(messages)
	if err != nil {
		return rpcFailure("encode read batch", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.readURL, bytes.NewReader(body))
	if err != nil {
		return rpcFailure("create read batch", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
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

func (c *Client) BatchRead(ctx context.Context, requests []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	reader := protocolio.RPCBatchReader{Call: c.batchRPC, Multicall: c.multicall, Codec: readAggregateCodec{}}
	return reader.BatchRead(ctx, requests)
}

type readAggregateCodec struct{}

func (readAggregateCodec) Encode(calls []protocolio.AggregateCall) ([]byte, error) {
	subcalls := make([]multicall.Multicall3Call3, len(calls))
	for i, call := range calls {
		subcalls[i] = multicall.Multicall3Call3{Target: call.Target, AllowFailure: true, CallData: call.Data}
	}
	return multicall.NewMulticall3().TryPackAggregate3(subcalls)
}
func (readAggregateCodec) Decode(raw []byte) ([]protocolio.AggregateResult, error) {
	decoded, err := multicall.NewMulticall3().UnpackAggregate3(raw)
	if err != nil {
		return nil, err
	}
	results := make([]protocolio.AggregateResult, len(decoded))
	for i, result := range decoded {
		results[i] = protocolio.AggregateResult{Success: result.Success, Data: result.ReturnData}
	}
	return results, nil
}
