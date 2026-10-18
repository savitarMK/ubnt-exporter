// Package config handles loading, validating, and managing YAML configurations.
package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServerConfig defines HTTP server settings.
type ServerConfig struct {
	Port                int      `yaml:"port"`
	Listen              string   `yaml:"listen,omitempty"`
	ScrapeInterval      int      `yaml:"scrape_interval"`
	ProbeAllowedTargets []string `yaml:"probe_allowed_targets,omitempty"`
}

// PerformanceConfig defines worker limits and timeout settings.
type PerformanceConfig struct {
	MaxWorkers     int `yaml:"max_workers"`
	LoginTimeout   int `yaml:"login_timeout"`
	RequestTimeout int `yaml:"request_timeout"`
	SocketTimeout  int `yaml:"socket_timeout"`
	FailureBackoff int `yaml:"failure_backoff"`
}

// FormatsConfig defines logging format toggles.
type FormatsConfig struct {
	CEF     bool `yaml:"cef"`
	RFC5424 bool `yaml:"rfc5424"`
	Syslog  bool `yaml:"syslog"`
	Text    bool `yaml:"text"`
	JSON    bool `yaml:"json"`
}

// FileLogConfig defines file-based logging properties.
type FileLogConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Path        string `yaml:"path"`
	MaxSizeMB   int    `yaml:"max_size_mb"`
	BackupCount int    `yaml:"backup_count"`
}

// DampeningConfig defines repetitive log throttling properties to prevent log storms.
type DampeningConfig struct {
	Enabled   bool `yaml:"enabled"`
	Threshold int  `yaml:"threshold"`
	Interval  int  `yaml:"interval"`
}

// LoggingConfig defines logging settings.
type LoggingConfig struct {
	Level          string          `yaml:"level"`
	SyslogFacility string          `yaml:"syslog_facility"`
	Formats        FormatsConfig   `yaml:"formats"`
	Console        bool            `yaml:"console"`
	File           FileLogConfig   `yaml:"file"`
	Dampening      DampeningConfig `yaml:"dampening"`
}

// SystemConfig aggregates server, performance, and logging configurations.
type SystemConfig struct {
	Server      ServerConfig      `yaml:"server"`
	Performance PerformanceConfig `yaml:"performance"`
	Logging     LoggingConfig     `yaml:"logging"`
}

// DefaultsConfig defines global fallback options for devices in the inventory.
type DefaultsConfig struct {
	Username        string            `yaml:"username,omitempty"`
	Password        string            `yaml:"password,omitempty"`
	CredentialsFile string            `yaml:"credentials_file,omitempty"`
	SSL             bool              `yaml:"ssl,omitempty"`
	Labels          map[string]string `yaml:"labels,omitempty"`
}

// DeviceConfig defines an individual Ubiquiti device to scrape.
type DeviceConfig struct {
	Name              string            `yaml:"name"`
	IP                string            `yaml:"ip"`
	Username          string            `yaml:"username,omitempty"`
	Password          string            `yaml:"password,omitempty"`
	SSL               *bool             `yaml:"ssl,omitempty"`
	Labels            map[string]string `yaml:"labels,omitempty"`
	EnabledCollectors map[string]bool   `yaml:"enabled_collectors,omitempty"`
}

// InventoryConfig defines device inventory and global options.
type InventoryConfig struct {
	Defaults          DefaultsConfig  `yaml:"defaults,omitempty"`
	Devices           []DeviceConfig  `yaml:"devices"`
	EnabledCollectors map[string]bool `yaml:"enabled_collectors,omitempty"`
}

// ValidSyslogFacilities lists all standard syslog facility names recognized by the exporter.
var ValidSyslogFacilities = map[string]struct{}{
	"kern": {}, "user": {}, "mail": {}, "daemon": {}, "auth": {}, "syslog": {},
	"lpr": {}, "news": {}, "uucp": {}, "cron": {}, "authpriv": {}, "ftp": {},
	"ntp": {}, "security": {}, "console": {}, "solaris-cron": {},
	"local0": {}, "local1": {}, "local2": {}, "local3": {},
	"local4": {}, "local5": {}, "local6": {}, "local7": {},
}

// ValidLogLevels lists accepted logging severity levels.
var ValidLogLevels = map[string]struct{}{
	"DEBUG":   {},
	"INFO":    {},
	"WARN":    {},
	"WARNING": {},
	"ERROR":   {},
}

// ValidCollectorNames lists all recognized telemetry collector module names.
var ValidCollectorNames = map[string]struct{}{
	"system":     {},
	"interfaces": {},
	"wireless":   {},
	"peers":      {},
	"airfiber":   {},
}

// IsValidPrometheusLabelName checks whether a label key conforms to the Prometheus data model
// ([a-zA-Z_][a-zA-Z0-9_]*) and does not use reserved internal prefixes (__).
func IsValidPrometheusLabelName(name string) bool {
	if name == "" || strings.HasPrefix(name, "__") {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		isAlpha := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
		isDigit := c >= '0' && c <= '9'
		if i == 0 && !isAlpha {
			return false
		}
		if !isAlpha && !isDigit {
			return false
		}
	}
	return true
}

// DefaultSystemConfig returns a SystemConfig populated with default values.
func DefaultSystemConfig() *SystemConfig {
	return &SystemConfig{
		Server: ServerConfig{
			Port:           8000,
			Listen:         "0.0.0.0:8000",
			ScrapeInterval: 30,
		},
		Performance: PerformanceConfig{
			MaxWorkers:     100,
			LoginTimeout:   5,
			RequestTimeout: 15,
			SocketTimeout:  2,
			FailureBackoff: 120,
		},
		Logging: LoggingConfig{
			Level:          "INFO",
			SyslogFacility: "local0",
			Formats: FormatsConfig{
				CEF:     true,
				RFC5424: false,
				Syslog:  false,
				Text:    false,
			},
			Console: true,
			File: FileLogConfig{
				Enabled:     false,
				Path:        "logs/app.log",
				MaxSizeMB:   5,
				BackupCount: 3,
			},
			Dampening: DampeningConfig{
				Enabled:   false,
				Threshold: 3,
				Interval:  300,
			},
		},
	}
}

// LoadSystemConfig reads and parses system.yaml.
func LoadSystemConfig(filename string) (*SystemConfig, error) {
	cleanPath := filepath.Clean(filename)
	// #nosec G304 -- configuration file path is supplied by the administrator via CLI or standard search paths
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filename, err)
	}

	cfg := DefaultSystemConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML in %s: %w", filename, err)
	}

	return cfg, nil
}

// LoadInventoryConfig reads and parses config.yaml.
func LoadInventoryConfig(filename string) (*InventoryConfig, error) {
	cleanPath := filepath.Clean(filename)
	// #nosec G304 -- inventory file path is supplied by the administrator via CLI or standard search paths
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filename, err)
	}

	inv := &InventoryConfig{
		Defaults: DefaultsConfig{
			Labels: make(map[string]string),
		},
		Devices:           make([]DeviceConfig, 0),
		EnabledCollectors: make(map[string]bool),
	}

	if err := yaml.Unmarshal(data, inv); err != nil {
		return nil, fmt.Errorf("parsing YAML in %s: %w", filename, err)
	}

	if inv.Defaults.CredentialsFile != "" {
		credsPath := inv.Defaults.CredentialsFile
		if !filepath.IsAbs(credsPath) {
			credsPath = filepath.Join(filepath.Dir(cleanPath), credsPath)
		}
		cleanCredsPath := filepath.Clean(credsPath)
		// #nosec G304 -- credentials_file path is explicitly configured by administrator in config.yaml
		cData, cErr := os.ReadFile(cleanCredsPath)
		if cErr != nil {
			return nil, fmt.Errorf("reading credentials_file %s: %w", credsPath, cErr)
		}
		var secrets struct {
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		}
		if err := yaml.Unmarshal(cData, &secrets); err != nil {
			return nil, fmt.Errorf("parsing YAML in credentials_file %s: %w", credsPath, err)
		}
		if inv.Defaults.Username == "" && secrets.Username != "" {
			inv.Defaults.Username = secrets.Username
		}
		if inv.Defaults.Password == "" && secrets.Password != "" {
			inv.Defaults.Password = secrets.Password
		}
	}

	return inv, nil
}

// ApplySystemDefaults populates missing or zero-value fields with defaults in memory.
func ApplySystemDefaults(current *SystemConfig) *SystemConfig {
	if current == nil {
		return DefaultSystemConfig()
	}
	defaults := DefaultSystemConfig()

	if current.Server.Port <= 0 {
		current.Server.Port = defaults.Server.Port
	}
	if current.Server.ScrapeInterval <= 0 {
		current.Server.ScrapeInterval = defaults.Server.ScrapeInterval
	}
	if current.Performance.MaxWorkers <= 0 {
		current.Performance.MaxWorkers = defaults.Performance.MaxWorkers
	}
	if current.Performance.LoginTimeout <= 0 {
		current.Performance.LoginTimeout = defaults.Performance.LoginTimeout
	}
	if current.Performance.RequestTimeout <= 0 {
		current.Performance.RequestTimeout = defaults.Performance.RequestTimeout
	}
	if current.Performance.SocketTimeout <= 0 {
		current.Performance.SocketTimeout = defaults.Performance.SocketTimeout
	}
	if current.Performance.FailureBackoff < 0 {
		current.Performance.FailureBackoff = defaults.Performance.FailureBackoff
	}
	if current.Logging.Level == "" {
		current.Logging.Level = defaults.Logging.Level
	}
	if current.Logging.SyslogFacility == "" {
		current.Logging.SyslogFacility = defaults.Logging.SyslogFacility
	}
	if !current.Logging.Formats.CEF && !current.Logging.Formats.RFC5424 &&
		!current.Logging.Formats.Syslog && !current.Logging.Formats.Text &&
		!current.Logging.Formats.JSON {
		current.Logging.Formats.CEF = true
	}
	if current.Logging.Dampening.Threshold <= 0 {
		current.Logging.Dampening.Threshold = defaults.Logging.Dampening.Threshold
	}
	if current.Logging.Dampening.Interval <= 0 {
		current.Logging.Dampening.Interval = defaults.Logging.Dampening.Interval
	}
	return current
}

// EnforceSystemDefaults applies in-memory defaults and satisfies compatibility callers.
func EnforceSystemDefaults(_ string, current *SystemConfig) (*SystemConfig, error) {
	return ApplySystemDefaults(current), nil
}

// DiscoverCustomLabels extracts all unique label keys found in device and default definitions.
func DiscoverCustomLabels(inv *InventoryConfig) []string {
	keySet := make(map[string]struct{})
	for k := range inv.Defaults.Labels {
		keySet[k] = struct{}{}
	}
	for _, dev := range inv.Devices {
		for k := range dev.Labels {
			keySet[k] = struct{}{}
		}
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ValidateIP checks whether an IP string is a valid IPv4 or IPv6 address.
func ValidateIP(ipStr string) bool {
	return net.ParseIP(strings.TrimSpace(ipStr)) != nil
}

func isAllowedHostRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-'
}

func isValidHostname(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 253 {
		return false
	}
	if name == "localhost" {
		return true
	}
	parts := strings.Split(name, ".")
	for _, part := range parts {
		if len(part) == 0 || len(part) > 63 {
			return false
		}
		if part[0] == '-' || part[len(part)-1] == '-' {
			return false
		}
		for _, r := range part {
			if !isAllowedHostRune(r) {
				return false
			}
		}
	}
	return true
}

func normalizeSingleAddress(target string, defaultPort int) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("empty listen target")
	}

	// Case 0: Unix Domain Socket (e.g. "unix:///run/ubnt-exporter.sock", "unix:/path.sock", "/path.sock")
	if strings.HasPrefix(target, "unix://") || strings.HasPrefix(target, "unix:") {
		sockPath := strings.TrimPrefix(strings.TrimPrefix(target, "unix://"), "unix:")
		if sockPath == "" {
			return "", fmt.Errorf("empty unix domain socket path in %q", target)
		}
		return "unix://" + sockPath, nil
	}
	if strings.HasPrefix(target, "/") {
		return "unix://" + target, nil
	}

	// Case 1: IP without port (e.g. "0.0.0.0", "127.0.0.1", "[::]", "::", "::1")
	trimmed := strings.Trim(target, "[]")
	if ip := net.ParseIP(trimmed); ip != nil {
		return net.JoinHostPort(trimmed, strconv.Itoa(defaultPort)), nil
	}

	// Case 2: Pure port starting with single colon (e.g. ":8000")
	if strings.HasPrefix(target, ":") && !strings.HasPrefix(target, "::") {
		pStr := strings.TrimPrefix(target, ":")
		p, err := strconv.Atoi(pStr)
		if err != nil || p < 1 || p > 65535 {
			return "", fmt.Errorf("invalid listen port: %q", target)
		}
		return fmt.Sprintf(":%d", p), nil
	}

	// Case 3: Pure numeric string (e.g. "8000")
	if p, err := strconv.Atoi(target); err == nil {
		if p < 1 || p > 65535 {
			return "", fmt.Errorf("invalid listen port: %d", p)
		}
		return fmt.Sprintf(":%d", p), nil
	}

	// Case 4: Already host:port or [ipv6]:port
	host, portStr, err := net.SplitHostPort(target)
	if err == nil {
		p, pErr := strconv.Atoi(portStr)
		if pErr != nil || p < 1 || p > 65535 {
			return "", fmt.Errorf("invalid port in listen target %q", target)
		}
		if host != "" {
			if net.ParseIP(host) == nil && !isValidHostname(host) {
				return "", fmt.Errorf("invalid host %q in listen target %q", host, target)
			}
		}
		return net.JoinHostPort(host, portStr), nil
	}

	// Case 5: Hostname without port (e.g. "localhost")
	if isValidHostname(target) {
		return net.JoinHostPort(target, strconv.Itoa(defaultPort)), nil
	}

	return "", fmt.Errorf("invalid listen address: %q", target)
}

// ParseListenAddresses parses and normalizes the server.listen string or falls back to server.port.
// It returns a slice of valid host:port addresses for net.Listen.
func ParseListenAddresses(listenStr string, defaultPort int) ([]string, error) {
	listenStr = strings.TrimSpace(listenStr)
	if listenStr == "" {
		if defaultPort < 1 || defaultPort > 65535 {
			return nil, fmt.Errorf("invalid default port: %d", defaultPort)
		}
		return []string{fmt.Sprintf(":%d", defaultPort)}, nil
	}

	rawTargets := strings.Fields(listenStr)
	normalized := make([]string, 0, len(rawTargets))
	seen := make(map[string]bool)

	for _, target := range rawTargets {
		addr, err := normalizeSingleAddress(target, defaultPort)
		if err != nil {
			return nil, err
		}
		if !seen[addr] {
			seen[addr] = true
			normalized = append(normalized, addr)
		}
	}

	return normalized, nil
}

// ValidateConfig validates both system and inventory configurations against production standards.
func ValidateConfig(sys *SystemConfig, inv *InventoryConfig) []string {
	var errs []string

	if sys == nil {
		errs = append(errs, "system.yaml is empty or missing")
		return errs
	}

	// 1. Server validation
	if sys.Server.Port < 1 || sys.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("invalid server.port: %d (must be 1-65535)", sys.Server.Port))
	}

	if sys.Server.ScrapeInterval < 1 {
		errs = append(errs, fmt.Sprintf("invalid server.scrape_interval: %ds (must be >= 1)", sys.Server.ScrapeInterval))
	}

	if sys.Server.Listen != "" {
		if _, err := ParseListenAddresses(sys.Server.Listen, sys.Server.Port); err != nil {
			errs = append(errs, fmt.Sprintf("invalid server.listen: %v", err))
		}
	}

	for _, entry := range sys.Server.ProbeAllowedTargets {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				errs = append(errs, fmt.Sprintf("invalid CIDR in server.probe_allowed_targets: %q", entry))
			}
		} else if net.ParseIP(entry) == nil && !isValidHostname(entry) {
			errs = append(errs, fmt.Sprintf("invalid IP or hostname in server.probe_allowed_targets: %q", entry))
		}
	}

	// 2. Performance validation
	if sys.Performance.MaxWorkers < 1 {
		errs = append(errs, fmt.Sprintf("invalid performance.max_workers: %d (must be >= 1)", sys.Performance.MaxWorkers))
	}
	if sys.Performance.RequestTimeout < 1 {
		errs = append(errs, fmt.Sprintf("invalid performance.request_timeout: %d (must be >= 1s)", sys.Performance.RequestTimeout))
	}
	if sys.Performance.LoginTimeout < 1 {
		errs = append(errs, fmt.Sprintf("invalid performance.login_timeout: %d (must be >= 1s)", sys.Performance.LoginTimeout))
	}
	if sys.Performance.SocketTimeout < 1 {
		errs = append(errs, fmt.Sprintf("invalid performance.socket_timeout: %d (must be >= 1s)", sys.Performance.SocketTimeout))
	}
	if sys.Performance.SocketTimeout > 0 && sys.Performance.RequestTimeout > 0 && sys.Performance.SocketTimeout > sys.Performance.RequestTimeout {
		errs = append(errs, fmt.Sprintf("invalid performance.socket_timeout (%ds): cannot exceed performance.request_timeout (%ds)", sys.Performance.SocketTimeout, sys.Performance.RequestTimeout))
	}
	if sys.Performance.FailureBackoff < 0 {
		errs = append(errs, fmt.Sprintf("invalid performance.failure_backoff: %d (must be >= 0s)", sys.Performance.FailureBackoff))
	}

	// 3. Logging validation
	lvl := strings.ToUpper(strings.TrimSpace(sys.Logging.Level))
	if lvl != "" {
		if _, ok := ValidLogLevels[lvl]; !ok {
			errs = append(errs, fmt.Sprintf("invalid logging.level: %q (must be DEBUG, INFO, WARN, WARNING, or ERROR)", sys.Logging.Level))
		}
	}

	fac := strings.ToLower(strings.TrimSpace(sys.Logging.SyslogFacility))
	if fac != "" {
		if _, ok := ValidSyslogFacilities[fac]; !ok {
			errs = append(errs, fmt.Sprintf("invalid logging.syslog_facility: %q", sys.Logging.SyslogFacility))
		}
	}

	formatCount := 0
	if sys.Logging.Formats.CEF {
		formatCount++
	}
	if sys.Logging.Formats.RFC5424 {
		formatCount++
	}
	if sys.Logging.Formats.Syslog {
		formatCount++
	}
	if sys.Logging.Formats.Text {
		formatCount++
	}
	if sys.Logging.Formats.JSON {
		formatCount++
	}
	if formatCount > 1 {
		errs = append(errs, "multiple logging formats enabled in system.yaml; please enable only one of cef, rfc5424, syslog, json, or text")
	}

	if sys.Logging.File.Enabled {
		if strings.TrimSpace(sys.Logging.File.Path) == "" {
			errs = append(errs, "logging.file.path cannot be empty when file logging is enabled")
		}
		if sys.Logging.File.MaxSizeMB < 1 {
			errs = append(errs, fmt.Sprintf("invalid logging.file.max_size_mb: %d (must be >= 1)", sys.Logging.File.MaxSizeMB))
		}
		if sys.Logging.File.BackupCount < 0 {
			errs = append(errs, fmt.Sprintf("invalid logging.file.backup_count: %d (must be >= 0)", sys.Logging.File.BackupCount))
		}
	}

	if sys.Logging.Dampening.Enabled {
		if sys.Logging.Dampening.Threshold < 1 {
			errs = append(errs, fmt.Sprintf("invalid logging.dampening.threshold: %d (must be >= 1)", sys.Logging.Dampening.Threshold))
		}
		if sys.Logging.Dampening.Interval < 1 {
			errs = append(errs, fmt.Sprintf("invalid logging.dampening.interval: %d (must be >= 1)", sys.Logging.Dampening.Interval))
		}
	}

	if !sys.Logging.Console && !sys.Logging.File.Enabled {
		errs = append(errs, "both logging.console and logging.file are disabled; no log output would be produced")
	}

	// 4. Inventory validation
	if inv == nil {
		errs = append(errs, "config.yaml is empty or missing")
		return errs
	}

	for colName := range inv.EnabledCollectors {
		if _, ok := ValidCollectorNames[colName]; !ok {
			errs = append(errs, fmt.Sprintf("unknown collector %q in global enabled_collectors (valid: system, interfaces, wireless, peers, airfiber)", colName))
		}
	}

	for k := range inv.Defaults.Labels {
		lowerK := strings.ToLower(strings.TrimSpace(k))
		if lowerK == "name" || lowerK == "ip" {
			errs = append(errs, fmt.Sprintf("defaults.labels contains reserved base label %q", k))
		} else if !IsValidPrometheusLabelName(k) {
			errs = append(errs, fmt.Sprintf("defaults.labels contains invalid Prometheus label name %q (must match [a-zA-Z_][a-zA-Z0-9_]*)", k))
		}
	}

	seenIPs := make(map[string]string)   // normalized IP -> dev.Name
	seenNames := make(map[string]string) // name -> dev.IP

	for idx, dev := range inv.Devices {
		name := strings.TrimSpace(dev.Name)
		if name == "" {
			errs = append(errs, fmt.Sprintf("device #%d is missing 'name'", idx+1))
		} else {
			if existingIP, exists := seenNames[name]; exists {
				errs = append(errs, fmt.Sprintf("duplicate device name %q detected (IPs: %s and %s)", name, existingIP, dev.IP))
			} else {
				seenNames[name] = dev.IP
			}
		}

		ipStr := strings.TrimSpace(dev.IP)
		if !ValidateIP(ipStr) {
			errs = append(errs, fmt.Sprintf("device '%s' has invalid IP: %s", dev.Name, dev.IP))
		} else {
			if existingName, exists := seenIPs[ipStr]; exists {
				errs = append(errs, fmt.Sprintf("duplicate device IP %q detected (used by '%s' and '%s')", ipStr, existingName, dev.Name))
			} else {
				seenIPs[ipStr] = dev.Name
			}
		}

		for colName := range dev.EnabledCollectors {
			if _, ok := ValidCollectorNames[colName]; !ok {
				errs = append(errs, fmt.Sprintf("device '%s' has unknown collector %q in enabled_collectors (valid: system, interfaces, wireless, peers, airfiber)", dev.Name, colName))
			}
		}

		// Detect device configuration properties accidentally placed inside labels
		reservedKeys := map[string]string{
			"ssl":                "ssl (boolean)",
			"username":           "username (string)",
			"password":           "password (string)",
			"enabled_collectors": "enabled_collectors (map)",
		}
		for k := range dev.Labels {
			lowerK := strings.ToLower(strings.TrimSpace(k))
			if desc, isReserved := reservedKeys[lowerK]; isReserved {
				errs = append(errs, fmt.Sprintf("device '%s' has reserved parameter %s placed inside 'labels'; configure it directly under device instead of under 'labels'", dev.Name, desc))
				continue
			}
			if strings.Contains(lowerK, "override") || strings.Contains(lowerK, "optional") {
				errs = append(errs, fmt.Sprintf("device '%s' has unexpected label %q; ensure comment markers '#' were not omitted from example configuration", dev.Name, k))
				continue
			}
			if lowerK == "name" || lowerK == "ip" {
				errs = append(errs, fmt.Sprintf("device '%s' has reserved base label %q in 'labels'", dev.Name, k))
				continue
			}
			if !IsValidPrometheusLabelName(k) {
				errs = append(errs, fmt.Sprintf("device '%s' has invalid Prometheus label name %q (must match [a-zA-Z_][a-zA-Z0-9_]*)", dev.Name, k))
			}
		}
	}

	return errs
}

// ResolveConfigPaths finds configuration files based on CLI parameters and search hierarchy.
func ResolveConfigPaths(cfgDir, customConfig, customSystem string) (string, string) {
	if cfgDir != "" {
		return filepath.Join(cfgDir, "config.yaml"), filepath.Join(cfgDir, "system.yaml")
	}

	cfgPath := customConfig
	sysPath := customSystem

	// If explicit paths already exist on disk, return them directly
	if fileExists(cfgPath) || fileExists(sysPath) {
		return cfgPath, sysPath
	}

	// Check XDG standard configuration directory
	xdgHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			xdgHome = filepath.Join(home, ".config")
		}
	}
	if xdgHome != "" {
		xdgCfg := filepath.Join(xdgHome, "ubnt-exporter", "config.yaml")
		xdgSys := filepath.Join(xdgHome, "ubnt-exporter", "system.yaml")
		if fileExists(xdgCfg) && fileExists(xdgSys) {
			return xdgCfg, xdgSys
		}
	}

	// Check system-wide /etc/ubnt-exporter
	etcCfg := "/etc/ubnt-exporter/config.yaml"
	etcSys := "/etc/ubnt-exporter/system.yaml"
	if fileExists(etcCfg) && fileExists(etcSys) {
		return etcCfg, etcSys
	}

	return cfgPath, sysPath
}

func fileExists(p string) bool {
	// #nosec G703 -- path checking for config discovery is performed on administrator-specified search paths
	info, err := os.Stat(filepath.Clean(p))
	return err == nil && !info.IsDir()
}

// IsTargetAllowed checks if target matches an entry or CIDR in the allowed targets list.
// If allowedList is empty, all targets are allowed.
func IsTargetAllowed(target string, allowedList []string) bool {
	if len(allowedList) == 0 {
		return true
	}
	target = strings.TrimSpace(target)
	targetIP := net.ParseIP(target)

	for _, entry := range allowedList {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// Check CIDR
		if strings.Contains(entry, "/") {
			_, ipNet, err := net.ParseCIDR(entry)
			if err == nil && targetIP != nil && ipNet.Contains(targetIP) {
				return true
			}
		} else {
			// Exact IP or hostname match
			if targetIP != nil && net.ParseIP(entry) != nil {
				if targetIP.Equal(net.ParseIP(entry)) {
					return true
				}
			} else if strings.EqualFold(target, entry) {
				return true
			}
		}
	}
	return false
}
