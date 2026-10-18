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
	collector.ProcessPeers(context.Background(), data, apLabels, sink)

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

func TestProcessAF5XHD_RealPayload_v155(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "AP BH Altepexi Master", "ip": "10.1.1.158"}

	data := map[string]any{
		"device": map[string]any{
			"powerTime": 808998.0,
			"uptime":    808998.0,
			"timestamp": 1788022214000.0,
			"cpu": []any{
				map[string]any{"usage": 0.0, "model": "", "cores": 2.0, "frequency": 0.0},
			},
			"memory": map[string]any{
				"total": 262144000.0,
				"free":  156540928.0,
				"usage": 40.0,
			},
			"temperatures": []any{
				map[string]any{"name": "Board (PHY)", "type": "board", "value": 45.0},
			},
			"gps": map[string]any{
				"fix":             true,
				"alt":             1315.0,
				"dim":             3.0,
				"dop":             0.81,
				"lat":             18.420944,
				"lon":             -97.283978,
				"sats":            15.0,
				"timeSyncEnabled": false,
			},
			"ntpClient": map[string]any{
				"timeSyncEnabled": true,
				"lastSync":        1788020030000.0,
			},
		},
		"interfaces": []any{
			map[string]any{
				"id":      "ath0/0",
				"name":    "ath0/0",
				"enabled": true,
				"mtu":     1500.0,
				"plugged": true,
			},
		},
		"wireless": map[string]any{
			"serviceUptime": 808949.0,
			"serviceTime":   808998.0,
			"radios": []any{
				map[string]any{
					"linkState":    "connected",
					"frequency":    map[string]any{"tx": 5840.0, "rx": 5215.0},
					"frameLength":  2.0,
					"gpsSyncState": 1.0,
					"channelWidth": map[string]any{"tx": 50.0, "rx": 50.0},
					"outputPower":  map[string]any{"conducted": 24.0, "eirp": 54.0, "autoPower": false},
					"noiseFloor":   -92.0,
					"dlRatio":      50.0,
					"rxEfficiency": 84.9,
					"dfs":          map[string]any{"enabled": false, "status": 0.0, "cacRemaining": 0.0, "lockouts": []any{}},
				},
			},
			"peers": []any{
				map[string]any{
					"common": map[string]any{
						"hostname":      "Station BH Altepexi Slave",
						"mgmtIp":        "10.1.1.159",
						"distance":      20229.0,
						"uptime":        808222.0,
						"powerTime":     808253.0,
						"serviceUptime": 808224.0,
						"serviceTime":   808253.0,
						"connected":     true,
						"active":        true,
						"identification": map[string]any{
							"mac":             "78:45:58:78:15:3C",
							"product":         "airFiber 5XHD",
							"model":           "airFiber 5XHD",
							"family":          "ltu",
							"firmwareVersion": "1.5.5",
							"firmware":        "af5xhd.amesoc3.v1.5.5.00002.251003.1210",
							"subsystemID":     "ae03",
							"countryCode":     "XH",
						},
						"netMode": "bridge",
						"counters": map[string]any{
							"txBytes":   4123615303497.0,
							"rxBytes":   527366855666.0,
							"txPackets": 2972742310.0,
							"rxPackets": 1277620737.0,
							"txRate":    68416048.0,
							"rxRate":    6706872.0,
							"txPPS":     6261.0,
							"rxPPS":     2739.0,
						},
					},
					"local": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"signal":              -43.0,
								"signalPerChain":      []any{-46.0, -45.0},
								"idealSignal":         -45.0,
								"idealSignalPerChain": []any{-48.0, -48.0},
								"capacity": map[string]any{
									"dl":            268290.0,
									"ul":            254200.0,
									"combined":      522490.0,
									"combinedIdeal": 522490.0,
								},
								"modulation": map[string]any{
									"tx":      7.0,
									"rx":      7.0,
									"idealTx": 7.0,
									"idealRx": 7.0,
									"txRate":  8.0,
									"rxRate":  8.0,
								},
								"linkScore":    map[string]any{"dl": 100.0, "ul": 94.8},
								"timeFraction": map[string]any{"rx": 6.4, "tx": 28.0},
								"rxHistogram": []any{
									map[string]any{"rate": 1.0, "value": 0.0},
									map[string]any{"rate": 8.0, "value": 1277620000.0},
								},
							},
						},
					},
					"remote": []any{
						map[string]any{
							"gpsSyncState": 0.0,
							"rxEfficiency": 16.23,
							"netMode":      "bridge",
							"gps": map[string]any{
								"fix":             true,
								"alt":             2108.0,
								"dim":             3.0,
								"dop":             1.12,
								"lat":             18.569470,
								"lon":             -97.380910,
								"sats":            9.0,
								"timeSyncEnabled": false,
							},
							"ntpClient": map[string]any{
								"timeSyncEnabled": true,
								"lastSync":        1788019813000.0,
							},
							"linkQuality": map[string]any{
								"signal":              -55.0,
								"signalPerChain":      []any{-57.0, -59.0},
								"idealSignal":         -45.0,
								"idealSignalPerChain": []any{-48.0, -48.0},
								"noiseFloor":          -96.0,
								"outputPower": map[string]any{
									"conducted": 24.0,
									"eirp":      54.0,
									"autoPower": false,
								},
								"rxHistogram": []any{
									map[string]any{"rate": 1.0, "value": 0.0},
									map[string]any{"rate": 8.0, "value": 2972740000.0},
								},
							},
						},
					},
				},
			},
		},
	}

	collector.ProcessSystem(context.Background(), data, apLabels, sink)
	collector.ProcessInterfaces(context.Background(), data, apLabels, sink)
	collector.ProcessAF5XHD(context.Background(), data, apLabels, nil, sink)
	collector.ProcessPeers(context.Background(), data, apLabels, sink)

	// 1. Remote outputPower read from remote[0].linkQuality.outputPower
	if sink.metrics["af_remote_conducted"] != 24.0 {
		t.Errorf("expected af_remote_conducted 24.0, got %v", sink.metrics["af_remote_conducted"])
	}
	if sink.metrics["af_remote_eirp"] != 54.0 {
		t.Errorf("expected af_remote_eirp 54.0, got %v", sink.metrics["af_remote_eirp"])
	}
	if sink.metrics["af_remote_autopower"] != 0.0 {
		t.Errorf("expected af_remote_autopower 0.0, got %v", sink.metrics["af_remote_autopower"])
	}

	// 2. Fallback to wireless.radios[0].noiseFloor (-92) for ap_snr and peer_snr (-43 - (-92) = 49)
	if sink.metrics["ap_snr"] != 49.0 {
		t.Errorf("expected ap_snr 49.0, got %v", sink.metrics["ap_snr"])
	}
	if sink.metrics["peer_noise_floor"] != -92.0 {
		t.Errorf("expected peer_noise_floor -92.0, got %v", sink.metrics["peer_noise_floor"])
	}
	if sink.metrics["peer_snr"] != 49.0 {
		t.Errorf("expected peer_snr 49.0, got %v", sink.metrics["peer_snr"])
	}

	// 3. rxHistogram without idx (index 1 -> rate 8.0)
	if sink.metrics["peer_mcs_rx_packets"] != 1277620000.0 {
		t.Errorf("expected peer_mcs_rx_packets 1277620000.0, got %v", sink.metrics["peer_mcs_rx_packets"])
	}
	if sink.metrics["peer_remote_mcs_rx_packets"] != 2972740000.0 {
		t.Errorf("expected peer_remote_mcs_rx_packets 2972740000.0, got %v", sink.metrics["peer_remote_mcs_rx_packets"])
	}

	// 4. Local & Remote GPS and NTP
	if sink.metrics["device_gps_dim"] != 3.0 {
		t.Errorf("expected device_gps_dim 3.0, got %v", sink.metrics["device_gps_dim"])
	}
	if sink.metrics["device_ntp_sync_enabled"] != 1.0 {
		t.Errorf("expected device_ntp_sync_enabled 1.0, got %v", sink.metrics["device_ntp_sync_enabled"])
	}
	if sink.metrics["device_ntp_last_sync"] != 1788020030.0 {
		t.Errorf("expected device_ntp_last_sync 1788020030.0, got %v", sink.metrics["device_ntp_last_sync"])
	}
	if sink.metrics["peer_gps_alt"] != 2108.0 {
		t.Errorf("expected peer_gps_alt 2108.0, got %v", sink.metrics["peer_gps_alt"])
	}
	if sink.metrics["peer_gps_sats"] != 9.0 {
		t.Errorf("expected peer_gps_sats 9.0, got %v", sink.metrics["peer_gps_sats"])
	}
	if sink.metrics["peer_gps_dop"] != 1.12 {
		t.Errorf("expected peer_gps_dop 1.12, got %v", sink.metrics["peer_gps_dop"])
	}
	if sink.metrics["peer_gps_dim"] != 3.0 {
		t.Errorf("expected peer_gps_dim 3.0, got %v", sink.metrics["peer_gps_dim"])
	}
	if sink.metrics["peer_ntp_sync_enabled"] != 1.0 {
		t.Errorf("expected peer_ntp_sync_enabled 1.0, got %v", sink.metrics["peer_ntp_sync_enabled"])
	}
	if sink.metrics["peer_ntp_last_sync"] != 1788019813.0 {
		t.Errorf("expected peer_ntp_last_sync 1788019813.0, got %v", sink.metrics["peer_ntp_last_sync"])
	}
	if sink.metrics["peer_remote_rx_efficiency"] != 16.23 {
		t.Errorf("expected peer_remote_rx_efficiency 16.23, got %v", sink.metrics["peer_remote_rx_efficiency"])
	}

	// 5. Peer info extended labels & ath0/0 interface classification
	peerInfoRecs := sink.records["peer_info"]
	if len(peerInfoRecs) != 1 {
		t.Fatalf("expected 1 peer_info record, got %d", len(peerInfoRecs))
	}
	pLabels := peerInfoRecs[0].labels
	if pLabels["peer_family"] != "ltu" {
		t.Errorf("expected peer_family=ltu, got %q", pLabels["peer_family"])
	}
	if pLabels["peer_subsystem_id"] != "ae03" {
		t.Errorf("expected peer_subsystem_id=ae03, got %q", pLabels["peer_subsystem_id"])
	}
	if pLabels["peer_country_code"] != "XH" {
		t.Errorf("expected peer_country_code=XH, got %q", pLabels["peer_country_code"])
	}
	if pLabels["peer_net_mode"] != "bridge" {
		t.Errorf("expected peer_net_mode=bridge, got %q", pLabels["peer_net_mode"])
	}

	ifaceInfoRecs := sink.records["iface_info"]
	if len(ifaceInfoRecs) != 1 {
		t.Fatalf("expected 1 iface_info record, got %d", len(ifaceInfoRecs))
	}
	iLabels := ifaceInfoRecs[0].labels
	if iLabels["interface_type"] != "wireless" || iLabels["parent_interface"] != "ath0" {
		t.Errorf("expected ath0/0 classified as wireless with parent ath0, got %v", iLabels)
	}
}

func TestProcessAF5XHD_EdgeCases(t *testing.T) {
	sink := newMockMetricSink()
	ctx := context.Background()
	apLabels := map[string]string{"name": "AF-Edge", "ip": "10.0.0.1"}
	enabled := map[string]bool{"airfiber": true}

	// Case 1: Missing device and wireless objects
	collector.ProcessAF5XHD(ctx, map[string]any{}, apLabels, enabled, sink)

	// Case 2: Incomplete payload exercising fallbacks
	data := map[string]any{
		"device": map[string]any{
			"temperatures": []any{},
			"gps":          map[string]any{"fix": false},
		},
		"wireless": map[string]any{
			"radios": []any{
				map[string]any{
					"linkState":  "disconnected",
					"noiseFloor": -98.0,
					"frequency": map[string]any{
						"rx": 5215.0,
					},
					"dfs": map[string]any{
						"cacRemaining": 0.0,
					},
				},
			},
			"peers": []any{
				map[string]any{
					"common": nil,
					"remote": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"outputPower": map[string]any{
									"conducted": 19.0,
									"autoPower": true,
								},
							},
						},
					},
					"local": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"signal": -50.0,
								"linkScore": map[string]any{
									"dl": 90.0,
									"ul": 80.0,
								},
								"modulation": map[string]any{
									"txIdx": 8.0,
									"rxIdx": 8.0,
								},
							},
						},
					},
				},
			},
		},
	}

	collector.ProcessAF5XHD(ctx, data, apLabels, enabled, sink)

	if sink.metrics["af_link_state"] != 0.0 {
		t.Errorf("expected af_link_state 0.0 for disconnected, got %v", sink.metrics["af_link_state"])
	}
	if sink.metrics["ap_frequency"] != 5215.0 {
		t.Errorf("expected ap_frequency 5215.0 from rx fallback, got %v", sink.metrics["ap_frequency"])
	}
	if sink.metrics["af_link_score"] != 85.0 {
		t.Errorf("expected af_link_score 85.0 from average, got %v", sink.metrics["af_link_score"])
	}
	if sink.metrics["af_remote_conducted"] != 19.0 {
		t.Errorf("expected af_remote_conducted 19.0 from linkQuality, got %v", sink.metrics["af_remote_conducted"])
	}
}
