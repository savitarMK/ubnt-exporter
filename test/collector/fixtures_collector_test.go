package collector_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"ubnt-exporter/internal/collector"
	"ubnt-exporter/internal/utils"
)

func loadFixtureRoot(t *testing.T, relPath string) map[string]any {
	t.Helper()
	// #nosec G304 -- test helper loading static repository fixtures from testdata/fixtures
	raw, err := os.ReadFile(relPath)
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", relPath, err)
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("failed to unmarshal fixture %s: %v", relPath, err)
	}
	switch v := parsed.(type) {
	case []any:
		if len(v) == 0 {
			t.Fatalf("fixture %s is an empty JSON array", relPath)
		}
		root, ok := v[0].(map[string]any)
		if !ok {
			t.Fatalf("fixture %s first element is not a JSON object", relPath)
		}
		return root
	case map[string]any:
		return v
	default:
		t.Fatalf("unexpected root type %T in fixture %s", parsed, relPath)
		return nil
	}
}

func TestCollectors_Fixture_AirMaxM_PtMP_Strict(t *testing.T) {
	root := loadFixtureRoot(t, "../../testdata/fixtures/airmax_m_ptmp.json")
	if fam := utils.DetectDeviceFamily(root); fam != "airmax" {
		t.Fatalf("expected DetectDeviceFamily='airmax' for airmax_m_ptmp.json, got %q", fam)
	}

	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "AP-M5-SECTOR-01", "ip": "192.168.1.30"}
	ctx := context.Background()

	collector.ProcessSystem(ctx, root, apLabels, sink)
	collector.ProcessInterfaces(ctx, root, apLabels, sink)
	collector.ProcessWireless(ctx, root, apLabels, nil, sink)
	collector.ProcessPeers(ctx, root, apLabels, sink)

	// 1. System strict checks
	if math.Abs(sink.metrics["device_timestamp"]-1383030812.717) > 1e-3 {
		t.Errorf("expected device_timestamp=1383030812.717, got %v", sink.metrics["device_timestamp"])
	}
	if sink.metrics["cpu_usage"] != 7.0 {
		t.Errorf("expected cpu_usage=7.0, got %v", sink.metrics["cpu_usage"])
	}
	if len(sink.records["cpu_info"]) != 1 || sink.records["cpu_info"][0].labels["cpu_identifier"] != "MIPS 74Kc V4.12" {
		t.Errorf("expected cpu_identifier='MIPS 74Kc V4.12', got %v", sink.records["cpu_info"])
	}
	if sink.metrics["ram_usage"] != 22.0 || sink.metrics["ram_free_bytes"] != 49098752.0 || sink.metrics["ram_total_bytes"] != 63627264.0 || sink.metrics["ram_used_bytes"] != 14528512.0 {
		t.Errorf("unexpected RAM metrics: usage=%v free=%v total=%v used=%v",
			sink.metrics["ram_usage"], sink.metrics["ram_free_bytes"], sink.metrics["ram_total_bytes"], sink.metrics["ram_used_bytes"])
	}
	if sink.metrics["uptime"] != 3464023.0 {
		t.Errorf("expected uptime=3464023.0, got %v", sink.metrics["uptime"])
	}
	if _, hasTemp := sink.metrics["device_temperature"]; hasTemp {
		t.Errorf("did not expect device_temperature on AirMAX M-series with empty temperatures array")
	}

	// 2. Interfaces strict checks (eth0, ath0, br1, br0, eth0.99, ath0.99)
	if len(sink.records["iface_info"]) != 6 {
		t.Fatalf("expected 6 iface_info records in AirMAX M fixture, got %d", len(sink.records["iface_info"]))
	}
	vlanCount := 0
	for _, rec := range sink.records["iface_info"] {
		if rec.labels["interface_type"] == "vlan" {
			vlanCount++
			if rec.labels["vlan_id"] != "99" {
				t.Errorf("expected vlan_id='99' for %s, got %q", rec.labels["interface"], rec.labels["vlan_id"])
			}
		}
	}
	if vlanCount != 2 {
		t.Errorf("expected 2 VLAN interfaces (eth0.99, ath0.99), got %d", vlanCount)
	}

	// 3. Wireless strict checks
	if sink.metrics["ap_link_state"] != 1.0 || sink.metrics["ap_frequency"] != 5380.0 || sink.metrics["ap_channel_width"] != 20.0 {
		t.Errorf("unexpected wireless radio metrics: link=%v freq=%v width=%v",
			sink.metrics["ap_link_state"], sink.metrics["ap_frequency"], sink.metrics["ap_channel_width"])
	}
	if _, hasCtrl := sink.metrics["ap_control_frequency"]; hasCtrl {
		t.Errorf("did not expect ap_control_frequency when frequency.control is absent in AirMAX M fixture")
	}
	if sink.metrics["ap_signal"] != -59.0 || sink.metrics["ap_noise"] != -99.0 || sink.metrics["ap_snr"] != 37.0 || sink.metrics["ap_clients"] != 4.0 {
		t.Errorf("unexpected wireless AP RF metrics: sig=%v noise=%v snr=%v clients=%v",
			sink.metrics["ap_signal"], sink.metrics["ap_noise"], sink.metrics["ap_snr"], sink.metrics["ap_clients"])
	}

	// 4. Peers strict checks (4 1x1 SISO stations: 3x AirGrid M5 HP, 1x LiteBeam M5)
	peerInfos := sink.records["peer_info"]
	if len(peerInfos) != 4 {
		t.Fatalf("expected 4 peer_info records, got %d", len(peerInfos))
	}
	wantModels := map[string]string{
		"CPE-M5-CLIENT-01": "AirGrid M5 HP",
		"CPE-M5-CLIENT-02": "LiteBeam M5",
		"CPE-M5-CLIENT-03": "AirGrid M5 HP",
		"CPE-M5-CLIENT-04": "AirGrid M5 HP",
	}
	for _, pi := range peerInfos {
		pName := pi.labels["peer_name"]
		if pi.labels["peer_family"] != "airmax-m" {
			t.Errorf("peer %s: expected peer_family='airmax-m', got %q", pName, pi.labels["peer_family"])
		}
		if pi.labels["peer_model"] != wantModels[pName] {
			t.Errorf("peer %s: expected fallback peer_model=%q, got %q", pName, wantModels[pName], pi.labels["peer_model"])
		}
	}
	if len(sink.records["peer_signal_c0"]) != 4 || len(sink.records["peer_signal_c1"]) != 4 {
		t.Errorf("expected 4 local chain0 and 4 local chain1 records on 2x2 AP, got c0=%d c1=%d",
			len(sink.records["peer_signal_c0"]), len(sink.records["peer_signal_c1"]))
	}
	if len(sink.records["peer_remote_signal_c0"]) != 4 {
		t.Errorf("expected 4 remote chain0 records on 1x1 SISO stations, got %d", len(sink.records["peer_remote_signal_c0"]))
	}
	if len(sink.records["peer_remote_signal_c1"]) != 0 {
		t.Errorf("expected 0 remote chain1 records for 1x1 SISO stations (signalPerChain[1]==0), got %d", len(sink.records["peer_remote_signal_c1"]))
	}
	if len(sink.records["peer_remote_imbalance"]) != 0 {
		t.Errorf("expected 0 peer_remote_imbalance records for 1x1 SISO stations, got %d", len(sink.records["peer_remote_imbalance"]))
	}
	if len(sink.records["peer_chain_imbalance"]) != 0 {
		t.Errorf("expected 0 peer_chain_imbalance records when remote stations are 1x1 SISO, got %d", len(sink.records["peer_chain_imbalance"]))
	}
}

func TestCollectors_Fixture_AirMaxAC_PtMP_Strict(t *testing.T) {
	root := loadFixtureRoot(t, "../../testdata/fixtures/airmax_ac_ptmp.json")
	if fam := utils.DetectDeviceFamily(root); fam != "airmax" {
		t.Fatalf("expected DetectDeviceFamily='airmax' for airmax_ac_ptmp.json, got %q", fam)
	}

	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "AP-SECTOR-01", "ip": "192.168.1.10"}
	ctx := context.Background()

	collector.ProcessSystem(ctx, root, apLabels, sink)
	collector.ProcessInterfaces(ctx, root, apLabels, sink)
	collector.ProcessWireless(ctx, root, apLabels, nil, sink)
	collector.ProcessPeers(ctx, root, apLabels, sink)

	if sink.metrics["ap_frequency"] != 5560.0 || sink.metrics["ap_control_frequency"] != 5560.0 || sink.metrics["ap_channel_width"] != 20.0 {
		t.Errorf("unexpected AirMAX AC radio frequency metrics: freq=%v ctrl=%v width=%v",
			sink.metrics["ap_frequency"], sink.metrics["ap_control_frequency"], sink.metrics["ap_channel_width"])
	}
	if sink.metrics["ap_output_power"] != 23.0 || sink.metrics["ap_eirp"] != 42.0 {
		t.Errorf("unexpected AirMAX AC power metrics: pwr=%v eirp=%v",
			sink.metrics["ap_output_power"], sink.metrics["ap_eirp"])
	}
	if sink.metrics["peer_chain_imbalance"] != 1.0 || sink.metrics["peer_remote_imbalance"] != 2.0 {
		t.Errorf("expected 2x2 MIMO chain imbalances local=1.0 remote=2.0, got local=%v remote=%v",
			sink.metrics["peer_chain_imbalance"], sink.metrics["peer_remote_imbalance"])
	}
	if sink.metrics["peer_cinr_dl"] != 31.0 || sink.metrics["peer_cinr_ul"] != 29.0 {
		t.Errorf("expected CINR dl=31.0 ul=29.0, got dl=%v ul=%v", sink.metrics["peer_cinr_dl"], sink.metrics["peer_cinr_ul"])
	}
	// Verify local GPS metrics use global ap_gps_* keys and zero af_* metrics are emitted on AirMAX AC
	if sink.metrics["ap_gps_fix"] != 0.0 || sink.metrics["ap_lat"] != 19.432608 || sink.metrics["ap_lon"] != -99.133209 {
		t.Errorf("unexpected AirMAX AC global GPS metrics: fix=%v lat=%v lon=%v",
			sink.metrics["ap_gps_fix"], sink.metrics["ap_lat"], sink.metrics["ap_lon"])
	}
	for k := range sink.metrics {
		if len(k) >= 3 && k[:3] == "af_" {
			t.Errorf("unexpected AirFiber metric %q emitted for AirMAX AC device", k)
		}
	}
}

func TestCollectors_Fixture_AF5XHD_PtP_Strict(t *testing.T) {
	root := loadFixtureRoot(t, "../../testdata/fixtures/af5xhd_ptp.json")
	if fam := utils.DetectDeviceFamily(root); fam != "airfiber" {
		t.Fatalf("expected DetectDeviceFamily='airfiber' for af5xhd_ptp.json, got %q", fam)
	}

	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "AF5XHD-MASTER-01", "ip": "192.168.1.20"}
	ctx := context.Background()

	collector.ProcessSystem(ctx, root, apLabels, sink)
	collector.ProcessInterfaces(ctx, root, apLabels, sink)
	collector.ProcessWireless(ctx, root, apLabels, nil, sink)
	collector.ProcessAF5XHD(ctx, root, apLabels, nil, sink)
	collector.ProcessPeers(ctx, root, apLabels, sink)

	if sink.metrics["ap_gps_fix"] != 1.0 || sink.metrics["ap_gps_sats"] != 12.0 || sink.metrics["ap_gps_dim"] != 3.0 || sink.metrics["ap_gps_alt"] != 1650.0 || sink.metrics["ap_gps_dop"] != 0.74 {
		t.Errorf("unexpected global ap_gps_* metrics on AF5XHD: fix=%v sats=%v dim=%v alt=%v dop=%v",
			sink.metrics["ap_gps_fix"], sink.metrics["ap_gps_sats"], sink.metrics["ap_gps_dim"], sink.metrics["ap_gps_alt"], sink.metrics["ap_gps_dop"])
	}

	if sink.metrics["af_frequency_tx"] != 5995.0 || sink.metrics["af_frequency_rx"] != 6065.0 || sink.metrics["af_channel_width_rx"] != 30.0 {
		t.Errorf("unexpected AF5XHD split-frequency metrics: tx=%v rx=%v width_rx=%v",
			sink.metrics["af_frequency_tx"], sink.metrics["af_frequency_rx"], sink.metrics["af_channel_width_rx"])
	}
	if sink.metrics["af_dl_ratio"] != 50.0 || sink.metrics["af_frame_length"] != 2.5 || sink.metrics["af_gps_sync_state"] != 0.0 || sink.metrics["af_remote_gps_sync_state"] != 0.0 {
		t.Errorf("unexpected AF5XHD TDD sync metrics: dl_ratio=%v frame=%v gps_sync=%v rem_gps_sync=%v",
			sink.metrics["af_dl_ratio"], sink.metrics["af_frame_length"], sink.metrics["af_gps_sync_state"], sink.metrics["af_remote_gps_sync_state"])
	}
	if sink.metrics["af_rx_efficiency"] != 12.72 || sink.metrics["af_local_autopower"] != 0.0 || sink.metrics["af_remote_autopower"] != 0.0 {
		t.Errorf("unexpected AF5XHD efficiency/autopower metrics: eff=%v local_auto=%v rem_auto=%v",
			sink.metrics["af_rx_efficiency"], sink.metrics["af_local_autopower"], sink.metrics["af_remote_autopower"])
	}
	if sink.metrics["ap_frequency"] != 5995.0 || sink.metrics["ap_channel_width"] != 30.0 || sink.metrics["ap_snr"] != 47.0 {
		t.Errorf("expected unified wireless metrics on AF5XHD (freq=5995, width=30, snr=47), got freq=%v width=%v snr=%v",
			sink.metrics["ap_frequency"], sink.metrics["ap_channel_width"], sink.metrics["ap_snr"])
	}
}
