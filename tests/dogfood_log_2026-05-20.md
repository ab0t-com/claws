# Dogfood Log — 2026-05-20

**Tester.** claude (this session), running as user `ubuntu`.
**Binary.** `/tmp/clawctl-dogfood` built from `feature/runtime-adapter` (commits `405f1b4` + `f4a3ba3`) on go1.22.12. Clean `go build`, no warnings.
**Test root.** `/tmp/clawctl-dogfood-XzY6Kj` (mktemp dir; *not* `~/.openclaw`).
**Safety.** Smoke script (`tests/smoke_dogfood.sh`) refused-by-design to run if `OPENCLAW_ROOT` resolved under `$HOME/.openclaw`; `CLAWS_SKIP_VALIDATE=1` set; no `start`/`stop`/`restart`/`upgrade` invoked; no `rm -rf`; test root preserved on disk for inspection.

## Top-line results

| Phase | Steps | Pass | Notes |
|-------|-------|------|-------|
| A. Go test suite | 3 | 3 | `go test ./...` → ok in 39.5s, 232 tests. `go vet` clean. `go build` clean. |
| B. First-run + help | 10 | 10 | Tiered help works; topic guides land. |
| C. Init + admin | 6 | 6 | `init` is idempotent, creates policy + access + audit + defaults + docker-compose. |
| D. Golden path | 17 | 17 | Create → list → status → health → config → token → tunnel → remove all work. |
| E. Groups & teams | 9 | 9 | `team create` is the right shortcut. Role topology generates correct override mounts. |
| F. Task queue | 6 | 6 | pending → claimed → done atomic renames work; full Task struct round-trips. |
| G. Channels (config) | 7 | 7 | Add → status → security → allow → send → deny → remove. Safe defaults applied. |
| H. Policy & access | 7 | 7 | `bind=wan` and disallowed image rejected; `access audit` shows full session. |
| I. Runtime adapter | 8 | 8 | `runtime add --from=openclaw` and `runtime init` both produce valid registry entries. |
| J. Errors / edges | 8 | 8 | Every expected-failure case failed with a directive message. |

**Total: 81/81 smoke checks pass.** Full transcript: `tests/dogfood_log_2026-05-20.out` (1296 lines).

## DX rubric (scored from observed behavior)

| Dimension | Score | Evidence / notes |
|-----------|-------|------------------|
| **Discoverability** — can a new operator find `setup` without docs? | **5/5** | First-run welcome is a 3-line nudge to `setup`/`init`/`help`. Hard to miss. |
| **Error quality** — do errors tell you the fix? | **4.5/5** | Almost every error suggests the next command. Example: `instance 'nope' does not exist — run: claws create nope`. Counter-example: `--bind=wan` rejection cites the policy but doesn't say which policy file or how to change it. |
| **Naming consistency** | **3/5** | "Instance" vs "agent" used interchangeably across help, prompts, and code (`instance.env` for what the README calls "agents"). `team` vs `group` are aliased but `team list` falls back to the `group` table header (`GROUP` column). Acknowledged in prior PMM audit as deliberate non-fix. |
| **Output density** | **4/5** | Create output is the right amount; status overview is genuinely scannable. Two minor noise points: (a) `status <name>` runs `docker compose ps` even when the container doesn't exist, leaving an empty header row; (b) "Next steps" hint reappears under each `create` even when chained inside `team create` (mitigated by `quietCreate` in the `setup` flow but not in `team create` or repeated `create` calls). |
| **Verbosity of common workflows** | **4.5/5** | `setup` (one command) and `claws create alice --auth=codex --telegram=TOKEN` (one command) both deliver on the README promise. The non-`setup` path still requires `create → auth → channel add → start → approve`, which is honest. |
| **JSON parity** | **3/5** | `list`, `status`, `health`, `runtime show` all support `--json`. Missing: `task list`, `channel status`, `channel security`, `group list`, `access show` is JSON-only output but with no `--json` *flag* (just dumps the file), `access audit` is text-only, `policy validate` is text-only. For a tool aiming at automation/CI, this is the biggest concrete gap. |
| **Safety** — destructive commands gated? | **4.5/5** | `remove --purge` and `group remove --purge` both prompt unless `--yes` is set. `storage sync --mirror` confirms before destructive. `proxy setup` writes to `/etc/caddy/conf.d/` (not main Caddyfile), backs up existing, and supports `--dry-run`. One soft spot: `claws init --force` is undocumented but exists and silently overwrites `policy.json` — could surprise an operator. |
| **README ↔ binary drift** | **5/5** | Every command in the README's "Commands" section exists and behaves as described. The "Note: tasks use rename() — local storage only" warning matches the actual `isFuseMount()` guard in `task.go`. |

**Overall: 4.2/5** for an internal, ops-focused CLI. Strong enough to publish; gap items below.

## Issues observed (with severity)

### S1 — gateway `bind: lan` hardcoded in JSON skeleton (LOW, confusing)

`commands.go:253` writes `gateway.bind = "lan"` into the new `openclaw.json` regardless of the `--bind=` flag. Meanwhile, the `--bind=` flag goes only to `OPENCLAW_GATEWAY_BIND` in `instance.env` (which becomes the gateway's `--bind` CLI flag) and `OPENCLAW_HOST_BIND` (which sets Docker port mapping). So an operator running `claws config show alpha --no-secrets` sees `"bind": "lan"` in the merged config even though the gateway binds to loopback at runtime. The two "bind" concepts use the same key name.

> *Suggested fix:* either resolve `gateway.bind` from the env at create time so it matches the actual runtime, or rename the JSON key to `gateway.listen` to disambiguate.

### S2 — `claws audit` enumerates **all** containers on the host, not those scoped to `OPENCLAW_ROOT` (MEDIUM, scope leak)

`scripts/security-audit.sh` step 3 walks `docker ps` output and reports on every running clawctl-style container. During the smoke run against an empty test root, it audited `team-lead`, `team-sarah`, `team-john`, `team1-ben`, `bob` — all from the operator's *real* `~/.openclaw`. This is informative but conceptually wrong: the audit takes an `OPENCLAW_ROOT` argument and should only audit instances rooted there.

> *Suggested fix:* filter `docker ps` by container name prefix that matches instances registered in `$ROOT/.port-registry`.

### S3 — `task list` and `channel status`/`channel security` are text-only (MEDIUM, scripting gap)

For a tool that wants to be embeddable in CI, manager-bot workflows, and "second clawctl" deployments, the absence of `--json` on the task queue and channel-status commands is the largest concrete blocker for scripting. The Task struct already JSON-marshals (the files on disk *are* JSON). The fix is a few `if jsonMode` branches.

### S4 — `claws init --force` is undocumented (LOW)

The flag exists in `init.go` paths (used by tests) but is not in `help.go`'s `subcommandHelp` for `init`. Documented behavior: idempotent. Actual behavior with `--force`: overwrites policy/access. An operator who knows to try `--force` likely knows what they're doing, but it's worth a docs line.

### S5 — `status <name>` shows an empty `docker compose ps` table when the container doesn't exist (LOW, cosmetic)

Two-line table header with no rows. Suppress the table when `containerStatus()` returns empty/created, or replace with `(no container — run: claws start <name>)`.

### S6 — Help section counter in my smoke script is wrong (NOT a claws bug)

Logged for honesty: my `grep -c ':$'` heuristic counts 0 sections because the headers are colored, so the section count in `dogfood_log_2026-05-20.out` reads `(help has 130 lines / 0 sections)`. Visually the help has the Getting-Started, Lifecycle, Info, Auth & Channels, Backup, Groups & Teams, Tasks, Shared Resources, Storage, Observability, Runtime, Image & Upgrade, Admin, Config, Diagnostics, Operations — sixteen sections, 130 lines.

## Boundaries we did not cross (and why)

- **No `start`/`stop`/`restart`/`upgrade`.** These call `docker compose up/down` for real. Smoke set `CLAWS_SKIP_VALIDATE=1` to skip the `docker compose config` validation but the actual lifecycle commands would still invoke Docker. Out of scope for a read-only audit. Health probes against the test root return `down` (correct verdict; no container).
- **No real auth (`claws auth … codex` / `… apikey`).** Codex OAuth needs an interactive TTY and a real OpenAI account; apikey would shell into the CLI service which requires a running container.
- **No real channels.** Telegram/Discord/Slack tokens are real secrets — we only exercised the config-write path.
- **No S3, no Caddy.** Storage and proxy commands would mutate AWS and `/etc/caddy/`.

These boundaries are documented in `tests/test_plan_2026-05-20.md` ("Out of scope") so a future tester knows what's covered and what isn't.

## Test root left for inspection

`/tmp/clawctl-dogfood-XzY6Kj` is preserved. Contains: `policy.json`, `.access.json`, `defaults.json`, `docker-compose.yml`, `.port-registry` (empty after teardown), `.audit.log`, `shared/`, `research/{lead,sarah,dev1,shared/{tasks,output,…}}`, `runtimes/demo.{json,-compose.yml}`. Operator can `ls -R` it freely. Removing it (when desired) is left to the operator.
