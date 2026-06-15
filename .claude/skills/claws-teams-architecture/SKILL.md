---
name: claws-teams-architecture
description: Conceptual reference for how a claws **team** is structured on disk, how the team-shared directory works (skills / hooks / workspace / tasks / output), how manager and worker agents coordinate through a filesystem task queue with atomic `rename()` transitions, and how to **design** a multi-agent team for a given use case (solo, coordinator+workers, peer pool, pipeline). Use whenever the user is *thinking about* the shape of a team — "how do teams work in claws", "explain the claws team hierarchy", "what is the shared folder / what does ~/.openclaw/<team>/shared/ contain", "how do agents in a team coordinate", "what does manager/worker mean in claws", "how does the task queue work", "shared hooks / shared skills / shared workspace", "should I use one agent or a team", or "design a multi-agent team that does X" / "I want a team that does X". Read this BEFORE designing a team so the role assignments, shared resources, and channel ownership decisions made at create-time are correct. Distinct from claws-add-agent (which mechanically adds A single agent into an already-designed team), claws-bootstrap-fresh-box (fresh-host install), claws-debug-agent (fixing a broken individual agent), and claws-release (cutting a release).
---

# claws — team architecture and design

This skill is the **conceptual** companion to `claws-add-agent`. `claws-add-agent` is the mechanical playbook for "create + auth + channel + start + ping" on a single agent. **This** skill is for the step before: deciding what the team should look like, which agents own which channels, what the shared directory should contain, and how work flows between manager and workers.

Read this when the user is designing a team. Hand off to `claws-add-agent` once the shape is decided and it's time to create agents.

## Mental model — a team is a directory

Everything about a team lives under one directory:

```
~/.openclaw/<team>/
├── .group.json              ← marks this dir as a team (group config)
├── defaults.json            ← team-wide defaults (merged into each agent)
├── shared/                  ← everything every team agent can see
│   ├── skills/              ← RO  → /home/node/.openclaw/bundled-skills
│   ├── hooks/               ← RO  → /home/node/.openclaw/shared-hooks
│   ├── workspace/           ← RW  → /home/node/.openclaw/shared
│   ├── tasks/               ← RW  → /home/node/.openclaw/tasks
│   │   ├── pending/         ← manager writes, workers claim
│   │   ├── claimed/         ← currently being worked on
│   │   └── done/            ← workers write result, manager picks up
│   └── output/              ← RW  → /home/node/.openclaw/output
└── <agent>/                 ← one subdir per team member
    ├── instance.env         ← port, gateway token, INSTANCE_ROLE, INSTANCE_MANAGER, model auth refs
    ├── openclaw.json        ← agent config (channels, model defaults, hooks, skills)
    ├── docker-compose.override.yml  ← auto-rebuilt — shared mounts + role mounts
    ├── workspace/           ← this agent's own working dir
    └── credentials/         ← channel + model credential files (mode 0600)
```

To back up a team, back up `~/.openclaw/<team>/`. To delete a team, `claws group remove <team> --purge`. There's no team state anywhere else.

The container-side mount points (right column above) come from `runtime.go` and are stable across runtimes — agent code reads from `/home/node/.openclaw/tasks` regardless of where on the host the team lives.

## Manager / worker, in one diagram

```
        ┌─────────────────┐         channel (Telegram/Discord/…)
        │  team/lead      │  ◄────────────────────────────────── user
        │  role=manager   │
        └────────┬────────┘
                 │ writes JSON to
                 ▼
       shared/tasks/pending/<id>.json
                 │
                 │ workers poll, then atomically:
                 │     rename pending/<id>.json → claimed/<id>.json
                 ▼
        ┌─────────────────┐   ┌─────────────────┐
        │  team/dev1      │   │  team/dev2      │
        │  role=worker    │   │  role=worker    │
        │  manager=lead   │   │  manager=lead   │
        └────────┬────────┘   └────────┬────────┘
                 │ does work, writes result, then:
                 │     rename claimed/<id>.json → done/<id>.json
                 ▼
       shared/tasks/done/<id>.json
                 │
                 ▼
        manager polls done/, picks up result,
        replies to user on the original channel
```

Key invariants:

- **The manager owns the channel.** Workers don't usually have channels of their own — they respond *through* the manager. (Exception: a worker can have its own channel if you want both inbound paths, but that's unusual.)
- **State transitions are filesystem `rename()`.** On a local POSIX filesystem this is atomic — two workers can't both claim the same task. See "FUSE warning" below.
- **`INSTANCE_ROLE` + `INSTANCE_MANAGER` live in `instance.env`** and drive `rebuildGroupOverride()` in `group.go`. Changing them requires `claws restart <agent>` to re-mount.
- **Workers get a read-only view of the manager's workspace**; managers get a read-only view of each worker's workspace. The runtime stitches these mounts in automatically via `rebuildGroupOverride()`.

## The shared directory — who can write what

Container-side paths in **bold**.

| Host dir | Container path | Mode | Who writes | Use it for |
|---|---|---|---|---|
| `shared/skills/<name>/SKILL.md` | **/home/node/.openclaw/bundled-skills** | RO for all agents | operator (host) | Skills every team agent should have without duplicating — drop a `<name>/SKILL.md` once, every container sees it |
| `shared/hooks/*.sh` | **/home/node/.openclaw/shared-hooks** | RO for all agents | operator (host) | Bash hooks that fire on agent lifecycle events (pre/post message, audit logging) — write once, every agent fires them |
| `shared/workspace/` | **/home/node/.openclaw/shared** | RW for all agents | any team agent | Scratch space all team agents share — handoffs that aren't structured tasks (large blobs, intermediate artifacts) |
| `shared/tasks/` | **/home/node/.openclaw/tasks** | RW for manager, RO for workers (workers still rename across subdirs) | manager writes pending; workers claim/complete | The structured work queue — see "Task lifecycle" |
| `shared/output/` | **/home/node/.openclaw/output** | RW for managers and workers | any team agent | Final results destined for outside the team (audit logs, files to ship to the user, results the manager needs) |

The override file (`docker-compose.override.yml`) is **auto-generated** by `rebuildGroupOverride()` every time a role changes or a `claws group shared` flag is toggled. Never hand-edit it — it'll be clobbered on the next role/share change.

## Task lifecycle on disk

```
shared/tasks/
├── pending/<id>.json    ← manager writes here
├── claimed/<id>.json    ← worker `rename`'d it here; populated `claimed_by` + `claimed_at`
├── done/<id>.json       ← worker `rename`'d it here; populated `result` + `completed_at`
└── archive/             ← optional cold storage; nothing in claws auto-moves here
```

A task JSON (from `task.go`):

```json
{
  "id": "<16-hex>",
  "title": "...",
  "description": "...",
  "created_by": "lead",
  "created_at": "RFC3339",
  "status": "pending|claimed|done",
  "claimed_by": "dev1",
  "claimed_at": "RFC3339",
  "completed_at": "RFC3339",
  "result": "..."
}
```

`claws task` exposes the lifecycle for humans and tests:

```bash
claws task create <team> "<title>" --from=<agent> [--description=<text>]
claws task list   <team> [--status=pending|claimed|done]
claws task claim  <team> <id> --by=<agent>
claws task complete <team> <id> [--result=...]
claws task status <team> <id>
```

In normal operation the **manager and workers do the rename themselves** inside the container — these CLI verbs are mostly for manually seeding the queue (e.g. testing without a channel), inspecting state, or unsticking a stuck task.

### FUSE warning — read this before designing anything with shared state

`shared/tasks/` **only works on a real POSIX filesystem.** The state transitions are `os.Rename()`, which is atomic on local disk but is silently non-atomic (or rejected entirely) on FUSE mounts — including the `claws storage mount` S3 adapter. `task.go` ships an `isFuseMount()` guard that detects `fuse.s3fs` / `fuse.mountpoint-s3` and refuses to operate.

**Practical rule:** if the user is mounting S3 (or any FUSE filesystem) into `~/.openclaw/`, keep at least `~/.openclaw/<team>/shared/tasks/` on local disk. The rest of the team dir can live anywhere, but the queue cannot.

## CLI surface (what to recommend)

### Designing / inspecting

```bash
claws team show <team>          # one-screen view: members, roles, channels, shared resources, queue depth
claws team list                 # all teams + member counts
claws group shared <team> --all # turn on shared skills / workspace / hooks for the whole team
```

`claws team show` is the canonical "what does this team look like right now" command — run it before making structural changes.

### Creating the structure

```bash
claws team create <team>                                # creates dir, .group.json, shared/skills, shared/workspace, shared/hooks, shared/tasks
claws create <team>/lead --role=manager                 # manager — owns channel, dispatches tasks
claws create <team>/dev1 --role=worker --manager=lead   # worker — claims from queue, reports to lead
claws group role <team>/<agent> manager|worker|none [--manager=<name>]   # change role after creation
```

`--role` and `--manager` land in `instance.env` as `INSTANCE_ROLE` / `INSTANCE_MANAGER` and drive the override regeneration in `rebuildGroupOverride()`. **Restart the agent (`claws restart <team>/<agent>`) after any role change** so the new mounts take effect.

### Driving / operating

```bash
claws start <team>            # fan-out: start every member
claws stop <team>             # fan-out: stop every member
claws restart <team>          # fan-out
claws status <team>           # per-member status
claws health <team>           # per-member health
claws upgrade <team> --image=...  # roll the whole team's runtime
claws team rotate-tokens <team>   # rotate gateway tokens across the team
claws team apply-config <team> <key> <value>   # set the same config key on every member
claws team apply-policy <team>    # enforce admin policy team-wide
```

Most team-noun verbs are thin wrappers that inject `--group=<team>` and delegate to the per-instance handler (`cmdTeamDelegate` in `group.go`). Pass-through flags (`--yes`, `--hard`, `--json`) reach the underlying handler unchanged.

### Manually exercising the queue (testing / unsticking)

```bash
claws task create <team> "review PR #42" --from=lead
claws task list  <team>
claws task claim <team> <id-prefix> --by=dev1
claws task complete <team> <id-prefix> --result="LGTM"
claws task status <team> <id-prefix>
```

Useful for: smoke-testing a new team end-to-end before plugging in the channel; manually retrying a task a worker dropped; seeding fake tasks during dev.

## Design patterns — pick the right team shape

### Solo agent — no team needed

User has one bot, one channel, one purpose ("a Telegram bot that answers questions about my notes").

```bash
claws create personal/sarah          # ungrouped or single-member group, doesn't matter
```

Don't create roles. Don't create shared/tasks. A team with one agent costs you complexity for nothing.

**Use when:** one channel, one persona, no delegation, no parallelism.

### Coordinator + workers — the canonical team

One manager owns the channel and decomposes work; specialized workers execute. The example in `README.md`:

```bash
claws team create research
claws create research/lead --role=manager
claws create research/dev1 --role=worker --manager=lead   # e.g. web search
claws create research/dev2 --role=worker --manager=lead   # e.g. paper summarization
claws create research/dev3 --role=worker --manager=lead   # e.g. code analysis
claws auth fleet codex --missing-only                     # per-agent OAuth — see below
claws channel add research/lead telegram --token=<lead-bot-token>
claws start research
```

**Use when:** one inbound channel, work that benefits from specialization or parallelism, the user wants a single conversational interface.

**Channel ownership:** only the manager. Workers don't have channels — they reply through the manager's response on the queue.

### Peer pool — one bot per service

Several independent agents, no hierarchy. Each agent has its own channel and its own purpose. They share `shared/skills/` and `shared/hooks/` for code reuse but **don't share `tasks/`**.

```bash
claws team create bots
claws create bots/support  # Telegram support bot
claws create bots/notify   # Slack notification bot
claws create bots/onboard  # Discord onboarding bot
# no --role flags — each owns its own channel
```

**Use when:** independent bots that should share *configuration* (skills, hooks, audit hooks) but not *work* (no delegation between them).

### Pipeline — A produces, B consumes

Stage agents that hand off through `shared/workspace/` (or `shared/output/`), often kicked by cron rather than chat.

```bash
claws team create pipeline
claws create pipeline/scraper   # cron: writes to shared/workspace/raw/
claws create pipeline/cleaner   # cron: reads raw/, writes clean/
claws create pipeline/reporter  # cron: reads clean/, writes shared/output/
```

**Use when:** scheduled work, no chat interface, structured intermediate artifacts.

(Optional: layer the manager/worker pattern on top if a chat agent should kick the pipeline on demand.)

## Critical context — do not skip

### 1. Per-agent OAuth, even within a team

OAuth refresh tokens are single-use. Two agents on the same upstream ChatGPT account that share a grant will both die with `refresh_token_reused` the first time one refreshes. **Every agent in the team needs its own grant**, even when they share an account:

```bash
claws auth fleet codex --missing-only      # OAuth each agent independently
```

API keys (`apikey openai|anthropic|openrouter`) have no collision risk and can be safely shared across the team.

### 2. Channel ownership lives on the manager

Don't add channels to workers unless you actually want a second inbound surface. The manager fronts the channel, decomposes requests, dispatches tasks, then replies to the user on the original channel with the synthesized result.

### 3. Shared/tasks must be on local disk

See FUSE warning above. If the user is using `claws storage mount` (S3 FUSE), the team dir can live on S3 **except for `shared/tasks/`** — that subtree must be on a real filesystem or the queue silently breaks.

### 4. Backup = back up the team directory

Everything for the team — including the queue state, shared workspace, per-agent credentials, per-agent containers' bind mounts — is under `~/.openclaw/<team>/`. There is no external state to back up. Per-agent `claws backup <name>` is the structured form; tarring `~/.openclaw/<team>/` is the brute-force form.

### 5. `claws team show <team>` first

Before designing changes, run `claws team show <team>` to see what's actually there: members, roles, channels per member, shared resource presence (skills / workspace / hooks each show `yes` or `no`), and live queue depth (`pending` / `claimed` / `done` counts).

## Distinguish from sibling skills

- **`claws-bootstrap-fresh-box`** — fresh-host install of claws itself. Use that one if claws isn't installed yet. Use *this* skill once they're past install and want to lay out a team.
- **`claws-add-agent`** — the mechanical "create + auth + channel + start + ping" loop for a single agent (with optional `--role` / `--manager` flags). Use that one once *this* skill has answered "what should the team look like".
- **`claws-debug-agent`** — when a team member is broken (silent on channel, 401s, restarting). Doesn't help you design a team.
- **`claws-release`** — cutting a claws release. Unrelated.

## Decision shortcuts

- User said "should I use one agent or a team?" → if there's one channel, one persona, no delegation: **solo**. Otherwise consider coordinator + workers.
- User said "design a team that does X" → walk the four patterns above, pick the one that matches X, name the manager + workers, list the channels each one owns, then hand off to `claws-add-agent` for the create loop.
- User said "what is the shared folder?" → walk the table in "The shared directory — who can write what", focusing on RO vs RW and what each is for.
- User said "how do agents in a team coordinate?" → walk the manager/worker diagram and the task lifecycle on disk. Mention atomic `rename()` and the FUSE caveat.
- User said "my queue isn't working" → first check `claws team show <team>` for queue depth, then check whether `shared/tasks/` is on a FUSE mount (`mount | grep openclaw`). If FUSE, that's the bug — move the queue to local disk.
- User wants every team agent to fire the same hook → drop it in `shared/hooks/<name>.sh`, `claws group shared <team> --hooks`, restart members.
- User wants every team agent to have a skill → drop it in `shared/skills/<name>/SKILL.md`, `claws group shared <team> --skills`, restart members.

## What success looks like

The user finishes this skill able to answer, for their use case:

1. **One agent or a team?** (solo vs one of the three team shapes)
2. **Which agent owns the channel?** (typically the manager)
3. **What goes in `shared/`?** (skills, hooks, workspace, tasks, output — which ones, and why)
4. **Where does the queue live?** (local disk, never FUSE)
5. **What's the next command?** (usually `claws team create <team>` followed by `claws-add-agent` per member)

If they can answer those five, they're ready to leave this skill and run the create loop.
