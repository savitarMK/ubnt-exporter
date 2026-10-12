// Package collector extracts metrics from parsed Ubiquiti device data.
package collector

import (
	"time"

	"ubnt-exporter/internal/metrics"
)

// RunCollectorTimed executes a collector function, measures duration, catches panics, and records telemetry.
func RunCollectorTimed(name string, fn func()) {
	start := time.Now()
	success := true
	defer func() {
		if r := recover(); r != nil {
			success = false
		}
		metrics.GetTelemetry().RecordCollector(name, time.Since(start), success)
	}()
	fn()
}

// runCollectorTimed is an internal alias for RunCollectorTimed.
func runCollectorTimed(name string, fn func()) {
	RunCollectorTimed(name, fn)
}

// getDeviceName extracts the device name label or returns a default fallback.
func getDeviceName(apLabels map[string]string) string {
	if apLabels != nil && apLabels["name"] != "" {
		return apLabels["name"]
	}
	return "device"
}
