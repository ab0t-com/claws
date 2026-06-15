# OAuth refresh-token collision (the `refresh_token_reused` class)

Use this reference when `claws logs <name>` or `claws auth diagnose` surfaces
`refresh_token_reused`, `Token refresh failed: 401`, or any "OAuth was working
yesterday, broken today, intermittent across agents" pattern.

## What is happening

OAuth refresh is single-use by design. When a consumer trades `refresh_token_A`
for a new access token, the provider issues `refresh_token_B` AND revokes A. If
two or more consumers share the same on-disk credential blob (mounted dir,
copied `auth.json`, host-CLI sharing tokens with the agent), one consumer wins
each refresh round and the others get `refresh_token_reused` plus a now-revoked
chain. Whichever lost the race fails next; whoever wins succeeds. The failure
rotates between agents over time, which is why the user sees "sometimes sarah
fails, sometimes john."

This is the only failure mode this file covers. WhatsApp 401s, channel-token
expiry, and API-key-class auth failures are different and live in SKILL.md.

## Recognizing it

Any of these are sufficient:

- `claws auth diagnose` shows ≥2 agents with `VERIFY ✗ refresh_token_reused`
  on the same provider.
- `claws auth diagnose` fires the bunched-auth risk signal (≥2 agents authed
  within 15 min on the same provider) AND the user reports intermittent
  failures.
- `claws logs <name> --grep=refresh_token_reused` returns recent hits across
  multiple agents.

## The fix (current production answer, v1.6.14+)

Give each agent its own independent OAuth grant. Each agent's container holds
its own credentials; no shared blob, no refresh-chain collision.

For a single affected agent:
```
claws auth <name> codex
```

For the whole fleet (recommended when bunched-auth fired or ≥2 agents are
already failing — saves doing it one at a time and avoids prompts that
require a TTY per agent):
```
claws auth fleet codex
```

Flags worth knowing:
- `--missing-only` — skip agents whose auth already verifies.
- `--group=<team>` — limit to one team.
- `--yes` — skip the 3-second countdown (for scripts; not for interactive use).

After re-auth, verify:
```
claws auth diagnose
claws agent ping <name>
```

Both should be fully green.

## Why we did not ship a credential broker

It is the architecturally clean answer for N consumers / central revocation /
shared audit. It is wrong for the current scale (one operator, ~5 agents on
one host) because the operational cost of a long-lived broker daemon dwarfs
the cost of N independent OAuth grants. The decision is documented in
`tickets/credential-broker-2026-05-24/` and the followup investigation lives
in `tickets/oauth-refresh-collision-followup-2026-05-24/`. Re-evaluate when a
second consumer materializes (e.g. a host-side CLI that needs to share
credentials with agents) AND the per-agent OAuth UX is the dominant pain.

Do not propose the broker as the fix here. The fix here is per-agent OAuth.

## What NOT to do

- Do not copy `~/.codex` from one agent's container into another's. That is
  exactly the failure mode.
- Do not mount a shared `auth.json` across agents.
- Do not `claws auth verify` in a loop hoping it stabilizes — refresh-chain
  damage does not heal; it has to be re-issued by re-running OAuth.
- Do not delete agents to "reset" them when re-auth is the actual fix.

## Pre-v1.6.13 caveat

Before v1.6.13, `claws start` did not verify auth after the container came up.
A broken OAuth chain surfaced hours later as "agent isn't replying" instead of
at startup. If the user is on an older build, `claws update` first — otherwise
the diagnostic loop is slower than it needs to be.
