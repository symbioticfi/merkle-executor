package merkl

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"reflect"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

const RewardAPI = "https://api.merkl.xyz"
const automaticPolicy = "merkl/authenticated-claim-v1"

type Automatic struct{ deps registry.Deps }
type recipeClaim struct {
	action       recipes.ManagedAction
	cfg          Config
	requirements provider.Requirements
}

func NewAutomatic(deps registry.Deps) (provider.Integration, error) {
	if deps.Chain == nil {
		return nil, errors.New("automatic Merkl requires chain reader")
	}
	return &Automatic{deps: deps}, nil
}
func rewardHTTP() protocolio.HTTPConfig {
	return protocolio.HTTPConfig{URL: RewardAPI, MaxResponseBytes: 8 << 20}
}

func (p *Automatic) derive(s chainio.Snapshot, r recipes.Recipe, t *recipes.Tree) (*recipeClaim, error) {
	actions, err := recipes.ManagedActions(r, t)
	if err != nil {
		return nil, err
	}
	method := binding.NewMerklDistributor().GetABI().Methods["claim"]
	var result *recipeClaim
	for _, action := range actions {
		data := action.Leaf.Data
		if len(data) < 4 || !bytes.Equal(data[:4], method.ID) {
			continue
		}
		if result != nil {
			return nil, errors.New("multiple Merkl claim actions require separate executors")
		}
		args, err := method.Inputs.Unpack(data[4:])
		if err != nil {
			return nil, errors.New("invalid Merkl claim template")
		}
		users, tokens, amounts, proofs := args[0].([]common.Address), args[1].([]common.Address), args[2].([]*big.Int), args[3].([][][32]byte)
		if len(tokens) < 1 || len(tokens) > 32 || len(users) != len(tokens) || len(amounts) != len(tokens) || len(proofs) != len(tokens) {
			return nil, errors.New("Merkl claim arrays must have the same bounded length")
		}
		expected, err := binding.NewMerklDistributor().TryPackClaim(users, tokens, amounts, proofs)
		if err != nil || !bytes.Equal(expected, data) {
			return nil, errors.New("noncanonical Merkl claim array layout is unsupported")
		}
		cfg := Config{HTTP: rewardHTTP(), Multicall3: protocolio.StandardMulticall3, Distributor: action.Leaf.Target.Hex(), Action: action.Key, CalldataBytes: len(data), MinClaimable: "1"}
		allowed := make(map[int]bool)
		amountOffsets := make(map[int]bool)
		array := 4 + int(new(big.Int).SetBytes(data[68:100]).Uint64())
		for i, token := range tokens {
			if users[i] != s.Account || token == (common.Address{}) {
				return nil, errors.New("Merkl claim must pin authenticated user and nonzero token")
			}
			cfg.Tokens = append(cfg.Tokens, token.Hex())
			cfg.ProofLengths = append(cfg.ProofLengths, len(proofs[i]))
			at := array + 32 + 32*i
			amountOffsets[at] = true
			for n := at; n < at+32; n++ {
				allowed[n] = true
			}
		}
		proofArray := 4 + int(new(big.Int).SetBytes(data[100:132]).Uint64())
		for i, proof := range proofs {
			relative := new(big.Int).SetBytes(data[proofArray+32+32*i : proofArray+64+32*i]).Uint64()
			at := proofArray + 32 + int(relative) + 32
			for n := at; n < at+32*len(proof); n++ {
				allowed[n] = true
			}
		}
		for i := range data {
			if allowed[i] {
				continue
			}
			if err := action.Pinned(i, 1); err != nil {
				return nil, errors.New("Merkl claim user, tokens, selector and proof layout must be pinned")
			}
		}
		requirements := provider.Requirements{Actions: []string{action.Key}}
		for _, b := range action.Bindings {
			if !amountOffsets[int(b.Offset)] {
				return nil, errors.New("Merkl supports numerical bindings only for cumulative amounts")
			}
			switch b.Value.Source {
			case recipes.Input:
				requirements.Inputs = append(requirements.Inputs, uint(b.Value.Value.Uint64()))
			case recipes.Result:
				if err := recipes.ReadDependent(r, b.Value); err != nil {
					return nil, err
				}
				requirements.Values = append(requirements.Values, b.Value)
			case recipes.Literal:
			default:
				return nil, errors.New("invalid Merkl amount source")
			}
		}
		// Cumulative inputs can be sparse; BindOutput fills them from the exact
		// root-verified claim bytes rather than guessing claimable delta mappings.
		result = &recipeClaim{action: action, cfg: cfg, requirements: requirements}
	}
	return result, nil
}

type automaticOperation struct {
	snapshot     chainio.Snapshot
	integration  *Automatic
	compiled     *recipeClaim
	requirements provider.Requirements
	saved        *provider.Record
}

func (*Automatic) ID() string     { return "merkl" }
func (*Automatic) Policy() string { return automaticPolicy + "/operation-v2" }
func (p *Automatic) Compile(program provider.Program) ([]provider.Operation, error) {
	program = program.Clone()
	compiled, err := p.derive(program.Snapshot, program.Recipe, program.Tree)
	if err != nil || compiled == nil {
		return nil, err
	}
	return []provider.Operation{&automaticOperation{integration: p, snapshot: program.Snapshot, compiled: compiled, requirements: compiled.requirements}}, nil
}
func (o *automaticOperation) ID() string                          { return provider.OperationID(o.requirements.Actions) }
func (o *automaticOperation) Requirements() provider.Requirements { return o.requirements.Clone() }
func (o *automaticOperation) Prepare(ctx context.Context, preparation provider.Preparation) (provider.Output, error) {
	if o.compiled == nil {
		return provider.Output{}, errors.New("restored operation cannot prepare")
	}
	if err := ctx.Err(); err != nil {
		return provider.Output{}, err
	}
	if err := preparation.ValidateSnapshot(o.snapshot); err != nil {
		return provider.Output{}, err
	}
	if preparation.Reader == nil {
		return provider.Output{}, errors.New("attempt chain reader required")
	}
	claim := *o.compiled
	s, results := preparation.Program.Snapshot, preparation.Results
	deps := o.integration.deps
	deps.Chain = preparation.Reader

	runtime, err := New(claim.cfg, deps)
	if err != nil {
		return provider.Output{}, err
	}
	out, err := runtime.Prepare(ctx, s)
	if err != nil {
		return provider.Output{}, err
	}
	if err = derived.BindOutput(derived.SignTemplate{Action: claim.action}, out.Actions[claim.cfg.Action], results, &out); err != nil {
		return provider.Output{}, err
	}
	return provider.EncodeOperation(o.integration.Policy(), claim.cfg, out)
}

func (p *Automatic) Restore(record provider.Record) (provider.Operation, error) {
	var cfg Config
	inner, err := provider.DecodeOperation(p.Policy(), record.Output, &cfg)
	if err != nil {
		return nil, err
	}

	if !reflect.DeepEqual(cfg.HTTP, rewardHTTP()) || cfg.Multicall3 != protocolio.StandardMulticall3 || cfg.MinClaimable != "1" || cfg.SkipIfEmpty || cfg.DedicatedClaimGroup || len(cfg.AmountInputs) != 0 || len(cfg.ClaimableInputs) != 0 {
		return nil, errors.New("persisted Merkl automatic constants differ")
	}
	runtime, err := New(cfg, p.deps)
	if err != nil {
		return nil, err
	}

	_ = runtime
	data := inner.Actions[cfg.Action]
	method := binding.NewMerklDistributor().GetABI().Methods["claim"]
	if len(data) != cfg.CalldataBytes || len(data) < 4 || !bytes.Equal(data[:4], method.ID) {
		return nil, errors.New("persisted Merkl claim selector or size differs")
	}
	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return nil, err
	}
	users, tokens, amounts, proofs := args[0].([]common.Address), args[1].([]common.Address), args[2].([]*big.Int), args[3].([][][32]byte)
	packed, err := binding.NewMerklDistributor().TryPackClaim(users, tokens, amounts, proofs)
	if err != nil || !bytes.Equal(packed, data) || len(tokens) != len(cfg.Tokens) || len(users) != len(tokens) || len(amounts) != len(tokens) || len(proofs) != len(tokens) {
		return nil, errors.New("persisted Merkl claim layout differs")
	}
	var evidence struct {
		Root   string `json:"root"`
		Tokens string `json:"tokens"`
	}
	if err := derived.StrictJSON(inner.Data, &evidence); err != nil {
		return nil, err
	}
	root := common.HexToHash(evidence.Root)
	if root == (common.Hash{}) || evidence.Tokens != strconv.Itoa(len(tokens)) {
		return nil, errors.New("persisted Merkl root invalid")
	}
	for i, token := range tokens {
		if users[i] == (common.Address{}) || users[i] != users[0] || token != common.HexToAddress(cfg.Tokens[i]) || len(proofs[i]) != cfg.ProofLengths[i] || !VerifyProof(users[i], token, amounts[i], proofs[i], root) {
			return nil, errors.New("persisted Merkl proof or identity differs")
		}
	}

	available := map[string]int{}
	for _, amount := range amounts {
		available[amount.String()]++
	}
	for _, value := range inner.Inputs {
		if available[value] == 0 {
			return nil, errors.New("persisted Merkl input differs from cumulative amount")
		}
		available[value]--
	}

	expected := provider.OutputRequirements(inner, nil)
	if len(expected.Actions) != 1 || expected.Actions[0] != cfg.Action {
		return nil, errors.New("persisted operation action scope differs")
	}
	if err = provider.ValidateRecord(record, p.ID(), p.Policy(), expected); err != nil {
		return nil, err
	}
	savedRecord := record.Clone()
	return &automaticOperation{integration: p, requirements: record.Requirements.Clone(), saved: &savedRecord}, nil
}
func (o *automaticOperation) Advance(ctx context.Context, s chainio.Snapshot, out provider.Output, state json.RawMessage) (provider.Progress, error) {
	if _, err := o.integration.Restore(provider.Record{Integration: o.integration.ID(), Policy: o.integration.Policy(), ID: o.ID(), Requirements: o.Requirements(), Output: out}); err != nil {
		return provider.Progress{}, err
	}

	if o.saved != nil && !provider.EqualOutput(o.saved.Output, out) {
		return provider.Progress{}, errors.New("operation output differs from restored record")
	}
	var cfg Config
	inner, err := provider.DecodeOperation(o.integration.Policy(), out, &cfg)
	if err != nil {
		return provider.Progress{}, err
	}

	if !reflect.DeepEqual(cfg.HTTP, rewardHTTP()) || cfg.Multicall3 != protocolio.StandardMulticall3 || cfg.MinClaimable != "1" || cfg.SkipIfEmpty || cfg.DedicatedClaimGroup || len(cfg.AmountInputs) != 0 || len(cfg.ClaimableInputs) != 0 {
		return provider.Progress{}, errors.New("persisted Merkl automatic constants differ")
	}
	runtime, err := New(cfg, o.integration.deps)
	if err != nil {
		return provider.Progress{}, err
	}
	if len(inner.Actions) != 1 || len(inner.SkipSteps) != 0 {
		return provider.Progress{}, errors.New("persisted Merkl claim shape differs")
	}
	return runtime.Advance(ctx, s, inner, state)
}
