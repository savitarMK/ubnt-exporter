package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"ubnt-exporter/internal/config"
)

// CmdCheck validates system.yaml and config.yaml and prints a diagnostic report.
func CmdCheck(sysConf *config.SystemConfig, sysErr error, invConf *config.InventoryConfig, invErr error) int {
	fmt.Println("\n CONFIGURATION VALIDATION")
	fmt.Println("==============================")

	var allErrs []string
	if sysErr != nil {
		allErrs = append(allErrs, fmt.Sprintf("system.yaml error: %v", sysErr))
	}
	if invErr != nil {
		allErrs = append(allErrs, fmt.Sprintf("config.yaml error: %v", invErr))
	}

	if sysConf != nil && invConf != nil {
		valErrs := config.ValidateConfig(sysConf, invConf)
		allErrs = append(allErrs, valErrs...)
	}

	if len(allErrs) > 0 {
		fmt.Println("RESULT: Configuration has ERRORS")
		for _, e := range allErrs {
			fmt.Printf(" - %s\n", e)
		}
		return 1
	}

	dynamicKeys := config.DiscoverCustomLabels(invConf)
	fmt.Println("RESULT: Configuration is VALID")
	fmt.Println("\n System Settings Verified:")
	listenAddrs, _ := config.ParseListenAddresses(sysConf.Server.Listen, sysConf.Server.Port)
	fmt.Printf("    Server Listen Targets: %v\n", listenAddrs)
	fmt.Printf("    Scrape Interval: %ds\n", sysConf.Server.ScrapeInterval)
	fmt.Printf("    Max Workers: %d\n", sysConf.Performance.MaxWorkers)
	fmt.Printf("    Timeouts (Login/Req/Socket): %ds / %ds / %ds\n", sysConf.Performance.LoginTimeout, sysConf.Performance.RequestTimeout, sysConf.Performance.SocketTimeout)
	fmt.Printf("    Failure Backoff: %ds\n", sysConf.Performance.FailureBackoff)
	fmt.Printf("    Log Level: %s | Syslog Facility: %s\n", sysConf.Logging.Level, sysConf.Logging.SyslogFacility)
	fmt.Printf("    Devices in Inventory: %d\n", len(invConf.Devices))
	fmt.Printf("    Dynamic Labels Found: %v\n", dynamicKeys)
	return 0
}

// CmdShow outputs the active configuration with masked passwords.
func CmdShow(sysConf *config.SystemConfig, invConf *config.InventoryConfig) {
	fmt.Println("ACTIVE CONFIGURATION")

	sysCopy := *sysConf
	invCopy := *invConf

	if invCopy.Defaults.Password != "" {
		invCopy.Defaults.Password = "******"
	}

	sanitizedDevices := make([]config.DeviceConfig, len(invConf.Devices))
	for i, d := range invConf.Devices {
		sanitized := d
		if sanitized.Password != "" {
			sanitized.Password = "******"
		}
		sanitizedDevices[i] = sanitized
	}
	invCopy.Devices = sanitizedDevices

	sysBytes, _ := yaml.Marshal(sysCopy)
	invBytes, _ := yaml.Marshal(invCopy)

	fmt.Print(string(sysBytes))
	fmt.Print(string(invBytes))
}

var (
	procFS       = "/proc"
	signalDaemon = func(pid int, sig os.Signal) error {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		return proc.Signal(sig)
	}
)

// SetProcFSForTesting overrides the proc filesystem path for unit tests.
func SetProcFSForTesting(path string) func() {
	orig := procFS
	procFS = path
	return func() { procFS = orig }
}

// SetSignalDaemonForTesting overrides daemon signal delivery for unit tests.
func SetSignalDaemonForTesting(fn func(pid int, sig os.Signal) error) func() {
	orig := signalDaemon
	signalDaemon = fn
	return func() { signalDaemon = orig }
}

func findRunningDaemonPID() (int, error) {
	selfPID := os.Getpid()
	entries, err := os.ReadDir(procFS)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", procFS, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, pErr := strconv.Atoi(entry.Name())
		if pErr != nil || pid <= 0 || pid == selfPID {
			continue
		}
		cmdBytes, cErr := os.ReadFile(filepath.Join(procFS, entry.Name(), "cmdline"))
		if cErr != nil || len(cmdBytes) == 0 {
			continue
		}
		parts := strings.Split(strings.TrimRight(string(cmdBytes), "\x00"), "\x00")
		if len(parts) == 0 {
			continue
		}
		binBase := filepath.Base(parts[0])
		if strings.Contains(binBase, "ubnt-exporter") {
			for _, arg := range parts[1:] {
				if arg == "run" {
					return pid, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("no running 'ubnt-exporter run' daemon process found")
}

// CmdReload validates both configuration files and sends SIGHUP to the active daemon process.
func CmdReload(configPath, systemPath string, sysConf *config.SystemConfig, sysErr error, invConf *config.InventoryConfig, invErr error) int {
	fmt.Println("\n HOT CONFIGURATION RELOAD (SIGHUP)")
	fmt.Println("==========================================")
	fmt.Printf("Target Files: %s | %s\n", configPath, systemPath)

	if code := CmdCheck(sysConf, sysErr, invConf, invErr); code != 0 {
		fmt.Println("\nABORTED: Fix configuration errors before reloading the daemon.")
		return code
	}

	pid, err := findRunningDaemonPID()
	if err != nil {
		fmt.Printf("\nERROR: Could not locate running daemon: %v\n", err)
		return 1
	}

	if err := signalDaemon(pid, syscall.SIGHUP); err != nil {
		fmt.Printf("\nERROR: Failed to send SIGHUP to PID %d: %v\n", pid, err)
		return 1
	}

	fmt.Printf("\nSIGHUP signal sent to ubnt-exporter daemon (PID %d).\n", pid)
	time.Sleep(400 * time.Millisecond)

	if sysConf != nil {
		lines, lErr := fetchLocalDaemonLogs(sysConf, 5, "")
		if lErr == nil && len(lines) > 0 {
			fmt.Println("\nRecent Daemon Log Entries:")
			for _, l := range lines {
				fmt.Printf("  %s\n", l)
			}
		}
	}
	return 0
}
