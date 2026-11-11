# Configuration Guide

The `ubnt-exporter` uses a dual-file configuration structure designed to separate target device inventories from daemon server runtime parameters.

---

## Table of Contents

- [Overview](#overview)
- [Configuration Resolution Hierarchy](#configuration-resolution-hierarchy)
  - [Resolution Precedence](#resolution-precedence)
  - [Using the Global `--cfg-dir` Flag](#using-the-global---cfg-dir-flag)
- [Device Inventory (`config.yaml`)](#device-inventory-configyaml)
  - [The `defaults` Section](#the-defaults-section)
  - [The `devices` Section](#the-devices-section)
  - [The `enabled_collectors` Section](#the-enabled_collectors-section)
  - [YAML Indentation and Parameter Placement](#yaml-indentation-and-parameter-placement)
- [External Secrets Management](#external-secrets-management)
  - [Centralized Secrets File (`credentials_file`)](#centralized-secrets-file-credentials_file)
  - [Environment Variables](#environment-variables)
- [Server & Runtime Options (`system.yaml`)](#server--runtime-options-systemyaml)
  - [The `server` Section](#the-server-section)
  - [Multi-Target Probe Security (`probe_allowed_targets`)](#multi-target-probe-security-probe_allowed_targets)
  - [Unix Domain Sockets](#unix-domain-sockets)
  - [The `performance` Section](#the-performance-section)
  - [The `logging` Section](#the-logging-section)
- [Dynamic Configuration Reloading (`SIGHUP`)](#dynamic-configuration-reloading-sighup)
- [Configuration Verification (`check` Command)](#configuration-verification-check-command)

---

## Overview

The configuration model divides settings into two distinct YAML files:

1. **`config.yaml` (Inventory):** Target device hosts, IP addresses, authentication credentials, dimensional Prometheus labels, and enabled collector modules.
2. **`system.yaml` (System):** HTTP listening endpoints, Unix domain sockets, worker pool concurrency, network timeouts, failure backoff, probe allowlists, and logging output formats.

---

## Configuration Resolution Hierarchy

The exporter automatically locates configuration files according to a strict search hierarchy, enabling deployment across standard Linux systems, containers, and user profiles.

### Resolution Precedence

Configuration paths are evaluated in the following order:

1. **Explicit CLI Flags:**

   ```bash
   ubnt-exporter -c /custom/path/config.yaml -s /custom/path/system.yaml run
   ```

2. **Global Config Directory Flag (`--cfg-dir`):**

   ```bash
   ubnt-exporter --cfg-dir /etc/ubnt-exporter run
   ```

   Looks for `/etc/ubnt-exporter/config.yaml` and `/etc/ubnt-exporter/system.yaml`.
3. **Current Working Directory:**
   `./config.yaml` and `./system.yaml`
4. **XDG User Configuration Directory:**
   `$XDG_CONFIG_HOME/ubnt-exporter/` (or `~/.config/ubnt-exporter/`)
5. **System-Wide Directory:**
   `/etc/ubnt-exporter/`

### Using the Global `--cfg-dir` Flag

When managing multiple sites or operational environments, `--cfg-dir` allows switching configuration sets without specifying individual file flags:

```bash
# Production inventory
ubnt-exporter --cfg-dir /etc/ubnt-exporter/prod run

# Staging inventory validation
ubnt-exporter --cfg-dir /etc/ubnt-exporter/staging check
```

---

## Device Inventory (`config.yaml`)

A template is provided in [`examples/config.yaml.example`](../examples/config.yaml.example).

### Complete `config.yaml` Example

```yaml
defaults:
  username: "monitoring_user"
  password: "GlobalSecretPassword123"
  ssl: false
  labels:
    region: "us-east"
    network: "core"

devices:
  # Host inheriting global defaults
  - name: "AP-SECTOR-01"
    ip: "192.168.1.10"
    labels:
      tower: "tower-north"
      sector: "alpha"

  # Host with dedicated credentials, HTTPS, and collector overrides
  - name: "AP-BACKHAUL-01"
    ip: "192.168.1.20"
    ssl: true
    username: "backhaul_admin"
    password: "CustomBackhaulPassword"
    labels:
      tower: "tower-south"
      role: "backhaul"
    enabled_collectors:
      peers: false

enabled_collectors:
  wireless: true
  airfiber: true
  system: true
  interfaces: true
  peers: true
```

### The `defaults` Section

Defines fallback values inherited by all devices that do not specify their own overrides:

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `username` | string | `""` | Default AirOS administrative username. |
| `password` | string | `""` | Default AirOS administrative password. |
| `ssl` | boolean | `false` | Default connection scheme (`true` for HTTPS, `false` for HTTP). Built-in client supports modern TLS ECDHE and legacy RSA cipher suites (`TLS_RSA_WITH_AES_*`) for AirOS v6. |
| `credentials_file` | string | `""` | Path to an external secrets file containing credentials. |
| `labels` | key/value map | `{}` | Global dimensional labels automatically inherited by all devices and injected into **every** exported metric (system, wireless, airfiber, interfaces, peers, and scrape status) for direct Grafana panel filtering and grouping. |

### The `devices` Section

Defines the list of Ubiquiti hardware targets:

| Parameter | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `name` | string | Yes | Unique human-readable identifier (e.g. `AP-TOWER-01`). Must be unique across all devices to prevent Prometheus label collisions and startup validation errors. Emitted as the `name` label in Prometheus metrics. |
| `ip` | string | Yes | IPv4 or IPv6 address of the target device. Emitted as the `ip` label. |
| `username` | string | No | Device-level administrative username override. Inherits from `defaults` if omitted. |
| `password` | string | No | Device-level administrative password override. Inherits from `defaults` if omitted. |
| `ssl` | boolean | No | Device-level SSL toggle (`true` for HTTPS, `false` for HTTP). Compatible with modern AirOS v8 and legacy AirOS v6 TLS cipher suites. Inherits from `defaults` if omitted. |
| `labels` | key/value map | No | Arbitrary dimensional Prometheus labels specific to this device (e.g., `tower`, `sector`, `pop`, `area`, `role`). Automatically discovered and propagated across all 159 metric definitions so Grafana template variables (`$pop`, `$area`, `$tower`, `$sector`) work directly on any metric without PromQL `group_left` joins. |
| `enabled_collectors` | key/value map | No | Per-device collector module overrides (e.g. disabling `peers` on Point-to-Point backhauls). |

### The `enabled_collectors` Section

Enables or disables metric collection modules across the inventory:

| Collector | Default | Description |
| :--- | :--- | :--- |
| `wireless` | `true` | General wireless operational status (AirMAX & AirFiber/LTU): link state, service uptime, noise floor, SNR, frequency, channel width, output power, DFS radar timers, and active client counts. |
| `airfiber` | `true` | AirFiber 5XHD / LTU exclusive TDD and split-frequency telemetry: split TX/RX frequencies, RX channel width, downlink duty-cycle ratio, frame length, local/remote GPS TDD sync state, local RX efficiency, and local/remote auto-power state. |
| `system` | `true` | Target device host telemetry: CPU load percentage, CPU identifier, RAM memory consumption, system/power uptime, internal board temperatures, local GPS coordinates/satellites/DOP, and NTP synchronization. |
| `interfaces` | `true` | Network interface and VLAN statistics (`eth0`, `ath0`, `ath0/0`, `ath0.99`, `br0`): Tx/Rx octets, packets, errors, drops, broadcast, multicast, SNR, link speed, duplex, and parent interface classification. |
| `peers` | `true` | Connected station / peer telemetry: Signal per chain, ideal signal, SNR, remote signal, chain imbalance, capacity (DL/UL/combined/ideal), link score, CINR, MCS rates & histograms, remote GPS/NTP, distance, and hardware info. |

### YAML Indentation and Parameter Placement

> [!IMPORTANT]
> **Correct Placement of Connection Settings**  
> In YAML, indentation establishes hierarchy. Connection properties (`username`, `password`, `ssl`, and `enabled_collectors`) must be indented at the device level (indented 4 spaces, matching `name` and `ip`).  
>
> If these parameters are inadvertently indented under `labels:`, YAML parses them as dimensional Prometheus metric labels rather than connection credentials, causing authentication to fail with `HTTP 401 Unauthorized`.

#### Correct Indentation

```yaml
devices:
  - name: "AP-SECTOR-01"
    ip: "192.168.1.10"
    ssl: true
    username: "admin"
    password: "SecretPassword"
    labels:
      tower: "tower-alpha"
```

#### Incorrect Indentation (Do Not Use)

```yaml
devices:
  - name: "AP-SECTOR-01"
    ip: "192.168.1.10"
    labels:
      tower: "tower-alpha"
      ssl: true                # ERROR: Parsed as a metric label, not a connection property
      username: "admin"        # ERROR: Authentication will fail
```

---

## External Secrets Management

To prevent storing plain-text credentials in version-controlled GitOps inventory files, `ubnt-exporter` supports an external secrets file (`credentials_file`) as well as environment variable injection. A complete reference template is provided in [`examples/secrets.yaml.example`](../examples/secrets.yaml.example).

### Centralized Secrets File (`credentials_file`)

You can reference an external `secrets.yaml` inside `config.yaml`. Relative paths are resolved relative to the directory containing `config.yaml`, while absolute paths (e.g., `/etc/ubnt-exporter/secrets.yaml` or `/run/secrets/ubnt_secrets.yaml`) are used directly:

```yaml
# config.yaml (safe to commit to Git — zero plain-text passwords)
defaults:
  credentials_file: "secrets.yaml"
  ssl: false
  labels:
    region: "us-east"
    network: "core"

devices:
  - name: "AP-SECTOR-01"
    ip: "192.168.1.10"
    labels:
      tower: "tower-north"
      sector: "alpha"

  - name: "AP-BACKHAUL-01"
    ip: "192.168.1.20"
    ssl: true
    labels:
      tower: "tower-south"
      role: "backhaul"
```

The external `secrets.yaml` file (see [`examples/secrets.yaml.example`](../examples/secrets.yaml.example)) supports both **global default credentials** and **optional per-device overrides** keyed by device `name` or `ip`:

```yaml
# secrets.yaml
# 1. Global default credentials (inherited by all devices unless overridden)
username: "admin_user"
password: "ProductionDevicePassword123"

# 2. Optional per-device credential overrides (matched by device 'name' or 'ip')
devices:
  AP-BACKHAUL-01:
    username: "custom_admin"
    password: "CustomPassword123"
  # "192.168.1.30":
  #   username: "legacy_admin"
  #   password: "LegacyPassword456"
```

Restrict file permissions on the host and mount the file read-only into the container:

```bash
chmod 600 /etc/ubnt-exporter/secrets.yaml

docker run -d \
  --name ubnt-exporter \
  -p 8000:8000 \
  -v /etc/ubnt-exporter/config.yaml:/app/config.yaml:ro \
  -v /etc/ubnt-exporter/secrets.yaml:/app/secrets.yaml:ro \
  -v /etc/ubnt-exporter/system.yaml:/app/system.yaml:ro \
  ghcr.io/savitarmk/ubnt-exporter:latest
```

> [!TIP]
> **Zero-Downtime Credential Rotation (`SIGHUP`)**  
> When you rotate passwords in `secrets.yaml` and run `ubnt-exporter reload` (or `kill -HUP <pid>`), the daemon re-reads `secrets.yaml`, detects which devices had their credentials updated, invalidates only those session tokens, and re-authenticates on the next cycle without dropping metrics or restarting the container.

### Credential Resolution Precedence

For each device in `config.yaml`, `username` and `password` are resolved independently in the following priority order (highest to lowest):

| Priority | Source | Description |
| :---: | :--- | :--- |
| **1** | `devices[].username` / `password` (`config.yaml`) | Explicit inline device-level override in `config.yaml`. |
| **2** | `devices[<name> \| <ip>]` (`secrets.yaml`) | Per-device override in `credentials_file` matched by `name` (first) or `ip` (second). |
| **3** | `defaults.username` / `password` (`config.yaml`) | Explicit inline global default in `config.yaml`. |
| **4** | `username` / `password` (`secrets.yaml`) | Top-level global default in `credentials_file`. |
| **5** | `UBNT_USERNAME` / `UBNT_PASSWORD` (Env) | Process environment variables when omitted from YAML files. |

### Environment Variables

If credentials are omitted from both `config.yaml` and `credentials_file`, the exporter automatically falls back to environment variables:

| Environment Variable | Description |
| :--- | :--- |
| `UBNT_USERNAME` | Global fallback username for AirOS API requests. |
| `UBNT_PASSWORD` | Global fallback password for AirOS API requests. |

---

## Server & Runtime Options (`system.yaml`)

A template is provided in [`examples/system.yaml.example`](../examples/system.yaml.example).

### Complete `system.yaml` Example

```yaml
server:
  port: 8000
  scrape_interval: 30
  listen: "0.0.0.0:8000"
  probe_allowed_targets:
    - "10.0.0.0/8"
    - "172.16.0.0/12"
    - "192.168.0.0/16"

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
    rfc5424: false
    syslog: false
    json: false
    text: false
  console: true
  file:
    enabled: false
    path: "logs/app.log"
    max_size_mb: 5
    backup_count: 3
  dampening:
    enabled: false
    threshold: 3
    interval: 300
```

### The `server` Section

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `port` | integer | `8000` | Primary TCP port for HTTP metrics serving (1-65535). |
| `listen` | string | `"0.0.0.0:<port>"` | Listening socket address. Supports multiple whitespace-separated endpoints, IPv6 addresses, and Unix domain sockets. When omitted, it is derived from `port`; when set, it takes precedence over `port`. |
| `scrape_interval` | integer | `30` | Interval in seconds between automated daemon scrape cycles (minimum: 1). |
| `probe_allowed_targets` | string list | `[]` | List of allowed IP subnets (CIDR notation) or hostnames permitted for multi-target scraping via `/probe`. When empty, only devices defined in `config.yaml` can be probed. |

### Multi-Target Probe Security (`probe_allowed_targets`)

When exposing the `/probe` endpoint to Prometheus, restricting probe targets prevents Server-Side Request Forgery (SSRF) and unauthorized network scanning:

```yaml
server:
  probe_allowed_targets:
    - "10.10.0.0/16"
    - "10.20.0.0/16"
```

Requests to `/probe?target=192.168.1.1` outside these ranges return `HTTP 403 Forbidden`.

### Unix Domain Sockets

For high-security or local-agent architectures, `ubnt-exporter` can bind directly to a Unix Domain Socket:

```yaml
server:
  listen: "unix:///run/ubnt-exporter/metrics.sock"
```

Prometheus can scrape Unix Domain Sockets directly using the `http_sd_configs` or standard HTTP client configurations with Unix socket transports.

### The `performance` Section

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `max_workers` | integer | `100` | Concurrency limit for the internal goroutine worker pool. |
| `login_timeout` | integer | `5` | Timeout in seconds for AirOS session authentication (`/api/v1.0/user/login`). |
| `request_timeout` | integer | `15` | Timeout in seconds for telemetry payload requests (`/api/v1.0/status`). |
| `socket_timeout` | integer | `2` | TCP dial and handshake socket connection timeout in seconds. |
| `failure_backoff` | integer | `120` | Cooling-off duration in seconds assigned to an unresponsive device before retrying connections. |

### The `logging` Section

| Parameter | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `level` | string | `"INFO"` | Logging verbosity: `DEBUG`, `INFO`, `WARN`, `WARNING`, or `ERROR`. |
| `syslog_facility` | string | `"local0"` | Standard Syslog facility name for enterprise SIEM routing. |
| `formats` | map | `cef: true` | Structured format toggle: `cef`, `rfc5424`, `syslog`, `json`, or `text` (only one active at a time). Enabling just one (e.g. `json: true`) is enough; CEF is used only when none is enabled. |
| `console` | boolean | `true` | Output formatted log messages to `stdout`. |
| `file.enabled` | boolean | `false` | Enable output to an append-only log file. |
| `file.path` | string | `"logs/app.log"` | Filesystem destination for log output. |
| `file.max_size_mb` | integer | `5` | File size threshold before rotating log files. |
| `file.backup_count` | integer | `3` | Number of historical rotated log files preserved. |
| `dampening.enabled` | boolean | `false` | Throttles repetitive identical messages to prevent log storms. |
| `dampening.threshold` | integer | `3` | Number of consecutive identical log events before suppression begins. |
| `dampening.interval` | integer | `300` | Duration in seconds before emitting a suppression summary and resetting counters. |

> [!NOTE]
> **Consolidated Templates Directory:**
> All configuration and alerting templates are maintained in the dedicated [`examples/`](../examples/) directory:
> - [`examples/config.yaml.example`](../examples/config.yaml.example): Radio inventory, credentials, labels, and module toggles.
> - [`examples/system.yaml.example`](../examples/system.yaml.example): Server endpoints, timeouts, worker pool, and logging formats.
> - [`examples/alerts.yaml.example`](../examples/alerts.yaml.example): Prometheus 3.x alerting rules.
> - [`examples/alertmanager.yaml.example`](../examples/alertmanager.yaml.example): Alertmanager 0.34+ notification routing and templates.

---

## Dynamic Configuration Reloading (`SIGHUP` & `reload`)

The exporter listens for POSIX `SIGHUP` signals. Sending `SIGHUP` (or running `ubnt-exporter reload`) reloads both `system.yaml` and `config.yaml` at runtime without interrupting active scrapes or restarting the container:
- Updates the logging severity level dynamically.
- Re-discovers custom dimensional labels (`defaults.labels` and `devices[].labels`, such as `pop`, `area`, `tower`, `sector`), updates all 159 metric descriptors in memory, and immediately purges stale label series if a device's name or custom label values changed.
- Preserves active `X-Auth-Token` sessions for existing devices whose credentials and SSL settings have not changed.

```bash
# Automated safe reload with pre-validation and log confirmation:
docker exec ubnt-exporter ubnt-exporter reload

# Manual POSIX signal:
pkill -HUP ubnt-exporter

# In Docker via docker kill:
docker kill -s HUP ubnt-exporter
```

---

## Configuration Verification (`check` Command)

Before starting the daemon, validate configuration syntax, check duplicate IP addresses, verify CIDR ranges, and detect misconfigured labels using the `check` command:

```bash
ubnt-exporter check -c config.yaml -s system.yaml
```

Example successful validation output:

```text
 CONFIGURATION VALIDATION
==============================
RESULT: Configuration is VALID

 System Settings Verified:
    Server Listen Targets: [0.0.0.0:8000]
    Scrape Interval: 30s
    Max Workers: 100
    Timeouts (Login/Req/Socket): 5s / 15s / 2s
    Failure Backoff: 120s
    Log Level: INFO | Syslog Facility: local0
    Devices in Inventory: 2
    Dynamic Labels Found: [network region role sector tower]
```
