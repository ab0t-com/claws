# template-engine-complete

**Filed:** 2026-05-24
**Owner:** TBD
**Target:** v1.3.0
**Status:** Open

## Background

The v1.1.0 `claws apply` and `claws quickstart` shipped the happy path:
fresh JSON profile → working agent. But a recent audit found four real
gaps that break the "declarative + idempotent + one-click" promise:

1. **Schema fields parsed but silently dropped.** `policy.*`, `runtime.image`,
   `channels[].dmPolicy`, `tools.profile`, and `team.shared` are all in the
   profile schema but ignored at apply-time. A profile that says "loopback
   only, DM pairing, pin image to v2026.3.25" gets a default-everything
   agent. Dishonest schema.

2. **`--template=<name>` doesn't work.** The marketplace framing assumes
   `claws apply --template=solo-telegram-coder` resolves to the bundled
   template. Today only `--file=<path>` works. Users have to know that
   bundled templates live at `~/.local/share/claws/templates/`.

3. **Channel + auth re-apply not idempotent.** `apply` is supposed to
   converge — re-running with the same profile should be a no-op. Today
   `cmdChannel add` may error on duplicates, and `cmdAuth apikey`
   re-runs unconditionally and overwrites credentials.

4. **No automatic security audit.** Non-technical users running
   `claws quickstart` aren't told to also run `claws audit`. The audit
   should happen automatically, with clear next-step output for any
   findings.

Plus three missing capabilities that block real templates:

5. **Skills.** OpenClaw runtime supports per-agent skill access (`shared/skills/`).
   Templates can't declare which skills an agent should have.

6. **Hooks.** Runtime lifecycle hooks (onStart, onMessage, etc.) can't
   be declared in a template.

7. **Arbitrary config.** Anything in `openclaw.json` should be settable
   from a profile via a `config:` block, not require a separate `claws
   config set` invocation.

## Goal

Make `claws apply` honest: every schema field either does what the docs
say or fails loud at parse time. Make `claws quickstart` truly one-click
for non-technical users — audit runs, defaults are safe, errors are
recoverable.

## Scope — In

### A. Fix the silent-drop bugs

- **A1.** Apply the `policy.*` block. Write declared fields to
  `policy.json` before agents are created. Validate any conflicts.
- **A2.** Apply `runtime.image` — pin instances to the declared image
  via the existing `OPENCLAW_IMAGE` env or per-instance `image` field
  in `instance.env`.
- **A3.** Pass `channels[].dmPolicy` through to `cmdChannel add` (it
  already supports the flag).
- **A4.** Apply `agents[].tools.profile` via `cmdConfig set`.
- **A5.** Document that `team.shared` is currently no-op (shared dir
  is created unconditionally); decide whether to honor `false` (skip
  shared dir) or remove the field.

### B. New schema fields

- **B1.** **`agents[].sandbox: bool`** — set `agents.defaults.sandbox`
  config. Default to `true` for safety (currently `false` by default,
  doctor warns).
- **B2.** **`agents[].skills: [string]`** — symlinks the named skills
  from `~/.claws-workspace/shared/skills/` into the agent's
  `workspace/.skills/` dir. (Or whatever the OpenClaw skills layout is —
  needs confirmation.)
- **B3.** **`agents[].hooks: {onStart, onMessage, onIdle, …}`** — writes
  hook definitions to the agent's `workspace/hooks/` dir.
- **B4.** **`agents[].config: {key: value, …}`** — arbitrary openclaw.json
  patches. Each entry becomes a `claws config set <name> <key> <json-value>`.
  Catch-all for anything not covered by a dedicated field.
- **B5.** **`agents[].tools: {profile, allow, deny}`** — extend the
  existing `tools.profile` with `allow` and `deny` lists.

### C. Template resolver

- **C1.** **`claws apply --template=<name>`** — search order:
  `./templates/<name>.json` → `$XDG_DATA_HOME/claws/templates/<name>.json`
  → bundled-in-tarball. Error clearly if not found.
- **C2.** **`claws template list`** — list templates discoverable by
  the search order above with their metadata (name, version,
  description, tags).
- **C3.** **`claws template show <name>`** — print the profile JSON
  for inspection.

### D. Idempotence

- **D1.** **Channel add idempotence.** Check `openclaw.json`
  `channels.<type>.enabled == true` before calling `cmdChannel add`.
  Skip if already configured.
- **D2.** **Auth apikey idempotence.** Check the credentials file for
  the provider before re-running. Skip with a "✓ already set" message
  unless `--force` is passed.
- **D3.** **Hooks/skills idempotence.** Check file existence before
  writing.

### E. Automatic audit

- **E1.** **`claws quickstart` ends with `claws audit`.** Surface any
  failures inline with the fix commands. Don't fail the quickstart on
  audit warnings, but exit non-zero if there are audit failures.
- **E2.** **`claws apply` ends with `claws audit`** (unless
  `--skip-audit`).
- **E3.** **Audit framing for non-technical users.** A freshly-created
  agent with no auth will have some audit checks fail (e.g. "no auth
  configured"). Frame this clearly: "These will resolve once you complete
  the next steps above."

### F. Documentation

- **F1.** Update CHANGELOG with everything above.
- **F2.** Update bundled `solo-telegram-coder.json` to use new fields
  (sandbox=true, tools.profile=coding, channels[].dmPolicy=pairing).
- **F3.** Add bundled `personal-assistant.json` demonstrating skills +
  hooks + config in a real template.
- **F4.** Add `templates/README.md` documenting the v1 schema with
  every field and what it does.

### G. Tests

- **G1.** Idempotence tests: re-apply same profile, assert exit 0 +
  no state changes.
- **G2.** Each new schema field: integration test that confirms it
  reaches the underlying config/file.
- **G3.** `--template=<name>` resolver tests covering all 3 search
  locations.
- **G4.** Schema rejection: unknown apiVersion, missing required fields.

## Scope — Out (defer to v1.4+)

- **`extends:` composition** — template inheritance with deep-merge.
- **Remote templates** — `--template=github:org/repo/path@ref`.
- **JSON Schema validation** — formal schema file + library validation.
  Current go struct unmarshal catches most issues.
- **Template signing** — cosign/sigstore. v2.0 target.
- **Template marketplace registry** — a real registry beyond the
  bundled set + README index.

## Acceptance criteria

A non-technical user can do this and get a working, secure agent:

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | sh
claws quickstart
# → creates personal assistant
# → runs doctor (environment OK)
# → runs audit (any failures clearly framed)
# → prints exact next-step commands with token-source guides

claws auth default/<name> apikey openai sk-…
claws channel add default/<name> telegram --token=…
# → both idempotent if re-run

claws start default/<name>
# → agent live, audit passes, secure defaults applied
```

Or fully declarative via a profile:

```bash
OPENAI_API_KEY=sk-… TELEGRAM_BOT_TOKEN=… \
  claws apply --template=solo-telegram-coder
# → schema fields all applied (policy, dmPolicy, tools, sandbox)
# → audit runs at end
# → re-running is a no-op
```

## Out-of-scope guardrails

- No new runtime adapter requirements — work entirely within the
  existing `Runtime` interface.
- No breaking changes to v1 schema — only additive fields.
- No new external dependencies — standard library + existing helpers.
- No reduction in security defaults — only tightening (sandbox=true).

## Tickets that block

None.

## Tickets that block on this

- v1.4: extends composition (waits for this to land first)
- v1.4: remote templates (waits for `--template=<name>` resolver from C1)
