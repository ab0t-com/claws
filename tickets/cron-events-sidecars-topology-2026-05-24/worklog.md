# cron + events + sidecars + topology — worklog

Append-only.

---

## 2026-05-24 — Kickoff

- Filed after v1.4.0 dogfood. Four asks:
  - Cron section (periodic actions)
  - Event injection / digest endpoint (intent-gateway pairing)
  - First-class sidecar helpers (sharedwatch, intent-gateway, future)
  - Topological team structures (multi-tier hierarchies, peer meshes)
- Coverage audit baseline: 9.5% line coverage (most tests integration-style,
  Go -cover doesn't see subprocess code).
- Sized: ~700 LOC + 4-6 templates + docs.
- Decision: keep sidecar declarations OPT-IN (configure-only, never
  install). Claws is orchestration, not a package manager for helpers.
- Decision: defer `extends:` + remote templates to v1.6+ (still).

## 2026-05-24 — Shipped (commit 9d6d970, tag v1.5.0)

**Phase A — Cron**
- validateCronSchedule accepts: 5-field crontab, @-aliases, "every <dur>".
- applyCron materialises to workspace/<runtime.CronDir>/claws.crontab,
  one combined file. Hook references resolved against runtime.HooksDir.
- Validation at parse-time: bad schedule, ambiguous action, missing hook ref → loud failure.
- Runtime.CronDir + CronFormat + Capabilities.Cron flags.

**Phase B — Events**
- ProfileEvents struct with enabled/digestMode/endpoint/allowFromIps.
- applyEventsConfig writes via cmdConfig set.
- Capabilities.Events flag (runtime decides whether to expose endpoint).

**Phase C — Sidecars**
- ProfileSidecar (name, kind, config). Built-in kinds: sharedwatch,
  intent-gateway, custom.
- applySidecar writes workspace/sidecars/<name>.json — configure-only.
- Unknown kind: warns inline, doesn't fail the run.

**Phase D — Topology**
- agents[].peers (list).
- validateTopology: cycle detection, self-references rejected,
  unknown-name rejected.
- applyTopology writes workspace/topology.json with manager/peers/workers.

**Phase E — Tests**
- 13 new tests: cron good+bad schedules, cron apply+rejection,
  events apply, sidecar apply + unknown-kind, topology validation (4),
  multi-tier artefacts.
- Full suite: green (~3 min).

**Phase F — Bundled templates**
- NEW teams/multi-tier (7 agents, depth-2).
- NEW teams/specialist-mesh (3 peers, no hierarchy).
- UPD teams/research-trio + cron.
- UPD specialty/knowledge-base + sharedwatch sidecar + cron.
- UPD specialty/oncall-rotation + events.
- All 11 templates dry-run validated.

**Phase G — Release**
- CHANGELOG v1.5.0.
- release.sh v1.5.0 → 4 tarballs, sha256, VERSION.
- commit 9d6d970, push main, tag v1.5.0, push tag — clean.
- Live install verified: curl install.sh | sh → claws v1.5.0,
  template list shows all 11 with correct grouping.

**Final state**
- origin/main at 9d6d970, v1.5.0 tag pushed.
- release/VERSION → v1.5.0.
- Status: **CLOSED**.

**Deferred to v1.6+:**
- extends: template composition
- Remote --template=github:org/repo
