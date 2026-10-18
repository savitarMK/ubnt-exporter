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
	defer func() {
		switch origLevel {
		case logger.LevelDebug:
			logger.SetLevel("DEBUG")
		case logger.LevelWarn:
			logger.SetLevel("WARN")
		case logger.LevelError:
			logger.SetLevel("ERROR")
		default:
			logger.SetLevel("INFO")
		}
	}()

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

func TestProcessSystem_AirMaxAC_EnrichedTelemetry(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "ap-ac-sector", "ip": "10.0.4.155"}

	data := map[string]any{
		"timestamp": 1766718625964.0,
		"device": map[string]any{
			"cpu": []any{
				map[string]any{
					"identifier": "MIPS 74Kc V4.12",
					"usage":      24.0,
				},
			},
			"ram": map[string]any{
				"usage": 60.0,
				"free":  25677824.0,
				"total": 63447040.0,
			},
			"temperatures": []any{
				map[string]any{
					"name":  "Board",
					"value": 52.5,
				},
			},
			"uptime": 249588.0,
			"gps": map[string]any{
				"fix": false,
				"lat": 18.412005,
				"lon": -97.300073,
			},
			"powerTime": 249587.0,
		},
	}

	collector.ProcessSystem(context.Background(), data, apLabels, sink)

	if sink.metrics["device_timestamp"] != 1766718625.964 {
		t.Errorf("expected device_timestamp 1766718625.964, got %v", sink.metrics["device_timestamp"])
	}
	if sink.metrics["cpu_usage"] != 24.0 {
		t.Errorf("expected cpu_usage 24.0, got %v", sink.metrics["cpu_usage"])
	}
	if sink.metrics["cpu_info"] != 1.0 {
		t.Errorf("expected cpu_info 1.0, got %v", sink.metrics["cpu_info"])
	}
	if len(sink.records["cpu_info"]) == 0 || sink.records["cpu_info"][0].labels["cpu_identifier"] != "MIPS 74Kc V4.12" {
		t.Errorf("expected cpu_identifier 'MIPS 74Kc V4.12', got %v", sink.records["cpu_info"])
	}
	if sink.metrics["ram_used_bytes"] != 37769216.0 {
		t.Errorf("expected ram_used_bytes 37769216.0, got %v", sink.metrics["ram_used_bytes"])
	}
	if sink.metrics["ap_gps_fix"] != 0.0 {
		t.Errorf("expected ap_gps_fix 0.0, got %v", sink.metrics["ap_gps_fix"])
	}
	if sink.metrics["ap_lat"] != 18.412005 {
		t.Errorf("expected ap_lat 18.412005, got %v", sink.metrics["ap_lat"])
	}
	if sink.metrics["ap_lon"] != -97.300073 {
		t.Errorf("expected ap_lon -97.300073, got %v", sink.metrics["ap_lon"])
	}
	if sink.metrics["device_temperature"] != 52.5 {
		t.Errorf("expected device_temperature 52.5, got %v", sink.metrics["device_temperature"])
	}
}

func TestProcessSystem_NilLabelsFallback(_ *testing.T) {
	sink := newMockMetricSink()
	data := map[string]any{"device": map[string]any{}}
	collector.ProcessSystem(context.Background(), data, nil, sink)
	collector.ProcessSystem(context.Background(), data, map[string]string{}, sink)
}
