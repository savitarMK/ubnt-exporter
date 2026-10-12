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
