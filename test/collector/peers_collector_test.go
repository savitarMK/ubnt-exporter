package collector_test

import (
	"context"
	"math"
	"testing"

	"ubnt-exporter/internal/collector"
)

func TestProcessPeers(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "test-ap", "ip": "10.0.0.1"}

	data := map[string]any{
		"wireless": map[string]any{
			"peers": []any{
				map[string]any{
					"common": map[string]any{
						"hostname":  "station-alpha",
						"mgmtIp":    "10.0.0.50",
						"distance":  2500.0,
						"uptime":    86400.0,
						"powerTime": 86399.0,
						"identification": map[string]any{
							"mac":             "00:27:22:AA:BB:CC",
							"product":         "PowerBeam",
							"model":           "PBE-5AC-Gen2",
							"family":          "AirMAX AC",
							"firmwareVersion": "8.7.1",
						},
						"cpu": []any{
							map[string]any{"usage": 15.0},
						},
						"ram": map[string]any{
							"usage": 45.0,
							"free":  32000000.0,
							"total": 64000000.0,
						},
						"counters": map[string]any{
							"txRate":  56000.0,
							"rxRate":  962000.0,
							"txBytes": 100000.0,
							"rxBytes": 200000.0,
						},
					},
					"local": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"signal":         -58.0,
								"idealSignal":    -55.0,
								"noiseFloor":     -94.0,
								"signalPerChain": []any{-58.0, -60.0},
								"capacity": map[string]any{
									"dl":            150000.0,
									"ul":            80000.0,
									"dlIdeal":       160000.0,
									"ulIdeal":       90000.0,
									"combined":      230000.0,
									"combinedIdeal": 250000.0,
								},
								"airtime": map[string]any{
									"dl": 600.0,
									"ul": 40.0,
								},
								"mcs": map[string]any{
									"txIdx":      5.0,
									"rxIdx":      6.0,
									"txIdxIdeal": 8.0,
									"rxIdxIdeal": 8.0,
									"txRate":     6.0,
									"rxRate":     6.0,
								},
							},
						},
					},
					"remote": []any{
						map[string]any{
							"antenna": map[string]any{
								"gain": 23.0,
								"loss": 1.0,
							},
							"linkQuality": map[string]any{
								"signal":         -62.0,
								"idealSignal":    -56.0,
								"noiseFloor":     -93.0,
								"signalPerChain": []any{-62.0, -63.0},
							},
						},
					},
				},
			},
		},
	}

	collector.ProcessPeers(context.Background(), data, apLabels, sink)

	if sink.metrics["peer_signal"] != -58.0 {
		t.Errorf("expected peer_signal -58.0, got %v", sink.metrics["peer_signal"])
	}
	if sink.metrics["peer_ideal_signal"] != -55.0 {
		t.Errorf("expected peer_ideal_signal -55.0, got %v", sink.metrics["peer_ideal_signal"])
	}
	if sink.metrics["peer_signal_delta_ideal"] != -3.0 {
		t.Errorf("expected peer_signal_delta_ideal -3.0, got %v", sink.metrics["peer_signal_delta_ideal"])
	}
	if sink.metrics["peer_remote_signal"] != -62.0 {
		t.Errorf("expected peer_remote_signal -62.0, got %v", sink.metrics["peer_remote_signal"])
	}
	if sink.metrics["peer_remote_ideal_signal"] != -56.0 {
		t.Errorf("expected peer_remote_ideal_signal -56.0, got %v", sink.metrics["peer_remote_ideal_signal"])
	}
	if sink.metrics["peer_remote_signal_delta_ideal"] != -6.0 {
		t.Errorf("expected peer_remote_signal_delta_ideal -6.0, got %v", sink.metrics["peer_remote_signal_delta_ideal"])
	}
	if sink.metrics["peer_cpu"] != 15.0 {
		t.Errorf("expected peer_cpu 15.0, got %v", sink.metrics["peer_cpu"])
	}
	if sink.metrics["peer_ram"] != 45.0 {
		t.Errorf("expected peer_ram 45.0, got %v", sink.metrics["peer_ram"])
	}
	if sink.metrics["peer_ram_free"] != 32000000.0 {
		t.Errorf("expected peer_ram_free 32000000.0, got %v", sink.metrics["peer_ram_free"])
	}
	if sink.metrics["peer_ram_total"] != 64000000.0 {
		t.Errorf("expected peer_ram_total 64000000.0, got %v", sink.metrics["peer_ram_total"])
	}
	if sink.metrics["peer_power_time"] != 86399.0 {
		t.Errorf("expected peer_power_time 86399.0, got %v", sink.metrics["peer_power_time"])
	}
	if sink.metrics["peer_signal_c0"] != -58.0 {
		t.Errorf("expected peer_signal_c0 -58.0, got %v", sink.metrics["peer_signal_c0"])
	}
	if sink.metrics["peer_signal_c1"] != -60.0 {
		t.Errorf("expected peer_signal_c1 -60.0, got %v", sink.metrics["peer_signal_c1"])
	}
	if sink.metrics["peer_chain_imbalance"] != 2.0 {
		t.Errorf("expected peer_chain_imbalance 2.0, got %v", sink.metrics["peer_chain_imbalance"])
	}
	if sink.metrics["peer_distance"] != 2500.0 {
		t.Errorf("expected peer_distance 2500.0, got %v", sink.metrics["peer_distance"])
	}
	if sink.metrics["peer_capacity_dl"] != 150.0 {
		t.Errorf("expected peer_capacity_dl 150.0, got %v", sink.metrics["peer_capacity_dl"])
	}
	if sink.metrics["peer_capacity_combined"] != 230.0 {
		t.Errorf("expected peer_capacity_combined 230.0, got %v", sink.metrics["peer_capacity_combined"])
	}
	if sink.metrics["peer_capacity_combined_ideal"] != 250.0 {
		t.Errorf("expected peer_capacity_combined_ideal 250.0, got %v", sink.metrics["peer_capacity_combined_ideal"])
	}
	if sink.metrics["peer_mcs_ideal_tx"] != 8.0 {
		t.Errorf("expected peer_mcs_ideal_tx 8.0, got %v", sink.metrics["peer_mcs_ideal_tx"])
	}
	if sink.metrics["peer_mcs_ideal_rx"] != 8.0 {
		t.Errorf("expected peer_mcs_ideal_rx 8.0, got %v", sink.metrics["peer_mcs_ideal_rx"])
	}
	if sink.metrics["peer_mcs_rate_tx"] != 6.0 {
		t.Errorf("expected peer_mcs_rate_tx 6.0, got %v", sink.metrics["peer_mcs_rate_tx"])
	}
	if sink.metrics["peer_mcs_rate_rx"] != 6.0 {
		t.Errorf("expected peer_mcs_rate_rx 6.0, got %v", sink.metrics["peer_mcs_rate_rx"])
	}
	if sink.metrics["peer_airtime"] != 60.0 {
		t.Errorf("expected peer_airtime 60.0, got %v", sink.metrics["peer_airtime"])
	}
	if sink.metrics["peer_airtime_ul"] != 4.0 {
		t.Errorf("expected peer_airtime_ul 4.0, got %v", sink.metrics["peer_airtime_ul"])
	}
	if sink.metrics["peer_remote_cable_loss"] != 1.0 {
		t.Errorf("expected peer_remote_cable_loss 1.0, got %v", sink.metrics["peer_remote_cable_loss"])
	}
	if math.Abs(sink.metrics["peer_tx_rate"]-0.056) > 1e-6 {
		t.Errorf("expected peer_tx_rate 0.056, got %v", sink.metrics["peer_tx_rate"])
	}
	if math.Abs(sink.metrics["peer_rx_rate"]-0.962) > 1e-6 {
		t.Errorf("expected peer_rx_rate 0.962, got %v", sink.metrics["peer_rx_rate"])
	}
	if sink.metrics["peer_info"] != 1.0 {
		t.Errorf("expected peer_info 1.0, got %v", sink.metrics["peer_info"])
	}
}
