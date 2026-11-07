package config

import (
	"math"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/transaction"
)

// TxManager exposes the deployment's absolute transaction spend limit.
type TxManager struct {
	MaxFeeGwei float64 `yaml:"maxFeeGwei"`
}

func (c TxManager) validate(required bool) error {
	if c.MaxFeeGwei < 0 || required && c.MaxFeeGwei == 0 || math.IsNaN(c.MaxFeeGwei) || math.IsInf(c.MaxFeeGwei, 0) {
		return errors.New("txManager.maxFeeGwei must be finite and positive")
	}
	return nil
}

func (c TxManager) SenderConfig() transaction.Config {
	return transaction.Config{Confirmations: DefaultConfirmations, MaxFeeGwei: c.MaxFeeGwei}
}
