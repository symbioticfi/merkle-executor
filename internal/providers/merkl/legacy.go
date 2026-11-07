package merkl

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// ImportLegacy converts only validated v1 envelopes without protocol I/O.
func ImportLegacy(deps registry.Deps, snapshot chainio.Snapshot, out provider.Output, progress provider.Progress) ([]provider.Record, error) {
	p, err := NewAutomatic(deps)
	if err != nil {
		return nil, err
	}
	var cfg Config
	inner, matched, err := derived.Decode(automaticPolicy, out, &cfg)
	if err != nil || !matched {
		return nil, err
	}
	output, err := provider.EncodeOperation(p.Policy(), cfg, inner)
	if err != nil {
		return nil, err
	}
	req := provider.OutputRequirements(inner, nil)
	record := provider.Record{Integration: p.ID(), Policy: p.Policy(), ID: provider.OperationID(req.Actions), Requirements: req, Output: output, Progress: progress.Clone()}
	if _, err = p.Restore(record); err != nil {
		return nil, err
	}

	args, err := binding.NewMerklDistributor().GetABI().Methods["claim"].Inputs.Unpack(inner.Actions[cfg.Action][4:])
	if err != nil {
		return nil, err
	}
	for _, account := range args[0].([]common.Address) {
		if account != snapshot.Account {
			return nil, errors.New("legacy Merkl account differs from snapshot")
		}
	}
	return []provider.Record{record}, nil
}
