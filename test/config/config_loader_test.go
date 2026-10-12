package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"ubnt-exporter/internal/config"
)

func TestLoadSystemConfigAndApplyDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "system.yaml")

	content := `
server:
  port: 8080
  scrape_interval: 45
performance:
  max_workers: 50
logging:
  level: "DEBUG"
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := config.LoadSystemConfig(configFile)
	if err != nil {
		t.Fatalf("LoadSystemConfig failed: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.ScrapeInterval != 45 {
		t.Errorf("expected scrape interval 45, got %d", cfg.Server.ScrapeInterval)
	}
	if cfg.Performance.MaxWorkers != 50 {
		t.Errorf("expected max workers 50, got %d", cfg.Performance.MaxWorkers)
	}
	if cfg.Performance.RequestTimeout != 15 {
		t.Errorf("expected default request timeout 15, got %d", cfg.Performance.RequestTimeout)
	}
	if cfg.Logging.Level != "DEBUG" {
		t.Errorf("expected level DEBUG, got %s", cfg.Logging.Level)
	}

	// Test ApplySystemDefaults
	emptyCfg := &config.SystemConfig{}
	applied := config.ApplySystemDefaults(emptyCfg)
	if applied.Server.Port != 8000 {
		t.Errorf("expected applied port 8000, got %d", applied.Server.Port)
	}
	if applied.Performance.MaxWorkers != 100 {
		t.Errorf("expected applied max workers 100, got %d", applied.Performance.MaxWorkers)
	}
}

func TestIsTargetAllowed(t *testing.T) {
	allowlist := []string{"10.0.0.0/8", "192.168.1.50", "radio.internal"}

	// When allowlist is empty, everything is allowed
	if !config.IsTargetAllowed("172.16.0.1", nil) {
		t.Errorf("expected empty allowlist to permit any target")
	}

	// Subnet match
	if !config.IsTargetAllowed("10.1.2.3", allowlist) {
		t.Errorf("expected 10.1.2.3 to be allowed by 10.0.0.0/8")
	}

	// Exact IP match
	if !config.IsTargetAllowed("192.168.1.50", allowlist) {
		t.Errorf("expected 192.168.1.50 to be allowed")
	}

	// Hostname match
	if !config.IsTargetAllowed("radio.internal", allowlist) {
		t.Errorf("expected radio.internal to be allowed")
	}

	// Disallowed IP
	if config.IsTargetAllowed("192.168.1.51", allowlist) {
		t.Errorf("expected 192.168.1.51 to be rejected")
	}

	// Disallowed outside subnet
	if config.IsTargetAllowed("172.16.1.1", allowlist) {
		t.Errorf("expected 172.16.1.1 to be rejected")
	}
}

func TestResolveConfigPaths(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := filepath.Join(tmpDir, "config.yaml")
	sys := filepath.Join(tmpDir, "system.yaml")

	_ = os.WriteFile(cfg, []byte("defaults:\n"), 0644)
	_ = os.WriteFile(sys, []byte("server:\n"), 0644)

	// Test --cfg-dir override
	rCfg, rSys := config.ResolveConfigPaths(tmpDir, "other.yaml", "other_sys.yaml")
	if rCfg != cfg || rSys != sys {
		t.Errorf("expected --cfg-dir to resolve to %s and %s, got %s and %s", cfg, sys, rCfg, rSys)
	}

	// Test explicit paths if existing
	rCfg2, rSys2 := config.ResolveConfigPaths("", cfg, sys)
	if rCfg2 != cfg || rSys2 != sys {
		t.Errorf("expected explicit existing files to resolve, got %s and %s", rCfg2, rSys2)
	}
}

func TestCredentialsFileLoading(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, "secrets.yaml")
	configFile := filepath.Join(tmpDir, "config.yaml")

	secretsContent := `
username: "secret_user"
password: "secret_password"
`
	configContent := `
defaults:
  credentials_file: "secrets.yaml"
devices:
  - name: "Dev1"
    ip: "10.0.0.1"
`
	if err := os.WriteFile(secretsFile, []byte(secretsContent), 0644); err != nil {
		t.Fatalf("failed to write secrets: %v", err)
	}
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	inv, err := config.LoadInventoryConfig(configFile)
	if err != nil {
		t.Fatalf("LoadInventoryConfig failed: %v", err)
	}

	if inv.Defaults.Username != "secret_user" {
		t.Errorf("expected username 'secret_user', got %q", inv.Defaults.Username)
	}
	if inv.Defaults.Password != "secret_password" {
		t.Errorf("expected password 'secret_password', got %q", inv.Defaults.Password)
	}
}
