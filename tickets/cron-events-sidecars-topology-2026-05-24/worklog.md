# cron + events + sidecars + topology — worklog

Append-only.

---

## 2026-05-24 — Kickoff

- Filed after v1.4.0 dogfood. Four asks:
  - Cron section (periodic actions)
  - Event injection / digest endpoint (intent-gateway pairing)
  - First-class sidecar helpers (sharedwatch, intent-gateway, future)
  - Topological team structures (multi-tier hierarchies, peer meshes)
- Coverage audit: pending (baseline number to be filled).
- Sized: ~700 LOC + 4-6 templates + docs.
- Decision: keep sidecar declarations OPT-IN (configure-only, never
  install). Claws is orchestration, not a package manager for helpers.
- Decision: defer `extends:` + remote templates to v1.6+ (still).
