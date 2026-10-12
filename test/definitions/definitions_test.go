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
		"af_capacity_combined",
		"af_capacity_combined_ideal",
		"af_signal_delta_ideal",
		"af_link_state",
		"af_channel_width_rx",
		"af_remote_gps_sync_state",
		"af_remote_ideal_signal",
		"af_service_uptime",
		"af_power_time",
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
