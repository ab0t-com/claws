# Worklog — errors-umbrella

Append-only.

---

## 2026-05-23 — Shipped (claude)

**Goal.** One screen answering "what just went wrong, across the fleet?" — composing container state + recent log errors + recent claws audit errors + orphan containers, with a "Fix paths" trailer listing the exact commands to run.

### What changed

| File | Change | Lines |
|---|---|---|
| `errors_cmd.go` (new) | `cmdErrors` dispatcher + `gatherErrorsReport` composer + `inspectContainerState` (restart count via `docker inspect`) + `gatherAuditErrors` (filters audit log to result=error) + `computeFixPaths` (deduplicated directive list) + `renderErrorsReport` (sectioned human output) + `stripANSI` and `stripLeadingDockerTimestamp` helpers. | +240 |
| `main.go` | Added `case "errors"` dispatch. Help section entry under Diagnostics. | +3 |
| `help.go` | Added `errors` subcommand help block. | +25 |

### Design decisions worth recording

1. **Pure composition; no new state.** Calls into existing functions (`containerStatus`, `recentLogErrors`, `discoverOrphans`) plus reads the audit log directly. If any of those sources is broken, the errors view degrades gracefully — the section is empty, not a panic.

2. **Fix paths are commands only, not commentary.** First draft included `# audit had: claws <cmd> ... (result=error at <ts>)` lines as "context." That was noise — operators reading "Fix paths" want commands they can copy-paste, not narrative. Stripped to just three categories: restart-loop diagnose, log-grep per affected instance, and orphan cleanup.

3. **Audit errors get their own section, not a fix-path treatment.** Re-running every audited error is rarely the right action (most are stale, many are read-only commands that failed because a key didn't exist, etc). The section provides context; the operator decides which need action.

4. **`stripANSI` extracted for reuse.** First version of the log-errors section rendered raw docker log lines including upstream ANSI sequences. The `truncate` cut mid-escape and left the terminal in a stuck color. Strip the ANSI first, then truncate. The helper is general-purpose — likely useful in other rendering paths (errors.go and observability.go both already strip ANSI now).

5. **`stripLeadingDockerTimestamp` cleans up double-stamping.** `docker compose logs --timestamps` prefixes every line with the docker daemon's timestamp; OpenClaw also stamps its own. Two timestamps in a 100-char-wide column eats budget. Strip the docker one (we already have `le.Time` parsed); keep the application one.

6. **Orphans are always global, never `--group=`-scoped.** Orphans are defined by their *absence* from claws's registry — they don't belong to any group. Scoping the orphans section to a group would mean "containers that don't exist in the registry but somehow we know they belong to this group," which is incoherent.

7. **`computeFixPaths` is order-independent (sorted output).** Same set of findings → same fix-path list. Means the "errors" view is diff-friendly for monitoring pipelines.

### Edge cases caught and resolved

| Case | Resolution |
|---|---|
| ANSI escapes in log details cause terminal color corruption | `stripANSI` helper applied to log-error details before truncation |
| Double timestamps (docker + openclaw) eat horizontal budget | `stripLeadingDockerTimestamp` drops the docker one; `le.Time` is the canonical source |
| Audit errors flooding the Fix paths block with non-actionable noise | Removed from `computeFixPaths`; remain visible in their own section |
| Empty fleet | Each section renders "(no instances)" / "(none in window)" / "(none)" — no nil-deref |
| `--group=` with no members | Container + log + audit sections empty; orphans still surface (they're global) |
| Log errors are already truncated to 80 chars inside `recentLogErrors` (activity.go) | Acknowledged. Future improvement: widen activity's truncation when called from errors view, or duplicate the log-scan path with a wider window. Not a regression; the existing data is just tight |

### Test results

- Build clean, vet clean.
- Live read-only smoke against production fleet:

  ```
  $ claws errors --since=2h
  Container state (4): all 4 agents running
  Log errors (10): all from team/sarah (the WhatsApp 401 loop)
  Audit errors (7): my own `auth verify` test runs that exited non-zero
  Orphans (3): the alpha-one/alpha-two/bob test leftovers
  Fix paths:
    claws logs team/sarah --grep=error --since=2h
    claws orphans clean --all --yes
  ```

  The fix-path list is exactly the right two actions for the current state: drill into team/sarah's errors, and clean the orphans.

- `--json` output validated: well-formed JSON with the four sections + fixPaths array.

### Acceptance criteria from the ticket — status

- [x] `claws errors [--since=2h] [--group=<name>] [--json]` ships a single-screen umbrella.
- [x] Includes a "Fix paths" block with exact commands.
- [x] JSON output is a flat object with the four sections plus fixPaths.
- [x] `--group=<name>` scopes container/log/audit sections; orphans remain global.
- [x] Empty-state messages friendly per section.

### Safety

- Live invocation was read-only.
- No agent containers touched.
- No `~/.openclaw/` mutations.

### Next

Ticket 15 — `claws orphans --reverse` + `claws drift` umbrella. The last open ticket in this session's batch. Picking up.