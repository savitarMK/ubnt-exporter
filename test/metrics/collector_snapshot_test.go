package metrics_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"ubnt-exporter/internal/metrics"
)

// Regression: Collect must not hold the store lock while a (slow) consumer reads metrics,
// otherwise device scrapes committing new values are blocked for the whole /metrics request.
func TestCollectDoesNotBlockStoreWriters(t *testing.T) {
	s := metrics.NewStore()
	for _, n := range []string{"a", "b", "c"} {
		s.UpdateMetric("cpu_usage", map[string]string{"name": n, "ip": "10.0.0." + n}, 1)
	}
	c := metrics.NewUbiquitiCollector(s)

	ch := make(chan prometheus.Metric) // unbuffered: Collect blocks on every send
	go func() { c.Collect(ch); close(ch) }()
	<-ch // Collect is now blocked sending the second metric

	written := make(chan struct{})
	go func() {
		s.UpdateMetric("cpu_usage", map[string]string{"name": "d", "ip": "10.0.0.4"}, 2)
		close(written)
	}()
	select {
	case <-written:
	case <-time.After(2 * time.Second):
		t.Fatal("store write blocked while Collect was waiting on a slow consumer")
	}
	for range ch { //nolint:revive // drain
	}
}

// Series that cannot be exposed are dropped and counted instead of silently disappearing.
func TestCollectCountsEmitErrors(t *testing.T) {
	s := metrics.NewStore()
	s.UpdateMetric("cpu_usage", map[string]string{"name": "ok", "ip": "10.0.0.1"}, 1)
	s.UpdateMetric("cpu_usage", map[string]string{"name": "bad\xff", "ip": "10.0.0.2"}, 1) // invalid UTF-8

	counter := metrics.GetTelemetry().MetricEmitErrors
	before := counterValue(t, counter)

	ch := make(chan prometheus.Metric, 10)
	metrics.NewUbiquitiCollector(s).Collect(ch)
	close(ch)
	if n := len(ch); n != 1 {
		t.Errorf("expected 1 valid metric exposed, got %d", n)
	}
	if got := counterValue(t, counter) - before; got != 1 {
		t.Errorf("expected emit error counter to increase by 1, got %v", got)
	}
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	mfs, err := reg.Gather()
	if err != nil || len(mfs) != 1 || len(mfs[0].GetMetric()) != 1 {
		t.Fatalf("gathering counter: %v", err)
	}
	return mfs[0].GetMetric()[0].GetCounter().GetValue()
}
