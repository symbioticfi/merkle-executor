package liquidlanediscount

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/liquidlaneconnector"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"math/big"
	"net/http"
)

type Terms = wire.LiquidLaneDiscountTerms
type Resolved = wire.LiquidLaneResolvedDiscount
type scope struct{ Connector, Token, Signer, Protocol, Recipient common.Address }

var discountTypeHash = crypto.Keccak256Hash([]byte("Discount(address tokenToRedeem,uint256 discount,address signer,address protocol,uint256 nonce,uint48 deadline)"))
var discountSwapTypeHash = crypto.Keccak256Hash([]byte("DiscountSwap(Discount discount,bytes signerSignature,uint48 protocolDeadline)Discount(address tokenToRedeem,uint256 discount,address signer,address protocol,uint256 nonce,uint48 deadline)"))

// validateWitness verifies the connector's 1e8-precision typed data without
// converting the API value or accepting another contract's domain.
func validateWitness(s provider.Snapshot, sc scope, w Resolved, validators ...func(string, common.Hash, common.Address) ([]byte, error)) (binding.ILiquidLaneConnectorDiscountSwap, error) {
	fail := func(message string) (binding.ILiquidLaneConnectorDiscountSwap, error) {
		return binding.ILiquidLaneConnectorDiscountSwap{}, errors.New(message)
	}
	for _, identity := range []struct {
		wire string
		want common.Address
	}{{w.Discount.Adapter, sc.Connector}, {w.Discount.TokenToRedeem, sc.Token}, {w.Discount.Signer, sc.Signer}, {w.Discount.Protocol, sc.Protocol}} {
		address, err := parse.Address(identity.wire)
		if err != nil || address != identity.want {
			return fail("discount witness identity differs from committed connector, token, signer or protocol")
		}
	}
	discount, err := parse.Uint256(w.Discount.Discount)
	if err != nil {
		return fail("discount witness requires uint256 discount")
	}
	nonce, err := parse.Uint256(w.Discount.Nonce)
	if err != nil {
		return fail("discount witness requires uint256 nonce")
	}
	if discount.Cmp(big.NewInt(100000000)) > 0 {
		return fail("discount exceeds connector precision 100000000")
	}
	if w.Discount.Deadline <= 0 || w.ProtocolDeadline <= 0 || uint64(w.Discount.Deadline) < s.Timestamp || uint64(w.ProtocolDeadline) < s.Timestamp || w.Discount.Deadline >= 1<<48 || w.ProtocolDeadline >= 1<<48 {
		return fail("discount witness deadline expired or exceeds uint48")
	}
	deadline, protocolDeadline := big.NewInt(w.Discount.Deadline), big.NewInt(w.ProtocolDeadline)
	structure := crypto.Keccak256Hash(discountTypeHash.Bytes(), protocolio.AddressWord(sc.Token), protocolio.Word(discount), protocolio.AddressWord(sc.Signer), protocolio.AddressWord(sc.Protocol), protocolio.Word(nonce), protocolio.Word(deadline))
	domain := protocolio.Domain("LiquidLaneConnector", "1", s.ChainID, sc.Connector)
	digest := protocolio.Digest(domain, structure)
	verify := signature
	if len(validators) > 0 {
		verify = validators[0]
	}
	signerSignature, err := verify(w.SignerSignature, digest, sc.Signer)
	if err != nil {
		return fail("discount witness signer signature does not verify for LiquidLaneConnector domain")
	}
	swap := crypto.Keccak256Hash(discountSwapTypeHash.Bytes(), structure.Bytes(), crypto.Keccak256(signerSignature), protocolio.Word(protocolDeadline))
	if _, err = verify(w.ProtocolSignature, protocolio.Digest(domain, swap), sc.Protocol); err != nil {
		return fail("discount witness protocol signature does not verify for LiquidLaneConnector domain")
	}
	id, err := protocolio.Hash(w.DiscountID)
	if err != nil || id != digest {
		return fail("discount ID differs from connector typed-data digest")
	}
	return binding.ILiquidLaneConnectorDiscountSwap{Discount: binding.ILiquidLaneConnectorDiscount{TokenToRedeem: sc.Token, Discount: discount, Signer: sc.Signer, Protocol: sc.Protocol, Nonce: nonce, Deadline: deadline}, SignerSignature: signerSignature, ProtocolDeadline: protocolDeadline}, nil
}
func signature(raw string, digest common.Hash, want common.Address) ([]byte, error) {
	sig, err := protocolio.Bytes(raw, 65)
	if err != nil || len(sig) != 65 || sig[64] < 27 || sig[64] > 28 {
		return nil, errors.New("expected canonical 65-byte EOA signature")
	}
	normalized := bytes.Clone(sig)
	normalized[64] -= 27
	if !crypto.ValidateSignatureValues(normalized[64], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:64]), true) {
		return nil, errors.New("noncanonical EOA signature")
	}
	key, err := crypto.SigToPub(digest.Bytes(), normalized)
	if err != nil || crypto.PubkeyToAddress(*key) != want {
		return nil, errors.New("signature signer mismatch")
	}
	return sig, nil
}
func resolve(ctx context.Context, client *protocolio.Client, sc scope) (Resolved, error) {
	var raw json.RawMessage
	selector := struct {
		Adapter       string `json:"adapter"`
		TokenToRedeem string `json:"tokenToRedeem"`
	}{sc.Connector.Hex(), sc.Token.Hex()}
	if _, err := client.JSON(ctx, http.MethodPost, "/api-internal/v1/discounts", selector, &raw); err != nil {
		return Resolved{}, err
	}
	var witness Resolved
	if err := derived.StrictJSON(raw, &witness); err != nil {
		return Resolved{}, errors.New("invalid signed-discount response wire")
	}
	return witness, nil
}

func signatureAt(ctx context.Context, chain provider.Reader, s provider.Snapshot, raw string, digest common.Hash, want common.Address) ([]byte, error) {
	sig, err := protocolio.Bytes(raw, 65)
	if err != nil || len(sig) != 65 {
		return nil, errors.New("expected fixed 65-byte signature")
	}
	if checked, err := signature(raw, digest, want); err == nil {
		return checked, nil
	}
	args := arguments("bytes32", "bytes")
	packed, err := args.Pack(digest, sig)
	if err != nil {
		return nil, err
	}
	data := append(crypto.Keccak256([]byte("isValidSignature(bytes32,bytes)"))[:4], packed...)
	response, err := protocolio.CallData(ctx, chain, s, want, data)
	expected := append(common.FromHex("0x1626ba7e"), make([]byte, 28)...)
	if err != nil || !bytes.Equal(response, expected) {
		return nil, errors.New("pinned ERC1271 signature validation failed")
	}
	return sig, nil
}
