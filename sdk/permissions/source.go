// Package permissions retrieves root-addressed permission artifacts for recipe validation.
package permissions

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// Fixed host policy matches protocolio.NewHTTP's established defaults. Normal
// service hosts normalize omitted fields and reject legacy transport overrides.
const HostTimeoutSeconds = 10
const HostMaxResponseBytes int64 = 2 << 20

func NormalizeHostConfig(c protocolio.HTTPConfig) (protocolio.HTTPConfig, error) {
	if err := ValidateConfig(c); err != nil {
		return protocolio.HTTPConfig{}, err
	}
	if c.TimeoutSeconds != 0 && c.TimeoutSeconds != HostTimeoutSeconds {
		return protocolio.HTTPConfig{}, errors.New("curator.timeoutSeconds is fixed to 10")
	}
	if c.MaxResponseBytes != 0 && c.MaxResponseBytes != HostMaxResponseBytes {
		return protocolio.HTTPConfig{}, errors.New("curator.maxResponseBytes is fixed to 2097152")
	}
	c.TimeoutSeconds = HostTimeoutSeconds
	c.MaxResponseBytes = HostMaxResponseBytes
	return c, nil
}

// ValidateConfig performs no network or environment access. Construction with
// New reads the configured credential only when a run/plan actually starts.
func ValidateConfig(c protocolio.HTTPConfig) error {
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("curator.url must be an HTTP(S) base URL without credentials, query or fragment")
	}
	if c.TimeoutSeconds < 0 || c.TimeoutSeconds > 120 {
		return errors.New("curator.timeoutSeconds must be 0..120")
	}
	if c.MaxResponseBytes < 0 || c.MaxResponseBytes > 16<<20 {
		return errors.New("curator.maxResponseBytes must be 0..16777216")
	}
	return nil
}

type Source struct{ client *protocolio.Client }

func New(c protocolio.HTTPConfig, h *http.Client) (*Source, error) {
	if err := ValidateConfig(c); err != nil {
		return nil, err
	}
	client, err := protocolio.NewHTTP(c, h)
	if err != nil {
		return nil, err
	}
	return &Source{client: client}, nil
}
func (s *Source) PermissionTree(ctx context.Context, chainID *big.Int, snapshot recipes.Snapshot) ([]byte, error) {
	if chainID == nil || chainID.Sign() <= 0 {
		return nil, errors.New("permission chain ID required")
	}
	var raw json.RawMessage
	_, err := s.client.JSON(ctx, http.MethodGet, "/merkle-trees/"+snapshot.Root.Hex()+"?chainId="+chainID.String(), nil, &raw)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
