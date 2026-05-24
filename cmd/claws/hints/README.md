# `cmd/claws/hints` — next-step suggestions for every command

Lifted from [sharedwatch's `internal/hints`](../../../sharedwatch/src/internal/hints/README.md)
under that package's "second project should adopt and eventually
extract" extraction note. Same engine, claws-specific Context fields +
providers.

Powers the `Next:` block in human output (and the `next: [...]` array
in JSON envelopes once those are wired) across every applicable claws
command. Intentionally small and portable: the rest of the binary
depends on it, but it depends on nothing in the rest of the binary.

When a third consumer adopts this package across the org, lift it out
to `github.com/ab0t-com/cli-hints` — both projects would then depend on
the module instead of carrying duplicates.

---

## Why this package exists separately

A first-time operator running a CLI has no way to know what command to
run *next* without reading every man page. The classic answer is
HATEOAS: each response carries the next reasonable action. This
package is that, for a CLI: each command's output attaches a small
ordered list of "what to run next" — derived from the command's actual
state, not from a static lookup table.

Two design forces shape the boundary:

1. **Reuse across our projects** (claws, sharedwatch, future tools).
   Designed to lift into a standalone Go module — see §"Extracting".
2. **Profile-driven verbosity.** Humans want 1–3 hints with reasons;
   AI agents reading JSON want the maximum useful set; some piped
   workflows want exactly one; some operators want none. One pipeline,
   four registers.

---

## Package layout

Keep the package to these five files. Don't grow horizontally.

| File | Purpose |
|---|---|
| [`hints.go`](hints.go) | Public types (`Hint`, `HintSet`, `Profile`, `Context`, `Provider`, `AgentRef`) + the engine (`Register`, `For`, `Clear`, `Toplevel`). |
| [`profiles.go`](profiles.go) | The four built-in profiles + `Limit()` + `IsValid()` + `ResolveProfile()`. |
| [`providers.go`](providers.go) | One provider function per command; all registered in `init()`. |
| [`render.go`](render.go) | `RenderText(w, set)` — the `Next:` block formatter. |
| [`hints_test.go`](hints_test.go) | Unit tests. |

**No subpackages, no `util.go`.** Shared helpers between providers go
at the bottom of `providers.go`.

---

## Load-bearing rules

1. **Zero imports from other claws packages.** The whole point is that
   the package consumes only a `Context` value passed in. Verify:
   ```bash
   go list -deps ./cmd/claws/hints | grep -E 'cmd/claws[^/]' && \
     echo BROKEN || echo CLEAN
   ```
   Must print `CLEAN`.
2. **`Context` is additive only.** New fields land with zero-value
   defaults so existing providers keep compiling.
3. **`Hint` JSON shape is frozen.** `name`, `command`, `reason`.
   Names are public API once shipped — never rename.
4. **Providers must be pure functions of `Context`.** No I/O, no
   global reads, no clock, no env. State comes in via `Context`.
5. **Providers don't truncate.** Return everything that's usefully
   suggestable. `For()` applies `Profile.Limit()` centrally.
6. **Names: `snake_case` with a stable semantic root.** Examples:
   `start_all`, `tail_logs`, `ping_one_healthy`, `triage_errors`.
7. **One source of truth for "what to do next".** No
   `fmt.Println("hint: ...")` buried in a handler.

---

## How to add hints for a new command

1. Identify the state your hint needs. If it's already in `Context`,
   skip to step 2. Otherwise add a field at the *bottom* of the struct
   with a comment naming the consumer command.

2. Write the provider in `providers.go`:
   ```go
   func providerCronList(ctx Context) []Hint {
       if ctx.AgentName == "" { return nil }
       return []Hint{{
           Name:    "edit_cron",
           Command: fmt.Sprintf("claws cron edit %s", ctx.AgentName),
           Reason:  "add or remove scheduled jobs",
       }}
   }
   ```
   Rules:
   - Return `nil` (not `[]Hint{}`) when no hints apply.
   - Reasons are short, lowercase first letter, no trailing period.
   - Commands fully qualified (`claws ...`).
   - Never check profiles in providers — the engine handles that.

3. Register in the `init()` block:
   ```go
   Register("cron list", providerCronList)
   ```

4. Wire the handler in `cmd/claws/*.go`:
   ```go
   ctx := hintsCtxCheap(paths)
   ctx.AgentName = name
   hintsRender("cron list", ctx)
   ```

5. Add tests: one populated case, one empty case. Use `For(...)` not
   the provider directly so you exercise registration + truncation.

---

## Profile cheat sheet

| Profile | `Limit()` | Reasons in output | When to use |
|---|---|---|---|
| `default` | 4 | yes | Interactive shell (humans) |
| `agent` | 8 | yes | AI agents reading JSON envelopes |
| `terse` | 1 | no (stripped by `For()`) | Pipes wanting exactly one suggestion |
| `off` | 0 | n/a | Capturing output for diff / CI logs |

Resolution order in `ResolveProfile()`:

1. `--hints <profile>` CLI flag (highest)
2. `CLAWS_HINTS` env var
3. JSON output → auto-promote to `agent`
4. Default: `default`

Unrecognised values fall through — a typo in env doesn't break the
tool, just gets the default register.

---

## Testing conventions

- One test per provider per representative state: at least a populated
  case and an empty case.
- Use the engine (`For(...)`) in tests, not providers directly — that
  way you also exercise registration, profile truncation, and the
  terse-strips-reasons behaviour.
- **Don't call `hints.Clear()`** in tests. Built-in providers register
  in package `init()`; clearing breaks every subsequent test in the
  binary. Use a test-only registration key (`__test_yourname`) instead.
- Golden-output assertions are fine for `RenderText`. For provider
  output, prefer asserting on `Name` + a substring of `Command`.

Run: `go test ./cmd/claws/hints -count=1`.

---

## Extracting to its own module

When a third consumer appears (e.g. `intent-gateway`), lift this out:

1. Create `github.com/ab0t-com/cli-hints` repo.
2. `git mv cmd/claws/hints/ <newmod>/hints/` (drop the
   claws-specific Context fields; provide a hook for downstream apps
   to extend or supply their own).
3. `go mod init github.com/ab0t-com/cli-hints/hints`.
4. Update `claws/go.mod` to depend on the extracted module.
5. Sharedwatch's copy at `internal/hints` follows.

The longer this package stays in-repo, the more in-repo callers it
has — extraction gets slightly harder each release. Flag the moment a
third project wants it; that's the trigger.

---

## References

- Sharedwatch original: [`../sharedwatch/src/internal/hints/README.md`](../../../sharedwatch/src/internal/hints/README.md)
- Wired in claws by: [`../hints_context.go`](../hints_context.go) (the
  Context populator that lives in `package main` so it can read
  registry/paths state without polluting the hints package).
