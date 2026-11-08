// Package transaction owns one process-local signer and its unresolved nonce.
// Hosts own order and claim discovery and explicitly start the transaction worker.
package transaction

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/symbioticfi/merkle-executor/sdk/signer"
)

// Backend uses the sending endpoint for nonce, funding and submission methods.
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

const DefaultMaxGas = params.MaxTxGas
const maxAttempts = 256 // Fail closed rather than evict any signed identity at an unresolved nonce.

type Config struct {
	MaxFeeGwei          float64
	MaxGas              uint64
	Confirmations       uint64
	PollInterval        time.Duration
	ReplacementInterval time.Duration
	ShutdownTimeout     time.Duration
}

type Request struct {
	To           common.Address
	Data         []byte
	Deadline     time.Time
	MaxFeePerGas *big.Int
	// Obsolete reports business invalidity; errors preserve the signed lifecycle.
	Obsolete func(context.Context) (bool, error)
	Label    string
	Solver   string
}

type Result struct {
	Hash        common.Hash
	Attempts    []common.Hash
	Receipt     *types.Receipt // Present only after canonical confirmation, including reverts.
	Err         error
	NotAdmitted bool
}

func (r Result) Included() bool { return r.Receipt != nil }
func (r Result) Reverted() bool {
	return r.Receipt != nil && r.Receipt.Status == types.ReceiptStatusFailed
}
func (r Result) Uncertain() bool {
	return !r.Included() && (r.Hash != (common.Hash{}) || len(r.Attempts) != 0)
}

var (
	ErrBusy            = errors.New("transaction already active")
	ErrNonceConsumed   = errors.New("transaction nonce consumed without this request's receipt")
	ErrRequestObsolete = errors.New("transaction request is obsolete")
	ErrFeeLimitReached = errors.New("transaction fee ceiling reached")
	ErrStopped         = errors.New("transaction manager stopped")
)

type work struct {
	req             Request
	strict          bool
	initialDeadline time.Time
	initialContext  context.Context
	ready           chan struct{}
	results         chan Result
	errors          chan error
	signed          bool // Published by closing ready; only the worker writes it.
	readyOnce       sync.Once
	finished        bool
}

type attempt struct {
	tx   *types.Transaction
	work *work
}
type fees struct{ tip, cap *big.Int }
type pendingNonce struct {
	nonce           uint64
	attempts        []attempt
	accepted        fees
	lastReplacement time.Time
}

// Manager has one unbuffered transaction handoff; domain backlogs stay in the solver.
// The worker alone changes the pending nonce and its signed attempts.
type Manager struct {
	backend                   Backend
	signer                    signer.Signer
	chainID, feeLimit         *big.Int
	cfg                       Config
	log                       logr.Logger
	queue                     chan *work
	slot                      chan struct{}
	stopping                  chan struct{}
	initialized, strict, full atomic.Bool
	demand                    atomic.Int64
	mu                        sync.Mutex
	started                   bool
	floor                     fees // A detached pricing snapshot for concurrent profitability reads.
	pending                   *pendingNonce
	current                   *work
	nonceFloor                uint64
}

// New validates local configuration only. Initialize and Start are explicit host actions.
func New(backend Backend, s signer.Signer, chainID *big.Int, cfg Config, log logr.Logger) (*Manager, error) {
	if backend == nil || s == nil || s.Address() == (common.Address{}) || chainID == nil || chainID.Sign() <= 0 || chainID.BitLen() > 256 || cfg.Confirmations > 1024 {
		return nil, errors.New("backend, signer, positive chain ID and bounded confirmations required")
	}
	if cfg.MaxFeeGwei < 0 || math.IsNaN(cfg.MaxFeeGwei) || math.IsInf(cfg.MaxFeeGwei, 0) {
		return nil, errors.New("finite nonnegative maxFeeGwei required")
	}
	limit, _ := new(big.Float).Mul(big.NewFloat(cfg.MaxFeeGwei), big.NewFloat(1_000_000_000)).Int(nil)
	if !uint256(limit) || cfg.MaxFeeGwei != 0 && limit.Sign() == 0 {
		return nil, errors.New("maxFeeGwei is outside the transaction fee range")
	}
	if cfg.MaxGas == 0 {
		cfg.MaxGas = DefaultMaxGas
	}
	if cfg.PollInterval < 0 || cfg.ReplacementInterval < 0 || cfg.ShutdownTimeout < 0 {
		return nil, errors.New("transaction durations must not be negative")
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.ReplacementInterval == 0 {
		cfg.ReplacementInterval = 30 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = time.Minute
	}
	return &Manager{backend: backend, signer: s, chainID: new(big.Int).Set(chainID), feeLimit: limit, cfg: cfg, log: log.WithName("transaction"), queue: make(chan *work), slot: make(chan struct{}, 1), stopping: make(chan struct{})}, nil
}

func (m *Manager) Initialize(ctx context.Context) error {
	if m.feeLimit.Sign() == 0 {
		return ErrFeeLimitReached
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := m.backend.NonceAt(ctx, m.signer.Address(), nil)
	if err != nil {
		return failure("initialize sending nonce", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.initialized.Store(true)
	return nil
}
func (m *Manager) Confirmations() uint64 { return m.cfg.Confirmations }
func (m *Manager) Available() bool {
	select {
	case <-m.stopping:
		return false
	default:
	}
	return m.initialized.Load() && !m.strict.Load() && !m.full.Load()
}
func (m *Manager) LaneReady() bool { return m.Available() && m.demand.Load() == 0 }

// Send waits for the actual admitted outcome; cancellation after admission cannot report "not sent".
func (m *Manager) Send(ctx context.Context, req Request) Result {
	result, accepted := m.SendAsync(ctx, req)
	if !accepted {
		return Result{Err: ctx.Err(), NotAdmitted: true}
	}
	return <-result
}
func (m *Manager) SendAsync(ctx context.Context, req Request) (<-chan Result, bool) {
	w, accepted := m.admit(ctx, req, false)
	if !accepted {
		return nil, false
	}
	return w.results, true
}
func (m *Manager) admit(ctx context.Context, req Request, strict bool) (*work, bool) {
	req.Data = bytes.Clone(req.Data)
	if req.MaxFeePerGas != nil {
		req.MaxFeePerGas = new(big.Int).Set(req.MaxFeePerGas)
	}
	w := &work{req: req, strict: strict, initialContext: ctx, ready: make(chan struct{}), results: make(chan Result, 1), errors: make(chan error, 1)}
	w.initialDeadline, _ = ctx.Deadline()
	fail := func(err error) (*work, bool) {
		w.results <- Result{Err: err, NotAdmitted: true}
		close(w.ready)
		return w, true
	}
	if err := ctx.Err(); err != nil {
		return nil, false
	}
	if !m.initialized.Load() {
		return fail(errors.New("transaction manager is not initialized"))
	}
	if req.To == (common.Address{}) {
		return fail(errors.New("nonzero transaction destination required"))
	}
	if !req.Deadline.IsZero() && !time.Now().Before(req.Deadline) {
		return fail(context.DeadlineExceeded)
	}
	if req.MaxFeePerGas != nil && (!uint256(req.MaxFeePerGas) || req.MaxFeePerGas.Sign() == 0) {
		return fail(errors.New("invalid request fee ceiling"))
	}
	waitCtx, cancel := context.WithCancel(ctx)
	if !req.Deadline.IsZero() {
		cancel()
		waitCtx, cancel = context.WithDeadline(ctx, req.Deadline)
	}
	defer cancel()
	m.demand.Add(1)
	accepted := false
	defer func() {
		if !accepted {
			m.demand.Add(-1)
		}
	}()
	select {
	case <-m.stopping:
		return fail(ErrStopped)
	default:
	}
	select {
	case m.slot <- struct{}{}:
	case <-waitCtx.Done():
		if ctx.Err() != nil {
			return nil, false
		}
		return fail(waitCtx.Err())
	case <-m.stopping:
		return fail(ErrStopped)
	}
	m.strict.Store(strict)
	select {
	case m.queue <- w:
		accepted = true
		return w, true
	case <-waitCtx.Done():
		m.strict.Store(false)
		<-m.slot
		if ctx.Err() != nil {
			return nil, false
		}
		return fail(waitCtx.Err())
	case <-m.stopping:
		m.strict.Store(false)
		<-m.slot
		return fail(ErrStopped)
	}
}

// Start owns all accepted work. Shutdown stops intake, then observes/rebroadcasts
// existing signatures for a bounded drain; it never signs a different transaction.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			close(m.stopping)
			drain, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.cfg.ShutdownTimeout)
			for m.current != nil || m.pending != nil {
				m.step(drain, false)
				if m.pending == nil {
					break
				}
				select {
				case <-drain.Done():
					goto drained
				case <-ticker.C:
				}
			}
		drained:
			cancel()
			if m.current != nil {
				m.finish(m.current, ErrStopped, nil)
			}
			return
		case w := <-m.queue:
			m.current = w
			m.initial(ctx, w)
		case <-ticker.C:
			m.step(ctx, true)
		}
	}
}

func (m *Manager) initial(ctx context.Context, w *work) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	caller := w.initialContext
	stopCaller := context.AfterFunc(caller, func() {
		if !errors.Is(caller.Err(), context.DeadlineExceeded) {
			cancel()
		}
	})
	defer stopCaller()
	defer func() { w.initialContext = nil }()
	if !w.initialDeadline.IsZero() {
		var c context.CancelFunc
		ctx, c = context.WithDeadline(ctx, w.initialDeadline)
		defer c()
	}
	if !w.req.Deadline.IsZero() {
		var c context.CancelFunc
		ctx, c = context.WithDeadline(ctx, w.req.Deadline)
		defer c()
	}
	if err := m.invalid(ctx, w); err != nil {
		m.finish(w, err, nil)
		return
	}
	m.observe(ctx)
	if m.pending != nil && len(m.pending.attempts) >= maxAttempts {
		m.finish(w, ErrBusy, nil)
		return
	}
	nonce, err := m.backend.NonceAt(ctx, m.signer.Address(), nil)
	if err != nil {
		m.finish(w, failure("read mined nonce", err), nil)
		return
	}
	if nonce < m.nonceFloor || nonce == math.MaxUint64 {
		m.finish(w, errors.New("inconsistent sending nonce"), nil)
		return
	}
	if m.pending != nil && nonce != m.pending.nonce {
		m.finish(w, ErrNonceConsumed, nil)
		return
	}
	remote, err := m.backend.PendingNonceAt(ctx, m.signer.Address())
	if err != nil {
		m.finish(w, failure("read pending nonce", err), nil)
		return
	}
	if m.pending == nil && remote != nonce || m.pending != nil && (remote < nonce || remote > nonce+1) {
		m.finish(w, errors.New("signer has unknown pending work"), nil)
		return
	}
	floor := fees{}
	if m.pending != nil {
		floor = m.pending.accepted
	}
	tx, err := m.prepare(ctx, w, nonce, floor)
	if err != nil {
		m.finish(w, err, nil)
		return
	}
	if m.pending == nil {
		m.pending = &pendingNonce{nonce: nonce}
	}
	stopCaller()
	m.capture(w, tx)
	err = m.broadcast(ctx, tx)
	if !definiteRejection(err) {
		m.accept(tx)
	}
	m.pending.lastReplacement = time.Now()
	if err != nil {
		m.report(w, err)
		if !w.strict && definiteRejection(err) {
			m.finish(w, err, nil)
		}
	}
}

func (m *Manager) invalid(ctx context.Context, w *work) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !w.req.Deadline.IsZero() && !time.Now().Before(w.req.Deadline) {
		return context.DeadlineExceeded
	}
	if w.req.Obsolete != nil {
		obsolete, err := w.req.Obsolete(ctx)
		if err != nil {
			return failure("check request validity", err)
		}
		if obsolete {
			return ErrRequestObsolete
		}
	}
	return ctx.Err()
}
func (m *Manager) prepare(ctx context.Context, w *work, nonce uint64, floor fees) (*types.Transaction, error) {
	gas, priced, err := m.quote(ctx, w.req, floor)
	if err != nil {
		return nil, err
	}
	unsigned := types.NewTx(&types.DynamicFeeTx{ChainID: m.chainID, Nonce: nonce, To: &w.req.To, Data: w.req.Data, Gas: gas, GasTipCap: priced.tip, GasFeeCap: priced.cap})
	if err := m.checkFunding(ctx, unsigned); err != nil {
		return nil, err
	}
	if err := m.invalid(ctx, w); err != nil {
		return nil, err
	}
	fresh, err := m.backend.NonceAt(ctx, m.signer.Address(), nil)
	if err != nil {
		return nil, failure("recheck mined nonce before signing", err)
	}
	if fresh != nonce || fresh < m.nonceFloor {
		return nil, ErrNonceConsumed
	}
	if m.pending == nil {
		pending, err := m.backend.PendingNonceAt(ctx, m.signer.Address())
		if err != nil {
			return nil, failure("recheck pending nonce before signing", err)
		}
		if pending != fresh {
			return nil, errors.New("signer has unknown pending work")
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !w.signed && w.initialContext != nil {
		if err := w.initialContext.Err(); err != nil {
			return nil, err
		}
	}
	signed, err := m.signer.SignTx(ctx, unsigned, new(big.Int).Set(m.chainID))
	if err != nil {
		return nil, failure("sign transaction", err)
	}
	txSigner := types.LatestSignerForChainID(m.chainID)
	if signed == nil {
		return nil, errors.New("signer returned no transaction")
	}
	from, err := types.Sender(txSigner, signed)
	if err != nil || from != m.signer.Address() || signed.Type() != types.DynamicFeeTxType || txSigner.Hash(signed) != txSigner.Hash(unsigned) {
		return nil, errors.New("signer changed transaction payload")
	}
	return signed, nil
}
func (m *Manager) capture(w *work, tx *types.Transaction) {
	for _, a := range m.pending.attempts {
		if a.tx.Hash() == tx.Hash() && a.work == w {
			return
		}
	}
	m.pending.attempts = append(m.pending.attempts, attempt{tx: tx, work: w})
	m.full.Store(len(m.pending.attempts) == maxAttempts)
	if !w.signed {
		w.signed = true
		w.readyOnce.Do(func() { close(w.ready) })
	}
	m.log.Info("transaction signed", "hash", tx.Hash().Hex(), "nonce", tx.Nonce(), "label", w.req.Label, "solver", w.req.Solver)
}
func (m *Manager) accept(tx *types.Transaction) {
	m.pending.accepted = fees{tip: tx.GasTipCap(), cap: tx.GasFeeCap()}
	m.mu.Lock()
	m.floor = fees{tip: tx.GasTipCap(), cap: tx.GasFeeCap()}
	m.mu.Unlock()
}
func (m *Manager) report(w *work, err error) {
	select {
	case w.errors <- err:
	default:
	}
}
func (m *Manager) finish(w *work, err error, receipt *types.Receipt) {
	if w.finished {
		return
	}
	result := Result{Err: err, Receipt: receipt}
	if m.pending != nil {
		for _, a := range m.pending.attempts {
			if a.work == w {
				result.Attempts = append(result.Attempts, a.tx.Hash())
				result.Hash = a.tx.Hash()
			}
		}
	}
	if receipt != nil {
		result.Hash = receipt.TxHash
	}
	w.finished = true
	w.results <- result
	w.readyOnce.Do(func() { close(w.ready) })
	if m.current == w {
		m.current = nil
		m.strict.Store(false)
		m.demand.Add(-1)
		<-m.slot
	}
}

func (m *Manager) step(ctx context.Context, replace bool) {
	if m.pending == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	blocked := m.observe(ctx)
	if m.current == nil {
		return
	}
	w := m.current
	if !w.req.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, w.req.Deadline)
		defer cancel()
	}
	if err := m.invalid(ctx, w); err != nil {
		m.report(w, err)
		// Failed business reads are not proof of obsolescence.
		expired := !w.req.Deadline.IsZero() && !time.Now().Before(w.req.Deadline)
		if !w.strict && (errors.Is(err, ErrRequestObsolete) || expired) {
			m.finish(w, err, nil)
		}
		return
	}
	if blocked {
		return
	}
	var latest *types.Transaction
	for i := len(m.pending.attempts) - 1; i >= 0; i-- {
		if m.pending.attempts[i].work == w {
			latest = m.pending.attempts[i].tx
			break
		}
	}
	if latest == nil {
		return
	}
	if replace && time.Since(m.pending.lastReplacement) >= m.cfg.ReplacementInterval {
		m.pending.lastReplacement = time.Now()
		if len(m.pending.attempts) >= maxAttempts {
			m.report(w, ErrBusy)
			return
		}
		tx, err := m.prepare(ctx, w, m.pending.nonce, m.pending.accepted)
		if err != nil {
			m.report(w, err)
			return
		}
		m.capture(w, tx)
		latest = tx
	}
	err := m.broadcast(ctx, latest)
	if !definiteRejection(err) {
		m.accept(latest)
	}
	if err != nil {
		m.report(w, err)
	}
}

// observe checks every signed variant; only its request may receive the winning receipt.
func (m *Manager) observe(ctx context.Context) bool {
	if m.pending == nil {
		return false
	}
	for _, a := range m.pending.attempts {
		receipt, err := m.backend.TransactionReceipt(ctx, a.tx.Hash())
		if errors.Is(err, ethereum.NotFound) {
			continue
		}
		if err != nil {
			if m.current != nil {
				m.report(m.current, failure("read transaction receipt", err))
			}
			return false
		}
		if _, err = executionFromReceipt(receipt, a.tx.Hash()); err != nil {
			if m.current != nil {
				m.report(m.current, err)
			}
			return false
		}
		receipt, err = confirmReceipt(ctx, m.backend, receipt, m.cfg.Confirmations)
		if err != nil {
			if m.current != nil {
				m.report(m.current, err)
			}
			return false
		}
		if receipt == nil {
			continue
		}
		if !a.work.finished {
			m.finish(a.work, nil, receipt)
		}
		if m.current != nil && m.current.signed {
			m.finish(m.current, ErrNonceConsumed, nil)
		}
		m.nonceFloor = m.pending.nonce + 1
		m.pending = nil
		m.full.Store(false)
		m.mu.Lock()
		m.floor = fees{}
		m.mu.Unlock()
		return true
	}
	nonce, err := m.backend.NonceAt(ctx, m.signer.Address(), nil)
	if m.current != nil {
		if err != nil {
			m.report(m.current, failure("read active nonce", err))
		} else if nonce > m.pending.nonce {
			m.report(m.current, ErrNonceConsumed)
			return true
		}
	}
	return false
}

func (m *Manager) checkFunding(ctx context.Context, tx *types.Transaction) error {
	cost := tx.Cost()
	if !uint256(cost) {
		return errors.New("invalid maximum transaction cost")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	balance, err := m.backend.TransactionSenderBalanceAt(ctx, m.signer.Address(), nil)
	if err != nil {
		return failure("read transaction sender balance", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !uint256(balance) {
		return errors.New("invalid transaction sender balance")
	}
	if balance.Cmp(cost) < 0 {
		return errors.New("insufficient transaction funding")
	}
	return nil
}
func uint256(n *big.Int) bool { return n != nil && n.Sign() >= 0 && n.BitLen() <= 256 }
func bump(n *big.Int) *big.Int {
	step := new(big.Int).Div(new(big.Int).Add(n, big.NewInt(7)), big.NewInt(8))
	if step.Sign() == 0 {
		step.SetInt64(1)
	}
	return new(big.Int).Add(n, step)
}

func (m *Manager) price(ctx context.Context, req Request, floor fees) (fees, *types.Header, error) {
	header, err := m.backend.HeaderByNumber(ctx, nil)
	if err != nil {
		return fees{}, nil, failure("read fee header", err)
	}
	if header == nil || header.Number == nil || !header.Number.IsUint64() || !uint256(header.BaseFee) || header.GasLimit == 0 {
		return fees{}, nil, errors.New("invalid EIP-1559 fee header")
	}
	history, err := m.backend.FeeHistory(ctx, 1, header.Number, []float64{50})
	if err != nil {
		return fees{}, nil, failure("read transaction fee", err)
	}
	if history == nil || len(history.Reward) != 1 || len(history.Reward[0]) != 1 || !uint256(history.Reward[0][0]) {
		return fees{}, nil, errors.New("invalid transaction fee reward")
	}
	tip := new(big.Int).Set(history.Reward[0][0])
	if tip.Sign() == 0 {
		tip.SetInt64(1)
	}
	cap := new(big.Int).Add(new(big.Int).Mul(header.BaseFee, big.NewInt(2)), tip)
	limit := new(big.Int).Set(m.feeLimit)
	if req.MaxFeePerGas != nil && req.MaxFeePerGas.Cmp(limit) < 0 {
		limit.Set(req.MaxFeePerGas)
	}
	if cap.Cmp(limit) > 0 {
		cap.Set(limit)
	}
	if floor.cap != nil {
		if n := bump(floor.tip); tip.Cmp(n) < 0 {
			tip = n
		}
		if n := bump(floor.cap); cap.Cmp(n) < 0 {
			cap = n
		}
	}
	minimum := new(big.Int).Add(header.BaseFee, tip)
	if !uint256(cap) || minimum.Cmp(limit) > 0 || floor.cap != nil && cap.Cmp(limit) > 0 {
		return fees{}, nil, ErrFeeLimitReached
	}
	if cap.Cmp(limit) > 0 {
		cap.Set(limit)
	}
	return fees{tip: tip, cap: cap}, header, nil
}
func (m *Manager) MaxFeePerGas(ctx context.Context) (*big.Int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m.mu.Lock()
	floor := m.floor
	m.mu.Unlock()
	priced, _, err := m.price(ctx, Request{}, floor)
	if err != nil {
		return nil, err
	}
	ceiling := bump(priced.cap)
	if ceiling.Cmp(m.feeLimit) > 0 {
		ceiling.Set(m.feeLimit)
	}
	return ceiling, nil
}
func (m *Manager) quote(ctx context.Context, req Request, floor fees) (uint64, fees, error) {
	priced, header, err := m.price(ctx, req, floor)
	if err != nil {
		return 0, fees{}, err
	}
	limit := min(m.cfg.MaxGas, header.GasLimit)
	gas, err := m.backend.EstimateGas(ctx, ethereum.CallMsg{From: m.signer.Address(), To: &req.To, Data: req.Data, Gas: limit, GasFeeCap: priced.cap, GasTipCap: priced.tip})
	if err != nil {
		return 0, fees{}, failure("estimate transaction gas", err)
	}
	if gas == 0 || gas > limit {
		return 0, fees{}, errors.New("transaction gas exceeds budget or block limit")
	}
	margin := gas / 10
	if gas%10 != 0 {
		margin++
	}
	if gas > math.MaxUint64-margin {
		return 0, fees{}, errors.New("transaction gas overflows")
	}
	return min(gas+margin, limit), priced, nil
}
func (m *Manager) broadcast(ctx context.Context, tx *types.Transaction) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := m.backend.SendTransaction(ctx, tx)
	if err != nil {
		message := strings.ToLower(err.Error())
		if !strings.Contains(message, "already known") && !strings.Contains(message, "already imported") {
			return failure("broadcast transaction; ownership retained", err)
		}
	}
	return nil
}
func definiteRejection(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	var operation *operationError
	if errors.As(err, &operation) {
		err = operation.cause
	}
	message := strings.ToLower(err.Error())
	for _, text := range []string{"nonce too low", "nonce is too low", "nonce has already been used", "underpriced", "insufficient funds", "intrinsic gas", "exceeds block gas", "invalid sender"} {
		if strings.Contains(message, text) {
			return true
		}
	}
	return false
}

// Keep endpoint text out of host logs while preserving cancellation and RPC error data.
type operationError struct {
	operation string
	cause     error
}

func (e *operationError) Error() string { return e.operation + " failed" }
func (e *operationError) Unwrap() error { return e.cause }
func failure(operation string, err error) error {
	return &operationError{operation: operation, cause: err}
}
