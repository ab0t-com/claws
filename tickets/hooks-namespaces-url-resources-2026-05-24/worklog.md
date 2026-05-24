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
