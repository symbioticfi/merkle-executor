// Package redemptions supplies issuer request and claim calldata for committed recipes.
package redemptions

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type Config struct {
	Family            string `yaml:"family"`
	Mode              string `yaml:"mode"`
	ChainID           uint64 `yaml:"chainId"`
	Token             string `yaml:"token"`
	OutputAsset       string `yaml:"outputAsset"`
	Target            string `yaml:"target"`
	Amount            string `yaml:"amount"`
	Action            string `yaml:"action"`
	SecondAction      string `yaml:"secondAction"`
	CDO               string `yaml:"cdo"`
	Strategy          string `yaml:"strategy"`
	Epoch             string `yaml:"epoch"`
	RequestID         string `yaml:"requestId"`
	AmountInput       *uint  `yaml:"amountInput"`
	IntermediateInput *uint  `yaml:"intermediateInput"`
}

type Provider struct {
	cfg                                 Config
	deps                                providerRegistry.Deps
	token, asset, target, cdo, strategy common.Address
	amount, epoch, requestID            *big.Int
}

func Register(registry *providerRegistry.Registry) error {
	return registry.Register("redemptions", Factory)
}

func Factory(raw yaml.Node, deps providerRegistry.Deps) (provider.Provider, error) {
	var cfg Config
	if err := parse.DecodeStrict(raw, &cfg); err != nil {
		return nil, err
	}
	if cfg.ChainID == 0 || deps.Chain == nil {
		return nil, errors.New("redemption chainId and chain reader required")
	}
	allowed := map[string][]string{
		"midas": {"request"}, "centrifuge": {"request", "claim"},
		"openeden": {"request"}, "figure": {"request"}, "noon": {"request", "claim", "redeem-usn"},
		"pareto": {"queue-request", "direct-request", "queue-claim", "direct-claim", "instant-claim"},
	}
	valid := false
	for _, mode := range allowed[cfg.Family] {
		valid = valid || mode == cfg.Mode
	}
	if !valid {
		return nil, errors.New("unsupported issuer family or redemption mode")
	}
	if err := protocolio.Action(cfg.Action); err != nil {
		return nil, err
	}
	p := &Provider{cfg: cfg, deps: deps}
	for _, item := range []struct {
		value string
		dst   *common.Address
	}{{cfg.Token, &p.token}, {cfg.OutputAsset, &p.asset}, {cfg.Target, &p.target}} {
		var err error
		*item.dst, err = parse.Address(item.value)
		if err != nil {
			return nil, err
		}
	}
	if p.token == p.asset {
		return nil, errors.New("redemption token and output asset must differ")
	}
	if cfg.Family == "noon" && cfg.Mode == "redeem-usn" {
		if cfg.ChainID != 1 || p.token != noonUSN || p.asset != noonUSDC || p.target != noonRedeemer {
			return nil, errors.New("native Noon redemption requires the reviewed Ethereum USN, USDC and RedeemHandlerV2 deployment")
		}
	}
	if cfg.Family == "pareto" {
		var err error
		if p.cdo, err = parse.Address(cfg.CDO); err != nil {
			return nil, err
		}
		if p.strategy, err = parse.Address(cfg.Strategy); err != nil {
			return nil, err
		}
	} else if cfg.CDO != "" || cfg.Strategy != "" {
		return nil, errors.New("cdo and strategy apply only to Pareto")
	}
	if cfg.Family == "figure" {
		if err := protocolio.Action(cfg.SecondAction); err != nil {
			return nil, err
		}
		if cfg.SecondAction == cfg.Action {
			return nil, errors.New("Figure unwrap and request actions must differ")
		}
	} else if cfg.SecondAction != "" || cfg.IntermediateInput != nil {
		return nil, errors.New("second action and intermediate input apply only to Figure")
	}
	if (cfg.Family != "pareto" && cfg.Family != "noon") || strings.HasSuffix(cfg.Mode, "request") || cfg.Mode == "redeem-usn" {
		var err error
		p.amount, err = parse.Uint256(cfg.Amount)
		if err != nil || p.amount.Sign() == 0 {
			return nil, errors.New("redemption amount must be positive uint256")
		}
	} else if cfg.Amount != "" {
		return nil, errors.New("issuer claim modes claim the complete request; omit amount")
	}
	if cfg.Mode == "queue-claim" {
		var err error
		p.epoch, err = parse.Uint256(cfg.Epoch)
		if err != nil {
			return nil, errors.New("Pareto queue claim requires an explicit uint256 epoch")
		}
	} else if cfg.Epoch != "" {
		return nil, errors.New("epoch applies only to Pareto queue claims")
	}
	if cfg.Family == "noon" && cfg.Mode == "claim" {
		var err error
		p.requestID, err = parse.Uint256(cfg.RequestID)
		if err != nil {
			return nil, errors.New("Noon claim requires an explicit uint256 requestId")
		}
	} else if cfg.RequestID != "" {
		return nil, errors.New("requestId applies only to Noon claims")
	}
	for _, index := range []*uint{cfg.AmountInput, cfg.IntermediateInput} {
		if index != nil && *index > parse.MaxInputIndex {
			return nil, errors.New("redemption input index exceeds limit")
		}
	}
	if cfg.AmountInput != nil && cfg.IntermediateInput != nil && *cfg.AmountInput == *cfg.IntermediateInput {
		return nil, errors.New("redemption input indices must differ")
	}
	return p, nil
}

func (p *Provider) Prepare(ctx context.Context, snapshot provider.Snapshot) (provider.Output, error) {
	if err := protocolio.Snapshot(snapshot); err != nil {
		return provider.Output{}, err
	}
	if snapshot.ChainID.Cmp(new(big.Int).SetUint64(p.cfg.ChainID)) != 0 {
		return provider.Output{}, errors.New("redemption chain mismatch")
	}
	out := provider.Output{Inputs: map[uint]string{}, Actions: map[string]hexutil.Bytes{}}
	var amount, intermediate *big.Int
	var err error
	switch p.cfg.Family {
	case "midas":
		amount, err = p.midas(ctx, snapshot, &out)
	case "centrifuge":
		amount, err = p.centrifuge(ctx, snapshot, &out)
	case "openeden":
		amount, err = p.openeden(ctx, snapshot, &out)
	case "figure":
		amount, intermediate, err = p.figure(ctx, snapshot, &out)
	case "pareto":
		amount, err = p.pareto(ctx, snapshot, &out)
	case "noon":
		if p.cfg.Mode == "redeem-usn" {
			amount, err = p.noonRedeemUSN(ctx, snapshot, &out)
		} else {
			amount, err = p.noon(ctx, snapshot, &out)
		}
	}
	if err != nil {
		return provider.Output{}, err
	}
	if p.cfg.AmountInput != nil {
		out.Inputs[*p.cfg.AmountInput] = amount.String()
	}
	if p.cfg.IntermediateInput != nil {
		out.Inputs[*p.cfg.IntermediateInput] = intermediate.String()
	}
	out.Data, err = json.Marshal(snapshot.Account)
	return out, err
}

// Requests finish at their confirmed submission. Issuer-owned queues retain the
// pending claim; a separate claim recipe can run without an account-lock deadlock.
func (p *Provider) Advance(ctx context.Context, snapshot provider.Snapshot, output provider.Output, _ json.RawMessage) (provider.Progress, error) {
	if err := protocolio.Execution(ctx, p.deps.Chain, snapshot); err != nil {
		return provider.Progress{}, err
	}
	var account common.Address
	if err := json.Unmarshal(output.Data, &account); err != nil || account != snapshot.Account || snapshot.ChainID.Cmp(new(big.Int).SetUint64(p.cfg.ChainID)) != 0 {
		return provider.Progress{}, errors.New("redemption execution identity mismatch")
	}
	target := p.target
	if p.cfg.Family == "noon" && p.cfg.Mode == "redeem-usn" {
		if err := p.noonRedeemExecuted(snapshot, output); err != nil {
			return provider.Progress{}, err
		}
	} else if p.cfg.Family == "figure" {
		if err := p.figureExecuted(snapshot, output); err != nil {
			return provider.Progress{}, err
		}
	} else {
		if p.cfg.Family == "pareto" && !strings.HasPrefix(p.cfg.Mode, "queue-") {
			target = p.cdo
		}
		if p.cfg.Family == "noon" && p.cfg.Mode == "request" {
			target = p.token
		}
		if err := protocolio.ExecutedCalls(snapshot, []protocolio.ManagedCall{{Target: target, Data: output.Actions[p.cfg.Action]}}); err != nil {
			return provider.Progress{}, err
		}
	}
	outcome := "requested"
	if strings.HasSuffix(p.cfg.Mode, "claim") {
		outcome = "claimed"
	} else if p.cfg.Mode == "redeem-usn" {
		outcome = "redeemed"
	}
	return provider.Progress{Done: true, Outcome: outcome, Reference: snapshot.Execution.TxHash.Hex()}, nil
}

func read[T any](ctx context.Context, p *Provider, s provider.Snapshot, target common.Address, data []byte, unpack func([]byte) (T, error)) (T, error) {
	var zero T
	raw, err := protocolio.CallData(ctx, p.deps.Chain, s, target, data)
	if err != nil {
		return zero, err
	}
	value, err := unpack(raw)
	if err != nil {
		return zero, errors.Errorf("decode issuer response: %w", err)
	}
	return value, nil
}

func readResult[T any](result protocolio.ReadResult, unpack func([]byte) (T, error)) (T, error) {
	var zero T
	raw, err := result.CallData()
	if err != nil {
		return zero, err
	}
	value, err := unpack(raw)
	if err != nil {
		return zero, errors.Errorf("decode issuer response: %w", err)
	}
	return value, nil
}
func checkRead(result protocolio.ReadResult, unpack func([]byte) (common.Address, error), expected common.Address) error {
	actual, err := readResult(result, unpack)
	if err != nil {
		return err
	}
	if actual != expected {
		return errors.New("issuer contract topology or asset mismatch")
	}
	return nil
}

func idle(reason string) error { return errors.Errorf("%w: %s", provider.ErrNotReady, reason) }

// ExclusiveTargets preserves each issuer action's receipt identity across
// providers and restarts. Figure owns both calls, including its bound output.
func (p *Provider) ExclusiveTargets() map[string]common.Address {
	if p.cfg.Family == "figure" {
		return map[string]common.Address{p.cfg.Action: p.token, p.cfg.SecondAction: p.target}
	}
	target := p.target
	if p.cfg.Family == "pareto" && !strings.HasPrefix(p.cfg.Mode, "queue-") {
		target = p.cdo
	}
	if p.cfg.Family == "noon" && p.cfg.Mode == "request" {
		target = p.token
	}
	return map[string]common.Address{p.cfg.Action: target}
}
