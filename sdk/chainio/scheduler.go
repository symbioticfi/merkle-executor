package chainio

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
)

// Limits are host policy. Zero does not silently select a runtime default.
// DefaultCallGas and DefaultOutputBytes conservatively charge undeclared reads;
// they never mutate the actual call message or reduce explicit gas limits.
type Limits struct {
	MaxMembers         int           `yaml:"maxMembers"`
	MaxRequestBytes    uint64        `yaml:"maxRequestBytes"`
	MaxResponseBytes   uint64        `yaml:"maxResponseBytes"`
	MaxCalldataBytes   uint64        `yaml:"maxCalldataBytes"`
	MaxGas             uint64        `yaml:"maxGas"`
	MaxOutputBytes     uint64        `yaml:"maxOutputBytes"`
	DefaultCallGas     uint64        `yaml:"defaultCallGas"`
	DefaultOutputBytes uint64        `yaml:"defaultOutputBytes"`
	MaxConcurrent      int           `yaml:"maxConcurrent"`
	MaxQueuedRequests  int           `yaml:"maxQueuedRequests"`
	MaxQueuedBytes     uint64        `yaml:"maxQueuedBytes"`
	CollectionWindow   time.Duration `yaml:"collectionWindow"`
}

func (l Limits) Validate() error {
	if l.MaxMembers < 1 || l.MaxMembers > MaxReadBatch || l.MaxRequestBytes < 1 || l.MaxRequestBytes > 32<<20 || l.MaxResponseBytes < 1 || l.MaxResponseBytes > 32<<20 || l.MaxCalldataBytes < 1 || l.MaxGas < 1 || l.MaxGas > ^uint64(0)/2 || l.MaxOutputBytes < 1 || l.MaxConcurrent < 1 || l.MaxQueuedRequests < 1 || l.MaxQueuedBytes < 1 || l.CollectionWindow < 0 || l.DefaultCallGas < 1 || l.DefaultCallGas > l.MaxGas || l.DefaultOutputBytes < 1 || l.DefaultOutputBytes > l.MaxOutputBytes || l.DefaultOutputBytes > l.MaxResponseBytes {
		return errors.New("invalid shared read scheduler limits")
	}
	return nil
}

type readCost struct{ request, calldata, gas, output uint64 }

func (c readCost) add(b readCost) readCost {
	return readCost{c.request + b.request, c.calldata + b.calldata, c.gas + b.gas, c.output + b.output}
}
func (c readCost) fits(l Limits) bool {
	return c.request <= l.MaxRequestBytes && c.calldata <= l.MaxCalldataBytes && c.gas <= l.MaxGas && c.output <= l.MaxOutputBytes && c.output <= l.MaxResponseBytes
}

type scheduledRead struct {
	ctx       context.Context
	requests  []ReadRequest
	costs     []readCost
	results   []ReadResult
	err       error
	remaining int
	bytes     uint64
	done      chan struct{}
	mu        sync.Mutex
}
type resultSlot struct {
	owner *scheduledRead
	index int
}
type readPartition struct {
	requests []ReadRequest
	slots    [][]resultSlot
	cost     readCost
}

// Scheduler coalesces currently ready stages and fairly interleaves their reads.
// It never retains results between waves or uses an owner's cancellation as the
// shared backend context. The queue budget covers all accepted outstanding work.
type Scheduler struct {
	backend         BatchReader
	limits          Limits
	mu              sync.Mutex
	queue           []*scheduledRead
	count           int
	bytes           uint64
	running, closed bool
	wake, stop      chan struct{}
	stopOnce        sync.Once
}

func NewScheduler(backend BatchReader, limits Limits) (*Scheduler, error) {
	if backend == nil {
		return nil, errors.New("shared read backend required")
	}
	if e := limits.Validate(); e != nil {
		return nil, e
	}
	return &Scheduler{backend: backend, limits: limits, wake: make(chan struct{}, 1), stop: make(chan struct{})}, nil
}
func (s *Scheduler) Close() error {
	s.stopOnce.Do(func() { s.mu.Lock(); s.closed = true; s.mu.Unlock(); close(s.stop) })
	return nil
}
func (s *Scheduler) BatchRead(ctx context.Context, requests []ReadRequest) ([]ReadResult, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if len(requests) > s.limits.MaxQueuedRequests {
		return nil, errors.New("read stage exceeds scheduler request queue bound")
	}
	if len(requests) == 0 {
		return []ReadResult{}, nil
	}
	job := &scheduledRead{ctx: ctx, requests: make([]ReadRequest, len(requests)), costs: make([]readCost, len(requests)), results: make([]ReadResult, len(requests)), remaining: len(requests), done: make(chan struct{})}
	for i, q := range requests {
		if q.Kind < ReadCall || q.Kind > ReadHeaderByHash {
			return nil, errors.New("invalid read request kind")
		}
		if uint64(len(q.Call.Data))+uint64(len(q.CodeOverride)) > s.limits.MaxCalldataBytes {
			return nil, errors.New("read exceeds scheduler calldata bound")
		}
		wire, e := requestWireBytes(q)
		if e != nil {
			return nil, e
		}
		cost := readCost{request: wire, calldata: uint64(len(q.Call.Data) + len(q.CodeOverride)), gas: max(q.Call.Gas, q.ExpectedGas), output: q.ExpectedOutputBytes}
		if q.Kind == ReadCall && cost.gas == 0 {
			cost.gas = s.limits.DefaultCallGas
		}
		if cost.output == 0 {
			cost.output = s.limits.DefaultOutputBytes
		}
		if !cost.fits(s.limits) {
			return nil, errors.New("read exceeds scheduler partition budget")
		}
		retained := cost.request + cost.output + 512
		if retained > s.limits.MaxQueuedBytes-job.bytes {
			return nil, errors.New("read stage exceeds scheduler queue budget")
		}
		job.bytes += retained
		job.requests[i], job.costs[i] = cloneRequest(q), cost
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("read scheduler is closed")
	}
	if len(requests) > s.limits.MaxQueuedRequests-s.count || job.bytes > s.limits.MaxQueuedBytes-s.bytes {
		s.mu.Unlock()
		return nil, errors.New("read scheduler queue is full")
	}
	s.count += len(requests)
	s.bytes += job.bytes
	s.queue = append(s.queue, job)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.stop:
		return nil, errors.New("read scheduler is closed")
	case <-job.done:
		if job.err != nil {
			return nil, job.err
		}
		return job.results, nil
	}
}
func (s *Scheduler) finish(job *scheduledRead, index int, result ReadResult, err error) {
	job.mu.Lock()
	job.results[index] = cloneResult(result)
	if err != nil && job.err == nil {
		job.err = err
	}
	job.remaining--
	done := job.remaining == 0
	job.mu.Unlock()
	if done {
		s.mu.Lock()
		s.count -= len(job.requests)
		s.bytes -= job.bytes
		s.mu.Unlock()
		close(job.done)
	}
}
func (s *Scheduler) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("read scheduler already running")
	}
	if s.closed {
		s.mu.Unlock()
		return errors.New("read scheduler is closed")
	}
	s.running = true
	s.mu.Unlock()
	workCtx, cancel := context.WithCancel(ctx)
	tasks := make(chan readPartition)
	var workers sync.WaitGroup
	for range s.limits.MaxConcurrent {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for p := range tasks {
				results, err := s.backend.BatchRead(workCtx, p.requests)
				if err == nil && len(results) != len(p.requests) {
					err = errors.New("read batch result count mismatch")
				}
				if err == nil {
					raw, e := json.Marshal(results)
					if e != nil || uint64(len(raw)) > s.limits.MaxResponseBytes {
						err = errors.New("read batch response exceeds scheduler limit")
					}
					var output uint64
					for i, r := range results {
						size := resultOutputBytes(r)
						output += size
						reserved := p.requests[i].ExpectedOutputBytes
						if reserved == 0 {
							reserved = s.limits.DefaultOutputBytes
						}
						if size > reserved {
							err = errors.New("read result exceeds reserved output bound")
						}
					}
					if output > s.limits.MaxOutputBytes {
						err = errors.New("read batch output exceeds scheduler limit")
					}
				}
				for i, slots := range p.slots {
					var result ReadResult
					if err == nil {
						result = results[i]
					}
					size := resultOutputBytes(result)
					for _, slot := range slots {
						if err == nil && size > slot.owner.costs[slot.index].output {
							s.finish(slot.owner, slot.index, ReadResult{}, errors.New("read result exceeds owner output bound"))
						} else {
							s.finish(slot.owner, slot.index, result, err)
						}
					}
				}
			}
		}()
	}
	var pending []readPartition
	defer func() {
		_ = s.Close()
		cancel()
		s.failPartitions(pending, errors.New("read scheduler stopped"))
		close(tasks)
		workers.Wait()
		s.mu.Lock()
		queue := s.queue
		s.queue = nil
		s.mu.Unlock()
		for _, job := range queue {
			for i := range job.requests {
				s.finish(job, i, ReadResult{}, errors.New("read scheduler stopped"))
			}
		}
	}()
	for {
		var send chan readPartition
		var next readPartition
		if len(pending) > 0 {
			send = tasks
			next = pending[0]
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stop:
			return nil
		case send <- next:
			pending = pending[1:]
		case <-s.wake:
			if s.limits.CollectionWindow > 0 {
				timer := time.NewTimer(s.limits.CollectionWindow)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-s.stop:
					timer.Stop()
					return nil
				case <-timer.C:
				}
			}
			s.mu.Lock()
			jobs := s.queue
			s.queue = nil
			s.mu.Unlock()
			incoming := s.partition(jobs)
			// Alternate old and newly ready waves so a large owner cannot monopolize
			// the endpoint while another owner becomes ready behind an in-flight read.
			merged := make([]readPartition, 0, len(pending)+len(incoming))
			for len(pending) > 0 || len(incoming) > 0 {
				if len(pending) > 0 {
					merged = append(merged, pending[0])
					pending = pending[1:]
				}
				if len(incoming) > 0 {
					merged = append(merged, incoming[0])
					incoming = incoming[1:]
				}
			}
			pending = merged
		}
	}
}
func resultOutputBytes(r ReadResult) uint64 {
	total := uint64(len(r.Data))
	if r.Header != nil {
		b, _ := json.Marshal(r.Header)
		total += uint64(len(b))
	}
	if r.Logs != nil {
		b, _ := json.Marshal(r.Logs)
		total += uint64(len(b))
	}
	if r.Receipt != nil {
		b, _ := json.Marshal(r.Receipt)
		total += uint64(len(b))
	}
	return total
}
func (s *Scheduler) failPartitions(parts []readPartition, err error) {
	for _, p := range parts {
		for _, slots := range p.slots {
			for _, slot := range slots {
				s.finish(slot.owner, slot.index, ReadResult{}, err)
			}
		}
	}
}
func (s *Scheduler) partition(jobs []*scheduledRead) []readPartition {
	type unique struct {
		q     ReadRequest
		cost  readCost
		slots []resultSlot
	}
	var reads []*unique
	known := map[string]*unique{}
	maxLen := 0
	for _, j := range jobs {
		maxLen = max(maxLen, len(j.requests))
	}
	// Round-robin ownership prevents a large stage from preceding all small stages.
	for i := 0; i < maxLen; i++ {
		for _, j := range jobs {
			if i >= len(j.requests) {
				continue
			}
			if err := j.ctx.Err(); err != nil {
				s.finish(j, i, ReadResult{}, err)
				continue
			}
			q := j.requests[i]
			key, shared := immutableKey(q)
			u := known[key]
			if !shared || u == nil {
				u = &unique{q: q, cost: j.costs[i]}
				reads = append(reads, u)
				if shared {
					known[key] = u
				}
			} else {
				u.cost.gas = max(u.cost.gas, j.costs[i].gas)
				u.cost.output = max(u.cost.output, j.costs[i].output)
			}
			u.slots = append(u.slots, resultSlot{j, i})
		}
	}
	var parts []readPartition
	p := readPartition{}
	for _, u := range reads {
		if len(p.requests) > 0 && (len(p.requests) >= s.limits.MaxMembers || !p.cost.add(u.cost).fits(s.limits)) {
			parts = append(parts, p)
			p = readPartition{}
		}
		u.q.ExpectedOutputBytes = u.cost.output
		p.requests = append(p.requests, u.q)
		p.slots = append(p.slots, u.slots)
		p.cost = p.cost.add(u.cost)
	}
	if len(p.requests) > 0 {
		parts = append(parts, p)
	}
	return parts
}
func immutableKey(q ReadRequest) (string, bool) {
	switch q.Kind {
	case ReadCall, ReadHeader:
		if q.Block == nil || q.Block.Sign() < 0 {
			return "", false
		}
	case ReadLogs:
		if q.Filter.BlockHash == nil && (q.Filter.ToBlock == nil || q.Filter.ToBlock.Sign() < 0 || q.Filter.FromBlock != nil && q.Filter.FromBlock.Sign() < 0) {
			return "", false
		}
	case ReadReceipt:
		return "", false
	}
	b, e := requestEncoding(q)
	return string(b), e == nil
}
func requestEncoding(q ReadRequest) ([]byte, error) {
	var filter any
	if q.Kind == ReadLogs {
		var e error
		filter, e = filterArgument(q.Filter)
		if e != nil {
			return nil, e
		}
	}
	return json.Marshal(struct {
		Kind         ReadKind
		Chain, Block string
		Call         map[string]any
		Filter       any
		Hash         common.Hash
		Code         []byte
		Safe         bool
	}{q.Kind, q.ChainID, BlockArgument(q.Block), CallArgument(q.Call), filter, q.Hash, q.CodeOverride, q.MulticallSafe})
}
func cloneRequest(q ReadRequest) ReadRequest {
	c := q
	c.Call.Data = bytes.Clone(q.Call.Data)
	c.CodeOverride = bytes.Clone(q.CodeOverride)
	if q.Block != nil {
		c.Block = new(big.Int).Set(q.Block)
	}
	if q.Call.To != nil {
		v := *q.Call.To
		c.Call.To = &v
	}
	cloneBig := func(x *big.Int) *big.Int {
		if x == nil {
			return nil
		}
		return new(big.Int).Set(x)
	}
	c.Call.Value = cloneBig(q.Call.Value)
	c.Call.GasPrice = cloneBig(q.Call.GasPrice)
	c.Call.GasFeeCap = cloneBig(q.Call.GasFeeCap)
	c.Call.GasTipCap = cloneBig(q.Call.GasTipCap)
	c.Call.BlobGasFeeCap = cloneBig(q.Call.BlobGasFeeCap)
	if q.Call.AccessList != nil {
		c.Call.AccessList = make(types.AccessList, len(q.Call.AccessList))
		for i, a := range q.Call.AccessList {
			c.Call.AccessList[i] = a
			if a.StorageKeys != nil {
				c.Call.AccessList[i].StorageKeys = append([]common.Hash{}, a.StorageKeys...)
			}
		}
	}
	if q.Call.BlobHashes != nil {
		c.Call.BlobHashes = append([]common.Hash{}, q.Call.BlobHashes...)
	}
	if q.Call.AuthorizationList != nil {
		c.Call.AuthorizationList = append([]types.SetCodeAuthorization{}, q.Call.AuthorizationList...)
	}
	c.Filter.FromBlock = cloneBig(q.Filter.FromBlock)
	c.Filter.ToBlock = cloneBig(q.Filter.ToBlock)
	if q.Filter.BlockHash != nil {
		h := *q.Filter.BlockHash
		c.Filter.BlockHash = &h
	}
	c.Filter.Addresses = append([]common.Address(nil), q.Filter.Addresses...)
	if q.Filter.Topics != nil {
		c.Filter.Topics = make([][]common.Hash, len(q.Filter.Topics))
		for i, t := range q.Filter.Topics {
			if t != nil {
				c.Filter.Topics[i] = append([]common.Hash{}, t...)
			}
		}
	}
	return c
}
func cloneResult(r ReadResult) ReadResult {
	r.Data = bytes.Clone(r.Data)
	if r.Header != nil {
		r.Header = types.CopyHeader(r.Header)
	}
	r.Logs = cloneLogs(r.Logs)
	if r.Receipt != nil {
		c := *r.Receipt
		if c.BlockNumber != nil {
			c.BlockNumber = new(big.Int).Set(c.BlockNumber)
		}
		if c.EffectiveGasPrice != nil {
			c.EffectiveGasPrice = new(big.Int).Set(c.EffectiveGasPrice)
		}
		if c.BlobGasPrice != nil {
			c.BlobGasPrice = new(big.Int).Set(c.BlobGasPrice)
		}
		c.PostState = bytes.Clone(c.PostState)
		c.Logs = make([]*types.Log, len(r.Receipt.Logs))
		for i, l := range r.Receipt.Logs {
			if l != nil {
				v := cloneLogs([]types.Log{*l})[0]
				c.Logs[i] = &v
			}
		}
		r.Receipt = &c
	}
	return r
}
func cloneLogs(logs []types.Log) []types.Log {
	if logs == nil {
		return nil
	}
	out := make([]types.Log, len(logs))
	for i, l := range logs {
		out[i] = l
		out[i].Data = bytes.Clone(l.Data)
		out[i].Topics = append([]common.Hash{}, l.Topics...)
	}
	return out
}
func (s *Scheduler) one(ctx context.Context, q ReadRequest) (ReadResult, error) {
	r, e := s.BatchRead(ctx, []ReadRequest{q})
	if e != nil {
		return ReadResult{}, e
	}
	return r[0], r[0].Err
}
func (s *Scheduler) CallContract(ctx context.Context, msg ethereum.CallMsg, b *big.Int) ([]byte, error) {
	r, e := s.one(ctx, ReadRequest{Kind: ReadCall, Call: msg, Block: b})
	return r.Data, e
}
func (s *Scheduler) CallContractWithCode(ctx context.Context, msg ethereum.CallMsg, b *big.Int, code []byte) ([]byte, error) {
	r, e := s.one(ctx, ReadRequest{Kind: ReadCall, Call: msg, Block: b, CodeOverride: code})
	return r.Data, e
}
func (s *Scheduler) HeaderByNumber(ctx context.Context, b *big.Int) (*types.Header, error) {
	r, e := s.one(ctx, ReadRequest{Kind: ReadHeader, Block: b})
	return r.Header, e
}
func (s *Scheduler) HeaderByHash(ctx context.Context, h common.Hash) (*types.Header, error) {
	r, e := s.one(ctx, ReadRequest{Kind: ReadHeaderByHash, Hash: h})
	return r.Header, e
}
func (s *Scheduler) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	r, e := s.one(ctx, ReadRequest{Kind: ReadLogs, Filter: q})
	return r.Logs, e
}
func (s *Scheduler) TransactionReceipt(ctx context.Context, h common.Hash) (*types.Receipt, error) {
	r, e := s.one(ctx, ReadRequest{Kind: ReadReceipt, Hash: h})
	return r.Receipt, e
}
