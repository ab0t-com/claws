# Memory Index (repo sync)

This is a sync copy of Claude memory files. The authoritative copies live in
`~/.claude/projects/*/memory/`. This directory ensures nothing is lost if the
Claude memory dir is cleared.

## Project-level (clawctl-go)

- [user_role.md](user_role.md) — User is a product owner + architect building an AI agent orchestration platform with real production agents
- [project_clawctl_vision.md](project_clawctl_vision.md) — claws is evolving into a generic agent control plane with adapter pattern, admin policy, multi-tenant access
- [feedback_no_manual_steps.md](feedback_no_manual_steps.md) — Commands must complete full lifecycle — never leave broken state requiring manual docker commands
- [feedback_memory_sync.md](feedback_memory_sync.md) — Always sync memory files to .claude-memory/ in the repo so nothing is lost

## Workspace-level (claw workspace)

- [feedback_no_secrets_in_memory.md](feedback_no_secrets_in_memory.md) — Never store tokens/keys/credentials in Claude memory; use regular files with 0600 permissions
