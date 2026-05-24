# auth-fleet-helpers — simple solution to the refresh-token-reuse pain

**Filed:** 2026-05-24
**Target:** v1.6.14
**Status:** Open
**Severity:** Medium (operational friction; root cause already documented in v1.6.13)
**Supersedes (for now):** [`tickets/credential-broker-2026-05-24/`](../credential-broker-2026-05-24/ticket.md) — broker design is right but over-engineered for current scale. See "Why not the broker" below.

---

## The problem (recap)

User hit `refresh_token_reused` on `team/john` because multiple agents
on one host share OAuth refresh chains. v1.6.13 makes the failure
**visible** at startup but doesn't fix the underlying collision.

## The simple fix — two helper commands, zero new state

### 1. `claws auth fleet codex` (or `apikey openai <key>`, etc.)

Fan-out form of `claws auth <name> <method>`. Mirrors the pattern
v1.6.10 introduced for `claws start <team>` — list members, print a
3-second countdown, then iterate.

```
$ claws auth fleet codex
This will run the Codex OAuth flow for each agent in sequence.
You'll be prompted to sign in once per agent (3 prompts total):

  • team/sarah
  • team/john
  • team/lead

  Starting in 3 seconds — Ctrl-C to cancel.
  3...   2...   1...
==> claws auth team/sarah codex
... (OAuth flow)
==> claws auth team/john codex
... (OAuth flow)
==> claws auth team/lead codex
... (OAuth flow)

✓ All 3 agents have independent OAuth grants now.
```

Solves the operational hump: today the operator has to type the
command 3+ times, knowing the names. `fleet` form auto-discovers and
sequences. Each agent ends up with its own refresh chain → collisions
are structurally impossible.

**Variants:**
- `claws auth fleet codex --group=<team>` — limit to one team
- `claws auth fleet codex --missing-only` — skip agents that already
  have a verified grant (idempotent re-runs)

### 2. `claws auth diagnose [name|fleet]`

Read-only diagnostic that aggregates state already in the host
(audit log, `verifyOneInstance` results, instance.env timestamps)
and renders a single screen with the operator's actual next moves:

```
$ claws auth diagnose

NAME              PROVIDER       VERIFY              LAST AUTH      NOTE
───────────────── ────────────── ─────────────────── ────────────── ─────────────────────────
team/sarah        openai-codex   ✓ verified 2s ago   1d ago         —
team/john         openai-codex   ✗ refresh_token_… 1d ago         re-auth: claws auth team/john codex
team/lead         openai-codex   ? not verified yet  1d ago         test: claws agent ping team/lead
super-team/agent-1 openai-codex   ✗ no grant         —              setup: claws auth super-team/agent-1 codex
team1/ben         openai-codex   ✓ verified 14m ago  3d ago         —

Risk signals:
  ⚠ team/sarah + team/john + team/lead were all authed within 8 minutes of
    each other against openai-codex. If they share an upstream ChatGPT
    account, refresh_token_reused will recur (you just hit this for john).
    Each agent should have its own OAuth grant:
      claws auth fleet codex --missing-only

Next:
  claws auth fleet codex --missing-only    set up the agents missing auth
  claws auth team/john codex               re-auth the failing agent
```

No new state files. The diagnostic reads:

- The port registry (already exists).
- The audit log at `<root>/.audit.log` (already exists; we already
  show `last auth` in `claws auth status`).
- The result of `verifyOneInstance(name)` per agent (same call
  v1.6.13 uses at startup).

That's it. Zero new files, zero new JSON, zero new daemon.

---

## What we explicitly are NOT doing here

| Idea | Why deferred |
|---|---|
| Token broker daemon | Right answer for v2.x. Over-engineered for one operator with ~5 agents. See [credential-broker ticket](../credential-broker-2026-05-24/ticket.md) for the full design. |
| Embedding HashiCorp Vault | Replaces the disease with a worse one (Vault init ceremony, unseal keys, audit backends). Wrong scope. |
| OS keyring storage (99designs/keyring) | Solves storage security, NOT the refresh-collision problem we actually have. |
| Central credential JSON owned by claws | Would require runtime cooperation (runtime must stop refreshing on its own). Without that, doesn't actually solve anything. |
| Claws-side refresh worker | Same as above — useless without the runtime contract change. |

The pattern across all of these: any solution that aims for "single
refresher per chain" requires either a daemon (broker) or a runtime
contract change (claws-side refresh). Both are too heavy for the
current scale. The simple answer — each agent has its own grant —
sidesteps the whole question.

## Why not the broker (right now)

The broker ticket is good design and still the right v2.x target. It's
wrong NOW because:

1. **Scale.** One operator, ~5 agents on one host. The collision pain
   is annoying but not catastrophic; an extra setup step (run
   OAuth N times) is acceptable for one user.
2. **Multi-day build.** Daemon + module + runtime coordination + new
   trust-boundary review = 1-2 weeks. The simple fix is half a day.
3. **Speculative.** No second consumer of the broker exists yet
   (sharedwatch + intent-gateway are hypothetical). Build it when
   the second adopter materialises, same lift-then-extract policy
   we already use for the hints package.
4. **Reversibility.** The simple solution is purely additive — `auth
   fleet` and `auth diagnose` are new verbs that don't constrain the
   broker design later. Shipping them now does NOT make the broker
   harder.

## Implementation sketch

```
cmd/claws/
├── auth_fleet.go        (cmdAuthFleet — ~80 LOC fan-out + countdown)
├── auth_diagnose.go     (cmdAuthDiagnose — ~150 LOC: enumerate + verify + risk heuristics)
└── commands.go          (router additions: "auth fleet ..." and "auth diagnose ...")
```

Risk heuristics for diagnose (cheap, all from existing state):

- **Bunched-auth signal:** multiple agents authed within N minutes of
  each other for the same provider → "if they share an upstream
  account, expect collisions".
- **Verify-failure pattern:** N agents with `refresh_token_reused`
  in the last 24h → "definitely sharing; re-auth each independently".
- **Missing-grant signal:** agent has no auth event in the audit log
  → "never authenticated; setup needed".

No new file format. No new on-disk state.

## Acceptance criteria

- `claws auth fleet codex` runs the OAuth flow for every agent that
  doesn't currently verify, in order, with a 3-second countdown +
  Ctrl-C cancel (same UX as `claws start <team>` from v1.6.10).
- `claws auth diagnose` prints a table covering every agent +
  provider, with a verify result, a last-auth time, and a remediation
  command per failing row.
- Both commands are read-only with respect to credentials —
  `diagnose` reads, `fleet` writes via the existing `cmdAuth` path
  (no new write codepath).
- After running `claws auth fleet codex --missing-only`, every agent
  has a verified grant (next `claws auth diagnose` shows all ✓).

## Out of scope (file as follow-ups if relevant)

- Detecting whether two agents literally share an on-disk credential
  file (mount-level inspection). Useful but needs container-level
  reads; punt to v1.6.15.
- Automatic remediation ("`claws auth diagnose --fix`" that re-auths
  failing agents). Mixing diagnostic + mutating verbs is a recipe for
  surprise. Keep them separate.
- Per-provider authentication broker support. That's the v2.x
  broker ticket.
