# PMM Audit: clawctl — One-Click Installer Vision

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
3. `go build -o clawctl .`
4. Have Docker installed and running
5. Have the OpenClaw image built locally

**No binary distribution. No `curl | sh` installer. No package manager.**

### Step 2: Init
```bash
clawctl init
```
This works. Creates dirs, checks Docker, copies compose template. **This is good.**

But then:
- No policy is created (insecure defaults)
- No access control is created
- No guidance on what to do next beyond "create an instance"

### Step 3: Create First Agent
```bash
clawctl create alice
```
This works. But:
- No auth is configured — the agent can't talk to any AI model
- No channels — the agent can't receive messages
- No personality — AGENTS.md, SOUL.md, IDENTITY.md are empty
- The user has to know that auth comes next, then channels, then start

### Step 4: Auth
```bash
clawctl auth alice codex
```
Requires interactive TTY for OAuth. Breaks when run via `clawctl exec`.
Or:
```bash
clawctl auth alice apikey anthropic sk-...
```
The onboarding prompt blocks on a security warning that defaults to "No" in non-TTY mode.

### Step 5: Connect Channels
```bash
clawctl channel add alice telegram --token=TOKEN
```
This works now (we built it). But:
- User must already have a bot token
- No guidance on how to get one
- No "which channels do you want?" prompt

### Step 6: Start
```bash
clawctl start alice
```
Works. Health checks. Waits.

### Step 7: Approve Pairing
```bash
clawctl approve alice telegram CODE
```
User must message the bot, see the code, come back to terminal. Works but two-context switching.

### Step 8: Create a Team
```bash
clawctl group create team
clawctl create team/bob
clawctl auth team/bob ...
clawctl channel add team/bob ...
clawctl start team/bob
clawctl approve team/bob ...
clawctl group shared team --all
```
Repeat steps 3-7 for each agent. **7 commands per agent.**

### Step 9: Security
```bash
clawctl policy init
clawctl policy enforce --restart
clawctl access init
```
Most users won't know to do this. Security is opt-in, not default.

**Total: 15+ manual commands, 5+ context switches, multiple pieces of external knowledge required.**

---

## Object Model — What Users Need to Understand

Currently the user must grasp these concepts:

| Object | What It Is | Discovered Via |
|--------|-----------|----------------|
| Instance | A running agent container | `clawctl list` |
| Group | A collection of instances with shared resources | `clawctl group list` |
| Role | Manager or worker within a group | `--role=` flag on create |
| Runtime | Which agent software to run | `clawctl runtime list` |
| Channel | A messaging platform connection | `clawctl channel status` |
| Policy | Admin security constraints | `clawctl policy show` |
| Access | Who can run what commands | `clawctl access show` |
| Task | A unit of work in the manager/worker queue | `clawctl task list` |

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
clawctl setup
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
clawctl setup --non-interactive \
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
clawctl setup --migrate
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

`clawctl init` should automatically:
- Create `policy.json` with secure defaults
- Create `.access.json` with current user as admin
- Enable audit logging
- Print what was configured and why

`clawctl create` should automatically:
- Set `tools.profile = "coding"` in the config
- Warn if sandbox is not enabled (not block, just warn)

---

## UX Problems by Category

### 1. Too Many Commands for Common Operations

| What User Wants | Commands Required | Should Be |
|----------------|-------------------|-----------|
| "Set up a new server" | `init` + `policy init` + `access init` | `clawctl setup` |
| "Create an agent with Telegram" | `create` + `auth` + `channel add` + `start` | `clawctl create alice --auth=codex --telegram=TOKEN` then `start` |
| "Create a team" | `group create` + N × (`create` + `auth` + `channel add` + `start`) | `clawctl setup` or `clawctl team create` |
| "Check everything is okay" | `health` + `audit` + `policy validate` | `clawctl status` (unified) |

### 2. Init Doesn't Do Enough

`clawctl init` creates directories but doesn't:
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
- `clawctl list` (instances)
- `clawctl health` (health probes)
- `clawctl policy validate` (policy compliance)
- `clawctl audit` (security audit)

Should be one command: `clawctl status` that shows a unified dashboard.

### 5. Help is Overwhelming

The help output has **75 lines and 13 sections**. A new user seeing this for the first time is lost. Should be tiered:

```
clawctl help           → 5-line quickstart
clawctl help commands  → full command list
clawctl help setup     → onboarding guide
clawctl help security  → security guide
```

### 6. No "Getting Started" Flow in the Binary

The binary assumes you know what you're doing. There's no `clawctl` (no args) that says:

```
Welcome to clawctl — AI agent team manager.

Looks like this is your first time. Run:
  clawctl setup    — guided setup (recommended)
  clawctl init     — manual setup (advanced)
  clawctl help     — see all commands
```

---

## Approved Changes

### P0: Ship-Blocking for One-Click Vision

1. **`clawctl setup` — guided interactive onboarding**
   - Combines init + policy + access + create + auth + channel + start
   - Interactive prompts with safe defaults
   - Non-interactive mode for scripting
   - Single command from zero to working agent

2. **`clawctl init` should create policy + access + audit**
   - Policy with secure defaults
   - Access control with current user as admin
   - Audit logging enabled
   - No extra commands needed

3. **First-run detection**
   - `clawctl` with no args on uninitialized system → "Welcome" message + setup prompt
   - `clawctl` with no args on initialized system → brief status + help hint

### P1: Essential UX

4. **`clawctl create` should accept auth and channel inline**
   ```
   clawctl create alice --auth=codex --telegram=TOKEN
   ```
   Chains create + auth + channel add in one command. Start still separate (explicit).

5. **Unified status command**
   ```
   clawctl status
   ```
   Shows: instance health, policy compliance, recent activity, warnings — one screen.

6. **Tiered help**
   - `clawctl` (no args) → quickstart
   - `clawctl help` → concise command list
   - `clawctl help <topic>` → detailed guide
   - `clawctl <cmd> --help` → command-specific (already works)

### P2: Nice to Have

7. **`clawctl team create <name>` shortcut**
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

## Implementation: `clawctl setup`

This is the centerpiece. Here's what it does:

```
$ clawctl setup

  Welcome to clawctl — AI agent team manager.

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
    Approve the pairing code: clawctl approve research/sarah telegram <CODE>

  Add another agent? [y/N]: n

  Done. Your team:
    research/sarah  :18789  healthy  telegram

  Next steps:
    clawctl list              — see all agents
    clawctl dashboard         — live status view
    clawctl audit             — security check
    clawctl setup             — add more agents
```

---

## Priority & Effort

| # | Change | Priority | Effort |
|---|--------|----------|--------|
| 1 | `clawctl setup` interactive onboarding | P0 | Large (4-6hr) |
| 2 | `init` creates policy + access + audit | P0 | Small (30min) |
| 3 | First-run detection | P0 | Small (30min) |
| 4 | Inline auth + channel on create | P1 | Medium (2hr) |
| 5 | Unified `clawctl status` | P1 | Medium (1hr) |
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
