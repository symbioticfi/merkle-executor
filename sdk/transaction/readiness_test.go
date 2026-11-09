package transaction

import (
	"context"
	"math/big"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-logr/logr"
)

func TestServiceReadinessSurvivesPendingExecution(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		m, err := New(chain, practicalSigning(t, "1"), big.NewInt(1), Config{
			MaxFeeGwei: 100, PollInterval: time.Second, ShutdownTimeout: time.Millisecond,
		}, logr.Discard())
		if err != nil {
			t.Fatal(err)
		}
		if m.Ready() || m.Available() {
			t.Fatal("manager is ready before initialization")
		}
		if err := m.Initialize(t.Context()); err != nil {
			t.Fatal(err)
		}
		if !m.Ready() || !m.Available() {
			t.Fatal("initialized manager is unavailable")
		}
		ctx, stop := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { defer close(done); m.Start(ctx) }()
		defer func() { stop(); <-done }()
		sender := m.Sender("quote-host")
		for _, confirmed := range []bool{true, false} {
			if err := sender.Submit(t.Context(), ExecutionRequest{To: common.Address{1}}); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			if !m.Ready() || m.Available() || m.LaneReady() {
				t.Fatal("pending signed execution changed service readiness or released admission")
			}
			if !confirmed {
				break
			}
			chain.mine(chain.attempts()[0], types.ReceiptStatusSuccessful, 0)
			time.Sleep(time.Second)
			synctest.Wait()
			result, err := sender.Poll(t.Context())
			if err != nil || result == nil || !m.Ready() || !m.Available() || !m.LaneReady() {
				t.Fatalf("canonical confirmation did not release admission: result=%v err=%v", result, err)
			}
		}
		stop()
		<-done
		if m.Ready() || m.Available() || m.LaneReady() {
			t.Fatal("stopped manager remains ready")
		}
	})
}
