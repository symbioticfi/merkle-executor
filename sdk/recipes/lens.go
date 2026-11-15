package recipes

import (
	"context"
	_ "embed"
	"maps"
	"math/big"
	"slices"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

//go:embed lens/bytecode.json
var lensArtifact []byte
var lensRuntime = artifactRuntime(lensArtifact)
var resolveSelector = crypto.Keccak256([]byte("resolve(address,bytes[],bytes[],uint256[],bool[],bytes[],bytes[],uint256[])"))[:4]

var lensArguments = Arguments("address", "bytes[]", "bytes[]", "uint256[]", "bool[]", "bytes[]", "bytes[]", "uint256[]")
var lensAnchorArgs = Arguments("uint256")
var lensResults = Arguments("uint256[]")

// ResolveValues resolves a pure numerical prefix. Hosts with authenticated prior
// effects use ResolveValuesWithRuntime with the already prepared runtime data.
func ResolveValues(ctx context.Context, reader provider.Reader, snapshot Snapshot, recipe Recipe, expressions []Value) (map[uint]*big.Int, error) {
	return ResolveValuesWithRuntime(ctx, reader, snapshot, recipe, expressions, PrefixRuntime{})
}

// ResolveValuesWithRuntime uses one same-executor code override for the union of
// signed-value dependencies, replaying authenticated Manage/Approve/Call effects
// before later reads. The full original execute validates the prepared witness.
func ResolveValuesWithRuntime(ctx context.Context, reader provider.Reader, snapshot Snapshot, recipe Recipe, expressions []Value, runtime PrefixRuntime) (map[uint]*big.Int, error) {
	program, err := buildPrefix(recipe, expressions, runtime)
	if err != nil {
		return nil, err
	}
	result := map[uint]*big.Int{}
	if len(program.needed) == 0 {
		return result, nil
	}
	indices, data, err := lensCall(snapshot, recipe, program, runtime.Inputs)
	if err != nil {
		return nil, err
	}
	override, ok := reader.(provider.CodeOverrideReader)
	if !ok {
		return nil, errors.New("recipe-derived amounts require RPC code-override support")
	}
	raw, err := override.CallContractWithCode(ctx, ethereum.CallMsg{From: snapshot.Caller, To: &snapshot.Executor, Data: data, Gas: 30_000_000}, new(big.Int).SetUint64(snapshot.BlockNumber), lensRuntime)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("recipe prefix simulation failed")
	}
	decoded, err := lensResults.Unpack(raw)
	if err != nil || len(decoded) != 1 {
		return nil, errors.New("invalid recipe prefix result")
	}
	returned, ok := decoded[0].([]*big.Int)
	if !ok || len(returned) != len(recipe.Steps) {
		return nil, errors.New("recipe prefix result length mismatch")
	}
	for _, index := range indices {
		result[index] = new(big.Int).Set(returned[index])
	}
	return result, nil
}

// lensCall encodes the shared prefix resolver for every needed index, in order.
func lensCall(snapshot Snapshot, recipe Recipe, program prefixProgram, inputs map[uint]string) ([]uint, []byte, error) {
	indices := slices.Sorted(maps.Keys(program.needed))
	packed := make([]*big.Int, len(indices))
	for i, index := range indices {
		packed[i] = new(big.Int).SetUint64(uint64(index))
		if recipe.Steps[index][0] != Read {
			continue
		}
		args, err := readArgs.Unpack(recipe.Steps[index][1:])
		if err != nil {
			return nil, nil, err
		}
		args, err = queryArgs.Unpack(recipe.Queries[args[0].(*big.Int).Uint64()])
		if err != nil {
			return nil, nil, err
		}
		if args[0] == snapshot.Executor {
			return nil, nil, errors.New("unsupported prefix: self-query cannot preserve original executor code")
		}
	}
	values, known, anchors, err := prefixInputs(program, inputs)
	if err != nil {
		return nil, nil, err
	}
	args, err := lensArguments.Pack(snapshot.Account, recipe.Queries, recipe.Steps, values, known, anchors, program.runtime, packed)
	if err != nil {
		return nil, nil, errors.New("cannot encode recipe prefix")
	}
	return indices, append(slices.Clone(resolveSelector), args...), nil
}

func ResultsWitness(results map[uint]*big.Int) map[uint]string {
	witness := make(map[uint]string, len(results))
	for index, value := range results {
		witness[index] = value.String()
	}
	return witness
}

// VerifyResults validates prepared indices against the original immutable steps
// and checks the original executor's returned values, including on recovery.
func VerifyResults(calldata, returned []byte, witness map[uint]string) error {
	if len(witness) == 0 {
		return nil
	}
	calls, results, err := executionResults(calldata, returned)
	if err != nil {
		return err
	}
	for i, call := range calls {
		decoded, err := decodeExecution(call)
		if err != nil {
			return err
		}
		if err = verifyWitness(decoded.recipe.Steps, results[i], witness); err != nil {
			return err
		}
	}
	return nil
}

func verifyWitness(steps [][]byte, returned []byte, witness map[uint]string) error {
	values, err := stepResults(returned, len(steps))
	if err != nil {
		return err
	}
	for index, text := range witness {
		if index >= uint(len(steps)) || len(steps[index]) == 0 || steps[index][0] > MulDiv {
			return errors.New("recipe witness index is not a numerical step")
		}
		expected, err := parse.Uint256(text)
		if err != nil || expected.String() != text {
			return errors.New("recipe witness value is not canonical uint256")
		}
		if values[index].Cmp(expected) != 0 {
			return errors.New("original recipe result differs from signed recipe-derived value")
		}
	}
	return nil
}

// AnchoredInputs fills deterministic non-provider inputs. A recipe comparison
// must uniquely equate the input to a deterministic numerical expression.
func AnchoredInputs(recipe Recipe, owned map[uint]bool) (map[uint]EconomicValue, error) {
	inspection, err := Inspect(recipe)
	if err != nil {
		return nil, err
	}
	anchors := map[uint]EconomicValue{}
	for _, index := range inspection.Inputs {
		if owned[index] {
			continue
		}
		v := Value{Source: Input, Value: new(big.Int).SetUint64(uint64(index))}
		anchor, err := Budget(recipe, v, uint(len(recipe.Steps)))
		if err != nil {
			return nil, errors.Errorf("input %d has no deterministic authorized source: %w", index, err)
		}
		anchors[index] = anchor
	}
	return anchors, nil
}
