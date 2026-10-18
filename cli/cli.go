// Package cli implements the integrated command-line interface, interactive shell,
// configuration validators, self-diagnostics, log inspector, and daemon controller.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"ubnt-exporter/internal/config"
	"ubnt-exporter/internal/definitions"
	"ubnt-exporter/internal/logger"
)

const loggingHelpText = `DEBUG
  - Purpose: Full visibility. Traces HTTP requests, headers, and individual steps.
  - Use: Development, debugging connections, or empty metrics.
  - Example: "DEBUG | MST4 [cycle-1] | GET https://10.1.1.5/api/v1.0/statistics returned HTTP 200"

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

// Options holds parsed CLI flags and command arguments.
type Options struct {
	Command    string
	Subcommand string
	ConfigPath string
	SystemPath string
	CfgDir     string
	Target     string
	Regex      string
	Lines      int
}

// PrintHelp displays the main CLI usage manual.
func PrintHelp() {
	fmt.Printf("Ubiquiti Exporter v%s\n\n", definitions.VERSION)
	fmt.Println("USAGE:")
	fmt.Println("  ubnt-exporter [flags] <command> [subcommand]")
	fmt.Println("\nAVAILABLE COMMANDS:")
	fmt.Println("  run       Starts the metrics server (Daemon).")
	fmt.Println("  check     Validates system.yaml and config.yaml against production standards.")
	fmt.Println("  show      Dumps active configuration with masked secrets.")
	fmt.Println("  reload    Validates configuration and sends SIGHUP to the running daemon.")
	fmt.Println("  test      Runs built-in self-tests (metrics, collectors, connectivity, endpoint).")
	fmt.Println("  logs      Inspects live daemon logs or tests log formatting (tail, test, status).")
	fmt.Println("  query     Queries live telemetry using semantic presets (signal, cpu, clients) or RE2 regexp.")
	fmt.Println("  metrics   Searches the 205 Prometheus metric definitions by RE2 regexp.")
	fmt.Println("  diag      Interactive terminal diagnostics and RF link quality table.")
	fmt.Println("  inspect   Deep per-collector telemetry breakdown for a target device.")
	fmt.Println("  export    One-shot scrape for debugging and Prometheus metrics preview.")
	fmt.Println("  cli       Opens the interactive administrative shell inside the container.")
	fmt.Println("\nFLAGS:")
	fmt.Println("  -c, --config string   Path to config.yaml (default \"config.yaml\")")
	fmt.Println("  -s, --system string   Path to system.yaml (default \"system.yaml\")")
	fmt.Println("  --cfg-dir string      Directory containing config.yaml and system.yaml")
	fmt.Println("  -t, --target string   Filter device by name, IP, label, or RE2 regexp (for diag, inspect, export, query, test, logs)")
	fmt.Println("  -r, --regex string    RE2 regular expression or preset filter (for query, inspect, export, metrics, logs)")
	fmt.Println("  -n, --lines int       Number of log lines to display in 'logs tail' (default 50)")
	fmt.Println("  -v, --version         Show application version")
	fmt.Println("  -log, --log           Show logging manual and exit")
	fmt.Println("  -h, --help            Show global or command-specific help")
	fmt.Println("\nEXAMPLES:")
	fmt.Println("  $ ubnt-exporter run")
	fmt.Println("  $ ubnt-exporter check")
	fmt.Println("  $ ubnt-exporter test")
	fmt.Println("  $ ubnt-exporter query signal -t MST4")
	fmt.Println("  $ ubnt-exporter query -r \"cpu|ram|temp\"")
	fmt.Println("  $ ubnt-exporter metrics -r \"gps|ntp\"")
	fmt.Println("  $ ubnt-exporter logs tail -n 30 -r \"ERROR|WARN\"")
	fmt.Println("  $ ubnt-exporter reload")
	fmt.Println("  $ ubnt-exporter diag -t \"^MST.*\"")
	fmt.Println("  $ ubnt-exporter inspect -t MST4 -r \"ath0\\.99\"")
	fmt.Println("  $ ubnt-exporter cli")
}

// ParseArgs parses command-line flags and positional arguments in any order.
func ParseArgs(args []string) (Options, int, bool) {
	opts := Options{
		ConfigPath: "config.yaml",
		SystemPath: "system.yaml",
		Lines:      50,
	}

	showHelp := false
	showVersion := false
	showLogManual := false

	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-v", "--version":
			showVersion = true
		case "-log", "--log":
			showLogManual = true
		case "-h", "--help", "help":
			showHelp = true
		case "-c", "--config":
			if i+1 < len(args) {
				opts.ConfigPath = args[i+1]
				i++
			}
		case "-s", "--system":
			if i+1 < len(args) {
				opts.SystemPath = args[i+1]
				i++
			}
		case "--cfg-dir":
			if i+1 < len(args) {
				opts.CfgDir = args[i+1]
				i++
			}
		case "-t", "--target":
			if i+1 < len(args) {
				opts.Target = args[i+1]
				i++
			}
		case "-r", "--regex", "--regexp", "-f", "--filter":
			if i+1 < len(args) {
				opts.Regex = args[i+1]
				i++
			}
		case "-n", "--lines":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					opts.Lines = n
				}
				i++
			}
		default:
			if strings.HasPrefix(arg, "-c=") {
				opts.ConfigPath = strings.TrimPrefix(arg, "-c=")
			} else if strings.HasPrefix(arg, "--config=") {
				opts.ConfigPath = strings.TrimPrefix(arg, "--config=")
			} else if strings.HasPrefix(arg, "-s=") {
				opts.SystemPath = strings.TrimPrefix(arg, "-s=")
			} else if strings.HasPrefix(arg, "--system=") {
				opts.SystemPath = strings.TrimPrefix(arg, "--system=")
			} else if strings.HasPrefix(arg, "--cfg-dir=") {
				opts.CfgDir = strings.TrimPrefix(arg, "--cfg-dir=")
			} else if strings.HasPrefix(arg, "-t=") {
				opts.Target = strings.TrimPrefix(arg, "-t=")
			} else if strings.HasPrefix(arg, "--target=") {
				opts.Target = strings.TrimPrefix(arg, "--target=")
			} else if strings.HasPrefix(arg, "-r=") {
				opts.Regex = strings.TrimPrefix(arg, "-r=")
			} else if strings.HasPrefix(arg, "--regex=") {
				opts.Regex = strings.TrimPrefix(arg, "--regex=")
			} else if strings.HasPrefix(arg, "--regexp=") {
				opts.Regex = strings.TrimPrefix(arg, "--regexp=")
			} else if strings.HasPrefix(arg, "-n=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(arg, "-n=")); err == nil && n > 0 {
					opts.Lines = n
				}
			} else if strings.HasPrefix(arg, "--lines=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--lines=")); err == nil && n > 0 {
					opts.Lines = n
				}
			} else if !strings.HasPrefix(arg, "-") {
				positionals = append(positionals, arg)
			}
		}
	}

	if len(positionals) > 0 {
		opts.Command = positionals[0]
	}
	if len(positionals) > 1 {
		opts.Subcommand = positionals[1]
	}
	if len(positionals) > 2 && opts.Target == "" {
		opts.Target = positionals[2]
	}

	if showVersion {
		fmt.Printf("ubnt-exporter %s\n", definitions.VERSION)
		return opts, 0, true
	}
	if showLogManual {
		fmt.Print(loggingHelpText)
		return opts, 0, true
	}
	if showHelp {
		if opts.Command != "" {
			PrintCommandHelp(opts.Command)
		} else {
			PrintHelp()
		}
		return opts, 0, true
	}

	return opts, 0, false
}

// Execute parses arguments and dispatches the requested CLI command.
func Execute(args []string) int {
	logger.Setup(config.DefaultSystemConfig())

	opts, exitCode, done := ParseArgs(args)
	if done {
		return exitCode
	}

	if opts.Command == "" {
		PrintHelp()
		return 0
	}

	switch opts.Command {
	case "run", "check", "show", "reload", "export", "diag", "inspect", "query", "q", "metrics", "test", "selftest", "logs", "cli", "shell":
	default:
		logger.Error("main", "Unknown command: %s", opts.Command)
		PrintHelp()
		return 1
	}

	if opts.Command == "metrics" {
		pattern := opts.Regex
		if pattern == "" {
			pattern = opts.Subcommand
		}
		if pattern == "" {
			pattern = opts.Target
		}
		return CmdMetricsCatalog(pattern)
	}

	opts.ConfigPath, opts.SystemPath = config.ResolveConfigPaths(opts.CfgDir, opts.ConfigPath, opts.SystemPath)

	sysConf, sysErr := config.LoadSystemConfig(opts.SystemPath)
	if sysConf != nil {
		logger.Setup(sysConf)
	}
	invConf, invErr := config.LoadInventoryConfig(opts.ConfigPath)

	switch opts.Command {
	case "check":
		return CmdCheck(sysConf, sysErr, invConf, invErr)
	case "reload":
		return CmdReload(opts.ConfigPath, opts.SystemPath, sysConf, sysErr, invConf, invErr)
	case "logs":
		if sysErr != nil {
			logger.Error("main", "Failed to load system configuration (%s): %v", opts.SystemPath, sysErr)
			return 1
		}
		logFilter := opts.Regex
		if logFilter == "" {
			logFilter = opts.Target
		}
		return CmdLogs(opts.SystemPath, sysConf, opts.Subcommand, opts.Lines, logFilter)
	case "cli", "shell":
		return CmdShell(opts.ConfigPath, opts.SystemPath)
	}

	if sysErr != nil || invErr != nil {
		if sysErr != nil {
			logger.Error("main", "Failed to load system configuration (%s): %v", opts.SystemPath, sysErr)
		}
		if invErr != nil {
			logger.Error("main", "Failed to load inventory configuration (%s): %v", opts.ConfigPath, invErr)
		}
		logger.Error("main", "Could not load configuration files. Run 'ubnt-exporter check'")
		return 1
	}

	switch opts.Command {
	case "run":
		CmdRun(opts.ConfigPath, opts.SystemPath, sysConf, invConf)
		return 0
	case "show":
		CmdShow(sysConf, invConf)
		return 0
	case "export":
		return CmdExport(sysConf, invConf, opts.Target, opts.Regex)
	case "diag":
		return CmdDiag(sysConf, invConf, opts.Target)
	case "inspect":
		target := opts.Target
		if target == "" {
			target = opts.Subcommand
		}
		return CmdInspect(sysConf, invConf, target, opts.Regex)
	case "query", "q":
		expr := opts.Regex
		if expr == "" {
			expr = opts.Subcommand
		}
		return CmdQuery(sysConf, invConf, expr, opts.Target)
	case "test", "selftest":
		return CmdTest(sysConf, invConf, opts.Subcommand, opts.Target)
	default:
		return 1
	}
}

// CmdShell starts an interactive REPL session for container diagnostics and administration.
func CmdShell(configPath, systemPath string) int {
	fmt.Printf("\n======================================================================\n")
	fmt.Printf("  UBNT-EXPORTER v%s — INTERACTIVE DIAGNOSTIC & ADMIN CONSOLE\n", definitions.VERSION)
	fmt.Printf("  Type 'help' for available commands or 'exit' to leave.\n")
	fmt.Printf("======================================================================\n\n")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("ubnt-cli> ")
		if !scanner.Scan() {
			fmt.Println()
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := normalizeShellFields(strings.Fields(line))
		if len(fields) == 0 {
			continue
		}
		cmd := strings.ToLower(fields[0])

		switch cmd {
		case "exit", "quit":
			fmt.Println("Exiting ubnt-cli.")
			return 0
		case "clear", "cls":
			fmt.Print("\033[H\033[2J")
			continue
		case "?":
			PrintHelp()
			continue
		case "help":
			if len(fields) > 1 {
				PrintCommandHelp(fields[1])
			} else {
				PrintHelp()
			}
			continue
		case "run":
			fmt.Println("The 'run' daemon command cannot be started inside the interactive shell.")
			continue
		case "cli", "shell":
			fmt.Println("Already inside the interactive ubnt-cli console.")
			continue
		}

		execArgs := append([]string{"-c", configPath, "-s", systemPath}, fields...)
		_ = Execute(execArgs)
		fmt.Println()
	}
	return 0
}

// normalizeShellFields strips leading "$", "docker exec ... ubnt-exporter", or "ubnt-exporter"
// prefixes when users paste full host commands directly into the interactive ubnt-cli> prompt.
func normalizeShellFields(fields []string) []string {
	if len(fields) > 0 && fields[0] == "$" {
		fields = fields[1:]
	}
	if len(fields) >= 3 && fields[0] == "docker" && fields[1] == "exec" {
		idx := 2
		for idx < len(fields) && strings.HasPrefix(fields[idx], "-") {
			idx++
		}
		if idx < len(fields) {
			// Skip container name
			idx++
		}
		fields = fields[idx:]
	}
	if len(fields) > 0 && (fields[0] == "ubnt-exporter" || strings.HasSuffix(fields[0], "/ubnt-exporter")) {
		fields = fields[1:]
	}
	return fields
}
