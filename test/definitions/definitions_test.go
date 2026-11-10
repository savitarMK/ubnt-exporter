package definitions_test

import (
	"strings"
	"testing"

	"ubnt-exporter/internal/definitions"
)

func TestMetricsDefinitionsIntegrity(t *testing.T) {
	if len(definitions.MetricsDef) == 0 {
		t.Fatalf("MetricsDef map is empty")
	}

	requiredMetrics := []string{
		"device_info",
		"scrape_success",
		"scrape_error",
		"cpu_usage",
		"ram_usage",
		"uptime",
		"ap_signal",
		"ap_noise",
		"ap_clients",
		"peer_signal",
		"peer_chain_imbalance",
		"iface_rx_bytes",
		"iface_tx_bytes",
		"af_frequency_tx",
		"af_frequency_rx",
		"af_channel_width_rx",
		"af_dl_ratio",
		"af_frame_length",
		"af_gps_sync_state",
		"af_rx_efficiency",
		"af_local_autopower",
		"af_remote_gps_sync_state",
		"af_remote_autopower",
		"ram_free_bytes",
		"ram_total_bytes",
		"device_power_time",
		"ap_link_state",
		"ap_service_uptime",
		"ap_dfs_status",
		"ap_dfs_cac_rem",
		"ap_dfs_lockout_count",
		"peer_cpu",
		"peer_ram",
		"peer_ram_free",
		"peer_ram_total",
		"peer_power_time",
		"peer_ideal_signal",
		"peer_signal_delta_ideal",
		"peer_remote_ideal_signal",
		"peer_remote_signal_delta_ideal",
		"peer_capacity_combined",
		"peer_capacity_combined_ideal",
		"peer_mcs_ideal_tx",
		"peer_mcs_ideal_rx",
		"peer_mcs_rate_tx",
		"peer_mcs_rate_rx",
		"peer_remote_cable_loss",
		"ap_snr",
		"peer_snr",
		"peer_remote_snr",
		"peer_rx_packets",
		"peer_tx_packets",
		"peer_rx_pps",
		"peer_tx_pps",
		"device_timestamp",
		"cpu_info",
		"ram_used_bytes",
		"ap_gps_fix",
		"device_temperature",
		"iface_info",
		"iface_enabled",
		"iface_plugged",
		"iface_speed",
		"iface_duplex",
		"iface_mtu",
		"iface_snr",
		"ap_service_time",
		"ap_dfs_enabled",
		"ap_dfs_cac_duration",
		"peer_service_uptime",
		"peer_service_time",
		"peer_connected",
		"peer_active",
		"peer_ideal_signal_c0",
		"peer_ideal_signal_c1",
		"peer_remote_ideal_signal_c0",
		"peer_remote_ideal_signal_c1",
		"peer_gps_fix",
		"peer_tx_rate_bps",
		"peer_rx_rate_bps",
		"peer_mcs_rx_packets",
		"peer_remote_mcs_rx_packets",
		"ap_gps_sats",
		"ap_gps_alt",
		"ap_gps_dop",
		"ap_gps_dim",
		"ap_gps_time_sync",
		"device_ntp_sync_enabled",
		"device_ntp_last_sync",
		"peer_gps_alt",
		"peer_gps_sats",
		"peer_gps_dop",
		"peer_gps_dim",
		"peer_gps_time_sync",
		"peer_ntp_sync_enabled",
		"peer_ntp_last_sync",
		"peer_remote_rx_efficiency",
	}

	seenNames := make(map[string]string, len(definitions.MetricsDef))
	for key, def := range definitions.MetricsDef {
		if prevKey, dup := seenNames[def.Name]; dup {
			t.Errorf("duplicate Prometheus metric Name %q shared by keys %q and %q", def.Name, prevKey, key)
		}
		seenNames[def.Name] = key

		if !strings.HasPrefix(def.Name, "ubnt_") {
			t.Errorf("metric %q Name does not have 'ubnt_' prefix: %s", key, def.Name)
		}
		if len(strings.TrimSpace(def.Help)) < 20 || !strings.HasSuffix(def.Help, ".") {
			t.Errorf("metric %q (%s) must have a descriptive Help sentence (>=20 chars ending with '.'): %q", key, def.Name, def.Help)
		}
		if def.Type == definitions.TypeCounter && !strings.HasSuffix(def.Name, "_total") {
			t.Errorf("counter metric %q (%s) must end with '_total'", key, def.Name)
		}
		if def.Type != definitions.TypeCounter && strings.HasSuffix(def.Name, "_total") {
			t.Errorf("non-counter metric %q (%s, type=%s) must not end with '_total'", key, def.Name, def.Type)
		}
		if def.Type == definitions.TypeInfo && !strings.HasSuffix(def.Name, "_info") {
			t.Errorf("info metric %q (%s) must end with '_info'", key, def.Name)
		}
	}

	if definitions.MetricsDef["ap_clients"].Name != "ubnt_ap_connected_clients" {
		t.Errorf("expected ap_clients Name 'ubnt_ap_connected_clients', got %q", definitions.MetricsDef["ap_clients"].Name)
	}
	if definitions.MetricsDef["peer_cpu"].Name != "ubnt_peer_cpu_usage_percent" {
		t.Errorf("expected peer_cpu Name 'ubnt_peer_cpu_usage_percent', got %q", definitions.MetricsDef["peer_cpu"].Name)
	}
	if definitions.MetricsDef["peer_ram"].Name != "ubnt_peer_ram_usage_percent" {
		t.Errorf("expected peer_ram Name 'ubnt_peer_ram_usage_percent', got %q", definitions.MetricsDef["peer_ram"].Name)
	}

	for _, req := range requiredMetrics {
		def, ok := definitions.MetricsDef[req]
		if !ok {
			t.Errorf("missing required metric definition %q", req)
			continue
		}
		if def.Name == "" {
			t.Errorf("metric %q has empty Name", req)
		}
		if !strings.HasPrefix(def.Name, "ubnt_") {
			t.Errorf("metric %q Name does not have 'ubnt_' prefix: %s", req, def.Name)
		}
		if def.Help == "" {
			t.Errorf("metric %q has empty Help description", req)
		}
		if def.Type != definitions.TypeGauge && def.Type != definitions.TypeCounter && def.Type != definitions.TypeInfo {
			t.Errorf("metric %q has invalid type: %s", req, def.Type)
		}
		if len(def.Labels) == 0 {
			t.Errorf("metric %q has no labels configured", req)
		}
	}
}

func TestVersionDefaults(t *testing.T) {
	if definitions.VERSION == "" {
		t.Errorf("VERSION should not be empty")
	}
	if definitions.Branch == "" {
		t.Errorf("Branch should not be empty")
	}
}
