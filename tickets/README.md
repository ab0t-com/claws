# Tickets

## Index

| # | Title | Priority | Status | Depends On |
|---|-------|----------|--------|------------|
| 1 | [Security Hardening](ticket_1_security-hardening.md) | P0 Critical | Open | — |
| 2 | [Admin Policy Layer](ticket_2_admin-policy.md) | P1 High | Open | — |
| 3 | [Runtime Adapter Pattern](ticket_3_runtime-adapter.md) | P2 Medium | Open | — |
| 4 | [Multi-Tenant Access Control](ticket_4_multi-tenant.md) | P2 Medium | Open | Ticket 2 |
| 5 | [Image Management](ticket_5_image-management.md) | P2 Medium | Open | — |
| 6 | [Security Matrix & Audit](ticket_6_security-matrix.md) | P1 High | Partial | — |

## Context

- [2026-03-25 Context Dump](2026-03-25_context.md) — full system state, security findings, exposure analysis

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
