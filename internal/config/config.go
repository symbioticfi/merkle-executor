// Package config decodes file-driven configuration and opaque provider settings.
package config

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/permissions"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type Config struct {
	Curator          protocolio.HTTPConfig `yaml:"curator"`
	RecipeCacheBytes int64                 `yaml:"recipeCacheBytes"`
	Mode             string                `yaml:"mode"`
	Chain            Chain                 `yaml:"chain"`
	Signer           Signer                `yaml:"signer"`
	TxManager        TxManager             `yaml:"txManager"`
	StateDir         string                `yaml:"stateDir"`
	PollInterval     time.Duration         `yaml:"pollInterval"`
	OperationTimeout time.Duration         `yaml:"operationTimeout"`
	HTTPTimeout      time.Duration         `yaml:"httpTimeout"`
	Observability    Observability         `yaml:"observability"`
	Strategies       []Strategy            `yaml:"strategies"`
}

type Chain struct {
	Multicall3  string `yaml:"multicall3"`
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
type Provider struct {
	Name   string    `yaml:"name"`
	Config yaml.Node `yaml:"config"`
}
type Strategy struct {
	Name       string        `yaml:"name"`
	Executor   string        `yaml:"executor"`
	RecipeHash string        `yaml:"recipeHash"`
	Interval   time.Duration `yaml:"interval"`
	Providers  []Provider    `yaml:"providers"`
}

const (
	// RuntimePolicyVersion changes when built-in preparation or timing semantics change.
	RuntimePolicyVersion                  = "recipe-derived-v2"
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

var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Errorf("read configuration: %w", err)
	}
	cfg := Config{RecipeCacheBytes: DefaultRecipeCacheBytes}
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
	if cfg.PollInterval == 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.OperationTimeout == 0 {
		cfg.OperationTimeout = DefaultOperationTimeout
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = DefaultHTTPTimeout
	}
	if cfg.TxManager.PollInterval == 0 {
		cfg.TxManager.PollInterval = DefaultTransactionPollInterval
	}
	if cfg.TxManager.Confirmations == 0 {
		cfg.TxManager.Confirmations = DefaultConfirmations
	}
	cfg.TxManager.applyDefaults()
	if cfg.Signer.KeyEnv == "" {
		cfg.Signer.KeyEnv = DefaultKeyEnv
	}
	if cfg.Chain.Multicall3 == "" {
		cfg.Chain.Multicall3 = StandardMulticall3
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
	for i := range cfg.Strategies {
		if cfg.Strategies[i].Interval == 0 {
			cfg.Strategies[i].Interval = DefaultStrategyInterval
		}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	curator, err := permissions.NormalizeHostConfig(c.Curator)
	if err != nil {
		return err
	}
	c.Curator = curator
	if c.RecipeCacheBytes < 0 {
		return errors.New("recipeCacheBytes must be nonnegative")
	}
	if c.Mode != "simulate" && c.Mode != "send" {
		return errors.New("mode must be simulate or send")
	}
	if c.Chain.Multicall3 != "" {
		if _, err := parse.Address(c.Chain.Multicall3); err != nil || common.HexToAddress(c.Chain.Multicall3) != common.HexToAddress(StandardMulticall3) {
			return errors.New("chain.multicall3 is fixed to the standard Multicall3 address")
		}
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
	if c.PollInterval <= 0 || c.OperationTimeout <= 0 || c.HTTPTimeout <= 0 || c.TxManager.PollInterval <= 0 {
		return errors.New("timeouts and intervals must be positive")
	}
	if len(c.Strategies) == 0 || len(c.Strategies) > 256 {
		return errors.New("configure 1..256 strategies")
	}
	if c.TxManager.Confirmations > 1024 {
		return errors.New("confirmation count exceeds limit")
	}
	if c.Mode == "send" {
		if c.Signer.KeyEnv == "" {
			return errors.New("send mode requires signer.keyEnv")
		}

	}
	if err := c.TxManager.validate(c.Mode == "send"); err != nil {
		return err
	}
	if c.RecipeCacheBytes != DefaultRecipeCacheBytes || c.PollInterval != DefaultPollInterval || c.OperationTimeout != DefaultOperationTimeout || c.HTTPTimeout != DefaultHTTPTimeout {
		return errors.New("recipe cache, polling and timeout settings are fixed by the runtime policy; remove legacy overrides")
	}
	if err := c.TxManager.validateRuntimePolicy(); err != nil {
		return err
	}
	names := make(map[string]bool)
	for i := range c.Strategies {
		s := &c.Strategies[i]
		executor, err := parse.Address(s.Executor)
		if err != nil {
			return errors.Errorf("strategy %s: invalid executor", s.Name)
		}
		if s.Name == "" {
			s.Name = "executor-" + strings.ToLower(executor.Hex())
		}
		if !validName.MatchString(s.Name) || names[s.Name] {
			return errors.New("strategy names must be unique filesystem-safe identifiers")
		}
		names[s.Name] = true
		if s.Interval != DefaultStrategyInterval {
			return errors.Errorf("strategy %s: interval is fixed to one minute; remove the legacy override", s.Name)
		}
		if s.RecipeHash != "" {
			b, err := hexutil.Decode(s.RecipeHash)
			if err != nil || len(b) != 32 {
				return errors.New("recipeHash must be bytes32")
			}
		}
		if len(s.Providers) != 0 {
			return errors.New("strategy.providers is obsolete; runtime values must be derived from the authenticated recipe")
		}
		for _, p := range s.Providers {
			if err := parse.RejectAliases(p.Config); err != nil {
				return err
			}
			if p.Name == "" {
				return errors.New("provider name required")
			}
		}
	}
	return nil
}

// Fingerprint binds durable work to semantic configuration. Addresses, provider
// mappings and endpoint selection cannot silently change during an active job.
func (s Strategy) Fingerprint() string {
	type semanticProvider struct {
		Name string
		Node any
	}
	ps := make([]semanticProvider, 0, len(s.Providers))
	for _, p := range s.Providers {
		ps = append(ps, semanticProvider{p.Name, nodeIdentity(p.Config)})
	}
	raw, _ := json.Marshal(struct {
		Name, Executor, RecipeHash string
		Interval                   time.Duration
		Providers                  []semanticProvider
	}{s.Name, s.Executor, s.RecipeHash, s.Interval, ps})
	return crypto.Keccak256Hash(raw).Hex()
}

// StrategyFingerprint binds immutable derived identity and source settings to
// durable work. Migration never rewrites an existing journal's fingerprint.
func (c *Config) StrategyFingerprint(s Strategy, account common.Address) string {
	raw, _ := json.Marshal(struct {
		Policy    string
		Strategy  string
		Account   common.Address
		Curator   protocolio.HTTPConfig
		TxManager TxManager
	}{RuntimePolicyVersion, s.Fingerprint(), account, c.Curator, c.TxManager})
	return crypto.Keccak256Hash(raw).Hex()
}

func nodeIdentity(n yaml.Node) any {
	children := make([]any, len(n.Content))
	for i, c := range n.Content {
		children[i] = nodeIdentity(*c)
	}
	return struct {
		Kind       yaml.Kind
		Tag, Value string
		Children   []any
	}{n.Kind, n.Tag, n.Value, children}
}
