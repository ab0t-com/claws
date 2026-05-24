# oauth-refresh-collision — agents share an upstream OAuth identity, reuse alarm will recur

**Filed:** 2026-05-24
**Status:** Open — investigate then remediate
**Severity:** Medium (silent broken auth surfaces every few hours; v1.6.13 makes it visible, v1.6.15 makes diagnosis fast, but the underlying collision is unfixed)
**Related:**
- [`tickets/auth-fleet-helpers-2026-05-24/`](../auth-fleet-helpers-2026-05-24/) — v1.6.14 shipped the operator tools (`auth fleet`, `auth diagnose`)
- [`tickets/credential-broker-2026-05-24/`](../credential-broker-2026-05-24/) — v2.x daemon design (deferred)

---

## The unresolved problem

After v1.6.15's probe fixes, `claws auth diagnose` reports every agent
on this host as `✓ readyz` — but that's *current state*, not
*future-proof state*. The `refresh_token_reused` alarm that hit
`team/john` will recur once any agent rotates its refresh token,
because the agents almost certainly share an upstream OAuth identity.

Mechanism (recap from v1.6.13 changelog):

1. OAuth refresh tokens are single-use. Exchanging `refresh_token_A`
   for a new access token issues `refresh_token_B` and *revokes* A.
2. If N agents share `refresh_token_A` (via mounted file, copied
   credential, or independent grants against the same upstream
   ChatGPT account that the provider's anti-abuse heuristic
   collapses to one chain), only one wins each refresh cycle. The
   others get `refresh_token_reused` → revoked chain → 401 on every
   model call until re-authed.
3. Right now all 4 agents show ✓ because whoever last refreshed got a
   fresh token. Whichever agent is "next in line" — typically every
   few hours, depending on access-token TTL — will hit the alarm.

## What to investigate

Read-only inspection, in this order:

1. **Where does the runtime persist OAuth credentials?**
   Confirmed during the v1.6.6 design work that `~/.codex/auth.json`
   is the typical location, but it's never been confirmed *on this
   host* whether that's container-local (ephemeral) or mounted from
   somewhere on disk. Run:
   ```bash
   for agent in team/sarah team/john team/lead super-team/agent-1; do
     container="openclaw-$(echo $agent | tr / -)-openclaw-gateway-1"
     echo "--- $agent ---"
     docker exec "$container" ls -la ~/.codex ~/.openai 2>/dev/null || echo "  (no codex/openai dirs in container)"
   done
   ```
   Goal: locate every `auth.json` (or equivalent) on the host that
   the runtime is reading/writing.

2. **Are they shared via mount?**
   Check `docker inspect <container> --format '{{range .Mounts}}{{.Source}}->{{.Destination}}{{"\n"}}{{end}}'`
   for each agent. If two containers map the same host path to
   `~/.codex` (or whatever the OAuth dir is), that's a definite
   shared-grant. Fix: remove the shared mount and per-agent
   re-auth.

3. **Are they shared via copied content?**
   For each per-agent auth dir, hash the credential file
   (`sha256sum ...`). Equal hashes between two agents → same
   credential was copied at setup. Fix: per-agent re-auth.

4. **Are they independent grants against the same upstream
   account?** This is harder to detect from the host — the JWT
   payload of the access token names the upstream user, but
   inspecting that pulls credential content. The signal would be:
   even after `claws auth fleet codex --missing-only` with separate
   browser sessions for each, collisions still recur within hours.
   That would indicate OpenAI's anti-abuse heuristic is collapsing
   them.

## Remediation paths

In order of preference:

### A. Per-agent re-auth (the simple v1.6.14 answer)

```bash
claws auth fleet codex
```

Runs OAuth per agent with the 3-second countdown. Each agent gets
its own refresh chain. Sidesteps the collision entirely *if* the
provider treats each browser session as a distinct grant.

If investigation step 4 shows OpenAI is collapsing them upstream,
this path doesn't fully work — the user would need either a
**separate ChatGPT account per agent** (expensive, anti-personal-use)
or **switch to API keys** (`claws auth fleet apikey openai sk-…`),
which don't rotate.

### B. Remove the shared mount (if investigation step 2 confirms)

Cooperate with the openclaw runtime maintainers to make sure each
agent's OAuth dir is per-agent, not shared. This is a runtime/compose
change, not a claws change. May already be the case — needs
confirmation.

### C. Token broker (the v2.x answer)

See [`tickets/credential-broker-2026-05-24/`](../credential-broker-2026-05-24/).
Single refresher per chain eliminates the collision class entirely.
Big lift; defer until A and B are exhausted or a second operator
hits this.

## Acceptance criteria

- Investigation produces a definitive answer to "are these agents
  actually sharing a refresh chain, and if so, how?". Either:
  - Confirmed shared mount → fix in compose/runtime config.
  - Confirmed copied credentials → re-auth per agent via path A.
  - Confirmed upstream collapse by provider → escalate to broker
    ticket or document the API-key workaround.
- Fleet runs ≥7 days without any agent hitting `refresh_token_reused`.
- `claws auth diagnose` shows ✓ for every agent at the start AND
  end of the test window.

## What this ticket is NOT

- Not a request to build the broker. That's the v2.x ticket.
- Not a request to read credential contents from outside the box.
  Hashing in-container (step 3 above) is fine; copying tokens out is
  not.
- Not a request to auto-remediate via `claws auth diagnose --fix` or
  similar. The auth flow must be operator-initiated.

## Why it can't auto-fix (operator action required)

OAuth flows require a browser. claws is a CLI on a server. The
operator's laptop has the browser. So `claws auth <name> codex`
prints a URL, the operator visits it in their browser, completes
the consent, copies the result back. There's no way for claws to
re-auth on its own — the human is in the loop by design.

What claws CAN do (and now does, since v1.6.15):

- Surface the failure at startup (`claws start` runs auth verify).
- Diagnose the cause (`claws auth diagnose` shows risk signals).
- Make multi-agent re-auth one command (`claws auth fleet codex`).
- Eventually: own the refresh chain via the broker daemon so the
  flow only happens once per provider, not once per agent.

## Reproducer (for whoever picks this up)

This host as of 2026-05-24:

```
$ claws auth diagnose
NAME               PROVIDER       VERIFY                 LAST AUTH      REMEDIATION
────────────────── ────────────── ────────────────────── ────────────── ──────────────────────────
team1/ben          ?              ✓ readyz               —              —
team/sarah         openai-codex   ✓ readyz               1d ago         —
team/john          openai-codex   ✓ readyz               —              —
team/lead          openai-codex   ✓ readyz               —              —
super-team/agent-1 ?              ? no recent activity   —              claws agent ping super-team/agent-1
```

Wait 2-6 hours. At least one agent will flip to:

```
team/<one of them>  openai-codex   ✗ refresh_token_reused …  …  claws auth team/<x> codex
```

The pattern was confirmed live on this host during the v1.6.13
investigation — `team/john` showed `refresh_token_reused` while
`team/sarah` showed `verified` simultaneously.
