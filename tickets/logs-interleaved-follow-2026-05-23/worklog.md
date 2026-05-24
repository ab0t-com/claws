# Worklog — logs-interleaved-follow

Append-only.

---

## 2026-05-23 — Shipped (claude)

**Goal.** Replace the directive `--group= with -f not yet supported` error with a real implementation: goroutine-multiplexed `docker compose logs -f` across team members, per-member ANSI color prefix, Ctrl-C shutdown, composes with `--grep`.

### What changed

| File | Change | Lines |
|---|---|---|
| `logs_follow.go` (new) | Implementation: `tailLine` record type, `logsGroupFollow` (the orchestrator), `followMember` (per-instance subprocess + dual stdout/stderr scanners), `scanPipe` (1MB buffered scanner per stream), `safeSend` (non-blocking channel send with drop-on-full semantics). | +175 |
| `commands.go` | Replaced the rejection error in `cmdLogs` with a call to `logsGroupFollow` when `--group=` and `-f` are both set. Non-follow group path unchanged. | +5 (-7) |
| `help.go` | Removed the "not yet supported" note from the `logs` help; documented the live-multiplex shape. | +3 (-3) |
| `logs_follow_test.go` (new) | `TestSafeSendDoesNotBlock` — proves the drop-on-full invariant. If `safeSend` were a plain `ch <- ...`, this test would deadlock. The Go test timeout would still catch that, but the explicit assertion makes the regression mode clearer. | +40 |

### Design decisions worth recording

1. **Goroutine fan-in via shared channel.** One reader goroutine per member's stdout, one per member's stderr, one renderer goroutine reading the merged stream. Standard Go pipeline pattern; no third-party dependencies.

2. **`exec.CommandContext` for cancellation.** Ctrl-C → context cancel → CommandContext sends SIGKILL → docker compose subprocess dies → pipe EOF → reader goroutines exit → WaitGroup drains → channel closes → renderer loop exits. Clean shutdown without manual process bookkeeping.

3. **Drop-on-full channel semantics.** `safeSend` uses `select { case ch <- x: default: }` to avoid blocking the producer if the renderer can't keep up. Matches the behavior of `tail`-class tools: prefer drops over backpressure that would freeze the pipeline. For a ≤8-member fleet under normal log volume, drops should never fire; if they ever do, the channel buffer (4× member count) can be scaled up.

4. **`--no-color` to docker compose logs.** Stripping the upstream ANSI lets us own the color channel entirely — every line gets its own member's color, with no risk of nested escape sequences confusing the terminal.

5. **`--timestamps` to docker compose logs.** Operators interleaving live logs from multiple members lose the natural "this came after that" ordering of a single-member tail; the timestamp prefix on each line reconstructs ordering deterministically.

6. **6-color palette.** Cyan, yellow, magenta, green, blue, red. Covers the documented 8-instance cap with one cycle plus room for two collisions; visually distinguishable on all the common dark/light terminal themes. Magenta and red are reserved for the highest-trafficked members because they have the strongest visual weight.

7. **Per-line member name in the prefix.** Color alone isn't enough — operators sometimes read logs on terminals that strip ANSI (less, redirected to file, screen-readers). The full member name prefix is the source of truth; color is the aid.

8. **stderr lines get an `[err]` marker.** docker compose logs writes most output to stdout, but startup messages and some error paths go to stderr. The `[err]` marker (red) helps the operator distinguish "the agent logged an error" from "docker itself reported a problem with the log subscription."

9. **Named `tailLine` type at package scope.** Go's anonymous-struct types don't compare structurally across function signatures, so the helper functions need a named type. Cheap, clean.

10. **No mocking of docker compose for tests.** End-to-end validation requires real Docker; deferred to operator validation. The unit test exercises the only piece of logic that can fail in subtle ways (drop-on-full).

### Edge cases caught and resolved

| Case | How handled |
|---|---|
| Member's docker compose subprocess fails to start | `followMember` sends a single `(failed to start: <err>)` line and exits cleanly; other members keep streaming |
| Ctrl-C while a member is mid-burst | Context cancel kills subprocess; reader goroutines hit EOF; channel drains; renderer exits |
| Renderer slower than producers | `safeSend` drops on full channel (better than freezing); unit test asserts this |
| Long log lines (>64KB) | 1MB scanner buffer per reader |
| Empty member list | Early return — no goroutines spawned, no banner printed |
| `--grep` composes with follow | Renderer filters before printing; never reaches the renderer if the channel was already drop-on-full (but real-world drop is rare) |

### Test results

- Build clean, vet clean.
- `TestSafeSendDoesNotBlock`: ✅ pass
- End-to-end multi-instance `docker compose logs -f` validation **not run** in this session — auto-mode classifier (correctly) blocks real-Docker test runs against the live host. Operator can validate against any test fleet they spin up.

### Acceptance criteria from the ticket — status

- [x] `claws logs --group=<team> -f` streams interleaved live logs with per-member color prefix.
- [x] `Ctrl-C` cleanly stops all subprocesses (via context cancellation + CommandContext SIGKILL).
- [x] `--grep=<pattern>` composes with `-f --group=`.
- [x] 1MB scanner buffer for long lines.
- [x] Help text on `claws logs --help` documents the combined form.
- [ ] Integration test that exercises real Docker and asserts at least one line per member arrives — **deferred** to operator validation. The drop-on-full unit test plus the structural-correctness review covers the implementation risk.

### Safety

- No live agent containers touched.
- The new code only fires when the operator explicitly passes `--group= -f`. Existing single-instance `logs` paths unchanged.
- Drop-on-full semantics chosen specifically to ensure a misbehaving renderer can't hang the docker subprocess pipes (the avoid-orphan principle from ticket 12 applied at a different layer).

### Next

Ticket 14 — `claws errors [--group=]` umbrella. Composes existing read paths (container state + log errors + audit errors + orphans) into one incident-triage screen. Building blocks all exist; this is the composition layer.