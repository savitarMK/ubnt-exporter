// Package collector extracts general wireless (AirMAX/LTU) metrics.
package collector

import (
	"context"
	"strings"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

// ProcessWireless processes AirMAX / LTU access point wireless radio statistics.
func ProcessWireless(
	ctx context.Context,
	dataRoot map[string]any,
	apLabels map[string]string,
	_ map[string]bool,
	store metrics.MetricSink,
) {
	devName := getDeviceName(apLabels)

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

		if sUptime, exists := wireless["serviceUptime"]; exists && sUptime != nil {
			store.UpdateMetric("ap_service_uptime", labels, utils.SafeFloat(sUptime))
		}
		if sTime, exists := wireless["serviceTime"]; exists && sTime != nil {
			store.UpdateMetric("ap_service_time", labels, utils.SafeFloat(sTime))
		}

		peersList, _ := wireless["peers"].([]any)
		radiosList, hasRadios := wireless["radios"].([]any)
		if !hasRadios || len(radiosList) == 0 {
			logger.DebugContext(ctx, devName, "Collector 'wireless': 'radios' list missing or empty in wireless data")
		}

		if len(radiosList) > 0 {
			if mainRadio, ok := radiosList[0].(map[string]any); ok {
				noiseRecorded := false
				if len(peersList) > 0 {
					if firstPeer, ok := peersList[0].(map[string]any); ok {
						if localList, ok := firstPeer["local"].([]any); ok && len(localList) > 0 {
							if localData, ok := localList[0].(map[string]any); ok {
								if linkQual, ok := localData["linkQuality"].(map[string]any); ok {
									noiseRaw, hasNoise := linkQual["noiseFloor"]
									sigRaw, hasSig := linkQual["signal"]

									if hasNoise && noiseRaw != nil {
										store.UpdateMetric("ap_noise", labels, utils.SafeFloat(noiseRaw))
										noiseRecorded = true
									} else {
										logger.DebugContext(ctx, devName, "Collector 'wireless': 'noiseFloor' field missing in linkQuality")
									}
									if hasSig && sigRaw != nil {
										store.UpdateMetric("ap_signal", labels, utils.SafeFloat(sigRaw))
									} else {
										logger.DebugContext(ctx, devName, "Collector 'wireless': 'signal' field missing in linkQuality")
									}
									if snrRaw, hasSNR := linkQual["snr"]; hasSNR && snrRaw != nil {
										store.UpdateMetric("ap_snr", labels, utils.SafeFloat(snrRaw))
									} else if hasSig && sigRaw != nil && hasNoise && noiseRaw != nil {
										store.UpdateMetric("ap_snr", labels, utils.SafeFloat(sigRaw)-utils.SafeFloat(noiseRaw))
									}
								}
							}
						}
					}
				}
				if !noiseRecorded {
					if noiseRaw, has := mainRadio["noiseFloor"]; has && noiseRaw != nil {
						store.UpdateMetric("ap_noise", labels, utils.SafeFloat(noiseRaw))
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

				if _, exists := wireless["serviceUptime"]; !exists {
					if sUptime, has := mainRadio["serviceUptime"]; has && sUptime != nil {
						store.UpdateMetric("ap_service_uptime", labels, utils.SafeFloat(sUptime))
					}
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
					if enabledVal, hasEnabled := dfs["enabled"]; hasEnabled && enabledVal != nil {
						val := 0.0
						if enabledVal == true || utils.SafeFloat(enabledVal) == 1.0 {
							val = 1.0
						}
						store.UpdateMetric("ap_dfs_enabled", labels, val)
					}
					if cacDur, hasDur := dfs["cacDuration"]; hasDur && cacDur != nil {
						store.UpdateMetric("ap_dfs_cac_duration", labels, utils.SafeFloat(cacDur))
					}
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

// IsCollectorEnabled returns whether a collector module is enabled in the configuration map.
func IsCollectorEnabled(conf map[string]bool, name string) bool {
	if conf == nil {
		return true
	}
	enabled, exists := conf[name]
	if !exists {
		return true
	}
	return enabled
}
