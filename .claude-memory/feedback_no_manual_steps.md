---
name: no-manual-steps
description: Commands must handle their full lifecycle — never leave the user with broken state requiring manual docker/file commands
type: feedback
---

When a clawctl command changes system state, it must complete the full operation. No half-done states.

**Why:** User explicitly said "our system needs to do that, our system needs to do everything needed" when `group add` moved files but left old Docker containers running under wrong project names.

**How to apply:**
- `group add` → stop old project, move files, update registry, rebuild overrides, start new project
- `channel add` → set config, restart gateway, print next steps
- `remove --purge` → confirm, stop, unregister, delete
- `group role` → update env, rebuild overrides for target AND manager, tell user to restart
- Never require the user to run raw `docker compose` or `docker` commands to fix state clawctl created
