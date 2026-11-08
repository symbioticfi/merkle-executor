package transaction

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics has no workers. Hosts register it explicitly and set Manager.Metrics before Start.
type Metrics struct {
	requests, broadcasts, gasUsed, feePaid *prometheus.CounterVec
	inflight                               *prometheus.GaugeVec
	unresolved                             prometheus.Gauge
}

// NewMetrics registers neutral metric names; hosts may prefix the registerer.
func NewMetrics(reg prometheus.Registerer) (*Metrics, error) {
	if reg == nil {
		return nil, errors.New("transaction metrics registerer required")
	}
	m := &Metrics{
		requests:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "transaction_requests_total", Help: "Admitted transaction requests by terminal result."}, []string{"solver", "label", "outcome"}),
		inflight:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "transaction_inflight", Help: "Admitted requests awaiting their result."}, []string{"solver", "label"}),
		broadcasts: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "transaction_broadcasts_total", Help: "Submission RPC attempts by kind and response; an accepted response does not prove inclusion."}, []string{"solver", "label", "kind", "outcome"}),
		gasUsed:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "transaction_gas_used_total", Help: "Gas used by owned canonical receipts, including late resolution."}, []string{"solver", "label", "outcome"}),
		feePaid:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "transaction_fee_paid_wei_total", Help: "Owned canonical receipt gas used multiplied by effective gas price, in wei."}, []string{"solver", "label", "outcome"}),
		unresolved: prometheus.NewGauge(prometheus.GaugeOpts{Name: "transaction_unresolved_nonce", Help: "One while any signed variants retain unresolved nonce ownership."}),
	}
	for _, collector := range []prometheus.Collector{m.requests, m.inflight, m.broadcasts, m.gasUsed, m.feePaid, m.unresolved} {
		if err := reg.Register(collector); err != nil {
			return nil, errors.Errorf("register transaction metric: %w", err)
		}
	}
	return m, nil
}

func (m *Metrics) begin(req Request) {
	if m != nil {
		m.inflight.WithLabelValues(req.Solver, req.Label).Inc()
	}
}

func (m *Metrics) finish(req Request, result Result) {
	if m == nil {
		return
	}
	outcome := "failed"
	switch {
	case result.Reverted():
		outcome = "reverted"
	case result.Included():
		outcome = "confirmed"
	case result.Uncertain():
		outcome = "uncertain"
	}
	m.requests.WithLabelValues(req.Solver, req.Label, outcome).Inc()
	m.inflight.WithLabelValues(req.Solver, req.Label).Dec()
}

func (m *Metrics) broadcast(req Request, kind string, err error) {
	if m == nil {
		return
	}
	outcome := "accepted"
	if err != nil {
		outcome = "uncertain"
		if definiteRejection(err) {
			outcome = "rejected"
		}
	}
	m.broadcasts.WithLabelValues(req.Solver, req.Label, kind, outcome).Inc()
}

func (m *Metrics) signed() {
	if m != nil {
		m.unresolved.Set(1)
	}
}

func (m *Metrics) receipt(req Request, receipt *types.Receipt) {
	if m == nil {
		return
	}
	outcome := "confirmed"
	if receipt.Status == types.ReceiptStatusFailed {
		outcome = "reverted"
	}
	m.gasUsed.WithLabelValues(req.Solver, req.Label, outcome).Add(float64(receipt.GasUsed))
	if uint256(receipt.EffectiveGasPrice) {
		fee := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), receipt.EffectiveGasPrice)
		value, _ := new(big.Float).SetInt(fee).Float64()
		m.feePaid.WithLabelValues(req.Solver, req.Label, outcome).Add(value)
	}
	m.unresolved.Set(0)
}
