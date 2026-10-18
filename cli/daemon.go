package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/definitions"
	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/scraper"
)

// IsValidTraceID validates incoming trace identifiers to avoid header injection.
func IsValidTraceID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isValid := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.'
		if !isValid {
			return false
		}
	}
	return true
}

// IsLoopbackRequest verifies if an HTTP request originated from a local loopback IP or Unix socket.
func IsLoopbackRequest(r *http.Request) bool {
	if r.RemoteAddr == "@" || strings.HasPrefix(r.RemoteAddr, "/") {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// CmdRun starts the Prometheus metrics HTTP server and asynchronous device scrape loop.
func CmdRun(configPath, systemPath string, sysConf *config.SystemConfig, invConf *config.InventoryConfig) {
	CmdRunContext(context.Background(), nil, configPath, systemPath, sysConf, invConf)
}

// CmdRunContext starts the daemon with a cancellable context and optional trigger channel for configuration reloads.
func CmdRunContext(parentCtx context.Context, reloadTrigger <-chan struct{}, configPath, systemPath string, sysConf *config.SystemConfig, invConf *config.InventoryConfig) {
	_, _ = config.EnforceSystemDefaults(systemPath, sysConf)

	if valErrs := config.ValidateConfig(sysConf, invConf); len(valErrs) > 0 {
		for _, e := range valErrs {
			logger.Error("main", "Configuration validation error: %s", e)
		}
		logger.Fatal("main", "Configuration validation failed with %d error(s). Run 'ubnt-exporter check'", len(valErrs))
		return
	}

	var stateMu sync.RWMutex
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

	scrapers := scraper.BuildScrapers(invConf, sysConf, dynamicKeys)
	logger.Info("main", "Monitoring %d devices. Interval: %ds", len(scrapers), sysConf.Server.ScrapeInterval)

	client := scraper.NewHTTPClient(time.Duration(sysConf.Performance.SocketTimeout) * time.Second)

	promOpts := promhttp.HandlerOpts{
		EnableOpenMetrics:   true,
		Timeout:             10 * time.Second,
		MaxRequestsInFlight: 10,
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promOpts))
	mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopbackRequest(r) {
			http.Error(w, "forbidden: /logs is restricted to local CLI loopback access", http.StatusForbidden)
			return
		}
		limit := 50
		if lStr := r.URL.Query().Get("limit"); lStr != "" {
			if n, err := strconv.Atoi(lStr); err == nil && n > 0 {
				limit = n
			}
		}
		filter := r.URL.Query().Get("filter")
		lines := logger.GetRecentLogs(limit, filter)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// #nosec G705 -- serves plain text diagnostic logs to authorized local loopback with text/plain and nosniff headers
		for _, line := range lines {
			_, _ = fmt.Fprintln(w, line)
		}
	})
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		target := r.URL.Query().Get("target")
		if target == "" {
			http.Error(w, "'target' parameter is required (e.g. /probe?target=IP)", http.StatusBadRequest)
			return
		}

		traceID := r.Header.Get("X-Request-ID")
		if !IsValidTraceID(traceID) {
			traceID = fmt.Sprintf("probe-%x", time.Now().UnixNano()%0xFFFFFF)
		}
		probeCtx := logger.WithTraceID(r.Context(), traceID)
		w.Header().Set("X-Request-ID", traceID)

		stateMu.RLock()
		allowedTargets := sysConf.Server.ProbeAllowedTargets
		curScrapers := scrapers
		curClient := client
		curDynamicKeys := make([]string, len(dynamicKeys))
		copy(curDynamicKeys, dynamicKeys)
		stateMu.RUnlock()

		if len(allowedTargets) > 0 && !config.IsTargetAllowed(target, allowedTargets) {
			logger.WarnContext(probeCtx, "probe", "Forbidden probe attempt for target outside probe_allowed_targets: %s", target)
			http.Error(w, fmt.Sprintf("target '%s' is not permitted by probe_allowed_targets", target), http.StatusForbidden)
			return
		}

		var targetScraper *scraper.DeviceScraper
		for _, s := range curScrapers {
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
		probeStore.RegisterDynamicLabels(curDynamicKeys)
		targetScraper.Scrape(probeCtx, curClient, probeStore, true)

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
		if network == "unix" {
			_ = os.Chmod(bindAddr, 0600)
		}
		srv := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 16,
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

	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, stop := signal.NotifyContext(parentCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)
	defer signal.Stop(hupCh)

	var cycleSeq uint64
	executeCycle := func() {
		stateMu.RLock()
		activeScrapers := scrapers
		activeSem := sem
		activeClient := client
		stateMu.RUnlock()

		cycleID := fmt.Sprintf("cycle-%d", atomic.AddUint64(&cycleSeq, 1))
		cycleCtx := logger.WithTraceID(ctx, cycleID)
		cycleStart := time.Now()
		var wg sync.WaitGroup
		var onlineCount int64
		for _, sc := range activeScrapers {
			wg.Add(1)
			go func(s *scraper.DeviceScraper) {
				defer wg.Done()
				activeSem <- struct{}{}
				defer func() { <-activeSem }()
				if s.Scrape(cycleCtx, activeClient, store, false) {
					atomic.AddInt64(&onlineCount, 1)
				}
			}(sc)
		}
		wg.Wait()
		duration := time.Since(cycleStart)
		logger.InfoContext(cycleCtx, "main", "Scrape cycle finished in %.2fs (%d/%d devices online)", duration.Seconds(), onlineCount, len(activeScrapers))
		metrics.GetTelemetry().SetScrapeCycle(duration, len(activeScrapers), int(onlineCount))
	}

	executeCycle()

	ticker := time.NewTicker(scrapeInterval)
	defer ticker.Stop()

	performReload := func() {
		logger.Info("main", "SIGHUP received, checking for configuration reload...")
		newSys, sysErr := config.LoadSystemConfig(systemPath)
		if sysErr != nil {
			logger.Warn("main", "Failed to reload system config (%s) on SIGHUP: %v", systemPath, sysErr)
			return
		}
		newSys = config.ApplySystemDefaults(newSys)
		newInv, invErr := config.LoadInventoryConfig(configPath)
		if invErr != nil {
			logger.Warn("main", "Failed to reload inventory config (%s) on SIGHUP: %v", configPath, invErr)
			return
		}
		if valErrs := config.ValidateConfig(newSys, newInv); len(valErrs) > 0 {
			for _, ve := range valErrs {
				logger.Warn("main", "SIGHUP reload validation error: %s", ve)
			}
			logger.Warn("main", "Aborting SIGHUP reload due to %d validation error(s); keeping active configuration", len(valErrs))
			return
		}

		logger.Setup(newSys)

		newDynamicKeys := config.DiscoverCustomLabels(newInv)
		store.RegisterDynamicLabels(newDynamicKeys)
		newScrapers := scraper.BuildScrapers(newInv, newSys, newDynamicKeys)
		newWorkers := newSys.Performance.MaxWorkers
		if newWorkers <= 0 {
			newWorkers = 100
		}

		stateMu.Lock()
		if newSys.Performance.SocketTimeout != sysConf.Performance.SocketTimeout {
			client = scraper.NewHTTPClient(time.Duration(newSys.Performance.SocketTimeout) * time.Second)
		}
		if newWorkers != cap(sem) {
			sem = make(chan struct{}, newWorkers)
		}
		oldByIP := make(map[string]*scraper.DeviceScraper, len(scrapers))
		for _, oldSc := range scrapers {
			oldByIP[oldSc.IP] = oldSc
		}
		newIPSet := make(map[string]bool, len(newScrapers))
		for _, newSc := range newScrapers {
			newIPSet[newSc.IP] = true
			if prev, ok := oldByIP[newSc.IP]; ok {
				if prev.Name != newSc.Name {
					store.RemoveDeviceMetrics(prev.Name, prev.IP)
				}
				if prev.Username == newSc.Username && prev.Password == newSc.Password && prev.SSL == newSc.SSL {
					newSc.Token = prev.Token
				}
			}
		}
		for _, oldSc := range scrapers {
			if !newIPSet[oldSc.IP] {
				store.RemoveDeviceMetrics(oldSc.Name, oldSc.IP)
			}
		}
		sysConf = newSys
		dynamicKeys = newDynamicKeys
		scrapers = newScrapers
		stateMu.Unlock()

		newInterval := time.Duration(newSys.Server.ScrapeInterval) * time.Second
		if newInterval < 1*time.Second {
			newInterval = 30 * time.Second
		}
		if newInterval != scrapeInterval {
			scrapeInterval = newInterval
			ticker.Reset(scrapeInterval)
		}

		logger.Info("main", "Runtime log level updated on reload: %s", newSys.Logging.Level)
		logger.Info("main", "Configuration reloaded on SIGHUP: %d devices, interval %ds", len(newScrapers), newSys.Server.ScrapeInterval)
		executeCycle()
	}

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
			performReload()
		case <-reloadTrigger:
			performReload()
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
			return
		case <-ticker.C:
			executeCycle()
		}
	}
}
