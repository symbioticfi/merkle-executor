package liquidlanerfq

import (
	"context"
	"math/big"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

// IndexedReactorLog matches the RFQ indexer's original raw event evidence.
type IndexedReactorLog struct {
	Address          common.Address `json:"address"`
	BlockNumber      string         `json:"blockNumber"`
	BlockHash        common.Hash    `json:"blockHash"`
	TransactionHash  common.Hash    `json:"transactionHash"`
	TransactionIndex uint           `json:"transactionIndex"`
	LogIndex         uint           `json:"logIndex"`
	Data             hexutil.Bytes  `json:"data"`
	Topics           []common.Hash  `json:"topics"`
}

// ReactorEventsResponse includes canonical indexed coverage, even for empty ranges.
type ReactorEventsResponse struct {
	Fills         []IndexedReactorLog `json:"fills"`
	Invalidations []IndexedReactorLog `json:"invalidations"`
	Coverage      struct {
		FromBlock string      `json:"fromBlock"`
		ToBlock   string      `json:"toBlock"`
		BlockHash common.Hash `json:"blockHash"`
	} `json:"coverage"`
	Truncated bool `json:"truncated"`
}

func decimalBlock(s string) (uint64, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || s != strconv.FormatUint(n, 10) {
		return 0, errors.New("RFQ invalid indexed block number")
	}
	return n, nil
}

func (p *Provider) indexedEvents(ctx context.Context, from, to uint64, swapper common.Address) ([]types.Log, *types.Header, bool, error) {
	var result ReactorEventsResponse
	path := "/api/v1/reactor-events/" + p.reactor.Hex() + "?fromBlock=" + strconv.FormatUint(from, 10) + "&toBlock=" + strconv.FormatUint(to, 10) + "&swapper=" + swapper.Hex()
	status, err := p.indexer.JSON(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, nil, false, errors.Errorf("RFQ indexed events unavailable (HTTP %d): %w", status, err)
	}
	first, e1 := decimalBlock(result.Coverage.FromBlock)
	last, e2 := decimalBlock(result.Coverage.ToBlock)
	if e1 != nil || e2 != nil || first > from || last < to || result.Coverage.BlockHash == (common.Hash{}) || len(result.Fills) > maxRangeLogs+1 || len(result.Invalidations) > maxRangeLogs+1-len(result.Fills) {
		return nil, nil, false, errors.New("RFQ invalid indexed event coverage")
	}
	// The indexed cursor and the requested range must agree with this node's chain.
	reads, err := protocolio.ReadGroup(ctx, p.chain, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).SetUint64(to)}, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).SetUint64(last)})
	if err != nil {
		return nil, nil, false, err
	}
	opening, cursor := reads.At(0), reads.At(1)
	if opening.Err != nil || !validHeader(opening.Header, to) || cursor.Err != nil || !validHeader(cursor.Header, last) || cursor.Header.Hash() != result.Coverage.BlockHash {
		return nil, nil, false, errors.New("RFQ indexed event coverage is noncanonical")
	}
	if result.Truncated {
		return nil, opening.Header, true, nil
	}
	if len(result.Fills)+len(result.Invalidations) > maxRangeLogs {
		return nil, nil, false, errors.New("RFQ unmarked truncated event range")
	}
	logs := make([]types.Log, 0, len(result.Fills)+len(result.Invalidations))
	for _, group := range [][]IndexedReactorLog{result.Fills, result.Invalidations} {
		for _, l := range group {
			n, e := decimalBlock(l.BlockNumber)
			if e != nil {
				return nil, nil, false, e
			}
			logs = append(logs, types.Log{Address: l.Address, BlockNumber: n, BlockHash: l.BlockHash, TxHash: l.TransactionHash, TxIndex: l.TransactionIndex, Index: l.LogIndex, Data: l.Data, Topics: l.Topics})
		}
	}
	return logs, opening.Header, false, nil
}
