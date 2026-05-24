# The one-click pathway — how `say GO and a bot appears` actually works

**Audience:** anyone who wants to understand what claws does under the
hood when a user runs the v1.6.3 demo flow, debug it when it doesn't,
or extend it (new templates, new sidecars, new runtimes).

**Sibling doc:** `docs/goal-instant-claw.md` (the design intent),
`quickstart_guide.md` (the run-through), `templates/README.md` (the
schema reference).

---

## The flow in one diagram

```
  ┌─────────────────────────────────────────────────────────────────┐
  │  1. install.sh        2. image bootstrap     3. setup-secrets   │
  │  (claws on PATH)      (openclaw:local)       (/tmp/claws-secrets│
  │                                                + placeholders)  │
  └──────────┬──────────────────┬──────────────────────┬────────────┘
             │                  │                      │
             ▼                  ▼                      ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │  4. operator pastes 2 tokens into /tmp/claws-secrets/*.{key,token}│
  └──────────┬──────────────────────────────────────────────────────┘
             ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │  5. claws apply --template=demo/instant-bot                     │
  │                 --secrets-dir=/tmp/claws-secrets                │
  │                                                                  │
  │     ├── resolves --template=demo/instant-bot via template.go    │
  │     ├── parses JSON profile + validates schema                  │
  │     ├── pre-checks every fromEnv ref (fail loud if missing)     │
  │     ├── activeSecretsDir = "/tmp/claws-secrets"                 │
  │     ├── init / policy / access / group / agent (skip if exists) │
  │     ├── per-agent: config sets, skills, hooks, cron, sidecars   │
  │     ├── auth (apikey openai) — reads /tmp/claws-secrets/openai.key│
  │     ├── channel add telegram — reads /tmp/claws-secrets/telegram.token│
  │     └── audit                                                    │
  └──────────┬──────────────────────────────────────────────────────┘
             ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │  6. claws start default/instant                                  │
  │     └── docker compose up (gateway + openclaw runtime container) │
  │     └── runtime image polls Telegram for messages                │
  └──────────┬──────────────────────────────────────────────────────┘
             ▼
  ┌─────────────────────────────────────────────────────────────────┐
  │  7. user DMs bot on phone → Telegram → runtime → OpenAI → reply │
  └─────────────────────────────────────────────────────────────────┘
```

---

## Each layer — what it does, where it lives, how to debug

### Layer 1: install.sh

**What:** Downloads the `claws` binary tarball from
`raw.githubusercontent.com/ab0t-com/claws/<tag>/release/`, verifies
its SHA256 against the published `SHA256SUMS`, installs to
`/usr/local/bin/claws`, drops static assets (compose template, docs,
html, templates) at `~/.local/share/claws/`.

**Code:** `scripts/install.sh`.

**Idempotent?** Yes — refuses to overwrite an existing newer binary
without `--force`. Re-runs are safe.

**Debug if it fails:**
- HTTPS unreachable? Network / firewall. The installer is HTTPS-only,
  no fallback.
- SHA mismatch? Tampering or a partial download. Re-run.
- Wrong arch? `uname -m` should be x86_64 or aarch64 → linux/darwin.
  Windows is unsupported (Unix syscalls).

### Layer 2: `claws image bootstrap`

**What:** Ensures `openclaw:local` Docker image is present. Three modes:
1. Already present → no-op.
2. `--source=<tag>` or `OPENCLAW_IMAGE_SOURCE` env → `docker pull`.
3. Fallback → `git clone github.com/openclaw/openclaw /tmp/claws-openclaw-build && docker build -t openclaw:local`.

**Code:** `cmd/claws/image_bootstrap.go`.

**Idempotent?** Yes — checks `docker image inspect openclaw:local`
first.

**Debug:** `docker image inspect openclaw:local` from a shell.
Missing? Re-run with `--yes`. Build fails? Read the `docker build`
output for missing deps.

### Layer 3: `setup-secrets.sh`

**What:** Creates `/tmp/claws-secrets/` (or `--dir=<path>`), chmod 700,
with 9 placeholder files (chmod 600). Each placeholder is a comment
header explaining where to get the value + a blank below it.

**Code:** `scripts/setup-secrets.sh`.

**Idempotent?** Yes — files that already contain a real value (any
non-blank, non-comment line) are preserved.

**Debug:** `ls -la /tmp/claws-secrets/`. Files all there but token
not resolving? Make sure your paste is on its own line, no surrounding
whitespace.

### Layer 4: pasting tokens

This is the only manual step. Two values:
- `openai.key` ← from platform.openai.com/api-keys
- `telegram.token` ← from BotFather `/newbot`

The file may contain `#` comment lines and blanks — they're stripped
at read time. The value itself goes on its own line.

### Layer 5: `claws apply`

This is the largest piece. Sub-steps:

**5a. Template resolution** (`cmd/claws/template.go`):
Search `./templates/`, `$XDG_DATA_HOME/claws/templates/`,
next-to-binary. `--template=demo/instant-bot` resolves to
`<one of those>/demo/instant-bot.json`. Bare names work too;
ambiguity errors loud.

**5b. Schema validation** (`cmd/claws/apply.go`):
Reject `apiVersion != claws.ab0t.com/v1`, missing `team.name`, empty
`agents`, bad agent names. Topology cycle detection.

**5c. Secret pre-flight** (`cmd/claws/apply.go`):
Walk every `tokenFrom.env`, `fromFile`, `fallbackApiKey.fromEnv`. Check
each resolves (either env var set, file present, or `--secrets-dir`
fallback file present). Missing → fail loud with provider URLs.
**This catches the v1.5/v1.6.0 silent footgun.**

**5d. `--secrets-dir` activation** (`cmd/claws/apply.go`):
Sets the package-global `activeSecretsDir`. The `SecretRef.resolve()`
method falls back to `<dir>/<derived-filename>` when an env var is
unset. The curated env→file map lives in the same file:

| env | file |
|---|---|
| OPENAI_API_KEY | openai.key |
| ANTHROPIC_API_KEY | anthropic.key |
| TELEGRAM_BOT_TOKEN | telegram.token |
| DISCORD_BOT_TOKEN | discord.token |
| SLACK_BOT_TOKEN | slack.bot-token |
| SLACK_APP_TOKEN | slack.app-token |
| (others) | derived: lowercased, _KEY→.key, _TOKEN→.token, _SECRET→.secret |

Order of precedence at apply time: **env var first, secrets-dir file second.**

**5e. init / policy / access / group** — all skip-if-exists.

**5f. Per-agent loop:**
- Create instance via `cmdCreate` (allocates port, writes
  `instance.env` incl. `CLAWS_INSTANCE_UUID`, writes
  `openclaw.json`).
- Apply `config:` map entries (sandbox, tools, custom keys) via
  `cmdConfig set`.
- Materialise skills → `<team>/shared/skills/`.
- Materialise hooks → `<team>/shared/hooks/<event>.sh`.
- Materialise cron → `<instance>/cron/jobs.json` in the runtime's
  JSON shape.
- Apply events config to `openclaw.json events.*`.
- Write sidecar declarations to `<instance>/workspace/sidecars/`.
- Add channels via `cmdChannel add` — pulls token via SecretRef
  (which goes through env → secrets-dir → file fallback chain).
- Run `cmdAuth apikey <provider> <key>` — same SecretRef chain.
- Write topology.json (manager, peers, auto-derived workers).

**5g. postSetup verbs** — `audit` runs by default (skippable with
`--skip-audit`).

**Idempotent?** Across the board, yes. Init/policy/access/group/agent
all skip-if-exists. Channel + auth have their own idempotence at
the runtime layer. Skills/hooks/cron files are content-hashed —
re-written only if the body differs.

**Debug:** `claws apply --dry-run` shows what it'd do without
mutating. `claws contract show` shows what the runtime adapter claims
to support. `claws agent show <name>` (when shipped) consolidates
per-agent state.

### Layer 6: `claws start`

**What:** `docker compose up -d` against the per-instance compose +
the override generated at create time. Starts two containers:
- `openclaw-gateway` (HTTP gateway, polls Telegram, dispatches to
  the runtime)
- `openclaw-cli` (optional CLI sidecar — for `claws exec`)

**Code:** `cmd/claws/commands.go` `cmdStart` + `compose.go`.

**Idempotent?** Yes — if already running, no-op. `--hard` recreates.

**Debug:** `docker ps | grep openclaw`, `claws logs <name>`,
`claws agent ping <name>`, `claws health <name>`.

### Layer 7: actual conversation

The runtime image (openclaw) is responsible for:
- Long-polling Telegram (`getUpdates`).
- Routing messages into the agent loop.
- Calling the LLM with the conversation context.
- Dispatching the LLM's reply back to Telegram.
- Honoring DM pairing policy (refusing unknown senders).
- Reading `<team>/shared/hooks/onMessage.sh` if present.
- Walking `<instance>/cron/jobs.json` on its own schedule.

claws does NOT mediate runtime traffic — once the container is up,
the runtime owns the conversation. claws is back to being a control
plane (config + lifecycle).

**Debug:** `claws logs <name> -f` to see runtime output.
`claws audit` for security posture. `claws errors` for incident triage.

---

## Security posture at each step

| Layer | Risk | Mitigation |
|---|---|---|
| install.sh | Tampered binary | SHA256 verification against published checksums |
| image bootstrap | Tampered openclaw image | Pinned to `openclaw:local`; operator can verify the source repo |
| setup-secrets.sh | World-readable secrets | Dir chmod 700, files chmod 600 |
| Secrets in /tmp | Wiped on reboot, world-mounted in some containers | Operator can use `--dir=/etc/claws/secrets` for persistence + access control |
| apply | Profile typos → policy violations | Schema validation; pre-flight missing-secret check; cycle detection |
| Channel tokens in openclaw.json | Plaintext on disk | `chmod 600 instance.env` + `claws audit` flags unprotected files |
| Container | Compromised agent could ransom files | `sandbox=true` default; `cap_drop: ALL` in compose; no Docker socket |
| Bot polling | Stranger DMs the bot | `dmDefault: pairing` requires pairing code on first contact |

---

## How to extend

### New template

1. Write a JSON profile under `templates/<namespace>/<name>.json`.
2. Validate: `claws apply --file=templates/<namespace>/<name>.json --dry-run`.
3. PR to `ab0t-com/claws-templates` (future repo) or bundle locally.

### New secret type

Two small edits in `cmd/claws/apply.go`:
- Add to `envToSecretFile` map: `"MY_NEW_KEY": "myservice.key"`.
- Add to `providerHint()` or `channelHint()` if you want a URL hint.

### New sidecar helper

Two edits:
- Add the kind to the switch in `applySidecar()` (currently
  `sharedwatch | intent-gateway | custom`).
- (Future) Wire up auto-injection of hooks/config that the sidecar
  needs to integrate.

### New runtime

1. Define a `Runtime` JSON file (40 fields — see `cmd/claws/runtime.go`
   for the openclaw example).
2. `claws runtime import <file>` registers it.
3. Templates can use `runtime: { name: "<your-name>" }`.

---

## What this pathway is NOT

- **Not a SaaS.** Everything runs on your host. Your tokens never
  leave. The claws repo is MIT.
- **Not a Telegram-specific tool.** Telegram is the demo channel
  because BotFather is the lowest-friction signup. Discord, Slack,
  WhatsApp, Signal all work via the same `channels[]` schema.
- **Not OpenAI-specific.** Anthropic, Google, Groq, OpenRouter all
  work via `fallbackApiKey.provider`. Codex OAuth too.
- **Not a substitute for the runtime image.** claws orchestrates;
  openclaw (or any other runtime adapter) actually runs the agent.
- **Not 100% silent.** The runtime emits logs; the operator watches
  them via `claws logs -f` or `claws fleet doctor` or `claws cron
  tail`.

---

## Quick reference — every command in the GO path

```bash
# One-time per machine
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | sh
claws image bootstrap --yes
bash <(curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/setup-secrets.sh)
$EDITOR /tmp/claws-secrets/openai.key
$EDITOR /tmp/claws-secrets/telegram.token

# Every time after that
claws apply --template=demo/instant-bot --secrets-dir=/tmp/claws-secrets
claws start default/instant

# Optional sanity checks
claws agent ping default/instant
claws fleet doctor
claws logs default/instant -f
```

Done.
