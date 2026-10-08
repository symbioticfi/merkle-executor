// Package securitize prepares issuer-pinned wallet and atomic redemption calls.
package securitize

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	legacy "github.com/symbioticfi/merkle-executor/api/bindings/securitizelegacyofframp"
	modern "github.com/symbioticfi/merkle-executor/api/bindings/securitizeofframp"
	"github.com/symbioticfi/merkle-executor/api/bindings/securitizetoken"
	"github.com/symbioticfi/merkle-executor/api/bindings/securitizetopology"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type Config struct {
	Catalog        string `yaml:"catalog"`
	Symbol         string `yaml:"symbol"`
	ChainID        uint64 `yaml:"chainId"`
	Mode           string `yaml:"mode"`
	Token          string `yaml:"token"`
	Target         string `yaml:"target"`
	PaymentToken   string `yaml:"paymentToken"`
	Amount         string `yaml:"amount"`
	MinOutput      string `yaml:"minOutput"`
	Action         string `yaml:"action"`
	AmountInput    *uint  `yaml:"amountInput"`
	MinOutputInput *uint  `yaml:"minOutputInput"`
}

type Provider struct {
	cfg                    Config
	deps                   providerRegistry.Deps
	token, target, payment common.Address
	amount, minimum        *big.Int
	decimals               *uint8
}

type prepared struct {
	ChainID  uint64         `json:"chainId"`
	Account  common.Address `json:"account"`
	Token    common.Address `json:"token"`
	Target   common.Address `json:"target"`
	Payment  common.Address `json:"payment"`
	Mode     string         `json:"mode"`
	Amount   string         `json:"amount"`
	Minimum  string         `json:"minimum"`
	Expected string         `json:"expected"`
}

func Register(r *providerRegistry.Registry) error { return r.Register("securitize", Factory) }

func Factory(raw yaml.Node, deps providerRegistry.Deps) (provider.Provider, error) {
	var c Config
	if err := parse.DecodeStrict(raw, &c); err != nil {
		return nil, err
	}
	if c.ChainID == 0 || deps.Chain == nil {
		return nil, errors.New("Securitize chainId and chain reader are required")
	}
	if c.Mode != "wallet" && c.Mode != "modern" && c.Mode != "legacy" {
		return nil, errors.New("Securitize mode must be wallet, modern or legacy")
	}
	if err := protocolio.Action(c.Action); err != nil {
		return nil, err
	}
	p := &Provider{cfg: c, deps: deps, minimum: new(big.Int)}
	var err error
	p.token, err = parse.Address(c.Token)
	if err != nil {
		return nil, err
	}
	p.target, err = parse.Address(c.Target)
	if err != nil || p.token == p.target {
		return nil, errors.New("invalid Securitize target")
	}
	p.amount, err = parse.Uint256(c.Amount)
	if err != nil || p.amount.Sign() == 0 {
		return nil, errors.New("Securitize amount must be positive uint256")
	}
	if c.Mode == "wallet" {
		if c.PaymentToken != "" || c.MinOutput != "" || c.MinOutputInput != nil {
			return nil, errors.New("Securitize wallet request cannot promise a payment token or minimum output")
		}
	} else {
		p.payment, err = parse.Address(c.PaymentToken)
		if err != nil || p.payment == p.token {
			return nil, errors.New("invalid Securitize payment token")
		}
		p.minimum, err = parse.Uint256(c.MinOutput)
		if err != nil || p.minimum.Sign() == 0 {
			return nil, errors.New("Securitize minOutput must be positive uint256")
		}
	}
	if c.AmountInput != nil && c.MinOutputInput != nil && *c.AmountInput == *c.MinOutputInput {
		return nil, errors.New("duplicate Securitize input assignment")
	}
	if err = p.validateCatalog(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Provider) Prepare(ctx context.Context, s provider.Snapshot) (provider.Output, error) {
	if err := p.snapshot(s); err != nil {
		return provider.Output{}, err
	}
	decimals, err := p.tokenDecimals(ctx, s, p.token)
	if err != nil {
		return provider.Output{}, err
	}
	if p.decimals != nil && decimals != *p.decimals {
		return provider.Output{}, errors.New("Securitize token decimals changed from pinned catalog")
	}
	expected := new(big.Int)
	if p.cfg.Mode != "wallet" {
		expected, err = p.quote(ctx, s, decimals)
		if err != nil {
			return provider.Output{}, err
		}
		if expected.Cmp(p.minimum) < 0 {
			return provider.Output{}, errors.Errorf("%w: Securitize quote below minOutput", provider.ErrNotReady)
		}
	}
	payload, err := p.payload()
	if err != nil {
		return provider.Output{}, err
	}
	inputs := make(map[uint]string)
	if p.cfg.AmountInput != nil {
		inputs[*p.cfg.AmountInput] = p.amount.String()
	}
	if p.cfg.MinOutputInput != nil {
		inputs[*p.cfg.MinOutputInput] = p.minimum.String()
	}
	data, err := json.Marshal(prepared{ChainID: p.cfg.ChainID, Account: s.Account, Token: p.token, Target: p.target, Payment: p.payment, Mode: p.cfg.Mode, Amount: p.amount.String(), Minimum: p.minimum.String(), Expected: expected.String()})
	if err != nil {
		return provider.Output{}, errors.Errorf("encode Securitize request identity: %w", err)
	}
	return provider.Output{Inputs: inputs, Actions: map[string]hexutil.Bytes{p.cfg.Action: payload}, Data: data}, nil
}

func (p *Provider) snapshot(s provider.Snapshot) error {
	if err := protocolio.Snapshot(s); err != nil {
		return err
	}
	if !s.ChainID.IsUint64() || s.ChainID.Uint64() != p.cfg.ChainID || s.Account == p.target {
		return errors.New("Securitize chain or account mismatch")
	}
	return nil
}

func (p *Provider) payload() ([]byte, error) {
	switch p.cfg.Mode {
	case "wallet":
		return erc20.NewERC20().TryPackTransfer(p.target, p.amount)
	case "modern":
		return modern.NewSecuritizeOffRamp().TryPackRedeem(p.amount, p.minimum)
	default:
		return legacy.NewSecuritizeLegacyOffRamp().TryPackRedeem(p.amount)
	}
}

func (p *Provider) call(ctx context.Context, s provider.Snapshot, target common.Address, a abi.ABI, method string, args ...any) ([]any, error) {
	data, err := a.Pack(method, args...)
	if err != nil {
		return nil, errors.Errorf("encode Securitize %s: %w", method, err)
	}
	raw, err := protocolio.CallData(ctx, p.deps.Chain, s, target, data)
	if err != nil {
		return nil, err
	}
	v, err := a.Unpack(method, raw)
	if err != nil || len(v) != 1 {
		return nil, errors.Errorf("invalid Securitize %s response", method)
	}
	return v, nil
}

func (p *Provider) tokenDecimals(ctx context.Context, s provider.Snapshot, token common.Address) (uint8, error) {
	v, err := p.call(ctx, s, token, securitizetoken.NewSecuritizeToken().GetABI(), "decimals")
	if err != nil {
		return 0, err
	}
	return v[0].(uint8), nil
}

func (p *Provider) quote(ctx context.Context, s provider.Snapshot, decimals uint8) (*big.Int, error) {
	a := modern.NewSecuritizeOffRamp().GetABI()
	assetMethod, liquidityMethod := "assetAddress", "liquidityProvider"
	if p.cfg.Mode == "legacy" {
		a = legacy.NewSecuritizeLegacyOffRamp().GetABI()
		assetMethod, liquidityMethod = "asset", "liquidity"
	}
	v, err := p.call(ctx, s, p.target, a, "paused")
	if err != nil {
		return nil, err
	}
	if v[0].(bool) {
		return nil, errors.Errorf("%w: Securitize offramp is paused", provider.ErrNotReady)
	}
	v, err = p.call(ctx, s, p.target, a, assetMethod)
	if err != nil {
		return nil, err
	}
	if v[0].(common.Address) != p.token {
		return nil, errors.New("Securitize offramp asset mismatch")
	}
	v, err = p.call(ctx, s, p.target, a, liquidityMethod)
	if err != nil {
		return nil, err
	}
	payment := v[0].(common.Address)
	liquidityTarget := p.target
	liquidityABI := a
	expected := new(big.Int)
	if p.cfg.Mode == "modern" {
		if payment == (common.Address{}) {
			return nil, errors.New("Securitize liquidity provider is zero")
		}
		v, err = p.call(ctx, s, payment, securitizetopology.NewSecuritizeTopology().GetABI(), "liquidityToken")
		if err != nil {
			return nil, err
		}
		payment = v[0].(common.Address)
		v, err = p.call(ctx, s, p.target, a, "calculateLiquidityTokenAmount", p.amount)
		if err != nil {
			return nil, err
		}
		expected = v[0].(*big.Int)
	} else {
		v, err = p.call(ctx, s, p.target, a, "settlement")
		if err != nil {
			return nil, err
		}
		liquidityTarget = v[0].(common.Address)
		liquidityABI = securitizetopology.NewSecuritizeTopology().GetABI()
		if liquidityTarget == (common.Address{}) {
			return nil, errors.New("Securitize settlement is zero")
		}
		outDecimals, err := p.tokenDecimals(ctx, s, p.payment)
		if err != nil {
			return nil, err
		}
		if outDecimals < decimals || outDecimals-decimals > 77 {
			return nil, errors.New("invalid Securitize legacy output normalization")
		}
		expected.Mul(p.amount, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(outDecimals-decimals)), nil))
		if expected.BitLen() > 256 {
			return nil, errors.New("Securitize normalized output overflows uint256")
		}
	}
	if payment != p.payment {
		return nil, errors.New("Securitize offramp payment token mismatch")
	}
	v, err = p.call(ctx, s, liquidityTarget, liquidityABI, "availableLiquidity")
	if err != nil {
		return nil, err
	}
	if v[0].(*big.Int).Cmp(expected) < 0 {
		return nil, errors.Errorf("%w: Securitize offramp liquidity is insufficient", provider.ErrNotReady)
	}
	return expected, nil
}

func (p *Provider) Advance(ctx context.Context, s provider.Snapshot, output provider.Output, _ json.RawMessage) (provider.Progress, error) {
	if err := p.snapshot(s); err != nil {
		return provider.Progress{}, err
	}
	var data prepared
	if err := json.Unmarshal(output.Data, &data); err != nil {
		return provider.Progress{}, errors.New("invalid persisted Securitize identity")
	}
	if data.ChainID != p.cfg.ChainID || data.Account != s.Account || data.Token != p.token || data.Target != p.target || data.Payment != p.payment || data.Mode != p.cfg.Mode || data.Amount != p.amount.String() || data.Minimum != p.minimum.String() {
		return provider.Progress{}, errors.New("persisted Securitize identity mismatch")
	}
	expected, err := parse.Uint256(data.Expected)
	if err != nil || (p.cfg.Mode == "wallet" && expected.Sign() != 0) || (p.cfg.Mode != "wallet" && expected.Cmp(p.minimum) < 0) {
		return provider.Progress{}, errors.New("invalid persisted Securitize quote")
	}
	payload, err := p.payload()
	if err != nil || !bytes.Equal(output.Actions[p.cfg.Action], payload) {
		return provider.Progress{}, errors.New("persisted Securitize action mismatch")
	}
	if err = protocolio.Execution(ctx, p.deps.Chain, s); err != nil {
		return provider.Progress{}, err
	}
	target := p.target
	if p.cfg.Mode == "wallet" {
		target = p.token
	}
	if err = protocolio.ExecutedCalls(s, []protocolio.ManagedCall{{Target: target, Data: payload}}); err != nil {
		return provider.Progress{}, err
	}
	outcome := "redeemed"
	if p.cfg.Mode == "wallet" {
		if p.transferred(s, p.token, s.Account, p.target).Cmp(p.amount) != 0 {
			return provider.Progress{}, errors.New("Securitize request receipt does not prove exact wallet delivery")
		}
		outcome = "requested"
	} else {
		if p.cfg.Mode == "modern" {
			expected, err = p.redemptionEvent(s)
			if err != nil {
				return provider.Progress{}, err
			}
		}
		if p.transferred(s, p.payment, common.Address{}, s.Account).Cmp(expected) < 0 {
			return provider.Progress{}, errors.New("Securitize redemption receipt does not prove payment delivery")
		}
	}
	state, err := json.Marshal(map[string]string{"transactionHash": s.Execution.TxHash.Hex(), "amount": p.amount.String(), "output": expected.String()})
	if err != nil {
		return provider.Progress{}, errors.Errorf("encode Securitize progress: %w", err)
	}
	return provider.Progress{Done: true, Outcome: outcome, State: state, Reference: s.Execution.TxHash.Hex()}, nil
}

// transferred nets wallet returns, so a round trip cannot prove a pending request.
// A zero from counts atomic payments; the recipe may subsequently move those funds.
func (p *Provider) transferred(s provider.Snapshot, token, from, to common.Address) *big.Int {
	amount := new(big.Int)
	contract := erc20.NewERC20()
	event := contract.GetABI().Events["Transfer"].ID
	for _, log := range s.Execution.Logs {
		if log.Address != token || len(log.Topics) != 3 || log.Topics[0] != event || len(log.Data) != 32 {
			continue
		}
		v, err := contract.UnpackTransferEvent(&log)
		if err != nil {
			continue
		}
		if v.To == to && (from == (common.Address{}) || v.From == from) {
			amount.Add(amount, v.Value)
		}
		if from != (common.Address{}) && v.From == to && v.To == from {
			amount.Sub(amount, v.Value)
		}
	}
	return amount
}

func (p *Provider) redemptionEvent(s provider.Snapshot) (*big.Int, error) {
	contract := modern.NewSecuritizeOffRamp()
	event := contract.GetABI().Events["RedemptionCompleted"].ID
	var received *big.Int
	for _, log := range s.Execution.Logs {
		if log.Address != p.target || len(log.Topics) != 3 || log.Topics[0] != event {
			continue
		}
		if len(log.Data) != 128 {
			return nil, errors.New("malformed Securitize redemption event")
		}
		v, err := contract.UnpackRedemptionCompletedEvent(&log)
		if err != nil || v.Redeemer != s.Account || v.LiquidityToken != p.payment || v.DsTokenValue.Cmp(p.amount) != 0 || v.LiquidityValue.Cmp(p.minimum) < 0 || received != nil {
			return nil, errors.New("Securitize redemption event mismatch or ambiguous multiple redemptions")
		}
		received = v.LiquidityValue
	}
	if received == nil {
		return nil, errors.New("Securitize redemption completion event missing")
	}
	return received, nil
}
