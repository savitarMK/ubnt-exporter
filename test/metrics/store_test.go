package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"ubnt-exporter/internal/metrics"
)

func TestStoreOperations(t *testing.T) {
	s := metrics.GetStore()
	s.ClearAllMetrics()

	labels := map[string]string{
		"name": "AP-01",
		"ip":   "192.168.1.20",
	}

	s.UpdateMetric("cpu_usage", labels, 25.5)
	s.IncrementMetric("scrape_error", map[string]string{
		"name":   "AP-01",
		"ip":     "192.168.1.20",
		"reason": "timeout",
	}, 2.0)

	cpuPoints := s.CountMetricDataPoints("cpu_usage")
	errPoints := s.CountMetricDataPoints("scrape_error")

	if cpuPoints != 1 {
		t.Errorf("expected 1 cpu data point, got %d", cpuPoints)
	}
	if errPoints != 1 {
		t.Errorf("expected 1 scrape_error data point, got %d", errPoints)
	}

	val, found := s.GetDeviceMetric("AP-01", "192.168.1.20", "cpu_usage")
	if !found || val != 25.5 {
		t.Errorf("expected cpu_usage 25.5, got %v (found: %v)", val, found)
	}

	// Remove device metrics
	s.RemoveDeviceMetrics("AP-01", "192.168.1.20")

	cpuPointsAfter := s.CountMetricDataPoints("cpu_usage")
	if cpuPointsAfter != 0 {
		t.Errorf("expected 0 cpu data points after removal, got %d", cpuPointsAfter)
	}
}

func TestBufferBatchCommit(t *testing.T) {
	s := metrics.GetStore()
	s.ClearAllMetrics()

	buf := metrics.NewBuffer()
	buf.UpdateMetric("cpu_usage", map[string]string{"name": "AP-02", "ip": "192.168.1.30"}, 40.0)
	buf.UpdateMetric("ram_usage", map[string]string{"name": "AP-02", "ip": "192.168.1.30"}, 65.0)

	buf.Commit(s, "AP-02", "192.168.1.30")

	cpuPoints := s.CountMetricDataPoints("cpu_usage")
	ramPoints := s.CountMetricDataPoints("ram_usage")

	if cpuPoints != 1 || ramPoints != 1 {
		t.Fatalf("expected 1 cpu and 1 ram point after buffer commit, got cpu=%d ram=%d", cpuPoints, ramPoints)
	}

	// Now replace with updated metrics via buffer
	buf2 := metrics.NewBuffer()
	buf2.UpdateMetric("cpu_usage", map[string]string{"name": "AP-02", "ip": "192.168.1.30"}, 55.0)
	buf2.Commit(s, "AP-02", "192.168.1.30")

	val, found := s.GetDeviceMetric("AP-02", "192.168.1.30", "cpu_usage")
	if !found || val != 55.0 {
		t.Errorf("expected updated cpu value 55.0, got %v (found: %v)", val, found)
	}

	ramLen := s.CountMetricDataPoints("ram_usage")
	if ramLen != 0 {
		t.Errorf("expected old ram metric to be atomically cleared, got %d", ramLen)
	}
}

func TestBuildInfoGauge(t *testing.T) {
	gauge := metrics.NewBuildInfoGauge()
	if gauge == nil {
		t.Fatal("expected non-nil gauge for build info")
	}

	reg := prometheus.NewRegistry()
	if err := reg.Register(gauge); err != nil {
		t.Fatalf("failed to register build info gauge: %v", err)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather build info: %v", err)
	}
	if len(mfs) != 1 || *mfs[0].Name != "ubnt_build_info" {
		t.Fatalf("expected 1 metric family ubnt_build_info, got %d", len(mfs))
	}
	metric := mfs[0].Metric[0]
	if *metric.Gauge.Value != 1.0 {
		t.Errorf("expected gauge value 1.0, got %v", *metric.Gauge.Value)
	}

	foundVer := false
	for _, lp := range metric.Label {
		if *lp.Name == "version" && *lp.Value == "1.1.0-beta" {
			foundVer = true
		}
	}
	if !foundVer {
		t.Errorf("version label '1.1.0-beta' not found in build info: %v", metric.Label)
	}
}
