# ubnt-exporter

[![Release](https://img.shields.io/badge/version-v1.1.2-blue.svg)](VERSION)
[![Go Version](https://img.shields.io/badge/go-1.27.2-00ADD8.svg)](go.mod)
[![Docker Image](https://img.shields.io/badge/docker-debian%3A13.7--slim-2496ED.svg)](Dockerfile)
[![Prometheus](https://img.shields.io/badge/prometheus-3.15%20ready-E6522C.svg)](https://prometheus.io)
[![License](https://img.shields.io/badge/license-GPL--2.0-green.svg)](LICENSE)

A high-performance Prometheus exporter for Ubiquiti AirMAX (M/AC), LTU, and AirFiber wireless networking devices, written in Go.

---

## Overview

The `ubnt-exporter` gathers operational telemetry and radio metrics from Ubiquiti wireless infrastructure running AirOS v8 (AirMAX AC), AirOS v6 (AirMAX M), LTU, and AirFiber 5XHD. Engineered for wireless internet service providers (WISPs) and enterprise networks, it concurrently queries hundreds of access points and backhaul links with low CPU and memory footprints.

```text
+-----------------------+           +-----------------------+           +-----------------------+
|  Prometheus Server    |  Scrape   |     ubnt-exporter     |   REST    | Ubiquiti Radios       |
|                       | --------> |                       | --------> |  - AirMAX AC/M        |
|  - /metrics (Daemon)  |           |  - Concurrency Pool   |   HTTPS   |  - AirFiber 5XHD      |
|  - /probe (On-demand) |           |  - Token Persist      |           |  - LTU Series         |
+-----------------------+           +-----------------------+           +-----------------------+
```

### Highlights

- **Lightweight Runtime Footprint:** Static Go binary consuming approximately 10 to 15 MB of RAM under load, packaged in an immutable Debian 13.7 Trixie Slim container image.
- **Broad Ubiquiti Hardware & TLS Compatibility:** Built-in support for both modern TLS ECDHE and legacy RSA cipher suites (`TLS_RSA_WITH_AES_*`) with TLS 1.0+ fallback, providing seamless HTTPS scraping for legacy AirMAX M-series radios (Rocket M5, NanoStation M5 on AirOS v6) as well as AirOS v8, LTU, and AirFiber devices.
- **Worker Pool Concurrency:** Semaphore-regulated goroutine pool scaling smoothly across large inventories without socket starvation.
- **AirOS REST Session Persistence:** Authenticates via `/api/v1.0/user/login`, reuses session tokens across cycles, and recovers transparently from expired sessions.
- **Zero-Contention Metric Storage:** Thread-safe buffered data structures ensure non-blocking writes during high-frequency scrape cycles.
- **OpenMetrics 2.0 & Prometheus 3.15 Ready:** Full content negotiation, Unix Domain Socket (`unix:///path.sock`) scraping, and standard `ubnt_build_info` telemetry.
- **Integrated Exporter Self-Telemetry:** Native OpenMetrics self-observability exposing module latency histograms (`ubnt_exporter_collector_duration_seconds`), execution counters, fleet availability ratios, and Go runtime stats (`go_*`, `process_*`).
- **Production-Ready Alertmanager 0.34+ Stack:** Turnkey alert rules ([`examples/alerts.yaml.example`](examples/alerts.yaml.example)) and Alertmanager routing templates ([`examples/alertmanager.yaml.example`](examples/alertmanager.yaml.example)) featuring hierarchical inhibition (AP/CPE storm suppression), Slack message editing for RF link flapping, and Telegram HTML delivery.
- **Dynamic Configuration Reloading:** Listens for `SIGHUP` signals to reload logging levels and runtime configuration on the fly.
- **Comprehensive SIEM & Cloud-Native Logging:** Native support for structured JSON lines (Kubernetes, Loki, Elastic), ArcSight CEF, IETF Syslog (RFC 5424), BSD Syslog, and formatted text output.
- **Asynchronous Request Tracing & Context Correlation:** Correlates log entries across concurrent daemon cycles (`cycle-N`) and on-demand `/probe` targets with `X-Request-ID` propagation across all logging formats.
- **Anti-Storm Log Dampening:** Built-in throttling mechanism silences repetitive identical error floods during network outages while emitting periodic summary counts and auto-resetting upon device recovery.
- **Resilient Startup & Fatal Error Channeling:** Early logger initialization guarantees structured error outputs on startup failures; asynchronous server routines channel critical listener errors directly to the runtime control loop for graceful termination.
- **Collector DEBUG Event Tracing:** Detailed telemetry diagnosis pinpoints omitted, malformed, or firmware-unsupported fields during troubleshooting cycles without silent discarding.

---

## Documentation

Comprehensive documentation is organized in the [`docs/`](docs/) directory:

- [**Configuration Guide**](docs/configuration.md)
  Dual-file architecture (`config.yaml` and `system.yaml`), `--cfg-dir` search precedence, external secrets management (`secrets.yaml`), probe security allowlists, and custom dimensional labels.
- [**Prometheus Integration & Exporter Guide**](docs/exporter.md)
  Continuous daemon vs. multi-target probe modes, OpenMetrics 2.0 content negotiation, full metric catalog, internal self-telemetry specifications, PromQL alert rules ([`examples/alerts.yaml.example`](examples/alerts.yaml.example)), Alertmanager 0.34+ Slack & Telegram integration ([`examples/alertmanager.yaml.example`](examples/alertmanager.yaml.example)), and production deployment manifests (Systemd, Docker Compose, Kubernetes).
- [**Diagnostics & Troubleshooting Guide**](docs/diagnostics.md)
  CLI verification (`check`), configuration inspection (`show`), dry-run preview (`export`), terminal diagnostics (`diag`), RF chain imbalance analysis, degraded Ethernet port detection, and logging levels.

---

## Quick Start

### 1. Using Docker Compose (Recommended)

1. Clone the repository and build the container image:

```bash
docker build -t ubnt-exporter:latest .
```

2. Prepare your configuration files:

```bash
cp examples/config.yaml.example config.yaml
cp examples/system.yaml.example system.yaml
```

3. Create your local `docker-compose.yml` (see [Docker Compose deployment](docs/exporter.md#docker-compose)):

```yaml
services:
  ubnt-exporter:
    image: ubnt-exporter:latest
    container_name: ubnt-exporter
    restart: unless-stopped
    ports:
      - "8000:8000"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./system.yaml:/app/system.yaml:ro
    command: ["run"]
```

4. Edit `config.yaml` with your radio IP addresses and credentials, then launch the service:

```bash
docker compose up -d
```

5. Verify metrics output:

```bash
# Verify Ubiquiti wireless and system metrics
curl -s http://localhost:8000/metrics | grep ubnt_

# Verify exporter self-telemetry and module performance
curl -s http://localhost:8000/metrics | grep ubnt_exporter_
```

### 2. Using Docker CLI

```bash
docker run -d \
  --name ubnt-exporter \
  --restart unless-stopped \
  -p 8000:8000 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v $(pwd)/system.yaml:/app/system.yaml:ro \
  ubnt-exporter:latest run
```

### 3. Running from Source or Binary

```bash
# Validate configuration files
ubnt-exporter check -c config.yaml -s system.yaml

# Run the metrics daemon
ubnt-exporter run
```

---

## Configuration Quick View

The exporter provides production-grade configuration and alerting templates in the dedicated [`examples/`](examples/) directory:

> [!NOTE]
> **Consolidated Templates Directory (`examples/`):**
> All configuration files and alert rule templates are organized in their own dedicated directory [`examples/`](examples/):
> - [`examples/config.yaml.example`](examples/config.yaml.example): Radio inventory (AirMAX AC/M, LTU, AirFiber 5XHD).
> - [`examples/system.yaml.example`](examples/system.yaml.example): Daemon runtime, timeouts, worker pools, and logging formats.
> - [`examples/alerts.yaml.example`](examples/alerts.yaml.example): Turnkey Prometheus 3.15+ alerting rules.
> - [`examples/alertmanager.yaml.example`](examples/alertmanager.yaml.example): Alertmanager 0.34+ routing, Slack & Telegram templates.

1. `config.yaml`: Device inventory, credentials, labels, and collector toggles (see [`examples/config.yaml.example`](examples/config.yaml.example)).
2. `system.yaml`: Port, scrape intervals, worker limits, and logging (see [`examples/system.yaml.example`](examples/system.yaml.example)).
3. `alerts.yaml`: Prometheus 3.15+ alerting rules for RF quality, hardware, and daemon self-health (see [`examples/alerts.yaml.example`](examples/alerts.yaml.example)).
4. `alertmanager.yaml`: Alertmanager 0.34+ notification routing, Slack/Telegram templates, and hierarchical inhibition (see [`examples/alertmanager.yaml.example`](examples/alertmanager.yaml.example)).

### Minimal `config.yaml`

```yaml
defaults:
  username: "monitoring_user"
  password: "SecureDevicePassword123"
  ssl: false
  labels:
    region: "us-east"
    network: "core"

devices:
  - name: "AP-SECTOR-01"
    ip: "10.0.0.1"
    labels:
      tower: "tower-north"
      sector: "alpha"

enabled_collectors:
  wireless: true
  airfiber: true
  system: true
  interfaces: true
  peers: true
```

### Minimal `system.yaml`

```yaml
server:
  port: 8000
  scrape_interval: 30
  listen: "0.0.0.0:8000"

performance:
  max_workers: 100
  request_timeout: 15
  login_timeout: 5
  socket_timeout: 2
  failure_backoff: 120

logging:
  level: "INFO"
  syslog_facility: "local0"
  formats:
    cef: true
  console: true
```

> [!IMPORTANT]
> Always place device connection properties (`username`, `password`, `ssl`, and `enabled_collectors`) directly under each device entry, not under `labels:`. For full details, see the [Configuration Guide](docs/configuration.md#yaml-indentation-and-parameter-placement).

---

## Internal Self-Telemetry & Observability

The `ubnt-exporter` provides native operational self-monitoring adhering to Prometheus 3.15 and OpenMetrics standards. When scraping `/metrics`, the exporter serves both Ubiquiti telemetry and daemon self-observability metrics:

| Metric Name | Type | Description |
| :--- | :--- | :--- |
| `ubnt_exporter_collector_duration_seconds` | Histogram | Latency histogram for each collector module (`system`, `interfaces`, `wireless`, `airfiber`, `peers`). |
| `ubnt_exporter_collector_success` | Gauge | Status of the most recent collector execution (`1` = success, `0` = failure or caught panic). |
| `ubnt_exporter_collector_executions_total` | Counter | Cumulative executions partitioned by `collector` and `status` (`success`, `error`). |
| `ubnt_exporter_scrape_requests_total` | Counter | Scrape attempts partitioned by result (`success`, `timeout`, `conn_error`, `auth_error`, `http_error`, `decode_error`). |
| `ubnt_exporter_scrape_cycle_duration_seconds` | Gauge | Execution time of the most recent inventory scrape cycle in seconds. |
| `ubnt_exporter_last_scrape_timestamp_seconds` | Gauge | Unix timestamp of the last completed scrape cycle. |
| `ubnt_exporter_devices_configured_total` | Gauge | Total target devices defined in `config.yaml`. |
| `ubnt_exporter_devices_online_total` | Gauge | Number of devices responding successfully in the current cycle. |
| `go_*` / `process_*` | Various | Standard Go runtime memory, GC pauses, goroutine count, and process CPU/RSS metrics. |

For PromQL alert examples and dashboard queries, see the [Prometheus Integration Guide](docs/exporter.md#exporter-internal-telemetry).

---

## Diagnostic CLI & Interactive Container Console

The `ubnt-exporter` executable features a modular command-line interface ([`cli/`](cli/)) providing live diagnostics, schema verification, zero-downtime hot reload, log buffer inspection, and safe linear-time RE2 regular expression queries directly from the host or inside a running container.

### Global Command-Line Flags

Flags can be specified in any order (before or after the command):

| Flag | Short | Default | Description |
| :--- | :---: | :---: | :--- |
| `--config <path>` | `-c` | `"config.yaml"` | Path to the inventory configuration file. |
| `--system <path>` | `-s` | `"system.yaml"` | Path to the daemon runtime configuration file. |
| `--cfg-dir <path>` | — | — | Directory containing both `config.yaml` and `system.yaml`. |
| `--target <pattern>` | `-t` | — | Filter target device by name, IP, custom label, or RE2 regex. |
| `--regex <pattern>` | `-r` | — | Filter metrics or log lines using semantic presets or RE2 patterns. |
| `--lines <int>` | `-n` | `50` | Number of historical log lines to display in `logs tail`. |
| `--version` | `-v` | — | Print exporter version (`1.1.2`) and exit. |
| `--log` | `-log` | — | Display the logging reference manual and exit. |
| `--help` | `-h` | — | Show contextual help for the application or any subcommand. |

### CLI Subcommands Overview

| Command | Purpose | Network Activity |
| :--- | :--- | :--- |
| `run` | Starts the production continuous metrics daemon (`/metrics`, `/probe`, `/logs`). | Continuous daemon |
| `cli` (or `shell`) | Opens an interactive administrative console (`ubnt-cli>`) inside the container. | Interactive REPL |
| `check` | Validates YAML syntax, Prometheus label schemas, collector names, and timeouts. | Offline (none) |
| `show` | Dumps merged in-memory configuration with masked passwords and tokens. | Offline (none) |
| `reload` | Validates configuration files and sends `SIGHUP` to the active daemon process. | Local IPC (`SIGHUP`) |
| `test` (or `selftest`) | Runs 4 built-in verification suites (`metrics`, `collectors`, `connectivity`, `endpoint`). | Local & network |
| `query` (or `q`) | Queries live metrics using semantic presets or RE2 regular expressions. | Active network |
| `metrics` | Searches the 205 Prometheus metric definitions by name, label, or description. | Offline (none) |
| `logs` | Tails live daemon logs from the in-memory ring buffer or tests formatters. | Local loopback `/logs` |
| `diag` | Formats an ASCII link quality table (Signal, Noise, SNR, Chain Delta, Capacity). | Active network |
| `inspect` | Performs a deep telemetry scrape of a device grouped into 4 functional domains. | Active network |
| `export` | Performs a one-shot dry-run scrape and prints Prometheus exposition output. | Active network |

### Real-World CLI Examples

```bash
# 1. Interactive administrative console inside the running Docker container
docker exec -it ubnt-exporter ubnt-exporter cli

# 2. Syntax & semantic configuration validation before starting or reloading
docker exec ubnt-exporter ubnt-exporter check

# 3. Inspect merged runtime configuration with masked passwords
docker exec ubnt-exporter ubnt-exporter show

# 4. Safe zero-downtime hot configuration reload (validates files and sends SIGHUP)
docker exec ubnt-exporter ubnt-exporter reload

# 5. Execute all 4 self-test suites (metrics dictionary, collectors, TLS connectivity, endpoint)
docker exec ubnt-exporter ubnt-exporter test
docker exec ubnt-exporter ubnt-exporter test connectivity -t MST4

# 6. Query live metrics using semantic presets (signal, cpu, clients, rf, traffic, gps, errors)
docker exec ubnt-exporter ubnt-exporter query signal -t MST4
docker exec ubnt-exporter ubnt-exporter query cpu -t "^MST.*"

# 7. Query live metrics using dynamic RE2 regular expressions
docker exec ubnt-exporter ubnt-exporter query -r "ubnt_peer_(rx|tx)_bytes|snr" -t MST4

# 8. Search the static Prometheus metric dictionary by RE2 pattern
docker exec ubnt-exporter ubnt-exporter metrics -r "gps|ntp|vlan"

# 9. Tail recent live daemon logs with line limit and regex filter
docker exec ubnt-exporter ubnt-exporter logs tail -n 20 -r "ERROR|WARN"
docker exec ubnt-exporter ubnt-exporter logs status
docker exec ubnt-exporter ubnt-exporter logs test

# 10. Audit RF link quality, SNR, and chain imbalance (>5 dB warnings highlighted)
docker exec ubnt-exporter ubnt-exporter diag -t "^MST.*"

# 11. Deep collector-by-collector inspection grouped by subsystem
docker exec ubnt-exporter ubnt-exporter inspect -t MST4 -r "ath0\.99"

# 12. Dry-run one-shot export to stdout for verification or debugging
docker exec ubnt-exporter ubnt-exporter export -t MST4
```

---

## Unit Testing & Verification

The project maintains a dedicated top-level test suite located in the [`test/`](test/) directory. Tests are strictly categorized into domain-specific subdirectories, with descriptive filenames that reflect their operational purpose:

```text
test/
├── cmd/
│   └── cli_test.go                  # Command-line flags, configuration check/show, and CLI subcommands
├── collector/
│   ├── airfiber_collector_test.go   # AirFiber 5XHD radio, capacity, and delta metrics processing
│   ├── interfaces_collector_test.go # Ethernet interface counters, bridge ports, and duplex speeds
│   ├── mock_sink_test.go            # Thread-safe mock metric sink for isolated collector unit testing
│   ├── peers_collector_test.go      # Connected station metrics, CPE chain balance, and signal levels
│   ├── system_collector_test.go     # Host telemetry, CPU, RAM, and uptime collector testing
│   ├── timed_runner_test.go         # Timed execution wrapper, telemetry tracking, and panic recovery
│   └── wireless_collector_test.go   # AirMAX wireless parameters, frequency, and channel width
├── config/
│   ├── config_loader_test.go        # Dual-file loading, default values, directory overrides, and secrets
│   └── config_validator_test.go     # IP validation, label discovery, address parsing, and error checks
├── definitions/
│   └── definitions_test.go          # Metric dictionary integrity, Prometheus prefixes, and build metadata
├── logger/
│   └── logger_test.go               # Multi-format outputs (CEF, RFC5424, Syslog, Text) and log file rotation
├── metrics/
│   ├── store_test.go                # In-memory metric store, buffered atomic commits, and build info gauge
│   └── telemetry_test.go            # Daemon self-observability metrics and collector telemetry recording
├── scraper/
│   └── device_scraper_test.go       # HTTP device scraper, mock AirOS REST server, and scrape error flows
└── utils/
    └── utils_test.go                # Type safety helpers, device family detection, and string utilities
```

### Running Tests

Execute the full unit test suite from the repository root:

```bash
# Run all unit tests
go test -v ./test/...

# Run all unit tests with code coverage
go test -v -cover ./test/...

# Run tests with package coverage against internal packages
go test -v -coverpkg=./internal/... ./test/...
```

### Running Tests in Docker

To run the complete test suite inside a clean, reproducible container matching the build environment:

```bash
docker run --rm -v "$(pwd)":/app -w /app golang:1.27.2-bookworm go test -v -buildvcs=false ./test/...
```

---

## Security

To report a vulnerability, please use GitHub Private Vulnerability Reporting as described in the [Security Policy](.github/SECURITY.md). Do not open public issues for security problems. The policy also documents the exporter security model and recommended deployment hardening.

---

## License

This project is licensed under the GNU General Public License v2.0. See [LICENSE](LICENSE) for details.
