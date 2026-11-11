package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/scraper"
)

const (
	// DefaultMaxConcurrentProbes bounds simultaneous /probe scrapes across all targets.
	DefaultMaxConcurrentProbes = 10
	// probeWriteMargin is added on top of the worst-case scrape time for the response write deadline.
	probeWriteMargin = 5 * time.Second
)

// ProbeState is a consistent snapshot of the daemon state needed to serve one /probe request.
type ProbeState struct {
	Scrapers       []*scraper.DeviceScraper
	Client         *http.Client
	AllowedTargets []string
	DynamicKeys    []string
}

// probeScrapeBudget returns the worst-case duration of a single device scrape:
// login + statistics, plus one re-authentication and retry after a 401.
func probeScrapeBudget(s *scraper.DeviceScraper) time.Duration {
	return 2 * (s.TimeoutLogin + s.TimeoutReq)
}

// NewProbeHandler builds the /probe multi-target handler. state is invoked once per request to
// obtain a consistent snapshot of the current configuration (it may change on SIGHUP reload).
// At most maxConcurrent probes run at once; excess requests get 503 with Retry-After instead of
// queueing. The response write deadline is extended per request to cover the worst-case device
// scrape, so slow devices are not cut off by the server-wide WriteTimeout.
func NewProbeHandler(state func() ProbeState, maxConcurrent int, promOpts promhttp.HandlerOpts) http.Handler {
	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrentProbes
	}
	sem := make(chan struct{}, maxConcurrent)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		st := state()

		if len(st.AllowedTargets) > 0 && !config.IsTargetAllowed(target, st.AllowedTargets) {
			logger.WarnContext(probeCtx, "probe", "Forbidden probe attempt for target outside probe_allowed_targets: %s", target)
			http.Error(w, fmt.Sprintf("target '%s' is not permitted by probe_allowed_targets", target), http.StatusForbidden)
			return
		}

		var targetScraper *scraper.DeviceScraper
		for _, s := range st.Scrapers {
			if s.IP == target || s.Name == target {
				targetScraper = s
				break
			}
		}
		if targetScraper == nil {
			http.Error(w, fmt.Sprintf("target '%s' not found in inventory", target), http.StatusNotFound)
			return
		}

		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			logger.WarnContext(probeCtx, "probe", "Rejecting probe for %s: %d concurrent probes already in flight", target, maxConcurrent)
			w.Header().Set("Retry-After", "1")
			http.Error(w, "too many concurrent probes, retry later", http.StatusServiceUnavailable)
			return
		}

		// The scraper mutex may be held by a running scrape cycle for the same device, so allow
		// for one full scrape of waiting plus our own scrape before the write deadline expires.
		// Scrape's own per-request timeouts bound the scrape itself; this context is a safety net.
		budget := 2 * probeScrapeBudget(targetScraper)
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(budget + probeWriteMargin)); err != nil {
			logger.DebugContext(probeCtx, "probe", "Could not extend write deadline: %v", err)
		}
		scrapeCtx, cancel := context.WithTimeout(probeCtx, budget)
		defer cancel()

		probeStore := metrics.NewStore()
		probeStore.RegisterDynamicLabels(st.DynamicKeys)
		targetScraper.Scrape(scrapeCtx, st.Client, probeStore, true)

		probeReg := prometheus.NewRegistry()
		probeReg.MustRegister(metrics.NewUbiquitiCollector(probeStore))
		probeReg.MustRegister(probeStore.ScrapeDuration)
		promhttp.HandlerFor(probeReg, promOpts).ServeHTTP(w, r)
	})
}
