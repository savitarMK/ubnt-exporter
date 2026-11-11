package scraper_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

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

	// Test default socket timeout fallback when <= 0
	cZero := scraper.NewHTTPClient(0)
	if cZero == nil {
		t.Fatalf("expected non-nil client for 0 timeout")
	}

	// Test redirect behavior (CheckRedirect returns http.ErrUseLastResponse)
	tsRedirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer tsRedirect.Close()

	respRedirect, err := cZero.Get(tsRedirect.URL + "/redirect")
	if err != nil || respRedirect.StatusCode != http.StatusFound {
		t.Errorf("expected 302 without following redirect, got %v (err=%v)", respRedirect.StatusCode, err)
	} else {
		_ = respRedirect.Body.Close()
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

	// 6. Failure backoff active in daemon mode (dryRun=false) skips network call immediately
	scDaemonFail.NextAllowedScrape = time.Now().Add(60 * time.Second)
	if scDaemonFail.Scrape(context.Background(), tsAuthFail.Client(), store, false) {
		t.Errorf("expected daemon scrape during active backoff window to return false")
	}

	// 7. Token expiration (HTTP 401 on first statistics request triggers automatic re-login and succeeds) & AirFiber family dispatch using NewHTTPClient
	attempts := 0
	tsReAuth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1.0/user/login" {
			w.Header().Set("x-auth-token", "fresh-token-999")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		attempts++
		if r.Header.Get("X-Auth-Token") != "fresh-token-999" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"device": {"identification": {"model": "airFiber 5XHD"}, "cpu": [{"usage": 22.0}]},
			"interfaces": [{"id": "eth0", "enabled": true, "statistics": {"rxBytes": 100}}],
			"wireless": {"serviceUptime": 100, "radios": [{"linkState": "connected", "frequency": {"tx": 5840, "rx": 5215}}], "peers": []}
		}`))
	}))
	defer tsReAuth.Close()

	realClient := scraper.NewHTTPClient(2 * time.Second)
	scReAuth := scraper.NewDeviceScraper(
		config.DeviceConfig{Name: "AF5XHD-Node", IP: strings.TrimPrefix(tsReAuth.URL, "http://"), Username: "u", Password: "p"},
		config.DefaultsConfig{},
		sys,
		map[string]bool{"system": true, "interfaces": true, "airfiber": true, "peers": true},
		[]string{"tower"},
	)
	scReAuth.Token = "expired-token"
	if !scReAuth.Scrape(context.Background(), realClient, store, false) {
		t.Errorf("expected scrape to succeed after automatic 401 re-authentication")
	}
	if attempts < 2 {
		t.Errorf("expected at least 2 statistics attempts (expired + retry), got %d", attempts)
	}

	// 8. Missing credentials returns false immediately
	scEmptyCreds := scraper.NewDeviceScraper(
		config.DeviceConfig{Name: "NoCreds", IP: "127.0.0.1", Username: "", Password: ""},
		config.DefaultsConfig{},
		sys,
		nil,
		nil,
	)
	if scEmptyCreds.Scrape(context.Background(), realClient, store, false) {
		t.Errorf("expected scrape to fail when credentials are missing")
	}

	// 9. HTTP 500 on statistics endpoint
	ts500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1.0/user/login" {
			w.Header().Set("x-auth-token", "tok500")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts500.Close()
	sc500 := scraper.NewDeviceScraper(
		config.DeviceConfig{Name: "HTTP500", IP: strings.TrimPrefix(ts500.URL, "http://"), Username: "u", Password: "p"},
		config.DefaultsConfig{},
		sys,
		nil,
		nil,
	)
	if sc500.Scrape(context.Background(), realClient, store, false) {
		t.Errorf("expected scrape to fail on HTTP 500")
	}

	// 10. BuildScrapers with nil inventory returns nil
	if scrapers := scraper.BuildScrapers(nil, sys, nil); scrapers != nil {
		t.Errorf("expected BuildScrapers(nil) to return nil")
	}
}

func TestOfflineScraper_FixtureReplay(t *testing.T) {
	store := metrics.NewStore()
	store.RegisterDynamicLabels([]string{"sector"})

	sc := scraper.NewOfflineScraper("AC-PTMP-FIXTURE", "192.168.1.10", map[string]string{"sector": "norte"}, []string{"sector"})
	if sc.Name != "AC-PTMP-FIXTURE" || sc.IP != "192.168.1.10" {
		t.Fatalf("unexpected offline scraper identity: %s (%s)", sc.Name, sc.IP)
	}

	// Default fallback name and IP
	scDefault := scraper.NewOfflineScraper("", "", nil, nil)
	if scDefault.Name != "offline-fixture" || scDefault.IP != "127.0.0.1" {
		t.Errorf("expected default offline scraper name/IP, got %q / %q", scDefault.Name, scDefault.IP)
	}

	ctx := context.Background()
	fixturePath := "../../testdata/fixtures/airmax_ac_ptmp.json"
	if err := sc.ScrapeFile(ctx, fixturePath, store, true); err != nil {
		t.Fatalf("expected ScrapeFile on airmax_ac_ptmp.json to succeed, got: %v", err)
	}

	// Verify real fixture metrics were extracted accurately
	if cpu, ok := store.GetDeviceMetric("AC-PTMP-FIXTURE", "192.168.1.10", "cpu_usage"); !ok || cpu != 12.0 {
		t.Errorf("expected cpu_usage=12.0 from fixture, got %v (ok=%v)", cpu, ok)
	}
	if sig, ok := store.GetDeviceMetric("AC-PTMP-FIXTURE", "192.168.1.10", "ap_signal"); !ok || sig != -47.0 {
		t.Errorf("expected ap_signal=-47.0 from fixture, got %v (ok=%v)", sig, ok)
	}
	if snr, ok := store.GetDeviceMetric("AC-PTMP-FIXTURE", "192.168.1.10", "ap_snr"); !ok || snr != 43.0 {
		t.Errorf("expected ap_snr=43.0 from fixture, got %v (ok=%v)", snr, ok)
	}
	if clients, ok := store.GetDeviceMetric("AC-PTMP-FIXTURE", "192.168.1.10", "ap_clients"); !ok || clients != 1.0 {
		t.Errorf("expected ap_clients=1.0 from fixture, got %v (ok=%v)", clients, ok)
	}

	// Verify AirFiber 5XHD PtP fixture replayed through unified pipeline (no duplicated metrics)
	scAF := scraper.NewOfflineScraper("AF5XHD-PTP-FIXTURE", "192.168.1.20", map[string]string{"sector": "troncal"}, []string{"sector"})
	afFixturePath := "../../testdata/fixtures/af5xhd_ptp.json"
	if err := scAF.ScrapeFile(ctx, afFixturePath, store, true); err != nil {
		t.Fatalf("expected ScrapeFile on af5xhd_ptp.json to succeed, got: %v", err)
	}
	if freqTx, ok := store.GetDeviceMetric("AF5XHD-PTP-FIXTURE", "192.168.1.20", "af_frequency_tx"); !ok || freqTx != 5995.0 {
		t.Errorf("expected af_frequency_tx=5995.0 from AF5XHD fixture, got %v (ok=%v)", freqTx, ok)
	}
	if freqRx, ok := store.GetDeviceMetric("AF5XHD-PTP-FIXTURE", "192.168.1.20", "af_frequency_rx"); !ok || freqRx != 6065.0 {
		t.Errorf("expected af_frequency_rx=6065.0 from AF5XHD fixture, got %v (ok=%v)", freqRx, ok)
	}
	if snr, ok := store.GetDeviceMetric("AF5XHD-PTP-FIXTURE", "192.168.1.20", "ap_snr"); !ok || snr != 47.0 {
		t.Errorf("expected ap_snr=47.0 from AF5XHD fixture, got %v (ok=%v)", snr, ok)
	}
	if dlRatio, ok := store.GetDeviceMetric("AF5XHD-PTP-FIXTURE", "192.168.1.20", "af_dl_ratio"); !ok || dlRatio != 50.0 {
		t.Errorf("expected af_dl_ratio=50.0 from AF5XHD fixture, got %v (ok=%v)", dlRatio, ok)
	}

	// Verify AirMAX M-series (airOS v6) PtMP fixture with 1x1 SISO stations (AirGrid M5 HP / LiteBeam M5)
	scM := scraper.NewOfflineScraper("M5-PTMP-FIXTURE", "192.168.1.30", map[string]string{"sector": "sur"}, []string{"sector"})
	mFixturePath := "../../testdata/fixtures/airmax_m_ptmp.json"
	if err := scM.ScrapeFile(ctx, mFixturePath, store, true); err != nil {
		t.Fatalf("expected ScrapeFile on airmax_m_ptmp.json to succeed, got: %v", err)
	}
	if freqM, ok := store.GetDeviceMetric("M5-PTMP-FIXTURE", "192.168.1.30", "ap_frequency"); !ok || freqM != 5380.0 {
		t.Errorf("expected ap_frequency=5380.0 from AirMAX M fixture, got %v (ok=%v)", freqM, ok)
	}
	if _, hasCtrlFreq := store.GetDeviceMetric("M5-PTMP-FIXTURE", "192.168.1.30", "ap_control_frequency"); hasCtrlFreq {
		t.Errorf("expected ap_control_frequency not to be emitted when 'control' frequency is absent on AirMAX M-series")
	}
	if clientsM, ok := store.GetDeviceMetric("M5-PTMP-FIXTURE", "192.168.1.30", "ap_clients"); !ok || clientsM != 4.0 {
		t.Errorf("expected ap_clients=4.0 from AirMAX M fixture, got %v (ok=%v)", clientsM, ok)
	}
	if _, hasSISOImbalance := store.GetDeviceMetric("M5-PTMP-FIXTURE", "192.168.1.30", "peer_chain_imbalance"); hasSISOImbalance {
		t.Errorf("expected peer_chain_imbalance not to be emitted for 1x1 SISO AirMAX M stations")
	}
	if _, hasSISORemImbalance := store.GetDeviceMetric("M5-PTMP-FIXTURE", "192.168.1.30", "peer_remote_imbalance"); hasSISORemImbalance {
		t.Errorf("expected peer_remote_imbalance not to be emitted for 1x1 SISO AirMAX M stations")
	}

	// Verify custom dynamic labels ("sector") are propagated across all metric families (AP, Peer, Interface, Scrape)
	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics.NewUbiquitiCollector(store))
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics from store: %v", err)
	}
	checkedFamilies := map[string]bool{
		"ubnt_ap_signal_dbm":        false,
		"ubnt_peer_signal_dbm":      false,
		"ubnt_iface_rx_bytes_total": false,
		"ubnt_scrape_success":       false,
	}
	for _, mf := range mfs {
		name := mf.GetName()
		if _, watch := checkedFamilies[name]; !watch {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "sector" && (lp.GetValue() == "norte" || lp.GetValue() == "troncal") {
					checkedFamilies[name] = true
				}
			}
		}
	}
	for fam, found := range checkedFamilies {
		if !found {
			t.Errorf("expected custom label 'sector' to be present on metric family %s", fam)
		}
	}

	// Error cases: nonexistent file, malformed JSON, empty array
	if err := sc.ScrapeFile(ctx, "/nonexistent/fixture.json", store, false); err == nil {
		t.Errorf("expected error for nonexistent fixture file")
	}
	if err := sc.ScrapeBytes(ctx, []byte("invalid-json"), store, false); err == nil {
		t.Errorf("expected error for malformed JSON bytes")
	}
	if err := sc.ScrapeBytes(ctx, []byte("[]"), store, false); err == nil {
		t.Errorf("expected error for empty JSON array")
	}
}

// Regression: IPv6 device addresses must be bracketed in the base URL, and surrounding
// whitespace (accepted by config validation) must not leak into the URL or the ip label.
func TestNewDeviceScraperURLHost(t *testing.T) {
	sslTrue := true
	cases := []struct {
		name    string
		ip      string
		ssl     *bool
		wantURL string
		wantIP  string
	}{
		{"ipv4", "10.0.0.1", nil, "http://10.0.0.1/api/v1.0", "10.0.0.1"},
		{"ipv4 with spaces", "  10.0.0.2 ", nil, "http://10.0.0.2/api/v1.0", "10.0.0.2"},
		{"ipv6", "fd00::1", nil, "http://[fd00::1]/api/v1.0", "fd00::1"},
		{"ipv6 https with spaces", " 2001:db8::10 ", &sslTrue, "https://[2001:db8::10]/api/v1.0", "2001:db8::10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := scraper.NewDeviceScraper(config.DeviceConfig{IP: tc.ip, SSL: tc.ssl}, config.DefaultsConfig{}, nil, nil, nil)
			if sc.BaseURL != tc.wantURL {
				t.Errorf("BaseURL: expected %q, got %q", tc.wantURL, sc.BaseURL)
			}
			if sc.IP != tc.wantIP {
				t.Errorf("IP: expected %q, got %q", tc.wantIP, sc.IP)
			}
			if sc.Name != tc.wantIP {
				t.Errorf("Name fallback: expected %q, got %q", tc.wantIP, sc.Name)
			}
		})
	}
}

// Regression: a device configured with an IPv6 address must be scraped end-to-end.
func TestScrapeIPv6Device(t *testing.T) {
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1.0/user/login":
			w.Header().Set("X-Auth-Token", "tok")
			w.WriteHeader(http.StatusOK)
		case "/api/v1.0/statistics":
			_, _ = w.Write([]byte(`{"device":{"cpu":{"usage":12},"ram":{"usage":34}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback not available: %v", err)
	}
	ts.Listener = ln
	ts.Start()
	defer ts.Close()

	_, port, _ := net.SplitHostPort(ln.Addr().String())
	sc := scraper.NewDeviceScraper(config.DeviceConfig{Name: "v6", IP: "::1", Username: "u", Password: "p"}, config.DefaultsConfig{}, nil, nil, nil)
	// The production URL has no explicit port; point it at the test server's port.
	sc.BaseURL = strings.Replace(sc.BaseURL, "[::1]", "[::1]:"+port, 1)

	store := metrics.NewStore()
	if ok := sc.Scrape(context.Background(), scraper.NewHTTPClient(2*time.Second), store, true); !ok {
		t.Fatalf("expected IPv6 scrape to succeed (BaseURL=%s)", sc.BaseURL)
	}
	if v, found := store.GetDeviceMetric("v6", "::1", "cpu_usage"); !found || v != 12 {
		t.Errorf("expected cpu_usage 12, got %v (found=%v)", v, found)
	}
}

// host:port (used by mock servers) must pass through unchanged; any IPv6-notation literal is bracketed.
func TestNewDeviceScraperURLHostPassthrough(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:8443":  "http://127.0.0.1:8443/api/v1.0",
		"::ffff:10.0.0.1": "http://[::ffff:10.0.0.1]/api/v1.0", // IPv4-mapped in IPv6 notation still needs brackets
	}
	for ip, want := range cases {
		sc := scraper.NewDeviceScraper(config.DeviceConfig{IP: ip}, config.DefaultsConfig{}, nil, nil, nil)
		if sc.BaseURL != want {
			t.Errorf("%s: expected %q, got %q", ip, want, sc.BaseURL)
		}
	}
}
