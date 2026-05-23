# Changelog

All notable changes to clawctl are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_(nothing yet)_

## [1.0.0] — 2026-05-23

First public release under the MIT license.

### Added — Fleet observability

- **`clawctl errors`** — incident-triage umbrella view. Composes container
  state, recent log errors, recent failed `clawctl` operations, and orphan
  Docker containers into one screen, then prints a "Fix paths" trailer of
  directive commands. Read-only; never executes anything.
  Flags: `--since=<dur>`, `--group=<name>`, `--json`.
- **`clawctl drift`** — four-dimension state consistency check (forward
  orphans, reverse orphans, disk drift, registry drift). Emits per-finding
  fix commands. Read-only.
- **`clawctl orphans`** — surface Docker containers that match the
  `openclaw-` naming prefix but are not in the port registry (e.g.
  containers a test run left behind). Subcommands: `list` (default),
  `clean <container> [--yes]`, `clean --all [--yes]`. Includes a
  `--reverse` mode that surfaces registry entries whose Docker container
  is missing.
- **`clawctl channels`** (pluralised) — fleet-wide channel matrix. Rows
  are agents, columns are channel types (telegram, discord, slack, signal,
  whatsapp). Cells show the dmPolicy when enabled, or `—` when absent.
  Flags: `--group=<name>`, `--json`. Singular `channel <verb>` continues
  to operate on one instance.
- **`clawctl logs --group=<name> -f`** — interleaved live tail across
  every member of a group with per-member ANSI colour prefix; Ctrl-C
  exits cleanly. Without `-f`, sequential dump with section headers.
  Supports `--since=<dur>` and `--grep=<pattern>` in both modes; `--grep`
  is in-process and preserves order.

### Added — Auth verification

- **`clawctl auth verify <name>`** — per-instance auth liveness check.
  Tries (1) the auth-check endpoint, (2) `/readyz` `failing[]` inspection,
  (3) log scan for auth errors in the last 5 minutes. Exits 0 only on
  verified ok. Honest about confidence: a log-scan "ok" means "no errors
  seen", not "next call will succeed".
- **`clawctl auth status --probe`** — adds a `VERIFIED` column to the
  fleet auth status table by running `verify` per row.
- **`clawctl auth codex --force`** — opt out of idempotence preflight
  when you specifically want to re-run OAuth.

### Added — Release infrastructure

- **MIT License** at the repo root and bundled inside every release
  tarball.
- **`scripts/rebuild.sh`** — local-dev inner-loop build script. Flags:
  `--quick` (build only), `--race` (with race detector). Version-stamps
  via `git describe`.
- **`scripts/release.sh`** — cross-compiles `linux/amd64`, `linux/arm64`,
  `darwin/amd64`, `darwin/arm64`. Each tarball contains the binary,
  `docker-compose.yml`, `install.sh`, `security-audit.sh`, `LICENSE`,
  `README.md`, `html/`, `docs/`, and a per-target `MANIFEST.txt`
  listing every file's SHA256. Produces a top-level `SHA256SUMS`.
  Builds are reproducible (`-trimpath -ldflags "-s -w"`).
- **`scripts/install.sh`** — three auto-detected modes:
  1. **Remote** — downloads release from
     `github.com/ab0t-com/claws/releases`, verifies SHA256 against
     `SHA256SUMS` before installing.
  2. **Local-release** — runs from inside an extracted tarball,
     installs the adjacent binary.
  3. **Local-dev** — invoked from a git checkout (or with
     `CLAWCTL_LOCAL_DEV=1`); builds from source via `go build`.
  HTTPS-only, fails on any HTTP error, refuses to overwrite existing
  install without `--force`, supports `--dry-run`.
- **`scripts/publish-release.sh`** — one-shot release driver. Validates
  clean tree, runs tests, tags the release, builds artifacts, and
  optionally pushes + creates a GitHub release via `gh`.

### Changed

- **Module path** — `clawctl` → `github.com/ab0t-com/claws`.
- **Repo layout** — all Go source moved from the repo root to
  `cmd/clawctl/`. HTML assets moved from the repo root to `html/`.
  Build command is now `go build ./cmd/clawctl/`.
- **`docker-compose.yml`** — gateway in-container bind hardcoded to
  `0.0.0.0`. Host-side exposure is governed by `OPENCLAW_HOST_BIND`.
  Prior coupling caused gateway to bind loopback inside the container
  and become unreachable from sibling containers.
- **Help text** for `auth` rewritten to be honest about per-strategy
  verification confidence.

### Fixed

- **Integration-test orphan cleanup** — tests now register a
  `t.Cleanup` hook that removes any Docker containers left behind by
  the harness, ending the "test run leaves containers running"
  drift class.
- **`status --group=<g>`** routing — `firstPositional()` helper now
  correctly skips leading flags before resolving the instance name.
- **Column alignment** in fleet tables — new `padVisible` helper
  measures string width excluding ANSI escape sequences.
- **`logs --grep` "Stdout already set"** — grep branch now builds the
  `docker compose logs` command directly instead of inheriting a
  pre-bound `Stdout`.

### Security

- **Channel security defaults** — outbound messaging is **OFF** by
  default; DMs default to **pairing**; policy enforcement covers
  `allowFrom`.
- **gitleaks pre-commit + pre-push + commit-msg hooks** ship with
  custom rules for Telegram bot tokens, the OpenClaw gateway token
  format, and Slack `xoxb-` / `xapp-` tokens, on top of gitleaks'
  built-in rules.
- **`SECURITY.md`** added with a coordinated-disclosure policy.

### Removed

- **Windows release target** — the codebase has always depended on
  Unix-only syscalls (`Flock`, `Statfs`). The 0.x release script
  listed `windows/amd64` but never produced a working binary; the
  1.0 release script is honest about this. WSL2 users on Windows
  should use the `linux/amd64` build.

---

[Unreleased]: https://github.com/ab0t-com/claws/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/ab0t-com/claws/releases/tag/v1.0.0
