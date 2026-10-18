package collector_test

import (
	"context"
	"testing"

	"ubnt-exporter/internal/collector"
)

func TestProcessInterfaces(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "test-ap", "ip": "10.0.0.1"}

	data := map[string]any{
		"interfaces": []any{
			map[string]any{
				"id": "eth0",
				"statistics": map[string]any{
					"rxBytes":   1048576.0,
					"txBytes":   2097152.0,
					"rxErrors":  2.0,
					"txErrors":  0.0,
					"rxDropped": 5.0,
					"txDropped": 1.0,
				},
			},
		},
	}

	collector.ProcessInterfaces(context.Background(), data, apLabels, sink)

	if sink.metrics["iface_rx_bytes"] != 1048576.0 {
		t.Errorf("expected iface_rx_bytes 1048576.0, got %v", sink.metrics["iface_rx_bytes"])
	}
	if sink.metrics["iface_tx_bytes"] != 2097152.0 {
		t.Errorf("expected iface_tx_bytes 2097152.0, got %v", sink.metrics["iface_tx_bytes"])
	}
	if sink.metrics["iface_rx_errors"] != 2.0 {
		t.Errorf("expected iface_rx_errors 2.0, got %v", sink.metrics["iface_rx_errors"])
	}
	if sink.metrics["iface_rx_drops"] != 5.0 {
		t.Errorf("expected iface_rx_drops 5.0, got %v", sink.metrics["iface_rx_drops"])
	}
}

func TestProcessInterfaces_AirMaxAC_ClassificationAndPhysicalState(t *testing.T) {
	sink := newMockMetricSink()
	apLabels := map[string]string{"name": "ap-ac-sector", "ip": "10.0.4.155"}

	data := map[string]any{
		"interfaces": []any{
			map[string]any{
				"id":      "eth0",
				"name":    "LAN0",
				"enabled": true,
				"plugged": true,
				"speed":   1000.0,
				"duplex":  true,
				"mtu":     1500.0,
				"snr":     30.0,
				"statistics": map[string]any{
					"txDropped":   0.0,
					"rxDropped":   0.0,
					"txErrors":    0.0,
					"rxErrors":    4.0,
					"txRate":      2994.0,
					"rxRate":      14733.0,
					"txBytes":     1117320659.0,
					"rxBytes":     19497011432.0,
					"txPackets":   5472348.0,
					"rxPackets":   18455112.0,
					"txPPS":       2.0,
					"rxPPS":       14.0,
					"txJumbo":     0.0,
					"rxJumbo":     0.0,
					"txFlowCtrl":  0.0,
					"rxFlowCtrl":  0.0,
					"txBroadcast": 0.0,
					"rxBroadcast": 0.0,
					"txMulticast": 0.0,
					"rxMulticast": 0.0,
				},
			},
			map[string]any{
				"id":         "ath0",
				"name":       "",
				"statistics": map[string]any{"txDropped": 1.0, "rxBytes": 1299795687.0},
			},
			map[string]any{
				"id":         "br0",
				"name":       "",
				"statistics": map[string]any{"rxBytes": 612915807.0},
			},
			map[string]any{
				"id":         "ath0.99",
				"name":       "",
				"statistics": map[string]any{"txBytes": 118681045.0},
			},
			map[string]any{
				"id":         "eth0.99",
				"name":       "MGMT-VLAN",
				"statistics": map[string]any{"txDropped": 4.0, "rxBytes": 106567816.0},
			},
			map[string]any{
				"id":         "br1",
				"name":       "",
				"statistics": map[string]any{"rxBytes": 101839766.0},
			},
		},
	}

	collector.ProcessInterfaces(context.Background(), data, apLabels, sink)

	infoRecords := sink.records["iface_info"]
	if len(infoRecords) != 6 {
		t.Fatalf("expected 6 iface_info records, got %d", len(infoRecords))
	}

	expectedTypes := map[string]struct {
		ifaceType string
		parent    string
		vlan      string
		name      string
	}{
		"eth0":    {"ethernet", "eth0", "", "LAN0"},
		"ath0":    {"wireless", "ath0", "", "ath0"},
		"br0":     {"bridge", "br0", "", "br0"},
		"ath0.99": {"vlan", "ath0", "99", "ath0.99"},
		"eth0.99": {"vlan", "eth0", "99", "MGMT-VLAN"},
		"br1":     {"bridge", "br1", "", "br1"},
	}

	for _, rec := range infoRecords {
		id := rec.labels["interface"]
		exp, ok := expectedTypes[id]
		if !ok {
			t.Errorf("unexpected interface id %q", id)
			continue
		}
		if rec.labels["interface_type"] != exp.ifaceType {
			t.Errorf("interface %s: expected type %q, got %q", id, exp.ifaceType, rec.labels["interface_type"])
		}
		if rec.labels["parent_interface"] != exp.parent {
			t.Errorf("interface %s: expected parent %q, got %q", id, exp.parent, rec.labels["parent_interface"])
		}
		if rec.labels["vlan_id"] != exp.vlan {
			t.Errorf("interface %s: expected vlan_id %q, got %q", id, exp.vlan, rec.labels["vlan_id"])
		}
		if rec.labels["interface_name"] != exp.name {
			t.Errorf("interface %s: expected interface_name %q, got %q", id, exp.name, rec.labels["interface_name"])
		}
	}

	if sink.metrics["iface_enabled"] != 1.0 {
		t.Errorf("expected iface_enabled 1.0, got %v", sink.metrics["iface_enabled"])
	}
	if sink.metrics["iface_plugged"] != 1.0 {
		t.Errorf("expected iface_plugged 1.0, got %v", sink.metrics["iface_plugged"])
	}
	if sink.metrics["iface_speed"] != 1000.0 {
		t.Errorf("expected iface_speed 1000.0, got %v", sink.metrics["iface_speed"])
	}
	if sink.metrics["iface_duplex"] != 1.0 {
		t.Errorf("expected iface_duplex 1.0, got %v", sink.metrics["iface_duplex"])
	}
	if sink.metrics["iface_mtu"] != 1500.0 {
		t.Errorf("expected iface_mtu 1500.0, got %v", sink.metrics["iface_mtu"])
	}
	if sink.metrics["iface_snr"] != 30.0 {
		t.Errorf("expected iface_snr 30.0, got %v", sink.metrics["iface_snr"])
	}
}
