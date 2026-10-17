package scraper_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/scraper"
)

func TestNewDeviceScraper(t *testing.T) {
	sslTrue := true
	dev := config.DeviceConfig{
		Name:     "AP-01",
		IP:       "10.0.0.1",
		Username: "custom_user",
		Password: "custom_password",
		SSL:      &sslTrue,
		Labels: map[string]string{
			"site": "alpha",
		},
		EnabledCollectors: map[string]bool{
			"peers": false,
		},
	}

	defaults := config.DefaultsConfig{
		Username: "default_user",
		Password: "default_password",
		SSL:      false,
		Labels: map[string]string{
			"region": "us-east",
		},
	}

	sys := config.DefaultSystemConfig()
	globalCollectors := map[string]bool{
		"wireless": true,
		"peers":    true,
	}

	sc := scraper.NewDeviceScraper(dev, defaults, sys, globalCollectors, []string{"region", "site"})

	if sc.Name != "AP-01" {
		t.Errorf("expected sc.Name 'AP-01', got %s", sc.Name)
	}
	if sc.IP != "10.0.0.1" {
		t.Errorf("expected sc.IP '10.0.0.1', got %s", sc.IP)
	}
	if sc.Username != "custom_user" {
		t.Errorf("expected overridden username 'custom_user', got %s", sc.Username)
	}
	if sc.Password != "custom_password" {
		t.Errorf("expected overridden password 'custom_password', got %s", sc.Password)
	}
	if !sc.SSL {
		t.Errorf("expected sc.SSL true")
	}
	if sc.BaseURL != "https://10.0.0.1/api/v1.0" {
		t.Errorf("expected baseURL 'https://10.0.0.1/api/v1.0', got %s", sc.BaseURL)
	}
	if sc.CustomLabels["region"] != "us-east" || sc.CustomLabels["site"] != "alpha" {
		t.Errorf("labels were not properly merged: %v", sc.CustomLabels)
	}
	if sc.EnabledModules["peers"] != false {
		t.Errorf("expected peers collector to be overridden to false")
	}
}

func TestBuildScrapers(t *testing.T) {
	inv := &config.InventoryConfig{
		Defaults: config.DefaultsConfig{
			Username: "admin",
			Password: "password",
		},
		Devices: []config.DeviceConfig{
			{Name: "AP1", IP: "10.0.0.1"},
			{Name: "AP2", IP: "10.0.0.2"},
		},
		EnabledCollectors: map[string]bool{
			"wireless": true,
		},
	}
	sys := config.DefaultSystemConfig()

	scrapers := scraper.BuildScrapers(inv, sys, []string{"region"})
	if len(scrapers) != 2 {
		t.Fatalf("expected 2 scrapers, got %d", len(scrapers))
	}
	if scrapers[0].Name != "AP1" || scrapers[0].IP != "10.0.0.1" {
		t.Errorf("unexpected scraper 0: %+v", scrapers[0])
	}
	if scrapers[1].Name != "AP2" || scrapers[1].IP != "10.0.0.2" {
		t.Errorf("unexpected scraper 1: %+v", scrapers[1])
	}
}

func TestNewHTTPClient(t *testing.T) {
	client := scraper.NewHTTPClient(3 * time.Second)
	if client == nil {
		t.Fatalf("expected client to not be nil")
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http.Transport")
	}
	if transport.MaxIdleConns != 1000 {
		t.Errorf("expected MaxIdleConns 1000, got %d", transport.MaxIdleConns)
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify true")
	}
}

func TestDeviceScraper_Scrape_MockServer(t *testing.T) {
	// Create mock AirOS server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1.0/user/login":
			w.Header().Set("X-Auth-Token", "mock-token-xyz")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success": true}`))
		case "/api/v1.0/statistics":
			token := r.Header.Get("X-Auth-Token")
			if token != "mock-token-xyz" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			payload := map[string]any{
				"device": map[string]any{
					"cpu": map[string]any{
						"usage": 15.0,
					},
					"ram": map[string]any{
						"usage": 30.0,
					},
					"uptime": 5000.0,
				},
				"wireless": map[string]any{
					"radios": []any{
						map[string]any{
							"frequency": map[string]any{
								"center": 5500.0,
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	hostPort := strings.TrimPrefix(ts.URL, "http://")
	dev := config.DeviceConfig{
		Name:     "MockAP",
		IP:       hostPort,
		Username: "admin",
		Password: "password",
	}

	sys := config.DefaultSystemConfig()
	store := metrics.GetStore()
	sc := scraper.NewDeviceScraper(dev, config.DefaultsConfig{}, sys, map[string]bool{"wireless": true, "system": true}, nil)
	client := ts.Client()

	ctx := logger.WithTraceID(context.Background(), "probe-mock-123")
	okScrape := sc.Scrape(ctx, client, store, true)
	if !okScrape {
		t.Errorf("expected scrape to succeed with trace context")
	}

	cpu, ok := store.GetDeviceMetric("MockAP", hostPort, "cpu_usage")
	if !ok || cpu != 15.0 {
		t.Errorf("expected cpu_usage 15.0 from mock server, got %v (found: %v)", cpu, ok)
	}
}

func TestDeviceScraper_Scrape_Errors(t *testing.T) {
	sys := config.DefaultSystemConfig()
	store := metrics.GetStore()

	// 1. Missing credentials
	scNoCreds := scraper.NewDeviceScraper(
		config.DeviceConfig{Name: "NoCreds", IP: "127.0.0.1"},
		config.DefaultsConfig{},
		sys,
		nil,
		nil,
	)
	if scNoCreds.Scrape(context.Background(), http.DefaultClient, store, true) {
		t.Errorf("expected scrape to fail when credentials are missing")
	}

	// 2. Auth failure
	tsAuthFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer tsAuthFail.Close()

	scAuthFail := scraper.NewDeviceScraper(
		config.DeviceConfig{Name: "AuthFail", IP: strings.TrimPrefix(tsAuthFail.URL, "http://"), Username: "u", Password: "p"},
		config.DefaultsConfig{},
		sys,
		nil,
		nil,
	)
	if scAuthFail.Scrape(context.Background(), tsAuthFail.Client(), store, true) {
		t.Errorf("expected scrape to fail when login returns 401")
	}

	// 3. Stats 500 error
	tsStatsError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1.0/user/login" {
			w.Header().Set("X-Auth-Token", "tok")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer tsStatsError.Close()

	scStatsError := scraper.NewDeviceScraper(
		config.DeviceConfig{Name: "StatsError", IP: strings.TrimPrefix(tsStatsError.URL, "http://"), Username: "u", Password: "p"},
		config.DefaultsConfig{},
		sys,
		nil,
		nil,
	)
	if scStatsError.Scrape(context.Background(), tsStatsError.Client(), store, true) {
		t.Errorf("expected scrape to fail when stats returns 500")
	}

	// 4. JSON decode error
	tsDecodeError := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1.0/user/login" {
			w.Header().Set("X-Auth-Token", "tok")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid-json-body`))
	}))
	defer tsDecodeError.Close()

	scDecodeError := scraper.NewDeviceScraper(
		config.DeviceConfig{Name: "DecodeError", IP: strings.TrimPrefix(tsDecodeError.URL, "http://"), Username: "u", Password: "p"},
		config.DefaultsConfig{},
		sys,
		nil,
		nil,
	)
	if scDecodeError.Scrape(context.Background(), tsDecodeError.Client(), store, true) {
		t.Errorf("expected scrape to fail on malformed JSON payload")
	}

	// 5. Daemon failure (dryRun=false) purges stale telemetry, sets scrape_success=0, and preserves scrape_error
	store.ClearAllMetrics()
	store.UpdateMetric("cpu_usage", map[string]string{"name": "DaemonFail", "ip": strings.TrimPrefix(tsAuthFail.URL, "http://")}, 50.0)
	scDaemonFail := scraper.NewDeviceScraper(
		config.DeviceConfig{Name: "DaemonFail", IP: strings.TrimPrefix(tsAuthFail.URL, "http://"), Username: "u", Password: "p"},
		config.DefaultsConfig{},
		sys,
		nil,
		nil,
	)
	if scDaemonFail.Scrape(context.Background(), tsAuthFail.Client(), store, false) {
		t.Errorf("expected daemon scrape to fail on 401")
	}
	if _, hasCPU := store.GetDeviceMetric("DaemonFail", scDaemonFail.IP, "cpu_usage"); hasCPU {
		t.Errorf("expected stale cpu_usage metric to be purged on daemon scrape failure")
	}
	succVal, hasSucc := store.GetDeviceMetric("DaemonFail", scDaemonFail.IP, "scrape_success")
	if !hasSucc || succVal != 0.0 {
		t.Errorf("expected scrape_success=0.0 on daemon scrape failure, got %v (found: %v)", succVal, hasSucc)
	}
	errVal, hasErr := store.GetDeviceMetric("DaemonFail", scDaemonFail.IP, "scrape_error")
	if !hasErr || errVal != 1.0 {
		t.Errorf("expected scrape_error=1.0 to be preserved on daemon scrape failure, got %v (found: %v)", errVal, hasErr)
	}
}
