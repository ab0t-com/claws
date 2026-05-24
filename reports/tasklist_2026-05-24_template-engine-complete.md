# Tasklist — template engine complete (v1.3.0)

**Filed:** 2026-05-24
**Ticket:** `tickets/template-engine-complete-2026-05-24/ticket.md`
**Target:** v1.3.0 — additive schema bump, no breaking changes
**Estimated:** ~600 LOC across ~6 files, ~2-3 hours focused

Each task is a discrete unit of work, claimable independently. Tasks
marked **[blocker]** must complete before others that depend on them.
Tasks marked **[parallel]** can be done concurrently.

---

## Phase A — Fix silent-drop bugs

| ID | Task | Files | Est. | Status |
|---|---|---|---|---|
| A1 | Apply `policy.*` block (loopbackOnly, dmDefault, outboundDefault, noNewPrivileges, memoryLimit) to `policy.json` before agents created | `apply.go` + new `applyPolicy()` helper | ~40 LOC | TODO |
| A2 | Apply `runtime.image` to per-instance `instance.env` (`OPENCLAW_IMAGE=...`) | `apply.go` | ~15 LOC | TODO |
| A3 | Pass `channels[].dmPolicy` through to `cmdChannel add --dm-policy=…` | `apply.go` | ~5 LOC | TODO |
| A4 | Apply `agents[].tools.profile` via `cmdConfig set ... tools.profile` | `apply.go` | ~10 LOC | TODO |
| A5 | Decide on `team.shared` semantics (honor false / remove field) | `apply.go` + docs | ~10 LOC | TODO |

## Phase B — New schema fields

| ID | Task | Files | Est. | Status |
|---|---|---|---|---|
| B1 | `agents[].sandbox: bool` — sets `agents.defaults.sandbox` (default `true` for safety) | `apply.go` + struct | ~15 LOC | TODO |
| B2 | `agents[].skills: [string]` — symlinks skills from shared/skills/ | `apply.go` + new `applySkills()` helper | ~40 LOC | TODO |
| B3 | `agents[].hooks: {onStart, onMessage, ...}` — writes hook files | `apply.go` + new `applyHooks()` helper | ~50 LOC | TODO |
| B4 | `agents[].config: map[string]interface{}` — arbitrary openclaw.json patches via `cmdConfig set` | `apply.go` + struct | ~25 LOC | TODO |
| B5 | `agents[].tools.allow/deny` — extend existing tools.profile | `apply.go` + struct | ~15 LOC | TODO |

## Phase C — Template resolver

| ID | Task | Files | Est. | Status |
|---|---|---|---|---|
| C1 [blocker] | `claws apply --template=<name>` — search ./templates/, XDG data dir, bundled | new `template.go` | ~60 LOC | TODO |
| C2 [parallel] | `claws template list` — enumerate discoverable templates with metadata | `template.go` | ~50 LOC | TODO |
| C3 [parallel] | `claws template show <name>` — print profile JSON for inspection | `template.go` | ~20 LOC | TODO |

## Phase D — Idempotence

| ID | Task | Files | Est. | Status |
|---|---|---|---|---|
| D1 | Channel add idempotence — check `openclaw.json` `channels.<type>.enabled` before add | `apply.go` + new `channelEnabled()` helper | ~25 LOC | TODO |
| D2 | Auth apikey idempotence — check credentials file before re-run, `--force` to override | `apply.go` + new `apikeyConfigured()` helper | ~20 LOC | TODO |
| D3 | Hooks/skills idempotence — check file existence before writing | `apply.go` | ~10 LOC | TODO |

## Phase E — Automatic audit

| ID | Task | Files | Est. | Status |
|---|---|---|---|---|
| E1 | `claws quickstart` runs `claws audit` at end, surfaces failures | `quickstart.go` | ~30 LOC | TODO |
| E2 | `claws apply` runs `claws audit` at end (unless `--skip-audit`) | `apply.go` | ~20 LOC | TODO |
| E3 | Audit framing — clear messaging for "pending auth" failures | `audit.go` or quickstart wrapper | ~15 LOC | TODO |

## Phase F — Bundled templates + docs

| ID | Task | Files | Est. | Status |
|---|---|---|---|---|
| F1 | CHANGELOG v1.3.0 entry covering all changes | `CHANGELOG.md` | ~80 LOC | TODO |
| F2 | Update `templates/solo-telegram-coder.json` to use new fields | `templates/solo-telegram-coder.json` | ~30 LOC | TODO |
| F3 | Add `templates/personal-assistant.json` demonstrating skills+hooks+config | new `templates/personal-assistant.json` | ~60 LOC | TODO |
| F4 | Add `templates/README.md` documenting v1 schema field-by-field | new `templates/README.md` | ~200 LOC | TODO |

## Phase G — Tests

| ID | Task | Files | Est. | Status |
|---|---|---|---|---|
| G1 | Idempotence tests for each new field (policy, sandbox, tools, channels, auth) | `quickstart_test.go` or new `apply_test.go` | ~150 LOC | TODO |
| G2 | Integration test: each new field actually reaches openclaw.json / policy.json / credentials/ | new `apply_test.go` | ~120 LOC | TODO |
| G3 | `--template=<name>` resolver tests (3 search locations) | new `template_test.go` | ~80 LOC | TODO |
| G4 | Schema rejection tests (unknown apiVersion, missing team, bad types) | `quickstart_test.go` | ~40 LOC | TODO |

## Phase H — Release

| ID | Task | Files | Est. | Status |
|---|---|---|---|---|
| H1 [blocker] | Full suite green, gitleaks clean | — | — | TODO |
| H2 | Build artifacts, write release/VERSION → v1.3.0 | `release.sh` invocation | — | TODO |
| H3 | Commit release/, push main, tag v1.3.0, push tag | — | — | TODO |
| H4 | Verify install.sh on a fresh fake home — `curl install.sh \| sh` + `claws quickstart` → audit runs + green | manual e2e test | — | TODO |
| H5 | Update worklog with everything shipped | `tickets/.../worklog.md` | — | TODO |

---

## Recommended execution order

1. **C1** first (template resolver is a blocker for end-to-end testing with bundled templates)
2. **A1-A5** in parallel (silent-drop fixes — straightforward)
3. **D1-D3** in parallel (idempotence — straightforward, mostly pre-checks)
4. **B1-B5** in dependency order — B4 (config catch-all) makes B1/B2/B3/B5 easier
5. **E1-E3** (audit) once apply is feature-complete
6. **F2-F3** (update bundled templates to use new fields)
7. **G1-G4** tests
8. **F1** (CHANGELOG) once scope is settled
9. **H1-H5** release
10. **F4** (templates/README.md) can land in any phase — doc-only

## Hard non-goals (do NOT do as part of this ticket)

- ❌ `extends:` template composition — defer to v1.4
- ❌ Remote templates (`--template=github:...`) — defer to v1.4
- ❌ JSON Schema library — defer indefinitely
- ❌ Marketplace UI / web component — defer indefinitely
- ❌ Template signing — defer to v2.0
- ❌ Breaking changes to v1 schema — only additive fields
