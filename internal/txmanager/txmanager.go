// Package txmanager adapts the vault-solver engine to durable Merkle job ownership.
// The engine owns fees, gas, nonce selection, replacement and shutdown. The adapter
// journals every signed variant before broadcast and releases a recipe only after
// canonical execution evidence and durable keeper acknowledgement.
package txmanager

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/signer"
	"github.com/symbioticfi/merkle-executor/internal/state"
	engine "github.com/symbioticfi/merkle-executor/internal/vaultsolver/txmanager"
)

type Backend interface{ engine.Backend }
type Config = engine.Config

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
var ErrNonceConflict = engine.ErrNonceConflict

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
	result engine.Result
}

// gate serializes callers; recordMu protects journal mutations from the engine's
// signing goroutine. Published records are immutable. Close drains the engine
// before the caller can close its state store or RPC clients.
type Manager struct {
	backend   Backend
	signer    signer.Signer
	chainID   *big.Int
	store     *state.Store
	cfg       Config
	log       logr.Logger
	engine    *engine.Manager
	gate      chan struct{}
	active    atomic.Pointer[record]
	admitted  atomic.Pointer[Request]
	fault     atomic.Pointer[storageError]
	recordMu  sync.Mutex
	intent    Request // set before admission; immutable while the engine owns work
	flight    *flight // gate protected
	startOnce sync.Once
	stop      context.CancelFunc
	done      chan struct{}
}

func New(backend Backend, s signer.Signer, chainID *big.Int, store *state.Store, cfg Config, log logr.Logger) (*Manager, error) {
	if backend == nil || s == nil || store == nil || chainID == nil || chainID.Sign() <= 0 {
		return nil, errors.New("backend, signer, positive chain ID and durable store required")
	}
	if cfg.MaxFeeGwei <= 0 || math.IsNaN(cfg.MaxFeeGwei) || math.IsInf(cfg.MaxFeeGwei, 0) {
		return nil, errors.New("finite positive maxFeeGwei required")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	m := &Manager{backend: backend, signer: s, chainID: new(big.Int).Set(chainID), store: store, cfg: cfg, log: log.WithName("txjournal"), gate: make(chan struct{}, 1), done: make(chan struct{})}
	var saved record
	found, err := store.Load(journalKey, &saved)
	if err != nil {
		return nil, err
	}
	if found {
		if _, err := m.decode(&saved); err != nil {
			return nil, err
		}
		m.active.Store(&saved)
	}
	m.engine = engine.New(backend, &journalSigner{m}, m.chainID, cfg, log)
	m.restoreFloor()
	if err := m.engine.ValidateFeeHeadroom(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) restoreFloor() {
	if r := m.active.Load(); r != nil && r.Terminal != "" {
		if txs, err := m.decode(r); err == nil {
			m.engine.RestoreConfirmedNonce(txs[0].Nonce())
		}
	}
}

// NewWithMetrics exposes the same bounded transaction telemetry as vault-solver.
func NewWithMetrics(backend Backend, s signer.Signer, chainID *big.Int, store *state.Store, cfg Config, log logr.Logger, registry prometheus.Registerer) (*Manager, error) {
	m, err := New(backend, s, chainID, store, cfg, log)
	if err != nil {
		return nil, err
	}
	metrics, err := engine.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	m.engine = engine.NewWithMetrics(backend, &journalSigner{m}, m.chainID, cfg, metrics, log)
	m.restoreFloor()
	return m, nil
}

// Start initializes RPC readiness before launching the worker. Call Close before
// closing the durable store. The CLI supplies its process context, not a tick.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.engine.Initialize(ctx); err != nil {
		return err
	}
	m.startOnce.Do(func() {
		var worker context.Context
		worker, m.stop = context.WithCancel(ctx)
		go func() { defer close(m.done); m.engine.Start(worker) }()
	})
	return nil
}
func (m *Manager) Close() {
	if m.stop != nil {
		m.stop()
		<-m.done
	}
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
	if r.Confirmations > 1024 || r.Version != 1 || r.ID == "" || r.ChainID != m.chainID.String() || r.Sender != m.signer.Address() || (r.Terminal != "" && r.Terminal != "confirmed" && r.Terminal != "reverted") {
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
		if err != nil || sender != m.signer.Address() || tx.ChainId().Cmp(m.chainID) != 0 || tx.To() == nil || tx.Value().Sign() != 0 || tx.Type() != types.DynamicFeeTxType {
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
			return result(r), nil
		}
	} else if m.flight == nil {
		if m.stop == nil {
			return Result{ID: req.ID}, errors.New("transaction manager not started")
		}
		m.intent = Request{ID: req.ID, To: req.To, Data: bytes.Clone(req.Data)}
		channel, accepted := m.engine.SendAsync(ctx, engine.Request{To: req.To, Data: req.Data, Label: "execute_recipe", Solver: "merkle"})
		if !accepted {
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
			m.admitted.Store(nil)
			return Result{ID: req.ID}, outcome.Err
		}
		// A definite initial rejection returned no tracked hash; no bytes were
		// accepted. Ambiguous errors and nonce races always retain their evidence.
		if outcome.Outcome == engine.OutcomeSubmissionError && len(outcome.Attempts) == 0 && outcome.Hash == (common.Hash{}) && definiteRejection(outcome.Err) {
			if err := m.store.Delete(journalKey); err != nil {
				return m.retainedResult(req.ID), m.failStorage("clear rejected transaction", err)
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

// journalSigner runs on the engine's lifecycle goroutine. Persistence must
// succeed before signed bytes are returned to the engine's broadcaster.
type journalSigner struct{ m *Manager }

func (s *journalSigner) Address() common.Address { return s.m.signer.Address() }
func (s *journalSigner) SignTx(ctx context.Context, unsigned *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	m := s.m
	signed, err := m.signer.SignTx(ctx, unsigned, chainID)
	if err != nil {
		return nil, err
	}
	if signed == nil {
		return nil, errors.New("signer returned no transaction")
	}
	txSigner := types.LatestSignerForChainID(m.chainID)
	sender, err := types.Sender(txSigner, signed)
	if err != nil || sender != m.signer.Address() || txSigner.Hash(signed) != txSigner.Hash(unsigned) || signed.ChainId().Cmp(m.chainID) != 0 || unsigned.To() == nil || *unsigned.To() != m.intent.To || !bytes.Equal(unsigned.Data(), m.intent.Data) {
		return nil, errors.New("signer changed transaction payload")
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return nil, errors.Errorf("encode signed transaction: %w", err)
	}
	m.recordMu.Lock()
	defer m.recordMu.Unlock()
	if err := m.storageFailure(); err != nil {
		return nil, err
	}
	next := &record{Version: 1, ID: m.intent.ID, ChainID: m.chainID.String(), Sender: m.signer.Address(), Raw: raw, Confirmations: m.cfg.Confirmations}
	if old := m.active.Load(); old != nil {
		copy := *old
		next = &copy
		if old.Terminal != "" {
			return nil, errors.New("recipe already has terminal evidence")
		}
		next.Attempts = append(append([]hexutil.Bytes(nil), old.Attempts...), raw)
	}
	if _, err := m.decode(next); err != nil {
		return nil, err
	}
	m.active.Store(next)
	if err := m.store.Save(journalKey, next); err != nil {
		return nil, m.failStorage("persist signed transaction", err)
	}
	return signed, nil
}

// receipt requires the exact signed hash, its canonical block and the configured
// number of blocks after inclusion. Terminal progress is durable before return.
func (m *Manager) receipt(ctx context.Context, r *record, tx *types.Transaction) (Result, bool, error) {
	pending := result(r)
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
	receipt, err = m.engine.ConfirmRecoveredReceipt(ctx, receipt, r.Confirmations)
	if err != nil {
		return pending, true, err
	}
	if receipt == nil {
		return pending, false, nil
	}
	if receipt.TxHash != tx.Hash() || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.Status > 1 {
		return pending, false, errors.New("invalid confirmed transaction receipt")
	}
	execution := &provider.Execution{TxHash: receipt.TxHash, BlockNumber: new(big.Int).Set(receipt.BlockNumber), BlockHash: receipt.BlockHash, Logs: make([]types.Log, len(receipt.Logs))}
	for i, log := range receipt.Logs {
		if log == nil || log.TxIndex != receipt.TransactionIndex {
			return pending, false, errors.New("invalid receipt log")
		}
		execution.Logs[i] = *log
	}
	if err := execution.Validate(); err != nil {
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
	m.engine.RestoreConfirmedNonce(tx.Nonce())
	m.log.Info("transaction terminal", "id", r.ID, "hash", tx.Hash().Hex(), "outcome", final.Terminal)
	return result(&final), true, nil
}

// Acknowledge follows durable keeper progress, never uncertain execution.
func (m *Manager) Acknowledge(id string) error {
	select {
	case m.gate <- struct{}{}:
		defer func() { <-m.gate }()
	default:
		return ErrLaneBusy
	}
	if err := m.storageFailure(); err != nil {
		return err
	}
	m.recordMu.Lock()
	defer m.recordMu.Unlock()
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
	if err := m.store.Delete(journalKey); err != nil {
		return m.failStorage("acknowledge transaction", err)
	}
	m.active.Store(nil)
	m.admitted.Store(nil)
	return nil
}
