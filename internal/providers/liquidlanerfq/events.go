package liquidlanerfq

import (
	"bytes"
	"context"
	"math/big"
	"sort"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	reactor "github.com/symbioticfi/merkle-executor/api/bindings/liquidlanereactor"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

const maxRangeLogs = 1024

func validHeader(h *types.Header, n uint64) bool {
	return h != nil && h.Number != nil && h.Number.IsUint64() && h.Number.Uint64() == n
}
func (p *Provider) scan(ctx context.Context, s provider.Snapshot, d prepared, st *progressState, used bool) (string, bool, error) {
	if st.NextBlock > d.PreparedBlock {
		h, err := p.chain.HeaderByNumber(ctx, new(big.Int).SetUint64(st.NextBlock-1))
		if err != nil || !validHeader(h, st.NextBlock-1) {
			return "", false, errors.New("RFQ scan anchor unavailable")
		}
		if h.Hash() != st.Anchor {
			st.NextBlock = d.PreparedBlock
			st.Anchor = common.Hash{}
		}
	}
	head := s.BlockNumber.Uint64()
	if st.NextBlock > head {
		return "", true, nil
	}
	end := head
	if head-st.NextBlock >= p.cfg.EventRangeBlocks {
		end = st.NextBlock + p.cfg.EventRangeBlocks - 1
	}
	r := reactor.NewLiquidLaneReactor()
	abi := r.GetABI()
	fillTopic := abi.Events["Fill"].ID
	cancelTopic := abi.Events["InvalidateNonce"].ID
	// Only a successfully read dense range is subdivided. Transport/member failures
	// remain failures: retrying those could hide an incomplete event history.
	var opening protocolio.ReadResult
	var logs []types.Log
	for {
		reads, err := protocolio.ReadGroup(ctx, p.chain,
			protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).SetUint64(end)},
			protocolio.ReadRequest{Kind: protocolio.ReadLogs, Filter: ethereum.FilterQuery{FromBlock: new(big.Int).SetUint64(st.NextBlock), ToBlock: new(big.Int).SetUint64(end), Addresses: []common.Address{p.reactor}, Topics: [][]common.Hash{{fillTopic}}}},
			protocolio.ReadRequest{Kind: protocolio.ReadLogs, Filter: ethereum.FilterQuery{FromBlock: new(big.Int).SetUint64(st.NextBlock), ToBlock: new(big.Int).SetUint64(end), Addresses: []common.Address{p.reactor}, Topics: [][]common.Hash{{cancelTopic}, {common.BytesToHash(d.Account.Bytes())}}}},
		)
		if err != nil {
			return "", false, err
		}
		opening = reads.At(0)
		if opening.Err != nil || !validHeader(opening.Header, end) {
			return "", false, errors.New("RFQ scan header unavailable")
		}
		fills, cancellations := reads.At(1), reads.At(2)
		if fills.Err != nil || cancellations.Err != nil {
			return "", false, errors.New("RFQ event range failed")
		}
		if len(fills.Logs) <= maxRangeLogs && len(cancellations.Logs) <= maxRangeLogs-len(fills.Logs) {
			logs = append(append([]types.Log(nil), fills.Logs...), cancellations.Logs...)
			break
		}
		if end == st.NextBlock {
			return "", false, errors.New("RFQ single block exceeds event log bound")
		}
		end = st.NextBlock + (end-st.NextBlock)/2
	}
	sort.Slice(logs, func(i, j int) bool {
		a, b := logs[i], logs[j]
		if a.BlockNumber != b.BlockNumber {
			return a.BlockNumber < b.BlockNumber
		}
		if a.TxIndex != b.TxIndex {
			return a.TxIndex < b.TxIndex
		}
		return a.Index < b.Index
	})
	var candidate *types.Log
	outcome := ""
	for i, l := range logs {
		if l.Removed || l.Address != p.reactor || l.BlockNumber < st.NextBlock || l.BlockNumber > end || l.TxHash == (common.Hash{}) || l.BlockHash == (common.Hash{}) || len(l.Topics) == 0 || i > 0 && l.BlockNumber == logs[i-1].BlockNumber && l.Index <= logs[i-1].Index {
			return "", false, errors.New("RFQ invalid event range identity")
		}
		if l.Topics[0] == fillTopic {
			if len(l.Topics) != 1 || len(l.Data) == 0 {
				return "", false, errors.New("RFQ malformed Fill")
			}
			event, err := r.UnpackFillEvent(&l)
			if err != nil {
				return "", false, errors.New("RFQ malformed Fill")
			}
			o := event.Order
			if o.Request.Nonce == nil {
				return "", false, errors.New("RFQ missing event nonce")
			}
			if o.Swapper != d.Account || o.Request.Nonce.Cmp(d.Request.Nonce) != 0 {
				continue
			}
			if candidate != nil {
				return "", false, errors.New("RFQ fill follows consumed nonce")
			}
			if !equalRequest(o.Request, d.Request) {
				return "", false, errors.New("RFQ nonce consumed by another request")
			}
			if len(o.Outputs) != 1 || o.Outputs[0].Amount == nil || o.Outputs[0].Amount.Cmp(d.Request.Outputs[0].Amount) < 0 || o.Outputs[0].Token != d.Request.Outputs[0].Token || o.Outputs[0].Recipient != d.Account {
				return "", false, errors.New("RFQ filled output mismatch")
			}
			outcome = "settled"
		} else if l.Topics[0] == cancelTopic {
			if len(l.Topics) != 2 || len(l.Data) != 32 || !bytes.Equal(l.Topics[1][:12], make([]byte, 12)) {
				return "", false, errors.New("RFQ malformed nonce invalidation")
			}
			event, err := r.UnpackInvalidateNonceEvent(&l)
			if err != nil || event.Nonce == nil {
				return "", false, errors.New("RFQ malformed nonce invalidation")
			}
			if event.Swapper != d.Account || event.Nonce.Cmp(d.Request.Nonce) != 0 {
				continue
			}
			outcome = "cancelled"
		} else {
			return "", false, errors.New("RFQ unexpected event topic")
		}
		if candidate == nil {
			candidate = &l
		}
		if outcome == "settled" {
			break
		}
	}
	if candidate != nil {
		if !used {
			return "", false, errors.New("RFQ nonce evidence contradicts pinned state")
		}
		if err := p.eventReceipt(ctx, *candidate, s); err != nil {
			return "", false, err
		}
	}
	closing, err := p.chain.HeaderByNumber(ctx, new(big.Int).SetUint64(end))
	if err != nil || !validHeader(closing, end) || closing.Hash() != opening.Header.Hash() {
		return "", false, errors.New("RFQ event range reorganized")
	}
	if err := protocolio.Canonical(ctx, p.chain, s); err != nil {
		return "", false, err
	}
	if candidate != nil {
		return outcome, true, nil
	}
	if end == ^uint64(0) {
		return "", false, errors.New("RFQ event cursor overflow")
	}
	st.NextBlock = end + 1
	st.Anchor = closing.Hash()
	return "", end == head, nil
}
func (p *Provider) eventReceipt(ctx context.Context, l types.Log, s provider.Snapshot) error {
	reads, err := protocolio.ReadGroup(ctx, p.chain, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).SetUint64(l.BlockNumber)}, protocolio.ReadRequest{Kind: protocolio.ReadReceipt, Hash: l.TxHash})
	if err != nil {
		return err
	}
	h, rr := reads.At(0), reads.At(1)
	r := rr.Receipt
	if h.Err != nil || !validHeader(h.Header, l.BlockNumber) || h.Header.Hash() != l.BlockHash || rr.Err != nil || r == nil || r.BlockNumber == nil || !r.BlockNumber.IsUint64() || r.BlockNumber.Uint64() != l.BlockNumber || r.BlockHash != l.BlockHash || r.TxHash != l.TxHash || r.Status != types.ReceiptStatusSuccessful || r.TransactionIndex != l.TxIndex || l.BlockNumber > s.BlockNumber.Uint64() {
		return errors.New("RFQ event lacks canonical successful receipt")
	}
	found := 0
	for _, x := range r.Logs {
		if x != nil && x.Index == l.Index {
			if x.Removed || x.Address != l.Address || x.BlockNumber != l.BlockNumber || x.BlockHash != l.BlockHash || x.TxHash != l.TxHash || x.TxIndex != l.TxIndex || !bytes.Equal(x.Data, l.Data) || len(x.Topics) != len(l.Topics) {
				return errors.New("RFQ receipt log identity mismatch")
			}
			for i := range l.Topics {
				if x.Topics[i] != l.Topics[i] {
					return errors.New("RFQ receipt topics mismatch")
				}
			}
			found++
		}
	}
	if found != 1 {
		return errors.New("RFQ event absent or repeated in receipt")
	}
	return nil
}
