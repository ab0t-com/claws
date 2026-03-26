# Ticket 6: Security Matrix — Deployment Checklist & Continuous Audit

**Priority:** P1 — High
**Created:** 2026-03-25
**Status:** Done

## Security Matrix

### Layer 1: Host

| Check | Default | Secure | How to Fix |
|-------|---------|--------|------------|
| Firewall active | No | UFW/iptables blocking gateway ports | `ufw allow 22/tcp && ufw enable` |
| SSH key-only auth | Varies | Yes | `/etc/ssh/sshd_config: PasswordAuthentication no` |
| Gateway ports firewalled | Open | Blocked (use SSH tunnel) | `ufw deny 18789:19999/tcp` |
| Docker socket permissions | 0660 docker group | Restricted | Only admin users in docker group |
| Automatic security updates | Varies | Enabled | `apt install unattended-upgrades` |

### Layer 2: clawctl Control Plane

| Check | Default | Secure | How to Fix |
|-------|---------|--------|------------|
| instance.env permissions | 0600 (new), 0664 (old) | 0600 | `clawctl doctor --fix` |
| .port-registry permissions | 0600 (new), 0664 (old) | 0600 | `clawctl doctor --fix` |
| credential file permissions | Mixed | 0600 | `find ~/.openclaw -path '*/credentials/*' -exec chmod 600 {} +` |
| Gateway bind mode | `lan` (0.0.0.0) | `loopback` (127.0.0.1) | Ticket 1: change default |
| File locking | Enabled | Enabled | Already implemented (flock.go) |
| Gitleaks pre-commit | Installed | Installed | `scripts/install-hooks.sh` |
| Admin policy | None | Enforced | Ticket 2: policy.json |
| Access control | None (any user) | Role-based | Ticket 4: .access.json |
| Audit logging | None | Enabled | Ticket 4: .audit.log |
| Token rotation | Manual | CLI command | Ticket 4: `clawctl token rotate` |

### Layer 3: Docker Container

| Check | Default | Secure | How to Fix |
|-------|---------|--------|------------|
| Run as non-root | `node` | `node` ✓ | Already correct |
| Privileged mode | false | false ✓ | Already correct |
| Docker socket mount | Not mounted | Not mounted ✓ | Don't uncomment in compose |
| cap_drop ALL | Not set | ALL dropped | Ticket 1: update compose template |
| no-new-privileges | Not set (gateway) | Set | Ticket 1: update compose template |
| Read-only rootfs | Writable | Read-only + tmpfs | Ticket 1: update compose template |
| Memory limit | Unlimited | 1GB per instance | Ticket 2: policy enforcement |
| CPU limit | Unlimited | 2 cores per instance | Ticket 2: policy enforcement |
| Network isolation | Per-project bridge ✓ | Per-project bridge ✓ | Already correct |
| Seccomp profile | Default | Default ✓ | Docker default is fine |

### Layer 4: OpenClaw Gateway

| Check | Default | Secure | How to Fix |
|-------|---------|--------|------------|
| Gateway token | 256-bit hex ✓ | Strong ✓ | Already correct |
| Control UI auth | No auth on / | Token required | OpenClaw config: `auth.mode=token` |
| DM policy | `pairing` | `pairing` ✓ | Already correct by default |
| Group policy | `allowlist` | `allowlist` ✓ | Already correct |
| Tool sandbox | Not set | Enabled | `agents.defaults.sandbox: true` |
| Tool profile | Varies | Set per instance | `tools.profile: coding` |
| Session isolation | Varies | `per-channel-peer` | `session.dmScope: per-channel-peer` |
| Model restrictions | None | Set allowed models | `models.allowlist` |
| Workspace isolation | Shared by default in groups | Intentional for groups | Document the trust boundary |

### Layer 5: Network Path

| Check | Default | Secure | How to Fix |
|-------|---------|--------|------------|
| HTTPS (Caddy proxy) | Not set up | Configured with auth | `clawctl proxy setup --domain=...` |
| Proxy auth headers | Injected by default | Injected ✓ | Already correct |
| SSH tunnel for dev | Manual | `clawctl tunnel` ✓ | Already correct |
| Internal TLS | None (loopback OK) | Not needed for loopback | Document this assumption |

## Audit Script

**File:** `scripts/security-audit.sh`

Already implemented. Checks:
1. File permissions (instance.env, .port-registry, credentials)
2. Network exposure (ports bound to 0.0.0.0, firewall)
3. Container security (user, privileges, caps, memory, docker socket)
4. Gateway auth (token strength, unauthenticated endpoints)
5. Tool & sandbox policy
6. Channel security (DM policies)
7. Image security (exists, default user)

### Proposed Additions
- Policy compliance check (ticket 2)
- Access control validation (ticket 4)
- Image version pinning check (ticket 5)
- SSL/TLS check for proxy (if configured)
- Audit log integrity check

## Integration

### `clawctl doctor` should include security summary
### `clawctl create` should warn on insecure defaults
### `clawctl start` should validate against policy before starting
### CI/CD: `scripts/security-audit.sh` as a gate before deployment
