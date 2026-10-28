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

type Reader interface {
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
	HeaderByNumber(context.Context, *big.Int) (*types.Header, error)
}

// Publication is an original recipe body with indexed canonical provenance.
type Publication struct {
	Executor        common.Address
	RecipeHash      common.Hash
	BlockNumber     uint64
	BlockHash       common.Hash
	TransactionHash common.Hash
	LogIndex        uint
	Recipe          Recipe
}

// PublicationSource reads indexed bodies; request-time RPC log discovery is forbidden.
type PublicationSource interface {
	RecipePublication(context.Context, common.Address, common.Hash, uint64) (Publication, error)
}

// Discover pins all reads and verifies an indexed recipe against current contract
// state and the canonical publication block. The reader must supply PublicationSource.
func Discover(ctx context.Context, reader Reader, executor common.Address, block *big.Int) (Snapshot, Recipe, error) {
	return discover(ctx, reader, executor, common.Address{}, block)
}

// DiscoverForAccount can read the configured account root with executor metadata,
// but accepts it only after verifying that the executor names that exact account.
func DiscoverForAccount(ctx context.Context, reader Reader, executor, expectedAccount common.Address, block *big.Int) (Snapshot, Recipe, error) {
	if expectedAccount == (common.Address{}) {
		return Snapshot{}, Recipe{}, errors.New("expected account must be nonzero")
	}
	return discover(ctx, reader, executor, expectedAccount, block)
}
func discover(ctx context.Context, reader Reader, executor, expectedAccount common.Address, block *big.Int) (Snapshot, Recipe, error) {
	snapshot, recipe, err := discoverForBuild(ctx, reader, executor, expectedAccount, block)
	if err != nil {
		return Snapshot{}, Recipe{}, err
	}
	if err = checkCanonical(ctx, reader, snapshot); err != nil {
		return Snapshot{}, Recipe{}, err
	}
	return snapshot, recipe, nil
}

// discoverForBuild is private: only a guarded operation may defer its final fence.
func discoverForBuild(ctx context.Context, reader Reader, executor, expectedAccount common.Address, block *big.Int) (Snapshot, Recipe, error) {
	header, err := reader.HeaderByNumber(ctx, block)
	if err != nil {
		return Snapshot{}, Recipe{}, errors.Errorf("pin block: %w", err)
	}
	if err = validateHeader(header, block); err != nil {
		return Snapshot{}, Recipe{}, err
	}
	snapshot, err := readSnapshot(ctx, reader, executor, expectedAccount, header)
	if err != nil {
		return Snapshot{}, Recipe{}, err
	}
	recipe, _, err := discoverRecipe(ctx, reader, snapshot, header)
	return snapshot, recipe, err
}

func discoverRecipe(ctx context.Context, reader Reader, snapshot Snapshot, header *types.Header) (Recipe, common.Hash, error) {
	if err := validateRecipeMetadata(snapshot); err != nil {
		return Recipe{}, common.Hash{}, err
	}
	eventBlock := new(big.Int).SetUint64(snapshot.RecipeBlock)
	source, ok := reader.(PublicationSource)
	if !ok {
		return Recipe{}, common.Hash{}, errors.New("indexed recipe publication source required")
	}
	publication, err := source.RecipePublication(ctx, snapshot.Executor, snapshot.RecipeHash, snapshot.RecipeBlock)
	if err != nil {
		return Recipe{}, common.Hash{}, errors.Errorf("read indexed recipe publication: %w", err)
	}
	eventHeader := header
	if snapshot.RecipeBlock != snapshot.BlockNumber {
		evidence, e := protocolio.ReadGroup(ctx, reader, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: eventBlock})
		if e != nil {
			return Recipe{}, common.Hash{}, e
		}
		result := evidence.At(0)
		if result.Err != nil {
			return Recipe{}, common.Hash{}, errors.Errorf("recipe block header: %w", result.Err)
		}
		eventHeader = result.Header
		if err = validateHeader(eventHeader, eventBlock); err != nil {
			return Recipe{}, common.Hash{}, err
		}
	}
	if publication.Executor != snapshot.Executor || publication.RecipeHash != snapshot.RecipeHash || publication.BlockNumber != snapshot.RecipeBlock || publication.BlockHash != eventHeader.Hash() || publication.TransactionHash == (common.Hash{}) {
		return Recipe{}, common.Hash{}, errors.New("invalid or noncanonical indexed recipe publication")
	}
	recipe := publication.Recipe
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

func snapshotRequests(executor, expectedAccount common.Address, block *big.Int) []protocolio.ReadRequest {
	requests := []protocolio.ReadRequest{
		protocolio.ContractRead(executor, executorABI.PackAccount(), block, true),
		protocolio.ContractRead(executor, executorABI.PackCaller(), block, true),
		protocolio.ContractRead(executor, executorABI.PackRecipeHash(), block, true),
		protocolio.ContractRead(executor, executorABI.PackRecipeBlock(), block, true),
	}
	if expectedAccount != (common.Address{}) {
		requests = append(requests, protocolio.ContractRead(expectedAccount, accountABI.PackRoot(), block, true))
	}
	return requests
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
	if expectedAccount != (common.Address{}) && snapshot.Account != expectedAccount {
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
func readSnapshot(ctx context.Context, reader Reader, executor, expectedAccount common.Address, header *types.Header) (Snapshot, error) {
	if executor == (common.Address{}) {
		return Snapshot{}, errors.New("zero executor address")
	}
	block := new(big.Int).Set(header.Number)
	group, err := protocolio.ReadGroup(ctx, reader, snapshotRequests(executor, expectedAccount, block)...)
	if err != nil {
		return Snapshot{}, errors.Errorf("read executor state: %w", err)
	}
	call := func(i int) ([]byte, error) { v := group.At(i); return v.Data, v.Err }
	snapshot, err := decodeExecutorSnapshot(executor, expectedAccount, header, call)
	if err != nil {
		return Snapshot{}, err
	}
	var raw []byte
	if expectedAccount != (common.Address{}) {
		raw, err = call(4)
	} else {
		raw, err = reader.CallContract(ctx, ethereum.CallMsg{To: &snapshot.Account, Data: accountABI.PackRoot()}, block)
	}
	return decodeSnapshotRoot(snapshot, raw, err)
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
func decodeCurrentSnapshot(snapshot Snapshot, header *types.Header, reads *protocolio.ReadResults) (Snapshot, error) {
	current, err := decodeExecutorSnapshot(snapshot.Executor, snapshot.Account, header, func(i int) ([]byte, error) { v := reads.At(i); return v.Data, v.Err })
	if err != nil {
		return Snapshot{}, err
	}
	root := reads.At(4)
	return decodeSnapshotRoot(current, root.Data, root.Err)
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
	opening, err := protocolio.ReadGroup(ctx, reader, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).SetUint64(snapshot.BlockNumber)}, protocolio.ReadRequest{Kind: protocolio.ReadHeader})
	if err != nil {
		return err
	}
	original := opening.At(0)
	if original.Err != nil {
		return errors.Errorf("check snapshot block: %w", original.Err)
	}
	if err := validateHeader(original.Header, new(big.Int).SetUint64(snapshot.BlockNumber)); err != nil {
		return err
	}
	if original.Header.Hash() != snapshot.BlockHash {
		return errors.New("snapshot block was reorganized")
	}
	latest := opening.At(1)
	if latest.Err != nil {
		return errors.Errorf("read current head: %w", latest.Err)
	}
	header := latest.Header
	if err = validateHeader(header, nil); err != nil {
		return err
	}
	if header.Number.Uint64() < snapshot.BlockNumber {
		return errors.New("current head precedes snapshot block")
	}
	current, err := readSnapshot(ctx, reader, snapshot.Executor, snapshot.Account, header)
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
func roleRequests(snapshot Snapshot, leaves []common.Hash) ([]common.Hash, []protocolio.ReadRequest) {
	checked := make(map[common.Hash]bool, len(leaves))
	unique := make([]common.Hash, 0, len(leaves))
	requests := make([]protocolio.ReadRequest, 0, len(leaves))
	for _, leaf := range leaves {
		if checked[leaf] {
			continue
		}
		checked[leaf] = true
		unique = append(unique, leaf)
		requests = append(requests, protocolio.ContractRead(snapshot.Account, accountABI.PackHasRole(leaf, snapshot.Executor), new(big.Int).SetUint64(snapshot.BlockNumber), true))
	}
	return unique, requests
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
func VerifyRoles(ctx context.Context, reader Reader, snapshot Snapshot, leaves []common.Hash) error {
	unique, requests := roleRequests(snapshot, leaves)
	reads, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return errors.Errorf("read leaf roles: %w", err)
	}
	if err = verifyRoleResults(unique, reads.At); err != nil {
		return err
	}
	return checkCanonical(ctx, reader, snapshot)
}
