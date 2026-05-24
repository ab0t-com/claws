# Ticket 7: Runtime UX — Make Multi-Runtime Practical for Real Users

**Priority:** P1 — High
**Created:** 2026-03-26
**Status:** Done
**Depends on:** Ticket 3 (Runtime Adapter — Done)
**Branch:** `feature/runtime-adapter` (or new branch from it)

---

## Context

Ticket 3 built the full runtime adapter pattern — 40+ configurable fields, custom runtime registry, per-instance binding, capability gating. The engineering is solid. But looking at this from a PMM perspective — how real users will actually interact with multiple runtimes — there are significant gaps between what we built and what people will need.

### The Claw Ecosystem Today

OpenClaw is the primary project. The "variants" that users will encounter are:

| Variant Type | Example | How It Differs | What User Needs |
|-------------|---------|----------------|-----------------|
| **Slim build** | `openclaw:slim` | Smaller image, fewer system packages | Just `--image=openclaw:slim` |
| **Extension-specific** | `openclaw:telegram-only` | Built with `--build-arg OPENCLAW_EXTENSIONS="telegram"` | Just `--image=openclaw:telegram-only` |
| **Version-pinned** | `openclaw:v2026.3.25` | Specific release tag | Just `--image=openclaw:v2026.3.25` |
| **Fork (compatible)** | `nemoclaw:latest` | Different branding, same API surface | `--image=` might work, or a runtime with tweaked fields |
| **Fork (divergent)** | `nanoclaw:latest` | Different health endpoints, different CLI, maybe different config format | Full runtime definition |
| **Different runtime** | `my-python-agent:v1` | Completely different tech stack, no OpenClaw CLI | Full runtime with most capabilities disabled |

The key insight: **80% of users only need `--image=`**. The full runtime adapter is for the 20% with genuinely different runtimes. But right now, both groups face the same complexity.

### What's Wrong Today

**Problem 1: The simple case is hidden behind the complex case.**

A user who wants to run a slim build has to discover that `--image=` exists. The help text and docs present `claws runtime add` as the way to use different images. A user reading the help thinks:

> "I want to use nemoclaw instead of openclaw. I need to register a runtime."

When actually:

> "I just need `claws create alice --image=nemoclaw:latest` and everything works."

**Problem 2: Creating a new runtime is too many manual steps.**

To properly register nemoclaw:

1. Build or pull the Docker image
2. Figure out what health endpoint it uses
3. Figure out what port it listens on internally
4. Figure out what compose services it needs
5. Create or copy a compose template
6. Run `claws runtime add nemoclaw --image=... --health=... --port=... --gateway-service=...`

That's 6 steps requiring knowledge of the runtime's internals. Most users don't know what a "gateway service" is. They have a Docker image and want to run it.

**Problem 3: No inheritance between runtimes.**

Most forks of OpenClaw change 1-2 things (image, maybe health endpoint). But `runtime add` starts from scratch — the user has to specify everything. We have `--from=` for instance templates but not for runtimes.

If nemoclaw is 99% compatible with OpenClaw (same ports, same CLI, different image), the user should say:

```bash
claws runtime add nemoclaw --from=openclaw --image=nemoclaw:latest
```

Not re-specify 40 fields.

**Problem 4: No way to validate before using.**

After `runtime add`, the user has no way to test if their definition is correct until they `create` + `start` an instance and watch it fail. There's no:

```bash
claws runtime test nemoclaw
```

That would pull the image, start a temporary container, hit the health endpoint, and report success/failure — all without creating a persistent instance.

**Problem 5: No way to share runtime definitions.**

If a team figures out the right settings for nemoclaw, there's no easy way to share. The JSON file is in `~/.openclaw/runtimes/nemoclaw.json` but there's no `claws runtime export/import` flow. Users would have to manually copy files.

**Problem 6: No compose template scaffolding.**

For non-OpenClaw runtimes, users need a compose template. But there's no guided way to create one. They'd have to copy `docker-compose.yml` and modify it by hand, knowing which parts to change and which to keep.

**Problem 7: No runtime discovery.**

If a user has a Docker image and doesn't know its internals, there's no way to auto-detect:
- What port does it listen on?
- What health endpoint does it have?
- What user does it run as?

A `claws runtime detect <image>` could inspect the image and suggest a runtime definition.

---

## User Stories

### Story 1: Simple Image Swap (80% case)
> "I built a custom OpenClaw image with only the extensions I need. I want to use it."

**Current flow:** User may not realize `--image=` exists, tries to figure out runtime add.
**Target flow:**
```bash
claws create alice --image=my-openclaw:slim
# That's it. Everything else is identical to default.
```
**Gap:** Documentation and help text don't lead with this path.

### Story 2: Compatible Fork (15% case)
> "nemoclaw is a fork of OpenClaw. Same ports, same CLI, different image and a different health endpoint."

**Current flow:** Manual `runtime add` with 6+ flags.
**Target flow:**
```bash
claws runtime add nemoclaw --from=openclaw --image=nemoclaw:latest --health=/status
# Inherits everything from openclaw, overrides image and health
```
**Gap:** No `--from=` inheritance on runtime add.

### Story 3: Different Runtime (5% case)
> "I built a Python agent. It has a REST API on port 8080, health at /health, no CLI service, no channels."

**Current flow:** Must know all the right flags.
**Target flow:**
```bash
claws runtime init my-agent
# Creates ~/.openclaw/runtimes/my-agent.json with documented fields
# Creates ~/.openclaw/runtimes/my-agent-compose.yml scaffold
# User edits both files

claws runtime test my-agent
# Validates the definition works
```
**Gap:** No `init` scaffolding, no `test` validation.

### Story 4: Team Sharing
> "I set up nemoclaw for our team. I want others to use the same definition."

**Current flow:** Manually copy JSON file.
**Target flow:**
```bash
# Export
claws runtime export nemoclaw > nemoclaw-runtime.json

# On another machine
claws runtime import nemoclaw-runtime.json
```
**Gap:** No `export/import` commands.

### Story 5: Unknown Image
> "Someone gave me a Docker image. I don't know what port it uses or where its health endpoint is."

**Target flow:**
```bash
claws runtime detect my-unknown-agent:latest
# Inspects image: exposes port 8080, runs as user 'app', entrypoint is python
# Suggests: claws runtime add my-agent --image=my-unknown-agent:latest --port=8080 --health=/health --no-channels --no-cli
```
**Gap:** No `detect` command.

---

## Implementation Plan

### 7.1 — `runtime add --from=<runtime>` (Inherit & Override)

**What:** When `--from=` is specified, start from that runtime's definition instead of OpenClaw defaults. Only override fields that have explicit flags.

**Files:** `runtime.go` cmdRuntimeAdd
**Effort:** Small

```go
// In cmdRuntimeAdd:
if fromName != "" {
    base, ok := getRuntimeByName(paths, fromName)
    if !ok {
        return errorf("base runtime '%s' not found", fromName)
    }
    rt = base        // start from base
    rt.Name = name   // override name
    rt.Description = "Based on " + fromName
}
// Then apply --flags on top
```

**Test:** `claws runtime add nemoclaw --from=openclaw --image=nemoclaw:latest` → runtime has OpenClaw's ports/health/CLI but nemoclaw's image.

---

### 7.2 — `runtime init <name>` (Scaffolding)

**What:** Creates a JSON definition and compose template with documented fields that the user can edit. Like `claws init` but for a runtime.

**Files:** New code in `runtime.go`
**Effort:** Medium

```bash
claws runtime init my-agent
# Creates:
#   ~/.openclaw/runtimes/my-agent.json         (with comments explaining each field)
#   ~/.openclaw/runtimes/my-agent-compose.yml   (minimal compose template)
# Prints: "Edit these files, then run: claws runtime test my-agent"
```

The scaffold compose template should be minimal and well-commented:
```yaml
# Compose template for my-agent runtime
# Edit this file to match your agent's requirements.
services:
  my-agent-gateway:
    image: ${OPENCLAW_IMAGE:-my-agent:latest}
    ports:
      - "${OPENCLAW_HOST_BIND:-127.0.0.1}:${OPENCLAW_GATEWAY_PORT:-18789}:8080"
    # ... documented fields ...
```

**Test:** `runtime init` creates both files, JSON is valid, compose template references the right service name.

---

### 7.3 — `runtime test <name>` (Validation)

**What:** Starts a temporary container from the runtime's image, waits for the health endpoint, reports success/failure, then tears down. Does not create a persistent instance.

**Files:** New code in `runtime.go`
**Effort:** Medium

```bash
claws runtime test my-agent
# 1. Pull image if not present
# 2. Start temporary container with --rm
# 3. Wait for health endpoint (up to 30s)
# 4. Report: "✓ my-agent runtime is healthy" or "✗ health check failed at /health"
# 5. Container auto-removed
```

**Implementation:**
```go
func cmdRuntimeTest(args []string) error {
    // Create temp dir, temp env file, start with docker compose
    // Hit health endpoint
    // Tear down
    // Report
}
```

**Test:** Test with openclaw runtime (known good), test with invalid runtime (should fail gracefully).

---

### 7.4 — `runtime export/import` (Sharing)

**What:** Export a runtime definition (and optionally its compose template) to a file that can be shared. Import from that file.

**Files:** New code in `runtime.go`
**Effort:** Small

```bash
# Export (includes compose template if present)
claws runtime export nemoclaw > nemoclaw.bundle.json

# Import
claws runtime import nemoclaw.bundle.json
```

The bundle format is just the runtime JSON with an optional `composeTemplate` field containing the compose YAML as a string:

```json
{
  "name": "nemoclaw",
  "defaultImage": "nemoclaw:latest",
  "healthEndpoint": "/status",
  "composeTemplate": "services:\n  nemoclaw-gateway:\n    image: ...\n",
  ...
}
```

**Test:** Export openclaw → import as nemoclaw-copy → verify identical fields.

---

### 7.5 — `runtime detect <image>` (Auto-Detection)

**What:** Inspects a Docker image and suggests a runtime definition based on what it finds.

**Files:** New code in `runtime.go`
**Effort:** Medium

```bash
claws runtime detect my-agent:latest
# Inspecting my-agent:latest...
#   Exposed ports: 8080
#   User: app
#   Entrypoint: ["python", "main.py"]
#   Health: (unknown — try /health, /healthz, /)
#
# Suggested command:
#   claws runtime add my-agent --image=my-agent:latest --port=8080 --container-home=/home/app --no-channels --no-cli
```

Uses `docker inspect` to read:
- `ExposedPorts` → suggests port
- `Config.User` → suggests container home
- `Config.Entrypoint` → suggests whether it's Node (might be OpenClaw-compatible) or something else
- `Config.Healthcheck` → uses built-in health check if present

**Test:** Detect openclaw:local → should suggest OpenClaw-compatible settings.

---

### 7.6 — Documentation & Help Text Rewrite

**What:** Restructure docs to lead with the simple case, progressively disclose complexity.

**Files:** README.md, help.go, docs/

**New documentation structure:**

```markdown
## Using Different Images

Most users only need to specify a different Docker image:

    claws create alice --image=openclaw:slim
    claws create bob --image=nemoclaw:latest
    claws upgrade alice --image=openclaw:v2026.4.1

This works for any image that's compatible with OpenClaw (same ports,
same health endpoints, same CLI). This covers:
- Slim builds
- Extension-specific builds
- Version-pinned images
- Compatible forks

## Custom Runtimes (Advanced)

If your agent has different ports, health endpoints, or CLI commands,
you need a custom runtime definition:

    # Start from OpenClaw and override what's different
    claws runtime add nemoclaw --from=openclaw --image=nemoclaw:latest --health=/status

    # Or scaffold from scratch
    claws runtime init my-python-agent
    # Edit the generated files, then:
    claws runtime test my-python-agent
    claws create alice --runtime=my-python-agent

    # Share with your team
    claws runtime export my-python-agent > my-python-agent.json
    claws runtime import my-python-agent.json
```

**Help text changes:**
- `create --help` should show `--image=` prominently, before `--runtime=`
- `runtime --help` should explain when you need a custom runtime vs just `--image=`
- `runtime add --help` should show `--from=` first

---

### 7.7 — Common Variant Recipes (Documentation)

**What:** A `docs/runtimes.md` guide with copy-paste recipes for common scenarios.

**Contents:**

1. **Running a slim build**
   ```bash
   docker build --build-arg OPENCLAW_VARIANT=slim -t openclaw:slim .
   claws create alice --image=openclaw:slim
   ```

2. **Running with specific extensions only**
   ```bash
   docker build --build-arg OPENCLAW_EXTENSIONS="telegram discord" -t openclaw:minimal .
   claws create alice --image=openclaw:minimal
   ```

3. **Running a compatible fork (nemoclaw)**
   ```bash
   claws runtime add nemoclaw --from=openclaw --image=nemoclaw:latest
   claws create alice --runtime=nemoclaw
   ```

4. **Running a divergent fork (different health endpoint)**
   ```bash
   claws runtime add nanoclaw --from=openclaw --image=nanoclaw:latest --health=/api/health --ready=
   claws create alice --runtime=nanoclaw
   ```

5. **Running a completely different agent (Python)**
   ```bash
   claws runtime init my-python-agent
   # Edit ~/.openclaw/runtimes/my-python-agent.json:
   #   "defaultImage": "my-python-agent:latest"
   #   "internalPort": 8080
   #   "healthEndpoint": "/health"
   #   "gatewayService": "agent"
   #   "cliService": ""  (no CLI)
   # Edit ~/.openclaw/runtimes/my-python-agent-compose.yml
   claws runtime test my-python-agent
   claws create alice --runtime=my-python-agent
   ```

6. **Running mixed runtimes in one group**
   ```bash
   claws group create research
   claws create research/gpt-agent --runtime=openclaw
   claws create research/claude-agent --runtime=nanoclaw
   claws create research/custom --runtime=my-python-agent
   # All three share the group workspace and task queue
   ```

---

## Testing Plan

| Test | Type | Validates |
|------|------|-----------|
| `runtime add --from=openclaw` inherits fields | Integration | 7.1 |
| `runtime add --from=openclaw --image=X` overrides image only | Integration | 7.1 |
| `runtime add --from=nonexistent` fails clearly | Integration | 7.1 |
| `runtime init` creates JSON + compose | Integration | 7.2 |
| `runtime init` JSON is valid and loadable | Unit | 7.2 |
| `runtime init` compose template has correct service name | Unit | 7.2 |
| `runtime test openclaw` passes (health check works) | Integration | 7.3 |
| `runtime test` with bad health endpoint fails gracefully | Integration | 7.3 |
| `runtime export` produces valid JSON | Integration | 7.4 |
| `runtime import` registers the runtime | Integration | 7.4 |
| `runtime export` + `import` round-trips correctly | Integration | 7.4 |
| `runtime detect openclaw:local` suggests correct settings | Integration | 7.5 |
| `runtime detect` with unknown image gives useful suggestions | Integration | 7.5 |
| `create --help` shows `--image=` before `--runtime=` | Integration | 7.6 |
| `runtime --help` explains when to use runtime vs image | Integration | 7.6 |

---

## Effort Estimate

| Subtask | Effort | Priority |
|---------|--------|----------|
| 7.1 `--from=` inheritance | Small (30min) | High — makes runtime add usable |
| 7.2 `runtime init` scaffolding | Medium (1hr) | High — makes custom runtimes approachable |
| 7.3 `runtime test` validation | Medium (1hr) | High — prevents broken runtimes |
| 7.4 `export/import` | Small (30min) | Medium — team sharing |
| 7.5 `runtime detect` | Medium (1hr) | Medium — nice to have, helps unknowns |
| 7.6 Docs/help rewrite | Medium (1hr) | High — the 80% case needs to be obvious |
| 7.7 Recipes doc | Small (30min) | Medium — practical guides |

**Total estimated effort:** ~6 hours

---

## Success Criteria

1. A user with a compatible fork can go from zero to running instance in 2 commands (`runtime add --from=`, `create`)
2. A user with a completely different agent can scaffold and test a runtime definition without reading source code
3. The help text and docs make it obvious that `--image=` is the simple path and `runtime` is the advanced path
4. Runtime definitions can be shared between team members or machines
5. `runtime test` catches broken definitions before they cause confusing failures at `create`/`start` time
6. All existing tests continue to pass — zero regressions
