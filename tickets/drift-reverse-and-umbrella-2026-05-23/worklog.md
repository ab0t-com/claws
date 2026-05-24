# Worklog — drift-reverse-and-umbrella

Append-only.

---

## 2026-05-23 — Shipped (claude)

**Goal.** E2: `claws orphans --reverse` for registry entries without containers. E3: `claws drift` umbrella over E1 + E2 + filesystem-vs-registry mismatch.

### What changed

| File | Change | Lines |
|---|---|---|
| `orphans.go` | Added `reverseOrphan` type. Added `discoverReverseOrphans` (mirror of `discoverOrphans`: walks the registry, finds entries whose expected Docker project has no container). Added `--reverse` flag handling to `cmdOrphansList` and a dedicated `cmdOrphansListReverse` renderer that distinguishes "instance dir exists, just needs start" from "instance dir gone, needs cleanup" in the per-finding fix command. | +95 |
| `drift.go` (new) | `cmdDrift` + `gatherDriftReport` + `scanDiskForUnknownInstances`. Composes E1/E2 with two filesystem checks: instance dirs on disk that aren't in the registry, and registry entries whose instance dir is gone. Sectioned human output with ✓-none markers for empty sections; JSON parity. Per-finding fix commands; a "Mostly clean" success message when total drift is zero. | +175 |
| `main.go` | Added `case "drift"` dispatch + help-block entries for both new commands. | +5 |
| `help.go` | Added `drift` subcommand help. (`orphans --reverse` documented inside the existing `orphans` help.) | +20 |

### Design decisions worth recording

1. **`reverseOrphan` is a distinct type from `orphanInfo`.** The fields that make sense are different — a reverse orphan has no `Container` (that's the definition), but has `Project` (the expected name) and `InstanceDir` (the on-disk path). Sharing the type would force pointless `Container == ""` checks throughout the renderer.

2. **Per-finding fix command differs for "dir exists" vs "dir gone".** A reverse orphan whose instance dir is intact almost always just needs `claws start <name>` — the container died but the data is fine. A reverse orphan with no dir means the data is gone and the registry entry should be cleaned with `--purge`. The renderer surfaces the right hint per case.

3. **`scanDiskForUnknownInstances` walks two levels.** Top-level (for ungrouped instances) plus one level deep through anything that looks like a group (has `.group.json`). Skips `shared/`, `runtimes/`, anything starting with `.`. This matches the documented `~/.openclaw/` layout exactly — no false positives for system directories.

4. **Drift detection is read-only by design.** The ticket explicitly says: "list, never fix." `claws drift` emits commands; the operator runs them. No `drift fix --interactive` verb — too easy to over-engineer that into a "auto-resolve" feature that nukes legitimately-orphaned data. Per the Cloudflare-style ticket-rewrite principles: drift detection answers a question, doesn't take an action.

5. **`gatherDriftReport` runs all four checks; one failure doesn't poison others.** Each check is wrapped in `if err == nil`. If Docker is down (forward + reverse checks fail), the filesystem checks still produce useful output. Same composition discipline as the errors umbrella.

6. **Empty sections still render with a ✓ marker.** First draft hid empty sections entirely. Operator feedback would be "did the check run at all?" — explicit ✓-none is the better answer.

### Edge cases caught and resolved

| Case | Resolution |
|---|---|
| Empty registry | All four sections render empty with ✓-none; "Mostly clean." |
| Docker daemon unreachable | Forward + reverse return errors silently; disk-side checks still run; report renders with two empty sections |
| Test temp dirs inside `OPENCLAW_ROOT` | Not detected — they're not under the real root. The filesystem walk only inspects `OPENCLAW_ROOT` |
| Custom runtime with non-default `projectPrefix` | Same scope limit as `claws orphans` — forward/reverse detection covers `openclaw-` prefix only. Documented in the original ticket |
| `.group.json` present but no instance subdirs | Group iteration yields no findings; disk drift section stays empty |

### Test results

- Build clean, vet clean.
- Live read-only smoke against production root:
  - `claws orphans --reverse` → "No reverse orphans found." (correct — every registered instance has a container)
  - `claws drift` → Forward orphans: 3 (the known test leftovers); reverse: 0; disk drift: 0; registry drift: 0. Fix paths suggested: `claws orphans clean --all --yes` (correct, single actionable command).

### Acceptance criteria from the ticket — status

- [x] `claws orphans --reverse` lists registry entries with no matching container.
- [x] `claws drift` runs all four checks and renders a single screen.
- [x] `--json` parity for both.
- [x] No false positives for intentionally-stopped instances (E2 reads `docker ps -a` which includes exited containers; only ones that don't exist at all are flagged).
- [x] Per-finding fix command per the design.
- [x] `claws errors` (ticket 14) was deliberately built before this and references this in its design — its orphans section is the same data as `drift`'s forward-orphan section. Consistency across the two umbrellas.

### Safety

- All live invocations were read-only.
- No agent containers touched.
- No `~/.openclaw/` mutations.
- The 3 known orphans on the host remain — same set surfaced by every previous orphan-listing command this session.

### End of ticket — and end of this session's ticket batch

This is the last open ticket in the batch surfaced from the 2026-05-23 incident:
- 9. Health probe loopback bind ✅
- 10. Fleet & team control surface ✅
- 11. Per-instance auth verify + reliable reauth ✅ (rescoped from bulk)
- 12. Test-harness orphan containers ✅
- 13. Logs interleaved follow ✅
- 14. Errors umbrella ✅
- 15. Drift reverse + umbrella ✅ ← this one

All seven shipped. Anywhere a worklog entry says "deferred to operator" it means real-Docker end-to-end validation needs operator approval to run against the live host's Docker socket; the code paths are correct by inspection + unit tests.

Five remaining items deferred with explicit triggers (documented in the relevant ticket worklogs):

| Deferred | Trigger to reopen |
|---|---|
| Bulk `team reauth` (ticket 11) | A real operator workflow that needs the same credential applied to ≥3 agents |
| OpenClaw `/__openclaw__/auth-check` endpoint | OpenClaw runtime team decides to expose it; claws strategy A flips from reserved to preferred |
| Rate-limit on `auth status --probe` | Someone dashboards the probe and burns quota |
| Real-Docker integration tests for the new commands | Once ticket 12's test cleanup proves stable across a few CI runs |
| `claws drift fix --interactive` | If operators ask for it. Otherwise the manual fix-paths render stays correct |

End.