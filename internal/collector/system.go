// Package collector extracts metrics from parsed Ubiquiti device data.
package collector

import (
	"context"
	"fmt"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

// ProcessSystem extracts CPU, RAM, uptime, power time, GPS, and hardware sensor metrics.
func ProcessSystem(ctx context.Context, dataRoot map[string]any, apLabels map[string]string, store metrics.MetricSink) {
	devName := getDeviceName(apLabels)
	labels := make(map[string]string, len(apLabels))
	for k, v := range apLabels {
		labels[k] = v
	}

	if tsRaw, hasTS := dataRoot["timestamp"]; hasTS && tsRaw != nil {
		tsVal := utils.SafeFloat(tsRaw)
		if tsVal > 1e11 {
			tsVal *= 0.001
		}
		if tsVal > 0 {
			store.UpdateMetric("device_timestamp", labels, tsVal)
		}
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
		cpuIdent := ""
		switch v := cpuRaw.(type) {
		case []any:
			if len(v) > 0 {
				var sum float64
				var count int
				for i, core := range v {
					if item, ok := core.(map[string]any); ok {
						sum += utils.SafeFloat(item["usage"])
						count++
						if i == 0 && item["identifier"] != nil {
							cpuIdent = utils.CleanString(item["identifier"])
						}
					}
				}
				if count > 0 {
					cpuVal = sum / float64(count)
				}
			}
		case map[string]any:
			cpuVal = utils.SafeFloat(v["usage"])
			if v["identifier"] != nil {
				cpuIdent = utils.CleanString(v["identifier"])
			}
		default:
			cpuVal = utils.SafeFloat(v)
		}
		store.UpdateMetric("cpu_usage", labels, cpuVal)
		if cpuIdent != "" && cpuIdent != "N/A" {
			cpuLabels := make(map[string]string, len(labels)+1)
			for k, v := range labels {
				cpuLabels[k] = v
			}
			cpuLabels["cpu_identifier"] = cpuIdent
			store.UpdateMetric("cpu_info", cpuLabels, 1.0)
		}
	}

	ramRaw, hasRAM := deviceData["ram"]
	if !hasRAM || ramRaw == nil {
		logger.DebugContext(ctx, devName, "Collector 'system': 'ram' field missing in device data")
	} else {
		ramVal := 0.0
		if rMap, ok := ramRaw.(map[string]any); ok {
			ramVal = utils.SafeFloat(rMap["usage"])
			freeVal, hasFree := rMap["free"]
			totalVal, hasTotal := rMap["total"]
			if hasFree && freeVal != nil {
				store.UpdateMetric("ram_free_bytes", labels, utils.SafeFloat(freeVal))
			}
			if hasTotal && totalVal != nil {
				store.UpdateMetric("ram_total_bytes", labels, utils.SafeFloat(totalVal))
			}
			if hasFree && freeVal != nil && hasTotal && totalVal != nil {
				freeF := utils.SafeFloat(freeVal)
				totalF := utils.SafeFloat(totalVal)
				if totalF >= freeF && totalF > 0 {
					store.UpdateMetric("ram_used_bytes", labels, totalF-freeF)
				}
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

	if gps, ok := deviceData["gps"].(map[string]any); ok && gps != nil {
		fixVal := 0.0
		if gps["fix"] == true {
			fixVal = 1.0
		}
		store.UpdateMetric("ap_gps_fix", labels, fixVal)
		if gps["sats"] != nil {
			store.UpdateMetric("af_gps_sats", labels, utils.SafeFloat(gps["sats"]))
		}
		if gps["dop"] != nil {
			store.UpdateMetric("af_gps_dop", labels, utils.SafeFloat(gps["dop"]))
		}
		if gps["alt"] != nil {
			store.UpdateMetric("af_gps_alt", labels, utils.SafeFloat(gps["alt"]))
		}
		if gps["dim"] != nil {
			store.UpdateMetric("device_gps_dim", labels, utils.SafeFloat(gps["dim"]))
		}
		if gps["timeSyncEnabled"] != nil {
			tsVal := 0.0
			if gps["timeSyncEnabled"] == true || utils.SafeFloat(gps["timeSyncEnabled"]) == 1.0 {
				tsVal = 1.0
			}
			store.UpdateMetric("device_gps_time_sync", labels, tsVal)
		}
		if gps["fix"] == true || (gps["lat"] != nil && gps["lon"] != nil) {
			lat := utils.SafeFloat(gps["lat"])
			lon := utils.SafeFloat(gps["lon"])
			if lat != 0 || lon != 0 {
				store.UpdateMetric("ap_lat", labels, lat)
				store.UpdateMetric("ap_lon", labels, lon)
			}
		}
	}

	if ntp, ok := deviceData["ntpClient"].(map[string]any); ok && ntp != nil {
		if ntp["timeSyncEnabled"] != nil {
			ntpVal := 0.0
			if ntp["timeSyncEnabled"] == true || utils.SafeFloat(ntp["timeSyncEnabled"]) == 1.0 {
				ntpVal = 1.0
			}
			store.UpdateMetric("device_ntp_sync_enabled", labels, ntpVal)
		}
		if ntp["lastSync"] != nil {
			ls := utils.SafeFloat(ntp["lastSync"])
			if ls > 1e11 {
				ls *= 0.001
			}
			if ls > 0 {
				store.UpdateMetric("device_ntp_last_sync", labels, ls)
			}
		}
	}

	if temps, ok := deviceData["temperatures"].([]any); ok && len(temps) > 0 {
		for i, item := range temps {
			tMap, ok := item.(map[string]any)
			if !ok || tMap["value"] == nil {
				continue
			}
			sensorName := fmt.Sprintf("sensor_%d", i)
			if tMap["name"] != nil {
				sensorName = utils.CleanString(tMap["name"])
			} else if tMap["type"] != nil {
				sensorName = utils.CleanString(tMap["type"])
			}
			tLabels := make(map[string]string, len(labels)+1)
			for k, v := range labels {
				tLabels[k] = v
			}
			tLabels["sensor"] = sensorName
			store.UpdateMetric("device_temperature", tLabels, utils.SafeFloat(tMap["value"]))
		}
	}
}
