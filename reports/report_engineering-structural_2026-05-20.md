# Engineering Structural Map — clawctl

**Date.** 2026-05-20
**Branch.** `feature/runtime-adapter` (commits `405f1b4`, `f4a3ba3`)
**Scope.** Full structural mapping for the "open it up" decision: what it does, how data flows, what's stub-or-missing, interface quality, current state, and an LLM-as-judge rubric.
**Source totals.** ~15.1k LOC total. ~9.4k LOC of production Go across 24 source files, ~5.7k LOC across 19 `_test.go` files (232 tests, full suite `go test ./...` clean in ~40s).
**Companion deliverables.** [PMM/positioning report](report_pmm-opensource_2026-05-20.md); [test plan + dogfood log](../tests/test_plan_2026-05-20.md).
**Prior context honored.** This report builds on but does not re-derive [architecture review 2026-03-17](report_architecture-review_2026-03-17.md) and the [PMM audit 2026-03-27](report_pmm-audit_2026-03-27.md). Where those flagged P0s have been fixed, I say so explicitly.

---

## 1. What this is, in one paragraph

`clawctl` is a single-binary Go CLI (~9.4k LOC, **zero external dependencies** — only the Go standard library) that operates a small fleet (1-8 by default cap) of containerized AI-agent **instances** on **one host**. It owns four concerns: (1) **lifecycle** — create/start/stop/remove containers via Docker Compose; (2) **state** — port allocation, instance config, multi-tenant overlays, all as plain files under `OPENCLAW_ROOT` (default `~/.openclaw/`); (3) **policy** — admin-set guardrails on bind modes, images, channels, and outbound messaging, plus role-based access control over which OS user can run which subcommand; (4) **coordination** — optional group/manager/worker topology with a filesystem-based task queue and shared workspace mounts. The native runtime is OpenClaw (a Node.js gateway image), but a **runtime-adapter** layer lets clawctl drive any container that exposes a health endpoint.

It is **not** Kubernetes, not Nomad, not a multi-host orchestrator. The competitive analogy is `multipass`, `lxc/lxd`, or `docker compose --profile` — first-class instance identity on one box for an operator with SSH.

---

## 2. Module map

### 2.1 Source files (production)

```
main.go              314  Command dispatcher, first-run welcome, audit-log wrapping
commands.go         1569  Lifecycle: create/start/stop/restart/remove/list/status/health/
                          dashboard/backup/restore/share/unshare/start-all/stop-all/tunnel/
                          stats/auth/logs/exec
config.go            221  Paths, validateName, hostBind, perms fixers, color/log helpers
compose.go           183  dc()/dcRun()/dcOutput() wrappers around `docker compose`,
                          container-name resolution, RAM reader
runtime.go          1044  Runtime struct + capabilities + built-in openclaw runtime +
                          `runtime list|show|add|init|test|export|import|detect|remove`
registry.go          150  Append-only .port-registry file, next-index allocator
flock.go              45  syscall.Flock wrappers (withFileLock, with{Registry,Group,Instance}Lock)
merge.go              83  deepMerge + 4-layer config merge for openclaw.json
shared.go            160  yamlVolume formatter, SharedFlags, rebuildOverride (ungrouped path)
group.go             678  InstanceRef parse, GroupConfig, group create/list/add/remove/shared,
                          rebuildGroupOverride (grouped/role-aware), team shortcut, role mgmt
channel.go           885  Per-channel profiles, safe defaults, add/remove/status/security/
                          send/allow/deny, approve (pairing), nested-config helpers
task.go              466  Task struct, FUSE-mount guard, task create/list/claim/complete/status
configcmd.go         224  config show/get/set/edit, secret masking
storage.go           663  S3 setup (aws bucket+versioning+block public), rclone sync,
                          mountpoint-s3 mount, cron, migrate workspace to S3
proxy.go             327  Caddy include-dir config generation, install, reload, status
access.go            464  AccessConfig, role resolution, enforceAccess (called pre-dispatch),
                          token rotate/show, access init/show/grant/revoke/audit
policy.go            473  Policy struct, enforcement helpers (bind/image/channel/dm/outbound),
                          policy init/show/validate/enforce --restart
activity.go          251  recentFiles (mtime scan), recentLogErrors (parse docker logs)
image.go             230  image list/pull/pin, upgrade with health-check rollback
init.go              189  First-run setup: dirs, policy, access, compose-template copy, doctor
setup.go             491  Guided interactive `clawctl setup` (6-step flow, --non-interactive)
help.go              771  printSubcommandHelp + topic help (setup/security/channels/groups/commands)
doctor.go            193  doctor (env checks) + version (build info + tool versions)
audit.go              46  Wraps scripts/security-audit.sh; falls back to doctor + policy validate
```

### 2.2 Layer view

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        OPERATOR CLI surface                              │
│ setup · init · doctor · create · start · status · health · list · group  │
│ team · runtime · channel · task · config · policy · access · audit · …   │
└─────────────────────┬───────────────────────────┬────────────────────────┘
                      │ dispatch + access check    │ audit wrap
                      ▼                            ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                          ENFORCEMENT layer                                │
│  access.enforceAccess()  ─  policy.enforce*()  ─  validateName()         │
│  withRegistryLock / withGroupLock / withInstanceLock (syscall.Flock)     │
└─────────────────────┬─────────────────────────────────────────────────────┘
                      ▼
┌─────────────────────────────────────┬───────────────────────────────────┐
│        STATE layer (files)          │   EXEC layer (subprocess)         │
│  ~/.openclaw/                       │   compose.go: dc/dcRun/dcOutput   │
│    .port-registry  (atomic alloc)   │     → `docker compose -f tmpl     │
│    policy.json     .access.json     │        [-f override] --env-file   │
│    .audit.log      defaults.json    │        -p <project> <args>`       │
│    docker-compose.yml               │   storage.go: aws / rclone /      │
│    runtimes/<name>.json (+ -compose)│        mount-s3 / crontab         │
│    <instance>/                      │   proxy.go: caddy / systemctl     │
│      instance.env  openclaw.json    │   audit.go: bash scripts/         │
│      docker-compose.override.yml    │     security-audit.sh             │
│      workspace/ credentials/ ...    │   activity.go: docker compose     │
│    <group>/                         │     logs --tail --timestamps      │
│      .group.json defaults.json      │                                   │
│      shared/{skills,workspace,      │                                   │
│              hooks,tasks/pending|   │                                   │
│              claimed|done,output}   │                                   │
│      <instance>/...                 │                                   │
└─────────────────────────────────────┴───────────────────────────────────┘
                                        │
                                        ▼
                            ┌───────────────────────────┐
                            │  Container Runtime        │
                            │  (OpenClaw / Custom)      │
                            │  port 18789+ → /healthz   │
                            └───────────────────────────┘
```

---

## 3. Core entities and lifecycle

### 3.1 Instance

```
create  → instance.env (0600), openclaw.json (merged), skeleton dirs,
          port registered under flock, compose override if shared/grouped
start   → docker compose up -d <gateway-service>, poll /healthz up to 30s
stop    → docker compose stop
restart → docker compose restart (--hard re-creates the container)
remove  → docker compose down, unregister port; data kept unless --purge
backup  → tar czf <name>-backup-<ts>.tar.gz (warns about credentials)
restore → tar xzf, re-register port, rewrite env paths/name if renamed
upgrade → save current image, swap, up -d, poll health 30s, rollback on fail
```

### 3.2 Group / role

A group is just a directory containing `.group.json` and `shared/`. Instances under it inherit auto-shared `skills/workspace/hooks` mounts plus a per-group `tasks/` queue. Two roles compose with shared mounts:

- **manager** — rw mount of `shared/tasks`, rw of `shared/output`, ro of *each* worker's `workspace/` (mounted at `/home/node/.openclaw/workers/<name>`). Generated dynamically in `rebuildGroupOverride()` (`group.go:454`) when a manager exists.
- **worker** — ro mount of `shared/tasks`, rw of `shared/output`, and optionally ro of the manager's workspace.

`group.go:646` re-renders the manager's override whenever a new worker is added so it sees them.

### 3.3 Task

`task.go:14` — JSON file in one of three sibling directories. Lifecycle:

```
                     atomic os.Rename                atomic os.Rename
pending/<id>.json  ────────────────►  claimed/      ────────────────►  done/
                                       (sets claimed_by/at)             (sets completed_at/result)
```

The atomicity invariant only holds on local POSIX filesystems. `isFuseMount()` (`task.go:31`) refuses to operate if the task directory is on a FUSE mount (e.g., `mountpoint-s3`). README and inline error message both call this out.

### 3.4 Runtime

`runtime.go:19` — a 40-field struct that captures every configurable thing about how clawctl talks to a containerized agent: image, compose service name(s), internal port, health/ready endpoints, container home, mount points, env-var names, CLI commands for channel/auth/config operations, and a capability flags struct (`Channels`, `Pairing`, `Auth`, `Config`, `Tasks`, `Shared`, `Bridge`). Built-in `openclawRuntime()` is hardcoded; user-defined runtimes are JSON files under `runtimes/`. Per-instance binding via `CLAWCTL_RUNTIME` in `instance.env`; `resolveRuntime()` (`runtime.go:217`) looks it up.

### 3.5 Policy

`policy.go:12` — administrator-set guardrails. Enforced at command time (`commands.go:159-176` for create; equivalents in `image.go`, `channel.go`). Fields and their enforcement points:

| Field | Enforced where | Effect |
|-------|----------------|--------|
| `allowedBindModes` | `cmdCreate`, `policy enforce` | Reject `--bind=` outside list; fixer rewrites to first allowed mode |
| `maxInstances` | `cmdCreate` | Reject create over cap (default 8) |
| `memoryLimitMB`, `cpuLimit` | (not enforced — informational only) | Reflected in audit script |
| `allowDockerSocket` | (not enforced — informational) | Audit script checks for socket mounts |
| `requireSandbox` | (not enforced — only warned at create) | Sandbox warning printed when nil |
| `requireDmPairing` | `cmdChannelAdd`, `policy validate/enforce` | Reject DM policies looser than `pairing` |
| `requireOutboundAllowlist` | Same | Reject `sendMessage:true` with empty `allowFrom` |
| `blockedChannels` | Same | Reject add of blocked channel |
| `allowedImages` | `cmdCreate`, `cmdImagePull/Pin`, `cmdUpgrade` | Glob-match against pattern list |
| `auditLog` | `writeAuditLog` in `main.go` | When true, append every command to `.audit.log` |

### 3.6 Access

`access.go:13` — RBAC over the CLI itself. Resolved by `enforceAccess()` (`access.go:67`) called from `main.go:24` *before* dispatch. Three default roles: `admin` (all commands), `operator` (read + lifecycle, no config-edit/policy/access), `user` (read-only). Per-role `instances` glob lets you scope a user to specific instances.

---

## 4. Data flow (the four canonical paths)

### 4.1 Operator types `clawctl create alpha`

```
main.go:14 cmd=create, args=[alpha]
  └─ enforceAccess(paths, "create", ["alpha"])            access.go:67
       └─ readAccessConfig → resolveRole(USER) → check Commands
  └─ cmdCreate(["alpha"])                                 commands.go:26
       ├─ parse flags (--from, --role, --bind, --image, --runtime, etc.)
       ├─ getRuntimeByName(paths, "openclaw")             runtime.go:208
       ├─ ParseRef("alpha")                               group.go:18
       ├─ resolve image: --image > $OPENCLAW_IMAGE > runtime.DefaultImage
       ├─ readPolicy(paths)                               policy.go:38
       │    enforceBindPolicy("loopback") / enforceImagePolicy(image) / enforceMaxInstances
       ├─ lockedAllocatePort(paths, "alpha")              registry.go:118
       │    └─ withRegistryLock → nextIndex → registerPort (append .port-registry)
       ├─ generate 256-bit token (crypto/rand)            commands.go:16
       ├─ mkdir credentials/agents/identity/workspace/sessions/canvas/logs
       ├─ write instance.env (mode 0600) with token, ports, bind, image, runtime
       ├─ mergeConfigLayers(global, group, --from, skeleton)  merge.go:27
       │    → ~/.openclaw/alpha/openclaw.json (0600)
       ├─ set tools.profile = "coding" if missing       configcmd.go:setNestedConfig
       ├─ warn if agents.defaults.sandbox is nil
       ├─ rebuildGroupOverride OR rebuildOverride (if shared/grouped)
       │    → docker-compose.override.yml (auto-generated)
       └─ validate compose: `docker compose -f tmpl -f override config`
                              (skipped if CLAWCTL_SKIP_VALIDATE set)
  └─ writeAuditLog(paths, "create", ["alpha"], "ok")     access.go:128
       (appends a JSON line to .audit.log iff policy.AuditLog)
```

### 4.2 `clawctl start alpha` → healthy

```
main.go:32 cmd=start
  └─ enforceAccess                                       access.go:67
  └─ cmdStart                                            commands.go:446
       ├─ requireInstance (instance.env must exist)      config.go:108
       ├─ dcRun("up", "-d", "openclaw-gateway")          compose.go:58
       │    └─ exec.Command("docker", "compose",
       │            "-f", composeTemplate, "-f", override,
       │            "--env-file", instance.env,
       │            "-p", project_name, ...args)
       └─ poll http://127.0.0.1:<port>/healthz every 2s × 15
           (runtime.HealthEndpoint and host:port from instance.env)
  └─ writeAuditLog "ok"
```

### 4.3 Manager dispatches a task to a worker

```
clawctl task create team "review PR #42"
  └─ cmdTaskCreate → write team/shared/tasks/pending/<id>.json
                     (Task{ID, Title, CreatedBy, CreatedAt, Status:"pending"})

(out-of-band: worker container's gateway watches the mount; or operator runs:)
clawctl task claim team <id> --by=dev1
  └─ isFuseMount guard
  └─ os.Rename pending/<id>.json → claimed/<id>.json     ← atomic on local FS
  └─ rewrite file with Status:"claimed", ClaimedBy/At

clawctl task complete team <id> --result="approved"
  └─ os.Rename claimed/<id>.json → done/<id>.json
  └─ rewrite file with Status:"done", CompletedAt, Result
```

Volume mounts (set up at `create --role=...` time) give the manager rw to `/home/node/.openclaw/tasks` and the worker ro to the same path, plus rw to `/home/node/.openclaw/output`. The agent inside the container interacts with these via its own runtime; clawctl provides the queue, not the consumer.

### 4.4 Operator runs `clawctl status` (overview)

```
cmdStatusOverview                                        commands.go:697
  ├─ readRegistry → for each instance:
  │    probeInstance(name)                               commands.go:1316
  │       ├─ containerStatus via `docker compose ps --format '{{.Status}}'`
  │       ├─ GET /healthz (liveness) → live bool
  │       └─ GET /readyz (readiness) → ready bool + failing[]
  │    classify: healthy | degraded | stopped | down
  ├─ Policy validation:
  │    readPolicy → for each instance:
  │       enforceBindPolicy + enforceImagePolicy +
  │       (per-channel) enforceChannelPolicy + enforceDmPolicy +
  │                     enforceOutboundAllowlist
  ├─ Audit on/off
  └─ Access configured?
```

A single screen shows: instance count, per-instance health line, policy violation count or ✓, audit log status, access-control status. This is the unified status that the March PMM audit asked for, now implemented.

---

## 5. What's stub-or-missing

### 5.1 Stubs that exist as scaffolding only

- **`OPENCLAW_BRIDGE_PORT`** — every instance allocates a bridge port (gateway+1) and reserves the host mapping in `docker-compose.yml`. No documented consumer in this repo; the OpenClaw image presumably uses it. Treat as a runtime-private contract.
- **Capability fields `Pairing`, `Bridge`** — used to gate UI messaging in `runtime show` and `cmdApprove`, but no failure mode for runtimes that lie about supporting them.
- **`activity` log-error parsing** — calls `docker compose logs --tail 200` and grep-matches "error|fatal|crash". Approximate but functional; the March audit's "wrong timestamps" complaint is fixed (`parseDockerLogTimestamp` works).

### 5.2 Missing entirely (gaps clearly visible to the reader)

| Gap | Where it would live | Impact |
|---|---|---|
| `task list --json`, `channel status --json`, `channel security --json`, `task status --json` | Trivial flag in each | Blocks scripting/CI use cases |
| `policy validate --json` | Same | Same |
| `clawctl config diff <name>` (effective config vs defaults) | new in `configcmd.go` | Hard to debug merge order issues |
| OpenClaw config schema validation post-merge | `merge.go` / new validator | Today: malformed merge ships to the container, fails at startup |
| Cross-instance event/callback firing | new "bus" file or transport | The 2026-03-17 INTEGRATION_ANALYSIS lists this as P3 future work |
| Webhook routing per instance through proxy | `proxy.go` extension | Same doc, P1 future work |
| Cross-host coordination | (out of scope by design) | If clawctl ever wants to be a generic agent control plane on N hosts, you're rewriting the registry layer |
| Structured release artifacts: binary distribution, package manager | `scripts/release.sh` exists, no CI yet | Today, "install" = clone + go build |
| `clawctl init --force` is not in help text | `help.go` `subcommandHelp["init"]` | LOW; works but undocumented |
| `clawctl audit` scopes to **all** host containers, not `$OPENCLAW_ROOT` | `scripts/security-audit.sh` step 3 | MEDIUM; informative but conceptually wrong (see dogfood log S2) |

### 5.3 The "second name" problem

The biggest *latent* cost is that two different conceptual axes share the name "bind":

- `openclaw.json` → `gateway.bind` is the *gateway's* internal listen mode (the JSON skeleton hardcodes `"lan"`).
- `instance.env` → `OPENCLAW_GATEWAY_BIND` is the same gateway's `--bind` CLI flag, set from `--bind=` and honored at runtime.
- `instance.env` → `OPENCLAW_HOST_BIND` is the Docker port-mapping host (127.0.0.1 vs 0.0.0.0), derived from `hostBind(mode)`.

`config show alpha` shows `gateway.bind: "lan"` even when the container is running with `--bind loopback`. Cosmetic, but a confusing first impression on a security-conscious operator. See dogfood log S1.

---

## 6. Interface quality

### 6.1 Public surface

- **CLI surface (~50 subcommands)** organized into 16 help sections. Top-level dispatch is a single `switch` in `main.go:30` — no flag library, no command framework. Adding a command is one switch case + one function. **Strength:** very low ceremony; new contributors can ship a command in 30 lines. **Weakness:** every command parses its own flags by manual `strings.HasPrefix` (typoed offsets are silent bugs); no introspection (no `clawctl <cmd> --json-schema`).
- **Go API surface (within `package main`):** there is none — everything is internal. **Implication for "opening it up":** if anyone outside this repo wants to *embed* clawctl as a library (e.g., another tool that wants to drive instances programmatically), they have to fork. Splitting into `cmd/clawctl` (main) + `pkg/clawctl` (library) is the precondition for any embeddability story.
- **Filesystem-as-API:** because state is files, anyone can write a parallel reader (e.g., a web UI that watches `.port-registry`, `.audit.log`, `tasks/`). This is a real interface and worth promoting. The README hints at it ("inspectable: every piece of state is a file you can cat, grep, or back up"), but there is no documented spec — schemas live in code.

### 6.2 What's good

- **`Runtime` adapter pattern.** 40-field JSON schema with capability flags lets users drive a Python or Rust agent with no Go changes. `runtime add --from=openclaw` for compatible forks, `runtime init <name>` for ground-up agents, `runtime detect <image>` to auto-fill from an image. This is the single best architectural decision in the codebase.
- **File-based state at this scale.** No DB, no etcd, no Redis. Atomic operations via `syscall.Flock()`; atomic queue transitions via `os.Rename()` with an explicit FUSE-mount guard. The March P0 ("no file locking") is fully addressed by `flock.go` + `lockedAllocatePort`/`lockedUnregisterPort`/`withGroupLock`/`withInstanceLock`.
- **Two-layer compose model.** Shared `docker-compose.yml` template + per-instance `docker-compose.override.yml` for shared mounts and role-aware volumes. The override is *generated* by clawctl (`rebuildGroupOverride` is the most subtle file in the repo); template is immutable.
- **4-layer config merge.** global defaults → group defaults → `--from` template → instance skeleton (skeleton always wins for port/auth). The merge correctly strips per-instance fields (`allowFrom`, `groups`, `actions`) when copying from a template (`merge.go:54`).
- **Security defaults that match the threat model.** Loopback by default; `cap_drop: ALL` and `no-new-privileges`; instance.env 0600; outbound messaging off by default; DM pairing required by default; `policy init` and `access init` run automatically inside `clawctl init`; audit log on by default. The PMM audit's "safe defaults" matrix is now entirely green.
- **Doctor + setup + tiered help.** A new operator gets a welcome screen, can run `setup` for one-command zero-to-running, and has topic help under `help setup|security|channels|groups|commands`. The March PMM gap ("no first-run experience, no doctor") is closed.

### 6.3 What's mediocre

- **Manual flag parsing.** Every command re-implements `strings.HasPrefix(a, "--foo=")`. No `flag` package, no library. Has caused at least one historical bug (the March review flagged a `--from=` offset mismatch). The case for a tiny `argparse` helper or `flag.FlagSet` is strong; the cost is one PR.
- **JSON parity gap.** `list`, `status`, `health`, `runtime show` have `--json`. `task list`, `channel status`, `channel security`, `policy validate`, `access audit`, `group list` do not. Scripting from another tool requires text parsing.
- **Naming inconsistency.** "Instance" in code, "agent" in user-facing copy, "team" and "group" used interchangeably (and `team list` falls back to a `group list` that says "GROUP" in the header). Cosmetic until the docs/site exist, then it's a public surface concern.
- **`status <name>` shells `docker compose ps` even when the container does not exist**, producing an empty-table footer. Cosmetic.
- **`audit` script's scope leak.** Audits all containers on the host even with a non-default `OPENCLAW_ROOT`. Surprising during scripted/CI runs.

### 6.4 What's risky

- **`proxy setup` still touches `/etc/caddy/`** (sudo install, sudo write, sudo systemctl reload). It now writes to `conf.d/clawctl.conf` and backs up existing — improvement on March's "overwrites system Caddyfile". But it remains a privileged op with side-effects beyond `OPENCLAW_ROOT`. Acceptable for self-hosted operators; surprising for a "personal AI agents" tool.
- **`storage cron --enable` modifies the operator's user crontab** with a `# clawctl-storage-sync` marker. Easy to undo, but again a host-scope mutation that is not contained in `OPENCLAW_ROOT`.
- **`policy enforce --restart`** rewrites instance configs *and* runs `dcRun "down"` + `up -d` against every affected instance. There is no `--dry-run`. Destructive by intent, but operators should be able to preview.

---

## 7. Current state, in one line per area

| Area | State |
|---|---|
| Build / vet / tests | **Green.** `go build` clean. `go vet` clean. `go test ./...` → 232 tests, ~40s, all pass. |
| Test coverage shape | 22 integration tests (build & invoke binary against `t.TempDir()` root); ~210 unit tests covering parsing, merging, policy, access, runtime, registry, locking, channels, tasks, storage, doctor, help. Integration tests intentionally avoid Docker via `CLAWCTL_SKIP_VALIDATE`. |
| Dependencies | **Zero** (stdlib only). Supply-chain risk = none. |
| Branch | `feature/runtime-adapter` is ahead of `main` with adapter pattern, admin policy, access control, image management, security matrix, runtime UX, and onboarding. Two recent commits with placeholder messages (`.`); needs a clean history before publishing. |
| Modified working tree | 7 modified, 7 untracked. Untracked include `setup.go` (491 LOC, used by tests + main), `setup_test.go`, `scripts/install.sh`, `scripts/release.sh`, three HTML mockups (`channels-guide.html`, `team-guide.html`, `landing*.html`), and the new `reports/tasklist_2026-03-27_onboarding.md`. `setup.go` is on disk and compiled into the binary I just tested — it's untracked, not unmade. |
| README ↔ binary drift | Zero observed. Every command in README "Commands" exists with the documented behavior. The `mountpoint-s3 rename` note matches the runtime guard. |
| Smoke pass rate | 81 / 81 (see `tests/dogfood_log_2026-05-20.md`). |
| March P0s | A1 (locking) **done**. A2 (`rclone sync` default destructive) **done** (now `rclone copy`, `--mirror` confirms). A3 (env file 0600) **done**. A4 (Caddyfile overwrite) **done** (include-dir + backup). |
| Open architecture follow-ups (P1/P2) | A6 (manager/worker task lifecycle) **done** — full `task` CLI shipped. A7 (activity timestamps) **done**. A8 (container-name `-1` suffix) **partially done** — `resolveContainerName` now tries `docker compose ps --format json` first, falls back to convention. A9 (override YAML generated by string concat) **still open** — pragmatic but fragile if a path ever contains a YAML metacharacter. A10 (no OpenClaw config schema validation) **still open**. A11 (error swallowing) **partially fixed**. A12 (`--json` everywhere) **partially done**. |

---

## 8. LLM-as-judge rubric

Scored 1-5 where 1 = absent/broken and 5 = production-grade. Total: **127 / 150** ≈ 4.2 / 5.

| # | Criterion | Score | Rationale |
|---|-----------|------:|-----------|
| 1 | **Problem fit** — does the code do what the README says? | 5 | Every README claim verified end-to-end in the smoke run. |
| 2 | **Architectural coherence** — does one design philosophy run through it? | 5 | "Files for state, Compose for execution, generated overrides for differentiation" is consistent everywhere. |
| 3 | **Code quality** — readable, conventional, idiomatic? | 4 | Idiomatic Go, low ceremony. Manual flag parsing and string-built YAML are the two style smells. |
| 4 | **Modularity** — can pieces be replaced without rewriting neighbors? | 4 | Runtime adapter is the standout (40-field JSON contract). Compose substrate is swappable in principle. Storage backend is hard-coded to rclone/S3. |
| 5 | **Test coverage + harness** | 5 | 232 tests, full lifecycle covered, integration tests use `CLAWCTL_SKIP_VALIDATE` cleanly so the harness runs without Docker. |
| 6 | **Failure modes — destructive ops are gated** | 4 | `--purge` and `--mirror` prompt without `--yes`. `policy enforce --restart` lacks `--dry-run`. Crontab/Caddy mutations are user-implicated. |
| 7 | **Concurrency safety** | 4 | `flock()` on registry, group, and instance writes. Atomic `os.Rename` for tasks with FUSE guard. Health probes are serialized (not parallelized — slight scalability cost at high refresh rates). |
| 8 | **Security defaults** | 5 | Loopback default, no Docker socket, cap_drop ALL, no-new-privileges, 0600 secrets, DM pairing required, outbound off, audit on. Policy + access auto-created on `init`. |
| 9 | **Observability** | 4 | Health (live+ready+failing), dashboard, activity (file mtime + log error parsing), audit log, security audit. Lacks per-instance metrics export and structured event stream. |
| 10 | **Operator DX (CLI + help + errors)** | 4.5 | First-run welcome, tiered help, error→fix hints, JSON output on the most common commands. JSON-parity gap and naming inconsistency drop it from 5. |
| 11 | **Discoverability** — would a new operator find what they need? | 4.5 | `clawctl` (no args), `clawctl setup`, `clawctl help <topic>`, `clawctl <cmd> --help` all exist. README is concise and accurate. No man page. |
| 12 | **Documentation completeness** | 4 | `README.md`, `docs/channels.md` (~500 lines), `docs/runtimes.md` (~150 lines), tickets/, reports/. No file-format reference (operators have to read source to know what `instance.env`/`openclaw.json` keys mean). |
| 13 | **Extensibility** — can a third party add a runtime/channel/storage backend without touching core? | 4 | Runtimes: yes, `runtime add --from=...` or `runtime init` covers it. Channels: harder — adding a new channel requires editing `channelProfiles` + `channelSafeDefaults` + (likely) the OpenClaw image. Storage backend: hardcoded. |
| 14 | **Release readiness** | 3.5 | `scripts/install.sh` and `scripts/release.sh` exist for binary distribution; no CI pipeline yet; placeholder commit messages on the active branch; untracked `setup.go` makes `git status` noisy. |
| 15 | **Open-source readiness (license, CoC, contributing)** | 3 | LICENSE: not present in working tree. CONTRIBUTING.md: not present. SECURITY.md: not present. CODE_OF_CONDUCT.md: not present. CI workflow file: not present. These are the cheapest pre-publication blockers to clear. |

**Aggregate verdict.** The control-plane is past prototype and into "ship to internal users / curated beta." Public open-source publication is one short hardening sprint away (Section 9).

---

## 9. Where to focus to "open it up"

In rough order of cost/risk vs. value:

1. **Repository hygiene** (hours). Add `LICENSE` (pick one), `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`. Commit `setup.go` and friends with real messages, squash the two `.` commits. Add a GitHub Actions workflow that runs `go test ./...` and `go vet`.
2. **Documentation reference layer** (1 day). Write a "file format reference" page that documents `instance.env`, `openclaw.json` skeleton, `policy.json`, `.access.json`, `.audit.log`, `.port-registry`, runtime JSON schema. Today these live only in code; without docs, the "files as API" promise is undocumented.
3. **JSON parity** (half a day). Add `--json` to `task list`, `channel status`, `channel security`, `policy validate`, `access audit`, `group list`. Cheapest scripting-friendliness improvement.
4. **Library split prep** (1-2 days). Move pure logic (runtime registry, config merge, policy enforce, access check, registry, flock, task queue) into `internal/clawctl/` packages with a stable Go API. Keep `main.go` as the thin shell. Don't promise *external* package stability yet, but make embeddability mechanically possible.
5. **CI binary distribution** (half day). Wire `scripts/release.sh` into a tag-triggered GitHub Action, publish releases, point `scripts/install.sh` at them. Optionally a Homebrew tap.
6. **Address dogfood findings** (2-3 hours): fix `audit` scope (S2), JSON-skeleton bind discrepancy (S1), `status <name>` empty-table noise (S5), document `init --force` (S4).
7. **Override YAML safety** (a few hours). Replace string concatenation in `rebuildGroupOverride` with a struct + `yaml.Marshal` (would introduce one dependency — `gopkg.in/yaml.v3`). The "zero deps" badge is sacred to this project; an alternative is per-field quoting + a fuzz test. Either is fine.
8. **Schema validation** (longer). Either ship a JSON schema for `openclaw.json` and validate post-merge, or do a `gateway --check-config` exec inside a container at create time. This unblocks a class of "ships, then fails at start" bugs.

Items 1-3 are the bare minimum to ship to the public; 4-5 are the difference between "another vendored tool" and "a platform people can build on."

---

*End of structural report. Companion: [PMM/open-source positioning report](report_pmm-opensource_2026-05-20.md).*
