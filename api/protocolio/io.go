// Package protocolio provides bounded, redacted I/O for protocol adapters.
package protocolio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/internal/provider"
)

// HTTPConfig never accepts literal credentials. All errors omit URLs, response bodies and headers.
type HTTPConfig struct {
	URL              string `yaml:"url"`
	APIKeyEnv        string `yaml:"apiKeyEnv"`
	TimeoutSeconds   int    `yaml:"timeoutSeconds"`
	MaxResponseBytes int64  `yaml:"maxResponseBytes"`
}
type Client struct {
	base    string
	bearer  string
	http    *http.Client
	limit   int64
	timeout time.Duration
}

func NewHTTP(c HTTPConfig, injected *http.Client) (*Client, error) {
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("protocol HTTP URL must be an http(s) base URL without credentials, query or fragment")
	}
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 10
	}
	if c.TimeoutSeconds < 1 || c.TimeoutSeconds > 120 {
		return nil, errors.New("HTTP timeout must be 1..120 seconds")
	}
	if c.MaxResponseBytes == 0 {
		c.MaxResponseBytes = 2 << 20
	}
	if c.MaxResponseBytes < 1 || c.MaxResponseBytes > 16<<20 {
		return nil, errors.New("HTTP response bound must be 1..16777216 bytes")
	}
	hc := http.Client{}
	if injected != nil {
		hc = *injected
	}
	hc.Timeout = time.Duration(c.TimeoutSeconds) * time.Second
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	bearer := ""
	if c.APIKeyEnv != "" {
		bearer = os.Getenv(c.APIKeyEnv)
		if bearer == "" || strings.ContainsAny(bearer, "\r\n") {
			return nil, errors.New("configured protocol API key is missing or invalid")
		}
	}
	return &Client{base: strings.TrimRight(c.URL, "/"), bearer: bearer, http: &hc, limit: c.MaxResponseBytes, timeout: hc.Timeout}, nil
}

// JSON returns a status even on non-2xx replies, allowing callers to distinguish 404 from an uncertain request.
func (c *Client) JSON(ctx context.Context, method, path string, in, out any) (int, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return 0, errors.New("invalid protocol API path")
	}
	var body io.Reader
	if in != nil {
		b, e := json.Marshal(in)
		if e != nil {
			return 0, errors.New("encode protocol request")
		}
		body = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if e != nil {
		return 0, errors.New("create protocol request")
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	resp, e := c.http.Do(req)
	if e != nil {
		return 0, errors.New("protocol HTTP transport failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, errors.Errorf("protocol HTTP status %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, c.limit+1))
	if e != nil {
		return resp.StatusCode, errors.New("read protocol response")
	}
	if int64(len(b)) > c.limit {
		return resp.StatusCode, errors.New("protocol response exceeds configured bound")
	}
	if out != nil {
		if e = json.Unmarshal(b, out); e != nil {
			return resp.StatusCode, errors.New("invalid protocol JSON response")
		}
	}
	return resp.StatusCode, nil
}
func Snapshot(s provider.Snapshot) error {
	if s.ChainID == nil || s.ChainID.Sign() <= 0 || s.ChainID.BitLen() > 256 || s.BlockNumber == nil || s.BlockNumber.Sign() < 0 || s.Account == (common.Address{}) {
		return errors.New("provider requires a pinned chain/account snapshot")
	}
	return nil
}

// CallData performs a pinned raw read; callers use generated typed Pack/Unpack methods.
func CallData(ctx context.Context, r provider.Reader, s provider.Snapshot, to common.Address, data []byte) ([]byte, error) {
	if r == nil {
		return nil, errors.New("protocol chain reader required")
	}
	if e := Snapshot(s); e != nil {
		return nil, e
	}
	if len(data) < 4 {
		return nil, errors.New("protocol call requires a method selector")
	}
	raw, e := r.CallContract(ctx, ethereum.CallMsg{To: &to, Data: data}, s.BlockNumber)
	if e != nil {
		return nil, errors.New("protocol chain read failed")
	}
	return raw, nil
}

func Hash(s string) (common.Hash, error) {
	b, e := hexutil.Decode(s)
	if e != nil || len(b) != 32 {
		return common.Hash{}, errors.New("expected bytes32 hex")
	}
	return common.BytesToHash(b), nil
}
func Bytes(s string, max int) ([]byte, error) {
	b, e := hexutil.Decode(s)
	if e != nil || len(b) > max {
		return nil, errors.New("invalid or oversized hex payload")
	}
	return b, nil
}

var actionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*):(0|[1-9][0-9]*)$`)

func Action(s string) error {
	if !actionPattern.MatchString(s) {
		return errors.New("action must be stepIndex:actionIndex")
	}
	return nil
}
func Word(n *big.Int) []byte              { return n.FillBytes(make([]byte, 32)) }
func AddressWord(a common.Address) []byte { return common.LeftPadBytes(a.Bytes(), 32) }
func Domain(name, version string, chainID *big.Int, contract common.Address) common.Hash {
	return crypto.Keccak256Hash(crypto.Keccak256([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)")), crypto.Keccak256([]byte(name)), crypto.Keccak256([]byte(version)), Word(chainID), AddressWord(contract))
}
func Digest(domain, structure common.Hash) common.Hash {
	return crypto.Keccak256Hash([]byte{0x19, 0x01}, domain.Bytes(), structure.Bytes())
}

// Canonical checks the pinned hash after external I/O and before external mutations.
// Numeric-block eth_call alone cannot detect an intervening reorganization.
func Canonical(ctx context.Context, r provider.Reader, s provider.Snapshot) error {
	if err := Snapshot(s); err != nil {
		return err
	}
	reader, ok := r.(interface {
		HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	})
	if !ok {
		return errors.New("protocol external phase requires canonical header reader")
	}
	if s.BlockHash == (common.Hash{}) {
		return errors.New("protocol snapshot block hash is required")
	}
	h, err := reader.HeaderByNumber(ctx, s.BlockNumber)
	if err != nil || h == nil || h.Number == nil || h.Number.Cmp(s.BlockNumber) != 0 || h.Hash() != s.BlockHash || h.Time != s.Timestamp {
		return errors.New("protocol pinned snapshot is no longer canonical")
	}
	return nil
}
