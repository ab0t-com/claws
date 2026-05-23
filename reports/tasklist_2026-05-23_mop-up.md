# Tasklist — 2026-05-23 — Mop-up after the incident-batch loop

The 7-ticket batch from 2026-05-23 has shipped; the original OpenAI Codex
incident is fixed on `team/sarah`. This is the residue: small,
operator-decision items that the loop deliberately stopped short of doing
unilaterally. None of them are urgent in isolation; each takes seconds to
minutes; they're listed so they can be claimed and closed cleanly.

**Status convention:** `[ ]` open, `[~]` claimed/in-progress, `[x]` done.
Each task says **what** to run, **why**, **risk**, and **how long.**

---

## T1. Verify `team/lead`, `team/john`, `team1/ben` still respond (post-OAuth-fix sweep)

**Status:** `[ ]`

**Why.** Sarah's OAuth refresh token died at 2026-05-23T04:27 and you just
re-OAuth'd her. The other openai-codex agents (`team/john`, `team/lead`)
share the *provider*, not the credential — but they were set up around the
same time. They might be on the same expiry timer. `team1/ben` is on
Anthropic (different provider entirely). Today's `clawctl auth status --probe`
shows `john` flipped to `✓ no errors 5m` (post-test-message activity, or
because nothing's tried recently). The reliable test is a real message.

**Run.**

```bash
# Per agent that's on Telegram (sarah, john; possibly lead, ben):
#   1. send the agent a quick Telegram message ("test")
#   2. wait ~5 seconds
#   3. verify
./clawctl auth verify team/john
./clawctl auth verify team/lead
./clawctl auth verify team1/ben
```

If verify reports `✗ failing`: run `./clawctl auth <name> codex` (for
openai-codex agents) or `./clawctl auth team1/ben apikey anthropic <key>`
(for ben, if Anthropic is what's set up).

**Risk.** Sending test messages is read-only-ish; reauth modifies
credentials on disk and restarts the gateway. Idempotence preflight means
re-running auth against an already-working agent is a no-op.

**Cost.** ≤ 2 minutes per agent.

---

## T2. Remove the WhatsApp config on `team/sarah` (Telegram-only fleet hygiene)

**Status:** `[ ]`

**Why.** You said earlier the agents are meant to be Telegram-only;
sarah's WhatsApp config is leftover. Today every ~30 seconds a Baileys
401 lands in her logs (`channel exited: {"data":{"reason":"401",
"location":"frc/cco/atn/..."}}`), filling the audit-log surface with
non-signal noise and burning a small amount of CPU on auto-restart
attempts.

**Run.**

```bash
./clawctl channel remove team/sarah whatsapp
# (no --yes needed; idempotent)
```

This sets `channels.whatsapp.enabled = false` in `~/.openclaw/team/sarah/
openclaw.json` and restarts the gateway (no `--hard`, so it's fast — ~2s).

**Risk.** Sarah's WhatsApp credentials in `credentials/whatsapp/` remain
on disk (not deleted). If you ever want to re-enable, you'd re-scan the
QR. The session is logged-out anyway (that's what 401 means here), so
there's nothing to preserve.

**Cost.** ~5 seconds for the command + ~2 seconds restart.

---

## T3. Clean the 3 orphan containers from prior test runs

**Status:** `[ ]`

**Why.** `openclaw-alpha-one`, `openclaw-alpha-two`, `openclaw-bob` are
restart-looping with mount paths that no longer exist. Each spends a few
percent of CPU on its retry cycle. They're invisible to `clawctl list`
(not in the registry) but visible to `clawctl orphans` and `clawctl
errors`. Caused by integration tests that ran *before* ticket 12's
cleanup hook shipped; the hook now prevents new occurrences but doesn't
retroactively clean these.

**Run.**

```bash
./clawctl orphans clean --all --yes
# Then confirm:
./clawctl orphans
# Expected: "No orphan containers found."
```

**Risk.** `docker rm -f` on three test-leftover containers. None of them
holds any state we care about (mount paths are deleted, registry doesn't
know about them, no operator interaction with them in months). Safe.

**Cost.** ~5 seconds.

---

## T4. Deploy ticket 9's compose-template fix to the live agents

**Status:** `[ ]`

**Why.** Ticket 9 fixed the `--bind=loopback` bug where the gateway
listened on the *container's* loopback (unreachable through Docker's
port mapping). The fix is in the repo's `docker-compose.yml`. Live
containers are still on the old template — `clawctl health` keeps
reporting "down" for all four agents, the README's SSH tunnel example
still doesn't reach them, and any external probe (including
`auth status --probe`'s Strategy A and B) gets connection-refused.

**Run.**

```bash
# 1. Copy the new compose template into OPENCLAW_ROOT.
cp /home/ubuntu/claw/workspace/clawctl-go/docker-compose.yml ~/.openclaw/docker-compose.yml

# 2. Recreate each container with the new template.
#    --hard does docker compose down + up -d, which is the only way to
#    pick up template changes (a soft restart doesn't re-read it).
./clawctl team restart team --hard --yes      # sarah, john, lead
./clawctl restart team1/ben --hard

# 3. Verify the fix worked.
./clawctl health
# Expected: all four flip from "down" to "healthy".
curl -s http://127.0.0.1:18789/healthz
# Expected: 200 OK (vs. today's connection refused).
```

**Risk.** Hard restart drops in-flight connections (~30s per agent
total). The auto-verify after each restart (task 11.2) will catch any
agent that fails to come back up — that's the safety net. Worst case,
one agent fails to restart cleanly and you have a per-instance issue to
debug, with the other three still up. No data risk: instance.env,
credentials, workspace all untouched.

**Cost.** ~2 minutes for the full sequence (sequential `--hard`
restarts with 30s health-wait each).

---

## T5. Commit/PR strategy for the 2026-05-23 batch

**Status:** `[ ]`

**Why.** This session produced a large local diff: 15 modified files, 8
new files, 7 new ticket directories with detailed worklogs, and 5 new
documents under `reports/`. None of it is committed yet. Before any of
this propagates further (other clones, CI, deployments), decide how to
land it in git history.

**The options (pick one):**

```
Option A. One commit per ticket, in dependency order.
   git checkout -b feature/fleet-control-2026-05-23
   # 7 commits: 9, 10 (the big one), 11, 12, 13, 14, 15
   git push -u origin feature/fleet-control-2026-05-23
   gh pr create ...

Option B. One umbrella commit.
   git commit -m "ship: 2026-05-23 incident batch (tickets 9-15)"
   # Squash into one; references all 7 tickets in the body.

Option C. Stage now, push later.
   git add .  # excluding clawctl/clawctl-v1 binaries and tmp paths
   # Operator reviews, commits/pushes when ready.

Option D. Keep local for now.
   No git ops. Useful if you want to keep iterating before publishing.
```

**My read:** Option A. Each ticket has a coherent self-contained scope
and a worklog that doubles as a PR description. CI/review benefits from
the granularity. Cost is ~10 minutes of `git add` + commit-message
authoring (the worklog headers are good commit-message material).

**Risk.** Whichever you pick: do **not** include the test orphan
binaries, `/tmp/clawctl-current`, or `clawctl-v2-mar27-backup`. Those
are session artifacts. Suggested `.gitignore` additions if you go for
Option A or B:

```
clawctl
clawctl-v1
clawctl-v2-mar27-backup
/tmp/
```

(`clawctl` is the deployed binary — discuss whether it should be in
git or treated as a build artifact. Today it's tracked because the
binary was committed early on; for a public release it should not be.)

**Cost.** 10-30 minutes depending on option.

---

## T6. (Follow-up enhancement) Surface restart-loop counts more loudly in `clawctl errors`

**Status:** `[ ]`

**Why.** During this session's wrap-up the operator asked: "do we have a
command to see restart-looped things?" Today, `clawctl errors` already
does — but for **orphans** specifically, the restart count isn't shown.
Bob has 1500+ restarts on it. A `restarts=1543` field next to each
orphan would make the urgency immediately obvious without needing to
`docker inspect` per container.

**Scope (minimal — keep this small):**

- Extend `orphanInfo` in `orphans.go` with `RestartCount int`.
- Populate it in `inspectOrphan` (one more `docker inspect --format
  '{{.RestartCount}}'` call per orphan — same shape as the per-instance
  restart-count code path in `errors_cmd.go`).
- Render in both `clawctl orphans` and the orphans section of
  `clawctl errors` as `(N restarts)` in yellow when > 5.
- JSON: add the field; existing consumers see one extra key, no break.

**Out of scope.** Don't add a new top-level `clawctl restart-loops`
verb. The signal already lives in `errors` and `orphans`; adding a verb
duplicates surface for no new information.

**Risk.** None — purely a render change with one extra docker inspect
per orphan.

**Cost.** ~30 minutes including a tiny unit test for the renderer.

---

## Sequencing recommendation

If you want to claim these in dependency-ish order:

1. **T3** (clean orphans) — most-bang-for-buck; clears the noise in
   every subsequent observability call.
2. **T2** (remove WhatsApp on sarah) — stops the audit-log spam.
3. **T1** (verify other agents) — confirms incident is fully closed.
4. **T4** (deploy ticket 9 fix) — bigger; one focused window when you're
   ready for the brief downtime.
5. **T6** (restart-loop surface enhancement) — small follow-up; can
   wait until next session.
6. **T5** (commit/PR) — last, so everything above is captured in the
   same commit batch.

---

## Append-only worklog

_Operators add a dated entry here as each task is claimed and completed._
