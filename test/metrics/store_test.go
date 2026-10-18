package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"ubnt-exporter/internal/definitions"
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
		if *lp.Name == "version" && *lp.Value == definitions.VERSION {
			foundVer = true
		}
	}
	if !foundVer {
		t.Errorf("version label %q not found in build info: %v", definitions.VERSION, metric.Label)
	}
}

func TestNewStoreIsolationAndScrapeErrorPersistence(t *testing.T) {
	global := metrics.GetStore()
	global.ClearAllMetrics()

	isolated := metrics.NewStore()
	isolated.UpdateMetric("cpu_usage", map[string]string{"name": "ISO-AP", "ip": "10.9.9.9"}, 88.0)

	if global.CountMetricDataPoints("cpu_usage") != 0 {
		t.Errorf("expected global store to remain unaffected by isolated store")
	}

	isolated.IncrementMetric("scrape_error", map[string]string{"name": "ISO-AP", "ip": "10.9.9.9", "reason": "timeout"}, 1.0)
	isolated.RemoveDeviceMetrics("ISO-AP", "10.9.9.9")

	if isolated.CountMetricDataPoints("cpu_usage") != 0 {
		t.Errorf("expected cpu_usage to be purged by RemoveDeviceMetrics")
	}
	errVal, foundErr := isolated.GetDeviceMetric("ISO-AP", "10.9.9.9", "scrape_error")
	if !foundErr || errVal != 1.0 {
		t.Errorf("expected scrape_error counter to be preserved across RemoveDeviceMetrics, got %v (found: %v)", errVal, foundErr)
	}

	// Test RegisterDynamicLabels idempotency and schema change
	isolated.RegisterDynamicLabels([]string{"tower", "sector"})
	isolated.RegisterDynamicLabels([]string{"tower", "sector"})
	isolated.RegisterDynamicLabels([]string{"tower", "region"})

	// Test UpdateMetric and IncrementMetric on Info, Counter, and unknown keys
	isolated.UpdateMetric("nonexistent_key", map[string]string{"name": "ISO-AP", "ip": "10.9.9.9"}, 1.0)
	isolated.IncrementMetric("nonexistent_key", map[string]string{"name": "ISO-AP", "ip": "10.9.9.9"}, 1.0)
	isolated.IncrementMetric("iface_rx_bytes", map[string]string{"name": "ISO-AP", "ip": "10.9.9.9", "interface": "eth0"}, 512.0)
	isolated.UpdateMetric("peer_signal", map[string]string{"ap_name": "ISO-AP", "ap_ip": "10.9.9.9", "peer_name": "CPE1", "peer_ip": "10.9.9.10"}, -55.0)
	isolated.UpdateMetric("device_info", map[string]string{"name": "ISO-AP", "ip": "10.9.9.9", "tower": "T1", "region": "R1"}, 1.0)

	if sig, ok := isolated.GetDeviceMetric("ISO-AP", "10.9.9.9", "peer_signal"); !ok || sig != -55.0 {
		t.Errorf("expected peer_signal -55.0 via ap_name/ap_ip lookup, got %v (ok=%v)", sig, ok)
	}

	// Test GetDeviceMetric misses
	if _, ok := isolated.GetDeviceMetric("ISO-AP", "10.9.9.9", "totally_unknown_key"); ok {
		t.Errorf("expected GetDeviceMetric to return false for unknown key")
	}
	if _, ok := isolated.GetDeviceMetric("WrongName", "10.9.9.9", "peer_signal"); ok {
		t.Errorf("expected GetDeviceMetric to return false for mismatched name")
	}
	if _, ok := isolated.GetDeviceMetric("ISO-AP", "10.9.9.88", "peer_signal"); ok {
		t.Errorf("expected GetDeviceMetric to return false for mismatched IP")
	}

	// Test UbiquitiCollector Describe & Collect
	col := metrics.NewUbiquitiCollector(isolated)
	col.Describe(nil)
	descCh := make(chan *prometheus.Desc, 10)
	col.Describe(descCh)
	reg := prometheus.NewRegistry()
	reg.MustRegister(col)
	metrics.GetTelemetry().Register(reg)
	mfs, err := reg.Gather()
	if err != nil || len(mfs) == 0 {
		t.Errorf("expected gathered metrics from UbiquitiCollector, got len=%d err=%v", len(mfs), err)
	}

	// Test empty buffer Commit and Len
	emptyBuf := metrics.NewBuffer()
	if emptyBuf.Len() != 0 {
		t.Errorf("expected empty buffer Len() == 0")
	}
	emptyBuf.Commit(nil, "ISO-AP", "10.9.9.9")
	emptyBuf.Commit(isolated, "ISO-AP", "10.9.9.9")

	// Test nil registerer
	metrics.GetTelemetry().Register(nil)
}
