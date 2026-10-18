// Package collector extracts AirFiber (AF5XHD) specific metrics.
package collector

import (
	"context"
	"strings"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

// ProcessAF5XHD processes AirFiber specific radio and backhaul link metrics.
func ProcessAF5XHD(
	ctx context.Context,
	dataRoot map[string]any,
	apLabels map[string]string,
	collectorsConf map[string]bool,
	store metrics.MetricSink,
) {
	devName := getDeviceName(apLabels)

	if IsCollectorEnabled(collectorsConf, "airfiber") {
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
			addMetric("ap_service_uptime", wireless, "serviceUptime", apLabels)
			addMetric("ap_service_time", wireless, "serviceTime", apLabels)

			// Temperatures (AirFiber specific gauge)
			if temps, ok := device["temperatures"].([]any); ok && len(temps) > 0 {
				if t0, ok := temps[0].(map[string]any); ok {
					addMetric("af_temperature", t0, "value", apLabels)
				}
			} else {
				logger.DebugContext(ctx, devName, "Collector 'airfiber': 'temperatures' list missing or empty in device data")
			}

			// GPS (AirFiber specific gauges when ProcessAF5XHD is invoked standalone)
			if gps, ok := device["gps"].(map[string]any); ok {
				if gps["fix"] == true {
					addMetric("af_gps_sats", gps, "sats", apLabels)
					addMetric("af_gps_dop", gps, "dop", apLabels)
					addMetric("af_gps_alt", gps, "alt", apLabels)
				} else {
					logger.DebugContext(ctx, devName, "Collector 'airfiber': GPS lock not acquired (fix=false)")
				}
			} else {
				logger.DebugContext(ctx, devName, "Collector 'airfiber': 'gps' telemetry missing in device data")
			}

			var radioNoiseFloor any

			// Radios
			if len(radios) > 0 {
				if radio, ok := radios[0].(map[string]any); ok {
					radioNoiseFloor = radio["noiseFloor"]

					if ls, ok := radio["linkState"].(string); ok {
						stateVal := 0.0
						if strings.ToLower(ls) == "connected" {
							stateVal = 1.0
						}
						store.UpdateMetric("af_link_state", apLabels, stateVal)
						store.UpdateMetric("ap_link_state", apLabels, stateVal)
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
						if enabledVal, hasEnabled := dfs["enabled"]; hasEnabled && enabledVal != nil {
							val := 0.0
							if enabledVal == true || utils.SafeFloat(enabledVal) == 1.0 {
								val = 1.0
							}
							store.UpdateMetric("ap_dfs_enabled", apLabels, val)
						}
						if cacDur, hasDur := dfs["cacDuration"]; hasDur && cacDur != nil {
							store.UpdateMetric("ap_dfs_cac_duration", apLabels, utils.SafeFloat(cacDur))
						}
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

			// AirFiber-specific peer metrics (af_*)
			if len(peers) > 0 {
				if peer, ok := peers[0].(map[string]any); ok {
					common, _ := peer["common"].(map[string]any)
					if common == nil {
						common = make(map[string]any)
					}

					peerName := utils.CleanString(common["hostname"])
					afPeerLabels := make(map[string]string, len(apLabels)+1)
					for k, v := range apLabels {
						afPeerLabels[k] = v
					}
					afPeerLabels["peer_name"] = peerName

					if peerCPU, ok := common["cpu"].([]any); ok && len(peerCPU) > 0 {
						if cpu0, ok := peerCPU[0].(map[string]any); ok {
							addMetric("af_peer_cpu", cpu0, "usage", afPeerLabels)
						}
					}

					if peerRAM, ok := common["ram"].(map[string]any); ok {
						addMetric("af_peer_ram", peerRAM, "usage", afPeerLabels)
					}

					if counters, ok := common["counters"].(map[string]any); ok {
						addMetric("af_peer_tx_bytes", counters, "txBytes", afPeerLabels)
						addMetric("af_peer_rx_bytes", counters, "rxBytes", afPeerLabels)
						addMetric("af_peer_tx_rate", counters, "txRate", afPeerLabels)
						addMetric("af_peer_rx_rate", counters, "rxRate", afPeerLabels)
						addMetric("af_peer_tx_pps", counters, "txPPS", afPeerLabels)
						addMetric("af_peer_rx_pps", counters, "rxPPS", afPeerLabels)
					}

					// Remote AirFiber metrics
					if remoteList, ok := peer["remote"].([]any); ok && len(remoteList) > 0 {
						if remoteData, ok := remoteList[0].(map[string]any); ok {
							addMetric("af_remote_rx_eff", remoteData, "rxEfficiency", apLabels)
							addMetric("af_remote_gps_sync_state", remoteData, "gpsSyncState", apLabels)

							if remAnt, ok := remoteData["antenna"].(map[string]any); ok {
								addMetric("af_remote_gain", remAnt, "gain", apLabels)
								addMetric("af_remote_cable_loss", remAnt, "loss", apLabels)
							}

							remLQ, _ := remoteData["linkQuality"].(map[string]any)

							remPwr, okPwr := remoteData["outputPower"].(map[string]any)
							if (!okPwr || remPwr == nil) && remLQ != nil {
								remPwr, _ = remLQ["outputPower"].(map[string]any)
							}
							if remPwr != nil {
								addMetric("af_remote_conducted", remPwr, "conducted", apLabels)
								addMetric("af_remote_eirp", remPwr, "eirp", apLabels)
								remAuto := 0.0
								if remPwr["autoPower"] == true {
									remAuto = 1.0
								}
								store.UpdateMetric("af_remote_autopower", apLabels, remAuto)
							}

							if remLQ != nil {
								addMetric("af_remote_noise", remLQ, "noiseFloor", apLabels)
								addMetric("af_remote_ideal_signal", remLQ, "idealSignal", apLabels)

								if remChains, ok := remLQ["signalPerChain"].([]any); ok && len(remChains) >= 2 {
									rc0 := utils.SafeFloat(remChains[0])
									rc1 := utils.SafeFloat(remChains[1])
									store.UpdateMetric("af_remote_chain0", apLabels, rc0)
									store.UpdateMetric("af_remote_chain1", apLabels, rc1)
								}
							}
						}
					}

					// Local AirFiber metrics
					if localList, ok := peer["local"].([]any); ok && len(localList) > 0 {
						if localData, ok := localList[0].(map[string]any); ok {
							if localLink, ok := localData["linkQuality"].(map[string]any); ok {
								if sig := localLink["signal"]; sig != nil {
									store.UpdateMetric("ap_signal", apLabels, utils.SafeFloat(sig))
								}
								if snr := localLink["snr"]; snr != nil {
									store.UpdateMetric("ap_snr", apLabels, utils.SafeFloat(snr))
								} else if localLink["signal"] != nil {
									effNoise := localLink["noiseFloor"]
									if effNoise == nil {
										effNoise = radioNoiseFloor
									}
									if effNoise != nil {
										store.UpdateMetric("ap_snr", apLabels, utils.SafeFloat(localLink["signal"])-utils.SafeFloat(effNoise))
									}
								}

								addMetric("af_ideal_signal", localLink, "idealSignal", apLabels)
								if idealSig := localLink["idealSignal"]; idealSig != nil {
									idealVal := utils.SafeFloat(idealSig)
									realSig := utils.SafeFloat(localLink["signal"])
									store.UpdateMetric("af_signal_delta_ideal", apLabels, realSig-idealVal)
								}

								if capacity, ok := localLink["capacity"].(map[string]any); ok {
									addMetric("af_capacity_dl", capacity, "dl", apLabels, 1000)
									addMetric("af_capacity_ul", capacity, "ul", apLabels, 1000)
									addMetric("af_capacity_dl_ideal", capacity, "dlIdeal", apLabels, 1000)
									addMetric("af_capacity_ul_ideal", capacity, "ulIdeal", apLabels, 1000)
									addMetric("af_capacity_combined", capacity, "combined", apLabels, 1000)
									addMetric("af_capacity_combined_ideal", capacity, "combinedIdeal", apLabels, 1000)
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
								}
							}
						}
					}
				}
			}
		})
	}
}
