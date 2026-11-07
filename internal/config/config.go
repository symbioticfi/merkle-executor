// Package config decodes strict YAML over the versioned file-driven defaults.
package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/internal/transaction"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/permissions"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"gopkg.in/yaml.v3"
)

//go:embed defaults.yaml
var defaults []byte

type Config struct {
	Curator       Curator            `yaml:"curator"`
	Mode          string             `yaml:"mode"`
	Chain         Chain              `yaml:"chain"`
	Signer        Signer             `yaml:"signer"`
	Transaction   transaction.Config `yaml:"transaction"`
	Runtime       Runtime            `yaml:"runtime"`
	Reads         chainio.Limits     `yaml:"reads"`
	StateDir      string             `yaml:"stateDir"`
	Observability Observability      `yaml:"observability"`
	Strategies    []string           `yaml:"strategies"`
}
type Runtime struct {
	PollInterval     time.Duration `yaml:"pollInterval"`
	OperationTimeout time.Duration `yaml:"operationTimeout"`
	HTTPTimeout      time.Duration `yaml:"httpTimeout"`
	StrategyInterval time.Duration `yaml:"strategyInterval"`
	ShutdownTimeout  time.Duration `yaml:"shutdownTimeout"`
	RecipeCacheBytes int64         `yaml:"recipeCacheBytes"`
	MaxPreparations  int           `yaml:"maxPreparations"`
	MaxStrategies    int           `yaml:"maxStrategies"`
}
type Curator struct {
	URL              string `yaml:"url"`
	APIKeyEnv        string `yaml:"apiKeyEnv,omitempty"`
	TimeoutSeconds   int    `yaml:"timeoutSeconds"`
	MaxResponseBytes int64  `yaml:"maxResponseBytes"`
}

func (c Curator) HTTPConfig() protocolio.HTTPConfig {
	return protocolio.HTTPConfig{URL: c.URL, APIKeyEnv: c.APIKeyEnv, TimeoutSeconds: c.TimeoutSeconds, MaxResponseBytes: c.MaxResponseBytes}
}

type Chain struct {
	RPCURL      string `yaml:"rpcUrl"`
	WriteRPCURL string `yaml:"writeRpcUrl"`
	ChainID     uint64 `yaml:"chainId"`
	Multicall3  string `yaml:"multicall3"`
}
type Signer struct {
	Address string `yaml:"address"`
	KeyEnv  string `yaml:"keyEnv"`
}
type Observability struct {
	Addr  string `yaml:"addr"`
	Debug bool   `yaml:"debug"`
}

func decode(raw []byte, out any) error {
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return errors.New("malformed YAML configuration")
	}
	var inspect func(*yaml.Node) error
	inspect = func(n *yaml.Node) error {
		if n.Kind == yaml.AliasNode || n.Anchor != "" {
			return errors.New("YAML anchors and aliases are unsupported")
		}
		for _, child := range n.Content {
			if err := inspect(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := inspect(&node); err != nil {
		return err
	}
	d := yaml.NewDecoder(bytes.NewReader(raw))
	d.KnownFields(true)
	if err := d.Decode(out); err != nil {
		return errors.New("invalid or unknown configuration field")
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("expected one YAML document")
	}
	return nil
}
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Errorf("read configuration: %w", err)
	}
	var cfg Config
	if err := decode(defaults, &cfg); err != nil {
		return nil, errors.Errorf("decode runtime defaults: %w", err)
	}
	if err := decode(raw, &cfg); err != nil {
		return nil, err
	}
	cfg.Chain.RPCURL = os.ExpandEnv(cfg.Chain.RPCURL)
	cfg.Chain.WriteRPCURL = os.ExpandEnv(cfg.Chain.WriteRPCURL)
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(cfg.StateDir) {
		cfg.StateDir = filepath.Join(base, cfg.StateDir)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
func (c *Config) Validate() error {
	if err := permissions.ValidateConfig(c.Curator.HTTPConfig()); err != nil {
		return err
	}
	if c.Curator.TimeoutSeconds <= 0 || c.Curator.MaxResponseBytes <= 0 {
		return errors.New("positive curator resource bounds required")
	}
	if c.Mode != "simulate" && c.Mode != "send" {
		return errors.New("mode must be simulate or send")
	}
	if c.Chain.ChainID == 0 {
		return errors.New("positive chainId required")
	}
	for _, endpoint := range []string{c.Chain.RPCURL, c.Chain.WriteRPCURL} {
		if endpoint == "" {
			continue
		}
		u, err := url.Parse(endpoint)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return errors.New("RPC endpoint must be HTTP(S)")
		}
	}
	if c.Chain.RPCURL == "" {
		return errors.New("rpcUrl required")
	}
	if _, err := parse.Address(c.Chain.Multicall3); err != nil {
		return errors.New("chain.multicall3 requires a nonzero address")
	}
	if c.Signer.Address != "" {
		if _, err := parse.Address(c.Signer.Address); err != nil {
			return errors.New("signer.address requires a nonzero address")
		}
	}
	if c.Mode == "send" && c.Signer.KeyEnv == "" {
		return errors.New("send mode requires signer.keyEnv")
	}
	if c.StateDir == "" {
		return errors.New("stateDir required")
	}
	if c.Runtime.PollInterval <= 0 || c.Runtime.OperationTimeout <= 0 || c.Runtime.HTTPTimeout <= 0 || c.Runtime.StrategyInterval < 0 || c.Runtime.ShutdownTimeout <= 0 || c.Runtime.RecipeCacheBytes < 0 || c.Runtime.MaxPreparations <= 0 || c.Runtime.MaxStrategies <= 0 {
		return errors.New("invalid runtime resource policy")
	}
	if err := c.Reads.Validate(); err != nil {
		return err
	}
	if c.Transaction.MaxFeeGwei < 0 || math.IsNaN(c.Transaction.MaxFeeGwei) || math.IsInf(c.Transaction.MaxFeeGwei, 0) {
		return errors.New("transaction.maxFeeGwei must be finite and nonnegative")
	}
	if c.Mode == "send" {
		if err := c.Transaction.Validate(); err != nil {
			return err
		}
	} else if err := c.Transaction.ValidatePolicy(); err != nil {
		return err
	}
	if len(c.Strategies) < 1 || len(c.Strategies) > c.Runtime.MaxStrategies {
		return errors.New("executor count exceeds runtime.maxStrategies")
	}
	seen := make(map[string]bool, len(c.Strategies))
	for i, value := range c.Strategies {
		executor, err := parse.Address(value)
		if err != nil {
			return errors.Errorf("strategy %d: invalid executor address", i+1)
		}
		canonical := strings.ToLower(executor.Hex())
		if seen[canonical] {
			return errors.New("duplicate executor address")
		}
		seen[canonical] = true
		c.Strategies[i] = canonical
	}
	return nil
}
func (c *Config) StrategyFingerprint(executor string, account, caller common.Address) string {
	raw, _ := json.Marshal(struct {
		Policy                    string
		Executor, Account, Caller common.Address
		ChainID                   uint64
		Curator                   Curator
		Transaction               transaction.Config
	}{"2", common.HexToAddress(executor), account, caller, c.Chain.ChainID, c.Curator, c.Transaction})
	return crypto.Keccak256Hash(raw).Hex()
}

// LegacyStrategyFingerprint preserves the exact v1 serializer for offline import.
func (c *Config) LegacyStrategyFingerprint(executor string, account, caller common.Address) string {
	type oldCurator struct {
		URL       string `yaml:"url"`
		APIKeyEnv string `yaml:"apiKeyEnv,omitempty"`
	}
	type oldTx struct {
		MaxFeeGwei float64 `yaml:"maxFeeGwei"`
	}
	raw, _ := json.Marshal(struct {
		Policy                    string
		Executor, Account, Caller common.Address
		ChainID                   uint64
		Curator                   oldCurator
		TxManager                 oldTx
	}{"1", common.HexToAddress(executor), account, caller, c.Chain.ChainID, oldCurator{c.Curator.URL, c.Curator.APIKeyEnv}, oldTx{c.Transaction.MaxFeeGwei}})
	return crypto.Keccak256Hash(raw).Hex()
}
