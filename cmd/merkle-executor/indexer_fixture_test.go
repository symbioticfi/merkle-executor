package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	reactor "github.com/symbioticfi/merkle-executor/sdk/bindings/liquidlanereactor"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	rfq "github.com/symbioticfi/merkle-executor/sdk/providers/liquidlanerfq"
)

var seededEvents = struct {
	sync.Mutex
	logs map[*ethclient.Client][]types.Log
}{logs: map[*ethclient.Client][]types.Log{}}

func rememberSeededEvents(client *ethclient.Client, receipt *types.Receipt) {
	seededEvents.Lock()
	defer seededEvents.Unlock()
	for _, l := range receipt.Logs {
		if l != nil {
			seededEvents.logs[client] = append(seededEvents.logs[client], *l)
		}
	}
}

// The fixture ingests seed receipts as an indexer would; HTTP handlers never scan logs.
func fixtureIndexer(t *testing.T, client *ethclient.Client) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seededEvents.Lock()
		logs := append([]types.Log(nil), seededEvents.logs[client]...)
		seededEvents.Unlock()
		if strings.HasPrefix(r.URL.Path, "/api/v1/recipes/") {
			address := common.HexToAddress(strings.TrimPrefix(r.URL.Path, "/api/v1/recipes/"))
			hash := common.HexToHash(r.URL.Query().Get("hash"))
			block, _ := strconv.ParseUint(r.URL.Query().Get("block"), 10, 64)
			binding := executor.NewRecipeExecutor()
			for i := len(logs) - 1; i >= 0; i-- {
				l := logs[i]
				if l.Address != address || l.BlockNumber != block || len(l.Topics) != 2 || l.Topics[0] != binding.GetABI().Events["SetRecipe"].ID || l.Topics[1] != hash {
					continue
				}
				event, err := binding.UnpackSetRecipeEvent(&l)
				if err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				queries, steps := []hexutil.Bytes{}, []hexutil.Bytes{}
				for _, q := range event.Queries {
					queries = append(queries, q)
				}
				for _, s := range event.Steps {
					steps = append(steps, s)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"executor": address, "recipeHash": hash, "blockNumber": strconv.FormatUint(block, 10), "blockHash": l.BlockHash, "transactionHash": l.TxHash, "logIndex": l.Index, "queries": queries, "steps": steps})
				return
			}
			w.WriteHeader(404)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/reactor-events/") {
			address := common.HexToAddress(strings.TrimPrefix(r.URL.Path, "/api/v1/reactor-events/"))
			swapper := common.HexToAddress(r.URL.Query().Get("swapper"))
			from, _ := strconv.ParseUint(r.URL.Query().Get("fromBlock"), 10, 64)
			to, _ := strconv.ParseUint(r.URL.Query().Get("toBlock"), 10, 64)
			header, err := client.HeaderByNumber(r.Context(), nil)
			if err != nil {
				w.WriteHeader(503)
				return
			}
			result := rfq.ReactorEventsResponse{}
			result.Coverage.FromBlock = "0"
			result.Coverage.ToBlock = header.Number.String()
			result.Coverage.BlockHash = header.Hash()
			binding := reactor.NewLiquidLaneReactor()
			for _, l := range logs {
				if l.Address != address || l.BlockNumber < from || l.BlockNumber > to || len(l.Topics) == 0 {
					continue
				}
				event := rfq.IndexedReactorLog{Address: l.Address, BlockNumber: strconv.FormatUint(l.BlockNumber, 10), BlockHash: l.BlockHash, TransactionHash: l.TxHash, TransactionIndex: l.TxIndex, LogIndex: l.Index, Data: l.Data, Topics: l.Topics}
				switch l.Topics[0] {
				case binding.GetABI().Events["Fill"].ID:
					e, err := binding.UnpackFillEvent(&l)
					if err == nil && e.Order.Swapper == swapper {
						result.Fills = append(result.Fills, event)
					}
				case binding.GetABI().Events["InvalidateNonce"].ID:
					if len(l.Topics) == 2 && l.Topics[1] == common.BytesToHash(swapper.Bytes()) {
						result.Invalidations = append(result.Invalidations, event)
					}
				}
			}
			_ = json.NewEncoder(w).Encode(result)
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { seededEvents.Lock(); delete(seededEvents.logs, client); seededEvents.Unlock() })
	return server.URL
}
