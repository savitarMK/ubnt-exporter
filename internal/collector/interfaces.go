// Package collector extracts network interface metrics.
package collector

import (
	"context"
	"fmt"
	"strings"

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

// classifyInterface determines the interface type, parent interface, and 802.1Q VLAN ID from its identifier.
func classifyInterface(ifaceID string) (ifaceType, parentIface, vlanID string) {
	parentIface = ifaceID
	vlanID = ""

	if strings.Contains(ifaceID, ".") {
		parts := strings.SplitN(ifaceID, ".", 2)
		parentIface = parts[0]
		vlanID = parts[1]
		ifaceType = "vlan"
		return ifaceType, parentIface, vlanID
	}

	if strings.Contains(ifaceID, "/") {
		parts := strings.SplitN(ifaceID, "/", 2)
		parentIface = parts[0]
	}

	lower := strings.ToLower(ifaceID)
	switch {
	case strings.HasPrefix(lower, "eth"):
		ifaceType = "ethernet"
	case strings.HasPrefix(lower, "ath") || strings.HasPrefix(lower, "wlan") || strings.HasPrefix(lower, "wifi"):
		ifaceType = "wireless"
	case strings.HasPrefix(lower, "br"):
		ifaceType = "bridge"
	default:
		ifaceType = "other"
	}
	return ifaceType, parentIface, vlanID
}

// ProcessInterfaces extracts traffic, classification metadata, and physical link statistics for all device network interfaces.
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

		ifaceName := ifaceID
		if rawName, hasName := ifaceMap["name"]; hasName && rawName != nil {
			cleaned := strings.TrimSpace(fmt.Sprintf("%v", rawName))
			if cleaned != "" {
				ifaceName = cleaned
			}
		}

		ifaceType, parentIface, vlanID := classifyInterface(ifaceID)

		labels := make(map[string]string, len(apLabels)+1)
		for k, v := range apLabels {
			labels[k] = v
		}
		labels["interface"] = ifaceID

		infoLabels := make(map[string]string, len(labels)+4)
		for k, v := range labels {
			infoLabels[k] = v
		}
		infoLabels["interface_name"] = ifaceName
		infoLabels["interface_type"] = ifaceType
		infoLabels["parent_interface"] = parentIface
		infoLabels["vlan_id"] = vlanID
		store.UpdateMetric("iface_info", infoLabels, 1.0)

		statusMap, _ := ifaceMap["status"].(map[string]any)

		extractField := func(key string) (any, bool) {
			if v, ok := ifaceMap[key]; ok && v != nil {
				return v, true
			}
			if statusMap != nil {
				if v, ok := statusMap[key]; ok && v != nil {
					return v, true
				}
			}
			return nil, false
		}

		if enabledVal, ok := extractField("enabled"); ok {
			val := 0.0
			if enabledVal == true || utils.SafeFloat(enabledVal) == 1.0 {
				val = 1.0
			}
			store.UpdateMetric("iface_enabled", labels, val)
		}

		if pluggedVal, ok := extractField("plugged"); ok {
			val := 0.0
			if pluggedVal == true || utils.SafeFloat(pluggedVal) == 1.0 {
				val = 1.0
			}
			store.UpdateMetric("iface_plugged", labels, val)
		}

		if speedVal, ok := extractField("speed"); ok {
			store.UpdateMetric("iface_speed", labels, utils.SafeFloat(speedVal))
		} else if speedVal, ok := extractField("currentSpeed"); ok {
			store.UpdateMetric("iface_speed", labels, utils.SafeFloat(speedVal))
		}

		if duplexVal, ok := extractField("duplex"); ok {
			val := 0.0
			switch d := duplexVal.(type) {
			case bool:
				if d {
					val = 1.0
				}
			case string:
				if strings.Contains(strings.ToLower(d), "full") {
					val = 1.0
				}
			default:
				if utils.SafeFloat(d) == 1.0 {
					val = 1.0
				}
			}
			store.UpdateMetric("iface_duplex", labels, val)
		}

		if mtuVal, ok := extractField("mtu"); ok {
			store.UpdateMetric("iface_mtu", labels, utils.SafeFloat(mtuVal))
		}

		if snrVal, ok := extractField("snr"); ok {
			store.UpdateMetric("iface_snr", labels, utils.SafeFloat(snrVal))
		}

		statsMap, ok := ifaceMap["statistics"].(map[string]any)
		if !ok || statsMap == nil {
			logger.DebugContext(ctx, devName, "Collector 'interfaces': interface %q missing 'statistics' map", ifaceID)
			continue
		}

		for metricKey, jsonKey := range ifaceMetricsMap {
			if val, exists := statsMap[jsonKey]; exists && val != nil {
				store.UpdateMetric(metricKey, labels, utils.SafeFloat(val))
			}
		}
	}
}
