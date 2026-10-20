package recipes

import (
	"context"
	"math/big"
	"sort"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

var probeArgs = abi.Arguments{{Type: mustType("bytes", nil)}, {Type: mustType("bytes", nil)}}
var probeResults = abi.Arguments{{Type: mustType("uint256[]", nil)}, {Type: mustType("bytes", nil)}}

// ProbePrefix replays an authenticated prefix at the original executor and probes
// its resulting state with host-owned eth_call-only code. Numerical witnesses must
// be verified against the full original execute before sending.
func ProbePrefix(ctx context.Context, reader provider.Reader, snapshot Snapshot, recipe Recipe, before uint, runtime PrefixRuntime, code, query []byte) ([]byte, map[uint]*big.Int, error) {
	prefix, expressions, err := probePrefix(recipe, before)
	if err != nil {
		return nil, nil, err
	}
	program, err := buildPrefix(prefix, expressions, runtime)
	if err != nil {
		return nil, nil, err
	}
	if err = validateProbeActions(prefix, runtime); err != nil {
		return nil, nil, err
	}
	override, ok := reader.(provider.CodeOverrideReader)
	if !ok {
		return nil, nil, errors.New("recipe probe requires RPC code-override support")
	}
	indices := make([]uint, 0, len(program.needed))
	for index := range program.needed {
		indices = append(indices, index)
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	packed := make([]*big.Int, len(indices))
	for i, index := range indices {
		packed[i] = new(big.Int).SetUint64(uint64(index))
		if prefix.Steps[index][0] == Read {
			q, _ := readArgs.Unpack(prefix.Steps[index][1:])
			decoded, _ := queryArgs.Unpack(prefix.Queries[q[0].(*big.Int).Uint64()])
			if decoded[0] == snapshot.Executor {
				return nil, nil, errors.New("unsupported prefix: self-query cannot preserve original executor code")
			}
		}
	}
	values, known, anchors, err := prefixInputs(program, runtime.Inputs)
	if err != nil {
		return nil, nil, err
	}
	args, err := lensArguments.Pack(snapshot.Account, prefix.Queries, prefix.Steps, values, known, anchors, program.runtime, packed)
	if err != nil {
		return nil, nil, errors.New("cannot encode recipe probe prefix")
	}
	resolve := append(crypto.Keccak256([]byte("resolve(address,bytes[],bytes[],uint256[],bool[],bytes[],bytes[],uint256[])"))[:4], args...)
	args, err = probeArgs.Pack(resolve, query)
	if err != nil {
		return nil, nil, err
	}
	data := append(crypto.Keccak256([]byte("probe(bytes,bytes)"))[:4], args...)
	raw, err := override.CallContractWithCode(ctx, ethereum.CallMsg{From: snapshot.Caller, To: &snapshot.Executor, Data: data, Gas: 30_000_000}, new(big.Int).SetUint64(snapshot.BlockNumber), code)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, nil, errors.New("recipe prefix probe simulation failed")
	}
	decoded, err := probeResults.Unpack(raw)
	if err != nil || len(decoded) != 2 {
		return nil, nil, errors.New("invalid recipe probe result")
	}
	returned := decoded[0].([]*big.Int)
	if len(returned) != len(prefix.Steps) {
		return nil, nil, errors.New("recipe probe result length mismatch")
	}
	witness := map[uint]*big.Int{}
	for _, index := range indices {
		if index < before {
			witness[index] = new(big.Int).Set(returned[index])
		}
	}
	return decoded[1].([]byte), witness, nil
}

// ValidateProbePrefix rejects unresolved runtime dependencies before network I/O.
func ValidateProbePrefix(recipe Recipe, before uint, runtime PrefixRuntime) error {
	prefix, expressions, err := probePrefix(recipe, before)
	if err != nil {
		return err
	}
	if err = ValidatePrefix(prefix, expressions, runtime); err != nil {
		return err
	}
	return validateProbeActions(prefix, runtime)
}
func probePrefix(recipe Recipe, before uint) (Recipe, []Value, error) {
	if before > uint(len(recipe.Steps)) {
		return Recipe{}, nil, errors.New("recipe probe prefix exceeds program")
	}
	prefix := CloneRecipe(recipe)
	prefix.Steps = prefix.Steps[:before]
	// A trailing constant forces effects at the end of the prefix to execute.
	zero, _ := binaryArgs.Pack(new(big.Int), new(big.Int))
	prefix.Steps = append(prefix.Steps, append([]byte{Add}, zero...))
	expressions := []Value{}
	for index, step := range prefix.Steps {
		if len(step) > 0 && step[0] <= MulDiv {
			expressions = append(expressions, Value{Source: Result, Value: big.NewInt(int64(index))})
		}
	}
	return prefix, expressions, nil
}

func validateProbeActions(prefix Recipe, runtime PrefixRuntime) error {
	if runtime.Tree == nil {
		return nil
	}
	owned := map[string]bool{}
	for key := range runtime.Actions {
		owned[key] = true
	}
	return ValidateMutableActions(prefix, runtime.Tree, owned)
}
