# Ticket 1: Security Hardening — Fix Immediate Exposure

**Priority:** P0 — Critical
**Created:** 2026-03-25
**Status:** Done

## Problem

The security audit (`scripts/security-audit.sh`) reports 4 failures and 27 warnings. Several are immediate exposure risks on any internet-facing server.

## Immediate Fixes (P0)

### 1.1 Fix file permissions on pre-existing instances
The `group add` command moved instances but preserved old 0664 permissions. New instances get 0600 but existing ones don't.

**Files:** `group.go` cmdGroupAdd — after moving, should `chmod 0600` the instance.env
**Also:** Add a `claws fix-permissions` or run it as part of `claws doctor --fix`

```bash
# Manual fix now:
find ~/.openclaw -name "instance.env" -exec chmod 600 {} +
find ~/.openclaw -path "*/credentials/*" -type f -exec chmod 600 {} +
chmod 600 ~/.openclaw/.port-registry
```

### 1.2 Harden docker-compose.yml template

Current gateway service has no security options. The CLI service already has `cap_drop` and `no-new-privileges` but the gateway doesn't.

**File:** `docker-compose.yml`

Add to `openclaw-gateway` service:
```yaml
cap_drop:
  - ALL
security_opt:
  - no-new-privileges:true
# read_only: true  # requires tmpfs for /tmp and writable paths
deploy:
  resources:
    limits:
      memory: 1G
      cpus: '2.0'
```

### 1.3 Default bind to loopback, not LAN

**File:** `commands.go:173` — `OPENCLAW_GATEWAY_BIND=lan`

Change default to `loopback`. Users who need LAN access opt in with `--bind=lan` on create.

```
OPENCLAW_GATEWAY_BIND=${OPENCLAW_GATEWAY_BIND:-loopback}
```

### 1.4 Add `claws doctor --fix` to auto-remediate permissions

**File:** `doctor.go`

When `--fix` is passed, doctor should:
- `chmod 0600` all instance.env files
- `chmod 0600` all credential files
- `chmod 0600` .port-registry
- Report what was fixed

## Testing
- `scripts/security-audit.sh` should report 0 failures after fixes
- Integration test: `TestIntegration_CreateDefaultsToLoopback`
- Integration test: `TestIntegration_DoctorFix`
