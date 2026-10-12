// Package collector extracts metrics from wireless peers / stations.
package collector

import (
	"context"
	"math"

	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

// ProcessPeers extracts stats from all wireless peers/clients connected to the AP.
func ProcessPeers(ctx context.Context, dataRoot map[string]any, apLabels map[string]string, store metrics.MetricSink) {
	devName := getDeviceName(apLabels)
	wireless, ok := dataRoot["wireless"].(map[string]any)
	if !ok || wireless == nil {
		logger.DebugContext(ctx, devName, "Collector 'peers': 'wireless' payload missing or invalid")
		return
	}

	peersRaw, okPeers := wireless["peers"].([]any)
	if !okPeers || peersRaw == nil {
		logger.DebugContext(ctx, devName, "Collector 'peers': 'peers' list missing in wireless data")
		return
	}

	for _, peerItem := range peersRaw {
		peer, ok := peerItem.(map[string]any)
		if !ok {
			logger.DebugContext(ctx, devName, "Collector 'peers': non-object peer entry encountered")
			continue
		}

		common, _ := peer["common"].(map[string]any)
		if common == nil {
			common = make(map[string]any)
		}

		var localRadio map[string]any
		if localList, ok := peer["local"].([]any); ok && len(localList) > 0 {
			localRadio, _ = localList[0].(map[string]any)
		}
		if localRadio == nil {
			localRadio = make(map[string]any)
		}

		var remoteRadio map[string]any
		if remoteList, ok := peer["remote"].([]any); ok && len(remoteList) > 0 {
			remoteRadio, _ = remoteList[0].(map[string]any)
		}
		if remoteRadio == nil {
			remoteRadio = make(map[string]any)
		}

		localLQ, _ := localRadio["linkQuality"].(map[string]any)
		if localLQ == nil {
			localLQ = make(map[string]any)
		}

		remoteLQ, _ := remoteRadio["linkQuality"].(map[string]any)
		if remoteLQ == nil {
			remoteLQ = make(map[string]any)
		}

		cnt, _ := common["counters"].(map[string]any)
		if cnt == nil {
			cnt = make(map[string]any)
		}

		ident, _ := common["identification"].(map[string]any)
		if ident == nil {
			ident = make(map[string]any)
		}

		peerName := utils.CleanString(common["hostname"])
		peerMgmtIP := utils.CleanString(common["mgmtIp"])

		apName := apLabels["name"]
		if apName == "" {
			apName = "N/A"
		}
		apIP := apLabels["ip"]
		if apIP == "" {
			apIP = "N/A"
		}

		labels := make(map[string]string, len(apLabels)+4)
		for k, v := range apLabels {
			labels[k] = v
		}
		labels["ap_name"] = apName
		labels["ap_ip"] = apIP
		labels["peer_name"] = peerName
		labels["peer_ip"] = peerMgmtIP

		addMetric := func(key string, src map[string]any, srcKey string, scale ...float64) {
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

		addMetric("peer_distance", common, "distance")
		addMetric("peer_uptime", common, "uptime")
		addMetric("peer_power_time", common, "powerTime")
		addMetric("peer_connection_time", localRadio, "connectionTime")

		if peerCPU, ok := common["cpu"].([]any); ok && len(peerCPU) > 0 {
			if cpu0, ok := peerCPU[0].(map[string]any); ok {
				addMetric("peer_cpu", cpu0, "usage")
			}
		}

		if peerRAM, ok := common["ram"].(map[string]any); ok {
			addMetric("peer_ram", peerRAM, "usage")
			addMetric("peer_ram_free", peerRAM, "free")
			addMetric("peer_ram_total", peerRAM, "total")
		}

		addMetric("peer_signal", localLQ, "signal")
		addMetric("peer_ideal_signal", localLQ, "idealSignal")
		if idealSig := localLQ["idealSignal"]; idealSig != nil {
			idealVal := utils.SafeFloat(idealSig)
			realSig := utils.SafeFloat(localLQ["signal"])
			store.UpdateMetric("peer_signal_delta_ideal", labels, realSig-idealVal)
		}

		addMetric("peer_noise_floor", localLQ, "noiseFloor")
		addMetric("peer_remote_signal", remoteLQ, "signal")
		addMetric("peer_remote_ideal_signal", remoteLQ, "idealSignal")
		if remIdealSig := remoteLQ["idealSignal"]; remIdealSig != nil {
			idealVal := utils.SafeFloat(remIdealSig)
			realSig := utils.SafeFloat(remoteLQ["signal"])
			store.UpdateMetric("peer_remote_signal_delta_ideal", labels, realSig-idealVal)
		}
		addMetric("peer_remote_noise", remoteLQ, "noiseFloor")

		// Local chain signals & imbalance
		if chainsLoc, ok := localLQ["signalPerChain"].([]any); ok && len(chainsLoc) >= 2 {
			c0 := utils.SafeFloat(chainsLoc[0])
			c1 := utils.SafeFloat(chainsLoc[1])
			store.UpdateMetric("peer_signal_c0", labels, c0)
			store.UpdateMetric("peer_signal_c1", labels, c1)
			store.UpdateMetric("peer_chain_imbalance", labels, math.Abs(c0-c1))
		}

		// Remote chain signals & imbalance
		if chainsRem, ok := remoteLQ["signalPerChain"].([]any); ok && len(chainsRem) >= 2 {
			rc0 := utils.SafeFloat(chainsRem[0])
			rc1 := utils.SafeFloat(chainsRem[1])
			store.UpdateMetric("peer_remote_signal_c0", labels, rc0)
			store.UpdateMetric("peer_remote_signal_c1", labels, rc1)
			store.UpdateMetric("peer_remote_imbalance", labels, math.Abs(rc0-rc1))
		}

		if mcs, ok := localLQ["mcs"].(map[string]any); ok {
			addMetric("peer_mcs_tx", mcs, "txIdx")
			addMetric("peer_mcs_rx", mcs, "rxIdx")
			addMetric("peer_mcs_ideal_tx", mcs, "txIdxIdeal")
			addMetric("peer_mcs_ideal_rx", mcs, "rxIdxIdeal")
			addMetric("peer_mcs_rate_tx", mcs, "txRate")
			addMetric("peer_mcs_rate_rx", mcs, "rxRate")
		}

		if airtimeObj, exists := localLQ["airtime"]; exists && airtimeObj != nil {
			if aMap, ok := airtimeObj.(map[string]any); ok {
				dlVal := utils.SafeFloat(aMap["dl"])
				ulVal := utils.SafeFloat(aMap["ul"])
				if dlVal > 100 || ulVal > 100 || (dlVal+ulVal > 100) {
					dlVal *= 0.1
					ulVal *= 0.1
				}
				store.UpdateMetric("peer_airtime", labels, dlVal)
				store.UpdateMetric("peer_airtime_ul", labels, ulVal)
			} else {
				val := utils.SafeFloat(airtimeObj)
				if val > 100 {
					val *= 0.1
				}
				store.UpdateMetric("peer_airtime", labels, val)
			}
		}

		if lscore, ok := localLQ["linkScore"].(map[string]any); ok {
			addMetric("peer_link_score_dl", lscore, "dl")
			addMetric("peer_link_score_ul", lscore, "ul")
			addMetric("peer_link_score_dl24", lscore, "dl24")
			addMetric("peer_link_score_ul24", lscore, "ul24")
		}

		if cinr, ok := localLQ["cinr"].(map[string]any); ok {
			addMetric("peer_cinr_dl", cinr, "dl")
			addMetric("peer_cinr_ul", cinr, "ul")
		}

		if nss, ok := localLQ["nss"].(map[string]any); ok {
			addMetric("peer_nss_dl", nss, "dl")
			addMetric("peer_nss_ul", nss, "ul")
		}

		if capMap, ok := localLQ["capacity"].(map[string]any); ok {
			addMetric("peer_capacity_dl", capMap, "dl", 0.001)
			addMetric("peer_capacity_ul", capMap, "ul", 0.001)
			addMetric("peer_capacity_ideal_dl", capMap, "dlIdeal", 0.001)
			addMetric("peer_capacity_ideal_ul", capMap, "ulIdeal", 0.001)
			addMetric("peer_capacity_combined", capMap, "combined", 0.001)
			addMetric("peer_capacity_combined_ideal", capMap, "combinedIdeal", 0.001)
		}

		if remAnt, ok := remoteRadio["antenna"].(map[string]any); ok {
			addMetric("peer_remote_gain", remAnt, "gain")
			addMetric("peer_remote_cable_loss", remAnt, "loss")
		}

		if remPwr, ok := remoteRadio["outputPower"].(map[string]any); ok {
			addMetric("peer_remote_power", remPwr, "conducted")
			addMetric("peer_remote_eirp", remPwr, "eirp")
		}

		if gpsData, ok := remoteRadio["gps"].(map[string]any); ok {
			if gpsData["fix"] == true || gpsData["lat"] != nil {
				addMetric("peer_lat", gpsData, "lat")
				addMetric("peer_lon", gpsData, "lon")
			}
		}

		addMetric("peer_tx_rate", cnt, "txRate", 0.000001)
		addMetric("peer_rx_rate", cnt, "rxRate", 0.000001)
		addMetric("peer_rx_bytes", cnt, "rxBytes")
		addMetric("peer_tx_bytes", cnt, "txBytes")

		infoLabels := make(map[string]string, len(labels)+5)
		for k, v := range labels {
			infoLabels[k] = v
		}
		infoLabels["peer_mac"] = utils.CleanString(ident["mac"])
		infoLabels["peer_product"] = utils.CleanString(ident["product"])
		infoLabels["peer_model"] = utils.CleanString(ident["model"])
		infoLabels["peer_family"] = utils.CleanString(ident["family"])
		infoLabels["peer_firmware"] = utils.CleanString(ident["firmwareVersion"])

		store.UpdateMetric("peer_info", infoLabels, 1.0)
	}
}
