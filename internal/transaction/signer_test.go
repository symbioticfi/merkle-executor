package transaction

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/symbioticfi/merkle-executor/internal/signer"
)

type mutatingSigner struct {
	signer.Signer
	change string
}

func (s mutatingSigner) SignTx(ctx context.Context, tx *types.Transaction, chain *big.Int) (*types.Transaction, error) {
	if s.change == "nil" {
		return nil, nil
	}
	to := *tx.To()
	payload := &types.DynamicFeeTx{ChainID: tx.ChainId(), Nonce: tx.Nonce(), To: &to, Data: append([]byte(nil), tx.Data()...), Value: tx.Value(), Gas: tx.Gas(), GasFeeCap: tx.GasFeeCap(), GasTipCap: tx.GasTipCap()}
	switch s.change {
	case "chain":
		payload.ChainID = big.NewInt(1)
		chain = big.NewInt(1)
	case "nonce":
		payload.Nonce++
	case "target":
		payload.To = new(common.Address)
	case "data":
		payload.Data[0]++
	case "value":
		payload.Value = big.NewInt(1)
	case "gas", "gas in place":
		payload.Gas++
	case "fee":
		payload.GasFeeCap.Add(payload.GasFeeCap, big.NewInt(1))
	case "priority":
		payload.GasTipCap.Add(payload.GasTipCap, big.NewInt(1))
	case "access list":
		payload.AccessList = types.AccessList{{Address: common.HexToAddress("0x3456")}}
	case "creation":
		payload.To = nil
	case "type":
		return s.Signer.SignTx(ctx, types.NewTx(&types.LegacyTx{Nonce: tx.Nonce(), To: tx.To(), Data: tx.Data(), Gas: tx.Gas(), GasPrice: tx.GasFeeCap()}), chain)
	}
	changed := types.NewTx(payload)
	if s.change == "gas in place" {
		raw, err := changed.MarshalBinary()
		if err != nil {
			return nil, err
		}
		if err := tx.UnmarshalBinary(raw); err != nil {
			return nil, err
		}
		changed = tx
	}
	return s.Signer.SignTx(ctx, changed, chain)
}

func TestSignerCannotChangeAuthorizedTransaction(t *testing.T) {
	for _, change := range []string{"nil", "chain", "nonce", "target", "data", "value", "gas", "gas in place", "fee", "priority", "access list", "creation", "type", "sender"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture()
			base := testSigner(t)
			signing := signer.Signer(mutatingSigner{Signer: base, change: change})
			if change == "sender" {
				other, err := signer.NewFromHexKey("0000000000000000000000000000000000000000000000000000000000000001")
				if err != nil {
					t.Fatal(err)
				}
				signing = wrongSenderSigner{Signer: other, address: base.Address()}
			}
			d, err := New(f, f, signing, big.NewInt(31337), config())
			if err != nil {
				t.Fatal(err)
			}
			r := d.Execute(t.Context(), intent())
			if !r.NotSubmitted || r.Err == nil || len(f.sent()) != 0 {
				t.Fatalf("signer mutation accepted: %+v", r)
			}
		})
	}
}

type wrongSenderSigner struct {
	signer.Signer
	address common.Address
}

func (s wrongSenderSigner) Address() common.Address { return s.address }
