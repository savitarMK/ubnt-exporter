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
	if err := os.WriteFile(configFile, []byte(content), 0600); err != nil {
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

	if !config.IsTargetAllowed("172.16.0.1", nil) {
		t.Errorf("expected empty allowlist to permit any target")
	}

	if !config.IsTargetAllowed("10.1.2.3", allowlist) {
		t.Errorf("expected 10.1.2.3 to be allowed by 10.0.0.0/8")
	}

	if !config.IsTargetAllowed("192.168.1.50", allowlist) {
		t.Errorf("expected 192.168.1.50 to be allowed")
	}

	if !config.IsTargetAllowed("radio.internal", allowlist) {
		t.Errorf("expected radio.internal to be allowed")
	}

	if config.IsTargetAllowed("192.168.1.51", allowlist) {
		t.Errorf("expected 192.168.1.51 to be rejected")
	}

	if config.IsTargetAllowed("172.16.1.1", allowlist) {
		t.Errorf("expected 172.16.1.1 to be rejected")
	}
}

func TestResolveConfigPaths(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := filepath.Join(tmpDir, "config.yaml")
	sys := filepath.Join(tmpDir, "system.yaml")

	_ = os.WriteFile(cfg, []byte("defaults:\n"), 0600)
	_ = os.WriteFile(sys, []byte("server:\n"), 0600)

	rCfg, rSys := config.ResolveConfigPaths(tmpDir, "other.yaml", "other_sys.yaml")
	if rCfg != cfg || rSys != sys {
		t.Errorf("expected --cfg-dir to resolve to %s and %s, got %s and %s", cfg, sys, rCfg, rSys)
	}

	rCfg2, rSys2 := config.ResolveConfigPaths("", cfg, sys)
	if rCfg2 != cfg || rSys2 != sys {
		t.Errorf("expected explicit existing files to resolve, got %s and %s", rCfg2, rSys2)
	}

	xdgDir := t.TempDir()
	xdgAppDir := filepath.Join(xdgDir, "ubnt-exporter")
	_ = os.MkdirAll(xdgAppDir, 0750)
	xdgCfg := filepath.Join(xdgAppDir, "config.yaml")
	xdgSys := filepath.Join(xdgAppDir, "system.yaml")
	_ = os.WriteFile(xdgCfg, []byte("defaults:\n"), 0600)
	_ = os.WriteFile(xdgSys, []byte("server:\n"), 0600)

	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	rCfg3, rSys3 := config.ResolveConfigPaths("", filepath.Join(tmpDir, "missing1.yaml"), filepath.Join(tmpDir, "missing2.yaml"))
	if rCfg3 != xdgCfg || rSys3 != xdgSys {
		t.Errorf("expected XDG_CONFIG_HOME paths %s and %s, got %s and %s", xdgCfg, xdgSys, rCfg3, rSys3)
	}

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "empty_xdg"))
	rCfg4, rSys4 := config.ResolveConfigPaths("", "nonexistent_a.yaml", "nonexistent_b.yaml")
	if rCfg4 != "nonexistent_a.yaml" || rSys4 != "nonexistent_b.yaml" {
		t.Errorf("expected fallback to original paths, got %s and %s", rCfg4, rSys4)
	}

	enforced, err := config.EnforceSystemDefaults("", &config.SystemConfig{})
	if err != nil || enforced.Server.Port != 8000 {
		t.Errorf("expected EnforceSystemDefaults port 8000, got %d, err=%v", enforced.Server.Port, err)
	}
}

func TestCredentialsFileLoading(t *testing.T) {
	tmpDir := t.TempDir()
	secretsFile := filepath.Join(tmpDir, "secrets.yaml")
	configFile := filepath.Join(tmpDir, "config.yaml")

	// #nosec G101 -- test mock credentials file
	secretsContent := `
username: "secret_user"
password: "secret_password"
devices:
  Dev2:
    username: "dev2_user"
    password: "dev2_password"
  "192.168.1.30":
    username: "ip_user"
    password: "ip_password"
`
	configContent := `
defaults:
  credentials_file: "secrets.yaml"
devices:
  - name: "Dev1"
    ip: "192.168.1.10"
  - name: "Dev2"
    ip: "192.168.1.20"
  - name: "Dev3"
    ip: "192.168.1.30"
`
	if err := os.WriteFile(secretsFile, []byte(secretsContent), 0600); err != nil {
		t.Fatalf("failed to write secrets: %v", err)
	}
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
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
	if len(inv.Devices) != 3 {
		t.Fatalf("expected 3 devices, got %d", len(inv.Devices))
	}
	if inv.Devices[1].Username != "dev2_user" || inv.Devices[1].Password != "dev2_password" {
		t.Errorf("expected Dev2 override from secrets.yaml by name, got user=%q pass=%q", inv.Devices[1].Username, inv.Devices[1].Password)
	}
	if inv.Devices[2].Username != "ip_user" || inv.Devices[2].Password != "ip_password" {
		t.Errorf("expected Dev3 override from secrets.yaml by IP, got user=%q pass=%q", inv.Devices[2].Username, inv.Devices[2].Password)
	}

	// Verify examples/secrets.yaml.example is valid YAML and loads cleanly when referenced
	exampleSecretsBytes, err := os.ReadFile("../../examples/secrets.yaml.example")
	if err != nil {
		t.Fatalf("expected examples/secrets.yaml.example to exist: %v", err)
	}
	if err := os.WriteFile(secretsFile, exampleSecretsBytes, 0600); err != nil {
		t.Fatalf("failed to copy examples/secrets.yaml.example: %v", err)
	}
	exampleCfgContent := `
defaults:
  credentials_file: "secrets.yaml"
devices:
  - name: "AP-SECTOR-01"
    ip: "192.168.1.10"
  - name: "AP-BACKHAUL-01"
    ip: "192.168.1.20"
`
	if err := os.WriteFile(configFile, []byte(exampleCfgContent), 0600); err != nil {
		t.Fatalf("failed to write example config: %v", err)
	}
	invExample, err := config.LoadInventoryConfig(configFile)
	if err != nil {
		t.Fatalf("LoadInventoryConfig with examples/secrets.yaml.example failed: %v", err)
	}
	if invExample.Defaults.Username != "YOUR_DEFAULT_USERNAME" || invExample.Devices[1].Username != "custom_admin" {
		t.Errorf("unexpected credentials loaded from examples/secrets.yaml.example: defaults=%q dev1=%q", invExample.Defaults.Username, invExample.Devices[1].Username)
	}

	// Verify error paths on missing/corrupt files
	if _, err := config.LoadSystemConfig(filepath.Join(tmpDir, "missing_sys.yaml")); err == nil {
		t.Errorf("expected error loading missing system.yaml")
	}
	if _, err := config.LoadInventoryConfig(filepath.Join(tmpDir, "missing_inv.yaml")); err == nil {
		t.Errorf("expected error loading missing config.yaml")
	}
}

// Regression: server.port must drive the listen address when server.listen is omitted,
// and an explicit server.listen must still take precedence.
func TestLoadSystemConfigPortDrivesListen(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		expected []string
	}{
		{"port only", "server:\n  port: 9100\n", []string{"0.0.0.0:9100"}},
		{"no server block", "logging:\n  level: INFO\n", []string{"0.0.0.0:8000"}},
		{"explicit listen wins", "server:\n  port: 9100\n  listen: \"127.0.0.1:9200\"\n", []string{"127.0.0.1:9200"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "system.yaml")
			if err := os.WriteFile(p, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.LoadSystemConfig(p)
			if err != nil {
				t.Fatalf("LoadSystemConfig failed: %v", err)
			}
			addrs, err := config.ParseListenAddresses(cfg.Server.Listen, cfg.Server.Port)
			if err != nil {
				t.Fatalf("ParseListenAddresses failed: %v", err)
			}
			if len(addrs) != len(tc.expected) || addrs[0] != tc.expected[0] {
				t.Errorf("expected %v, got %v", tc.expected, addrs)
			}
		})
	}
}

// Regression: enabling a single non-CEF log format must not collide with the CEF default,
// and omitting formats entirely must still fall back to CEF.
func TestLoadSystemConfigSingleLogFormat(t *testing.T) {
	cases := []struct {
		name    string
		content string
		check   func(config.FormatsConfig) bool
	}{
		{"json only", "logging:\n  formats:\n    json: true\n", func(f config.FormatsConfig) bool { return f.JSON && !f.CEF }},
		{"text only", "logging:\n  formats:\n    text: true\n", func(f config.FormatsConfig) bool { return f.Text && !f.CEF }},
		{"formats omitted", "server:\n  port: 8000\n", func(f config.FormatsConfig) bool { return f.CEF && !f.JSON && !f.Text }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "system.yaml")
			if err := os.WriteFile(p, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.LoadSystemConfig(p)
			if err != nil {
				t.Fatalf("LoadSystemConfig failed: %v", err)
			}
			if !tc.check(cfg.Logging.Formats) {
				t.Errorf("unexpected formats: %+v", cfg.Logging.Formats)
			}
			if errs := config.ValidateConfig(cfg, &config.InventoryConfig{}); len(errs) > 0 {
				t.Errorf("expected no validation errors, got %v", errs)
			}
		})
	}
}
