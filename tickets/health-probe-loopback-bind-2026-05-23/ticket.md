# Ticket: `clawctl health` reports false "down" when `--bind=loopback`

**Created:** 2026-05-23
**Status:** Open
**Priority:** P1 — High (silently misleading operators about live production state)
**Discovered:** During live diagnosis of a perceived Telegram/OpenAI auth incident on `~/.openclaw/` agents (`team/sarah`, `team/lead`, `team/john`, `team1/ben`). All four were running, but `clawctl health` told the operator they were down. The operator could easily over-react to that false signal (e.g., recreate containers, re-auth credentials) when nothing about the agent itself is broken.

---

## Summary

`clawctl health <name>` performs an HTTP `GET /healthz` from the *host* against `127.0.0.1:<gateway-port>`. With the current default `--bind=loopback`, the in-container gateway listens on the container's own `127.0.0.1:18789`, not on the container's external network interface. Docker's port mapping forwards host:127.0.0.1:18789 → container *external* IP:18789 — which has nothing listening — so the host-side probe always fails.

The Docker `healthcheck` defined in `docker-compose.yml` runs `fetch('http://127.0.0.1:18789/healthz')` *inside* the container's network namespace where the loopback socket is reachable, so it passes. Hence `docker ps` and `clawctl list` (which reads `docker compose ps`) report `(healthy)` while `clawctl health` reports `down`.

The two commands disagree because they are measuring two different network paths against a gateway that is actually fine.

## Repro

Tested on `feature/runtime-adapter` against four live agents (~/.openclaw/team/{sarah,lead,john}, team1/ben), all configured with `OPENCLAW_GATEWAY_BIND=loopback` and `OPENCLAW_HOST_BIND=127.0.0.1`.

```
$ ./clawctl list
NAME            PORT     STATUS       RAM        UPTIME
team1/ben       :18989   healthy      195.7MiB   27 hours
team/sarah      :18789   healthy      410.7MiB   27 hours
team/john       :18889   healthy      324.5MiB   27 hours
team/lead       :19089   healthy      198.3MiB   27 hours

$ ./clawctl health
NAME            PORT     VERDICT      LIVE       DETAILS
team1/ben       :18989   down         no         gateway not responding
team/sarah      :18789   down         no         gateway not responding
team/john       :18889   down         no         gateway not responding
team/lead       :19089   down         no         gateway not responding

$ docker ps --filter "name=openclaw" --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'
NAMES                                    STATUS                  PORTS
openclaw-team1-ben-openclaw-gateway-1    Up 27 hours (healthy)   127.0.0.1:18989->18789/tcp, 127.0.0.1:18990->18790/tcp
openclaw-team-lead-openclaw-gateway-1    Up 27 hours (healthy)   127.0.0.1:19089->18789/tcp, 127.0.0.1:19090->18790/tcp
openclaw-team-sarah-openclaw-gateway-1   Up 27 hours (healthy)   127.0.0.1:18789-18790->18789-18790/tcp
openclaw-team-john-openclaw-gateway-1    Up 27 hours (healthy)   127.0.0.1:18889->18789/tcp, 127.0.0.1:18890->18790/tcp

$ for p in 18789 18889 18989 19089; do
    curl -m 2 -s -o /dev/null -w "$p HTTP %{http_code}\n" "http://127.0.0.1:$p/healthz"
  done
18789 HTTP 000
18889 HTTP 000
18989 HTTP 000
19089 HTTP 000
```

The Docker healthcheck *inside* each container is what produces `(healthy)`, and it passes because it has access to the container's own loopback. The host-side curl (and clawctl's `http.Get`) sees the same nothing-on-the-other-end that any external client would.

## Why this happens

In `docker-compose.yml`:

```yaml
ports:
  - "${OPENCLAW_HOST_BIND:-127.0.0.1}:${OPENCLAW_GATEWAY_PORT:-18789}:18789"
command:
  - "node"
  - "dist/index.js"
  - "gateway"
  - "--bind"
  - "${OPENCLAW_GATEWAY_BIND:-loopback}"
  - "--port"
  - "18789"
```

In `config.go`:

```go
func hostBind(bindMode string) string {
    switch bindMode {
    case "loopback":
        return "127.0.0.1"          // → OPENCLAW_HOST_BIND
    case "lan", "wan":
        return "0.0.0.0"
    default:
        return "127.0.0.1"
    }
}
```

In `commands.go:cmdCreate` (lines ~239-244 in current source):

```go
OPENCLAW_GATEWAY_BIND=%s   // ← bindMode written to instance.env
OPENCLAW_HOST_BIND=%s       // ← hostBind(bindMode) — what Docker uses for port mapping
```

The intent is clear: `OPENCLAW_HOST_BIND` should control whether the *host* exposes the port to the LAN/WAN, and the in-container gateway should always be reachable through the container's external interface (i.e., listen on `0.0.0.0:18789` inside the container). But `OPENCLAW_GATEWAY_BIND=loopback` is being passed straight to the gateway's `--bind` flag, making the gateway listen on the container's own loopback only — which Docker's port mapping cannot reach. The two env vars are doing overlapping jobs and their semantics collide for `loopback`.

The bug is invisible while operators don't try to run `clawctl health` (or anything else that probes via TCP on the host) — they see `clawctl list` say "healthy" and trust that. It also breaks the SSH tunnel use case from the README — `ssh -L 18789:127.0.0.1:18789 ubuntu@server` forwards the host's loopback, which routes through Docker NAT, which has nothing to deliver to.

## Confirmed unaffected paths

- The Docker internal healthcheck (passes — runs inside the container netns).
- The OpenClaw gateway process itself (running, holding its WS server).
- Inbound channel traffic for Telegram/etc. (those are outbound polls *from* the container, so they don't depend on host-side port mapping).

So today, agents are *processing channel traffic correctly* but cannot be reached over HTTP from outside the container, and `clawctl health` is therefore wrong.

## Proposed fix

**One-line fix in the compose template.** Always bind the gateway to `0.0.0.0` inside the container. Leave host-side exposure entirely up to `OPENCLAW_HOST_BIND` (which is already what controls reachability from outside the box).

Option A — change the compose default:

```yaml
command:
  - "node"
  - "dist/index.js"
  - "gateway"
  - "--bind"
  - "0.0.0.0"           # ← was: ${OPENCLAW_GATEWAY_BIND:-loopback}
  - "--port"
  - "18789"
```

…and drop the `OPENCLAW_GATEWAY_BIND` env var from `instance.env` writes. Operator network exposure is then exclusively governed by `OPENCLAW_HOST_BIND` (which `hostBind(bindMode)` already sets correctly: `127.0.0.1` for loopback, `0.0.0.0` for lan/wan).

Option B — keep `OPENCLAW_GATEWAY_BIND` but translate it at the boundary:

```go
// new helper
func gatewayBindInContainer(bindMode string) string { return "0.0.0.0" }
```

…and pass that to the compose template instead of the raw `bindMode`. The CLI flag retains its operator-visible meaning ("loopback / lan / wan") but it no longer breaks the in-container listener.

Option A is simpler; Option B keeps room for a future feature (e.g., binding to a specific container interface). Either way: **stop passing `loopback` to the gateway's `--bind`.**

## Side effects of the fix

- **Health probes from the host start working** (`clawctl health`, `clawctl status` overview, the `/readyz` probe in the smoke harness).
- **SSH tunnel becomes useful again** — the README example `ssh -L 18789:127.0.0.1:18789 ubuntu@<server>` actually reaches the gateway.
- **No change in attack surface for `bind=loopback`** — host still binds to `127.0.0.1` only via `OPENCLAW_HOST_BIND`, so external clients still can't reach the agent. The threat model is preserved.
- **Operators who restart with the new compose template will become reachable on their tunnels for the first time.** Probably what they actually wanted.

## Required updates if Option A is taken

- `docker-compose.yml` — drop the `${OPENCLAW_GATEWAY_BIND:-loopback}` interpolation; hardcode `--bind 0.0.0.0`.
- `commands.go:cmdCreate` — keep `OPENCLAW_GATEWAY_BIND` in `instance.env` (consumers like `policy.enforceBindPolicy` still read it for policy compliance) but the compose template no longer references it.
- `policy.go:enforceBindPolicy` — unchanged.
- `runtime.go` — if any custom runtime templates also use `OPENCLAW_GATEWAY_BIND`, mirror the change.
- Tests:
  - Add an integration test (off by default — requires Docker) that creates an instance with `--bind=loopback`, starts it, and asserts `curl http://127.0.0.1:<port>/healthz` succeeds from the host. Today this test would catch a regression of the current bug.
  - Update `integration_test.go` env-file assertion if it currently asserts the bind value is propagated to the gateway command line.

## How `clawctl health` could *also* be defensive about this

Even after the in-container bind is fixed, `clawctl health` could be more useful when an operator runs a custom build that genuinely listens on container loopback. Two cheap improvements:

1. **Fall back to `docker exec` probe** when the host-side probe fails *and* `docker compose ps` reports the container as `(healthy)`. Today: host-probe fails → reports `down`. Better: host-probe fails → check Docker health → if Docker says healthy, report `degraded — gateway is up inside the container but not reachable from the host (check bind/port mapping)`.
2. **Surface the bind/port mapping in `clawctl status <name>`** so the operator can see at a glance whether the host-side port forwarding is sane. Today the gateway port is shown but not how it's exposed.

These are nice-to-haves on top of the actual fix.

## Acceptance criteria

- [ ] On a fresh `clawctl create alpha` (default `--bind=loopback`), then `clawctl start alpha`, `curl -s http://127.0.0.1:<port>/healthz` from the host returns HTTP 200.
- [ ] `clawctl health alpha` returns verdict `healthy`.
- [ ] `clawctl list` and `clawctl health` no longer contradict each other for the same running instance.
- [ ] An SSH tunnel forwarding the host's `127.0.0.1:<port>` reaches the gateway's HTTP API.
- [ ] No regression in network-restriction behavior: with `--bind=loopback` the gateway is **still** unreachable from any non-`127.0.0.1` host address (confirmed by `curl -s http://<server-LAN-IP>:<port>/healthz` → connection refused).
- [ ] Integration test that exercises a real Docker start + host curl is added (gated behind a build tag or env var so it's not part of the default CI fast-path).

## Notes for the implementer

- The migration is **lossy if the operator already has running instances on the broken bind**: they need to recreate the container (a soft `restart` does not pick up compose-template changes; use `restart --hard` or `down` + `up -d`). Document this in the CHANGELOG.
- `clawctl policy enforce --restart` will re-apply the new template across all instances at once if `OPENCLAW_GATEWAY_BIND` is in the policy's `allowedBindModes` list. Test that path explicitly.
- The four currently-running production agents on this host (`team/sarah`, `team/lead`, `team/john`, `team1/ben` — all with `OPENCLAW_GATEWAY_BIND=loopback`) will need a `clawctl restart --hard` after the fix lands to recreate their containers from the new template. Their workspace/credentials are unaffected.

## Evidence dump

Filed alongside this ticket: the diagnostic transcript from the 2026-05-23 incident is in `tests/dogfood_log_2026-05-20.md` (Section §"Issues observed" item S5 — the *cosmetic* form of the same family of issues) and in this session's live grep results. The conclusion is independent of the unrelated Telegram poller stall that triggered the diagnosis — that's tracked separately.

## Related

- This issue is mentioned (but not filed) as Section 5 / §"What's mediocre" / `status <name>` empty `docker compose ps` table in `reports/report_engineering-structural_2026-05-20.md` — that's a cosmetic sibling. *This* ticket is the higher-impact case: `clawctl health` lying about live state.
- See also `report_architecture-review_2026-03-17.md` §2 ("Container Naming Assumption") for an adjacent flaky behavior in how clawctl talks to Docker.
- Memory entry [project_live_agents_on_host](~/.claude/projects/.../memory/project_live_agents_on_host.md) — the live agents that surfaced this bug.
