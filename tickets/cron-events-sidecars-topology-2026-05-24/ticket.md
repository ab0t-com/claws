# cron + events + sidecars + topology

**Filed:** 2026-05-24
**Target:** v1.5.0
**Status:** Open

## Background

v1.4.0 shipped the hook-adapter + namespaced templates + URL-loaded
resources. User dogfood pass surfaces four follow-ups that touch the
real shape of how claws agents collaborate with the rest of the
ecosystem:

1. **Cron section in templates.** Periodic actions (heartbeats, daily
   summaries, backup snapshots, RAG re-indexing) currently require
   external cron config. Templates should declare them inline.

2. **Event injection / digest endpoint.** As the sibling project
   `../intent-gateway` matures, agents need a standard way to *receive*
   events from outside (webhooks, queues, ingest pipelines). The
   template should declare "this agent accepts events, optionally in
   digest mode" so the gateway can route correctly.

3. **First-class sidecar helpers.** Two sibling CLIs already exist:
   - `../sharedwatch` — SQLite-backed file-watcher for multi-agent
     coordination. MIT, single Go binary.
   - `../intent-gateway` — event ingest + intent routing.

   And there will be more. Templates should be able to declare
   "this template runs sharedwatch watching the shared workspace"
   and have claws materialise the appropriate sidecar config.

4. **Topological structures for teams.** v1.4 supports manager/worker
   pairs (one-level hierarchy). Real teams have multi-tier hierarchies
   (lead-of-leads → team-leads → workers), peer meshes (3 specialists
   collaborating), and hybrid shapes. The schema should be expressive
   enough — and the bundled templates should demonstrate each shape.

## Goal

Make claws templates a complete *system description* — periodic work,
event ingest, helper sidecars, and arbitrary team topology — without
breaking the v1.4 schema. Plus: lift the test coverage bar so every
new field is covered.

## Scope — In

### Phase A — Cron section (per-agent)

- **A1.** `agents[].cron: [{name, schedule, command|hook|exec, timezone?, enabled?}]`
  - `schedule`: standard 5-field cron OR `@hourly|@daily|@weekly|@monthly|@reboot` OR `every <duration>` (Go duration syntax)
  - `command`: inline shell command (mutually exclusive with `hook`/`exec`)
  - `hook`: name of an event in `agents[].hooks` (DRY — reuses hook content)
  - `exec`: array form (Docker-friendly; no shell interpretation)
- **A2.** Materialise to `<instance>/workspace/cron/<name>.crontab` or
  similar — runtime adapter declares the path/format.
- **A3.** Runtime adapter additions:
  - `Runtime.CronDir string` (workspace-relative)
  - `Runtime.CronFormat string` (`"crontab" | "systemd-timer" | "json"`)
  - `Runtime.SupportsCron bool` (in Capabilities)
- **A4.** OpenClaw defaults: `CronDir = "cron"`, `CronFormat = "crontab"`.
- **A5.** Validation: invalid schedule → reject at parse-time.

### Phase B — Event injection

- **B1.** `agents[].events: {enabled, digestMode, endpoint?, allowFromIps?}`
  - `enabled: bool` — whether the agent accepts external events
  - `digestMode: bool` — when true, events are batched into periodic
    digests rather than processed individually (intent-gateway pairing)
  - `endpoint: string` — relative path the runtime should expose
    (default: `/events/<agent-name>`)
  - `allowFromIps: [string]` — CIDR allowlist; empty = any
- **B2.** Apply: writes `openclaw.json` `events.{enabled, digestMode, ...}`
  via cmdConfig set — runtime decides whether/how to expose the endpoint.
- **B3.** Capability gate: only apply if `Runtime.Capabilities.Events`
  is true (new capability flag).

### Phase C — First-class sidecar helpers

- **C1.** New top-level `sidecars: [{name, kind, config}]` block AND
  `agents[].sidecars: […]` for per-agent.
  - `kind: "sharedwatch" | "intent-gateway" | "custom"`
  - `config: {…}` — sidecar-specific config (e.g. for sharedwatch:
    `{watchDir, actor, cursorName, leasePatterns}`)
- **C2.** Sidecar registry: each sidecar declares how it integrates
  (env-var contributions, mount points, lifecycle hook injection).
  - `sharedwatch` integration: declares an `onStart` hook that runs
    `sharedwatch watch --root <workspace> --actor <agent>` in the
    background, and a `claws config set sidecar.sharedwatch.enabled true`.
  - `intent-gateway` integration: writes the agent's
    events-endpoint config + registers the agent with the gateway
    via a side-file the gateway reads.
- **C3.** Each sidecar is OPT-IN — claws doesn't pull/install the
  sidecar binary itself; it just configures the integration. The
  operator installs sharedwatch/intent-gateway separately (their own
  one-click installers).

### Phase D — Topology

- **D1.** Schema extension: `agents[].peers: [string]` — explicit peer
  references (non-hierarchical). Used for: shared task queue at a tier,
  mesh communication, mutual visibility.
- **D2.** `agents[].manager` already exists — extend to support
  multi-level (the named agent doesn't need to be `role: "manager"`,
  just any agent in the team).
- **D3.** Apply: writes per-agent `topology.json` listing
  `manager`, `peers`, `workers` (auto-derived from who declares this
  agent as manager).
- **D4.** Validation: topology must be acyclic for manager chains;
  no agent can have itself as manager.
- **D5.** New bundled template `teams/multi-tier.json` — lead-of-leads
  + 2 team leads + 4 workers, demonstrating depth-2 hierarchy.
- **D6.** New bundled template `teams/specialist-mesh.json` — 3 peers
  with no hierarchy, shared workspace.

### Phase E — Test coverage audit + fill

- **E1.** Run `go test -cover` baseline; report.
- **E2.** Add tests for any uncovered new field (cron, events,
  sidecars, peers).
- **E3.** Reach ≥ 60% line coverage on `cmd/claws/` (current ~?).

### Phase F — Updated bundled templates

- **F1.** `teams/research-trio.json` gets `cron` (daily summary).
- **F2.** `specialty/knowledge-base.json` gets `sidecars: [sharedwatch]`
  watching the docs/ dir.
- **F3.** `specialty/oncall-rotation.json` gets `events: {enabled: true, digestMode: false}`
  for live PagerDuty webhook ingest.
- **F4.** Two new topology templates (D5, D6).

### Phase G — Docs + release

- **G1.** Update `templates/README.md` schema reference.
- **G2.** Add section on sidecar integration.
- **G3.** CHANGELOG v1.5.0.
- **G4.** Cut release.

## Scope — Out (defer)

- **`extends:` template composition** — still v1.6+.
- **Remote `--template=github:org/repo`** — still v1.6+.
- **Actually pulling/installing sidecar binaries** — operator
  responsibility; claws only configures.
- **Cron daemon inside the container** — runtime's job, not claws's.
  Claws writes the crontab; runtime executes it (or doesn't).

## Acceptance criteria

1. `claws apply --template=teams/research-trio` produces a `workspace/cron/`
   dir with a daily-summary entry.
2. `claws apply --template=specialty/knowledge-base` writes a
   sidecar declaration the operator can use to spin up sharedwatch.
3. `claws apply --template=specialty/oncall-rotation` sets
   `events.enabled = true` in openclaw.json.
4. `claws apply --template=teams/multi-tier` creates 7 agents in the
   correct hierarchy (1 lead-of-leads + 2 team-leads + 4 workers),
   with each agent's `topology.json` listing the correct
   `manager`/`workers`.
5. Cycle in topology (e.g. agent A's manager is B, B's manager is A)
   → reject at parse-time.
6. Cron schedule `not a valid schedule` → reject at parse-time.
7. `go test -cover` shows ≥ 60% line coverage.
8. All bundled templates dry-run clean.

## Estimated

~700 LOC + 4-6 new template files + docs. ~3 hours focused.
