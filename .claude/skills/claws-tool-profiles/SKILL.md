---
name: claws-tool-profiles
description: Choose and assign a `tools.profile` (or build a custom `tools.alsoAllow` list) for a claws agent — including the specific case of creating a "sudo admin / power-user / ops" agent in a fleet of otherwise-restricted workers. Use whenever the user asks any variant of "what profile should I use for this agent", "create a sudo admin agent in claws", "create a power-user / admin / privileged / high-access claws agent", "give this agent more access" / "loosen restrictions on team/X", "the coding profile is too restrictive", "how do I let this agent download / install / curl / wget / run sudo / install packages", "what tools profiles does claws expose", "what's the difference between the coding profile and \<other>", "claws agent that can install packages", or "make team/X able to do \<thing the coding profile blocks>". Walks through the `tools.profile` vs `tools.alsoAllow` data model, how to **discover** which profiles the running openclaw runtime image actually exposes (because profile names are runtime-defined, not claws-CLI-defined), and the explicit-allowlist fallback when no curated profile matches. Encodes the host-vs-container boundary (sudo inside the container is NOT host sudo) and the per-agent OAuth requirement that applies even more strongly to high-access agents. Do NOT use this skill for the generic "add an agent" flow (use claws-add-agent), fixing a broken agent (use claws-debug-agent), conceptual team design (use claws-teams-architecture), auditing existing posture (use claws-security-audit), fresh-host install (use claws-bootstrap-fresh-box), or cutting a release (use claws-release).
---

# claws — choosing a tool profile (and building a high-access agent)

This skill is for **designing the capability surface** of a claws agent at create time. Sibling skill `claws-add-agent` covers the mechanical create→auth→channel→start flow for a vanilla agent; **this** skill is the right one when the question is "what tools should this agent have access to" rather than "how do I add an agent in general". The motivating case is "add an `ops` / `admin` / `power-user` agent to a fleet where the other workers run the restrictive `coding` profile" — but the same logic applies any time the user wants to deviate from defaults.

If claws itself isn't installed yet, hand off to `claws-bootstrap-fresh-box`. If the agent already exists and is broken, hand off to `claws-debug-agent`. If they're auditing posture across the fleet, hand off to `claws-security-audit`.

## Mental model — the `tools` block

Every agent's `~/.openclaw/<team>/<name>/openclaw.json` carries a `tools` object that defines its capability surface:

```json
{
  "tools": {
    "profile": "coding",
    "alsoAllow": []
  }
}
```

Two knobs:

- **`profile`** — name of a curated tool set defined **inside the openclaw runtime image**, not in the claws CLI. `"coding"` is the canonical default for production worker agents on the v1.6.x line — it lets the agent edit files and run code in a sandbox but deliberately omits outbound network beyond the model API, privileged shell (sudo / package install), and arbitrary process spawning outside a small allowlist. Other profile names may exist in newer runtime images, but they're **runtime-version-specific** — what's available on `openclaw:local` from three months ago is not necessarily what's available on the upstream image today.
- **`alsoAllow`** — extra tool names ADDED on top of whatever the profile grants. List of strings (tool names). Use it to lift a single restriction off an otherwise-good profile (e.g. `coding` + one explicit `fetch`).

The **escape hatch** is `profile: null` (or omit it) plus an explicit `alsoAllow: [...]` that lists every tool you want. More verbose, but stable across runtime upgrades — the profile-name approach is at the mercy of whoever maintains the runtime's profile registry.

A real example, as deployed on this host (`~/.openclaw/team/sarah/openclaw.json`):

```json
"tools": { "profile": "coding", "alsoAllow": [] }
```

That's the shape every existing agent on this fleet uses. The job of this skill is to know when to keep that shape, when to extend it via `alsoAllow`, and when to throw the profile out entirely.

## Playbook

Run in order. The first three steps are about deciding what to set; the rest are about applying it correctly.

### Step 1 — Decide whether a profile change is even needed

Skip if the user has explicitly said "I want a power-user agent" / "high-access agent" / "an agent that can run sudo / curl / install things" — they've already decided. Otherwise:

| User intent | Action |
|---|---|
| "Add a normal worker agent" / nothing about restrictions | Don't touch `tools`. Use `claws-add-agent`. Defaults are right. |
| "The `coding` profile is too restrictive for X" | Try `alsoAllow: ["<X-tool>"]` first — keeps the sandbox, adds the one thing. Often enough. |
| "I want a power-user / admin / ops / unrestricted agent" | Replace the profile entirely. Go to Step 2. |
| "I need host-level sudo" | **STOP.** That's a runtime change, not a config change. See Step 4. |

### Step 2 — Discover what profiles the running runtime actually exposes

Profile names live inside the openclaw container image, not the claws CLI source. So the profiles available depend on which runtime image is running here, not which `claws` binary version is installed. Run these probes in order — the first one that returns useful output wins:

```bash
# Pick any running agent's container as the probe target.
# Containers are named: openclaw-<team>-<name>-openclaw-gateway-1
CONTAINER=$(docker ps --format '{{.Names}}' | grep openclaw-gateway-1 | head -1)

# Probe 1 — file-based profile registry (common shape)
docker exec "$CONTAINER" ls /home/node/.openclaw/profiles 2>/dev/null

# Probe 2 — runtime CLI (if it ships one)
docker exec "$CONTAINER" openclaw tools list-profiles 2>/dev/null

# Probe 3 — JSON registry
docker exec "$CONTAINER" cat /home/node/.openclaw/profiles.json 2>/dev/null
```

What you're looking for: the names you can paste into `tools.profile`. Plausible names in newer images: `coding`, `power-user`, `power`, `unrestricted`, `engineer`, `ops`, `admin`. Do not assume any of these exist without confirmation from one of the probes.

**If none of the probes return anything useful**, the runtime doesn't enumerate its profiles in a discoverable place — fall through to the explicit-allowlist approach (Step 3, Option B). That's the safer default anyway.

### Step 3 — Pick: profile name vs explicit allowlist

Two paths. Pick based on what Step 2 surfaced and how stability-sensitive this agent is.

**Option A — Name a curated profile** (fastest, most fragile):

Use when Step 2 surfaced a profile whose intent matches what the user wants ("there's a `power-user` profile — yes, use that").

- Pro: one-line config; whatever the runtime considers the right tool set for that role.
- Con: the profile's contents can change between runtime image versions. A `claws upgrade <name> --image=...` later could silently widen or narrow what the agent can do.
- Apply with: `claws exec <team>/<name> config set tools.profile '"<profile-name>"' --json`.

**Option B — Explicit `alsoAllow`, no profile** (verbose, stable):

Use when Step 2 found no matching profile, or when the user wants precise control, or when this is the "sudo admin" case and you want to be deliberate about exactly which capabilities are granted.

- Pro: stable across runtime upgrades; the on-disk config documents the exact capability surface.
- Con: verbose; you have to know the tool names the runtime recognises.
- Apply with: `claws exec <team>/<name> config set tools.profile null --json` then `claws exec <team>/<name> config set tools.alsoAllow '[...]' --json`.

For the high-access / sudo-admin / ops-agent case **prefer Option B unless a profile clearly fits** — being explicit about a privileged capability surface is good hygiene.

### Step 4 — For the high-access case specifically: what to grant

Tool **categories** typically wanted for a power-user agent (the specific names are runtime-defined — verify against Step 2's output or the runtime docs; do not hardcode):

| Capability | Likely tool name(s) | Why |
|---|---|---|
| Arbitrary shell | `bash`, `shell` | Run any command available inside the container |
| Outbound HTTP fetch | `fetch`, `curl`, `download` | Pull tarballs / docs / scripts beyond the model API |
| Write to workspace | `fs.write`, `file.write` | Persist downloaded artifacts to the agent's dir |
| Package install | `apt`, `dnf`, `pacman`, `pip`, `npm` | Install runtime dependencies (which apt verb depends on base image) |
| Spawn processes | `process.spawn`, `exec` | Long-running children outside the allowlist |

**NOT usually grantable via `tools.alsoAllow`:**

- `sudo` / host-level escalation — the container runs as a **non-root user inside its own container**; sudo inside the container does not reach host sudo. `/etc/sudoers` on the host is invisible to the agent.
- `host.exec` / docker.sock — would require the container to have the docker socket mounted, which is a runtime change (`docker-compose.override.yml` + restart with `--hard`), not a config change.
- `--privileged` capabilities (`CAP_SYS_ADMIN`, etc.) — same story; runtime change.

> **Host-vs-container boundary — flag this loudly to the user.** If the actual use case needs host-level capability (modify `/etc/something` on the host, restart a systemd unit, manage other docker containers, etc.), that is a **runtime change with major security implications** — mounting host `/`, adding `--privileged`, mounting `docker.sock` — and warrants a separate ticket, not a `tools.alsoAllow` tweak. The `tools` block governs what the agent can do **inside its own container only.**

If the user is OK with "powerful inside the container, no host reach," `tools.alsoAllow` is the right tool. If they need host reach, stop here and surface it as a deeper change.

### Step 5 — Apply the profile (full create sequence)

Mechanical sequence. This assumes claws is installed and prereqs are satisfied — see `claws-bootstrap-fresh-box` if not, and `claws-add-agent` for the parts of this flow that are not profile-specific.

```bash
# 1. Create the instance — allocates port, generates gateway token, writes config.
claws create <team>/<name>

# 2. Set the tool surface.
#    Option A — name a profile (verify the name from Step 2 first):
claws exec <team>/<name> config set tools.profile '"<profile-name>"' --json

#    Option B — explicit allowlist, no profile:
claws exec <team>/<name> config set tools.profile null --json
claws exec <team>/<name> config set tools.alsoAllow '["bash","fetch","apt","fs.write"]' --json

# 3. Per-agent OAuth — REQUIRED if multiple agents in the fleet use Codex.
#    Do NOT copy another agent's grant; that triggers refresh_token_reused.
#    (See claws-debug-agent for the collision class.)
claws auth <team>/<name> codex

# 4. Channel — only if this agent should talk to a human directly.
claws channel add <team>/<name> telegram --token=<botfather-token>
claws exec <team>/<name> config set channels.telegram.dmPolicy '"pairing"' --json

# 5. Start + verify.
claws start <team>/<name>
claws agent ping <team>/<name>
```

JSON quoting matters:

- `tools.profile` is a **string** — `'"name"'` (single-quoted shell wrapping double-quoted JSON), or `null` for "no profile".
- `tools.alsoAllow` is an **array of strings** — `'["a","b","c"]'`.

`claws exec <agent> config set <path> <json-value> --json` is the supported way to edit a running agent's config without restart-loop churn. There is **no** top-level `claws config set` and no `--tools` flag on `claws create` — don't invent either; the only way in is `claws exec ... config set`.

### Step 6 — Verify the profile actually does what you intended

Profile/allowlist contents are runtime-defined, so the only honest test is to ask the agent to do the thing you expected to be allowed and see whether the runtime blocks it:

```bash
# Simple capability probe — adjust to whatever the agent should be able to do.
claws exec <team>/<name> tools test fetch https://example.com
# Or DM the agent on its channel and ask it to perform the action.
```

If the runtime returns a permission / "tool not allowed" error, the profile did not include that tool. Go back to Step 3:

- If you used Option A, either switch to a different profile (re-probe Step 2) or extend with `alsoAllow` adding the missing tool name.
- If you used Option B, add the missing tool name to the `alsoAllow` list.

Restart the agent (`claws restart <team>/<name>`) only if the runtime needs it — most `tools` changes are picked up on the next request, but some runtime versions cache. If a tool change doesn't seem to take effect, `claws restart` is the cheap diagnostic.

### Step 7 — Audit the change

Host policy.json has `auditLog: true` by default, which means every command the new agent issues lands in `~/.openclaw/.audit.log`. For a higher-access agent this is exactly the file you want to watch:

```bash
claws access audit --since=24h --group=<team>     # focused on the team
claws access tail -f                              # live tail during smoke-testing
```

Recommend the operator review the audit slice **daily for the first week** after deploying a higher-access agent, and longer if the agent is doing genuinely sensitive work. The audit log is append-only JSONL; `claws-security-audit` has the `jq` cookbook for slicing it.

## Critical context — do not skip

### 1. The CLI verb is `claws exec <agent> config set <path> <json-value> --json`

There is no `claws config set`. There is no `--tools` / `--profile` flag on `claws create`. The only supported way to edit `tools.profile` or `tools.alsoAllow` after create is `claws exec <team>/<name> config set tools.profile '"name"' --json`. Do not invent alternative syntax; the user will copy/paste exactly what you suggest.

### 2. Per-agent OAuth applies even more strongly to high-access agents

Codex OAuth refresh tokens are single-use. Two agents that share a grant both die on `refresh_token_reused` the first time one refreshes — the high-access agent included. **Every agent in the fleet, especially the high-access one, needs its own OAuth grant.** `claws auth <team>/<name> codex`. Fleet-wide reauthing of every unauth'd agent: `claws auth fleet codex --missing-only`. The collision class is documented in `claws-debug-agent`.

### 3. Host policy caps apply to ALL agents, including the privileged one

`~/.openclaw/policy.json` caps `memoryLimitMB` (commonly 1024 or 2048), `cpuLimit` (commonly 2.0), and `maxInstances` (commonly 8). These apply to the high-access agent too — having `bash` and `apt` in `alsoAllow` doesn't let the agent ignore its RAM ceiling. Don't loosen the host policy to give one agent more resources — that loosens it for everyone. Use per-instance `meta.policyOverrides` if the runtime supports it, or accept the cap. (Loosening the host policy for one agent is the most common security regression in claws-fleet hardening; reject the temptation.)

### 4. Sudo inside the container ≠ host sudo

The agent runs as a non-root user inside its container. The container's `/etc/sudoers` is the container's, not the host's. Adding `sudo` (or whatever the runtime's privileged-shell tool is called) to `alsoAllow` does NOT give the agent any reach onto the host's filesystem or processes — the host is invisible to it. If the use case genuinely needs host-level capability, that requires mounting host `/`, adding `--privileged`, or mounting the docker socket — all runtime-level changes with real attack-surface implications. File a separate ticket, do not paper over it in `tools.alsoAllow`.

### 5. `team/shared/` is the right channel for "admin fetches, workers consume"

Every team agent mounts `~/.openclaw/<team>/shared/` RW (mapped to `/home/node/.openclaw/shared` in-container). The high-access agent can `fetch` a binary or download a dataset and write it to `shared/output/`, then the restricted workers in the same team can read from there without ever needing the fetch capability themselves. This is the canonical pattern for "give the admin the privilege; let the workers consume the result" — it keeps the privileged surface to one agent and lets the rest stay on the conservative `coding` profile. See `claws-teams-architecture` for the full shared-directory model.

### 6. DM pairing matters MORE for high-access agents, not less

It can be tempting to switch `channels.telegram.dmPolicy` from `"pairing"` to `"open"` because "I'm the only one who'll DM the admin bot". Don't — `dmPolicy: "open"` means anyone who finds the bot's t.me link can DM it, and on a high-access agent that means anyone can drive privileged commands. Keep `dmPolicy: "pairing"` on, complete the pairing handshake from your own Telegram client, and (optionally) populate `channels.telegram.allowFrom` with your Telegram user ID for belt-and-braces. `claws-security-audit` has the full set of channel-security defaults.

### 7. `alsoAllow` adds to the profile; it never subtracts

`tools.alsoAllow` is additive only — it can ADD tools on top of the profile's set, never REMOVE them. If a curated profile grants something you don't want, the way to remove it is to drop the profile (`profile: null`) and rebuild the surface via `alsoAllow` listing only the tools you do want. There is no `tools.deny` mechanism in the v1.6.x line.

## Decision shortcuts

- User says "the coding profile is too restrictive for one thing" → keep `profile: "coding"`, add the one tool to `alsoAllow`. Don't replace the profile.
- User says "I want a sudo admin / power-user agent" → Step 2 probe the runtime; if a `power-user`-class profile exists, use it; if not, Step 3 Option B with an explicit `alsoAllow` list.
- User says "I need this agent to run sudo on the host" → STOP. Surface the host-vs-container boundary; that's a runtime change, separate ticket.
- User says "what's the difference between `coding` and `<other>`" → Step 2 probe the runtime; the answer lives in `/home/node/.openclaw/profiles/<name>` inside the container, not in claws.
- User wants the admin agent to fetch + restricted workers to consume → admin writes to `team/shared/output/`, workers read from there. No need to widen the workers' profile.
- User asks for the admin agent's DM channel to be `open` so it's easier to reach → push back; keep `pairing`, surface them adding their Telegram ID to `allowFrom` if they want belt-and-braces.

## What success looks like

After the playbook:

1. `claws list` shows the new agent `healthy` on its allocated port.
2. `claws agent ping <team>/<name>` returns all four green checks.
3. `claws exec <team>/<name> config get tools` returns the profile + allowlist you intended.
4. A simple capability probe (Step 6) confirms the agent can do at least one thing the restricted workers cannot.
5. `claws auth diagnose` shows no shared-account risk for this agent (its OAuth grant is its own).
6. `~/.openclaw/.audit.log` shows the new agent's first commands attributed to it.

If all six are true, the high-access agent is deployed and observable. Continue to monitor the audit log daily for the first week.

## Out of scope — redirect to sibling skills

- "Add another agent in general" / mechanical create→auth→channel→start flow without a profile angle → **`claws-add-agent`**. That's the right one when the question isn't about capability surface.
- "How do teams work" / "what's in `shared/`" / designing the team shape → **`claws-teams-architecture`**. That's the conceptual reference; this skill is about ONE agent's capability surface within an already-designed team.
- "The agent is broken / silent / 401-looping" → **`claws-debug-agent`**. That's for fixing broken agents; this skill is for designing the access surface at create time.
- "Audit my fleet's security posture" / `claws audit` interpretation → **`claws-security-audit`**. That's for posture review across the fleet; this skill is for choosing posture for one agent.
- "Install claws on a fresh box" → **`claws-bootstrap-fresh-box`**. This skill assumes claws + runtime image are already present.
- "Cut a claws release" → **`claws-release`**. Unrelated.
