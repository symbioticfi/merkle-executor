package main

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

type discoveryReader struct {
	t                       *testing.T
	head                    *types.Header
	headers, calls, batches int
	mode                    string
}

func (r *discoveryReader) HeaderByNumber(ctx context.Context, b *big.Int) (*types.Header, error) {
	r.headers++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.headers == 1 {
		switch r.mode {
		case "null head":
			return nil, nil
		case "negative head":
			return &types.Header{Number: big.NewInt(-1)}, nil
		case "overflow head":
			return &types.Header{Number: new(big.Int).Lsh(big.NewInt(1), 65)}, nil
		}
	}
	if b != nil && b.Uint64() != 12 {
		r.t.Fatal("closing unpinned header")
	}
	if r.headers > 1 {
		h := types.CopyHeader(r.head)
		switch r.mode {
		case "reorg":
			h.Extra = []byte("reorg")
		case "wrong closing height":
			h.Number = big.NewInt(13)
		case "closing error":
			return nil, errors.New("rpc private")
		}
		return h, nil
	}
	return r.head, nil
}
func (r *discoveryReader) CallContract(ctx context.Context, m ethereum.CallMsg, b *big.Int) ([]byte, error) {
	r.calls++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil || b.Uint64() != 12 || m.To == nil || m.From != (common.Address{}) {
		r.t.Fatal("account discovery changed context")
	}
	if r.mode == "member" {
		return nil, errors.New("private RPC")
	}
	if r.mode == "malformed" {
		return []byte{1}, nil
	}
	binding := executor.NewRecipeExecutor()
	if bytes.Equal(m.Data, binding.PackCaller()) {
		caller := common.HexToAddress("0xabcd")
		switch r.mode {
		case "zero caller":
			caller = common.Address{}
		case "different callers":
			caller = *m.To
		case "malformed caller":
			return []byte{1}, nil
		case "caller member":
			return nil, errors.New("private RPC")
		}
		return binding.GetABI().Methods["caller"].Outputs.Pack(caller)
	}
	if !bytes.Equal(m.Data, binding.PackAccount()) {
		r.t.Fatal("unexpected getter")
	}
	account := common.BigToAddress(new(big.Int).Add(m.To.Big(), big.NewInt(0x700)))
	if r.mode == "zero account" {
		account = common.Address{}
	}
	return executor.NewRecipeExecutor().GetABI().Methods["account"].Outputs.Pack(account)
}
func (r *discoveryReader) FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error) {
	r.t.Fatal("unexpected logs")
	return nil, nil
}

type discoveryBatch struct{ *discoveryReader }

func (r discoveryBatch) BatchRead(ctx context.Context, qs []protocolio.ReadRequest) ([]protocolio.ReadResult, error) {
	r.batches++
	if len(qs) > 256 {
		r.t.Fatal("unbounded group")
	}
	if r.mode == "transport" {
		return nil, errors.New("private transport")
	}
	if r.mode == "shape" {
		return nil, nil
	}
	results := make([]protocolio.ReadResult, len(qs))
	for i, q := range qs {
		if q.Kind != protocolio.ReadCall || !q.MulticallSafe {
			r.t.Fatal("account getter lost aggregation")
		}
		results[i].Data, results[i].Err = r.CallContract(ctx, q.Call, q.Block)
	}
	return results, nil
}
func TestStartupAccountsAreAtomicDeduplicatedAndCanonicallyClosed(t *testing.T) {
	for _, mode := range []string{"valid", "chunked", "zero executor", "null head", "negative head", "overflow head", "member", "malformed", "zero account", "zero caller", "different callers", "malformed caller", "caller member", "reorg", "wrong closing height", "closing error", "transport", "shape", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			r := &discoveryReader{t: t, head: &types.Header{Number: big.NewInt(12)}, mode: mode}
			targets := []common.Address{common.HexToAddress("0x01"), common.HexToAddress("0x02"), common.HexToAddress("0x01")}
			want := 2
			if mode == "zero executor" {
				targets[1] = common.Address{}
			}
			if mode == "chunked" {
				targets = nil
				for i := range 257 {
					targets = append(targets, common.BigToAddress(big.NewInt(int64(i+1))))
				}
				want = 257
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			result, caller, err := discoverIdentity(ctx, discoveryBatch{r}, targets)
			if mode == "valid" || mode == "chunked" {
				if err != nil || len(result) != want || r.calls != 2*want || r.headers != 2 || r.batches != (2*want+255)/256 || caller != common.HexToAddress("0xabcd") {
					t.Fatalf("discovery result%v calls%d headers%d batches%d: %v", result, r.calls, r.headers, r.batches, err)
				}
				for _, target := range targets {
					wantAccount := common.BigToAddress(new(big.Int).Add(target.Big(), big.NewInt(0x700)))
					if result[target] != wantAccount {
						t.Fatalf("executor %s got account %s, want %s", target, result[target], wantAccount)
					}
				}
			} else if err == nil || result != nil {
				t.Fatalf("partial or invalid discovery escaped: %v %v", result, err)
			}
			if (mode == "transport" || mode == "shape") && r.calls != 0 {
				t.Fatal("batch failure retried")
			}
		})
	}
}
func TestStartupAccountsSingleReaderKeepsSamePin(t *testing.T) {
	r := &discoveryReader{t: t, head: &types.Header{Number: big.NewInt(12)}}
	got, caller, err := discoverIdentity(t.Context(), r, []common.Address{common.HexToAddress("0x01")})
	if err != nil || len(got) != 1 || r.calls != 2 || r.headers != 2 || caller != common.HexToAddress("0xabcd") {
		t.Fatal(got, err)
	}
}
