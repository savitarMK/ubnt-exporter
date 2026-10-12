// Package definitions contains version info, label constants, and Prometheus metric definitions.
package definitions

var (
	// VERSION defines the application version, which can be overridden via -ldflags -X.
	VERSION = "1.1.1"
	// Revision holds the git commit hash injected during compilation.
	Revision = "unknown"
	// Branch holds the git branch name injected during compilation.
	Branch = "main"
	// BuildDate holds the build timestamp injected during compilation.
	BuildDate = "unknown"
)

// MetricType defines the type of Prometheus metric.
type MetricType string

// Supported metric types.
const (
	// TypeGauge represents a numerical gauge metric.
	TypeGauge MetricType = "gauge"
	// TypeCounter represents a monotonically increasing counter metric.
	TypeCounter MetricType = "counter"
	// TypeInfo represents a static informational metric with labels.
	TypeInfo MetricType = "info"
)

// MetricDefinition describes a Prometheus metric.
type MetricDefinition struct {
	Type   MetricType
	Name   string
	Help   string
	Labels []string
}

// BaseLabels are the default labels applied to most device metrics.
var BaseLabels = []string{"name", "ip"}

// MetricsDef contains all metric definitions matching the Python exporter.
var MetricsDef = map[string]MetricDefinition{
	"device_info": {
		Type:   TypeInfo,
		Name:   "ubnt_device_static_info",
		Help:   "Static device information with dynamic metadata",
		Labels: []string{"name", "ip"},
	},
	"scrape_error": {
		Type:   TypeCounter,
		Name:   "ubnt_scrape_error_total",
		Help:   "Total scrape errors categorized by reason",
		Labels: []string{"name", "ip", "reason"},
	},
	"scrape_success": {
		Type:   TypeGauge,
		Name:   "ubnt_scrape_success",
		Help:   "1 if scrape was successful, 0 otherwise",
		Labels: []string{"name", "ip"},
	},
	"cpu_usage": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_cpu_usage_percent",
		Help:   "CPU Usage %",
		Labels: []string{"name", "ip"},
	},
	"ram_usage": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_ram_usage_percent",
		Help:   "RAM Usage %",
		Labels: []string{"name", "ip"},
	},
	"uptime": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_uptime_seconds",
		Help:   "Uptime (seconds)",
		Labels: []string{"name", "ip"},
	},
	"ap_frequency": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_frequency_mhz",
		Help:   "Center Frequency",
		Labels: []string{"name", "ip"},
	},
	"ap_control_frequency": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_control_frequency_mhz",
		Help:   "Control Frequency",
		Labels: []string{"name", "ip"},
	},
	"ap_channel_width": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_channel_width_mhz",
		Help:   "Channel Width",
		Labels: []string{"name", "ip"},
	},
	"ap_output_power": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_output_power_dbm",
		Help:   "Output Power",
		Labels: []string{"name", "ip"},
	},
	"ap_noise": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_noise_floor_dbm",
		Help:   "Noise Floor",
		Labels: []string{"name", "ip"},
	},
	"ap_signal": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_signal_dbm",
		Help:   "AP Signal",
		Labels: []string{"name", "ip"},
	},
	"ap_snr": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_snr_db",
		Help:   "AP Signal-to-Noise Ratio (SNR) in dB",
		Labels: []string{"name", "ip"},
	},
	"ap_clients": {
		Type:   TypeGauge,
		Name:   "ubnt_connected_clients_total",
		Help:   "Connected Clients",
		Labels: []string{"name", "ip"},
	},
	"ap_eirp": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_output_power_eirp_dbm",
		Help:   "AP Effective Isotropic Radiated Power",
		Labels: []string{"name", "ip"},
	},
	"ap_lat": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_latitude",
		Help:   "Device Latitude",
		Labels: []string{"name", "ip"},
	},
	"ap_lon": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_longitude",
		Help:   "Device Longitude",
		Labels: []string{"name", "ip"},
	},
	"iface_rx_bytes": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_rx_bytes_total",
		Help:   "RX Bytes",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_bytes": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_tx_bytes_total",
		Help:   "TX Bytes",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_rx_errors": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_rx_errors_total",
		Help:   "RX Errors",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_errors": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_tx_errors_total",
		Help:   "TX Errors",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_rx_drops": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_rx_drops_total",
		Help:   "RX Drops",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_drops": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_tx_drops_total",
		Help:   "TX Drops",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_rx_rate": {
		Type:   TypeGauge,
		Name:   "ubnt_iface_rx_rate_bps",
		Help:   "RX Rate bps",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_rate": {
		Type:   TypeGauge,
		Name:   "ubnt_iface_tx_rate_bps",
		Help:   "TX Rate bps",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_rx_pps": {
		Type:   TypeGauge,
		Name:   "ubnt_iface_rx_pps",
		Help:   "RX PPS",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_pps": {
		Type:   TypeGauge,
		Name:   "ubnt_iface_tx_pps",
		Help:   "TX PPS",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_rx_packets": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_rx_packets_total",
		Help:   "Total RX Packets",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_packets": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_tx_packets_total",
		Help:   "Total TX Packets",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_rx_broadcast": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_rx_broadcast_total",
		Help:   "RX Broadcast Packets",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_broadcast": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_tx_broadcast_total",
		Help:   "TX Broadcast Packets",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_rx_multicast": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_rx_multicast_total",
		Help:   "RX Multicast Packets",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_multicast": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_tx_multicast_total",
		Help:   "TX Multicast Packets",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_rx_flow_ctrl": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_rx_flow_control_total",
		Help:   "RX Flow Control (Pause Frames)",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_flow_ctrl": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_tx_flow_control_total",
		Help:   "TX Flow Control (Pause Frames)",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_rx_jumbo": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_rx_jumbo_packets_total",
		Help:   "RX Jumbo Frames",
		Labels: []string{"name", "ip", "interface"},
	},
	"iface_tx_jumbo": {
		Type:   TypeCounter,
		Name:   "ubnt_iface_tx_jumbo_packets_total",
		Help:   "TX Jumbo Frames",
		Labels: []string{"name", "ip", "interface"},
	},
	"peer_info": {
		Type: TypeInfo,
		Name: "ubnt_peer_static_info",
		Help: "Peer Metadata",
		Labels: []string{
			"ap_name", "ap_ip", "peer_name", "peer_ip", "peer_mac",
			"peer_product", "peer_model", "peer_family", "peer_firmware", "peer_firmware_build",
		},
	},
	"peer_signal": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_signal_dbm",
		Help:   "Peer Signal",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_connection_time": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_connection_time_seconds",
		Help:   "Wireless Session Duration",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_eirp": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_eirp_dbm",
		Help:   "Remote Peer EIRP",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_link_score_dl24": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_link_score_dl_24h_percent",
		Help:   "Link Score DL 24h Avg",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_link_score_ul24": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_link_score_ul_24h_percent",
		Help:   "Link Score UL 24h Avg",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_imbalance": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_chain_imbalance_db",
		Help:   "Remote Chain Imbalance",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_signal": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_signal_dbm",
		Help:   "Peer Remote Signal",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_distance": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_distance_meters",
		Help:   "Distance",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_tx_rate": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_tx_rate_mbps",
		Help:   "TX Rate",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_rx_rate": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_rx_rate_mbps",
		Help:   "RX Rate",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_capacity_dl": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_capacity_dl_mbps",
		Help:   "Capacity DL",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_capacity_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_capacity_ul_mbps",
		Help:   "Capacity UL",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_airtime": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_airtime_dl_percent",
		Help:   "Airtime %",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_uptime": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_uptime_seconds",
		Help:   "Peer Uptime",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_noise_floor": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_noise_floor_dbm",
		Help:   "Peer Noise Floor",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_rx_bytes": {
		Type:   TypeCounter,
		Name:   "ubnt_peer_rx_bytes_total",
		Help:   "Peer RX Bytes",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_tx_bytes": {
		Type:   TypeCounter,
		Name:   "ubnt_peer_tx_bytes_total",
		Help:   "Peer TX Bytes",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_chain_imbalance": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_chain_imbalance_db",
		Help:   "Chain Imbalance",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_lat": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_latitude",
		Help:   "Latitude",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_lon": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_longitude",
		Help:   "Longitude",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_link_score_dl": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_link_score_dl_percent",
		Help:   "Link Score Download %",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_link_score_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_link_score_ul_percent",
		Help:   "Link Score Upload %",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_cinr_dl": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_cinr_dl_db",
		Help:   "CINR Download (dB)",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_cinr_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_cinr_ul_db",
		Help:   "CINR Upload (dB)",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_nss_dl": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_nss_dl",
		Help:   "Spatial Streams DL (MIMO Status)",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_nss_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_nss_ul",
		Help:   "Spatial Streams UL (MIMO Status)",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_capacity_ideal_dl": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_capacity_ideal_dl_mbps",
		Help:   "Ideal Capacity DL",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_capacity_ideal_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_capacity_ideal_ul_mbps",
		Help:   "Ideal Capacity UL",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_gain": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_antenna_gain_dbi",
		Help:   "Remote Antenna Gain",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_power": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_output_power_dbm",
		Help:   "Remote Output Power",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_mcs_tx": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_mcs_tx_index",
		Help:   "MCS TX Index",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_mcs_rx": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_mcs_rx_index",
		Help:   "MCS RX Index",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_signal_c0": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_signal_chain0_dbm",
		Help:   "Local Signal Chain 0",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_signal_c1": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_signal_chain1_dbm",
		Help:   "Local Signal Chain 1",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_signal_c0": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_signal_chain0_dbm",
		Help:   "Remote Signal Chain 0",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_signal_c1": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_signal_chain1_dbm",
		Help:   "Remote Signal Chain 1",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_noise": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_noise_floor_dbm",
		Help:   "Remote Noise Floor",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_airtime_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_airtime_ul_percent",
		Help:   "Airtime Upload %",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"af_link_score": {
		Type:   TypeGauge,
		Name:   "ubnt_af_link_score_percent",
		Help:   "AirFiber Link Score",
		Labels: []string{"name", "ip"},
	},
	"af_link_score_dl": {
		Type:   TypeGauge,
		Name:   "ubnt_af_link_score_dl_percent",
		Help:   "Link Score DL %",
		Labels: []string{"name", "ip"},
	},
	"af_link_score_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_af_link_score_ul_percent",
		Help:   "Link Score UL %",
		Labels: []string{"name", "ip"},
	},
	"af_capacity_dl": {
		Type:   TypeGauge,
		Name:   "ubnt_af_capacity_dl_bps",
		Help:   "Capacity DL (bps)",
		Labels: []string{"name", "ip"},
	},
	"af_capacity_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_af_capacity_ul_bps",
		Help:   "Capacity UL (bps)",
		Labels: []string{"name", "ip"},
	},
	"af_capacity_rx": {
		Type:   TypeGauge,
		Name:   "ubnt_af_capacity_rx_bps",
		Help:   "AirFiber Capacity RX",
		Labels: []string{"name", "ip"},
	},
	"af_capacity_tx": {
		Type:   TypeGauge,
		Name:   "ubnt_af_capacity_tx_bps",
		Help:   "AirFiber Capacity TX",
		Labels: []string{"name", "ip"},
	},
	"af_cinr_dl": {
		Type:   TypeGauge,
		Name:   "ubnt_af_cinr_dl_db",
		Help:   "CINR DL (dB)",
		Labels: []string{"name", "ip"},
	},
	"af_cinr_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_af_cinr_ul_db",
		Help:   "CINR UL (dB)",
		Labels: []string{"name", "ip"},
	},
	"af_modulation_rate": {
		Type:   TypeGauge,
		Name:   "ubnt_af_modulation_rate_x",
		Help:   "Modulation Rate (10x, 8x, etc)",
		Labels: []string{"name", "ip", "direction"},
	},
	"af_gps_sats": {
		Type:   TypeGauge,
		Name:   "ubnt_af_gps_satellites",
		Help:   "GPS Satellites Count",
		Labels: []string{"name", "ip"},
	},
	"af_rx_efficiency": {
		Type:   TypeGauge,
		Name:   "ubnt_af_rx_efficiency_percent",
		Help:   "RX Efficiency (Airtime efficiency)",
		Labels: []string{"name", "ip"},
	},
	"af_ideal_signal": {
		Type:   TypeGauge,
		Name:   "ubnt_af_ideal_signal_dbm",
		Help:   "Ideal Signal calculated by device",
		Labels: []string{"name", "ip"},
	},
	"af_mcs_index": {
		Type:   TypeGauge,
		Name:   "ubnt_af_mcs_index",
		Help:   "MCS Index (0-12)",
		Labels: []string{"name", "ip", "direction"},
	},
	"af_dfs_cac_rem": {
		Type:   TypeGauge,
		Name:   "ubnt_af_dfs_cac_remaining_seconds",
		Help:   "DFS CAC Remaining Time",
		Labels: []string{"name", "ip"},
	},
	"af_dfs_status": {
		Type:   TypeGauge,
		Name:   "ubnt_af_dfs_blocked_status",
		Help:   "1 if DFS is blocking TX, 0 otherwise",
		Labels: []string{"name", "ip"},
	},
	"af_temperature": {
		Type:   TypeGauge,
		Name:   "ubnt_af_temperature_celsius",
		Help:   "Device Temperature",
		Labels: []string{"name", "ip"},
	},
	"af_gps_alt": {
		Type:   TypeGauge,
		Name:   "ubnt_af_gps_altitude_meters",
		Help:   "GPS Altitude",
		Labels: []string{"name", "ip"},
	},
	"af_dl_ratio": {
		Type:   TypeGauge,
		Name:   "ubnt_af_duty_cycle_dl_percent",
		Help:   "Duty Cycle DL Ratio",
		Labels: []string{"name", "ip"},
	},
	"af_remote_rx_eff": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_rx_efficiency_percent",
		Help:   "Remote Side RX Efficiency",
		Labels: []string{"name", "ip"},
	},
	"af_control_frequency": {
		Type:   TypeGauge,
		Name:   "ubnt_af_control_frequency_mhz",
		Help:   "Control Frequency",
		Labels: []string{"name", "ip"},
	},
	"af_frequency_tx": {
		Type:   TypeGauge,
		Name:   "ubnt_af_frequency_tx_mhz",
		Help:   "TX Frequency",
		Labels: []string{"name", "ip"},
	},
	"af_frequency_rx": {
		Type:   TypeGauge,
		Name:   "ubnt_af_frequency_rx_mhz",
		Help:   "RX Frequency",
		Labels: []string{"name", "ip"},
	},
	"af_frame_length": {
		Type:   TypeGauge,
		Name:   "ubnt_af_frame_length_ms",
		Help:   "Frame Length Duration",
		Labels: []string{"name", "ip"},
	},
	"af_gps_sync_state": {
		Type:   TypeGauge,
		Name:   "ubnt_af_gps_sync_status",
		Help:   "GPS Sync State (1=Sync, 0=No Sync)",
		Labels: []string{"name", "ip"},
	},
	"af_remote_gain": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_antenna_gain_dbi",
		Help:   "Remote Peer Antenna Gain",
		Labels: []string{"name", "ip"},
	},
	"af_remote_cable_loss": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_cable_loss_db",
		Help:   "Remote Peer Cable Loss",
		Labels: []string{"name", "ip"},
	},
	"af_peer_tx_bytes": {
		Type:   TypeCounter,
		Name:   "ubnt_af_peer_tx_bytes_total",
		Help:   "Wireless Peer TX Bytes",
		Labels: []string{"name", "ip", "peer_name"},
	},
	"af_peer_rx_bytes": {
		Type:   TypeCounter,
		Name:   "ubnt_af_peer_rx_bytes_total",
		Help:   "Wireless Peer RX Bytes",
		Labels: []string{"name", "ip", "peer_name"},
	},
	"af_peer_tx_rate": {
		Type:   TypeGauge,
		Name:   "ubnt_af_peer_tx_rate_bps",
		Help:   "Wireless Peer TX Rate",
		Labels: []string{"name", "ip", "peer_name"},
	},
	"af_peer_rx_rate": {
		Type:   TypeGauge,
		Name:   "ubnt_af_peer_rx_rate_bps",
		Help:   "Wireless Peer RX Rate",
		Labels: []string{"name", "ip", "peer_name"},
	},
	"af_peer_tx_pps": {
		Type:   TypeGauge,
		Name:   "ubnt_af_peer_tx_pps",
		Help:   "Wireless Peer TX PPS",
		Labels: []string{"name", "ip", "peer_name"},
	},
	"af_peer_rx_pps": {
		Type:   TypeGauge,
		Name:   "ubnt_af_peer_rx_pps",
		Help:   "Wireless Peer RX PPS",
		Labels: []string{"name", "ip", "peer_name"},
	},
	"af_nss_dl": {
		Type:   TypeGauge,
		Name:   "ubnt_af_nss_dl",
		Help:   "Spatial Streams DL (MIMO)",
		Labels: []string{"name", "ip"},
	},
	"af_nss_ul": {
		Type:   TypeGauge,
		Name:   "ubnt_af_nss_ul",
		Help:   "Spatial Streams UL (MIMO)",
		Labels: []string{"name", "ip"},
	},
	"af_capacity_dl_ideal": {
		Type:   TypeGauge,
		Name:   "ubnt_af_capacity_dl_ideal_bps",
		Help:   "Ideal Capacity DL",
		Labels: []string{"name", "ip"},
	},
	"af_capacity_ul_ideal": {
		Type:   TypeGauge,
		Name:   "ubnt_af_capacity_ul_ideal_bps",
		Help:   "Ideal Capacity UL",
		Labels: []string{"name", "ip"},
	},
	"af_signal_chain0": {
		Type:   TypeGauge,
		Name:   "ubnt_af_signal_chain0_dbm",
		Help:   "Local Signal Chain 0",
		Labels: []string{"name", "ip"},
	},
	"af_signal_chain1": {
		Type:   TypeGauge,
		Name:   "ubnt_af_signal_chain1_dbm",
		Help:   "Local Signal Chain 1",
		Labels: []string{"name", "ip"},
	},
	"af_remote_chain0": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_signal_chain0_dbm",
		Help:   "Remote Signal Chain 0",
		Labels: []string{"name", "ip"},
	},
	"af_remote_chain1": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_signal_chain1_dbm",
		Help:   "Remote Signal Chain 1",
		Labels: []string{"name", "ip"},
	},
	"af_remote_noise": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_noise_floor_dbm",
		Help:   "Remote Noise Floor",
		Labels: []string{"name", "ip"},
	},
	"af_peer_cpu": {
		Type:   TypeGauge,
		Name:   "ubnt_af_peer_cpu_usage_percent",
		Help:   "Remote Peer CPU Usage",
		Labels: []string{"name", "ip", "peer_name"},
	},
	"af_peer_ram": {
		Type:   TypeGauge,
		Name:   "ubnt_af_peer_ram_usage_percent",
		Help:   "Remote Peer RAM Usage",
		Labels: []string{"name", "ip", "peer_name"},
	},
	"af_remote_conducted": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_output_power_dbm",
		Help:   "Remote Peer Conducted Power",
		Labels: []string{"name", "ip"},
	},
	"af_remote_eirp": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_eirp_dbm",
		Help:   "Remote Peer EIRP",
		Labels: []string{"name", "ip"},
	},
	"af_remote_autopower": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_autopower_state",
		Help:   "Remote Auto Power (1=On, 0=Off)",
		Labels: []string{"name", "ip"},
	},
	"af_local_autopower": {
		Type:   TypeGauge,
		Name:   "ubnt_af_local_autopower_state",
		Help:   "Local Auto Power (1=On, 0=Off)",
		Labels: []string{"name", "ip"},
	},
	"af_gps_dop": {
		Type:   TypeGauge,
		Name:   "ubnt_af_gps_dop",
		Help:   "GPS Dilution of Precision (Lower is better)",
		Labels: []string{"name", "ip"},
	},
	"af_dfs_lockout_count": {
		Type:   TypeGauge,
		Name:   "ubnt_af_dfs_lockout_total",
		Help:   "Number of DFS frequencies locked out",
		Labels: []string{"name", "ip"},
	},
	"af_capacity_combined": {
		Type:   TypeGauge,
		Name:   "ubnt_af_capacity_combined_bps",
		Help:   "Total combined DL+UL capacity (bps)",
		Labels: []string{"name", "ip"},
	},
	"af_capacity_combined_ideal": {
		Type:   TypeGauge,
		Name:   "ubnt_af_capacity_combined_ideal_bps",
		Help:   "Total combined ideal DL+UL capacity (bps)",
		Labels: []string{"name", "ip"},
	},
	"af_signal_delta_ideal": {
		Type:   TypeGauge,
		Name:   "ubnt_af_signal_delta_ideal_db",
		Help:   "Signal difference between measured and ideal signal (dB)",
		Labels: []string{"name", "ip"},
	},
	"af_link_state": {
		Type:   TypeGauge,
		Name:   "ubnt_af_link_state",
		Help:   "Radio Link State (1=connected, 0=disconnected)",
		Labels: []string{"name", "ip"},
	},
	"af_channel_width_rx": {
		Type:   TypeGauge,
		Name:   "ubnt_af_channel_width_rx_mhz",
		Help:   "RX Channel Width (MHz)",
		Labels: []string{"name", "ip"},
	},
	"af_remote_gps_sync_state": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_gps_sync_status",
		Help:   "Remote peer GPS Sync State (1=Sync, 0=No Sync)",
		Labels: []string{"name", "ip"},
	},
	"af_remote_ideal_signal": {
		Type:   TypeGauge,
		Name:   "ubnt_af_remote_ideal_signal_dbm",
		Help:   "Ideal signal at remote peer radio (dBm)",
		Labels: []string{"name", "ip"},
	},
	"af_service_uptime": {
		Type:   TypeGauge,
		Name:   "ubnt_af_service_uptime_seconds",
		Help:   "Wireless radio service uptime in seconds",
		Labels: []string{"name", "ip"},
	},
	"af_power_time": {
		Type:   TypeGauge,
		Name:   "ubnt_af_power_time_seconds",
		Help:   "Device uptime since cold power-on in seconds",
		Labels: []string{"name", "ip"},
	},
	"ram_free_bytes": {
		Type:   TypeGauge,
		Name:   "ubnt_ram_free_bytes",
		Help:   "Free system RAM in bytes",
		Labels: []string{"name", "ip"},
	},
	"ram_total_bytes": {
		Type:   TypeGauge,
		Name:   "ubnt_ram_total_bytes",
		Help:   "Total system RAM in bytes",
		Labels: []string{"name", "ip"},
	},
	"device_power_time": {
		Type:   TypeGauge,
		Name:   "ubnt_device_power_time_seconds",
		Help:   "Device uptime since cold power-on in seconds",
		Labels: []string{"name", "ip"},
	},
	"ap_link_state": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_link_state",
		Help:   "AP Radio Link State (1=connected, 0=disconnected)",
		Labels: []string{"name", "ip"},
	},
	"ap_service_uptime": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_service_uptime_seconds",
		Help:   "AP wireless radio service uptime in seconds",
		Labels: []string{"name", "ip"},
	},
	"ap_dfs_status": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_dfs_status",
		Help:   "AP Dynamic Frequency Selection status (0=normal, 1=CAC scanning)",
		Labels: []string{"name", "ip"},
	},
	"ap_dfs_cac_rem": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_dfs_cac_rem_seconds",
		Help:   "AP DFS CAC remaining countdown in seconds",
		Labels: []string{"name", "ip"},
	},
	"ap_dfs_lockout_count": {
		Type:   TypeGauge,
		Name:   "ubnt_ap_dfs_lockout_count",
		Help:   "Number of DFS frequencies locked out on AP",
		Labels: []string{"name", "ip"},
	},
	"peer_cpu": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_cpu_usage",
		Help:   "Remote station CPU usage percent",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_ram": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_ram_usage",
		Help:   "Remote station RAM usage percent",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_ram_free": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_ram_free_bytes",
		Help:   "Remote station free RAM in bytes",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_ram_total": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_ram_total_bytes",
		Help:   "Remote station total RAM in bytes",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_power_time": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_power_time_seconds",
		Help:   "Remote station uptime since cold power-on in seconds",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_ideal_signal": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_ideal_signal_dbm",
		Help:   "Theoretical ideal signal calculated by airOS (dBm)",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_signal_delta_ideal": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_signal_delta_ideal_db",
		Help:   "Signal delta between measured and theoretical ideal signal (dB)",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_ideal_signal": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_ideal_signal_dbm",
		Help:   "Theoretical ideal signal at remote station (dBm)",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_signal_delta_ideal": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_signal_delta_ideal_db",
		Help:   "Remote signal delta between measured and theoretical ideal signal (dB)",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_capacity_combined": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_capacity_combined_mbps",
		Help:   "Total combined DL+UL capacity in Mbps",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_capacity_combined_ideal": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_capacity_combined_ideal_mbps",
		Help:   "Total combined ideal DL+UL capacity in Mbps",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_mcs_ideal_tx": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_mcs_ideal_tx_index",
		Help:   "Ideal MCS TX Index",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_mcs_ideal_rx": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_mcs_ideal_rx_index",
		Help:   "Ideal MCS RX Index",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_mcs_rate_tx": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_mcs_rate_tx",
		Help:   "Transmit modulation multiplier",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_mcs_rate_rx": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_mcs_rate_rx",
		Help:   "Receive modulation multiplier",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_cable_loss": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_cable_loss_db",
		Help:   "Remote Peer Cable Loss in dB",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_snr": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_snr_db",
		Help:   "Signal-to-Noise Ratio (SNR) in dB measured at the AP",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_remote_snr": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_remote_snr_db",
		Help:   "Signal-to-Noise Ratio (SNR) in dB measured at the remote station",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_rx_packets": {
		Type:   TypeCounter,
		Name:   "ubnt_peer_rx_packets_total",
		Help:   "Total packets received from the remote station",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_tx_packets": {
		Type:   TypeCounter,
		Name:   "ubnt_peer_tx_packets_total",
		Help:   "Total packets transmitted to the remote station",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_rx_pps": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_rx_pps",
		Help:   "Packets per second received from the remote station",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
	"peer_tx_pps": {
		Type:   TypeGauge,
		Name:   "ubnt_peer_tx_pps",
		Help:   "Packets per second transmitted to the remote station",
		Labels: []string{"ap_name", "ap_ip", "peer_name", "peer_ip"},
	},
}
