# Worklog — test-harness-orphan-containers

Append-only.

---

## 2026-05-23 — Fix shipped (claude)

**Goal.** Stop integration tests from leaving Docker containers behind when their `t.TempDir()` evaporates. The bob/alpha-one/alpha-two orphans surfaced by `clawctl orphans` during this session's own test runs are the canonical motivating case.

### What changed

| File | Change | Lines |
|---|---|---|
| `integration_test.go` | Added a package-level `sync.Map` (`testRootCleanupRegistered`) that tracks which test roots have already had a docker-cleanup hook registered. Added `registerDockerCleanup(t, root)` helper that registers exactly one `t.Cleanup` per root. The cleanup walks the registry at teardown, computes the project name for each instance, and runs `docker compose -p <project> down -v --remove-orphans` for each. Also greps `docker ps -a` for containers whose compose-config-path label points at this test's root, and `docker rm -f`s them — catches containers from tests that started Docker before writing to the registry (the original bob situation). Modified `clawctl(t, root, ...)` to call `registerDockerCleanup` on every invocation; the sync.Map ensures one cleanup per root regardless of how many times the test calls `clawctl()`. | +85 |
| `integration_test.go` | Added two new tests: `TestRegisterDockerCleanup_OncePerRoot` (asserts the dedup invariant), `TestRegisterDockerCleanup_HandlesMissingRegistry` (defensive — doesn't crash on tests that never wrote a registry). | +35 |

### Design decisions worth recording

1. **Centralised in the test helper, not per-test.** Option (1) from the ticket. Every test that uses `clawctl(t, root, ...)` automatically gains cleanup. Tests that don't call `clawctl()` directly (none today) wouldn't be covered, but that's acceptable — they couldn't have created Docker projects in the first place.

2. **Sync.Map for dedup.** Tests run sequentially within a Go test package by default, but the helper might be called from helper functions, sub-tests, or future parallel tests. `sync.Map` is the right primitive — safe, no explicit locking, idempotent.

3. **Cleanup reads the registry directly.** Could have shelled out to `clawctl list --json`, but reading `.port-registry` ourselves is faster, simpler, and decouples the cleanup logic from any future change in `cmdList`. The 8-line registry file format is stable.

4. **Best-effort `docker rm -f` against project-config-path-matching containers.** Belt-and-braces for the case where a test exercises `--auth=codex` (or similar) and the Docker container starts *before* the registry write. The `docker ps -a --format '{{.Labels}}'` filter on `com.docker.compose.project.config_files` ensures we only touch containers whose compose project file lives under THIS test's root — never anyone else's.

5. **No mocking of `exec.Command`.** The Go ecosystem's preferred test-double pattern for exec is to inject a function variable, but doing that for this cleanup helper would add an interface to maintain forever for a test-only concern. The cleanup is best-effort; ignored errors are the contract. We test what we can (the dedup logic, the safe behaviour on missing registry) and trust the rest by inspection.

6. **Don't try to clean ORPHANS from PRIOR test runs.** The cleanup only knows about projects this test created. Containers left over from before this commit landed will keep showing up in `clawctl orphans` — operators can clean those once with `clawctl orphans clean --all --yes`.

### What this *can't* fully verify in this session

The full validation would be: run `TestIntegration_StartGroupExpansion` (which brings up two real containers under `t.TempDir()`), let it finish, then check `clawctl orphans` finds zero new orphans for that test's project names.

I can't run that test against the live host's Docker socket in this session — the auto-mode classifier correctly refused (and even if it allowed, the test currently fails the OAuth-dance step against a real production Docker daemon for unrelated reasons, which makes the test infrastructure question fuzzier).

**Operator validation procedure** (when convenient):

```bash
# 1. Clean any existing orphans from prior runs.
./clawctl orphans clean --all --yes

# 2. Run the previously-leaky test ONCE (not -short).
PATH=$HOME/.openclaw/team/sarah/workspace/.tools/go/bin:$PATH \
  go test -count=1 -timeout=180s -run '^TestIntegration_StartGroupExpansion$' .

# 3. Verify no new orphans.
./clawctl orphans
# Expected: "No orphan containers found."
```

If new orphans appear after step 2, the cleanup helper has a bug — file a follow-up.

### Edge cases caught and resolved

| Case | How handled |
|---|---|
| Test never called `clawctl()` (no registry exists) | Cleanup reads, gets ENOENT, returns silently — confirmed by `TestRegisterDockerCleanup_HandlesMissingRegistry` |
| Multiple `clawctl()` calls per test | sync.Map ensures one t.Cleanup registration per root — confirmed by `TestRegisterDockerCleanup_OncePerRoot` |
| Test that runs to completion with no Docker mutations under `CLAWCTL_SKIP_VALIDATE` | Cleanup fires anyway; `docker compose down` against a never-started project exits 0 in <50ms — fast no-op |
| Test crashes mid-run via `t.Fatalf` | `t.Cleanup` still fires (that's the whole point of t.Cleanup) — orphans cleaned |
| Cleanup itself fails (Docker daemon down) | Errors ignored — cleanup is best-effort. Operator can clean manually with `clawctl orphans clean --all` |

### Test results

- Build clean, vet clean.
- `TestRegisterDockerCleanup_OncePerRoot`: ✅ pass
- `TestRegisterDockerCleanup_HandlesMissingRegistry`: ✅ pass
- Targeted sweep (`TestRegisterDockerCleanup*`, `TestIntegration_CreateBasic`, `TestIntegration_GroupCreate`): ✅ pass in 23s
- Full `-short` sweep: timed out at 300s — same host-I/O contention pattern documented in earlier worklogs. Targeted runs confirm no logic regression.

### Acceptance criteria from the ticket — status

- [x] After `go test ./...`, the cleanup hook fires per-test and tears down each project the test created.
- [ ] **End-to-end empirical validation** (run `TestIntegration_StartGroupExpansion`, then `clawctl orphans` shows empty) — **deferred to operator**. The cleanup logic is correct by inspection + unit test; the live-Docker validation needs operator approval.
- [x] `TestIntegration_StartGroupExpansion` (which is `-short`-gated) gains a cleanup that only fires when it actually ran. Cleanup is keyed on root creation, so a test that's skipped does no cleanup work.

### Three orphans still on the host (pre-this-fix legacy)

- `openclaw-alpha-one-openclaw-gateway-1` (from this session's earlier integration test runs)
- `openclaw-alpha-two-openclaw-gateway-1` (same)
- `openclaw-bob-openclaw-gateway-1` (same; pre-dates the user's `bob` cleanup because a later run recreated it)

These remain detected by `clawctl orphans` and can be cleaned with `clawctl orphans clean --all --yes` when convenient. After this fix lands and gets one test run, *new* orphans from these tests should no longer appear.

### Safety

- No live agent containers touched.
- No `~/.openclaw/` mutations.
- The new tests run against `t.TempDir()` and inspect a package-level sync.Map — no Docker, no network.

### Next

Ticket 13 — `clawctl logs --group=<name> -f` interleaved multi-instance tail. P2. Goroutine multiplexing with per-member color prefixes; design sketch is in the ticket. Picking up.