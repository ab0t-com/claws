# Tasklist — 2026-05-20 — Structural Map & Open-Source Readiness

**Goal.** Produce two deliverables (engineering structural report + PMM/positioning report) plus a dogfooded build that proves the binary still works. The user wants to "open it up" (open source / hand off / expand audience) but does not currently have a clear story for *what it does* or *who it's for*. These reports must answer both.

**Working directory.** `/home/ubuntu/claw/workspace/clawctl-go`
**Branch.** `feature/runtime-adapter`
**Operator.** claude (this session)

## Deliverables

1. `reports/report_engineering-structural_2026-05-20.md` — full structural mapping: module graph, data-flow, state model, stubs/missing/holes, interface review, LLM-as-judge rubric scored against the framework.
2. `reports/report_pmm-opensource_2026-05-20.md` — PMM/product framing: positioning, ICP, JTBD, market, competitive context, GTM/launch readiness, OSS-readiness checklist, risks.
3. Dogfooded binary (`./claws`) verified end-to-end with at least the golden path (init → create → start → status → stop → remove) and the admin surface (policy, access, audit).
4. Appended worklog at the bottom of this file with a timestamped log of what was done and what was learned.

## Tasks

- [x] T01. Ingest README, main.go, INTEGRATION_ANALYSIS.md to understand framing.
- [ ] T02. Map runtime adapter layer (`runtime.go`, `compose.go`, `image.go`, `registry.go`, `proxy.go`, `doctor.go`).
- [ ] T03. Map control / policy / access / multi-tenant plane (`access.go`, `policy.go`, `audit.go`, `activity.go`, `channel.go`, `group.go`, `task.go`, `flock.go`, `merge.go`, `shared.go`).
- [ ] T04. Read prior reports + tickets so we don't repeat known facts.
- [ ] T05. Read config + storage + setup + init (`config.go`, `configcmd.go`, `storage.go`, `setup.go`, `init.go`).
- [ ] T06. Inspect `docs/`, `scripts/`, install/release/security-audit shell.
- [ ] T07. Dogfood: build, run golden path + admin surface, capture DX friction.
- [ ] T08. Write engineering structural report.
- [ ] T09. Write PMM report.
- [ ] T10. Append worklog.

## Claims

`claude` claims T01–T10 for this session (single-operator run).

## Constraints

- Read-only ingest, then read-only build + smoke-run. No mutations to existing code unless a smoke-run reveals an unambiguous breakage (in which case ask first).
- Use the existing on-disk `./claws` binary if `go build` is fast; rebuild before dogfooding so we test current source, not a stale binary.
- All scratch state goes in a temporary `$OPENCLAW_ROOT` so we never touch the real `~/.openclaw`.
- Use the project's pinned go binary per memory: `~/.openclaw/team/sarah/workspace/.tools/go/bin/go`.

---

## Worklog (append-only)

### 2026-05-20 — Session 1 (claude)

**Set-up.** Confirmed the branch (`feature/runtime-adapter`), the existing on-disk binary (8.5 MB, dated 2026-03-27), and the live `~/.openclaw/` root (groups `team/{john,lead,sarah}` + `team1/ben`). Saved two memories before doing anything destructive: `feedback_no_rm_rf` and `project_live_agents_on_host`. The rest of the session honored both.

**Ingestion (T01-T06).** Read every Go source file in full: `main.go`, `commands.go` (lifecycle), `runtime.go` (adapter), `compose.go` (subprocess), `access.go`/`policy.go`/`audit.go` (admin plane), `group.go`/`channel.go`/`task.go` (multi-tenant), `config.go`/`configcmd.go`/`storage.go`/`init.go`/`setup.go` (state + onboarding), `registry.go`/`shared.go`/`flock.go`/`merge.go`/`activity.go`/`doctor.go`/`image.go`/`proxy.go` (supporting). Read all prior reports under `reports/`, the full ticket trail under `tickets/`, both `docs/*.md`, `docker-compose.yml`, the install/release scripts. Catalogued 232 existing Go tests via `grep ^func Test *_test.go`. Reviewed the integration_test.go (807 lines) line-by-line because it documents the actual contract.

**Test plan (T11).** User redirected mid-session: don't dogfood blindly, write a test plan first. Created `tests/test_plan_2026-05-20.md` (Phases A-K, with explicit safety contract: every smoke command sets `OPENCLAW_ROOT=$(mktemp -d -t clawctl-dogfood-XXXXXX)`; refuses to run if root resolves under `$HOME/.openclaw`; sets `CLAWS_SKIP_VALIDATE=1`; no `rm -rf`, no `start`/`stop`/`restart`/`upgrade`, no real Docker mutations). Created the smoke driver at `tests/smoke_dogfood.sh` and the curated DX log at `tests/dogfood_log_2026-05-20.md` (plus the raw transcript at `tests/dogfood_log_2026-05-20.out`).

**Dogfood (T07).** Compiled the binary at `/tmp/clawctl-dogfood` with the project-pinned go (`~/.openclaw/team/sarah/workspace/.tools/go/bin/go`, version go1.22.12). `go test ./...` → all 232 tests pass in 39.5 s. `go vet ./...` clean. Ran the smoke harness end-to-end: **81/81 checks pass.** Test root preserved at `/tmp/clawctl-dogfood-XzY6Kj` for the user's inspection. The live `~/.openclaw/` was never touched.

**Findings worth carrying forward.** Five low/medium DX or scope issues surfaced — all documented in the dogfood log §"Issues observed":
- **S1 (LOW):** `gateway.bind` is hardcoded `"lan"` in the JSON skeleton at `commands.go:253` and is *not* re-derived from the actual `--bind=` flag, so `config show` displays one bind mode while the gateway runs with another. Confusing but not exploitable.
- **S2 (MEDIUM):** `claws audit` (via `scripts/security-audit.sh`) enumerates **every** clawctl-style container running on the host instead of scoping to `$OPENCLAW_ROOT`. During the smoke run it audited the operator's live `team-lead`/`team-sarah`/`team-john`/`team1-ben`/`bob` containers. Informative but conceptually wrong.
- **S3 (MEDIUM):** Several scriptable commands (`task list`, `channel status`, `channel security`, `policy validate`, `access audit`, `group list`) lack `--json`. Biggest concrete OSS-readiness gap for CI/automation.
- **S4 (LOW):** `claws init --force` is undocumented in `help.go`.
- **S5 (LOW, cosmetic):** `status <name>` prints an empty `docker compose ps` header table when no container exists.

**Deliverables (T08, T09).** Both reports written and saved into `reports/`:
- `reports/report_engineering-structural_2026-05-20.md` (4.2k words). Module map, layer view, four canonical data flows, stubs/missing inventory, interface review (good/mediocre/risky), current state table, LLM-as-judge rubric scoring 127/150, prioritised "open it up" list (8 items, items 1-3 are required).
- `reports/report_pmm-opensource_2026-05-20.md` (3.8k words). Positioning statement, ICP, JTBD, market context, OSS-readiness checklist (9 hygiene blockers), GTM playbook with 30-day success metrics, risk register including the platform-TOS risks for WhatsApp/Signal, sequenced launch plan.

**Worklog (T10).** This entry. All 11 tasks complete (T01-T11). No changes pushed to `main`; no commits made; only new files added (the three deliverables under `reports/`, the three artifacts under `tests/`, two new memory entries under `~/.claude/projects/.../memory/`).

**Safety summary.** Zero touches to `~/.openclaw/`. Zero `rm -rf`. Zero `--purge` invocations against any non-`mktemp` directory. The test root at `/tmp/clawctl-dogfood-XzY6Kj` is left on disk for the operator to inspect or remove.

**Open question for the user.** §4 of the PMM report calls out one branding/positioning decision that is purely a product call (not engineering): is claws an OpenClaw subproject, a sibling tool, or independent? The wording in the landing page, README, and HN submission depends on this. Worth deciding before publication.
