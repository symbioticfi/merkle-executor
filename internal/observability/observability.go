// Package observability exposes logs and bounded operational metrics.
package observability

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func NewLogger(debug bool) (logr.Logger, func(), error) {
	cfg := zap.NewProductionConfig()
	if debug {
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	z, err := cfg.Build()
	if err != nil {
		return logr.Logger{}, nil, err
	}
	return zapr.NewLogger(z), func() { _ = z.Sync() }, nil
}

type Metrics struct {
	ready      atomic.Bool
	iterations *prometheus.CounterVec
	registry   *prometheus.Registry
}

func NewMetrics() *Metrics {
	r := prometheus.NewRegistry()
	c := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "merkle_executor_iterations_total", Help: "Keeper iterations by configured strategy and outcome."}, []string{"strategy", "outcome"})
	r.MustRegister(c)
	return &Metrics{registry: r, iterations: c}
}
func (m *Metrics) Observe(strategy, outcome string) {
	m.iterations.WithLabelValues(strategy, outcome).Inc()
}
func (m *Metrics) Ready(value bool) { m.ready.Store(value) }
func (m *Metrics) Server(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !m.ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
}

// Registerer shares the service registry with neutral transaction metrics.
func (m *Metrics) Registerer() prometheus.Registerer { return m.registry }
