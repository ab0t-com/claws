# claws prereq installers — strict variant

Same shape as [`scripts/prereqs/`](../prereqs/), with extra guards for
**audit-conscious, regulated, or corporate-managed environments**.

If the simple installers work for you, use those. Use these when:

- Your security team needs to **review every command before it runs**
- You need an **audit trail** for compliance
- You're running on a **hardened or policy-managed host** (CIS, STIG,
  internal baseline)
- You want **opt-out switches** for policies that conflict with
  defaults (e.g. don't put users in the `docker` group)

---

## What's different from `scripts/prereqs/`

| Feature | Simple | **Strict** |
|---|---|---|
| Echo every command before running | no | **yes** (`$ <cmd>` line) |
| `--audit` mode (print plan, run nothing) | `--dry-run` only | **`--audit`** + `--dry-run` |
| Log every action to `/tmp/claws-prereqs-<ts>.log` | no | **yes** |
| `CLAWS_NO_INSTALL=1` env opt-out | no | **yes** (refuses to install, exit 0) |
| Refuses to run inside containers | no | **yes** |
| Refuses to overwrite `/etc/docker/daemon.json` | no | **yes** |
| `--no-group` to skip docker group membership | no | **yes** |
| `--method=getdocker\|distro\|skip` for docker | no | **yes** |
| CI environments auto-skip prompts | no | **yes** |
| openSUSE / zypper support | no | **yes** |
| `--json` output (for `check.sh`) | no | **yes** |
| **Amazon Linux auto-route** | yes (v1.6.18+) | **yes** (v1.6.18+) |
| Idempotent (never uninstalls anything) | yes | **yes** |
| Self-contained (no source dependencies) | yes | **yes** |

---

## Quick start

### Audit mode — print every command, change nothing

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/install-all.sh | bash -s -- --audit
```

Output is the full sequence of commands that would run, plus
detected environment (OS, package manager, CI, proxy, container).
Hand this to your security team for review.

### Install for real after audit

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/install-all.sh | bash
```

Prompts for confirmation before any change. The log file path is
printed at the end.

### Check current state

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/check.sh | bash
```

Read-only. Add `--json` for machine-readable output:

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/check.sh | bash -s -- --json | jq
```

---

## Flags

All scripts support:

| Flag | Effect |
|---|---|
| `--audit` | Print every command, change nothing. Best for security review. |
| `--dry-run` | Same as `--audit` (compatibility). |
| `--yes` / `-y` | Skip confirmation prompts. |
| `--help` / `-h` | Show usage. |

`install-docker.sh` additionally supports:

| Flag | Effect |
|---|---|
| `--no-group` | Don't add `$USER` to the docker group. |
| `--method=getdocker` | (Default.) Install via Docker's official `get.docker.com` script. |
| `--method=distro` | Install via the OS package manager (`apt`, `dnf`, `zypper`, `pacman`, `apk`). |
| `--method=skip` | Don't install docker; only do post-install steps. |

`install-all.sh`:

| Flag | Effect |
|---|---|
| `--required-only` | Skip optional installers (e.g. `git`). |

---

## Environment variables

| Variable | Effect |
|---|---|
| `CLAWS_NO_INSTALL=1` | Every script refuses to install. Exits 0 cleanly. Use this as a global "no installs on this host" policy switch. |
| `CLAWS_PREREQS_LOG=<path>` | Override log file location. Default `/tmp/claws-prereqs-<timestamp>-<pid>.log`. |
| `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` | Honoured by `curl` + `apt` automatically; logged for audit. |
| `CI`, `GITHUB_ACTIONS`, `GITLAB_CI`, `CIRCLECI`, `BUILDKITE` | Detected; prompts auto-skipped but no install happens unless `--yes` is also passed. |

---

## What every script does on safety

Common preamble in every install script (runs first):

1. **`CLAWS_NO_INSTALL=1`?** → exit 0 with a warning.
2. **Inside a container?** → refuse with a clear error.
3. **CI detected?** → set `--yes` automatically (prompts would hang).
4. **Non-TTY stdin?** → set `--yes` automatically (cloud-init / agent
   automation / curl-pipe).
5. **Running as root?** → note it; sudo not needed.
6. **Detect OS** → print the detected family, distro, version,
   package manager, architecture.
7. **Print the plan** → show every command that will run.
8. **Wait for explicit confirmation** (unless `--yes`).
9. **Echo each command before running it** — no surprises.
10. **Mirror all output to the log file**.
11. **Verify** at the end and print success/failure.

---

## What every script does NOT do

- **Uninstall anything.** Ever.
- **Overwrite existing configuration files.** Specifically
  `/etc/docker/daemon.json`. We leave it alone and warn.
- **Modify users or groups beyond adding to `docker`** — and even
  that is skippable via `--no-group`.
- **Disable existing services.** We only enable `docker`.
- **Modify firewall, networking, or SELinux/AppArmor policies.**

If you need any of those, do them yourself or through your existing
config-management system. These scripts are bootstrap helpers, not
config management.

---

## OS support

| OS | Tested | Path |
|---|---|---|
| Ubuntu 20.04+ | ✓ | `apt` + `get.docker.com` |
| Debian 11+ | ✓ | `apt` + `get.docker.com` |
| Fedora 38+ | ✓ | `dnf` + `get.docker.com` |
| RHEL / CentOS / Rocky / Alma 8+ | ✓ | `yum`/`dnf` + `get.docker.com` |
| **Amazon Linux 2** | ✓ | `amazon-linux-extras install docker` + compose plugin from GitHub |
| **Amazon Linux 2023** | ✓ | `dnf install docker` + compose plugin from GitHub |
| openSUSE / SLES | ✓ (strict only) | `zypper` |
| Arch Linux | ✓ | `pacman` + `get.docker.com` |
| Alpine | partial | `apk` |
| macOS (Intel + Apple Silicon) | ✓ | `brew install --cask docker` |

Amazon Linux gets a dedicated path because `get.docker.com` rejects
`amzn` with `ERROR: Unsupported distribution 'amzn'`. The script
auto-detects `amzn` in `/etc/os-release` and overrides
`--method=getdocker` to a new internal `amzn` method — operators
don't need to know. Explicit `--method=distro` still wins.
The compose v2 plugin isn't in Amazon Linux's repos, so it's
installed from `github.com/docker/compose/releases`.

For Windows: WSL2 (Ubuntu), then run the installer inside WSL.

---

## Audit log format

Each script writes a timestamped log to
`/tmp/claws-prereqs-<timestamp>-<pid>.log`. Format:

```
2026-05-24T07:31:02Z STEP: Plan
2026-05-24T07:31:02Z OK: docker + compose already installed
2026-05-24T07:31:02Z EXEC: sudo apt-get update -y
2026-05-24T07:31:08Z OK: git --version 2.43.0
```

Categories: `STEP`, `EXEC`, `OK`, `WARN`, `DIE`, `PRESENT`,
`MISSING REQUIRED`, `MISSING OPTIONAL`, `DAEMON`, `RESULT`, `PROXY`,
`OS`, `ROOT`.

Easy to grep, easy to ship to a SIEM.

---

## See also

- [`scripts/prereqs/`](../prereqs/) — the simpler / friendlier variant
- [`scripts/install.sh`](../install.sh) — the claws binary installer
- [`tickets/prereq-installer-2026-05-24/`](../../tickets/prereq-installer-2026-05-24/ticket.md) — design doc
