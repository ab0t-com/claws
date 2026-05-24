# Changelog

All notable changes to claws are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_(nothing yet)_

## [v1.6.5] — 2026-05-24

Install path hotfix — `curl install.sh | sh` was failing on Ubuntu
because the script uses bash features (`set -o pipefail`) that dash
(Ubuntu's `/bin/sh`) doesn't support. First-time users hit
`sh: 25: set: Illegal option -o pipefail` and bounced.

### Fixed

- **`scripts/install.sh` — detect shell at startup** and re-exec under
  bash, or fail with a clear actionable error message if bash isn't
  available. Three invocation paths now all work:
  - `curl … | bash` — recommended, works as before.
  - `curl … | sh` — re-execs under bash if available; otherwise prints
    install instructions for getting bash. No more cryptic dash error.
  - `./install.sh` — direct execution, works as before.
- **`--help` no longer breaks under `curl | bash`** — it was reading
  `$0` to slice the docstring out of the script header, but `$0` is
  literally `bash` when piped. Replaced with an inline heredoc.

### Docs

- README, CHANGELOG, `docs/goal-instant-claw.md`, and
  `docs/one-click-pathway.md` all now say `| bash` instead of `| sh`.
- `install.sh` header comment updated to recommend `| bash`.

### Why this matters

A non-technical user can't recover from `sh: 25: set: Illegal option
-o pipefail` — they have to know that Ubuntu's `/bin/sh` is dash and
that `set -o pipefail` is bash-only. The right behavior is for the
script to deal with it transparently.

## [v1.6.4] — 2026-05-24

Non-technical user pass — closes the "I need to copy a 46-char Telegram
token from my phone to an SSH session" friction point. Project owner
directive: "our users are non-technical, our goal is to hold their
hand and make it as easy as possible."

### Added — `claws paste-secret <name>`

Ephemeral local HTTP listener that bridges phone → server for any
secret value. Use case: BotFather replies with a 46-char token on your
phone; you don't want to email it to yourself, install Telegram Desktop,
or type it character-by-character.

```
$ claws paste-secret telegram.token

  Open on your phone:
      http://192.168.1.42:8765/aBc3K9p
  Enter this code on the page:
      417-302
  Listening on 0.0.0.0:8765 for 5m0s ... (Ctrl-C to cancel)
```

User taps the URL on their phone → sees a mobile-friendly form
(textarea + code field) → pastes the token + enters the 6-digit code
→ submits → server writes `/tmp/claws-secrets/telegram.token`, exits.

**Security model:**
- 7-char random URL token (28 bits of entropy) — unguessable on a LAN
- 6-digit code shown on terminal must echo from the phone (defends
  against someone with the URL but not terminal access)
- Single-use — server exits after the first successful paste
- 5-minute auto-expire (configurable via `--timeout=<dur>`)
- `--bind=127.0.0.1` mode requires SSH port-forward (no LAN exposure)
- HTTP-only — fine for ephemeral local-network paste; URL+code ARE the
  secret. HTTPS would need cert generation that's out of scope for this.

**Flags:** `--secrets-dir=<path>` (default `/tmp/claws-secrets`),
`--port=<n>` (default 8765), `--bind=<addr>` (default `0.0.0.0`),
`--timeout=<dur>` (default 5m).

### Enhanced — `claws setup`

Two integrations make the wizard truly hand-holding:

1. **Step 1 (prereqs)**: if `openclaw:local` not present, offers to
   run `claws image bootstrap --yes` inline. Clear "this takes 5-10
   minutes, one time only" framing.
2. **Step 6 (channel)**: when picking a non-WhatsApp channel, asks how
   the user wants to enter the bot token:
   ```
   How do you want to enter the bot token?
     1. Paste here  (good if you've got the token in your clipboard)
     2. Phone-paste (open a URL on your phone, paste there — easier from BotFather)
   ```
   Picking 2 invokes `paste-secret` inline; the wizard reads the
   resulting file and continues.

### Tests

- 3 new tests: helper randomness/shape, invalid-name rejection,
  end-to-end POST round-trip (wrong-code rejected, right-code accepted,
  file written, server exits).
- Full suite remains green.

### Internals

- `paste-secret` command in `cmd/claws/paste_secret.go`. ~280 LOC,
  net/http only (no new deps).
- LAN IP discovery walks `net.Interfaces()` for the URL hint.
- Mobile-friendly form: viewport meta, large touch targets, monospace
  textarea for the token (which is mixed-case + symbols).

### Docs

- `docs/goal-instant-claw.md` updated to lead with `claws setup` as
  the primary path; the manual two-paste flow remains documented as
  the power-user alternative.

### Why this matters

A non-technical user who's never SSH'd before could conceivably do:

```bash
ssh me@my-server
claws setup
# answers prompts; when it asks "how do you want to enter the bot token",
# picks "phone-paste"; opens the URL on their phone; pastes from BotFather;
# enters the 6-digit code shown on the SSH terminal; bot is online.
```

No multi-device clipboard juggling. No file editing. No knowledge of
`--secrets-dir`, `apply`, or `chmod`. The whole flow fits in one
terminal session + one phone tap.

## [v1.6.3] — 2026-05-24

Adds the team variant of the say-GO demo.

### Added — `templates/demo/instant-team.json`

Minimum-input team: 1 user-facing **coordinator** agent on Telegram +
1 backend **worker** agent. DM the coordinator → it delegates via the
shared task queue → worker writes a result → coordinator relays back.

Demonstrates the manager/worker topology with the same two tokens as
`instant-bot` (no extra setup required — `OPENAI_API_KEY` +
`TELEGRAM_BOT_TOKEN`).

```bash
claws apply --template=demo/instant-team --secrets-dir=/tmp/claws-secrets
claws start instant-team/coordinator
claws start instant-team/worker
# DM the coordinator. Ask it to research something.
```

### Why this exists

The v1.6.2 `instant-bot` template gets a solo agent on Telegram in
under 5 minutes. `instant-team` does the same for the multi-agent
case — without forcing the operator to provision two Telegram bots
(the v1.5 `teams/coding-pair` template needed two separate bot
tokens; this one needs one).

## [v1.6.2] — 2026-05-24

The "say GO, have a bot on your phone" patch. Closes the gap between
"claws install works" and "user has a responding bot in their pocket"
to ~3 commands + 2 token pastes.

See `docs/goal-instant-claw.md` for the design intent.

### Added — `scripts/setup-secrets.sh`

Initialises a claws secrets directory (default `/tmp/claws-secrets/`)
with placeholder files for 9 known providers/channels (OpenAI, Anthropic,
Google, Groq, OpenRouter, Telegram, Discord, Slack bot+app). Each
placeholder file is `chmod 600`; the directory is `chmod 700`. Each
placeholder explains where to get the value (with the URL) and how to
paste it. Includes a README.md inside explaining the naming convention.

Idempotent: re-runs preserve files that contain a real value (any
non-blank, non-comment line), only create the ones still empty.

### Added — `claws apply --secrets-dir=<path>`

Auto-resolves any unresolved `tokenFrom.env` reference to a file under
`<path>` keyed off the env-var name. Convention matches the setup
script:

| env var | secrets file |
|---|---|
| `OPENAI_API_KEY` | `openai.key` |
| `ANTHROPIC_API_KEY` | `anthropic.key` |
| `GOOGLE_API_KEY` / `GEMINI_API_KEY` | `google.key` |
| `GROQ_API_KEY` | `groq.key` |
| `OPENROUTER_API_KEY` | `openrouter.key` |
| `TELEGRAM_BOT_TOKEN` | `telegram.token` |
| `DISCORD_BOT_TOKEN` | `discord.token` |
| `SLACK_BOT_TOKEN` | `slack.bot-token` |
| `SLACK_APP_TOKEN` | `slack.app-token` |

Anything not in the curated map falls through to a derivation rule:
`*_API_KEY` / `*_KEY` → `.key`, `*_TOKEN` → `.token`, `*_SECRET` →
`.secret`, with underscores normalised to dashes.

Files may contain comments (lines starting with `#`) and blank lines —
both are stripped at read time. The remaining content is the value.

Order of precedence at resolve time: **env var first, secrets-dir
file second.** Lets operators override per-shell without editing the
file.

### Added — `templates/demo/instant-bot.json`

New bundled template: minimum-input — one agent on Telegram with
OpenAI auth, sandbox enabled, conservative DM/policy defaults. Designed
to pair with `setup-secrets.sh`:

```bash
./scripts/setup-secrets.sh
$EDITOR /tmp/claws-secrets/openai.key
$EDITOR /tmp/claws-secrets/telegram.token
claws apply --template=demo/instant-bot --secrets-dir=/tmp/claws-secrets
claws start default/instant
# → DM your bot. It replies.
```

Three commands + two token pastes from a working install to a
responding bot on your phone.

### Added — `docs/goal-instant-claw.md`

Documents the design intent of the "GO" path: why this matters, the
exact acceptance criteria, what's in scope for v1.6.2 vs deferred, why
`/tmp/claws-secrets/` is the default location (no sudo, easy to wipe,
operator can override anywhere), and what other tasks are still
claimable after this lands.

### Added — `templates/demo/` namespace

New top-level namespace alongside `solo/`, `teams/`, `specialty/`.
Signals "for trying things out, not for production." Future:
`demo/instant-team` (group chat), `demo/local-only` (no channel), etc.

### Tests

- 7 new tests for `--secrets-dir`: curated map lookup, derivation
  fallback, no-dir-active short-circuit, comment-and-blank stripping,
  comments-only = effectively empty, end-to-end apply with secrets
  resolving from files, empty-file-still-missing case.
- Full suite green.

### Not in this release

- `templates/demo/instant-team.json` — group-chat variant. Deferred;
  solo bot is the MVP.
- Auto-detection of `~/.config/claws/secrets/` — operator can pass
  `--secrets-dir=~/.config/claws/secrets` explicitly.
- Interactive `--prompt` mode for missing values — still deferred.

## [v1.6.1] — 2026-05-24

Patch release — three day-one friction fixes for new users. Each one
closes a concrete failure mode an audit surfaced. No schema changes,
no breaking changes; pure additive UX.

### Added — `claws agent ping <name>`

Single-screen "is my agent responding?" command. Combines
`/healthz` + `/readyz` + auth-verify chain + recent-log tail into
one read-only check with a clear pass/fail summary and per-failure
fix command. Exits non-zero on any check failure so it composes
with shell pipelines.

Replaces the previous "grep across multiple log files and guess"
workflow with one command.

### Added — Missing-env detection in `claws apply`

`claws apply` now pre-checks every `tokenFrom.env` / `tokenFrom.file` /
`fallbackApiKey.fromEnv` reference at parse-time. If any are
unresolved, **fails loud before any state mutates** with a table:

```
cannot apply profile "solo-telegram-coder": 2 secret(s) not resolvable

  WHERE                                              SOURCE                         GET ONE AT
  agents[0].auth.fallbackApiKey (openai)             env:OPENAI_API_KEY             https://platform.openai.com/api-keys
  agents[0].channels[0].telegram tokenFrom           env:TELEGRAM_BOT_TOKEN         https://t.me/BotFather (/newbot)

Fix one of these ways:
  export OPENAI_API_KEY=<value>
  export TELEGRAM_BOT_TOKEN=<value>
```

Provider URLs hard-coded for OpenAI / Anthropic / Google / Groq /
OpenRouter (auth) and BotFather / Discord-dev / Slack-apps (channels).

Flags:
- `--allow-missing` keeps the v1.6.0 silent-skip behavior.
- `--dry-run` bypasses the check (inspection mode).
- `command:` references are not pre-checked (they exec dynamically).

This was the **single most common silent failure** in v1.6.0 — apply
exited 0 with no auth and no channels because env vars weren't set,
leaving the user with an agent that never responded.

### Added — `claws image bootstrap`

New command. One step from "fresh host" to "openclaw:local image
present". Three modes ordered fastest → slowest:

1. **Already present** — no-op.
2. **Pull** — if `--source=<image:tag>` or `OPENCLAW_IMAGE_SOURCE`
   env is set, `docker pull` from it (then tag as `openclaw:local`).
3. **Build from source** — `git clone github.com/openclaw/openclaw`
   (overridable via `--source-repo=`) + `docker build -t openclaw:local`.
   Requires `--yes` because the first build is 5-10 minutes.

Idempotent: re-running when the image already exists is a no-op.
`git pull --ff-only` on repeat builds instead of full clone.

Closes the "I followed install.sh and `claws doctor` says no image
and now what" gap.

### Tests

- 4 new tests for missing-env detection: rejection on missing,
  `--allow-missing` escape hatch, `--dry-run` bypass, provider hint
  lookup table.
- `claws agent ping` smoke-tested via a created-but-not-started
  agent (correctly reports 4 failing checks with fix commands).
- `claws image bootstrap` smoke-tested: idempotent skip when present.

### Internals

- `readEnvFile` helper for parsing `instance.env` cleanly (used by
  `agent ping`; previously every caller did ad-hoc parsing).
- New `agent` command namespace (reserved for future per-agent
  operator commands; only `ping` today).
- New `image bootstrap` subcommand under the existing `image` namespace.

## [v1.6.0] — 2026-05-24

**Contract alignment + fleet operator visibility + agent UUIDs.**

This release fixes silent feature failures in v1.5 (cron / hooks / skills
were written to paths the runtime didn't read), gives operators a clear
view into the data claws already writes, and introduces stable per-agent
UUIDs for cross-system integration.

### Fixed (silent v1.5 bugs)

- **Cron jobs now actually fire.** v1.5 wrote `workspace/cron/claws.crontab`
  in crontab format; the runtime image actually reads `<instance>/cron/jobs.json`
  in its own JSON format. v1.6 writes the runtime's shape, with each
  `agents[].cron[]` entry becoming a `{kind: systemEvent, text: ...}`
  payload the runtime dispatches as a prompt to the agent.
- **Hooks now actually run.** v1.5 wrote `<instance>/workspace/hooks/<event>.sh`
  per-agent; the runtime mounts `<team>/shared/hooks/` RO at
  `/home/node/.openclaw/shared-hooks`. v1.6 writes team-scoped by default.
- **Skills now actually mount.** Same scope mismatch — v1.5 wrote
  per-agent; runtime expects team-shared. v1.6 writes
  `<team>/shared/skills/` by default.
- **Per-agent paths still supported** via opt-in `Runtime.HooksScope =
  "agent" | "both"` (default `"team"`), same for `SkillsScope`. Only
  matters for non-openclaw runtimes.

### Added — Cron schema enhancement

- **`agents[].cron[].prompt`** — natural-language system event sent to
  the agent on each fire. Most natural mapping to the runtime's payload
  model. Legacy `command/hook/exec` continue to work and get wrapped as
  best-effort text payloads.

### Added — Migration helpers

- **`claws migrate cron`** — converts any legacy v1.5
  `workspace/cron/claws.crontab` to the new `cron/jobs.json` shape.
  Idempotent. Doesn't delete the legacy file (operator removes after verifying).
- **`claws migrate uuids`** — populates `CLAWS_INSTANCE_UUID` in every
  existing agent's `instance.env` (and mirrors to `openclaw.json meta.id`).
  Idempotent.
- **`claws migrate all`** — runs both.

### Added — Fleet/team operator visibility

- **`claws team tree <team>`** — ASCII topology renderer:
  ```
  org/
  └── lead (manager)
      ├── alpha-lead (manager)
      │   ├── alpha-worker-1 (worker) (peers: alpha-worker-2)
      │   └── alpha-worker-2 (worker) (peers: alpha-worker-1)
      └── beta-lead (manager)
          ├── beta-worker-1 (worker)
          └── beta-worker-2 (worker)
  ```
  With `--json` for tooling.
- **`claws cron list <agent>`** — runtime cron state: jobs + schedule +
  next-run + last-run + last-status. Reads `<instance>/cron/jobs.json`.
- **`claws cron tail <agent>`** — streams `cron/runs/*.jsonl` events
  newline-delimited (poll-based, 2s).
- **`claws fleet doctor`** — runs `claws doctor` + `audit` + `drift` +
  `orphans` in sequence with sectioned output and a summary. Exits
  non-zero on any failure.
- **`claws contract show [<runtime>]`** — prints the runtime adapter's
  declared contract (capabilities, hook/cron/skills paths and formats,
  mount points). Use this to verify "this runtime supports X" before
  writing a template that depends on X.
- **`claws contract list`** — list all registered runtimes.

### Added — Agent UUIDs (stable cross-system identifier)

- **Every new agent gets a UUID** at create time, stored in
  `instance.env` as `CLAWS_INSTANCE_UUID` and mirrored to
  `openclaw.json meta.id`. UUID v4, randomly generated.
- **`claws id <name>`** — print the UUID (script-friendly, one line).
- **`claws by-id <uuid>`** — reverse lookup → `team/name`.
- **Existing agents** get UUIDs via `claws migrate uuids` (idempotent).

### Added — Runtime adapter additions (Phase A-D)

- **`Runtime.HooksScope`** (`"team" | "agent" | "both"`, default `"team"`)
- **`Runtime.SkillsScope`** (same)
- **`Runtime.CronFormat`** can now be `"claws-jobs.json"` (v1.6, openclaw)
  or `"crontab"` (v1.5 legacy, for non-openclaw runtimes that prefer it).
- OpenClaw runtime updated: declares `HooksScope=team`, `SkillsScope=team`,
  `CronFormat=claws-jobs.json` (matches the live runtime image's actual contract).

### Tests

- 7 new unit tests for cron schedule conversion and payload shaping.
- 1 new test for legacy crontab parser (used by `migrate cron`).
- Updated v1.5 tests to assert the new v1.6 paths (jobs.json,
  team-shared hooks/skills).
- Full suite green.

### Honest notes

- **The events injection block (`agents[].events`)** still writes to
  `openclaw.json events.*` via `cmdConfig set`, but whether the runtime
  exposes an actual HTTP endpoint is **unverified** — `claws contract
  show` now flags this with a warning.
- **Sidecar declarations** (`agents[].sidecars[]`) remain
  configure-only: claws writes the integration JSON; the operator
  installs sharedwatch / intent-gateway separately.

### Migration guide (v1.5 → v1.6)

If you applied templates with cron / hooks / skills under v1.5:

```bash
# 1. Update claws
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | bash

# 2. Convert cron jobs to the new format (per-agent legacy crontab → jobs.json)
claws migrate cron

# 3. Populate UUIDs for existing agents
claws migrate uuids

# 4. Re-apply your templates so hooks/skills land at the new team-scoped paths
claws apply --template=<your-template>

# 5. Verify
claws contract show
claws team tree <your-team>
claws cron list <your-agent>
claws fleet doctor
```

### Not in this release

- `extends:` template composition — still v1.7+.
- Remote `--template=github:org/repo` — still v1.7+.
- Sidecar one-click installer — still v1.7+.

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
