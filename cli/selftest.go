package cli

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"ubnt-exporter/internal/collector"
	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/definitions"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/scraper"
	"ubnt-exporter/internal/utils"
)

type selfTestSink struct {
	values map[string]float64
	labels map[string]map[string]string
}

func newSelfTestSink() *selfTestSink {
	return &selfTestSink{
		values: make(map[string]float64),
		labels: make(map[string]map[string]string),
	}
}

func (s *selfTestSink) UpdateMetric(key string, labels map[string]string, value float64) {
	s.values[key] = value
	cp := make(map[string]string, len(labels))
	for k, v := range labels {
		cp[k] = v
	}
	s.labels[key] = cp
}

// CmdTest executes built-in verification suites inside the container (metrics, collectors, connectivity, endpoint).
func CmdTest(sysConf *config.SystemConfig, invConf *config.InventoryConfig, suite string, targetFilter string) int {
	suite = strings.ToLower(strings.TrimSpace(suite))
	if suite == "" {
		suite = "all"
	}

	fmt.Println("\n UBNT-EXPORTER BUILT-IN SELF-TEST & VERIFICATION SUITE")
	fmt.Println("======================================================================")

	totalFailed := 0

	if suite == "all" || suite == "metrics" {
		if !runSuiteMetrics() {
			totalFailed++
		}
	}

	if suite == "all" || suite == "collectors" {
		if !runSuiteCollectors() {
			totalFailed++
		}
	}

	if suite == "all" || suite == "connectivity" {
		if !runSuiteConnectivity(sysConf, invConf, targetFilter) {
			totalFailed++
		}
	}

	if suite == "all" || suite == "endpoint" {
		if !runSuiteEndpoint(sysConf) {
			totalFailed++
		}
	}

	fmt.Println("======================================================================")
	if totalFailed > 0 {
		fmt.Printf("SELF-TEST RESULT: FAILED (%d suite(s) reported errors)\n\n", totalFailed)
		return 1
	}
	fmt.Println("SELF-TEST RESULT: ALL SUITES PASSED OK")
	fmt.Println()
	return 0
}

func runSuiteMetrics() bool {
	return RunSuiteMetricsWith(definitions.MetricsDef)
}

// RunSuiteMetricsWith validates metric definitions against naming, help, type, and label schema rules.
func RunSuiteMetricsWith(defs map[string]definitions.MetricDefinition) bool {
	fmt.Println("\n[Suite 1/4] Metrics Dictionary & Prometheus Schema Integrity")
	count := len(defs)
	if count == 0 {
		fmt.Println("  [FAIL] MetricsDef dictionary is empty")
		return false
	}

	errs := 0
	for key, def := range defs {
		if !strings.HasPrefix(def.Name, "ubnt_") {
			fmt.Printf("  [FAIL] metric %q missing 'ubnt_' prefix (%s)\n", key, def.Name)
			errs++
		}
		if strings.TrimSpace(def.Help) == "" {
			fmt.Printf("  [FAIL] metric %q has empty Help string\n", key)
			errs++
		}
		if def.Type != definitions.TypeGauge && def.Type != definitions.TypeCounter && def.Type != definitions.TypeInfo {
			fmt.Printf("  [FAIL] metric %q has invalid type %q\n", key, def.Type)
			errs++
		}
		for _, lbl := range def.Labels {
			if !config.IsValidPrometheusLabelName(lbl) {
				fmt.Printf("  [FAIL] metric %q has invalid label name %q\n", key, lbl)
				errs++
			}
		}
	}

	if errs > 0 {
		fmt.Printf("  [FAIL] %d schema error(s) across %d metric definitions\n", errs, count)
		return false
	}
	fmt.Printf("  [PASS] Verified %d metric definitions (names, types, HELP, and label schemas)\n", count)
	return true
}

func runSuiteCollectors() bool {
	fmt.Println("\n[Suite 2/4] Collector Engine Unit Tests (system, interfaces, wireless, peers, airfiber)")
	apLabels := map[string]string{"name": "selftest-unit", "ip": "127.0.0.1"}

	// 1. AirMAX AC synthetic payload verification
	acSink := newSelfTestSink()
	acData := map[string]any{
		"timestamp": 1792247211000.0,
		"device": map[string]any{
			"cpu":          []any{map[string]any{"identifier": "MIPS 74Kc V5.0", "usage": 38.0}},
			"ram":          map[string]any{"usage": 31.0, "free": 90000000.0, "total": 130000000.0},
			"uptime":       100000.0,
			"powerTime":    100000.0,
			"temperatures": []any{map[string]any{"name": "Board", "type": "board", "value": 44.0}},
			"gps":          map[string]any{"fix": true, "lat": 18.45, "lon": -97.42, "alt": 1730.0, "dop": 0.8, "dim": 3.0, "sats": 11.0},
		},
		"interfaces": []any{
			map[string]any{
				"id": "eth0", "name": "LAN0", "enabled": true, "plugged": true, "speed": 1000.0, "duplex": true, "mtu": 1500.0, "snr": 30.0,
				"statistics": map[string]any{"rxBytes": 5000.0, "txBytes": 3000.0},
			},
			map[string]any{"id": "ath0.99", "name": "", "statistics": map[string]any{"rxBytes": 100.0}},
		},
		"wireless": map[string]any{
			"serviceUptime": 99900.0,
			"serviceTime":   100000.0,
			"radios": []any{
				map[string]any{
					"linkState":    "connected",
					"frequency":    map[string]any{"center": 5595.0, "control": 5595.0},
					"channelWidth": map[string]any{"current": 20.0},
					"outputPower":  map[string]any{"conducted": 21.0, "eirp": 32.0},
					"noiseFloor":   -88.0,
					"dfs":          map[string]any{"enabled": true, "status": 1.0, "cacRemaining": 0.0, "cacDuration": 60.0},
				},
			},
			"peers": []any{
				map[string]any{
					"common": map[string]any{
						"hostname": "peer-ac-01", "mgmtIp": "10.0.0.10", "distance": 1500.0, "uptime": 50000.0,
						"connected": true, "active": true, "countryCode": "AR",
						"identification": map[string]any{"mac": "00:11:22:33:44:55", "model": "LBE-5AC-Gen2", "family": "AirMAX AC"},
						"counters":       map[string]any{"txBytes": 1000.0, "rxBytes": 2000.0, "txRate": 10000000.0, "rxRate": 20000000.0},
					},
					"local": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"signal": -58.0, "noiseFloor": -88.0, "idealSignal": -56.0,
								"signalPerChain": []any{-58.0, -59.0}, "idealSignalPerChain": []any{-56.0, -56.0},
								"capacity": map[string]any{"dl": 120000.0, "ul": 110000.0},
							},
						},
					},
					"remote": []any{
						map[string]any{
							"netMode": "router",
							"gps":     map[string]any{"fix": true, "lat": 18.46, "lon": -97.43},
							"linkQuality": map[string]any{
								"signal": -60.0, "noiseFloor": -90.0, "idealSignal": -56.0,
							},
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	collector.ProcessSystem(ctx, acData, apLabels, acSink)
	collector.ProcessInterfaces(ctx, acData, apLabels, acSink)
	collector.ProcessWireless(ctx, acData, apLabels, nil, acSink)
	collector.ProcessPeers(ctx, acData, apLabels, acSink)

	if acSink.values["cpu_usage"] != 38.0 || acSink.values["ap_snr"] != 30.0 || acSink.values["peer_snr"] != 30.0 {
		fmt.Printf("  [FAIL] AirMAX AC collector test failed (cpu=%v, ap_snr=%v, peer_snr=%v)\n",
			acSink.values["cpu_usage"], acSink.values["ap_snr"], acSink.values["peer_snr"])
		return false
	}
	if acSink.labels["iface_info"]["vlan_id"] != "99" || acSink.labels["iface_info"]["parent_interface"] != "ath0" {
		fmt.Printf("  [FAIL] Interface VLAN classification failed: %v\n", acSink.labels["iface_info"])
		return false
	}
	fmt.Println("  [PASS] AirMAX AC pipeline (system, interfaces, wireless, peers) verified OK")

	// 2. AirFiber 5XHD synthetic payload verification
	afSink := newSelfTestSink()
	afData := map[string]any{
		"device": map[string]any{
			"ntpClient": map[string]any{"timeSyncEnabled": true, "lastSync": 1788020030000.0},
		},
		"interfaces": []any{
			map[string]any{"id": "ath0/0", "name": "ath0/0", "enabled": true},
		},
		"wireless": map[string]any{
			"serviceUptime": 808949.0,
			"radios": []any{
				map[string]any{
					"linkState":    "connected",
					"frequency":    map[string]any{"tx": 5840.0, "rx": 5215.0},
					"channelWidth": map[string]any{"tx": 50.0, "rx": 50.0},
					"outputPower":  map[string]any{"conducted": 24.0, "eirp": 54.0, "autoPower": false},
					"noiseFloor":   -92.0,
				},
			},
			"peers": []any{
				map[string]any{
					"common": map[string]any{
						"hostname":       "af5xhd-slave",
						"identification": map[string]any{"family": "ltu", "subsystemID": "ae03"},
					},
					"local": []any{
						map[string]any{
							"linkQuality": map[string]any{
								"signal": -43.0, "idealSignal": -45.0,
								"capacity":    map[string]any{"dl": 268290.0, "ul": 254200.0, "combined": 522490.0},
								"rxHistogram": []any{map[string]any{"rate": 8.0, "value": 99999.0}},
							},
						},
					},
					"remote": []any{
						map[string]any{
							"rxEfficiency": 16.23,
							"gps":          map[string]any{"fix": true, "alt": 2108.0, "sats": 9.0, "dop": 1.12, "dim": 3.0},
							"ntpClient":    map[string]any{"timeSyncEnabled": true, "lastSync": 1788019813000.0},
							"linkQuality": map[string]any{
								"signal":      -55.0,
								"outputPower": map[string]any{"conducted": 24.0, "eirp": 54.0, "autoPower": false},
							},
						},
					},
				},
			},
		},
	}

	collector.ProcessSystem(ctx, afData, apLabels, afSink)
	collector.ProcessInterfaces(ctx, afData, apLabels, afSink)
	collector.ProcessAF5XHD(ctx, afData, apLabels, nil, afSink)
	collector.ProcessPeers(ctx, afData, apLabels, afSink)

	if afSink.values["af_remote_eirp"] != 54.0 || math.Abs(afSink.values["ap_snr"]-49.0) > 1e-4 || math.Abs(afSink.values["peer_snr"]-49.0) > 1e-4 {
		fmt.Printf("  [FAIL] AirFiber 5XHD collector test failed (af_remote_eirp=%v, ap_snr=%v, peer_snr=%v)\n",
			afSink.values["af_remote_eirp"], afSink.values["ap_snr"], afSink.values["peer_snr"])
		return false
	}
	if afSink.labels["iface_info"]["parent_interface"] != "ath0" || afSink.values["peer_gps_alt"] != 2108.0 {
		fmt.Printf("  [FAIL] AirFiber 5XHD interface/GPS verification failed\n")
		return false
	}
	fmt.Println("  [PASS] AirFiber 5XHD pipeline (remote outputPower, SNR fallback, GPS/NTP, ath0/0) verified OK")
	return true
}

func runSuiteConnectivity(sysConf *config.SystemConfig, invConf *config.InventoryConfig, targetFilter string) bool {
	fmt.Println("\n[Suite 3/4] Inventory Device Connectivity, TLS Certificate & API Handshake")
	if invConf == nil || len(invConf.Devices) == 0 {
		fmt.Println("  [WARN] No devices configured in config.yaml")
		return true
	}

	dynamicKeys := config.DiscoverCustomLabels(invConf)
	scrapers := scraper.BuildScrapers(invConf, sysConf, dynamicKeys)
	filtered, tErr := FilterScrapersByTarget(scrapers, targetFilter)
	if tErr != nil {
		fmt.Printf("  [FAIL] Invalid RE2 target filter: %v\n", tErr)
		return false
	}

	client := scraper.NewHTTPClient(time.Duration(sysConf.Performance.SocketTimeout) * time.Second)

	matched := len(filtered)
	failed := 0

	for _, sc := range filtered {
		fmt.Printf("  -> Target: %s (%s, SSL=%v)\n", sc.Name, sc.IP, sc.SSL)

		port := 80
		if sc.SSL {
			port = 443
		}
		addr := sc.IP
		if _, _, splitErr := net.SplitHostPort(sc.IP); splitErr != nil {
			addr = net.JoinHostPort(sc.IP, fmt.Sprintf("%d", port))
		}

		// Step A: TCP Socket Reachability
		conn, err := net.DialTimeout("tcp", addr, sc.TimeoutSocket)
		if err != nil {
			fmt.Printf("     [FAIL] TCP Socket (%s): %v\n", addr, err)
			failed++
			continue
		}
		_ = conn.Close()
		fmt.Printf("     [PASS] TCP Socket (%s) reachable\n", addr)

		// Step B: TLS Handshake & Certificate Inspection
		if sc.SSL {
			tlsDialer := &net.Dialer{Timeout: sc.TimeoutSocket}
			// #nosec G402 -- Ubiquiti AirOS/AirFiber devices use embedded self-signed certificates and legacy TLS 1.0+
			tlsConn, tErr := tls.DialWithDialer(tlsDialer, "tcp", addr, &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS10,
			})
			if tErr != nil {
				fmt.Printf("     [FAIL] TLS Handshake: %v\n", tErr)
				failed++
				continue
			}
			state := tlsConn.ConnectionState()
			if len(state.PeerCertificates) > 0 {
				cert := state.PeerCertificates[0]
				fmt.Printf("     [PASS] TLS Handshake OK (CN=%q, Serial=%s)\n", cert.Subject.CommonName, cert.SerialNumber.String())
			} else {
				fmt.Printf("     [PASS] TLS Handshake OK\n")
			}
			_ = tlsConn.Close()
		}

		// Step C: API Login & Statistics Collection
		loginURL := fmt.Sprintf("%s/user/login", sc.BaseURL)
		payload, _ := json.Marshal(map[string]string{"username": sc.Username, "password": sc.Password})
		reqCtx, cancel := context.WithTimeout(context.Background(), sc.TimeoutLogin)
		req, _ := http.NewRequestWithContext(reqCtx, "POST", loginURL, bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, lErr := client.Do(req)
		cancel()
		if lErr != nil {
			fmt.Printf("     [FAIL] API Login (%s): %v\n", loginURL, lErr)
			failed++
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("     [FAIL] API Login returned HTTP %d (check credentials)\n", resp.StatusCode)
			failed++
			continue
		}
		token := resp.Header.Get("X-Auth-Token")
		if token == "" {
			token = resp.Header.Get("x-auth-token")
		}
		sc.Token = token
		fmt.Printf("     [PASS] API Login HTTP 200 OK (X-Auth-Token acquired)\n")

		// Step D: Full Scrape Dry-Run Verification
		tmpStore := metrics.NewStore()
		tmpStore.RegisterDynamicLabels(dynamicKeys)
		if !sc.Scrape(context.Background(), client, tmpStore, false) {
			fmt.Printf("     [FAIL] Telemetry scrape failed for %s\n", sc.Name)
			failed++
			continue
		}
		fmt.Printf("     [PASS] Statistics payload decoded and collectors executed OK\n")
	}

	if matched == 0 {
		fmt.Printf("  [FAIL] No inventory device matched target filter %q\n", targetFilter)
		return false
	}
	return failed == 0
}

func runSuiteEndpoint(sysConf *config.SystemConfig) bool {
	fmt.Println("\n[Suite 4/4] Local Prometheus HTTP Endpoint (/metrics)")
	base := resolveLocalHTTPBase(sysConf)
	metricsURL := base + "/metrics"

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(metricsURL)
	if err != nil {
		fmt.Printf("  [WARN] Local daemon not reachable at %s (%v) — skipped (daemon not running)\n", metricsURL, err)
		return true
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  [FAIL] %s returned HTTP %d\n", metricsURL, resp.StatusCode)
		return false
	}

	lines := strings.Split(string(body), "\n")
	ubntSeries := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "ubnt_") {
			ubntSeries++
		}
	}
	fmt.Printf("  [PASS] %s responded HTTP 200 OK (%d ubnt_* metric series exposed, %s)\n",
		metricsURL, ubntSeries, utils.Truncate(fmt.Sprintf("%d bytes", len(body)), 20))
	return true
}
