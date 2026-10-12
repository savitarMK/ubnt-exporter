package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_Help(t *testing.T) {
	cmd := exec.Command("go", "run", "../../cmd/ubnt-exporter", "-h")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error running help: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "Ubiquiti Exporter") {
		t.Errorf("expected header 'Ubiquiti Exporter', got: %s", output)
	}
	if !strings.Contains(output, "AVAILABLE COMMANDS:") {
		t.Errorf("expected 'AVAILABLE COMMANDS:', got: %s", output)
	}
}

func TestCLI_Version(t *testing.T) {
	cmd := exec.Command("go", "run", "../../cmd/ubnt-exporter", "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error running -v: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "ubnt-exporter") {
		t.Errorf("expected version output containing 'ubnt-exporter', got: %s", output)
	}
}

func TestCLI_Check_InvalidConfig(t *testing.T) {
	cmd := exec.Command("go", "run", "../../cmd/ubnt-exporter", "check", "-c", "/nonexistent/config.yaml")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit code for nonexistent config, output: %s", string(out))
	}
	if !strings.Contains(string(out), "RESULT: Configuration has ERRORS") {
		t.Errorf("expected error notification in output, got: %s", string(out))
	}
}

func TestCLI_Check_ValidConfigExample(t *testing.T) {
	cmd := exec.Command("go", "run", "../../cmd/ubnt-exporter", "check", "-c", "../../config.yaml.example", "-s", "../../system.yaml.example")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error checking example configs: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "RESULT: Configuration is VALID") {
		t.Errorf("expected 'RESULT: Configuration is VALID', got: %s", output)
	}
}

func TestCLI_Show_SanitizedConfig(t *testing.T) {
	cmd := exec.Command("go", "run", "../../cmd/ubnt-exporter", "show", "-c", "../../config.yaml.example", "-s", "../../system.yaml.example")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error running show: %v, output: %s", err, string(out))
	}
	output := string(out)
	if !strings.Contains(output, "ACTIVE CONFIGURATION") {
		t.Errorf("expected 'ACTIVE CONFIGURATION', got: %s", output)
	}
	// Verify passwords are masked
	if strings.Contains(output, "password: \"password\"") || strings.Contains(output, "password: password") {
		t.Errorf("passwords were not properly sanitized in output: %s", output)
	}
}

func TestCLI_Run_InvalidConfig_FatalExit(t *testing.T) {
	cmd := exec.Command("go", "run", "../../cmd/ubnt-exporter", "run", "-c", "/nonexistent/config.yaml")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit code for nonexistent config in run command, output: %s", string(out))
	}
	output := string(out)
	if !strings.Contains(output, "Failed to load inventory configuration") && !strings.Contains(output, "Could not load configuration files") {
		t.Errorf("expected formatted fatal error log, got: %s", output)
	}
}

func TestCLI_Run_ValidationFailure_FatalExit(t *testing.T) {
	tmpDir := t.TempDir()
	badSys := filepath.Join(tmpDir, "system.yaml")
	_ = os.WriteFile(badSys, []byte("server:\n  port: 99999\n"), 0644)
	badCfg := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(badCfg, []byte("devices: []\n"), 0644)

	cmd := exec.Command("go", "run", "../../cmd/ubnt-exporter", "run", "-c", badCfg, "-s", badSys)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit code for validation error in run command, output: %s", string(out))
	}
	output := string(out)
	if !strings.Contains(output, "Configuration validation error") || !strings.Contains(output, "invalid server.port") {
		t.Errorf("expected configuration validation error log, got: %s", output)
	}
}
