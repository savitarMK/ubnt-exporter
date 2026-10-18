package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubnt-exporter/internal/config"
)

func TestValidateIP(t *testing.T) {
	if !config.ValidateIP("192.168.1.1") {
		t.Errorf("Valid IPv4 was rejected")
	}
	if !config.ValidateIP("2001:db8::1") {
		t.Errorf("Valid IPv6 was rejected")
	}
	if config.ValidateIP("999.999.999.999") {
		t.Errorf("Invalid IP was accepted")
	}
	if config.ValidateIP("not-an-ip") {
		t.Errorf("Invalid string was accepted as IP")
	}
}

func TestDiscoverCustomLabels(t *testing.T) {
	inv := &config.InventoryConfig{
		Devices: []config.DeviceConfig{
			{
				Name: "Dev1",
				IP:   "10.0.0.1",
				Labels: map[string]string{
					"zone": "A",
					"pop":  "central",
				},
			},
			{
				Name: "Dev2",
				IP:   "10.0.0.2",
				Labels: map[string]string{
					"zone": "B",
					"tier": "gold",
				},
			},
		},
	}

	keys := config.DiscoverCustomLabels(inv)
	expected := []string{"pop", "tier", "zone"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("at index %d, expected %s, got %s", i, expected[i], k)
		}
	}
}

func TestParseListenAddresses(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		defaultPort int
		expected    []string
		wantErr     bool
	}{
		{
			name:        "Empty listen string defaults to port",
			input:       "",
			defaultPort: 8000,
			expected:    []string{":8000"},
			wantErr:     false,
		},
		{
			name:        "IPv4 with port",
			input:       "0.0.0.0:8000",
			defaultPort: 8000,
			expected:    []string{"0.0.0.0:8000"},
			wantErr:     false,
		},
		{
			name:        "IPv6 bracketed with port",
			input:       "[::]:8000",
			defaultPort: 8000,
			expected:    []string{"[::]:8000"},
			wantErr:     false,
		},
		{
			name:        "IPv4 without port",
			input:       "127.0.0.1",
			defaultPort: 8000,
			expected:    []string{"127.0.0.1:8000"},
			wantErr:     false,
		},
		{
			name:        "IPv6 without port",
			input:       "::1",
			defaultPort: 8000,
			expected:    []string{"[::1]:8000"},
			wantErr:     false,
		},
		{
			name:        "Port only with colon",
			input:       ":9100",
			defaultPort: 8000,
			expected:    []string{":9100"},
			wantErr:     false,
		},
		{
			name:        "Port only digits",
			input:       "9100",
			defaultPort: 8000,
			expected:    []string{":9100"},
			wantErr:     false,
		},
		{
			name:        "Multiple space-separated addresses",
			input:       "0.0.0.0:8000 [::]:8000",
			defaultPort: 8000,
			expected:    []string{"0.0.0.0:8000", "[::]:8000"},
			wantErr:     false,
		},
		{
			name:        "Invalid port out of range",
			input:       "0.0.0.0:99999",
			defaultPort: 8000,
			expected:    nil,
			wantErr:     true,
		},
		{
			name:        "Unix Domain Socket URI",
			input:       "unix:///run/ubnt-exporter.sock",
			defaultPort: 8000,
			expected:    []string{"unix:///run/ubnt-exporter.sock"},
			wantErr:     false,
		},
		{
			name:        "Unix Domain Socket absolute path",
			input:       "/var/run/ubnt.sock",
			defaultPort: 8000,
			expected:    []string{"unix:///var/run/ubnt.sock"},
			wantErr:     false,
		},
		{
			name:        "Invalid address string",
			input:       "invalid@@host",
			defaultPort: 8000,
			expected:    nil,
			wantErr:     true,
		},
		{
			name:        "Empty unix socket path",
			input:       "unix://",
			defaultPort: 8000,
			expected:    nil,
			wantErr:     true,
		},
		{
			name:        "Invalid colon port format",
			input:       ":abc",
			defaultPort: 8000,
			expected:    nil,
			wantErr:     true,
		},
		{
			name:        "Digits port out of range",
			input:       "99999",
			defaultPort: 8000,
			expected:    nil,
			wantErr:     true,
		},
		{
			name:        "Invalid port in host:port",
			input:       "127.0.0.1:abc",
			defaultPort: 8000,
			expected:    nil,
			wantErr:     true,
		},
		{
			name:        "Hostname without port",
			input:       "localhost",
			defaultPort: 8000,
			expected:    []string{"localhost:8000"},
			wantErr:     false,
		},
		{
			name:        "Hostname with port",
			input:       "radio.site.net:8080",
			defaultPort: 8000,
			expected:    []string{"radio.site.net:8080"},
			wantErr:     false,
		},
		{
			name:        "Invalid hostname starting with hyphen",
			input:       "-badhost.net:8080",
			defaultPort: 8000,
			expected:    nil,
			wantErr:     true,
		},
		{
			name:        "Invalid hostname with double dot",
			input:       "bad..host:8080",
			defaultPort: 8000,
			expected:    nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := config.ParseListenAddresses(tt.input, tt.defaultPort)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseListenAddresses(%q) error = %v, wantErr = %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(res) != len(tt.expected) {
					t.Fatalf("expected len %d, got len %d (%v)", len(tt.expected), len(res), res)
				}
				for i := range res {
					if res[i] != tt.expected[i] {
						t.Errorf("at index %d: expected %s, got %s", i, tt.expected[i], res[i])
					}
				}
			}
		})
	}
}

func TestValidateConfigAllOptions(t *testing.T) {
	validInv := &config.InventoryConfig{
		Devices: []config.DeviceConfig{
			{Name: "Dev1", IP: "10.0.0.1"},
		},
	}

	t.Run("Valid System Config Passes", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) != 0 {
			t.Fatalf("expected 0 errors, got: %v", errs)
		}
	})

	t.Run("Invalid Server Port", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Server.Port = 70000
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "invalid server.port") {
			t.Fatalf("expected server.port error, got: %v", errs)
		}
	})

	t.Run("Invalid Scrape Interval", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Server.ScrapeInterval = 0
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "invalid server.scrape_interval") {
			t.Fatalf("expected scrape_interval error, got: %v", errs)
		}
	})

	t.Run("Invalid Listen Socket", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Server.Listen = "bad_host@@:8000"
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "invalid server.listen") {
			t.Fatalf("expected server.listen error, got: %v", errs)
		}
	})

	t.Run("Invalid Performance Workers and Timeouts", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Performance.MaxWorkers = 0
		sys.Performance.RequestTimeout = 0
		sys.Performance.LoginTimeout = 0
		sys.Performance.SocketTimeout = 0
		sys.Performance.FailureBackoff = -1
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) < 5 {
			t.Fatalf("expected at least 5 performance errors, got %d: %v", len(errs), errs)
		}
	})

	t.Run("Invalid Logging Level", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Logging.Level = "INVALID_LEVEL"
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "invalid logging.level") {
			t.Fatalf("expected logging.level error, got: %v", errs)
		}
	})

	t.Run("Invalid Syslog Facility", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Logging.SyslogFacility = "non_existent_facility"
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "invalid logging.syslog_facility") {
			t.Fatalf("expected logging.syslog_facility error, got: %v", errs)
		}
	})

	t.Run("Multiple Logging Formats", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Logging.Formats.CEF = true
		sys.Logging.Formats.Text = true
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "multiple logging formats enabled") {
			t.Fatalf("expected multiple logging formats error, got: %v", errs)
		}
	})

	t.Run("Valid JSON Logging Format Passes", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Logging.Formats.CEF = false
		sys.Logging.Formats.JSON = true
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) != 0 {
			t.Fatalf("expected 0 errors for JSON logging format, got: %v", errs)
		}
	})

	t.Run("File Logging Without Path", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Logging.File.Enabled = true
		sys.Logging.File.Path = ""
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "logging.file.path cannot be empty") {
			t.Fatalf("expected empty path error, got: %v", errs)
		}
	})

	t.Run("File Logging With Invalid MaxSize or BackupCount", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Logging.File.Enabled = true
		sys.Logging.File.Path = "test.log"
		sys.Logging.File.MaxSizeMB = 0
		sys.Logging.File.BackupCount = -1
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) < 2 {
			t.Fatalf("expected file rotation errors, got: %v", errs)
		}
	})

	t.Run("Console and File Both Disabled", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Logging.Console = false
		sys.Logging.File.Enabled = false
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "both logging.console and logging.file are disabled") {
			t.Fatalf("expected disabled log error, got: %v", errs)
		}
	})

	t.Run("Dampening Valid Configuration Passes", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Logging.Dampening.Enabled = true
		sys.Logging.Dampening.Threshold = 5
		sys.Logging.Dampening.Interval = 60
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) != 0 {
			t.Fatalf("expected 0 errors for valid dampening config, got: %v", errs)
		}
	})

	t.Run("Dampening Invalid Threshold and Interval", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Logging.Dampening.Enabled = true
		sys.Logging.Dampening.Threshold = 0
		sys.Logging.Dampening.Interval = -10
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) < 2 {
			t.Fatalf("expected 2 dampening validation errors, got: %v", errs)
		}
		if !strings.Contains(errs[0], "invalid logging.dampening.threshold") || !strings.Contains(errs[1], "invalid logging.dampening.interval") {
			t.Fatalf("unexpected dampening validation error messages: %v", errs)
		}
	})

	t.Run("Duplicate Device IP", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		dupInv := &config.InventoryConfig{
			Devices: []config.DeviceConfig{
				{Name: "Dev1", IP: "10.0.0.1"},
				{Name: "Dev2", IP: "10.0.0.1"},
			},
		}
		errs := config.ValidateConfig(sys, dupInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "duplicate device IP") {
			t.Fatalf("expected duplicate IP error, got: %v", errs)
		}
	})

	t.Run("Duplicate Device Name", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		dupInv := &config.InventoryConfig{
			Devices: []config.DeviceConfig{
				{Name: "Dev1", IP: "10.0.0.1"},
				{Name: "Dev1", IP: "10.0.0.2"},
			},
		}
		errs := config.ValidateConfig(sys, dupInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "duplicate device name") {
			t.Fatalf("expected duplicate name error, got: %v", errs)
		}
	})

	t.Run("Reserved Parameter in Labels", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		badInv := &config.InventoryConfig{
			Devices: []config.DeviceConfig{
				{
					Name: "Dev1",
					IP:   "10.0.0.1",
					Labels: map[string]string{
						"ssl":      "true",
						"password": "secret",
					},
				},
			},
		}
		errs := config.ValidateConfig(sys, badInv)
		if len(errs) < 2 {
			t.Fatalf("expected at least 2 reserved parameter errors, got: %v", errs)
		}
		if !strings.Contains(errs[0], "reserved parameter") && !strings.Contains(errs[1], "reserved parameter") {
			t.Fatalf("expected 'reserved parameter' in error message, got: %v", errs)
		}
	})

	t.Run("Unknown Collector Name", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		badInv := &config.InventoryConfig{
			EnabledCollectors: map[string]bool{
				"wireles_typo": true,
			},
			Devices: []config.DeviceConfig{
				{
					Name: "Dev1",
					IP:   "10.0.0.1",
					EnabledCollectors: map[string]bool{
						"peer_typo": true,
					},
				},
			},
		}
		errs := config.ValidateConfig(sys, badInv)
		if len(errs) < 2 {
			t.Fatalf("expected 2 unknown collector errors, got: %v", errs)
		}
	})

	t.Run("Invalid Prometheus Label Name and Reserved Base Labels", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		badInv := &config.InventoryConfig{
			Defaults: config.DefaultsConfig{
				Labels: map[string]string{
					"name": "reserved",
				},
			},
			Devices: []config.DeviceConfig{
				{
					Name: "Dev1",
					IP:   "10.0.0.1",
					Labels: map[string]string{
						"tower-id": "invalid-hyphen",
						"__meta":   "invalid-prefix",
					},
				},
			},
		}
		errs := config.ValidateConfig(sys, badInv)
		if len(errs) < 3 {
			t.Fatalf("expected at least 3 label validation errors, got: %v", errs)
		}
	})

	t.Run("SocketTimeout Exceeds RequestTimeout", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Performance.SocketTimeout = 20
		sys.Performance.RequestTimeout = 10
		errs := config.ValidateConfig(sys, validInv)
		if len(errs) == 0 || !strings.Contains(errs[0], "cannot exceed performance.request_timeout") {
			t.Fatalf("expected socket_timeout > request_timeout error, got: %v", errs)
		}
	})

	t.Run("Nil System and Nil Inventory", func(t *testing.T) {
		if errs := config.ValidateConfig(nil, validInv); len(errs) == 0 || !strings.Contains(errs[0], "system.yaml is empty") {
			t.Errorf("expected error for nil sys")
		}
		sys := config.DefaultSystemConfig()
		if errs := config.ValidateConfig(sys, nil); len(errs) == 0 || !strings.Contains(errs[0], "config.yaml is empty") {
			t.Errorf("expected error for nil inv")
		}
	})

	t.Run("Server and Performance Edge Cases", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		sys.Server.Port = 70000
		sys.Server.ScrapeInterval = 0
		sys.Server.Listen = "invalid:listen:99999"
		sys.Server.ProbeAllowedTargets = []string{"", "10.0.0.0/999", "invalid@host"}
		sys.Performance.MaxWorkers = 0
		sys.Performance.LoginTimeout = 0
		sys.Performance.SocketTimeout = 0
		sys.Performance.FailureBackoff = -1
		sys.Logging.Level = "INVALID"
		sys.Logging.SyslogFacility = "INVALID"
		sys.Logging.Dampening.Enabled = true
		sys.Logging.Dampening.Threshold = 0
		sys.Logging.Dampening.Interval = 0

		errs := config.ValidateConfig(sys, validInv)
		if len(errs) < 10 {
			t.Fatalf("expected >= 10 validation errors, got %d: %v", len(errs), errs)
		}
	})

	t.Run("Inventory Edge Cases", func(t *testing.T) {
		sys := config.DefaultSystemConfig()
		badInv := &config.InventoryConfig{
			Defaults: config.DefaultsConfig{
				Labels: map[string]string{
					"ip": "reserved",
				},
			},
			Devices: []config.DeviceConfig{
				{
					Name: "",
					IP:   "invalid-ip",
					Labels: map[string]string{
						"optional_note": "sample",
						"name":          "reserved",
					},
				},
				{
					Name: "DevDuplicateIP1",
					IP:   "10.20.30.40",
				},
				{
					Name: "DevDuplicateIP2",
					IP:   "10.20.30.40",
				},
			},
		}

		errs := config.ValidateConfig(sys, badInv)
		if len(errs) < 5 {
			t.Fatalf("expected >= 5 inventory validation errors, got %d: %v", len(errs), errs)
		}
	})
}

func TestLoadConfig_MalformedYAML(t *testing.T) {
	badFile := filepath.Join(t.TempDir(), "malformed.yaml")
	_ = os.WriteFile(badFile, []byte(":::invalid: yaml:::"), 0600)

	if _, err := config.LoadSystemConfig(badFile); err == nil {
		t.Errorf("expected LoadSystemConfig to fail on malformed YAML")
	}
	if _, err := config.LoadInventoryConfig(badFile); err == nil {
		t.Errorf("expected LoadInventoryConfig to fail on malformed YAML")
	}
}
