# Ticket 8: One-Click Onboarding — `claws setup` and Safe-by-Default Init

**Priority:** P0 — Ship-blocking for one-click vision
**Created:** 2026-03-27
**Status:** Open
**Depends on:** All previous tickets (1-7) — Done

---

## Problem

Getting from zero to a working team of agents requires 15+ commands, 5+ context switches, and expert-level knowledge. Security is opt-in. There's no guided flow. The help output is 75 lines across 13 sections. A new user is lost.

The vision is: **one command from nothing to a working team**.

---

## Changes

### 8.1 — `claws init` creates policy + access + audit automatically

**Current:** `init` creates directories and copies compose template. Security is a separate step.
**After:** `init` also creates `policy.json` (secure defaults), `.access.json` (current user as admin), enables audit logging. No extra commands.

**Files:** `init.go`
**Effort:** Small

**What changes:**
```go
// After creating dirs and compose template:
if !policyExists(paths) {
    cmdPolicyInit([]string{})  // creates secure default policy
}
if !accessExists(paths) {
    cmdAccessInit([]string{})  // current user as admin
}
```

**Test:** `claws init` → `policy.json` exists, `.access.json` exists, audit log enabled.

---

### 8.2 — First-run detection

**Current:** `clawctl` with no args prints 75-line help regardless of state.
**After:** If OPENCLAW_ROOT doesn't exist, prints welcome message with `setup` suggestion. If it exists, prints brief status.

**Files:** `main.go`
**Effort:** Small

```go
if len(os.Args) < 2 {
    if !rootExists(paths) {
        printWelcome()  // "Run: claws setup"
    } else {
        printBriefStatus()  // instance count, health summary
    }
    os.Exit(0)
}
```

---

### 8.3 — `claws setup` — guided interactive onboarding

**Current:** Doesn't exist.
**After:** Single command that walks through init + team + agent + auth + channel + start.

**Files:** New `setup.go`
**Effort:** Large

**Flow:**
1. Check prerequisites (Docker, image)
2. Create workspace (init + policy + access)
3. Ask team name → `group create`
4. Ask agent name → `create <team>/<name>`
5. Ask auth method → `auth` (codex or apikey)
6. Ask channel → `channel add` (telegram/whatsapp/discord/skip)
7. Start agent → `start`
8. Print status + next steps
9. Ask "add another?" → loop 4-7

**Non-interactive mode:**
```bash
claws setup --non-interactive \
  --team=research \
  --agent=sarah \
  --auth=apikey --anthropic-key=sk-... \
  --telegram=TOKEN
```

**Requirements:**
- Must read from stdin for interactive prompts
- Must handle Ctrl+C gracefully (clean up partial state)
- Must be idempotent — running setup twice adds to existing, doesn't break
- Must skip steps that are already done (init already run → skip)

---

### 8.4 — Default tool profile on `create`

**Current:** New instances have no tool profile — audit warns.
**After:** New instances get `tools.profile = "coding"` in their config.

**Files:** `commands.go` cmdCreate — set in skeleton config
**Effort:** Small

```go
skeleton := map[string]any{
    "gateway": map[string]any{...},
    "tools": map[string]any{"profile": "coding"},
}
```

---

### 8.5 — Unified `claws status`

**Current:** User must run list + health + policy validate + audit separately.
**After:** One command shows everything.

**Files:** New `status.go` or extend `commands.go`
**Effort:** Medium

```
$ claws status

  Team: research (3 agents)

  NAME              PORT    HEALTH    CHANNELS          RAM
  research/sarah    :18789  healthy   telegram,whatsapp  512MB
  research/john     :18889  healthy   telegram           420MB
  research/lead     :19089  healthy   —                  230MB

  Security: ✓ policy active, ✓ access control, ✓ audit logging
  Warnings: 3 agents without sandbox mode

  Last activity: 2 minutes ago (research/sarah: file change in workspace)
```

---

### 8.6 — Tiered help

**Current:** One 75-line help dump.
**After:**

- `clawctl` (no args, first run) → welcome + setup
- `clawctl` (no args, initialized) → brief status + "run help for commands"
- `claws help` → concise grouped commands (current, but shorter)
- `claws help setup` → onboarding guide
- `claws help security` → security guide
- `claws help channels` → channel guide
- `claws <cmd> --help` → command-specific (already works)

**Files:** `main.go`, `help.go`
**Effort:** Medium

---

### 8.7 — `claws team create` shortcut

**Current:** `group create` + `group shared --all` = 2 commands.
**After:** `team create <name>` does both in one.

**Files:** `main.go`, `group.go` or new alias
**Effort:** Small

```bash
claws team create research
# = claws group create research + claws group shared research --all
```

---

## Testing Plan

| Test | Validates |
|------|-----------|
| `init` creates policy.json | 8.1 |
| `init` creates .access.json | 8.1 |
| `init` is idempotent (doesn't overwrite existing) | 8.1 |
| First-run message when no OPENCLAW_ROOT | 8.2 |
| Brief status when initialized | 8.2 |
| `setup --non-interactive --team=t --agent=a` creates team + agent | 8.3 |
| `setup` is idempotent | 8.3 |
| New instance has tools.profile=coding | 8.4 |
| `claws status` shows all agents with health | 8.5 |
| `claws help` is shorter than current | 8.6 |
| `claws team create` creates group + shared | 8.7 |

---

## Implementation Order

1. **8.1** (init creates policy+access) — unblocks everything else
2. **8.4** (default tool profile) — trivial, reduces audit noise
3. **8.2** (first-run detection) — changes what new users see
4. **8.7** (team create shortcut) — small win
5. **8.5** (unified status) — one-screen overview
6. **8.6** (tiered help) — reduces information overload
7. **8.3** (claws setup) — the big one, needs all above working first
