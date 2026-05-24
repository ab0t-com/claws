# Ticket 4: Multi-Tenant Control Plane — Admin vs User Access

**Priority:** P2 — Medium
**Created:** 2026-03-25
**Status:** Done
**Depends on:** Ticket 2 (Admin Policy)

## Problem

claws currently has a single trust level — whoever can run the binary has full access to all instances, credentials, and configuration. There is:

- No concept of "admin" vs "user"
- No way to give a user access to their instance but not others
- No audit trail of who did what
- No way to restrict destructive operations to admins
- No token rotation without manual file editing

## Design

### Access Levels

| Level | Can Do | Cannot Do |
|-------|--------|-----------|
| **admin** | Everything: create, remove, policy, storage, proxy, group manage | — |
| **operator** | start, stop, restart, logs, exec, health, status, config show, backup | create, remove --purge, policy, storage setup, proxy setup |
| **user** | status, health, logs (own instance), config show (own, --no-secrets) | Anything destructive, any other instance |

### Access Control File: `~/.openclaw/.access.json`

```json5
{
  "roles": {
    "admin": {
      "users": ["ubuntu"],                    // OS usernames
      "commands": ["*"]
    },
    "operator": {
      "users": ["deploy-bot"],
      "commands": ["start", "stop", "restart", "logs", "exec", "health",
                   "status", "config show", "backup", "list", "dashboard",
                   "activity", "channel status", "tunnel"]
    },
    "user": {
      "users": ["alice-user"],
      "instances": ["team/alice"],            // scoped to specific instances
      "commands": ["status", "health", "logs", "config show --no-secrets"]
    }
  }
}
```

### Enforcement

In `main()`, before dispatching to the command handler:

```go
func enforceAccess(cmd string, args []string) error {
    access := readAccessConfig(paths)
    currentUser := os.Getenv("USER")
    role := resolveRole(access, currentUser)

    if !role.canRun(cmd, args) {
        return errorf("access denied: user '%s' (role: %s) cannot run '%s'", currentUser, role.Name, cmd)
    }

    // Instance scoping: if role has instance restrictions, validate target
    if len(role.Instances) > 0 && len(args) > 0 {
        target := args[0]
        if !role.canAccessInstance(target) {
            return errorf("access denied: user '%s' cannot access instance '%s'", currentUser, target)
        }
    }

    return nil
}
```

### Audit Log: `~/.openclaw/.audit.log`

When `policy.json` has `"auditLog": true`, every claws command is logged:

```jsonl
{"ts":"2026-03-25T10:30:00Z","user":"ubuntu","cmd":"create","args":["team/alice"],"result":"ok"}
{"ts":"2026-03-25T10:31:00Z","user":"deploy-bot","cmd":"start","args":["team/alice"],"result":"ok"}
{"ts":"2026-03-25T10:32:00Z","user":"alice-user","cmd":"remove","args":["team/bob","--purge"],"result":"denied"}
```

### Token Rotation

```bash
claws token rotate <instance>    # generate new token, update env, restart
claws token show <instance>      # show current token (admin only)
```

**File:** New `token.go`

### Commands

```
claws access show                 # show current access config
claws access grant <user> <role>  # add user to role
claws access revoke <user>        # remove user access
claws access audit [--since=24h]  # show audit log
claws token rotate <instance>     # rotate gateway token
claws token show <instance>       # show token (admin only)
```

## Implementation Order

1. `access.go` — access config loading, role resolution, enforcement
2. Wire `enforceAccess()` into `main.go` before command dispatch
3. `audit.go` — JSONL audit logging
4. `token.go` — token rotation
5. `claws access` subcommands
6. Integration with policy (ticket 2) — `policy.json` enables audit log

## Testing
- Unit test: role resolution (user → role mapping)
- Unit test: command authorization (role × command matrix)
- Unit test: instance scoping (user can access own instance only)
- Integration test: unprivileged user blocked from admin commands
- Integration test: audit log written on command execution
- Integration test: token rotation generates new token and restarts
