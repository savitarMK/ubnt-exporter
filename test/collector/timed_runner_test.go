package collector_test

import (
	"testing"

	"ubnt-exporter/internal/collector"
)

func TestRunCollectorTimed(t *testing.T) {
	// Normal execution
	ran := false
	collector.RunCollectorTimed("test_normal", func() {
		ran = true
	})
	if !ran {
		t.Errorf("expected normal collector to run")
	}

	// Panic execution should be caught and not crash
	panicked := false
	collector.RunCollectorTimed("test_panic", func() {
		panicked = true
		panic("simulated collector crash")
	})
	if !panicked {
		t.Errorf("expected panicking collector to execute")
	}
}
