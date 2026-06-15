---
name: claws-security-audit
description: Audit and harden a claws deployment — runs and interprets `claws audit`, `claws doctor`, `claws policy validate/enforce`, the comprehensive `scripts/security-audit.sh`, plus the access-control and audit-log subsystems. Use whenever a user asks any variant of "audit my claws security", "check claws security settings", "review claws permissions / access", "harden my claws install" / "harden claws", "what does claws audit show", "set up security policy" / "configure claws policy", "claws policy enforce", "who can run claws", "rotate claws tokens", "what's in the claws audit log", "is my claws deployment secure", or "explain claws security model". Walks the operator through interpreting findings (file permissions, channel `dmPolicy` / `allowFrom`, bind modes, gateway-token rotation, OAuth/channel auth state) and applying remediations (`claws doctor --fix`, `claws policy enforce --restart`, `claws token rotate`, `claws access grant/revoke`). Encodes the v1.6.x channel-security defaults, the loopback-only gateway binding, and the "policy enforce REWRITES per-instance configs" warning. Do NOT use this skill for fresh-box install (use claws-bootstrap-fresh-box), adding agents (use claws-add-agent), debugging a broken agent's runtime health (use claws-debug-agent), or cutting a claws release (use claws-release).
---

# claws-security-audit

Audit a claws deployment for security and walk the operator from findings to remediation. This skill is for **understanding posture and hardening**, not for diagnosing a single broken agent (that's `claws-debug-agent`). If claws itself isn't installed, hand off to `claws-bootstrap-fresh-box`.

## Mental model

claws holds three classes of secret on this host:

1. **Gateway tokens** — per-instance shared secret between the runtime container and the host. Lives in `~/.openclaw/<team>/<agent>/instance.env` as `OPENCLAW_GATEWAY_TOKEN`. Whoever reads this can drive the agent's WebSocket gateway.
2. **Model credentials** — Codex OAuth refresh tokens, OpenAI / Anthropic / OpenRouter API keys. Per-agent (each agent gets an independent OAuth grant so multi-agent setups don't `refresh_token_reused`).
3. **Channel credentials** — Telegram bot tokens, WhatsApp session keys, Discord/Slack tokens. Live under `~/.openclaw/<team>/<agent>/credentials/`.

All three must be mode `0600`, and their containing dirs `0700`. The defense is layered:

- **File permissions** → `claws doctor --fix` (idempotent chmod).
- **Per-instance config sanity** (loopback bind, `dmPolicy: pairing`, `allowFrom` populated for outbound) → `claws policy enforce --restart`.
- **Who-can-run-what on the host** → `claws access` (admin / operator / user roles).
- **What ran when, by whom** → `~/.openclaw/.audit.log` (append-only JSONL).
- **Deeper container-level checks** (`cap_drop`, `no-new-privileges`, memory limits, docker-socket exposure, public-port scan, firewall presence) → `scripts/security-audit.sh`.

A "secure deployment" satisfies all five.

## When to use which command

| Question | Command |
|---|---|
| Quick "how am I doing right now?" | `claws audit` |
| Quick "any of my files world-readable?" + auto-fix | `claws doctor --fix` |
| "Does every instance comply with admin policy?" (read-only) | `claws policy validate` |
| "Rewrite every instance to comply with admin policy" | `claws policy enforce --restart` |
| Deep container + network + image hardening review | `bash scripts/security-audit.sh ~/.openclaw \| tee audit-$(date +%F).log` |
| "Who has access to claws on this host?" | `claws access show` |
| "What ran in the last 24h and who ran it?" | `claws access audit --since=24h` |
| "Tail the audit log live" (during incident triage) | `claws access tail -f` |
| "Rotate a possibly-leaked gateway token" | `claws token rotate <name>` |
| "Are all agents still model-authenticated?" | `claws auth diagnose` |

Use the real binary — `claws` if installed on PATH, otherwise `./clawctl` in the repo dir. Live agents on this host live under `~/.openclaw/`; never edit files there by hand, only via claws subcommands. `claws audit` and the script are read-only by default; `policy enforce` and `token rotate` mutate state.

## Playbook

Run in order. Each step's output narrows what the next step needs.

### Step 1 — Run the umbrella check

```bash
claws audit
```

`claws audit` prefers the bundled `scripts/security-audit.sh` if present and falls back to a minimal built-in (`doctor` + `policy validate`). The aggregate covers:

- File-permission audit (`~/.openclaw/`, instance dirs, credential dirs — must be `0700` / `0600`)
- Policy validation (every registered instance against the active policy)
- Access-control state (who has which role)
- Recent audit log entries (failed commands, denied accesses)
- Token rotation age
- Channel auth state per agent

Useful flags:

- `claws audit --since=24h` — narrow audit-log slice (default is 24h already, but explicit is good).
- `claws audit --json` — machine-readable for piping into a dashboard / ticket.

**Read the summary line first.** If it's `0 failures`, you're at "no urgent action." Warnings (yellow) are hardening recommendations; failures (red) need same-day remediation.

### Step 2 — Environment-level: `claws doctor`

```bash
claws doctor             # read-only diagnosis
claws doctor --fix       # safe, idempotent: chmod 0700/0600 anything wrong
```

`claws doctor` covers what the audit can't fix automatically:

- Docker daemon running, version, compose v2 plugin present.
- File permissions on `~/.openclaw/` and per-instance dirs.
- Image presence (`openclaw:local` or `$OPENCLAW_IMAGE`).
- Disk space, free ports.
- Tool presence (curl, tar, `ss` for diagnostics).

`--fix` only adjusts file permissions (chmod 0700 on dirs, 0600 on `instance.env` and credentials). It never restarts containers, never rewrites configs, never touches docker. Safe to run on a healthy system — exits 0 if nothing needed fixing.

If `claws doctor --fix` reports it changed permissions on files an instance is currently using, no restart needed — the container reads them at start.

### Step 3 — Understand the policy system

The policy is `~/.openclaw/policy.json`. It is the **floor** every instance must clear; per-instance overrides can be stricter but never looser.

```bash
claws policy show                  # current policy (or "no policy" if absent)
claws policy init                  # write secure defaults (refuses if exists; --force overrides)
claws policy validate              # check every instance, list violations (read-only)
claws policy validate --json       # machine-readable
claws policy validate --group=team # narrow to one group
claws policy enforce               # rewrite per-instance configs to comply (changes disk)
claws policy enforce --restart     # also `docker compose down/up` so changes take effect
```

**Secure defaults** that `claws policy init` writes:

| Field | Default | Why |
|---|---|---|
| `allowedBindModes` | `["loopback"]` | Gateway port bound to `127.0.0.1` only — access via `claws tunnel` or SSH tunnel. |
| `maxInstances` | `8` | Caps fleet size; prevents accidental fork-bomb. Bump if you actually need more. |
| `memoryLimitMB` | `2048` | Per-container RAM cap. |
| `cpuLimit` | `2.0` | Per-container CPU cap. |
| `allowDockerSocket` | `false` | No agent gets docker.sock unless explicitly turned on (sandbox-mode only). |
| `requireDmPairing` | `true` | New DMs need an auth code before the agent will reply. |
| `requireOutboundAllowlist` | `true` | If a channel enables `sendMessage`, it MUST have a non-empty `allowFrom`. |
| `allowedImages` | `["openclaw:*"]` | Glob allowlist — rejects images outside the `openclaw` namespace. |
| `auditLog` | `true` | Turns on JSONL logging to `~/.openclaw/.audit.log`. |

**Critical: `claws policy enforce` rewrites per-instance `openclaw.json` and `instance.env`.** If the operator hand-edited an instance config to (e.g.) point at a different image or open a channel, enforce will overwrite that. Always run `claws policy validate` first, read the violation list, and confirm the rewrites are wanted before running enforce.

The audit log captures every `policy enforce` invocation, so reverts are traceable.

### Step 4 — Understand the access-control system

`~/.openclaw/.access.json` defines who on this host can run which claws commands.

```bash
claws access show                                      # current grants
claws access init                                      # bootstrap; running user becomes admin
claws access grant <user> <admin|operator|user>        # promote/demote
claws access revoke <user>                             # remove
claws access audit --since=1h                          # who ran what in the last hour
claws access audit --since=24h --group=team            # narrow to one group
claws access tail -f                                   # live tail of the audit log during triage
```

Three role tiers:

| Role | Can do | Cannot do |
|---|---|---|
| `admin` | Everything (commands `*`) | Nothing restricted — also the only role that can edit policy or access |
| `operator` | Lifecycle (`start`/`stop`/`restart`/`logs`/`exec`/`backup`), auth, channel ops, tunnels, status read | Cannot run `policy ...`, `access ...`, or arbitrary subcommands not in their allowlist |
| `user` | Read-only: `status`, `health`, `logs`, `list` | Cannot start/stop, cannot rotate, cannot grant |

Scoping to a specific instance is supported by editing `.access.json` and adding `"instances": ["team/alice"]` to the role — a `user`-roled person scoped to `team/alice` cannot even read logs of `team/bob`.

**Multi-tenant rule:** if multiple humans share this host, do NOT share the `admin` user account. Give each their own OS account, then `claws access grant alice operator` and so on. The audit log keys on `$USER` — without per-user OS accounts, you lose attribution.

### Step 5 — Read the audit log

`~/.openclaw/.audit.log` is append-only JSONL:

```json
{"ts":"2026-06-15T22:43:11Z","user":"alice","cmd":"start","args":["team/sarah"],"result":"ok"}
{"ts":"2026-06-15T22:43:19Z","user":"alice","cmd":"auth","args":["team/sarah","codex"],"result":"ok"}
{"ts":"2026-06-15T22:48:02Z","user":"bob","cmd":"policy","args":["enforce","--restart"],"result":"error"}
```

Fields: `ts` (UTC RFC3339), `user` (`$USER` of the caller), `cmd` (top-level claws command), `args` (slice), `result` (`ok` or `error`).

claws **never** rewrites this file. Rotate it from the outside (e.g., `logrotate`) if it grows — claws does not yet ship its own rotator. Recommend the operator add a `logrotate.d/claws` entry as a deployment step.

Useful queries (use `jq`, the log is JSONL):

```bash
jq 'select(.result == "error")' ~/.openclaw/.audit.log               # all failures
jq 'select(.user != "alice")' ~/.openclaw/.audit.log                 # who else has been running claws
jq 'select(.cmd == "policy")' ~/.openclaw/.audit.log                 # every policy change
jq 'select(.cmd == "token" and .args[0] == "rotate")' ~/.openclaw/.audit.log   # token rotations
jq -r 'select(.cmd == "auth") | "\(.ts) \(.args[0])"' ~/.openclaw/.audit.log   # when each agent was last (re)authed
```

The `auth` rows are also what `claws auth status` / `claws auth diagnose` use to drive their "bunched-auth" heuristic (multiple agents (re)authed within a short window can indicate a shared OAuth account — see `claws-debug-agent` for the refresh-token-collision story).

### Step 6 — Gateway-token rotation

Gateway tokens are per-instance secrets between the runtime container and host tools. They sit in `instance.env` as `OPENCLAW_GATEWAY_TOKEN`. Treat them like SSH host keys: rotate quarterly, or immediately on any suspected leak.

```bash
claws token show <name>            # truncated view (first/last 4 chars) — confirms current token
claws token show <name> --full     # full token, only when you actually need to paste it
claws token rotate <name>          # generate new, rewrite instance.env + openclaw.json, prompt to restart
claws token rotate --group=team    # fan out across a team (interactive confirm; --yes to skip)
```

Rotation prints both the old (truncated) and new (truncated) tokens so you can verify the change took. The runtime needs a restart (`claws restart <name>`) to pick up the new token — the rotate command will tell you.

**Recommended cadence:** every 90 days; after any "did I just leak a token?" moment; after revoking an operator who had read access to `instance.env`.

### Step 7 — Channel credential and auth state

Channel credentials (Telegram bot tokens, WhatsApp session keys) live in `~/.openclaw/<team>/<agent>/credentials/`. Must be `0600`.

```bash
claws auth status [name]    # what's configured per agent (provider, model, channel-cred count, last-auth time)
claws auth verify <name>    # actively verify model auth via log scan / readyz / endpoint probe
claws auth diagnose         # fleet-wide aggregate with risk heuristics (bunched-auth, refresh-token reuse)
```

`claws auth diagnose` is the security view of `claws-debug-agent`'s flow — same engine, but framed as "tell me what's risky" rather than "tell me what's broken." Run it after any policy enforce, after adding a new agent, and as part of the periodic audit.

If model auth fails or shows risk:

- One agent broken → `claws auth <name> codex` (or `... apikey`)
- Fleet-wide unverified → `claws auth fleet codex --missing-only` (each agent gets an independent OAuth grant)

### Step 8 — The comprehensive script

`scripts/security-audit.sh` runs deeper checks than `claws audit` aggregates today. Use this during deployment hardening (or after a major version upgrade), not every day. It's bash, prints human-readable PASS / WARN / FAIL, and exits non-zero on any FAIL.

```bash
bash scripts/security-audit.sh ~/.openclaw | tee audit-$(date +%F).log
```

Sections it covers:

1. **Secret file protection** — `instance.env` perms, `.port-registry` perms, every file under `credentials/`.
2. **Network exposure** — for every registered port: is it bound to `0.0.0.0`? does the host have a firewall (ufw / iptables) at all? Flags the "config says loopback but actual bind is wildcard" case (means restart needed).
3. **Container isolation** — for each running container: non-root user, no `--privileged`, `cap_drop: ALL`, `no-new-privileges`, memory limit set, no docker.sock mount, restart count, recent OOM kills, memory pressure.
4. **Agent authentication** — gateway token present and ≥32 chars; gateway HTTP returns 401/403 on unauthenticated request.
5. **Agent permissions** — sandbox mode on, tool profile set.
6. **Messaging security** — for each enabled channel: `dmPolicy: open` → FAIL; `dmPolicy: pairing` → PASS; `allowlist` with empty list → WARN.
7. **Docker image** — image present locally; image's default user is non-root.

Capture the output with `tee` if this is going into a hardening ticket — the script overwrites nothing.

### Step 9 — Map findings to fixes

Common findings and the exact remediation:

| Finding | Fix |
|---|---|
| `instance.env` is `0644` / credentials dir `0755` | `claws doctor --fix` |
| Bind mode is `wildcard` but policy says `loopback` | `claws policy enforce --restart` |
| `dmPolicy: open` on a Telegram/Discord channel | `claws policy enforce --restart` (sets to `pairing`), or `claws config set <name> channels.<ch>.dmPolicy '"pairing"'` |
| Channel has `sendMessage: true` but `allowFrom: []` | `claws policy enforce --restart` (disables `sendMessage` until allowlist populated) |
| Registry says instance exists, container doesn't | `claws drift` (preview), then `claws orphans clean` (sweep) |
| Container exists, registry doesn't list it | `claws orphans clean` |
| Token last rotated >90 days ago | `claws token rotate <name>` then `claws restart <name>` |
| Agent unverified by `auth diagnose` | `claws auth <name> codex` or `claws auth fleet codex --missing-only` |
| Operator no longer needs access | `claws access revoke <user>` |
| Container running as root / privileged / docker.sock mounted | Update `docker-compose.yml` overlay + `claws restart <name> --hard` (recreate container) |
| Host firewall absent | `ufw allow 22/tcp && ufw enable` (or your cloud-provider security group) — claws does not manage this |

### Step 10 — Persist what changed

After remediations:

1. `claws audit` again — confirm `0 failures`.
2. `claws access audit --since=1h` — verify every mutation is attributed to a real operator (no `unknown` user).
3. If running interactively for someone else, share the audit log slice as proof.

## Critical context (must include in any walkthrough)

- **`claws policy enforce` rewrites per-instance `openclaw.json` and `instance.env`.** Hand-edits to those files will be overwritten. Always run `claws policy validate` first and confirm.
- **`allowFrom` is the single most important per-channel control.** A channel with `dmPolicy: open` AND empty `allowFrom` AND `groupPolicy: open` means anyone who finds the bot can DM it. Set `allowFrom: [<sender-id>, ...]` OR set `dmPolicy: pairing` so new DMs require an auth code. The shipped policy defaults make this the only acceptable shape.
- **The gateway is loopback-only on purpose.** The runtime container exposes the gateway on its internal `127.0.0.1`; the host mapping is `127.0.0.1:<host>:18789`, never `0.0.0.0`. This is why claws's own probes use `containerHealth` / `containerProbe` (the v1.6.12 / v1.6.15 fixes), not external HTTP. If audit reports the port on `0.0.0.0`, that's a real misconfiguration — fix with `claws policy enforce --restart`.
- **The audit log can grow unbounded.** claws does not rotate it. Recommend a `/etc/logrotate.d/claws` entry as part of deployment hardening:
  ```
  /home/*/.openclaw/.audit.log {
      weekly
      rotate 12
      compress
      missingok
      notifempty
      copytruncate
  }
  ```
- **`claws update --check` reports new versions but does NOT install.** Install only happens via explicit `claws update`. The installer (and `claws update`) verify the published SHA256 against the release; never bypass that. Don't pipe install scripts you didn't read.
- **Multi-tenant: per-instance access + scoped grants are the way.** Do not share the `admin` OS account. Give each operator their own OS user and grant via `claws access grant`. The audit log keys on `$USER`.
- **Channel tokens (Telegram bot tokens, etc.) are bearer secrets.** If one leaks, rotate it at the channel provider (BotFather for Telegram, Discord developer portal, etc.) AND re-add to claws via `claws channel add <agent> <channel> --token=...`. claws cannot rotate them upstream for you.

## What this skill does NOT cover

- **Fresh install** → `claws-bootstrap-fresh-box`
- **Adding a new agent** → `claws-add-agent`
- **Diagnosing one broken agent's runtime health** → `claws-debug-agent` (uses the same `auth diagnose` engine but framed for operational triage, not security review)
- **Cutting a claws release** → `claws-release`
- **Designing team / workspace structure** → `claws-teams-architecture`

If the user's question is "my agent isn't responding," hand off to `claws-debug-agent` even if it sounds security-adjacent. This skill is for posture and hardening; that skill is for "why is sarah silent right now."
