// Package collector extracts metrics from parsed Ubiquiti device data.
package collector

import (
	"context"
	"runtime/debug"
	"time"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
)

// RunCollectorTimed executes a collector function, measures duration, catches panics, and records telemetry.
func RunCollectorTimed(name string, fn func()) {
	RunCollectorTimedContext(context.Background(), "", name, fn)
}

// RunCollectorTimedContext is RunCollectorTimed with device and trace context: a recovered panic
// is logged at ERROR level with the device name and stack trace instead of being swallowed, so
// malformed device payloads that crash a collector are visible and reproducible.
func RunCollectorTimedContext(ctx context.Context, device, name string, fn func()) {
	start := time.Now()
	success := true
	defer func() {
		if r := recover(); r != nil {
			success = false
			caller := device
			if caller == "" {
				caller = "collector"
			}
			logger.ErrorContext(ctx, caller, "Collector '%s' panicked (metrics of this collector are incomplete): %v\n%s", name, r, debug.Stack())
		}
		metrics.GetTelemetry().RecordCollector(name, time.Since(start), success)
	}()
	fn()
}

// getDeviceName extracts the device name label or returns a default fallback.
func getDeviceName(apLabels map[string]string) string {
	if apLabels != nil && apLabels["name"] != "" {
		return apLabels["name"]
	}
	return "device"
}
