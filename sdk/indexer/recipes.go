// Package indexer reads original recipe publications from the RFQ indexer.
package indexer

import (
	"context"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type Client struct{ http *protocolio.Client }

// New uses the service base URL, without /api/v1, and bounded credential-free HTTP.
func New(baseURL string, transport *http.Client) (*Client, error) {
	c, err := protocolio.NewHTTP(protocolio.HTTPConfig{URL: baseURL}, transport, nil)
	if err != nil {
		return nil, err
	}
	return &Client{http: c}, nil
}

func (c *Client) RecipePublication(ctx context.Context, executor common.Address, hash common.Hash, block uint64) (recipes.Publication, error) {
	var body struct {
		Executor        common.Address  `json:"executor"`
		RecipeHash      common.Hash     `json:"recipeHash"`
		BlockNumber     string          `json:"blockNumber"`
		BlockHash       common.Hash     `json:"blockHash"`
		TransactionHash common.Hash     `json:"transactionHash"`
		LogIndex        uint            `json:"logIndex"`
		Queries         []hexutil.Bytes `json:"queries"`
		Steps           []hexutil.Bytes `json:"steps"`
	}
	status, err := c.http.JSON(ctx, http.MethodGet, "/api/v1/recipes/"+executor.Hex()+"?hash="+hash.Hex()+"&block="+strconv.FormatUint(block, 10), nil, &body)
	if err != nil {
		return recipes.Publication{}, errors.Errorf("indexed recipe unavailable (HTTP %d): %w", status, err)
	}
	n, err := strconv.ParseUint(body.BlockNumber, 10, 64)
	if err != nil || body.BlockNumber != strconv.FormatUint(n, 10) || len(body.Queries) > 4096 || len(body.Steps) > 4096 {
		return recipes.Publication{}, errors.New("invalid indexed recipe response")
	}
	p := recipes.Publication{Executor: body.Executor, RecipeHash: body.RecipeHash, BlockNumber: n, BlockHash: body.BlockHash, TransactionHash: body.TransactionHash, LogIndex: body.LogIndex}
	for _, q := range body.Queries {
		p.Recipe.Queries = append(p.Recipe.Queries, []byte(q))
	}
	for _, s := range body.Steps {
		p.Recipe.Steps = append(p.Recipe.Steps, []byte(s))
	}
	return p, nil
}
