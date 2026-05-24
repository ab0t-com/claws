# Ticket 2: Admin Policy Layer — Global Configuration & Constraints

**Priority:** P1 — High
**Created:** 2026-03-25
**Status:** Done

## Problem

There is no way for an admin to enforce security policies across all instances. Each instance is configured independently via its own `openclaw.json`. An admin cannot:

- Force loopback-only binding
- Enforce sandbox mode
- Restrict tool profiles
- Set memory/CPU limits
- Block specific channels
- Require pairing-only DM policy
- Restrict which images can be used

## Design

### Policy File: `~/.openclaw/policy.json`

A global policy file that constrains what instances can do. Policy is enforced by claws at create/start time and validated by the security audit.

```json5
{
  // Network
  "allowedBindModes": ["loopback"],          // restrict bind to loopback only
  "maxInstances": 8,                          // hard cap

  // Container
  "memoryLimitMB": 1024,                     // per-instance memory limit
  "cpuLimit": 2.0,                           // per-instance CPU limit
  "requireNoNewPrivileges": true,
  "requireCapDropAll": true,
  "allowDockerSocket": false,                // never allow docker.sock mount

  // Agent
  "requireSandbox": true,                   // force sandbox mode
  "allowedToolProfiles": ["coding", "messaging"],  // restrict tool profiles
  "requireDmPairing": true,                 // force pairing on all channels
  "blockedChannels": [],                     // channels that cannot be enabled

  // Image
  "allowedImages": ["openclaw:local", "openclaw:v*"],  // glob patterns
  "requireImageDigest": false,              // require sha256 pinning

  // Access
  "allowedUsers": [],                       // OS users who can run claws (empty = any)
  "auditLog": true                          // log all claws commands to audit file
}
```

### Enforcement Points

| Policy | Enforced At | File |
|--------|------------|------|
| `allowedBindModes` | `cmdCreate` | `commands.go` |
| `maxInstances` | `cmdCreate` | `commands.go` (already exists) |
| `memoryLimitMB` / `cpuLimit` | compose override generation | `shared.go`, `group.go` |
| `requireNoNewPrivileges` | compose override generation | `shared.go`, `group.go` |
| `requireSandbox` | `cmdCreate`, `cmdStart` | `commands.go` |
| `allowedToolProfiles` | `cmdCreate`, config set | `commands.go`, `configcmd.go` |
| `requireDmPairing` | `cmdChannelAdd` | `channel.go` |
| `blockedChannels` | `cmdChannelAdd` | `channel.go` |
| `allowedImages` | `cmdCreate` | `commands.go` |
| `allowedUsers` | `main()` | `main.go` |
| `auditLog` | all commands | `main.go` or middleware |

### Implementation

1. Create `policy.go` with `readPolicy()`, `enforcePolicy()`, `validatePolicy()`
2. `claws policy show` — print current policy
3. `claws policy init` — create default policy.json with secure defaults
4. `claws policy validate` — check all instances against policy
5. Integrate enforcement into create, start, channel add, config set
6. Add policy check to `scripts/security-audit.sh`

### Resource Limits in Compose Override

When policy specifies memory/CPU limits, inject into the generated override:

```yaml
services:
  openclaw-gateway:
    deploy:
      resources:
        limits:
          memory: 1024M
          cpus: '2.0'
```

## Testing
- Unit test: policy loading and enforcement
- Integration test: create with policy violation → rejected
- Integration test: start with policy violation → rejected
- Integration test: channel add with blocked channel → rejected
- Audit script validates policy compliance
