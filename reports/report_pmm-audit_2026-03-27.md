# PMM Audit: claws — One-Click Installer Vision

**Date:** 2026-03-27
**Perspective:** Product Marketing Manager — user onboarding, safe defaults, object model clarity, one-click installer goal
**Scope:** Full system review from zero to running team

---

## The Vision

> "One-click installer for a clawbot team"

This means: a new user on a fresh server should go from nothing to a working team of AI agents with channels connected, security configured, and shared workspace active — with minimal decisions and zero expertise required.

**Current reality:** 15+ commands to get a team running. No guided flow. Expert-level knowledge assumed.

---

## The Current Journey (Honest Assessment)

### Step 1: Install
There is no install. User must:
1. Have Go installed
2. Clone the repo
3. `go build -o claws .`
4. Have Docker installed and running
5. Have the OpenClaw image built locally

**No binary distribution. No `curl | sh` installer. No package manager.**

### Step 2: Init
```bash
claws init
```
This works. Creates dirs, checks Docker, copies compose template. **This is good.**

But then:
- No policy is created (insecure defaults)
- No access control is created
- No guidance on what to do next beyond "create an instance"

### Step 3: Create First Agent
```bash
claws create alice
```
This works. But:
- No auth is configured — the agent can't talk to any AI model
- No channels — the agent can't receive messages
- No personality — AGENTS.md, SOUL.md, IDENTITY.md are empty
- The user has to know that auth comes next, then channels, then start

### Step 4: Auth
```bash
claws auth alice codex
```
Requires interactive TTY for OAuth. Breaks when run via `claws exec`.
Or:
```bash
claws auth alice apikey anthropic sk-...
```
The onboarding prompt blocks on a security warning that defaults to "No" in non-TTY mode.

### Step 5: Connect Channels
```bash
claws channel add alice telegram --token=TOKEN
```
This works now (we built it). But:
- User must already have a bot token
- No guidance on how to get one
- No "which channels do you want?" prompt

### Step 6: Start
```bash
claws start alice
```
Works. Health checks. Waits.

### Step 7: Approve Pairing
```bash
claws approve alice telegram CODE
```
User must message the bot, see the code, come back to terminal. Works but two-context switching.

### Step 8: Create a Team
```bash
claws group create team
claws create team/bob
claws auth team/bob ...
claws channel add team/bob ...
claws start team/bob
claws approve team/bob ...
claws group shared team --all
```
Repeat steps 3-7 for each agent. **7 commands per agent.**

### Step 9: Security
```bash
claws policy init
claws policy enforce --restart
claws access init
```
Most users won't know to do this. Security is opt-in, not default.

**Total: 15+ manual commands, 5+ context switches, multiple pieces of external knowledge required.**

---

## Object Model — What Users Need to Understand

Currently the user must grasp these concepts:

| Object | What It Is | Discovered Via |
|--------|-----------|----------------|
| Instance | A running agent container | `claws list` |
| Group | A collection of instances with shared resources | `claws group list` |
| Role | Manager or worker within a group | `--role=` flag on create |
| Runtime | Which agent software to run | `claws runtime list` |
| Channel | A messaging platform connection | `claws channel status` |
| Policy | Admin security constraints | `claws policy show` |
| Access | Who can run what commands | `claws access show` |
| Task | A unit of work in the manager/worker queue | `claws task list` |

**That's 8 objects.** For a "one-click" experience, users should only need to think about 3:
1. **Team** — "I want a team of agents"
2. **Agent** — "Each agent has a name, a model, and channels"
3. **Channel** — "How people message the agent"

Everything else (groups, roles, runtimes, policies, access, tasks) should be handled automatically or deferred.

---

## What "One-Click" Actually Means

### The Ideal Flow

```bash
# Install
curl -sL https://get.clawctl.dev | sh

# Setup (one interactive command)
claws setup
```

The `setup` command would:

1. **Check prerequisites** (Docker, image) — pull image if missing
2. **Create OPENCLAW_ROOT** (like `init`)
3. **Create policy** with secure defaults (like `policy init`)
4. **Create access control** with current user as admin (like `access init`)
5. **Ask: "What's your team name?"** → creates group
6. **Ask: "Name your first agent?"** → creates instance in group
7. **Ask: "How should it authenticate? (Codex OAuth / API key)"** → runs auth
8. **Ask: "Connect a channel? (Telegram / WhatsApp / Discord / Skip)"** → runs channel add
9. **Start the agent, wait for health**
10. **Print: "Your agent is live. Message @bot on Telegram."**
11. **Ask: "Add another agent? (y/n)"** → repeat 6-8

One command. Interactive prompts. Safe defaults. Working team at the end.

### For Non-Interactive (CI/CD, Scripts)

```bash
claws setup --non-interactive \
  --team=research \
  --agent=sarah \
  --auth=codex \
  --channel=telegram --telegram-token=TOKEN \
  --agent=john \
  --auth=apikey --anthropic-key=sk-... \
  --channel=telegram --telegram-token=TOKEN2
```

### For Existing Users Migrating

```bash
claws setup --migrate
# Discovers existing instances, creates group, applies policy
```

---

## Safe Defaults Audit

| Setting | Current Default | Safe Default | Status |
|---------|----------------|-------------|--------|
| Network bind | `loopback` | `loopback` | **Good** ✓ |
| Memory limit | `2G` | `2G` | **Good** ✓ |
| Cap drop | `ALL` | `ALL` | **Good** ✓ |
| No-new-privileges | `true` | `true` | **Good** ✓ |
| DM policy | `pairing` | `pairing` | **Good** ✓ |
| Gateway token | 256-bit random | 256-bit random | **Good** ✓ |
| File permissions | `0600` | `0600` | **Good** ✓ |
| Docker socket | Not mounted | Not mounted | **Good** ✓ |
| Policy created on init | **No** | Should be yes | **Gap** ✗ |
| Access control on init | **No** | Should be yes | **Gap** ✗ |
| Audit logging | **Off** | Should be on | **Gap** ✗ |
| Sandbox mode | **Off** | Should be on | **Gap** ✗ |
| Tool profile | **Not set** | Should default to `coding` | **Gap** ✗ |
| Group sharing on group create | **No auto-share** | Should enable workspace + skills | **Already done** ✓ |

### Fixes Needed

`claws init` should automatically:
- Create `policy.json` with secure defaults
- Create `.access.json` with current user as admin
- Enable audit logging
- Print what was configured and why

`claws create` should automatically:
- Set `tools.profile = "coding"` in the config
- Warn if sandbox is not enabled (not block, just warn)

---

## UX Problems by Category

### 1. Too Many Commands for Common Operations

| What User Wants | Commands Required | Should Be |
|----------------|-------------------|-----------|
| "Set up a new server" | `init` + `policy init` + `access init` | `claws setup` |
| "Create an agent with Telegram" | `create` + `auth` + `channel add` + `start` | `claws create alice --auth=codex --telegram=TOKEN` then `start` |
| "Create a team" | `group create` + N × (`create` + `auth` + `channel add` + `start`) | `claws setup` or `claws team create` |
| "Check everything is okay" | `health` + `audit` + `policy validate` | `claws status` (unified) |

### 2. Init Doesn't Do Enough

`claws init` creates directories but doesn't:
- Create policy (security)
- Create access control (who can use this)
- Enable audit logging
- Check/pull the Docker image
- Offer to create first agent

### 3. Create Doesn't Chain

After `create`, the user must separately:
1. Add auth
2. Add channels
3. Start

Each is a separate command requiring the instance name again. Should be chainable or offered as next-step prompts.

### 4. No Unified Status

To know if everything is okay, user must run:
- `claws list` (instances)
- `claws health` (health probes)
- `claws policy validate` (policy compliance)
- `claws audit` (security audit)

Should be one command: `claws status` that shows a unified dashboard.

### 5. Help is Overwhelming

The help output has **75 lines and 13 sections**. A new user seeing this for the first time is lost. Should be tiered:

```
claws help           → 5-line quickstart
claws help commands  → full command list
claws help setup     → onboarding guide
claws help security  → security guide
```

### 6. No "Getting Started" Flow in the Binary

The binary assumes you know what you're doing. There's no `clawctl` (no args) that says:

```
Welcome to claws — AI agent team manager.

Looks like this is your first time. Run:
  claws setup    — guided setup (recommended)
  claws init     — manual setup (advanced)
  claws help     — see all commands
```

---

## Approved Changes

### P0: Ship-Blocking for One-Click Vision

1. **`claws setup` — guided interactive onboarding**
   - Combines init + policy + access + create + auth + channel + start
   - Interactive prompts with safe defaults
   - Non-interactive mode for scripting
   - Single command from zero to working agent

2. **`claws init` should create policy + access + audit**
   - Policy with secure defaults
   - Access control with current user as admin
   - Audit logging enabled
   - No extra commands needed

3. **First-run detection**
   - `clawctl` with no args on uninitialized system → "Welcome" message + setup prompt
   - `clawctl` with no args on initialized system → brief status + help hint

### P1: Essential UX

4. **`claws create` should accept auth and channel inline**
   ```
   claws create alice --auth=codex --telegram=TOKEN
   ```
   Chains create + auth + channel add in one command. Start still separate (explicit).

5. **Unified status command**
   ```
   claws status
   ```
   Shows: instance health, policy compliance, recent activity, warnings — one screen.

6. **Tiered help**
   - `clawctl` (no args) → quickstart
   - `claws help` → concise command list
   - `claws help <topic>` → detailed guide
   - `claws <cmd> --help` → command-specific (already works)

### P2: Nice to Have

7. **`claws team create <name>` shortcut**
   - Creates group + shared resources in one step
   - Equivalent to `group create` + `group shared --all`

8. **Default tool profile on create**
   - New instances get `tools.profile = "coding"` unless overridden
   - Reduces audit warnings

9. **Binary distribution**
   - GitHub Releases with pre-built binaries
   - `curl -sL https://get.clawctl.dev | sh` installer script
   - Homebrew tap for macOS

---

## Implementation: `claws setup`

This is the centerpiece. Here's what it does:

```
$ claws setup

  Welcome to claws — AI agent team manager.

  This will set up your server to run a team of AI agents.
  Everything is stored locally. Agents connect to messaging
  apps (Telegram, WhatsApp, Discord, etc.) so people can
  message them.

  [1/6] Checking prerequisites...
    ✓ Docker running (v29.3.0)
    ✓ Docker Compose v2
    ✓ OpenClaw image found
    ✓ 74 GB free disk

  [2/6] Creating workspace...
    ✓ /home/ubuntu/.openclaw
    ✓ Security policy (loopback-only, DM pairing required)
    ✓ Access control (you are admin)
    ✓ Audit logging enabled

  [3/6] What's your team name? [my-team]: research

  [4/6] Create your first agent.
    Name: [agent-1]: sarah
    Auth method:
      1. Codex (OAuth — recommended)
      2. API key (Anthropic, OpenAI, etc.)
      Choice [1]: 1
    Starting OAuth flow... (opens browser)
    ✓ Auth complete.

  [5/6] Connect a channel?
      1. Telegram (need bot token from @BotFather)
      2. WhatsApp (need phone + QR scan)
      3. Discord (need bot token from developer portal)
      4. Skip for now
      Choice [4]: 1
    Telegram bot token: ****
    ✓ Telegram configured.

  [6/6] Starting sarah...
    ✓ Healthy on :18789
    ✓ Telegram connected

  Your agent is live!
    Message @sarah_bot on Telegram to test.
    Approve the pairing code: claws approve research/sarah telegram <CODE>

  Add another agent? [y/N]: n

  Done. Your team:
    research/sarah  :18789  healthy  telegram

  Next steps:
    claws list              — see all agents
    claws dashboard         — live status view
    claws audit             — security check
    claws setup             — add more agents
```

---

## Priority & Effort

| # | Change | Priority | Effort |
|---|--------|----------|--------|
| 1 | `claws setup` interactive onboarding | P0 | Large (4-6hr) |
| 2 | `init` creates policy + access + audit | P0 | Small (30min) |
| 3 | First-run detection | P0 | Small (30min) |
| 4 | Inline auth + channel on create | P1 | Medium (2hr) |
| 5 | Unified `claws status` | P1 | Medium (1hr) |
| 6 | Tiered help | P1 | Medium (1hr) |
| 7 | `team create` shortcut | P2 | Small (30min) |
| 8 | Default tool profile | P2 | Small (15min) |
| 9 | Binary distribution | P2 | Medium (2hr) |

---

## Success Metrics

1. **Time from zero to first message received:** Currently ~15min with 15+ commands. Target: ~5min with 1 command.
2. **Security audit score on fresh setup:** Currently 0 passed (nothing configured). Target: 41+ passed, 0 failures.
3. **Commands to create a team of 3 agents:** Currently ~25. Target: 1 (`setup`) or 3 (`setup` + 2 × "add another").
4. **User can explain what they have:** Can answer "how many agents, what channels, what's the security posture" with one command.
