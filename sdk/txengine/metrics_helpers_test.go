package txengine

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"math"
	"testing"
)

func histogramCount(tb testing.TB, observer prometheus.Observer) uint64 {
	tb.Helper()
	return histogram(tb, observer).GetSampleCount()
}

func requireHistogram(tb testing.TB, observer prometheus.Observer, wantCount uint64, wantSum float64) {
	tb.Helper()
	value := histogram(tb, observer)
	if got := value.GetSampleCount(); got != wantCount {
		tb.Fatalf("histogram count = %d, want %d", got, wantCount)
	}
	if got := value.GetSampleSum(); math.Abs(got-wantSum) > 1e-12*max(1, math.Abs(got), math.Abs(wantSum)) {
		tb.Fatalf("histogram sum = %v, want %v", got, wantSum)
	}
}

func histogram(tb testing.TB, observer prometheus.Observer) *dto.Histogram {
	tb.Helper()
	metric, ok := observer.(prometheus.Metric)
	if !ok {
		tb.Fatal("histogram observer does not implement prometheus.Metric")
	}
	var value dto.Metric
	if err := metric.Write(&value); err != nil {
		tb.Fatalf("write histogram: %v", err)
	}
	return value.GetHistogram()
}
