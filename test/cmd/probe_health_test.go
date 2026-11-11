package cmd_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ubnt-exporter/cli"
	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/scraper"
)

// mockDevice serves the AirOS login/statistics API, calling onStats before answering statistics.
func mockDevice(t *testing.T, onStats func()) (*httptest.Server, string) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1.0/user/login":
			w.Header().Set("X-Auth-Token", "tok")
			w.WriteHeader(http.StatusOK)
		case "/api/v1.0/statistics":
			onStats()
			_, _ = w.Write([]byte(`{"device":{"cpu":{"usage":42},"ram":{"usage":10}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, strings.TrimPrefix(ts.URL, "http://")
}

func probeState(t *testing.T, hostPort string) func() cli.ProbeState {
	t.Helper()
	sys := config.DefaultSystemConfig()
	sys.Performance.LoginTimeout = 2
	sys.Performance.RequestTimeout = 3
	sc := scraper.NewDeviceScraper(config.DeviceConfig{Name: "dev", IP: hostPort, Username: "u", Password: "p"}, config.DefaultsConfig{}, sys, nil, nil)
	client := scraper.NewHTTPClient(time.Second)
	return func() cli.ProbeState {
		return cli.ProbeState{Scrapers: []*scraper.DeviceScraper{sc}, Client: client}
	}
}

// Regression: a probe of a device slower than the server-wide WriteTimeout must still be
// delivered, because the handler extends the write deadline to the worst-case scrape time.
func TestProbeHandler_SlowDeviceBeyondServerWriteTimeout(t *testing.T) {
	_, hostPort := mockDevice(t, func() { time.Sleep(1200 * time.Millisecond) })

	srv := httptest.NewUnstartedServer(cli.NewProbeHandler(probeState(t, hostPort), 2, promhttp.HandlerOpts{}))
	srv.Config.WriteTimeout = 300 * time.Millisecond // much shorter than the device response
	srv.Start()
	defer srv.Close()

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(srv.URL + "/probe?target=dev")
	if err != nil {
		t.Fatalf("probe response was cut off by the server WriteTimeout: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading probe body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "ubnt_ap_cpu_usage_percent") {
		t.Fatalf("expected 200 with device metrics, got %d:\n%s", resp.StatusCode, body)
	}
}

// Regression: probes beyond the global concurrency limit are rejected with 503 + Retry-After.
func TestProbeHandler_ConcurrencyLimit(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	_, hostPort := mockDevice(t, func() {
		entered <- struct{}{}
		<-release
	})

	srv := httptest.NewServer(cli.NewProbeHandler(probeState(t, hostPort), 1, promhttp.HandlerOpts{}))
	defer srv.Close()
	client := &http.Client{Timeout: 10 * time.Second}

	firstDone := make(chan int, 1)
	go func() {
		resp, err := client.Get(srv.URL + "/probe?target=dev")
		if err != nil {
			firstDone <- -1
			return
		}
		_ = resp.Body.Close()
		firstDone <- resp.StatusCode
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first probe never reached the device")
	}

	resp, err := client.Get(srv.URL + "/probe?target=dev")
	if err != nil {
		t.Fatalf("second probe: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" {
		t.Errorf("expected 503 with Retry-After while limit is reached, got %d (Retry-After=%q)", resp.StatusCode, resp.Header.Get("Retry-After"))
	}

	close(release)
	if code := <-firstDone; code != http.StatusOK {
		t.Errorf("expected first probe to succeed with 200, got %d", code)
	}
}

func startDaemon(t *testing.T, listen string) (sysPath, cfgPath string, stop func()) {
	t.Helper()
	dir := t.TempDir()
	sysPath = filepath.Join(dir, "system.yaml")
	cfgPath = filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(sysPath, []byte(fmt.Sprintf("server:\n  listen: %q\n  scrape_interval: 60\nlogging:\n  level: ERROR\n", listen)), 0600)
	_ = os.WriteFile(cfgPath, []byte("devices: []\n"), 0600)
	sysConf, err := config.LoadSystemConfig(sysPath)
	if err != nil {
		t.Fatal(err)
	}
	invConf, _ := config.LoadInventoryConfig(cfgPath)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.CmdRunContext(ctx, nil, cfgPath, sysPath, sysConf, invConf)
	}()
	deadline := time.Now().Add(3 * time.Second)
	for cli.CmdHealth(sysConf) != 0 {
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("daemon on %s never became healthy", listen)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return sysPath, cfgPath, func() { cancel(); <-done }
}

func TestHealthEndpointAndCommand(t *testing.T) {
	sysPath, cfgPath, stop := startDaemon(t, "127.0.0.1:18994")

	resp, err := http.Get("http://127.0.0.1:18994/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	_ = resp.Body.Close()

	if code := cli.Execute([]string{"health", "-s", sysPath, "-c", cfgPath}); code != 0 {
		t.Errorf("expected 'health' command to exit 0 while daemon runs, got %d", code)
	}

	stop()
	sysConf, _ := config.LoadSystemConfig(sysPath)
	if code := cli.CmdHealth(sysConf); code != 1 {
		t.Errorf("expected exit 1 after daemon shutdown, got %d", code)
	}
}

func TestHealthCommandUnixSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "ue") // short path: unix socket paths are limited to ~108 bytes
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	sock := filepath.Join(dir, "m.sock")

	sysPath, cfgPath, stop := startDaemon(t, "unix://"+sock)
	defer stop()
	if code := cli.Execute([]string{"health", "-s", sysPath, "-c", cfgPath}); code != 0 {
		t.Errorf("expected 'health' over unix socket to exit 0, got %d", code)
	}
}
