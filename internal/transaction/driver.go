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
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/internal/signer"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
)

// ReadBackend never routes observation, fees or simulation to the submission relay.
type ReadBackend interface {
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	FeeHistory(context.Context, uint64, *big.Int, []float64) (*ethereum.FeeHistory, error)
	EstimateGas(context.Context, ethereum.CallMsg) (uint64, error)
}

// WriteBackend reads the mined nonce and publishes signed bytes on the same endpoint.
// Pending nonce counts do not grant authority to skip an unconsumed nonce.
type WriteBackend interface {
	NonceAt(context.Context, common.Address, *big.Int) (uint64, error)
	SendTransaction(context.Context, *types.Transaction) error
}

type Config struct {
	MaxFeeGwei          float64       `yaml:"maxFeeGwei"`
	Confirmations       uint64        `yaml:"confirmations"`
	PollInterval        time.Duration `yaml:"pollInterval"`
	BroadcastTimeout    time.Duration `yaml:"broadcastTimeout"`
	ReplacementInterval time.Duration `yaml:"replacementInterval"`
	PendingTimeout      time.Duration `yaml:"pendingTimeout"`
	ShutdownTimeout     time.Duration `yaml:"shutdownTimeout"`
	ReadTimeout         time.Duration `yaml:"readTimeout"`
	EstimateTimeout     time.Duration `yaml:"estimateTimeout"`
	Horizon             HorizonConfig `yaml:"horizon"`
}

// Validate requires complete YAML policy; zero never silently selects another policy.
func (c Config) Validate() error {
	if math.IsNaN(c.MaxFeeGwei) || math.IsInf(c.MaxFeeGwei, 0) || c.MaxFeeGwei <= 0 {
		return errors.New("finite positive transaction fee ceiling required")
	}
	if err := c.ValidatePolicy(); err != nil {
		return err
	}
	if reserveFeeBump(gweiToWei(c.MaxFeeGwei)).Cmp(gweiToWei(c.Horizon.CongestedTipFloorGwei)) < 0 {
		return errors.New("transaction fee ceiling cannot support policy tip floor and replacement")
	}
	return nil
}

// ValidatePolicy also applies in simulation mode, where there is no spending ceiling.
func (c Config) ValidatePolicy() error {
	if c.Confirmations == 0 || c.Confirmations > 1024 || c.PollInterval <= 0 || c.BroadcastTimeout <= 0 || c.ReplacementInterval <= 0 || c.PendingTimeout <= 0 || c.ShutdownTimeout <= 0 || c.ReadTimeout <= 0 || c.EstimateTimeout <= 0 {
		return errors.New("positive transaction timings and 1..1024 confirmations required")
	}
	if err := c.Horizon.Validate(); err != nil {
		return err
	}
	return nil
}

var (
	ErrReserved        = errors.New("signer reservation unavailable")
	ErrPending         = errors.New("transaction observation stopped with ownership retained")
	ErrNonceConsumed   = errors.New("mined nonce consumed without an owned receipt")
	ErrFeeLimitReached = errors.New("transaction fee limit reached")
)

// Driver has one lane, one active observer and no queue or alternate submission mode.
// The coordinator owns process contexts and durable workflow transitions.
type Driver struct {
	read    ReadBackend
	write   WriteBackend
	signer  signer.Signer
	chainID *big.Int
	cfg     Config
	limit   *big.Int
	policy  horizonPolicy

	mu             sync.Mutex
	owner          string
	active         bool
	blocked        bool
	releaseSafe    bool
	current        *rt.Transaction
	confirmedFloor uint64
}

var _ rt.Driver = (*Driver)(nil)

func New(read ReadBackend, write WriteBackend, signing signer.Signer, chainID *big.Int, cfg Config) (*Driver, error) {
	if read == nil || write == nil || signing == nil || signing.Address() == (common.Address{}) || chainID == nil || chainID.Sign() <= 0 {
		return nil, errors.New("transaction backend, signer and positive chain ID required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Driver{read: read, write: write, signer: signing, chainID: new(big.Int).Set(chainID), cfg: cfg, limit: gweiToWei(cfg.MaxFeeGwei), policy: newHorizonPolicy(cfg.Horizon)}, nil
}

func (d *Driver) Reserve(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id == "" {
		return errors.New("transaction reservation ID required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.blocked || d.owner != "" && d.owner != id {
		return ErrReserved
	}
	d.owner = id
	return nil
}

func (d *Driver) Release(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if id == "" || d.owner != id || d.active || d.blocked {
		return ErrReserved
	}
	if d.current != nil && len(d.current.Attempts) > 0 && !d.releaseSafe {
		return ErrPending
	}
	d.owner = ""
	d.current = nil
	d.releaseSafe = false
	return nil
}

func (d *Driver) begin(id string, record *rt.Transaction) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.blocked || d.owner != id || d.active {
		return ErrReserved
	}
	if record == nil && d.current != nil {
		return ErrPending
	}
	if record != nil && d.current != nil && !sameRecord(*d.current, *record) {
		return errors.New("transaction recovery record differs from owned journal")
	}
	d.active = true
	if record != nil {
		copy := cloneTransaction(*record)
		d.current = &copy
		d.releaseSafe = false
	}
	return nil
}
func (d *Driver) finish() { d.mu.Lock(); d.active = false; d.mu.Unlock() }

func (d *Driver) save(tx rt.Transaction, persist rt.PersistTransaction) error {
	if persist == nil {
		return errors.New("transaction persistence callback required")
	}
	if err := persist(cloneTransaction(tx)); err != nil {
		d.mu.Lock()
		d.blocked = true
		d.mu.Unlock()
		return errors.Errorf("persist transaction ownership: %w", err)
	}
	copy := cloneTransaction(tx)
	d.mu.Lock()
	d.current = &copy
	if tx.Terminal != "" {
		d.confirmedFloor = max(d.confirmedFloor, tx.Nonce+1)
		d.releaseSafe = true
	}
	d.mu.Unlock()
	return nil
}

func (d *Driver) Execute(ctx context.Context, intent rt.Intent, persist rt.PersistTransaction) rt.TransactionResult {
	intent.Data = append(hexutil.Bytes(nil), intent.Data...)
	if err := validateIntent(intent); err != nil {
		return rt.TransactionResult{NotSubmitted: true, Err: err}
	}
	if persist == nil {
		return rt.TransactionResult{NotSubmitted: true, Err: errors.New("transaction persistence callback required")}
	}
	if err := ctx.Err(); err != nil {
		return rt.TransactionResult{NotSubmitted: true, Err: err}
	}
	if err := d.begin(intent.ID, nil); err != nil {
		return rt.TransactionResult{Pending: true, Err: err}
	}
	defer d.finish()
	ctx, cancel := context.WithTimeout(ctx, d.cfg.PendingTimeout)
	defer cancel()
	nonce, err := d.minedNonce(ctx)
	if err != nil {
		return rt.TransactionResult{NotSubmitted: true, Err: err}
	}
	d.mu.Lock()
	nonce = max(nonce, d.confirmedFloor)
	d.mu.Unlock()
	if nonce == math.MaxUint64 {
		return rt.TransactionResult{NotSubmitted: true, Err: errors.New("transaction nonce exhausted")}
	}
	tx := rt.Transaction{Intent: intent, Sender: d.signer.Address(), ChainID: new(big.Int).Set(d.chainID), Nonce: nonce, Admitted: true}
	if err = d.save(tx, persist); err != nil {
		return rt.TransactionResult{NotSubmitted: true, Err: err}
	}
	snapshot, parent, err := d.readFees(ctx)
	if err != nil {
		return rt.TransactionResult{NotSubmitted: true, Err: err}
	}
	gas, _, err := d.estimate(ctx, intent, parent)
	if err != nil {
		return rt.TransactionResult{NotSubmitted: true, Err: err}
	}
	fees, err := horizonFees(snapshot, gas, reserveFeeBump(d.limit), d.policy)
	if err != nil {
		return rt.TransactionResult{NotSubmitted: true, Err: err}
	}
	if err := d.guardNonce(ctx, tx.Nonce); err != nil {
		return rt.TransactionResult{NotSubmitted: true, Err: err}
	}
	signed, err := d.sign(ctx, &tx, gas, fees, persist)
	if err != nil {
		if signed != nil {
			return rt.TransactionResult{Pending: true, Err: err}
		}
		return resultFor(tx, err)
	}
	sendErr := d.broadcast(ctx, signed)
	state := tracking{latest: signed, fees: fees, head: max(snapshot.head, parent.Number.Uint64()), sentHead: max(snapshot.head, parent.Number.Uint64()), sentAt: time.Now(), lastEvaluation: time.Now(), uncertain: ambiguous(sendErr), contention: nonceConflict(sendErr)}
	return d.track(ctx, &tx, persist, &state)
}

// Resume is deliberately observation-only. An admitted unsigned record proves no
// network mutation occurred; its caller may durably halt/refresh and release it.
func (d *Driver) Resume(ctx context.Context, tx rt.Transaction, persist rt.PersistTransaction) rt.TransactionResult {
	tx = cloneTransaction(tx)
	if err := d.validate(tx); err != nil {
		return resultFor(tx, err)
	}
	if persist == nil {
		return resultFor(tx, errors.New("transaction persistence callback required"))
	}
	if err := ctx.Err(); err != nil {
		return resultFor(tx, err)
	}
	if err := d.begin(tx.Intent.ID, &tx); err != nil {
		return rt.TransactionResult{Pending: true, Err: err}
	}
	defer d.finish()
	if len(tx.Attempts) == 0 {
		return rt.TransactionResult{NotSubmitted: true}
	}
	ctx, cancel := context.WithTimeout(ctx, d.cfg.PendingTimeout)
	defer cancel()
	return d.track(ctx, &tx, persist, nil)
}

type tracking struct {
	latest                 *types.Transaction
	fees                   feeQuote
	head, sentHead         uint64
	sentAt, lastEvaluation time.Time
	stallRebroadcasts      int
	uncertain, contention  bool
}

func (d *Driver) track(ctx context.Context, tx *rt.Transaction, persist rt.PersistTransaction, state *tracking) rt.TransactionResult {
	poll := time.NewTicker(d.cfg.PollInterval)
	defer poll.Stop()
	draining := false
	var lastErr error
	for {
		if ctx.Err() != nil {
			if draining {
				return resultFor(*tx, pendingError(ctx, lastErr))
			}
			// Cancellation stops all mutations. Observation alone gets one bounded drain.
			drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.cfg.ShutdownTimeout)
			defer cancel()
			ctx, draining = drainCtx, true
			state = nil
		}
		seen, result := d.observe(ctx, tx, persist)
		if result != nil {
			return *result
		}
		if ctx.Err() != nil {
			continue
		}
		nonce, err := d.minedNonce(ctx)
		if err != nil {
			if !errors.Is(lastErr, ErrNonceConsumed) {
				lastErr = err
			}
		} else if nonce > tx.Nonce {
			// Receipt reads and mined-nonce reads can straddle inclusion or lag
			// independently. Keep observing every owned hash, but stop all mutations.
			state = nil
			if !seen {
				lastErr = ErrNonceConsumed
			}
		}
		if state != nil && err == nil && nonce <= tx.Nonce && !seen {
			if err := d.advance(ctx, tx, persist, state); err != nil {
				lastErr = err
				if errors.Is(err, ErrNonceConsumed) {
					state = nil
				}
				d.mu.Lock()
				blocked := d.blocked
				d.mu.Unlock()
				if blocked {
					return resultFor(*tx, err)
				}
			}
		}
		select {
		case <-ctx.Done():
		case <-poll.C:
		}
		if draining && ctx.Err() != nil {
			return resultFor(*tx, pendingError(ctx, lastErr))
		}
	}
}

// observe is shared by fresh execution, drain and restart. No other component polls receipts.
func (d *Driver) observe(ctx context.Context, tx *rt.Transaction, persist rt.PersistTransaction) (bool, *rt.TransactionResult) {
	seen := false
	for _, raw := range tx.Attempts {
		var signed types.Transaction
		if err := signed.UnmarshalBinary(raw); err != nil {
			r := resultFor(*tx, err)
			return seen, &r
		}
		readCtx, cancel := context.WithTimeout(ctx, d.cfg.ReadTimeout)
		receipt, err := d.read.TransactionReceipt(readCtx, signed.Hash())
		if err != nil || receipt == nil {
			cancel()
			continue
		}
		if receipt.TxHash != signed.Hash() || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockHash == (common.Hash{}) || receipt.Status > 1 {
			cancel()
			continue
		}
		seen = true
		confirmed, err := chainio.VerifyReceipt(readCtx, d.read, receipt, d.cfg.Confirmations)
		cancel()
		if err != nil || confirmed == nil {
			continue
		}
		evidence, err := chainio.ReceiptExecution(confirmed)
		if err != nil {
			continue
		}
		terminal := "confirmed"
		if confirmed.Status == types.ReceiptStatusFailed {
			terminal = "reverted"
		}
		candidate := cloneTransaction(*tx)
		candidate.Terminal = terminal
		candidate.Execution = evidence.Clone()
		if err := d.save(candidate, persist); err != nil {
			r := resultFor(*tx, err)
			return seen, &r
		}
		*tx = candidate
		r := rt.TransactionResult{Reverted: terminal == "reverted", Execution: evidence.Clone()}
		return seen, &r
	}
	return seen, nil
}

func (d *Driver) advance(ctx context.Context, tx *rt.Transaction, persist rt.PersistTransaction, state *tracking) error {
	if state.contention {
		return nil
	}
	if state.uncertain {
		if err := d.guardNonce(ctx, tx.Nonce); err != nil {
			return err
		}
		state.uncertain = false
		err := d.broadcast(ctx, state.latest)
		state.contention = nonceConflict(err)
		state.sentAt = time.Now()
		state.sentHead = state.head
		return err
	}
	snapshot, parent, err := d.readFees(ctx)
	var current *feeQuote
	replace := false
	gas := state.latest.Gas()
	if err != nil {
		if time.Since(state.lastEvaluation) < d.cfg.ReplacementInterval {
			return nil
		}
		state.lastEvaluation = time.Now()
		replace = true
	} else {
		state.lastEvaluation = time.Now()
		if snapshot.head <= state.head {
			return nil
		}
		state.head = snapshot.head
		decision := decidePending(snapshot, state.fees, gas, judgedSentHead(state.sentHead, snapshot.head, state.sentAt, time.Now(), d.policy.blockTime), d.policy)
		switch decision.action {
		case pendingHold:
			return nil
		case pendingReprice:
			replace = true
		case pendingStall:
			estimate, raw, estimateErr := d.estimate(ctx, tx.Intent, parent)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if estimateErr == nil && raw > gas {
				gas = estimate
				replace = true
			} else if state.stallRebroadcasts >= 2 {
				replace = true
			} else {
				if err := d.guardNonce(ctx, tx.Nonce); err != nil {
					return err
				}
				state.stallRebroadcasts++
				err := d.broadcast(ctx, state.latest)
				state.contention = nonceConflict(err)
				state.sentAt = time.Now()
				state.sentHead = state.head
				return err
			}
		}
		fresh, quoteErr := horizonFees(snapshot, gas, nil, d.policy)
		if quoteErr != nil {
			return quoteErr
		}
		current = &fresh
	}
	if !replace {
		return nil
	}
	fees, err := replacementFees(state.fees, current, d.limit)
	if err != nil {
		// At the ceiling, retain the last authorized bytes; never weaken the bump.
		if err := d.guardNonce(ctx, tx.Nonce); err != nil {
			return err
		}
		broadcastErr := d.broadcast(ctx, state.latest)
		state.contention = nonceConflict(broadcastErr)
		state.sentAt = time.Now()
		state.sentHead = state.head
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// A fresh write-endpoint nonce check closes the potentially slow estimate/fee phase.
	if err := d.guardNonce(ctx, tx.Nonce); err != nil {
		return err
	}
	signed, err := d.sign(ctx, tx, gas, fees, persist)
	if err != nil {
		return err
	}
	state.latest = signed
	state.fees = fees
	state.sentAt = time.Now()
	state.sentHead = state.head
	state.stallRebroadcasts = 0
	err = d.broadcast(ctx, signed)
	state.uncertain = ambiguous(err)
	state.contention = nonceConflict(err)
	return err
}

func (d *Driver) readFees(ctx context.Context) (feeSnapshot, *types.Header, error) {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.ReadTimeout)
	defer cancel()
	header, err := d.read.HeaderByNumber(ctx, nil)
	if err != nil {
		return feeSnapshot{}, nil, errors.Errorf("read fee header: %w", err)
	}
	if header == nil || header.Number == nil || !header.Number.IsUint64() || header.GasLimit == 0 {
		return feeSnapshot{}, nil, errors.New("invalid fee header")
	}
	history, err := d.read.FeeHistory(ctx, uint64(d.policy.window()), nil, []float64{d.policy.rewardPercentile})
	if err != nil {
		return feeSnapshot{}, nil, errors.Errorf("read fee history: %w", err)
	}
	snapshot, err := parseFeeSnapshot(history, header.GasLimit)
	return snapshot, header, err
}

// NextBlockGasEstimator preserves block number/time overrides where supported.
type NextBlockGasEstimator interface {
	EstimateGasNextBlock(context.Context, ethereum.CallMsg, *types.Header, time.Duration) (uint64, error)
}

func (d *Driver) estimate(ctx context.Context, intent rt.Intent, parent *types.Header) (limit, raw uint64, err error) {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.EstimateTimeout)
	defer cancel()
	call := ethereum.CallMsg{From: d.signer.Address(), To: &intent.To, Value: new(big.Int), Data: append([]byte(nil), intent.Data...)}
	var gas uint64
	bps := d.policy.fallbackGasHeadroomBps
	if next, ok := d.read.(NextBlockGasEstimator); ok {
		gas, err = next.EstimateGasNextBlock(ctx, call, parent, d.policy.blockTime)
		if err == nil {
			bps = d.policy.gasHeadroomBps
		} else if !isBlockOverrideUnsupported(err) {
			return 0, 0, errors.Errorf("estimate next-block gas: %w", err)
		}
		if err != nil {
			gas, err = d.read.EstimateGas(ctx, call)
		}
	} else {
		gas, err = d.read.EstimateGas(ctx, call)
	}
	if err != nil {
		return 0, 0, errors.Errorf("estimate transaction gas: %w", err)
	}
	if gas == 0 || gas > math.MaxUint64/uint64(basisPoints+bps)*basisPoints {
		return 0, 0, errors.New("invalid gas estimate")
	}
	return withGasHeadroom(gas, bps), gas, nil
}
func (d *Driver) minedNonce(ctx context.Context) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.ReadTimeout)
	defer cancel()
	return d.write.NonceAt(ctx, d.signer.Address(), nil)
}

// Every mutation closes its preparation phase against the submission endpoint.
func (d *Driver) guardNonce(ctx context.Context, owned uint64) error {
	nonce, err := d.minedNonce(ctx)
	if err != nil {
		return err
	}
	if nonce > owned {
		return ErrNonceConsumed
	}
	return nil
}
func (d *Driver) broadcast(ctx context.Context, tx *types.Transaction) error {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.BroadcastTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	return d.write.SendTransaction(ctx, tx)
}
func (d *Driver) sign(ctx context.Context, tx *rt.Transaction, gas uint64, fees feeQuote, persist rt.PersistTransaction) (*types.Transaction, error) {
	unsigned := types.NewTx(&types.DynamicFeeTx{ChainID: new(big.Int).Set(d.chainID), Nonce: tx.Nonce, To: &tx.Intent.To, Data: append([]byte(nil), tx.Intent.Data...), Value: new(big.Int), Gas: gas, GasFeeCap: fees.maxFee, GasTipCap: fees.tip})
	expectedHash := types.LatestSignerForChainID(d.chainID).Hash(unsigned)
	signed, err := d.signer.SignTx(ctx, unsigned, new(big.Int).Set(d.chainID))
	if err != nil {
		return nil, errors.Errorf("sign transaction: %w", err)
	}
	if signed == nil || types.LatestSignerForChainID(d.chainID).Hash(signed) != expectedHash {
		return nil, errors.New("signer changed transaction payload")
	}
	if err := d.validateSigned(*tx, signed); err != nil {
		return nil, err
	}
	if signed.GasFeeCap().Cmp(d.limit) > 0 {
		return nil, errors.New("signed transaction exceeds current fee ceiling")
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return nil, err
	}
	candidate := cloneTransaction(*tx)
	candidate.Attempts = append(candidate.Attempts, raw)
	if err := d.save(candidate, persist); err != nil {
		return signed, err
	}
	*tx = candidate
	return signed, nil
}

func validateIntent(i rt.Intent) error {
	if i.ID == "" || i.To == (common.Address{}) || len(i.Data) == 0 {
		return errors.New("transaction ID, destination and calldata required")
	}
	return nil
}
func (d *Driver) validateSigned(tx rt.Transaction, signed *types.Transaction) error {
	if signed.Type() != types.DynamicFeeTxType || signed.ChainId().Cmp(d.chainID) != 0 || signed.Nonce() != tx.Nonce || signed.To() == nil || *signed.To() != tx.Intent.To || signed.Value().Sign() != 0 || !bytes.Equal(signed.Data(), tx.Intent.Data) || signed.Gas() == 0 || signed.GasFeeCap().Sign() < 0 || signed.GasTipCap().Sign() < 0 || signed.GasTipCap().Cmp(signed.GasFeeCap()) > 0 {
		return errors.New("signed transaction differs from durable intent")
	}
	from, err := types.Sender(types.LatestSignerForChainID(d.chainID), signed)
	if err != nil || from != tx.Sender {
		return errors.New("signed transaction sender mismatch")
	}
	return nil
}
func (d *Driver) validate(tx rt.Transaction) error {
	if err := validateIntent(tx.Intent); err != nil {
		return err
	}
	if !tx.Admitted || tx.Sender != d.signer.Address() || tx.ChainID == nil || tx.ChainID.Cmp(d.chainID) != 0 || tx.Nonce == math.MaxUint64 {
		return errors.New("transaction recovery ownership mismatch")
	}
	if tx.Terminal != "" && tx.Terminal != "confirmed" && tx.Terminal != "reverted" {
		return errors.New("unknown transaction terminal state")
	}
	if (tx.Terminal != "") != (tx.Execution != nil) {
		return errors.New("transaction terminal evidence mismatch")
	}
	seen := map[common.Hash]bool{}
	for _, raw := range tx.Attempts {
		var signed types.Transaction
		if err := signed.UnmarshalBinary(raw); err != nil {
			return errors.New("invalid signed transaction journal")
		}
		if err := d.validateSigned(tx, &signed); err != nil {
			return err
		}
		if seen[signed.Hash()] {
			return errors.New("duplicate signed transaction journal variant")
		}
		seen[signed.Hash()] = true
	}
	if tx.Execution != nil {
		if err := tx.Execution.Validate(); err != nil {
			return err
		}
		if !seen[tx.Execution.TxHash] {
			return errors.New("execution is not an owned signed variant")
		}
	}
	return nil
}
func nonceConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, m := range []string{"nonce too low", "nonce is too low", "nonce has already been used", "replacement transaction underpriced"} {
		if strings.Contains(message, m) {
			return true
		}
	}
	return false
}
func ambiguous(err error) bool {
	if err == nil || nonceConflict(err) {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, m := range []string{"already known", "insufficient funds", "intrinsic gas too low", "invalid sender", "transaction type not supported"} {
		if strings.Contains(message, m) {
			return false
		}
	}
	return true
}
func resultFor(tx rt.Transaction, err error) rt.TransactionResult {
	return rt.TransactionResult{Pending: len(tx.Attempts) > 0, NotSubmitted: len(tx.Attempts) == 0, Err: err}
}

func pendingError(ctx context.Context, lastErr error) error {
	if lastErr != nil {
		return errors.Errorf("%w: %w", ErrPending, lastErr)
	}
	return errors.Errorf("%w: %w", ErrPending, ctx.Err())
}
func cloneTransaction(tx rt.Transaction) rt.Transaction {
	tx.Intent.Data = append(hexutil.Bytes(nil), tx.Intent.Data...)
	if tx.ChainID != nil {
		tx.ChainID = new(big.Int).Set(tx.ChainID)
	}
	attempts := make([]hexutil.Bytes, len(tx.Attempts))
	for i := range tx.Attempts {
		attempts[i] = append(hexutil.Bytes(nil), tx.Attempts[i]...)
	}
	tx.Attempts = attempts
	tx.Execution = tx.Execution.Clone()
	return tx
}
func sameRecord(a, b rt.Transaction) bool {
	if a.Intent.ID != b.Intent.ID || a.Intent.To != b.Intent.To || !bytes.Equal(a.Intent.Data, b.Intent.Data) || a.Sender != b.Sender || a.Nonce != b.Nonce || a.Admitted != b.Admitted || a.ChainID == nil || b.ChainID == nil || a.ChainID.Cmp(b.ChainID) != 0 || len(a.Attempts) != len(b.Attempts) || a.Terminal != b.Terminal {
		return false
	}
	for i := range a.Attempts {
		if !bytes.Equal(a.Attempts[i], b.Attempts[i]) {
			return false
		}
	}
	return true
}
