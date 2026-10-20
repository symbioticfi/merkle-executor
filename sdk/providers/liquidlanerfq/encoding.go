package liquidlanerfq

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	reactor "github.com/symbioticfi/merkle-executor/sdk/bindings/liquidlanereactor"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	wire "github.com/symbioticfi/merkle-executor/sdk/protocoltypes"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

var requestTypeHash = crypto.Keccak256Hash([]byte("Request(address tokenIn,uint256 amountIn,Output[] outputs,uint256 deadline,uint256 nonce,address protocol)Output(address token,uint256 amount,address recipient)"))
var outputTypeHash = crypto.Keccak256Hash([]byte("Output(address token,uint256 amount,address recipient)"))

func strictJSON(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return errors.New("RFQ malformed authority data")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("RFQ multiple JSON values")
	}
	return nil
}
func decimal(s string) (*big.Int, error) {
	if s == "" {
		return nil, errors.New("RFQ missing uint256")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return nil, errors.New("RFQ invalid decimal uint256")
		}
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return new(big.Int), nil
	}
	if len(s) > 78 {
		return nil, errors.New("RFQ uint256 overflow")
	}
	return parse.Uint256(s)
}
func sameAddress(s string, a common.Address) bool {
	return common.IsHexAddress(s) && common.HexToAddress(s) == a
}
func (p *Provider) quote(raw json.RawMessage, owner common.Address) (reactor.IReactorRequest, wire.RfqQuote, error) {
	var q wire.RfqQuote
	var r reactor.IReactorRequest
	if json.Unmarshal(raw, &q) != nil || !uuid.MatchString(q.QuoteID) || len(q.OrderInfo.Outputs) != 1 || len(q.AggregatedOutputs) != 1 || q.OrderInfo.Deadline <= 0 || uint64(q.OrderInfo.Deadline) > maxSafeInteger {
		return r, q, errors.New("RFQ malformed quote identity or output shape")
	}
	o := q.OrderInfo.Outputs[0]
	ag := q.AggregatedOutputs[0]
	amount, err := decimal(q.OrderInfo.AmountIn)
	if err != nil || amount.Cmp(p.amount) != 0 || !sameAddress(q.OrderInfo.TokenIn, p.tokenIn) {
		return r, q, errors.New("RFQ quote input mismatch")
	}
	output, err := decimal(o.Amount)
	if err != nil || output.Sign() == 0 || !sameAddress(o.Token, p.tokenOut) || !sameAddress(o.Recipient, owner) {
		return r, q, errors.New("RFQ quote output mismatch")
	}
	aggregate, err := decimal(ag.Amount)
	if err != nil || aggregate.Cmp(output) != 0 || !sameAddress(ag.Token, p.tokenOut) {
		return r, q, errors.New("RFQ quote aggregate mismatch")
	}
	nonce, err := decimal(q.OrderInfo.Nonce)
	if err != nil {
		return r, q, err
	}
	r = reactor.IReactorRequest{TokenIn: p.tokenIn, AmountIn: amount, Outputs: []reactor.IReactorOutput{{Token: p.tokenOut, Amount: output, Recipient: owner}}, Deadline: big.NewInt(q.OrderInfo.Deadline), Nonce: nonce, Protocol: p.protocol}
	return r, q, nil
}

type typedField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (p *Provider) typedData(data wire.RfqSignatureData, s provider.Snapshot, r reactor.IReactorRequest) error {
	var domain struct {
		Name              string `json:"name"`
		Version           string `json:"version"`
		ChainID           int64  `json:"chainId"`
		VerifyingContract string `json:"verifyingContract"`
	}
	if strictJSON(data.Domain, &domain) != nil || domain.Name != "Reactor" || domain.Version != "1" || domain.ChainID != s.ChainID.Int64() || !sameAddress(domain.VerifyingContract, p.reactor) || data.PrimaryType != "Request" {
		return errors.New("RFQ typed domain mismatch")
	}
	var schema map[string][]typedField
	expected := map[string][]typedField{"Output": {{"token", "address"}, {"amount", "uint256"}, {"recipient", "address"}}, "Request": {{"tokenIn", "address"}, {"amountIn", "uint256"}, {"outputs", "Output[]"}, {"deadline", "uint256"}, {"nonce", "uint256"}, {"protocol", "address"}}}
	if strictJSON(data.Types, &schema) != nil || !reflect.DeepEqual(schema, expected) {
		return errors.New("RFQ typed schema mismatch")
	}
	var value struct {
		TokenIn  string           `json:"tokenIn"`
		AmountIn string           `json:"amountIn"`
		Outputs  []wire.RfqOutput `json:"outputs"`
		Deadline string           `json:"deadline"`
		Nonce    string           `json:"nonce"`
		Protocol string           `json:"protocol"`
	}
	if strictJSON(data.Value, &value) != nil || len(value.Outputs) != 1 || !sameAddress(value.TokenIn, r.TokenIn) || !sameAddress(value.Protocol, r.Protocol) || !sameAddress(value.Outputs[0].Token, r.Outputs[0].Token) || !sameAddress(value.Outputs[0].Recipient, r.Outputs[0].Recipient) {
		return errors.New("RFQ typed request identity mismatch")
	}
	for _, v := range []struct {
		s string
		n *big.Int
	}{{value.AmountIn, r.AmountIn}, {value.Outputs[0].Amount, r.Outputs[0].Amount}, {value.Deadline, r.Deadline}, {value.Nonce, r.Nonce}} {
		n, err := decimal(v.s)
		if err != nil || n.Cmp(v.n) != 0 {
			return errors.New("RFQ typed request amount mismatch")
		}
	}
	return nil
}
func hashPlan() []account.IMerkleAccountHash {
	return []account.IMerkleAccountHash{{Offset: big.NewInt(224), Length: big.NewInt(128), Destination: big.NewInt(224)}, {Offset: big.NewInt(224), Length: big.NewInt(32), Destination: big.NewInt(96)}, {Offset: big.NewInt(0), Length: big.NewInt(224), Destination: big.NewInt(0)}}
}
func authorization(domain common.Hash, r reactor.IReactorRequest) ([]byte, common.Hash, error) {
	if len(r.Outputs) != 1 || r.AmountIn == nil || r.Nonce == nil || r.Deadline == nil || r.Outputs[0].Amount == nil {
		return nil, common.Hash{}, errors.New("RFQ invalid request encoding")
	}
	for _, n := range []*big.Int{r.AmountIn, r.Nonce, r.Deadline, r.Outputs[0].Amount} {
		if n.Sign() < 0 || n.BitLen() > 256 {
			return nil, common.Hash{}, errors.New("RFQ integer outside uint256")
		}
	}
	o := r.Outputs[0]
	buffer := make([]byte, 0, 352)
	for _, word := range [][]byte{requestTypeHash[:], r.TokenIn.Bytes(), r.AmountIn.Bytes(), make([]byte, 32), r.Deadline.Bytes(), r.Nonce.Bytes(), r.Protocol.Bytes(), outputTypeHash[:], o.Token.Bytes(), o.Amount.Bytes(), o.Recipient.Bytes()} {
		buffer = append(buffer, common.LeftPadBytes(word, 32)...)
	}
	collapsed := append([]byte(nil), buffer...)
	copy(collapsed[224:256], crypto.Keccak256(collapsed[224:352]))
	copy(collapsed[96:128], crypto.Keccak256(collapsed[224:256]))
	digest := protocolio.Digest(domain, crypto.Keccak256Hash(collapsed[:224]))
	data, err := account.NewMerkleAccount().TryPackSign(domain, buffer, hashPlan())
	return data, digest, err
}
func equalRequest(a, b reactor.IReactorRequest) bool {
	if a.TokenIn != b.TokenIn || a.Protocol != b.Protocol || len(a.Outputs) != 1 || len(b.Outputs) != 1 || a.Outputs[0].Token != b.Outputs[0].Token || a.Outputs[0].Recipient != b.Outputs[0].Recipient {
		return false
	}
	for _, pair := range [][2]*big.Int{{a.AmountIn, b.AmountIn}, {a.Nonce, b.Nonce}, {a.Deadline, b.Deadline}, {a.Outputs[0].Amount, b.Outputs[0].Amount}} {
		if pair[0] == nil || pair[1] == nil || pair[0].Cmp(pair[1]) != 0 {
			return false
		}
	}
	return true
}
func authorizingSign(e *provider.Execution, d prepared, domain common.Hash) error {
	a := account.NewMerkleAccount()
	expected, _, err := authorization(domain, d.Request)
	if err != nil {
		return err
	}
	count := 0
	for _, log := range e.Logs {
		if log.Address != d.Account || len(log.Topics) == 0 || log.Topics[0] != a.GetABI().Events["Sign"].ID {
			continue
		}
		if len(log.Topics) != 3 {
			return errors.New("RFQ malformed authorization event")
		}
		event, err := a.UnpackSignEvent(&log)
		if err != nil {
			return errors.New("RFQ malformed authorization event")
		}
		if common.Hash(event.Digest) != d.Digest {
			continue
		}
		payload, err := a.TryPackSign(event.DomainSeparator, event.Buffer, event.Plan)
		if err != nil || !bytes.Equal(payload, expected) {
			return errors.New("RFQ authorizing Sign differs from request")
		}
		count++
	}
	if count != 1 {
		return errors.New("RFQ exact authorizing Sign event required")
	}
	return nil
}
