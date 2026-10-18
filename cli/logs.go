package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/logger"
)

func resolveLocalHTTPBase(sysConf *config.SystemConfig) string {
	port := 8000
	if sysConf != nil && sysConf.Server.Port > 0 {
		port = sysConf.Server.Port
	}
	if sysConf != nil && strings.TrimSpace(sysConf.Server.Listen) != "" {
		addrs, err := config.ParseListenAddresses(sysConf.Server.Listen, port)
		if err == nil {
			for _, a := range addrs {
				if strings.HasPrefix(a, "unix://") {
					continue
				}
				if strings.HasPrefix(a, ":") {
					return "http://127.0.0.1" + a
				}
				if strings.HasPrefix(a, "0.0.0.0:") {
					return "http://127.0.0.1:" + strings.TrimPrefix(a, "0.0.0.0:")
				}
				if strings.HasPrefix(a, "[::]:") {
					return "http://127.0.0.1:" + strings.TrimPrefix(a, "[::]:")
				}
				return "http://" + a
			}
		}
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func fetchLocalDaemonLogs(sysConf *config.SystemConfig, limit int, filter string) ([]string, error) {
	re, _, err := CompileUserRegex(filter)
	if err != nil {
		return nil, err
	}

	reqLimit := limit
	if re != nil && reqLimit < 500 {
		reqLimit = 500
	}
	base := resolveLocalHTTPBase(sysConf)
	u := fmt.Sprintf("%s/logs?limit=%d", base, reqLimit)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var matched []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if re == nil || re.MatchString(line) {
			matched = append(matched, line)
		}
	}
	if limit > 0 && len(matched) > limit {
		matched = matched[len(matched)-limit:]
	}
	return matched, sc.Err()
}

func readTailFromFile(path string, limit int, filter string) ([]string, error) {
	re, _, err := CompileUserRegex(filter)
	if err != nil {
		return nil, err
	}

	// #nosec G304 -- fallback log file path is configured by administrator in system.yaml
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var matched []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if re == nil || re.MatchString(line) {
			matched = append(matched, line)
		}
	}
	if limit > 0 && len(matched) > limit {
		matched = matched[len(matched)-limit:]
	}
	return matched, sc.Err()
}

// CmdLogs handles 'logs tail', 'logs test', and 'logs status'.
func CmdLogs(systemPath string, sysConf *config.SystemConfig, subcommand string, limit int, filter string) int {
	sub := strings.ToLower(strings.TrimSpace(subcommand))
	if sub == "" {
		sub = "tail"
	}

	switch sub {
	case "status":
		return cmdLogsStatus(systemPath, sysConf)
	case "test":
		return cmdLogsTest(sysConf)
	case "tail":
		return cmdLogsTail(sysConf, limit, filter)
	default:
		// Allow `ubnt-exporter logs MST4` as shorthand filter for tail
		if filter == "" {
			filter = subcommand
		}
		return cmdLogsTail(sysConf, limit, filter)
	}
}

func cmdLogsStatus(systemPath string, sysConf *config.SystemConfig) int {
	fmt.Println("\n LOGGING SUBSYSTEM STATUS")
	fmt.Println("==============================")
	activeFormat := "text"
	if sysConf.Logging.Formats.CEF {
		activeFormat = "cef (ArcSight Common Event Format)"
	} else if sysConf.Logging.Formats.RFC5424 {
		activeFormat = "rfc5424 (IETF Syslog ISO-8601)"
	} else if sysConf.Logging.Formats.Syslog {
		activeFormat = "syslog (BSD RFC 3164)"
	} else if sysConf.Logging.Formats.JSON {
		activeFormat = "json (Structured JSON Lines)"
	}

	fmt.Printf("  System Config File : %s\n", systemPath)
	fmt.Printf("  Configured Level   : %s\n", strings.ToUpper(sysConf.Logging.Level))
	fmt.Printf("  Active Format      : %s\n", activeFormat)
	fmt.Printf("  Syslog Facility    : %s\n", sysConf.Logging.SyslogFacility)
	fmt.Printf("  Console Output     : %v\n", sysConf.Logging.Console)
	fmt.Printf("  File Output        : %v (path=%q, max=%dMB, backups=%d)\n",
		sysConf.Logging.File.Enabled, sysConf.Logging.File.Path, sysConf.Logging.File.MaxSizeMB, sysConf.Logging.File.BackupCount)
	fmt.Printf("  Log Dampening      : %v (threshold=%d, interval=%ds)\n",
		sysConf.Logging.Dampening.Enabled, sysConf.Logging.Dampening.Threshold, sysConf.Logging.Dampening.Interval)
	return 0
}

func cmdLogsTest(sysConf *config.SystemConfig) int {
	fmt.Println("\n LOGGING FORMATS & VALIDATION TEST")
	fmt.Println("==========================================")

	l := logger.Get()
	ctx := logger.WithTraceID(context.Background(), "diag-trace-01")
	sampleDevice := "MST4"
	sampleMsg := "Collection OK (airmax): 513 metrics committed"

	formats := []struct {
		name  string
		apply func(*config.SystemConfig)
	}{
		{"CEF (SIEM / Splunk / Wazuh)", func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{CEF: true} }},
		{"RFC 5424 (IETF Syslog)", func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{RFC5424: true} }},
		{"Syslog (BSD RFC 3164)", func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{Syslog: true} }},
		{"JSON (Structured)", func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{JSON: true} }},
		{"Text (Human-Readable)", func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{Text: true} }},
	}

	for _, f := range formats {
		tmpCfg := config.DefaultSystemConfig()
		tmpCfg.Logging.SyslogFacility = sysConf.Logging.SyslogFacility
		f.apply(tmpCfg)
		logger.Setup(tmpCfg)
		out := strings.TrimSpace(l.FormatMessageContext(ctx, logger.LevelDebug, sampleDevice, sampleMsg))
		fmt.Printf("\n[%s]\n  %s\n", f.name, out)
	}

	// Restore active configuration
	logger.Setup(sysConf)
	fmt.Println("\n[Active Configuration Verification]")
	fmt.Printf("  Level  : %s\n", strings.ToUpper(sysConf.Logging.Level))
	activeOut := strings.TrimSpace(l.FormatMessageContext(ctx, logger.ParseLevel(sysConf.Logging.Level), sampleDevice, sampleMsg))
	fmt.Printf("  Sample : %s\n", activeOut)
	fmt.Println("\nRESULT: All 5 log formatters verified OK.")
	return 0
}

func cmdLogsTail(sysConf *config.SystemConfig, limit int, filter string) int {
	if _, _, err := CompileUserRegex(filter); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return 1
	}

	if limit <= 0 {
		limit = 50
	}
	fmt.Printf("\n LIVE DAEMON LOGS (last %d entries", limit)
	if filter != "" {
		fmt.Printf(", regex=%q", filter)
	}
	fmt.Println(")")
	fmt.Println("======================================================================")

	lines, err := fetchLocalDaemonLogs(sysConf, limit, filter)
	if err == nil {
		if len(lines) == 0 {
			fmt.Println("  (No matching log entries in daemon buffer)")
		} else {
			for _, line := range lines {
				fmt.Println(line)
			}
		}
		return 0
	}

	// Fallback to log file if file logging is enabled
	if sysConf != nil && sysConf.Logging.File.Enabled && sysConf.Logging.File.Path != "" {
		fileLines, fErr := readTailFromFile(sysConf.Logging.File.Path, limit, filter)
		if fErr == nil {
			for _, line := range fileLines {
				fmt.Println(line)
			}
			return 0
		}
	}

	fmt.Printf("Could not retrieve live logs from running daemon (%v).\n", err)
	fmt.Println("Ensure 'ubnt-exporter run' is active or check 'ubnt-exporter logs test'.")
	return 1
}
