// Package main is the entry point for the ubnt-exporter application.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v3"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/definitions"
	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/scraper"
	"ubnt-exporter/internal/utils"
)

const loggingHelpText = `DEBUG
  - Purpose: Full visibility. Traces HTTP requests, headers, and individual steps.
  - Use: Development, debugging connections, or empty metrics.
  - Example: "DEBUG | client | Starting new HTTP connection: X.X.X.X"

INFO (Default)
  - Purpose: Operational summary. Startup status, device count, and cycle times.
  - Use: Standard production monitoring.
  - Example: "INFO | main | Scrape cycle finished in 0.45s"

WARNING
  - Purpose: Recoverable anomalies. Auth failures, expired tokens.
  - Use: Detecting network instability or configuration changes.
  - Example: "WARN | DEVICE | Token expired. Re-authenticating..."

ERROR
  - Purpose: Critical failures. Timeouts, unreachable hosts, or syntax errors.
  - Use: Alerts for downed devices.
  - Example: "ERROR | DEVICE | Connection Error: No route to host"
`

func printHelp() {
	fmt.Printf("Ubiquiti Exporter v%s\n\n", definitions.VERSION)
	fmt.Println("USAGE:")
	fmt.Println("  ubnt-exporter [flags] <command>")
	fmt.Println("\nAVAILABLE COMMANDS:")
	fmt.Println("  run       Starts the metrics server (Daemon).")
	fmt.Println("  check     Validates configuration.")
	fmt.Println("  show      Dumps active configuration with masked secrets.")
	fmt.Println("  export    One-shot scrape for debugging and preview.")
	fmt.Println("  diag      Interactive terminal diagnostics and RF link quality table.")
	fmt.Println("\nFLAGS:")
	fmt.Println("  -c, --config string   Path to config.yaml (default \"config.yaml\")")
	fmt.Println("  -s, --system string   Path to system.yaml (default \"system.yaml\")")
	fmt.Println("  --cfg-dir string      Directory containing config.yaml and system.yaml")
	fmt.Println("  -t, --target string   Filter device by name or IP for export/diag")
	fmt.Println("  -v, --version         Show application version")
	fmt.Println("  -log, --log           Show logging manual and exit")
	fmt.Println("\nEXAMPLES:")
	fmt.Println("  $ ubnt-exporter run")
	fmt.Println("  $ ubnt-exporter check")
	fmt.Println("  $ ubnt-exporter diag")
	fmt.Println("  $ ubnt-exporter --cfg-dir /etc/ubnt-exporter run")
}

func main() {
	// Initialize default logger early to ensure all early startup messages follow formatting rules
	logger.Setup(config.DefaultSystemConfig())

	args := os.Args[1:]

	configPath := "config.yaml"
	systemPath := "system.yaml"
	var cfgDir string
	var targetFilter string
	var command string

	// Flexible argument parsing (handles flags before or after command)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-v", "--version":
			fmt.Printf("ubnt-exporter %s\n", definitions.VERSION)
			os.Exit(0)
		case "-log", "--log":
			fmt.Print(loggingHelpText)
			os.Exit(0)
		case "-h", "--help", "help":
			printHelp()
			os.Exit(0)
		case "-c", "--config":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}
		case "-s", "--system":
			if i+1 < len(args) {
				systemPath = args[i+1]
				i++
			}
		case "--cfg-dir":
			if i+1 < len(args) {
				cfgDir = args[i+1]
				i++
			}
		case "-t", "--target":
			if i+1 < len(args) {
				targetFilter = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(arg, "-c=") {
				configPath = strings.TrimPrefix(arg, "-c=")
			} else if strings.HasPrefix(arg, "--config=") {
				configPath = strings.TrimPrefix(arg, "--config=")
			} else if strings.HasPrefix(arg, "-s=") {
				systemPath = strings.TrimPrefix(arg, "-s=")
			} else if strings.HasPrefix(arg, "--system=") {
				systemPath = strings.TrimPrefix(arg, "--system=")
			} else if strings.HasPrefix(arg, "--cfg-dir=") {
				cfgDir = strings.TrimPrefix(arg, "--cfg-dir=")
			} else if strings.HasPrefix(arg, "-t=") {
				targetFilter = strings.TrimPrefix(arg, "-t=")
			} else if strings.HasPrefix(arg, "--target=") {
				targetFilter = strings.TrimPrefix(arg, "--target=")
			} else if !strings.HasPrefix(arg, "-") && command == "" {
				command = arg
			}
		}
	}

	if command == "" {
		printHelp()
		os.Exit(0)
	}

	configPath, systemPath = config.ResolveConfigPaths(cfgDir, configPath, systemPath)

	sysConf, sysErr := config.LoadSystemConfig(systemPath)
	if sysConf != nil {
		logger.Setup(sysConf)
	}

	invConf, invErr := config.LoadInventoryConfig(configPath)

	if command == "check" {
		cmdCheck(sysConf, sysErr, invConf, invErr)
		return
	}

	if sysErr != nil || invErr != nil {
		if sysErr != nil {
			logger.Error("main", "Failed to load system configuration (%s): %v", systemPath, sysErr)
		}
		if invErr != nil {
			logger.Error("main", "Failed to load inventory configuration (%s): %v", configPath, invErr)
		}
		logger.Fatal("main", "Could not load configuration files. Run 'ubnt-exporter check'")
	}

	switch command {
	case "run":
		cmdRun(systemPath, sysConf, invConf)
	case "show":
		cmdShow(sysConf, invConf)
	case "export":
		cmdExport(sysConf, invConf, targetFilter)
	case "diag":
		cmdDiag(sysConf, invConf, targetFilter)
	default:
		logger.Error("main", "Unknown command: %s", command)
		printHelp()
		os.Exit(1)
	}
}

func cmdCheck(sysConf *config.SystemConfig, sysErr error, invConf *config.InventoryConfig, invErr error) {
	fmt.Println("\n CONFIGURATION VALIDATION")
	fmt.Println("==============================")

	var allErrs []string
	if sysErr != nil {
		allErrs = append(allErrs, fmt.Sprintf("system.yaml error: %v", sysErr))
	}
	if invErr != nil {
		allErrs = append(allErrs, fmt.Sprintf("config.yaml error: %v", invErr))
	}

	if sysConf != nil && invConf != nil {
		valErrs := config.ValidateConfig(sysConf, invConf)
		allErrs = append(allErrs, valErrs...)
	}

	if len(allErrs) > 0 {
		fmt.Println("RESULT: Configuration has ERRORS")
		for _, e := range allErrs {
			fmt.Printf(" - %s\n", e)
		}
		os.Exit(1)
	}

	dynamicKeys := config.DiscoverCustomLabels(invConf)
	fmt.Println("RESULT: Configuration is VALID")
	fmt.Println("\n System Settings Verified:")
	listenAddrs, _ := config.ParseListenAddresses(sysConf.Server.Listen, sysConf.Server.Port)
	fmt.Printf("    Server Listen Targets: %v\n", listenAddrs)
	fmt.Printf("    Scrape Interval: %ds\n", sysConf.Server.ScrapeInterval)
	fmt.Printf("    Max Workers: %d\n", sysConf.Performance.MaxWorkers)
	fmt.Printf("    Timeouts (Login/Req/Socket): %ds / %ds / %ds\n", sysConf.Performance.LoginTimeout, sysConf.Performance.RequestTimeout, sysConf.Performance.SocketTimeout)
	fmt.Printf("    Failure Backoff: %ds\n", sysConf.Performance.FailureBackoff)
	fmt.Printf("    Log Level: %s | Syslog Facility: %s\n", sysConf.Logging.Level, sysConf.Logging.SyslogFacility)
	fmt.Printf("    Devices in Inventory: %d\n", len(invConf.Devices))
	fmt.Printf("    Dynamic Labels Found: %v\n", dynamicKeys)
	os.Exit(0)
}

func cmdShow(sysConf *config.SystemConfig, invConf *config.InventoryConfig) {
	fmt.Println("ACTIVE CONFIGURATION")

	sysCopy := *sysConf
	invCopy := *invConf

	// Sanitize passwords
	if invCopy.Defaults.Password != "" {
		invCopy.Defaults.Password = "******"
	}

	sanitizedDevices := make([]config.DeviceConfig, len(invConf.Devices))
	for i, d := range invConf.Devices {
		sanitized := d
		if sanitized.Password != "" {
			sanitized.Password = "******"
		}
		sanitizedDevices[i] = sanitized
	}
	invCopy.Devices = sanitizedDevices

	sysBytes, _ := yaml.Marshal(sysCopy)
	invBytes, _ := yaml.Marshal(invCopy)

	fmt.Print(string(sysBytes))
	fmt.Print(string(invBytes))
}

func cmdExport(sysConf *config.SystemConfig, invConf *config.InventoryConfig, targetFilter string) {
	fmt.Println("\n ONE-SHOT EXPORT (Dry Run)")

	store := metrics.GetStore()
	dynamicKeys := config.DiscoverCustomLabels(invConf)
	store.RegisterDynamicLabels(dynamicKeys)

	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics.NewUbiquitiCollector(store))
	reg.MustRegister(store.ScrapeDuration)
	reg.MustRegister(metrics.NewBuildInfoGauge())
	metrics.GetTelemetry().Register(reg)

	scrapers := buildScrapers(invConf, sysConf, dynamicKeys)
	var filtered []*scraper.DeviceScraper
	for _, sc := range scrapers {
		if targetFilter == "" || strings.EqualFold(sc.Name, targetFilter) || sc.IP == targetFilter {
			filtered = append(filtered, sc)
		}
	}

	if len(filtered) == 0 {
		fmt.Printf("No matching devices found in inventory for target %q.\n", targetFilter)
		return
	}

	client := scraper.NewHTTPClient(time.Duration(sysConf.Performance.SocketTimeout) * time.Second)

	exportCtx := logger.WithTraceID(context.Background(), "export")
	var wg sync.WaitGroup
	for _, sc := range filtered {
		wg.Add(1)
		go func(s *scraper.DeviceScraper) {
			defer wg.Done()
			s.Scrape(exportCtx, client, store, true)
		}(sc)
	}
	wg.Wait()

	fmt.Println("\n--- PROMETHEUS METRICS PREVIEW ---")
	mfs, err := reg.Gather()
	if err != nil {
		fmt.Printf("Error gathering metrics: %v\n", err)
		return
	}

	for _, mf := range mfs {
		fmt.Printf("# HELP %s %s\n", *mf.Name, *mf.Help)
		fmt.Printf("# TYPE %s %s\n", *mf.Name, strings.ToLower(mf.Type.String()))
		for _, m := range mf.Metric {
			var labelStrs []string
			for _, lp := range m.Label {
				labelStrs = append(labelStrs, fmt.Sprintf("%s=\"%s\"", *lp.Name, *lp.Value))
			}
			labelBlock := ""
			if len(labelStrs) > 0 {
				labelBlock = "{" + strings.Join(labelStrs, ",") + "}"
			}
			if m.Gauge != nil {
				fmt.Printf("%s%s %v\n", *mf.Name, labelBlock, *m.Gauge.Value)
			} else if m.Counter != nil {
				fmt.Printf("%s%s %v\n", *mf.Name, labelBlock, *m.Counter.Value)
			} else if m.Histogram != nil {
				for _, b := range m.Histogram.Bucket {
					bLabels := append([]string{}, labelStrs...)
					bLabels = append(bLabels, fmt.Sprintf("le=\"%v\"", *b.UpperBound))
					fmt.Printf("%s_bucket{%s} %v\n", *mf.Name, strings.Join(bLabels, ","), *b.CumulativeCount)
				}
				infLabels := append([]string{}, labelStrs...)
				infLabels = append(infLabels, "le=\"+Inf\"")
				fmt.Printf("%s_bucket{%s} %v\n", *mf.Name, strings.Join(infLabels, ","), *m.Histogram.SampleCount)
				fmt.Printf("%s_sum%s %v\n", *mf.Name, labelBlock, *m.Histogram.SampleSum)
				fmt.Printf("%s_count%s %v\n", *mf.Name, labelBlock, *m.Histogram.SampleCount)
			}
		}
	}
}

// BuildScrapers creates scrapers for all configured inventory devices.
func BuildScrapers(inv *config.InventoryConfig, sys *config.SystemConfig, dynamicKeys []string) []*scraper.DeviceScraper {
	return scraper.BuildScrapers(inv, sys, dynamicKeys)
}

func buildScrapers(inv *config.InventoryConfig, sys *config.SystemConfig, dynamicKeys []string) []*scraper.DeviceScraper {
	return scraper.BuildScrapers(inv, sys, dynamicKeys)
}

func isValidTraceID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

func cmdRun(systemPath string, sysConf *config.SystemConfig, invConf *config.InventoryConfig) {
	_, _ = config.EnforceSystemDefaults(systemPath, sysConf)

	if valErrs := config.ValidateConfig(sysConf, invConf); len(valErrs) > 0 {
		for _, e := range valErrs {
			logger.Error("main", "Configuration validation error: %s", e)
		}
		logger.Fatal("main", "Configuration validation failed with %d error(s). Run 'ubnt-exporter check'", len(valErrs))
	}

	dynamicKeys := config.DiscoverCustomLabels(invConf)
	store := metrics.GetStore()
	store.RegisterDynamicLabels(dynamicKeys)

	logger.Info("main", "UBNT Exporter v%s Started", definitions.VERSION)
	logger.Info("main", "Detected Custom Labels: %v", dynamicKeys)

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	reg.MustRegister(metrics.NewUbiquitiCollector(store))
	reg.MustRegister(store.ScrapeDuration)
	reg.MustRegister(metrics.NewBuildInfoGauge())
	metrics.GetTelemetry().Register(reg)

	scrapers := buildScrapers(invConf, sysConf, dynamicKeys)
	logger.Info("main", "Monitoring %d devices. Interval: %ds", len(scrapers), sysConf.Server.ScrapeInterval)

	client := scraper.NewHTTPClient(time.Duration(sysConf.Performance.SocketTimeout) * time.Second)

	promOpts := promhttp.HandlerOpts{
		EnableOpenMetrics:   true,
		Timeout:             10 * time.Second,
		MaxRequestsInFlight: 10,
	}

	// Setup HTTP routing
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promOpts))
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		if target == "" {
			http.Error(w, "'target' parameter is required (e.g. /probe?target=IP)", http.StatusBadRequest)
			return
		}

		traceID := r.Header.Get("X-Request-ID")
		if !isValidTraceID(traceID) {
			traceID = fmt.Sprintf("probe-%x", time.Now().UnixNano()%0xFFFFFF)
		}
		probeCtx := logger.WithTraceID(r.Context(), traceID)
		w.Header().Set("X-Request-ID", traceID)

		if len(sysConf.Server.ProbeAllowedTargets) > 0 && !config.IsTargetAllowed(target, sysConf.Server.ProbeAllowedTargets) {
			logger.WarnContext(probeCtx, "probe", "Forbidden probe attempt for target outside probe_allowed_targets: %s", target)
			http.Error(w, fmt.Sprintf("target '%s' is not permitted by probe_allowed_targets", target), http.StatusForbidden)
			return
		}

		var targetScraper *scraper.DeviceScraper
		for _, s := range scrapers {
			if s.IP == target || s.Name == target {
				targetScraper = s
				break
			}
		}

		if targetScraper == nil {
			http.Error(w, fmt.Sprintf("target '%s' not found in inventory", target), http.StatusNotFound)
			return
		}

		probeStore := metrics.NewStore()
		probeStore.RegisterDynamicLabels(dynamicKeys)
		targetScraper.Scrape(probeCtx, client, probeStore, true)

		probeReg := prometheus.NewRegistry()
		probeReg.MustRegister(metrics.NewUbiquitiCollector(probeStore))
		probeReg.MustRegister(probeStore.ScrapeDuration)
		promhttp.HandlerFor(probeReg, promOpts).ServeHTTP(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>UBNT Exporter</title></head><body><h1>Ubiquiti Exporter (Go)</h1><p><a href="/metrics">Metrics</a></p></body></html>`))
	})

	listenAddrs, err := config.ParseListenAddresses(sysConf.Server.Listen, sysConf.Server.Port)
	if err != nil {
		logger.Fatal("main", "Invalid server listen configuration: %v", err)
	}

	var servers []*http.Server
	var unixSockets []string
	fatalErrCh := make(chan error, len(listenAddrs))

	for _, addr := range listenAddrs {
		network := "tcp"
		bindAddr := addr
		if strings.HasPrefix(addr, "unix://") {
			network = "unix"
			bindAddr = strings.TrimPrefix(addr, "unix://")
			_ = os.Remove(bindAddr)
			unixSockets = append(unixSockets, bindAddr)
		}

		ln, listenErr := net.Listen(network, bindAddr)
		if listenErr != nil {
			logger.Fatal("main", "Failed to bind listener on %s (%s): %v", bindAddr, network, listenErr)
		}
		srv := &http.Server{
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		servers = append(servers, srv)
		logger.Info("main", "Serving metrics on %s (%s)", bindAddr, network)
		go func(s *http.Server, l net.Listener, a, netType string) {
			if sErr := s.Serve(l); sErr != nil && sErr != http.ErrServerClosed {
				logger.Error("main", "HTTP server on %s (%s) terminated: %v", a, netType, sErr)
				fatalErrCh <- fmt.Errorf("server on %s (%s): %w", a, netType, sErr)
			}
		}(srv, ln, bindAddr, network)
	}

	workers := sysConf.Performance.MaxWorkers
	if workers <= 0 {
		workers = 100
	}
	sem := make(chan struct{}, workers)

	scrapeInterval := time.Duration(sysConf.Server.ScrapeInterval) * time.Second
	if scrapeInterval < 1*time.Second {
		scrapeInterval = 30 * time.Second
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)
	defer signal.Stop(hupCh)

	var cycleSeq uint64
	executeCycle := func() {
		cycleID := fmt.Sprintf("cycle-%d", atomic.AddUint64(&cycleSeq, 1))
		cycleCtx := logger.WithTraceID(ctx, cycleID)
		cycleStart := time.Now()
		var wg sync.WaitGroup
		var onlineCount int64
		for _, sc := range scrapers {
			wg.Add(1)
			go func(s *scraper.DeviceScraper) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if s.Scrape(cycleCtx, client, store, false) {
					atomic.AddInt64(&onlineCount, 1)
				}
			}(sc)
		}
		wg.Wait()
		duration := time.Since(cycleStart)
		logger.InfoContext(cycleCtx, "main", "Scrape cycle finished in %.2fs (%d/%d devices online)", duration.Seconds(), onlineCount, len(scrapers))
		metrics.GetTelemetry().SetScrapeCycle(duration, len(scrapers), int(onlineCount))
	}

	// Initial scrape cycle on daemon startup
	executeCycle()

	ticker := time.NewTicker(scrapeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("main", "Shutdown signal received, shutting down gracefully...")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			for _, s := range servers {
				_ = s.Shutdown(shutdownCtx)
			}
			for _, sock := range unixSockets {
				_ = os.Remove(sock)
			}
			cancel()
			logger.Close()
			return
		case <-hupCh:
			logger.Info("main", "SIGHUP received, checking for configuration reload...")
			newSys, loadErr := config.LoadSystemConfig(systemPath)
			if loadErr != nil {
				logger.Warn("main", "Failed to reload system config on SIGHUP: %v", loadErr)
			} else {
				logger.SetLevel(newSys.Logging.Level)
				logger.Info("main", "Runtime log level updated on reload: %s", newSys.Logging.Level)
			}
		case sErr := <-fatalErrCh:
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			for _, s := range servers {
				_ = s.Shutdown(shutdownCtx)
			}
			for _, sock := range unixSockets {
				_ = os.Remove(sock)
			}
			cancel()
			logger.Fatal("main", "Fatal server listener failure: %v", sErr)
		case <-ticker.C:
			executeCycle()
		}
	}
}

type diagDeviceResult struct {
	Name        string
	IP          string
	Online      bool
	Signal      string
	Noise       string
	ChainDelta  string
	DegradedImb bool
	Clients     string
	Capacity    string
}

func cmdDiag(sysConf *config.SystemConfig, invConf *config.InventoryConfig, targetFilter string) {
	fmt.Println("\n LIVE WIRELESS DIAGNOSTICS")
	fmt.Println("==========================================================================================")

	store := metrics.GetStore()
	dynamicKeys := config.DiscoverCustomLabels(invConf)
	store.RegisterDynamicLabels(dynamicKeys)

	scrapers := buildScrapers(invConf, sysConf, dynamicKeys)
	client := scraper.NewHTTPClient(time.Duration(sysConf.Performance.SocketTimeout) * time.Second)

	var filtered []*scraper.DeviceScraper
	for _, sc := range scrapers {
		if targetFilter == "" || strings.EqualFold(sc.Name, targetFilter) || sc.IP == targetFilter {
			filtered = append(filtered, sc)
		}
	}

	if len(filtered) == 0 {
		fmt.Printf("No matching devices found in inventory for target %q.\n", targetFilter)
		return
	}

	diagCtx := logger.WithTraceID(context.Background(), "diag")
	var wg sync.WaitGroup
	for _, sc := range filtered {
		wg.Add(1)
		go func(s *scraper.DeviceScraper) {
			defer wg.Done()
			s.Scrape(diagCtx, client, store, true)
		}(sc)
	}
	wg.Wait()

	results := make([]diagDeviceResult, len(filtered))
	for idx, s := range filtered {
		res := diagDeviceResult{
			Name:       s.Name,
			IP:         s.IP,
			Signal:     "-",
			Noise:      "-",
			ChainDelta: "-",
			Clients:    "-",
			Capacity:   "-",
		}

		if val, ok := store.GetDeviceMetric(s.Name, s.IP, "scrape_success"); ok && val == 1.0 {
			res.Online = true

			if sig, ok := store.GetDeviceMetric(s.Name, s.IP, "ap_signal"); ok {
				res.Signal = fmt.Sprintf("%.0f dBm", sig)
			} else if sig, ok := store.GetDeviceMetric(s.Name, s.IP, "peer_signal"); ok {
				res.Signal = fmt.Sprintf("%.0f dBm", sig)
			}

			if noise, ok := store.GetDeviceMetric(s.Name, s.IP, "ap_noise"); ok {
				res.Noise = fmt.Sprintf("%.0f dBm", noise)
			} else if noise, ok := store.GetDeviceMetric(s.Name, s.IP, "peer_noise_floor"); ok {
				res.Noise = fmt.Sprintf("%.0f dBm", noise)
			}

			if imb, ok := store.GetDeviceMetric(s.Name, s.IP, "peer_chain_imbalance"); ok {
				if imb > 5.0 {
					res.ChainDelta = fmt.Sprintf("%.1f dB (!)", imb)
					res.DegradedImb = true
				} else {
					res.ChainDelta = fmt.Sprintf("%.1f dB", imb)
				}
			}

			if c, ok := store.GetDeviceMetric(s.Name, s.IP, "ap_clients"); ok {
				res.Clients = fmt.Sprintf("%.0f", c)
			}

			if capDL, ok := store.GetDeviceMetric(s.Name, s.IP, "peer_capacity_dl"); ok && capDL > 0 {
				res.Capacity = fmt.Sprintf("%.0f Mbps", capDL)
			} else if txRate, ok := store.GetDeviceMetric(s.Name, s.IP, "peer_tx_rate"); ok && txRate > 0 {
				res.Capacity = fmt.Sprintf("%.0f Mbps", txRate)
			}
		}

		results[idx] = res
	}

	// Print ASCII Table
	fmt.Println("+----------------------+---------------+------------+-----------+-----------+---------------+----------+----------+")
	fmt.Println("| Access Point         | Target IP     | Status     | Signal    | Noise     | Chain Delta   | Clients  | Capacity |")
	fmt.Println("+======================+===============+============+===========+===========+===============+==========+==========+")

	onlineCount := 0
	degradedCount := 0

	for _, r := range results {
		statusStr := "Offline"
		if r.Online {
			statusStr = "Online"
			onlineCount++
		}
		if r.DegradedImb {
			degradedCount++
		}

		fmt.Printf("| %-20s | %-13s | %-10s | %-9s | %-9s | %-13s | %-8s | %-8s |\n",
			truncate(r.Name, 20),
			truncate(r.IP, 13),
			statusStr,
			r.Signal,
			r.Noise,
			r.ChainDelta,
			r.Clients,
			r.Capacity,
		)
	}

	fmt.Println("+----------------------+---------------+------------+-----------+-----------+---------------+----------+----------+")
	fmt.Printf("Audited devices: %d | Online: %d | Degraded chain delta (>5 dB): %d\n\n", len(results), onlineCount, degradedCount)
}

// Truncate truncates a string to maxLen and appends ellipsis if truncated.
func Truncate(s string, maxLen int) string {
	return utils.Truncate(s, maxLen)
}

func truncate(s string, maxLen int) string {
	return utils.Truncate(s, maxLen)
}
