package collector_test

import (
	"context"
	"testing"

	"ubnt-exporter/internal/collector"
	"ubnt-exporter/internal/metrics"
)

func peerWith(hostname, mgmtIP, mac string, signal float64) map[string]any {
	common := map[string]any{"identification": map[string]any{}}
	if hostname != "" {
		common["hostname"] = hostname
	}
	if mgmtIP != "" {
		common["mgmtIp"] = mgmtIP
	}
	if mac != "" {
		common["identification"].(map[string]any)["mac"] = mac
	}
	return map[string]any{
		"common": common,
		"local":  []any{map[string]any{"linkQuality": map[string]any{"signal": signal}}},
	}
}

// Regression: stations sharing hostname/mgmtIp (or reporting none) must not overwrite each
// other's series in the store; peer_mac is the unique identity label.
func TestProcessPeersUniqueIdentity(t *testing.T) {
	cases := []struct {
		name  string
		peers []any
		macs  []string
	}{
		{"missing hostname and ip", []any{peerWith("", "", "AA:AA:AA:00:00:01", -50), peerWith("", "", "AA:AA:AA:00:00:02", -70)},
			[]string{"aa:aa:aa:00:00:01", "aa:aa:aa:00:00:02"}},
		{"duplicated hostname and ip", []any{peerWith("CPE", "192.168.1.20", "aa:00:00:00:00:01", -50), peerWith("CPE", "192.168.1.20", "aa:00:00:00:00:02", -70)},
			[]string{"aa:00:00:00:00:01", "aa:00:00:00:00:02"}},
		{"no identity at all", []any{peerWith("", "", "", -50), peerWith("", "", "", -70)},
			[]string{"unknown-0", "unknown-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := map[string]any{"wireless": map[string]any{"peers": tc.peers}}
			apLabels := map[string]string{"name": "AP", "ip": "10.0.0.1"}

			buf := metrics.NewBuffer()
			collector.ProcessPeers(context.Background(), root, apLabels, buf)
			store := metrics.NewStore()
			buf.Commit(store, "AP", "10.0.0.1")

			if got := store.CountMetricDataPoints("peer_signal"); got != len(tc.peers) {
				t.Errorf("expected %d distinct peer_signal series, got %d", len(tc.peers), got)
			}
			if got := store.CountMetricDataPoints("peer_info"); got != len(tc.peers) {
				t.Errorf("expected %d distinct peer_info series, got %d", len(tc.peers), got)
			}

			sink := newMockMetricSink()
			collector.ProcessPeers(context.Background(), root, apLabels, sink)
			for i, rec := range sink.records["peer_signal"] {
				if rec.labels["peer_mac"] != tc.macs[i] {
					t.Errorf("peer %d: expected peer_mac %q, got %q", i, tc.macs[i], rec.labels["peer_mac"])
				}
			}
		})
	}
}
