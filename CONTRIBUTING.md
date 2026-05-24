# Contributing to claws

Thanks for your interest in claws. This document covers everything you need to know to develop, test, and submit changes.

## Quick start

```bash
git clone https://github.com/ab0t-com/claws.git
cd claws

# Install git hooks (gitleaks secret-scanning at commit / push time)
./scripts/install-hooks.sh

# Build + vet + short tests
./scripts/rebuild.sh
```

## Prerequisites

- **Go 1.22+** — single dependency. The whole project uses standard library only.
- **Docker with Compose v2** — needed to run integration tests that exercise real instance lifecycles.
- **gitleaks** — installed automatically by `scripts/install-hooks.sh`.
- **bash** — all scripts target bash, not POSIX `sh`.

## Repository layout

```
cmd/claws/         All Go source (package main, ~60 files)
html/                Static UI / landing pages bundled in releases
docs/                Markdown user documentation
scripts/             Build, release, install, security audit, git hooks
docker-compose.yml   Substrate template instances are built from
```

There is intentionally no `internal/` package split. Everything lives in `cmd/claws/` as one package to keep call sites short and avoid premature abstraction. If you find yourself wanting to add `internal/`, open an issue first to discuss why.

## Development loop

```bash
./scripts/rebuild.sh             # full: build + vet + -short tests (~100s)
./scripts/rebuild.sh --quick     # build only (~5s)
./scripts/rebuild.sh --race      # build + tests with -race (~180s)
```

The binary lands at `./claws` so you can run it immediately:

```bash
./claws help
./claws list
```

For dogfooding against a scratch state directory:

```bash
TMP=$(mktemp -d)
OPENCLAW_ROOT=$TMP ./claws init
OPENCLAW_ROOT=$TMP ./claws create demo
# ...
```

**Never** test against `~/.openclaw/` — that directory holds live production agents.

## Tests

```bash
go test ./cmd/claws/...                              # full suite
go test -short ./cmd/claws/...                       # skip slow integration tests
go test -run TestIntegration_Drift ./cmd/claws/...   # one test
go test -race -short ./cmd/claws/...                 # race detector
```

The integration tests build the `claws` binary into a tempdir and exercise it as a subprocess. They do not require Docker for most cases (they stub out the compose calls). Tests that genuinely need Docker are gated behind `testing.Short()` or skip cleanly when Docker is absent.

### Writing tests

- Use `t.TempDir()` for any filesystem state. The test harness registers cleanup automatically.
- Prefer table-driven tests for any function with branching logic.
- For new commands, add at least one integration test that runs the command end-to-end (`cmd/claws/integration_test.go`).

## Coding conventions

- **No external Go dependencies.** Standard library only. If you need a dep, open an issue first.
- **File locking via `syscall.Flock`** on every registry/group/instance write — see `flock.go`.
- **Atomic state transitions via `os.Rename`** — never read-modify-write.
- **Commands live in `commands.go`** (large file, scoped sections) or a dedicated `<verb>.go` file when they exceed ~150 lines (e.g. `drift.go`, `errors_cmd.go`).
- **Comments explain the why, not the what.** If a comment restates the code, delete it.
- **Help text is part of the API.** When you add or rename a command, update `help.go`.

## Pre-commit hooks (gitleaks)

`./scripts/install-hooks.sh` installs three hooks:

- **pre-commit** — scans staged changes for secrets, blocks commit on match
- **pre-push** — full repo history scan
- **commit-msg** — scans the commit message itself

If a hook fires on a false positive, add the path to `.gitleaks.toml`'s `[allowlist]` block. Never use `--no-verify` to bypass it — that defeats the purpose.

## Branch / commit / PR conventions

- **Branch from `main`** for changes destined for release. `main` is protected.
- **One logical change per PR.** If you find yourself adding "and also fixed X" to the description, split it.
- **Commit messages** — first line ≤ 72 chars, imperative ("add drift command" not "added drift command"), explain *why* in the body if non-obvious.
- **Run `./scripts/rebuild.sh` before pushing.** CI runs the same thing; failing fast locally is faster than failing slow on CI.

Example commit message:

```
add `drift` command for state consistency checks

Combines forward/reverse orphan detection with disk/registry drift into
one umbrella view. Each finding emits a copy-paste fix command rather
than executing anything itself — same pattern as `policy validate`.

Resolves #N.
```

## Releasing

Releases are cut by maintainers using `./scripts/publish-release.sh <version>`. See that script's `--help` for the full process.

## Security

If you find a security issue, please **do not** open a public GitHub issue. See [SECURITY.md](SECURITY.md) for the disclosure process.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
