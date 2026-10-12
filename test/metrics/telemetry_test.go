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
		"ubnt_exporter_devices_configured_total",
		"ubnt_exporter_devices_online_total",
	}

	for _, exp := range expectedMetrics {
		if !foundMetrics[exp] {
			t.Errorf("expected telemetry metric %q not found in gathered metrics", exp)
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
