// Package transaction implements one in-process signer lane.
package transaction

import (
	"context"
	"math"
	"math/big"
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

// ReadBackend keeps simulation and canonical evidence off the submission relay.
type ReadBackend interface {
	chainio.ReceiptReader
	EstimateGas(context.Context, ethereum.CallMsg) (uint64, error)
}

// WriteBackend reads mined nonce and publishes signed bytes on the same endpoint.
type WriteBackend interface {
	NonceAt(context.Context, common.Address, *big.Int) (uint64, error)
	SendTransaction(context.Context, *types.Transaction) error
}

type Config struct {
	MaxFeeGwei       float64       `yaml:"maxFeeGwei"`
	PriorityFeeGwei  float64       `yaml:"priorityFeeGwei"`
	GasHeadroomBps   int           `yaml:"gasHeadroomBps"`
	Confirmations    uint64        `yaml:"confirmations"`
	PollInterval     time.Duration `yaml:"pollInterval"`
	BroadcastTimeout time.Duration `yaml:"broadcastTimeout"`
	ReadTimeout      time.Duration `yaml:"readTimeout"`
	EstimateTimeout  time.Duration `yaml:"estimateTimeout"`
}

// Validate requires complete YAML policy without hidden defaults.
func (c Config) Validate() error {
	if !positiveGwei(c.MaxFeeGwei) || c.PriorityFeeGwei > c.MaxFeeGwei {
		return errors.New("finite positive transaction fee ceiling covering priority fee required")
	}
	return c.ValidatePolicy()
}

// ValidatePolicy applies in simulation mode, which has no spending ceiling.
func (c Config) ValidatePolicy() error {
	if math.IsNaN(c.PriorityFeeGwei) || math.IsInf(c.PriorityFeeGwei, 0) || c.PriorityFeeGwei < 0 || gweiToWei(c.PriorityFeeGwei).BitLen() > 256 || c.GasHeadroomBps < 1 || c.GasHeadroomBps > basisPoints {
		return errors.New("finite nonnegative priority fee and 1..10000 gas headroom basis points required")
	}
	if c.Confirmations == 0 || c.Confirmations > 1024 || c.PollInterval <= 0 || c.BroadcastTimeout <= 0 || c.ReadTimeout <= 0 || c.EstimateTimeout <= 0 {
		return errors.New("positive transaction timings and 1..1024 confirmations required")
	}
	return nil
}

var (
	ErrReserved        = errors.New("signer operation already owned")
	ErrPending         = errors.New("transaction observation stopped with ownership retained")
	ErrNonceConsumed   = errors.New("mined nonce consumed without an owned receipt")
	ErrFeeLimitReached = errors.New("transaction fee limit reached")
)

// Driver signs one attempt, broadcasts once and observes it without replacements.
type Driver struct {
	read    ReadBackend
	write   WriteBackend
	signer  signer.Signer
	sender  common.Address
	chainID *big.Int
	cfg     Config
	feeCap  *big.Int
	tipCap  *big.Int

	mu             sync.Mutex
	owned          bool
	confirmedFloor uint64
}

var _ rt.Driver = (*Driver)(nil)

func New(read ReadBackend, write WriteBackend, signing signer.Signer, chainID *big.Int, cfg Config) (*Driver, error) {
	if read == nil || write == nil || signing == nil || signing.Address() == (common.Address{}) || chainID == nil || chainID.Sign() <= 0 || chainID.BitLen() > 256 {
		return nil, errors.New("transaction backend, signer and positive chain ID required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Driver{read: read, write: write, signer: signing, sender: signing.Address(), chainID: new(big.Int).Set(chainID), cfg: cfg, feeCap: gweiToWei(cfg.MaxFeeGwei), tipCap: gweiToWei(cfg.PriorityFeeGwei)}, nil
}

func (d *Driver) Execute(ctx context.Context, intent rt.Intent) rt.TransactionResult {
	intent.Data = append(hexutil.Bytes(nil), intent.Data...)
	if intent.ID == "" || intent.To == (common.Address{}) || len(intent.Data) == 0 {
		return notSubmitted(errors.New("transaction ID, destination and calldata required"))
	}
	if err := ctx.Err(); err != nil {
		return notSubmitted(err)
	}
	d.mu.Lock()
	if d.owned {
		d.mu.Unlock()
		return notSubmitted(ErrReserved)
	}
	d.owned = true
	floor := d.confirmedFloor
	d.mu.Unlock()
	// Only pre-broadcast failure and canonical evidence release this process's lane.
	release := true
	defer func() {
		if release {
			d.mu.Lock()
			d.owned = false
			d.mu.Unlock()
		}
	}()

	nonce, err := d.minedNonce(ctx)
	if err != nil {
		return notSubmitted(errors.Errorf("read mined nonce: %w", err))
	}
	if nonce < floor {
		return notSubmitted(errors.New("write endpoint mined nonce is behind confirmed execution"))
	}
	if nonce == math.MaxUint64 {
		return notSubmitted(errors.New("transaction nonce exhausted"))
	}
	gas, err := d.estimate(ctx, intent)
	if err != nil {
		return notSubmitted(err)
	}
	if err := d.validateFees(ctx); err != nil {
		return notSubmitted(err)
	}
	signed, err := d.sign(ctx, intent, nonce, gas)
	if err != nil {
		return notSubmitted(err)
	}
	// Signing may perform remote I/O; close preparation with the mined write nonce.
	current, err := d.minedNonce(ctx)
	if err != nil {
		return notSubmitted(errors.Errorf("recheck mined nonce: %w", err))
	}
	if current != nonce {
		return notSubmitted(ErrNonceConsumed)
	}
	broadcastCtx, cancel := context.WithTimeout(ctx, d.cfg.BroadcastTimeout)
	defer cancel()
	if err := broadcastCtx.Err(); err != nil {
		return notSubmitted(err)
	}
	release = false
	sendErr := d.write.SendTransaction(broadcastCtx, signed)
	cancel()
	result := d.observe(ctx, signed, sendErr)
	if result.Execution != nil && result.Err == nil {
		d.mu.Lock()
		d.confirmedFloor = nonce + 1
		d.mu.Unlock()
		release = true
	}
	return result
}

func (d *Driver) observe(ctx context.Context, signed *types.Transaction, lastErr error) rt.TransactionResult {
	poll := time.NewTicker(d.cfg.PollInterval)
	defer poll.Stop()
	for {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return rt.TransactionResult{Err: errors.Errorf("%w: %w: %w", ErrPending, err, lastErr)}
			}
			return rt.TransactionResult{Err: errors.Errorf("%w: %w", ErrPending, err)}
		}
		readCtx, cancel := context.WithTimeout(ctx, d.cfg.ReadTimeout)
		receipt, err := d.read.TransactionReceipt(readCtx, signed.Hash())
		if err == nil && receipt != nil && receipt.TxHash == signed.Hash() {
			var confirmed *types.Receipt
			confirmed, err = chainio.VerifyReceipt(readCtx, d.read, receipt, d.cfg.Confirmations)
			if err == nil && confirmed != nil {
				evidence, evidenceErr := chainio.ReceiptExecution(confirmed)
				if evidenceErr == nil {
					cancel()
					return rt.TransactionResult{Reverted: confirmed.Status == types.ReceiptStatusFailed, Execution: evidence}
				}
				err = evidenceErr
			}
		}
		cancel()
		if err != nil && !errors.Is(err, ethereum.NotFound) && !errors.Is(lastErr, ErrNonceConsumed) {
			lastErr = err
		}
		// A foreign receipt or lagging read node cannot authorize a second send.
		current, nonceErr := d.minedNonce(ctx)
		if nonceErr == nil && current > signed.Nonce() {
			lastErr = ErrNonceConsumed
		}
		select {
		case <-ctx.Done():
		case <-poll.C:
		}
	}
}

func (d *Driver) minedNonce(ctx context.Context) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.ReadTimeout)
	defer cancel()
	return d.write.NonceAt(ctx, d.sender, nil)
}

func (d *Driver) estimate(ctx context.Context, intent rt.Intent) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.EstimateTimeout)
	defer cancel()
	call := ethereum.CallMsg{From: d.sender, To: &intent.To, Value: new(big.Int), Data: append([]byte(nil), intent.Data...)}
	gas, err := d.read.EstimateGas(ctx, call)
	if err != nil {
		return 0, errors.Errorf("estimate transaction gas: %w", err)
	}
	limit, err := withGasHeadroom(gas, d.cfg.GasHeadroomBps)
	if err != nil {
		return 0, err
	}
	return limit, nil
}

func (d *Driver) validateFees(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.ReadTimeout)
	defer cancel()
	header, err := d.read.HeaderByNumber(ctx, nil)
	if err != nil {
		return errors.Errorf("read fee header: %w", err)
	}
	if header == nil || header.Number == nil || !header.Number.IsUint64() || header.BaseFee == nil || header.BaseFee.Sign() < 0 {
		return errors.New("invalid fee header")
	}
	if new(big.Int).Add(header.BaseFee, d.tipCap).Cmp(d.feeCap) > 0 {
		return ErrFeeLimitReached
	}
	return nil
}

func (d *Driver) sign(ctx context.Context, intent rt.Intent, nonce, gas uint64) (*types.Transaction, error) {
	unsigned := types.NewTx(&types.DynamicFeeTx{ChainID: new(big.Int).Set(d.chainID), Nonce: nonce, To: &intent.To, Data: append([]byte(nil), intent.Data...), Value: new(big.Int), Gas: gas, GasFeeCap: new(big.Int).Set(d.feeCap), GasTipCap: new(big.Int).Set(d.tipCap)})
	validationSigner := types.LatestSignerForChainID(d.chainID)
	expected := validationSigner.Hash(unsigned)
	signed, err := d.signer.SignTx(ctx, unsigned, new(big.Int).Set(d.chainID))
	if err != nil {
		return nil, errors.Errorf("sign transaction: %w", err)
	}
	// Hashing before calling the signer also detects mutation of its input in place.
	if signed == nil || signed.Type() != types.DynamicFeeTxType || signed.ChainId().Cmp(d.chainID) != 0 || validationSigner.Hash(signed) != expected {
		return nil, errors.New("signer changed transaction payload")
	}
	from, err := types.Sender(validationSigner, signed)
	if err != nil || from != d.sender {
		return nil, errors.New("signed transaction sender mismatch")
	}
	return signed, nil
}

func notSubmitted(err error) rt.TransactionResult {
	return rt.TransactionResult{NotSubmitted: true, Err: err}
}
