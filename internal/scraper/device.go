// Package scraper implements asynchronous HTTP device scraping.
package scraper

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"ubnt-exporter/internal/collector"
	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/utils"
)

// DeviceScraper represents a scraper instance for an individual Ubiquiti device.
type DeviceScraper struct {
	mu                sync.Mutex
	Name              string
	IP                string
	Username          string
	Password          string
	SSL               bool
	BaseURL           string
	CustomLabels      map[string]string
	DynamicKeys       []string
	EnabledModules    map[string]bool
	TimeoutLogin      time.Duration
	TimeoutReq        time.Duration
	TimeoutSocket     time.Duration
	BackoffTime       time.Duration
	NextAllowedScrape time.Time
	Token             string
}

// NewDeviceScraper constructs a new DeviceScraper with inherited defaults and overrides.
func NewDeviceScraper(
	dev config.DeviceConfig,
	defaults config.DefaultsConfig,
	sys *config.SystemConfig,
	globalEnabled map[string]bool,
	dynamicKeys []string,
) *DeviceScraper {
	username := dev.Username
	if username == "" {
		username = defaults.Username
	}
	if username == "" {
		username = os.Getenv("UBNT_USERNAME")
	}

	password := dev.Password
	if password == "" {
		password = defaults.Password
	}
	if password == "" {
		password = os.Getenv("UBNT_PASSWORD")
	}

	name := dev.Name
	if name == "" {
		name = dev.IP
	}

	useSSL := defaults.SSL
	if dev.SSL != nil {
		useSSL = *dev.SSL
	}

	protocol := "http"
	if useSSL {
		protocol = "https"
	}
	baseURL := fmt.Sprintf("%s://%s/api/v1.0", protocol, dev.IP)

	// Inherit default labels and merge with device-level overrides
	mergedLabels := make(map[string]string, len(defaults.Labels)+len(dev.Labels))
	for k, v := range defaults.Labels {
		mergedLabels[k] = v
	}
	for k, v := range dev.Labels {
		mergedLabels[k] = v
	}

	// Merge global collectors with device-specific collector overrides
	effectiveModules := make(map[string]bool, len(globalEnabled)+len(dev.EnabledCollectors))
	for k, v := range globalEnabled {
		effectiveModules[k] = v
	}
	for k, v := range dev.EnabledCollectors {
		effectiveModules[k] = v
	}

	loginTimeout := 5
	reqTimeout := 15
	socketTimeout := 2
	failureBackoff := 120

	if sys != nil {
		if sys.Performance.LoginTimeout > 0 {
			loginTimeout = sys.Performance.LoginTimeout
		}
		if sys.Performance.RequestTimeout > 0 {
			reqTimeout = sys.Performance.RequestTimeout
		}
		if sys.Performance.SocketTimeout > 0 {
			socketTimeout = sys.Performance.SocketTimeout
		}
		if sys.Performance.FailureBackoff >= 0 {
			failureBackoff = sys.Performance.FailureBackoff
		}
	}

	return &DeviceScraper{
		Name:           name,
		IP:             dev.IP,
		Username:       username,
		Password:       password,
		SSL:            useSSL,
		BaseURL:        baseURL,
		CustomLabels:   mergedLabels,
		DynamicKeys:    dynamicKeys,
		EnabledModules: effectiveModules,
		TimeoutLogin:   time.Duration(loginTimeout) * time.Second,
		TimeoutReq:     time.Duration(reqTimeout) * time.Second,
		TimeoutSocket:  time.Duration(socketTimeout) * time.Second,
		BackoffTime:    time.Duration(failureBackoff) * time.Second,
	}
}

// NewHTTPClient creates an HTTP client configured with high-capacity connection pooling for large inventories.
func NewHTTPClient(socketTimeout time.Duration) *http.Client {
	if socketTimeout <= 0 {
		socketTimeout = 2 * time.Second
	}
	transport := &http.Transport{
		// #nosec G402 -- Ubiquiti AirOS v6/v8 and AirFiber devices use embedded self-signed certificates and legacy TLS 1.0+
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS10,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			},
		},
		DialContext: (&net.Dialer{
			Timeout: socketTimeout,
		}).DialContext,
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (d *DeviceScraper) login(ctx context.Context, client *http.Client, store *metrics.Store) (bool, error) {
	loginURL := fmt.Sprintf("%s/user/login", d.BaseURL)
	payload := map[string]string{
		"username": d.Username,
		"password": d.Password,
	}
	jsonBody, _ := json.Marshal(payload)

	reqCtx, cancel := context.WithTimeout(ctx, d.TimeoutLogin)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", loginURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	logger.DebugContext(ctx, d.Name, "Authenticating via POST %s", loginURL)
	resp, err := client.Do(req)
	if err != nil {
		logger.ErrorContext(ctx, d.Name, "Login Unreachable: %v", err)
		store.IncrementMetric("scrape_error", map[string]string{
			"name": d.Name, "ip": d.IP, "reason": "connection_error",
		}, 1.0)
		metrics.GetTelemetry().RecordScrapeResult("conn_error")
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		token := resp.Header.Get("X-Auth-Token")
		if token == "" {
			token = resp.Header.Get("x-auth-token")
		}
		d.Token = token
		logger.DebugContext(ctx, d.Name, "Login OK (HTTP %d, token acquired)", resp.StatusCode)
		return true, nil
	}

	logger.ErrorContext(ctx, d.Name, "Login Failed (Status: %d)", resp.StatusCode)
	store.IncrementMetric("scrape_error", map[string]string{
		"name": d.Name, "ip": d.IP, "reason": "auth_failed",
	}, 1.0)
	metrics.GetTelemetry().RecordScrapeResult("auth_error")
	return false, fmt.Errorf("login failed status %d", resp.StatusCode)
}

// Scrape performs data collection from the Ubiquiti device.
func (d *DeviceScraper) Scrape(ctx context.Context, client *http.Client, store *metrics.Store, dryRun bool) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	metaLabels := map[string]string{"name": d.Name, "ip": d.IP}

	if !dryRun && !d.NextAllowedScrape.IsZero() && time.Now().Before(d.NextAllowedScrape) {
		store.UpdateMetric("scrape_success", metaLabels, 0.0)
		return false
	}

	startTime := time.Now()
	success := false

	defer func() {
		if success {
			duration := time.Since(startTime).Seconds()
			store.ScrapeDuration.WithLabelValues(d.Name, d.IP).Observe(duration)
			store.UpdateMetric("scrape_success", metaLabels, 1.0)
		} else {
			if !dryRun {
				// Purge stale telemetry metrics on failure (preserves scrape_error)
				store.RemoveDeviceMetrics(d.Name, d.IP)
			}
			store.UpdateMetric("scrape_success", metaLabels, 0.0)
		}
	}()

	if d.Username == "" || d.Password == "" {
		logger.WarnContext(ctx, d.Name, "Missing credentials")
		metrics.GetTelemetry().RecordScrapeResult("auth_error")
		return false
	}

	if d.Token == "" {
		ok, _ := d.login(ctx, client, store)
		if !ok {
			d.NextAllowedScrape = time.Now().Add(d.BackoffTime)
			return false
		}
	}

	if dryRun {
		logger.InfoContext(ctx, d.Name, "Connecting")
	}

	statsURL := fmt.Sprintf("%s/statistics", d.BaseURL)
	body, statusCode, err := d.fetchStatistics(ctx, client, statsURL)

	// Re-authenticate if token expired (401)
	if statusCode == http.StatusUnauthorized {
		logger.WarnContext(ctx, d.Name, "Token expired. Re-authenticating")
		ok, _ := d.login(ctx, client, store)
		if ok {
			body, statusCode, err = d.fetchStatistics(ctx, client, statsURL)
		} else {
			d.NextAllowedScrape = time.Now().Add(d.BackoffTime)
			return false
		}
	}

	if err != nil {
		d.NextAllowedScrape = time.Now().Add(d.BackoffTime)
		if errors.Is(err, context.DeadlineExceeded) {
			logger.ErrorContext(ctx, d.Name, "Timeout. Backing off for %vs", d.BackoffTime.Seconds())
			store.IncrementMetric("scrape_error", map[string]string{
				"name": d.Name, "ip": d.IP, "reason": "timeout",
			}, 1.0)
			metrics.GetTelemetry().RecordScrapeResult("timeout")
		} else {
			logger.ErrorContext(ctx, d.Name, "Conn Error: %v. Backing off for %vs", err, d.BackoffTime.Seconds())
			store.IncrementMetric("scrape_error", map[string]string{
				"name": d.Name, "ip": d.IP, "reason": "connection_error",
			}, 1.0)
			metrics.GetTelemetry().RecordScrapeResult("conn_error")
			d.Token = ""
		}
		return false
	}

	if statusCode != http.StatusOK {
		logger.WarnContext(ctx, d.Name, "HTTP Stats Error: %d", statusCode)
		store.IncrementMetric("scrape_error", map[string]string{
			"name": d.Name, "ip": d.IP, "reason": fmt.Sprintf("http_%d", statusCode),
		}, 1.0)
		metrics.GetTelemetry().RecordScrapeResult("http_error")
		return false
	}

	data, err := decodeJSONSafe(body)
	if err != nil {
		logger.ErrorContext(ctx, d.Name, "JSON decode error: %v", err)
		store.IncrementMetric("scrape_error", map[string]string{
			"name": d.Name, "ip": d.IP, "reason": "decode_error",
		}, 1.0)
		metrics.GetTelemetry().RecordScrapeResult("decode_error")
		return false
	}

	d.processData(ctx, data, store, dryRun)
	success = true
	metrics.GetTelemetry().RecordScrapeResult("success")
	d.NextAllowedScrape = time.Time{}
	logger.ResetDampening(d.Name)
	return true
}

func (d *DeviceScraper) fetchStatistics(ctx context.Context, client *http.Client, url string) ([]byte, int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, d.TimeoutReq)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.Token != "" {
		req.Header.Set("X-Auth-Token", d.Token)
	}

	logger.DebugContext(ctx, d.Name, "Fetching statistics via GET %s", url)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	const maxResponseBodyBytes = 10 * 1024 * 1024 // 10 MiB safety limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	logger.DebugContext(ctx, d.Name, "GET %s returned HTTP %d (%d bytes)", url, resp.StatusCode, len(body))
	return body, resp.StatusCode, err
}

func decodeJSONSafe(content []byte) (any, error) {
	var result any
	err := json.Unmarshal(content, &result)
	return result, err
}

func (d *DeviceScraper) processData(ctx context.Context, data any, store *metrics.Store, dryRun bool) {
	var root map[string]any

	switch v := data.(type) {
	case []any:
		if len(v) > 0 {
			root, _ = v[0].(map[string]any)
		}
	case map[string]any:
		root = v
	}

	if root == nil {
		return
	}

	infoLabels := map[string]string{
		"name": d.Name,
		"ip":   d.IP,
	}
	for _, key := range d.DynamicKeys {
		infoLabels[key] = d.CustomLabels[key]
	}

	standardLabels := map[string]string{
		"name": d.Name,
		"ip":   d.IP,
	}

	family := utils.DetectDeviceFamily(root)

	buf := metrics.NewBuffer()
	buf.UpdateMetric("device_info", infoLabels, 1.0)

	if collector.IsCollectorEnabled(d.EnabledModules, "system") {
		collector.RunCollectorTimed("system", func() {
			collector.ProcessSystem(ctx, root, standardLabels, buf)
		})
	}

	if collector.IsCollectorEnabled(d.EnabledModules, "interfaces") {
		collector.RunCollectorTimed("interfaces", func() {
			collector.ProcessInterfaces(ctx, root, standardLabels, buf)
		})
	}

	if collector.IsCollectorEnabled(d.EnabledModules, "peers") {
		collector.RunCollectorTimed("peers", func() {
			collector.ProcessPeers(ctx, root, standardLabels, buf)
		})
	}

	switch family {
	case "airfiber":
		if collector.IsCollectorEnabled(d.EnabledModules, "airfiber") {
			collector.ProcessAF5XHD(ctx, root, standardLabels, d.EnabledModules, buf)
		} else {
			logger.DebugContext(ctx, d.Name, "AirFiber detected but module disabled")
		}
	default:
		if collector.IsCollectorEnabled(d.EnabledModules, "wireless") {
			collector.ProcessWireless(ctx, root, standardLabels, d.EnabledModules, buf)
		} else {
			logger.DebugContext(ctx, d.Name, "AirMax detected but wireless module disabled")
		}
	}

	totalMetrics := buf.Len()
	buf.Commit(store, d.Name, d.IP)

	if dryRun {
		logger.InfoContext(ctx, d.Name, "Data processed OK (%s). Labels: %v", family, infoLabels)
	} else {
		logger.DebugContext(ctx, d.Name, "Collection OK (%s): %d metrics committed", family, totalMetrics)
	}
}

// BuildScrapers constructs scrapers for all configured inventory devices.
func BuildScrapers(inv *config.InventoryConfig, sys *config.SystemConfig, dynamicKeys []string) []*DeviceScraper {
	if inv == nil {
		return nil
	}
	scrapers := make([]*DeviceScraper, 0, len(inv.Devices))
	for _, dev := range inv.Devices {
		sc := NewDeviceScraper(dev, inv.Defaults, sys, inv.EnabledCollectors, dynamicKeys)
		scrapers = append(scrapers, sc)
	}
	return scrapers
}
