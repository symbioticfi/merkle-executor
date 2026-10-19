package securitize

import (
	"bytes"
	"io"

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

func decodeCatalog(raw []byte) (Catalog, error) {
	var err error
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
