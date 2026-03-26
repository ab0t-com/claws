# clawctl Product Marketing & User Experience Review

**Date:** 2026-03-17
**Reviewer Perspective:** Product Marketing Manager — user understanding, market positioning, experience gaps
**Scope:** clawctl-go CLI as a multi-instance orchestration layer for OpenClaw

---

## Executive Summary

clawctl solves a real, emerging problem: **managing a fleet of personal AI agent instances on a single server**. The core lifecycle commands (create/start/stop/list/status/remove) form a tight, intuitive loop. The group and shared-resource system hints at a powerful coordination layer. However, the product has significant gaps in onboarding, discoverability, and the completeness of its most differentiated feature (manager/worker roles). This report identifies what's working, what's missing, and where to focus before putting this in front of users.

---

## 1. Value Proposition

### What clawctl is

A CLI that lets a single operator run **multiple isolated OpenClaw AI agent instances** on one machine, each with:
- Its own identity, credentials, and messaging channels
- Its own workspace, sessions, and configuration
- Its own Docker container with health monitoring
- Optional grouping with shared resources and role-based coordination

### Who it's for

- **Power users** running personal AI assistants across multiple messaging platforms
- **Small teams** wanting isolated agents per team member or per function (support agent, dev agent, research agent)
- **Homelab operators** who want everything on one box with SSH tunnel access
- **AI-first builders** experimenting with multi-agent coordination patterns

### Competitive positioning

This is not Kubernetes. The competitive frame is closer to:
- **multipass** (Canonical) — multiple Ubuntu VMs, simple CLI
- **lxc/lxd** — container management for humans
- **docker compose profiles** — but with first-class instance identity

The messaging should lean into: **"Your agents, your server, your rules."**

---

## 2. User Journey Analysis

### Happy Path (Strong)

```
clawctl create alice
  -> Prints port, directory, next steps
clawctl auth alice codex
  -> OAuth flow
clawctl start alice
  -> Health-checked startup with wait loop
clawctl list
  -> Table with NAME, PORT, STATUS, RAM, UPTIME
clawctl tunnel
  -> Prints ready-to-paste SSH command
```

This is clean. The "next steps" hints after `create` are a strong UX touch — they eliminate the "what now?" moment. The tunnel command generating a copy-pasteable SSH line is excellent.

### First-Run Path (Broken)

There is no first-run experience. A brand-new user must:

1. Know that OpenClaw exists and have its Docker image built or pulled
2. Know that `~/.openclaw` (or `OPENCLAW_ROOT`) is the root directory
3. Know that `docker-compose.yml` must be in a specific location
4. Trust that port 18789+ won't conflict with existing services

**There is no `clawctl init`, no `clawctl doctor`, no `clawctl version`.** The first command a new user runs will be `clawctl create foo` and it will fail with an opaque Docker error if prerequisites aren't met.

**Recommendation:** Add `clawctl init` (creates root dir, validates Docker, pulls image) and `clawctl doctor` (checks Docker, image, disk, ports, AWS CLI, rclone).

### Discovery Path (Weak)

The `--help` output is well-organized into sections (Lifecycle, Info, Auth & Channels, etc.) but:

- No man page, no `--help` per subcommand (e.g., `clawctl create --help`)
- No way to discover what `--from=` templates are available
- No way to see what a group contains without `clawctl group list` + `clawctl list`
- The manager/worker role system is listed in help but has no explanation of what it does

### Advanced Path (Incomplete)

Groups and roles are the most differentiated feature. But:

- Creating a worker sets up volume mounts for a task queue... that has no CLI
- There's no `clawctl task dispatch`, `clawctl task list`, or `clawctl task claim`
- The shared workspace and skills mounts work, but there's no way to inspect what's shared
- A user who creates `backend/manager` and `backend/worker-1` gets directories and nothing else

---

## 3. Feature-by-Feature UX Assessment

| Feature | UX Quality | Notes |
|---------|-----------|-------|
| **create / start / stop** | Strong | Clean output, health-check wait, rollback on failure |
| **list** | Strong | Color-coded status, RAM, uptime — scannable |
| **status** | Good | Shows key details, token truncation is a nice touch |
| **health** | Strong | Live + ready probes, failing subsystem detail |
| **dashboard** | Good | Live refresh, but no keyboard shortcuts (q to quit, etc.) |
| **backup / restore** | Good | Handles name remapping, port re-registration |
| **auth** | Good | Codex OAuth + API key paths both work |
| **channel** | Adequate | Thin wrapper — delegates to OpenClaw CLI |
| **tunnel** | Excellent | Generates multi-port SSH command for all instances |
| **groups** | Adequate | Create/list/add/remove work, but shared config is opaque |
| **share / unshare** | Adequate | Works but requires restart; no way to inspect current state |
| **storage** | Good | Setup flow is well-guided; status command is thorough |
| **proxy** | Risky | Overwrites system Caddyfile without backup or confirmation |
| **activity** | Weak | Timestamps are wrong (all show current time, not event time) |
| **manager/worker roles** | Incomplete | Directory scaffolding only — no task lifecycle |
| **stats** | Minimal | Raw `docker stats` passthrough |
| **migrate** | Good | Careful stop-copy-repoint-start flow |

---

## 4. Missing Commands

| Command | Purpose | Priority |
|---------|---------|----------|
| `clawctl init` | First-run setup: create root dir, validate Docker, pull image | P0 |
| `clawctl doctor` | Diagnose: Docker, image, ports, disk, tools | P0 |
| `clawctl version` | Show clawctl version, Go version, Docker version, image tag | P0 |
| `clawctl create --help` | Per-command help with examples | P1 |
| `clawctl config show <name>` | Show merged config for an instance | P1 |
| `clawctl config edit <name>` | Open config in $EDITOR | P2 |
| `clawctl template list` | List instances that can be used with `--from=` | P2 |
| `clawctl task dispatch` | Send task from manager to worker queue | P1 (if roles ship) |
| `clawctl task list` | Show pending/claimed/done tasks | P1 (if roles ship) |
| `clawctl upgrade <name>` | Pull new image and restart | P2 |

---

## 5. Naming & Terminology

### Strengths
- "Instance" is the right abstraction — not "container" (too low), not "agent" (too loaded)
- "Group" is intuitive for logical grouping
- The `group/instance` reference format mirrors filesystem paths — familiar

### Concerns
- **"clawctl" vs "openclaw"** — the naming relationship is unclear. Is clawctl part of OpenClaw? A companion tool? The `openclaw-` prefix on Docker projects and compose files suggests tight coupling, but the CLI name suggests independence.
- **"Manager" and "Worker"** — these terms imply an active coordination protocol that doesn't exist yet. Consider "coordinator" and "member" until the task dispatch system is built.
- **"Shared"** — overloaded. `--shared-skills`, `--shared-workspace`, and `clawctl group shared` all mean different things (instance-level vs group-level sharing).

---

## 6. Capacity & Limits

The 8-instance hard cap (`maxInstances`) and 6-instance warning threshold (`warnInstances`) are set in code but:

- Not documented in `--help`
- Not shown in `clawctl list` or `clawctl status`
- The RAM model (230MB base per instance on Node 22) is implicit
- No `clawctl capacity` or similar command to show headroom

**Recommendation:** Add instance count and estimated RAM to the dashboard. Surface the cap in the error message with guidance: "Maximum 8 instances reached. Each instance uses ~230MB RAM. Current server: 4GB total."

---

## 7. Market Readiness Assessment

| Criterion | Ready? | Gap |
|-----------|--------|-----|
| Core lifecycle (create/start/stop/remove) | Yes | — |
| Onboarding / first-run | No | No init, no doctor, no version |
| Documentation | No | No README, no man pages, no per-command help |
| Error messages | Partial | Some are helpful, some are raw Docker errors |
| Multi-instance management | Yes | list, dashboard, health, start-all/stop-all |
| Backup & recovery | Yes | backup/restore with re-registration |
| Grouping | Partial | Works but manager/worker is incomplete |
| Storage (S3) | Partial | Setup is good but sync is destructive by default |
| Proxy | No | Overwrites system config, no auth layer |
| Structured output (JSON) | No | Blocks CI/CD and scripting use cases |

### Ship/No-Ship Recommendation

**Ship the core lifecycle + groups (without roles) + backup + storage setup.** Hold back manager/worker roles, proxy setup, and the activity command until they're complete.

---

## 8. Key Recommendations

1. **Add `clawctl init` + `clawctl doctor` + `clawctl version`** — these are table stakes for any CLI tool
2. **Write a README.md** with a 5-minute quickstart: init, create, auth, start, tunnel
3. **Scope down roles** — remove `--role=manager|worker` from `create` until the task dispatch system exists, or clearly label it as experimental
4. **Add `--json` output** to `list`, `status`, `health` — enables scripting and CI/CD
5. **Fix the activity command** — wrong timestamps undermine trust in observability
6. **Change `rclone sync` to `rclone copy`** — destructive default is a data-loss risk
7. **Add confirmation prompts** to `remove --purge`, `group remove --purge`, and `proxy setup`

---

*Report generated from source review of clawctl-go @ commit cd0e260 on branch master.*
