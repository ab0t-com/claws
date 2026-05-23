# Tickets

## Index

| # | Title | Priority | Status | Depends On |
|---|-------|----------|--------|------------|
| 1 | [Security Hardening](ticket_1_security-hardening.md) | P0 Critical | **Done** | — |
| 2 | [Admin Policy Layer](ticket_2_admin-policy.md) | P1 High | **Done** | — |
| 3 | [Runtime Adapter Pattern](ticket_3_runtime-adapter.md) | P2 Medium | **Done** | — |
| 4 | [Multi-Tenant Access Control](ticket_4_multi-tenant.md) | P2 Medium | **Done** | Ticket 2 |
| 5 | [Image Management](ticket_5_image-management.md) | P2 Medium | **Done** | — |
| 6 | [Security Matrix & Audit](ticket_6_security-matrix.md) | P1 High | **Done** | — |
| 7 | [Runtime UX](ticket_7_runtime-ux.md) | P1 High | **Done** | Ticket 3 |
| 8 | [One-Click Onboarding](ticket_8_onboarding.md) | P0 Critical | Open | All (1-7) |
| 9 | [Health Probe Loopback Bind](health-probe-loopback-bind-2026-05-23/ticket.md) | P1 High | **Done** (fix in repo; migration to live needs operator restart — see worklog) | — |
| 10 | [Fleet & Team Control Surface](fleet-team-control-surface-2026-05-23/ticket.md) | P1 High | **Done** (11/14 shipped, 3 deferred — see worklog) | — |
| 11 | [Per-instance Auth Verify + Reliable Reauth](auth-fleet-reauth-2026-05-23/ticket.md) | P0 Critical | **Done** (V1 shipped; rate-limit deferred — see worklog) | OpenClaw runtime cooperation for full Strategy A |
| 12 | [Test-harness Orphan Containers](test-harness-orphan-containers-2026-05-23/ticket.md) | P1 High | **Done** (cleanup hook + dedup; live end-to-end validation deferred to operator) | — |
| 13 | [Logs Interleaved Follow](logs-interleaved-follow-2026-05-23/ticket.md) | P2 Medium | **Done** (real-Docker end-to-end deferred to operator) | — |
| 14 | [Errors Umbrella Command](errors-umbrella-2026-05-23/ticket.md) | P2 Medium | **Done** | — |
| 15 | [Drift Reverse + Umbrella](drift-reverse-and-umbrella-2026-05-23/ticket.md) | P3 Low | **Done** | — |

## Context

- [2026-03-25 Context Dump](2026-03-25_context.md) — full system state, security findings, exposure analysis
- 2026-05-23 incident: original operator complaint was "agents have OpenAI auth issues on Telegram" — needed detection *and* a fix loop the operator could trust. Ticket 10 shipped the visibility surface (`auth status`, `channels`, `list --rich`, `logs --grep`); ticket 11 captures the actual remaining gap — a per-instance `auth verify` primitive and idempotent reauth that doesn't lie about success. The earlier draft of ticket 11 included bulk team-reauth; that was rescoped out as premature aggregation (no observed use case for shared credentials across many agents yet). Tickets 9, 12-15 are sibling follow-ups surfaced during the same triage.

## Tools

- `scripts/security-audit.sh` — automated security checklist (7 categories, 50+ checks)
- `scripts/install-hooks.sh` — gitleaks pre-commit/pre-push hooks

## Execution Order

1. **Ticket 1** — fix immediate security exposure (P0, can ship today)
2. **Ticket 6** — security matrix is documented, audit script exists, needs integration with doctor
3. **Ticket 2** — admin policy layer (foundation for tickets 4 and 5)
4. **Ticket 5** — image management (independent, unblocks multi-version deployments)
5. **Ticket 4** — multi-tenant access control (needs policy layer from ticket 2)
6. **Ticket 3** — runtime adapter (largest refactor, lowest urgency)
