// Package collector extracts AirFiber (AF5XHD / LTU) exclusive radio and TDD backhaul metrics.
package collector

import (
	"context"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

// ProcessAF5XHD processes AirFiber-exclusive radio and backhaul link metrics
// (split TX/RX frequencies, RX channel width, TDD duty cycle, frame length, GPS sync,
// local RX efficiency, and local/remote auto-power state).
// Common system, interface, general wireless, and peer telemetry is processed by
// ProcessSystem, ProcessInterfaces, ProcessWireless, and ProcessPeers without duplication.
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
			wireless, okWireless := dataRoot["wireless"].(map[string]any)
			if !okWireless || wireless == nil {
				logger.DebugContext(ctx, devName, "Collector 'airfiber': 'wireless' payload missing in status data")
				return
			}

			radios, okRadios := wireless["radios"].([]any)
			if !okRadios || len(radios) == 0 {
				logger.DebugContext(ctx, devName, "Collector 'airfiber': 'radios' list missing or empty in wireless data")
			}
			peers, _ := wireless["peers"].([]any)

			addMetric := func(key string, src map[string]any, srcKey string, labels map[string]string) {
				if src == nil {
					return
				}
				if val, exists := src[srcKey]; exists && val != nil {
					store.UpdateMetric(key, labels, utils.SafeFloat(val))
				}
			}

			// 1. AirFiber-exclusive local radio parameters
			if len(radios) > 0 {
				if radio, ok := radios[0].(map[string]any); ok {
					if freqData, ok := radio["frequency"].(map[string]any); ok && freqData != nil {
						addMetric("af_frequency_tx", freqData, "tx", apLabels)
						addMetric("af_frequency_rx", freqData, "rx", apLabels)
					}

					if width, ok := radio["channelWidth"].(map[string]any); ok && width != nil {
						addMetric("af_channel_width_rx", width, "rx", apLabels)
					}

					if pwrData, ok := radio["outputPower"].(map[string]any); ok && pwrData != nil {
						if pwrData["autoPower"] != nil {
							autoPwr := 0.0
							if pwrData["autoPower"] == true || utils.SafeFloat(pwrData["autoPower"]) == 1.0 {
								autoPwr = 1.0
							}
							store.UpdateMetric("af_local_autopower", apLabels, autoPwr)
						}
					}

					addMetric("af_dl_ratio", radio, "dlRatio", apLabels)
					addMetric("af_frame_length", radio, "frameLength", apLabels)
					addMetric("af_gps_sync_state", radio, "gpsSyncState", apLabels)
					addMetric("af_rx_efficiency", radio, "rxEfficiency", apLabels)
				}
			}

			// 2. AirFiber-exclusive remote backhaul parameters
			if len(peers) > 0 {
				if peer, ok := peers[0].(map[string]any); ok {
					if remoteList, ok := peer["remote"].([]any); ok && len(remoteList) > 0 {
						if remoteData, ok := remoteList[0].(map[string]any); ok {
							addMetric("af_remote_gps_sync_state", remoteData, "gpsSyncState", apLabels)

							remLQ, _ := remoteData["linkQuality"].(map[string]any)
							remPwr, okPwr := remoteData["outputPower"].(map[string]any)
							if (!okPwr || remPwr == nil) && remLQ != nil {
								remPwr, _ = remLQ["outputPower"].(map[string]any)
							}
							if remPwr != nil && remPwr["autoPower"] != nil {
								remAuto := 0.0
								if remPwr["autoPower"] == true || utils.SafeFloat(remPwr["autoPower"]) == 1.0 {
									remAuto = 1.0
								}
								store.UpdateMetric("af_remote_autopower", apLabels, remAuto)
							}
						}
					}
				}
			}
		})
	}
}
