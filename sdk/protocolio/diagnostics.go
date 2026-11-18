package protocolio

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"net"
	"strings"
	"syscall"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-errors/errors"
)

// CallFailure retains public call identity, never arguments, signatures or URLs.
func CallFailure(method string, call ethereum.CallMsg, block *big.Int, err error) error {
	if err == nil {
		return nil
	}
	return requestFailure(rpc.BatchElem{Method: method, Args: []any{CallArgument(call), BlockArgument(block)}}, err)
}

type requestError struct {
	details string
	cause   error
}

func (e *requestError) Error() string { return e.details + ": " + failureReason(e.cause) }
func (e *requestError) Unwrap() error { return e.cause }

func requestFailure(request rpc.BatchElem, err error) error {
	if err == nil {
		return nil
	}
	method := "unknown"
	switch request.Method {
	case "eth_call", "eth_estimateGas", "eth_chainId", "eth_getBlockByNumber", "eth_getBlockByHash", "eth_getLogs", "eth_getTransactionReceipt", "eth_getTransactionCount", "eth_sendRawTransaction", "eth_getBalance", "eth_feeHistory":
		method = request.Method
	}
	details := "method=" + method
	if (method == "eth_call" || method == "eth_estimateGas") && len(request.Args) > 0 {
		if call, ok := request.Args[0].(map[string]any); ok {
			if target, ok := call["to"].(*common.Address); ok && target != nil {
				details += " target=" + target.Hex()
			}
			if data, ok := call["input"].(hexutil.Bytes); ok && len(data) >= 4 {
				details += " selector=" + hexutil.Encode(data[:4])
			}
		}
		if len(request.Args) > 1 {
			if block, ok := request.Args[1].(string); ok {
				switch block {
				case "latest", "pending", "earliest", "safe", "finalized":
					details += " block=" + block
				default:
					if len(block) <= 66 {
						if _, err := hexutil.DecodeBig(block); err == nil {
							details += " block=" + block
						}
					}
				}
			}
		}
	}
	return &requestError{details: details, cause: err}
}

func batchFailure(batch []rpc.BatchElem, err error) error {
	if err == nil {
		return nil
	}
	if len(batch) == 1 {
		return requestFailure(batch[0], err)
	}
	return Failure(fmt.Sprintf("RPC batch request_count=%d", len(batch)), err)
}

// Only typed numeric fields and fixed classifications may cross the redaction
// boundary. Provider messages, ErrorData, HTTP status text/body and net.Error
// strings may contain credentials even when the endpoint itself is omitted.
func failureReason(err error) string {
	for _, sentinel := range []error{context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, sentinel) {
			return sentinel.Error()
		}
	}
	var status rpc.HTTPError
	if errors.As(err, &status) {
		return fmt.Sprintf("HTTP error http_status=%d", status.StatusCode)
	}
	var statusPointer *rpc.HTTPError
	if errors.As(err, &statusPointer) && statusPointer != nil {
		return fmt.Sprintf("HTTP error http_status=%d", statusPointer.StatusCode)
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "DNS error"
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return "network timeout"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection refused"
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return "connection reset"
	}
	classification := "unknown error"
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "tls:") {
		classification = "TLS error"
	}
	for _, sentinel := range []error{io.ErrUnexpectedEOF, io.EOF} {
		if errors.Is(err, sentinel) {
			return sentinel.Error()
		}
	}
	for _, kind := range []string{"nonce too low", "nonce is too low", "nonce has already been used", "replacement transaction underpriced", "already known", "already imported", "insufficient funds", "intrinsic gas too low", "invalid sender", "transaction type not supported", "execution reverted", "rate limit", "too many requests", "missing trie node", "header not found", "historical state", "method not found", "invalid params"} {
		if strings.Contains(message, kind) {
			classification = kind
			break
		}
	}
	var coded rpc.Error
	if errors.As(err, &coded) {
		return fmt.Sprintf("JSON-RPC error rpc_code=%d classification=%s", coded.ErrorCode(), classification)
	}
	if errors.Is(err, ethereum.NotFound) {
		return "not found"
	}
	if classification == "unknown error" && errors.As(err, &network) {
		return "network error"
	}
	return classification
}
