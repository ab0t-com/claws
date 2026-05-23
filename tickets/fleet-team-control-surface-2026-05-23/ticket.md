# Ticket: Fleet & team-level control plane — close the operator visibility/control gap

**Created:** 2026-05-23
**Status:** Open
**Priority:** P1 — High (limits everyday ops; surfaced repeatedly in the 2026-05-23 incident triage and the engineering/PMM audits)
**Owner:** unassigned
**Tracked in:** `tickets/fleet-team-control-surface-2026-05-23/`

---

## Problem

Today, clawctl is **strong at per-instance lifecycle** (create / start / stop / restart / status / health / config / auth / channel — each operates on a single instance) and **competent at fleet lifecycle** (start-all / stop-all / dashboard / list — each operates on every instance). What's **missing is the middle layer**: the *team* scope, and the *fleet-identity* read surface.

An operator with even 4 agents already feels this. Three concrete things that came up during the 2026-05-23 production triage:

1. **"What are these agents actually on?"** — to discover that `team1/ben` was on `anthropic/claude-opus-4-6` and the rest were on `openai-codex/gpt-5.4`, I had to run `clawctl config get` four times *and then* cross-check against the gateway's startup log line, because the model isn't surfaced by any list/status command. The operator didn't remember setting Ben to Anthropic; if the CLI had told them at a glance, they wouldn't have had to remember.

2. **"Is this thing on Telegram or WhatsApp?"** — same shape: `clawctl channel status <name>` four times. No fleet view.

3. **"There's a container my CLI doesn't know about."** — `bob` was an orphan from a test run on 2026-05-20, restart-looping 1500+ times against deleted mounts. Nothing in clawctl tells you a container with the `openclaw-` project prefix exists outside the port registry. You only find it via `docker ps` — i.e., by going around the CLI.

The general shape of the gap: **the CLI tells you about lifecycle but not identity, and it tells you about *one* or *all* but not about *some*.**

## What we already have (to not undersell it)

For honesty's sake — the existing surface is meaningful:

- `team create <name>` / `team list` — team shortcut over groups
- `group create|list|add|remove|shared|role` — full group hierarchy
- `task create|list|claim|complete|status` — manager/worker queue
- `start-all` / `stop-all` — fleet lifecycle
- `health [name name name ...]` — accepts an explicit instance list
- `dashboard [--interval]` — live refreshing all-instances view
- `activity --group=<name>` — **the only existing group filter in the codebase**
- `tunnel [name name ...]` — multi-port SSH command builder
- `group shared <name> --all` — group-scoped sharing
- `policy enforce --restart` — apply policy to every instance, restart affected
- `upgrade --all` — fleet image upgrade with per-instance health-check rollback

A grep of the source confirms 18 commands iterate the registry via `readRegistry(paths)`, but only `cmdActivity` (`activity.go:37`) accepts a `--group=` filter. Every other fleet-aware command is "one or all" with no scope in between.

## A day in the life of a clawctl operator (what's hard today)

| What the operator wants to do | Commands today | Friction |
|---|---|---|
| "Show me everything about my team" | `team list` (just names), then per-agent `status`/`channel status`/`config get` | 3-5 commands per agent, no aggregation |
| "Restart just team1 because something flaked" | Loop `clawctl restart team1/ben` per member, manually | No `--group=`, no `team restart` |
| "Who's authed to what?" | `ls ~/.openclaw/<group>/<name>/credentials/` per agent | Reads raw files; no CLI surface |
| "Which agents are connected to Telegram?" | `clawctl channel status` per agent | No matrix view |
| "Is there something running that I don't know about?" | `docker ps`, eyeball against `clawctl list` | Manual diff; bob situation |
| "Show me the audit log for just this group" | `clawctl access audit --since=...` (no filter) | All-instances stream, no group scope |
| "Which agents need their config restarted to apply changes?" | None | We *say* "Restart to apply" in many subcommands but track nothing |
| "Is anyone close to RAM limit?" | `clawctl stats` (raw docker stats passthrough) | No alerting, no aggregation |
| "Bulk rotate tokens for this team" | Per-agent `token rotate` | No `team rotate` |
| "Tail logs across the team" | `clawctl logs <name> -f` per agent in separate terminals | No multi-agent tail, no `--group=` |

## What's missing — the actual list

Organized by user need, not by command. Cost estimates are for one focused engineer.

### A. Fleet identity at a glance (P0 for ops; biggest lift-to-effort)

The single highest-value gap. `clawctl list` shows port/status/RAM/uptime; it should optionally show **identity** too.

**A1. `clawctl list --rich` (or `--wide`):**
```
NAME            PORT     STATUS   MODEL                       ROLE    CHANNELS         RAM       UPTIME
team/sarah      :18789   healthy  openai-codex/gpt-5.4        worker  telegram,whats…  410.7MiB  27h
team/lead       :19089   healthy  openai-codex/gpt-5.4        manager —                198.3MiB  27h
team/john       :18889   healthy  openai-codex/gpt-5.4        worker  telegram         324.5MiB  27h
team1/ben       :18989   healthy  anthropic/claude-opus-4-6   —       —                195.7MiB  27h
```
- Data sources: `agents.defaults.model.primary` (or the resolved-at-startup model, see A2), `INSTANCE_ROLE` in env, `channels.<ch>.enabled==true` in openclaw.json, existing RAM/uptime.
- Cost: 2-3 hours. All data already on disk.

**A2. Resolved-model surfacing.** Today the gateway logs `agent model: <provider/model>` on startup but `clawctl` can't see that — it reads `openclaw.json` which may not have the model set (e.g., ben's). Two options:
- (a) Read the gateway's `/__openclaw__/info` endpoint if one exists (low effort) — would also unblock the version/build info surface.
- (b) Tail the gateway startup log for the `agent model:` line at probe time (works today, more fragile).
- (c) Just fall back to "unknown" when `agents.defaults.model.primary` is absent, with a note that the gateway's resolved default applies. (Cheapest.)
- Cost: option (c) = trivial; (a) = 2 hours; (b) = 1 hour.

**A3. `clawctl info <name>`** — the inverse: a single-agent deep-info command that consolidates `status` + `channel status` + `auth status` + `runtime show` + model + role + recent activity into one screen. Operators who want to drill in get one command instead of five. Cost: 3 hours.

### B. Team-scoped operations — `--group=` parity (P1)

Add a `--group=<name>` flag to the existing fleet-aware commands. This is the smallest code change with the largest leverage. The current `cmdActivity` implementation is the reference (`activity.go:37`).

| Command | Today | After |
|---|---|---|
| `list` | all | `list --group=team` |
| `status` (overview) | all | `status --group=team` |
| `health` | all or named | `health --group=team` |
| `restart` | one | `restart --group=team` (+ confirmation) |
| `start` / `stop` | one | `start --group=team`, `stop --group=team` |
| `logs` | one | `logs --group=team -f` (interleaved tail) |
| `policy validate` | all | `policy validate --group=team` |
| `token rotate` | one | `token rotate --group=team` (with confirmation) |
| `upgrade` | one or `--all` | `upgrade --group=team --image=...` |
| `access audit` | all | `access audit --group=team` |

Implementation pattern (mirroring `activity.go`):

```go
// Parse:
case strings.HasPrefix(a, "--group="):
    filterGroup = a[len("--group="):]

// Filter, in each fleet loop:
for _, e := range entries {
    if filterGroup != "" {
        ref, _ := ParseRef(e.Name)
        if ref.Group != filterGroup { continue }
    }
    // ... existing per-instance work
}
```

Cost: 30 minutes per command × ~10 commands = ~5 hours total. Plus tests.

### C. `team` subcommand suite — the "team noun" experience (P1)

Today `team` only exists as `team create|list`. Promote it to a proper noun with verbs that mirror per-instance lifecycle:

| Command | Behavior |
|---|---|
| `team start <name>` | Start every member |
| `team stop <name>` | Stop every member |
| `team restart <name>` | Restart every member, sequentially (manager last so workers see fresh state — or first; document the choice) |
| `team status <name>` | Per-member health table + role topology + task-queue depth |
| `team show <name>` | All-in-one: members, models, channels, roles, shared resources, task queue size |
| `team health <name>` | Health probe filtered to team members |
| `team logs <name> [-f]` | Interleaved tail across the team |
| `team rotate-tokens <name>` | Bulk token rotation |

These should be thin wrappers over `--group=` versions of the per-instance commands (i.e., implement B first, then C is mostly delegation).

Cost (after B is done): half day for the wrappers + a coherent help block + tests.

### D. Channel & auth observability (P1)

The user shouldn't have to read `~/.openclaw/<g>/<n>/credentials/` to know what's authed.

**D1. `clawctl channels` (no args)** — fleet-wide channel matrix:
```
                  telegram   discord   slack    whatsapp   signal
team/sarah        ✓ paired   —         —        ✗ logged-out  —
team/lead         —          —         —        —          —
team/john         ✓ paired   —         —        —          —
team1/ben         —          —         —        —          —
```
With `--json` for scripting.
Cost: 3 hours.

**D2. `clawctl auth status [name]`** — read what providers are configured per agent (no secrets shown). Read `credentials/` filenames + `agents.defaults.model.primary` to infer state:
```
NAME            CONFIGURED                     ACTIVE                      LAST-USED-MODEL
team/sarah      openai-codex (oauth)           openai-codex/gpt-5.4        2026-05-22 ...
team/lead       openai-codex (oauth)           openai-codex/gpt-5.4        (no activity 48h)
team1/ben       anthropic-apikey               anthropic/claude-opus-4-6   (no activity 48h)
```
With `--json` for scripting.
Cost: 4 hours (some inference logic; LRU vs filesystem mtime).

**D3. `clawctl channels expiry <name>`** — for channels that can expire (WhatsApp Baileys sessions, Telegram bot tokens revoked upstream), surface "last successful event" so the operator sees a session is going bad *before* it fully fails. Cost: stretch — design first, defer.

### E. Drift detection — surface orphan/stale state (P1)

**E1. `clawctl orphans`** — find Docker containers matching `openclaw-*` whose project isn't in `~/.openclaw/.port-registry`. The bob case automatic.

```
$ clawctl orphans
ORPHAN                                    SINCE              MOUNTS                                   ACTION
openclaw-bob-openclaw-gateway-1           2026-05-20 03:45   /tmp/TestIntegration_.../bob (missing)   clawctl orphans clean openclaw-bob-...
```

With `clawctl orphans clean [name]` to `docker rm -f` it (with confirmation). Cost: 2 hours.

**E2. `clawctl orphans --reverse`** — registry entries whose container doesn't exist (created but never started, or container was nuked outside clawctl). Cost: 1 hour.

**E3. `clawctl drift` (umbrella)** — runs E1 + E2 + checks for instance dirs on disk without registry entries (and vice versa), surfaces stale `.port-registry.lock` files, etc. One command, one screen, do-no-harm. Cost: 3 hours after E1/E2.

### F. Better incident-time observability (P2)

These shorten the diagnostic loop, which the 2026-05-23 incident demonstrated is currently bashed-together-by-hand.

**F1. `clawctl logs --group=<name> -f`** (covered by B) — interleaved tail.

**F2. `clawctl logs <name> --since=24h --grep=<pattern>`** — push the grep into the binary so operators don't have to pipe through bash and lose color codes.

**F3. `clawctl audit tail [-f]`** — read `.audit.log` with optional follow. Doesn't need anything new, just a UX layer over what's there.

**F4. `clawctl errors [--group=]`** — combined view of activity errors + audit "error" entries + container exit codes + restart counts. The "what just went wrong?" command.

Cost: F1 free with B; F2 = 2 hours; F3 = 1 hour; F4 = 3 hours.

### G. JSON parity — for embedding clawctl in your own dashboards (P2)

Today `list`, `status`, `health`, `runtime show` support `--json`. These do not (and should):
- `task list`
- `channel status`, `channel security`
- `policy validate`, `policy show` is already JSON because of how files are stored
- `access audit`
- `group list`, `team list`
- (new) `info`, `auth status`, `channels` (matrix), `orphans`, `drift`

Pattern is the same as existing JSON paths. Cost: ~30 minutes per command, ~6 hours total. (Listed in the engineering structural report 2026-05-20 §5.2 as the biggest concrete OSS scripting gap.)

### H. Bulk operations on the team noun (P2)

Already partially covered by B/C but worth calling out:
- `clawctl team rotate-tokens <team>`
- `clawctl team upgrade <team> --image=...`
- `clawctl team apply-policy <team>` (push policy to every member, restart affected)
- `clawctl team apply-config <team> <key> <value>` (set the same config across the team, e.g., `tools.profile`)

Cost: each = thin wrapper over per-instance commands; ~half day total once B exists.

## Why the small "hyper plane" framing matters

Clawctl operates on a single host, ≤ 8 agents — by design. The "hyper plane" here is *not* multi-host orchestration. It's the **scope above per-instance, below fleet** — the team. That's the level your live setup naturally clusters at (`team/` and `team1/` are already groups of 3 and 1 in `~/.openclaw/`). The CLI today expresses lifecycle at the wrong altitudes for what you actually have.

Adding the team scope is the difference between treating clawctl like a per-agent control surface (today) and treating it like a *team-level* control surface (where it's actually used). It also keeps clawctl honest about its scope: not Kubernetes, not multi-host, just *the right amount of orchestration for the one-server, small-team setup* — which is the positioning in the PMM report.

## Sequencing recommendation

Build in this order; each step unlocks the next:

1. **B — `--group=` parity** across the fleet-aware commands. Done first because everything else builds on it. ~1 day.
2. **A — fleet identity in `list --rich` + `clawctl info <name>`**. The single largest UX win. ~1 day.
3. **E1 — `clawctl orphans`**. Closes the bob class of bugs forever. ~2 hours.
4. **C — `team` subcommand suite**. Mostly delegation once B exists. ~half day.
5. **D — channels matrix + `auth status`**. ~1 day.
6. **G — JSON parity** across the new commands. ~half day amortized.
7. **F + H + E2/E3** — incident-time observability and bulk team ops. ~1 day.

Total: about **4-5 focused engineering days** for the whole batch. The first three steps (B, A, E1) take less than two days and cover ~70% of the everyday-ops pain.

## Acceptance criteria

A user who has a 4-agent team running should be able to:

- [ ] Run **one command** and see model + role + channels + image + uptime per agent in their team.
- [ ] Run **one command** and see whether any container exists on the host that clawctl doesn't know about, with a one-command clean.
- [ ] Restart their whole team with one command — `clawctl team restart <name>` or `clawctl restart --group=<name>` — without writing a bash loop.
- [ ] Get JSON output for every read command so they can feed an external dashboard / Slack alerting / GitHub Actions.
- [ ] Tail logs across the team interleaved, with grep at the CLI level, without juggling tmux panes.
- [ ] Run `clawctl info <name>` and not need any other command to know what state that agent is in.

## Related

- `tickets/health-probe-loopback-bind-2026-05-23/ticket.md` — sibling: the deployed binary's *existing* `clawctl health` is currently broken under loopback bind. The work proposed here assumes `health` returns correct verdicts; if implemented before that fix, `--group=` filters on `health` will inherit the same false-negative.
- `tickets/test-harness-orphan-containers-<dated>/ticket.md` — sibling (not yet filed): the test code path that leaves `bob`-style orphans. Filing separately because that's a test-isolation bug, not a CLI feature.
- `reports/report_engineering-structural_2026-05-20.md` §5.2 (`What's mediocre — JSON parity gap`), §9 item 4 (library split for embeddability), and the LLM-as-judge rubric line 10 (Operator DX).
- `reports/report_pmm-opensource_2026-05-20.md` §4 OSS readiness — having a coherent fleet/team view is the difference between "another vendored tool" and "a platform people build on."

## Out of scope (parked deliberately)

- Multi-host orchestration: not in clawctl's positioning; deliberately not addressed.
- Web UI / dashboard server: the existing `clawctl dashboard` (live TUI) is enough for v0; HTML lives in the landing-page concern.
- Cross-team / multi-tenant isolation beyond the current `.access.json` scoping: separate ticket if/when it becomes a need.
- "Always-on agent state machine" (e.g., orchestrated handoffs between agents via tasks) — separate concept; the existing `task` CLI covers the minimum.
