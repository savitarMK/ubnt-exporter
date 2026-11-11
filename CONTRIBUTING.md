**Looking for help?** Have a question about deploying `ubnt-exporter`, tuning worker pools, writing PromQL alert rules, or building Grafana dashboards? Start a conversation in [GitHub Discussions](https://github.com/savitarMK/ubnt-exporter/discussions)!

<div align="center">
  <h3>
    🐛 <a href="#bug-reporting-bugs">Report a Bug</a> &middot;
    ✨ <a href="#sparkles-feature-requests">Suggest a Feature</a> &middot;
    📡 <a href="#satellite-contributing-hardware-fixtures">Submit a Hardware Fixture</a>
  </h3>
  <h3>
    🚀 <a href="#rocket-submitting-pull-requests">Submit a Pull Request</a> &middot;
    📊 <a href="#bar_chart-prometheus-3x-metric-standards">Prometheus 3.x Metric Standards</a> &middot;
    🛠️ <a href="#hammer_and_wrench-local-development--offline-replay">Offline Development Guide</a>
  </h3>
</div>

---

## ℹ️ Project Overview & Contribution Philosophy

`ubnt-exporter` is a high-performance Prometheus 3.x and OpenMetrics 2.0 exporter written in Go (`1.27.4`) for Ubiquiti wireless infrastructure—including **AirMAX AC (airOS v8)**, **AirMAX M-Series (airOS v6 1x1 SISO / 2x2 MIMO)**, **LTU**, and **AirFiber 5XHD**.

Designed for Wireless Internet Service Providers (WISPs), telecom operators, and enterprise networks, every contribution must prioritize:
1. **Zero-Contention Performance & Low Footprint:** Non-blocking concurrent scraping and atomic in-memory metric updates.
2. **Strict Prometheus 3.x & OpenMetrics 2.0 Compliance:** Accurate metric types (`Gauge`, `Counter`, `Histogram`, `Info`), Hybrid Native Histograms, explicit base units, and complete `# HELP` descriptions.
3. **Hardware-Free Reproducibility:** Full verification against anonymized `/api/v1.0/statistics` JSON fixtures (`testdata/fixtures/`) using the `-f` / `--from-file` replay engine.
4. **Strict Data Sanitization:** Zero real passwords, tokens, or public IP addresses in code, fixtures, or documentation (all examples and captures must use RFC 1918 `192.168.1.x` addresses).

> [!CAUTION]
> All issues must be submitted using one of our [official GitHub Issue Templates](https://github.com/savitarMK/ubnt-exporter/issues/new/choose). Blank issues are disabled to ensure every bug report, feature request, performance observation, or hardware fixture submission contains the technical context required for rapid triage.

> [!IMPORTANT]
> **Security Vulnerabilities:** Do **not** report security vulnerabilities in public GitHub issues. Please review our [Security Policy](.github/SECURITY.md) for responsible disclosure instructions.

---

## 🐛 Reporting Bugs

Bug reports call attention to reproducible defects in `ubnt-exporter`, such as collector parsing errors, inaccurate metric values, TLS handshake failures on legacy AirOS firmware, or CLI issues.

1. **Verify on the Latest Release:** Ensure you are running the [latest release](https://github.com/savitarMK/ubnt-exporter/releases) of `ubnt-exporter`.
2. **Run Built-in Diagnostics First:**
   ```bash
   # Validate configuration files and schema rules
   ubnt-exporter check

   # Run built-in metric dictionary and collector self-tests
   ubnt-exporter test metrics
   ubnt-exporter test collectors
   ```
3. **Search Existing Issues:** Check the [issues list](https://github.com/savitarMK/ubnt-exporter/issues) to see if the defect has already been reported. If so, add a 👍 reaction and any additional firmware or hardware context in a comment.
4. **Submit a Complete Bug Report:** Open a [Bug Report](https://github.com/savitarMK/ubnt-exporter/issues/new?template=02-bug_report.yaml) and include:
   - Exact `ubnt-exporter -v`, Go runtime, and Prometheus versions.
   - Target Ubiquiti hardware model and firmware version (e.g., `Rocket Prism 5AC Gen2 WA.v8.7.19`, `AirGrid M5 HP XW.v6.3.6`, `AF-5XHD AF09.v1.5.1`).
   - Anonymized `/api/v1.0/statistics` JSON snippet if the bug involves missing or miscalculated metrics.

---

## ✨ Feature Requests

1. **Check Existing Metrics First:** Run `ubnt-exporter metrics` (or search [`docs/exporter.md`](docs/exporter.md)) to verify whether the metric or capability is already available among the **159 exported metrics**.
2. **Submit a Structured Proposal:** Use the [Feature Request template](https://github.com/savitarMK/ubnt-exporter/issues/new?template=01-feature_request.yaml) and specify:
   - The target hardware family (`AirMAX AC`, `AirMAX M-Series`, `AirFiber / LTU`, or global).
   - Proposed Prometheus metric names (`ubnt_ap_*`, `ubnt_peer_*`, `ubnt_af_*`, `ubnt_iface_*`, `ubnt_device_*`), metric types (`Gauge`, `Counter`, `Histogram`, `Info`), and base units (`dbm`, `db`, `mhz`, `mbps`, `bps`, `percent`, `seconds`, `bytes`).
   - How the proposed feature improves WISP/telecom observability, alerting, or capacity planning.

---

## 📡 Contributing Hardware Fixtures

Because Ubiquiti firmware families (`airOS v6`, `airOS v8`, `LTU`, `AirFiber`) exhibit subtle JSON schema variations (such as 1x1 SISO `signalPerChain: [-57, 0]` sentinels, `ath0/0` split interfaces, or `identification.product` fallbacks), real device JSON captures are one of the most valuable contributions to `ubnt-exporter`.

To contribute a new hardware fixture:
1. Capture the raw JSON output from `/api/v1.0/statistics` on the target radio.
2. **Anonymize all sensitive fields:**
   - Replace all IP addresses with RFC 1918 `192.168.1.x` addresses.
   - Replace hostnames with generic identifiers (`AP-SECTOR-01`, `CPE-CLIENT-01`, `AF5XHD-MASTER-01`).
   - Anonymize MAC addresses and GPS coordinates if they identify a private site.
3. Verify the capture locally without network access:
   ```bash
   ./ubnt-exporter inspect -f your_capture.json
   ./ubnt-exporter export -f your_capture.json
   ./ubnt-exporter test collectors -f your_capture.json
   ```
4. Submit it via the [Hardware & Firmware Support template](https://github.com/savitarMK/ubnt-exporter/issues/new?template=05-hardware_support.yaml) or include it in a pull request under `testdata/fixtures/`.

---

## 📊 Prometheus 3.x Metric Standards

All metric definitions live in [`internal/definitions/definitions.go`](internal/definitions/definitions.go) and are enforced by `ubnt-exporter test metrics` and `test/definitions/definitions_test.go`. Every metric must adhere to these rules:

| Rule | Standard | Example |
| :--- | :--- | :--- |
| **Namespace Prefix** | Global AP radio/GPS metrics use `ubnt_ap_*`; remote station metrics use `ubnt_peer_*`; AirFiber-exclusive TDD/split-frequency metrics use `ubnt_af_*`; interfaces use `ubnt_iface_*`; hardware/system metrics use `ubnt_device_*`. | `ubnt_ap_gps_satellites`, `ubnt_peer_signal_dbm`, `ubnt_af_frequency_tx_mhz` |
| **Counter Suffix (`_total`)** | Monotonically increasing `TypeCounter` metrics **must** end with `_total`. Non-counter metrics (`TypeGauge`, `TypeInfo`) **must never** end with `_total`. | `ubnt_iface_rx_bytes_total` (`Counter`) vs. `ubnt_ap_connected_clients` (`Gauge`) |
| **Info Suffix (`_info`)** | Static metadata metrics (`TypeInfo`, constant value `1`) **must** end with `_info`. | `ubnt_device_static_info`, `ubnt_peer_static_info`, `ubnt_iface_info` |
| **Base Unit Suffix** | Metric names and `Unit` fields must reflect their unit (`_seconds`, `_bytes`, `_bps`, `_mbps`, `_pps`, `_percent`, `_dbm`, `_db`, `_dbi`, `_mhz`, `_meters`, `_celsius`, `_ms`). | `ubnt_peer_cpu_usage_percent`, `ubnt_ap_uptime_seconds` |
| **`# HELP` Quality** | Every `Help` string must be a complete, self-contained sentence in American English (at least 20 characters) ending with a period (`.`), explicitly stating the unit or state encoding (`1 = ..., 0 = ...`). | `"Local AP primary wireless radio link state (1 = connected, 0 = disconnected)."` |

---

## 🛠️ Local Development & Offline Replay

You do not need physical Ubiquiti radios to develop or test `ubnt-exporter`.

```bash
# 1. Clone and build the binary
git clone https://github.com/savitarMK/ubnt-exporter.git
cd ubnt-exporter
go build -o ubnt-exporter ./cmd/ubnt-exporter

# 2. Replay bundled hardware fixtures offline
./ubnt-exporter diag -f testdata/fixtures/airmax_ac_ptmp.json
./ubnt-exporter inspect -f testdata/fixtures/airmax_m_ptmp.json
./ubnt-exporter export -f testdata/fixtures/af5xhd_ptp.json

# 3. Run all quality, race detector, and vulnerability checks
go mod verify
go vet ./...
go test -v -race -coverpkg=./internal/...,./cli/... ./test/...
./ubnt-exporter test metrics
./ubnt-exporter test collectors
```

---

## 🚀 Submitting Pull Requests

1. **Branching:** Create your feature or bugfix branch from `main` (or the active `release/v*` branch when targeting an upcoming release).
2. **Pull Request Template & Release Classification:** Complete all sections of [`.github/PULL_REQUEST_TEMPLATE.md`](.github/PULL_REQUEST_TEMPLATE.md), including the `release-note` block tagged with one of the official Prometheus classification prefixes:
   - `[CHANGE]` — Breaking schema changes, metric renames, or collector deduplication.
   - `[FEATURE]` — New metrics, collectors, CLI commands, or exporter features.
   - `[ENHANCEMENT]` — Improvements to existing collectors, metadata, or diagnostics.
   - `[PERF]` — Concurrency, memory, or latency optimizations.
   - `[BUGFIX]` — Reproducible bug fixes and firmware edge-case handling.
3. **Automated CI & Release Pipeline:**
   - Every pull request is validated by [`.github/workflows/security.yml`](.github/workflows/security.yml) (`govulncheck`, `gitleaks`, `go vet`, `go test -race`, built-in self-tests, and `Trivy`).
   - When maintainers push a version tag (`v*.*.*`), [`.github/workflows/release.yml`](.github/workflows/release.yml) automatically updates `CHANGELOG.md`, cross-compiles binaries for Linux/macOS/Windows (`amd64`, `arm64`, `armv7`), generates `sha256sums.txt`, publishes the GitHub Release, and pushes multi-arch container images to `ghcr.io`.
