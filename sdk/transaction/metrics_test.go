package transaction

import (
	"context"
	"math/big"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
)

func metricValue(t *testing.T, reg *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			matches := true
			for key, want := range labels {
				found := false
				for _, label := range metric.Label {
					found = found || label.GetName() == key && label.GetValue() == want
				}
				matches = matches && found
			}
			if matches {
				if metric.Counter != nil {
					return metric.GetCounter().GetValue()
				}
				return metric.GetGauge().GetValue()
			}
		}
	}
	return 0
}

func metricManager(t *testing.T, backend Backend, depth uint64) (*Manager, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	metrics, err := NewMetrics(prometheus.WrapRegistererWithPrefix("test_", reg))
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(backend, practicalSigning(t, "1"), big.NewInt(1), Config{
		MaxFeeGwei: 100, Confirmations: depth, PollInterval: time.Second,
		ReplacementInterval: 30 * time.Second, ShutdownTimeout: time.Millisecond,
	}, logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	m.Metrics = metrics
	if err = m.Initialize(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); m.Start(ctx) }()
	t.Cleanup(func() { stop(); <-done })
	return m, reg
}

func TestMetricsRequireExplicitRegistry(t *testing.T) {
	if _, err := NewMetrics(nil); err == nil {
		t.Fatal("nil registry accepted")
	}
	reg := prometheus.NewRegistry()
	if _, err := NewMetrics(reg); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMetrics(reg); err == nil {
		t.Fatal("duplicate registration accepted")
	}
}

func TestMetricsCountOnlyCanonicalOwnedReceipts(t *testing.T) {
	for _, status := range []uint64{types.ReceiptStatusSuccessful, types.ReceiptStatusFailed} {
		synctest.Test(t, func(t *testing.T) {
			chain := newPracticalChain()
			m, reg := metricManager(t, chain, 1)
			if err := m.Sender("merkle").Submit(t.Context(), ExecutionRequest{To: common.Address{1}}); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			tx := chain.attempts()[0]
			chain.mine(tx, status, 0)
			time.Sleep(time.Second)
			synctest.Wait()
			if metricValue(t, reg, "test_transaction_gas_used_total", nil) != 0 || metricValue(t, reg, "test_transaction_inflight", nil) != 1 || metricValue(t, reg, "test_transaction_unresolved_nonce", nil) != 1 {
				t.Fatal("unconfirmed receipt changed canonical accounting")
			}
			chain.orphan(tx.Hash())
			time.Sleep(time.Second)
			synctest.Wait()
			if metricValue(t, reg, "test_transaction_gas_used_total", nil) != 0 {
				t.Fatal("reorganized receipt counted")
			}
			chain.mine(tx, status, 1)
			time.Sleep(time.Second)
			synctest.Wait()
			outcome := "confirmed"
			if status == types.ReceiptStatusFailed {
				outcome = "reverted"
			}
			if metricValue(t, reg, "test_transaction_requests_total", map[string]string{"outcome": outcome}) != 1 || metricValue(t, reg, "test_transaction_gas_used_total", nil) != 21_000 || metricValue(t, reg, "test_transaction_fee_paid_wei_total", nil) != 441_000_000_000_000 || metricValue(t, reg, "test_transaction_inflight", nil) != 0 || metricValue(t, reg, "test_transaction_unresolved_nonce", nil) != 0 {
				t.Fatal("canonical request, cost or ownership accounting wrong")
			}
			time.Sleep(5 * time.Second)
			if metricValue(t, reg, "test_transaction_gas_used_total", nil) != 21_000 {
				t.Fatal("canonical cost counted twice")
			}
		})
	}
}

func TestMetricsObserveLateCostWithoutReclassifyingRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chain := newPracticalChain()
		m, reg := metricManager(t, chain, 0)
		results, accepted := m.SendAsync(t.Context(), Request{To: common.Address{1}, Solver: "native", Label: "claim", Deadline: time.Now().Add(2 * time.Second)})
		if !accepted {
			t.Fatal("request not admitted")
		}
		result := <-results
		if !result.Uncertain() || metricValue(t, reg, "test_transaction_unresolved_nonce", nil) != 1 || metricValue(t, reg, "test_transaction_inflight", nil) != 0 {
			t.Fatal("expired signed work lost ownership metrics")
		}
		chain.mine(chain.attempts()[0], types.ReceiptStatusSuccessful, 0)
		time.Sleep(time.Second)
		synctest.Wait()
		if metricValue(t, reg, "test_transaction_requests_total", map[string]string{"outcome": "uncertain"}) != 1 || metricValue(t, reg, "test_transaction_requests_total", map[string]string{"outcome": "confirmed"}) != 0 || metricValue(t, reg, "test_transaction_gas_used_total", nil) != 21_000 || metricValue(t, reg, "test_transaction_unresolved_nonce", nil) != 0 {
			t.Fatal("late receipt missed costs or changed original result")
		}
	})
}

type initialResponseBackend struct {
	*practicalChain
	response error
}

func (b *initialResponseBackend) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	if err := b.practicalChain.SendTransaction(ctx, tx); err != nil {
		return err
	}
	return b.response
}

func TestMetricsClassifyBroadcastResponses(t *testing.T) {
	for _, tc := range []struct {
		err     error
		outcome string
	}{
		{nil, "accepted"}, {context.DeadlineExceeded, "uncertain"}, {errors.New("replacement transaction underpriced"), "rejected"},
	} {
		synctest.Test(t, func(t *testing.T) {
			backend := &initialResponseBackend{practicalChain: newPracticalChain(), response: tc.err}
			m, reg := metricManager(t, backend, 0)
			if err := m.Sender("merkle").Submit(t.Context(), ExecutionRequest{To: common.Address{1}}); err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			if metricValue(t, reg, "test_transaction_broadcasts_total", map[string]string{"kind": "initial", "outcome": tc.outcome}) != 1 {
				t.Fatal("submission response incorrectly classified")
			}
		})
	}
}
