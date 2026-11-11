# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html), and uses explicit Prometheus release classification tags (`[CHANGE]`, `[FEATURE]`, `[ENHANCEMENT]`, `[PERF]`, `[BUGFIX]`).

## [Unreleased]

### Security

- **`[SECURITY]` Go toolchain 1.27.3 → 1.27.4**: Fixes GO-2026-6859 (HTTP/1 server crash due to `MaxBytesReader` header race in `net/http`), reported by `govulncheck` as reachable from the exporter. Updated `go.mod`, `Dockerfile` builder image (`golang:1.27.4-bookworm`), CI workflows, and documentation.

### Added

- **`[FEATURE]` `/healthz` liveness endpoint and `ubnt-exporter health` command**: Lightweight endpoint that does not trigger metric collection. The `health` command resolves the daemon address from `system.yaml` (TCP or Unix domain socket) and exits 0/1.

### Changed

- **`[PERF]` Disabled log levels skip formatting**: The severity level is now stored atomically and `Debug`/`Info`/`Warn`/`Error` return before `fmt.Sprintf` when the level is filtered out, avoiding formatting cost for the many debug statements in the scrape path.

- **`[CHANGE]` Container `HEALTHCHECK` uses `ubnt-exporter health`**: Previously `curl -f http://localhost:8000/metrics`, which ran a full metric collection every 30s and reported the container unhealthy whenever `server.port`/`server.listen` differed from `8000` or only a Unix socket was configured.

- **`[CHANGE]` New `peer_mac` identity label on all `ubnt_peer_*` metrics** (breaking label-set change): Station series were identified only by `peer_name` + `peer_ip`; CPEs without hostname/management IP (reported as `Unknown`) or with duplicated values collapsed into a single series, silently overwriting each other. All 81 per-station metrics (including `ubnt_peer_mcs_rx_packets` / `ubnt_peer_remote_mcs_rx_packets`) now carry `peer_mac` (lower-case). `ubnt_peer_static_info` keeps `peer_mac`, now normalized to lower-case and set to `unknown-<index>` instead of `Unknown` when the station reports no MAC. Queries aggregating by `peer_name` keep working; dashboards or recording rules matching the exact label set may need updating, and existing station series are replaced by new ones after upgrade (`internal/definitions/definitions.go`, `internal/collector/peers.go`).

### Fixed

- **`[BUGFIX]` `server.port` ignored when `server.listen` is omitted**: The built-in `listen: "0.0.0.0:8000"` default was pre-seeded before decoding `system.yaml`, so a file setting only `port: 9100` still bound to `:8000`. `listen` is now derived from `port` when absent (`internal/config/config.go`).
- **`[BUGFIX]` Enabling a single non-CEF log format failed validation**: The CEF default was pre-seeded before decoding, so `formats: { json: true }` resulted in two active formats and a *"multiple logging formats enabled"* error. CEF is now applied only when no format is enabled.
- **`[BUGFIX]` IPv6 devices could never be scraped**: The device base URL was built as `http://fd00::1/api/v1.0` (invalid). IPv6 literals are now bracketed (`http://[fd00::1]/api/v1.0`, RFC 3986). Leading/trailing whitespace in `ip` (accepted by validation) is also trimmed before building the URL and the `ip`/`name` labels (`internal/scraper/device.go`).
- **`[BUGFIX]` `/probe` responses cut off for slow devices**: The worst-case probe scrape (login + statistics + re-authentication, up to 40s with defaults) exceeded the server `WriteTimeout` of 10s, so the response was dropped. The write deadline is now extended per probe request (`cli/probe.go`).
- **`[BUGFIX]` Unbounded concurrent `/probe` scrapes**: `MaxRequestsInFlight` applied to a new handler created on every request, so it never limited anything. Probes are now capped at 10 concurrent requests globally, answering `503` with `Retry-After` when saturated.
- **`[BUGFIX]` Unbounded log dampening table (slow memory leak)**: Dampening keys include the full message text, so messages with variable content (durations, error strings, IPs) added a new entry on every occurrence that was never removed. The table is now capped at 1024 entries, pruning expired windows first and then the least recently emitted record.
- **`[BUGFIX]` CEF extension escaping**: Extension values (`msg`, `sproc`, `shost`, `cs1`) now escape `\` and `=` and encode newlines as `\n`, per the CEF specification; previously `=` was left unescaped (allowing forged `key=value` pairs in SIEMs) while `|` was wrongly escaped. Header fields escape `\` and `|`.
- **`[BUGFIX]` Invalid RFC 5424 records**: The device name was used verbatim as `MSGID`, breaking the record when it contained spaces or non-ASCII characters; it is now sanitized to 1–32 printable ASCII characters. Structured-data values escape `"`, `\` and `]`.
- **`[BUGFIX]` Log line injection**: Messages containing newlines could forge additional records in the `text`, `syslog` and `rfc5424` formats; newlines are now flattened to spaces (JSON and CEF encode them).
- **`[BUGFIX]` Stale series for devices removed or renamed on SIGHUP reload**: `ubnt_scrape_error_total` and `ubnt_scrape_duration_seconds` were never deleted when a device left the inventory or changed its name/labels, so they were exported forever. Added `Store.PurgeDevice`, used by the reload path; failed scrapes still preserve `scrape_error` via `RemoveDeviceMetrics` (`internal/metrics/store.go`, `cli/daemon.go`).

## [v1.2.0] - 2026-11-10

### Added (`[FEATURE]`)

- **`[FEATURE]` Prometheus 3.x Hybrid Native Histograms**:
  - Enabled high-resolution sparse exponential Native Histograms (`NativeHistogramBucketFactor: 1.1`, `NativeHistogramMaxBucketNumber: 100`, `NativeHistogramMinResetDuration: 1h`) alongside classic fixed `_bucket` series on `ubnt_scrape_duration_seconds` (`internal/metrics/store.go`) and `ubnt_exporter_collector_duration_seconds` (`internal/metrics/telemetry.go`) for full compatibility with Prometheus 3.x and OpenMetrics 2.0.
- **`[FEATURE]` Offline Fixture Replay & Hardware-Free Collector Development (`-f` / `--from-file` / `--fixture`)**:
  - Added `scraper.NewOfflineScraper`, `ScrapeBytes`, and `ScrapeFile` in `internal/scraper/device.go` to execute the complete collector pipeline (`ProcessSystem`, `ProcessInterfaces`, `ProcessWireless`, `ProcessPeers`, and `ProcessAF5XHD`) directly against raw `/api/v1.0/statistics` JSON captures without opening network sockets or authenticating.
  - Added `-f` / `--from-file` / `--fixture` CLI flag support across `diag`, `inspect`, `export`, `query`, and `test collectors` in `cli/` (`cli.go`, `diag_inspect.go`, `query_regex.go`, `selftest.go`), allowing contributors and CI pipelines to develop, inspect, filter, and verify metrics without live hardware or configuration files.
  - Added 3 anonymized, RFC1918-compliant hardware JSON fixtures in `testdata/fixtures/` along with strict collector fixture tests (`test/collector/fixtures_collector_test.go`):
    - `testdata/fixtures/airmax_ac_ptmp.json`: Real Ubiquiti AirMAX AC (`RP-5AC-Gen2` / `WA.v8.7.19`) Point-to-Multipoint Access Point capture (`AP-SECTOR-01` at `192.168.1.10`) with 2x2 MIMO CPE telemetry, per-chain signals, CINR, and MCS rate histograms.
    - `testdata/fixtures/airmax_m_ptmp.json`: Real Ubiquiti AirMAX M-series (`XW.v6.3.6` / `MIPS 74Kc V4.12`) Point-to-Multipoint Access Point capture with 4 associated 1x1 SISO stations (`AirGrid M5 HP` and `LiteBeam M5`, `CPE-M5-CLIENT-01` through `CPE-M5-CLIENT-04` at `192.168.1.201`–`204`), bridge/VLAN sub-interfaces (`br0`, `br1`, `eth0.99`, `ath0.99`), and per-peer CPU/RAM/RF telemetry.
    - `testdata/fixtures/af5xhd_ptp.json`: Real Ubiquiti AirFiber 5XHD (`AF-5XHD` / `AF09.v1.5.1`) Point-to-Point Master capture (`AF5XHD-MASTER-01` at `192.168.1.20`) with full-duplex split frequencies, conducted/EIRP power, GPS synchronization, and LTU modulation histograms.
- **`[FEATURE]` Universal Custom Label Propagation for Grafana Dashboards**:
  - Updated `Store.RegisterDynamicLabels` (`internal/metrics/store.go`) and `DeviceScraper` (`internal/scraper/device.go`) so custom dimensional labels declared in `config.yaml` (`defaults.labels` and `devices[].labels`, such as `region`, `network`, `tower`, `sector`, `pop`, `area`, or `role`) are injected into **every** exported Prometheus metric (`ubnt_*`), not only `ubnt_device_static_info`.
  - Enabled direct Grafana template variable filtering (`label_values(ubnt_scrape_success, pop)`) and native PromQL `sum by (pop) (...)` / `avg by (tower) (...)` aggregations on any metric without requiring complex `on(name, ip) group_left(...)` joins.
  - Enhanced `SIGHUP` hot reload (`cli/daemon.go`) to compare each device's `CustomLabels` map and immediately purge stale label series from the in-memory `Store` whenever a device's custom labels are added, modified, or removed in `config.yaml`.
- **`[FEATURE]` GitOps External Secrets Template & Per-Device Credential Overrides (`secrets.yaml`)**:
  - Added `examples/secrets.yaml.example` and enhanced `LoadInventoryConfig` (`internal/config/config.go`) so `defaults.credentials_file` supports both global default credentials (`username` / `password`) and per-device credential overrides (`devices:` map matched by device `name` or `ip`), keeping `config.yaml` 100% free of plain-text passwords in version control.

### Changed (`[CHANGE]` & `[ENHANCEMENT]`)

- **`[CHANGE]` Prometheus 3.x Strict Metric Type & Suffix Compliance**:
  - Removed the `_total` suffix from `Gauge` metrics (strictly reserved for `Counter` metrics in Prometheus 3.x and OpenMetrics 2.0 `promtool` validation):
    - Renamed `ubnt_connected_clients_total` (`Gauge`) to `ubnt_ap_connected_clients`.
    - Renamed `ubnt_exporter_devices_configured_total` (`Gauge`) to `ubnt_exporter_devices_configured`.
    - Renamed `ubnt_exporter_devices_online_total` (`Gauge`) to `ubnt_exporter_devices_online`.
  - Standardized unit suffixes on remote station resource metrics by renaming `ubnt_peer_cpu_usage` to `ubnt_peer_cpu_usage_percent` and `ubnt_peer_ram_usage` to `ubnt_peer_ram_usage_percent` (`Gauge`), matching `ubnt_ap_cpu_usage_percent` and `ubnt_ap_ram_usage_percent`.
- **`[CHANGE]` Global AP GPS Namespace Unification (`ubnt_ap_gps_*`)**:
  - Migrated all local device GPS telemetry in `internal/definitions/definitions.go` and `internal/collector/system.go` from the legacy `ubnt_af_gps_*` and `ubnt_device_gps_*` prefixes to the unified `ubnt_ap_gps_*` namespace (`ubnt_ap_gps_fix`, `ubnt_ap_gps_satellites`, `ubnt_ap_gps_dop`, `ubnt_ap_gps_altitude_meters`, `ubnt_ap_gps_dimension`, and `ubnt_ap_gps_time_sync_enabled`). AirMAX devices and configurations with `airfiber: false` now emit zero `ubnt_af_*` metrics.
- **`[CHANGE]` AirMAX AC, AirMAX M-Series & AirFiber Collector Deduplication**:
  - Unified the scraping pipeline in `internal/scraper/device.go` so `ProcessAF5XHD` runs purely as an additive collector alongside `ProcessSystem`, `ProcessInterfaces`, `ProcessWireless`, and `ProcessPeers` when `family == "airfiber"` is detected.
  - Removed 46 duplicated metric definitions and redundant JSON extractions from `internal/collector/airfiber.go` and `internal/definitions/definitions.go` (reducing `MetricsDef` from 205 to 159 unique definitions), retaining only the 10 AirFiber-exclusive TDD and split-frequency metrics in `ProcessAF5XHD`.
- **`[CHANGE]` Toolchain & Dependency Modernization (Go 1.27.3)**:
  - Upgraded the project toolchain in `go.mod` and `Dockerfile` to `go 1.27.3` (`golang:1.27.3-bookworm`).
  - Updated `golang.org/x/sys` to `v0.43.0` and `gopkg.in/yaml.v3` to `v3.0.1`, passing `govulncheck ./...` with zero vulnerabilities.
- **`[ENHANCEMENT]` Complete `# HELP`, `# TYPE`, and `Unit` Metadata Across All 159 Metrics**:
  - Added the `Unit` field to `MetricDefinition` (`internal/definitions/definitions.go`, `internal/metrics/store.go`) and rewrote all 159 metric `Help` descriptions with complete, unit-explicit technical sentences ending in periods for Prometheus 3.x and Grafana metric explorer tooltips.
- **`[ENHANCEMENT]` Strict `promtool`-Style Schema Linter in Self-Test Suite**:
  - Enhanced `RunSuiteMetricsWith` (`cli/selftest.go`) and `TestMetricsDefinitionsIntegrity` (`test/definitions/definitions_test.go`) to enforce `_total` exclusivity on `Counter` metrics, `_info` suffixes on `Info` metrics, duplicate metric name detection, and descriptive `Help` sentence validation.
- **`[ENHANCEMENT]` Sub-Interface & IEEE 802.1Q VLAN Classification**:
  - Enhanced `internal/collector/interfaces.go` to classify `ath0/0` (AirFiber split radio) and `ath0.99` / `eth0.99` / `br1` VLAN and bridge interfaces via `interface_type`, `parent_interface`, and `vlan_id` labels on `ubnt_iface_info`.
- **`[ENHANCEMENT]` Documentation, Examples, Issue/PR Templates, and RFC1918 Anonymization**:
  - Updated `README.md`, `CONTRIBUTING.md`, `docs/configuration.md`, `docs/diagnostics.md`, `docs/exporter.md`, `examples/*.example`, `.github/ISSUE_TEMPLATE/`, `.github/PULL_REQUEST_TEMPLATE.md`, and `.github/workflows/release.yml` for Prometheus 3.x compatibility, Hybrid Native Histograms, the 159 unified metrics, universal custom label propagation, GitOps external secrets precedence, hot-reload series lifecycle, and automated release/changelog generation.
  - Standardized all IP addresses, device names, and GPS coordinates across documentation, configuration examples, CLI help messages, fixtures, and unit tests to use generic RFC1918 addresses (`192.168.1.x`) and generic hostnames (`AP1`, `AP-SECTOR-01`, `AP-BACKHAUL-01`, `CPE-CLIENT-01`, `CPE-M5-CLIENT-01`, `AF5XHD-MASTER-01`, `AF5XHD-SLAVE-01`).

### Performance (`[PERF]`)

- **`[PERF]` Zero-Contention Atomic Device Metric Replacement**:
  - Buffered per-device metric collection in `metrics.Buffer` (`internal/metrics/store.go`) and committed device snapshots atomically via `ReplaceDeviceMetrics` under a single mutex lock, eliminating lock contention across concurrent worker goroutines.
- **`[PERF]` Bounded Native Histogram Bucket Allocation**:
  - Configured `NativeHistogramMaxBucketNumber: 100` and `NativeHistogramMinResetDuration: 1h` on exporter latency histograms to bound memory usage during continuous high-frequency scraping.

### Fixed (`[BUGFIX]`)

- **`[BUGFIX]` AirMAX M-Series (AirOS v6) 1x1 SISO Chain Handling & False Imbalance Suppression**:
  - Filtered out `0 dBm` inactive chain sentinels on 1x1 SISO stations (`AirGrid M5 HP`, `LiteBeam M5`, `signalPerChain: [-57, 0]`) in `internal/collector/peers.go` so Chain 1 (`ubnt_peer_signal_chain1_dbm` / `ubnt_peer_remote_signal_chain1_dbm`) is not falsely reported as `0 dBm` and false 50+ dB MIMO `chain_imbalance` alerts are suppressed when the remote station is a 1x1 SISO radio.
- **`[BUGFIX]` AirOS v6 Station Model Fallback (`identification.product`)**:
  - Added fallback for `peer_model` to `identification.product` when `identification.model` is empty (`""`) on AirOS v6 firmware in `internal/collector/peers.go`.
- **`[BUGFIX]` Conditional Control Frequency Emission on AirOS v6**:
  - Guarded `ap_control_frequency` (`ubnt_ap_control_frequency_mhz`) in `internal/collector/wireless.go` so it is only emitted when `frequency.control` is present in the radio payload rather than emitting `0 MHz`.
- **`[BUGFIX]` AirFiber 5XHD Remote Output Power & SNR Fallback**:
  - Fixed `internal/collector/peers.go` and `internal/collector/wireless.go` to extract nested `remote[0].linkQuality.outputPower` and compute `ubnt_ap_snr_db` / `ubnt_peer_snr_db` from `signal - mainRadio.noiseFloor` when `linkQuality.noiseFloor` is `null`.
- **`[BUGFIX]` Config-Less Execution for Offline Self-Tests**:
  - Fixed `cli/cli.go` so `ubnt-exporter test metrics` and `ubnt-exporter test collectors` execute cleanly even when `config.yaml` is absent in the working directory.
