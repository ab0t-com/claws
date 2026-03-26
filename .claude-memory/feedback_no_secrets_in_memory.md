---
name: no-secrets-in-memory
description: Never store tokens, keys, passwords, or any credentials in Claude memory files
type: feedback
---

NEVER store secrets, tokens, API keys, passwords, or any credentials in Claude memory files.

**Why:** Memory files are part of the Claude context system and are not a secure credential store. The user explicitly corrected this — secrets go in regular files on disk with restricted permissions (0600), not in .claude/memory/.

**How to apply:** When asked to "save" or "keep safe" a credential, write it to a plain file in ~/ or the user's preferred location with chmod 600. Never put credential values in memory .md files.
