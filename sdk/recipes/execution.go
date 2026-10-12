package recipes

import (
	"context"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

// PrepareExecution encloses discovery, preparation and execution validation in
// one canonical guard. build may read provider data and assemble authorized bytes,
// but must not sign, persist or mutate external state. Its snapshot is provisional:
// callers must not use the prepared result unless this operation succeeds.
func PrepareExecution(ctx context.Context, reader Reader, executor, account, caller common.Address, build func(Snapshot, Recipe) ([]byte, []common.Hash, error)) error {
	if build == nil {
		return errors.New("expected account, caller and preparation callback required")
	}
	return PrepareExecutionWithOptions(ctx, reader, executor, account, caller, PreparationOptions{}, func(s Snapshot, r Recipe, _ protocolio.ReadAccessor) ([]byte, []common.Hash, error) {
		return build(s, r)
	})
}

// PreparationOptions enables attempt-local staged reads without changing the
// independently guarded discovery APIs. Stage must perform only pure local work.
type PreparationOptions struct {
	ChainID *big.Int
	Cache   *RecipeCache
	Stage   func(protocolio.PreparationPin) []protocolio.ReadRequest
}

// PrepareExecutionWithOptions retains every execution guard while allowing
// independent provider reads beside initial metadata. The build callback only
// receives staged results after recipe evidence and configured identity checks.
func PrepareExecutionWithOptions(ctx context.Context, reader Reader, executor, account, caller common.Address, options PreparationOptions, build func(Snapshot, Recipe, protocolio.ReadAccessor) ([]byte, []common.Hash, error)) error {
	if account == (common.Address{}) || caller == (common.Address{}) || build == nil {
		return errors.New("expected account, caller and preparation callback required")
	}
	snapshot, recipe, staged, pending, err := discoverPreparation(ctx, reader, executor, account, caller, options)
	if err != nil {
		return errors.Errorf("discover recipe: %w", err)
	}
	if snapshot.Caller != caller {
		return errors.New("configured caller/account mismatch")
	}
	buildRecipe := recipe
	if pending != nil {
		buildRecipe = cloneRecipe(recipe)
	}
	calldata, leaves, err := build(snapshot, buildRecipe, staged)
	if err != nil {
		return err
	}
	current, err := openExecution(ctx, reader, snapshot, caller)
	if err != nil {
		return err
	}
	unique, roles := roleRequests(snapshot, leaves)
	requests := snapshotRequests(executor, account, current.Number)
	requests = append(requests, roles...)
	requests = append(requests, executionRead(snapshot, caller, calldata, new(big.Int).SetUint64(snapshot.BlockNumber)))
	reads, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return err
	}
	// Preserve the planner's permission -> simulation -> current metadata priority.
	// The opening headers necessarily run first to pin independent current reads.
	if err = verifyRoleResults(unique, func(i int) protocolio.ReadResult { return reads.At(5 + i) }); err != nil {
		return err
	}
	if reads.At(len(requests)-1).Err != nil {
		return errors.New("full recipe simulation failed")
	}
	if err = validateCurrent(snapshot, current, reads, caller); err != nil {
		return err
	}
	if err = closeExecution(ctx, reader, snapshot, current); err != nil {
		return err
	}
	options.Cache.put(pending)
	return nil
}

// ValidateExecution rereads all current metadata even when the head hash matches
// a persisted snapshot. The exact caller-bound calldata executes at nil/latest,
// isolated from metadata getters; both captured block pins are checked afterward.
func ValidateExecution(ctx context.Context, reader Reader, snapshot Snapshot, caller common.Address, calldata []byte) error {
	current, err := openExecution(ctx, reader, snapshot, caller)
	if err != nil {
		return err
	}
	requests := snapshotRequests(snapshot.Executor, snapshot.Account, current.Number)
	requests = append(requests, executionRead(snapshot, caller, calldata, nil))
	reads, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return err
	}
	if err = validateCurrent(snapshot, current, reads, caller); err != nil {
		return err
	}
	if reads.At(len(requests)-1).Err != nil {
		return errors.New("prepared recipe no longer simulates")
	}
	return closeExecution(ctx, reader, snapshot, current)
}
func executionRead(snapshot Snapshot, caller common.Address, calldata []byte, block *big.Int) protocolio.ReadRequest {
	return protocolio.ReadRequest{Kind: protocolio.ReadCall, Call: ethereum.CallMsg{From: caller, To: &snapshot.Executor, Data: calldata}, Block: block}
}
func validateCurrent(snapshot Snapshot, header *types.Header, reads *protocolio.ReadResults, caller common.Address) error {
	current, err := decodeCurrentSnapshot(snapshot, header, reads)
	if err != nil {
		return err
	}
	if current.Account != snapshot.Account || current.Caller != caller || current.RecipeHash != snapshot.RecipeHash || current.RecipeBlock != snapshot.RecipeBlock || current.Root != snapshot.Root {
		return errors.New("executor identity, caller, recipe or root changed; prepare again")
	}
	return nil
}
func openExecution(ctx context.Context, reader Reader, snapshot Snapshot, caller common.Address) (*types.Header, error) {
	if caller == (common.Address{}) || snapshot.Caller != caller {
		return nil, errors.New("snapshot caller does not match configured sender")
	}
	originalNumber := new(big.Int).SetUint64(snapshot.BlockNumber)
	reads, err := protocolio.ReadGroup(ctx, reader, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: originalNumber}, protocolio.ReadRequest{Kind: protocolio.ReadHeader})
	if err != nil {
		return nil, err
	}
	original := reads.At(0)
	if original.Err != nil {
		return nil, errors.Errorf("check snapshot block: %w", original.Err)
	}
	if err = validateHeader(original.Header, originalNumber); err != nil {
		return nil, err
	}
	if original.Header.Hash() != snapshot.BlockHash {
		return nil, errors.New("snapshot block was reorganized")
	}
	latest := reads.At(1)
	if latest.Err != nil {
		return nil, errors.Errorf("read current head: %w", latest.Err)
	}
	if err = validateHeader(latest.Header, nil); err != nil {
		return nil, err
	}
	if latest.Header.Number.Uint64() < snapshot.BlockNumber {
		return nil, errors.New("current head precedes snapshot block")
	}
	if latest.Header.Number.Uint64() == snapshot.BlockNumber && latest.Header.Hash() != snapshot.BlockHash {
		return nil, errors.New("conflicting headers at snapshot height")
	}
	return latest.Header, nil
}
func closeExecution(ctx context.Context, reader Reader, snapshot Snapshot, current *types.Header) error {
	requests := []protocolio.ReadRequest{{Kind: protocolio.ReadHeader, Block: new(big.Int).SetUint64(snapshot.BlockNumber)}}
	expected := []common.Hash{snapshot.BlockHash}
	if current.Number.Uint64() == snapshot.BlockNumber {
		if current.Hash() != snapshot.BlockHash {
			return errors.New("conflicting headers at snapshot height")
		}
	} else {
		requests = append(requests, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).Set(current.Number)})
		expected = append(expected, current.Hash())
	}
	reads, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return err
	}
	for i, request := range requests {
		member := reads.At(i)
		if member.Err != nil {
			return errors.Errorf("close execution block: %w", member.Err)
		}
		if err = validateHeader(member.Header, request.Block); err != nil {
			return err
		}
		if member.Header.Hash() != expected[i] {
			return errors.New("execution block was reorganized")
		}
	}
	return nil
}

func discoverPreparation(ctx context.Context, reader Reader, executor, account, caller common.Address, options PreparationOptions) (Snapshot, Recipe, protocolio.ReadAccessor, *recipeEvidence, error) {
	if options.Stage == nil && options.Cache == nil {
		snapshot, recipe, err := discoverForBuild(ctx, reader, executor, account, nil)
		return snapshot, recipe, nil, nil, err
	}
	header, err := reader.HeaderByNumber(ctx, nil)
	if err != nil {
		return Snapshot{}, Recipe{}, nil, nil, errors.Errorf("pin block: %w", err)
	}
	if err = validateHeader(header, nil); err != nil {
		return Snapshot{}, Recipe{}, nil, nil, err
	}
	if executor == (common.Address{}) {
		return Snapshot{}, Recipe{}, nil, nil, errors.New("zero executor address")
	}
	pin := protocolio.PreparationPin{BlockNumber: header.Number.Uint64(), BlockHash: header.Hash(), Timestamp: header.Time, Executor: executor, Account: account, Caller: caller}
	if options.ChainID != nil {
		pin.ChainID = options.ChainID.String()
	}
	if err = pin.Validate(); err != nil {
		return Snapshot{}, Recipe{}, nil, nil, err
	}
	key := evidenceKey{chainID: pin.ChainID, executor: executor}
	hint := options.Cache.load(key) // Freeze once; never consult a later concurrent entry.
	requests := snapshotRequests(executor, account, header.Number)
	var extra []protocolio.ReadRequest
	if options.Stage != nil {
		extra = options.Stage(pin)
	}
	requests = append(requests, extra...)
	hintIndex := len(requests)
	if hint != nil && hint.block != pin.BlockNumber {
		requests = append(requests, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).SetUint64(hint.block)})
	}
	reads, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return Snapshot{}, Recipe{}, nil, nil, errors.Errorf("read executor state: %w", err)
	}
	snapshot, err := decodeCurrentSnapshot(Snapshot{Executor: executor, Account: account}, header, reads)
	if err != nil {
		return Snapshot{}, Recipe{}, nil, nil, err
	}
	if err = validateRecipeMetadata(snapshot); err != nil {
		return Snapshot{}, Recipe{}, nil, nil, err
	}
	var recipe Recipe
	var eventHash common.Hash
	hit := false
	if hint != nil && hint.hash == snapshot.RecipeHash && hint.block == snapshot.RecipeBlock {
		eventHeader := header
		if hint.block != pin.BlockNumber {
			result := reads.At(hintIndex)
			if result.Err != nil {
				return Snapshot{}, Recipe{}, nil, nil, errors.Errorf("cached recipe block header: %w", result.Err)
			}
			eventHeader = result.Header
		}
		if err = validateHeader(eventHeader, new(big.Int).SetUint64(hint.block)); err != nil {
			return Snapshot{}, Recipe{}, nil, nil, err
		}
		if eventHeader.Hash() == hint.eventHash {
			hash, hashErr := HashRecipe(hint.recipe)
			_, inspectErr := Inspect(hint.recipe)
			hit = hashErr == nil && inspectErr == nil && hash == snapshot.RecipeHash && snapshot.RecipeHash != (common.Hash{}) && snapshot.RecipeBlock <= snapshot.BlockNumber
			if hit {
				recipe, eventHash = hint.recipe, hint.eventHash
			}
		}
	}
	if !hit {
		options.Cache.discard(hint)
		recipe, eventHash, err = discoverRecipe(ctx, reader, snapshot, header)
		if err != nil {
			return Snapshot{}, Recipe{}, nil, nil, err
		}
	}
	// Preserve verified bytes separately from the mutable build callback's clone.
	pending := &recipeEvidence{key: key, recipe: recipe, hash: snapshot.RecipeHash, block: snapshot.RecipeBlock, eventHash: eventHash}
	if !options.Cache.retainable(pending) {
		pending = nil
	}
	return snapshot, recipe, func(i int) protocolio.ReadResult {
		if i < 0 || i >= len(extra) {
			return protocolio.ReadResult{Err: errors.New("staged result index outside preparation")}
		}
		return reads.At(5 + i)
	}, pending, nil
}
