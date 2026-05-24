# template-engine-complete — worklog

Append-only log of work done on this ticket.

---

## 2026-05-24 — Kickoff + planning

- Filed ticket from audit findings during v1.2.0 dogfood.
- Sized scope: ~600 LOC across `apply.go`, new `template.go`,
  `quickstart.go`, plus tests + 2 bundled-template updates + 1 new
  bundled template.
- Tasklist drafted at
  `reports/tasklist_2026-05-24_template-engine-complete.md`.
- Decision: ship as v1.3.0 (additive schema fields = minor bump).
- Decision: defer `extends:` + remote templates to v1.4 to keep this
  change reviewable.
