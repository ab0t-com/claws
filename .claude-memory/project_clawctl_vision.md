---
name: clawctl-vision
description: clawctl is evolving from an OpenClaw-specific manager into a generic agent control plane with admin policy, multi-runtime support, and multi-tenant access
type: project
---

clawctl is not just an OpenClaw manager — it's being designed as a **generic control plane for AI agent teams**.

**Why:** Multiple "claw-like" agent runtimes exist. The control plane pattern (port management, groups, shared resources, task queue, proxy, storage) is runtime-agnostic. clawctl should be the orchestration layer that works with any containerized agent gateway.

**Key decisions:**
- Adapter pattern for runtimes (not hardcoded to OpenClaw)
- Admin policy layer for security constraints
- Multi-tenant access control (admin/operator/user roles)
- Secure-by-default (loopback binding, cap_drop ALL, sandbox enforcement)
- Image management (build/pull/pin/upgrade with rollback)

**How to apply:** When building new features, design them runtime-agnostic. Don't hardcode OpenClaw-specific paths, commands, or config formats. The Runtime interface (ticket 3) should be the abstraction boundary.
