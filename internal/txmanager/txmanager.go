// Package txmanager connects the shared Merkle journal to this daemon's sender.
package txmanager

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/symbioticfi/merkle-executor/internal/signer"
	engine "github.com/symbioticfi/merkle-executor/internal/txengine"
	"github.com/symbioticfi/merkle-executor/sdk/state"
	"github.com/symbioticfi/merkle-executor/sdk/submission"
	"math"
	"math/big"
	"sync"
)

type Backend interface{ engine.Backend }
type Config = engine.Config
type Request = submission.Request
type Result = submission.Result

var ErrLaneBusy = submission.ErrLaneBusy
var ErrNonceConflict = engine.ErrNonceConflict

type Manager struct {
	*submission.Manager
	engine    *engine.Manager
	transport *transport
	startOnce sync.Once
	stop      context.CancelFunc
	done      chan struct{}
}

type transport struct {
	engine  *engine.Manager
	signer  signer.Signer
	mu      sync.Mutex
	persist submission.Persist
	started bool
}

func (t *transport) Hold(string) error                  { return nil }
func (t *transport) Release(string) error               { return nil }
func (t *transport) RestoreConfirmedNonce(nonce uint64) { t.engine.RestoreConfirmedNonce(nonce) }
func (t *transport) SendAsync(ctx context.Context, req submission.Request, persist submission.Persist) (<-chan submission.Outcome, bool) {
	t.mu.Lock()
	if !t.started {
		t.mu.Unlock()
		return nil, false
	}
	t.persist = persist
	t.mu.Unlock()
	channel, accepted := t.engine.SendAsync(ctx, engine.Request{To: req.To, Data: req.Data, Label: "execute_recipe"})
	if !accepted {
		return nil, false
	}
	result := make(chan submission.Outcome, 1)
	go func() {
		out := <-channel
		result <- submission.Outcome{Hash: out.Hash, Attempts: out.Attempts, Rejected: out.Outcome == engine.OutcomeSubmissionError, Err: out.Err}
	}()
	return result, true
}

// SignTx is the standalone engine's signer boundary. SDK persistence runs before
// returning any signed bytes to the engine, including every replacement.
func (t *transport) Address() common.Address { return t.signer.Address() }
func (t *transport) SignTx(ctx context.Context, unsigned *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	signed, err := t.signer.SignTx(ctx, unsigned, chainID)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	persist := t.persist
	t.mu.Unlock()
	if persist == nil {
		return nil, errors.New("transaction has no journal owner")
	}
	if err := persist(unsigned, signed); err != nil {
		return nil, err
	}
	return signed, nil
}

func New(backend Backend, s signer.Signer, chainID *big.Int, store *state.Store, cfg Config, log logr.Logger) (*Manager, error) {
	return newManagerWithMetrics(backend, s, chainID, store, cfg, log, nil)
}
func newManagerWithMetrics(backend Backend, s signer.Signer, chainID *big.Int, store *state.Store, cfg Config, log logr.Logger, metrics *engine.Metrics) (*Manager, error) {
	if backend == nil || s == nil || store == nil || chainID == nil || chainID.Sign() <= 0 {
		return nil, errors.New("backend, signer, positive chain ID and durable store required")
	}
	if cfg.MaxFeeGwei <= 0 || math.IsNaN(cfg.MaxFeeGwei) || math.IsInf(cfg.MaxFeeGwei, 0) {
		return nil, errors.New("finite positive maxFeeGwei required")
	}
	t := &transport{signer: s}
	t.engine = engine.NewWithMetrics(backend, t, chainID, cfg, metrics, log)
	if err := t.engine.ValidateFeeHeadroom(); err != nil {
		return nil, err
	}
	journal, err := submission.New(backend, s.Address(), chainID, store, submission.Config{Confirmations: cfg.Confirmations, PollInterval: cfg.PollInterval}, t, log)
	if err != nil {
		return nil, err
	}
	return &Manager{Manager: journal, engine: t.engine, transport: t, done: make(chan struct{})}, nil
}
func NewWithMetrics(backend Backend, s signer.Signer, chainID *big.Int, store *state.Store, cfg Config, log logr.Logger, registry prometheus.Registerer) (*Manager, error) {
	metrics, err := engine.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	return newManagerWithMetrics(backend, s, chainID, store, cfg, log, metrics)
}
func (m *Manager) Start(ctx context.Context) error {
	if err := m.engine.Initialize(ctx); err != nil {
		return err
	}
	m.startOnce.Do(func() {
		worker, stop := context.WithCancel(ctx)
		m.stop = stop
		m.transport.mu.Lock()
		m.transport.started = true
		m.transport.mu.Unlock()
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

// Keep the backend contract explicit without adding RPC mutations to the SDK.
var _ submission.Backend = (Backend)(nil)
