# Tasklist — hooks register + namespaced templates + URL resources (v1.4.0)

**Filed:** 2026-05-24
**Ticket:** `tickets/hooks-namespaces-url-resources-2026-05-24/ticket.md`
**Target:** v1.4.0

| ID | Task | Files | Status |
|---|---|---|---|
| 1a | Runtime struct: SupportedHookEvents, HooksDir, HookFileExt | `cmd/claws/runtime.go` | TODO |
| 1b | Populate openclaw runtime with hook contract | `cmd/claws/runtime.go` | TODO |
| 1c | applyHooks consults runtime adapter (event allow-list, dir, ext) | `cmd/claws/apply.go` | TODO |
| 1d | `claws runtime show` prints hook contract | `cmd/claws/runtime.go` | TODO |
| 2a | Resolver searches subdirs; supports full + bare names; errors on ambiguity | `cmd/claws/template.go` | TODO |
| 2b | `claws template list` groups by namespace | `cmd/claws/template.go` | TODO |
| 2c | Restructure existing templates under solo/ | `templates/solo/*.json` | TODO |
| 2d | Back-compat: flat-layout templates still resolve by bare name | `cmd/claws/template.go` | TODO |
| 3a | ResourceRef type with name/from/fromUrl/sha256 | `cmd/claws/apply.go` | TODO |
| 3b | agents[].skills accepts mix of strings + ResourceRefs | `cmd/claws/apply.go` | TODO |
| 3c | agents[].hooks event values accept string OR ResourceRef-like | `cmd/claws/apply.go` | TODO |
| 3d | URL fetcher: https-only, sha256 verify, cache, 30s timeout | `cmd/claws/fetch.go` (new) | TODO |
| 3e | Schema rejection: non-https URL → error at parse time | `cmd/claws/apply.go` | TODO |
| 4a | Move solo.json → templates/solo/solo.json | `templates/` | TODO |
| 4b | Move telegram-coder → templates/solo/telegram-coder.json | `templates/` | TODO |
| 4c | New: templates/solo/discord-companion.json | new | TODO |
| 4d | New: templates/solo/whatsapp-family.json | new | TODO |
| 4e | New: templates/teams/research-trio.json | new | TODO |
| 4f | New: templates/teams/coding-pair.json | new | TODO |
| 4g | New: templates/specialty/oncall-rotation.json | new | TODO |
| 4h | New: templates/specialty/knowledge-base.json | new | TODO |
| 5a | templates/README.md updated with namespaces + URL syntax | docs | TODO |
| 5b | Integration tests for namespace resolver + URL fetcher + hooks adapter | tests | TODO |
| 6a | CHANGELOG v1.4.0 | docs | TODO |
| 6b | Cut release: build, commit, push, tag | release | TODO |
| 6c | End-to-end install verify | manual | TODO |

## Execution order

1. **1a–1d** (Runtime hook register) — small, self-contained
2. **2a–2d** (Namespace resolver) — independent of hooks
3. **3a–3e** (URL fetcher) — independent of namespaces
4. **4a–4h** (Templates) — depends on 2 + 3 being in
5. **5a–5b** (Docs + tests)
6. **6a–6c** (Release)

Each phase commits separately so reviewable.
