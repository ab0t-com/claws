# BACKLOG — open items + deferred work

**Last updated:** 2026-05-24 (after v1.6.4)
**Source of truth.** When work lands, move the item to "Done in" and link
the version. When work is filed but not done, add it here so it doesn't
get forgotten across CHANGELOG entries, worklogs, and chat.

---

## P0 — Real day-one polish for non-technical users

| # | Item | Notes | Est. |
|---|---|---|---|
| 1 | **Auto-launch `claws setup` after first install** | `install.sh` ends with "Run claws setup now? [Y/n]" — closes the "now what" gap between install and first agent | ~30 LOC |
| 2 | **`claws setup` prompt rewording** | Replace remaining technical jargon (`role`, `auth method`, `dmPolicy`) with non-technical phrasing (e.g. "should this agent manage other agents?" not "role: manager/worker") | ~50 LOC |
| 3 | **Persistent secrets promotion** | First successful `apply` could copy `/tmp/claws-secrets/` → `~/.config/claws/secrets/` so a reboot doesn't wipe them | ~40 LOC |
| 4 | **`claws setup` final success: show the bot DM URL** | Currently shows pairing-code instructions but not `t.me/<botname>` — needs runtime to expose bot username via API or we parse from token | ~30 LOC |

## P1 — Runtime contract questions (need your answers, then easy patches)

These are the 5 questions filed against the runtime author in
`tickets/contract-alignment-fleet-ops-2026-05-24/ticket.md`. Each one
unlocks a small patch:

| # | Question | What changes if confirmed | Est. patch |
|---|---|---|---|
| 5 | Cron `jobs.json` editable shape — anything beyond what we observed in the live `.bak` file? | None if confirmed; small `runAtMs` adjustment if extra fields needed | 0-20 LOC |
| 6 | Events endpoint — does the runtime actually expose `/events/<agent>`? At what path? Auth? | Flip "unverified" → "verified" in `contract show`; possibly add endpoint to compose override | 10-50 LOC |
| 7 | Hooks contract — does the runtime really scan `shared/hooks/<event>.sh`? Invocation env (CWD, env vars)? | None if confirmed; switch to `HooksScope=both` if not | 0-15 LOC |
| 8 | Skills layout under `shared/skills/` — per-skill subdir with `SKILL.md` inside? | Patch `applySkills` to write per-skill subdir instead of flat MANIFEST | ~20 LOC |
| 9 | `openclaw.json meta.id` — does the runtime use it? Just passthrough? | None either way (we write it for cross-system tools, not the runtime) | — |

## P2 — Helper ecosystem (one-click installers)

| # | Item | Notes | Est. |
|---|---|---|---|
| 10 | **`claws sidecar install sharedwatch`** | Clones `../sharedwatch`, builds, installs to `/usr/local/bin`, registers config. Depends on you confirming the canonical repo URL | ~100 LOC |
| 11 | **`claws sidecar install intent-gateway`** | Same shape as 10, for the intent-gateway sibling project | ~50 LOC (after 10) |
| 12 | **Sidecar status in `claws fleet doctor`** | Detect whether installed sidecars are running; surface in the existing fleet doctor output | ~40 LOC |

## P3 — Template system v1.7+

| # | Item | Notes | Est. |
|---|---|---|---|
| 13 | **`extends:` template composition** | Parent → child deep-merge; lets `teams/research-extended` inherit from `teams/research-trio` and override only specific fields | ~150 LOC |
| 14 | **Remote `--template=github:org/repo@ref`** | Fetch template files from GitHub directly (separate from URL-loaded resources within a template, which we already have) | ~100 LOC |
| 15 | **Templates marketplace at `claws.run` or similar** | Real registry with semver, ratings, signing. Long horizon — only after v1.7 validates the model | many days |

## P4 — Operator UX gaps

| # | Item | Notes | Est. |
|---|---|---|---|
| 16 | **`claws agent show <name>`** consolidated overview | Single-screen: id, team, role, manager, peers, workers, channels, auth, sandbox, tools, skills, hooks, cron, sidecars. Today scattered across `info`, `agent ping`, `cron list`, `team tree` | ~150 LOC |
| 17 | **`claws team task-graph <team>`** | pending/claimed/done counts + per-task summary. Existing `team show` covers most but not the task queue specifically | ~80 LOC |
| 18 | **`claws bulk reauth`** | When an auth provider rotates keys, re-auth every affected agent. Held off pending real demand | ~80 LOC |
| 19 | **Friendly `claws audit --first-run`** mode | Strips noise; surfaces only what a new operator should act on | ~40 LOC |

## P5 — Engineering quality

| # | Item | Notes | Est. |
|---|---|---|---|
| 20 | **Subprocess test-coverage merging** | Lift the reported 9.5% coverage number to something meaningful. Tests spawn binary as subprocess; Go 1.20+ supports `GOCOVERDIR` for merged coverage across processes | ~3 hr |
| 21 | **GitHub Actions release workflow** | Tag push → automatic build + release artifacts. Today release.sh runs locally and commits binaries to the repo (works fine, but CI would be cleaner) | ~100 LOC YAML |
| 22 | **`claws contract verify <runtime>`** | Runs a probe against a live instance and reports which of the declared contract features actually work. Honest answer to "is this runtime really doing what it says?" | ~150 LOC |

## P6 — Long horizon

| # | Item | Notes |
|---|---|---|
| 23 | **Template signing (cosign/sigstore)** | v2.0 ticket. Required if marketplace ever opens to community-submitted templates with trust levels. |
| 24 | **`ab0t-sandbox` runtime adapter** | Second first-party runtime alongside openclaw. Depends on the ab0t-sandbox product shipping its own image. v1.3 design doc mentions this. |
| 25 | **REST API for claws-as-a-service** | Future product question, not a v1 concern. Would benefit from UUIDs (already shipped) as the canonical reference. |

---

## How to use this doc

- **Add new items** to whichever P-level they fit. Keep one-line item +
  notes + estimate.
- **When work lands**, move the entry to "Done in v1.X.Y" at the bottom
  + link the commit/tag.
- **When sized differently after investigation**, update the estimate
  inline.
- **When deferred indefinitely**, move to P6 with a one-line "why"
  explaining the bar to revive.

---

## Done in v1.6.X

| Item | Shipped |
|---|---|
| `claws image bootstrap` | v1.6.1 |
| Missing-env detection in apply | v1.6.1 |
| `claws agent ping` | v1.6.1 |
| `--secrets-dir` + setup-secrets.sh + `templates/demo/instant-bot.json` | v1.6.2 |
| `templates/demo/instant-team.json` | v1.6.3 |
| `docs/one-click-pathway.md` | v1.6.3 (post-tag) |
| `claws paste-secret` | v1.6.4 |
| `claws setup` hand-hold enhancements (image bootstrap + phone-paste) | v1.6.4 |
