# credential-broker — central OAuth token authority for the agent fleet

**Filed:** 2026-05-24
**Target:** v2.x (not v1.x — architectural surface change)
**Status:** Open — design
**Severity:** High (silent inference failure across the fleet whenever OAuth refresh races)
**Owner:** TBD
**Module scope:** New separate Go module (`github.com/ab0t-com/cred-broker` or similar). Used by claws AND by any sibling project that talks to OpenAI / Anthropic / OpenRouter / Codex on the operator's behalf.

---

## TL;DR

Build a **per-host credential broker daemon** that owns the operator's
OAuth refresh tokens, exposes a small "give me a fresh access token"
RPC over a unix-domain socket inside each agent container, and is the
**single consumer** of every refresh chain. Eliminates the
`refresh_token_reused` collision class entirely. Doubles as the
**central trust boundary** for all model credentials: agents never
hold the refresh token, only short-lived access material.

Shape it from day one as a **standalone module** that claws + sibling
projects (sharedwatch, intent-gateway, future tools) all depend on —
same extraction-friendly posture as `cmd/claws/hints/`. The module is
the security boundary; claws is one consumer.

---

## Background — the failure mode we just hit

User-observable: `claws start team` reported all three agents started
cleanly. `claws agent ping team/john` failed with an OpenAI Codex
auth error. Logs showed:

```
[openai-codex] Token refresh failed: 401 {
    "message": "Your refresh token has already been used to generate a new access token.
                Please try signing in again.",
    "code": "refresh_token_reused"
}
```

This is **OAuth's anti-theft alarm**. OAuth refresh is single-use by
design: when you exchange `refresh_token_A` for a new access token,
the server issues `refresh_token_B` and **revokes** A. Any subsequent
use of A is treated as a leaked-credential incident — the whole chain
gets invalidated.

It fires whenever two consumers share the same refresh chain. On the
user's host, three agents (sarah/john/lead) authenticate as the same
ChatGPT Plus account. Each container has its own ~/.codex/auth.json
**eventually** but they were either:

- bind-mounted to the same on-disk file (one host file, three readers
  trying to refresh independently), OR
- copy-initialised from the same source credential (the v1.6.6 design
  notes specifically warned against this — manual OAuth-token copying
  creates "a fragile second user of that token"), OR
- the operator separately ran `claws auth … codex` for each agent
  against the same upstream ChatGPT account, and the provider's
  refresh-chain anti-abuse heuristic kicked in regardless.

The race rotates: whichever agent refreshes first wins; the rest get
the reuse alarm on their next attempt. From the operator's view: a
random one of N agents stops responding every few hours.

v1.6.13 surfaces the failure at startup ("Auth check FAILED — agent
won't respond"). That's a **better diagnostic**, not a fix. The
collision still happens; we just tell the operator about it sooner.

The architectural fix is to ensure there is exactly **one consumer**
of each refresh chain.

---

## Goal

Eliminate the `refresh_token_reused` collision class for any fleet
sharing an OAuth identity, without:

- Forcing the operator to maintain N independent OAuth grants
- Requiring runtime changes that break existing agent images
- Holding long-lived credentials inside every container (broader
  attack surface than today)

Plus: make this the **trust boundary** all future "consume credential"
features hang off — audit log of every fetch, per-agent revocation,
per-agent quota, optional approval flows for sensitive providers.

---

## Non-goals

- **Replacing API-key auth.** API keys don't refresh; the collision
  class doesn't apply. `claws auth … apikey …` stays as-is.
- **A full secrets manager** (Vault-style key/value, dynamic DB creds,
  PKI). Scope is strictly "model provider OAuth tokens, plus the
  obvious adjacent stuff like Telegram bot tokens that the runtime
  already needs at boot".
- **Cross-host federation.** This is a per-host daemon. A future
  multi-host story would layer above it (broker-of-brokers, mTLS,
  etc.) and is explicitly out of scope here.
- **Solving "the operator's ChatGPT account got rate-limited"** —
  that's a usage problem, not a credential problem.

---

## Why a separate module / library

Three reasons, in order of importance:

### 1. Security boundary should not be in the agent-management code

If the broker is *part of claws*, then a vulnerability in any
unrelated claws codepath (template rendering, `claws update`'s
download, `claws orphans clean`'s docker-rm logic, the new
`paste-secret` HTTP listener, hooks, …) is potentially a path to
credential exfiltration. Putting the broker in its own process with
its own narrow interface confines blast radius. Trust boundary = process
boundary.

This is the same principle behind `ssh-agent`, `gpg-agent`,
`pinentry`, `pass`, `1Password CLI`, AWS instance metadata service,
GCE metadata server — the credential authority is always a process
distinct from the consumer.

### 2. Reusable across our other tools

sharedwatch and intent-gateway will also want OpenAI / Anthropic
access at some point. If the broker is a separate module, both can
depend on it and benefit from the same single-refresher property.

Mirrors the path we already took with `cmd/claws/hints/` (lifted from
sharedwatch, will extract to `github.com/ab0t-com/cli-hints` on third
consumer). Token broker has stronger reuse pressure than hints —
every tool that calls a model needs it.

### 3. Testable in isolation

Pure module = unit tests with fake providers, fuzz tests against the
refresh-chain state machine, easy to swap implementations (mock
broker for integration tests, real broker for prod).

---

## Proposed architecture

```
                ┌────────────────────────────────────────────────────────┐
                │ HOST                                                    │
                │                                                         │
                │   ┌──────────────────────────────────────────┐         │
                │   │ claws-credd (the broker daemon)          │         │
                │   │                                            │         │
                │   │   /var/lib/claws-credd/store.db          │         │
                │   │     refresh_tokens (encrypted at rest)   │         │
                │   │     per-agent leases                     │         │
                │   │     audit log                            │         │
                │   │                                            │         │
                │   │   Listens on:                            │         │
                │   │     /var/run/claws-credd.sock (host)    │         │
                │   │                                            │         │
                │   │   Refresh worker:                        │         │
                │   │     - single consumer of each chain     │         │
                │   │     - background pre-refresh (no race)  │         │
                │   │     - per-provider exponential backoff  │         │
                │   └──────────────────────────────────────────┘         │
                │                       ▲                                 │
                │                       │ unix socket                     │
                │                       │ (mounted into each container)   │
                │                       │                                 │
                │   ┌───────────────────┴────────────────────┐           │
                │   │ Container: openclaw-team-sarah-gateway  │           │
                │   │                                          │           │
                │   │   /var/run/claws-credd.sock (read-only) │           │
                │   │                                          │           │
                │   │   openclaw runtime — when about to call │           │
                │   │   OpenAI/Codex, asks broker for an      │           │
                │   │   access token instead of refreshing    │           │
                │   │   ~/.codex/auth.json itself             │           │
                │   └─────────────────────────────────────────┘           │
                │   ┌─────────────────────────────────────────┐           │
                │   │ Container: openclaw-team-john-gateway   │  (same)   │
                │   └─────────────────────────────────────────┘           │
                │   ┌─────────────────────────────────────────┐           │
                │   │ Container: openclaw-team-lead-gateway   │  (same)   │
                │   └─────────────────────────────────────────┘           │
                │                                                         │
                └────────────────────────────────────────────────────────┘
                                       ▲
                                       │
                                       │ HTTPS
                                       ▼
                          OpenAI / Anthropic / etc.
                          (sees one refresher per chain)
```

### Wire protocol — minimal

Plain HTTP-over-unix-socket. Two endpoints, both authenticated by the
caller's process-level identity (the broker can `getpeercred(2)` the
socket peer and look up which agent container owns that PID — or in
the container-side case, by the bind-mount + token model below).

```
GET /v1/token?provider=openai-codex&agent=team/sarah
→ 200 { "access_token": "<short-lived>", "expires_in": 3600 }
→ 401 { "error": "no_grant", "fix": "claws auth team/sarah codex" }
→ 503 { "error": "refresh_failed", "detail": "..." }

POST /v1/grant
  body: { "provider": "openai-codex", "agent": "team/sarah",
          "refresh_token": "<from OAuth flow>" }
→ 200 { "ok": true }
→ 409 { "error": "grant_exists" }  (use `claws auth ... --force`)

GET /v1/audit?agent=team/sarah&since=2h
→ 200 [ { ts, provider, action, ok, ... }, ... ]

GET /v1/health
→ 200 { "providers": { "openai-codex": "ok", "anthropic": "ok" } }
```

No JSON-RPC, no gRPC, no protobuf. HTTP over unix socket because
every language can speak it and the runtime already does HTTP.

### Container side — how the openclaw runtime gets a token

The hard part. Today the runtime reads `~/.codex/auth.json`. Two
adapter paths:

**Path A — Drop-in replacement (preferred long term).** The runtime
gains a `--credential-source=broker|file` flag. When `broker`, it
calls the unix socket on every model request instead of reading the
file. Cleanest, but requires a runtime contract change — coordination
with the openclaw maintainers.

**Path B — File-backed shim (ship first).** A tiny in-container
sidecar process (~50 LOC) that the broker pushes fresh access tokens
into via the socket. The sidecar writes them to a tmpfs-backed
`~/.codex/auth.json` shape that mimics what the runtime expects.
Runtime keeps reading the file; the file just always has a fresh
token. No runtime changes. Doesn't get the "agent never holds the
token" property — the access token lives in container memory briefly
— but the **refresh** token never enters the container, which is the
critical property.

Recommendation: **ship path B first**. It's a strict improvement
(refresh chain singularity), it doesn't block on runtime cooperation,
and migration to path A later is straightforward.

### Storage

`/var/lib/claws-credd/store.db` — SQLite single file.

- `grants(provider, agent, refresh_token_encrypted, created_at,
   last_refreshed_at)` — refresh tokens, encrypted at rest with a key
   derived from a machine-bound source (TPM if available, fallback to
   age-encrypted file under `/root/.config/claws-credd/key`).
- `audit(ts, provider, agent, action, ok, error)` — every fetch /
   refresh / grant / revoke. Append-only.
- `leases(provider, agent, last_fetched_at, fetch_count_24h)` — for
   rate-limit visibility + future quota enforcement.

Encryption at rest is not "real" defense against root on the host
(root reads the key file). It IS defense against backup theft,
casual `cat`, accidental log inclusion, and curl-able paths.

### Process model

- Systemd service: `claws-credd.service`. `Type=notify`,
  `RestartSec=2s`, `LimitNOFILE=1024`.
- Runs as a dedicated unprivileged user (`claws-credd`). The socket
  is owned by that user with group `claws-agent` (or similar);
  containers mount the socket via bind mount.
- Refresh worker fires per chain on a schedule (e.g. 15 min before
  expiry) — pro-active, not reactive. Eliminates the
  "every-call-is-a-coin-flip" pattern current OAuth refresh has.

---

## Module shape

```
github.com/ab0t-com/cred-broker/        (new repo)
├── README.md                            (extraction story, security model)
├── LICENSE
├── go.mod
├── cmd/
│   ├── claws-credd/                    (the daemon binary)
│   │   └── main.go
│   └── claws-credctl/                  (operator-side CLI for the broker)
│       └── main.go                     (grant / revoke / audit / status)
├── internal/
│   ├── server/                         (HTTP-over-unix-socket handler)
│   ├── store/                          (SQLite + encryption-at-rest)
│   ├── refresh/                        (per-provider refresh logic)
│   │   ├── openai_codex.go
│   │   ├── anthropic.go
│   │   ├── openrouter.go
│   │   └── ...
│   ├── audit/                          (append-only event log)
│   └── policy/                         (rate-limit, per-agent quota)
└── pkg/
    └── client/                         (the import that claws + sharedwatch use)
        ├── client.go                   (GetToken, Grant, Revoke, Audit)
        └── doc.go
```

`pkg/client/` is the only public package — same posture as the
hints package. Internal/* is the daemon's implementation.

### claws's view

```go
import "github.com/ab0t-com/cred-broker/pkg/client"

// In cmdStart, after healthcheck:
broker := client.New(brokerSocketPath())
if err := broker.HealthCheck(provider); err != nil {
    warn("credential broker reports %s as unhealthy: %v", provider, err)
}
```

No claws code holds refresh tokens. The shim sidecar (path B above) lives
in the openclaw image and is also a `cred-broker` client. claws's
involvement is purely: install/start the daemon, run the OAuth flow
to mint the initial refresh token, write the grant to the broker via
`claws auth <name> codex` (which becomes "open OAuth flow, then POST
/v1/grant to the broker"), and consume `/v1/health` for diagnostics.

---

## Security model

### What we defend against

| Threat | Mitigation |
|---|---|
| Refresh token leaks via container exec / compromised agent | Refresh token never enters the container in path B (and in path A, never enters at all). Containers only see short-lived access tokens. |
| `refresh_token_reused` collision | Single refresher per chain (the broker). |
| Multi-tenant misuse — one agent's compromise reads another's tokens | Per-agent grants; broker enforces `agent` claim on every fetch. |
| Backup-tape exfiltration | Refresh tokens encrypted at rest. |
| Operator typo (`claws auth wrong-name`) | Audit log; revocation per agent. |

### What we explicitly do NOT defend against

- **Root on the host.** Root can read the broker's key file, snapshot
  memory, attach a debugger. Same as `ssh-agent`. Out of scope.
- **A malicious openclaw runtime image.** If the runtime image is
  compromised, the access tokens it briefly holds are exfiltratable.
  Mitigated by short-lived access tokens (typically 1 hour) and
  per-agent revocation; not eliminated.
- **Network adversary between broker and provider.** Standard HTTPS
  + cert pinning to provider domains.
- **Cross-host theft.** Broker is per-host; cross-host federation
  is a separate ticket (see §Non-goals).

### Principle of least authority

The broker daemon runs as a dedicated unprivileged user. It needs:

- Read/write `/var/lib/claws-credd/store.db`
- Read `/root/.config/claws-credd/key` (or TPM access)
- Listen on `/var/run/claws-credd.sock`
- Outbound HTTPS to provider domains only (use a per-provider allowlist
  enforced via `nftables` egress rules at install time when possible)

It does **not** need: docker socket access, sudo, root, the host
filesystem outside its own dirs, the network outside provider
domains.

---

## Implementation phases

### Phase 0 — Spike (1-2 days)

- Bare daemon: HTTP-over-unix-socket, in-memory store, openai-codex
  refresh only.
- Stub `claws auth team/sarah codex` to POST the grant to the daemon.
- File-backed shim in a custom test image so sarah's container reads
  fresh tokens from `~/.codex/auth.json` written by the shim.
- Goal: prove the path B shape end-to-end against one provider.

### Phase 1 — Production daemon (1 week)

- SQLite store + encryption at rest.
- Systemd unit.
- All three current providers: openai-codex, anthropic, openrouter.
- `claws auth ... codex` writes grants to the broker.
- `claws-credctl audit / grant / revoke / status`.
- Tests: fake provider server in pkg/client for integration tests.
- Extract `cred-broker` to its own repo + module from the start —
  same lift policy as the hints package.

### Phase 2 — Coordinated runtime change (collaborate with openclaw maintainers)

- Runtime gains `--credential-source=broker` flag (path A).
- Removes the file-shim from path B; runtime calls the broker socket
  directly.
- Migration: existing file-mode installations keep working; new
  agents default to broker.

### Phase 3 — Operator-facing polish

- `claws auth diagnose <name>` (also useful before the broker exists —
  could ship in v1.6.x to detect the shared-mount root cause).
- Per-agent revocation: `claws auth revoke team/john`.
- Per-agent quota visibility: `claws auth budget team/john`.
- Audit roll-up under `claws audit` (already exists; adds broker
  events).

### Phase 4 — Optional: hardware-backed key (TPM / Secure Enclave)

- Encryption key derived from TPM-sealed secret on Linux.
- Mac: Keychain item gated by Touch ID for unlock.
- Best-effort; falls back to age-encrypted key file.

---

## Open questions

1. **Does the openclaw runtime have a "credential source" abstraction
   yet?** If yes, path A becomes the day-1 target and path B is
   skippable. If no, we ship path B and need to coordinate the
   abstraction for path A. Need to ask the runtime author.

2. **Can the broker run inside a container itself** (so users without
   systemd — macOS, WSL — get the same thing via docker-compose)?
   Probably yes; the unix socket would live on a shared bind-mount
   instead of `/var/run/`. Worth verifying that
   `getpeercred(2)`-style peer-pid identification still works through
   a docker mount.

3. **How do we handle provider-specific quirks?** E.g. OpenAI Codex
   sometimes returns 429 on refresh during rate-limit windows; we
   need backoff that doesn't blast the chain. Per-provider
   `refresh/` files isolate this.

4. **Cross-language clients.** The openclaw runtime is Node.js. If
   path A lands, the runtime needs a Go client equivalent in Node.
   Should the broker speak a contract simple enough that any HTTP
   client can do it (yes — that's why we picked HTTP/unix), without
   a generated SDK?

5. **Existing API-key auth.** The broker should also store API keys
   for symmetry. They don't need refresh, but the audit / per-agent
   revocation story applies to them too. Easy to add; just treat
   `apikey` as a provider with a no-op refresh strategy.

6. **What about WhatsApp's session tokens?** Different shape
   (Baileys-style session blobs, not OAuth). Could be a separate
   credential class the broker handles, or stay container-local
   because the WhatsApp daemon needs to write session updates back.
   Probably the latter — out of scope for v1 of the broker.

---

## Acceptance criteria

- A fleet of 5 agents on one host, all authenticated against the
  same ChatGPT account via Codex OAuth, can run continuously for
  ≥7 days without any agent hitting `refresh_token_reused`.
- `claws auth team/sarah codex` opens the OAuth flow, completes, and
  the grant lands in the broker. Subsequent `claws auth status
  team/sarah` shows the grant; `claws-credctl audit` shows the event.
- Killing the broker daemon: each agent's next model call surfaces a
  clear "broker offline" error within the model-fallback path; agents
  do not silently fail.
- Restarting the broker daemon: agents resume cleanly within one
  model-call cycle.
- A grant revoked via `claws auth revoke team/sarah` causes sarah's
  next model call to fail explicitly (no zombie access-token reuse
  beyond the access token's natural TTL).
- The broker module is published as `github.com/ab0t-com/cred-broker`
  with its own tests, its own CHANGELOG, and at least one consumer
  outside claws (sharedwatch, intent-gateway, or a stub demo client).

---

## Risks

- **Coordination tax with the openclaw runtime maintainers** for
  path A. Mitigated by shipping path B first (no coordination
  needed) and treating path A as a follow-up.
- **Adoption friction.** Operators today are used to "claws auth
  works, agent gets a token". The broker adds a daemon to install,
  start, and monitor. Mitigated by `install.sh` and `claws setup`
  installing + starting the broker automatically; operator never
  needs to know it's there for the happy path.
- **Single point of failure on the host.** Mitigated by:
  - Systemd auto-restart
  - Pre-refresh (broker keeps a fresh access token cached for each
    grant, so a daemon hiccup doesn't immediately fail every agent)
  - Clear failure mode (every error path returns a structured
    "broker unavailable" message — better than "auth mysteriously
    failed")
- **Scope creep into "claws's secrets manager".** Explicit non-goal,
  but easy to drift into. Each addition needs to be argued against
  "would `pass` or `1password-cli` do this just as well?".

---

## Background reading

- OAuth 2.0 Refresh Token Rotation (RFC 6749 §6 + the OAuth Security
  BCP) — refresh-token-reuse detection is mandatory for confidential
  clients per OAuth Security BCP §4.13.
- Token Exchange (RFC 8693) — the formal protocol for "broker mints
  short-lived tokens on demand". We don't need the full RFC 8693
  surface but the conceptual model is identical.
- GCE metadata server / AWS IMDSv2 / HashiCorp Vault transit — three
  shipped implementations of "single refresher, many consumers".
- ssh-agent, gpg-agent — UDS + getpeercred peer identification.
- `cmd/claws/hints/` README (this repo) — the
  "lift-then-extract-on-third-consumer" module discipline we should
  apply here from day one (just skip the lift step and start
  extracted, since we already know cred-broker is multi-consumer
  from inception).

---

## Out of this ticket

- The actual immediate fix on the user's host: re-run
  `claws auth team/john codex` (and same for other affected
  agents). v1.6.13's startup check now flags broken auth so the
  re-auth loop is visible. Doesn't fix the root cause; the broker
  does.

- `claws auth diagnose <name>` — read-only "is this shared with
  another agent" detector. Could ship in v1.6.14 ahead of the broker
  to give operators visibility into the failure mode.
