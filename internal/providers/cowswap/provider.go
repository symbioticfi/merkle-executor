// Package cowswap prepares MerkleAccount ERC-1271 authorization and reconciles CoW orders.
package cowswap

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	settlementbinding "github.com/symbioticfi/merkle-executor/api/bindings/cowsettlement"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	accountbinding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type Config struct {
	HTTP            protocolio.HTTPConfig `yaml:"http"`
	Settlement      string                `yaml:"settlement"`
	SellToken       string                `yaml:"sellToken"`
	BuyToken        string                `yaml:"buyToken"`
	SellAmount      string                `yaml:"sellAmount"`
	MinBuyAmount    string                `yaml:"minBuyAmount"`
	SlippageBPS     uint64                `yaml:"slippageBps"`
	ValiditySeconds uint64                `yaml:"validitySeconds"`
	AppData         string                `yaml:"appData"`
	Action          string                `yaml:"action"`
	SellAmountInput *uint                 `yaml:"sellAmountInput"`
	BuyAmountInput  *uint                 `yaml:"buyAmountInput"`
}
type Client struct{ http *protocolio.Client }

func NewClient(c protocolio.HTTPConfig, h *http.Client) (*Client, error) {
	v, e := protocolio.NewHTTP(c, h)
	if e != nil {
		return nil, e
	}
	return &Client{http: v}, nil
}
func (c *Client) Quote(ctx context.Context, request wire.CowQuoteRequest) (wire.CowQuoteResponse, error) {
	var r wire.CowQuoteResponse
	_, e := c.http.JSON(ctx, http.MethodPost, "/api/v1/quote", request, &r)
	return r, e
}
func (c *Client) Get(ctx context.Context, uid string) (wire.CowOrderStatus, int, error) {
	var r wire.CowOrderStatus
	code, e := c.http.JSON(ctx, http.MethodGet, "/api/v1/orders/"+uid, nil, &r)
	return r, code, e
}
func (c *Client) Post(ctx context.Context, order wire.CowOrder) (string, int, error) {
	var uid string
	code, e := c.http.JSON(ctx, http.MethodPost, "/api/v1/orders", order, &uid)
	return uid, code, e
}

type Provider struct {
	cfg                             Config
	deps                            providerRegistry.Deps
	client                          *Client
	settlement, sellToken, buyToken common.Address
	sellAmount, minBuy              *big.Int
	appData                         common.Hash
}
type prepared struct {
	Version       int            `json:"version"`
	ChainID       string         `json:"chainId"`
	Account       common.Address `json:"account"`
	PreparedBlock string         `json:"preparedBlock"`
	Order         wire.CowOrder  `json:"order"`
	Digest        common.Hash    `json:"digest"`
	UID           string         `json:"uid"`
}
type phase struct {
	Phase string `json:"phase"`
}

func progress(d prepared, name string, done bool) provider.Progress {
	state, _ := json.Marshal(phase{Phase: name})
	outcome := ""
	if done {
		outcome = name
	}
	return provider.Progress{Done: done, Outcome: outcome, State: state, Reference: d.UID}
}
func Register(r *providerRegistry.Registry) error { return r.Register("cowswap", Factory) }
func Factory(raw yaml.Node, deps providerRegistry.Deps) (provider.Provider, error) {
	var c Config
	if e := parse.DecodeStrict(raw, &c); e != nil {
		return nil, e
	}
	if e := protocolio.Action(c.Action); e != nil {
		return nil, e
	}
	if c.SlippageBPS > 1000 {
		return nil, errors.New("CoW slippageBps outside 0..1000")
	}
	if c.ValiditySeconds == 0 || c.ValiditySeconds > 86400 {
		return nil, errors.New("CoW validitySeconds outside 1..86400")
	}
	p := &Provider{cfg: c, deps: deps}
	var e error
	for _, x := range []struct {
		raw string
		dst *common.Address
	}{{c.Settlement, &p.settlement}, {c.SellToken, &p.sellToken}, {c.BuyToken, &p.buyToken}} {
		*x.dst, e = parse.Address(x.raw)
		if e != nil {
			return nil, e
		}
	}
	if p.sellToken == p.buyToken {
		return nil, errors.New("CoW tokens must differ")
	}
	p.sellAmount, e = parse.Uint256(c.SellAmount)
	if e != nil || p.sellAmount.Sign() == 0 {
		return nil, errors.New("CoW sellAmount must be positive uint256")
	}
	p.minBuy, e = parse.Uint256(c.MinBuyAmount)
	if e != nil || p.minBuy.Sign() == 0 {
		return nil, errors.New("CoW minBuyAmount must be positive uint256")
	}
	p.appData, e = protocolio.Hash(c.AppData)
	if e != nil {
		return nil, e
	}
	for _, index := range []*uint{c.SellAmountInput, c.BuyAmountInput} {
		if index != nil {
			if err := parse.InputIndex(*index); err != nil {
				return nil, err
			}
		}
	}
	if c.SellAmountInput != nil && c.BuyAmountInput != nil && *c.SellAmountInput == *c.BuyAmountInput {
		return nil, errors.New("CoW duplicate input index")
	}
	p.client, e = NewClient(c.HTTP, deps.HTTP)
	if e != nil {
		return nil, e
	}
	if deps.Chain == nil {
		return nil, errors.New("CoW chain reader required")
	}
	return p, nil
}
func (p *Provider) domain(ctx context.Context, s provider.Snapshot) (common.Hash, error) {
	d := protocolio.Domain("Gnosis Protocol", "v2", s.ChainID, p.settlement)
	a := settlementbinding.NewCowSettlement()
	raw, e := protocolio.CallData(ctx, p.deps.Chain, s, p.settlement, a.PackDomainSeparator())
	if e != nil {
		return common.Hash{}, e
	}
	domain, e := a.UnpackDomainSeparator(raw)
	if e != nil {
		return common.Hash{}, errors.New("invalid CoW domain separator response")
	}
	if common.Hash(domain) != d {
		return common.Hash{}, errors.New("CoW settlement domain differs from configured chain/address")
	}
	return d, nil
}
func (p *Provider) Prepare(ctx context.Context, s provider.Snapshot) (provider.Output, error) {
	if e := protocolio.Snapshot(s); e != nil {
		return provider.Output{}, e
	}
	if s.Timestamp > uint64(^uint32(0))-p.cfg.ValiditySeconds {
		return provider.Output{}, errors.New("CoW expiry outside uint32")
	}
	domain, e := p.domain(ctx, s)
	if e != nil {
		return provider.Output{}, e
	}
	validTo := int64(s.Timestamp + p.cfg.ValiditySeconds)
	q, e := p.client.Quote(ctx, wire.CowQuoteRequest{SellToken: p.sellToken.Hex(), BuyToken: p.buyToken.Hex(), From: s.Account.Hex(), Receiver: s.Account.Hex(), SellAmountBeforeFee: p.sellAmount.String(), Kind: "sell", SigningScheme: "eip1271", ValidTo: validTo, AppData: p.appData.Hex(), PartiallyFillable: false})
	if e != nil {
		return provider.Output{}, e
	}
	if e := protocolio.Canonical(ctx, p.deps.Chain, s); e != nil {
		return provider.Output{}, e
	}
	o := q.Quote
	if o.ValidTo != validTo || o.Kind != "sell" || o.PartiallyFillable {
		return provider.Output{}, errors.New("CoW quote not compatible with full-fill sell authorization")
	}
	quotedSell, e := parse.Uint256(o.SellAmount)
	if e != nil || quotedSell.Sign() == 0 || quotedSell.Cmp(p.sellAmount) > 0 {
		return provider.Output{}, errors.New("CoW quoted sell amount exceeds configured gross budget")
	}
	quotedFee, e := parse.Uint256(o.FeeAmount)
	if e != nil || new(big.Int).Add(quotedSell, quotedFee).Cmp(p.sellAmount) > 0 {
		return provider.Output{}, errors.New("CoW quote fee and sell amount exceed configured gross budget")
	}
	if !sameAddress(o.SellToken, p.sellToken) || !sameAddress(o.BuyToken, p.buyToken) || !sameAddress(o.Receiver, s.Account) {
		return provider.Output{}, errors.New("CoW quote token or receiver mismatch")
	}
	app, e := protocolio.Hash(o.AppData)
	if e != nil || app != p.appData {
		return provider.Output{}, errors.New("CoW quote appData mismatch")
	}
	if o.SellTokenBalance != "" && o.SellTokenBalance != "erc20" || o.BuyTokenBalance != "" && o.BuyTokenBalance != "erc20" {
		return provider.Output{}, errors.New("CoW quote balance mode mismatch")
	}
	buy, e := parse.Uint256(o.BuyAmount)
	if e != nil || buy.Sign() == 0 {
		return provider.Output{}, errors.New("CoW invalid quote amount")
	}
	buy.Mul(buy, new(big.Int).SetUint64(10000-p.cfg.SlippageBPS))
	buy.Div(buy, big.NewInt(10000))
	if buy.Cmp(p.minBuy) < 0 {
		return provider.Output{}, errors.New("CoW quote below minBuyAmount after slippage")
	}
	o.BuyAmount = buy.String()
	o.SellToken = p.sellToken.Hex()
	o.BuyToken = p.buyToken.Hex()
	o.Receiver = s.Account.Hex()
	o.From = s.Account.Hex()
	o.SigningScheme = "eip1271"
	o.Signature = "0x"
	o.FeeAmount = "0"
	o.SellTokenBalance = "erc20"
	o.BuyTokenBalance = "erc20"
	buffer, e := EncodeOrder(o)
	if e != nil {
		return provider.Output{}, e
	}
	digest := protocolio.Digest(domain, crypto.Keccak256Hash(buffer))
	uid := UID(digest, s.Account, uint32(o.ValidTo))
	payload, e := accountbinding.NewMerkleAccount().TryPackSign(domain, buffer, []accountbinding.IMerkleAccountHash{{Offset: big.NewInt(0), Length: big.NewInt(int64(len(buffer))), Destination: big.NewInt(0)}})
	if e != nil {
		return provider.Output{}, errors.New("encode CoW account authorization")
	}
	data, e := json.Marshal(prepared{Version: 1, ChainID: s.ChainID.String(), Account: s.Account, PreparedBlock: s.BlockNumber.String(), Order: o, Digest: digest, UID: uid})
	if e != nil {
		return provider.Output{}, errors.New("encode CoW persisted order")
	}
	inputs := map[uint]string{}
	if p.cfg.SellAmountInput != nil {
		inputs[*p.cfg.SellAmountInput] = quotedSell.String()
	}
	if p.cfg.BuyAmountInput != nil {
		inputs[*p.cfg.BuyAmountInput] = buy.String()
	}
	return provider.Output{Inputs: inputs, Actions: map[string]hexutil.Bytes{p.cfg.Action: payload}, Data: data}, nil
}

// Advance is called only after the keeper confirms the authorization recipe.
// A current confirmed Snapshot is required; API state alone can never mark a fill done.
func (p *Provider) Advance(ctx context.Context, s provider.Snapshot, out provider.Output, _ json.RawMessage) (provider.Progress, error) {
	if e := protocolio.Snapshot(s); e != nil {
		return provider.Progress{}, e
	}
	var d prepared
	if json.Unmarshal(out.Data, &d) != nil {
		return provider.Progress{}, errors.New("invalid persisted CoW order")
	}
	if d.Version != 1 || d.Account != s.Account || d.ChainID != s.ChainID.String() {
		return provider.Progress{}, errors.New("persisted CoW chain/account mismatch")
	}
	block, e := parse.Uint256(d.PreparedBlock)
	if e != nil || s.BlockNumber.Cmp(block) <= 0 {
		return provider.Progress{}, errors.New("CoW Advance requires a confirmed snapshot after preparation")
	}
	if e = p.validateOrder(d.Order, s.Account); e != nil {
		return provider.Progress{}, e
	}
	domain := protocolio.Domain("Gnosis Protocol", "v2", s.ChainID, p.settlement)
	buffer, e := EncodeOrder(d.Order)
	if e != nil {
		return provider.Progress{}, e
	}
	digest := protocolio.Digest(domain, crypto.Keccak256Hash(buffer))
	if digest != d.Digest || UID(digest, s.Account, uint32(d.Order.ValidTo)) != d.UID {
		return provider.Progress{}, errors.New("persisted CoW order digest/UID mismatch")
	}
	expected, e := accountbinding.NewMerkleAccount().TryPackSign(domain, buffer, []accountbinding.IMerkleAccountHash{{Offset: big.NewInt(0), Length: big.NewInt(int64(len(buffer))), Destination: big.NewInt(0)}})
	if e != nil || !bytes.Equal(out.Actions[p.cfg.Action], expected) {
		return provider.Progress{}, errors.New("persisted CoW authorization calldata differs from persisted order")
	}
	if p.cfg.SellAmountInput != nil && out.Inputs[*p.cfg.SellAmountInput] != d.Order.SellAmount || p.cfg.BuyAmountInput != nil && out.Inputs[*p.cfg.BuyAmountInput] != d.Order.BuyAmount {
		return provider.Progress{}, errors.New("persisted CoW numerical inputs differ from authorized order")
	}
	uidBytes, e := hexutil.Decode(d.UID)
	if e != nil {
		return provider.Progress{}, errors.New("invalid CoW UID")
	}
	settlement := settlementbinding.NewCowSettlement()
	account := accountbinding.NewMerkleAccount()
	reads, e := protocolio.ReadGroup(ctx, p.deps.Chain,
		protocolio.ContractRead(p.settlement, settlement.PackDomainSeparator(), s.BlockNumber, true),
		protocolio.ContractRead(p.settlement, settlement.PackFilledAmount(uidBytes), s.BlockNumber, true),
		protocolio.ContractRead(s.Account, account.PackSignatures(d.Digest), s.BlockNumber, true))
	if e != nil {
		return provider.Progress{}, e
	}
	raw, e := reads.At(0).CallData()
	if e != nil {
		return provider.Progress{}, e
	}
	declared, e := settlement.UnpackDomainSeparator(raw)
	if e != nil || common.Hash(declared) != domain {
		return provider.Progress{}, errors.New("CoW settlement domain mismatch")
	}
	raw, e = reads.At(1).CallData()
	if e != nil {
		return provider.Progress{}, e
	}
	filled, e := settlement.UnpackFilledAmount(raw)
	if e != nil {
		return provider.Progress{}, errors.New("invalid CoW filled amount response")
	}
	invalidated := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	if filled.Cmp(invalidated) == 0 {
		return progress(d, "invalidated", false), errors.New("CoW order invalidated on-chain")
	}
	orderSell, _ := parse.Uint256(d.Order.SellAmount)
	if filled.Cmp(orderSell) == 0 {
		if e := protocolio.Canonical(ctx, p.deps.Chain, s); e != nil {
			return provider.Progress{}, e
		}
		return progress(d, "settled", true), nil
	}
	if filled.Sign() != 0 {
		return progress(d, "invalidated-or-unexpected-fill", false), errors.New("CoW order invalidated or has unexpected partial fill")
	}
	if uint64(d.Order.ValidTo) < s.Timestamp {
		if e := protocolio.Canonical(ctx, p.deps.Chain, s); e != nil {
			return provider.Progress{}, e
		}
		return progress(d, "expired", true), nil
	}
	raw, e = reads.At(2).CallData()
	if e != nil {
		return provider.Progress{}, e
	}
	approved, e := account.UnpackSignatures(raw)
	if e != nil {
		return provider.Progress{}, errors.New("invalid CoW account approval response")
	}
	if !approved {
		return progress(d, "authorization-missing", false), errors.New("CoW account digest is not approved at confirmed snapshot")
	}
	remote, status, e := p.client.Get(ctx, d.UID)
	if canonicalErr := protocolio.Canonical(ctx, p.deps.Chain, s); canonicalErr != nil {
		return provider.Progress{}, canonicalErr
	}
	if e == nil {
		if remote.UID != d.UID || !sameAddress(remote.Owner, s.Account) || remote.SellAmount != d.Order.SellAmount || remote.BuyAmount != d.Order.BuyAmount || remote.ValidTo != d.Order.ValidTo {
			return progress(d, "response-mismatch", false), errors.New("CoW orderbook identity or order fields mismatch")
		}
		switch remote.Status {
		case "open", "presignaturePending":
			return progress(d, "open", false), nil
		case "fulfilled":
			return progress(d, "awaiting-confirmed-fill", false), nil
		case "expired", "cancelled":
			return progress(d, remote.Status, false), errors.New("CoW orderbook reports terminal unfilled order")
		default:
			return progress(d, "unknown", false), errors.New("unknown CoW order status")
		}
	}
	if status != http.StatusNotFound {
		return progress(d, "lookup-uncertain", false), e
	}
	if e := protocolio.Canonical(ctx, p.deps.Chain, s); e != nil {
		return provider.Progress{}, e
	}
	returned, code, postErr := p.client.Post(ctx, d.Order)
	if postErr != nil {
		// A timeout, duplicate response or gateway error can hide acceptance. Keep the exact UID
		// and reconcile GET before any later repost; the persisted Output survives this call.
		if code == 0 || code == http.StatusConflict || code >= 500 {
			return progress(d, "post-uncertain", false), nil
		}
		return progress(d, "post-rejected", false), postErr
	}
	if !strings.EqualFold(returned, d.UID) {
		return progress(d, "post-identity-mismatch", false), errors.New("CoW returned a different order UID")
	}
	return progress(d, "posted", false), nil
}
func (p *Provider) validateOrder(o wire.CowOrder, account common.Address) error {
	sell, e := parse.Uint256(o.SellAmount)
	if e != nil || sell.Sign() == 0 || sell.Cmp(p.sellAmount) > 0 {
		return errors.New("persisted CoW sell amount exceeds budget")
	}
	buy, e := parse.Uint256(o.BuyAmount)
	if e != nil || buy.Cmp(p.minBuy) < 0 {
		return errors.New("persisted CoW buy amount below bound")
	}
	app, e := protocolio.Hash(o.AppData)
	if e != nil || app != p.appData {
		return errors.New("persisted CoW appData mismatch")
	}
	if !sameAddress(o.SellToken, p.sellToken) || !sameAddress(o.BuyToken, p.buyToken) || !sameAddress(o.Receiver, account) || !sameAddress(o.From, account) || o.Kind != "sell" || o.PartiallyFillable || o.FeeAmount != "0" || o.SigningScheme != "eip1271" || o.Signature != "0x" || o.SellTokenBalance != "erc20" || o.BuyTokenBalance != "erc20" || o.ValidTo <= 0 || uint64(o.ValidTo) > uint64(^uint32(0)) {
		return errors.New("persisted CoW order violates configured account policy")
	}
	return nil
}
func sameAddress(s string, a common.Address) bool {
	p, e := parse.Address(s)
	return e == nil && p == a
}

// EncodeOrder is abi.encode(GPv2Order.TYPE_HASH, order), including hashed string fields.
func EncodeOrder(o wire.CowOrder) ([]byte, error) {
	sell, e := parse.Address(o.SellToken)
	if e != nil {
		return nil, e
	}
	buy, e := parse.Address(o.BuyToken)
	if e != nil {
		return nil, e
	}
	receiver, e := parse.Address(o.Receiver)
	if e != nil {
		return nil, e
	}
	sa, e := parse.Uint256(o.SellAmount)
	if e != nil {
		return nil, e
	}
	ba, e := parse.Uint256(o.BuyAmount)
	if e != nil {
		return nil, e
	}
	fee, e := parse.Uint256(o.FeeAmount)
	if e != nil {
		return nil, e
	}
	app, e := protocolio.Hash(o.AppData)
	if e != nil {
		return nil, e
	}
	if o.ValidTo < 0 || uint64(o.ValidTo) > uint64(^uint32(0)) {
		return nil, errors.New("CoW validTo outside uint32")
	}
	typeHash := crypto.Keccak256([]byte("Order(address sellToken,address buyToken,address receiver,uint256 sellAmount,uint256 buyAmount,uint32 validTo,bytes32 appData,uint256 feeAmount,string kind,bool partiallyFillable,string sellTokenBalance,string buyTokenBalance)"))
	partial := big.NewInt(0)
	if o.PartiallyFillable {
		partial.SetInt64(1)
	}
	parts := [][]byte{typeHash, protocolio.AddressWord(sell), protocolio.AddressWord(buy), protocolio.AddressWord(receiver), protocolio.Word(sa), protocolio.Word(ba), protocolio.Word(big.NewInt(o.ValidTo)), app.Bytes(), protocolio.Word(fee), crypto.Keccak256([]byte(o.Kind)), protocolio.Word(partial), crypto.Keccak256([]byte(o.SellTokenBalance)), crypto.Keccak256([]byte(o.BuyTokenBalance))}
	buf := make([]byte, 0, 416)
	for _, part := range parts {
		buf = append(buf, part...)
	}
	return buf, nil
}
func UID(digest common.Hash, owner common.Address, validTo uint32) string {
	b := make([]byte, 56)
	copy(b, digest[:])
	copy(b[32:], owner[:])
	binary.BigEndian.PutUint32(b[52:], validTo)
	return hexutil.Encode(b)
}
