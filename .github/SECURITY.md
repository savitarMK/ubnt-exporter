# Security Policy

This document describes how to report security vulnerabilities in `ubnt-exporter`, how reports are handled, the security model of the exporter, and the hardening practices recommended for production deployments.

---

## Table of Contents

- [Supported Versions](#supported-versions)
- [Reporting a Vulnerability](#reporting-a-vulnerability)
  - [How to Report](#how-to-report)
  - [What to Include](#what-to-include)
  - [Response Process and Timelines](#response-process-and-timelines)
  - [Advisories, GHSA, and CVE Assignment](#advisories-ghsa-and-cve-assignment)
- [Scope](#scope)
- [Security Model and Known Limitations](#security-model-and-known-limitations)
- [Deployment Security Best Practices](#deployment-security-best-practices)
- [Maintainer Security Rules](#maintainer-security-rules)

---

## Supported Versions

Security fixes are applied only to the latest release line. Users should always run the most recent release.

| Version | Supported |
| :--- | :--- |
| 1.1.x | Yes |
| < 1.1 | No |

Container images are rebuilt for every release so that the Debian base image includes the latest available security updates.

---

## Reporting a Vulnerability

If you believe you have found a security vulnerability in `ubnt-exporter`, please report it privately so that it can be fixed before public disclosure.

### How to Report

Please **do not** report security vulnerabilities through public GitHub issues, discussions, or pull requests.

Report vulnerabilities privately using GitHub **Private Vulnerability Reporting**:

1. Open the **Security** tab of this repository.
2. Select **Report a vulnerability** under **Advisories** (or open [New Security Advisory](https://github.com/savitarMK/ubnt-exporter/security/advisories/new) directly).
3. Complete the advisory form with the details described below.

If Private Vulnerability Reporting is unavailable, contact the maintainer privately using the contact information on the maintainer's GitHub profile and request a secure channel before sharing technical details.

### What to Include

To help us triage and reproduce the issue quickly, please provide:

- A clear description of the vulnerability and its security impact (confidentiality, integrity, or availability).
- The affected version (`ubnt-exporter --version`), deployment method (binary, Docker, Kubernetes), and relevant configuration excerpts with all credentials removed.
- Step-by-step reproduction instructions or a proof of concept.
- Any known mitigations, workarounds, or proposed patches.

Never include real device credentials, session tokens, or production IP inventories in a report.

### Response Process and Timelines

| Stage | Target |
| :--- | :--- |
| Acknowledgment of the report | Within 72 hours |
| Initial triage and severity assessment (CVSS v3.1 or v4.0) | Within 7 days |
| Fix for Critical severity | Within 7 days of confirmation |
| Fix for High severity | Within 30 days of confirmation |
| Fix for Medium severity | Within 90 days of confirmation |
| Fix for Low severity | Next scheduled release |

We follow coordinated disclosure. We ask reporters to allow up to 90 days from the initial report before public disclosure, or until a fixed release is available, whichever comes first. We will keep you informed of progress throughout the process.

Reporters are credited in the release notes and in the published advisory unless they prefer to remain anonymous.

### Advisories, GHSA, and CVE Assignment

Confirmed vulnerabilities are handled through GitHub Security Advisories:

1. A draft advisory is opened and a temporary private fork is used to develop the fix.
2. A **GHSA** identifier is assigned to the advisory.
3. When the vulnerability warrants it, a **CVE** identifier is requested through GitHub, which acts as the CVE Numbering Authority (CNA).
4. The fix is released, the advisory is published, and the vulnerability becomes visible to the GitHub Advisory Database, Dependabot, and the Go vulnerability database (`govulncheck`).

---

## Scope

**In scope:**

- The `ubnt-exporter` source code in this repository (`cmd/`, `internal/`).
- The official container image built from the repository `Dockerfile`.
- Example configuration files (`*.example`) and documentation, when following them would result in an insecure deployment.

**Out of scope:**

- Vulnerabilities in Ubiquiti AirOS, LTU, or AirFiber firmware. Please report these directly to Ubiquiti.
- Vulnerabilities in third-party Go modules or Debian packages that do not affect `ubnt-exporter` in practice. Report these upstream; we track them and update dependencies as fixes become available.
- Issues that require an attacker who already has write access to the configuration files, the host, or the container runtime.
- Volumetric denial-of-service attacks against the network.
- Automated scanner output without a demonstrated impact on `ubnt-exporter`.

---

## Security Model and Known Limitations

Operators should understand the following characteristics of the current release when planning a deployment:

1. **Device TLS certificates are not verified.** Ubiquiti radios ship with self-signed certificates, so HTTPS connections to devices (`ssl: true`) are encrypted but do not validate the device certificate. An attacker positioned on the management network could impersonate a device and capture its credentials.
2. **`ssl: false` sends credentials in plain text.** With HTTP, AirOS usernames and passwords are transmitted unencrypted to `/api/v1.0/user/login`. Use `ssl: true` wherever the device supports it.
3. **Exporter endpoints are unauthenticated.** `/metrics` and `/probe` are served over plain HTTP without authentication. The metrics expose device names, IP addresses, firmware versions, and custom labels.
4. **`/probe` is restricted to the inventory.** The probe endpoint only scrapes devices that are already defined in `config.yaml`; arbitrary targets are rejected with `HTTP 404`. When `probe_allowed_targets` is set, requests outside the allowlist are rejected with `HTTP 403`. Every accepted probe request triggers a live request to the radio, so the endpoint should not be reachable by untrusted clients.
5. **Credentials are masked in diagnostic output.** The `show` command replaces passwords with `******`, and credentials are never written to logs.

---

## Deployment Security Best Practices

### 1. Network Segmentation

- `ubnt-exporter` listens on TCP port `8000` by default. Restrict access with firewall rules, security groups, or Kubernetes `NetworkPolicy` so that only authorized Prometheus servers can reach it.
- Never expose port `8000` to untrusted networks or the public internet.
- Place the exporter and the radios on a dedicated, trusted management VLAN to reduce the risk described in the [Security Model](#security-model-and-known-limitations).
- When Prometheus runs on the same host, prefer a Unix domain socket (`listen: "unix:///run/ubnt-exporter/metrics.sock"`) and restrict the socket directory permissions to the Prometheus service account.
- If authentication or TLS is required for scrapes, place the exporter behind a reverse proxy (for example, NGINX, Caddy, or Traefik) that enforces TLS, authentication, and request rate limits.

### 2. Device Credentials and Transport

- Create a dedicated AirOS account for monitoring. Use a read-only account where the firmware supports it, and never reuse the device administrator password.
- Enable `ssl: true` in `defaults` or per device whenever the firmware supports HTTPS.
- Rotate monitoring credentials periodically and immediately after any suspected exposure.

### 3. Multi-Target `/probe` Safeguards

- Configure `probe_allowed_targets` with the narrowest CIDR ranges or hostnames that cover your inventory (for example, `10.10.0.0/16`).
- When hostnames are used, ensure the DNS resolver is trusted, because DNS determines which host receives the connection and the credentials.
- If you do not use Prometheus multi-target scraping, block `/probe` at the reverse proxy or firewall.

### 4. Secrets Management

- Do not store credentials in plain text in `config.yaml` when it is shared or version-controlled. `config.yaml` and `system.yaml` are excluded by the repository `.gitignore`; keep it that way.
- Use `credentials_file` to reference an external secrets file and restrict its permissions:

  ```bash
  chmod 600 /etc/ubnt-exporter/secrets.yaml
  chown ubnt-exporter:ubnt-exporter /etc/ubnt-exporter/secrets.yaml
  ```

- Alternatively, inject credentials through the `UBNT_USERNAME` and `UBNT_PASSWORD` environment variables from a secret store (Docker secrets, Kubernetes `Secret`, HashiCorp Vault, or similar).
- In Alertmanager, prefer file-based secret references (`slack_api_url_file`, `bot_token_file`, and `credentials_file` under `http_config.authorization`) over inline tokens.

### 5. Container and Runtime Hardening

- The official image already runs as the unprivileged user `appuser` (UID `1000`). Do not override it with `root`.
- Mount configuration files read-only (`./config.yaml:/app/config.yaml:ro`), as shown in [docs/exporter.md](../docs/exporter.md#docker-compose).
- Apply additional runtime restrictions where your platform supports them:

  ```yaml
  services:
    exporter:
      read_only: true
      cap_drop:
        - ALL
      security_opt:
        - no-new-privileges:true
      tmpfs:
        - /app/logs
  ```

- For Kubernetes, set `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`, and drop all capabilities in the container `securityContext`.
- For systemd deployments, use the hardening directives documented in [docs/exporter.md](../docs/exporter.md#systemd-service-unit).

### 6. Logging

- Logs contain device names, IP addresses, and error details, but never credentials or session tokens.
- When file logging is enabled, restrict access to the log directory to the service account and your log shipping agent.
- Use `DEBUG` level only temporarily during troubleshooting, and revert to `INFO` with `SIGHUP` once finished.

---

## Maintainer Security Rules

The following rules apply to every change merged into the default branch and to every release.

### Dependency Policy

- All Go dependencies are pinned through `go.mod` and verified through `go.sum`. Never edit `go.sum` manually.
- Dependencies are reviewed and updated at least monthly, and immediately when a GHSA or CVE affects a module in use.
- New dependencies require justification. Prefer the Go standard library when it provides equivalent functionality.

### Secrets Policy

- Real credentials, tokens, webhook URLs, and production inventories must never be committed.
- Example files (`*.example`) and documentation must use obvious placeholders (for example, `YOUR_USERNAME`, `YOUR_PASSWORD`, `REPLACE_WITH_TOKEN`) or file-based secret references.
- Any credential committed by mistake must be considered compromised and rotated immediately; removing it from history is not sufficient.

### Pre-Release Security Verification

Every release must pass the following checks with no unresolved findings of High or Critical severity. Accepted risks must be documented in this file under [Security Model and Known Limitations](#security-model-and-known-limitations).

| Check | Purpose | Command |
| :--- | :--- | :--- |
| Go vulnerability scan | Known GHSA / CVE entries reachable from the code | `govulncheck ./...` |
| Static analysis | Insecure coding patterns (CWE) | `gosec -exclude-dir=test ./...` |
| Vet and race detection | Correctness and data races | `go vet ./... && go test -race ./test/...` |
| Secret scanning | Credentials committed to the repository | `gitleaks dir . --redact` |
| Repository scan | Dependency CVEs and Dockerfile misconfigurations | `trivy fs --scanners vuln,secret,misconfig .` |
| Container image scan | OS package and Go binary CVEs in the image | `trivy image ubnt-exporter:<version>` |

All checks can be executed in containers matching the build environment, for example:

```bash
docker run --rm -v "$(pwd)":/app -w /app golang:1.27.2-bookworm \
  sh -c 'go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...'
```

### Repository Settings

- GitHub Private Vulnerability Reporting is enabled.
- Dependabot alerts and Dependabot security updates are enabled.
- Secret scanning and push protection are enabled.
- The default branch is protected and requires passing checks before merging.
