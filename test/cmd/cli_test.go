package cmd_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"ubnt-exporter/cli"
	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/definitions"
	"ubnt-exporter/internal/logger"
)

func TestCLI_Help(t *testing.T) {
	if code := cli.Execute([]string{"-h"}); code != 0 {
		t.Errorf("expected exit code 0 for -h, got %d", code)
	}
}

func TestCLI_Version(t *testing.T) {
	if code := cli.Execute([]string{"-v"}); code != 0 {
		t.Errorf("expected exit code 0 for -v, got %d", code)
	}
}

func TestCLI_Check_InvalidConfig(t *testing.T) {
	if code := cli.Execute([]string{"check", "-c", "/nonexistent/config.yaml"}); code == 0 {
		t.Errorf("expected non-zero exit code for nonexistent config")
	}
}

func TestCLI_Check_ValidConfigExample(t *testing.T) {
	if code := cli.Execute([]string{"check", "-c", "../../examples/config.yaml.example", "-s", "../../examples/system.yaml.example"}); code != 0 {
		t.Errorf("expected exit code 0 for valid example configs, got %d", code)
	}
}

func TestCLI_Show_SanitizedConfig(t *testing.T) {
	if code := cli.Execute([]string{"show", "-c", "../../examples/config.yaml.example", "-s", "../../examples/system.yaml.example"}); code != 0 {
		t.Errorf("expected exit code 0 for show, got %d", code)
	}
}

func TestCLI_Run_InvalidConfig_FatalExit(t *testing.T) {
	var fatalCalled bool
	logger.SetExitFunc(func(_ int) {
		fatalCalled = true
	})
	defer logger.ResetExitFunc()

	code := cli.Execute([]string{"run", "-c", "/nonexistent/config.yaml"})
	if code == 0 && !fatalCalled {
		t.Fatalf("expected non-zero exit code or fatal exit for nonexistent config")
	}
}

func TestCLI_Run_ValidationFailure_FatalExit(t *testing.T) {
	tmpDir := t.TempDir()
	badSys := filepath.Join(tmpDir, "system.yaml")
	_ = os.WriteFile(badSys, []byte("server:\n  port: 99999\n"), 0600)
	badCfg := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(badCfg, []byte("devices: []\n"), 0600)

	var fatalCalled bool
	logger.SetExitFunc(func(_ int) {
		fatalCalled = true
	})
	defer logger.ResetExitFunc()

	code := cli.Execute([]string{"run", "-c", badCfg, "-s", badSys})
	if code == 0 && !fatalCalled {
		t.Fatalf("expected non-zero exit code or fatal exit for validation error")
	}
}

func TestCLI_CmdRunContext_LifecycleAndEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	sysPath := filepath.Join(tmpDir, "system.yaml")
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	daemonPort := 18991
	daemonAddr := fmt.Sprintf("127.0.0.1:%d", daemonPort)

	_ = os.WriteFile(sysPath, []byte(fmt.Sprintf(`server:
  port: %d
  listen: "%s"
  scrape_interval: 10
  probe_allowed_targets:
    - "127.0.0.1"
performance:
  max_workers: 5
  login_timeout: 1
  request_timeout: 1
  socket_timeout: 1
logging:
  level: DEBUG
  console: true
  formats:
    cef: false
    text: true
`, daemonPort, daemonAddr)), 0600)

	_ = os.WriteFile(cfgPath, []byte(`devices:
  - name: "DAEMON-AP"
    ip: "127.0.0.1"
    username: "u"
    password: "p"
`), 0600)

	sysConf, _ := config.LoadSystemConfig(sysPath)
	invConf, _ := config.LoadInventoryConfig(cfgPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reloadCh := make(chan struct{}, 1)

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		cli.CmdRunContext(ctx, reloadCh, cfgPath, sysPath, sysConf, invConf)
	}()

	time.Sleep(300 * time.Millisecond)

	client := &http.Client{Timeout: 2 * time.Second}

	// 1. GET /metrics
	respMetrics, err := client.Get(fmt.Sprintf("http://%s/metrics", daemonAddr))
	if err != nil || respMetrics.StatusCode != http.StatusOK {
		t.Fatalf("failed GET /metrics: %v", err)
	}
	_ = respMetrics.Body.Close()

	// 2. GET /logs
	respLogs, err := client.Get(fmt.Sprintf("http://%s/logs?limit=5", daemonAddr))
	if err != nil || respLogs.StatusCode != http.StatusOK {
		t.Errorf("failed GET /logs: %v", err)
	} else {
		_ = respLogs.Body.Close()
	}

	// 3. GET /probe valid target
	reqProbe, _ := http.NewRequest("GET", fmt.Sprintf("http://%s/probe?target=127.0.0.1", daemonAddr), nil)
	reqProbe.Header.Set("X-Request-ID", "probe-test-trace-01")
	respProbe, err := client.Do(reqProbe)
	if err != nil {
		t.Errorf("failed GET /probe: %v", err)
	} else {
		_ = respProbe.Body.Close()
	}

	// 4. GET /probe missing target
	respProbeMissing, _ := client.Get(fmt.Sprintf("http://%s/probe", daemonAddr))
	if respProbeMissing == nil || respProbeMissing.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for /probe without target")
	} else {
		_ = respProbeMissing.Body.Close()
	}

	// 5. GET /probe disallowed target
	respProbeForbidden, _ := client.Get(fmt.Sprintf("http://%s/probe?target=192.168.99.99", daemonAddr))
	if respProbeForbidden == nil || respProbeForbidden.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for disallowed target")
	} else {
		_ = respProbeForbidden.Body.Close()
	}

	// 6. Trigger in-process configuration reload
	reloadCh <- struct{}{}
	time.Sleep(200 * time.Millisecond)

	cancel()
	<-doneCh

	// Test IsValidTraceID
	if !cli.IsValidTraceID("valid-trace_123.abc") {
		t.Errorf("expected valid trace ID")
	}
	if cli.IsValidTraceID("") || cli.IsValidTraceID("bad trace with spaces") || cli.IsValidTraceID(strings.Repeat("x", 65)) {
		t.Errorf("expected invalid trace IDs to be rejected")
	}

	// Test IsLoopbackRequest
	reqLoopback := &http.Request{RemoteAddr: "127.0.0.1:12345"}
	if !cli.IsLoopbackRequest(reqLoopback) {
		t.Errorf("expected 127.0.0.1 to be recognized as loopback")
	}
	reqUnix := &http.Request{RemoteAddr: "@"}
	if !cli.IsLoopbackRequest(reqUnix) {
		t.Errorf("expected @ to be recognized as loopback socket")
	}
	reqExt := &http.Request{RemoteAddr: "192.168.1.100:12345"}
	if cli.IsLoopbackRequest(reqExt) {
		t.Errorf("expected external IP not to be recognized as loopback")
	}
}

func TestCLI_ParseArgs(t *testing.T) {
	opts, exitCode, handled := cli.ParseArgs([]string{"logs", "tail", "-n", "25", "-r", "ERROR|WARN", "-t", "MST4", "--cfg-dir", "/conf", "-c", "/custom/cfg.yaml", "-s", "/custom/sys.yaml"})
	if handled || exitCode != 0 {
		t.Errorf("expected handled=false and exitCode=0, got handled=%v exitCode=%d", handled, exitCode)
	}
	if opts.Command != "logs" || opts.Subcommand != "tail" {
		t.Errorf("expected Command='logs' and Subcommand='tail', got %q %q", opts.Command, opts.Subcommand)
	}
	if opts.Lines != 25 {
		t.Errorf("expected Lines=25, got %d", opts.Lines)
	}
	if opts.Regex != "ERROR|WARN" {
		t.Errorf("expected Regex='ERROR|WARN', got %q", opts.Regex)
	}
	if opts.Target != "MST4" {
		t.Errorf("expected Target='MST4', got %q", opts.Target)
	}
	if opts.CfgDir != "/conf" || opts.ConfigPath != "/custom/cfg.yaml" || opts.SystemPath != "/custom/sys.yaml" {
		t.Errorf("unexpected config paths: %q, %q, %q", opts.CfgDir, opts.ConfigPath, opts.SystemPath)
	}
}

func TestCLI_SelfTest_MetricsAndCollectors(t *testing.T) {
	code := cli.Execute([]string{"test", "metrics", "-c", "../../examples/config.yaml.example", "-s", "../../examples/system.yaml.example"})
	if code != 0 {
		t.Errorf("expected exit code 0 for 'test metrics', got %d", code)
	}

	code = cli.Execute([]string{"test", "collectors", "-c", "../../examples/config.yaml.example", "-s", "../../examples/system.yaml.example"})
	if code != 0 {
		t.Errorf("expected exit code 0 for 'test collectors', got %d", code)
	}
}

func TestCLI_LogsCommands(t *testing.T) {
	code := cli.Execute([]string{"logs", "test", "-c", "../../examples/config.yaml.example", "-s", "../../examples/system.yaml.example"})
	if code != 0 {
		t.Errorf("expected exit code 0 for 'logs test', got %d", code)
	}

	code = cli.Execute([]string{"logs", "status", "-c", "../../examples/config.yaml.example", "-s", "../../examples/system.yaml.example"})
	if code != 0 {
		t.Errorf("expected exit code 0 for 'logs status', got %d", code)
	}

	// Test all 5 logging formats in logs status
	for _, formatSetup := range []func(*config.SystemConfig){
		func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{CEF: true} },
		func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{RFC5424: true} },
		func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{Syslog: true} },
		func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{JSON: true} },
		func(c *config.SystemConfig) { c.Logging.Formats = config.FormatsConfig{Text: true} },
	} {
		tmpSys := config.DefaultSystemConfig()
		formatSetup(tmpSys)
		tmpPath := filepath.Join(t.TempDir(), "system.yaml")
		_ = os.WriteFile(tmpPath, []byte("logging:\n  level: INFO\n"), 0600)
		cli.CmdLogs(tmpPath, tmpSys, "status", 50, "")
	}

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "ubnt-exporter.log")
	_ = os.WriteFile(logFile, []byte("2026-10-17T12:00:00Z | INFO  | main | Started daemon\n2026-10-17T12:01:00Z | DEBUG | scraper | Scraping 10.1.1.5\n"), 0600)
	sysFile := filepath.Join(tmpDir, "system.yaml")
	_ = os.WriteFile(sysFile, []byte("server:\n  port: 18999\nlogging:\n  level: DEBUG\n  file:\n    enabled: true\n    path: "+logFile+"\n"), 0600)

	code = cli.Execute([]string{"logs", "tail", "-n", "10", "-r", "DEBUG.*10\\.1\\.1\\.5", "-s", sysFile})
	if code != 0 {
		t.Errorf("expected exit code 0 for 'logs tail' fallback to file with RE2 regex, got %d", code)
	}
}

func TestCLI_RegexPresetsAndRE2Validation(t *testing.T) {
	// 1. Verify pre-compiled MustCompile semantic presets (Spanish & English aliases)
	for _, alias := range []string{"signal", "señal", "cpu", "clientes", "clients", "rf", "capacidad", "trafico", "vlan", "gps", "ntp", "errores"} { //nolint:misspell // Spanish alias
		re, desc, err := cli.CompileUserRegex(alias)
		if err != nil || re == nil {
			t.Fatalf("expected valid preset for %q, got err=%v", alias, err)
		}
		if !strings.HasPrefix(desc, "preset:") {
			t.Errorf("expected preset description for %q, got %q", alias, desc)
		}
	}

	// 2. Verify dynamic RE2 regexp.Compile
	re, _, err := cli.CompileUserRegex("ubnt_peer_(rx|tx)_signal_dbm")
	if err != nil || re == nil {
		t.Fatalf("expected valid dynamic RE2 regex, got err=%v", err)
	}
	if !re.MatchString("ubnt_peer_rx_signal_dbm") {
		t.Errorf("expected regex to match 'ubnt_peer_rx_signal_dbm'")
	}

	// 3. Verify clean error capture on unsupported RE2 lookahead (?=...) and malformed brackets
	_, _, lookaheadErr := cli.CompileUserRegex("ubnt_(?=signal)")
	if lookaheadErr == nil || !strings.Contains(lookaheadErr.Error(), "RE2") || !strings.Contains(lookaheadErr.Error(), "lookarounds") {
		t.Errorf("expected descriptive RE2 lookaround error, got: %v", lookaheadErr)
	}

	_, _, syntaxErr := cli.CompileUserRegex("[unclosed_bracket")
	if syntaxErr == nil || !strings.Contains(syntaxErr.Error(), "invalid RE2 regular expression") {
		t.Errorf("expected invalid RE2 regular expression error, got: %v", syntaxErr)
	}
}

func TestCLI_MetricsCatalogAndContextualHelp(t *testing.T) {
	// Test metrics dictionary explorer with RE2 filter
	code := cli.Execute([]string{"metrics", "-r", "gps|ntp"})
	if code != 0 {
		t.Errorf("expected exit code 0 for 'metrics -r gps|ntp', got %d", code)
	}

	// Test invalid regex in metrics command returns exit code 1 cleanly without panic
	code = cli.Execute([]string{"metrics", "-r", "invalid(?=lookahead)"})
	if code == 0 {
		t.Errorf("expected non-zero exit code for invalid RE2 lookahead in metrics command")
	}

	// Test query presets listing
	code = cli.Execute([]string{"query", "presets", "-c", "../../examples/config.yaml.example", "-s", "../../examples/system.yaml.example"})
	if code != 0 {
		t.Errorf("expected exit code 0 for 'query presets', got %d", code)
	}

	// Test contextual help (-h / --help) on specific subcommands and -log flag
	for _, helpArgs := range [][]string{
		{"query", "-h"},
		{"inspect", "--help"},
		{"logs", "-h"},
		{"metrics", "--help"},
		{"-log"},
	} {
		if c := cli.Execute(helpArgs); c != 0 {
			t.Errorf("expected exit code 0 for %v, got %d", helpArgs, c)
		}
	}
}

func TestCLI_UnknownCommand(t *testing.T) {
	code := cli.Execute([]string{"nonexistent-cmd"})
	if code == 0 {
		t.Errorf("expected non-zero exit code for unknown command")
	}
}

func TestCLI_LiveMockDevice_DiagInspectExportQuery(t *testing.T) {
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1.0/user/login":
			w.Header().Set("X-Auth-Token", "mock-cli-token-123")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/v1.0/statistics":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"host": {"hostname": "MST4", "uptime": 3600, "cpuload": 12.5, "fwversion": "v8.7.11", "devmodel": "LiteAP AC", "netrole": "bridge", "temperature": 48},
				"interfaces": [{"ifname": "ath0", "enabled": true, "status": {"plugged": true, "speed": 1000, "duplex": true}, "statistics": {"rx_bytes": 1024, "tx_bytes": 2048}}],
				"wireless": {"essid": "AP-SECTOR", "mode": "ap-ptmp", "frequency": 5500, "chanbw": 40, "noisef": -90, "txpower": 22, "count": 1, "polling": {"dl_capacity": 85000, "ul_capacity": 70000, "use": 15, "tx_use": 10, "rx_use": 5}, "sta": [{"mac": "AA:BB:CC:DD:EE:FF", "lastip": "10.1.1.50", "signal": -52, "noisefloor": -90, "chain_rssi": [-53, -55], "tx": 300, "rx": 300, "uptime": 1800, "remote": {"hostname": "CPE-01", "platform": "LiteBeam 5AC", "version": "WA.v8.7.11", "cpuload": 10, "signal": -54, "noisefloor": -91}}]}
			}`))
		case "/metrics":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("# HELP ubnt_scrape_success Scrape success\n# TYPE ubnt_scrape_success gauge\nubnt_scrape_success{ip=\"10.1.1.5\",name=\"MST4\"} 1\n"))
		case "/logs":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("2026-10-17 12:00:00 | INFO | [MST4] Collection OK\n2026-10-17 12:00:05 | ERROR | [MST4] Sample error line\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockSrv.Close()

	hostAddr := strings.TrimPrefix(mockSrv.URL, "http://")
	tmpDir := t.TempDir()
	sysPath := filepath.Join(tmpDir, "system.yaml")
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	_ = os.WriteFile(sysPath, []byte("server:\n  port: 18080\n  listen: \""+hostAddr+"\"\n  scrape_interval: 30\nperformance:\n  max_workers: 10\n  login_timeout: 5\n  request_timeout: 10\n  socket_timeout: 2\n  failure_backoff: 30\nlogging:\n  level: INFO\n  console: true\n  formats:\n    text: true\n"), 0600)
	_ = os.WriteFile(cfgPath, []byte("devices:\n  - name: \"MST4\"\n    ip: \""+hostAddr+"\"\n    ssl: false\n    username: \"admin\"\n    password: \"secret\"\n    labels:\n      tower: \"tower-b\"\n    enabled_collectors:\n      peers: true\n"), 0600)

	for _, cmdArgs := range [][]string{
		{"check", "-c", "../../examples/config.yaml.example", "-s", "../../examples/system.yaml.example"},
		{"show", "-c", cfgPath, "-s", sysPath},
		{"diag", "-t", "^MST.*", "-c", cfgPath, "-s", sysPath},
		{"inspect", "-t", "tower-b", "-r", "signal|snr", "-c", cfgPath, "-s", sysPath},
		{"export", "-t", "MST4", "-r", "ubnt_ap_.*", "-c", cfgPath, "-s", sysPath},
		{"query", "señal", "-t", "MST4", "-c", cfgPath, "-s", sysPath},
		{"query", "-r", "cpu|ram", "-t", "tower-b", "-c", cfgPath, "-s", sysPath},
		{"test", "all", "-t", "MST4", "-c", cfgPath, "-s", sysPath},
		{"logs", "tail", "-n", "10", "-r", "ERROR", "-c", cfgPath, "-s", sysPath},
		{"-v"},
		{"-h"},
		{"diag", "-h"},
		{"export", "-h"},
		{"test", "-h"},
	} {
		if code := cli.Execute(cmdArgs); code != 0 {
			t.Errorf("expected exit code 0 for %v, got %d", cmdArgs, code)
		}
	}

	// Verify reload aborts cleanly when no daemon is running or config is invalid
	if code := cli.Execute([]string{"reload", "-c", "../../examples/config.yaml.example", "-s", "../../examples/system.yaml.example"}); code == 0 {
		t.Errorf("expected non-zero exit code for reload when no daemon is running")
	}
	if code := cli.Execute([]string{"reload", "-c", cfgPath, "-s", sysPath}); code == 0 {
		t.Errorf("expected non-zero exit code for reload when config has validation errors")
	}

	// Test zero matching targets and invalid target regexes across commands
	cli.Execute([]string{"diag", "-t", "nonexistent_device", "-c", cfgPath, "-s", sysPath})
	cli.Execute([]string{"inspect", "-t", "nonexistent_device", "-c", cfgPath, "-s", sysPath})
	cli.Execute([]string{"export", "-t", "nonexistent_device", "-c", cfgPath, "-s", sysPath})
	cli.Execute([]string{"query", "signal", "-t", "nonexistent_device", "-c", cfgPath, "-s", sysPath})
	cli.Execute([]string{"test", "connectivity", "-t", "nonexistent_device", "-c", cfgPath, "-s", sysPath})

	cli.Execute([]string{"diag", "-t", "invalid(?=regex)", "-c", cfgPath, "-s", sysPath})
	cli.Execute([]string{"inspect", "-t", "invalid(?=regex)", "-c", cfgPath, "-s", sysPath})
	cli.Execute([]string{"export", "-t", "invalid(?=regex)", "-c", cfgPath, "-s", sysPath})
	cli.Execute([]string{"query", "signal", "-t", "invalid(?=regex)", "-c", cfgPath, "-s", sysPath})
	cli.Execute([]string{"test", "connectivity", "-t", "invalid(?=regex)", "-c", cfgPath, "-s", sysPath})

	// Verify interactive shell REPL handles pasted docker exec commands, help, and exit cleanly
	origStdin := os.Stdin
	rPipe, wPipe, pErr := os.Pipe()
	if pErr == nil {
		_, _ = wPipe.WriteString("$ docker exec -it ubnt-exporter ubnt-exporter logs status\nhelp query\n?\nclear\nrun\ncli\nexit\n")
		_ = wPipe.Close()
		os.Stdin = rPipe
		defer func() {
			os.Stdin = origStdin
			_ = rPipe.Close()
		}()
		if code := cli.Execute([]string{"cli", "-c", cfgPath, "-s", sysPath}); code != 0 {
			t.Errorf("expected exit code 0 for interactive shell, got %d", code)
		}
	}
}

func TestCLI_FindRunningDaemonPID_And_CmdReload(t *testing.T) {
	tmpProc := t.TempDir()
	restoreProc := cli.SetProcFSForTesting(tmpProc)
	defer restoreProc()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	sysPath := filepath.Join(t.TempDir(), "system.yaml")
	_ = os.WriteFile(cfgPath, []byte("devices: []\n"), 0600)
	_ = os.WriteFile(sysPath, []byte("server:\n  port: 8000\n"), 0600)
	sysConf, sysErr := config.LoadSystemConfig(sysPath)
	invConf, invErr := config.LoadInventoryConfig(cfgPath)

	// CmdReload should fail locating running daemon
	if code := cli.CmdReload(cfgPath, sysPath, sysConf, sysErr, invConf, invErr); code != 1 {
		t.Errorf("expected CmdReload to return 1 when no daemon is running, got %d", code)
	}

	// CmdReload with invalid configuration aborts before locating daemon
	badSysConf := &config.SystemConfig{}
	if code := cli.CmdReload(cfgPath, sysPath, badSysConf, nil, invConf, nil); code == 0 {
		t.Errorf("expected CmdReload to abort on invalid configuration")
	}

	// Create simulated /proc entries:
	_ = os.Mkdir(filepath.Join(tmpProc, "self"), 0750)
	_ = os.Mkdir(filepath.Join(tmpProc, "100"), 0750)
	_ = os.WriteFile(filepath.Join(tmpProc, "100", "cmdline"), []byte("nginx\x00-g\x00daemon off;\x00"), 0600)
	_ = os.Mkdir(filepath.Join(tmpProc, "101"), 0750)
	_ = os.WriteFile(filepath.Join(tmpProc, "101", "cmdline"), []byte("ubnt-exporter\x00check\x00"), 0600)
	_ = os.Mkdir(filepath.Join(tmpProc, "102"), 0750)
	_ = os.WriteFile(filepath.Join(tmpProc, "102", "cmdline"), []byte("/usr/local/bin/ubnt-exporter\x00run\x00"), 0600)

	// Mock signalDaemon returning error
	restoreSig := cli.SetSignalDaemonForTesting(func(_ int, _ os.Signal) error {
		return fmt.Errorf("permission denied")
	})

	if code := cli.CmdReload(cfgPath, sysPath, sysConf, sysErr, invConf, invErr); code != 1 {
		t.Errorf("expected CmdReload to return 1 when signalDaemon returns error, got %d", code)
	}

	// Mock signalDaemon succeeding
	signaledPID := 0
	var receivedSig os.Signal
	cli.SetSignalDaemonForTesting(func(pid int, sig os.Signal) error {
		signaledPID = pid
		receivedSig = sig
		return nil
	})
	defer restoreSig()

	if code := cli.CmdReload(cfgPath, sysPath, sysConf, sysErr, invConf, invErr); code != 0 {
		t.Errorf("expected CmdReload to return 0 when signaling succeeds, got %d", code)
	}
	if signaledPID != 102 {
		t.Errorf("expected signaled PID 102, got %d", signaledPID)
	}
	if receivedSig != syscall.SIGHUP {
		t.Errorf("expected SIGHUP signal, got %v", receivedSig)
	}
}

func TestCLI_RunSuiteMetrics_AllBranches(t *testing.T) {
	if cli.RunSuiteMetricsWith(map[string]definitions.MetricDefinition{}) {
		t.Errorf("expected RunSuiteMetricsWith to fail on empty defs")
	}

	if cli.RunSuiteMetricsWith(map[string]definitions.MetricDefinition{
		"m1": {Name: "missing_prefix", Help: "help", Type: definitions.TypeGauge},
	}) {
		t.Errorf("expected RunSuiteMetricsWith to fail on missing ubnt_ prefix")
	}

	if cli.RunSuiteMetricsWith(map[string]definitions.MetricDefinition{
		"m2": {Name: "ubnt_m2", Help: "  ", Type: definitions.TypeGauge},
	}) {
		t.Errorf("expected RunSuiteMetricsWith to fail on empty help")
	}

	if cli.RunSuiteMetricsWith(map[string]definitions.MetricDefinition{
		"m3": {Name: "ubnt_m3", Help: "help", Type: "unknown_type"},
	}) {
		t.Errorf("expected RunSuiteMetricsWith to fail on invalid type")
	}

	if cli.RunSuiteMetricsWith(map[string]definitions.MetricDefinition{
		"m4": {Name: "ubnt_m4", Help: "help", Type: definitions.TypeGauge, Labels: []string{"bad-label-name"}},
	}) {
		t.Errorf("expected RunSuiteMetricsWith to fail on invalid label name")
	}

	if !cli.RunSuiteMetricsWith(map[string]definitions.MetricDefinition{
		"valid": {Name: "ubnt_valid", Help: "valid gauge", Type: definitions.TypeGauge, Labels: []string{"name", "ip"}},
	}) {
		t.Errorf("expected RunSuiteMetricsWith to pass on valid def")
	}
}

func TestCLI_ParseArgs_Flags_And_Commands_Exhaustive(t *testing.T) {
	if code := cli.Execute([]string{"--log"}); code != 0 {
		t.Errorf("expected exit 0 for --log")
	}
	if code := cli.Execute([]string{"-v"}); code != 0 {
		t.Errorf("expected exit 0 for -v")
	}

	for _, cmd := range []string{"query", "inspect", "diag", "export", "metrics", "logs", "check", "show", "reload", "test", "cli", "run"} {
		if code := cli.Execute([]string{"help", cmd}); code != 0 {
			t.Errorf("expected exit 0 for help %s", cmd)
		}
		if code := cli.Execute([]string{cmd, "--help"}); code != 0 {
			t.Errorf("expected exit 0 for %s --help", cmd)
		}
	}

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	sysFile := filepath.Join(tmpDir, "system.yaml")
	_ = os.WriteFile(cfgFile, []byte("devices: []\n"), 0600)
	_ = os.WriteFile(sysFile, []byte("server:\n  port: 8000\n"), 0600)

	cli.Execute([]string{"--config=" + cfgFile, "--system=" + sysFile, "check"})
	cli.Execute([]string{"-c=" + cfgFile, "-s=" + sysFile, "check"})
	cli.Execute([]string{"--cfg-dir=" + tmpDir, "check"})
	cli.Execute([]string{"--cfg-dir", tmpDir, "check"})
	cli.Execute([]string{"-t=TEST", "-r=.*", "-n=20", "query", "signal", "-c", cfgFile, "-s", sysFile})
	cli.Execute([]string{"--target=TEST", "--regex=.*", "--lines=20", "query", "signal", "-c", cfgFile, "-s", sysFile})
	cli.Execute([]string{"--regexp=.*", "query", "signal", "-c", cfgFile, "-s", sysFile})

	if code := cli.Execute([]string{"nonexistent_command"}); code != 1 {
		t.Errorf("expected exit 1 for nonexistent command, got %d", code)
	}

	if code := cli.Execute([]string{}); code != 0 {
		t.Errorf("expected exit 0 for empty args, got %d", code)
	}

	if code := cli.Execute([]string{"metrics"}); code != 0 {
		t.Errorf("expected exit 0 for metrics catalog")
	}
	if code := cli.Execute([]string{"metrics", "-r", "signal"}); code != 0 {
		t.Errorf("expected exit 0 for metrics catalog with regex")
	}
	if code := cli.Execute([]string{"metrics", "-r", "invalid(?=regex)"}); code != 1 {
		t.Errorf("expected exit 1 for metrics catalog with invalid regex")
	}

	if code := cli.Execute([]string{"query", "presets", "-c", cfgFile, "-s", sysFile}); code != 0 {
		t.Errorf("expected exit 0 for query presets")
	}
	if code := cli.Execute([]string{"query", "list", "-c", cfgFile, "-s", sysFile}); code != 0 {
		t.Errorf("expected exit 0 for query list")
	}
	if code := cli.Execute([]string{"query", "-c", cfgFile, "-s", sysFile}); code != 0 {
		t.Errorf("expected exit 0 for query empty")
	}
	if code := cli.Execute([]string{"query", "invalid(?=regex)", "-c", cfgFile, "-s", sysFile}); code != 1 {
		t.Errorf("expected exit 1 for query with invalid regex")
	}

	if code := cli.Execute([]string{"logs", "status", "-s", sysFile}); code != 0 {
		t.Errorf("expected exit 0 for logs status")
	}
	if code := cli.Execute([]string{"logs", "test", "-s", sysFile}); code != 0 {
		t.Errorf("expected exit 0 for logs test")
	}
	if code := cli.Execute([]string{"logs", "-s", "/nonexistent/system.yaml"}); code != 1 {
		t.Errorf("expected exit 1 for logs with nonexistent system.yaml")
	}
	if code := cli.Execute([]string{"logs", "tail", "-r", "invalid(?=regex)", "-s", sysFile}); code != 1 {
		t.Errorf("expected exit 1 for logs tail with invalid regex")
	}
}

func TestCLI_IsLoopbackRequest_EdgeCases(t *testing.T) {
	cases := []struct {
		addr     string
		expected bool
	}{
		{"@", true},
		{"/var/run/ubnt.sock", true},
		{"127.0.0.1:8000", true},
		{"127.0.0.1", true},
		{"[::1]:8000", true},
		{"::1", true},
		{"192.168.1.100:8000", false},
		{"10.0.0.1", false},
		{"invalid_address", false},
	}

	for _, tc := range cases {
		r, _ := http.NewRequest("GET", "http://localhost/probe", nil)
		r.RemoteAddr = tc.addr
		if got := cli.IsLoopbackRequest(r); got != tc.expected {
			t.Errorf("IsLoopbackRequest(%q) = %v, expected %v", tc.addr, got, tc.expected)
		}
	}
}

func TestCLI_CmdRunContext_ValidationFailure(t *testing.T) {
	var exitCode int
	logger.SetExitFunc(func(code int) { exitCode = code })
	defer logger.ResetExitFunc()

	badSys := &config.SystemConfig{Server: config.ServerConfig{Port: 99999}}
	cli.CmdRunContext(context.Background(), nil, "bad.yaml", "bad.yaml", badSys, &config.InventoryConfig{})
	if exitCode != 1 {
		t.Errorf("expected exitCode 1 on CmdRunContext validation failure, got %d", exitCode)
	}
}

func TestCLI_RunSuiteEndpoint_And_Connectivity_EdgeCases(t *testing.T) {
	tsErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsErr.Close()

	sysErrPort := &config.SystemConfig{
		Server: config.ServerConfig{
			Listen: strings.TrimPrefix(tsErr.URL, "http://"),
		},
	}
	code := cli.CmdTest(sysErrPort, &config.InventoryConfig{}, "endpoint", "")
	if code == 0 {
		t.Errorf("expected CmdTest endpoint to return 1 when server returns HTTP 500")
	}

	if c := cli.CmdTest(sysErrPort, &config.InventoryConfig{Devices: []config.DeviceConfig{}}, "connectivity", ""); c != 0 {
		t.Errorf("expected CmdTest connectivity with empty inventory to return 0, got %d", c)
	}
}

func TestCLI_MatchesMetricFilter(t *testing.T) {
	if !cli.MatchesMetricFilter(nil, "ubnt_test", "{name=\"AP\"}", "Test help") {
		t.Errorf("expected MatchesMetricFilter(nil) to return true")
	}
	reName := regexp.MustCompile("^ubnt_test")
	if !cli.MatchesMetricFilter(reName, "ubnt_test", "{}", "") {
		t.Errorf("expected match on metric name")
	}
	reLabel := regexp.MustCompile("eth0")
	if !cli.MatchesMetricFilter(reLabel, "other", "{interface=\"eth0\"}", "") {
		t.Errorf("expected match on labelBlock")
	}
	reHelp := regexp.MustCompile("Transmitted bytes")
	if !cli.MatchesMetricFilter(reHelp, "other", "{}", "Transmitted bytes counter") {
		t.Errorf("expected match on help string")
	}
	reNone := regexp.MustCompile("nomatch")
	if cli.MatchesMetricFilter(reNone, "other", "{interface=\"eth0\"}", "help text") {
		t.Errorf("expected no match")
	}
}

func TestCLI_LiveMockTLS_ConnectivityAndDiagInspect(t *testing.T) {
	tsTLS := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1.0/user/login":
			w.Header().Set("X-Auth-Token", "tls-token-123")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "/api/v1.0/statistics":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"device": {
					"identification": {"model": "NanoStation 5AC", "firmwareVersion": "8.7.1"},
					"cpu": [{"identifier": "MIPS 74Kc", "usage": 15.0}],
					"ram": {"usage": 45.0, "free": 64000000.0, "total": 128000000.0},
					"uptime": 72000.0
				},
				"interfaces": [
					{"id": "eth0", "enabled": true, "statistics": {"rxBytes": 2048.0, "txBytes": 1024.0}}
				],
				"wireless": {
					"serviceUptime": 71000.0,
					"radios": [{"linkState": "connected", "frequency": {"center": 5200.0, "control": 5200.0}, "noiseFloor": -95.0, "channelWidth": {"current": 40.0}}],
					"peers": []
				}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tsTLS.Close()

	tlsHostPort := strings.TrimPrefix(tsTLS.URL, "https://")

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	sysPath := filepath.Join(tmpDir, "system.yaml")

	cfgContent := `
devices:
  - name: "Mock-TLS-AP"
    ip: "127.0.0.1"
    ssl: true
    username: "ubnt"
    password: "secretpassword"
`
	sysContent := fmt.Sprintf(`
server:
  port: 8000
  listen: "%s"
performance:
  request_timeout: 5
  socket_timeout: 3
  login_timeout: 3
`, tlsHostPort)

	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0600)
	_ = os.WriteFile(sysPath, []byte(sysContent), 0600)

	sysConf, _ := config.LoadSystemConfig(sysPath)
	invConf, _ := config.LoadInventoryConfig(cfgPath)
	invConf.Devices[0].IP = tlsHostPort

	// 1. Run connectivity suite with TLS verification
	code := cli.CmdTest(sysConf, invConf, "connectivity", "Mock-TLS-AP")
	if code != 0 {
		t.Errorf("expected connectivity suite over TLS to succeed, got %d", code)
	}

	// 2. CmdInspect with filter and non-matching filter
	if c := cli.CmdInspect(sysConf, invConf, "Mock-TLS-AP", "cpu"); c != 0 {
		t.Errorf("expected inspect with cpu filter to succeed, got %d", c)
	}
	if c := cli.CmdInspect(sysConf, invConf, "Mock-TLS-AP", "nomatch_filter"); c != 0 {
		t.Errorf("expected inspect with nomatch to succeed, got %d", c)
	}

	// 3. CmdExport with filter
	if c := cli.CmdExport(sysConf, invConf, "Mock-TLS-AP", "ubnt_device_cpu"); c != 0 {
		t.Errorf("expected export with cpu filter to succeed, got %d", c)
	}

	// 4. CmdDiag
	if c := cli.CmdDiag(sysConf, invConf, "Mock-TLS-AP"); c != 0 {
		t.Errorf("expected diag to succeed, got %d", c)
	}
}

func TestCLI_CmdLogs_AllSubcommandsAndFileFallback(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "app.log")
	sysFile := filepath.Join(tmpDir, "system.yaml")

	logData := "2026-10-18 | INFO | System started\n2026-10-18 | ERROR | MST4 signal dropped\n2026-10-18 | INFO | Scrape complete\n"
	_ = os.WriteFile(logFile, []byte(logData), 0600)

	sysContent := fmt.Sprintf(`
server:
  port: 8000
  listen: "unix:///tmp/ubnt.sock"
logging:
  level: INFO
  console: true
  file:
    enabled: true
    path: %q
    max_size_mb: 5
    backup_count: 2
`, logFile)
	_ = os.WriteFile(sysFile, []byte(sysContent), 0600)
	sysConf, _ := config.LoadSystemConfig(sysFile)

	// Test subcommands
	if c := cli.CmdLogs(sysFile, sysConf, "", 5, ""); c != 0 {
		t.Errorf("expected CmdLogs empty sub to fallback to tail on file, got %d", c)
	}
	if c := cli.CmdLogs(sysFile, sysConf, "status", 5, ""); c != 0 {
		t.Errorf("expected CmdLogs status to succeed, got %d", c)
	}
	if c := cli.CmdLogs(sysFile, sysConf, "test", 5, ""); c != 0 {
		t.Errorf("expected CmdLogs test to succeed, got %d", c)
	}
	if c := cli.CmdLogs(sysFile, sysConf, "tail", 2, "ERROR"); c != 0 {
		t.Errorf("expected CmdLogs tail with regex on file to succeed, got %d", c)
	}
	if c := cli.CmdLogs(sysFile, sysConf, "MST4", 5, ""); c != 0 {
		t.Errorf("expected CmdLogs shorthand filter on file to succeed, got %d", c)
	}
	if c := cli.CmdLogs(sysFile, sysConf, "custom", 5, "INFO"); c != 0 {
		t.Errorf("expected CmdLogs custom sub with filter on file to succeed, got %d", c)
	}

	// Address resolution variations
	for _, listen := range []string{":8000", "0.0.0.0:8000", "[::]:8000", "192.168.1.100:8000", "invalid"} {
		sysConf.Server.Listen = listen
		_ = cli.CmdLogs(sysFile, sysConf, "tail", 1, "")
	}
}
