package recipes

import (
	"context"
	"math/big"
	"sort"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type Reader interface {
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
	FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error)
}

func recipeFromLogs(snapshot Snapshot, eventHeader *types.Header, logs []types.Log) (Recipe, common.Hash, error) {
	eventID := executorABI.GetABI().Events["SetRecipe"].ID
	if len(logs) == 0 {
		return Recipe{}, common.Hash{}, errors.New("SetRecipe event missing")
	}
	sort.Slice(logs, func(i, j int) bool { return logs[i].Index < logs[j].Index })
	for i, log := range logs {
		if log.Removed || log.Address != snapshot.Executor || log.BlockNumber != snapshot.RecipeBlock || log.BlockHash != eventHeader.Hash() || len(log.Topics) != 2 || log.Topics[0] != eventID {
			return Recipe{}, common.Hash{}, errors.New("invalid or noncanonical SetRecipe log")
		}
		if i > 0 && logs[i-1].Index == log.Index {
			return Recipe{}, common.Hash{}, errors.New("duplicate SetRecipe log index")
		}
	}
	latest := &logs[len(logs)-1]
	if latest.Topics[1] != snapshot.RecipeHash {
		return Recipe{}, common.Hash{}, errors.New("latest SetRecipe event differs from current commitment")
	}
	event, err := executorABI.UnpackSetRecipeEvent(latest)
	if err != nil {
		return Recipe{}, common.Hash{}, errors.Errorf("decode SetRecipe: %w", err)
	}
	recipe := Recipe{Queries: event.Queries, Steps: event.Steps}
	hash, err := HashRecipe(recipe)
	if err != nil {
		return Recipe{}, common.Hash{}, err
	}
	if hash != snapshot.RecipeHash {
		return Recipe{}, common.Hash{}, errors.New("SetRecipe body hash mismatch")
	}
	if _, err = Inspect(recipe); err != nil {
		return Recipe{}, common.Hash{}, errors.Errorf("invalid committed recipe: %w", err)
	}
	return recipe, eventHeader.Hash(), nil
}

func validateRecipeMetadata(snapshot Snapshot) error {
	if snapshot.RecipeHash == (common.Hash{}) {
		return errors.New("executor recipe is unconfigured")
	}
	if snapshot.RecipeBlock > snapshot.BlockNumber {
		return errors.New("recipe block is in the future")
	}
	return nil
}

func snapshotRequests(executor common.Address, block *big.Int) []protocolio.ReadRequest {
	return []protocolio.ReadRequest{
		protocolio.ContractRead(executor, executorABI.PackAccount(), block, true),
		protocolio.ContractRead(executor, executorABI.PackCaller(), block, true),
		protocolio.ContractRead(executor, executorABI.PackRecipeHash(), block, true),
		protocolio.ContractRead(executor, executorABI.PackRecipeBlock(), block, true),
	}
}
func decodeExecutorSnapshot(executor, expectedAccount common.Address, header *types.Header, call func(int) ([]byte, error)) (Snapshot, error) {
	snapshot := Snapshot{BlockNumber: header.Number.Uint64(), BlockHash: header.Hash(), Timestamp: header.Time, Executor: executor}
	if executor == (common.Address{}) {
		return Snapshot{}, errors.New("zero executor address")
	}
	raw, err := call(0)
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
	if snapshot.Account != expectedAccount {
		return Snapshot{}, errors.New("executor account differs from configured account")
	}
	raw, err = call(1)
	if err != nil {
		return Snapshot{}, errors.Errorf("read caller: %w", err)
	}
	snapshot.Caller, err = executorABI.UnpackCaller(raw)
	if err != nil {
		return Snapshot{}, errors.Errorf("decode caller: %w", err)
	}
	raw, err = call(2)
	if err != nil {
		return Snapshot{}, errors.Errorf("read recipe hash: %w", err)
	}
	hash, err := executorABI.UnpackRecipeHash(raw)
	if err != nil {
		return Snapshot{}, errors.Errorf("decode recipe hash: %w", err)
	}
	snapshot.RecipeHash = hash
	raw, err = call(3)
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

	return snapshot, nil
}
func decodeSnapshotRoot(snapshot Snapshot, raw []byte, err error) (Snapshot, error) {
	if err != nil {
		return Snapshot{}, errors.Errorf("read account root: %w", err)
	}
	root, err := accountABI.UnpackRoot(raw)
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

func verifyRoleResults(leaves []common.Hash, result func(int) protocolio.ReadResult) error {
	for i, leaf := range leaves {
		member := result(i)
		if member.Err != nil {
			return errors.Errorf("read leaf role %s: %w", leaf, member.Err)
		}
		allowed, err := accountABI.UnpackHasRole(member.Data)
		if err != nil {
			return errors.Errorf("decode leaf role %s: %w", leaf, err)
		}
		if !allowed {
			return errors.Errorf("executor lacks leaf role %s", leaf)
		}
	}
	return nil
}
