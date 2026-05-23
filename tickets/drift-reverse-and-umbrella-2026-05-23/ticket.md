# Ticket: Drift detection — reverse direction (E2) and umbrella (E3)

**Created:** 2026-05-23
**Status:** Open
**Priority:** P3 — Low (forward direction `clawctl orphans` already shipped covers the common case)
**Owner:** unassigned

---

## Problem

`clawctl orphans` (ticket 10, Task E1) detects **forward drift**: Docker containers that exist but aren't in the port registry. That's the `bob` case.

Two related drift cases are not yet detected:

### E2. Reverse drift — registry entry without a container

The registry says `team/sarah` exists at port 18789 but `docker ps -a` shows no container. Could happen if:

- The operator ran `docker rm -f openclaw-team-sarah-openclaw-gateway-1` directly.
- A `docker system prune -a` blew away the container.
- The container failed to start originally and clawctl's create rollback was incomplete.

Today: `clawctl list` shows the instance with status "created" or "missing"; `clawctl health` says down. Neither says "the container that should exist for this entry is gone." Operator has to compose the diagnosis.

Expected: `clawctl orphans --reverse` lists registry entries whose project has no corresponding container.

### E3. Filesystem drift (the umbrella)

Two more inconsistencies:

- **Instance directory on disk without a registry entry** — `~/.openclaw/foo/` exists with an `instance.env`, but `foo` is not in `.port-registry`. Happens if the registry file was edited by hand or restored from an old backup.
- **Registry entry whose instance directory is missing** — `.port-registry` has `0:foo` but `~/.openclaw/foo/` doesn't exist. Happens if someone `rm -rf`d an instance dir manually.

Plus the existing forward orphans (E1) and reverse orphans (E2), this is the full picture.

`clawctl drift` should run all four checks and produce a single screen:

```
$ clawctl drift
==> Forward orphans (containers not in registry) — 0
==> Reverse orphans (registry entries without containers) — 0
==> Disk drift (instance dirs not in registry) — 1
   /home/ubuntu/.openclaw/oldname/   (last modified 2026-04-01)
   Action: clawctl <add this instance> | rm the dir
==> Registry drift (entries pointing nowhere) — 0

✓ Mostly clean.
```

JSON parity. Per-finding "fix path" hint.

## Acceptance criteria

- [ ] `clawctl orphans --reverse` lists registry entries with no matching container.
- [ ] `clawctl drift` runs forward + reverse orphan checks plus filesystem inconsistency checks and renders a single-screen summary.
- [ ] `--json` parity for both.
- [ ] No false positives for instances that are intentionally stopped (Container `Exited` is still "matching" — they have a container; it's just not running).

## Subtleties

- **`drift` should not flag stopped instances as missing**. `docker ps -a` includes stopped containers; the existence check is "is there a container with this project name", not "is it running."
- **Filesystem walk needs to skip known non-instance dirs**: `shared/`, `runtimes/`, anything starting with `.` should be ignored.
- **Read-only**: `drift` and `orphans --reverse` only list, never fix. Cleanup verbs (`orphans clean`, hypothetical `drift fix --interactive`) are separate and need confirmation.

## Related

- `tickets/fleet-team-control-surface-2026-05-23/` Task E1 worklog: documents why E2/E3 were deferred.
- `clawctl errors` (separate ticket) should include the drift summary in its umbrella output.

## Effort

- E2 alone: ~2 hours
- E3 umbrella: ~3 hours after E2

Worth doing together so the renderer doesn't get rewritten twice.
