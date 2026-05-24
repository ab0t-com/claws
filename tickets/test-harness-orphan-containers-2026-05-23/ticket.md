# Ticket: Integration tests leave orphan Docker containers

**Created:** 2026-05-23
**Status:** Open
**Priority:** P1 — High (silently degrades the host every time tests run, including by contributors who'd assume the test harness cleans up after itself)
**Owner:** unassigned

---

## Problem

Two integration tests in `integration_test.go` spawn real Docker containers that survive past test teardown:

1. **`TestIntegration_CreateInlineFlagsParsed`** — exercises `claws create bob --auth=codex ...`. The `--auth=codex` chain calls `cmdAuth codex` which runs `docker compose run --rm openclaw-cli ...`. `--rm` removes the CLI sidecar after exit, but **brings up the `openclaw-gateway` dependency** which has no `--rm`. The gateway keeps running with mounts pointing into `t.TempDir()`.

2. **`TestIntegration_StartGroupExpansion`** — exercises `claws start --group=alpha`. Real `docker compose up -d` per member. Container outlives `t.TempDir()`.

When the test ends, `t.TempDir()` cleans the host paths. The Docker containers stay running with broken mounts and enter `Restarting` state, looping forever. Each `go test ./...` run leaves more.

Surfaced repeatedly by `claws orphans` (ticket 10, Task E1) — that's how I noticed it during the 2026-05-23 audit. By the time the bob ticket was filed, my own session had created **3 more orphans** through the very tests that built `claws orphans`.

## Repro

```
go test -count=1 -run TestIntegration_CreateInlineFlagsParsed ./...
docker ps --filter name=openclaw-bob --format '{{.Names}} {{.Status}}'
# openclaw-bob-openclaw-gateway-1 Restarting (1) ...
```

## Proposed fix

Each test that spins real Docker should `t.Cleanup` a `docker compose -p <project> down -v` against every project it created. Two implementation options:

1. **Helper in `integration_test.go`**: extend the `claws(t, root, ...)` helper to track every `create` invocation's project name and register a single `t.Cleanup` that tears them all down. Cheap.

2. **Test-only `claws test-cleanup` subcommand**: a hidden command that walks the registry and `docker compose down -v`s every project. Tests call it in `t.Cleanup`. More general, more surface.

Recommended: option 1 — keep the cleanup logic in test code, don't add production CLI surface for a test-harness concern.

## Acceptance criteria

- [ ] After `go test ./...`, no openclaw-prefixed Docker containers remain that weren't there before.
- [ ] `claws orphans` returns empty against any host that has just finished a clean test run.
- [ ] The `TestIntegration_StartGroupExpansion` test (which is already `-short`-gated) gains a cleanup that fires only when it runs (so `go test -short` is unaffected).

## Related

- `tickets/fleet-team-control-surface-2026-05-23/` — Task E1 worklog documents this in detail.
- The 3 orphans currently on the host (`openclaw-alpha-one`, `openclaw-alpha-two`, `openclaw-bob`) were created by my session and are still there at the time of writing. The operator can clean them with `claws orphans clean --all --yes`. After this fix, that wouldn't have happened.

## Effort

~1 hour.
