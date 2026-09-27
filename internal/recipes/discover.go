package recipes

import (
	"context"
	"math/big"
	"sort"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
)

type Reader interface {
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error)
}

// Discover pins all reads, finds the last SetRecipe event at recipeBlock, and
// verifies both its indexed commitment and reconstructed ABI hash. Reorgs fail
// closed. No event-supplied body is used without checking current contract state.
func Discover(ctx context.Context, reader Reader, executor common.Address, block *big.Int) (Snapshot, Recipe, error) {
	header, err := reader.HeaderByNumber(ctx, block)
	if err != nil {
		return Snapshot{}, Recipe{}, errors.Errorf("pin block: %w", err)
	}
	if err = validateHeader(header, block); err != nil {
		return Snapshot{}, Recipe{}, err
	}
	snapshot, err := readSnapshot(ctx, reader, executor, header)
	if err != nil {
		return Snapshot{}, Recipe{}, err
	}
	if snapshot.RecipeHash == (common.Hash{}) {
		return Snapshot{}, Recipe{}, errors.New("executor recipe is unconfigured")
	}
	if snapshot.RecipeBlock > snapshot.BlockNumber {
		return Snapshot{}, Recipe{}, errors.New("recipe block is in the future")
	}
	eventBlock := new(big.Int).SetUint64(snapshot.RecipeBlock)
	eventHeader := header
	if snapshot.RecipeBlock != snapshot.BlockNumber {
		eventHeader, err = reader.HeaderByNumber(ctx, eventBlock)
		if err != nil {
			return Snapshot{}, Recipe{}, errors.Errorf("recipe block header: %w", err)
		}
		if err = validateHeader(eventHeader, eventBlock); err != nil {
			return Snapshot{}, Recipe{}, err
		}
	}
	eventID := executorABI.GetABI().Events["SetRecipe"].ID
	logs, err := reader.FilterLogs(ctx, ethereum.FilterQuery{FromBlock: eventBlock, ToBlock: eventBlock, Addresses: []common.Address{executor}, Topics: [][]common.Hash{{eventID}}})
	if err != nil {
		return Snapshot{}, Recipe{}, errors.Errorf("read SetRecipe logs: %w", err)
	}
	if len(logs) == 0 {
		return Snapshot{}, Recipe{}, errors.New("SetRecipe event missing")
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].Index < logs[j].Index })
	for i, log := range logs {
		if log.Removed || log.Address != executor || log.BlockNumber != snapshot.RecipeBlock || log.BlockHash != eventHeader.Hash() || len(log.Topics) != 2 || log.Topics[0] != eventID {
			return Snapshot{}, Recipe{}, errors.New("invalid or noncanonical SetRecipe log")
		}
		if i > 0 && logs[i-1].Index == log.Index {
			return Snapshot{}, Recipe{}, errors.New("duplicate SetRecipe log index")
		}
	}
	latest := &logs[len(logs)-1]
	if latest.Topics[1] != snapshot.RecipeHash {
		return Snapshot{}, Recipe{}, errors.New("latest SetRecipe event differs from current commitment")
	}
	event, err := executorABI.UnpackSetRecipeEvent(latest)
	if err != nil {
		return Snapshot{}, Recipe{}, errors.Errorf("decode SetRecipe: %w", err)
	}
	recipe := Recipe{Queries: event.Queries, Steps: event.Steps}
	hash, err := HashRecipe(recipe)
	if err != nil {
		return Snapshot{}, Recipe{}, err
	}
	if hash != snapshot.RecipeHash {
		return Snapshot{}, Recipe{}, errors.New("SetRecipe body hash mismatch")
	}
	if _, err = Inspect(recipe); err != nil {
		return Snapshot{}, Recipe{}, errors.Errorf("invalid committed recipe: %w", err)
	}
	if err = checkCanonical(ctx, reader, snapshot); err != nil {
		return Snapshot{}, Recipe{}, err
	}
	return snapshot, recipe, nil
}

func readSnapshot(ctx context.Context, reader Reader, executor common.Address, header *types.Header) (Snapshot, error) {
	snapshot := Snapshot{BlockNumber: header.Number.Uint64(), BlockHash: header.Hash(), Timestamp: header.Time, Executor: executor}
	if executor == (common.Address{}) {
		return Snapshot{}, errors.New("zero executor address")
	}
	block := new(big.Int).Set(header.Number)
	call := func(data []byte) ([]byte, error) {
		return reader.CallContract(ctx, ethereum.CallMsg{To: &executor, Data: data}, block)
	}
	raw, err := call(executorABI.PackAccount())
	if err != nil {
		return Snapshot{}, errors.Errorf("read executor account: %w", err)
	}
	snapshot.Account, err = executorABI.UnpackAccount(raw)
	if err != nil {
		return Snapshot{}, errors.Errorf("decode executor account: %w", err)
	}
	if snapshot.Account == (common.Address{}) {
		return Snapshot{}, errors.New("zero executor account")
	}
	raw, err = call(executorABI.PackCaller())
	if err != nil {
		return Snapshot{}, errors.Errorf("read caller: %w", err)
	}
	snapshot.Caller, err = executorABI.UnpackCaller(raw)
	if err != nil {
		return Snapshot{}, errors.Errorf("decode caller: %w", err)
	}
	raw, err = call(executorABI.PackRecipeHash())
	if err != nil {
		return Snapshot{}, errors.Errorf("read recipe hash: %w", err)
	}
	hash, err := executorABI.UnpackRecipeHash(raw)
	if err != nil {
		return Snapshot{}, errors.Errorf("decode recipe hash: %w", err)
	}
	snapshot.RecipeHash = hash
	raw, err = call(executorABI.PackRecipeBlock())
	if err != nil {
		return Snapshot{}, errors.Errorf("read recipe block: %w", err)
	}
	recipeBlock, err := executorABI.UnpackRecipeBlock(raw)
	if err != nil {
		return Snapshot{}, errors.Errorf("decode recipe block: %w", err)
	}
	if recipeBlock == nil || recipeBlock.Sign() < 0 || recipeBlock.BitLen() > 48 {
		return Snapshot{}, errors.New("recipe block does not fit uint48")
	}
	snapshot.RecipeBlock = recipeBlock.Uint64()
	raw, err = reader.CallContract(ctx, ethereum.CallMsg{To: &snapshot.Account, Data: accountABI.PackMerkleRoot()}, block)
	if err != nil {
		return Snapshot{}, errors.Errorf("read account root: %w", err)
	}
	root, err := accountABI.UnpackMerkleRoot(raw)
	if err != nil {
		return Snapshot{}, errors.Errorf("decode account root: %w", err)
	}
	snapshot.Root = root
	return snapshot, nil
}

func validateHeader(header *types.Header, expected *big.Int) error {
	if header == nil || header.Number == nil || !header.Number.IsUint64() {
		return errors.New("invalid block header")
	}
	if expected != nil && header.Number.Cmp(expected) != 0 {
		return errors.New("RPC returned another block")
	}
	return nil
}

func checkCanonical(ctx context.Context, reader Reader, snapshot Snapshot) error {
	block := new(big.Int).SetUint64(snapshot.BlockNumber)
	header, err := reader.HeaderByNumber(ctx, block)
	if err != nil {
		return errors.Errorf("check snapshot block: %w", err)
	}
	if err = validateHeader(header, block); err != nil {
		return err
	}
	if header.Hash() != snapshot.BlockHash {
		return errors.New("snapshot block was reorganized")
	}
	return nil
}

// ValidateSnapshot checks the original block remains canonical and re-reads the
// executor identity, caller, recipe and active account root at the current head.
func ValidateSnapshot(ctx context.Context, reader Reader, snapshot Snapshot, expectedCaller common.Address) error {
	if expectedCaller == (common.Address{}) || snapshot.Caller != expectedCaller {
		return errors.New("snapshot caller does not match configured sender")
	}
	if err := checkCanonical(ctx, reader, snapshot); err != nil {
		return err
	}
	header, err := reader.HeaderByNumber(ctx, nil)
	if err != nil {
		return errors.Errorf("read current head: %w", err)
	}
	if err = validateHeader(header, nil); err != nil {
		return err
	}
	if header.Number.Uint64() < snapshot.BlockNumber {
		return errors.New("current head precedes snapshot block")
	}
	current, err := readSnapshot(ctx, reader, snapshot.Executor, header)
	if err != nil {
		return err
	}
	if current.Account != snapshot.Account || current.Caller != expectedCaller || current.RecipeHash != snapshot.RecipeHash || current.RecipeBlock != snapshot.RecipeBlock || current.Root != snapshot.Root {
		return errors.New("executor identity, caller, recipe or root changed; prepare again")
	}
	return checkCanonical(ctx, reader, current)
}

// VerifyRoles checks only assembled (not explicitly skipped) leaves at the
// snapshot block. The leaf role belongs to the executor, never the keeper EOA.
func VerifyRoles(ctx context.Context, reader Reader, snapshot Snapshot, leaves []common.Hash) error {
	checked := make(map[common.Hash]bool, len(leaves))
	for _, leaf := range leaves {
		if checked[leaf] {
			continue
		}
		checked[leaf] = true
		raw, err := reader.CallContract(ctx, ethereum.CallMsg{To: &snapshot.Account, Data: accountABI.PackHasRole(leaf, snapshot.Executor)}, new(big.Int).SetUint64(snapshot.BlockNumber))
		if err != nil {
			return errors.Errorf("read leaf role %s: %w", leaf, err)
		}
		allowed, err := accountABI.UnpackHasRole(raw)
		if err != nil {
			return errors.Errorf("decode leaf role %s: %w", leaf, err)
		}
		if !allowed {
			return errors.Errorf("executor lacks leaf role %s", leaf)
		}
	}
	return checkCanonical(ctx, reader, snapshot)
}
