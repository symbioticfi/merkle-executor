package txmanager

import (
	"context"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
)

// ConfirmRecoveredReceipt exposes the engine's canonical ancestry and depth
// policy to the Merkle durable journal. Nil without error means a reorged
// attempt: continue scanning other owned hashes. This adds no broadcast path.
func (m *Manager) ConfirmRecoveredReceipt(ctx context.Context, receipt *types.Receipt, depth uint64) (*types.Receipt, error) {
	if receipt == nil {
		return nil, errors.New("receipt required")
	}
	if err := validateReceipt(receipt.TxHash, receipt); err != nil {
		return nil, err
	}
	err := m.confirmCanonicalReceipt(ctx, receipt)
	if err == nil {
		receipt, err = m.waitForConfirmations(ctx, receipt.TxHash, receipt, depth)
	}
	if errors.Is(err, errReceiptReorged) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

// RestoreConfirmedNonce applies the engine's owned-nonce floor after the journal
// has validated or persisted terminal canonical evidence, including a revert.
func (m *Manager) RestoreConfirmedNonce(nonce uint64) { m.rememberConfirmedNonce(nonce) }
