# PMM & Open-Source Readiness — clawctl

**Date.** 2026-05-20
**Perspective.** Product Marketing Manager — positioning, ICP, JTBD, GTM, OSS launch readiness, naming, risk.
**Scope.** Reframes the codebase from the operator's and the prospective contributor's points of view, then issues a concrete go/no-go on "open it up."
**Source artifacts honored.** [PMM review 2026-03-17](report_pmm-review_2026-03-17.md), [PMM audit 2026-03-27](report_pmm-audit_2026-03-27.md), [engineering structural map (companion)](report_engineering-structural_2026-05-20.md), [dogfood log](../tests/dogfood_log_2026-05-20.md).

---

## 0. TL;DR for the decision

**Question.** "We want to open it up, but we don't know what it does."

**Answer.**

- **What it does:** clawctl is a single-binary command line that turns one Linux server into a private multi-tenant home for **1-8 messaging-connected AI agents**. Each agent is an isolated Docker container with its own identity, credentials, workspace, channels (WhatsApp/Telegram/Discord/Slack/Signal), and security policy. Optionally, agents form teams with a shared workspace and a manager/worker task queue.
- **Who it's for:** the operator who already self-hosts things (Caddy, syncthing, Tailscale, nextcloud) and who wants to run their own AI agents instead of paying for an opaque SaaS. **Not** a SaaS replacement for the casual user; **not** Kubernetes for agent fleets.
- **Should you open-source it?** **Yes, with one short hardening sprint.** The product is real (verified end-to-end in [dogfood log](../tests/dogfood_log_2026-05-20.md)). The remaining blockers are repository hygiene (LICENSE, CONTRIBUTING, CoC, SECURITY, CI) and a positioning page — not product features.
- **What to call it (positioning).** "The control panel for your private AI agents" or "Run your own AI team on one server." Lean into self-host, privacy, identity-per-agent, and the messaging-channel surface. Don't position against Kubernetes; position against "the SaaS that has your WhatsApp messages."

---

## 1. Product framing (the one-pager)

### 1.1 Positioning statement

> **For** self-hosting operators and small AI-native teams **who** want to run a fleet of personal/role-based AI agents on their own server **without** outsourcing identity, conversations, or credentials,
> **clawctl** is a single-binary control plane that orchestrates 1-8 isolated agents on one host,
> **unlike** Kubernetes or cloud agent platforms that either over-engineer the problem or hand your data to a vendor,
> **clawctl** runs entirely on plain files + Docker Compose, ships safe-by-default security, and connects each agent to WhatsApp / Telegram / Discord / Slack / Signal with consent-based DM pairing.

### 1.2 ICP — three concentric circles

| Tier | Persona | Why they care |
|------|---------|---------------|
| **Bullseye** | Self-hosting indie operator (homelab, founder, "AI tinkerer") | Already runs Caddy, Tailscale, syncthing; messaging agents are next; wants a sane CLI not a Helm chart |
| **Adjacent** | 2-10 person teams running internal AI agents (one per function: support, dev, ops, sales) | Need isolation between agents, shared knowledge between them, and an audit trail |
| **Aspirational** | Small consultancies or "AI-first" boutiques deploying client agents on per-tenant boxes | Need multi-tenant access control and the runtime adapter (different agent stacks per client) |

What clawctl is **not** for: large fleets (>8 instances per host), multi-host orchestration, public cloud auto-scaling, anyone who needs a UI today (no web UI ships in-repo, only an `INTEGRATION_ANALYSIS.md`-style landing HTML and a `dashboard` TUI).

### 1.3 Jobs to be done (in operator language)

| Job | clawctl command(s) | How it shows up in marketing |
|-----|--------------------|------------------------------|
| "Stand up a private AI agent on my own server in 5 minutes" | `clawctl setup` | **The one-command quickstart.** |
| "Give my Mum / customer / colleague their own agent that messages them on WhatsApp" | `create` + `channel add whatsapp` + `approve` | **"Each agent is reachable on the platform people already use."** |
| "Run a team of agents that share a workspace and dispatch tasks to each other" | `team create` + `--role=manager|worker` + `task create/claim/complete` | **"Multi-agent coordination without a message bus."** |
| "Lock down what these agents can do (channels, networking, who can talk to them)" | `policy init` + `policy enforce --restart` + `channel security` | **"Safe by default. Verifiable by audit."** |
| "Let my deploy bot run lifecycle commands but not edit configs" | `access init` + `access grant` | **"RBAC over the CLI itself, not just the API."** |
| "Back this up nightly and restore on a new box" | `backup` + `storage sync` + `restore` | **"Files in, files out. No vendor lock-in."** |
| "Drive a non-OpenClaw container with the same control plane" | `runtime add --from=openclaw` or `runtime init` | **"Bring your own agent runtime."** |

### 1.4 Pricing / commercial model (if applicable)

OSS launch implies **free** under whichever license you pick (BSL-style or Apache-2.0 are the two reasonable choices — Apache-2.0 if you want maximum contributor reach; BSL/Elastic-style if you ever want to monetize a managed offering). Future commercial wedges, listed for the user's awareness, not as a recommendation today:

- Managed control plane (clawctl-as-a-service over your boxes via SSH/agent push)
- Hardened runtime images, signed
- Multi-host orchestrator (a second SKU)
- "Tonic" — premium support and incident SLAs for self-host operators

These are *not* near-term decisions. Just inventorying optionality.

---

## 2. Object model — what users need to grok

The 2026-03-27 PMM audit nailed this: today's clawctl exposes **eight** primary objects (Instance, Group, Role, Runtime, Channel, Policy, Access, Task). For a casual operator, that's at least five too many. The `setup` command was the right intervention — it lets a new user think only about three:

- **Team** (= group, hidden from new users)
- **Agent** (= instance, with auth + a channel)
- **Channel** (the messaging platform connection)

Anything else (runtime, policy, access, task, role) is *defaulted intelligently* and only surfaced when needed. Marketing copy should keep this discipline: never mention "instance," "group," "runtime," "role" in the first paragraph of any landing page. Lead with "agent" and "team."

> **Recommendation:** add a `clawctl --explain` or a `concepts` topic to `help` that walks through the three-object mental model for new users. Today's `help groups` topic already does this for the team/agent piece — extend it.

---

## 3. Market context

### 3.1 Adjacent solutions

| Category | Examples | How clawctl differs |
|----------|----------|---------------------|
| **Multi-VM-on-one-host CLIs** | `multipass`, `lxc/lxd`, `podman quadlet` | Same shape (instance lifecycle on one box), but those are generic VMs/containers — no first-class agent identity, no channel integration, no policy on tools/DMs |
| **Agent-runner toolkits** | LangGraph, AutoGen, CrewAI, Letta | Those are *frameworks* the agent author uses. clawctl is the *operational layer* underneath the framework. Complementary, not competitive |
| **Hosted agent platforms** | OpenAI Assistants, Anthropic Workbench, Vercel AI SDK hosted | Hosted = vendor owns conversations + credentials + storage. clawctl is the self-hosted opposite |
| **K8s/Nomad-class orchestrators** | Kubernetes operators (e.g., Argo, kagent), Nomad jobs | Multi-host, multi-tenant, multi-cluster. clawctl is deliberately one-host, fleet ≤ 8 — *that's the whole point* |
| **Self-host home-automation CLIs** | Home Assistant supervisor, Nextcloud `occ`, `tailscale` CLI | Closest spiritual cousins. Same operator persona, same self-host ethos. clawctl borrows the file-based-state + tiered-help patterns |

### 3.2 Where clawctl is uniquely positioned

The combination of {single host, ≤ 8 instances, file-based state, zero external Go deps, messaging-first channels, RBAC over the CLI, runtime-adapter pattern, safe-by-default security} does not exist in any single competing tool. This is a *category-defining* small tool — but only in the niche of "self-hosted private AI agents on one box."

### 3.3 What it should not try to be

- Not a "AI agent Kubernetes." Cap is 8, period. Pushing past that demands a real registry and inter-host communication.
- Not a generic message queue or workflow engine. The task queue is a coordination primitive between agents in the same group, not an Airflow.
- Not a hosted product. The whole value prop is self-host.

---

## 4. Open-source launch readiness checklist

Scored against typical OSS launch criteria.

| Criterion | Status | Action |
|---|---|---|
| **README** | ✅ Concise, accurate, has Quick Start. | None — current README is publication quality. |
| **LICENSE** | ❌ **Missing** from working tree. | **Blocker.** Pick Apache-2.0 (max-reach) or BSL (preserve commercial optionality). Add. |
| **CONTRIBUTING.md** | ❌ Missing. | Write a 1-page contributor guide: build, test (`go test ./...`), branch flow, conventional commit hint, where to file issues. |
| **CODE_OF_CONDUCT.md** | ❌ Missing. | Standard Contributor Covenant. Copy-paste. |
| **SECURITY.md** | ❌ Missing. | "Report vulns to <email>. We take secret leakage and command injection seriously. PGP key: …" Critical for a tool that handles channel tokens. |
| **CHANGELOG.md** | ❌ Missing. | Start one at the launch tag. |
| **CI** | ❌ No GitHub Actions in repo. | Add `.github/workflows/ci.yml` that runs `go test ./...` and `go vet`. Optionally `gitleaks` (already wired in `scripts/install-hooks.sh`). |
| **Reproducible build / release pipeline** | 🟡 `scripts/release.sh` builds 5 targets locally; no automation. | Tag-triggered Action that runs `release.sh` and uploads to GitHub Releases. |
| **Install path** | 🟡 `scripts/install.sh` ready (assumes GitHub Releases). | Stand up the release artifacts first. |
| **Project page / landing** | 🟡 `landing.html`/`landing2.html` and `console.html` exist in working tree, untracked. | Cull to one. Ship as `docs/index.html` or move to a separate `clawctl-site` repo. |
| **Docs site** | 🟡 `docs/channels.md`, `docs/runtimes.md` are good. | Add `docs/concepts.md` (3-object mental model), `docs/file-formats.md` (the on-disk schemas — see engineering report §9 item 2), and `docs/security.md` (consolidate `security` help topic + audit-script behavior). |
| **Examples** | ❌ No `examples/` dir. | Add three: (1) one-agent quickstart (`setup --non-interactive` script), (2) two-person team with shared workspace, (3) custom runtime (Python agent). |
| **Issue + PR templates** | ❌ Missing. | Standard `.github/ISSUE_TEMPLATE/*.md` (bug, feature, security-redirect) + `PULL_REQUEST_TEMPLATE.md`. |
| **Branding** | 🟡 Three landing HTMLs exist; "clawctl" / "openclaw" relationship is unclear in the README. | Settle this *before* publishing: is clawctl an OpenClaw subproject, a sibling tool, or independent? Today the binary lives outside the OpenClaw repo. Treat as "clawctl: control plane for OpenClaw (and other) agent runtimes." |
| **Demo / screencast** | ❌ None recorded. | One asciicast of `clawctl setup` from blank box to Telegram message. <60s. |
| **Roadmap** | 🟡 Implicit in `tickets/` and reports. | Move to a public `ROADMAP.md` so contributors see the direction. |
| **Code quality bar** | ✅ `go test ./...` green, 232 tests, `go vet` clean, zero external Go deps. | None. |
| **Security defaults** | ✅ Loopback default, 0600 perms, cap_drop ALL, DM pairing, audit on, outbound off. | None — these *are* the headline. |
| **Drift between docs and binary** | ✅ Zero observed in dogfood. | None. |
| **Naming** | 🟡 "Instance" in code vs "agent" in docs; "team"/"group" used interchangeably. | Live with it for v0; flag as "known cosmetic" in CHANGELOG. |

**Bottom line:** **9 must-fix items** (LICENSE, CONTRIBUTING, CoC, SECURITY, CHANGELOG, CI workflow, release pipeline, issue/PR templates, branding decision). All low effort. Total estimated work: **1.5 days for one person.**

---

## 5. GTM — what to do on launch day

### 5.1 The minimum viable launch

1. **GitHub repo public** with the 9 hygiene items above.
2. **One landing page** at e.g. `clawctl.dev` (use one of the existing `landing*.html` mockups; pick best, ship). Three sections: hero ("Run your own AI team on one server"), the `clawctl setup` asciicast, three quickstart commands. Link to GitHub.
3. **One blog post** — 1500 words. Hook: "We didn't want a hosted assistant. We wanted a server full of agents we could trust." Walk through `setup`, the safe defaults, and the runtime adapter.
4. **One Hacker News submission** ("Show HN: clawctl — multi-instance AI agent manager for one server"). Mention zero deps, file-based state, runtime adapter.
5. **One asciicast** (asciinema), one demo video (≤2 min, no editing required), embedded in landing and blog.
6. **One reference deployment** — Hetzner / DigitalOcean playbook (Ansible or just a shell script) that goes from a fresh Ubuntu box to a running `team/sarah` agent in 5 minutes. This is the *proof* the README's promise is real.

### 5.2 Channels for distribution

- **Self-host community**: r/selfhosted, lobste.rs, awesome-selfhosted PR.
- **AI builder community**: Hacker News, r/LocalLLaMA (positioning: "infra for your local models that have to face the world"), Latent Space podcast cold pitch.
- **Mastodon/Bluesky**: link the demo video; tag self-host and AI tooling community.
- **OpenClaw repo cross-link**: README of OpenClaw points at clawctl as the recommended way to run "more than one openclaw instance."

### 5.3 Success metrics for the first 30 days

| Metric | Target | Why this number |
|---|---:|---|
| GitHub stars | 200+ | Indicates the niche found you |
| `clawctl setup` invocations (anonymized opt-in telemetry, optional) | n/a unless you add it | Don't add telemetry for v0 — wrong tone for the persona |
| Open-source issues filed by non-staff | 5+ | Validates that strangers can use it |
| External contributors (≥1 merged PR) | 1-2 | Pulse check on community formation |
| Cited in 1 "best self-host AI" listicle | 1 | Distribution unlock |
| Referenced by 1 other tool (e.g., an agent framework's docs) | 1 | Ecosystem signal |

If you hit 4 of 6 in 30 days, you have a project. If you hit 1, the positioning needs to move (probably away from "self-host" toward "managed for small teams").

---

## 6. Risks (PMM eye, not engineering eye)

### 6.1 Naming and identity

- **clawctl vs OpenClaw.** Most operators encountering the GitHub repo won't have heard of OpenClaw. The README assumes familiarity. Either: (a) embed a 3-line "what's OpenClaw?" in the README and the landing page, or (b) reposition clawctl as "the multi-agent control plane (runs OpenClaw by default)" so the relationship is explicit.
- **Trademarks / pre-existing names.** Quick web due diligence on "clawctl," "openclaw," and "claw" before publishing. Avoid surprises.

### 6.2 Compliance and platform-policy risk

The product wires AI agents to **WhatsApp, Telegram, Discord, Slack, Signal**. Each has TOS implications when you build automation on top:

- **WhatsApp** via Baileys (the underlying library OpenClaw uses) is *unofficial*. Meta has banned numbers running it. Operators take this risk individually; clawctl should document the risk on `docs/channels.md` and on the WhatsApp section of the landing page — *not bury it*.
- **Telegram, Discord, Slack** have official bot APIs and are fine.
- **Signal** uses signal-cli, also unofficial but tolerated.

**Recommended:** add a "Channel risk matrix" to `docs/channels.md` distinguishing official-API channels (low risk) from web-API channels (terms-of-service risk on the operator).

### 6.3 Security responsibility

clawctl by design holds messaging tokens (very sensitive — full takeover of a person's messaging channel), gateway tokens, and potentially API keys (Anthropic, OpenAI, Codex). If a bad release ever logs a token to stdout, it's a national headline. **Mitigations already in place:** config masking in `config show --no-secrets`, token truncation in `status`, 0600 file mode, audit script that checks credential perms. **Add for v1:**

- A SECURITY.md that explains the threat model in plain English.
- A pre-commit gitleaks hook *enabled by default* (the hook script exists; install it in `scripts/install-hooks.sh` and document).
- A `clawctl --version`-included build provenance (commit SHA, build time) so users can verify they're on the version they think they are.

### 6.4 Maintenance load

Going public introduces an **issue stream** that the maintainer must field. clawctl's surface is broad (~50 commands, 8 channels, 6 capability axes per runtime, S3/proxy/cron integrations). One person on a side project can be overwhelmed in 30 days. Mitigation:

- Issue templates that triage by component (channels / runtime / lifecycle / policy / docs).
- Explicit "known limitations" list in README and `docs/concepts.md`: 1-host only, ≤ 8 instances, no Windows, requires Docker Compose v2, no support for FUSE-mounted task queues.
- "Stable surface" list — explicitly mark what's stable (the file formats) vs unstable (CLI flags subject to change).

### 6.5 Drift between the README and a moving codebase

Today there is zero drift (verified). Six months from now, with a contributor community, that won't be free. **Recommended:** add a doc-test that grep-runs every code-block in `README.md` to the smoke harness. If a command in the README doesn't exist in the binary, CI fails. Small change; saves embarrassment.

---

## 7. What the dogfood log changes about this report

The full dogfood log is at [`tests/dogfood_log_2026-05-20.md`](../tests/dogfood_log_2026-05-20.md). The relevant PMM-flavored takeaways:

1. **The `setup` flow really does work as advertised.** A new operator can go zero-to-running-team in one command. This *is* the marketing line; it's not aspirational.
2. **Audit script's scope leak (S2 in dogfood log)** is the kind of thing that, the first time a contributor sees it, makes them think "this is sloppy." Fix before publication — it's a 10-minute filter on `docker ps` output.
3. **JSON-parity gap (dogfood S3)** matters more for OSS than for internal use. The first contributor who tries to wire clawctl into a dashboard will hit this within ten minutes. Worth landing before launch.
4. **`gateway.bind` cosmetic issue (S1)** is exactly the kind of thing security-curious operators write blog posts about. "I configured loopback but `config show` says lan!" Even though the *runtime* binds correctly, the optics are bad. Fix is small.

---

## 8. Final recommendation, with sequencing

1. **This week**: ship the 9 hygiene items (§4), settle the clawctl/OpenClaw branding question, fix dogfood findings S1/S2/S3/S5 (engineering report §9 items 3 + 6).
2. **Next week**: build the landing page (pick one of the three HTML mocks), record the asciicast and demo, draft the launch post and ROADMAP.md.
3. **Week 3**: soft-launch — share with 5 self-host operators you trust, get fresh-eye feedback, iterate on language.
4. **Week 4**: public launch — repo public, blog post live, HN submission, r/selfhosted post.

This is a small, defensible launch for a product that is real and that fills a real category gap. The 30-day metrics in §5.3 will tell you whether to invest further or treat it as a useful in-house tool you happen to publish.

---

*End of PMM report. Companion: [engineering structural map](report_engineering-structural_2026-05-20.md). Evidence: [test plan](../tests/test_plan_2026-05-20.md) + [dogfood log](../tests/dogfood_log_2026-05-20.md).*
