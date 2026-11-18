package recipes

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

var probeArgs = Arguments("bytes", "bytes")
var probeResults = Arguments("uint256[]", "bytes")

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
	indices, resolve, err := lensCall(snapshot, prefix, program, runtime.Inputs)
	if err != nil {
		return nil, nil, err
	}
	args, err := probeArgs.Pack(resolve, query)
	if err != nil {
		return nil, nil, err
	}
	data := append(crypto.Keccak256([]byte("probe(bytes,bytes)"))[:4], args...)
	raw, err := callPrefix(ctx, reader, snapshot, data, code)
	if err != nil {
		return nil, nil, errors.Errorf("recipe prefix probe simulation failed: %w", err)
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
	return ValidatePrefix(prefix, expressions, runtime)
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
