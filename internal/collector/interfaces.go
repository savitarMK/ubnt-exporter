// Package collector extracts network interface metrics.
package collector

import (
	"context"
	"fmt"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

var ifaceMetricsMap = map[string]string{
	"iface_rx_bytes":     "rxBytes",
	"iface_tx_bytes":     "txBytes",
	"iface_rx_errors":    "rxErrors",
	"iface_tx_errors":    "txErrors",
	"iface_rx_drops":     "rxDropped",
	"iface_tx_drops":     "txDropped",
	"iface_rx_rate":      "rxRate",
	"iface_tx_rate":      "txRate",
	"iface_rx_pps":       "rxPPS",
	"iface_tx_pps":       "txPPS",
	"iface_rx_packets":   "rxPackets",
	"iface_tx_packets":   "txPackets",
	"iface_rx_broadcast": "rxBroadcast",
	"iface_tx_broadcast": "txBroadcast",
	"iface_rx_multicast": "rxMulticast",
	"iface_tx_multicast": "txMulticast",
	"iface_rx_flow_ctrl": "rxFlowCtrl",
	"iface_tx_flow_ctrl": "txFlowCtrl",
	"iface_rx_jumbo":     "rxJumbo",
	"iface_tx_jumbo":     "txJumbo",
}

// ProcessInterfaces extracts traffic and packet statistics for all device network interfaces.
func ProcessInterfaces(ctx context.Context, dataRoot map[string]any, apLabels map[string]string, store metrics.MetricSink) {
	devName := getDeviceName(apLabels)
	ifacesRaw, ok := dataRoot["interfaces"].([]any)
	if !ok || ifacesRaw == nil {
		logger.DebugContext(ctx, devName, "Collector 'interfaces': 'interfaces' list missing or invalid")
		return
	}

	for _, ifaceItem := range ifacesRaw {
		ifaceMap, ok := ifaceItem.(map[string]any)
		if !ok {
			logger.DebugContext(ctx, devName, "Collector 'interfaces': non-object interface entry encountered")
			continue
		}

		ifaceID := "Unknown"
		if idVal, hasID := ifaceMap["id"]; hasID && idVal != nil {
			ifaceID = fmt.Sprintf("%v", idVal)
		}

		statsMap, ok := ifaceMap["statistics"].(map[string]any)
		if !ok || statsMap == nil {
			logger.DebugContext(ctx, devName, "Collector 'interfaces': interface %q missing 'statistics' map", ifaceID)
			continue
		}

		labels := make(map[string]string, len(apLabels)+1)
		for k, v := range apLabels {
			labels[k] = v
		}
		labels["interface"] = ifaceID

		for metricKey, jsonKey := range ifaceMetricsMap {
			if val, exists := statsMap[jsonKey]; exists && val != nil {
				store.UpdateMetric(metricKey, labels, utils.SafeFloat(val))
			}
		}
	}
}
