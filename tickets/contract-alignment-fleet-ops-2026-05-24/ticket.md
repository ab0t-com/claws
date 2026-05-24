# contract-alignment + fleet-ops + agent-uuids

**Filed:** 2026-05-24
**Target:** v1.6.0
**Status:** Open
**Severity:** **Mixed — contract-alignment items are silent feature failures (urgent); fleet-ops items are missing operator visibility (high but not breaking).**

## Background

Live-host dogfood of v1.5.0 surfaced **two distinct classes** of work:

### Class A — Silent feature failures (the dishonest bits)

Templates ship features (cron, hooks, skills, sidecars) that claws writes
to disk but the running runtime image **silently ignores** because the
paths/formats don't match the runtime's actual contract:

| Feature | claws v1.5 writes | Runtime actually reads | Result |
|---|---|---|---|
| Cron | `workspace/cron/claws.crontab` (crontab format) | `<instance>/cron/jobs.json` (JSON, `{version, jobs: [{id, runAtMs, nextRunAtMs, ...}]}`) | Cron jobs never fire |
| Hooks | `workspace/hooks/<event>.sh` per-agent | `<team>/shared/hooks/` team-scoped, mounted RO at `/home/node/.openclaw/shared-hooks` | Hooks never run |
| Skills | `workspace/skills/MANIFEST.txt` per-agent | `<team>/shared/skills/` mounted RO at `/home/node/.openclaw/bundled-skills` | Skill manifest ignored |
| Sidecars | `workspace/sidecars/<name>.json` (declaration only) | Nothing reads this; sidecar binary not installed | Sidecar config orphaned |
| Events | `openclaw.json events.enabled=true` | Unknown — runtime may not expose `/events/<agent>` endpoint | Unverified |

This is **worse than not shipping** the features — templates promise capability the runtime doesn't deliver, and there's no error message.

### Class B — Missing fleet/team operator visibility

The data exists on disk; the operator can't see it without `find` + `cat`:

- No `team show <name>` that renders the topology tree.
- No `team task-graph` showing pending/claimed/done at a glance.
- No `cron list/tail` showing next-run/last-run/failures.
- No `fleet doctor` aggregating env + image + every-instance health.
- No combined "this agent has cron N, hooks N, sidecars N, channels N" overview.
- No team-scoped audit roll-up (`claws audit` is host-wide).

### Class C — No stable agent identifier

Agents today are identified by `(team, name)` only. Names are
human-friendly but renameable; positions in the port registry are
brittle. Cross-system integrations (intent-gateway, sharedwatch,
future REST API, audit chain) need a **stable opaque ID**.

The runtime *already* uses UUIDs internally for cron jobs
(`c69712fd-4247-4718-bd76-8aa2bdfe0da2` style). Agent UUIDs would
fit naturally.

## Goal

Ship v1.6.0 that:
1. **Aligns the runtime contract** so cron/hooks/skills actually work.
2. **Surfaces fleet/team visibility** so operators don't need to grep
   the filesystem to understand state.
3. **Gives every agent a UUID** so external systems have a stable
   reference key.

## Scope — In

### Class A — Contract alignment (silent-failure bugs)

- **A1.** **Cron format/path** — write `<instance>/cron/jobs.json` in
  the runtime's JSON format. Convert claws schedules (crontab,
  `@aliases`, `every <dur>`) to the runtime's `runAtMs` /
  `nextRunAtMs` shape. Old `workspace/cron/claws.crontab` writes
  deprecated; remove in v1.7.
- **A2.** **Hooks path/scope** — write to `<team>/shared/hooks/` by
  default (team-scoped, runtime-mounted RO at `/shared-hooks`).
  Keep per-agent `workspace/hooks/` as an opt-in via
  `Runtime.HooksScope = "team" | "agent" | "both"`.
- **A3.** **Skills scope** — same shape as hooks. Default team-scoped
  (`<team>/shared/skills/`), per-agent as opt-in.
- **A4.** **Runtime capability declaration** — Runtime struct gains
  `HooksScope`, `SkillsScope`, `CronFormat = "claws-jobs.json" | "crontab"`,
  and capability flags reflect what's actually wired.
- **A5.** **Events endpoint verification** — `claws apply` warns
  inline if `events.enabled=true` but the runtime adapter doesn't
  declare events support. Hard to fix without runtime cooperation;
  the warn is the honest middle-ground.
- **A6.** **Migration helper** — `claws migrate cron <instance>`
  reads any old claws-managed `workspace/cron/claws.crontab` and
  converts to the new `cron/jobs.json` shape. Idempotent.

### Class B — Fleet/team operator visibility

- **B1.** **`claws team show <team>`** — renders the topology tree as ASCII:
  ```
  research/
  └── lead (manager, codex, slack)
      ├── lit-review (worker, openai-apikey)
      └── data-analyst (worker, openai-apikey)
  ```
  With `--json` flag for tooling.
- **B2.** **`claws team task-graph <team>`** — shows the task queue:
  pending count + each task summary, claimed (by whom), done (recent).
- **B3.** **`claws cron list <agent>`** — runtime-cron state: jobs +
  last-run timestamp + next-run + last-status. Reads `cron/jobs.json`
  + `cron/runs/*.jsonl`.
- **B4.** **`claws cron tail <agent>`** — streams `cron/runs/*.jsonl`
  newline-delimited.
- **B5.** **`claws fleet doctor`** — combines `claws doctor` (env) +
  `claws audit` (security) + `claws drift` + `claws orphans` into one
  command with sectioned output. Exit non-zero on any failure.
- **B6.** **`claws agent show <name>`** — combined "everything about
  this agent on one screen": id (uuid), team, role, manager, peers,
  workers, channels, auth, sandbox, tools, skills, hooks, cron, sidecars.
  Reads cleanly from disk; no runtime probe.

### Class C — Agent UUIDs

- **C1.** **Generate UUID at create-time** — `cmdCreate` writes
  `CLAWS_INSTANCE_UUID=<uuid-v4>` to `instance.env`. Stable for the
  agent's lifetime; survives rename (if we add rename later).
- **C2.** **Store in openclaw.json `meta.id`** so the runtime can read
  the same UUID.
- **C3.** **`claws id <name>`** — print the UUID for a named agent
  (script-friendly).
- **C4.** **`claws by-id <uuid>`** — reverse lookup (returns
  `team/name`).
- **C5.** **`claws list --rich`** gains an `ID` column (truncated
  to first 8 chars; full via `--json`).
- **C6.** **Migration for existing agents** — `claws migrate uuids`
  walks every agent without `CLAWS_INSTANCE_UUID`, generates one,
  appends to instance.env. Idempotent.

### Class D — Honest documentation

- **D1.** **`claws contract show [<runtime>]`** — prints what the
  selected runtime adapter supports:
  ```
  openclaw runtime (default)
    Cron:          ✓ (format: claws-jobs.json, path: <instance>/cron/jobs.json)
    Hooks:         ✓ (scope: team, path: <team>/shared/hooks/<event>.sh)
    Skills:        ✓ (scope: team, path: <team>/shared/skills/<name>/)
    Events:        ? (unverified — see docs/runtimes.md)
    Sidecars:      configure-only (operator installs sharedwatch / intent-gateway)
  ```
- **D2.** **CHANGELOG entry honesty** — call out that v1.5 cron/hooks
  paths were wrong; v1.6 fixes them. Anyone running v1.5 needs to
  re-apply for the fixes to take effect.
- **D3.** **templates/README.md** updated to reflect new paths.

### Class E — Tests

- **E1.** Unit tests for cron JSON conversion (schedules → ms timestamps).
- **E2.** Integration tests: cron jobs.json written to the right path,
  hooks land in `shared/hooks/` by default, skills land in
  `shared/skills/` by default.
- **E3.** UUID generation: new agents get one; existing agents get
  one on migrate; `id`/`by-id` round-trip works.
- **E4.** `team show` snapshot test for a multi-tier template.
- **E5.** `fleet doctor` exits non-zero on a fabricated failure.

## Scope — Out (defer)

- **`extends:` template composition** — still v1.7+.
- **Remote `--template=github:org/repo`** — still v1.7+.
- **Sidecar one-click installer** (`claws sidecar install
  sharedwatch`) — needs sharedwatch + intent-gateway to ship their
  install.sh first; we configure-only for now.
- **REST API for claws** — would benefit from UUIDs but separate
  v2.0 ticket.
- **Agent rename** (keep UUID, change name) — separate small ticket,
  v1.6.x.

## Acceptance criteria

1. `claws apply --template=teams/research-trio` produces a
   `<instance>/cron/jobs.json` that the running runtime image
   actually consumes. Verified by reading a started instance's
   `cron/runs/*.jsonl` after a scheduled run.
2. Hooks materialised by templates land at `<team>/shared/hooks/<event>.sh`
   (not `workspace/hooks/`). Verified by checking the live agent's
   `/home/node/.openclaw/shared-hooks/` mount sees them.
3. Skills land at `<team>/shared/skills/`. Verified same way.
4. Every newly-created agent has a `CLAWS_INSTANCE_UUID` in
   `instance.env`.
5. `claws migrate uuids` populates UUIDs for all existing agents.
6. `claws team show <team>` renders the topology tree without
   touching the runtime (read-from-disk only).
7. `claws cron list <agent>` shows jobs + next-run + last-run.
8. `claws fleet doctor` returns exit 0 on a clean fleet, non-zero
   on any failure surfaced.
9. `claws contract show openclaw` prints the contract claws v1.6
   negotiates with the runtime.
10. CHANGELOG v1.6.0 honestly notes the v1.5 path bugs and the
    fixes.

## Acceptance non-goals

- Doesn't require runtime cooperation to ship — claws writes to the
  paths the runtime *currently* uses. If the runtime later changes,
  we adjust the adapter.
- Doesn't require sharedwatch/intent-gateway to be installed —
  sidecar declarations remain configure-only.

## Estimated

~900 LOC + 5-7 new tests + docs. ~3-4 hours focused.

## Risk

- **Migration safety**: changing where cron/hooks/skills get written
  affects existing v1.5 deployments. `claws migrate cron` and
  `claws migrate uuids` are the safety net. We do NOT auto-delete
  old `workspace/cron/`; just write the new path. Operator removes
  the old one when satisfied.
- **UUID rollout**: if any existing system (audit log, intent-gateway)
  starts indexing by UUID before all agents have one, that's a
  consistency problem. Migration must run before any system depends
  on UUIDs.

## Open questions for the runtime author (you)

1. **Cron jobs.json schema** — confirm the fields we observed:
   `{id, summary, runAtMs, nextRunAtMs, ...}`. Anything else
   required (cron expression vs ms epoch only)?
2. **Events endpoint** — does the runtime expose an HTTP endpoint
   for event injection? If so, at what path, what auth model?
3. **Hooks contract** — does the runtime really scan
   `/home/node/.openclaw/shared-hooks/<event>.sh` and invoke on each
   lifecycle event? What's the invocation env (CWD, env vars)?
4. **Skills contract** — what's the expected layout under
   `shared/skills/`? Per-skill subdir? `SKILL.md` inside?
5. **Should agent UUIDs flow into openclaw.json `meta.id`** so the
   runtime can include them in its own logs/events?
