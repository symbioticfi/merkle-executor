// Package submission adds durable Merkle job ownership to a host transaction sender.
// The host owns fees, gas, nonce selection, replacement and shutdown. The SDK
// journals every signed variant before broadcast and releases a recipe only after
// canonical execution evidence and durable keeper acknowledgement.
package submission

import (
	"bytes"
	"context"
	"math/big"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// Backend supplies receipt and canonical ancestry reads only.
type Backend interface {
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
}
type Config struct {
	Confirmations uint64
	PollInterval  time.Duration
}
type Store interface {
	Load(string, any) (bool, error)
	Save(string, any) error
	Delete(string) error
}

// Transport belongs to the host. Persist MUST succeed for each signed variant
// before any broadcast. Hold reserves the host's shared signer lane until Release.
// Accepted work outlives the admission context and eventually returns one outcome.
type Transport interface {
	SendAsync(context.Context, Request, Persist) (<-chan Outcome, bool)
	Hold(string) error
	Release(string) error
	RestoreConfirmedNonce(uint64)
}
type Persist func(unsigned, signed *types.Transaction) error
type Outcome struct {
	Hash     common.Hash
	Attempts []common.Hash
	Rejected bool
	Err      error
}

type Request struct {
	ID   string
	To   common.Address
	Data []byte
}
type Result struct {
	ID                           string
	Hash                         common.Hash
	Attempts                     []common.Hash
	Confirmed, Reverted, Pending bool
	Execution                    *provider.Execution
}

var ErrLaneBusy = errors.New("transaction lane occupied")

const journalKey = "txmanager.active.v1"

type record struct {
	Version       int                 `json:"version"`
	ID            string              `json:"id"`
	ChainID       string              `json:"chainId"`
	Sender        common.Address      `json:"sender"`
	Raw           hexutil.Bytes       `json:"raw"`
	Attempts      []hexutil.Bytes     `json:"attempts,omitempty"`
	Confirmations uint64              `json:"confirmations"`
	Terminal      string              `json:"terminal,omitempty"`
	Execution     *provider.Execution `json:"execution,omitempty"`
}
type storageError struct{ err error }
type flight struct {
	done   chan struct{}
	result Outcome
}

// gate serializes callers; recordMu protects journal mutations from the engine's
// signing goroutine. Published records are immutable. Close drains the engine
// before the caller can close its state store or RPC clients.
type Manager struct {
	backend   Backend
	sender    common.Address
	chainID   *big.Int
	store     Store
	cfg       Config
	log       logr.Logger
	transport Transport
	gate      chan struct{}
	active    atomic.Pointer[record]
	admitted  atomic.Pointer[Request]
	fault     atomic.Pointer[storageError]
	recordMu  sync.Mutex
	intent    Request // set before admission; immutable while the engine owns work
	flight    *flight // gate protected
}

func New(backend Backend, sender common.Address, chainID *big.Int, store Store, cfg Config, transport Transport, log logr.Logger) (*Manager, error) {
	if backend == nil || sender == (common.Address{}) || store == nil || transport == nil || chainID == nil || chainID.Sign() <= 0 || cfg.Confirmations > 1024 {
		return nil, errors.New("backend, sender, positive chain ID, durable store, transport and bounded confirmations required")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	m := &Manager{backend: backend, sender: sender, chainID: new(big.Int).Set(chainID), store: store, cfg: cfg, transport: transport, log: log.WithName("txjournal"), gate: make(chan struct{}, 1)}
	var saved record
	found, err := store.Load(journalKey, &saved)
	if err != nil {
		return nil, err
	}
	if found {
		if _, err := m.decode(&saved); err != nil {
			return nil, err
		}
		if err := transport.Hold(saved.ID); err != nil {
			return nil, err
		}
		m.active.Store(&saved)
	}
	return m, nil
}

func (m *Manager) PendingID() string {
	if r := m.active.Load(); r != nil {
		return r.ID
	}
	if req := m.admitted.Load(); req != nil {
		return req.ID
	}
	return ""
}
func (m *Manager) storageFailure() error {
	if f := m.fault.Load(); f != nil {
		return f.err
	}
	return nil
}
func (m *Manager) failStorage(operation string, err error) error {
	wrapped := errors.Errorf("%s; broadcasting blocked: %w", operation, err)
	m.fault.Store(&storageError{wrapped})
	return wrapped
}

func (m *Manager) decode(r *record) ([]*types.Transaction, error) {
	if r.Confirmations > 1024 || r.Version != 1 || r.ID == "" || r.ChainID != m.chainID.String() || r.Sender != m.sender || (r.Terminal != "" && r.Terminal != "confirmed" && r.Terminal != "reverted") {
		return nil, errors.New("transaction journal identity mismatch")
	}
	raws := append([]hexutil.Bytes{r.Raw}, r.Attempts...)
	txs := make([]*types.Transaction, 0, len(raws))
	seen := map[common.Hash]bool{}
	for _, raw := range raws {
		var tx types.Transaction
		if err := tx.UnmarshalBinary(raw); err != nil {
			return nil, errors.Errorf("decode signed journal: %w", err)
		}
		sender, err := types.Sender(types.LatestSignerForChainID(m.chainID), &tx)
		if err != nil || sender != m.sender || tx.ChainId().Cmp(m.chainID) != 0 || tx.To() == nil || tx.Value().Sign() != 0 || tx.Type() != types.DynamicFeeTxType {
			return nil, errors.New("signed journal transaction mismatch")
		}
		if len(txs) > 0 {
			first := txs[0]
			if tx.Nonce() != first.Nonce() || *tx.To() != *first.To() || !bytes.Equal(tx.Data(), first.Data()) {
				return nil, errors.New("replacement changed recipe identity")
			}
		}
		if seen[tx.Hash()] {
			return nil, errors.New("duplicate signed journal attempt")
		}
		seen[tx.Hash()] = true
		txs = append(txs, &tx)
	}
	if r.Execution != nil {
		if err := r.Execution.Validate(); err != nil {
			return nil, err
		}
		if r.Terminal == "" || !seen[r.Execution.TxHash] {
			return nil, errors.New("journal execution does not match terminal transaction")
		}
	}
	if r.Terminal != "" && r.Execution == nil {
		return nil, errors.New("terminal journal has no execution evidence")
	}
	return txs, nil
}
func result(r *record) Result {
	res := Result{ID: r.ID, Pending: r.Terminal == "", Confirmed: r.Terminal == "confirmed", Reverted: r.Terminal == "reverted", Execution: r.Execution.Clone()}
	for _, raw := range append([]hexutil.Bytes{r.Raw}, r.Attempts...) {
		var tx types.Transaction
		if tx.UnmarshalBinary(raw) == nil {
			res.Attempts = append(res.Attempts, tx.Hash())
			res.Hash = tx.Hash()
		}
	}
	if res.Execution != nil {
		res.Hash = res.Execution.TxHash
	}
	return res
}
func (m *Manager) retainedResult(id string) Result {
	if r := m.active.Load(); r != nil {
		return result(r)
	}
	if req := m.admitted.Load(); req != nil {
		return Result{ID: req.ID, Pending: true}
	}
	return Result{ID: id}
}
func (m *Manager) Send(ctx context.Context, req Request) (Result, error) {
	select {
	case m.gate <- struct{}{}:
		defer func() { <-m.gate }()
	default:
		return m.retainedResult(req.ID), ErrLaneBusy
	}
	if err := m.storageFailure(); err != nil {
		return m.retainedResult(req.ID), err
	}
	if err := ctx.Err(); err != nil {
		return m.retainedResult(req.ID), err
	}
	if req.ID == "" || req.To == (common.Address{}) {
		return Result{}, errors.New("transaction request ID and destination required")
	}
	if m.flight != nil && (req.ID != m.intent.ID || req.To != m.intent.To || !bytes.Equal(req.Data, m.intent.Data)) {
		return m.retainedResult(req.ID), ErrLaneBusy
	}
	if r := m.active.Load(); r != nil {
		txs, err := m.decode(r)
		if err != nil {
			return result(r), err
		}
		if req.ID != r.ID || *txs[0].To() != req.To || !bytes.Equal(txs[0].Data(), req.Data) {
			return result(r), ErrLaneBusy
		}
		if r.Terminal != "" {
			return m.terminal(ctx, r, txs)
		}
	} else if m.flight == nil {
		if err := m.transport.Hold(req.ID); err != nil {
			return Result{ID: req.ID}, err
		}
		m.intent = Request{ID: req.ID, To: req.To, Data: bytes.Clone(req.Data)}
		channel, accepted := m.transport.SendAsync(ctx, Request{ID: m.intent.ID, To: m.intent.To, Data: bytes.Clone(m.intent.Data)}, m.recordSigned)
		if !accepted {
			if err := m.transport.Release(req.ID); err != nil {
				return Result{ID: req.ID}, err
			}
			return Result{ID: req.ID}, ErrLaneBusy
		}
		intent := m.intent
		m.admitted.Store(&intent)
		current := &flight{done: make(chan struct{})}
		m.flight = current
		go func() { current.result = <-channel; close(current.done) }()
	}
	// An admitted engine lifecycle continues beyond caller cancellation. A later
	// keeper tick joins it. Recovered journals have no flight and only read receipts.
	if m.flight != nil {
		select {
		case <-ctx.Done():
			return m.retainedResult(req.ID), ctx.Err()
		case <-m.flight.done:
		}
		outcome := m.flight.result
		m.flight = nil
		if err := m.storageFailure(); err != nil {
			return m.retainedResult(req.ID), err
		}
		if m.active.Load() == nil {
			if err := m.transport.Release(req.ID); err != nil {
				return m.retainedResult(req.ID), err
			}
			m.admitted.Store(nil)
			return Result{ID: req.ID}, outcome.Err
		}
		// A definite initial rejection returned no tracked hash; no bytes were
		// accepted. Ambiguous errors and nonce races always retain their evidence.
		if outcome.Rejected && len(outcome.Attempts) == 0 && outcome.Hash == (common.Hash{}) && definiteRejection(outcome.Err) {
			if err := m.store.Delete(journalKey); err != nil {
				return m.retainedResult(req.ID), m.failStorage("clear rejected transaction", err)
			}
			if err := m.transport.Release(req.ID); err != nil {
				return m.retainedResult(req.ID), err
			}
			m.active.Store(nil)
			m.admitted.Store(nil)
			return Result{ID: req.ID}, outcome.Err
		}
	}
	return m.wait(ctx)
}
func (m *Manager) wait(ctx context.Context) (Result, error) {
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()
	for {
		r := m.active.Load()
		txs, err := m.decode(r)
		if err != nil {
			return result(r), err
		}
		var sweepErr error
		for _, tx := range txs {
			res, _, err := m.receipt(ctx, r, tx)
			if res.Confirmed || res.Reverted {
				return res, nil
			}
			if err != nil && sweepErr == nil {
				sweepErr = err
			}
		}
		if sweepErr != nil {
			return m.retainedResult(r.ID), sweepErr
		}
		select {
		case <-ctx.Done():
			return m.retainedResult(r.ID), ctx.Err()
		case <-ticker.C:
		}
	}
}

// recordSigned runs in the host lifecycle before broadcast. The SDK never signs.
func (m *Manager) recordSigned(unsigned, signed *types.Transaction) error {
	if signed == nil || unsigned == nil {
		return errors.New("signed and unsigned transaction required")
	}
	txSigner := types.LatestSignerForChainID(m.chainID)
	sender, err := types.Sender(txSigner, signed)
	if err != nil || sender != m.sender || txSigner.Hash(signed) != txSigner.Hash(unsigned) || signed.ChainId().Cmp(m.chainID) != 0 || unsigned.To() == nil || *unsigned.To() != m.intent.To || !bytes.Equal(unsigned.Data(), m.intent.Data) {
		return errors.New("signer changed transaction payload")
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return errors.Errorf("encode signed transaction: %w", err)
	}
	m.recordMu.Lock()
	defer m.recordMu.Unlock()
	if err := m.storageFailure(); err != nil {
		return err
	}
	next := &record{Version: 1, ID: m.intent.ID, ChainID: m.chainID.String(), Sender: m.sender, Raw: raw, Confirmations: m.cfg.Confirmations}
	if old := m.active.Load(); old != nil {
		copy := *old
		next = &copy
		if old.Terminal != "" {
			return errors.New("recipe already has terminal evidence")
		}
		next.Attempts = append(append([]hexutil.Bytes(nil), old.Attempts...), raw)
	}
	if _, err := m.decode(next); err != nil {
		return err
	}
	m.active.Store(next)
	if err := m.store.Save(journalKey, next); err != nil {
		return m.failStorage("persist signed transaction", err)
	}
	return nil
}

// receipt requires the exact signed hash, its canonical block and the configured
// number of blocks after inclusion. Terminal progress is durable before return.
func (m *Manager) receipt(ctx context.Context, r *record, tx *types.Transaction) (Result, bool, error) {
	pending := result(r)
	pending.Confirmed, pending.Reverted, pending.Pending = false, false, true
	pending.Execution = nil
	receipt, err := m.backend.TransactionReceipt(ctx, tx.Hash())
	if errors.Is(err, ethereum.NotFound) {
		return pending, false, nil
	}
	if err != nil {
		return pending, false, errors.Errorf("read receipt: %w", err)
	}
	if receipt == nil || receipt.TxHash != tx.Hash() || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockHash == (common.Hash{}) || receipt.Status > 1 {
		return pending, false, errors.New("invalid transaction receipt")
	}
	receipt, err = ConfirmReceipt(ctx, m.backend, receipt, r.Confirmations)
	if err != nil {
		return pending, true, err
	}
	if receipt == nil {
		return pending, false, nil
	}
	if receipt.TxHash != tx.Hash() || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.Status > 1 {
		return pending, false, errors.New("invalid confirmed transaction receipt")
	}
	execution, err := executionFromReceipt(receipt)
	if err != nil {
		return pending, false, err
	}
	m.recordMu.Lock()
	defer m.recordMu.Unlock()
	final := *m.active.Load()
	final.Terminal = "confirmed"
	final.Execution = execution.Clone()
	if receipt.Status == types.ReceiptStatusFailed {
		final.Terminal = "reverted"
	}
	if err := m.store.Save(journalKey, &final); err != nil {
		return pending, true, m.failStorage("persist terminal receipt", err)
	}
	m.active.Store(&final)
	m.transport.RestoreConfirmedNonce(tx.Nonce())
	m.log.Info("transaction terminal", "id", r.ID, "hash", tx.Hash().Hex(), "outcome", final.Terminal)
	return result(&final), true, nil
}

// Acknowledge follows durable keeper progress, never uncertain execution.
func (m *Manager) Acknowledge(ctx context.Context, id string) error {
	select {
	case m.gate <- struct{}{}:
		defer func() { <-m.gate }()
	default:
		return ErrLaneBusy
	}
	if err := m.storageFailure(); err != nil {
		return err
	}
	r := m.active.Load()
	if r == nil {
		if m.admitted.Load() != nil {
			return ErrLaneBusy
		}
		return nil
	}
	if id != r.ID || r.Terminal == "" {
		return errors.New("only matching terminal transaction can be acknowledged")
	}
	txs, err := m.decode(r)
	if err != nil {
		return err
	}
	if _, err := m.terminal(ctx, r, txs); err != nil {
		return err
	}
	m.recordMu.Lock()
	defer m.recordMu.Unlock()
	if err := m.store.Delete(journalKey); err != nil {
		return m.failStorage("acknowledge transaction", err)
	}
	if err := m.transport.Release(id); err != nil {
		return m.failStorage("release acknowledged lane", err)
	}
	m.active.Store(nil)
	m.admitted.Store(nil)
	return nil
}

// terminal rechecks persisted finality before exposing evidence or releasing the
// host lane. Constructor state alone never advances the shared nonce floor.
func (m *Manager) terminal(ctx context.Context, r *record, txs []*types.Transaction) (Result, error) {
	pending := result(r)
	pending.Confirmed, pending.Reverted, pending.Pending = false, false, true
	pending.Execution = nil
	receipt, err := m.backend.TransactionReceipt(ctx, r.Execution.TxHash)
	if err != nil {
		return pending, err
	}
	if receipt == nil || receipt.TxHash != r.Execution.TxHash || receipt.BlockNumber == nil || receipt.Status > 1 {
		return pending, errors.New("invalid saved terminal receipt")
	}
	receipt, err = ConfirmReceipt(ctx, m.backend, receipt, r.Confirmations)
	if err != nil {
		return pending, err
	}
	if receipt == nil {
		return pending, errors.New("saved terminal receipt is no longer canonical and confirmed")
	}
	execution, err := executionFromReceipt(receipt)
	if err != nil {
		return pending, err
	}
	// Changed inclusion/status/logs cannot acknowledge business progress committed
	// for a previous execution. Retain the journal for explicit reconciliation.
	if (receipt.Status == types.ReceiptStatusSuccessful) != (r.Terminal == "confirmed") || !reflect.DeepEqual(execution.Clone(), r.Execution.Clone()) {
		return pending, errors.New("saved terminal execution changed; workflow reconciliation required")
	}
	for _, tx := range txs {
		if tx.Hash() == receipt.TxHash {
			m.transport.RestoreConfirmedNonce(tx.Nonce())
			return result(r), nil
		}
	}
	return pending, errors.New("terminal receipt has no owned signed attempt")
}
func executionFromReceipt(receipt *types.Receipt) (*provider.Execution, error) {
	if receipt == nil || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.Status > 1 {
		return nil, errors.New("invalid execution receipt")
	}
	execution := &provider.Execution{TxHash: receipt.TxHash, BlockNumber: new(big.Int).Set(receipt.BlockNumber), BlockHash: receipt.BlockHash, Logs: make([]types.Log, len(receipt.Logs))}
	for i, log := range receipt.Logs {
		if log == nil || log.TxIndex != receipt.TransactionIndex {
			return nil, errors.New("invalid receipt log")
		}
		execution.Logs[i] = *log
	}
	if err := execution.Validate(); err != nil {
		return nil, err
	}
	return execution, nil
}
