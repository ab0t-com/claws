# hooks-register + namespaced-templates + url-loaded-resources

**Filed:** 2026-05-24
**Target:** v1.4.0
**Status:** Open

## Background

v1.3.0 shipped the template engine end-to-end. Three concrete asks from
the dogfood pass:

1. **Hooks should respect the adapter pattern.** Right now `applyHooks`
   hardcodes `workspace/hooks/<event>.sh` — that's an OpenClaw assumption.
   Different runtimes (ab0t-sandbox, custom) may use different hook dirs,
   different event names, or no hooks at all. The Runtime struct needs to
   declare its hook contract so apply can target it correctly.

2. **More real templates, namespaced.** The current 3 bundled templates
   are useful but flat. A real marketplace needs categorisation
   (solo/teams/specialty) and templates that actually do something
   beyond "create one agent".

3. **Templates should be able to reference URLs.** Skills + hooks should
   be loadable from `https://raw.githubusercontent.com/.../foo.sh` so
   that templates can compose shared skill repos without inlining everything.
   Security gate: HTTPS only, SHA256 verification recommended.

## Goal

Make the template engine *runtime-adaptive* (hooks via adapter contract),
*organised* (namespaced templates), and *composable* (URL-loaded resources
with SHA verification). Plus: 6-8 genuinely useful new templates.

## Scope — In

### Phase 1 — Hook register on Runtime adapter

- **1a.** Add to `Runtime` struct:
  - `SupportedHookEvents []string` — names of events this runtime
    actually wires up (e.g. `["onStart","onMessage","onIdle","onError","onShutdown"]`)
  - `HooksDir string` — workspace-relative dir for hook scripts
    (e.g. `"hooks"` → `<instance>/workspace/hooks/`)
  - `HookFileExt string` — extension for hook files (e.g. `".sh"`)
- **1b.** Populate openclaw runtime with sensible defaults.
- **1c.** Update `applyHooks` in apply.go to:
  - Look up the agent's runtime, use its `HooksDir` + `HookFileExt`
  - Warn (don't fail) if a hook event isn't in `SupportedHookEvents`
  - Skip cleanly when `HooksDir` is empty (runtime doesn't support hooks)
- **1d.** `claws runtime show <name>` already exists — make sure it
  prints the hook contract.

### Phase 2 — Namespaced templates

- **2a.** Template resolver searches recursively under `templates/`,
  matching either:
  - Full namespaced name: `claws apply --template=solo/telegram-coder`
  - Bare name: `claws apply --template=telegram-coder` (errors if
    ambiguous; lists candidates)
- **2b.** `claws template list` groups output by namespace dir.
- **2c.** Restructure existing bundled templates:
  - `templates/solo/` — solo.json, telegram-coder.json, personal-assistant.json
  - `templates/teams/` — (new, see Phase 4)
  - `templates/specialty/` — (new, see Phase 4)
- **2d.** Resolver back-compat: bare-name lookup still finds existing
  templates in either flat or namespaced layout.

### Phase 3 — URL-loaded resources

- **3a.** New `ResourceRef` type:
  ```json
  { "name": "calendar" }                              // bundled by name
  { "from": "templates/skills/calendar.md" }          // local file
  { "fromUrl": "https://…", "sha256": "abc…" }       // remote
  { "fromUrl": "https://…" }                          // remote, no pin (WARN)
  ```
- **3b.** `agents[].skills` accepts mix of strings + ResourceRef
  objects.
- **3c.** `agents[].hooks` event values accept either:
  - String (inline command) — existing behavior
  - `{ "command": "..." }` — explicit inline
  - `{ "from": "path.sh" }` — local file
  - `{ "fromUrl": "https://...", "sha256": "..." }` — remote
- **3d.** Fetch helper:
  - HTTPS-only (refuse `http://`)
  - SHA256 verification when `sha256` declared
  - Warning printed if no `sha256`
  - Cached at `~/.cache/claws/fetched/<sha256-or-url-hash>`
  - Cache hit → skip download
  - Timeout 30s, fail with clear error
- **3e.** Schema rejection: any URL without `https://` prefix → error.

### Phase 4 — Real templates (8 new)

Each template must do something genuinely useful, not just demo
schema fields. Real defaults, real channel choices, real policy.

Solo/:
- **4a.** `solo/personal-assistant.json` (move + enhance existing)
- **4b.** `solo/telegram-coder.json` (move + enhance existing)
- **4c.** `solo/discord-companion.json` (new) — Discord bot for a
  small server, allowlist by guild ID, moderation skills
- **4d.** `solo/whatsapp-family.json` (new) — WhatsApp family helper,
  allowlist by phone number, calendar+reminders skills

Teams/:
- **4e.** `teams/research-trio.json` (new) — manager + lit-review
  worker + data-analysis worker, shared workspace, Slack channel
  for manager only, web-search skill
- **4f.** `teams/coding-pair.json` (new) — pair programmer + reviewer
  agents, shared workspace, both with `tools.profile=coding`

Specialty/:
- **4g.** `specialty/oncall-rotation.json` (new) — paging webhook
  receiver + ack handler, allowlist by Slack user ID, escalation hook
- **4h.** `specialty/knowledge-base.json` (new) — RAG-style agent
  with web-search + memory + file-watch skills, no public channel
  (read-only via tunnel)

### Phase 5 — Documentation + tests

- **5a.** Update `templates/README.md` with:
  - Namespacing rules
  - URL-loaded resource syntax + security guidance
  - Each new template described
  - Hook adapter contract section
- **5b.** Integration tests:
  - Namespace resolver (full path, bare name, ambiguous)
  - URL fetcher with httptest.Server (no real network in tests)
  - SHA256 mismatch rejected
  - Cache hit skip
  - Hook register: skip events not in SupportedHookEvents
- **5c.** Update bundled `templates/README.md`

### Phase 6 — Release

- **6a.** CHANGELOG v1.4.0
- **6b.** Cut release: build, commit, push, tag, push tag
- **6c.** End-to-end install verify on fresh fake HOME

## Scope — Out

- Template signing (cosign/sigstore) — v2.0
- `extends:` template composition — separate v1.4 ticket
- Remote `--template=github:org/repo` — separate v1.4 ticket (this
  ticket handles URLs for *resources within* a template, not the
  template file itself)
- Marketplace registry beyond bundled+README — long horizon
- Hot-reload of templates — too risky for v1.x

## Acceptance criteria

1. `claws apply --template=teams/research-trio` resolves and applies
   the namespaced template end-to-end.
2. `claws apply --template=research-trio` (bare name) does the same.
3. A template can reference a URL for a hook script:
   ```json
   "hooks": {
     "onStart": { "fromUrl": "https://raw.githubusercontent.com/ab0t-com/hooks/v1/cleanup-tmp.sh",
                  "sha256": "abc123..." }
   }
   ```
   …and the script lands at `<instance>/workspace/<runtime-hook-dir>/onStart.sh`.
4. URL without `https://` or with `http://` is rejected at parse time.
5. URL without `sha256` prints a warning but still applies.
6. `claws template list` groups by namespace.
7. The OpenClaw runtime declares its hook contract via `SupportedHookEvents`,
   `HooksDir`, `HookFileExt`.
8. All 8 new templates parse via `claws apply --dry-run --template=<name>`
   without errors.
9. Full test suite green.

## Estimated

~800 LOC + 8 template files + docs. ~3 hours focused.
