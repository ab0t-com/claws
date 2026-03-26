# clawctl × intent-gateway — Structural Analysis & Integration Plan

## STEP 1: Entity Definition

### clawctl Entities

| Entity | State/Lifecycle | Description |
|---|---|---|
| **Instance** | created → started → healthy → stopped → removed | A single OpenClaw gateway + workspace |
| **Group** | created → populated → removed | Collection of instances sharing a namespace |
| **PortRegistry** | file, append/remove | Maps index → instance name, allocates ports |
| **InstanceEnv** | file, mutable | Per-instance config (port, token, paths, flags) |
| **ComposeOverride** | file, regenerated | Volume mounts for sharing + roles |
| **SharedDir** | directory | Global or group-level shared resources |
| **StorageConfig** | file, mutable | S3 bucket/region/mount state |
| **Caddyfile** | file, regenerated | Reverse proxy routing rules |
| **ManagerRole** | flag on instance | Can read worker workspaces, write task queue |
| **WorkerRole** | flag on instance | Reads task queue, writes output |
| **TaskQueue** | directory (pending/claimed/done) | File-based job queue between manager and workers |

### intent-gateway (sarahctl) Entities

| Entity | State/Lifecycle | Description |
|---|---|---|
| **ApprovalGrant** | issued → active → expired | Time-bound permission: actor + tools + paths + window |
| **ApprovalRequest** | received → evaluated → allowed/denied | Runtime check against grants |
| **ComplianceLedger** | append-only JSONL, hash-chained | Immutable record of all messages and decisions |
| **MessageRecord** | ingested → normalized → hash-chained | Inbound message with intent classification |
| **DecisionRecord** | logged → hash-chained | Approval check result with audit trail |
| **TransportEvent** | received → deduped → ingested | Inbound event from any channel |
| **TransportRegistry** | file, marks seen events | Deduplication by event_id |
| **HookEvent** | emitted → logged (JSONL) | Structured event for automation |
| **Schedule** | added → active → runs on cron | Cron-triggered callback |
| **Callback** | configured → fired | Named action (stdout, file, webhook) |
| **Worker** | disabled → enabled → running → polling | Processes inbox events via adapters |

### Canonicalization (Shared Concepts)

| clawctl concept | intent-gateway concept | Same thing? |
|---|---|---|
| Task queue (pending/claimed/done) | Worker inbox + transport events | **Related.** clawctl's task queue is a directory convention. The intent-gateway's worker+transport is a processing pipeline. They solve the same problem at different levels. |
| Health check (/healthz + /readyz) | Worker status (healthy/enabled) | **Complementary.** clawctl checks the container. intent-gateway checks the processing pipeline. |
| Activity feed (file scans) | Hook events (JSONL stream) | **Complementary.** clawctl observes from outside. intent-gateway logs from inside. |
| Shared workspace | Compliance log directory | **Overlapping.** Both write to shared directories. Compliance logs could live in shared workspace. |
| Instance role (manager/worker) | Approval grants (actor/scope/window) | **Different but composable.** clawctl controls who sees what files. intent-gateway controls who can do what actions. |

---

## STEP 2: Structural Graph

```
┌─────────────────────────────────────────────────────────────────────┐
│                          OPERATOR (human)                           │
└──────────┬──────────────────────────────────────────┬───────────────┘
           │                                          │
           │ [Explicit, High]                         │ [Explicit, High]
           ▼                                          ▼
┌─────────────────────┐                   ┌─────────────────────────┐
│      clawctl         │                   │   OpenClaw Gateway       │
│  (instance manager)  │                   │   (AI agent runtime)     │
└──────────┬──────────┘                   └───────────┬─────────────┘
           │                                          │
     ┌─────┼──────────────────┐              ┌────────┼────────┐
     │     │                  │              │        │        │
     ▼     ▼                  ▼              ▼        ▼        ▼
┌────────┐┌──────┐     ┌──────────┐    ┌────────┐┌──────┐┌────────┐
│Instance││Group │     │ Storage  │    │WhatsApp││Agent ││ Tools  │
│Lifecycle││Mgmt  │     │(S3/FUSE) │    │Channel ││Engine││Profile │
└────┬───┘└──┬───┘     └────┬─────┘    └───┬────┘└──┬───┘└───┬────┘
     │       │              │               │        │        │
     │       │              │               │ [Explicit, High]│
     │       │              │               ▼        │        │
     │       │              │         ┌──────────┐   │        │
     │       │              │         │ Inbound  │   │        │
     │       │              │         │ Message  │   │        │
     │       │              │         └────┬─────┘   │        │
     │       │              │              │         │        │
     │       │              │    [INTEGRATION POINT 1]│        │
     │       │              │              │         │        │
     │       │              │              ▼         ▼        │
     │       │              │    ┌──────────────────────┐     │
     │       │              │    │   intent-gateway      │     │
     │       │              │    │   (sarahctl)          │     │
     │       │              │    └──────────┬───────────┘     │
     │       │              │               │                 │
     │       │              │    ┌──────────┼──────────┐      │
     │       │              │    │          │          │      │
     │       │              │    ▼          ▼          ▼      │
     │       │              │ ┌────────┐┌──────┐┌─────────┐  │
     │       │              │ │Approval││Comply ││Transport│  │
     │       │              │ │Engine  ││Ledger ││Bridge   │  │
     │       │              │ └────┬───┘└──┬───┘└────┬────┘  │
     │       │              │      │       │         │       │
     │       │              │      │       │         │       │
     │    [INT. POINT 3]    │  [INT. POINT 2]   [INT. POINT 4]
     │       │              │      │       │         │       │
     ▼       ▼              ▼      ▼       ▼         ▼       ▼
┌──────────────────────────────────────────────────────────────────┐
│                     FILESYSTEM LAYER                              │
│                                                                  │
│  ~/.openclaw/<group>/<instance>/                                 │
│    ├── workspace/           ← agent files + intent-gateway repo  │
│    ├── openclaw.json        ← instance config                    │
│    └── credentials/         ← never shared                       │
│                                                                  │
│  ~/.openclaw/<group>/shared/                                     │
│    ├── tasks/               ← clawctl task queue                 │
│    │   ├── pending/                                              │
│    │   ├── claimed/                                              │
│    │   └── done/                                                 │
│    ├── workspace/           ← shared collaboration space         │
│    ├── compliance/          ← intent-gateway ledger [NEW]        │
│    │   ├── logs/                                                 │
│    │   └── registry/                                             │
│    └── hooks/               ← shared hook scripts                │
│                                                                  │
│  /mnt/s3/openclaw/          ← S3-FUSE mount (optional)          │
└──────────────────────────────────────────────────────────────────┘
```

### Link Table

| From | To | Type | Certainty | Evidence |
|---|---|---|---|---|
| Operator → clawctl | creates/manages instances | Explicit | High | CLI commands |
| Operator → OpenClaw Gateway | sends messages via WhatsApp | Explicit | High | Chat interface |
| clawctl → Instance (Docker) | compose up/down/restart | Explicit | High | `dc()` wrapper |
| clawctl → Port Registry | allocates/frees ports | Explicit | High | `registry.go` |
| clawctl → Compose Override | generates volume mounts | Explicit | High | `shared.go`, `group.go` |
| clawctl → S3 | backup/sync/mount | Explicit | High | `storage.go` |
| OpenClaw → Agent | runs AI model, processes messages | Explicit | High | OpenClaw source |
| Agent → Workspace | reads/writes files | Explicit | High | Tool profile (read/write/exec) |
| Agent → intent-gateway | runs `sarahctl` commands | **Inferred** | Med | The agent built this tool and runs it from workspace |
| intent-gateway → Compliance Ledger | appends hash-chained records | Explicit | High | `compliance.go` |
| intent-gateway → Approval Registry | stores/checks grants | Explicit | High | `approval.go` |
| intent-gateway → Transport Registry | deduplicates events | Explicit | High | `transport.go` |
| intent-gateway → Hook Log | emits JSONL events | Explicit | High | `hooks.go` |
| intent-gateway → Worker Inbox | processes queued events | Explicit | High | `worker.go` |
| clawctl task queue ↔ intent-gateway worker | **could** share the same directory | **Inferred** | Med | Both use file-based job queues |
| clawctl activity ↔ intent-gateway hooks | **could** read from same JSONL | **Inferred** | Med | Both produce event streams |
| clawctl health ↔ intent-gateway worker status | **could** aggregate | **Inferred** | Med | Both report health |
| Approval grants ↔ clawctl roles | **could** enforce who can do what | **Inferred** | Low | Different mechanisms today |

---

## STEP 3: Gap Analysis

### Orphan Nodes (concepts with no clear connection)

| Node | Why it's orphaned | Impact |
|---|---|---|
| **Approval engine** | clawctl has no concept of "permissions" between instances. Manager/worker is filesystem mounts, not approval-gated actions. | A manager could grant time-bound tool access to workers, but nothing enforces this today. |
| **Hash-chained compliance ledger** | clawctl has no audit trail for its own operations. `clawctl create`, `remove`, `start` leave no immutable log. | Operator actions are invisible to the compliance system. |
| **Transport bridge (webhook adapter)** | The intent-gateway can receive webhooks, but clawctl doesn't expose ports or routes for webhooks. | External systems can't push events to the intent-gateway without manual port forwarding. |
| **Scheduler** | The intent-gateway has cron scheduling, but clawctl doesn't know about it. Scheduled tasks run inside one agent's workspace. | No cross-instance scheduling. Manager can't schedule work for workers. |
| **Callback system** | The intent-gateway can fire named callbacks, but there's no way for one instance to fire a callback on another. | Inter-instance event-driven automation is missing. |

### Ambiguities

| Relationship | What's unclear |
|---|---|
| Who runs `sarahctl`? | The agent runs it from its workspace. But should clawctl also be able to invoke it? Or should the intent-gateway be a sidecar container? |
| Where do compliance logs live? | Currently in the agent's private workspace (`.compliance-staging/`). Should they be in shared workspace so the manager can audit them? |
| Is the intent-gateway per-instance or shared? | Sarah built it for herself. Should every instance get a copy? Or should one instance run it as a service for all? |
| How do approval grants propagate? | If a manager grants an approval, does every worker see it? Or only the ones the grant specifies? |

---

## STEP 4: Integration Points

### Integration 1: Transport Bridge → clawctl Proxy

**Problem:** The intent-gateway can receive webhook events, but there's no way to route external webhooks to a specific instance.

**Solution:** clawctl's Caddy proxy can route webhook paths to the right instance:

```
https://claw.example.com/backend/sarah/webhook → :18789/webhook
```

The intent-gateway registers a webhook handler at a known path. Caddy routes it. The agent processes it via the worker pipeline.

**Certainty:** High (both systems exist, just need wiring)

### Integration 2: Compliance Ledger → Shared Workspace

**Problem:** Compliance logs are in each agent's private workspace. The manager can't audit them.

**Solution:** Configure intent-gateway's `project.yaml` to write logs to the shared workspace:

```yaml
compliance:
  audit_log_dir: /home/node/.openclaw/shared/compliance/logs
```

Since clawctl already mounts `shared/` into every grouped instance, all agents write to the same compliance ledger. The manager gets a unified view. Hash chains still work because writes are append-only.

**Certainty:** High (just a config change)

### Integration 3: Approval Engine → clawctl Group Roles

**Problem:** clawctl's manager/worker roles are filesystem-level (who can read what). The intent-gateway's approval engine is action-level (who can do what, when). They don't talk to each other.

**Solution:** When clawctl creates a manager, it also grants a default approval:

```bash
clawctl create backend/lead --role=manager
# Under the hood, also runs:
sarahctl approval grant '{
  "actor": "lead",
  "intent": "*",
  "tools": ["*"],
  "paths": ["shared/*"],
  "start": "2026-03-17T00:00:00Z",
  "end": "2026-12-31T23:59:59Z",
  "no_delete": true
}'
```

Workers get narrower grants — only the tools and paths their role needs. The intent-gateway enforces it. clawctl scaffolds it.

**Certainty:** Medium (requires clawctl to know about sarahctl's CLI)

### Integration 4: clawctl Task Queue → intent-gateway Worker

**Problem:** clawctl scaffolds `pending/claimed/done` directories. The intent-gateway has a worker that processes inbox events. They're two separate job queues.

**Solution:** Unify them. The intent-gateway's worker reads from the same `pending/` directory that clawctl's task queue uses. The worker's transport bridge normalizes task files into compliance records.

```
Manager writes:  shared/tasks/pending/review-pr-42.md
Worker's intent-gateway picks it up:
  → transport.Ingest() → compliance ledger → hook emitted → agent processes
```

The intent-gateway adds: deduplication, hash-chaining, compliance logging, hook events. clawctl's task queue becomes the intake for the intent-gateway's processing pipeline.

**Certainty:** Medium (requires adapting the worker to read markdown task files)

### Integration 5: Hook Events → clawctl Activity Feed

**Problem:** clawctl's activity feed scans file modification times — a blunt instrument. The intent-gateway emits structured JSONL hook events with timestamps, event types, and payloads.

**Solution:** `clawctl activity` reads intent-gateway hook logs in addition to file scans:

```go
// In activity.go, add:
hookLogPath := filepath.Join(dir, "workspace", ".compliance-staging", "logs", "events.jsonl")
hookEntries := parseHookLog(hookLogPath, cutoff)
```

This gives the activity feed real event data: "sarah granted an approval," "worker processed 3 events," "compliance check denied a delete."

**Certainty:** High (just reading a JSONL file)

### Integration 6: Health Aggregation

**Problem:** clawctl checks container health (/healthz, /readyz). The intent-gateway has its own worker health status. They're separate.

**Solution:** `clawctl health` also reads the intent-gateway's worker status:

```go
workerStatusPath := filepath.Join(dir, "workspace", ".compliance-staging", "runtime", "worker-status.json")
// Parse and include in health output
```

Health output becomes:
```
NAME            PORT     GATEWAY    WORKER     DETAILS
backend/sarah   :18789   healthy    healthy    3 events processed
backend/worker1 :18889   healthy    degraded   inbox backlog: 12 events
```

**Certainty:** High (just reading a JSON file)

---

## How It All Fits Together

```
USER EXPERIENCE (what the operator sees):

  "I want a team of agents that coordinate, audit their work,
   and I can monitor from one place."

  clawctl group create backend
  clawctl create backend/lead --role=manager
  clawctl create backend/dev1 --role=worker --manager=lead
  clawctl create backend/dev2 --role=worker --manager=lead
  clawctl start backend/lead
  clawctl start backend/dev1
  clawctl start backend/dev2

  # Manager assigns work (via chat or file):
  #   "Review PR #42 and write a summary"
  #   → lands in shared/tasks/pending/

  # Worker picks it up:
  #   → intent-gateway dedupes, logs to compliance ledger
  #   → agent processes, writes result to shared/output/
  #   → intent-gateway emits hook: "task.completed"

  # Operator checks:
  clawctl dashboard           # live view: all 3 agents healthy
  clawctl activity --group=backend  # "dev1 completed review-pr-42"
  clawctl health              # gateway + worker health per instance

  # Compliance audit (anytime):
  #   Hash-chained JSONL logs prove every action, every approval,
  #   every message was recorded and never tampered with.
```

### What clawctl Provides

- **Lifecycle** — create, start, stop, remove instances
- **Structure** — groups, manager/worker topology, port allocation
- **Sharing** — filesystem mounts for collaboration
- **Observability** — health, dashboard, activity feed
- **Storage** — S3 backup and FUSE mount
- **Access** — proxy, tunnels

### What intent-gateway Adds

- **Approval gating** — time-bound, scoped permissions for agent actions
- **Compliance ledger** — hash-chained, append-only, immutable audit trail
- **Transport bridge** — deduplication, normalization, webhook ingestion
- **Worker pipeline** — retry logic, adapter pattern, status reporting
- **Structured hooks** — typed events with payloads for automation
- **Scheduler** — cron-based recurring tasks with callback system

### What's Missing (Integration Work)

| Item | Effort | Priority |
|---|---|---|
| Configure compliance logs to shared workspace | Config change | P0 |
| Read hook JSONL in `clawctl activity` | ~30 lines Go | P0 |
| Read worker-status.json in `clawctl health` | ~20 lines Go | P0 |
| Proxy webhook routes per instance | Caddy config gen | P1 |
| Scaffold default approval grants for manager/worker | ~50 lines Go | P1 |
| Unify task queue with worker inbox | Adapter in intent-gateway | P2 |
| Cross-instance callback firing | New transport adapter | P3 |
| Cross-instance scheduling | Shared scheduler registry | P3 |

---

## Integration 7: The Sidecar Pattern (Sarah's Ticket)

Sarah wrote a ticket (`2026-03-10-ingestion-threading-stream.md`) requesting exactly what clawctl + intent-gateway together provide. Her core problem:

> "Background agents must not interrupt or derail the core process."

She wants her coding work (main process) separated from ingestion, status checks, and event routing (background process). Today both run in the same agent — when a WhatsApp message arrives while she's deep in a coding task, it interrupts her context.

### What She Asked For vs What Exists

| Requirement | Built? | Where |
|---|---|---|
| Ingestion pipeline | Yes | intent-gateway `transport.Ingest()` |
| Thread-aware routing | Partial | `thread_id` stored, not routed |
| Hook/event bus | Yes | intent-gateway `hooks.Manager` |
| Background workers | Yes | intent-gateway `worker.RunLoop()` |
| Foreground/background separation | **No** | **This is the gap** |
| Append-only audit | Yes | intent-gateway compliance ledger |
| Deduplication | Yes | intent-gateway transport registry |
| Multi-threaded contexts | **No** | thread_id exists, routing doesn't |

### The Solution: clawctl Sidecar Instance

```bash
# Create the group
clawctl group create backend

# Sarah's main instance — coding, reviewing, building
clawctl create backend/sarah --role=manager

# Sidecar instance — ingestion, threading, status reporting
clawctl create backend/sarah-ingest --role=worker --manager=sarah
```

The sidecar instance:
- Runs `sarahctl worker run` continuously (polls inbox)
- Receives transport events from OpenClaw's message hooks
- Deduplicates, normalizes, hash-chains into compliance ledger
- Routes threaded events to the right task queue dir
- Emits hook events for the activity feed
- **Never interrupts sarah's coding process**

```
┌──────────────────────────────────┐
│  sarah (manager)                 │
│  Role: coding, reviewing         │
│  Reads: tasks/pending/           │
│  Reads: workers/sarah-ingest/ (ro)│
│  Writes: tasks/ (dispatch)       │
└────────────────┬─────────────────┘
                 │ shared workspace
                 ▼
┌──────────────────────────────────┐
│  shared/                         │
│  ├── tasks/pending/              │
│  ├── tasks/claimed/              │
│  ├── tasks/done/                 │
│  ├── compliance/logs/            │
│  └── workspace/                  │
└────────────────┬─────────────────┘
                 │ shared workspace
                 ▼
┌──────────────────────────────────┐
│  sarah-ingest (worker/sidecar)   │
│  Role: ingestion, routing        │
│  Runs: sarahctl worker run       │
│  Reads: tasks/ (read-only)       │
│  Writes: compliance logs         │
│  Writes: output/ (results)       │
│  Never touches sarah's context   │
└──────────────────────────────────┘
```

### Thread Routing (The Missing Piece)

The ticket asks for "thread-aware event routing." The intent-gateway has `thread_id` on transport events but doesn't route by it. The sidecar pattern makes this solvable:

1. WhatsApp group message arrives with a thread/topic
2. OpenClaw hook fires → writes to sidecar's inbox
3. Sidecar's worker picks it up via `sarahctl worker run-once`
4. Worker classifies: is this a new thread or continuation?
5. Routes to the right task file: `shared/tasks/pending/thread-<id>.md`
6. Sarah's main instance sees it in her task queue when she's ready
7. She picks it up without being interrupted mid-coding

This is the pub/sub pattern from distributed systems — the sidecar is the subscriber, the task queue is the topic, and sarah polls at her own pace.

### What Needs to Be Built

| Item | Where | Effort |
|---|---|---|
| Thread routing logic in intent-gateway worker | `internal/worker/worker.go` | Medium — classify by thread_id, write to per-thread task files |
| OpenClaw hook that forwards messages to sidecar inbox | Shared hooks directory | Small — hook script that appends to sidecar's inbox JSONL |
| Sidecar startup script (runs worker loop on boot) | Instance workspace | Small — `sarahctl run` as the container entrypoint override |
| clawctl sidecar command | `clawctl create --sidecar` shorthand | Small — creates worker with auto-start worker loop |

### User Experience

```bash
# Setup (once)
clawctl group create backend
clawctl create backend/sarah --role=manager
clawctl create backend/sarah-ingest --role=worker --manager=sarah --sidecar
clawctl start backend/sarah
clawctl start backend/sarah-ingest

# Daily: sarah codes. Messages arrive. She's never interrupted.
# The sidecar ingests, dedupes, routes, and logs.
# Sarah checks her task queue when SHE decides to.

# Monitoring
clawctl dashboard
# Shows:
#   backend/sarah          :18789  healthy  coding          3h uptime
#   backend/sarah-ingest   :18889  healthy  12 events/hr    3h uptime

clawctl activity --group=backend
# Shows:
#   10:15  sarah-ingest  transport  ingested whatsapp event e-4821
#   10:15  sarah-ingest  compliance message hash-chained
#   10:16  sarah-ingest  routing    routed to thread-pr-review
#   10:45  sarah         task       claimed thread-pr-review
#   11:02  sarah         task       completed thread-pr-review
```

This is the architecture Sarah designed in her ticket. She built the engine (intent-gateway). We built the orchestration layer (clawctl). The sidecar pattern connects them.
