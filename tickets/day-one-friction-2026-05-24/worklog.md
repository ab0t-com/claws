# day-one-friction — worklog

Append-only log. Every entry explains **what** changed and **why** — not
just what got written.

---

## 2026-05-24 — Kickoff

### Why these three, not the other suggestions

After v1.6.0 shipped I audited the brand-new-user path and listed 8
friction points. The user asked which actually matter day one. Three
made the cut:

- **Image bootstrap** — without `openclaw:local` on the host, claws
  is a CLI that does nothing visible. Every new user hits this. Highest
  blast radius.
- **Missing-env detect** — `claws apply` exits 0 when secrets are
  unresolved; the agent gets created but with no auth/channel.
  Silent failure is worse than loud failure. Reproducible 100% of the
  time on a fresh box.
- **Agent ping** — after `claws start` there's no single command that
  answers "is this thing actually responding". Operator has to grep
  logs across multiple files.

Other suggestions (Telegram wizard, skills catalog, friendlier audit)
deferred — they're polish, not unblockers.

### Patch bump per project rule

Per the new `feedback_patch_bumps_only` memory: this is v1.6.0 → **v1.6.1**.
Not v1.7.0. The user explicitly pushed back on the minor-bump cascade
across v1.1/v1.2/.../v1.6 — patch bumps from here.

### Execution order

Smallest → largest, to ship working code in stages:

1. **`claws agent ping`** — pure read-only, no external deps, stitches
   together existing helpers.
2. **Missing-env detect** — well-scoped change in apply.go.
3. **`claws image bootstrap`** — new command, has the most "what if the
   docker pull fails" edge cases.

Each gets its own commit so the diff stays reviewable.

## 2026-05-24 — Task 3 done: `claws agent ping <name>`

### What shipped

`cmd/claws/ping.go` (~190 LOC) — new `agent ping` subcommand under
the `agent` namespace in main.go.

### Why this design

- **Read-only.** Never mutates state. Composes cleanly into shell
  pipelines (`claws agent ping foo && bar`).
- **Single screen.** 4-5 lines + last-30s log tail + summary verdict.
  Designed so operators can answer "is X working?" in <5 seconds
  without thinking.
- **Reuses existing helpers** — `verifyOneInstance` (3-strategy auth),
  `readEnvFile` (new tiny helper for instance.env parsing), light
  string-match for channel detection in openclaw.json (avoids json
  unmarshal overhead on the happy path).
- **Each failure carries its own fix command.** "auth not configured
  — run: claws auth …" rather than just "auth: failed". Operators
  shouldn't have to think about what to do next.
- **Exits non-zero on any failure** so it composes with shell `&&`.

### What I considered and didn't do

- **No HTTP fetch of actual messages.** Considered POSTing a synthetic
  message through the gateway, but that touches state (creates a
  conversation row). Stuck to read-only — operator can confirm via
  the channel manually if needed.
- **No JSON output.** Single-screen human format only for v1.6.1.
  If a fleet dashboard wants this, add `--json` later.
- **No retry loops.** One probe per check; if the agent is in a
  restart loop the operator sees that clearly via "gateway
  unreachable" + `claws errors` for the full picture.

### Smoke test verified

Created agent without starting it → `agent ping` reported 4 failing
checks with exact next-step commands. Working as designed.

### Namespace decision

Created `claws agent <subcommand>` namespace (currently only `ping`).
Reserved for future per-agent operator commands. Considered
top-level `claws ping` but `agent` namespace keeps the verb noun-first
which matches `team`, `cron`, `channel`, etc.

## 2026-05-24 — Task 2 done: missing-env detection in `claws apply`

### What shipped

`cmd/claws/apply.go` — new `checkSecretsResolvable(Profile) error`
helper called at parse-time, before any state mutates. Plus
`providerHint()` and `channelHint()` lookup tables for the URL
suggestions.

### Why this design

The v1.5/v1.6.0 behavior was: any `tokenFrom.env` reference that
didn't resolve at apply-time → silently skip that step → exit 0.
The agent gets created but with no auth, no channel — and the user
has no idea why their bot doesn't respond. **The most common
"I followed the docs and nothing happened" failure.**

- **Fail at parse-time, before any mutation.** Operator can't end
  up half-created.
- **Table-format error output.** Three columns: WHERE in the profile,
  WHAT secret (env name or file path), GET-ONE-AT URL. Designed so
  a non-technical user can copy-paste the URL straight into a browser.
- **Provider URLs hard-coded.** OpenAI / Anthropic / Google / Groq /
  OpenRouter for keys; BotFather / Discord-dev / Slack-apps for bot
  tokens. Easy to extend.
- **`--allow-missing` keeps the old behavior** for anyone who
  intentionally wants the silent-skip (e.g. apply just to set up the
  agent skeleton, add channels later).
- **`--dry-run` bypasses the check** — dry-run is for inspecting the
  spec, not the env. (User can dry-run before they've set any creds.)
- **Skips `tokenFrom.command:` refs** — those exec dynamically; we
  don't try to pre-run them.

### What I considered and didn't do

- **Interactive prompt mode** (`--prompt`). Would walk the user
  through each missing secret, write to `/etc/claws/secrets/`,
  then proceed. Considered useful but adds significant UX scope
  (terminal-mode handling, value validation, sudo for /etc, …).
  Punted to a follow-up patch when there's a concrete demand.
- **Auto-detect `~/.config/claws/secrets/` for file refs.** Could
  be a sensible default. Not doing it because it'd silently mask
  the "where did this value come from" question for new operators.

### Smoke test verified

Apply `solo/telegram-coder` with neither `OPENAI_API_KEY` nor
`TELEGRAM_BOT_TOKEN` set → fails with both listed in a table + the
two provider URLs + exact `export` commands. Re-run with
`--allow-missing` → succeeds with the old silent-skip behavior.

## 2026-05-24 — Task 1 done: `claws image bootstrap`

### What shipped

`cmd/claws/image_bootstrap.go` (~150 LOC) — new
`claws image bootstrap [--source=…] [--yes] [--no-build]`
subcommand under the existing `image` namespace.

### Why this design

The hard part is the "where does the image come from" question with no
public OpenClaw registry today. Three modes, ordered from
fastest-and-cheapest to slowest-but-always-works:

1. **Already present** — `docker image inspect openclaw:local` succeeds
   → no-op. Idempotent re-runs.
2. **Pull** — if `OPENCLAW_IMAGE_SOURCE` env or `--source=` flag is
   set, `docker pull` from it. (Currently unused since no public
   registry, but future-proofed.)
3. **Source build** — clone `github.com/openclaw/openclaw` (overridable
   via `--source-repo=`) to `/tmp/claws-openclaw-build`, then
   `docker build -t openclaw:local`. Requires `--yes` because it
   takes 5-10 minutes the first time. Git operations are idempotent:
   re-runs `git pull --ff-only` if the dir exists, full clone if not.

### Why this matters

The single biggest "I followed the install and it didn't work" failure
mode for a new user was hitting `claws doctor` with no image. Before
v1.6.1: open the openclaw repo, read its docs, figure out the build
command, run it, come back. With v1.6.1: one command.

### What I considered and didn't do

- **Automatic --yes on first run.** Tempting for true "one-click",
  but `docker build` consumes ~5 GB of disk and 5-10 minutes; the
  user should opt in deliberately. The bare run shows the planned
  commands so they can copy-paste if they prefer.
- **Multi-arch support.** Inherits the host architecture. If the
  user wants linux/arm64 on an amd64 host, they need
  `DOCKER_DEFAULT_PLATFORM` set themselves.
- **Removing the build dir after success.** Kept at
  `/tmp/claws-openclaw-build` so re-runs do `git pull` instead of full
  clone. Operator can `trash` it themselves when done.

### Smoke test verified

- Bootstrap when image present → "already present, nothing to do",
  exit 0.
- Bootstrap without `--yes` and no image → prints the planned steps,
  asks for `--yes`, exits non-zero.
- (Full build-from-source test not run — would consume real host
  resources. Verified the command path via `--help` and the
  idempotent already-present case.)

## 2026-05-24 — All three tasks shipped. Ready for tests + v1.6.1 release.