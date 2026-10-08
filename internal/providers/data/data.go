// Package data supplies protocol-neutral static, RPC and HTTP runtime inputs.
package data

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type StaticConfig struct {
	Inputs    map[uint]string   `yaml:"inputs"`
	Actions   map[string]string `yaml:"actions"`
	SkipSteps []uint            `yaml:"skipSteps"`
}

type Read struct {
	Target string `yaml:"target"`
	Data   string `yaml:"data"`
	Offset uint   `yaml:"offset"`
	Input  uint   `yaml:"input"`
}

// Word binds a resolved input to one ABI word in a configured action template.
type Word struct {
	Offset uint `yaml:"offset"`
	Input  uint `yaml:"input"`
}

type Action struct {
	Data  string `yaml:"data"`
	Words []Word `yaml:"words"`
}

type RPCConfig struct {
	Reads   []Read            `yaml:"reads"`
	Actions map[string]Action `yaml:"actions"`
}

// HTTPConfig maps RFC6901 pointers into a bounded JSON response. Endpoint and
// request are operator configuration; neither can be supplied by remote output.
type HTTPConfig struct {
	URL              string            `yaml:"url"`
	Method           string            `yaml:"method"`
	Body             string            `yaml:"body"`
	HeadersEnv       map[string]string `yaml:"headersEnv"`
	Inputs           map[uint]string   `yaml:"inputs"`
	Actions          map[string]string `yaml:"actions"`
	MaxResponseBytes int64             `yaml:"maxResponseBytes"`
}

type source struct {
	static *StaticConfig
	rpc    *RPCConfig
	http   *HTTPConfig
	deps   providerRegistry.Deps
}

func Register(r *providerRegistry.Registry) error {
	for name, f := range map[string]providerRegistry.Factory{"static": staticFactory, "rpc": rpcFactory, "http": httpFactory} {
		if err := r.Register(name, f); err != nil {
			return err
		}
	}
	return nil
}

func staticFactory(raw yaml.Node, deps providerRegistry.Deps) (provider.Provider, error) {
	var cfg StaticConfig
	if err := parse.DecodeStrict(raw, &cfg); err != nil {
		return nil, err
	}
	for _, value := range cfg.Inputs {
		if _, err := parse.Uint256(value); err != nil {
			return nil, err
		}
	}
	return &source{static: &cfg, deps: deps}, nil
}

func rpcFactory(raw yaml.Node, deps providerRegistry.Deps) (provider.Provider, error) {
	var cfg RPCConfig
	if err := parse.DecodeStrict(raw, &cfg); err != nil {
		return nil, err
	}
	if deps.Chain == nil || len(cfg.Reads) == 0 || len(cfg.Reads) > 256 {
		return nil, errors.New("rpc requires a reader and 1..256 reads")
	}
	seen := make(map[uint]bool)
	for _, read := range cfg.Reads {
		if seen[read.Input] || read.Input > 4095 {
			return nil, errors.New("duplicate or excessive RPC input index")
		}
		seen[read.Input] = true
	}
	return &source{rpc: &cfg, deps: deps}, nil
}

func httpFactory(raw yaml.Node, deps providerRegistry.Deps) (provider.Provider, error) {
	var cfg HTTPConfig
	if err := parse.DecodeStrict(raw, &cfg); err != nil {
		return nil, err
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	if cfg.Method != http.MethodGet && cfg.Method != http.MethodPost {
		return nil, errors.New("data HTTP method must be GET or POST")
	}
	u, err := url.Parse(cfg.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return nil, errors.New("invalid data endpoint")
	}
	if deps.HTTP == nil {
		return nil, errors.New("HTTP client required")
	}
	if cfg.MaxResponseBytes == 0 {
		cfg.MaxResponseBytes = 1 << 20
	}
	if cfg.MaxResponseBytes < 1 || cfg.MaxResponseBytes > 16<<20 {
		return nil, errors.New("invalid HTTP response bound")
	}
	if len(cfg.Inputs) == 0 && len(cfg.Actions) == 0 {
		return nil, errors.New("HTTP output mappings required")
	}
	return &source{http: &cfg, deps: deps}, nil
}

func (s *source) Prepare(ctx context.Context, snap provider.Snapshot) (provider.Output, error) {
	out := provider.Output{Inputs: make(map[uint]string), Actions: make(map[string]hexutil.Bytes)}
	if s.static != nil {
		for index, value := range s.static.Inputs {
			out.Inputs[index] = value
		}
		for key, value := range s.static.Actions {
			data, err := hexutil.Decode(expand(value, snap))
			if err != nil {
				return out, errors.Errorf("static action %s: invalid hex", key)
			}
			out.Actions[key] = data
		}
		out.SkipSteps = append([]uint(nil), s.static.SkipSteps...)
		return out, nil
	}
	if s.rpc != nil {
		return s.readRPC(ctx, snap, out)
	}
	return s.readHTTP(ctx, snap, out)
}

func (s *source) readRPC(ctx context.Context, snap provider.Snapshot, out provider.Output) (provider.Output, error) {
	for _, read := range s.rpc.Reads {
		target, err := parse.Address(expand(read.Target, snap))
		if err != nil {
			return out, err
		}
		data, err := hexutil.Decode(expand(read.Data, snap))
		if err != nil {
			return out, errors.New("invalid RPC calldata")
		}
		result, err := s.deps.Chain.CallContract(ctx, ethereum.CallMsg{From: snap.Account, To: &target, Data: data}, snap.BlockNumber)
		if err != nil {
			return out, errors.New("RPC data read failed")
		}
		if read.Offset > uint(len(result)) || uint(len(result))-read.Offset < 32 {
			return out, errors.New("RPC result word out of bounds")
		}
		out.Inputs[read.Input] = new(big.Int).SetBytes(result[read.Offset : read.Offset+32]).String()
	}
	for key, action := range s.rpc.Actions {
		data, err := hexutil.Decode(expand(action.Data, snap))
		if err != nil {
			return out, errors.New("invalid RPC action template")
		}
		end := uint(4)
		for _, word := range action.Words {
			if word.Offset < end || word.Offset > uint(len(data)) || uint(len(data))-word.Offset < 32 {
				return out, errors.New("invalid action word offset")
			}
			value, err := parse.Uint256(out.Inputs[word.Input])
			if err != nil {
				return out, errors.New("action refers to missing RPC input")
			}
			value.FillBytes(data[word.Offset : word.Offset+32])
			end = word.Offset + 32
		}
		out.Actions[key] = data
	}
	return out, nil
}

func (s *source) readHTTP(ctx context.Context, snap provider.Snapshot, out provider.Output) (provider.Output, error) {
	cfg := s.http
	body := expand(cfg.Body, snap)
	if body != "" && !json.Valid([]byte(body)) {
		return out, errors.New("HTTP data body must be JSON")
	}
	req, err := http.NewRequestWithContext(ctx, cfg.Method, expand(cfg.URL, snap), strings.NewReader(body))
	if err != nil {
		return out, errors.New("invalid HTTP data request")
	}
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for header, env := range cfg.HeadersEnv {
		value, exists := os.LookupEnv(env)
		if !exists || value == "" {
			return out, errors.Errorf("missing HTTP header environment variable %s", env)
		}
		req.Header.Set(header, value)
	}
	resp, err := s.deps.HTTP.Do(req)
	if err != nil {
		return out, errors.New("HTTP data request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, errors.Errorf("HTTP data status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, cfg.MaxResponseBytes+1))
	if err != nil || int64(len(raw)) > cfg.MaxResponseBytes {
		return out, errors.New("HTTP data response unreadable or too large")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return out, errors.New("invalid HTTP data JSON")
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return out, errors.New("trailing HTTP data JSON")
	}
	for index, pointer := range cfg.Inputs {
		v, err := jsonPointer(value, pointer)
		if err != nil {
			return out, err
		}
		text, ok := v.(string)
		if !ok {
			if n, numeric := v.(json.Number); numeric {
				text = n.String()
			} else {
				return out, errors.New("HTTP input must be an integer string")
			}
		}
		if _, err := parse.Uint256(text); err != nil {
			return out, err
		}
		out.Inputs[index] = text
	}
	for key, pointer := range cfg.Actions {
		v, err := jsonPointer(value, pointer)
		if err != nil {
			return out, err
		}
		text, ok := v.(string)
		if !ok {
			return out, errors.New("HTTP action must be hex string")
		}
		data, err := hexutil.Decode(text)
		if err != nil {
			return out, errors.New("HTTP action is invalid hex")
		}
		out.Actions[key] = data
	}
	return out, nil
}

func (*source) Advance(context.Context, provider.Snapshot, provider.Output, json.RawMessage) (provider.Progress, error) {
	return provider.Progress{Done: true}, nil
}

func expand(value string, snap provider.Snapshot) string {
	chainID := ""
	if snap.ChainID != nil {
		chainID = snap.ChainID.String()
	}
	return strings.NewReplacer("{{account}}", strings.ToLower(snap.Account.Hex()), "{{executor}}", strings.ToLower(snap.Executor.Hex()), "{{chainId}}", chainID, "{{accountWord}}", strings.Repeat("0", 24)+strings.ToLower(snap.Account.Hex()[2:])).Replace(value)
}

func jsonPointer(value any, pointer string) (any, error) {
	if pointer == "" {
		return value, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("JSON pointer must start with /")
	}
	for _, part := range strings.Split(pointer[1:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch node := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = node[part]
			if !ok {
				return nil, errors.New("JSON pointer not found")
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node) {
				return nil, errors.New("JSON array pointer out of bounds")
			}
			value = node[index]
		default:
			return nil, errors.New("JSON pointer traverses scalar")
		}
	}
	return value, nil
}
