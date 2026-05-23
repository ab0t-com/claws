# Ticket: Per-instance auth verification + reliable reauth

**Created:** 2026-05-23
**Status:** Open
**Priority:** P0 — Critical (the actual unmet half of the 2026-05-23 incident)
**Owner:** unassigned

> **History note.** The first draft of this ticket was titled "Fleet-level auth reauth + auth health verification" and built a multi-section spec around `team reauth <team> codex` with a "shared-account flavor" that propagated one OAuth credential to N agents. That was premature — the operator's problem was *not* "I can't reauth four agents at once," it was *"I can't tell whether my one reauth worked."* This rewrite drops the bulk premise and focuses on the missing per-instance primitive. Bulk lives in a clearly-marked "future, conditional" section at the bottom.

---

## The actual problem

The 2026-05-23 incident:

> "our agents the deployed ones currently running, are having openai auth issues, now firstly does our cli able to detect that? and what is the procedure we need to reauth them and reconnect them to a working model"

After the fleet/team-control work (ticket 10), the operator can:

- See what's configured: `clawctl auth status`, `clawctl info <name>`, `clawctl list --rich`
- See what's broken at the channel level: `clawctl logs <name> --grep=401`
- See what's configured per channel: `clawctl channels`

What the operator still can't do, **even on one agent**:

1. **Verify the model auth actually works.** `auth status` shows "configured: yes, last auth: 2 weeks ago". Both can be true while the agent is silently 401-ing on every model call (expired refresh token, revoked key, account suspended). The only verification today is to send a real message in a real channel and wait — slow, requires another human, half the time the channel itself is the problem.

2. **Trust the reauth verb.** `clawctl auth <name> codex` runs the OAuth dance and restarts the gateway. It reports `==> Auth complete for '<name>'`. That message is *aspirational* — it means "we made it through the CLI flow without erroring." It doesn't mean the agent can now hit the model. Operators learn to distrust this message.

That's the operator-facing gap. Fix that and the original incident is closed; everything else is a future optimisation.

---

## Minimum viable surface

### V1 — `clawctl auth verify <name>`

A single primitive: prove that this agent can call its model. One instance. Returns `0` on success, non-zero with a directive error on failure. JSON output for scripting.

```
$ clawctl auth verify team/sarah
==> Verifying model auth for 'team/sarah'...
   provider:  openai-codex
   model:     gpt-5.4
   ✓ model responded in 412ms
   ✓ auth ok

$ clawctl auth verify team/john
==> Verifying model auth for 'team/john'...
   provider:  openai-codex
   model:     gpt-5.4
   ✗ provider returned 401 (Unauthorized)
   Fix: clawctl auth team/john codex
```

Implementation: the gateway exposes (or we add) a minimal `POST /__openclaw__/auth-check` endpoint that performs the cheapest possible model round-trip — a 1-token completion, or a `/models` listing call against the provider, whichever is supported and cheapest. clawctl issues that one HTTP call per verify and reports the result.

That's it. One verb, one instance, one signal.

### V1 — `clawctl auth <name> codex` (and `apikey`): tighten the existing verb

The verb already exists. Two improvements that make it *trustworthy* without adding new commands:

1. **Auto-verify on success.** After the OAuth dance completes and the gateway restarts, automatically run `auth verify` and report the result inline. If verify fails, surface the directive error and exit non-zero — don't lie about "Auth complete" when the credential the operator just installed doesn't work.

2. **Idempotence.** Running `clawctl auth team/sarah codex` against an agent whose Codex auth already works should be a no-op with a clear "✓ already authed and verified" message, *not* a fresh OAuth dance. Operators retry; idempotent operations make retry safe.

Same for `apikey`: after the key is installed and the gateway restarts, run `auth verify` and report.

### V1 — `clawctl auth status --probe`

The fleet-read version of `auth verify`. Iterates every instance and runs the same per-instance verify. Read-only across the fleet; one *read* per instance, never a write.

```
$ clawctl auth status --probe
NAME           MODEL                       AUTH         VERIFIED
team1/ben      anthropic/claude-opus-4-6   ✓ working    just now (412ms)
team/sarah     openai-codex/gpt-5.4        ✗ 401        just now
team/john      openai-codex/gpt-5.4        ✗ 401        just now
team/lead      openai-codex/gpt-5.4        ✓ working    just now (380ms)

3 instance(s) need attention:
  clawctl auth team/sarah codex
  clawctl auth team/john codex
```

`--group=` filter (already a Task B primitive). `--json` parity. Per-row fix command.

This is a read — it composes the per-instance primitive. It is *not* a fleet write, and that's the line.

---

## Why this is enough

The operator workflow after V1:

```bash
clawctl auth status --probe         # 1. fleet read: which are broken?
clawctl auth team/sarah codex       # 2. fix one
clawctl auth team/john codex        # 3. fix another (auto-verify inline)
clawctl auth status --probe         # 4. confirm
```

Four commands. Each does one thing. None of them is bulk. The bulk feeling comes from the read (`auth status --probe`) being fleet-scoped — which is already the right composition because *reading* doesn't carry the blast radius of *writing*.

The original incident closes with this surface. The operator can detect, fix, and verify, per-instance, with confidence.

---

## What is deliberately not in scope

### `clawctl team reauth <team> codex` — bulk re-OAuth

**Why not now:** the use case for bulk reauth is "all my agents share one credential and I need to refresh that credential across them." That's a real future need *if* the shared-credential pattern is what operators adopt. It's not what the 2026-05-23 incident was about. The operator had four agents on `openai-codex/gpt-5.4` with separate OAuth tokens; bulk would have saved them three browser windows. That's a convenience, not a missing capability.

**When to revisit:** when (a) at least one operator hits a real "I need to push the same credential to N agents" workflow that the per-instance verb makes painful enough to file an issue, OR (b) the "shared OpenAI account across an instance pool" pattern becomes the dominant deployment shape. Until one of those, designing the surface invites premature complexity (which "flavor" of bulk? shared-account vs per-instance dance? blast-radius semantics? how does it interact with policy.allowedAuthProviders?). All those questions are easier to answer with real usage.

### `clawctl auth refresh <name>` — silent refresh-token exchange

**Why not now:** the gateway is *already* responsible for refresh-token exchange — that's standard OAuth client behavior. If the gateway's silent refresh is failing in production often enough that operators need a manual lever, that's an OpenClaw runtime bug, not a clawctl feature. Surfacing it as a clawctl verb risks normalizing the bug rather than fixing it.

**When to revisit:** if `auth verify` reveals a pattern of "refresh tokens going bad on a predictable timer that the runtime isn't handling." Then file the fix on the OpenClaw side, and at most add a thin `clawctl exec <name> auth refresh` shortcut that pokes the runtime.

### `clawctl auth status --probe --group=<name> --json | jq` — fancy aggregation

**Why not in this ticket:** the building blocks (`--group=`, `--json`) shipped in ticket 10. Combining them in `auth status --probe` falls out for free. No new design.

---

## Engineering principles (Cloudflare-style)

The bulk vs primitive question is the same one Cloudflare's control plane has resolved many times. The principles that apply:

1. **Primitives before aggregations.** Ship the per-resource verb (`auth verify <name>`). Ship the per-resource write (`auth <name> codex`). Aggregations (`team reauth`, `auth status --probe`) come *after* the primitive proves reliable, and only when the aggregation has a concrete use case operators can articulate. This is how Cloudflare's API evolved — `zone/dns_records/<id>` shipped years before bulk DNS endpoints.

2. **Verify is a first-class verb, not a side effect.** Don't conflate "I applied a credential" with "the credential works." The fact that today's `clawctl auth ... codex` happily prints "Auth complete" without verifying is a control-plane lie. The fix isn't to add a different verb that does both — it's to make verify the cheap, callable primitive that the write verb also runs as its last step. That way `verify` is composable, observable, and rerunnable.

3. **Idempotence is operator-facing safety.** Operators retry. If `clawctl auth <name> codex` always starts a fresh OAuth dance even when the existing creds work, retrying becomes destructive (you've now invalidated the working creds). Idempotent reauth is just "check, run only if needed, verify either way."

4. **The blast radius lives in the verb name.** `auth <name>` is per-instance — one safe blast radius. `team reauth` is fleet — N blast radii — and the operator should have to spell that out. The visual asymmetry between "narrow per-instance" and "broad team-noun" verbs is a feature: an operator reading their own bash history can see at a glance which commands affected the whole team.

5. **Don't ship features for use cases you can imagine but haven't observed.** Imagined use cases generate spec sprawl. Observed use cases generate small focused tickets. The original bulk-reauth design in this ticket was the former. This rewrite is the latter.

6. **Composition over configuration.** `auth status --probe` is `auth verify` composed across the fleet. If we ever do need bulk-write semantics, that's `auth <name> codex` composed across a group — same primitive, different composition. No new code paths to maintain.

7. **Honest scope boundaries.** Refresh-token mechanics belong to the OpenClaw runtime. Cred storage location belongs to the OpenClaw runtime. clawctl's job is to *trigger* and *observe*, not to own the implementation. Anywhere we're tempted to add a clawctl verb that mirrors a runtime behavior, the right move is usually to fix the runtime instead.

These aren't novel principles — they're just the discipline of resisting the urge to pre-build for shapes that haven't appeared yet. The cost of premature aggregation is paid in maintenance complexity and confusing operator mental models forever; the cost of deferred aggregation is a few weeks of mild inconvenience until the use case is concrete enough to design against.

---

## Acceptance criteria

- [ ] `clawctl auth verify <name>` returns 0 on a working model auth, non-zero on failure, with a per-failure directive fix message. JSON output (`{verified: bool, provider, model, latency_ms, error?}`).
- [ ] `clawctl auth <name> codex` and `clawctl auth <name> apikey <provider> <key>` are idempotent: running them against already-working auth is a no-op with a "✓ already authed" line, exit 0.
- [ ] After a successful credential install, both verbs automatically run `auth verify` and either confirm success or report the verify failure and exit non-zero.
- [ ] `clawctl auth status --probe` runs `auth verify` for every instance (or every member of `--group=`) and renders the result inline. JSON parity.
- [ ] `--probe` is rate-limited (default: do not re-probe within 30s per instance) to avoid burning model quota on tight loops.
- [ ] Help text on `clawctl auth --help` documents `verify`, the auto-verify behaviour, and `status --probe`.
- [ ] Audit log records every `auth verify` and every `auth` reauth with provider + result.

## Dependencies

- **OpenClaw runtime cooperation**: needs a minimal "auth check" endpoint (cheapest possible model round-trip) at a stable path. File the OpenClaw side first; clawctl is downstream.

## Future, conditional

Sections that should *only* be opened once their triggering use case is observed:

- **Bulk apply credential** (`team reauth <team> apikey <provider> <key>` or similar). Trigger: an operator hits a real workflow where they're applying the same key to ≥3 agents per week and the per-instance verb is the bottleneck. Likely shape if/when needed: thin wrapper over the per-instance verb using the existing `runOnGroup` helper from Task B, with explicit confirmation. Half a day.
- **Shared-credential propagation** (one agent's working OAuth token applied to others). Trigger: an operator's deployment model is "one OpenAI account, many agents" and they want to avoid N browser dances. Likely shape: requires OpenClaw to expose credential export/import; not a clawctl-only feature.
- **Silent refresh trigger** (`auth refresh <name>`). Trigger: production evidence that the gateway's automatic refresh-token exchange is unreliable. Most likely fixed in the runtime, not in clawctl.

Each of these gets its own ticket *when* the trigger fires, sized against the real demand at that point.

## Related

- `tickets/fleet-team-control-surface-2026-05-23/` — parent ticket. Shipped the visibility surface this builds on.
- `tickets/health-probe-loopback-bind-2026-05-23/` — sibling P1. If the gateway's HTTP listener isn't reachable from the host (the loopback-bind bug), `auth verify`'s HTTP probe can't reach it. Should land before `auth status --probe` is rolled out for ops use.

## Effort

- `auth verify <name>` (clawctl side, assuming OpenClaw exposes the check endpoint): 2 hours
- OpenClaw `/__openclaw__/auth-check` endpoint: estimate at runtime
- Idempotence + auto-verify on existing `auth` verbs: 2 hours
- `auth status --probe`: 1 hour (composes the per-instance verify)
- Rate-limit on `--probe`: 1 hour
- Help text + audit log + integration test gating: 2 hours

Total: ~1 day clawctl + the OpenClaw-side endpoint. **Half of what the first-draft ticket estimated, addressing the actual problem instead of the imagined bulk one.**
