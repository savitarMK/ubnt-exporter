// Package collector extracts AirFiber (AF5XHD) specific metrics.
package collector

import (
	"context"
	"math"
	"strings"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

// ProcessAF5XHD processes metrics from an AirFiber device.
func ProcessAF5XHD(
	ctx context.Context,
	dataRoot map[string]any,
	apLabels map[string]string,
	collectorsConf map[string]bool,
	store metrics.MetricSink,
) {
	devName := getDeviceName(apLabels)

	if isCollectorEnabled(collectorsConf, "system") {
		runCollectorTimed("system", func() {
			ProcessSystem(ctx, dataRoot, apLabels, store)
		})
	}

	if isCollectorEnabled(collectorsConf, "interfaces") {
		runCollectorTimed("interfaces", func() {
			ProcessInterfaces(ctx, dataRoot, apLabels, store)
		})
	}

	if isCollectorEnabled(collectorsConf, "airfiber") {
		runCollectorTimed("airfiber", func() {
			device, okDev := dataRoot["device"].(map[string]any)
			if !okDev || device == nil {
				device = make(map[string]any)
				logger.DebugContext(ctx, devName, "Collector 'airfiber': 'device' payload missing in status data")
			}

			wireless, okWireless := dataRoot["wireless"].(map[string]any)
			if !okWireless || wireless == nil {
				wireless = make(map[string]any)
				logger.DebugContext(ctx, devName, "Collector 'airfiber': 'wireless' payload missing in status data")
			}

			radios, okRadios := wireless["radios"].([]any)
			if !okRadios || len(radios) == 0 {
				logger.DebugContext(ctx, devName, "Collector 'airfiber': 'radios' list missing or empty in wireless data")
			}
			peers, _ := wireless["peers"].([]any)

			addMetric := func(key string, src map[string]any, srcKey string, labels map[string]string, scale ...float64) {
				if src == nil {
					return
				}
				if val, exists := src[srcKey]; exists && val != nil {
					s := 1.0
					if len(scale) > 0 {
						s = scale[0]
					}
					store.UpdateMetric(key, labels, utils.SafeFloat(val)*s)
				}
			}

			addMetric("af_service_uptime", wireless, "serviceUptime", apLabels)

			// Temperatures
			if temps, ok := device["temperatures"].([]any); ok && len(temps) > 0 {
				if t0, ok := temps[0].(map[string]any); ok {
					addMetric("af_temperature", t0, "value", apLabels)
				}
			} else {
				logger.DebugContext(ctx, devName, "Collector 'airfiber': 'temperatures' list missing or empty in device data")
			}

			// GPS
			if gps, ok := device["gps"].(map[string]any); ok {
				if gps["fix"] == true {
					addMetric("af_gps_sats", gps, "sats", apLabels)
					addMetric("af_gps_dop", gps, "dop", apLabels)
					addMetric("af_gps_alt", gps, "alt", apLabels)
					addMetric("ap_lat", gps, "lat", apLabels)
					addMetric("ap_lon", gps, "lon", apLabels)
				} else {
					logger.DebugContext(ctx, devName, "Collector 'airfiber': GPS lock not acquired (fix=false)")
				}
			} else {
				logger.DebugContext(ctx, devName, "Collector 'airfiber': 'gps' telemetry missing in device data")
			}

			// Radios
			if len(radios) > 0 {
				if radio, ok := radios[0].(map[string]any); ok {
					if ls, ok := radio["linkState"].(string); ok {
						stateVal := 0.0
						if strings.ToLower(ls) == "connected" {
							stateVal = 1.0
						}
						store.UpdateMetric("af_link_state", apLabels, stateVal)
					}

					freqData, _ := radio["frequency"].(map[string]any)
					if freqData != nil {
						freqVal := freqData["tx"]
						if freqVal == nil {
							freqVal = freqData["rx"]
						}
						if freqVal != nil {
							f := utils.SafeFloat(freqVal)
							store.UpdateMetric("ap_frequency", apLabels, f)
							store.UpdateMetric("ap_control_frequency", apLabels, f)
						}
						addMetric("af_frequency_tx", freqData, "tx", apLabels)
						addMetric("af_frequency_rx", freqData, "rx", apLabels)
					}

					addMetric("af_frame_length", radio, "frameLength", apLabels)
					addMetric("af_gps_sync_state", radio, "gpsSyncState", apLabels)

					if width, ok := radio["channelWidth"].(map[string]any); ok {
						addMetric("ap_channel_width", width, "tx", apLabels)
						addMetric("af_channel_width_rx", width, "rx", apLabels)
					}

					if pwrData, ok := radio["outputPower"].(map[string]any); ok {
						addMetric("ap_output_power", pwrData, "conducted", apLabels)
						addMetric("ap_eirp", pwrData, "eirp", apLabels)
						autoPwr := 0.0
						if pwrData["autoPower"] == true {
							autoPwr = 1.0
						}
						store.UpdateMetric("af_local_autopower", apLabels, autoPwr)
					}

					addMetric("ap_noise", radio, "noiseFloor", apLabels)
					addMetric("af_dl_ratio", radio, "dlRatio", apLabels)
					addMetric("af_rx_efficiency", radio, "rxEfficiency", apLabels)

					if dfs, ok := radio["dfs"].(map[string]any); ok {
						cacRem := utils.SafeFloat(dfs["cacRemaining"])
						store.UpdateMetric("af_dfs_cac_rem", apLabels, cacRem)
						dfsStatus := 0.0
						if cacRem > 0 {
							dfsStatus = 1.0
						}
						store.UpdateMetric("af_dfs_status", apLabels, dfsStatus)
						if lockouts, ok := dfs["lockouts"].([]any); ok {
							store.UpdateMetric("af_dfs_lockout_count", apLabels, float64(len(lockouts)))
						} else {
							store.UpdateMetric("af_dfs_lockout_count", apLabels, 0.0)
						}
					}
				}
			}

			store.UpdateMetric("ap_clients", apLabels, float64(len(peers)))

			// Peers
			if len(peers) > 0 {
				if peer, ok := peers[0].(map[string]any); ok {
					common, _ := peer["common"].(map[string]any)
					if common == nil {
						common = make(map[string]any)
					}

					peerName := utils.CleanString(common["hostname"])
					peerLabels := make(map[string]string, len(apLabels)+4)
					for k, v := range apLabels {
						peerLabels[k] = v
					}
					peerLabels["peer_name"] = peerName
					peerLabels["peer_ip"] = utils.CleanString(common["mgmtIp"])
					apName := apLabels["name"]
					if apName == "" {
						apName = "N/A"
					}
					apIP := apLabels["ip"]
					if apIP == "" {
						apIP = "N/A"
					}
					peerLabels["ap_name"] = apName
					peerLabels["ap_ip"] = apIP

					addMetric("peer_distance", common, "distance", peerLabels)
					addMetric("peer_uptime", common, "uptime", peerLabels)
					addMetric("peer_power_time", common, "powerTime", peerLabels)

					if peerCPU, ok := common["cpu"].([]any); ok && len(peerCPU) > 0 {
						if cpu0, ok := peerCPU[0].(map[string]any); ok {
							addMetric("af_peer_cpu", cpu0, "usage", peerLabels)
							addMetric("peer_cpu", cpu0, "usage", peerLabels)
						}
					}

					if peerRAM, ok := common["ram"].(map[string]any); ok {
						addMetric("af_peer_ram", peerRAM, "usage", peerLabels)
						addMetric("peer_ram", peerRAM, "usage", peerLabels)
						addMetric("peer_ram_free", peerRAM, "free", peerLabels)
						addMetric("peer_ram_total", peerRAM, "total", peerLabels)
					}

					if counters, ok := common["counters"].(map[string]any); ok {
						addMetric("af_peer_tx_bytes", counters, "txBytes", peerLabels)
						addMetric("af_peer_rx_bytes", counters, "rxBytes", peerLabels)
						addMetric("peer_tx_bytes", counters, "txBytes", peerLabels)
						addMetric("peer_rx_bytes", counters, "rxBytes", peerLabels)
						addMetric("af_peer_tx_rate", counters, "txRate", peerLabels)
						addMetric("af_peer_rx_rate", counters, "rxRate", peerLabels)
						addMetric("peer_tx_rate", counters, "txRate", peerLabels, 0.000001)
						addMetric("peer_rx_rate", counters, "rxRate", peerLabels, 0.000001)
						addMetric("af_peer_tx_pps", counters, "txPPS", peerLabels)
						addMetric("af_peer_rx_pps", counters, "rxPPS", peerLabels)
						addMetric("peer_tx_pps", counters, "txPPS", peerLabels)
						addMetric("peer_rx_pps", counters, "rxPPS", peerLabels)
						addMetric("peer_tx_packets", counters, "txPackets", peerLabels)
						addMetric("peer_rx_packets", counters, "rxPackets", peerLabels)
					}

					// Remote
					if remoteList, ok := peer["remote"].([]any); ok && len(remoteList) > 0 {
						if remoteData, ok := remoteList[0].(map[string]any); ok {
							addMetric("af_remote_rx_eff", remoteData, "rxEfficiency", apLabels)
							addMetric("af_remote_gps_sync_state", remoteData, "gpsSyncState", apLabels)

							if remAnt, ok := remoteData["antenna"].(map[string]any); ok {
								addMetric("af_remote_gain", remAnt, "gain", apLabels)
								addMetric("af_remote_cable_loss", remAnt, "loss", apLabels)
								addMetric("peer_remote_gain", remAnt, "gain", peerLabels)
								addMetric("peer_remote_cable_loss", remAnt, "loss", peerLabels)
							}

							if remPwr, ok := remoteData["outputPower"].(map[string]any); ok {
								addMetric("af_remote_conducted", remPwr, "conducted", apLabels)
								addMetric("af_remote_eirp", remPwr, "eirp", apLabels)
								remAuto := 0.0
								if remPwr["autoPower"] == true {
									remAuto = 1.0
								}
								store.UpdateMetric("af_remote_autopower", apLabels, remAuto)
							}

							if remLQ, ok := remoteData["linkQuality"].(map[string]any); ok {
								addMetric("af_remote_noise", remLQ, "noiseFloor", apLabels)
								addMetric("peer_remote_noise", remLQ, "noiseFloor", peerLabels)
								addMetric("peer_remote_snr", remLQ, "snr", peerLabels)

								if remSig := remLQ["signal"]; remSig != nil {
									store.UpdateMetric("peer_remote_signal", peerLabels, utils.SafeFloat(remSig))
								}

								addMetric("af_remote_ideal_signal", remLQ, "idealSignal", apLabels)
								addMetric("peer_remote_ideal_signal", remLQ, "idealSignal", peerLabels)
								if remIdealSig := remLQ["idealSignal"]; remIdealSig != nil {
									idealVal := utils.SafeFloat(remIdealSig)
									realSig := utils.SafeFloat(remLQ["signal"])
									store.UpdateMetric("peer_remote_signal_delta_ideal", peerLabels, realSig-idealVal)
								}

								if remChains, ok := remLQ["signalPerChain"].([]any); ok && len(remChains) >= 2 {
									rc0 := utils.SafeFloat(remChains[0])
									rc1 := utils.SafeFloat(remChains[1])
									store.UpdateMetric("af_remote_chain0", apLabels, rc0)
									store.UpdateMetric("af_remote_chain1", apLabels, rc1)
									store.UpdateMetric("peer_remote_signal_c0", peerLabels, rc0)
									store.UpdateMetric("peer_remote_signal_c1", peerLabels, rc1)
									store.UpdateMetric("peer_remote_imbalance", peerLabels, math.Abs(rc0-rc1))
								}
							}

							if remGPS, ok := remoteData["gps"].(map[string]any); ok {
								if remGPS["fix"] == true {
									addMetric("peer_lat", remGPS, "lat", peerLabels)
									addMetric("peer_lon", remGPS, "lon", peerLabels)
								}
							}
						}
					}

					// Local
					if localList, ok := peer["local"].([]any); ok && len(localList) > 0 {
						if localData, ok := localList[0].(map[string]any); ok {
							if localLink, ok := localData["linkQuality"].(map[string]any); ok {
								if sig := localLink["signal"]; sig != nil {
									s := utils.SafeFloat(sig)
									store.UpdateMetric("ap_signal", apLabels, s)
									store.UpdateMetric("peer_signal", peerLabels, s)
								}
								if snr := localLink["snr"]; snr != nil {
									val := utils.SafeFloat(snr)
									store.UpdateMetric("ap_snr", apLabels, val)
									store.UpdateMetric("peer_snr", peerLabels, val)
								}

								addMetric("af_ideal_signal", localLink, "idealSignal", apLabels)
								addMetric("peer_ideal_signal", localLink, "idealSignal", peerLabels)
								if idealSig := localLink["idealSignal"]; idealSig != nil {
									idealVal := utils.SafeFloat(idealSig)
									realSig := utils.SafeFloat(localLink["signal"])
									store.UpdateMetric("af_signal_delta_ideal", apLabels, realSig-idealVal)
									store.UpdateMetric("peer_signal_delta_ideal", peerLabels, realSig-idealVal)
								}

								if capacity, ok := localLink["capacity"].(map[string]any); ok {
									addMetric("af_capacity_dl", capacity, "dl", apLabels, 1000)
									addMetric("af_capacity_ul", capacity, "ul", apLabels, 1000)
									addMetric("af_capacity_dl_ideal", capacity, "dlIdeal", apLabels, 1000)
									addMetric("af_capacity_ul_ideal", capacity, "ulIdeal", apLabels, 1000)
									addMetric("af_capacity_combined", capacity, "combined", apLabels, 1000)
									addMetric("af_capacity_combined_ideal", capacity, "combinedIdeal", apLabels, 1000)

									// Standard peer capacity in Mbps
									addMetric("peer_capacity_dl", capacity, "dl", peerLabels, 0.001)
									addMetric("peer_capacity_ul", capacity, "ul", peerLabels, 0.001)
									addMetric("peer_capacity_ideal_dl", capacity, "dlIdeal", peerLabels, 0.001)
									addMetric("peer_capacity_ideal_ul", capacity, "ulIdeal", peerLabels, 0.001)
									addMetric("peer_capacity_combined", capacity, "combined", peerLabels, 0.001)
									addMetric("peer_capacity_combined_ideal", capacity, "combinedIdeal", peerLabels, 0.001)
								}

								if score, ok := localLink["linkScore"].(map[string]any); ok {
									addMetric("af_link_score_dl", score, "dl", apLabels)
									addMetric("af_link_score_ul", score, "ul", apLabels)
									if totalScore, hasTotal := score["linkScore"]; hasTotal && totalScore != nil {
										addMetric("af_link_score", score, "linkScore", apLabels)
									} else if dlRaw, hasDL := score["dl"]; hasDL && dlRaw != nil {
										if ulRaw, hasUL := score["ul"]; hasUL && ulRaw != nil {
											store.UpdateMetric("af_link_score", apLabels, (utils.SafeFloat(dlRaw)+utils.SafeFloat(ulRaw))/2.0)
										}
									}
								}

								if cinr, ok := localLink["cinr"].(map[string]any); ok {
									addMetric("af_cinr_dl", cinr, "dl", apLabels)
									addMetric("af_cinr_ul", cinr, "ul", apLabels)
								}

								if nss, ok := localLink["nss"].(map[string]any); ok {
									addMetric("af_nss_dl", nss, "dl", apLabels)
									addMetric("af_nss_ul", nss, "ul", apLabels)
								}

								mcsMap, okMCS := localLink["mcs"].(map[string]any)
								if !okMCS || mcsMap == nil {
									mcsMap, _ = localLink["modulation"].(map[string]any)
								}
								if mcsMap != nil {
									txLabels := make(map[string]string, len(apLabels)+1)
									rxLabels := make(map[string]string, len(apLabels)+1)
									for k, v := range apLabels {
										txLabels[k] = v
										rxLabels[k] = v
									}
									txLabels["direction"] = "tx"
									rxLabels["direction"] = "rx"

									addMetric("af_mcs_index", mcsMap, "txIdx", txLabels)
									addMetric("af_mcs_index", mcsMap, "rxIdx", rxLabels)
									addMetric("af_modulation_rate", mcsMap, "txRate", txLabels)
									addMetric("af_modulation_rate", mcsMap, "rxRate", rxLabels)
								}

								if chains, ok := localLink["signalPerChain"].([]any); ok && len(chains) >= 2 {
									c0 := utils.SafeFloat(chains[0])
									c1 := utils.SafeFloat(chains[1])
									store.UpdateMetric("af_signal_chain0", apLabels, c0)
									store.UpdateMetric("af_signal_chain1", apLabels, c1)
									store.UpdateMetric("peer_chain_imbalance", peerLabels, math.Abs(c0-c1))
								}
							}
						}
					}

					// Info
					if ident, ok := common["identification"].(map[string]any); ok {
						infoLabels := make(map[string]string, len(peerLabels)+6)
						for k, v := range peerLabels {
							infoLabels[k] = v
						}
						infoLabels["peer_mac"] = utils.CleanString(ident["mac"])
						infoLabels["peer_product"] = utils.CleanString(ident["product"])
						infoLabels["peer_model"] = utils.CleanString(ident["model"])
						infoLabels["peer_family"] = utils.CleanString(ident["family"])
						infoLabels["peer_firmware"] = utils.CleanString(ident["firmwareVersion"])
						infoLabels["peer_firmware_build"] = utils.CleanString(ident["firmware"])
						store.UpdateMetric("peer_info", infoLabels, 1.0)
					}
				}
			}
		})
	}
}
