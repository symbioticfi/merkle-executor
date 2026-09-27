// Package liquidlane resolves signed inventory discounts for LiquidLaneConnector.
package liquidlane

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	binding "github.com/symbioticfi/merkle-executor/api/bindings/liquidconnector"
	accountbinding "github.com/symbioticfi/merkle-executor/api/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/api/protocolio"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	"github.com/symbioticfi/merkle-executor/internal/parse"
	"github.com/symbioticfi/merkle-executor/internal/provider"
)

type Config struct {
	HTTP               protocolio.HTTPConfig `yaml:"http"`
	Format             string                `yaml:"format"`
	Connector          string                `yaml:"connector"`
	Token              string                `yaml:"token"`
	InputAsset         string                `yaml:"inputAsset"`
	Recipient          string                `yaml:"recipient"`
	Protocol           string                `yaml:"protocol"`
	DiscountID         string                `yaml:"discountId"`
	AmountIn           string                `yaml:"amountIn"`
	MaxAmountOut       string                `yaml:"maxAmountOut"`
	Action             string                `yaml:"action"`
	CalldataBytes      int                   `yaml:"calldataBytes"`
	MaxSignatureBytes  int                   `yaml:"maxSignatureBytes"`
	MinValiditySeconds uint64                `yaml:"minValiditySeconds"`
	AmountInInput      *uint                 `yaml:"amountInInput"`
	AmountOutInput     *uint                 `yaml:"amountOutInput"`
}

// Client deliberately separates unsigned discovery from fresh signed resolution.
type Client struct{ http *protocolio.Client }

func NewClient(c protocolio.HTTPConfig, h *http.Client) (*Client, error) {
	x, e := protocolio.NewHTTP(c, h)
	if e != nil {
		return nil, e
	}
	return &Client{http: x}, nil
}
func (c *Client) List(ctx context.Context) (wire.LLListResponse, error) {
	var v wire.LLListResponse
	_, e := c.http.JSON(ctx, http.MethodGet, "/api-internal/v1/discounts", nil, &v)
	return v, e
}
func (c *Client) Resolve(ctx context.Context, id string) (wire.LLResolved, error) {
	var v wire.LLResolveResponse
	_, e := c.http.JSON(ctx, http.MethodPost, "/api-internal/v1/discounts", map[string]string{"discountId": id}, &v)
	if e != nil {
		return wire.LLResolved{}, e
	}
	if len(v.Discounts) > 0 {
		if len(v.Discounts) != 1 || v.DiscountID != "" {
			return wire.LLResolved{}, errors.New("ambiguous LL resolution")
		}
		return v.Discounts[0], nil
	}
	if v.DiscountID == "" {
		return wire.LLResolved{}, errors.New("empty LL resolution")
	}
	return wire.LLResolved{DiscountID: v.DiscountID, Discount: v.Discount, SignerSignature: v.SignerSignature, ProtocolDeadline: v.ProtocolDeadline, ProtocolSignature: v.ProtocolSignature}, nil
}

type Provider struct {
	cfg                                          Config
	deps                                         provider.Deps
	client                                       *Client
	connector, token, asset, recipient, protocol common.Address
	amount, maxOut                               *big.Int
	discountID                                   common.Hash
}

func Register(r *provider.Registry) error { return r.Register("liquidlane", Factory) }
func Factory(raw yaml.Node, deps provider.Deps) (provider.Provider, error) {
	var c Config
	if e := parse.DecodeStrict(raw, &c); e != nil {
		return nil, e
	}
	if c.Format != "connector-v1" {
		return nil, errors.New("LL format must be connector-v1; LiquidLaneAdapter signatures are incompatible")
	}
	if e := protocolio.Action(c.Action); e != nil {
		return nil, e
	}
	if c.CalldataBytes < 4 || c.CalldataBytes > 65536 {
		return nil, errors.New("LL calldataBytes must bind an approved layout of 4..65536 bytes")
	}
	if c.MaxSignatureBytes == 0 {
		c.MaxSignatureBytes = 1024
	}
	if c.MaxSignatureBytes < 1 || c.MaxSignatureBytes > 8192 {
		return nil, errors.New("LL maxSignatureBytes outside 1..8192")
	}
	p := &Provider{cfg: c, deps: deps}
	var e error
	for _, x := range []struct {
		src string
		dst *common.Address
	}{{c.Connector, &p.connector}, {c.Token, &p.token}, {c.InputAsset, &p.asset}, {c.Recipient, &p.recipient}, {c.Protocol, &p.protocol}} {
		*x.dst, e = parse.Address(x.src)
		if e != nil {
			return nil, e
		}
	}
	if p.token == p.asset {
		return nil, errors.New("LL input asset and RWA must differ")
	}
	p.discountID, e = protocolio.Hash(c.DiscountID)
	if e != nil {
		return nil, e
	}
	p.amount, e = parse.Uint256(c.AmountIn)
	if e != nil || p.amount.Sign() == 0 {
		return nil, errors.New("LL amountIn must be positive uint256")
	}
	p.maxOut, e = parse.Uint256(c.MaxAmountOut)
	if e != nil || p.maxOut.Sign() == 0 {
		return nil, errors.New("LL maxAmountOut must be positive uint256")
	}
	if c.AmountInInput != nil && c.AmountOutInput != nil && *c.AmountInInput == *c.AmountOutInput {
		return nil, errors.New("LL duplicate input index")
	}
	p.client, e = NewClient(c.HTTP, deps.HTTP)
	if e != nil {
		return nil, e
	}
	if deps.Chain == nil {
		return nil, errors.New("LL chain reader required")
	}
	return p, nil
}
func (p *Provider) Prepare(ctx context.Context, s provider.Snapshot) (provider.Output, error) {
	if e := protocolio.Snapshot(s); e != nil {
		return provider.Output{}, e
	}
	a := binding.NewLiquidConnector()
	call := func(data []byte) ([]byte, error) { return protocolio.CallData(ctx, p.deps.Chain, s, p.connector, data) }
	raw, e := call(a.PackAccount())
	if e != nil {
		return provider.Output{}, e
	}
	account, e := a.UnpackAccount(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid LL account response")
	}
	if account != s.Account {
		return provider.Output{}, errors.New("LL connector account mismatch")
	}
	raw, e = call(a.PackInputAsset())
	if e != nil {
		return provider.Output{}, e
	}
	inputAsset, e := a.UnpackInputAsset(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid LL input asset response")
	}
	if inputAsset != p.asset {
		return provider.Output{}, errors.New("LL connector input asset mismatch")
	}
	raw, e = call(a.PackEip712Domain())
	if e != nil {
		return provider.Output{}, e
	}
	declaredDomain, e := a.UnpackEip712Domain(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid LL EIP712 domain response")
	}
	if declaredDomain.Fields != [1]byte{15} || declaredDomain.Salt != [32]byte{} || len(declaredDomain.Extensions) != 0 || declaredDomain.Name != "LiquidLaneConnector" || declaredDomain.Version != "1" || declaredDomain.ChainId.Cmp(s.ChainID) != 0 || declaredDomain.VerifyingContract != p.connector {
		return provider.Output{}, errors.New("LL connector EIP712 domain mismatch")
	}
	r, e := p.client.Resolve(ctx, p.cfg.DiscountID)
	if e != nil {
		return provider.Output{}, e
	}
	id, e := protocolio.Hash(r.DiscountID)
	if e != nil || id != p.discountID {
		return provider.Output{}, errors.New("LL resolved discount identity mismatch")
	}
	d, signerSig, protocolSig, e := p.terms(r, s)
	if e != nil {
		return provider.Output{}, e
	}
	raw, e = call(a.PackOwner())
	if e != nil {
		return provider.Output{}, e
	}
	owner, e := a.UnpackOwner(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid LL owner response")
	}
	raw, e = call(a.PackSigner())
	if e != nil {
		return provider.Output{}, e
	}
	signer, e := a.UnpackSigner(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid LL signer response")
	}
	if d.Signer != owner && d.Signer != signer {
		return provider.Output{}, errors.New("LL discount signer is no longer authorized")
	}
	raw, e = call(a.PackIsUsedNonce(p.token, d.Nonce))
	if e != nil {
		return provider.Output{}, e
	}
	used, e := a.UnpackIsUsedNonce(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid LL nonce response")
	}
	if used {
		return provider.Output{}, errors.New("LL discount nonce invalidated")
	}
	raw, e = call(a.PackMinDiscount(p.token))
	if e != nil {
		return provider.Output{}, e
	}
	minimum, e := a.UnpackMinDiscount(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid LL minimum discount response")
	}
	if d.Discount.Cmp(minimum) < 0 {
		return provider.Output{}, errors.New("LL discount below current minimum")
	}
	domain := protocolio.Domain("LiquidLaneConnector", "1", s.ChainID, p.connector)
	dh, wh := Hashes(domain, d, signerSig, big.NewInt(r.ProtocolDeadline))
	if dh != p.discountID {
		return provider.Output{}, errors.New("LL discount ID/domain mismatch: legacy LiquidLaneAdapter terms are unsupported")
	}
	if !validSignature(ctx, p.deps.Chain, s, d.Signer, dh, signerSig) || !validSignature(ctx, p.deps.Chain, s, d.Protocol, wh, protocolSig) {
		return provider.Output{}, errors.New("LL signatures do not validate for connector domain; legacy adapter signatures are unsupported")
	}
	raw, e = call(a.PackGetAmountOut(p.token, p.amount))
	if e != nil {
		return provider.Output{}, e
	}
	quote, e := a.UnpackGetAmountOut(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid LL quote response")
	}
	out := new(big.Int).Mul(quote, new(big.Int).Sub(big.NewInt(1_000_000), d.Discount))
	if out.BitLen() > 256 {
		return provider.Output{}, errors.New("LL quote multiplication overflows connector arithmetic")
	}
	out.Div(out, big.NewInt(1_000_000))
	if out.Sign() == 0 || out.Cmp(p.maxOut) > 0 {
		return provider.Output{}, errors.New("LL current quote outside configured payment bound")
	}
	raw, e = call(a.PackRemainingAssets(p.token))
	if e != nil {
		return provider.Output{}, e
	}
	remaining, e := a.UnpackRemainingAssets(raw)
	if e != nil {
		return provider.Output{}, errors.New("invalid LL remaining budget response")
	}
	if out.Cmp(remaining) > 0 {
		return provider.Output{}, errors.Errorf("%w: LL current budget insufficient", provider.ErrNotReady)
	}
	payload, e := a.TryPackSwap0(binding.ILiquidLaneConnectorDiscountSwap{Discount: d, SignerSignature: signerSig, ProtocolDeadline: big.NewInt(r.ProtocolDeadline)}, protocolSig, p.recipient, p.amount)
	if e != nil {
		return provider.Output{}, errors.New("encode LL connector swap")
	}
	if len(payload) != p.cfg.CalldataBytes {
		return provider.Output{}, errors.New("LL signed payload does not fit approved calldataBytes layout")
	}
	inputs := map[uint]string{}
	if p.cfg.AmountInInput != nil {
		inputs[*p.cfg.AmountInInput] = p.amount.String()
	}
	if p.cfg.AmountOutInput != nil {
		inputs[*p.cfg.AmountOutInput] = out.String()
	}
	data, _ := json.Marshal(map[string]string{"discountId": dh.Hex(), "amountOut": out.String()})
	return provider.Output{Inputs: inputs, Actions: map[string]hexutil.Bytes{p.cfg.Action: payload}, Data: data}, nil
}
func (p *Provider) terms(r wire.LLResolved, s provider.Snapshot) (binding.ILiquidLaneConnectorDiscount, []byte, []byte, error) {
	var d binding.ILiquidLaneConnectorDiscount
	fail := func() (binding.ILiquidLaneConnectorDiscount, []byte, []byte, error) {
		return d, nil, nil, errors.New("invalid or mismatched LL connector terms")
	}
	addr, e := parse.Address(r.Discount.Adapter)
	if e != nil || addr != p.connector {
		return fail()
	}
	d.TokenToRedeem, e = parse.Address(r.Discount.TokenToRedeem)
	if e != nil || d.TokenToRedeem != p.token {
		return fail()
	}
	d.Signer, e = parse.Address(r.Discount.Signer)
	if e != nil {
		return fail()
	}
	d.Protocol, e = parse.Address(r.Discount.Protocol)
	if e != nil || d.Protocol != p.protocol {
		return fail()
	}
	d.Discount, e = parse.Uint256(r.Discount.Discount)
	if e != nil || d.Discount.Cmp(big.NewInt(1_000_000)) > 0 {
		return fail()
	}
	d.Nonce, e = parse.Uint256(r.Discount.Nonce)
	if e != nil {
		return fail()
	}
	if r.Discount.Deadline <= 0 || r.ProtocolDeadline <= 0 || uint64(r.Discount.Deadline) >= 1<<48 || uint64(r.ProtocolDeadline) >= 1<<48 || s.Timestamp > ^uint64(0)-p.cfg.MinValiditySeconds {
		return fail()
	}
	min := s.Timestamp + p.cfg.MinValiditySeconds
	if uint64(r.Discount.Deadline) < min || uint64(r.ProtocolDeadline) < min {
		return fail()
	}
	d.Deadline = big.NewInt(r.Discount.Deadline)
	ss, e := protocolio.Bytes(r.SignerSignature, p.cfg.MaxSignatureBytes)
	if e != nil || len(ss) == 0 {
		return fail()
	}
	ps, e := protocolio.Bytes(r.ProtocolSignature, p.cfg.MaxSignatureBytes)
	if e != nil || len(ps) == 0 {
		return fail()
	}
	return d, ss, ps, nil
}

// Hashes computes the exact connector Discount and DiscountSwap EIP712 digests.
func Hashes(domain common.Hash, d binding.ILiquidLaneConnectorDiscount, signature []byte, deadline *big.Int) (common.Hash, common.Hash) {
	th := crypto.Keccak256([]byte("Discount(address tokenToRedeem,uint256 discount,address signer,address protocol,uint256 nonce,uint48 deadline)"))
	h := crypto.Keccak256Hash(th, protocolio.AddressWord(d.TokenToRedeem), protocolio.Word(d.Discount), protocolio.AddressWord(d.Signer), protocolio.AddressWord(d.Protocol), protocolio.Word(d.Nonce), protocolio.Word(d.Deadline))
	wh := crypto.Keccak256Hash(crypto.Keccak256([]byte("DiscountSwap(Discount discount,bytes signerSignature,uint48 protocolDeadline)Discount(address tokenToRedeem,uint256 discount,address signer,address protocol,uint256 nonce,uint48 deadline)")), h.Bytes(), crypto.Keccak256(signature), protocolio.Word(deadline))
	return protocolio.Digest(domain, h), protocolio.Digest(domain, wh)
}
func validSignature(ctx context.Context, r provider.Reader, s provider.Snapshot, who common.Address, digest common.Hash, sig []byte) bool {
	if len(sig) == 65 {
		v := append([]byte(nil), sig...)
		if v[64] >= 27 {
			v[64] -= 27
		}
		rr := new(big.Int).SetBytes(v[:32])
		ss := new(big.Int).SetBytes(v[32:64])
		if crypto.ValidateSignatureValues(v[64], rr, ss, true) {
			pub, e := crypto.SigToPub(digest.Bytes(), v)
			if e == nil && crypto.PubkeyToAddress(*pub) == who {
				return true
			}
		}
	}
	a := accountbinding.NewMerkleAccount()
	raw, e := protocolio.CallData(ctx, r, s, who, a.PackIsValidSignature(digest, sig))
	if e != nil {
		return false
	}
	magic, e := a.UnpackIsValidSignature(raw)
	return e == nil && magic == [4]byte{0x16, 0x26, 0xba, 0x7e}
}
func (p *Provider) Advance(_ context.Context, _ provider.Snapshot, _ provider.Output, _ json.RawMessage) (provider.Progress, error) {
	return provider.Progress{Done: true}, nil
}
