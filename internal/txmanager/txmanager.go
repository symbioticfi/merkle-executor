// Package txmanager owns one durable signed transaction lifecycle per sending EOA.
// An uncertain broadcast or cancelled context never authorizes a new nonce.
package txmanager

import (
	"bytes"
	"context"
	"math/big"
	"sync/atomic"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"

	"github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/signer"
	"github.com/symbioticfi/merkle-executor/internal/state"
)

type Backend interface {
	NonceAt(context.Context, common.Address, *big.Int) (uint64, error)
	PendingNonceAt(context.Context, common.Address) (uint64, error)
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
	EstimateGas(context.Context, ethereum.CallMsg) (uint64, error)
	SendTransaction(context.Context, *types.Transaction) error
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	HeaderByHash(context.Context, common.Hash) (*types.Header, error)
}

type Config struct {
	Confirmations                      uint64
	PollInterval                       time.Duration
	MaxFeePerGas, MaxPriorityFeePerGas *big.Int
	GasLimit                           uint64
}

type Request struct {
	ID   string
	To   common.Address
	Data []byte
}
type Result struct {
	ID                           string
	Hash                         common.Hash
	Confirmed, Reverted, Pending bool
	Execution                    *provider.Execution `json:"execution,omitempty"`
}

var (
	ErrLaneBusy      = errors.New("transaction lane occupied")
	ErrNonceConflict = errors.New("external or unresolved nonce conflict")
)

const journalKey = "txmanager.active.v1"

type record struct {
	Version       uint                `json:"version"`
	ID            string              `json:"id"`
	ChainID       string              `json:"chainId"`
	Sender        common.Address      `json:"sender"`
	Raw           hexutil.Bytes       `json:"raw"`
	Confirmations uint64              `json:"confirmations"`
	Terminal      string              `json:"terminal,omitempty"`
	Execution     *provider.Execution `json:"execution,omitempty"`
}

// Manager admits one Send/Acknowledge at a time. The immutable active snapshot
// makes PendingID safe while Send is waiting on RPC or confirmations.
type Manager struct {
	backend        Backend
	signer         signer.Signer
	chainID        *big.Int
	store          *state.Store
	cfg            Config
	log            logr.Logger
	gate           chan struct{}
	active         atomic.Pointer[record]
	storageFailure error // guarded by gate; uncertain persistence blocks this process
}

func New(backend Backend, s signer.Signer, chainID *big.Int, store *state.Store, cfg Config, log logr.Logger) (*Manager, error) {
	if backend == nil || s == nil || store == nil || chainID == nil || chainID.Sign() <= 0 {
		return nil, errors.New("transaction manager requires backend, signer, store and positive chain ID")
	}
	if cfg.MaxFeePerGas == nil || cfg.MaxFeePerGas.Sign() <= 0 || cfg.MaxPriorityFeePerGas == nil || cfg.MaxPriorityFeePerGas.Sign() < 0 || cfg.MaxPriorityFeePerGas.Cmp(cfg.MaxFeePerGas) > 0 {
		return nil, errors.New("positive max fee and nonnegative priority fee within max fee required")
	}
	if cfg.PollInterval < 0 {
		return nil, errors.New("negative poll interval")
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 2 * time.Second
	}
	cfg.MaxFeePerGas = new(big.Int).Set(cfg.MaxFeePerGas)
	cfg.MaxPriorityFeePerGas = new(big.Int).Set(cfg.MaxPriorityFeePerGas)
	m := &Manager{backend: backend, signer: s, chainID: new(big.Int).Set(chainID), store: store, cfg: cfg, log: log.WithName("txmanager"), gate: make(chan struct{}, 1)}
	var saved record
	found, err := store.Load(journalKey, &saved)
	if err != nil {
		return nil, errors.Errorf("load transaction journal: %w", err)
	}
	if found {
		if _, err := m.decode(&saved); err != nil {
			return nil, err
		}
		m.active.Store(&saved)
	}
	return m, nil
}

// PendingID also returns terminal-but-unacknowledged work. The keeper must persist
// its terminal progress before Acknowledge, including after a process restart.
func (m *Manager) PendingID() string {
	if r := m.active.Load(); r != nil {
		return r.ID
	}
	return ""
}

func (m *Manager) decode(r *record) (*types.Transaction, error) {
	if r.Version != 1 || r.ID == "" || r.ChainID != m.chainID.String() || r.Sender != m.signer.Address() || (r.Terminal != "" && r.Terminal != "confirmed" && r.Terminal != "reverted") {
		return nil, errors.New("transaction journal identity or version mismatch")
	}
	var tx types.Transaction
	if err := tx.UnmarshalBinary(r.Raw); err != nil {
		return nil, errors.Errorf("decode journal transaction: %w", err)
	}
	from, err := types.Sender(types.LatestSignerForChainID(m.chainID), &tx)
	if err != nil || from != r.Sender || tx.ChainId().Cmp(m.chainID) != 0 || tx.To() == nil || tx.Value().Sign() != 0 || tx.Type() != types.DynamicFeeTxType {
		return nil, errors.New("transaction journal signed payload mismatch")
	}
	if r.Execution != nil {
		if err := r.Execution.Validate(); err != nil {
			return nil, err
		}
		if r.Terminal == "" || r.Execution.TxHash != tx.Hash() {
			return nil, errors.New("transaction journal execution mismatch")
		}
	}
	return &tx, nil
}

func result(r *record, tx *types.Transaction) Result {
	return Result{ID: r.ID, Hash: tx.Hash(), Confirmed: r.Terminal == "confirmed", Reverted: r.Terminal == "reverted", Pending: r.Terminal == "", Execution: r.Execution.Clone()}
}

// Send reconciles retained work first. It only re-broadcasts its exact signed
// bytes and never replaces calldata at another nonce, including after timeout.
func (m *Manager) Send(ctx context.Context, req Request) (Result, error) {
	select {
	case m.gate <- struct{}{}:
		defer func() { <-m.gate }()
	default:
		return Result{ID: req.ID, Pending: m.PendingID() != ""}, ErrLaneBusy
	}
	if m.storageFailure != nil {
		return Result{ID: req.ID, Pending: m.PendingID() != ""}, m.storageFailure
	}
	if err := ctx.Err(); err != nil {
		return m.retainedResult(req.ID), err
	}
	if req.ID == "" || req.To == (common.Address{}) {
		return Result{ID: req.ID}, errors.New("request ID and destination required")
	}
	r := m.active.Load()
	var tx *types.Transaction
	var err error
	if r != nil {
		tx, err = m.decode(r)
		if err != nil {
			return Result{ID: req.ID, Pending: true}, err
		}
		if r.ID != req.ID {
			return result(r, tx), ErrLaneBusy
		}
		if *tx.To() != req.To || !bytes.Equal(tx.Data(), req.Data) {
			return result(r, tx), errors.New("transaction ID reused with different payload")
		}
		if r.Terminal != "" {
			return result(r, tx), nil
		}
		if terminal, seen, err := m.receipt(ctx, r, tx); err != nil || terminal.Confirmed || terminal.Reverted {
			return terminal, err
		} else if seen {
			return m.wait(ctx, r, tx)
		}
	} else {
		r, tx, err = m.prepare(ctx, req)
		if err != nil {
			return m.retainedResult(req.ID), err
		}
	}
	if err := m.checkOwnedNonce(ctx, tx.Nonce()); err != nil {
		return result(r, tx), err
	}
	// Persistence in prepare precedes this only transaction-producing side effect.
	err = m.backend.SendTransaction(ctx, tx)
	if err != nil {
		if terminal, _, checkErr := m.receipt(ctx, r, tx); checkErr == nil && (terminal.Confirmed || terminal.Reverted) {
			return terminal, nil
		}
		return result(r, tx), errors.Errorf("broadcast uncertain; signed transaction retained: %w", err)
	}
	m.log.Info("transaction submitted", "id", r.ID, "hash", tx.Hash().Hex(), "nonce", tx.Nonce())
	return m.wait(ctx, r, tx)
}

func (m *Manager) retainedResult(id string) Result {
	if r := m.active.Load(); r != nil {
		if tx, err := m.decode(r); err == nil {
			return result(r, tx)
		}
		return Result{ID: r.ID, Pending: true}
	}
	return Result{ID: id}
}

func (m *Manager) prepare(ctx context.Context, req Request) (*record, *types.Transaction, error) {
	latest, err := m.backend.NonceAt(ctx, m.signer.Address(), nil)
	if err != nil {
		return nil, nil, errors.Errorf("read mined nonce: %w", err)
	}
	pending, err := m.backend.PendingNonceAt(ctx, m.signer.Address())
	if err != nil {
		return nil, nil, errors.Errorf("read pending nonce: %w", err)
	}
	if latest != pending {
		return nil, nil, ErrNonceConflict
	}
	call := ethereum.CallMsg{From: m.signer.Address(), To: &req.To, Data: bytes.Clone(req.Data), Value: new(big.Int), Gas: m.cfg.GasLimit, GasFeeCap: new(big.Int).Set(m.cfg.MaxFeePerGas), GasTipCap: new(big.Int).Set(m.cfg.MaxPriorityFeePerGas)}
	if call.Gas == 0 {
		call.Gas, err = m.backend.EstimateGas(ctx, call)
		if err != nil {
			return nil, nil, errors.Errorf("estimate exact call: %w", err)
		}
		if call.Gas == 0 {
			return nil, nil, errors.New("gas estimate was zero")
		}
	}
	if _, err := m.backend.CallContract(ctx, call, nil); err != nil {
		return nil, nil, errors.Errorf("simulate exact call: %w", err)
	}
	unsigned := types.NewTx(&types.DynamicFeeTx{ChainID: m.chainID, Nonce: latest, To: &req.To, Data: bytes.Clone(req.Data), Value: new(big.Int), Gas: call.Gas, GasFeeCap: call.GasFeeCap, GasTipCap: call.GasTipCap})
	signed, err := m.signer.SignTx(ctx, unsigned, m.chainID)
	if err != nil {
		return nil, nil, errors.Errorf("sign transaction: %w", err)
	}
	if signed == nil {
		return nil, nil, errors.New("signer returned no transaction")
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		return nil, nil, errors.Errorf("encode signed transaction: %w", err)
	}
	r := &record{Version: 1, ID: req.ID, ChainID: m.chainID.String(), Sender: m.signer.Address(), Raw: raw, Confirmations: m.cfg.Confirmations}
	verified, err := m.decode(r)
	if err != nil {
		return nil, nil, err
	}
	txSigner := types.LatestSignerForChainID(m.chainID)
	if txSigner.Hash(verified) != txSigner.Hash(unsigned) {
		return nil, nil, errors.New("signer changed transaction payload")
	}
	m.active.Store(r)
	if err := m.store.Save(journalKey, r); err != nil {
		m.storageFailure = errors.Errorf("persist signed transaction; broadcasting blocked: %w", err)
		return nil, nil, m.storageFailure
	}
	return r, verified, nil
}

func (m *Manager) checkOwnedNonce(ctx context.Context, nonce uint64) error {
	latest, err := m.backend.NonceAt(ctx, m.signer.Address(), nil)
	if err != nil {
		return errors.Errorf("reconcile mined nonce: %w", err)
	}
	pending, err := m.backend.PendingNonceAt(ctx, m.signer.Address())
	if err != nil {
		return errors.Errorf("reconcile pending nonce: %w", err)
	}
	if latest != nonce || pending < nonce || (pending > nonce && pending-nonce > 1) {
		return ErrNonceConflict
	}
	return nil
}

func (m *Manager) wait(ctx context.Context, r *record, tx *types.Transaction) (Result, error) {
	ticker := time.NewTicker(m.cfg.PollInterval)
	defer ticker.Stop()
	for {
		terminal, seen, err := m.receipt(ctx, r, tx)
		if err != nil || terminal.Confirmed || terminal.Reverted {
			return terminal, err
		}
		if !seen {
			if err := m.checkOwnedNonce(ctx, tx.Nonce()); err != nil {
				return result(r, tx), err
			}
		}
		select {
		case <-ctx.Done():
			return result(r, tx), ctx.Err()
		case <-ticker.C:
		}
	}
}

// receipt requires the exact signed hash, its canonical block and the configured
// number of blocks after inclusion. Terminal progress is durable before return.
func (m *Manager) receipt(ctx context.Context, r *record, tx *types.Transaction) (Result, bool, error) {
	pending := result(r, tx)
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
	witnessNumber := new(big.Int).Add(receipt.BlockNumber, new(big.Int).SetUint64(r.Confirmations))
	witness, err := m.backend.HeaderByNumber(ctx, witnessNumber)
	if errors.Is(err, ethereum.NotFound) {
		return pending, true, nil
	}
	if err != nil {
		return pending, true, errors.Errorf("read confirmation block: %w", err)
	}
	if witness == nil || witness.Number == nil || witness.Number.Cmp(witnessNumber) != 0 {
		return pending, true, nil
	}
	ancestor := witness
	for ancestor.Number.Cmp(receipt.BlockNumber) > 0 {
		parent, readErr := m.backend.HeaderByHash(ctx, ancestor.ParentHash)
		if readErr != nil {
			return pending, true, errors.Errorf("read confirmation ancestor: %w", readErr)
		}
		if parent == nil || parent.Number == nil || parent.Hash() != ancestor.ParentHash || new(big.Int).Sub(ancestor.Number, parent.Number).Cmp(big.NewInt(1)) != 0 {
			return pending, true, errors.New("inconsistent confirmation ancestry")
		}
		ancestor = parent
	}
	if ancestor.Hash() != receipt.BlockHash {
		return pending, false, nil
	}
	stable, err := m.backend.HeaderByNumber(ctx, witnessNumber)
	if err != nil {
		return pending, true, errors.Errorf("recheck confirmation head: %w", err)
	}
	if stable == nil || stable.Hash() != witness.Hash() {
		return pending, true, nil
	}
	final := *r
	final.Terminal = "confirmed"
	final.Execution = execution.Clone()
	if receipt.Status == types.ReceiptStatusFailed {
		final.Terminal = "reverted"
	}
	if err := m.store.Save(journalKey, &final); err != nil {
		m.storageFailure = errors.Errorf("persist terminal receipt: %w", err)
		return pending, true, m.storageFailure
	}
	m.active.Store(&final)
	m.log.Info("transaction terminal", "id", r.ID, "hash", tx.Hash().Hex(), "outcome", final.Terminal)
	return result(&final, tx), true, nil
}

// Acknowledge must only follow durable keeper progress. It is idempotent after
// success, but cannot release an unresolved or different transaction's record.
func (m *Manager) Acknowledge(id string) error {
	select {
	case m.gate <- struct{}{}:
		defer func() { <-m.gate }()
	default:
		return ErrLaneBusy
	}
	if m.storageFailure != nil {
		return m.storageFailure
	}
	r := m.active.Load()
	if r == nil {
		return nil
	}
	if id != r.ID || r.Terminal == "" {
		return errors.New("only matching terminal transaction can be acknowledged")
	}
	if err := m.store.Delete(journalKey); err != nil {
		m.storageFailure = errors.Errorf("acknowledge transaction: %w", err)
		return m.storageFailure
	}
	m.active.Store(nil)
	return nil
}
