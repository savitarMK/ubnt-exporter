package cli

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/definitions"
	"ubnt-exporter/internal/logger"
	"ubnt-exporter/internal/metrics"
	"ubnt-exporter/internal/scraper"
)

// RegexPreset defines a static, pre-compiled RE2 regular expression for common telemetry domains.
// Static patterns use regexp.MustCompile at initialization to fail fast at startup and avoid
// recompiling fixed expressions on every query cycle.
type RegexPreset struct {
	Name        string
	Aliases     []string
	Description string
	Pattern     string
	Re          *regexp.Regexp
}

// SemanticRegexPresets holds pre-compiled RE2 expressions for natural-language CLI queries.
var SemanticRegexPresets = []RegexPreset{
	{
		Name:        "signal",
		Aliases:     []string{"signal", "señal", "senal", "snr", "noise", "ruido", "rssi", "chains"},
		Description: "Signal level (dBm), noise floor, SNR, MIMO chains, CINR, and chain imbalance",
		Pattern:     `(?i)(signal|noise|snr|rssi|chain|cinr|imbalance|isolation)`,
		Re:          regexp.MustCompile(`(?i)(signal|noise|snr|rssi|chain|cinr|imbalance|isolation)`),
	},
	{
		Name:        "cpu",
		Aliases:     []string{"cpu", "sistema", "system", "ram", "memoria", "memory", "uptime", "temp"},
		Description: "CPU usage, RAM usage, uptime, temperatures, power time, and hardware info",
		Pattern:     `(?i)(cpu|ram|memory|uptime|temp|power_time|static_info|timestamp)`,
		Re:          regexp.MustCompile(`(?i)(cpu|ram|memory|uptime|temp|power_time|static_info|timestamp)`),
	},
	{
		Name:        "clients",
		Aliases:     []string{"clients", "clientes", "peers", "estaciones", "stations", "cpe"}, //nolint:misspell // Spanish alias
		Description: "Connected station count and per-client/peer RF and traffic telemetry",
		Pattern:     `(?i)(wireless_connected_peers|peer_)`,
		Re:          regexp.MustCompile(`(?i)(wireless_connected_peers|peer_)`),
	},
	{
		Name:        "rf",
		Aliases:     []string{"rf", "capacidad", "capacity", "airtime", "ccq", "wireless", "radio", "frecuencia"},
		Description: "Link capacity (DL/UL), Airtime, CCQ, TX power, frequency, channel width, and modulation",
		Pattern:     `(?i)(capacity|airtime|ccq|tx_power|frequency|channel|rate|modulation|distance|polling|wlan|efficiency|duplex)`,
		Re:          regexp.MustCompile(`(?i)(capacity|airtime|ccq|tx_power|frequency|channel|rate|modulation|distance|polling|wlan|efficiency|duplex)`),
	},
	{
		Name:        "traffic",
		Aliases:     []string{"traffic", "trafico", "tráfico", "interfaces", "iface", "vlan", "eth"},
		Description: "Network interface counters (RX/TX bytes, packets), physical link state, and VLANs",
		Pattern:     `(?i)(iface_|throughput)`,
		Re:          regexp.MustCompile(`(?i)(iface_|throughput)`),
	},
	{
		Name:        "gps",
		Aliases:     []string{"gps", "ntp", "ubicacion", "coordenadas", "satelites", "time"}, //nolint:misspell // Spanish alias
		Description: "GPS coordinates, altitude, satellites, DOP, 3D fix, and local/remote NTP sync",
		Pattern:     `(?i)(gps|latitude|longitude|altitude|dop|satellites|ntp)`,
		Re:          regexp.MustCompile(`(?i)(gps|latitude|longitude|altitude|dop|satellites|ntp)`),
	},
	{
		Name:        "errors",
		Aliases:     []string{"errors", "errores", "drops", "fallos", "crc", "fcs"},
		Description: "Interface and wireless packet errors, drops, CRC/FCS failures, retries, and missed beacons",
		Pattern:     `(?i)(error|drop|crc|fcs|collision|carrier|fail|invalid|missed|bmiss|retries)`,
		Re:          regexp.MustCompile(`(?i)(error|drop|crc|fcs|collision|carrier|fail|invalid|missed|bmiss|retries)`),
	},
}

var presetLookup = func() map[string]RegexPreset {
	m := make(map[string]RegexPreset)
	for _, p := range SemanticRegexPresets {
		for _, alias := range p.Aliases {
			m[strings.ToLower(alias)] = p
		}
	}
	return m
}()

// CompileUserRegex resolves a static semantic alias (via pre-compiled MustCompile presets)
// or dynamically compiles a user-supplied RE2 regular expression via regexp.Compile.
// Dynamic compilation captures syntax errors cleanly without panicking.
func CompileUserRegex(expr string) (*regexp.Regexp, string, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return nil, "", nil
	}

	if preset, ok := presetLookup[strings.ToLower(trimmed)]; ok {
		return preset.Re, fmt.Sprintf("preset:%s (%s)", preset.Name, preset.Pattern), nil
	}

	pattern := trimmed
	if !strings.HasPrefix(pattern, "(?") {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		if strings.Contains(trimmed, "(?=") || strings.Contains(trimmed, "(?!") ||
			strings.Contains(trimmed, "(?<=") || strings.Contains(trimmed, "(?<!") {
			return nil, "", fmt.Errorf("invalid RE2 regular expression %q: %v (Go uses RE2 linear-time syntax to prevent ReDoS and does not support lookarounds like (?=...) or (?!...))", trimmed, err)
		}
		return nil, "", fmt.Errorf("invalid RE2 regular expression %q: %v", trimmed, err)
	}
	return re, pattern, nil
}

// FilterScrapersByTarget filters device scrapers by exact name/IP match or by a dynamic RE2 regular expression
// evaluated against device name, IP, and custom labels (e.g. tower="tower-b").
func FilterScrapersByTarget(scrapers []*scraper.DeviceScraper, targetPattern string) ([]*scraper.DeviceScraper, error) {
	trimmed := strings.TrimSpace(targetPattern)
	if trimmed == "" {
		return scrapers, nil
	}

	// First check exact match on Name or IP
	var exact []*scraper.DeviceScraper
	for _, sc := range scrapers {
		if strings.EqualFold(sc.Name, trimmed) || sc.IP == trimmed {
			exact = append(exact, sc)
		}
	}
	if len(exact) > 0 {
		return exact, nil
	}

	// Dynamic RE2 compilation for pattern matching on Name, IP, or ExtraLabels
	re, _, err := CompileUserRegex(trimmed)
	if err != nil {
		return nil, err
	}

	var matched []*scraper.DeviceScraper
	for _, sc := range scrapers {
		if re.MatchString(sc.Name) || re.MatchString(sc.IP) {
			matched = append(matched, sc)
			continue
		}
		for k, v := range sc.CustomLabels {
			if re.MatchString(k+"="+v) || re.MatchString(v) {
				matched = append(matched, sc)
				break
			}
		}
	}
	return matched, nil
}

// MatchesMetricFilter evaluates whether a metric series matches the compiled RE2 expression
// by checking the metric name, formatted label block, or HELP description.
func MatchesMetricFilter(re *regexp.Regexp, metricName, labelBlock, helpText string) bool {
	if re == nil {
		return true
	}
	return re.MatchString(metricName) || re.MatchString(labelBlock) || re.MatchString(helpText)
}

// PrintRegexPresets displays the built-in semantic aliases and RE2 syntax examples.
func PrintRegexPresets() {
	fmt.Println("\n BUILT-IN SEMANTIC QUERY PRESETS & RE2 REGEXP GUIDE")
	fmt.Println("==========================================================================================")
	fmt.Printf("  %-12s | %-34s | %s\n", "Preset", "Aliases (ES / EN)", "Telemetry Matched")
	fmt.Println("------------------------------------------------------------------------------------------")
	for _, p := range SemanticRegexPresets {
		fmt.Printf("  %-12s | %-34s | %s\n", p.Name, strings.Join(p.Aliases, ", "), p.Description)
	}
	fmt.Println("------------------------------------------------------------------------------------------")
	fmt.Println("RE2 Syntax Examples:")
	fmt.Println("  ubnt-exporter query señal -t MST4")
	fmt.Println("  ubnt-exporter query cpu")
	fmt.Println("  ubnt-exporter query clientes") //nolint:misspell // Spanish example
	fmt.Println("  ubnt-exporter query -r \"signal|snr|ccq\" -t \"^MST.*\"")
	fmt.Println("  ubnt-exporter inspect -t MST4 -r \"interface=\\\"ath0\\.99\\\"\"")
	fmt.Println("  ubnt-exporter metrics -r \"gps|ntp\"")
	fmt.Println("  ubnt-exporter logs tail -n 30 -r \"ERROR|WARN|login\"")
	fmt.Println()
}

// CmdQuery scrapes matching devices and filters live metrics using semantic presets or RE2 regular expressions.
func CmdQuery(sysConf *config.SystemConfig, invConf *config.InventoryConfig, queryExpr string, targetFilter string) int {
	trimmed := strings.TrimSpace(queryExpr)
	if trimmed == "" || strings.EqualFold(trimmed, "presets") || strings.EqualFold(trimmed, "list") {
		PrintRegexPresets()
		return 0
	}

	re, resolvedDesc, err := CompileUserRegex(trimmed)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
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
		fmt.Printf("No matching devices found in inventory for target %q.\n", targetFilter)
		return 1
	}

	fmt.Printf("\n LIVE TELEMETRY QUERY: %s\n", resolvedDesc)
	fmt.Println("==========================================================================================")

	client := scraper.NewHTTPClient(time.Duration(sysConf.Performance.SocketTimeout) * time.Second)
	ctx := logger.WithTraceID(context.Background(), "query")

	var wg sync.WaitGroup
	for _, sc := range filtered {
		wg.Add(1)
		go func(s *scraper.DeviceScraper) {
			defer wg.Done()
			s.Scrape(ctx, client, store, true)
		}(sc)
	}
	wg.Wait()

	reg := prometheus.NewRegistry()
	reg.MustRegister(metrics.NewUbiquitiCollector(store))
	mfs, gErr := reg.Gather()
	if gErr != nil {
		fmt.Printf("Error gathering metrics: %v\n", gErr)
		return 1
	}

	matchedFamilies := 0
	matchedSeries := 0

	for _, mf := range mfs {
		name := mf.GetName()
		help := mf.GetHelp()
		var lines []string

		for _, m := range mf.GetMetric() {
			var lbls []string
			for _, lp := range m.GetLabel() {
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
			lines = append(lines, fmt.Sprintf("  %-42s %-40s = %v", name, labelBlock, val))
		}

		if len(lines) > 0 {
			matchedFamilies++
			matchedSeries += len(lines)
			sort.Strings(lines)
			fmt.Printf("\n# %s — %s\n", name, help)
			for _, l := range lines {
				fmt.Println(l)
			}
		}
	}

	if matchedSeries == 0 {
		fmt.Printf("\nNo active metric series matched expression %q across %d device(s).\n", trimmed, len(filtered))
		return 0
	}

	fmt.Printf("\n------------------------------------------------------------------------------------------\n")
	fmt.Printf("Matched %d series across %d metric definitions (Devices queried: %d)\n\n", matchedSeries, matchedFamilies, len(filtered))
	return 0
}

// CmdMetricsCatalog searches the static dictionary of 205 Prometheus metric definitions using RE2.
func CmdMetricsCatalog(pattern string) int {
	re, resolvedDesc, err := CompileUserRegex(pattern)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return 1
	}

	fmt.Println("\n PROMETHEUS METRICS DICTIONARY EXPLORER")
	if resolvedDesc != "" {
		fmt.Printf(" Filter: %s\n", resolvedDesc)
	}
	fmt.Println("==========================================================================================")

	var keys []string
	for k := range definitions.MetricsDef {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	matched := 0
	for _, k := range keys {
		def := definitions.MetricsDef[k]
		fullName := def.Name
		lblStr := strings.Join(def.Labels, ",")
		if re != nil && !re.MatchString(k) && !re.MatchString(fullName) && !re.MatchString(def.Help) && !re.MatchString(lblStr) {
			continue
		}
		matched++
		fmt.Printf("  %-44s | %-7s | labels=[%s]\n", fullName, def.Type, lblStr)
		fmt.Printf("    HELP: %s\n", def.Help)
	}

	fmt.Println("------------------------------------------------------------------------------------------")
	fmt.Printf("Total matching metric definitions: %d / %d\n\n", matched, len(definitions.MetricsDef))
	return 0
}

// PrintCommandHelp displays contextual help when -h / --help is passed with a specific command.
func PrintCommandHelp(cmd string) {
	switch strings.ToLower(strings.TrimSpace(cmd)) {
	case "query", "q":
		fmt.Println("USAGE: ubnt-exporter query [preset|regex] [-r regex] [-t target_regex]")
		fmt.Println("\nQueries live telemetry from devices using semantic presets or RE2 regular expressions.")
		PrintRegexPresets()
	case "inspect":
		fmt.Println("USAGE: ubnt-exporter inspect [-t target_regex] [-r metric_regex]")
		fmt.Println("\nPerforms a live scrape of a device and groups all collected series by collector domain.")
		fmt.Println("Examples:")
		fmt.Println("  ubnt-exporter inspect -t MST4")
		fmt.Println("  ubnt-exporter inspect -t MST4 -r \"signal|snr|ccq\"")
		fmt.Println("  ubnt-exporter inspect -t MST4 -r \"interface=\\\"ath0\\.99\\\"\"")
	case "diag":
		fmt.Println("USAGE: ubnt-exporter diag [-t target_regex]")
		fmt.Println("\nDisplays an interactive RF link quality table (Signal, Noise, SNR, Chain Delta, Clients, Capacity).")
		fmt.Println("Examples:")
		fmt.Println("  ubnt-exporter diag")
		fmt.Println("  ubnt-exporter diag -t \"^MST.*\"")
		fmt.Println("  ubnt-exporter diag -t \"tower-b\"")
	case "export":
		fmt.Println("USAGE: ubnt-exporter export [-t target_regex] [-r metric_regex]")
		fmt.Println("\nPerforms a one-shot scrape and prints Prometheus exposition format output.")
		fmt.Println("Examples:")
		fmt.Println("  ubnt-exporter export -t MST4")
		fmt.Println("  ubnt-exporter export -t MST4 -r \"ubnt_peer_.*\"")
	case "logs":
		fmt.Println("USAGE: ubnt-exporter logs [tail|test|status] [-n lines] [-r regex]")
		fmt.Println("\nInspects live daemon logs from the in-memory ring buffer or tests log formatters.")
		fmt.Println("Examples:")
		fmt.Println("  ubnt-exporter logs tail -n 30")
		fmt.Println("  ubnt-exporter logs tail -n 50 -r \"ERROR|WARN|login\"")
		fmt.Println("  ubnt-exporter logs test")
		fmt.Println("  ubnt-exporter logs status")
	case "test", "selftest":
		fmt.Println("USAGE: ubnt-exporter test [all|metrics|collectors|connectivity|endpoint] [-t target_regex]")
		fmt.Println("\nRuns built-in verification and unit test suites inside the container.")
	case "metrics":
		fmt.Println("USAGE: ubnt-exporter metrics [-r regex]")
		fmt.Println("\nSearches the 205 Prometheus metric definitions by name, label, or HELP description.")
		fmt.Println("Examples:")
		fmt.Println("  ubnt-exporter metrics -r \"gps|ntp\"")
		fmt.Println("  ubnt-exporter metrics -r \"signal\"")
	default:
		PrintHelp()
	}
}
