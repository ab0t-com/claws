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
