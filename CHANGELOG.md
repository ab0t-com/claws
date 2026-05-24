# Changelog

All notable changes to claws are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_(nothing yet)_

## [v1.5.0] — 2026-05-24

### Added — Cron section in templates

- **`agents[].cron[]`** — declare periodic jobs inline:
  ```json
  "cron": [
    {"name": "daily-summary", "schedule": "@daily",    "command": "echo summary"},
    {"name": "heartbeat",     "schedule": "every 30m", "hook": "onIdle"}
  ]
  ```
- Schedule formats: standard 5-field crontab, `@hourly|@daily|@weekly|@monthly|@yearly|@reboot`, and `every <Go-duration>`.
- Job actions are mutually exclusive: `command` (shell), `hook` (reference an
  event in `agents[].hooks`), or `exec` (array, no shell interpretation).
- Optional `timezone` (CRON_TZ prefix), `enabled` (default true; false →
  written as a commented-out DISABLED line).
- Materialised to `<instance>/workspace/<runtime.CronDir>/claws.crontab`
  in `<runtime.CronFormat>` (default: `crontab`). Idempotent — only
  re-written when content changes.
- Validation at apply-time: invalid schedules, missing actions, and
  references to unknown hooks all fail loud at parse time.

### Added — Event injection block

- **`agents[].events: {enabled, digestMode, endpoint, allowFromIps}`** —
  declare the agent accepts external events. Maps to `openclaw.json`
  `events.*` via `cmdConfig set`. The runtime decides whether/how to
  expose the HTTP endpoint based on its `Capabilities.Events` flag.
- Designed to pair with the sibling `../intent-gateway` project — the
  gateway reads the events config off agents and routes accordingly.
- `digestMode: true` = events batched into periodic digests;
  `digestMode: false` = each event processed individually.

### Added — First-class sidecar helpers

- **`agents[].sidecars: [{name, kind, config}]`** — declare a helper
  CLI that integrates with the agent. **Configure-only**: claws writes
  the integration JSON to `workspace/sidecars/<name>.json` but does
  NOT install or run the sidecar binary itself.
- Built-in registry: `sharedwatch`, `intent-gateway`, `custom`.
- Examples:
  - `sharedwatch` — SQLite-backed file-watcher for multi-agent
    coordination (sibling project at `../sharedwatch`).
  - `intent-gateway` — event ingest + intent routing (sibling project).
- Unknown `kind` → apply warns inline and skips, doesn't fail the run.

### Added — Topology

- **`agents[].peers: [string]`** — explicit peer references for
  non-hierarchical relationships (mesh teams).
- **Multi-level manager chains** — `manager: <name>` already worked
  for one level; now validated for arbitrary depth.
- **`workspace/topology.json`** materialised per-agent listing
  `manager`, `peers`, and auto-derived `workers` (agents that declare
  this one as their manager).
- **Cycle detection** at apply-time: agent → manager chain that
  revisits any name fails loud, with the offending agent named.
- Self-manager and self-peer references rejected.
- Manager/peer references to non-existent agents rejected.

### Added — Runtime adapter additions (Phase A-D)

- **`Runtime.CronDir`** + **`Runtime.CronFormat`** — declare where
  cron jobs land + the format.
- **`Runtime.Capabilities.Cron`**, **`Runtime.Capabilities.Events`**,
  **`Runtime.Capabilities.Sidecars`** — three new capability flags.
- OpenClaw runtime declares all three as `true`, with
  `CronDir="cron"`, `CronFormat="crontab"`.

### Added — Bundled templates

New:
- **`templates/teams/multi-tier.json`** — 7 agents in a depth-2
  hierarchy (lead-of-leads → 2 team-leads → 4 workers). Demonstrates
  multi-tier manager chains + intra-tier peers.
- **`templates/teams/specialist-mesh.json`** — 3 peer specialists
  (researcher + writer + reviewer) with no hierarchy. Demonstrates
  mesh topology + all-to-all peer references.

Updated:
- **`templates/teams/research-trio.json`** — lead now has cron section
  (daily-summary at `@daily`, heartbeat at `every 30m` referencing
  `onMessage` hook).
- **`templates/specialty/knowledge-base.json`** — librarian gets a
  `sharedwatch` sidecar watching the docs/ dir + hourly reindex cron.
- **`templates/specialty/oncall-rotation.json`** — oncall agent gets
  `events: {enabled: true, endpoint: "/events/oncall"}` for live
  PagerDuty webhook ingest.

### Tests

- 13 new tests: cron schedule validation (good + bad), cron apply
  end-to-end with hook references + disabled jobs, cron rejection of
  ambiguous actions, events apply, sidecar apply + unknown-kind warning,
  topology cycle detection (4 cases), apply-time topology.json artefacts.
- All 11 bundled templates dry-run validated.

### Test coverage baseline

- `go test -cover ./cmd/claws/...` reports **9.5%** line coverage
  (measured at v1.4.0 → v1.5.0 boundary). Note: most tests are
  integration-style (spawn binary as subprocess), which Go's `-cover`
  doesn't measure across process boundaries. Real coverage is
  substantially higher; the v1.5 unit tests for new helpers do count.

### Not in this release

- `extends:` template composition — still v1.6+.
- Remote `--template=github:org/repo` — still v1.6+.

## [v1.4.0] — 2026-05-24

### Added — Hook register on Runtime adapter

- **`Runtime.SupportedHookEvents`**, **`Runtime.HooksDir`**, and
  **`Runtime.HookFileExt`** — adapters now declare their lifecycle hook
  contract. `applyHooks` consults the runtime; events not in the
  supported set print a warning but still write (some runtimes may
  silently ignore unknown events, others may panic — operator's call).
- OpenClaw runtime declares: events
  `onStart, onMessage, onIdle, onError, onShutdown`,
  `HooksDir="hooks"`, `HookFileExt=".sh"`.

### Added — Namespaced templates

- **`claws apply --template=solo/telegram-coder`** — namespaced lookup
  resolves a single specific template.
- **`claws apply --template=telegram-coder`** — bare-name lookup
  searches recursively across namespace dirs; errors clearly with
  qualified suggestions if multiple namespaces have the same name.
- **`claws template list`** — groups output by namespace
  (`solo/`, `teams/`, `specialty/`, plus `(flat)` for top-level).
- Flat-layout templates still resolve for back-compat (no breaking
  change to v1.3 templates dir).
- Path-traversal and absolute-path names rejected at resolve time.

### Added — URL-loaded resources for skills + hooks

- **`SkillRef`** and **`HookRef`** types: each accepts either a bare
  string (legacy) or an object with `name`, `from`, `fromUrl`, `sha256`.
- **`fromUrl`** — fetches the resource at apply-time, writes to the
  agent's workspace. HTTPS-only (refuses `http://`, `file://`, etc.).
- **`sha256`** — when declared, body is verified against the digest;
  mismatch fails apply.
- **Cache** at `${XDG_CACHE_HOME:-~/.cache}/claws/fetched/<sha-or-url-hash>`.
  Cache hits skip the download.
- **Warning** printed at apply-time if a `fromUrl` is used without a
  `sha256` pin — fetch still succeeds but operator is told.
- 4 MB body cap, 30s timeout.

### Added — 6 new real-world bundled templates (+ 3 relocated)

Relocated under `templates/solo/`:
- `solo/solo.json` (was `solo.json`)
- `solo/telegram-coder.json` (was `solo-telegram-coder.json`)
- `solo/personal-assistant.json` (was `personal-assistant.json`)

New:
- **`solo/discord-companion.json`** — Discord bot for a small server,
  allowlist by guild ID, moderation-aware tools, in-channel replies.
- **`solo/whatsapp-family.json`** — WhatsApp helper for a family,
  allowlist by phone number, calendar + reminders + shopping-list
  skills, QR-scan auth.
- **`teams/research-trio.json`** — manager + lit-review + data-analysis
  workers. Shared workspace, task queue, Slack on manager only.
  Demonstrates manager/worker role topology + per-agent personas.
- **`teams/coding-pair.json`** — implementer + reviewer agents on
  separate Telegram bots, shared workspace, reviewer is tool-restricted
  (no edit/bash).
- **`specialty/oncall-rotation.json`** — Slack-facing oncall agent.
  Receives PagerDuty webhooks, posts structured handoffs, tracks acks.
  Declares a `warnings:` block (loopbackOnly=false for webhook ingress).
- **`specialty/knowledge-base.json`** — RAG-style librarian. No public
  channel (tunnel-only), read-only tools, web-search + file-watch +
  embedding-index skills, large maxTokens.

### Tests

- 3 fetcher tests (HTTPS validation, sha256 verify happy path + bad-sha
  rejection, cache hit skip).
- 5 resolver tests (namespaced, bare unambiguous, bare ambiguous error,
  flat back-compat, traversal rejected).
- 1 list test (namespace grouping).
- All 9 bundled templates validated via `claws apply --dry-run`.

### Not in this release

- `extends:` template composition — separate v1.5 ticket.
- Remote templates (`--template=github:org/repo`) — separate v1.5 ticket.

## [v1.3.0] — 2026-05-24

### Added — template resolver + management

- **`claws apply --template=<name>`** — resolves bundled or local
  templates by name. Search order: `./templates/`, `$XDG_DATA_HOME/claws/templates/`,
  next to the binary.
- **`claws template list`** — lists discoverable templates with metadata.
- **`claws template show <name>`** — prints the JSON profile for inspection.
- **`claws template resolve <name>`** — prints the absolute path.
- **`claws apply --skip-audit`** — opt out of the auto-audit.

### Added — new profile schema fields (v1, additive)

- **`agents[].image`** — pin a specific runtime image per agent.
- **`agents[].sandbox`** — toggle `agents.defaults.sandbox` from the profile.
- **`agents[].tools.allow`** + **`agents[].tools.deny`** — explicit
  tool allowlist/denylist (alongside the existing `tools.profile`).
- **`agents[].skills`** — list of skill names; written to
  `workspace/skills/MANIFEST.txt`.
- **`agents[].hooks`** — map of lifecycle event → command/script;
  written as `workspace/hooks/<event>.sh` (chmod 755).
- **`agents[].config`** — arbitrary catch-all for `openclaw.json`
  patches via `cmdConfig set`. Anything not covered by a dedicated
  field can be set here.

### Fixed — silent-drop bugs in `claws apply`

- **A1 `policy.*`** — was parsed and discarded. Now applied to
  `policy.json` via `writePolicy`. Maps `loopbackOnly` →
  `allowedBindModes`, `dmDefault` → `requireDmPairing`,
  `outboundDefault` → `requireOutboundAllowlist`.
- **A2 `runtime.image`** — was ignored. Now passed to `cmdCreate`
  via `--image=`. Per-agent `image` field overrides the profile-level
  `runtime.image`.
- **A3 `channels[].dmPolicy`** — was parsed but not passed to
  `cmdChannel add`. Now appended as `--dmPolicy=<value>`.
- **A4 `agents[].tools.profile`** — was ignored. Now applied via
  `cmdConfig set <agent> tools.profile`.

### Changed — idempotence guarantees

- **D1 channel re-apply** is now idempotent. `claws apply` checks
  `openclaw.json` `channels.<type>.enabled == true` before calling
  `cmdChannel add` — skips with `✓ already configured` if so.
- **D2 auth apikey** has best-effort pre-check via
  `credentials/<provider>.key`. The underlying `cmdAuth` retains its
  own "already authed and verified" idempotence (works at runtime
  level regardless of the file check).
- **D3 skills + hooks** are content-hashed — files only rewritten
  when the body changes. Safe to re-apply repeatedly.

### Changed — `claws quickstart` now runs the security audit

- After agent creation, quickstart runs `claws audit` and surfaces
  findings inline with framing for non-technical users ("some checks
  will warn until you complete the next steps below — that's expected").
- `claws apply` also runs the audit at the end unless `--skip-audit`
  is passed.

### Added — bundled templates expanded

- **`templates/personal-assistant.json`** — new bundled template demonstrating
  the full feature set: sandbox, tools allow/deny, skills, lifecycle hooks,
  arbitrary config, Codex+OpenAI auth, Telegram channel.
- **`templates/solo-telegram-coder.json`** — updated to use new fields
  (sandbox=true, tools.profile=coding, config catch-all, explicit
  dmPolicy).
- **`templates/README.md`** — comprehensive schema reference for the v1
  profile format, covering every field with examples and idempotence
  guarantees.

### Tests

- 6 new integration tests covering: every new schema field actually
  reaches its target file, channel idempotence, template resolver
  from CWD, `template list/show/resolve`, schema rejection for
  unknown apiVersion + missing required fields.

### Not in this release (deferred)

- `extends:` template composition — v1.4.
- Remote templates (`--template=github:org/repo`) — v1.4.
- JSON Schema library validation — indefinite (struct unmarshal is
  catching the real issues today).
- Template signing — v2.0.

## [v1.2.0] — 2026-05-24

### Changed — Default workspace directory (back-compat preserved)

- **Host workspace default renamed: `~/.openclaw` → `~/.claws-workspace`.**
  Avoids collision with anyone running OpenClaw separately at its
  conventional `~/.openclaw` location. The container mount path
  (`/home/node/.openclaw` inside the container — what the OpenClaw runtime
  expects) is unchanged.
- **New env var `CLAWS_ROOT`** takes precedence over `OPENCLAW_ROOT`.
  `OPENCLAW_ROOT` is still respected as a legacy alias (no removal planned).
- **Back-compat for upgrading users:** if `~/.claws-workspace` doesn't exist
  but `~/.openclaw/.port-registry` does (meaning a v1.1 install with real
  agents), claws keeps using `~/.openclaw`. Existing fleets are not stranded.

Resolution order: `CLAWS_ROOT` → `OPENCLAW_ROOT` → `~/.claws-workspace`
(if exists) → `~/.openclaw` (if has instances) → `~/.claws-workspace` (fresh).

### Changed — `claws quickstart` default agent is a random personal assistant

- Default agent name is now a random pick from a curated 28-name set
  (ada, ari, ava, avery, bo, charlie, ellis, finn, grace, jamie, jules,
  kit, lex, max, milo, nia, nova, pax, piper, quinn, river, sage, sky,
  tess, val, wren, zane, zoe) — short, gender-neutral, easy to type.
- **Stronger idempotence:** re-running `claws quickstart` no longer picks
  a fresh random name each time. If the team already has any agents,
  quickstart picks up the first one for next-step hints rather than
  spawning more. Same end state on every run.
- Explicit naming still works: `claws quickstart research sarah` →
  `research/sarah`.

### Tests

- 6 new tests for `defaultRoot()` precedence covering all 5 resolution
  cases plus the legacy back-compat branch.
- 1 new test confirming `pickAssistantName()` only returns names from the
  curated set (100 iterations).
- Existing quickstart tests updated to assert against the personal-assistant
  name set + the new idempotence behavior (1 agent after N quickstart runs).

## [v1.1.0] — 2026-05-24

### Added — One-click & declarative install

- **`claws quickstart [team] [agent]`** — one-click first agent. Idempotent.
  Smart defaults (`team=default`, `agent=agent-1`). Runs init → policy init →
  access init → group create → agent create, skipping each step if already
  done. Auth and channels are NOT auto-run (they need user input) — printed
  as explicit next-step commands. Re-running is a no-op.
- **`claws apply --file=<profile.json>`** — declarative profile loader.
  Reads a JSON profile conforming to schema `claws.ab0t.com/v1` and
  reconciles host state. Idempotent. Supports `--dry-run` and `--yes` (for
  profiles that declare elevated-permission warnings).
- **Profile schema v1** with secret resolution via `tokenFrom.env`,
  `tokenFrom.file`, or `tokenFrom.command` — profiles contain no secrets.
- **Bundled templates** in `templates/`:
  - `templates/solo.json` — bare minimum (1 agent, no channel)
  - `templates/solo-telegram-coder.json` — 1 agent on Telegram + Codex OAuth
- **Help entries** for both new verbs (`claws quickstart --help`,
  `claws apply --help`) and listings under "Getting Started" / "Commands".

### Changed

- README quickstart section now leads with `claws quickstart`.

## [v1.0.1] — 2026-05-24

### Fixed

- **`claws init` post-install lookup** — the binary now finds
  `docker-compose.yml` at `${XDG_DATA_HOME:-$HOME/.local/share}/claws/`
  (where the installer places it), in addition to the existing OPENCLAW_ROOT,
  next-to-binary, and CWD lookups. Before this, fresh `curl … | sh` installs
  failed at `claws init` with "docker-compose.yml not found".

### Changed

- **Release distribution model.** Binaries now live inside the repo at
  `release/`, fetched by `install.sh` from `raw.githubusercontent.com`. No
  GitHub Release page is required — `git tag v1.0.1 && git push` is the
  entire release flow. Older versions remain reachable via tag-anchored URLs.
- **`install.sh`** now reads `release/VERSION` from `main` to resolve "latest"
  and falls back to source-build (git clone + go build) if no matching
  prebuilt tarball exists.

### Added

- **`release/VERSION`** — single-line file written by `release.sh` so
  `install.sh` can resolve the latest version without GitHub API or auth.
- **Source-build fallback in `install.sh`** — used only when no committed
  binary exists for the requested platform/version. Requires `git` and
  Go 1.22+ on the host.

## [v1.0.0] — 2026-05-24

First public release under the MIT license.

### Added — Fleet observability

- **`claws errors`** — incident-triage umbrella view. Composes container
  state, recent log errors, recent failed `claws` operations, and orphan
  Docker containers into one screen, then prints a "Fix paths" trailer of
  directive commands. Read-only; never executes anything.
  Flags: `--since=<dur>`, `--group=<name>`, `--json`.
- **`claws drift`** — four-dimension state consistency check (forward
  orphans, reverse orphans, disk drift, registry drift). Emits per-finding
  fix commands. Read-only.
- **`claws orphans`** — surface Docker containers that match the
  `openclaw-` naming prefix but are not in the port registry (e.g.
  containers a test run left behind). Subcommands: `list` (default),
  `clean <container> [--yes]`, `clean --all [--yes]`. Includes a
  `--reverse` mode that surfaces registry entries whose Docker container
  is missing.
- **`claws channels`** (pluralised) — fleet-wide channel matrix. Rows
  are agents, columns are channel types (telegram, discord, slack, signal,
  whatsapp). Cells show the dmPolicy when enabled, or `—` when absent.
  Flags: `--group=<name>`, `--json`. Singular `channel <verb>` continues
  to operate on one instance.
- **`claws logs --group=<name> -f`** — interleaved live tail across
  every member of a group with per-member ANSI colour prefix; Ctrl-C
  exits cleanly. Without `-f`, sequential dump with section headers.
  Supports `--since=<dur>` and `--grep=<pattern>` in both modes; `--grep`
  is in-process and preserves order.

### Added — Auth verification

- **`claws auth verify <name>`** — per-instance auth liveness check.
  Tries (1) the auth-check endpoint, (2) `/readyz` `failing[]` inspection,
  (3) log scan for auth errors in the last 5 minutes. Exits 0 only on
  verified ok. Honest about confidence: a log-scan "ok" means "no errors
  seen", not "next call will succeed".
- **`claws auth status --probe`** — adds a `VERIFIED` column to the
  fleet auth status table by running `verify` per row.
- **`claws auth codex --force`** — opt out of idempotence preflight
  when you specifically want to re-run OAuth.

### Added — Release infrastructure

- **MIT License** at the repo root and bundled inside every release
  tarball.
- **`scripts/rebuild.sh`** — local-dev inner-loop build script. Flags:
  `--quick` (build only), `--race` (with race detector). Version-stamps
  via `git describe`.
- **`scripts/release.sh`** — cross-compiles `linux/amd64`, `linux/arm64`,
  `darwin/amd64`, `darwin/arm64`. Each tarball contains the binary,
  `docker-compose.yml`, `install.sh`, `security-audit.sh`, `LICENSE`,
  `README.md`, `html/`, `docs/`, and a per-target `MANIFEST.txt`
  listing every file's SHA256. Produces a top-level `SHA256SUMS`.
  Builds are reproducible (`-trimpath -ldflags "-s -w"`).
- **`scripts/install.sh`** — three auto-detected modes:
  1. **Remote** — downloads release from
     `github.com/ab0t-com/claws/releases`, verifies SHA256 against
     `SHA256SUMS` before installing.
  2. **Local-release** — runs from inside an extracted tarball,
     installs the adjacent binary.
  3. **Local-dev** — invoked from a git checkout (or with
     `CLAWS_LOCAL_DEV=1`); builds from source via `go build`.
  HTTPS-only, fails on any HTTP error, refuses to overwrite existing
  install without `--force`, supports `--dry-run`.
- **`scripts/publish-release.sh`** — one-shot release driver. Validates
  clean tree, runs tests, tags the release, builds artifacts, and
  optionally pushes + creates a GitHub release via `gh`.

### Changed

- **Binary renamed: `clawctl` → `claws`.** All commands, help text,
  install/release scripts, and documentation refer to the new name.
  Anyone with a prior `clawctl` binary on PATH should remove it and
  reinstall as `claws`.
- **Env-var prefix renamed: `CLAWCTL_*` → `CLAWS_*`.** Affects
  `CLAWCTL_BASE_PORT`, `CLAWCTL_LOCAL_DEV`, `CLAWCTL_CONFIG_DIR`,
  `CLAWCTL_GATEWAY_PORT`, `CLAWCTL_RUNTIME`, and `CLAWCTL_SKIP_VALIDATE`.
  `OPENCLAW_*` env vars (which govern the OpenClaw runtime itself) are
  unchanged.
- **Module path** — `clawctl` → `github.com/ab0t-com/claws`.
- **Repo layout** — all Go source moved from the repo root to
  `cmd/claws/`. HTML assets moved from the repo root to `html/`.
  Build command is now `go build ./cmd/claws/`.
- **`docker-compose.yml`** — gateway in-container bind hardcoded to
  `0.0.0.0`. Host-side exposure is governed by `OPENCLAW_HOST_BIND`.
  Prior coupling caused gateway to bind loopback inside the container
  and become unreachable from sibling containers.
- **Help text** for `auth` rewritten to be honest about per-strategy
  verification confidence.

### Fixed

- **Integration-test orphan cleanup** — tests now register a
  `t.Cleanup` hook that removes any Docker containers left behind by
  the harness, ending the "test run leaves containers running"
  drift class.
- **`status --group=<g>`** routing — `firstPositional()` helper now
  correctly skips leading flags before resolving the instance name.
- **Column alignment** in fleet tables — new `padVisible` helper
  measures string width excluding ANSI escape sequences.
- **`logs --grep` "Stdout already set"** — grep branch now builds the
  `docker compose logs` command directly instead of inheriting a
  pre-bound `Stdout`.

### Security

- **Channel security defaults** — outbound messaging is **OFF** by
  default; DMs default to **pairing**; policy enforcement covers
  `allowFrom`.
- **gitleaks pre-commit + pre-push + commit-msg hooks** ship with
  custom rules for Telegram bot tokens, the OpenClaw gateway token
  format, and Slack `xoxb-` / `xapp-` tokens, on top of gitleaks'
  built-in rules.
- **`SECURITY.md`** added with a coordinated-disclosure policy.

### Removed

- **Windows release target** — the codebase has always depended on
  Unix-only syscalls (`Flock`, `Statfs`). The 0.x release script
  listed `windows/amd64` but never produced a working binary; the
  1.0 release script is honest about this. WSL2 users on Windows
  should use the `linux/amd64` build.

---

[Unreleased]: https://github.com/ab0t-com/claws/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/ab0t-com/claws/releases/tag/v1.0.0
