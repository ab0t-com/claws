# template-engine-complete — worklog

Append-only log of work done on this ticket.

---

## 2026-05-24 — Kickoff + planning

- Filed ticket from audit findings during v1.2.0 dogfood.
- Sized scope: ~600 LOC across `apply.go`, new `template.go`,
  `quickstart.go`, plus tests + 2 bundled-template updates + 1 new
  bundled template.
- Tasklist drafted at
  `reports/tasklist_2026-05-24_template-engine-complete.md`.
- Decision: ship as v1.3.0 (additive schema fields = minor bump).
- Decision: defer `extends:` + remote templates to v1.4 to keep this
  change reviewable.

## 2026-05-24 — Shipped (commit 9720fe5, tag v1.3.0)

**Phase C — Template resolver (C1, C2, C3)**
- New `cmd/claws/template.go` (~180 LOC).
- 3-tier search: CWD/templates → XDG data dir → next-to-binary.
- `claws template list/show/resolve` subcommands.
- `claws apply --template=<name>` wired (mutually exclusive with `--file=`).

**Phase A — Silent-drop fixes (A1, A2, A3, A4)**
- A1: `applyProfilePolicy()` helper maps ProfilePolicy → Policy struct
  and calls `writePolicy()`. Verified policy.json reflects loopbackOnly,
  dmDefault, outboundDefault after apply.
- A2: per-agent `image` field + profile `runtime.image` flow through to
  `cmdCreate` via `--image=`.
- A3: `channels[].dmPolicy` appended to `cmdChannel add` as `--dmPolicy=`.
- A4: `agents[].tools.profile` applied via `cmdConfig set`.
- A5: `team.shared` left as no-op (shared dir always created); documented
  in templates/README.md.

**Phase B — New schema fields (B1–B5)**
- B1 `sandbox` (*bool) → `agents.defaults.sandbox`.
- B2 `skills` ([]string) → `workspace/skills/MANIFEST.txt` (content-hashed).
- B3 `hooks` (map) → `workspace/hooks/<event>.sh` (content-hashed, chmod 755).
- B4 `config` (map) → arbitrary `cmdConfig set` patches — catch-all.
- B5 `tools.allow` + `tools.deny` ([]string) → JSON-marshalled into
  openclaw.json via `cmdConfig set`.
- All collected into a single `applyAgentConfig()` helper for clean output.

**Phase D — Idempotence (D1, D2, D3)**
- D1 `channelEnabled()` pre-check reads agent's openclaw.json before
  attempting `cmdChannel add`. Verified end-to-end: re-apply prints
  "✓ channel telegram already configured (skipping)".
- D2 `apikeyConfigured()` best-effort check on credentials/<provider>.key.
  Underlying `cmdAuth` has stronger own-idempotence at runtime layer.
- D3 skills + hooks rewritten only on content diff.

**Phase E — Auto-audit (E1, E2, E3)**
- E1 `claws quickstart` now ends with `cmdDoctor` (env check) + `cmdAudit`
  (security check). Non-technical framing: "(some checks will warn until
  you complete steps 1 and 2 below — that's expected)".
- E2 `claws apply` ends with `cmdAudit` unless `--skip-audit` is passed.
- E3 audit framing inline in quickstart output.

**Phase F — Templates + docs**
- F1 CHANGELOG v1.3.0 entry (full section).
- F2 `templates/solo-telegram-coder.json` updated to use sandbox, tools,
  config, explicit dmPolicy.
- F3 `templates/personal-assistant.json` new — demonstrates the full
  feature set (sandbox + tools allow/deny + skills + hooks + config +
  Codex+OpenAI auth + Telegram).
- F4 `templates/README.md` — full v1 schema reference doc.

**Phase G — Tests**
- 6 new integration tests in `cmd/claws/apply_features_test.go`:
  - `ApplyAppliesAllSchemaFields` — policy, sandbox, tools, skills,
    hooks, config catch-all all land in the right files
  - `ApplyChannelIdempotent` — re-apply skips already-enabled channel
  - `TemplateResolverCWD` — `--template=<name>` resolves from cwd
  - `TemplateList` — list shows metadata
  - `TemplateShow` — prints profile JSON
  - `TemplateResolveUnknown` — clear error on miss
- Added `clawsCwd` helper for tests that need cwd-scoped runs.
- Full suite: 153s, green.

**Phase H — Release**
- H1 Full suite + gitleaks clean.
- H2 `./scripts/release.sh v1.3.0` → 4 tarballs + SHA256SUMS + VERSION.
- H3 Commit `9720fe5`, push main, tag v1.3.0, push tag — all clean.
- H4 Live install verification pending (deferred to next session — would
  need fresh fake HOME end-to-end).
- H5 This worklog update closes the ticket.

**Final state**
- `origin/main` at `9720fe5 release v1.3.0: template engine complete`.
- Tag `v1.3.0` pushed.
- `release/VERSION` → `v1.3.0`.
- All 7 sub-tickets completed.
- Status: **CLOSED**.

**Non-goals confirmed deferred:**
- `extends:` template composition → v1.4
- Remote templates (`--template=github:org/repo`) → v1.4
- JSON Schema library → indefinite
- Template signing → v2.0
