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
							"firmware":        "XC.qca956x.v8.7.1.42832.201209.1309",
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
							"txRate":    56000.0,
							"rxRate":    962000.0,
							"txBytes":   100000.0,
							"rxBytes":   200000.0,
							"txPackets": 4926.0,
							"rxPackets": 5125.0,
							"txPPS":     1.0,
							"rxPPS":     2.0,
						},
					},
					"local": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"signal":         -58.0,
								"snr":            36.0,
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
								"snr":            31.0,
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
	if sink.metrics["peer_snr"] != 36.0 {
		t.Errorf("expected peer_snr 36.0, got %v", sink.metrics["peer_snr"])
	}
	if sink.metrics["peer_remote_snr"] != 31.0 {
		t.Errorf("expected peer_remote_snr 31.0, got %v", sink.metrics["peer_remote_snr"])
	}
	if sink.metrics["peer_tx_packets"] != 4926.0 {
		t.Errorf("expected peer_tx_packets 4926.0, got %v", sink.metrics["peer_tx_packets"])
	}
	if sink.metrics["peer_rx_packets"] != 5125.0 {
		t.Errorf("expected peer_rx_packets 5125.0, got %v", sink.metrics["peer_rx_packets"])
	}
	if sink.metrics["peer_tx_pps"] != 1.0 {
		t.Errorf("expected peer_tx_pps 1.0, got %v", sink.metrics["peer_tx_pps"])
	}
	if sink.metrics["peer_rx_pps"] != 2.0 {
		t.Errorf("expected peer_rx_pps 2.0, got %v", sink.metrics["peer_rx_pps"])
	}
}

func TestProcessPeers_AirMaxAC_FullPayload(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "ap-ac-sector", "ip": "10.0.4.155"}

	data := map[string]any{
		"wireless": map[string]any{
			"peers": []any{
				map[string]any{
					"common": map[string]any{
						"hostname":      "Blanca Flor Fernandez Martinez [Nativitas]",
						"mgmtIp":        "100.100.106.18",
						"uptime":        593292.0,
						"distance":      900.0,
						"powerTime":     593291.0,
						"serviceUptime": 592833.0,
						"serviceTime":   593181.0,
						"identification": map[string]any{
							"mac":             "24:5a:4c:d8:ef:74",
							"product":         "LiteBeam 5AC",
							"model":           "LBE-5AC-Gen2",
							"family":          "airmax-ac",
							"firmwareVersion": "8.7.15",
							"firmware":        "WA.ar934x.v8.7.15.47952.250103.1017",
						},
						"cpu": []any{map[string]any{"usage": 7.0}},
						"ram": map[string]any{"usage": 58.0, "free": 26607616.0, "total": 63447040.0},
						"counters": map[string]any{
							"rxRate":  23000.0,
							"txRate":  4000.0,
							"rxBytes": 51087997214.0,
							"txBytes": 3019849419.0,
						},
					},
					"local": []any{
						map[string]any{
							"id":             "main",
							"connected":      true,
							"active":         true,
							"connectionTime": 248956.0,
							"linkQuality": map[string]any{
								"airtime":             map[string]any{"dl": 10.0, "ul": 10.0},
								"nss":                 map[string]any{"dl": 2.0, "ul": 2.0},
								"capacity":            map[string]any{"dl": 145080.0, "ul": 143520.0, "dlIdeal": 156000.0, "ulIdeal": 156000.0, "combined": 144300.0, "combinedIdeal": 156000.0},
								"cinr":                map[string]any{"dl": 31.0, "ul": 34.0},
								"linkScore":           map[string]any{"dl": 92.0, "ul": 92.0, "dl24": 92.0, "ul24": 92.0},
								"signal":              -47.0,
								"idealSignal":         -53.0,
								"noiseFloor":          -90.0,
								"signalPerChain":      []any{-55.0, -54.0},
								"idealSignalPerChain": []any{-53.0, -53.0},
								"mcs": map[string]any{
									"txIdx":      8.0,
									"txIdxIdeal": 8.0,
									"rxIdx":      8.0,
									"rxIdxIdeal": 8.0,
									"rxRate":     8.0,
									"txRate":     8.0,
									"rxHistogram": []any{
										map[string]any{"idx": 7.0, "rate": 6.0, "value": 210.0},
										map[string]any{"idx": 8.0, "rate": 8.0, "value": 1755706.0},
									},
								},
							},
						},
					},
					"remote": []any{
						map[string]any{
							"id":      "main",
							"antenna": map[string]any{"gain": 23.0, "loss": 0.0},
							"linkQuality": map[string]any{
								"outputPower":         map[string]any{"conducted": 18.0, "eirp": 41.0},
								"serviceUptime":       592833.0,
								"signal":              -51.0,
								"idealSignal":         -53.0,
								"noiseFloor":          -88.0,
								"signalPerChain":      []any{-52.0, -48.0},
								"idealSignalPerChain": []any{-53.0, -53.0},
								"mcs": map[string]any{
									"rxHistogram": []any{
										map[string]any{"idx": 7.0, "rate": 6.0, "value": 733.0},
										map[string]any{"idx": 8.0, "rate": 8.0, "value": 3230936.0},
									},
								},
							},
							"gps": map[string]any{
								"fix": false,
								"lat": 18.417010,
								"lon": -97.303787,
							},
						},
					},
				},
			},
		},
	}

	collector.ProcessPeers(context.Background(), data, apLabels, sink)

	if sink.metrics["peer_remote_power"] != 18.0 {
		t.Errorf("expected peer_remote_power 18.0 from remote.linkQuality.outputPower, got %v", sink.metrics["peer_remote_power"])
	}
	if sink.metrics["peer_remote_eirp"] != 41.0 {
		t.Errorf("expected peer_remote_eirp 41.0 from remote.linkQuality.outputPower, got %v", sink.metrics["peer_remote_eirp"])
	}
	if sink.metrics["peer_snr"] != 43.0 {
		t.Errorf("expected calculated peer_snr 43.0 (-47 - -90), got %v", sink.metrics["peer_snr"])
	}
	if sink.metrics["peer_remote_snr"] != 37.0 {
		t.Errorf("expected calculated peer_remote_snr 37.0 (-51 - -88), got %v", sink.metrics["peer_remote_snr"])
	}
	if sink.metrics["peer_service_uptime"] != 592833.0 {
		t.Errorf("expected peer_service_uptime 592833.0, got %v", sink.metrics["peer_service_uptime"])
	}
	if sink.metrics["peer_service_time"] != 593181.0 {
		t.Errorf("expected peer_service_time 593181.0, got %v", sink.metrics["peer_service_time"])
	}
	if sink.metrics["peer_connected"] != 1.0 {
		t.Errorf("expected peer_connected 1.0, got %v", sink.metrics["peer_connected"])
	}
	if sink.metrics["peer_active"] != 1.0 {
		t.Errorf("expected peer_active 1.0, got %v", sink.metrics["peer_active"])
	}
	if sink.metrics["peer_ideal_signal_c0"] != -53.0 || sink.metrics["peer_ideal_signal_c1"] != -53.0 {
		t.Errorf("expected peer_ideal_signal_c0/c1 -53.0, got %v / %v", sink.metrics["peer_ideal_signal_c0"], sink.metrics["peer_ideal_signal_c1"])
	}
	if sink.metrics["peer_remote_ideal_signal_c0"] != -53.0 || sink.metrics["peer_remote_ideal_signal_c1"] != -53.0 {
		t.Errorf("expected peer_remote_ideal_signal_c0/c1 -53.0, got %v / %v", sink.metrics["peer_remote_ideal_signal_c0"], sink.metrics["peer_remote_ideal_signal_c1"])
	}
	if sink.metrics["peer_gps_fix"] != 0.0 {
		t.Errorf("expected peer_gps_fix 0.0, got %v", sink.metrics["peer_gps_fix"])
	}
	if sink.metrics["peer_tx_rate_bps"] != 4000.0 || sink.metrics["peer_rx_rate_bps"] != 23000.0 {
		t.Errorf("expected peer_tx_rate_bps=4000 / peer_rx_rate_bps=23000, got %v / %v", sink.metrics["peer_tx_rate_bps"], sink.metrics["peer_rx_rate_bps"])
	}
	if len(sink.records["peer_mcs_rx_packets"]) != 2 || sink.metrics["peer_mcs_rx_packets"] != 1755706.0 {
		t.Errorf("expected 2 local MCS histogram buckets ending with 1755706, got %v", sink.records["peer_mcs_rx_packets"])
	}
	if len(sink.records["peer_remote_mcs_rx_packets"]) != 2 || sink.metrics["peer_remote_mcs_rx_packets"] != 3230936.0 {
		t.Errorf("expected 2 remote MCS histogram buckets ending with 3230936, got %v", sink.records["peer_remote_mcs_rx_packets"])
	}
}
