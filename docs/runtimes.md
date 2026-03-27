# Runtime Guide — Using Different Agent Runtimes

clawctl can manage any containerized AI agent, not just OpenClaw. This guide covers the common scenarios from simplest to most advanced.

## Most Users: Just Use `--image=`

If your agent is OpenClaw-compatible (same ports, same health endpoints, same CLI), you only need a different Docker image. No runtime registration needed.

```bash
# Slim build (fewer system packages, smaller image)
docker build --build-arg OPENCLAW_VARIANT=slim -t openclaw:slim .
clawctl create alice --image=openclaw:slim

# Extension-specific build (only Telegram + Discord)
docker build --build-arg OPENCLAW_EXTENSIONS="telegram discord" -t openclaw:minimal .
clawctl create bob --image=openclaw:minimal

# Version-pinned
clawctl create charlie --image=openclaw:v2026.3.25

# Upgrade later
clawctl upgrade alice --image=openclaw:v2026.4.1
```

This works because these are all the same OpenClaw runtime with different Docker images. The ports, health endpoints, CLI commands, and config format are identical.

## Compatible Fork: `--from=openclaw`

If your fork changes a few things (health endpoint, default port) but is mostly OpenClaw-compatible, inherit from openclaw and override what's different:

```bash
# nemoclaw — same as openclaw but different image
clawctl runtime add nemoclaw --from=openclaw --image=nemoclaw:latest
clawctl create alice --runtime=nemoclaw

# nanoclaw — different health endpoint
clawctl runtime add nanoclaw --from=openclaw --image=nanoclaw:latest --health=/api/health --ready=
clawctl create bob --runtime=nanoclaw

# Fork with no channel support
clawctl runtime add lite-claw --from=openclaw --image=lite-claw:v1 --no-channels --no-pairing
clawctl create charlie --runtime=lite-claw
```

## Different Agent: Scaffold from Scratch

If your agent is a completely different codebase (Python, Rust, etc.), scaffold a runtime definition:

```bash
# 1. Scaffold — creates runtime JSON + compose template
clawctl runtime init my-python-agent

# 2. Edit the generated files:
#    ~/.openclaw/runtimes/my-python-agent.json        — settings
#    ~/.openclaw/runtimes/my-python-agent-compose.yml  — Docker compose

# 3. Test it
clawctl runtime test my-python-agent

# 4. Create instances
clawctl create researcher --runtime=my-python-agent
clawctl start researcher
```

### What to Edit in the JSON

The key fields to change:

```json
{
  "name": "my-python-agent",
  "defaultImage": "my-python-agent:latest",
  "gatewayService": "my-python-agent-gateway",
  "internalPort": 8080,
  "healthEndpoint": "/health",
  "readyEndpoint": "",
  "containerHome": "/app",
  "configFileName": "config.json",
  "capabilities": {
    "channels": false,
    "pairing": false,
    "auth": false,
    "config": false,
    "tasks": true,
    "shared": true,
    "bridge": false
  }
}
```

### What to Edit in the Compose Template

```yaml
services:
  my-python-agent-gateway:
    image: ${OPENCLAW_IMAGE:-my-python-agent:latest}
    command: ["python", "main.py", "--port", "8080"]
    ports:
      - "${OPENCLAW_HOST_BIND:-127.0.0.1}:${OPENCLAW_GATEWAY_PORT:-8080}:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://127.0.0.1:8080/health"]
```

## Auto-Detect from Docker Image

If you have an image but don't know its settings:

```bash
clawctl runtime detect my-agent:latest
# Shows: exposed ports, user, entrypoint
# Suggests the right clawctl runtime add command
```

## Mixed Runtimes in One Group

Different runtimes can share a group's workspace and task queue:

```bash
clawctl group create research

clawctl create research/gpt-agent --runtime=openclaw
clawctl create research/claude-agent --runtime=nanoclaw
clawctl create research/custom --runtime=my-python-agent

# All three share workspace, tasks, and output
clawctl group shared research --all
```

## Sharing Runtime Definitions

Export a runtime definition and share it with your team:

```bash
# Export (includes compose template if present)
clawctl runtime export nemoclaw > nemoclaw.json

# On another machine
clawctl runtime import nemoclaw.json
```

## Quick Reference

| Scenario | Command |
|----------|---------|
| Same runtime, different image | `clawctl create alice --image=openclaw:slim` |
| Compatible fork | `clawctl runtime add nemoclaw --from=openclaw --image=nemoclaw:latest` |
| Fork with different health | `clawctl runtime add nanoclaw --from=openclaw --health=/status` |
| Completely different agent | `clawctl runtime init my-agent` then edit files |
| Unknown image | `clawctl runtime detect my-image:latest` |
| Share with team | `clawctl runtime export/import` |
| Validate before use | `clawctl runtime test my-agent` |
