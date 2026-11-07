// Package liquidlanerfq authorizes and reconciles consumer RFQ redemptions.
package liquidlanerfq

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	reactor "github.com/symbioticfi/merkle-executor/api/bindings/liquidlanereactor"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type Config struct {
	HTTP               protocolio.HTTPConfig `yaml:"http"`
	Reactor            string                `yaml:"reactor"`
	Protocol           string                `yaml:"protocol"`
	TokenIn            string                `yaml:"tokenIn"`
	TokenOut           string                `yaml:"tokenOut"`
	AmountIn           string                `yaml:"amountIn"`
	MinAmountOut       string                `yaml:"minAmountOut"`
	MinValiditySeconds uint64                `yaml:"minValiditySeconds"`
	MaxValiditySeconds uint64                `yaml:"maxValiditySeconds"`
	Action             string                `yaml:"action"`
	AmountInInput      *uint                 `yaml:"amountInInput"`
	AmountOutInput     *uint                 `yaml:"amountOutInput"`
	DeadlineInput      *uint                 `yaml:"deadlineInput"`
	NonceInput         *uint                 `yaml:"nonceInput"`
	SolverIDs          []string              `yaml:"solverIds"`
	MaxLookupPages     int                   `yaml:"maxLookupPages"`
	EventRangeBlocks   uint64                `yaml:"eventRangeBlocks"`
}
type chainReader interface {
	provider.Reader
	provider.LogReader
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	TransactionReceipt(context.Context, common.Hash) (*types.Receipt, error)
}
type Provider struct {
	cfg                                  Config
	chain                                chainReader
	http                                 *protocolio.Client
	reactor, protocol, tokenIn, tokenOut common.Address
	amount, minimum                      *big.Int
}
type prepared struct {
	Version       int                     `json:"version"`
	ChainID       string                  `json:"chainId"`
	Account       common.Address          `json:"account"`
	Reactor       common.Address          `json:"reactor"`
	Protocol      common.Address          `json:"protocol"`
	PreparedBlock uint64                  `json:"preparedBlock"`
	Quote         json.RawMessage         `json:"quote"`
	Request       reactor.IReactorRequest `json:"request"`
	Digest        common.Hash             `json:"digest"`
	Signature     string                  `json:"signature"`
}
type progressState struct {
	Version   int          `json:"version"`
	Digest    common.Hash  `json:"digest"`
	OrderID   string       `json:"orderId,omitempty"`
	NextBlock uint64       `json:"nextBlock"`
	Anchor    common.Hash  `json:"anchor"`
	Lookup    *lookupState `json:"lookup,omitempty"`
}

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var cursorPattern = regexp.MustCompile(`^oc_[A-Za-z0-9_-]{1,512}$`)

const maxSafeInteger = uint64(1<<53 - 1)

// New accepts the immutable carrier derived from an authenticated recipe.
func New(c Config, deps registry.Deps) (*Provider, error) {
	if err := protocolio.Action(c.Action); err != nil {
		return nil, err
	}
	p := &Provider{cfg: c}
	var err error
	for _, v := range []struct {
		s string
		a *common.Address
	}{{c.Reactor, &p.reactor}, {c.Protocol, &p.protocol}, {c.TokenIn, &p.tokenIn}, {c.TokenOut, &p.tokenOut}} {
		if *v.a, err = parse.Address(v.s); err != nil {
			return nil, errors.New("RFQ requires nonzero reactor, protocol and token addresses")
		}
	}
	if p.tokenIn == p.tokenOut {
		return nil, errors.New("RFQ tokens must differ")
	}
	if p.amount, err = parse.Uint256(c.AmountIn); err != nil || p.amount.Sign() == 0 {
		return nil, errors.New("RFQ positive amountIn required")
	}
	if p.minimum, err = parse.Uint256(c.MinAmountOut); err != nil || p.minimum.Sign() == 0 {
		return nil, errors.New("RFQ positive minAmountOut required")
	}
	if c.MinValiditySeconds == 0 || c.MaxValiditySeconds < c.MinValiditySeconds || c.MaxValiditySeconds > maxSafeInteger {
		return nil, errors.New("RFQ invalid validity bounds")
	}
	if c.MaxLookupPages < 1 || c.MaxLookupPages > 100 || c.EventRangeBlocks < 1 || c.EventRangeBlocks > 100000 {
		return nil, errors.New("RFQ lookup pages must be 1..100 and event range 1..100000")
	}
	seenInputs := map[uint]bool{}
	for _, index := range []*uint{c.AmountInInput, c.AmountOutInput, c.DeadlineInput, c.NonceInput} {
		if index != nil {
			if err := parse.InputIndex(*index); err != nil {
				return nil, err
			}
			if seenInputs[*index] {
				return nil, errors.New("RFQ input mappings must differ")
			}
			seenInputs[*index] = true
		}
	}
	if c.AmountInInput != nil && c.AmountOutInput != nil && *c.AmountInInput == *c.AmountOutInput {
		return nil, errors.New("RFQ input mappings must differ")
	}
	seen := map[string]bool{}
	if len(c.SolverIDs) > 50 {
		return nil, errors.New("RFQ solver count exceeds 50")
	}
	for _, id := range c.SolverIDs {
		if strings.TrimSpace(id) != id || id == "" || seen[id] {
			return nil, errors.New("RFQ solver IDs must be unique nonempty values")
		}
		seen[id] = true
	}
	var ok bool
	if p.chain, ok = deps.Chain.(chainReader); !ok {
		return nil, errors.New("RFQ requires contract, log, receipt and canonical header reads")
	}
	p.http, err = protocolio.NewHTTP(c.HTTP, deps.HTTP)
	return p, err
}
func (p *Provider) PlanPreparation(pin protocolio.PreparationPin) (protocolio.PreparationPlan, error) {
	if err := pin.Validate(); err != nil {
		return protocolio.PreparationPlan{}, err
	}
	return protocolio.PreparationPlan{Requests: []protocolio.ReadRequest{protocolio.ContractRead(p.reactor, reactor.NewLiquidLaneReactor().PackEip712Domain(), new(big.Int).SetUint64(pin.BlockNumber), true)}, Resolve: func(ctx context.Context, s provider.Snapshot, at protocolio.ReadAccessor) (provider.Output, error) {
		if !pin.Matches(s) {
			return provider.Output{}, errors.New("RFQ staged identity mismatch")
		}
		return p.prepare(ctx, s, at(0))
	}}, nil
}
func (p *Provider) Prepare(ctx context.Context, s provider.Snapshot) (provider.Output, error) {
	if err := protocolio.Snapshot(s); err != nil {
		return provider.Output{}, err
	}
	reads, err := protocolio.ReadGroup(ctx, p.chain, protocolio.ContractRead(p.reactor, reactor.NewLiquidLaneReactor().PackEip712Domain(), s.BlockNumber, true))
	if err != nil {
		return provider.Output{}, err
	}
	return p.prepare(ctx, s, reads.At(0))
}
func (p *Provider) domain(s provider.Snapshot, result protocolio.ReadResult) (common.Hash, error) {
	raw, err := result.CallData()
	if err != nil {
		return common.Hash{}, err
	}
	v, err := reactor.NewLiquidLaneReactor().UnpackEip712Domain(raw)
	if err != nil || v.Fields != [1]byte{15} || v.Name != "Reactor" || v.Version != "1" || v.ChainId == nil || v.ChainId.Cmp(s.ChainID) != 0 || v.VerifyingContract != p.reactor || v.Salt != [32]byte{} || len(v.Extensions) != 0 {
		return common.Hash{}, errors.New("RFQ Reactor domain mismatch")
	}
	return protocolio.Domain("Reactor", "1", s.ChainID, p.reactor), nil
}
func (p *Provider) prepare(ctx context.Context, s provider.Snapshot, domainResult protocolio.ReadResult) (provider.Output, error) {
	if err := protocolio.Snapshot(s); err != nil {
		return provider.Output{}, err
	}
	if !s.BlockNumber.IsUint64() || !s.ChainID.IsUint64() || s.ChainID.Uint64() > maxSafeInteger || s.Timestamp > maxSafeInteger-p.cfg.MaxValiditySeconds {
		return provider.Output{}, errors.New("RFQ chain or expiry exceeds API integer bound")
	}
	domain, err := p.domain(s, domainResult)
	if err != nil {
		return provider.Output{}, err
	}
	var response wire.RfqQuoteResponse
	code, err := p.http.JSON(ctx, http.MethodPost, "/quote", wire.RfqQuoteRequest{TokenInChainID: s.ChainID.Int64(), TokenOutChainID: s.ChainID.Int64(), TokenIn: p.tokenIn.Hex(), TokenOut: p.tokenOut.Hex(), Type: "EXACT_INPUT", Amount: p.amount.String(), Swapper: s.Account.Hex(), RoutingPreference: "BEST_PRICE", SolverIDs: p.cfg.SolverIDs, Outputs: []wire.RfqQuoteOutput{{Token: p.tokenOut.Hex(), Recipient: s.Account.Hex()}}}, &response)
	if err != nil {
		if code == http.StatusNotFound {
			return provider.Output{}, provider.ErrNotReady
		}
		return provider.Output{}, err
	}
	if response.Quotes == nil {
		return provider.Output{}, errors.New("RFQ missing quotes array")
	}
	for _, option := range response.Quotes {
		if option.Solver.ID == "" {
			return provider.Output{}, errors.New("RFQ missing solver identity")
		}
		if _, err := parse.Address(option.Solver.Filler); err != nil {
			return provider.Output{}, errors.New("RFQ invalid solver filler")
		}
		allowed := len(p.cfg.SolverIDs) == 0
		for _, id := range p.cfg.SolverIDs {
			allowed = allowed || id == option.Solver.ID
		}
		if !allowed {
			continue
		}
		request, _, err := p.quote(option.Quote, s.Account)
		if err != nil {
			return provider.Output{}, err
		}
		if err = p.typedData(option.SignatureData, s, request); err != nil {
			return provider.Output{}, err
		}
		deadline := request.Deadline.Uint64()
		if deadline < s.Timestamp+p.cfg.MinValiditySeconds || deadline > s.Timestamp+p.cfg.MaxValiditySeconds || request.Outputs[0].Amount.Cmp(p.minimum) < 0 {
			continue
		}
		reads, err := protocolio.ReadGroup(ctx, p.chain, protocolio.ContractRead(p.reactor, reactor.NewLiquidLaneReactor().PackIsUsedNonce(s.Account, request.Nonce), s.BlockNumber, true))
		if err != nil {
			return provider.Output{}, err
		}
		raw, err := reads.At(0).CallData()
		if err != nil {
			return provider.Output{}, err
		}
		used, err := reactor.NewLiquidLaneReactor().UnpackIsUsedNonce(raw)
		if err != nil {
			return provider.Output{}, errors.New("RFQ invalid nonce response")
		}
		if used {
			return provider.Output{}, errors.New("RFQ quote nonce already consumed")
		}
		payload, digest, err := authorization(domain, request)
		if err != nil {
			return provider.Output{}, err
		}
		if err = protocolio.Canonical(ctx, p.chain, s); err != nil {
			return provider.Output{}, err
		}
		saved, err := json.Marshal(prepared{Version: 1, ChainID: s.ChainID.String(), Account: s.Account, Reactor: p.reactor, Protocol: p.protocol, PreparedBlock: s.BlockNumber.Uint64(), Quote: append(json.RawMessage(nil), option.Quote...), Request: request, Digest: digest, Signature: "0x00"})
		if err != nil {
			return provider.Output{}, err
		}
		out := provider.Output{Actions: map[string]hexutil.Bytes{p.cfg.Action: payload}, Inputs: map[uint]string{}, Data: saved}
		if p.cfg.AmountInInput != nil {
			out.Inputs[*p.cfg.AmountInInput] = request.AmountIn.String()
		}
		if p.cfg.AmountOutInput != nil {
			out.Inputs[*p.cfg.AmountOutInput] = request.Outputs[0].Amount.String()
		}
		if p.cfg.DeadlineInput != nil {
			out.Inputs[*p.cfg.DeadlineInput] = request.Deadline.String()
		}
		if p.cfg.NonceInput != nil {
			out.Inputs[*p.cfg.NonceInput] = request.Nonce.String()
		}
		return out, nil
	}
	return provider.Output{}, provider.ErrNotReady
}
func (p *Provider) persisted(s provider.Snapshot, out provider.Output) (prepared, common.Hash, error) {
	var d prepared
	if err := strictJSON(out.Data, &d); err != nil || d.Version != 1 || d.ChainID != s.ChainID.String() || d.Account != s.Account || d.Reactor != p.reactor || d.Protocol != p.protocol || d.Signature != "0x00" || !s.BlockNumber.IsUint64() || s.BlockNumber.Uint64() < d.PreparedBlock {
		return d, common.Hash{}, errors.New("RFQ persisted identity mismatch")
	}
	request, _, err := p.quote(d.Quote, s.Account)
	if err != nil {
		return d, common.Hash{}, err
	}
	if request.Outputs[0].Amount.Cmp(p.minimum) < 0 {
		return d, common.Hash{}, errors.New("RFQ persisted output below minimum")
	}
	if !equalRequest(request, d.Request) {
		return d, common.Hash{}, errors.New("RFQ persisted quote/request mismatch")
	}
	domain := protocolio.Domain("Reactor", "1", s.ChainID, p.reactor)
	payload, digest, err := authorization(domain, request)
	if err != nil || digest != d.Digest || len(out.Actions) != 1 || !bytes.Equal(out.Actions[p.cfg.Action], payload) || len(out.SkipSteps) != 0 {
		return d, domain, errors.New("RFQ persisted authorization mismatch")
	}
	expected := map[uint]string{}
	if p.cfg.AmountInInput != nil {
		expected[*p.cfg.AmountInInput] = request.AmountIn.String()
	}
	if p.cfg.AmountOutInput != nil {
		expected[*p.cfg.AmountOutInput] = request.Outputs[0].Amount.String()
	}
	if p.cfg.DeadlineInput != nil {
		expected[*p.cfg.DeadlineInput] = request.Deadline.String()
	}
	if p.cfg.NonceInput != nil {
		expected[*p.cfg.NonceInput] = request.Nonce.String()
	}
	if len(out.Inputs) != len(expected) {
		return d, domain, errors.New("RFQ persisted inputs mismatch")
	}
	for k, v := range expected {
		if out.Inputs[k] != v {
			return d, domain, errors.New("RFQ persisted inputs mismatch")
		}
	}
	return d, domain, nil
}
func (p *Provider) Advance(ctx context.Context, s provider.Snapshot, out provider.Output, state json.RawMessage) (provider.Progress, error) {
	if err := protocolio.Snapshot(s); err != nil {
		return provider.Progress{}, err
	}
	d, domain, err := p.persisted(s, out)
	if err != nil {
		return provider.Progress{}, err
	}
	if err = protocolio.Execution(ctx, p.chain, s); err != nil {
		return provider.Progress{}, err
	}
	if err = authorizingSign(s.Execution, d, domain); err != nil {
		return provider.Progress{}, err
	}
	st := progressState{Version: 1, Digest: d.Digest, NextBlock: d.PreparedBlock}
	if len(state) != 0 {
		if err = strictJSON(state, &st); err != nil || st.Version != 1 || st.Digest != d.Digest || st.NextBlock < d.PreparedBlock || st.NextBlock > s.BlockNumber.Uint64()+1 || (st.NextBlock == d.PreparedBlock && st.Anchor != (common.Hash{})) || (st.NextBlock > d.PreparedBlock && st.Anchor == (common.Hash{})) || (st.OrderID != "" && !uuid.MatchString(st.OrderID)) {
			return provider.Progress{}, errors.New("RFQ invalid persisted scan cursor")
		}
	}
	r := reactor.NewLiquidLaneReactor()
	a := account.NewMerkleAccount()
	token := erc20.NewERC20()
	reads, err := protocolio.ReadGroup(ctx, p.chain, protocolio.ContractRead(p.reactor, r.PackEip712Domain(), s.BlockNumber, true), protocolio.ContractRead(p.reactor, r.PackIsUsedNonce(s.Account, d.Request.Nonce), s.BlockNumber, true), protocolio.ContractRead(s.Account, a.PackSignatures(d.Digest), s.BlockNumber, true), protocolio.ContractRead(p.tokenIn, token.PackBalanceOf(s.Account), s.BlockNumber, false), protocolio.ContractRead(p.tokenIn, token.PackAllowance(s.Account, p.reactor), s.BlockNumber, false))
	if err != nil {
		return provider.Progress{}, err
	}
	if _, err = p.domain(s, reads.At(0)); err != nil {
		return provider.Progress{}, err
	}
	raw, err := reads.At(1).CallData()
	if err != nil {
		return provider.Progress{}, err
	}
	used, err := r.UnpackIsUsedNonce(raw)
	if err != nil {
		return provider.Progress{}, errors.New("RFQ invalid nonce response")
	}
	outcome, complete, err := p.scan(ctx, s, d, &st, used)
	if err != nil {
		return provider.Progress{}, err
	}
	finish := func(name string, done bool) (provider.Progress, error) {
		if err := protocolio.Canonical(ctx, p.chain, s); err != nil {
			return provider.Progress{}, err
		}
		if done {
			st.Lookup = nil
		}
		raw, err := json.Marshal(st)
		return provider.Progress{Done: done, Outcome: name, Reference: st.OrderID, State: raw}, err
	}
	if outcome != "" {
		return finish(outcome, true)
	}
	if !complete {
		return finish("scanning", false)
	}
	if used {
		return provider.Progress{}, errors.New("RFQ nonce consumed without matching canonical evidence")
	}
	if new(big.Int).SetUint64(s.Timestamp).Cmp(d.Request.Deadline) > 0 {
		return finish("expired", true)
	}
	raw, err = reads.At(2).CallData()
	if err != nil {
		return provider.Progress{}, err
	}
	approved, err := a.UnpackSignatures(raw)
	if err != nil || !approved {
		return provider.Progress{}, errors.New("RFQ account authorization unavailable")
	}
	raw, err = reads.At(3).CallData()
	if err != nil {
		return provider.Progress{}, err
	}
	balance, err := token.UnpackBalanceOf(raw)
	if err != nil || balance.Cmp(d.Request.AmountIn) < 0 {
		return provider.Progress{}, errors.New("RFQ input balance unavailable")
	}
	raw, err = reads.At(4).CallData()
	if err != nil {
		return provider.Progress{}, err
	}
	allowance, err := token.UnpackAllowance(raw)
	if err != nil || allowance.Cmp(d.Request.AmountIn) < 0 {
		return provider.Progress{}, errors.New("RFQ Reactor allowance unavailable")
	}
	found, complete, err := p.lookup(ctx, s, d, &st)
	if err != nil {
		return provider.Progress{}, err
	}
	if err = protocolio.Canonical(ctx, p.chain, s); err != nil {
		return provider.Progress{}, err
	}
	if !complete {
		return finish("lookup-scanning", false)
	}
	if found {
		return finish("published", false)
	}
	var response wire.RfqOrderResponse
	code, postErr := p.http.JSON(ctx, http.MethodPost, "/order", wire.RfqOrderRequest{Quote: d.Quote, Signature: d.Signature}, &response)
	if postErr != nil {
		if code == 0 || code == http.StatusConflict || code >= 500 || code >= 200 && code < 300 {
			return finish("publication-uncertain", false)
		}
		return provider.Progress{}, postErr
	}
	if !uuid.MatchString(response.OrderID) || !knownStatus(response.OrderStatus) {
		return finish("publication-uncertain", false)
	}
	st.OrderID = response.OrderID
	return finish("published", false)
}

var _ provider.Provider = (*Provider)(nil)
