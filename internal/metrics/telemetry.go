// Package metrics implements exporter telemetry and Prometheus self-observability.
package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ExporterTelemetry encapsulates operational self-metrics of the ubnt-exporter daemon.
type ExporterTelemetry struct {
	mu sync.RWMutex

	CollectorDuration   *prometheus.HistogramVec
	CollectorSuccess    *prometheus.GaugeVec
	CollectorExecutions *prometheus.CounterVec

	ScrapeRequestsTotal *prometheus.CounterVec
	ScrapeCycleDuration prometheus.Gauge
	LastScrapeTimestamp prometheus.Gauge
	DevicesConfigured   prometheus.Gauge
	DevicesOnline       prometheus.Gauge
}

var defaultTelemetry *ExporterTelemetry
var telemetryOnce sync.Once

// NewExporterTelemetry initializes all Prometheus telemetry metrics.
func NewExporterTelemetry() *ExporterTelemetry {
	t := &ExporterTelemetry{
		CollectorDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ubnt_exporter_collector_duration_seconds",
				Help:    "Duration of individual collector execution in seconds.",
				Buckets: []float64{0.00005, 0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
			},
			[]string{"collector"},
		),
		CollectorSuccess: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ubnt_exporter_collector_success",
				Help: "Status of the last collector execution (1 = success, 0 = failure).",
			},
			[]string{"collector"},
		),
		CollectorExecutions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ubnt_exporter_collector_executions_total",
				Help: "Total count of collector module executions.",
			},
			[]string{"collector", "status"},
		),
		ScrapeRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ubnt_exporter_scrape_requests_total",
				Help: "Total count of target device scrape attempts by status result.",
			},
			[]string{"status"},
		),
		ScrapeCycleDuration: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ubnt_exporter_scrape_cycle_duration_seconds",
				Help: "Duration of the most recent scrape cycle over all configured devices in seconds.",
			},
		),
		LastScrapeTimestamp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ubnt_exporter_last_scrape_timestamp_seconds",
				Help: "Unix timestamp of the most recent completed scrape cycle.",
			},
		),
		DevicesConfigured: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ubnt_exporter_devices_configured_total",
				Help: "Total number of devices configured in the inventory.",
			},
		),
		DevicesOnline: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ubnt_exporter_devices_online_total",
				Help: "Number of monitored devices responding successfully (online) in the current cycle.",
			},
		),
	}

	// Initialize known collector success gauges to 1
	for _, c := range []string{"system", "interfaces", "wireless", "airfiber", "peers"} {
		t.CollectorSuccess.WithLabelValues(c).Set(1)
	}

	return t
}

// GetTelemetry returns the singleton ExporterTelemetry instance.
func GetTelemetry() *ExporterTelemetry {
	telemetryOnce.Do(func() {
		defaultTelemetry = NewExporterTelemetry()
	})
	return defaultTelemetry
}

// Register registers all exporter telemetry collectors with the provided Prometheus registerer.
func (t *ExporterTelemetry) Register(reg prometheus.Registerer) {
	if reg == nil {
		return
	}
	reg.MustRegister(
		t.CollectorDuration,
		t.CollectorSuccess,
		t.CollectorExecutions,
		t.ScrapeRequestsTotal,
		t.ScrapeCycleDuration,
		t.LastScrapeTimestamp,
		t.DevicesConfigured,
		t.DevicesOnline,
	)
}

// RecordCollector records execution duration, success gauge, and execution count for a collector.
func (t *ExporterTelemetry) RecordCollector(collector string, duration time.Duration, success bool) {
	status := "success"
	statusVal := 1.0
	if !success {
		status = "error"
		statusVal = 0.0
	}

	t.CollectorDuration.WithLabelValues(collector).Observe(duration.Seconds())
	t.CollectorSuccess.WithLabelValues(collector).Set(statusVal)
	t.CollectorExecutions.WithLabelValues(collector, status).Inc()
}

// RecordScrapeResult increments the scrape requests counter for a specific outcome.
func (t *ExporterTelemetry) RecordScrapeResult(status string) {
	t.ScrapeRequestsTotal.WithLabelValues(status).Inc()
}

// SetScrapeCycle records overall cycle duration, timestamp, and device availability counts.
func (t *ExporterTelemetry) SetScrapeCycle(duration time.Duration, configured int, online int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.ScrapeCycleDuration.Set(duration.Seconds())
	t.LastScrapeTimestamp.Set(float64(time.Now().Unix()))
	t.DevicesConfigured.Set(float64(configured))
	t.DevicesOnline.Set(float64(online))
}
