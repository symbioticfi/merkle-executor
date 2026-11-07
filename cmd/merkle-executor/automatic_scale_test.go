package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	distributor "github.com/symbioticfi/merkle-executor/api/bindings/merkldistributor"
	multicall "github.com/symbioticfi/merkle-executor/api/bindings/multicall3"
	wire "github.com/symbioticfi/merkle-executor/api/protocoltypes"
	"github.com/symbioticfi/merkle-executor/internal/chain"
	"github.com/symbioticfi/merkle-executor/internal/config"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/service"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	accountbinding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/permissions"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	rt "github.com/symbioticfi/merkle-executor/sdk/runtime"
)

// These fixtures replace only network boundaries. The automatic registry,
// authentication, recipe cache, numerical lens, scheduler, ABI transport and
// simulation witness validation are the production implementations.
type automaticScaleExecutor struct {
	account common.Address
	recipe  recipes.Recipe
	hash    common.Hash
	leaf    recipes.Leaf
	merkl   bool
}

type automaticScaleMetrics struct {
	RPCRequests, Arrays, Singles, RPCMembers, LogicalReads, UniqueReads int
	Aggregates, AggregateMembers, MaxAggregateMembers                   int
	MaxRPCMembers, MaxRequestBytes, MaxResponseBytes, MaxCalldataBytes  int
	MaxConcurrent, CrossOwnerAggregates, FullSimulations, Lenses        int
	Logs, CuratorCalls, RewardCalls                                     int
	Duration                                                            time.Duration
	AllocatedBytes, Allocations                                         uint64
	PeakQueuedRequests                                                  int
	PeakQueuedBytes                                                     uint64
	ReadyWaves, Partitions, DispatchedReads, SharedDeliveries           uint64
	OutstandingRequests                                                 int
	OutstandingBytes                                                    uint64
}

type automaticScaleFixture struct {
	mu                sync.Mutex
	metrics           automaticScaleMetrics
	keys              map[string]bool
	active            int
	executors         map[common.Address]automaticScaleExecutor
	accounts          map[common.Address]common.Hash
	distributors      map[common.Address]common.Hash
	trees             map[common.Hash][]byte
	caller, token     common.Address
	header            *types.Header
	rpc, api          *httptest.Server
	http              *http.Client
	cfg               *config.Config
	path              string
	failExecutorBatch bool
	padding           int
	delayed           common.Address
	release           <-chan struct{}
	apiEntered        chan struct{}
	staticReady       chan struct{}
	apiOnce           sync.Once
	staticOnce        sync.Once
	reads             *chainio.Scheduler
	baseline          chainio.SchedulerStats
	failedTargets     map[common.Address]bool
}

func scaleStep(tb testing.TB, operation byte, names []string, values ...any) []byte {
	tb.Helper()
	arguments := make(abi.Arguments, len(names))
	for i, name := range names {
		kind, err := abi.NewType(name, "", nil)
		if err != nil {
			tb.Fatal(err)
		}
		arguments[i] = abi.Argument{Type: kind}
	}
	raw, err := arguments.Pack(values...)
	if err != nil {
		tb.Fatal(err)
	}
	return append([]byte{operation}, raw...)
}

func newAutomaticScaleFixture(tb testing.TB, count int) *automaticScaleFixture {
	tb.Helper()
	f := &automaticScaleFixture{
		executors: map[common.Address]automaticScaleExecutor{}, accounts: map[common.Address]common.Hash{},
		distributors: map[common.Address]common.Hash{}, trees: map[common.Hash][]byte{}, keys: map[string]bool{},
		caller: common.HexToAddress("0x7000"), token: common.HexToAddress("0x8000"),
		header:     &types.Header{Number: big.NewInt(12), Difficulty: new(big.Int), Time: 1000},
		apiEntered: make(chan struct{}), staticReady: make(chan struct{}),
	}
	var strategies []string
	for i := range count {
		// Four executors share each account. Alternating groups exercise Merkl
		// and intrinsic recipe preparation under the same automatic registry.
		group := i / 4
		account := common.BigToAddress(big.NewInt(int64(0x10000 + group)))
		target := common.BigToAddress(big.NewInt(int64(0x20000 + i)))
		dist := common.BigToAddress(big.NewInt(int64(0x30000 + group)))
		merkl := group%2 == 0
		var leaf recipes.Leaf
		var recipe recipes.Recipe
		if merkl {
			claim := distributor.NewMerklDistributor().PackClaim([]common.Address{account}, []common.Address{f.token}, []*big.Int{big.NewInt(100)}, [][][32]byte{{}})
			leaf = recipes.Leaf{Target: dist, Value: new(big.Int), Data: claim, DataMask: make([]byte, len(claim))}
			f.distributors[dist] = crypto.Keccak256Hash(protocolio.AddressWord(account), protocolio.AddressWord(f.token), protocolio.Word(big.NewInt(100)))
		} else {
			transfer := erc20.NewERC20().PackTransfer(f.caller, new(big.Int))
			mask := make([]byte, len(transfer))
			for j := 36; j < 68; j++ {
				mask[j] = 0xff
			}
			leaf = recipes.Leaf{Target: f.token, Value: new(big.Int), Data: transfer, DataMask: mask}
		}
		var err error
		leaf.Hash, err = recipes.HashLeaf(leaf)
		if err != nil {
			tb.Fatal(err)
		}
		if merkl {
			recipe.Steps = [][]byte{scaleStep(tb, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{}})}
		} else {
			result := func(i int64) *big.Int { return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(i)) }
			input := new(big.Int).Lsh(big.NewInt(1), 216)
			query := scaleStep(tb, 0, []string{"address", "bytes", "uint256[]", "uint256"}, f.token, erc20.NewERC20().PackBalanceOf(account), []*big.Int{}, new(big.Int))[1:]
			recipe = recipes.Recipe{Queries: [][]byte{query}, Steps: [][]byte{
				scaleStep(tb, recipes.Read, []string{"uint256"}, new(big.Int)),
				scaleStep(tb, recipes.Min, []string{"uint256", "uint256"}, result(0), big.NewInt(17)),
				scaleStep(tb, recipes.Equal, []string{"uint256", "uint256"}, input, result(1)),
				scaleStep(tb, recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, big.NewInt(1), [][32]byte{leaf.Hash}, [][]*big.Int{{new(big.Int).Or(new(big.Int).Lsh(big.NewInt(36), 224), input)}}),
			}}
		}
		hash, err := recipes.HashRecipe(recipe)
		if err != nil {
			tb.Fatal(err)
		}
		f.executors[target] = automaticScaleExecutor{account: account, recipe: recipe, hash: hash, leaf: leaf, merkl: merkl}
		f.accounts[account] = leaf.Hash
		permission, err := json.Marshal(map[string]any{"root": leaf.Hash.Hex(), "chainId": 1, "leaves": []any{map[string]any{"hash": leaf.Hash.Hex(), "target": leaf.Target.Hex(), "value": "zero", "data": hexutil.Encode(leaf.Data), "dataMask": hexutil.Encode(leaf.DataMask), "proof": []string{}}}})
		if err != nil {
			tb.Fatal(err)
		}
		f.trees[leaf.Hash] = permission
		strategies = append(strategies, target.Hex())
	}
	f.rpc = httptest.NewServer(http.HandlerFunc(f.serveRPC))
	f.api = httptest.NewServer(http.HandlerFunc(f.serveAPI))
	tb.Cleanup(f.rpc.Close)
	tb.Cleanup(f.api.Close)
	fixtureURL, _ := url.Parse(f.api.URL)
	f.http = &http.Client{Transport: serviceRewriteTransport{host: "api.merkl.xyz", target: fixtureURL}, Timeout: 15 * time.Second}
	var yaml strings.Builder
	fmt.Fprintf(&yaml, "curator:\n  url: %s\nchain:\n  chainId: 1\n  rpcUrl: %s\nstrategies:\n", f.api.URL, f.rpc.URL)
	for _, strategy := range strategies {
		fmt.Fprintf(&yaml, "  - '%s'\n", strategy)
	}
	f.path = filepath.Join(tb.TempDir(), "config.yaml")
	if err := os.WriteFile(f.path, []byte(yaml.String()), 0600); err != nil {
		tb.Fatal(err)
	}
	var err error
	f.cfg, err = config.Load(f.path)
	if err != nil {
		tb.Fatal(err)
	}
	return f
}

func (f *automaticScaleFixture) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metrics, f.keys = automaticScaleMetrics{}, map[string]bool{}
	if f.reads != nil {
		f.baseline = f.reads.Stats()
	}
}

func (f *automaticScaleFixture) snapshot() automaticScaleMetrics {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.metrics
	m.UniqueReads = len(f.keys)
	if f.reads != nil {
		stats := f.reads.Stats()
		m.PeakQueuedRequests, m.PeakQueuedBytes = stats.PeakQueuedRequests, stats.PeakQueuedBytes
		m.ReadyWaves, m.Partitions = stats.ReadyWaves-f.baseline.ReadyWaves, stats.DispatchedPartitions-f.baseline.DispatchedPartitions
		m.DispatchedReads, m.SharedDeliveries = stats.DispatchedUniqueReads-f.baseline.DispatchedUniqueReads, stats.DeduplicatedReadDeliveries-f.baseline.DeduplicatedReadDeliveries
		m.OutstandingRequests, m.OutstandingBytes = stats.QueuedRequests, stats.QueuedBytes
	}
	return m
}

func (f *automaticScaleFixture) serveAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Query().Get("chainId") != "1" {
		http.Error(w, "unexpected fixture request", http.StatusBadRequest)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/merkle-trees/") {
		root := common.HexToHash(strings.TrimPrefix(r.URL.Path, "/merkle-trees/"))
		f.mu.Lock()
		f.metrics.CuratorCalls++
		f.mu.Unlock()
		if tree := f.trees[root]; tree != nil {
			_, _ = w.Write(tree)
			return
		}
	} else if strings.HasPrefix(r.URL.Path, "/v4/users/") && strings.HasSuffix(r.URL.Path, "/rewards") {
		account := common.HexToAddress(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v4/users/"), "/rewards"))
		f.mu.Lock()
		f.metrics.RewardCalls++
		f.mu.Unlock()
		if account == f.delayed && f.release != nil {
			f.apiOnce.Do(func() { close(f.apiEntered) })
			select {
			case <-f.release:
			case <-r.Context().Done():
				return
			}
		}
		if _, ok := f.accounts[account]; ok {
			_ = json.NewEncoder(w).Encode([]wire.MerklEntry{{Chain: wire.MerklChain{ID: 1}, Rewards: []wire.MerklReward{{Token: wire.MerklToken{Address: f.token.Hex()}, Amount: "100", Claimed: "80", Pending: "0", Proofs: []string{}}}}})
			return
		}
	}
	http.Error(w, "unknown fixture resource", http.StatusNotFound)
}

type automaticScaleRPC struct {
	ID     json.RawMessage   `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
}

func (f *automaticScaleFixture) serveRPC(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil || len(raw) == 0 {
		http.Error(w, "missing request", http.StatusBadRequest)
		return
	}
	batch := raw[0] == '['
	var requests []automaticScaleRPC
	if batch {
		err = json.Unmarshal(raw, &requests)
	} else {
		requests = make([]automaticScaleRPC, 1)
		err = json.Unmarshal(raw, &requests[0])
	}
	if err != nil {
		http.Error(w, "malformed request", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.metrics.RPCRequests++
	f.metrics.RPCMembers += len(requests)
	f.metrics.MaxRPCMembers = max(f.metrics.MaxRPCMembers, len(requests))
	f.metrics.MaxRequestBytes = max(f.metrics.MaxRequestBytes, len(raw))
	if batch {
		f.metrics.Arrays++
	} else {
		f.metrics.Singles++
	}
	f.active++
	f.metrics.MaxConcurrent = max(f.metrics.MaxConcurrent, f.active)
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.active--; f.mu.Unlock() }()
	f.mu.Lock()
	if batch && f.failExecutorBatch {
		f.failedTargets = map[common.Address]bool{}
		for _, request := range requests {
			if request.Method != "eth_call" || len(request.Params) == 0 {
				continue
			}
			var call struct {
				To    common.Address
				Input hexutil.Bytes
			}
			if json.Unmarshal(request.Params[0], &call) != nil {
				continue
			}
			if _, exists := f.executors[call.To]; exists {
				f.failedTargets[call.To] = true
			}
			if call.To == common.HexToAddress(protocolio.StandardMulticall3) && len(call.Input) >= 4 {
				method := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
				if args, err := method.Inputs.Unpack(call.Input[4:]); err == nil {
					calls := *abi.ConvertType(args[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
					for _, member := range calls {
						if _, exists := f.executors[member.Target]; exists {
							f.failedTargets[member.Target] = true
						}
					}
				}
			}
		}
		if len(f.failedTargets) > 0 {
			f.failExecutorBatch = false
			f.mu.Unlock()
			http.Error(w, "failed local batch", http.StatusServiceUnavailable)
			return
		}
	}
	f.mu.Unlock()
	responses := make([]any, len(requests))
	calldataBytes := 0
	for i, request := range requests {
		value, err := f.rpcValue(request, &calldataBytes)
		if err != nil {
			responses[i] = map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": map[string]any{"code": -32602, "message": err.Error()}}
		} else {
			responses[i] = map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": value}
		}
	}
	var encoded []byte
	if batch {
		encoded, err = json.Marshal(responses)
	} else {
		encoded, err = json.Marshal(responses[0])
	}
	if err != nil {
		http.Error(w, "cannot encode response", http.StatusInternalServerError)
		return
	}
	f.mu.Lock()
	f.metrics.MaxResponseBytes = max(f.metrics.MaxResponseBytes, len(encoded))
	f.metrics.MaxCalldataBytes = max(f.metrics.MaxCalldataBytes, calldataBytes)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(encoded)
}

func (f *automaticScaleFixture) recordRead(method string, params []json.RawMessage) {
	raw, _ := json.Marshal(params)
	f.mu.Lock()
	f.metrics.LogicalReads++
	f.keys[method+string(raw)] = true
	f.mu.Unlock()
}

func (f *automaticScaleFixture) rpcValue(q automaticScaleRPC, calldataBytes *int) (any, error) {
	if q.Method != "eth_call" {
		f.recordRead(q.Method, q.Params)
	}
	switch q.Method {
	case "eth_chainId":
		return "0x1", nil
	case "eth_getBlockByNumber":
		var tag string
		if len(q.Params) == 0 || json.Unmarshal(q.Params[0], &tag) != nil || tag != "latest" && tag != "0xc" && tag != "0xb" {
			return nil, errors.Errorf("unexpected header pin")
		}
		if tag == "0xb" {
			h := types.CopyHeader(f.header)
			h.Number = big.NewInt(11)
			return h, nil
		}
		return f.header, nil
	case "eth_getLogs":
		var filter struct{ Address []common.Address }
		if len(q.Params) != 1 || json.Unmarshal(q.Params[0], &filter) != nil || len(filter.Address) != 1 {
			return nil, errors.Errorf("unexpected recipe filter")
		}
		target := filter.Address[0]
		entry, ok := f.executors[target]
		if !ok {
			return nil, errors.Errorf("unknown executor")
		}
		event := executor.NewRecipeExecutor().GetABI().Events["SetRecipe"]
		raw, err := event.Inputs.NonIndexed().Pack(entry.recipe.Queries, entry.recipe.Steps)
		f.mu.Lock()
		f.metrics.Logs++
		f.mu.Unlock()
		h := types.CopyHeader(f.header)
		h.Number = big.NewInt(11)
		return []types.Log{{Address: target, Topics: []common.Hash{event.ID, entry.hash}, Data: raw, BlockNumber: 11, BlockHash: h.Hash()}}, err
	case "eth_call":
		var call struct {
			To, From common.Address
			Input    hexutil.Bytes
		}
		if len(q.Params) < 2 || json.Unmarshal(q.Params[0], &call) != nil {
			return nil, errors.Errorf("invalid call")
		}
		*calldataBytes += len(call.Input)
		if len(q.Params) == 3 {
			var overrides map[common.Address]struct{ Code hexutil.Bytes }
			if json.Unmarshal(q.Params[2], &overrides) != nil || len(overrides[call.To].Code) == 0 {
				return nil, errors.Errorf("missing lens override")
			}
			*calldataBytes += len(overrides[call.To].Code)
		}
		if call.To == common.HexToAddress(protocolio.StandardMulticall3) {
			return f.aggregate(call.Input, q.Params[1])
		}
		f.recordRead(q.Method, q.Params)
		if entry, ok := f.executors[call.To]; ok && call.From == f.caller {
			if string(q.Params[1]) != `"0xc"` {
				return nil, errors.Errorf("simulation lost numbered pin")
			}
			lens := len(q.Params) == 3
			method := executor.NewRecipeExecutor().GetABI().Methods["execute"]
			if !lens && (len(call.Input) < 4 || !bytes.Equal(call.Input[:4], method.ID)) {
				return nil, errors.Errorf("unknown caller-bound call")
			}
			if !lens {
				args, err := method.Inputs.Unpack(call.Input[4:])
				if err != nil {
					return nil, err
				}
				if !equalScaleSteps(args[1].([][]byte), entry.recipe.Steps) {
					return nil, errors.Errorf("committed recipe changed")
				}
			}
			values := make([]*big.Int, len(entry.recipe.Steps))
			for i := range values {
				values[i] = new(big.Int)
			}
			if !entry.merkl {
				values[0], values[1], values[2] = big.NewInt(25), big.NewInt(17), big.NewInt(1)
			}
			f.mu.Lock()
			if lens {
				f.metrics.Lenses++
			} else {
				f.metrics.FullSimulations++
			}
			f.mu.Unlock()
			if !entry.merkl && !lens {
				f.staticOnce.Do(func() { close(f.staticReady) })
			}
			raw, err := method.Outputs.Pack(values)
			if !lens {
				raw = append(raw, make([]byte, f.padding)...)
			}
			return hexutil.Bytes(raw), err
		}
		raw, err := f.getter(call.To, call.Input)
		return hexutil.Bytes(raw), err
	default:
		return nil, errors.Errorf("unexpected RPC method %s", q.Method)
	}
}

func equalScaleSteps(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

func (f *automaticScaleFixture) aggregate(data []byte, block json.RawMessage) (hexutil.Bytes, error) {
	method := multicall.NewMulticall3().GetABI().Methods["aggregate3"]
	if len(data) < 4 || !bytes.Equal(data[:4], method.ID) {
		return nil, errors.Errorf("unexpected aggregate")
	}
	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return nil, err
	}
	calls := *abi.ConvertType(args[0], new([]multicall.Multicall3Call3)).(*[]multicall.Multicall3Call3)
	results := make([]multicall.Multicall3Result, len(calls))
	owners := map[common.Address]bool{}
	for i, call := range calls {
		if _, ok := f.executors[call.Target]; ok {
			owners[call.Target] = true
		}
		raw, err := f.getter(call.Target, call.CallData)
		if err != nil {
			return nil, err
		}
		argument, _ := json.Marshal(map[string]any{"to": call.Target, "input": hexutil.Bytes(call.CallData), "from": common.Address{}})
		f.recordRead("eth_call", []json.RawMessage{argument, block})
		results[i] = multicall.Multicall3Result{Success: true, ReturnData: raw}
	}
	f.mu.Lock()
	f.metrics.Aggregates++
	f.metrics.AggregateMembers += len(calls)
	f.metrics.MaxAggregateMembers = max(f.metrics.MaxAggregateMembers, len(calls))
	if len(owners) > 1 {
		f.metrics.CrossOwnerAggregates++
	}
	f.mu.Unlock()
	raw, err := method.Outputs.Pack(results)
	return raw, err
}

func (f *automaticScaleFixture) getter(to common.Address, data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, errors.Errorf("short getter")
	}
	var contract abi.ABI
	entry, isExecutor := f.executors[to]
	_, isAccount := f.accounts[to]
	_, isDistributor := f.distributors[to]
	switch {
	case isExecutor:
		contract = executor.NewRecipeExecutor().GetABI()
	case isAccount:
		contract = accountbinding.NewMerkleAccount().GetABI()
	case isDistributor:
		contract = distributor.NewMerklDistributor().GetABI()
	default:
		return nil, errors.Errorf("unknown getter target %s", to)
	}
	method, err := contract.MethodById(data[:4])
	if err != nil {
		return nil, err
	}
	var values []any
	switch method.Name {
	case "account":
		values = []any{entry.account}
	case "caller":
		values = []any{f.caller}
	case "recipeHash":
		values = []any{entry.hash}
	case "recipeBlock":
		values = []any{big.NewInt(11)}
	case "root":
		values = []any{f.accounts[to]}
	case "hasRole":
		values = []any{true}
	case "getMerkleRoot":
		values = []any{f.distributors[to]}
	case "claimRecipient":
		values = []any{common.Address{}}
	case "claimed":
		values = []any{big.NewInt(80), big.NewInt(900), f.distributors[to]}
	default:
		return nil, errors.Errorf("unexpected getter %s", method.Name)
	}
	return method.Outputs.Pack(values...)
}

func (f *automaticScaleFixture) planner(tb testing.TB) (*rt.Planner, []rt.Strategy) {
	tb.Helper()
	backend, err := chain.Dial(tb.Context(), f.cfg.Chain.RPCURL, "", big.NewInt(1), common.HexToAddress(f.cfg.Chain.Multicall3))
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(backend.Close)
	reads, err := chainio.NewScheduler(backend, f.cfg.Reads)
	if err != nil {
		tb.Fatal(err)
	}
	f.reads = reads
	ctx, cancel := context.WithCancel(tb.Context())
	done := make(chan error, 1)
	go func() { done <- reads.Run(ctx) }()
	tb.Cleanup(func() { _ = reads.Close(); cancel(); <-done })
	var targets []common.Address
	for _, name := range f.cfg.Strategies {
		targets = append(targets, common.HexToAddress(name))
	}
	accounts, caller, err := service.DiscoverIdentity(tb.Context(), reads, targets)
	if err != nil {
		tb.Fatal(err)
	}
	registered, err := service.DefaultIntegrations(registry.Deps{Chain: reads, HTTP: f.http})
	if err != nil {
		tb.Fatal(err)
	}
	source, err := permissions.New(f.cfg.Curator.HTTPConfig(), f.http)
	if err != nil {
		tb.Fatal(err)
	}
	strategies := make([]rt.Strategy, len(targets))
	for i, target := range targets {
		strategies[i] = rt.Strategy{Config: rt.Config{Name: target.Hex(), Executor: target.Hex(), Account: accounts[target].Hex()}, Integrations: registered}
	}
	return &rt.Planner{PermissionSource: source, Chain: reads, ChainID: big.NewInt(1), Caller: caller, RecipeCache: recipes.NewRecipeCache(len(targets), f.cfg.Runtime.RecipeCacheBytes), MaxConcurrent: f.cfg.Runtime.MaxPreparations}, strategies
}

func (f *automaticScaleFixture) prepare(tb testing.TB, planner *rt.Planner, strategies []rt.Strategy) ([]rt.PreparationResult, automaticScaleMetrics) {
	tb.Helper()
	f.reset()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	results := planner.PrepareAll(tb.Context(), strategies)
	duration := time.Since(start)
	runtime.ReadMemStats(&after)
	m := f.snapshot()
	m.Duration, m.AllocatedBytes, m.Allocations = duration, after.TotalAlloc-before.TotalAlloc, after.Mallocs-before.Mallocs
	return results, m
}

func (f *automaticScaleFixture) verify(tb testing.TB, results []rt.PreparationResult, m automaticScaleMetrics, warm bool) {
	tb.Helper()
	merkl, generic := 0, 0
	for _, result := range results {
		if result.Err != nil {
			tb.Fatal(result.Err)
		}
		entry := f.executors[result.Plan.Snapshot.Executor]
		if entry.merkl {
			merkl++
			if len(result.Plan.Operations) != 1 || result.Plan.Operations[0].Integration != "merkl" || !bytes.Equal(result.Plan.Operations[0].Output.Actions["0:0"], entry.leaf.Data) {
				tb.Fatal("automatic Merkl operation or authorized claim changed")
			}
		} else {
			generic++
			if len(result.Plan.Operations) != 0 || result.Plan.Results[0] != "25" || result.Plan.Results[1] != "17" {
				tb.Fatal("intrinsic numerical preparation changed")
			}
		}
	}
	if m.FullSimulations != len(results) || m.Lenses != generic || m.CuratorCalls != len(results) || m.RewardCalls != merkl {
		tb.Fatalf("preparation counts: %+v", m)
	}
	wantLogs := len(results)
	if warm {
		wantLogs = 0
	}
	if m.Logs != wantLogs {
		tb.Fatalf("recipe logs=%d want=%d", m.Logs, wantLogs)
	}
	if len(results) > 1 && m.CrossOwnerAggregates == 0 {
		tb.Fatal("ready executors were never batched across owners")
	}
	if m.Singles != 0 || m.MaxRPCMembers > f.cfg.Reads.MaxMembers || m.MaxAggregateMembers > f.cfg.Reads.MaxMembers || m.MaxConcurrent > f.cfg.Reads.MaxConcurrent || uint64(m.MaxRequestBytes) > f.cfg.Reads.MaxRequestBytes || uint64(m.MaxResponseBytes) > f.cfg.Reads.MaxResponseBytes || uint64(m.MaxCalldataBytes) > f.cfg.Reads.MaxCalldataBytes {
		tb.Fatalf("wire batching bounds exceeded: %+v", m)
	}
	if m.OutstandingRequests != 0 || m.OutstandingBytes != 0 || m.PeakQueuedRequests > f.cfg.Reads.MaxQueuedRequests || m.PeakQueuedBytes > f.cfg.Reads.MaxQueuedBytes || m.Partitions != uint64(m.Arrays) {
		tb.Fatalf("scheduler queue or dispatch accounting changed: %+v", m)
	}
	if len(results) > 1 && m.SharedDeliveries == 0 {
		tb.Fatal("shared-account and pinned reads were never deduplicated")
	}
}

func TestAutomaticRegistryScale(t *testing.T) {
	for _, count := range []int{1, 16, 64, 256, 1024} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f := newAutomaticScaleFixture(t, count)
			planner, strategies := f.planner(t)
			t.Logf("startup metrics=%+v", f.snapshot())
			for _, warm := range []bool{false, true} {
				results, m := f.prepare(t, planner, strategies)
				f.verify(t, results, m, warm)
				t.Logf("warm=%t metrics=%+v", warm, m)
			}
		})
	}
}

func TestAutomaticCLICompositionScale(t *testing.T) {
	for _, count := range []int{1, 16, 64, 256, 1024} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f := newAutomaticScaleFixture(t, count)
			var out bytes.Buffer
			if err := runBotWithHTTP(t.Context(), f.path, true, true, &out, f.http); err != nil {
				t.Fatal(err)
			}
			var plans map[string]rt.Plan
			if err := json.Unmarshal(out.Bytes(), &plans); err != nil || len(plans) != count {
				t.Fatalf("automatic CLI plans=%d error=%v", len(plans), err)
			}
			m := f.snapshot()
			if m.FullSimulations != count || count > 1 && m.CrossOwnerAggregates == 0 {
				t.Fatalf("CLI did not use shared automatic preparation: %+v", m)
			}
		})
	}
}

func TestAutomaticSlowAPIDoesNotHoldReadyExecutors(t *testing.T) {
	f := newAutomaticScaleFixture(t, 16)
	release := make(chan struct{})
	f.delayed, f.release = common.HexToAddress("0x10000"), release
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	planner, strategies := f.planner(t)
	done := make(chan []rt.PreparationResult, 1)
	go func() { done <- planner.PrepareAll(t.Context(), strategies) }()
	select {
	case <-f.apiEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("delayed API was not requested")
	}
	select {
	case <-f.staticReady:
	case <-time.After(5 * time.Second):
		t.Fatal("ready generic executor waited for delayed Merkl API")
	}
	close(release)
	for _, result := range <-done {
		if result.Err != nil {
			t.Fatal(result.Err)
		}
	}
}

func TestAutomaticFailedBatchIsNotRetriedAsSingles(t *testing.T) {
	f := newAutomaticScaleFixture(t, 16)
	planner, strategies := f.planner(t)
	f.reset()
	f.failExecutorBatch = true // Account-only partitions may arrive before executor metadata.
	results := planner.PrepareAll(t.Context(), strategies)
	failed := 0
	for i, result := range results {
		if result.Err != nil {
			failed++
		}
		if f.failedTargets[common.HexToAddress(strategies[i].Config.Executor)] && result.Err == nil {
			t.Fatal("executor consumed a failed partition through a retry")
		}
	}
	if failed == 0 || len(f.failedTargets) == 0 || f.snapshot().Singles != 0 {
		t.Fatalf("failed partition accepted or retried as singles: failed=%d targets=%d metrics=%+v", failed, len(f.failedTargets), f.snapshot())
	}
}

func TestAutomaticSimulationOutputBound(t *testing.T) {
	for _, tc := range []struct {
		name     string
		padding  int
		rejected bool
	}{{"large valid", 16 << 10, false}, {"exceeds reservation", 70 << 10, true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAutomaticScaleFixture(t, 1)
			f.padding = tc.padding
			planner, strategies := f.planner(t)
			results, m := f.prepare(t, planner, strategies)
			if (results[0].Err != nil) != tc.rejected || m.FullSimulations != 1 || m.Singles != 0 {
				t.Fatalf("output bound rejected=%t error=%v metrics=%+v", tc.rejected, results[0].Err, m)
			}
		})
	}
}

func BenchmarkAutomaticRegistryScale(b *testing.B) {
	for _, count := range []int{1, 16, 64, 256, 1024} {
		for _, warm := range []bool{false, true} {
			b.Run(fmt.Sprintf("executors=%d/warm=%t", count, warm), func(b *testing.B) {
				b.StopTimer()
				f := newAutomaticScaleFixture(b, count)
				planner, strategies := f.planner(b)
				if warm {
					results, m := f.prepare(b, planner, strategies)
					f.verify(b, results, m, false)
				}
				b.ReportAllocs()
				var total automaticScaleMetrics
				for range b.N {
					if !warm {
						planner.RecipeCache = recipes.NewRecipeCache(count, f.cfg.Runtime.RecipeCacheBytes)
					}
					f.reset()
					b.StartTimer()
					results := planner.PrepareAll(b.Context(), strategies)
					b.StopTimer()
					m := f.snapshot()
					f.verify(b, results, m, warm)
					total.RPCRequests += m.RPCRequests
					total.LogicalReads += m.LogicalReads
					total.UniqueReads += m.UniqueReads
					total.CuratorCalls += m.CuratorCalls
					total.RewardCalls += m.RewardCalls
				}
				b.ReportMetric(float64(total.RPCRequests)/float64(b.N), "rpc-http/op")
				b.ReportMetric(float64(total.LogicalReads)/float64(b.N), "logical-reads/op")
				b.ReportMetric(float64(total.UniqueReads)/float64(b.N), "unique-reads/op")
				b.ReportMetric(float64(total.CuratorCalls+total.RewardCalls)/float64(b.N), "api-http/op")
			})
		}
	}
}
