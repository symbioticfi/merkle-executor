package recipes

import (
	"bytes"
	"context"
	"math/big"
	"strconv"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// PreparationOptions configures authenticated recipe discovery and its RAM cache.
type PreparationOptions struct {
	ChainID *big.Int
	Cache   *RecipeCache
}

// PreparedRecipe is provisional authenticated discovery. Providers may inspect a
// copy of Recipe; prepared-batch admission authenticates assembled execution bytes.
type PreparedRecipe struct {
	Snapshot Snapshot
	Recipe   Recipe
	pending  *recipeEvidence
	cache    *RecipeCache
}

// MemberError identifies runtime data that must be deferred without discarding
// other independent candidates. Contract and gas rejection concern the envelope.
type MemberError struct {
	Index int
	// Indices contains every inactive member after a successful canonical envelope.
	// Index remains the first member for callers handling one error at a time.
	Indices []int
	Err     error
}

func (e *MemberError) Error() string {
	return "execution member " + strconv.Itoa(e.Index) + ": " + e.Err.Error()
}
func (e *MemberError) Unwrap() error { return e.Err }

// DiscoveryResult keeps one candidate's authenticated evidence or local failure.
// An error-free result still requires prepared-batch admission before execution.
type DiscoveryResult struct {
	Prepared PreparedRecipe
	Err      error
}

// DiscoverBatch requires every configured member to authenticate successfully.
// Startup callers retain strict behavior while sharing the partial discovery core.
func DiscoverBatch(ctx context.Context, reader Reader, executors, accounts []common.Address, caller common.Address, options PreparationOptions) ([]PreparedRecipe, error) {
	results, err := DiscoverCandidates(ctx, reader, executors, accounts, caller, options)
	if err != nil {
		return nil, err
	}
	prepared := make([]PreparedRecipe, len(results))
	for i, result := range results {
		if result.Err != nil {
			return nil, &MemberError{Index: i, Err: result.Err}
		}
		prepared[i] = result.Prepared
	}
	return prepared, nil
}

// DiscoverCandidates pins all candidates to one head and batches metadata,
// shared roots and event evidence. A local member failure preserves independent
// results; transport failures and the closing common block guard fail the group.
func DiscoverCandidates(ctx context.Context, reader Reader, executors, accounts []common.Address, caller common.Address, options PreparationOptions) ([]DiscoveryResult, error) {
	if len(executors) == 0 || len(executors) != len(accounts) || caller == (common.Address{}) {
		return nil, errors.New("executor batch requires matching identities and caller")
	}
	if options.Cache != nil && (options.ChainID == nil || options.ChainID.Sign() <= 0) {
		return nil, errors.New("recipe cache requires a positive chain ID")
	}
	snapshots := make([]Snapshot, len(executors))
	seen := map[common.Address]bool{}
	for i, executor := range executors {
		if executor == (common.Address{}) || accounts[i] == (common.Address{}) || seen[executor] {
			return nil, errors.New("invalid or duplicate batch executor identity")
		}
		seen[executor] = true
		snapshots[i] = Snapshot{Executor: executor, Account: accounts[i]}
	}
	header, err := reader.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, errors.Errorf("pin recipe batch: %w", err)
	}
	if err = validateHeader(header, nil); err != nil {
		return nil, err
	}
	requests, indices := batchSnapshotRequests(snapshots, header.Number)
	reads, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return nil, err
	}
	results := make([]DiscoveryResult, len(snapshots))
	hints := make([]*recipeEvidence, len(snapshots))
	matched := make([]bool, len(snapshots))
	eventIndices := map[uint64]int{}
	logIndices := map[int]int{}
	requests = nil
	for i, snapshot := range snapshots {
		snapshot, err = decodeBatchSnapshot(snapshot, header, reads, indices[i])
		if err == nil && snapshot.Caller != caller {
			err = errors.New("batch executor caller mismatch")
		}
		if err == nil {
			err = validateRecipeMetadata(snapshot)
		}
		if err != nil {
			results[i].Err = err
			continue
		}
		snapshots[i] = snapshot
		results[i].Prepared = PreparedRecipe{Snapshot: snapshot, cache: options.Cache}
		if options.Cache != nil {
			hints[i] = options.Cache.load(evidenceKey{chainID: options.ChainID.String(), executor: snapshot.Executor})
			matched[i] = hints[i] != nil && hints[i].hash == snapshot.RecipeHash && hints[i].block == snapshot.RecipeBlock
		}
		if snapshot.RecipeBlock != snapshot.BlockNumber {
			if _, exists := eventIndices[snapshot.RecipeBlock]; !exists {
				eventIndices[snapshot.RecipeBlock] = len(requests)
				requests = append(requests, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).SetUint64(snapshot.RecipeBlock)})
			}
		}
		if !matched[i] {
			logIndices[i] = len(requests)
			requests = append(requests, recipeLogRead(snapshot))
		}
	}
	evidence, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return nil, err
	}
	var fallback []protocolio.ReadRequest
	fallbackIndices := map[int]int{}
	for i, snapshot := range snapshots {
		if results[i].Err != nil {
			continue
		}
		eventHeader := header
		if snapshot.RecipeBlock != snapshot.BlockNumber {
			result := evidence.At(eventIndices[snapshot.RecipeBlock])
			if result.Err != nil {
				results[i].Err = errors.Errorf("recipe event header: %w", result.Err)
				continue
			}
			eventHeader = result.Header
		}
		if err = validateHeader(eventHeader, new(big.Int).SetUint64(snapshot.RecipeBlock)); err != nil {
			results[i].Err = err
			continue
		}
		if matched[i] && eventHeader.Hash() == hints[i].eventHash {
			hash, hashErr := HashRecipe(hints[i].recipe)
			_, inspectErr := Inspect(hints[i].recipe)
			if hashErr == nil && inspectErr == nil && hash == snapshot.RecipeHash {
				setPreparedRecipe(&results[i].Prepared, hints[i].recipe, eventHeader.Hash(), options.ChainID)
				continue
			}
		}
		options.Cache.discard(hints[i])
		if matched[i] {
			fallbackIndices[i] = len(fallback)
			fallback = append(fallback, recipeLogRead(snapshot))
			continue
		}
		result := evidence.At(logIndices[i])
		if result.Err != nil {
			results[i].Err = errors.Errorf("read recipe events: %w", result.Err)
			continue
		}
		recipe, eventHash, e := recipeFromLogs(snapshot, eventHeader, result.Logs)
		if e != nil {
			results[i].Err = e
			continue
		}
		setPreparedRecipe(&results[i].Prepared, recipe, eventHash, options.ChainID)
	}
	if len(fallback) != 0 {
		logs, e := protocolio.ReadGroup(ctx, reader, fallback...)
		if e != nil {
			return nil, e
		}
		for i := range snapshots {
			index, exists := fallbackIndices[i]
			if !exists {
				continue
			}
			snapshot := snapshots[i]
			eventHeader := header
			if snapshot.RecipeBlock != snapshot.BlockNumber {
				eventHeader = evidence.At(eventIndices[snapshot.RecipeBlock]).Header
			}
			result := logs.At(index)
			if result.Err != nil {
				results[i].Err = errors.Errorf("read recipe events: %w", result.Err)
				continue
			}
			recipe, eventHash, e := recipeFromLogs(snapshot, eventHeader, result.Logs)
			if e != nil {
				results[i].Err = e
				continue
			}
			setPreparedRecipe(&results[i].Prepared, recipe, eventHash, options.ChainID)
		}
	}
	closing, err := protocolio.ReadGroup(ctx, reader, protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).Set(header.Number)})
	if err != nil {
		return nil, err
	}
	if err = verifyPins([]preparedPin{{header.Number.Uint64(), header.Hash()}}, closing.At); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func setPreparedRecipe(prepared *PreparedRecipe, recipe Recipe, eventHash common.Hash, chainID *big.Int) {
	prepared.Recipe = recipe
	if prepared.cache != nil {
		pending := &recipeEvidence{key: evidenceKey{chainID: chainID.String(), executor: prepared.Snapshot.Executor}, recipe: CloneRecipe(recipe), hash: prepared.Snapshot.RecipeHash, block: prepared.Snapshot.RecipeBlock, eventHash: eventHash}
		if prepared.cache.retainable(pending) {
			prepared.pending = pending
		}
	}
}

func recipeLogRead(snapshot Snapshot) protocolio.ReadRequest {
	block := new(big.Int).SetUint64(snapshot.RecipeBlock)
	return protocolio.ReadRequest{Kind: protocolio.ReadLogs, Filter: ethereum.FilterQuery{FromBlock: block, ToBlock: block, Addresses: []common.Address{snapshot.Executor}, Topics: [][]common.Hash{{executorABI.GetABI().Events["SetRecipe"].ID}}}}
}

func batchSnapshotRequests(snapshots []Snapshot, block *big.Int) ([]protocolio.ReadRequest, [][5]int) {
	var requests []protocolio.ReadRequest
	indices := make([][5]int, len(snapshots))
	roots := map[common.Address]int{}
	for i, snapshot := range snapshots {
		for j, request := range snapshotRequests(snapshot.Executor, block) {
			indices[i][j] = len(requests)
			requests = append(requests, request)
		}
		index, exists := roots[snapshot.Account]
		if !exists {
			index = len(requests)
			roots[snapshot.Account] = index
			requests = append(requests, protocolio.ContractRead(snapshot.Account, accountABI.PackRoot(), block, true))
		}
		indices[i][4] = index
	}
	return requests, indices
}

func decodeBatchSnapshot(snapshot Snapshot, header *types.Header, reads *protocolio.ReadResults, indices [5]int) (Snapshot, error) {
	current, err := decodeExecutorSnapshot(snapshot.Executor, snapshot.Account, header, func(j int) ([]byte, error) { result := reads.At(indices[j]); return result.Data, result.Err })
	if err != nil {
		return Snapshot{}, err
	}
	root := reads.At(indices[4])
	return decodeSnapshotRoot(current, root.Data, root.Err)
}

func decodeBatchSnapshots(snapshots []Snapshot, header *types.Header, reads *protocolio.ReadResults, indices [][5]int) ([]Snapshot, error) {
	out := make([]Snapshot, len(snapshots))
	for i, snapshot := range snapshots {
		current, err := decodeBatchSnapshot(snapshot, header, reads, indices[i])
		if err != nil {
			return nil, &MemberError{Index: i, Err: err}
		}
		out[i] = current
	}
	return out, nil
}

// ValidatePreparedBatch authenticates all selected members at one fresh head,
// simulates the exact transaction and closes every distinct block pin together.
// Cache entries become visible only after complete admission succeeds.
// The optional maxGas budget defaults to 16,777,216 when omitted or zero and is
// bounded by the fresh block gas limit. More than one budget is invalid.
func ValidatePreparedBatch(ctx context.Context, reader Reader, prepared []PreparedRecipe, calls []ExecutorCall, leaves [][]common.Hash, witnesses []map[uint]string, caller common.Address, maxGas ...uint64) error {
	return validatePreparedBatch(ctx, reader, prepared, calls, leaves, witnesses, caller, nil, maxGas)
}

// PreparedBatchSimulationRequest contains independent copies of authenticated
// calls and their fresh execution context. Mutating it cannot change admission.
type PreparedBatchSimulationRequest struct {
	Calls       []ExecutorCall
	Caller      common.Address
	BlockNumber *big.Int
	GasLimit    uint64
}

// PreparedBatchSimulation must simulate the exact selected calls inside the
// unchanged external envelope at the supplied block and gas limit. The host owns
// external usefulness and settlement semantics; no inner return bytes are needed.
type PreparedBatchSimulation func(context.Context, PreparedBatchSimulationRequest) error

// ValidatePreparedBatchWithSimulation shares recipe, identity, role and canonical
// guards with ValidatePreparedBatch. Its mandatory callback validates an external
// envelope instead of generic result witnesses and HasWork classification.
// The optional maxGas budget has the same defaults and bounds as that entry point.
func ValidatePreparedBatchWithSimulation(ctx context.Context, reader Reader, prepared []PreparedRecipe, calls []ExecutorCall, leaves [][]common.Hash, caller common.Address, simulate PreparedBatchSimulation, maxGas ...uint64) error {
	if simulate == nil {
		return errors.New("external prepared batch simulation required")
	}
	return validatePreparedBatch(ctx, reader, prepared, calls, leaves, nil, caller, simulate, maxGas)
}

func validatePreparedBatch(ctx context.Context, reader Reader, prepared []PreparedRecipe, calls []ExecutorCall, leaves [][]common.Hash, witnesses []map[uint]string, caller common.Address, simulate PreparedBatchSimulation, maxGas []uint64) error {
	if len(maxGas) > 1 {
		return errors.New("one transaction gas budget allowed")
	}
	gasLimit := params.MaxTxGas
	if len(maxGas) == 1 && maxGas[0] != 0 {
		gasLimit = maxGas[0]
	}
	if len(prepared) == 0 || len(prepared) != len(calls) || len(prepared) != len(leaves) || (simulate == nil && len(prepared) != len(witnesses)) || caller == (common.Address{}) {
		return errors.New("prepared batch member count or caller mismatch")
	}
	snapshots := make([]Snapshot, len(prepared))
	for i, member := range prepared {
		snapshot := member.Snapshot
		if snapshot.Caller != caller || snapshot.Executor != calls[i].Executor || snapshot.Account == (common.Address{}) {
			return errors.New("prepared batch identity mismatch")
		}
		hash, err := HashRecipe(member.Recipe)
		if err != nil || hash != snapshot.RecipeHash {
			return errors.New("prepared recipe commitment mismatch")
		}
		entries, err := Executions(calls[i].Data)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			recipe, _, _ := decodeExecution(entry)
			hash, _ := HashRecipe(recipe)
			if hash != snapshot.RecipeHash {
				return errors.New("execution differs from prepared recipe")
			}
		}
		snapshots[i] = snapshot
	}
	current, err := reader.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	if err = validateHeader(current, nil); err != nil {
		return err
	}
	gasLimit = min(gasLimit, current.GasLimit)
	if gasLimit == 0 {
		return errors.New("positive block gas limit required")
	}
	pins, err := preparedPins(snapshots, current)
	if err != nil {
		return err
	}
	requests, indices := batchSnapshotRequests(snapshots, current.Number)
	roleStart := len(requests)
	type roleKey struct {
		account, executor common.Address
		leaf              common.Hash
	}
	seen := map[roleKey]bool{}
	var roles []common.Hash
	var roleOwners []int
	for i, snapshot := range snapshots {
		for _, leaf := range leaves[i] {
			key := roleKey{snapshot.Account, snapshot.Executor, leaf}
			if seen[key] {
				continue
			}
			seen[key] = true
			roles = append(roles, leaf)
			roleOwners = append(roleOwners, i)
			requests = append(requests, protocolio.ContractRead(snapshot.Account, accountABI.PackHasRole(leaf, snapshot.Executor), current.Number, true))
		}
	}
	openingStart := len(requests)
	requests = append(requests, pinReads(pins)...)
	direct := simulate == nil && len(calls) == 1 && !isMulticall(calls[0].Data)
	simulationIndex := len(requests)
	if direct {
		requests = append(requests, protocolio.ReadRequest{Kind: protocolio.ReadCall, Call: ethereum.CallMsg{From: caller, To: &calls[0].Executor, Data: calls[0].Data, Gas: gasLimit}, Block: current.Number})
	}
	reads, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return err
	}
	if err = verifyPins(pins, func(i int) protocolio.ReadResult { return reads.At(openingStart + i) }); err != nil {
		return err
	}
	fresh, err := decodeBatchSnapshots(snapshots, current, reads, indices)
	if err != nil {
		return err
	}
	for i, snapshot := range snapshots {
		if fresh[i].Account != snapshot.Account || fresh[i].Caller != caller || fresh[i].RecipeHash != snapshot.RecipeHash || fresh[i].RecipeBlock != snapshot.RecipeBlock || fresh[i].Root != snapshot.Root {
			return &MemberError{Index: i, Err: errors.New("executor identity, caller, recipe or root changed; prepare again")}
		}
	}
	for i, leaf := range roles {
		if err = verifyRoleResults([]common.Hash{leaf}, func(int) protocolio.ReadResult { return reads.At(roleStart + i) }); err != nil {
			return &MemberError{Index: roleOwners[i], Err: err}
		}
	}
	var inactive []int
	if simulate != nil {
		request := PreparedBatchSimulationRequest{Calls: make([]ExecutorCall, len(calls)), Caller: caller, BlockNumber: new(big.Int).Set(current.Number), GasLimit: gasLimit}
		for i, call := range calls {
			request.Calls[i] = ExecutorCall{Executor: call.Executor, Data: bytes.Clone(call.Data)}
		}
		if err = simulate(ctx, request); err != nil {
			return simulationFailure(ctx, err)
		}
		if err = ctx.Err(); err != nil {
			return err
		}
	} else {
		var results [][]byte
		if len(calls) == 1 {
			pin := snapshots[0]
			pin.BlockNumber, pin.BlockHash, pin.Timestamp = current.Number.Uint64(), current.Hash(), current.Time
			if isMulticall(calls[0].Data) {
				backend, ok := reader.(BatchReader)
				if !ok {
					return errors.New("batch validation requires code-override support")
				}
				entries, _ := Executions(calls[0].Data)
				returned, e := SimulateBatch(ctx, backend, pin, entries, gasLimit)
				if e != nil {
					return e
				}
				raw, e := batchArguments.Pack(returned)
				if e != nil {
					return e
				}
				results = [][]byte{raw}
			} else {
				simulation := reads.At(simulationIndex)
				if simulation.Err != nil {
					return simulationFailure(ctx, simulation.Err)
				}
				results = [][]byte{simulation.Data}
			}
		} else {
			backend, ok := reader.(BatchReader)
			if !ok {
				return errors.New("executor batch requires code-override support")
			}
			results, err = SimulateExecutors(ctx, backend, caller, calls, current.Number, gasLimit)
			if err != nil {
				return err
			}
		}
		for i, call := range calls {
			if err = VerifyResults(call.Data, results[i], witnesses[i]); err != nil {
				return &MemberError{Index: i, Err: err}
			}
			work, e := HasWork(call.Data, results[i])
			if e != nil {
				return &MemberError{Index: i, Err: e}
			}
			if !work {
				inactive = append(inactive, i)
			}
		}
	}
	closing, err := protocolio.ReadGroup(ctx, reader, pinReads(pins)...)
	if err != nil {
		return err
	}
	if err = verifyPins(pins, closing.At); err != nil {
		return err
	}
	for _, member := range prepared {
		member.cache.put(member.pending)
	}
	if len(inactive) != 0 {
		return &MemberError{Index: inactive[0], Indices: inactive, Err: provider.ErrNotReady}
	}
	return nil
}

type preparedPin struct {
	number uint64
	hash   common.Hash
}

func preparedPins(snapshots []Snapshot, current *types.Header) ([]preparedPin, error) {
	pins := []preparedPin{{current.Number.Uint64(), current.Hash()}}
	seen := map[uint64]common.Hash{current.Number.Uint64(): current.Hash()}
	for _, snapshot := range snapshots {
		if snapshot.BlockNumber > current.Number.Uint64() || snapshot.BlockHash == (common.Hash{}) {
			return nil, errors.New("invalid prepared block pin")
		}
		if hash, exists := seen[snapshot.BlockNumber]; exists {
			if hash != snapshot.BlockHash {
				return nil, errors.New("conflicting prepared block hashes")
			}
			continue
		}
		seen[snapshot.BlockNumber] = snapshot.BlockHash
		pins = append(pins, preparedPin{snapshot.BlockNumber, snapshot.BlockHash})
	}
	return pins, nil
}
func pinReads(pins []preparedPin) []protocolio.ReadRequest {
	requests := make([]protocolio.ReadRequest, len(pins))
	for i, pin := range pins {
		requests[i] = protocolio.ReadRequest{Kind: protocolio.ReadHeader, Block: new(big.Int).SetUint64(pin.number)}
	}
	return requests
}
func verifyPins(pins []preparedPin, result func(int) protocolio.ReadResult) error {
	for i, pin := range pins {
		member := result(i)
		if member.Err != nil {
			return errors.Errorf("check prepared block: %w", member.Err)
		}
		if err := validateHeader(member.Header, new(big.Int).SetUint64(pin.number)); err != nil {
			return err
		}
		if member.Header.Hash() != pin.hash {
			return errors.New("prepared execution block was reorganized")
		}
	}
	return nil
}
