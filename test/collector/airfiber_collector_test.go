package collector_test

import (
	"context"
	"math"
	"testing"

	"ubnt-exporter/internal/collector"
)

func TestProcessAF5XHD(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "test-af", "ip": "10.0.0.2"}

	data := map[string]any{
		"device": map[string]any{
			"temperatures": []any{
				map[string]any{
					"value": 48.0,
				},
			},
			"gps": map[string]any{
				"fix":  true,
				"sats": 11.0,
				"dop":  1.2,
				"alt":  450.0,
				"lat":  -34.6037,
				"lon":  -58.3816,
			},
		},
		"wireless": map[string]any{
			"serviceUptime": 86400.0,
			"radios": []any{
				map[string]any{
					"linkState": "connected",
					"frequency": map[string]any{
						"tx": 5200.0,
						"rx": 5300.0,
					},
					"frameLength":  2.5,
					"gpsSyncState": 1.0,
					"channelWidth": map[string]any{
						"tx": 50.0,
						"rx": 50.0,
					},
					"outputPower": map[string]any{
						"conducted": 19.0,
						"eirp":      49.0,
						"autoPower": true,
					},
					"noiseFloor":   -92.0,
					"dlRatio":      67.0,
					"rxEfficiency": 99.5,
					"dfs": map[string]any{
						"cacRemaining": 0.0,
						"lockouts":     []any{},
					},
				},
			},
			"peers": []any{
				map[string]any{
					"common": map[string]any{
						"hostname": "remote-af",
						"mgmtIp":   "10.0.0.3",
						"distance": 8100.0,
						"uptime":   43200.0,
						"identification": map[string]any{
							"mac":             "00:27:22:11:22:33",
							"product":         "airFiber",
							"model":           "AF-5XHD",
							"family":          "airFiber",
							"firmwareVersion": "v1.5.4",
						},
						"counters": map[string]any{
							"txBytes": 1000000.0,
							"rxBytes": 2000000.0,
							"txRate":  10500000.0,
							"rxRate":  12000000.0,
							"txPPS":   500.0,
							"rxPPS":   600.0,
						},
					},
					"remote": []any{
						map[string]any{
							"gpsSyncState": 0.0,
							"linkQuality": map[string]any{
								"signal":         -55.0,
								"idealSignal":    -48.0,
								"noiseFloor":     -94.0,
								"signalPerChain": []any{-55.0, -56.0},
							},
						},
					},
					"local": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"signal":         -55.0,
								"idealSignal":    -48.0,
								"noiseFloor":     -94.0,
								"signalPerChain": []any{-55.0, -56.0},
								"capacity": map[string]any{
									"dl":            294400.0,
									"ul":            294400.0,
									"combined":      588800.0,
									"combinedIdeal": 588800.0,
								},
								"modulation": map[string]any{
									"txRate": 10.5,
									"rxRate": 12.0,
								},
								"linkScore": map[string]any{
									"dl": 96.0,
									"ul": 98.0,
								},
								"timeFraction": map[string]any{
									"rx": 50.0,
									"tx": 50.0,
								},
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
	}

	collector.ProcessAF5XHD(context.Background(), data, apLabels, collectorsConf, sink)

	if sink.metrics["af_service_uptime"] != 86400.0 {
		t.Errorf("expected af_service_uptime 86400.0, got %v", sink.metrics["af_service_uptime"])
	}
	if sink.metrics["af_link_state"] != 1.0 {
		t.Errorf("expected af_link_state 1.0, got %v", sink.metrics["af_link_state"])
	}
	if sink.metrics["af_temperature"] != 48.0 {
		t.Errorf("expected af_temperature 48.0, got %v", sink.metrics["af_temperature"])
	}
	if sink.metrics["af_gps_sats"] != 11.0 {
		t.Errorf("expected af_gps_sats 11.0, got %v", sink.metrics["af_gps_sats"])
	}
	if sink.metrics["af_gps_sync_state"] != 1.0 {
		t.Errorf("expected af_gps_sync_state 1.0, got %v", sink.metrics["af_gps_sync_state"])
	}
	if sink.metrics["af_frame_length"] != 2.5 {
		t.Errorf("expected af_frame_length 2.5, got %v", sink.metrics["af_frame_length"])
	}
	if sink.metrics["af_frequency_tx"] != 5200.0 {
		t.Errorf("expected af_frequency_tx 5200.0, got %v", sink.metrics["af_frequency_tx"])
	}
	if sink.metrics["af_frequency_rx"] != 5300.0 {
		t.Errorf("expected af_frequency_rx 5300.0, got %v", sink.metrics["af_frequency_rx"])
	}
	if sink.metrics["ap_channel_width"] != 50.0 {
		t.Errorf("expected ap_channel_width 50.0, got %v", sink.metrics["ap_channel_width"])
	}
	if sink.metrics["af_channel_width_rx"] != 50.0 {
		t.Errorf("expected af_channel_width_rx 50.0, got %v", sink.metrics["af_channel_width_rx"])
	}
	if sink.metrics["ap_eirp"] != 49.0 {
		t.Errorf("expected ap_eirp 49.0, got %v", sink.metrics["ap_eirp"])
	}
	if sink.metrics["af_local_autopower"] != 1.0 {
		t.Errorf("expected af_local_autopower 1.0, got %v", sink.metrics["af_local_autopower"])
	}
	if sink.metrics["ap_clients"] != 1.0 {
		t.Errorf("expected ap_clients 1.0, got %v", sink.metrics["ap_clients"])
	}
	if sink.metrics["af_dfs_status"] != 0.0 {
		t.Errorf("expected af_dfs_status 0.0, got %v", sink.metrics["af_dfs_status"])
	}
	if sink.metrics["af_remote_gps_sync_state"] != 0.0 {
		t.Errorf("expected af_remote_gps_sync_state 0.0, got %v", sink.metrics["af_remote_gps_sync_state"])
	}
	if sink.metrics["af_remote_ideal_signal"] != -48.0 {
		t.Errorf("expected af_remote_ideal_signal -48.0, got %v", sink.metrics["af_remote_ideal_signal"])
	}
	if sink.metrics["peer_remote_signal"] != -55.0 {
		t.Errorf("expected peer_remote_signal -55.0, got %v", sink.metrics["peer_remote_signal"])
	}
	if sink.metrics["peer_remote_noise"] != -94.0 {
		t.Errorf("expected peer_remote_noise -94.0, got %v", sink.metrics["peer_remote_noise"])
	}
	if sink.metrics["peer_remote_imbalance"] != 1.0 {
		t.Errorf("expected peer_remote_imbalance 1.0, got %v", sink.metrics["peer_remote_imbalance"])
	}
	if sink.metrics["af_ideal_signal"] != -48.0 {
		t.Errorf("expected af_ideal_signal -48.0, got %v", sink.metrics["af_ideal_signal"])
	}
	if sink.metrics["af_signal_delta_ideal"] != -7.0 {
		t.Errorf("expected af_signal_delta_ideal -7.0, got %v", sink.metrics["af_signal_delta_ideal"])
	}
	if sink.metrics["af_capacity_combined"] != 588800000.0 {
		t.Errorf("expected af_capacity_combined 588800000.0, got %v", sink.metrics["af_capacity_combined"])
	}
	if sink.metrics["af_capacity_combined_ideal"] != 588800000.0 {
		t.Errorf("expected af_capacity_combined_ideal 588800000.0, got %v", sink.metrics["af_capacity_combined_ideal"])
	}
	if math.Abs(sink.metrics["peer_capacity_dl"]-294.4) > 1e-4 {
		t.Errorf("expected peer_capacity_dl 294.4, got %v", sink.metrics["peer_capacity_dl"])
	}
	if math.Abs(sink.metrics["peer_capacity_ul"]-294.4) > 1e-4 {
		t.Errorf("expected peer_capacity_ul 294.4, got %v", sink.metrics["peer_capacity_ul"])
	}
	if sink.metrics["peer_tx_rate"] != 10.5 {
		t.Errorf("expected peer_tx_rate 10.5, got %v", sink.metrics["peer_tx_rate"])
	}
	if sink.metrics["peer_rx_rate"] != 12.0 {
		t.Errorf("expected peer_rx_rate 12.0, got %v", sink.metrics["peer_rx_rate"])
	}
	if sink.metrics["peer_uptime"] != 43200.0 {
		t.Errorf("expected peer_uptime 43200.0, got %v", sink.metrics["peer_uptime"])
	}
	if math.Abs(sink.metrics["peer_capacity_combined"]-588.8) > 1e-4 {
		t.Errorf("expected peer_capacity_combined 588.8, got %v", sink.metrics["peer_capacity_combined"])
	}
	if sink.metrics["peer_ideal_signal"] != -48.0 {
		t.Errorf("expected peer_ideal_signal -48.0, got %v", sink.metrics["peer_ideal_signal"])
	}
	if sink.metrics["peer_signal_delta_ideal"] != -7.0 {
		t.Errorf("expected peer_signal_delta_ideal -7.0, got %v", sink.metrics["peer_signal_delta_ideal"])
	}
	if sink.metrics["af_link_score"] != 97.0 {
		t.Errorf("expected af_link_score 97.0, got %v", sink.metrics["af_link_score"])
	}
	if sink.metrics["af_modulation_rate"] != 12.0 {
		t.Errorf("expected af_modulation_rate 12.0, got %v", sink.metrics["af_modulation_rate"])
	}
}
