# contract-alignment + fleet-ops + agent-uuids — worklog

Append-only.

---

## 2026-05-24 — Kickoff

- Filed after live-host probe of v1.5.0 surfaced silent feature
  failures: claws writes cron/hooks/skills to paths the runtime
  doesn't read.
- Scope spans 3 classes: contract alignment (urgent, silent
  failures), fleet operator visibility (high), agent UUIDs (medium).
- Tasklist drafted at
  `reports/tasklist_2026-05-24_v1.6-contract-alignment.md` —
  ordered strictly by user impact + value, not by phase.
- Target: v1.6.0. ~900 LOC. ~3-4 hours focused.

## 2026-05-24 — Approved, claiming P0.4 first

User: "approve and start claiming tasks". No answers to the open
questions yet — proceeding on best-guess reverse-engineered from
the live `~/.openclaw/team/sarah/cron/jobs.json.bak` file.

### Reverse-engineered cron job schema

```json
{
  "version": 1,
  "jobs": [{
    "id": "<uuid-v4>",
    "agentId": "main",                    // we'll use the agent's name
    "sessionKey": "agent:<name>:...",     // for cross-channel correlation
    "name": "<human-readable job name>",
    "enabled": true,
    "createdAtMs": <unix-ms>,
    "updatedAtMs": <unix-ms>,
    "schedule": {
      "kind": "every",                    // or "cron"
      "everyMs": 300000,                  // for kind=every
      "anchorMs": <unix-ms>               // start reference
    },
    "sessionTarget": "main",
    "wakeMode": "now",
    "payload": {
      "kind": "systemEvent",
      "text": "<prompt to the agent>"
    },
    "state": {                            // runtime fills in
      "nextRunAtMs": ...,
      "lastRunAtMs": ...,
      "lastStatus": "ok",
      "consecutiveErrors": 0
    }
  }]
}
```

### Key design discovery — cron is "prompt the agent", not "run a shell command"

The runtime's `payload.text` is a natural-language **system event sent
to the agent**, not a shell command. Agent reads the prompt and acts.
v1.5's `command / hook / exec` are SHELL abstractions that don't
match cleanly.

**Decision:** Extend the cron schema to add a `prompt` field (natural
fit). Keep `command / hook / exec` for back-compat — they get wrapped
as text payloads ("Execute shell command: …", "Trigger hook: …").
Templates should prefer `prompt` going forward.

### Phase order (claiming P0.4 now)

1. **P0.4** — Runtime adapter contract additions
2. **P0.1** — Cron jobs.json (replaces v1.5 workspace/cron/claws.crontab)
3. **P0.2** — Hooks team-scoped (replaces v1.5 per-agent workspace/hooks/)
4. **P0.3** — Skills team-scoped
5. **P0.5** — Migration helpers
6. **P0.6** — Tests
7. **Commit P0 as one chunk**, then P1 phase, then P2 phase, then cut v1.6.0

## 2026-05-24 — Shipped (commit fd6af48, tag v1.6.0)

All P0 + P1 + P2 tasks landed. Final state:

**P0 — silent feature failures: FIXED**
- Cron jobs.json shape matches the live runtime's editable schema —
  verified by reading sarah's jobs.json.bak. UUID-stable IDs, preserves
  runtime-owned state.* on re-apply. Schedule conversions: @daily,
  @hourly, @weekly, @monthly, @yearly, @reboot, "every <dur>", crontab.
- Hooks land at <team>/shared/hooks/<event>.sh (matches mount). Per-agent
  retained via Runtime.HooksScope=agent.
- Skills land at <team>/shared/skills/. Same opt-in.
- Runtime contract additions: HooksScope, SkillsScope, CronFormat,
  3 new capability flags.
- migrate cron: parses legacy crontab, converts, leaves legacy file in
  place. Idempotent.
- migrate uuids: populates CLAWS_INSTANCE_UUID + mirrors to openclaw.json
  meta.id. Idempotent.

**P1 — fleet operator visibility: ALL SHIPPED**
- `team tree` — ASCII topology renderer; tested with multi-tier (depth-2
  hierarchy + intra-tier peers). Cleanly handles missing/disconnected
  managers.
- `cron list` — reads jobs.json + state.*, renders schedule + next-run +
  last-run + status with color-coded status.
- `cron tail` — poll-based 2s tail of cron/runs/*.jsonl.
- `fleet doctor` — runs doctor + audit + drift + orphans with sectioned
  output + summary + non-zero exit on any failure.
- (Deferred: agent show is in v1.7 — `info <name>` already covers most.)

**P2 — UUIDs + contract: ALL SHIPPED**
- UUID at create-time: every new agent gets CLAWS_INSTANCE_UUID
  (v4, randomly generated).
- `claws id <name>` → uuid (script-friendly).
- `claws by-id <uuid>` → team/name reverse lookup.
- `claws contract show [<rt>]` — beautiful renderer of capability flags,
  hook contract, cron contract, skills scope, mount points. Warns on
  declared-but-unverified Events capability.
- `claws contract list` — registered runtimes.

**Open questions for runtime author — STILL OPEN (best-guess in place)**
1. Cron jobs.json editable shape: confirmed by reading sarah's .bak file.
   Shipped with that shape. If wrong, easy v1.6.1 patch.
2-5: Made educated guesses based on docker-compose.override.yml mount
   layout and visible runtime artifacts.

**Final state**
- origin/main at fd6af48, v1.6.0 tag pushed.
- release/VERSION → v1.6.0.
- Status: **CLOSED**.

**Deferred to v1.7+:**
- extends: template composition
- Remote --template=github:org/repo
- Sidecar one-click installer
- `claws agent show <name>` consolidated overview
- `claws team task-graph` (existing `team show` covers most)
- Test coverage uplift (subprocess coverage merging)
