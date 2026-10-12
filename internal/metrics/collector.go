// Package metrics implements the custom Prometheus collector.
package metrics

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"ubnt-exporter/internal/definitions"
)

// NewBuildInfoGauge constructs a standard Prometheus build info gauge metric.
func NewBuildInfoGauge() prometheus.Gauge {
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ubnt_build_info",
		Help: "A metric with a constant '1' value labeled by version, revision, branch, and goversion from which ubnt-exporter was built.",
		ConstLabels: prometheus.Labels{
			"version":   definitions.VERSION,
			"revision":  definitions.Revision,
			"branch":    definitions.Branch,
			"goversion": runtime.Version(),
		},
	})
	g.Set(1)
	return g
}

// UbiquitiCollector collects metrics from the Store for Prometheus scrapers.
type UbiquitiCollector struct {
	store *Store
}

// NewUbiquitiCollector creates a new UbiquitiCollector.
func NewUbiquitiCollector(store *Store) *UbiquitiCollector {
	return &UbiquitiCollector{store: store}
}

// Describe sends nothing to mark this as an unchecked dynamic collector.
func (c *UbiquitiCollector) Describe(_ chan<- *prometheus.Desc) {
	// Intentionally left blank for dynamic descriptors
}

// Collect takes a snapshot of the metric store and yields Prometheus metrics.
func (c *UbiquitiCollector) Collect(ch chan<- prometheus.Metric) {
	c.store.mu.RLock()
	defer c.store.mu.RUnlock()

	for key, def := range c.store.defs {
		dataPoints, ok := c.store.data[key]
		if !ok || len(dataPoints) == 0 {
			continue
		}

		desc := c.store.descs[key]
		if desc == nil {
			desc = prometheus.NewDesc(def.Name, def.Help, def.Labels, nil)
		}

		for compKey, val := range dataPoints {
			labelVals := c.store.labelsMap[key][compKey]

			var valueType prometheus.ValueType
			metricVal := val

			switch def.Type {
			case definitions.TypeCounter:
				valueType = prometheus.CounterValue
			case definitions.TypeGauge:
				valueType = prometheus.GaugeValue
			case definitions.TypeInfo:
				valueType = prometheus.GaugeValue
				metricVal = 1.0
			default:
				valueType = prometheus.GaugeValue
			}

			m, err := prometheus.NewConstMetric(desc, valueType, metricVal, labelVals...)
			if err == nil {
				ch <- m
			}
		}
	}
}
