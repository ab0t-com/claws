# Security Policy

clawctl manages AI agent instances that hold sensitive credentials (gateway tokens, OAuth refresh tokens, channel API keys). We take security reports seriously.

## Supported versions

| Version    | Supported          |
|------------|--------------------|
| `1.x.x`    | ✅ Active           |
| `< 1.0.0`  | ❌ Pre-release; please upgrade |

We do not backport security fixes to pre-`1.0` releases.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security problems.**

Report privately, in order of preference:

1. **GitHub Security Advisory** (preferred):
   - Go to https://github.com/ab0t-com/claws/security/advisories/new
   - This gives us a private channel and tracks the disclosure timeline.

2. **Email**: security@ab0t.com
   - PGP encryption available on request.
   - Please include "clawctl" in the subject line.

### What to include

- A description of the issue and its impact.
- Steps to reproduce (a minimal proof-of-concept is ideal).
- The clawctl version (`clawctl version`) and OS.
- Any suggested mitigation or fix (optional).

### What to expect

- **Acknowledgement** within 72 hours.
- **Initial assessment** within 7 days, including severity rating (CVSS v3.1).
- **Fix or mitigation timeline** communicated within 14 days for confirmed issues.
- **Public disclosure** coordinated with you, typically 90 days from initial report, sooner if a fix is shipped earlier.

## In scope

Vulnerabilities in:

- The `clawctl` binary and its libraries.
- The `docker-compose.yml` template shipped with clawctl.
- The release tarballs and the `install.sh` installer (e.g. checksum bypass, MITM).
- The git hooks and `gitleaks` configuration where a flaw would let a real secret pass undetected.

Examples of in-scope issues:

- Privilege escalation from a compromised agent container to the host.
- Credential leakage through `clawctl` output, logs, or audit files.
- Authentication bypass on the gateway port.
- Path traversal or arbitrary file write via crafted instance names or config.
- Supply-chain issues in the install or release flow.

## Out of scope

- Vulnerabilities in the OpenClaw runtime image itself — report to https://github.com/openclaw/openclaw.
- Vulnerabilities in Docker, Caddy, or other upstream dependencies — report to the upstream project.
- Configuration weaknesses that are addressed by `clawctl audit` warnings (e.g. running without a firewall) — these are documented hardening recommendations, not vulnerabilities.
- Social-engineering attacks against operators (e.g. tricking someone into running a malicious `clawctl create`).
- Denial-of-service achievable only with local shell access on the host.

## Hardening recommendations

Even with a clean vulnerability report, deployments should:

1. Run `clawctl audit` after install and after upgrades.
2. Run `clawctl policy enforce --restart` after a security update changes default policy.
3. Keep gateway ports loopback-only (`OPENCLAW_HOST_BIND=127.0.0.1`); use `clawctl tunnel` to access from outside.
4. Use a host firewall (`ufw`, `iptables`, cloud security group) as defense in depth.
5. Rotate gateway tokens periodically with `clawctl token rotate`.

## Disclosure history

No disclosures yet. Past advisories will be linked here once published.

---

Thank you for helping keep clawctl and its users safe.
