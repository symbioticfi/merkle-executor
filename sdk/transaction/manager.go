// Package transaction submits one process-owned EIP-1559 transaction at a time.
// Restart loses ownership: hosts must use one signer owner and drain before restart.
package transaction

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/signer"
)

// Backend uses the sending endpoint for nonce methods, TransactionSenderBalanceAt
// and SendTransaction. Other reads may use a separate endpoint on the same chain.
type Backend interface {
	NonceAt(context.Context, common.Address, *big.Int) (uint64, error)
	PendingNonceAt(context.Context, common.Address) (uint64, error)
	TransactionSenderBalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error)
	FeeHistory(context.Context, uint64, *big.Int, []float64) (*ethereum.FeeHistory, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
	EstimateGas(context.Context, ethereum.CallMsg) (uint64, error)
	SendTransaction(context.Context, *types.Transaction) error
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
}

// DefaultMaxGas is a conservative deployment budget; hosts may override it for
// chains with different transaction limits.
const DefaultMaxGas = params.MaxTxGas

type Config struct {
	MaxFeeGwei    float64
	Confirmations uint64
	MaxGas        uint64 // Zero uses DefaultMaxGas.
}
type Request struct {
	To   common.Address
	Data []byte
}
type Result struct {
	Hash      common.Hash
	Execution *provider.Execution
	Reverted  bool
}

var (
	ErrBusy          = errors.New("transaction already active")
	ErrNonceConsumed = errors.New("transaction nonce consumed without an owned receipt")
)

// Manager has no worker or storage. The host schedules Poll until Busy is false.
// Signed work remains owned through every nonterminal error, including cancellation.
type Manager struct {
	mu                sync.Mutex
	backend           Backend
	signer            signer.Signer
	chainID, feeLimit *big.Int
	confirmations     uint64
	maxGas            uint64
	active            *types.Transaction
	nonceFloor        uint64
	log               logr.Logger
}

func New(backend Backend, s signer.Signer, chainID *big.Int, cfg Config, log logr.Logger) (*Manager, error) {
	if backend == nil || s == nil || s.Address() == (common.Address{}) || chainID == nil || chainID.Sign() <= 0 || chainID.BitLen() > 256 || cfg.Confirmations > 1024 {
		return nil, errors.New("backend, signer, positive chain ID and bounded confirmations required")
	}
	if cfg.MaxFeeGwei <= 0 || math.IsNaN(cfg.MaxFeeGwei) || math.IsInf(cfg.MaxFeeGwei, 0) {
		return nil, errors.New("finite positive maxFeeGwei required")
	}
	limit, _ := new(big.Float).Mul(big.NewFloat(cfg.MaxFeeGwei), big.NewFloat(1_000_000_000)).Int(nil)
	if limit.Sign() <= 0 || limit.BitLen() > 256 {
		return nil, errors.New("maxFeeGwei is outside the transaction fee range")
	}
	if cfg.MaxGas == 0 {
		cfg.MaxGas = DefaultMaxGas
	}
	return &Manager{backend: backend, signer: s, chainID: new(big.Int).Set(chainID), feeLimit: limit, confirmations: cfg.Confirmations, maxGas: cfg.MaxGas, log: log.WithName("transaction")}, nil
}

func (m *Manager) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active != nil
}

// Submit signs and sends once. A returned error may still leave Busy true;
// callers must Poll that same work instead of constructing another transaction.
func (m *Manager) Submit(ctx context.Context, req Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active != nil {
		return ErrBusy
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	if req.To == (common.Address{}) {
		return errors.New("nonzero transaction destination required")
	}
	req.Data = bytes.Clone(req.Data)
	gas, tip, fee, err := m.quote(ctx, req)
	if err != nil {
		return err
	}
	// Read after estimation, immediately before signing. Existing pending work is
	// never overwritten when this process has no corresponding signed identity.
	nonce, err := m.backend.NonceAt(ctx, m.signer.Address(), nil)
	if err != nil {
		return failure("read mined nonce", err)
	}
	pending, err := m.backend.PendingNonceAt(ctx, m.signer.Address())
	if err != nil {
		return failure("read pending nonce", err)
	}
	if nonce != pending || nonce < m.nonceFloor || nonce == math.MaxUint64 {
		return errors.New("signer has pending work or inconsistent nonce state")
	}
	unsigned := types.NewTx(&types.DynamicFeeTx{ChainID: m.chainID, Nonce: nonce, To: &req.To, Data: req.Data, Gas: gas, GasTipCap: tip, GasFeeCap: fee})
	if err := m.checkFunding(ctx, unsigned); err != nil {
		return err
	}
	signed, err := m.signer.SignTx(ctx, unsigned, new(big.Int).Set(m.chainID))
	if err != nil {
		return failure("sign transaction", err)
	}
	txSigner := types.LatestSignerForChainID(m.chainID)
	if signed == nil {
		return errors.New("signer returned no transaction")
	}
	sender, err := types.Sender(txSigner, signed)
	if err != nil || sender != m.signer.Address() || signed.Type() != types.DynamicFeeTxType || txSigner.Hash(signed) != txSigner.Hash(unsigned) {
		return errors.New("signer changed transaction payload")
	}
	// Ownership is published before any network mutation, even if ctx expired
	// during signing or the node accepts a broadcast whose response is lost.
	m.active = signed
	m.log.Info("transaction signed", "hash", signed.Hash().Hex(), "nonce", nonce)
	return m.broadcast(ctx)
}

func (m *Manager) checkFunding(ctx context.Context, tx *types.Transaction) error {
	cost := tx.Cost()
	if cost.Sign() < 0 || cost.BitLen() > 256 {
		return errors.New("invalid maximum transaction cost")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	balance, err := m.backend.TransactionSenderBalanceAt(ctx, m.signer.Address(), nil)
	if err != nil {
		return failure("read transaction sender balance", err)
	}
	if err := ctx.Err(); err != nil {
		return failure("read transaction sender balance", err)
	}
	if balance == nil || balance.Sign() < 0 || balance.BitLen() > 256 {
		return errors.New("invalid transaction sender balance")
	}
	if balance.Cmp(cost) < 0 {
		return errors.Errorf("insufficient transaction funding: balance %s, maximum cost %s", balance, cost)
	}
	return nil
}

func (m *Manager) quote(ctx context.Context, req Request) (uint64, *big.Int, *big.Int, error) {
	header, err := m.backend.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, nil, nil, failure("read fee header", err)
	}
	if header == nil || header.Number == nil || !header.Number.IsUint64() || header.BaseFee == nil || header.BaseFee.Sign() < 0 || header.BaseFee.BitLen() > 256 || header.GasLimit == 0 {
		return 0, nil, nil, errors.New("invalid EIP-1559 fee header")
	}
	history, err := m.backend.FeeHistory(ctx, 1, header.Number, []float64{50})
	if err != nil {
		return 0, nil, nil, failure("read transaction fee", err)
	}
	if history == nil || len(history.Reward) != 1 || len(history.Reward[0]) != 1 || history.Reward[0][0] == nil || history.Reward[0][0].Sign() < 0 || history.Reward[0][0].BitLen() > 256 {
		return 0, nil, nil, errors.New("invalid transaction fee reward")
	}
	tip := new(big.Int).Set(history.Reward[0][0])
	if tip.Sign() == 0 {
		tip.SetInt64(1)
	}
	minimum := new(big.Int).Add(header.BaseFee, tip)
	if minimum.Cmp(m.feeLimit) > 0 {
		return 0, nil, nil, errors.New("transaction fee ceiling exceeded")
	}
	fee := new(big.Int).Add(new(big.Int).Mul(header.BaseFee, big.NewInt(2)), tip)
	if fee.Cmp(m.feeLimit) > 0 {
		fee.Set(m.feeLimit)
	}
	gasLimit := min(m.maxGas, header.GasLimit)
	gas, err := m.backend.EstimateGas(ctx, ethereum.CallMsg{From: m.signer.Address(), To: &req.To, Data: req.Data, Gas: gasLimit, GasFeeCap: fee, GasTipCap: tip})
	if err != nil {
		return 0, nil, nil, failure("estimate transaction gas", err)
	}
	if gas == 0 || gas > gasLimit {
		return 0, nil, nil, errors.New("transaction gas exceeds budget or block limit")
	}
	margin := gas / 10
	if gas%10 != 0 {
		margin++
	}
	if gas > math.MaxUint64-margin {
		return 0, nil, nil, errors.New("transaction gas overflows")
	}
	gas = min(gas+margin, gasLimit)
	return gas, tip, fee, nil
}

func (m *Manager) broadcast(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := m.backend.SendTransaction(ctx, m.active)
	if err != nil {
		message := strings.ToLower(err.Error())
		if !strings.Contains(message, "already known") && !strings.Contains(message, "already imported") {
			return failure("broadcast transaction; ownership retained", err)
		}
	}
	return nil
}

// Poll returns terminal evidence once, clearing the active transaction only after
// canonical confirmation. Missing receipts and foreign nonce consumption retain it.
func (m *Manager) Poll(ctx context.Context) (*Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	receipt, err := m.backend.TransactionReceipt(ctx, m.active.Hash())
	if errors.Is(err, ethereum.NotFound) {
		nonce, nonceErr := m.backend.NonceAt(ctx, m.signer.Address(), nil)
		if nonceErr != nil {
			return nil, failure("read active nonce", nonceErr)
		}
		if nonce > m.active.Nonce() {
			// Inclusion can race the initial receipt read. Resolve the exact owned
			// hash again before classifying the advanced nonce as foreign work.
			receipt, err = m.backend.TransactionReceipt(ctx, m.active.Hash())
			if errors.Is(err, ethereum.NotFound) {
				return nil, ErrNonceConsumed
			}
		} else {
			broadcastErr := m.broadcast(ctx)
			if broadcastErr == nil || !nonceConflict(broadcastErr) {
				return nil, broadcastErr
			}
			// Inclusion can also race the nonce read and rebroadcast. A nonce
			// rejection is only a hint; terminal evidence still requires our receipt.
			receipt, err = m.backend.TransactionReceipt(ctx, m.active.Hash())
			if errors.Is(err, ethereum.NotFound) {
				nonce, nonceErr = m.backend.NonceAt(ctx, m.signer.Address(), nil)
				if nonceErr != nil {
					return nil, errors.Join(broadcastErr, failure("recheck active nonce", nonceErr))
				}
				if nonce > m.active.Nonce() {
					return nil, ErrNonceConsumed
				}
				return nil, broadcastErr
			}
			if err != nil {
				return nil, errors.Join(broadcastErr, failure("recheck transaction receipt", err))
			}
		}
	}
	if err != nil {
		return nil, failure("read transaction receipt", err)
	}
	if _, err := executionFromReceipt(receipt, m.active.Hash()); err != nil {
		return nil, err
	}
	receipt, err = confirmReceipt(ctx, m.backend, receipt, m.confirmations)
	if err != nil || receipt == nil {
		return nil, err
	}
	execution, err := executionFromReceipt(receipt, m.active.Hash())
	if err != nil {
		return nil, err
	}
	result := &Result{Hash: m.active.Hash(), Execution: execution, Reverted: receipt.Status == types.ReceiptStatusFailed}
	m.nonceFloor = m.active.Nonce() + 1
	m.active = nil
	m.log.Info("transaction confirmed", "hash", result.Hash.Hex(), "reverted", result.Reverted)
	return result, nil
}

func nonceConflict(err error) bool {
	var operation *operationError
	if errors.As(err, &operation) {
		err = operation.cause
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "nonce too low") || strings.Contains(message, "nonce is too low") || strings.Contains(message, "nonce has already been used")
}

func executionFromReceipt(receipt *types.Receipt, hash common.Hash) (*provider.Execution, error) {
	if receipt == nil || receipt.TxHash != hash || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockHash == (common.Hash{}) || receipt.Status > 1 {
		return nil, errors.New("invalid transaction receipt")
	}
	execution := &provider.Execution{TxHash: receipt.TxHash, BlockNumber: new(big.Int).Set(receipt.BlockNumber), BlockHash: receipt.BlockHash, Logs: make([]types.Log, len(receipt.Logs))}
	for i, log := range receipt.Logs {
		if log == nil || log.TxIndex != receipt.TransactionIndex {
			return nil, errors.New("invalid transaction receipt log")
		}
		execution.Logs[i] = *log
	}
	if err := execution.Validate(); err != nil {
		return nil, err
	}
	return execution.Clone(), nil
}

// Keep endpoint text out of host logs while preserving typed cancellation causes.
type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string { return e.operation + " failed" }
func (e *operationError) Unwrap() error { return e.cause }
func failure(operation string, err error) error {
	return &operationError{operation: operation, cause: err}
}
