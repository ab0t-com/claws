---
name: claws-add-agent
description: Add a NEW agent to an EXISTING claws fleet on this host — claws is already installed, docker prereqs are satisfied, other agents are already running. Walks through fleet discovery, name picking, `claws create`, per-agent auth (Codex OAuth or API key), channel wiring (Telegram / Discord / Slack / WhatsApp), start, and end-to-end verification with `claws agent ping`. Use when the user says any variant of "add a new agent", "add another claws", "add another worker to my team", "spin up a new bot called X", "create another agent for telegram/discord/slack", "new whatsapp agent", "add <name> to my fleet", "add a worker under <manager>", or asks how to bring up an Nth agent on an already-bootstrapped box. Also covers manager/worker role assignment and the per-agent OAuth isolation that prevents `refresh_token_reused` collisions when multiple agents share one ChatGPT account. Do NOT use this skill for "set up claws" / "install claws" on a fresh box (use claws-bootstrap-fresh-box), fixing an already-created broken agent (use claws-debug-agent), or cutting a claws release (use claws-release).
---

# claws — add an agent to an existing fleet

claws is already installed on this host and there are agents running. The user wants to add another one and get it talking on a channel. This skill is **only** for that flow — if claws itself isn't installed, hand off to `claws-bootstrap-fresh-box`; if an existing agent is broken, hand off to `claws-debug-agent`.

## Mental model

Each agent lives at `~/.openclaw/<team>/<agent>/` with its own port, gateway token, credentials, channels, and container. Agents in the same team share a `defaults.json` but are otherwise isolated. Adding an agent is six commands in order — create, auth, channel, start, ping, (optional) role-assign — each of which prints its own "what to run next" hint.

A successful add means:

1. `claws list` shows the new agent in `healthy` state with a port and RAM number.
2. `claws agent ping <team>/<name>` prints four green checks (gateway, readyz, auth, channels).
3. A message to the new channel gets a reply.

If all three are true, stop. If any fail, the failure modes section at the bottom covers the common cases.

## Playbook

Run in order. Skip steps marked "skip if" when the condition holds.

### Step 1 — See the current fleet

```bash
claws list
```

Read the output for:

- **Naming convention** — the part before `/` is the team. If existing rows are `team/sarah`, `team/john`, the operator uses `team/<short-name>`. Match that pattern unless the user explicitly says otherwise.
- **Existing names** — pick something that doesn't collide. `claws create` rejects duplicates.
- **Health** — if every existing agent is `healthy` and on the same provider, plan to reuse that auth method for the new one.

Skip if the user already told you the team and name to use, and `claws list` has been run this session.

### Step 2 — Detect what auth credentials are already in use

```bash
claws auth diagnose
```

This scans the audit log and probes each existing agent. Read the output for the **provider in use** (most commonly `openai-codex`) and the **per-agent verification status**. Two important takeaways:

- If existing agents use Codex OAuth and are all healthy, the new agent should also use Codex OAuth — but with its **own grant** (see Step 5 — sharing causes refresh-token collisions).
- If existing agents use an API key, the new agent can reuse the same key with no collision risk.

Skip if the user explicitly specified the auth method (`apikey openai`, `codex`, etc.) up front.

### Step 3 — Pick a name

Convention is `<team>/<short-name>`:

- **team** — match existing teams from `claws list` unless the user wants a new team.
- **short-name** — lowercase, friendly, no collision with rows in `claws list`.

If the user gave a name with a slash, use it verbatim. If they said "add a bot called *robin*" and existing agents are `team/sarah` and `team/john`, propose `team/robin`.

### Step 4 — Create

```bash
claws create <team>/<name>
```

This allocates a port (smart-skips orphan-held ports as of v1.6.11), generates a gateway token, and writes `instance.env` + `openclaw.json` under `~/.openclaw/<team>/<name>/`.

**If the name collides** with an existing agent, `claws create` errors out with a clear message. Either pick a different name and re-run, or — if the user invoked `claws setup` instead — step 4 of the wizard (since v1.6.8) offers reuse / rename / cancel.

**If building a multi-agent team**, add role flags here (see "Manager / worker roles" below):

```bash
claws create <team>/<name> --role=worker --manager=<manager-name>
```

### Step 5 — Set up auth

Two paths. Codex OAuth is the default for ChatGPT Plus subscribers; API key is simpler and has no collision risk.

**Path A — Codex OAuth (most common):**

```bash
claws auth <team>/<name> codex
```

This opens the OAuth flow **inside the container**. The user logs in with their ChatGPT account in the browser, then the container captures the grant.

> **CRITICAL — refresh-token-reuse trap.** If multiple agents share the same upstream ChatGPT account but reuse one grant, OpenAI will hit `refresh_token_reused` and silently break inference on all of them. **Every agent needs its own OAuth grant.** Run the `codex` flow per agent. For fleet-wide bulk auth: `claws auth fleet codex --missing-only` runs OAuth for every unauthenticated agent in sequence.

**Path B — API key:**

```bash
# OpenAI
claws auth <team>/<name> apikey openai sk-...

# Anthropic
claws auth <team>/<name> apikey anthropic sk-ant-...

# OpenRouter (multi-provider)
claws auth <team>/<name> apikey openrouter sk-or-...
```

API keys don't refresh, so there's no collision risk even when the same key is reused across agents.

### Step 6 — Add a channel

Interactive wizard (preferred when the user hasn't told you the channel + token):

```bash
claws channel <team>/<name> telegram
```

Direct, when the user already has the token in hand:

| Channel | Command |
|---|---|
| Telegram | `claws channel add <team>/<name> telegram --token=<botfather-token>` |
| Discord | `claws channel add <team>/<name> discord --token=<bot-token>` |
| Slack | `claws channel add <team>/<name> slack --bot-token=<xoxb-...> --app-token=<xapp-...>` |
| WhatsApp | QR-scan pairing — `claws exec <team>/<name> channels login --channel whatsapp` and scan with the WhatsApp client on a dedicated phone number |

**Phone-to-laptop secret tip** — if the bot token is on the user's phone but they're SSH'd from a laptop with no shared clipboard:

```bash
claws paste-secret telegram.token
# opens a phone-friendly local form
claws channel add <team>/<name> telegram --token-file=/tmp/claws-secrets/telegram.token
```

### Step 7 — Start

```bash
claws start <team>/<name>
```

This runs `docker compose up -d`, waits for the container `HEALTHCHECK`, then runs `auth verify`. Three possible end-states:

| Output | Meaning | Next |
|---|---|---|
| `✓ Auth verified (logs)` | Fully operational. | Step 8 (ping). |
| `Auth not verified yet — run claws agent ping <name>` | Gateway is up but no inference signal yet — common right after start. | Run ping to force one. |
| `WARNING: Auth check FAILED` | Auth is broken; the line below it prints the exact fix command. | Run that command verbatim. |

### Step 8 — Verify end-to-end

```bash
claws agent ping <team>/<name>
```

Four checks must be green: `gateway` (container HEALTHCHECK), `readyz` (HTTP 200), `auth` (verified via logs strategy), `channels` (configured count > 0). Then send a real message to the configured channel and confirm a reply.

If any check is red, the output itself prints the exact fix command — surface it verbatim, don't paraphrase. Common failures are listed below.

## Manager / worker roles

Only relevant if the user is building a multi-agent team where one agent coordinates work and others execute. Skip otherwise.

```bash
# 1. Create the manager first.
claws create team/lead --role=manager

# 2. Create workers, each pointing at the manager.
claws create team/dev1 --role=worker --manager=lead
claws create team/dev2 --role=worker --manager=lead
```

Then auth + channel + start each one as in Steps 5–7. The manager receives requests on its channel; tasks flow via:

```bash
claws task create team "<task description>" --from=lead
claws task claim  team <task-id> --by=dev1
```

State transitions use atomic filesystem `rename()` — **local storage only**, never on an S3 FUSE mount (atomicity guarantees don't hold).

## Common failure modes

In rough order of likelihood.

**Port already allocated (orphan container)**

Symptom: `claws create` or `claws start` says the port is in use. v1.6.11+ pre-flights this with smart allocation, but if an orphan slips through:

```bash
claws orphans                 # list orphan containers + the ports they hold
claws orphans clean <name>    # remove a specific orphan
```

Then retry.

**`refresh_token_reused` after sharing a ChatGPT account**

Symptom: multiple agents on the same upstream account all stop responding within minutes of each other; `claws auth diagnose` shows OAuth failures across the fleet.

Fix — give each agent its own grant:

```bash
claws auth fleet codex
```

Then `claws agent ping <each-name>` to confirm recovery.

**WhatsApp 401 loop**

Symptom: agent logs show repeated 401s from the WhatsApp endpoint; session expired and needs re-pairing.

```bash
claws exec <team>/<name> channels login --channel whatsapp
# scan the QR on the WhatsApp client
```

**Docker daemon not running**

Symptom: `claws create` or `claws start` says it can't talk to the Docker socket.

```bash
sudo systemctl start docker   # Linux
open -a Docker                # macOS Docker Desktop
```

**Name collision on `claws create`**

Symptom: `claws create` errors with "instance already exists". Pick a different name and re-run, or use `claws setup` (since v1.6.8 its step 4 offers reuse / rename / cancel).

**`auth verify` says "not verified yet" after start**

Not actually a failure — the gateway is up but no inference traffic has happened yet, so the log-scan strategy has nothing to verify against. Run `claws agent ping <team>/<name>` to force a check. If it still fails after that, fall through to `claws-debug-agent`.

## Decision shortcuts

- User just said "add a bot for telegram" and named no team → reuse the most common team prefix from `claws list`; name it after what the user called it.
- User has ChatGPT Plus and existing agents use Codex → use `claws auth <name> codex` per agent, never share a grant.
- User has an OpenAI / Anthropic API key in hand → use `claws auth <name> apikey <provider> <key>`, no collision risk.
- User is on a phone trying to paste a token over SSH → `claws paste-secret`.
- User wants several agents added at once → loop the playbook per agent, then `claws auth fleet codex --missing-only` to bulk-auth at the end if using OAuth.
- User is building a coordinator + workers setup → create the manager first with `--role=manager`, then workers with `--role=worker --manager=<lead>`.

## What success looks like

```
$ claws list
NAME              PORT     STATUS    RAM        UPTIME    NEXT
─────────────── ──────── ───────── ────────── ───────── ────
team/sarah        :18789   healthy   128.4MiB   3 hours   claws agent ping team/sarah
team/john         :18889   healthy   131.0MiB   3 hours   claws agent ping team/john
team/robin        :18989   healthy   126.8MiB   1 min     claws agent ping team/robin

$ claws agent ping team/robin
  ✓ gateway:   container reports healthy on :18989
  ✓ readyz:    /readyz 200 — agent ready to receive
  ✓ auth:      verified via logs strategy
  ✓ channels:  1 configured: telegram

✓ team/robin looks healthy
```

Send a message to the new channel, confirm a reply. Done.

## Out of scope — redirect to sibling skills

- claws is not installed on this host yet → `claws-bootstrap-fresh-box`.
- The new agent was created but is broken (auth failing, channel silent, container restarting) → `claws-debug-agent`.
- Cutting a new claws release (`scripts/publish-release.sh`, patch-only version bumps) → `claws-release`.
