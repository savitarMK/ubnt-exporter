// Package collector extracts metrics from parsed Ubiquiti device data.
package collector

import (
	"context"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

// ProcessSystem extracts CPU, RAM, and uptime metrics.
func ProcessSystem(ctx context.Context, dataRoot map[string]any, apLabels map[string]string, store metrics.MetricSink) {
	devName := getDeviceName(apLabels)
	labels := make(map[string]string, len(apLabels))
	for k, v := range apLabels {
		labels[k] = v
	}

	deviceData, ok := dataRoot["device"].(map[string]any)
	if !ok || deviceData == nil {
		logger.DebugContext(ctx, devName, "Collector 'system': 'device' payload missing or invalid")
		return
	}

	cpuRaw, hasCPU := deviceData["cpu"]
	if !hasCPU || cpuRaw == nil {
		logger.DebugContext(ctx, devName, "Collector 'system': 'cpu' field missing in device data")
	} else {
		cpuVal := 0.0
		switch v := cpuRaw.(type) {
		case []any:
			if len(v) > 0 {
				if item, ok := v[0].(map[string]any); ok {
					cpuVal = utils.SafeFloat(item["usage"])
				}
			}
		case map[string]any:
			cpuVal = utils.SafeFloat(v["usage"])
		default:
			cpuVal = utils.SafeFloat(v)
		}
		store.UpdateMetric("cpu_usage", labels, cpuVal)
	}

	ramRaw, hasRAM := deviceData["ram"]
	if !hasRAM || ramRaw == nil {
		logger.DebugContext(ctx, devName, "Collector 'system': 'ram' field missing in device data")
	} else {
		ramVal := 0.0
		if rMap, ok := ramRaw.(map[string]any); ok {
			ramVal = utils.SafeFloat(rMap["usage"])
			if freeVal, hasFree := rMap["free"]; hasFree && freeVal != nil {
				store.UpdateMetric("ram_free_bytes", labels, utils.SafeFloat(freeVal))
			}
			if totalVal, hasTotal := rMap["total"]; hasTotal && totalVal != nil {
				store.UpdateMetric("ram_total_bytes", labels, utils.SafeFloat(totalVal))
			}
		} else {
			ramVal = utils.SafeFloat(ramRaw)
		}
		store.UpdateMetric("ram_usage", labels, ramVal)
	}

	if uptimeVal, hasUptime := deviceData["uptime"]; hasUptime && uptimeVal != nil {
		store.UpdateMetric("uptime", labels, utils.SafeFloat(uptimeVal))
	} else {
		logger.DebugContext(ctx, devName, "Collector 'system': 'uptime' field missing in device data")
	}

	if pt, hasPt := deviceData["powerTime"]; hasPt && pt != nil {
		store.UpdateMetric("af_power_time", labels, utils.SafeFloat(pt))
		store.UpdateMetric("device_power_time", labels, utils.SafeFloat(pt))
	}
}
