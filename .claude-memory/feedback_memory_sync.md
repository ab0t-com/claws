---
name: memory-sync-to-repo
description: All memory files must have a sync copy in the repo's .claude-memory/ directory so nothing is lost
type: feedback
---

When creating or updating any Claude memory file, always sync a copy to `.claude-memory/` in the repo root.

**Why:** Claude memory lives in `~/.claude/projects/*/memory/` which can be cleared. The user wants a durable copy in the repo so context is never lost.

**How to apply:** After writing to `~/.claude/projects/.../memory/foo.md`, also copy it to `<repo>/.claude-memory/foo.md` and update `<repo>/.claude-memory/MEMORY.md` index.
