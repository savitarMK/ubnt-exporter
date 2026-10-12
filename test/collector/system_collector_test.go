package collector_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"ubnt-exporter/internal/collector"
	"ubnt-exporter/internal/logger"
)

func TestProcessSystem(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "test-ap", "ip": "10.0.0.1"}

	data := map[string]any{
		"device": map[string]any{
			"cpu": map[string]any{
				"usage": 25.5,
			},
			"ram": map[string]any{
				"usage": 42.0,
				"free":  104857600.0,
				"total": 268435456.0,
			},
			"uptime":    12345.0,
			"powerTime": 54321.0,
		},
	}

	collector.ProcessSystem(context.Background(), data, apLabels, sink)

	if sink.metrics["cpu_usage"] != 25.5 {
		t.Errorf("expected cpu_usage 25.5, got %v", sink.metrics["cpu_usage"])
	}
	if sink.metrics["ram_usage"] != 42.0 {
		t.Errorf("expected ram_usage 42.0, got %v", sink.metrics["ram_usage"])
	}
	if sink.metrics["ram_free_bytes"] != 104857600.0 {
		t.Errorf("expected ram_free_bytes 104857600.0, got %v", sink.metrics["ram_free_bytes"])
	}
	if sink.metrics["ram_total_bytes"] != 268435456.0 {
		t.Errorf("expected ram_total_bytes 268435456.0, got %v", sink.metrics["ram_total_bytes"])
	}
	if sink.metrics["uptime"] != 12345.0 {
		t.Errorf("expected uptime 12345.0, got %v", sink.metrics["uptime"])
	}
	if sink.metrics["af_power_time"] != 54321.0 {
		t.Errorf("expected af_power_time 54321.0, got %v", sink.metrics["af_power_time"])
	}
	if sink.metrics["device_power_time"] != 54321.0 {
		t.Errorf("expected device_power_time 54321.0, got %v", sink.metrics["device_power_time"])
	}
}

func TestProcessSystem_MissingFields_DebugLogging(t *testing.T) {
	origLevel := logger.GetLevel()
	logger.SetLevel("DEBUG")
	defer logger.SetLevel(string(rune(origLevel)))

	var buf bytes.Buffer
	logger.SetOutput(&buf)
	defer logger.SetOutput(os.Stdout)

	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "test-debug-ap", "ip": "10.0.0.1"}

	// Partial data missing cpu, ram, uptime
	data := map[string]any{
		"device": map[string]any{},
	}

	collector.ProcessSystem(context.Background(), data, apLabels, sink)

	logOut := buf.String()
	if !strings.Contains(logOut, "Collector 'system'") {
		t.Errorf("expected collector diagnostic tag in debug log, got: %s", logOut)
	}
	if !strings.Contains(logOut, "'cpu' field missing") {
		t.Errorf("expected cpu missing debug message, got: %s", logOut)
	}
}
