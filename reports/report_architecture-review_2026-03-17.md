# claws Architecture Review — Orchestration & Control Plane

**Date:** 2026-03-17
**Reviewer Perspective:** Principal Architect — orchestration systems, control planes, operational correctness
**Scope:** clawctl-go codebase (~2,500 LOC Go), Docker Compose substrate, file-based state, S3 integration

---

## Executive Summary

claws is a well-scoped control plane for managing multiple OpenClaw instances via Docker Compose. The file-based state model is correct for the 1-8 instance scale. The Docker Compose abstraction layer is clean. The 4-layer config inheritance system is well-designed. However, the codebase has **no concurrency protection** (file locking), a **destructive S3 sync default**, a **half-implemented coordination layer** (manager/worker), and an **unsafe proxy setup** that overwrites system configuration. This report categorizes findings by severity and provides specific remediation guidance.

---

## 1. State Management

### Design: File-Based State Store

```
~/.openclaw/                        # OPENCLAW_ROOT
  .port-registry                    # JSON: [{name, index}] — port allocation
  .storage.json                     # S3 config
  defaults.json                     # Global config defaults
  docker-compose.yml                # Shared compose template
  shared/                           # Global shared resources
    skills/
    workspace/
  <instance>/
    instance.env                    # Instance-specific env vars (ports, tokens, paths)
    openclaw.json                   # Merged OpenClaw config
    docker-compose.override.yml     # Generated compose override (shared mounts, roles)
    workspace/
    credentials/
    ...
  <group>/
    .group.json                     # Group metadata
    defaults.json                   # Group-level config defaults
    shared/                         # Group-level shared resources
      skills/
      workspace/
      hooks/
      tasks/                        # Manager/worker task queue (filesystem-based)
        pending/
        claimed/
        done/
      output/                       # Worker output directory
    <instance>/                     # Grouped instances nested under group dir
      instance.env
      ...
```

### Assessment

**Strengths:**
- Inspectable: every piece of state is a file you can `cat`, `grep`, or back up
- No external dependencies: no database, no etcd, no Redis
- Correct for scale: 1-8 instances don't need distributed consensus
- The port registry is a single file — easy to reason about, easy to back up
- Config inheritance (global defaults -> group defaults -> template -> skeleton) is a clean layered model

**Weaknesses:**

#### P0: No File Locking

The port registry (`.port-registry`) is read-modify-written without any file locking. Every write path follows this pattern:

```go
entries, _ := readRegistry(paths)  // READ
// ... compute new entry ...
registerPort(paths, index, name)   // WRITE (appends to registry)
```

Two concurrent `claws create` calls will race on this file. The same applies to `.group.json`, `.storage.json`, and instance env files.

**Impact:** Port collision, registry corruption, duplicate entries.

**Fix:** Wrap all registry and config writes in `syscall.Flock()`:

```go
func withRegistryLock(paths Paths, fn func() error) error {
    f, _ := os.OpenFile(filepath.Join(paths.Root, ".port-registry.lock"), os.O_CREATE|os.O_RDWR, 0644)
    defer f.Close()
    syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
    defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
    return fn()
}
```

#### P2: Port Allocation Race

`nextIndex()` finds the next available port index by scanning the registry. Between the scan and the write, another process could claim the same index. This is a subset of the locking issue above but worth calling out: the port allocation should be atomic with the registry write.

---

## 2. Docker Compose Abstraction

### Design

```
compose.go: dc() / dcRun() / dcOutput()
  -> docker compose -f <template> [-f <override>] --env-file <instance.env> -p <project> <args>
```

The compose template (`docker-compose.yml`) is shared across all instances. Per-instance differentiation comes from:
1. `instance.env` — ports, tokens, image, paths
2. `docker-compose.override.yml` — generated for shared mounts and role-specific volumes

### Assessment

**Strengths:**
- Clean separation: template is immutable, override is generated
- Project naming (`openclaw-<name>` or `openclaw-<group>-<name>`) prevents container collisions
- Override generation in `rebuildGroupOverride()` correctly handles all mount combinations
- Health check in compose uses Node.js one-liner against `/healthz` — lightweight and correct

**Concerns:**

#### Compose Template Binding

The gateway binds internally to port 18789 always (`--port 18789` in the command), with the host port mapped via `OPENCLAW_GATEWAY_PORT`. This is correct — the container-internal port is fixed, only the host mapping varies. Good.

However, `OPENCLAW_GATEWAY_BIND=lan` is set at create time and baked into `instance.env`. There's no way to change it post-creation without manually editing the env file. Consider `claws config set <name> bind <value>`.

#### Override Generation

`rebuildGroupOverride()` generates YAML by string concatenation rather than marshaling a struct. This works today because the output is simple, but it's fragile:
- Volume paths with special characters (spaces, colons) will break
- No YAML escaping on any values
- The indent is hardcoded to 6 spaces with string literals

**Fix:** Use a minimal YAML struct and `yaml.Marshal`, or at minimum quote all paths.

#### Container Naming Assumption

```go
containerName := ref.ProjectName() + "-openclaw-gateway-1"
```

This assumes Docker Compose's default naming scheme. Compose v2.24+ changed the separator from `-` to `-` (consistent) but the `-1` suffix depends on scale=1. If a user manually scales or Docker Compose changes naming, this breaks silently.

**Fix:** Use `docker compose ps --format json` to get the actual container name dynamically.

---

## 3. Configuration System

### Design: 4-Layer Merge

```
mergeConfigLayers(defaultsPath, groupDefaultsPath, fromConfigPath, fromInstance, skeletonFile, outputPath)
```

Merge order: global defaults -> group defaults -> template (--from) -> instance skeleton

### Assessment

**Strengths:**
- Deep merge is correct — nested objects are merged, not replaced
- The skeleton always wins for port and auth (instance-specific values)
- `--from=` copies config from an existing instance — useful for fleet consistency

**Concerns:**

#### No Config Diffing

There's no way to see what an instance's effective config is after merge, or how it differs from defaults. A `claws config diff <name>` would be valuable for debugging.

#### No Config Validation

The merged `openclaw.json` is written to disk but never validated against OpenClaw's schema. The compose `config` validation runs (`dcOutput(paths, ref.RegistryName(), "config")`), but this only validates the Docker Compose file, not the OpenClaw config. A malformed merge could produce a config that Docker accepts but OpenClaw rejects at startup.

**Fix:** Either validate against a JSON schema, or do a dry-run gateway start that checks config and exits.

---

## 4. Security Analysis

### Token Management

- Tokens are 32-byte random hex (256 bits) — cryptographically sufficient
- `crypto/rand.Read()` is used correctly
- Token is written to `instance.env` with mode 0644 — **this is too permissive**. Credentials files should be 0600.
- Token is displayed truncated in `status` output (first 8 + last 8 chars) — good

**Fix:** `os.WriteFile(envFile, []byte(envContent), 0600)` — restrict env file permissions since it contains the gateway token.

### Proxy Security

The reverse proxy setup (`proxy.go`) has several security issues:

1. **No authentication at the proxy layer.** Caddy serves as a pass-through reverse proxy with no auth headers. The gateway token is the only protection, but it's not injected into the Caddy config. Anyone who can reach the Caddy endpoint can hit the gateway.

2. **`handle_path` strips the prefix.** The OpenClaw gateway receives requests at `/` regardless of the original path. If the gateway does any path-based routing or serves a web UI at `/`, all instances' UIs will be accessible at different paths — this may confuse the gateway's client-side routing.

3. **No TLS between Caddy and the gateway.** The reverse proxy connects to `127.0.0.1:<port>` over plain HTTP. This is acceptable for loopback but should be documented.

4. **System Caddyfile overwrite.** `proxy setup` writes directly to `/etc/caddy/Caddyfile` without backing up the existing config. If the server already runs other sites behind Caddy, they'll be destroyed.

**Fix:**
- Back up existing Caddyfile before overwriting
- Write to a Caddy include directory (`/etc/caddy/conf.d/claws.conf`) instead
- Add `basic_auth` or `forward_auth` directive with the gateway token
- Add a `--dry-run` flag that prints the config without writing

### Backup Security

`cmdBackup` creates a tarball of the entire instance directory, **including credentials/**. This is correct for a full backup but should warn the user:

```
WARNING: Backup includes credentials. Store securely.
```

The `storage sync` command correctly excludes `*/credentials/**` from S3 sync — this is good.

### Command Injection Surface

The codebase shells out extensively via `exec.Command()`. All arguments are passed as discrete parameters (not concatenated into a shell string), so there's **no command injection risk** in the current code. This is correct.

However, instance names are used directly in filesystem paths. The `validateName()` function (in `config.go`) should reject names containing `..`, `/` (for ungrouped), or null bytes. Verify this is the case.

---

## 5. Manager/Worker Coordination Layer

### Current State

The role system scaffolds a filesystem-based task queue:

```
shared/tasks/
  pending/    # Manager writes task files here
  claimed/    # Worker moves task here when picked up
  done/       # Worker moves task here when complete
shared/output/ # Workers write results here
```

Volume mounts:
- **Manager:** RW to tasks/, RW to output/, RO to each worker's workspace
- **Worker:** RO to tasks/, RW to output/, RO to manager's workspace (if `--manager=` set)

### Assessment

**The mount topology is well-designed** — managers can see everything, workers can only see the task queue and write to output. Read-only cross-mounts prevent workers from modifying each other or the manager.

**But there is no coordination protocol:**
- No CLI command to create a task
- No file format defined for task files
- No claim mechanism (atomic rename from pending/ to claimed/)
- No completion reporting
- No timeout/retry logic
- No way for a manager to discover available workers

This is the equivalent of building a message queue's storage layer but shipping no producer/consumer API.

### Recommendation

**Option A (ship minimal):** Remove `--role=manager|worker` from the create command. Ship groups with shared resources only. Add roles back when the task CLI exists.

**Option B (complete it):** Implement a minimal task lifecycle:

```
claws task create <group> --from=<manager> "summarize the Q1 report"
  -> writes JSON to shared/tasks/pending/<uuid>.json

claws task list <group> [--status=pending|claimed|done]
  -> lists tasks with status

claws task claim <group> <task-id> --by=<worker>
  -> atomic rename pending/ -> claimed/

claws task complete <group> <task-id>
  -> rename claimed/ -> done/
```

The atomic rename (`os.Rename`) is safe on local filesystems but **not safe on S3 FUSE mounts** (mountpoint-s3 doesn't support rename). This means the task queue can only work on local storage — document this constraint.

---

## 6. S3 Storage Layer

### Assessment

The storage setup flow is well-guided:
1. Check AWS CLI
2. Create bucket (idempotent)
3. Enable versioning
4. Block public access
5. Configure rclone
6. Save config

**However:**

#### P0: `rclone sync` is Destructive

`rclone sync` mirrors source to destination — files on the destination that don't exist on the source are **deleted**. If a user:
1. Sets up server A, creates instances, syncs to S3
2. Sets up server B (fresh), runs `claws storage sync`

The sync from the empty server B will **delete all data in S3**.

**Fix:** Default to `rclone copy` (additive only). Add `--mirror` flag for full sync with an explicit confirmation prompt.

#### FUSE Mount Limitations

`mountpoint-s3` has significant limitations that aren't documented:
- No `rename()` support (breaks task queue, some editors)
- Eventually consistent reads
- No file locking
- Append-only for new files (no random writes)

The workspace migration (`claws migrate --to s3`) moves the OpenClaw workspace to a FUSE mount. OpenClaw's workspace includes markdown files that agents edit frequently. This should work for reads but may have issues with concurrent writes from the agent runtime.

**Fix:** Document limitations. Consider making FUSE mount read-only and using rclone sync for writes.

---

## 7. Observability

### Health Probes

The health system probes both `/healthz` (liveness) and `/readyz` (readiness) with proper distinction:
- Live but not ready = **degraded** (container is up, some subsystem is failing)
- Not live = **down**
- Container stopped = **stopped**

The `/readyz` response includes a `failing` array of subsystem names — this is surfaced in the health table. Good design.

### Activity Feed

`activity.go` has a correctness issue:

```go
// activity.go:186
entries = append(entries, logEntry{
    Time:   time.Now(), // approximate — log timestamps vary
    Detail: truncate(strings.TrimSpace(line), 80),
})
```

All log-sourced activity entries get `time.Now()` as their timestamp instead of the actual log timestamp. This means:
- The activity feed doesn't show when errors happened
- Sorting by time is meaningless for error entries
- The `--since` filter doesn't actually filter errors by time

**Fix:** Parse Docker's log timestamp format (`2026-03-17T10:30:45.123456789Z`) from the log line prefix.

### Dashboard

The dashboard (`cmdDashboard`) clears the screen and re-renders every `--interval` seconds. Each render calls `probeInstance()` for every instance, which makes HTTP requests to `/healthz` and `/readyz` plus Docker CLI calls for container status and RAM.

For 8 instances with 2 HTTP calls + 2 Docker CLI calls each, that's 32 subprocess invocations per refresh. At the default 5s interval, this is fine. At 1s with 8 instances, it could cause visible latency.

**Consideration:** Batch the Docker CLI calls (one `docker stats --no-stream` for all containers) and parallelize the health probes with goroutines.

---

## 8. Code Quality

### Strengths

- **Zero external dependencies.** The entire codebase uses only the Go standard library. This is exceptional for the scope of functionality and eliminates supply chain risk.
- **~2,500 LOC** for lifecycle, groups, roles, storage, proxy, activity, dashboard, backup/restore, config merge, health probes. This is tight.
- **Consistent error pattern.** Every command returns `error`, main handles display and exit code.
- **Rollback on create failure.** `cleanup()` removes the directory and unregisters the port.

### Issues

#### Dead Code

```go
// main.go:162
_ = strings.Join(nil, "") // suppress unused import
```

The `strings` import is used elsewhere in the file (in `printHelp`). This line is unnecessary and should be removed.

#### Error Swallowing

Several locations silently discard errors:

| File | Line | Issue |
|------|------|-------|
| `storage.go` | 152 | `cmd.CombinedOutput()` return discarded (public access block) |
| `storage.go` | 189 | `writeStorageConfig()` error not checked |
| `group.go` | 194 | `os.MkdirAll()` errors ignored in group create |
| `commands.go` | 199-200 | `os.WriteFile` for skeleton not checked, `json.MarshalIndent` error discarded |
| `commands.go` | 916-918 | `cmdStart` errors in `cmdStartAll` are silently ignored |

#### Inconsistent Arg Parsing

The codebase uses manual `strings.HasPrefix` parsing for flags:

```go
case strings.HasPrefix(a, "--bucket="):
    bucket = a[9:]
```

This works but is error-prone (wrong offset = wrong value). Consider using `flag.FlagSet` per subcommand, or at minimum a helper function:

```go
func flagValue(arg, prefix string) (string, bool) {
    if strings.HasPrefix(arg, prefix) {
        return arg[len(prefix):], true
    }
    return "", false
}
```

---

## 9. Severity Summary

| ID | Severity | Component | Issue | Fix Effort |
|----|----------|-----------|-------|-----------|
| A1 | **P0** | State | No file locking on registry/config writes | Small — add flock wrapper |
| A2 | **P0** | Storage | `rclone sync` deletes destination-only files | Small — change to `rclone copy` |
| A3 | **P0** | Security | `instance.env` written with 0644 (contains token) | Trivial — change to 0600 |
| A4 | **P1** | Proxy | Overwrites system Caddyfile without backup | Small — backup + include dir |
| A5 | **P1** | Proxy | No authentication at proxy layer | Medium — add forward_auth |
| A6 | **P1** | Coordination | Manager/worker roles have no task lifecycle | Large — implement or remove |
| A7 | **P1** | Observability | Activity timestamps are all `time.Now()` | Small — parse Docker log timestamps |
| A8 | **P2** | Compose | Container name assumes `-1` suffix | Small — use `docker compose ps --format json` |
| A9 | **P2** | Compose | Override YAML generated by string concat | Medium — use struct + marshal |
| A10 | **P2** | Config | No config validation against OpenClaw schema | Medium |
| A11 | **P2** | Code | Error swallowing in 5+ locations | Small — add error checks |
| A12 | **P3** | CLI | No structured output (`--json`) | Medium — needed for automation |
| A13 | **P3** | Code | Manual flag parsing, no `flag.FlagSet` | Medium — refactor |
| A14 | **P3** | Code | Dead code in `main.go:162` | Trivial |

---

## 10. Architecture Diagram

```
                          claws CLI (Go binary)
                               |
                    +----------+----------+
                    |                     |
              File-Based State      Docker Compose
              (~/.openclaw/)        (subprocess calls)
                    |                     |
         +----+----+----+         +------+------+
         |    |    |    |         |             |
      .port  .env  .json  compose   docker      docker
      registry      configs  template  compose    stats
                                       up/down   ps/logs
                    |                     |
              S3 / rclone            Container Runtime
              (optional)                  |
                                   OpenClaw Gateway
                                   (Node.js, port 18789)
                                         |
                                   +-----+-----+
                                   |           |
                                /healthz   /readyz
                                (liveness) (readiness)
```

---

## 11. Recommendations by Priority

### Immediate (before any external use)

1. **Add file locking** around all registry and config writes
2. **Change `rclone sync` to `rclone copy`** as the default
3. **Fix `instance.env` permissions** to 0600
4. **Back up Caddyfile** before overwriting in proxy setup

### Short-term (before v1.0)

5. **Either complete the task lifecycle or remove `--role=`** from create
6. **Fix activity timestamps** by parsing Docker log format
7. **Add `claws version` and `claws doctor`**
8. **Quote volume paths** in generated override YAML

### Medium-term (v1.x)

9. **Add `--json` output** to list, status, health
10. **Batch Docker CLI calls** in dashboard for performance
11. **Add config validation** against OpenClaw schema
12. **Use `flag.FlagSet`** for consistent arg parsing

---

## 12. What's Architecturally Sound

Despite the issues listed above, the core design decisions are correct:

- **File-based state at this scale** avoids operational overhead without sacrificing inspectability
- **Docker Compose as execution substrate** is the right abstraction — it handles restart policies, health checks, networking, and image management
- **Template + override separation** keeps the shared compose file immutable while allowing per-instance customization
- **4-layer config merge** mirrors how operators think about defaults and overrides
- **Zero external Go dependencies** eliminates supply chain risk and simplifies builds
- **Health probes distinguishing liveness from readiness** is production-grade observability

The foundation is solid. The issues are in the edges — concurrency, security hardening, and feature completeness — not in the core architecture.

---

*Report generated from source review of clawctl-go @ commit cd0e260 on branch master.*
