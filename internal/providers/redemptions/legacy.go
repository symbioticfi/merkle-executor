package redemptions

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// v1 bundles are decoded only for offline import, never prepared or advanced.
type durableRoute struct {
	Config Config          `json:"config"`
	Output provider.Output `json:"output"`
}
type durableRoutes struct {
	Account common.Address `json:"account"`
	Routes  []durableRoute `json:"routes"`
}

func (p *automatic) restoreLegacy(out provider.Output) ([]*Provider, []provider.Output, error) {
	var bundle durableRoutes
	inner, matched, err := derived.Decode(PolicyVersion, out, &bundle)
	if err != nil {
		return nil, nil, err
	}
	if !matched {
		return nil, nil, nil
	}
	if bundle.Account == (common.Address{}) {
		return nil, nil, errors.New("persisted issuer account missing")
	}
	if len(bundle.Routes) == 0 {
		return nil, nil, errors.New("issuer envelope has no routes")
	}
	var runtimes []*Provider
	var outputs []provider.Output
	inputs := map[uint]string{}
	actions := map[string]hexutil.Bytes{}
	targets := map[common.Address]bool{}
	for _, route := range bundle.Routes {
		if err := p.validateKnownRoute(route.Config); err != nil {
			return nil, nil, err
		}
		runtime, err := New(route.Config, p.deps)
		if err != nil {
			return nil, nil, err
		}
		if err = runtime.validatePersisted(route.Output, bundle.Account); err != nil {
			return nil, nil, err
		}
		for _, target := range runtime.ExclusiveTargets() {
			if targets[target] {
				return nil, nil, errors.New("persisted issuer target is repeated")
			}
			targets[target] = true
		}
		for i, v := range route.Output.Inputs {
			if _, ok := inputs[i]; ok {
				return nil, nil, errors.New("persisted issuer input is repeated")
			}
			inputs[i] = v
		}
		for key, data := range route.Output.Actions {
			if _, ok := actions[key]; ok {
				return nil, nil, errors.New("persisted issuer action is repeated")
			}
			actions[key] = data
		}
		if len(route.Output.Actions) != len(runtime.ExclusiveTargets()) || len(route.Output.SkipSteps) != 0 {
			return nil, nil, errors.New("persisted issuer action scope mismatch")
		}
		runtimes = append(runtimes, runtime)
		outputs = append(outputs, route.Output)
	}
	if len(inner.Actions) != len(actions) || len(inner.Inputs) != len(inputs) || len(inner.SkipSteps) != 0 || len(inner.Data) != 0 {
		return nil, nil, errors.New("persisted issuer aggregate shape mismatch")
	}
	for key, data := range actions {
		if !bytes.Equal(inner.Actions[key], data) {
			return nil, nil, errors.New("persisted issuer aggregate action mismatch")
		}
	}
	for i, v := range inputs {
		if inner.Inputs[i] != v {
			return nil, nil, errors.New("persisted issuer aggregate input mismatch")
		}
	}
	return runtimes, outputs, nil
}

// ImportLegacy validates the v1 aggregate before splitting its stable route records.
func ImportLegacy(deps registry.Deps, snapshot chainio.Snapshot, out provider.Output, progress provider.Progress) ([]provider.Record, error) {
	raw, err := NewAutomatic(deps)
	if err != nil {
		return nil, err
	}
	p := raw.(*automatic)
	runtimes, outputs, err := p.restoreLegacy(out)
	if err != nil {
		return nil, err
	}
	var bundle durableRoutes
	_, _, err = derived.Decode(PolicyVersion, out, &bundle)
	if err != nil {
		return nil, err
	}
	if len(runtimes) > 0 && bundle.Account != snapshot.Account {
		return nil, errors.New("legacy account differs from snapshot")
	}
	var records []provider.Record
	for i, runtime := range runtimes {

		var cfg Config
		for _, route := range bundle.Routes {
			if route.Config.Action == runtime.cfg.Action {
				cfg = route.Config
				break
			}
		}
		envelope := routeEnvelope{Config: cfg, Account: snapshot.Account}

		output, err := provider.EncodeOperation(p.Policy(), envelope, outputs[i])
		if err != nil {
			return nil, err
		}
		req := provider.OutputRequirements(output, runtime.ExclusiveTargets())
		record := provider.Record{Integration: p.ID(), Policy: p.Policy(), ID: provider.OperationID(req.Actions), Requirements: req, Output: output, Progress: progress.Clone()}
		if _, err = p.Restore(record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}
