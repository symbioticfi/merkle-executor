package config

import (
	"math"
	"strconv"
	"strings"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/transaction"
	"gopkg.in/yaml.v3"
)

// TxManager exposes deployment transaction budgets.
type TxManager struct {
	MaxFeeGwei float64 `yaml:"maxFeeGwei"`
	MaxGas     uint64  `yaml:"maxGas"`
}

func (c *TxManager) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return errors.New("txManager must contain transaction budgets")
	}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		switch key.Value {
		case "maxFeeGwei":
		case "maxGas":
			// yaml.v3 otherwise truncates fractional numbers and can round overflow.
			if value.Kind == yaml.AliasNode {
				value = value.Alias
			}
			if value.Tag != "!!int" {
				return errors.New("txManager.maxGas must be a uint64 integer")
			}
			if _, err := strconv.ParseUint(strings.TrimPrefix(strings.ReplaceAll(value.Value, "_", ""), "+"), 0, 64); err != nil {
				return errors.New("txManager.maxGas must be a uint64 integer")
			}
		default:
			return errors.New("unknown txManager field")
		}
	}
	type plain TxManager
	return node.Decode((*plain)(c))
}

func (c TxManager) validate(required bool) error {
	if c.MaxFeeGwei < 0 || required && c.MaxFeeGwei == 0 || math.IsNaN(c.MaxFeeGwei) || math.IsInf(c.MaxFeeGwei, 0) {
		return errors.New("txManager.maxFeeGwei must be finite and positive")
	}
	return nil
}

func (c TxManager) SenderConfig() transaction.Config {
	return transaction.Config{Confirmations: DefaultConfirmations, MaxFeeGwei: c.MaxFeeGwei, MaxGas: c.MaxGas}
}
