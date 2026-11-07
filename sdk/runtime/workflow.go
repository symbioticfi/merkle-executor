package runtime

import (
	"bytes"
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
)

const Namespace = "v2"

func WorkflowKey(name string) string { return "workflow." + name }

// Workflow is one atomic ownership and reconciliation checkpoint. Legacy retains
// the exact source JSON for imported work; its identity is never rewritten as v2.
type Workflow struct {
	Version         int             `json:"version"`
	IdentityVersion int             `json:"identityVersion"`
	Name            string          `json:"name"`
	ID              string          `json:"id"`
	ConfigHash      string          `json:"configHash"`
	Phase           string          `json:"phase"`
	Plan            Plan            `json:"plan"`
	Transaction     *Transaction    `json:"transaction,omitempty"`
	Reserved        bool            `json:"reserved"`
	FinishedAt      time.Time       `json:"finishedAt,omitempty"`
	Legacy          json.RawMessage `json:"legacy,omitempty"`
}

func (w Workflow) Clone() Workflow {
	w.Plan = w.Plan.Clone()
	w.Legacy = bytes.Clone(w.Legacy)
	if w.Transaction != nil {
		tx := cloneTransaction(*w.Transaction)
		w.Transaction = &tx
	}
	return w
}
func cloneTransaction(tx Transaction) Transaction {
	tx.Intent.Data = bytes.Clone(tx.Intent.Data)
	if tx.ChainID != nil {
		tx.ChainID = new(big.Int).Set(tx.ChainID)
	}
	tx.Attempts = append([]hexutil.Bytes(nil), tx.Attempts...)
	for i := range tx.Attempts {
		tx.Attempts[i] = bytes.Clone(tx.Attempts[i])
	}
	tx.Execution = tx.Execution.Clone()
	return tx
}
func (w Workflow) active() bool {
	return w.Phase == "prepared" || w.Phase == "pending" || w.Phase == "authorized"
}
func validateTransaction(w Workflow, chainID *big.Int, caller common.Address) error {
	tx := w.Transaction
	if tx == nil {
		if w.Phase == "pending" || w.Phase == "authorized" || w.Reserved {
			return errors.New("workflow ownership has no transaction")
		}
		return nil
	}
	if !tx.Admitted || tx.Intent.ID != w.ID || tx.Intent.To != w.Plan.Snapshot.Executor || !bytes.Equal(tx.Intent.Data, w.Plan.Calldata) || tx.Sender != caller || tx.ChainID == nil || tx.ChainID.Cmp(chainID) != 0 {
		return errors.New("workflow transaction identity mismatch")
	}
	seen := map[common.Hash]bool{}
	for _, raw := range tx.Attempts {
		var signed types.Transaction
		if err := signed.UnmarshalBinary(raw); err != nil {
			return errors.New("invalid signed workflow attempt")
		}
		sender, err := types.Sender(types.LatestSignerForChainID(chainID), &signed)
		if err != nil || sender != caller || signed.Type() != types.DynamicFeeTxType || signed.ChainId().Cmp(chainID) != 0 || signed.Nonce() != tx.Nonce || signed.To() == nil || *signed.To() != tx.Intent.To || signed.Value().Sign() != 0 || !bytes.Equal(signed.Data(), tx.Intent.Data) || seen[signed.Hash()] {
			return errors.New("signed workflow attempt differs from immutable intent")
		}
		seen[signed.Hash()] = true
	}
	if tx.Terminal != "" && tx.Terminal != "confirmed" && tx.Terminal != "reverted" {
		return errors.New("invalid workflow transaction terminal phase")
	}
	if tx.Terminal == "" {
		if tx.Execution != nil || w.Phase == "authorized" || w.Phase == "settled" || w.Phase == "failed" {
			return errors.New("terminal workflow lacks transaction evidence")
		}
	} else {
		if err := tx.Execution.Validate(); err != nil {
			return err
		}
		if !seen[tx.Execution.TxHash] {
			if w.IdentityVersion != 1 || w.Reserved || len(tx.Attempts) > 0 {
				return errors.New("workflow evidence has no owned attempt")
			}
			if err := validateLegacyTerminal(w); err != nil {
				return err
			}
		}
		if tx.Terminal == "reverted" && w.Phase != "failed" || tx.Terminal == "confirmed" && w.Phase != "authorized" && w.Phase != "settled" {
			return errors.New("workflow phase differs from canonical transaction")
		}
	}
	if len(tx.Attempts) > 0 && tx.Terminal == "" && (!w.Reserved || w.Phase != "pending") {
		return errors.New("signed pending workflow lost reservation")
	}
	return nil
}
