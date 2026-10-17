package collector_test

import (
	"context"
	"testing"

	"ubnt-exporter/internal/collector"
)

func TestProcessWireless(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "test-ap", "ip": "10.0.0.1"}

	data := map[string]any{
		"wireless": map[string]any{
			"serviceUptime": 12060671.0,
			"radios": []any{
				map[string]any{
					"linkState": "connected",
					"frequency": map[string]any{
						"center":  5745.0,
						"control": 5745.0,
					},
					"channelWidth": map[string]any{
						"tx": 40.0,
					},
					"outputPower": map[string]any{
						"conducted": 24.0,
						"eirp":      45.0,
					},
					"dfs": map[string]any{
						"cacRemaining": 0.0,
						"lockouts":     []any{},
					},
				},
			},
			"peers": []any{
				map[string]any{
					"local": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"signal":     -55.0,
								"noiseFloor": -92.0,
								"snr":        37.0,
							},
						},
					},
				},
				map[string]any{
					"local": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"signal":     -60.0,
								"noiseFloor": -92.0,
							},
						},
					},
				},
			},
		},
	}

	collectorsConf := map[string]bool{
		"system":     false,
		"interfaces": false,
		"peers":      false,
	}

	collector.ProcessWireless(context.Background(), data, apLabels, collectorsConf, sink)

	if sink.metrics["ap_service_uptime"] != 12060671.0 {
		t.Errorf("expected ap_service_uptime 12060671.0, got %v", sink.metrics["ap_service_uptime"])
	}
	if sink.metrics["ap_link_state"] != 1.0 {
		t.Errorf("expected ap_link_state 1.0, got %v", sink.metrics["ap_link_state"])
	}
	if sink.metrics["ap_output_power"] != 24.0 {
		t.Errorf("expected ap_output_power 24.0, got %v", sink.metrics["ap_output_power"])
	}
	if sink.metrics["ap_eirp"] != 45.0 {
		t.Errorf("expected ap_eirp 45.0, got %v", sink.metrics["ap_eirp"])
	}
	if sink.metrics["ap_frequency"] != 5745.0 {
		t.Errorf("expected ap_frequency 5745.0, got %v", sink.metrics["ap_frequency"])
	}
	if sink.metrics["ap_channel_width"] != 40.0 {
		t.Errorf("expected ap_channel_width 40.0, got %v", sink.metrics["ap_channel_width"])
	}
	if sink.metrics["ap_clients"] != 2.0 {
		t.Errorf("expected ap_clients 2.0, got %v", sink.metrics["ap_clients"])
	}
	if sink.metrics["ap_signal"] != -55.0 {
		t.Errorf("expected ap_signal -55.0, got %v", sink.metrics["ap_signal"])
	}
	if sink.metrics["ap_noise"] != -92.0 {
		t.Errorf("expected ap_noise -92.0, got %v", sink.metrics["ap_noise"])
	}
	if sink.metrics["ap_snr"] != 37.0 {
		t.Errorf("expected ap_snr 37.0, got %v", sink.metrics["ap_snr"])
	}
}

func TestProcessWireless_NoPeers_RadioNoiseFallback(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "test-ap-empty", "ip": "10.0.0.10"}

	data := map[string]any{
		"wireless": map[string]any{
			"radios": []any{
				map[string]any{
					"linkState":  "connected",
					"noiseFloor": -95.0,
				},
			},
			"peers": []any{},
		},
	}

	collectorsConf := map[string]bool{
		"system":     false,
		"interfaces": false,
		"peers":      false,
	}

	collector.ProcessWireless(context.Background(), data, apLabels, collectorsConf, sink)

	if sink.metrics["ap_noise"] != -95.0 {
		t.Errorf("expected ap_noise fallback -95.0 from mainRadio, got %v", sink.metrics["ap_noise"])
	}
	if sink.metrics["ap_clients"] != 0.0 {
		t.Errorf("expected ap_clients 0.0, got %v", sink.metrics["ap_clients"])
	}
}
