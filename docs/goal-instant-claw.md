# GOAL — say "GO", have a claw on your phone

> **v1.6.4 update:** `claws setup` is now the single command to run.
> It walks you through everything — including building the openclaw
> image inline and bridging your phone token via `claws paste-secret`
> when you don't want to type a 46-char string into SSH. The
> "paste-two-tokens manually" flow below remains valid for power users.

# GOAL — say "GO", have a claw on your phone

**Status:** target for v1.6.2 (patch bump)
**Filed:** 2026-05-24

## The dream (the user's exact framing)

> "I want to say GO, and then on my phone I'd have a new bot or bot team
> group chat or something like that ready in an instant."

This is the day-one acceptance test for a brand-new user. Not the
contract, not the schema, not the LOC count — the lived experience of
"command → working bot in my pocket."

## Why this matters

Everything we shipped — install.sh, quickstart, apply, templates,
cron, hooks, sidecars, topology, UUIDs, fleet ops, contract show — is
infrastructure. None of it matters if a new user can't get from
`curl install.sh | sh` to **a bot that responds on Telegram** in
under 5 minutes.

The current happy path requires:
1. Install claws (works).
2. Build/pull openclaw image (fixed in v1.6.1 — `claws image bootstrap`).
3. Get a Telegram bot token from BotFather (5 minutes, user's job).
4. Get an OpenAI API key (5 minutes, user's job).
5. Stash them somewhere (currently: as env vars, easy to lose).
6. `claws apply` a template referencing those secrets.
7. `claws start` the agent.
8. DM the bot, see it reply.

Steps 1, 2, 6, 7, 8 are mechanical. Step 5 is the gap.

## The "GO" command (what shipping looks like)

```bash
# One-time per machine (~30 seconds):
./scripts/setup-secrets.sh
$EDITOR /tmp/claws-secrets/openai.key          # paste your key
$EDITOR /tmp/claws-secrets/telegram.token      # paste your BotFather token

# Every time after that, anywhere:
claws apply --template=demo/instant-bot --secrets-dir=/tmp/claws-secrets
claws start default/instant
# → DM your bot. It replies.
```

Three lines, idempotent, re-runnable. Pause anywhere, resume. Switch
machines: copy `/tmp/claws-secrets/` over, re-run the same three
lines, same bot.

## What ships in v1.6.2 to make this work

1. **`scripts/setup-secrets.sh`** — initialises `/tmp/claws-secrets/`
   with placeholder files (`openai.key`, `telegram.token`,
   `anthropic.key`, etc.), mode `0600`, plus a `README.md` explaining
   where each value comes from. Idempotent: re-runs preserve existing
   values, only create what's missing.

2. **`templates/demo/instant-bot.json`** — minimum-input template:
   one agent, one Telegram channel, OpenAI auth, sandbox enabled,
   conservative defaults. References secrets via `fromFile` so the
   template itself stays publishable.

3. **`claws apply --secrets-dir=<path>`** — new flag. When set, every
   unresolved `fromEnv: X` reference auto-checks for `<path>/<name>.key`
   or `<path>/<name>.token` as a fallback. Removes the "I have to
   either export env vars OR set fromFile in the template" friction —
   the secrets dir convention bridges both.

4. **`templates/demo/instant-team.json`** (stretch) — two bots in a
   Telegram group, peer mesh, shared workspace. Same secrets file
   convention.

## Acceptance criteria

A brand-new user on a brand-new EC2 box can do this and have a bot
responding on Telegram in **under 5 minutes wall-clock** (excluding
the OpenAI signup + Telegram BotFather time):

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | sh
claws image bootstrap --yes        # ~5 min the first time (build)
./scripts/setup-secrets.sh         # ~10 seconds
$EDITOR /tmp/claws-secrets/openai.key
$EDITOR /tmp/claws-secrets/telegram.token
claws apply --template=demo/instant-bot --secrets-dir=/tmp/claws-secrets
claws start default/instant
# → DM bot, it replies
```

## What's explicitly NOT in scope for v1.6.2

- Auto-installing OpenAI / Telegram credentials (operator's job).
- Interactive "tell me your token" prompt — operator pastes into a
  file. Keeps the flow scriptable + reproducible.
- Persistent credential storage beyond `/tmp/claws-secrets/`. If the
  box reboots and `/tmp` gets wiped, operator re-pastes. **For
  longer-lived storage**, the operator promotes to
  `~/.config/claws/secrets/` or `/etc/claws/secrets/` manually.
- The "instant-team" group-chat template if it adds complexity — solo
  bot is the MVP.

## Design notes

### Why `/tmp` for secrets

- No sudo required.
- Survives the user's current session; gets wiped on reboot.
- For dev / demo / first-run, that's the right trade-off. Production
  uses `/etc/claws/secrets/` or a vault.
- The setup script accepts `--dir=<path>` so anyone can override.

### Why `--secrets-dir` as a flag

- Pairs cleanly with the v1.6.1 missing-env detection: instead of
  failing because `OPENAI_API_KEY` is unset, the apply auto-resolves
  to `<secrets-dir>/openai.key`.
- Works for ANY template (not just demo ones) — the convention is
  `<env-var-lowercased>.{key,token,secret}` → file.
- Operator can override with explicit `fromFile:` references when
  they want a different layout.

### Why "demo/" namespace

- Sits alongside `solo/`, `teams/`, `specialty/`.
- Signals "this is for trying things out, not for production." Implies
  conservative defaults but not necessarily the locked-down policy
  posture an `enterprise/` template would have.
- Future: `demo/instant-team`, `demo/discord-test`, `demo/local-only`
  (no channel, just the gateway), etc.

## What else is claimable after v1.6.2

(Mentioned for completeness — not in v1.6.2 scope.)

From the v1.6.0 deferred list:
- `extends:` template composition
- Remote `--template=github:org/repo@ref`
- Sidecar one-click installer (`claws sidecar install sharedwatch`)
- `claws agent show <name>` consolidated overview
- `claws team task-graph` (existing `team show` covers most)
- Subprocess test-coverage merging (lifts the 9.5% number)

From the v1.6.1 deferred list:
- `--prompt` mode for missing-env (interactive collection)
- Telegram BotFather walkthrough wizard
- Skills catalog browser
- `~/.config/claws/secrets/` auto-detect

Plus the 5 still-open runtime contract questions (my recommendations
documented in the chat — your call to confirm each one).
