package liquidlanerfq

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	reactor "github.com/symbioticfi/merkle-executor/sdk/bindings/liquidlanereactor"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func validHeader(h *types.Header, n uint64) bool {
	return h != nil && h.Number != nil && h.Number.IsUint64() && h.Number.Uint64() == n
}

// The orders API locates the transaction; only its canonical receipt proves a fill.
func (p *Provider) fillReceipt(ctx context.Context, s provider.Snapshot, d prepared, hash common.Hash) error {
	receipt, err := p.chain.TransactionReceipt(ctx, hash)
	if err != nil {
		return errors.Errorf("RFQ fill receipt unavailable: %w", err)
	}
	if receipt == nil || receipt.BlockNumber == nil || !receipt.BlockNumber.IsUint64() || receipt.BlockNumber.Uint64() < d.PreparedBlock || receipt.BlockNumber.Cmp(s.BlockNumber) > 0 || receipt.BlockHash == (common.Hash{}) || receipt.TxHash != hash || receipt.Status != types.ReceiptStatusSuccessful {
		return errors.New("RFQ fill lacks successful pinned receipt")
	}
	header, err := p.chain.HeaderByNumber(ctx, receipt.BlockNumber)
	if err != nil || !validHeader(header, receipt.BlockNumber.Uint64()) || header.Hash() != receipt.BlockHash {
		return errors.New("RFQ fill receipt is noncanonical")
	}
	r := reactor.NewLiquidLaneReactor()
	fillTopic := r.GetABI().Events["Fill"].ID
	found := false
	for _, log := range receipt.Logs {
		if log == nil || log.Address != p.reactor || len(log.Topics) == 0 || log.Topics[0] != fillTopic {
			continue
		}
		if log.Removed || len(log.Topics) != 1 || log.BlockNumber != receipt.BlockNumber.Uint64() || log.BlockHash != receipt.BlockHash || log.TxHash != hash || log.TxIndex != receipt.TransactionIndex {
			return errors.New("RFQ invalid receipt Fill identity")
		}
		event, err := r.UnpackFillEvent(log)
		if err != nil || event.Order.Request.Nonce == nil {
			return errors.New("RFQ malformed receipt Fill")
		}
		order := event.Order
		if order.Swapper != d.Account || order.Request.Nonce.Cmp(d.Request.Nonce) != 0 {
			continue
		}
		if found || !equalRequest(order.Request, d.Request) {
			return errors.New("RFQ receipt repeats nonce or fills another request")
		}
		if len(order.Outputs) != 1 || order.Outputs[0].Amount == nil || order.Outputs[0].Amount.Cmp(d.Request.Outputs[0].Amount) < 0 || order.Outputs[0].Token != d.Request.Outputs[0].Token || order.Outputs[0].Recipient != d.Account {
			return errors.New("RFQ filled output mismatch")
		}
		found = true
	}
	if !found {
		return errors.New("RFQ matching Fill absent from order receipt")
	}
	closing, err := p.chain.HeaderByNumber(ctx, new(big.Int).Set(receipt.BlockNumber))
	if err != nil || !validHeader(closing, receipt.BlockNumber.Uint64()) || closing.Hash() != receipt.BlockHash {
		return errors.New("RFQ fill receipt reorganized")
	}
	return nil
}
