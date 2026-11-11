package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubnt-exporter/internal/config"
)

// Regression: typos in configuration keys used to be ignored silently.
func TestUnknownFieldsAreReported(t *testing.T) {
	dir := t.TempDir()
	sys := filepath.Join(dir, "system.yaml")
	inv := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(sys, []byte("server:\n  port: 9100\n  scrape_intervall: 10\nlogging:\n  levle: DEBUG\n"), 0600)
	_ = os.WriteFile(inv, []byte("devices:\n  - name: AP1\n    ip: 10.0.0.1\n    pasword: x\n"), 0600)

	sw := config.UnknownSystemFields(sys)
	if len(sw) != 2 || !strings.Contains(sw[0], "scrape_intervall") || !strings.Contains(sw[0], "line 3") || !strings.Contains(sw[1], "levle") {
		t.Errorf("unexpected system warnings: %v", sw)
	}
	iw := config.UnknownInventoryFields(inv)
	if len(iw) != 1 || !strings.Contains(iw[0], "pasword") || !strings.HasPrefix(iw[0], "config.yaml:") {
		t.Errorf("unexpected inventory warnings: %v", iw)
	}

	// The lenient loader must keep accepting these files (warnings are not fatal).
	if _, err := config.LoadSystemConfig(sys); err != nil {
		t.Errorf("LoadSystemConfig must still accept unknown keys: %v", err)
	}
	if _, err := config.LoadInventoryConfig(inv); err != nil {
		t.Errorf("LoadInventoryConfig must still accept unknown keys: %v", err)
	}
}

// Shipped examples must only use known keys.
func TestExampleConfigsHaveNoUnknownFields(t *testing.T) {
	if w := config.UnknownSystemFields("../../examples/system.yaml.example"); len(w) > 0 {
		t.Errorf("system.yaml.example has unknown keys: %v", w)
	}
	if w := config.UnknownInventoryFields("../../examples/config.yaml.example"); len(w) > 0 {
		t.Errorf("config.yaml.example has unknown keys: %v", w)
	}
}
