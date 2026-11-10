## Summary

<!-- Provide a clear, concise summary of the changes introduced by this pull request. -->

Closes #<!-- Issue number, if applicable -->

---

## Release Note & Changelog Classification

<!--
Select ONE primary classification tag for automated CHANGELOG.md and GitHub Release generation.
Write a single-line or bulleted release note prefixed with the selected tag:
  - [CHANGE]      Breaking schema, metric rename, or architectural change
  - [FEATURE]     New metric, collector, CLI command, or exporter capability
  - [ENHANCEMENT] Improvement to existing collector, CLI, or telemetry behavior
  - [PERF]        Performance, concurrency, or memory optimization
  - [BUGFIX]      Fix for a reproducible defect or parsing error
-->

```release-note
[ENHANCEMENT] Describe your change here in American English for CHANGELOG.md.
```

---

## Affected Subsystems & Hardware Families

### Exporter Subsystems
- [ ] `internal/collector` (`system`, `interfaces`, `wireless`, `peers`, `airfiber`)
- [ ] `internal/definitions` (Prometheus 3.x metric definitions, `Unit`, `# HELP`, labels)
- [ ] `internal/metrics` (`Store`, `Buffer`, `UbiquitiCollector`, Native Histograms, self-telemetry)
- [ ] `internal/scraper` (HTTP/HTTPS client, TLS 1.0+ cipher suites, offline fixture replay)
- [ ] `internal/config` (`config.yaml`, `system.yaml`, `secrets.yaml`, dynamic labels, `SIGHUP`)
- [ ] `cli/` (`run`, `diag`, `inspect`, `export`, `query`, `metrics`, `check`, `reload`, `test`)
- [ ] `internal/logger` (JSON, ArcSight CEF, RFC 5424, BSD Syslog, anti-storm dampening)
- [ ] `docs/` & `examples/` (Documentation, PromQL alerts, Alertmanager templates)
- [ ] `.github/` / `Dockerfile` (CI/CD workflows, container build, security scanning)

### Ubiquiti Hardware Platforms Tested
- [ ] AirMAX AC Series (`airOS v8` — `testdata/fixtures/airmax_ac_ptmp.json`)
- [ ] AirMAX M-Series (`airOS v6` 1x1 SISO / 2x2 MIMO — `testdata/fixtures/airmax_m_ptmp.json`)
- [ ] AirFiber 5XHD / LTU Series (`testdata/fixtures/af5xhd_ptp.json`)
- [ ] Live Lab Hardware (specify model and firmware: `____________________`)

---

## Quality & Compliance Checklist

- [ ] **Go Standards & Static Analysis:** `go vet ./...` passes with zero warnings.
- [ ] **Race Detector & Unit Tests:** `go test -race -coverpkg=./internal/...,./cli/... ./test/...` passes 100%.
- [ ] **Built-in Self-Test Suites:** `./ubnt-exporter test metrics` and `./ubnt-exporter test collectors` pass with zero errors.
- [ ] **Prometheus 3.x & OpenMetrics 2.0 Compliance:**
  - Every new or modified metric has an explicit `Unit` and a complete `# HELP` sentence ending with a period (`.`).
  - `_total` suffix is used **exclusively** on `Counter` metrics (never on `Gauge` or `Info`).
  - `_info` suffix is used on all `Info` metrics.
  - Global AP metrics use `ubnt_ap_*`, station metrics use `ubnt_peer_*`, and AirFiber-only metrics use `ubnt_af_*`.
- [ ] **Data Sanitization & RFC 1918 Anonymization:** All IP addresses in fixtures, tests, and documentation use generic `192.168.1.x` addresses and contain no real credentials or private hostnames.
- [ ] **Documentation Updated:** Relevant tables in `README.md` and `docs/` (`exporter.md`, `configuration.md`, `diagnostics.md`) are synchronized.
