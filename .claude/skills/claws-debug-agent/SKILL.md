---
name: claws-debug-agent
description: Systematically diagnose a misbehaving claws agent — silent on its channel, "started but not responding", refresh-token / OAuth errors, port conflicts, false health warnings, container in `created` / `stopped` / `missing` state. Use whenever a user describes a broken claws agent in any of these shapes — "my agent isn't responding", "[name] won't reply on telegram / whatsapp / discord / slack", "claws agent is broken", "claws fleet is acting up", "team/sarah stopped working", "auth keeps failing on [agent]", "[agent] says refresh_token_reused", "port already allocated when starting claws", "claws health says unreachable but docker says healthy", "debug claws agent", "fix my claws bot". Walks a five-step playbook (`list` → `agent ping` → `auth diagnose` → `logs` → `errors`) and maps observed symptoms to the exact remediation command. NOT for fresh-box setup (use claws-bootstrap-fresh-box), adding a new agent (use claws-add-agent), or cutting a release (use claws-release).
---

# claws-debug-agent

Diagnose a misbehaving claws agent on this server. Every step ends with a concrete next command; do not skip steps unless the previous step already pinned the root cause.

## The diagnostic always works the same way

1. `claws list` — what state is the fleet in?
2. `claws agent ping <name>` — the one-screen verdict for the affected agent.
3. `claws auth diagnose` — fleet-wide auth health + risk heuristics.
4. `claws logs <name>` — grep for the well-known error strings.
5. `claws errors` — the incident-triage aggregate view.

Run them in order. Each step's output narrows what the next step needs to look at. Most cases resolve at step 2 or 3.

Use the real binary, not `go run`: invoke `./clawctl` (or `claws` if installed on PATH). Live agents on this host live under `~/.openclaw/`; never mutate those directories directly — only via claws subcommands.

## Step 1 — `claws list`

```
$ claws list
NAME               PORT     STATUS       RAM        UPTIME     NEXT
team1/ben          :18989   healthy      130.2MiB   10 hours   claws agent ping team1/ben
team/sarah         :18789   created      —          —          claws start team/sarah
team/john          :18889   stopped      —          —          claws logs team/john
```

The `STATUS` column on the affected row is the first branch point. The `NEXT` column already tells the user the next move; if it does, run that first.

| STATUS     | What it means                                 | First action                                       |
|------------|-----------------------------------------------|----------------------------------------------------|
| `healthy`  | Gateway container reports healthy             | Issue is downstream — go to step 2 (`agent ping`)  |
| `created`  | Never started since `claws create`            | `claws start <name>` then ping; usually not broken |
| `stopped`  | Was running, exited                           | `claws logs <name>` for the exit cause, then start |
| `missing`  | Registry says it exists; container does not   | `claws orphans` then `claws create <name>` to redo |
| stuck `starting` forever | Health probe never went green     | `claws logs <name>` — usually init script in image |

If status is `created`, the problem is "never started," not "broken." Do not dive into auth/channel debugging until you've actually started it.

## Step 2 — `claws agent ping <name>`

This is the one-screen verdict. It checks four things in order; the first failing one is the root cause.

```
$ claws agent ping team/sarah
  ✓ gateway:   container reports healthy on :18789
  ✓ readyz:    /readyz 200 — agent ready to receive
  ✓ auth:      verified via logs strategy
  ✓ channels:  2 configured: telegram, whatsapp
✓ team/sarah looks healthy
```

| Failing check | Meaning                                                          | Fix                                                                 |
|---------------|------------------------------------------------------------------|---------------------------------------------------------------------|
| `gateway` ✗   | Container down / unhealthy per Docker HEALTHCHECK                 | `claws start <name>`; if it won't start, `claws logs <name>`        |
| `readyz` ✗    | Gateway up but `/readyz` not 200 — model or channel not wired     | Inspect `/readyz` body's `failing[]` list; usually a channel        |
| `auth` ✗      | `verifyOneInstance` (logs / readyz strategy) shows broken auth    | Go to step 3 (`claws auth diagnose`); usually re-auth this agent    |
| `channels` ✗  | No channel configured for the agent                               | `claws channel add <name> <type>` (telegram / discord / slack / whatsapp) |

If `ping` is fully green but the user still reports "agent isn't replying," the issue is downstream of claws — channel-side (Telegram bot token muted, WhatsApp session expired between checks, Discord intents). Tail logs (step 4) while the user sends a real message and watch what arrives.

### If you see `gateway unreachable` on an agent with port ≠ 18789

This was the v1.6.15 bug (probe-from-wrong-namespace — host-side HTTP probe of a container-internal-only port). If the user is on a version older than v1.6.15, the diagnostic itself is lying. Tell them: `claws update`, then re-run `claws agent ping`. Check version with `claws version`.

## Step 3 — `claws auth diagnose`

Fleet-wide auth state. Read-only. Aggregates audit log + per-agent verify + provider config.

```
NAME          PROVIDER       VERIFY                 LAST AUTH   REMEDIATION
team1/ben     openai-codex   ✓ logs                 3d ago      —
team/sarah    openai-codex   ✓ logs                 1d ago      —
team/john     openai-codex   ✗ refresh_token_reused 1d ago      claws auth team/john codex
team/lead     openai-codex   ? no recent activity   1d ago      claws agent ping team/lead

Risk signals:
  ⚠ 3 agents authed within 8m for openai-codex (...).
    If they share an upstream account, refresh_token_reused will recur.
    Each agent should have its own OAuth grant:
      claws auth fleet codex --missing-only
```

Two risk heuristics auto-fire:

- **Bunched auth events** — ≥2 agents authed within 15 min for the same provider → they may share an upstream account, in which case `refresh_token_reused` is a future-tense problem.
- **Confirmed reuse** — ≥2 agents currently failing with `refresh_token_reused`-class errors → collision is already happening; re-auth each one independently via `claws auth fleet codex`.

Follow the `REMEDIATION` column for any non-green row. The command shown there is the fix; do not invent your own.

## Step 4 — `claws logs <name>`

For a stopped agent or one whose `ping` showed `auth ✗` / `readyz ✗`, read the recent log lines:

```bash
claws logs <name>                # recent lines
claws logs <name> -f             # tail
claws logs <name> --grep=<pat>   # filter
```

Map the grep hit to the fix. **Do not guess; match the string.**

| Log pattern                                                       | Class                          | Fix                                                                                       |
|-------------------------------------------------------------------|--------------------------------|-------------------------------------------------------------------------------------------|
| `refresh_token_reused` / `Token refresh failed: 401`              | OAuth refresh chain collision  | `claws auth <name> codex` to re-auth independently. Multiple agents hit? `claws auth fleet codex`. See [references/oauth-collision.md](references/oauth-collision.md). |
| `401 Unauthorized` from WhatsApp                                  | WhatsApp session expired       | Runtime auto-restarts in a loop; needs QR re-pair: `claws exec <name> channels login --channel whatsapp` |
| `port is already allocated`                                       | Orphan container holds the port| `claws orphans` to list → `claws orphans clean <container>` to remove. On v1.6.11+ this should be caught by start-time preflight; if you see it on ≥v1.6.11 the orphan was created after preflight ran. |
| `Health check didn't pass in 30s` while `docker inspect` says healthy | Pre-v1.6.12 false alarm    | `claws update` to v1.6.12+. The host HTTP probe is lying about the container-internal gateway. |
| Init script errors during `starting`                              | Runtime image bug              | Image-side; capture lines and check runtime image version                                  |

For oauth-class errors the operator history matters (single-account vs per-agent grants); read references/oauth-collision.md before recommending a fix beyond one re-auth.

## Step 5 — `claws errors`

Final incident-triage view. Aggregates container state + log errors + audit errors + orphans into one screen. Run this when:

- Multiple agents are misbehaving simultaneously (fleet-level event)
- You've fixed what step 2-4 surfaced and want to confirm no other site is broken
- The user can't articulate which agent is broken — `errors` lists them all

## Decision tree (combined)

```
claws list shows the affected agent as...
├── created                       → claws start <name>; verify with claws agent ping <name>
├── stopped                       → claws logs <name>; address exit cause; claws start <name>
├── healthy + ping: auth ✗        → claws auth diagnose; re-auth via claws auth <name> codex
│                                  (if multiple agents fail same way: claws auth fleet codex)
├── healthy + ping: readyz ✗      → inspect /readyz failing[]; usually channel-side
├── healthy + ping: channels ✗    → claws channel add <name> <type>
├── healthy + ping ✓ but silent   → tail logs while user sends a real message; channel-side
├── missing                       → claws orphans; claws create <name> to recreate
└── starting (stuck)              → claws logs <name>; usually image init script broken
```

## Version-gated bugs to rule out first

If the user is on an old build, the diagnostic itself can lie. Check `claws version` early:

| Symptom                                                          | First seen fixed in | Action                                |
|------------------------------------------------------------------|---------------------|---------------------------------------|
| `port is already allocated` with no preflight warning            | v1.6.11             | `claws update`, then `claws orphans`  |
| `claws start` falsely says "Health check didn't pass in 30s"     | v1.6.12             | `claws update`, retry start           |
| `claws start` doesn't catch broken OAuth at startup              | v1.6.13             | `claws update` to get auth verify-on-start |
| Need fleet-wide independent re-auth in one command               | v1.6.14             | `claws update` to get `claws auth fleet codex` |
| `agent ping` / `health` say unreachable on any port ≠ 18789      | v1.6.15             | `claws update`, then re-ping          |

Anything reported as broken on ≥v1.6.15 is most likely a real agent-side issue, not a diagnostic-side bug.

## Sibling skills — don't trigger here for these

- **Fresh-box / first-time install** → `claws-bootstrap-fresh-box`
- **Adding a new agent to an existing fleet** → `claws-add-agent`
- **Cutting a claws release** → `claws-release`

This skill is only for diagnosing an already-existing agent that is misbehaving.
