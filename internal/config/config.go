// Package config decodes strict file-driven runtime configuration.
package config

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/permissions"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type Config struct {
	IndexerURL    string        `yaml:"indexerUrl"`
	Curator       Curator       `yaml:"curator"`
	Mode          string        `yaml:"mode"`
	Chain         Chain         `yaml:"chain"`
	Signer        Signer        `yaml:"signer"`
	TxManager     TxManager     `yaml:"txManager"`
	StateDir      string        `yaml:"stateDir"`
	Observability Observability `yaml:"observability"`
	Strategies    []string      `yaml:"strategies"`
}

// Curator contains the operator-supplied permission service settings.
type Curator struct {
	URL       string `yaml:"url"`
	APIKeyEnv string `yaml:"apiKeyEnv,omitempty"`
}

func (c Curator) HTTPConfig() protocolio.HTTPConfig {
	return protocolio.HTTPConfig{URL: c.URL, APIKeyEnv: c.APIKeyEnv, TimeoutSeconds: permissions.HostTimeoutSeconds, MaxResponseBytes: permissions.HostMaxResponseBytes}
}

type Chain struct {
	RPCURL      string `yaml:"rpcUrl"`
	WriteRPCURL string `yaml:"writeRpcUrl"`
	ChainID     uint64 `yaml:"chainId"`
}
type Signer struct {
	Address string `yaml:"address"`
	KeyEnv  string `yaml:"keyEnv"`
}
type Observability struct {
	Addr  string `yaml:"addr"`
	Debug bool   `yaml:"debug"`
}

const (
	// RuntimePolicyVersion changes when built-in preparation or timing semantics change.
	RuntimePolicyVersion                  = "1"
	DefaultKeyEnv                         = "MERKLE_EXECUTOR_PRIVATE_KEY"
	DefaultRecipeCacheBytes        int64  = 16 << 20
	DefaultPollInterval                   = 15 * time.Second
	DefaultOperationTimeout               = 30 * time.Second
	DefaultHTTPTimeout                    = 15 * time.Second
	DefaultStrategyInterval               = time.Minute
	DefaultConfirmations           uint64 = 2
	DefaultTransactionPollInterval        = 2 * time.Second
	StandardMulticall3                    = protocolio.StandardMulticall3
)

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Errorf("read configuration: %w", err)
	}
	var cfg Config
	d := yaml.NewDecoder(bytes.NewReader(raw))
	d.KnownFields(true)
	if err := d.Decode(&cfg); err != nil {
		return nil, errors.Errorf("decode configuration: %w", err)
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("expected one YAML document")
	}
	// Only operational endpoints are expanded. Secret-bearing provider configuration
	// keeps environment variable names, never interpolated values.
	cfg.Chain.RPCURL = os.ExpandEnv(cfg.Chain.RPCURL)
	cfg.Chain.WriteRPCURL = os.ExpandEnv(cfg.Chain.WriteRPCURL)
	if cfg.Mode == "" {
		cfg.Mode = "simulate"
	}
	if cfg.Signer.KeyEnv == "" {
		cfg.Signer.KeyEnv = DefaultKeyEnv
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if cfg.StateDir == "" {
		cfg.StateDir = "state"
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
	if _, err := protocolio.NewHTTP(protocolio.HTTPConfig{URL: c.IndexerURL}, nil, nil); err != nil {
		return errors.New("indexerUrl must be a credential-free HTTP(S) base URL")
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
	if c.Signer.Address != "" {
		if _, err := parse.Address(c.Signer.Address); err != nil {
			return errors.New("signer.address must be a nonzero address when used as an assertion")
		}
	}
	if len(c.Strategies) == 0 || len(c.Strategies) > 256 {
		return errors.New("configure 1..256 strategies")
	}
	if c.Mode == "send" {
		if c.Signer.KeyEnv == "" {
			return errors.New("send mode requires signer.keyEnv")
		}
	}
	if err := c.TxManager.validate(c.Mode == "send"); err != nil {
		return err
	}
	seen := make(map[string]bool, len(c.Strategies))
	for i, value := range c.Strategies {
		executor, err := parse.Address(value)
		if err != nil {
			return errors.Errorf("strategy %d: invalid executor address", i+1)
		}
		canonical := strings.ToLower(executor.Hex())
		if seen[canonical] {
			return errors.Errorf("duplicate executor address %s", canonical)
		}
		seen[canonical] = true
		c.Strategies[i] = canonical
	}
	return nil
}

// StrategyFingerprint binds the canonical executor, derived deployment identity
// and effective runtime policy to durable work.
func (c *Config) StrategyFingerprint(executor string, account, caller common.Address) string {
	raw, _ := json.Marshal(struct {
		Policy     string
		Executor   common.Address
		Account    common.Address
		Caller     common.Address
		ChainID    uint64
		Curator    Curator
		TxManager  TxManager
		IndexerURL string
	}{RuntimePolicyVersion, common.HexToAddress(executor), account, caller, c.Chain.ChainID, c.Curator, c.TxManager, c.IndexerURL})
	return crypto.Keccak256Hash(raw).Hex()
}
