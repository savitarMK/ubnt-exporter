# ubnt-exporter

[![Release](https://img.shields.io/badge/version-v1.1.1-blue.svg)](VERSION)
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

The exporter provides production-grade configuration and alerting templates in the [`examples/`](examples/) directory:

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

## Diagnostic CLI Commands

The `ubnt-exporter` executable provides diagnostic utilities for terminal workflows:

```bash
# Validate syntax and check for duplicate IPs or indentation errors
ubnt-exporter check

# Inspect merged configuration with masked credentials
ubnt-exporter show

# Execute a one-shot dry-run scrape and output Prometheus metrics to stdout
ubnt-exporter export

# Display live wireless link diagnostics in an ASCII summary table
ubnt-exporter diag

# View logging manual and severity levels
ubnt-exporter -log
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
