# Ticket 3: Runtime Adapter Pattern — Support Multiple Agent Runtimes

**Priority:** P2 — Medium
**Created:** 2026-03-25
**Status:** Open

## Problem

clawctl is hardcoded to OpenClaw:
- Compose command: `node dist/index.js gateway` (`docker-compose.yml:33`)
- Health check: `fetch('http://127.0.0.1:18789/healthz')` (`docker-compose.yml:44`)
- CLI commands: `openclaw channels add`, `openclaw pairing approve` (`channel.go`, `commands.go`)
- Config format: `openclaw.json` (`config everywhere`)
- Internal port: always 18789 (`docker-compose.yml:24,37`)

But the control plane pattern (port management, groups, shared resources, task queue, proxy, storage) is generic. Other "claw-like" agent runtimes exist or could be built.

## Design

### Runtime Interface

```go
// Runtime defines how clawctl interacts with a specific agent gateway runtime.
type Runtime struct {
    Name            string   // "openclaw", "custom", etc.
    Image           string   // default Docker image
    InternalPort    int      // port the gateway listens on inside the container
    BridgePort      int      // bridge/companion port offset (0 = none)
    HealthEndpoint  string   // e.g., "/healthz"
    ReadyEndpoint   string   // e.g., "/readyz"
    ConfigFileName  string   // e.g., "openclaw.json"
    ComposeTemplate string   // path to compose template for this runtime

    // CLI commands (empty = not supported)
    ChannelAddCmd   []string // e.g., ["channels", "add"]
    PairingCmd      []string // e.g., ["pairing", "approve"]
    AuthCmd         []string // e.g., ["models", "auth", "login"]
    ConfigGetCmd    []string // e.g., ["config", "get"]
    ConfigSetCmd    []string // e.g., ["config", "set"]

    // Entrypoint
    GatewayCommand  []string // e.g., ["node", "dist/index.js", "gateway"]
    CLIEntrypoint   []string // e.g., ["node", "dist/index.js"]
}
```

### Runtime Registry: `~/.openclaw/runtimes/`

```
~/.openclaw/runtimes/
  openclaw.json         # built-in OpenClaw runtime definition
  custom-agent.json     # user-defined runtime
```

### Compose Template Per Runtime

Each runtime provides its own compose template:
```
~/.openclaw/runtimes/
  openclaw.json
  openclaw-compose.yml        # OpenClaw's compose template
  custom-agent.json
  custom-agent-compose.yml    # Custom runtime's compose template
```

### Instance-Level Runtime Selection

```bash
clawctl create alice --runtime=openclaw          # default
clawctl create bob --runtime=custom-agent        # different runtime
```

Stored in `instance.env`:
```
CLAWCTL_RUNTIME=openclaw
```

### What Changes in clawctl

| Current | With Adapter |
|---------|-------------|
| Hardcoded `openclaw.json` | `runtime.ConfigFileName` |
| Hardcoded `/healthz` | `runtime.HealthEndpoint` |
| Hardcoded `node dist/index.js gateway` | `runtime.GatewayCommand` |
| Hardcoded `channels add` | `runtime.ChannelAddCmd` |
| Hardcoded port 18789 | `runtime.InternalPort` |
| Single `docker-compose.yml` | `runtime.ComposeTemplate` |

### Commands

```
clawctl runtime list                    # list available runtimes
clawctl runtime show <name>             # show runtime config
clawctl runtime add <name> --image=...  # register a new runtime
clawctl runtime remove <name>           # remove a runtime
```

## Implementation Order

1. Define `Runtime` struct in `runtime.go`
2. Load built-in OpenClaw runtime as default
3. Add `--runtime=` flag to `cmdCreate`
4. Store runtime in `instance.env`
5. Refactor `probeInstance()` to use runtime endpoints
6. Refactor `dc()` to use runtime compose template
7. Refactor channel/auth commands to use runtime CLI commands
8. Add `clawctl runtime` subcommands
9. Document how to create a custom runtime definition

## Testing
- Unit test: runtime loading and defaults
- Integration test: create with default runtime
- Integration test: create with custom runtime definition
- Health probe works with custom endpoints
