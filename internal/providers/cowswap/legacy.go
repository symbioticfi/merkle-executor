package cowswap

import (
	"github.com/go-errors/errors"
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

	var saved prepared
	if err := derived.StrictJSON(inner.Data, &saved); err != nil {
		return nil, err
	}
	if snapshot.ChainID == nil || saved.ChainID != snapshot.ChainID.String() || saved.Account != snapshot.Account {
		return nil, errors.New("legacy protocol chain/account differs from snapshot")
	}
	return []provider.Record{record}, nil
}
