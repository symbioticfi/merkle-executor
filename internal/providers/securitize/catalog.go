package securitize

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	"github.com/symbioticfi/merkle-executor/sdk/parse"
)

// Catalog is an operator-reviewed snapshot, not permission to adopt feed changes.
type Catalog struct {
	Version    int     `yaml:"version"`
	Source     string  `yaml:"source"`
	ObservedAt string  `yaml:"observedAt"`
	Assets     []Asset `yaml:"assets"`
}

type Asset struct {
	Symbol           string  `yaml:"symbol"`
	ChainID          uint64  `yaml:"chainId"`
	Token            string  `yaml:"token"`
	Decimals         uint8   `yaml:"decimals"`
	RedemptionWallet string  `yaml:"redemptionWallet"`
	Offramps         []Route `yaml:"offramps"`
	Unavailable      string  `yaml:"unavailable,omitempty"`
}

type Route struct {
	Mode          string `yaml:"mode"`
	Target        string `yaml:"target"`
	PaymentToken  string `yaml:"paymentToken"`
	PaymentSymbol string `yaml:"paymentSymbol"`
}

// LoadCatalog reads a local YAML snapshot. No network response can change targets.
func LoadCatalog(path string) (Catalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return Catalog{}, errors.Errorf("open Securitize catalog: %w", err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return Catalog{}, errors.New("Securitize catalog unreadable or larger than 1 MiB")
	}
	var c Catalog
	d := yaml.NewDecoder(bytes.NewReader(raw))
	d.KnownFields(true)
	if err = d.Decode(&c); err != nil {
		return c, errors.Errorf("decode Securitize catalog: %w", err)
	}
	var extra any
	if err = d.Decode(&extra); !errors.Is(err, io.EOF) {
		return c, errors.New("Securitize catalog must contain one YAML document")
	}
	if c.Version != 1 || c.Source == "" || c.ObservedAt == "" || len(c.Assets) == 0 {
		return c, errors.New("invalid Securitize catalog metadata")
	}
	seen := make(map[string]map[uint64]bool)
	for _, a := range c.Assets {
		if a.Symbol == "" || a.ChainID == 0 || a.Decimals > 77 || seen[a.Symbol][a.ChainID] {
			return c, errors.New("invalid or duplicate Securitize catalog asset")
		}
		if seen[a.Symbol] == nil {
			seen[a.Symbol] = make(map[uint64]bool)
		}
		seen[a.Symbol][a.ChainID] = true
		if _, err = parse.Address(a.Token); err != nil {
			return c, errors.New("invalid Securitize catalog token")
		}
		if a.RedemptionWallet != "" {
			if _, err = parse.Address(a.RedemptionWallet); err != nil {
				return c, errors.New("invalid Securitize redemption wallet")
			}
		}
		if (a.RedemptionWallet == "" && len(a.Offramps) == 0) != (a.Unavailable != "") {
			return c, errors.New("Securitize catalog unavailable status does not match routes")
		}
		for _, r := range a.Offramps {
			if r.Mode != "modern" && r.Mode != "legacy" {
				return c, errors.New("invalid Securitize catalog offramp mode")
			}
			if _, err = parse.Address(r.Target); err != nil {
				return c, errors.New("invalid Securitize catalog offramp target")
			}
			if _, err = parse.Address(r.PaymentToken); err != nil {
				return c, errors.New("invalid Securitize catalog payment token")
			}
		}
	}
	return c, nil
}

func (p *Provider) validateCatalog() error {
	if p.cfg.Catalog == "" && p.cfg.Symbol == "" {
		return nil
	}
	if p.cfg.Catalog == "" || p.cfg.Symbol == "" {
		return errors.New("Securitize catalog and symbol must be configured together")
	}
	path := p.cfg.Catalog
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.deps.ConfigDir, path)
	}
	c, err := LoadCatalog(path)
	if err != nil {
		return err
	}
	for _, a := range c.Assets {
		if a.Symbol != p.cfg.Symbol || a.ChainID != p.cfg.ChainID {
			continue
		}
		token, _ := parse.Address(a.Token)
		if token != p.token || a.Unavailable != "" {
			return errors.New("Securitize catalog token mismatch or unavailable issuer route")
		}
		p.decimals = &a.Decimals
		if p.cfg.Mode == "wallet" {
			wallet, err := parse.Address(a.RedemptionWallet)
			if err == nil && wallet == p.target {
				return nil
			}
		}
		for _, r := range a.Offramps {
			target, _ := parse.Address(r.Target)
			payment, _ := parse.Address(r.PaymentToken)
			if r.Mode == p.cfg.Mode && target == p.target && payment == p.payment {
				return nil
			}
		}
		return errors.New("Securitize route does not match pinned catalog")
	}
	return errors.New("Securitize symbol and chain not found in catalog")
}
