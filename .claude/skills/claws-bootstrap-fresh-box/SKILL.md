---
name: claws-bootstrap-fresh-box
description: End-to-end bootstrap of claws (the Go CLI that runs a team of AI agents on one server) on a fresh Linux/macOS host, from "nothing installed" to "first agent responding to a Telegram message". Use when the user says any variant of "set up claws on this box", "install claws from scratch", "fresh EC2 / VPS / droplet, get claws running", "bootstrap claws", "spin up claws on a new server", "deploy claws to a new host", or asks how to onboard a brand-new machine to the claws fleet. Covers OS + TTY + root detection, friendly vs. audit-managed prereq installation, the claws installer, the `claws setup` wizard vs. scripted agent creation, channel wiring (Telegram/Discord/Slack/WhatsApp), and end-to-end verification with `claws agent ping`. Do NOT use this skill for adding an agent to an already-installed claws (use claws-add-agent), debugging an existing broken agent (use claws-debug-agent), or cutting a new claws release (use claws-release).
---

# claws — fresh-box bootstrap

Bring claws up on a brand-new Linux box (sometimes macOS, sometimes WSL2), from zero to a first agent that replies on Telegram. This skill is for the **first install on this host only**. For follow-on agents use `claws-add-agent`; for fixing an already-created but broken agent use `claws-debug-agent`.

## Mental model

claws is a single static Go binary (`claws`) that manages a fleet of containerised AI agents under `~/.openclaw/<team>/<agent>/`. Each agent has its own docker container, port, credentials, persona, and channel (Telegram / Discord / Slack / WhatsApp). The host needs `docker` + `docker compose v2` + `bash` + `curl`. Everything else (Go, source) is optional.

A "successful bootstrap" means:

1. `docker` and `docker compose` work for the current user without `sudo`.
2. `claws version` prints a version.
3. `claws list` shows one agent.
4. `claws agent ping <team>/<agent>` shows four green checks (gateway, readyz, auth, channels).
5. A message sent to the agent's Telegram bot gets a reply.

If all five are true, stop. If any are false, the fix is almost always in step 7 ("Common failure modes") below.

## Playbook

Run the steps in order. Each step has a "skip if" condition — honour it; re-running prereqs on a fully provisioned box wastes minutes.

### Step 1 — Detect the host

Before doing anything else, run these probes (cheap, sets the strategy for every later step):

```bash
uname -a
cat /etc/os-release 2>/dev/null || sw_vers 2>/dev/null
id -u            # 0 = root, anything else = unprivileged
[ -t 0 ] && echo "tty" || echo "no-tty"
echo "${CLAWS_NO_INSTALL:-unset}"
```

Decisions to make from the output:

| Signal | Implication |
|---|---|
| `id -u` is `0` | Don't need `sudo`; installers run directly. Also: docker group membership is moot. |
| `[ -t 0 ]` prints `no-tty` | Pass `--yes` / `-s -- --yes` to installers; the wizard's prompts will not work — fall back to scripted setup. |
| `CLAWS_NO_INSTALL=1` is set | Host is policy-managed. Use `scripts/prereqs-strict/` only, and start with `--audit` to print commands without running them. Surface the env var to the user. |
| `/etc/os-release` shows `ID=ubuntu`/`debian`/`fedora`/`rhel`/`arch`/`alpine` | Friendly installers support all of these. |
| `sw_vers` returns macOS | Docker Desktop is needed; the daemon is started via `open -a Docker`, not `systemctl`. |
| `uname -r` contains `microsoft` / `WSL2` | Tell the user to run inside WSL Ubuntu, not native Windows PowerShell. |

### Step 2 — Check what's missing

Prefer the **local** copy of the check script if there's a claws checkout at the current working directory or under `~/claws*`:

```bash
# inside a claws checkout
./scripts/prereqs/check.sh

# or, on a managed host, machine-readable form
./scripts/prereqs-strict/check.sh --json
```

Otherwise pull it from the published main:

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/check.sh | bash
```

This is read-only. It prints which of `docker`, `docker compose`, `curl`, `git` are present and which are missing. If it says everything is present **and** `docker info` succeeds for the current user without `sudo`, skip Step 3.

### Step 3 — Install missing prereqs

Two paths. Pick based on Step 1.

**Friendly (default for personal VPS / EC2):**

```bash
# Interactive TTY
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-all.sh | bash

# Non-TTY / cloud-init / agent automation / root-only box
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-all.sh | bash -s -- --yes
```

This auto-detects the OS, installs docker engine + compose v2, starts the daemon, and (if not root) adds `$USER` to the docker group. It's idempotent — re-running is safe.

**Audit-managed (corporate host, `CLAWS_NO_INSTALL=1`, or user mentions compliance):**

Always preview first:

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/install-all.sh | bash -s -- --audit
```

Then, only after the user confirms the printed plan is acceptable, re-run without `--audit`:

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/install-all.sh | bash
```

Other strict flags worth knowing: `--no-group` skips docker-group membership (use when the host already has fine-grained policy). `CLAWS_PREREQS_LOG=/path/to.log` redirects the audit log.

After install, verify docker works **as the current user, without sudo**:

```bash
docker version && docker compose version && docker info >/dev/null && echo OK
```

If `docker info` fails with a permission error, jump to Step 7 (docker group fix).

### Step 4 — Install the claws binary

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | bash
claws version
```

The installer SHA256-verifies the published tarball, drops the binary in `/usr/local/bin` (or `~/.local/bin` if the former isn't writable), and keeps a `.previous` for one-step rollback. If the user needs a pinned version: `bash -s -- --version=v1.6.17`.

Skip if `claws version` already prints a recent version — but mention that `claws update` exists for self-updating later.

### Step 5 — Bootstrap the first agent

Two paths.

**Path A — Interactive wizard (default, when there's a TTY):**

```bash
claws setup
```

The wizard is 6 steps: docker prereq recheck → team name → agent name → auth method → channel → start. It auto-detects credentials already present:

- `$OPENAI_API_KEY` / `$ANTHROPIC_API_KEY` in env
- `~/.codex/` (existing Codex OAuth session)
- `~/.claude/` (existing Claude credentials)
- Any existing claws agents (offers to clone config)

If the docker check inside the wizard fails, the wizard prints the install-all.sh URL — surface it to the user verbatim, don't paraphrase.

**Path B — Scripted (non-TTY, automation, or user explicitly wants reproducible commands):**

```bash
# Substitute real values for <team>, <agent>, <key>, <bot-token>
claws create <team>/<agent>
claws auth   <team>/<agent> apikey openai <key>
claws channel add <team>/<agent> telegram --token=<bot-token>
claws start  <team>/<agent>
```

Token-pasting tip: if the user is going to type a 46-character Telegram bot token over SSH from a phone, run `claws paste-secret` instead — it opens a small local HTTP form that's friendly to mobile copy/paste, so the token never goes through the SSH session.

Auth shapes:

| Method | Command shape | When |
|---|---|---|
| OpenAI API key | `claws auth <agent> apikey openai <sk-...>` | User has an OpenAI key. |
| Anthropic API key | `claws auth <agent> apikey anthropic <sk-ant-...>` | User has an Anthropic key. |
| OpenRouter | `claws auth <agent> apikey openrouter <sk-or-...>` | Multi-provider via OpenRouter. |
| Codex OAuth | `claws auth <agent> codex` (interactive) | User wants to use their ChatGPT subscription. |

### Step 6 — Verify end-to-end

Two commands, both must look healthy:

```bash
claws agent ping <team>/<agent>
claws list
```

`agent ping` should print four green checks: `gateway`, `readyz`, `auth`, `channels`. `list` should show the agent in `healthy` state with a port and RAM number. Then send a real message to the Telegram bot and confirm a reply arrives.

If any check fails, the output itself prints the exact fix command — surface it to the user, don't invent one. Common fixes are in Step 7.

### Step 7 — Common failure modes

These cover ~95% of fresh-box failures. Check them in order.

**Docker daemon isn't running**

Symptom: `docker info` says "Cannot connect to the Docker daemon."

```bash
# Linux
sudo systemctl start docker
sudo systemctl enable docker

# macOS Docker Desktop
open -a Docker
```

**User isn't in the docker group**

Symptom: `docker info` says "permission denied while trying to connect to the Docker daemon socket."

```bash
# Apply group membership in this shell only:
newgrp docker

# Or log out and back in for all sessions to pick it up.
# (sudo usermod -aG docker $USER was already done by install-all.sh.)
```

If the user is `root`, this isn't applicable — root talks to the docker socket directly.

**Wizard fails the docker prereq check**

The wizard will print a URL like `https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-all.sh`. Surface that URL exactly; the user runs it (with `bash -s -- --yes` on a non-TTY host) and then re-runs `claws setup`.

**`claws update` vs `claws upgrade` — these are different**

- `claws update` — self-updates the **claws binary**.
- `claws upgrade` — upgrades agent **container images**.

Never use one when the user meant the other. On a fresh bootstrap, neither should be needed.

**WSL2 / Windows**

claws is a Linux/macOS binary. On Windows, the user must run everything inside WSL Ubuntu — `claws setup` from PowerShell will not work. If the prompt shows `PS C:\>`, tell the user to `wsl` first.

**Policy-managed host with `CLAWS_NO_INSTALL=1`**

Friendly installers will run anyway, but the strict ones will refuse. This is intentional. Use `scripts/prereqs-strict/` with `--audit` first to produce the plan; the user (or their ops team) executes it manually if they don't want the script to.

## Decision shortcuts

- Fresh personal EC2 / VPS, has sudo, has a TTY → friendly path, interactive wizard.
- Cloud-init / Terraform user_data / root-only box / no TTY → friendly path with `--yes`, then scripted agent creation.
- Corporate host or `CLAWS_NO_INSTALL=1` → strict path with `--audit` preview first.
- macOS dev box → friendly path; remember Docker Desktop, `open -a Docker`, no systemctl.
- WSL2 → only inside WSL Ubuntu; otherwise stop and redirect.
- User wants to paste a long secret from a phone → `claws paste-secret`.

## What success looks like

```
$ claws list
NAME              PORT     STATUS    RAM        UPTIME    NEXT
─────────────── ──────── ───────── ────────── ───────── ────
team/sarah        :18789   healthy   128.4MiB   2 min     claws agent ping team/sarah

$ claws agent ping team/sarah
  ✓ gateway:   container reports healthy on :18789
  ✓ readyz:    /readyz 200 — agent ready to receive
  ✓ auth:      verified via logs strategy
  ✓ channels:  1 configured: telegram

✓ team/sarah looks healthy
```

Send a Telegram message to the bot, confirm a reply. That's done.

## Out of scope — redirect to sibling skills

- Adding a second / third / Nth agent to a working claws install → `claws-add-agent`.
- An agent that was created but is unhealthy (auth failing, channel silent, container restarting) → `claws-debug-agent`.
- Cutting a new claws release (`scripts/publish-release.sh`, patch-only version bumps) → `claws-release`.
