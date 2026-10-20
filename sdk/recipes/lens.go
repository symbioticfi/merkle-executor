package recipes

import (
	"context"
	_ "embed"
	"encoding/json"
	"math/big"
	"sort"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

//go:embed lens/bytecode.json
var lensArtifact []byte

var lensArguments = abi.Arguments{{Type: mustType("address", nil)}, {Type: mustType("bytes[]", nil)}, {Type: mustType("bytes[]", nil)}, {Type: mustType("uint256[]", nil)}, {Type: mustType("bool[]", nil)}, {Type: mustType("bytes[]", nil)}, {Type: mustType("bytes[]", nil)}, {Type: mustType("uint256[]", nil)}}
var lensAnchorArgs = abi.Arguments{{Type: mustType("uint256", nil)}}
var lensResults = abi.Arguments{{Type: mustType("uint256[]", nil)}}

// ResolveValues resolves a pure numerical prefix. Hosts with authenticated prior
// effects use ResolveValuesWithRuntime with the already prepared runtime data.
func ResolveValues(ctx context.Context, reader provider.Reader, snapshot Snapshot, recipe Recipe, expressions []Value) (map[uint]*big.Int, error) {
	return ResolveValuesWithRuntime(ctx, reader, snapshot, recipe, expressions, PrefixRuntime{})
}

// ResolveValuesWithRuntime uses one same-executor code override for the union of
// signed-value dependencies, replaying authenticated Manage/Approve/Call effects
// before later reads. The full original execute validates the durable witness.
func ResolveValuesWithRuntime(ctx context.Context, reader provider.Reader, snapshot Snapshot, recipe Recipe, expressions []Value, runtime PrefixRuntime) (map[uint]*big.Int, error) {
	program, err := buildPrefix(recipe, expressions, runtime)
	if err != nil {
		return nil, err
	}
	result := map[uint]*big.Int{}
	if len(program.needed) == 0 {
		return result, nil
	}
	indices := make([]uint, 0, len(program.needed))
	for index := range program.needed {
		indices = append(indices, index)
		if recipe.Steps[index][0] != Read {
			continue
		}
		args, e := readArgs.Unpack(recipe.Steps[index][1:])
		if e != nil {
			return nil, e
		}
		args, e = queryArgs.Unpack(recipe.Queries[args[0].(*big.Int).Uint64()])
		if e != nil {
			return nil, e
		}
		if args[0] == snapshot.Executor {
			return nil, errors.New("unsupported prefix: self-query cannot preserve original executor code")
		}
	}
	sort.Slice(indices, func(i, j int) bool { return indices[i] < indices[j] })
	override, ok := reader.(provider.CodeOverrideReader)
	if !ok {
		return nil, errors.New("recipe-derived amounts require RPC code-override support")
	}
	var artifact struct {
		Runtime hexutil.Bytes `json:"runtime"`
	}
	if err = json.Unmarshal(lensArtifact, &artifact); err != nil || len(artifact.Runtime) == 0 {
		return nil, errors.New("invalid recipe lens artifact")
	}
	packedIndices := make([]*big.Int, len(indices))
	for i, index := range indices {
		packedIndices[i] = new(big.Int).SetUint64(uint64(index))
	}
	values, known, anchors, err := prefixInputs(program, runtime.Inputs)
	if err != nil {
		return nil, err
	}
	args, err := lensArguments.Pack(snapshot.Account, recipe.Queries, recipe.Steps, values, known, anchors, program.runtime, packedIndices)
	if err != nil {
		return nil, errors.New("cannot encode recipe prefix")
	}
	data := append(crypto.Keccak256([]byte("resolve(address,bytes[],bytes[],uint256[],bool[],bytes[],bytes[],uint256[])"))[:4], args...)
	raw, err := override.CallContractWithCode(ctx, ethereum.CallMsg{From: snapshot.Caller, To: &snapshot.Executor, Data: data, Gas: 30_000_000}, new(big.Int).SetUint64(snapshot.BlockNumber), artifact.Runtime)
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

func ResultsWitness(results map[uint]*big.Int) map[uint]string {
	witness := make(map[uint]string, len(results))
	for index, value := range results {
		witness[index] = value.String()
	}
	return witness
}

// VerifyResults validates durable indices against the original immutable steps
// and checks the original executor's returned values, including on recovery.
func VerifyResults(calldata, returned []byte, witness map[uint]string) error {
	if len(witness) == 0 {
		return nil
	}
	method := executorABI.GetABI().Methods["execute"]
	if len(calldata) < 4 || !equalBytes(calldata[:4], method.ID) {
		return errors.New("recipe witness has invalid execution bytes")
	}
	args, err := method.Inputs.Unpack(calldata[4:])
	if err != nil {
		return errors.New("recipe witness cannot decode execution")
	}
	steps, ok := args[1].([][]byte)
	if !ok {
		return errors.New("recipe witness has invalid steps")
	}
	decoded, err := method.Outputs.Unpack(returned)
	if err != nil || len(decoded) != 1 {
		return errors.New("original recipe returned invalid numerical results")
	}
	values, ok := decoded[0].([]*big.Int)
	if !ok || len(values) != len(steps) {
		return errors.New("original recipe numerical result length mismatch")
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

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// AnchoredInputs fills deterministic non-provider inputs. A recipe comparison
// must uniquely equate the input to a read-dependent numerical expression.
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
