# Ticket: `clawctl errors [--group=<name>]` — combined error/incident surface

**Created:** 2026-05-23
**Status:** Open
**Priority:** P2 — Medium (one-stop incident triage view; building blocks already shipped)
**Owner:** unassigned

---

## Problem

During the 2026-05-23 incident the operator needed to answer "what just went wrong, across the fleet?" The signal lives in four places today, each surfaced by its own command:

1. `clawctl activity --since=2h` — log-error lines from each container
2. `clawctl access audit --since=2h` — clawctl operations that returned `error`
3. `docker ps -a` filtered to `exited` / `restarting` — container-runtime failures
4. `clawctl orphans` — containers running outside the registry's awareness

Each of these is one read. Composing them is the operator's job today. `clawctl errors` should do it.

## Proposed shape

```
$ clawctl errors --since=2h
==> Container state
   team/sarah     restarting      0 restarts in window     (last exit: 0)
   team/john      running         0 restarts in window
   team/lead      running         0 restarts in window
   team1/ben      running         0 restarts in window

==> Recent container log errors (3)
   00:22  team/sarah  whatsapp 401 logged out (×24 in window)
   00:11  team/sarah  whatsapp 401 logged out (×N)
   00:08  team/sarah  whatsapp 401 logged out (×N)

==> Recent clawctl operations that returned error (1)
   00:13  ubuntu  create  error  bob --auth=codex (test harness)

==> Orphan containers (3)
   openclaw-alpha-one-openclaw-gateway-1     restarting   4 mounts ✗
   openclaw-alpha-two-openclaw-gateway-1     restarting   4 mounts ✗
   openclaw-bob-openclaw-gateway-1           created      2 mounts ✗

==> 3 instances need attention. Fix paths:
   clawctl logs team/sarah --grep=401 --since=2h    (whatsapp session needs re-login)
   clawctl orphans clean --all --yes                (remove leftover test containers)
```

JSON parity for dashboards: a flat object `{ containers, log_errors, clawctl_errors, orphans, summary }`.

## What this is and isn't

**Is**: a composition of four existing read paths, grouped + sorted + summarised on one screen. Adds no new state to clawctl. Adds no new failure detection logic. Surfaces what the existing commands already know, in one place.

**Is not**: an alerting system, a daemon, a metric collector. If an operator wants alerting they should script `clawctl errors --json | jq ...` into their existing pipeline.

## Subtleties

- **Time semantics differ across sources**. Container log errors come from `docker logs --timestamps`. Audit-log errors have RFC3339 timestamps. `docker ps -a` "Status" is relative ("restarting (1) 38 seconds ago"). The renderer needs to normalize to a single relative-time format ("2h ago") for the summary, with original timestamps available in JSON.
- **Restart-count deltas**: today's clawctl can read `RestartCount` from `docker inspect`, but it shows lifetime restarts. To show "restarts in window", remember last-seen restart count between invocations — easiest place is a tiny `.errors-state.json` next to the registry. Or just show total restart count and let the operator judge.
- **Don't duplicate `clawctl activity`**. The activity command already shows recent file changes + log errors. `clawctl errors` should focus on the *abnormal* (errors, restarts, orphans), not the normal activity.

## Acceptance criteria

- [ ] `clawctl errors [--since=2h] [--group=<name>] [--json]` produces a single-screen summary across container state, container log errors, clawctl audit errors, and orphans.
- [ ] Includes a "Fix paths" block with the exact command to address each finding.
- [ ] JSON output is a single flat object with the four sections plus a summary.
- [ ] `--group=<name>` scopes container/log sections (orphans are global by definition).
- [ ] Empty state: friendly "no errors in the last 2h" message.

## Related

- `tickets/fleet-team-control-surface-2026-05-23/` Task F+H worklog: documents why this was deferred from the parent ticket.
- `tickets/auth-fleet-reauth-2026-05-23/` — `clawctl errors` should include "auth probe failed" once `auth status --probe` ships.

## Effort

~half day.
