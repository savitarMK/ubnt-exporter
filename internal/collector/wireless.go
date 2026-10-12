// Package collector extracts general wireless (AirMAX/LTU) metrics.
package collector

import (
	"context"
	"strings"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

// ProcessWireless processes AirMAX / LTU device wireless statistics.
func ProcessWireless(
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

	if isCollectorEnabled(collectorsConf, "peers") {
		runCollectorTimed("peers", func() {
			ProcessPeers(ctx, dataRoot, apLabels, store)
		})
	}

	runCollectorTimed("wireless", func() {
		labels := make(map[string]string, len(apLabels))
		for k, v := range apLabels {
			labels[k] = v
		}

		wireless, ok := dataRoot["wireless"].(map[string]any)
		if !ok || wireless == nil {
			logger.DebugContext(ctx, devName, "Collector 'wireless': 'wireless' payload missing or invalid")
			return
		}

		peersList, _ := wireless["peers"].([]any)
		radiosList, hasRadios := wireless["radios"].([]any)
		if !hasRadios || len(radiosList) == 0 {
			logger.DebugContext(ctx, devName, "Collector 'wireless': 'radios' list missing or empty in wireless data")
		}

		if len(radiosList) > 0 {
			if mainRadio, ok := radiosList[0].(map[string]any); ok {
				if len(peersList) > 0 {
					if firstPeer, ok := peersList[0].(map[string]any); ok {
						if localList, ok := firstPeer["local"].([]any); ok && len(localList) > 0 {
							if localData, ok := localList[0].(map[string]any); ok {
								if linkQual, ok := localData["linkQuality"].(map[string]any); ok {
									if noiseRaw, has := linkQual["noiseFloor"]; has && noiseRaw != nil {
										store.UpdateMetric("ap_noise", labels, utils.SafeFloat(noiseRaw))
									} else {
										logger.DebugContext(ctx, devName, "Collector 'wireless': 'noiseFloor' field missing in linkQuality")
									}
									if sigRaw, has := linkQual["signal"]; has && sigRaw != nil {
										store.UpdateMetric("ap_signal", labels, utils.SafeFloat(sigRaw))
									} else {
										logger.DebugContext(ctx, devName, "Collector 'wireless': 'signal' field missing in linkQuality")
									}
								}
							}
						}
					}
				}

				if ls, ok := mainRadio["linkState"].(string); ok {
					stateVal := 0.0
					if strings.ToLower(ls) == "connected" {
						stateVal = 1.0
					}
					store.UpdateMetric("ap_link_state", labels, stateVal)
				} else {
					logger.DebugContext(ctx, devName, "Collector 'wireless': 'linkState' field missing or invalid in main radio")
				}

				if sUptime, exists := wireless["serviceUptime"]; exists && sUptime != nil {
					store.UpdateMetric("ap_service_uptime", labels, utils.SafeFloat(sUptime))
				} else if sUptime, exists := mainRadio["serviceUptime"]; exists && sUptime != nil {
					store.UpdateMetric("ap_service_uptime", labels, utils.SafeFloat(sUptime))
				}

				if freqData, ok := mainRadio["frequency"].(map[string]any); ok {
					if centerFreq, has := freqData["center"]; has && centerFreq != nil {
						store.UpdateMetric("ap_frequency", labels, utils.SafeFloat(centerFreq))
						store.UpdateMetric("ap_control_frequency", labels, utils.SafeFloat(freqData["control"]))
					}
				} else {
					logger.DebugContext(ctx, devName, "Collector 'wireless': 'frequency' map missing in main radio")
				}

				if chWidthMap, ok := mainRadio["channelWidth"].(map[string]any); ok {
					store.UpdateMetric("ap_channel_width", labels, utils.SafeFloat(chWidthMap["tx"]))
				} else {
					logger.DebugContext(ctx, devName, "Collector 'wireless': 'channelWidth' map missing in main radio")
				}

				if pwrMap, ok := mainRadio["outputPower"].(map[string]any); ok {
					store.UpdateMetric("ap_output_power", labels, utils.SafeFloat(pwrMap["conducted"]))
					store.UpdateMetric("ap_eirp", labels, utils.SafeFloat(pwrMap["eirp"]))
				}

				if dfs, ok := mainRadio["dfs"].(map[string]any); ok {
					cacRem := utils.SafeFloat(dfs["cacRemaining"])
					store.UpdateMetric("ap_dfs_cac_rem", labels, cacRem)
					dfsStatus := 0.0
					if cacRem > 0 {
						dfsStatus = 1.0
					}
					store.UpdateMetric("ap_dfs_status", labels, dfsStatus)
					if lockouts, ok := dfs["lockouts"].([]any); ok {
						store.UpdateMetric("ap_dfs_lockout_count", labels, float64(len(lockouts)))
					} else {
						store.UpdateMetric("ap_dfs_lockout_count", labels, 0.0)
					}
				}
			}
		}

		clientCount := float64(len(peersList))
		store.UpdateMetric("ap_clients", labels, clientCount)
	})
}

func isCollectorEnabled(conf map[string]bool, name string) bool {
	if conf == nil {
		return true
	}
	enabled, exists := conf[name]
	if !exists {
		return true
	}
	return enabled
}
