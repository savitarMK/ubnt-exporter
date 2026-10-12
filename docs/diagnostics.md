# Operational Diagnostics & Troubleshooting Guide

The `ubnt-exporter` provides built-in diagnostic commands and structured logging formats to validate configurations, audit live wireless links, detect degraded Ethernet ports, and trace API interactions directly from your terminal.

---

## Table of Contents

- [CLI Command Overview](#cli-command-overview)
- [Configuration Validation (`check`)](#configuration-validation-check)
  - [Common Validation Warnings and Fixes](#common-validation-warnings-and-fixes)
- [Active Configuration Inspection (`show`)](#active-configuration-inspection-show)
- [Dry-Run Scrape Preview (`export`)](#dry-run-scrape-preview-export)
- [Live Terminal Diagnostics (`diag`)](#live-terminal-diagnostics-diag)
  - [Station Health & Signal Auditing](#station-health--signal-auditing)
  - [Detecting RF Chain Imbalance](#detecting-rf-chain-imbalance)
  - [Detecting Degraded Ethernet Ports](#detecting-degraded-ethernet-ports)
- [AirFiber Diagnostic Audits](#airfiber-diagnostic-audits)
  - [GPS Synchronization](#gps-synchronization)
  - [DFS Radar CAC and Lockout Timers](#dfs-radar-cac-and-lockout-timers)
- [Logging & SIEM Observability](#logging--siem-observability)
  - [Log Severity Levels](#log-severity-levels)
  - [Structured SIEM Formats (CEF, RFC 5424, Syslog)](#structured-siem-formats-cef-rfc-5424-syslog)
  - [Dynamic Log Level Reloading via SIGHUP](#dynamic-log-level-reloading-via-sighup)
- [Troubleshooting Common AirOS Quirks](#troubleshooting-common-airos-quirks)

---

## CLI Command Overview

| Command | Purpose | Network Activity |
| :--- | :--- | :--- |
| `ubnt-exporter check` | Verifies YAML syntax, duplicate IPs, and indentation. | None (offline) |
| `ubnt-exporter show` | Dumps merged in-memory configuration with masked secrets. | None (offline) |
| `ubnt-exporter export` | Performs a one-shot live scrape and outputs Prometheus metrics. | Active network queries |
| `ubnt-exporter diag` | Renders ASCII diagnostic tables of wireless stations and links. | Active network queries |
| `ubnt-exporter run` | Starts the production metrics daemon. | Continuous daemon |

Global options:

- `-c, --config <path>`: Path to `config.yaml`.
- `-s, --system <path>`: Path to `system.yaml`.
- `--cfg-dir <path>`: Directory containing both configuration files.
- `-v, --version`: Displays exporter version and exits.
- `-log, --log`: Prints logging reference manual.

---

## Configuration Validation (`check`)

Use `ubnt-exporter check` to catch configuration defects before starting the daemon:

```bash
ubnt-exporter check -c config.yaml -s system.yaml
```

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

### Common Validation Warnings and Fixes

#### 1. Indentation Error: Connection Properties Under `labels`

```text
 - device 'AP-SECTOR-01' has reserved parameter ssl (boolean) placed inside 'labels'; configure it directly under device instead of under 'labels'
```

**Fix:** Move `ssl`, `username`, `password`, or `enabled_collectors` out of the `labels:` block and indent them directly under the `- name:` list item.

#### 2. Duplicate IP Address

```text
 - duplicate device IP "10.0.0.1" detected (used by 'AP-01' and 'AP-02')
```

**Fix:** Ensure every device in `config.yaml` has a distinct IP address.

#### 3. Invalid Listen Address

```text
 - invalid server.listen: invalid port in listen target "0.0.0.0:99999"
```

**Fix:** Ensure server ports are between `1` and `65535`.

---

## Active Configuration Inspection (`show`)

To inspect merged defaults and verified inventory entries without exposing credentials, run `ubnt-exporter show`:

```bash
ubnt-exporter show -c config.yaml -s system.yaml
```

Output:

```yaml
ACTIVE CONFIGURATION
server:
    port: 8000
    listen: 0.0.0.0:8000
    scrape_interval: 30
performance:
    max_workers: 100
    login_timeout: 5
    request_timeout: 15
    socket_timeout: 2
    failure_backoff: 120
logging:
    level: INFO
    syslog_facility: local0
    formats:
        cef: true
        rfc5424: false
        syslog: false
        text: false
        json: false
    console: true
    file:
        enabled: false
        path: logs/app.log
        max_size_mb: 5
        backup_count: 3
    dampening:
        enabled: false
        threshold: 3
        interval: 300
defaults:
    username: YOUR_USERNAME
    password: '******'
    labels:
        network: core
        region: us-east
devices:
    - name: AP-SECTOR-01
      ip: 10.0.0.1
      labels:
        sector: alpha
        tower: tower-north
    - name: AP-BACKHAUL-01
      ip: 10.0.0.2
      username: custom_admin
      password: '******'
      ssl: true
      labels:
        role: backhaul
        tower: tower-south
      enabled_collectors:
        peers: false
enabled_collectors:
    airfiber: true
    interfaces: true
    peers: true
    system: true
    wireless: true
```

---

## Dry-Run Scrape Preview (`export`)

The `export` command connects to target devices, authenticates via AirOS REST endpoints, executes a single scrape cycle, and prints the raw Prometheus text exposition directly to `stdout`:

```bash
ubnt-exporter export -c config.yaml -s system.yaml
```

Output excerpt:

```text
 ONE-SHOT EXPORT (Dry Run)

--- PROMETHEUS METRICS PREVIEW ---
# HELP ubnt_build_info Build and version information for ubnt-exporter
# TYPE ubnt_build_info gauge
ubnt_build_info{branch="main",goversion="go1.27.2",revision="manual",version="1.1.1"} 1
# HELP ubnt_ap_cpu_usage_percent CPU Usage %
# TYPE ubnt_ap_cpu_usage_percent gauge
ubnt_ap_cpu_usage_percent{ip="10.0.0.1",name="AP-SECTOR-01",network="core",region="us-east",sector="alpha",tower="tower-north"} 14
# HELP ubnt_peer_signal_dbm Peer Signal
# TYPE ubnt_peer_signal_dbm gauge
ubnt_peer_signal_dbm{ap_ip="10.0.0.1",ap_name="AP-SECTOR-01",peer_ip="10.0.0.55",peer_name="CPE-CLIENT-01"} -58
# HELP ubnt_peer_chain_imbalance_db Chain Imbalance
# TYPE ubnt_peer_chain_imbalance_db gauge
ubnt_peer_chain_imbalance_db{ap_ip="10.0.0.1",ap_name="AP-SECTOR-01",peer_ip="10.0.0.55",peer_name="CPE-CLIENT-01"} 1.2
```

---

## Live Terminal Diagnostics (`diag`)

The `diag` command queries the configured devices and formats wireless link quality into an ASCII diagnostic table.

```bash
ubnt-exporter diag
```

Sample output:

```text
+----------------+---------------+------------+-----------+-----------+---------------+----------+----------+
| Access Point   | Target IP     | Status     | Signal    | Noise     | Chain Delta   | Clients  | Capacity |
+================+===============+============+===========+===========+===============+==========+==========+
| AP-SECTOR-01   | 10.0.0.1      | Online     | -58 dBm   | -96 dBm   | 1.0 dB        | 18       | 285 Mbps |
| AP-BACKHAUL-01 | 10.0.0.2      | Online     | -62 dBm   | -98 dBm   | 0.5 dB        | 1        | 450 Mbps |
| AP-SECTOR-02   | 10.0.0.3      | Online     | -79 dBm   | -91 dBm   | 6.5 dB (!)    | 7        | 110 Mbps |
+----------------+---------------+------------+-----------+-----------+---------------+----------+----------+
Audited devices: 3 | Online: 3 | Warning chain delta (>5 dB): 1
```

### Station Health & Signal Auditing

For access points serving wireless subscribers (AirMAX or LTU), individual stations can be filtered by RF health:

- **Signal Threshold:** Optimal client signal ranges between `-50 dBm` and `-65 dBm`. Signals below `-75 dBm` experience high frame retransmissions and lower modulation rates.
- **Noise Floor:** A healthy noise floor is below `-95 dBm`. A noise floor elevated above `-85 dBm` indicates adjacent channel interference.

### Detecting RF Chain Imbalance

Dual-polarity wireless systems use horizontal and vertical chains:
$$\text{Chain Imbalance} = |C_0 - C_1|$$

- **Normal ($0 \text{ to } 3 \text{ dB}$):** Optimal line-of-sight propagation with aligned antenna polarization.
- **Acceptable ($3 \text{ to } 5 \text{ dB}$):** Minor reflection or partial Fresnel zone encroachment.
- **Degraded ($> 5 \text{ dB}$):** Severe antenna misalignment, loose mounting hardware, water ingress in the RF jumper cable, or Fresnel zone obstruction.

### Detecting Degraded Ethernet Ports

AirMAX and AirFiber hardware communicate with tower switches over gigabit Ethernet interfaces. Damaged Ethernet cabling, crimping defects, or water in RJ45 connectors frequently cause radios to negotiate down to `100 Mbps` or `10 Mbps` half-duplex:

1. Inspect `ubnt_iface_rx_rate_bps` and `ubnt_iface_tx_rate_bps`.
2. Check `ubnt_iface_rx_errors_total` and `ubnt_iface_rx_drops_total` counters. High CRC error rates along with low link rates indicate physical cable damage.

---

## AirFiber Diagnostic Audits

### GPS Synchronization

AirFiber radios depend on GPS satellite timing to synchronize TDD frame transmit and receive intervals with adjacent towers:

- `ubnt_af_gps_satellites`: Should track $\ge 4$ satellites for a valid 3D position and timing fix.
- `ubnt_af_gps_sync_status`: Value `1` confirms active TDD synchronization; `0` indicates the radio has lost sync and fallen back to free-running mode, which can cause self-interference with collocated radios.

### DFS Radar CAC and Lockout Timers

When operating in Dynamic Frequency Selection (DFS) bands (5.3 GHz and 5.4 GHz):

- `ubnt_af_dfs_blocked_status`: Value `1` indicates Channel Availability Check (CAC) is currently scanning the channel before transmission can begin.
- `ubnt_af_dfs_cac_remaining_seconds`: Shows the remaining CAC countdown timer in seconds (typically 60 to 600 seconds).
- `ubnt_af_dfs_lockout_total`: Increments whenever radar energy is detected, causing the radio to evacuate the frequency for a mandatory 30-minute non-occupancy period.

---

## Logging & SIEM Observability

### Log Severity Levels

Run `ubnt-exporter -log` to view the logging level guide:

- **`DEBUG`:** Full protocol tracing, HTTP request headers, and payload parsing steps.
- **`INFO` (Default):** Startup details, discovered custom labels, device counts, and scrape cycle elapsed duration.
- **`WARN`:** Recoverable events such as expired session tokens, authentication retries, and temporary network backoff.
- **`ERROR`:** Critical operational errors such as host unreachable, connection refused, or YAML syntax faults.

### Structured SIEM Formats (CEF, RFC 5424, Syslog)

Configure `system.yaml` to route logs directly to your enterprise SIEM or central syslog collector:

#### ArcSight Common Event Format (CEF)

```yaml
logging:
  formats:
    cef: true
```

Log output:

```text
CEF:0|Ubiquiti|ubnt-exporter|1.1.1|STATUS|Scrape Finished|3|src=10.0.0.1 msg=Scrape cycle finished in 0.35s
```

#### IETF Syslog Protocol (RFC 5424)

```yaml
logging:
  formats:
    rfc5424: true
```

Log output:

```text
<134>1 2026-10-11T12:00:00Z router ubnt-exporter 1234 - - Scrape cycle finished in 0.35s
```

#### Structured JSON Lines (Loki, Elasticsearch, Kubernetes)

```yaml
logging:
  formats:
    json: true
```

Log output:

```json
{"timestamp":"2026-10-11T12:00:00Z","level":"INFO","caller":"main","trace_id":"cycle-1","message":"Scrape cycle finished in 0.35s (10/10 devices online)","host":"monitoring-host","pid":1234}
```

### Request Tracing & Context Correlation

The exporter correlates log messages across asynchronous operations using context propagation (`context.Context`):

- **Daemon Scrape Cycles (`cycle-N`):** Each automated scrape cycle assigns an incremental trace ID (e.g. `cycle-1`, `cycle-2`). All concurrent goroutines scraping target devices inherit this identifier, allowing operators to trace all events belonging to a specific cycle.
- **On-Demand Probes (`/probe`):** The `/probe` endpoint inspects the incoming `X-Request-ID` HTTP header. If absent, it automatically generates a unique identifier (e.g. `probe-1a2b3c`). The ID is propagated through device authentication, statistics querying, and response rendering, and echoed back in the HTTP response headers.

Format representation:

- **JSON:** Exposed as the `"trace_id"` key in each JSON log object.
- **Text:** Appended in brackets beside the caller name (`MockAP [probe-1a2b3c]`).
- **CEF:** Exported as custom extension `cs1=... cs1Label=TraceID`.
- **RFC 5424:** Encoded as structured data `[trace@4711 id="..."]`.

### Dynamic Log Level Reloading via SIGHUP

When troubleshooting an active production incident, elevate the logging level to `DEBUG` without interrupting continuous Prometheus scraping:

1. Edit `system.yaml` and set `level: "DEBUG"`.
2. Send `SIGHUP` to the process:

   ```bash
   pkill -HUP ubnt-exporter
   ```

3. The exporter logs:

   ```text
   INFO | main | SIGHUP received, checking for configuration reload...
   INFO | main | Runtime log level updated on reload: DEBUG
   ```

4. Once debugging is complete, set `level: "INFO"` and re-send `SIGHUP`.

### Log Dampening & Anti-Storm Throttling

When wireless links experience degradation or remote towers lose connectivity, failing scrape cycles can rapidly flood log outputs with thousands of identical error lines. The built-in log dampener suppresses repeated duplicate log messages while preserving visibility into state transitions:

```yaml
logging:
  dampening:
    enabled: true
    threshold: 3   # Emit the first 3 identical occurrences
    interval: 300  # Suppress duplicates for 300 seconds (5 minutes)
```

Behavior:

- **Initial Occurrences:** The first `threshold` identical error events are emitted normally.
- **Suppression Window:** Subsequent identical error events are throttled and counted internally without polluting disk or console streams.
- **Periodic Summary:** Once `interval` seconds have elapsed, the next occurrence is emitted along with an annotation indicating how many identical messages were suppressed:

  ```text
  2026-10-11 12:05:00 | ERROR | AP-South | Login Unreachable: context deadline exceeded (suppressed 9 identical messages)
  ```

- **Automatic Reset on Recovery:** When a scrape cycle completes successfully, the exporter automatically clears active dampening counters for that device, ensuring future issues are logged immediately without suppression delay.

### Early Startup Logging & Fatal Error Channeling

The exporter initializes logging infrastructure at early process startup, ensuring consistent structured log formatting across the entire application lifecycle:

- **Early Initialization:** The logger is configured before argument parsing and configuration resolution. If `system.yaml` defines a custom structured format (e.g., JSON lines for Kubernetes/Loki), early configuration failures or missing inventory files are formatted and emitted in that exact format rather than raw unstructured text.
- **Fail-Fast Validation in Daemon Mode:** When running `ubnt-exporter run`, comprehensive configuration validation is performed prior to allocating worker pools or binding network listeners. Any validation errors are logged at `ERROR` level, followed by a structured fatal termination.
- **Listener Fatal Error Channeling:** HTTP server routines running in background goroutines channel critical listener failures back to the main control loop. This guarantees that unrecoverable network errors trigger immediate clean shutdowns and descriptive structured logs, preventing silent headless zombie processes.

### Collector DEBUG Event Tracing (Missing & Unrecognized Payload Fields)

When diagnosing why specific metrics do not appear for a target radio model or custom firmware version, elevating the logging level to `DEBUG` activates fine-grained event tracing across all metric collectors (`wireless`, `airfiber`, `system`, `interfaces`, `peers`).

Rather than failing silently or causing unexpected zero values, collectors emit precise diagnostic entries when fields are omitted or malformed in the device payload:

- **Missing Subsystems:**

  ```text
  DEBUG | AP-South [cycle-1] | Collector 'airfiber': 'gps' telemetry missing in device data
  ```

- **Incomplete Radio Metrics:**

  ```text
  DEBUG | AP-Tower-North [cycle-2] | Collector 'wireless': 'radios' list missing or empty in wireless data
  ```

- **Interface Parsing Anomalies:**

  ```text
  DEBUG | Backhaul-5XHD [probe-a1b2] | Collector 'interfaces': interface "eth1" missing 'statistics' map
  ```

This diagnostic visibility allows network engineers to verify whether an absent Prometheus metric stems from firmware capabilities, disabled hardware features (such as GPS antennas), or unexpected JSON schema changes without altering exporter code.

---

## Troubleshooting Common AirOS Quirks

| Symptom | Cause | Resolution |
| :--- | :--- | :--- |
| Exporter reports `HTTP 401 Unauthorized` | Invalid username/password or credential placed under `labels:`. | Validate credentials in browser; check YAML indentation with `ubnt-exporter check`. |
| Exporter reports `HTTP 404 Not Found` | Target device runs legacy AirOS v5 or unsupported firmware. | The REST API requires AirOS v8 (AirMAX AC), AirFiber 5XHD firmware, or LTU. |
| Zero peers returned for Access Point | `peers` collector disabled or device operating in Station (CPE) mode. | Verify `enabled_collectors.peers: true` and confirm target radio is configured as an AP. |
| Timeouts during scrape cycle | High radio CPU utilization or wireless link packet loss. | Increase `performance.request_timeout` in `system.yaml` or adjust `failure_backoff`. |
