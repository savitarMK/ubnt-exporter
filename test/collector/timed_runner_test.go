package collector_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"ubnt-exporter/internal/collector"
	"ubnt-exporter/internal/logger"
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

// Regression: a recovered collector panic must be logged with device, collector and stack trace.
func TestRunCollectorTimedContextLogsPanic(t *testing.T) {
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	defer logger.SetOutput(os.Stdout)

	collector.RunCollectorTimedContext(context.Background(), "AP-SECTOR-01", "peers", func() {
		panic("simulated collector crash")
	})

	out := buf.String()
	for _, want := range []string{"AP-SECTOR-01", "Collector 'peers' panicked", "simulated collector crash", "goroutine"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in panic log, got: %q", want, out)
		}
	}
}
