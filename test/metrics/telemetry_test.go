package metrics_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"ubnt-exporter/internal/metrics"
)

func TestExporterTelemetryRecording(t *testing.T) {
	tel := metrics.NewExporterTelemetry()

	// Test RecordCollector
	tel.RecordCollector("system", 15*time.Millisecond, true)
	tel.RecordCollector("system", 20*time.Millisecond, false)

	// Test RecordScrapeResult
	tel.RecordScrapeResult("success")
	tel.RecordScrapeResult("timeout")

	// Test SetScrapeCycle
	tel.SetScrapeCycle(500*time.Millisecond, 10, 8)

	// Test Register with a fresh registry
	reg := prometheus.NewRegistry()
	tel.Register(reg)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather telemetry metrics: %v", err)
	}

	foundMetrics := make(map[string]bool)
	for _, mf := range mfs {
		foundMetrics[*mf.Name] = true
	}

	expectedMetrics := []string{
		"ubnt_exporter_collector_duration_seconds",
		"ubnt_exporter_collector_success",
		"ubnt_exporter_collector_executions_total",
		"ubnt_exporter_scrape_requests_total",
		"ubnt_exporter_scrape_cycle_duration_seconds",
		"ubnt_exporter_last_scrape_timestamp_seconds",
		"ubnt_exporter_devices_configured",
		"ubnt_exporter_devices_online",
	}

	for _, exp := range expectedMetrics {
		if !foundMetrics[exp] {
			t.Errorf("expected telemetry metric %q not found in gathered metrics", exp)
		}
	}

	// Verify Prometheus 3.x Hybrid Native Histogram + Classic Buckets on CollectorDuration
	for _, mf := range mfs {
		if *mf.Name == "ubnt_exporter_collector_duration_seconds" {
			if len(mf.Metric) == 0 || mf.Metric[0].Histogram == nil {
				t.Fatalf("expected non-empty Histogram on ubnt_exporter_collector_duration_seconds")
			}
			h := mf.Metric[0].Histogram
			if len(h.Bucket) == 0 {
				t.Errorf("expected classic buckets on ubnt_exporter_collector_duration_seconds")
			}
			if h.Schema == nil || len(h.PositiveSpan) == 0 {
				t.Errorf("expected Prometheus 3.x Native Histogram schema and spans on ubnt_exporter_collector_duration_seconds")
			}
		}
	}
}

func TestGetTelemetrySingleton(t *testing.T) {
	t1 := metrics.GetTelemetry()
	t2 := metrics.GetTelemetry()
	if t1 == nil || t2 == nil || t1 != t2 {
		t.Errorf("GetTelemetry should return non-nil identical singleton pointer")
	}
}
