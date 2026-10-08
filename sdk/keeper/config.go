package keeper

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/crypto"
	"time"
)

// Config describes an authorized workflow. Revision must fingerprint all host-owned
// provider configuration so recovery rejects a changed active workflow.
type Config struct {
	Name, Executor, Account, RecipeHash, Tree string
	Interval                                  time.Duration
	Revision                                  string
}

func (c Config) Fingerprint() string {
	data, _ := json.Marshal(c)
	return crypto.Keccak256Hash(data).Hex()
}
