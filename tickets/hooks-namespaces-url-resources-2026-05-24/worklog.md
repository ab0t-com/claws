# hooks-namespaces-url-resources — worklog

Append-only.

---

## 2026-05-24 — Kickoff

- Filed ticket after user feedback on v1.3.0 dogfood.
- Three asks: adapter-aware hooks, namespaced templates, URL-loaded
  resources.
- Plus 8 new genuinely-useful templates as Phase 4 deliverable.
- Scope estimate: ~800 LOC + 8 templates + docs. Target v1.4.0.
- Decision: defer `extends:` composition and remote-template loading
  (different ticket; URL fetcher in this ticket only handles
  resources-within-a-template, not the template file itself).

## 2026-05-24 — Shipped (commit 8fa8990, tag v1.4.0)

**Phase 1 — Hook register**
- Runtime struct extended with SupportedHookEvents/HooksDir/HookFileExt.
- OpenClaw declares onStart/onMessage/onIdle/onError/onShutdown.
- applyHooks consults runtime, prints inline warning on unknown events.

**Phase 2 — Namespaced templates**
- Resolver rewritten: namespaced form ("ns/name") + bare-name (recursive
  one-level walk with ambiguity error + qualified-name suggestions).
- listTemplates groups by namespace; `template list` shows sections.
- Existing 3 templates relocated under solo/ via `git mv`.
- templateInfo gains Namespace field + QualifiedName() method.

**Phase 3 — URL-loaded resources**
- New fetch.go: validateFetchURL (https-only), fetchResource (with sha256
  verify), cache at $XDG_CACHE_HOME/claws/fetched/, 4 MB cap, 30s timeout,
  httpFetcher var for test injection.
- SkillRef + HookRef types in apply.go with custom UnmarshalJSON for
  string-or-object polymorphism (back-compat with v1.3 string lists).
- applySkills + applyHooks resolve from URL/file/inline, content-hashed
  idempotence preserved.

**Phase 4 — 8 bundled templates (6 new + 3 relocated)**
- All 9 templates dry-run validated.
- Real-world scopes: discord-companion (small server), whatsapp-family
  (allowlist), research-trio (manager/2 workers), coding-pair (impl+rev),
  oncall-rotation (Slack + warnings:), knowledge-base (no-channel RAG).

**Phase 5 — Tests**
- fetch_test.go: 3 tests (HTTPS validation, sha verify, cache hit).
- template_resolver_test.go: 5 tests (namespaced, bare unambiguous,
  bare ambiguous, flat back-compat, traversal rejection) + list-groups.
- Full suite green (rebuild.sh exit 0).

**Phase 6 — Release**
- CHANGELOG v1.4.0 entry.
- release.sh v1.4.0 → 4 tarballs + SHA256SUMS + VERSION.
- commit 8fa8990, push main, tag v1.4.0, push tag — all clean.
- Live install verification: pending end-of-session smoke test.

**Final state**
- origin/main at `8fa8990 release v1.4.0`.
- Tag v1.4.0 pushed.
- release/VERSION → v1.4.0.
- Status: **CLOSED**.

**Non-goals confirmed deferred to v1.5:**
- `extends:` template composition
- Remote template loading (--template=github:org/repo)
