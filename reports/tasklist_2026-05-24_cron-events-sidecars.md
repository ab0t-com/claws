# Tasklist — cron + events + sidecars + topology (v1.5.0)

**Ticket:** `tickets/cron-events-sidecars-topology-2026-05-24/ticket.md`
**Filed:** 2026-05-24

| ID | Task | Files | Status |
|---|---|---|---|
| A1 | `agents[].cron[]` schema + validation (schedule format) | `cmd/claws/apply.go` | TODO |
| A2 | applyCron writes to runtime's CronDir, format-aware | `cmd/claws/apply.go` (or new cron.go) | TODO |
| A3 | Runtime.{CronDir, CronFormat, SupportsCron} + openclaw defaults | `cmd/claws/runtime.go` | TODO |
| A4 | Validation: invalid schedule → reject parse | `cmd/claws/apply.go` | TODO |
| B1 | `agents[].events` schema (enabled, digestMode, endpoint, allowFromIps) | `cmd/claws/apply.go` | TODO |
| B2 | applyEvents writes to openclaw.json via cmdConfig set | `cmd/claws/apply.go` | TODO |
| B3 | Runtime.Capabilities.Events flag | `cmd/claws/runtime.go` | TODO |
| C1 | `sidecars: [{name, kind, config}]` top-level + per-agent | `cmd/claws/apply.go` | TODO |
| C2 | applySidecars writes integration config + auto-injects onStart hook | `cmd/claws/apply.go` (or sidecars.go) | TODO |
| C3 | Sidecar registry — built-in: sharedwatch, intent-gateway | `cmd/claws/sidecars.go` (new) | TODO |
| D1 | `agents[].peers` schema | `cmd/claws/apply.go` | TODO |
| D2 | Multi-level manager chains (already partially supported) | apply.go | TODO |
| D3 | applyTopology writes per-agent topology.json | `cmd/claws/apply.go` | TODO |
| D4 | Cycle detection at parse time | `cmd/claws/apply.go` | TODO |
| D5 | New: `templates/teams/multi-tier.json` | new | TODO |
| D6 | New: `templates/teams/specialist-mesh.json` | new | TODO |
| E1 | Coverage baseline reported | — | DONE (9.5%) |
| E2 | Unit tests for new helpers (cron, events, sidecars, topology) | `*_test.go` | TODO |
| F1 | `teams/research-trio.json` adds cron block | template | TODO |
| F2 | `specialty/knowledge-base.json` adds sharedwatch sidecar | template | TODO |
| F3 | `specialty/oncall-rotation.json` adds events block | template | TODO |
| G1 | templates/README.md updated for new fields | docs | TODO |
| G2 | Sidecar integration section in README | docs | TODO |
| G3 | CHANGELOG v1.5.0 | docs | TODO |
| G4 | Cut release v1.5.0 | release | TODO |

## Execution order

1. **A1–A5** Cron — self-contained, well-defined
2. **B1–B3** Events — small, just a config block
3. **C1–C3** Sidecars — biggest, has external integration shape
4. **D1–D6** Topology — schema + 2 new templates
5. **E2 + F1–F4** Tests + template updates (parallel)
6. **G1–G4** Docs + release
