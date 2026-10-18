package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/scraper"
	"ubnt-exporter/internal/utils"
)

type diagDeviceResult struct {
	Name        string
	IP          string
	Online      bool
	Signal      string
	Noise       string
	SNR         string
	ChainDelta  string
	DegradedImb bool
	Clients     string
	Capacity    string
}

// CmdDiag performs a live diagnostic scrape and prints an RF link quality ASCII table.
func CmdDiag(sysConf *config.SystemConfig, invConf *config.InventoryConfig, targetFilter string) int {
	fmt.Println("\n LIVE WIRELESS DIAGNOSTICS")
	fmt.Println("=====================================================================================================")

	store := metrics.GetStore()
	dynamicKeys := config.DiscoverCustomLabels(invConf)
	store.RegisterDynamicLabels(dynamicKeys)

	scrapers := scraper.BuildScrapers(invConf, sysConf, dynamicKeys)
	filtered, err := FilterScrapersByTarget(scrapers, targetFilter)
	if err != nil {
		fmt.Printf("ERROR in target filter: %v\n", err)
		return 1
	}

	if len(filtered) == 0 {
		fmt.Printf("No matching devices found in inventory for target %q.\n", targetFilter)
		return 1
	}

	client := scraper.NewHTTPClient(time.Duration(sysConf.Performance.SocketTimeout) * time.Second)
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
			SNR:        "-",
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

			if snr, ok := store.GetDeviceMetric(s.Name, s.IP, "ap_snr"); ok {
				res.SNR = fmt.Sprintf("%.0f dB", snr)
			} else if snr, ok := store.GetDeviceMetric(s.Name, s.IP, "peer_snr"); ok {
				res.SNR = fmt.Sprintf("%.0f dB", snr)
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

	fmt.Println("+----------------------+---------------+----------+-----------+-----------+--------+---------------+----------+----------+")
	fmt.Println("| Access Point         | Target IP     | Status   | Signal    | Noise     | SNR    | Chain Delta   | Clients  | Capacity |")
	fmt.Println("+======================+===============+==========+===========+===========+========+===============+==========+==========+")

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

		fmt.Printf("| %-20s | %-13s | %-8s | %-9s | %-9s | %-6s | %-13s | %-8s | %-8s |\n",
			utils.Truncate(r.Name, 20),
			utils.Truncate(r.IP, 13),
			statusStr,
			r.Signal,
			r.Noise,
			r.SNR,
			r.ChainDelta,
			r.Clients,
			r.Capacity,
		)
	}

	fmt.Println("+----------------------+---------------+----------+-----------+-----------+--------+---------------+----------+----------+")
	fmt.Printf("Audited devices: %d | Online: %d | Degraded chain delta (>5 dB): %d\n\n", len(results), onlineCount, degradedCount)
	return 0
}

// CmdInspect performs a live scrape of a device and groups all collected metrics by collector module.
func CmdInspect(sysConf *config.SystemConfig, invConf *config.InventoryConfig, targetFilter string, regexFilter string) int {
	re, resolvedDesc, rErr := CompileUserRegex(regexFilter)
	if rErr != nil {
		fmt.Printf("ERROR: %v\n", rErr)
		return 1
	}

	store := metrics.NewStore()
	dynamicKeys := config.DiscoverCustomLabels(invConf)
	store.RegisterDynamicLabels(dynamicKeys)

	scrapers := scraper.BuildScrapers(invConf, sysConf, dynamicKeys)
	filtered, tErr := FilterScrapersByTarget(scrapers, targetFilter)
	if tErr != nil {
		fmt.Printf("ERROR in target filter: %v\n", tErr)
		return 1
	}

	if len(filtered) == 0 {
		fmt.Printf("No matching device found in inventory for %q.\n", targetFilter)
		return 1
	}
	target := filtered[0]

	fmt.Printf("\n DEEP COLLECTOR INSPECTION: %s (%s)\n", target.Name, target.IP)
	if resolvedDesc != "" {
		fmt.Printf(" Filter: %s\n", resolvedDesc)
	}
	fmt.Println("======================================================================")

	client := scraper.NewHTTPClient(time.Duration(sysConf.Performance.SocketTimeout) * time.Second)
	ctx := logger.WithTraceID(context.Background(), "inspect")
	if !target.Scrape(ctx, client, store, true) {
		fmt.Printf("ERROR: Scrape failed for device %s (%s).\n", target.Name, target.IP)
		return 1
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics.NewUbiquitiCollector(store))
	mfs, err := reg.Gather()
	if err != nil {
		fmt.Printf("Error gathering metrics: %v\n", err)
		return 1
	}

	groups := map[string][]string{
		"1. System, CPU, Memory, GPS & NTP": {},
		"2. Network Interfaces & VLANs":     {},
		"3. Wireless Radio & AirFiber RF":   {},
		"4. Connected Peers & Stations":     {},
	}

	totalSeries := 0
	for _, mf := range mfs {
		name := mf.GetName()
		help := mf.GetHelp()
		for _, m := range mf.GetMetric() {
			var lbls []string
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "name" || lp.GetName() == "ip" || lp.GetName() == "ap_name" || lp.GetName() == "ap_ip" {
					continue
				}
				lbls = append(lbls, fmt.Sprintf("%s=%q", lp.GetName(), lp.GetValue()))
			}
			labelBlock := "{" + strings.Join(lbls, ",") + "}"
			if !MatchesMetricFilter(re, name, labelBlock, help) {
				continue
			}

			val := 0.0
			if m.Gauge != nil {
				val = m.Gauge.GetValue()
			} else if m.Counter != nil {
				val = m.Counter.GetValue()
			}
			line := fmt.Sprintf("  %-44s %-36s = %v", name, labelBlock, val)
			totalSeries++

			switch {
			case strings.HasPrefix(name, "ubnt_iface_"):
				groups["2. Network Interfaces & VLANs"] = append(groups["2. Network Interfaces & VLANs"], line)
			case strings.HasPrefix(name, "ubnt_peer_"):
				groups["4. Connected Peers & Stations"] = append(groups["4. Connected Peers & Stations"], line)
			case strings.HasPrefix(name, "ubnt_device_") || strings.HasPrefix(name, "ubnt_af_gps_") ||
				strings.HasPrefix(name, "ubnt_ap_cpu_") || strings.HasPrefix(name, "ubnt_ap_ram_") ||
				strings.HasPrefix(name, "ubnt_ap_uptime_") || strings.HasPrefix(name, "ubnt_ap_gps_") ||
				strings.HasPrefix(name, "ubnt_ap_latitude") || strings.HasPrefix(name, "ubnt_ap_longitude") ||
				strings.HasPrefix(name, "ubnt_af_power_time"):
				groups["1. System, CPU, Memory, GPS & NTP"] = append(groups["1. System, CPU, Memory, GPS & NTP"], line)
			default:
				groups["3. Wireless Radio & AirFiber RF"] = append(groups["3. Wireless Radio & AirFiber RF"], line)
			}
		}
	}

	keys := []string{
		"1. System, CPU, Memory, GPS & NTP",
		"2. Network Interfaces & VLANs",
		"3. Wireless Radio & AirFiber RF",
		"4. Connected Peers & Stations",
	}
	for _, k := range keys {
		lines := groups[k]
		if re != nil && len(lines) == 0 {
			continue
		}
		sort.Strings(lines)
		fmt.Printf("\n[%s] (%d series)\n", k, len(lines))
		for _, l := range lines {
			fmt.Println(l)
		}
	}

	fmt.Printf("\nTotal matching series for %s (%s): %d\n\n", target.Name, target.IP, totalSeries)
	return 0
}

// CmdExport performs a one-shot scrape and prints the full Prometheus exposition output.
func CmdExport(sysConf *config.SystemConfig, invConf *config.InventoryConfig, targetFilter string, regexFilter string) int {
	re, resolvedDesc, rErr := CompileUserRegex(regexFilter)
	if rErr != nil {
		fmt.Printf("ERROR: %v\n", rErr)
		return 1
	}

	fmt.Println("\n ONE-SHOT EXPORT (Dry Run)")
	if resolvedDesc != "" {
		fmt.Printf(" Filter: %s\n", resolvedDesc)
	}

	store := metrics.GetStore()
	dynamicKeys := config.DiscoverCustomLabels(invConf)
	store.RegisterDynamicLabels(dynamicKeys)

	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics.NewUbiquitiCollector(store))
	reg.MustRegister(store.ScrapeDuration)
	reg.MustRegister(metrics.NewBuildInfoGauge())
	metrics.GetTelemetry().Register(reg)

	scrapers := scraper.BuildScrapers(invConf, sysConf, dynamicKeys)
	filtered, tErr := FilterScrapersByTarget(scrapers, targetFilter)
	if tErr != nil {
		fmt.Printf("ERROR in target filter: %v\n", tErr)
		return 1
	}

	if len(filtered) == 0 {
		fmt.Printf("No matching devices found in inventory for target %q.\n", targetFilter)
		return 1
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
		return 1
	}

	for _, mf := range mfs {
		name := *mf.Name
		help := *mf.Help
		var metricLines []string

		for _, m := range mf.Metric {
			var labelStrs []string
			for _, lp := range m.Label {
				labelStrs = append(labelStrs, fmt.Sprintf("%s=\"%s\"", *lp.Name, *lp.Value))
			}
			labelBlock := ""
			if len(labelStrs) > 0 {
				labelBlock = "{" + strings.Join(labelStrs, ",") + "}"
			}
			if !MatchesMetricFilter(re, name, labelBlock, help) {
				continue
			}

			if m.Gauge != nil {
				metricLines = append(metricLines, fmt.Sprintf("%s%s %v", name, labelBlock, *m.Gauge.Value))
			} else if m.Counter != nil {
				metricLines = append(metricLines, fmt.Sprintf("%s%s %v", name, labelBlock, *m.Counter.Value))
			} else if m.Histogram != nil {
				for _, b := range m.Histogram.Bucket {
					bLabels := append([]string{}, labelStrs...)
					bLabels = append(bLabels, fmt.Sprintf("le=\"%v\"", *b.UpperBound))
					metricLines = append(metricLines, fmt.Sprintf("%s_bucket{%s} %v", name, strings.Join(bLabels, ","), *b.CumulativeCount))
				}
				infLabels := append([]string{}, labelStrs...)
				infLabels = append(infLabels, "le=\"+Inf\"")
				metricLines = append(metricLines, fmt.Sprintf("%s_bucket{%s} %v", name, strings.Join(infLabels, ","), *m.Histogram.SampleCount))
				metricLines = append(metricLines, fmt.Sprintf("%s_sum%s %v", name, labelBlock, *m.Histogram.SampleSum))
				metricLines = append(metricLines, fmt.Sprintf("%s_count%s %v", name, labelBlock, *m.Histogram.SampleCount))
			}
		}

		if len(metricLines) > 0 {
			fmt.Printf("# HELP %s %s\n", name, help)
			fmt.Printf("# TYPE %s %s\n", name, strings.ToLower(mf.Type.String()))
			for _, line := range metricLines {
				fmt.Println(line)
			}
		}
	}
	return 0
}
