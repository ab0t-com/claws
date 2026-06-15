# prereq-installer — bootstrap docker / git / curl on a fresh box, and fail helpfully when they're missing

**Filed:** 2026-05-24
**Target:** v1.6.16
**Status:** Open
**Severity:** High (a fresh-box install currently fails opaquely; non-technical users have no path to recovery)
**Owner:** TBD

---

## The user-observable problem

A client tried to use claws on a fresh box that didn't have docker
installed. claws blew up mid-command with the standard Go error:

```
exec: "docker": executable file not found in $PATH
```

A non-technical user cannot act on that. They don't know:

- What "exec" is
- That docker is supposed to be installed
- How to install docker on their OS
- That claws also needs the `docker compose` plugin specifically
  (not the legacy `docker-compose` binary)
- That the daemon also needs to be running

Three problems stacked:

1. **No prereq documentation.** README's "Install" section assumes
   docker is present.
2. **No prereq installer.** A non-technical user shouldn't have to
   read Docker's docs and pick the right install path for their
   distro.
3. **Unhelpful CLI errors.** When the prereq is missing, claws
   leaks Go's exec.LookPath error instead of saying anything useful.

---

## What ships in this ticket (v1.6.16)

### 1. `scripts/prereqs/` — a folder of self-contained installer scripts

Each script is a single bash file with no source dependencies, so
they all work correctly when fetched via `curl … | bash`.

| File | Purpose |
|---|---|
| `check.sh` | Read-only diagnostic. Reports installed/missing for each prereq, exits 0 if all required are present. |
| `install-all.sh` | Universal orchestrator. Detects OS, runs each sub-installer for what's missing. |
| `install-docker.sh` | Docker engine + compose plugin. Linux: `get.docker.com`. macOS: `brew install --cask docker`. |
| `install-git.sh` | git via the OS package manager (or Xcode CLT on macOS). |
| `install-curl.sh` | curl via the OS package manager. (Chicken/egg: provided for completeness.) |
| `README.md` | Full reference: OS support matrix, flags, failure modes, reuse-in-other-tools instructions. |

**Why each script is self-contained:** they need to work when curl-piped
to bash. No source files at the same path, no shared lib (each script
inlines the ~25-line OS detection block). DRY violation, intentional —
auditability and "one curl URL = one self-contained install" beat DRY
for installer scripts.

**Why Docker's `get.docker.com`** rather than per-distro `apt`/`dnf`/etc:
it's officially maintained by Docker, supports every Linux distro,
auto-installs from Docker's own repos (not stale distro packages),
and includes the compose v2 plugin. The Docker docs themselves
recommend it for non-production / first-install.

### 2. CLI prereq guard — `cmd/claws/prereqs.go`

A `validatePrereqsForCommand(cmd)` function called from main.go's
dispatch loop BEFORE running any command. It:

1. Looks up `cmd` in a deny-list of commands that don't need docker
   (`version`, `help`, `update`, `doctor`, `init`, `paste-secret`,
   no-args invocation).
2. For everything else, calls `requireDocker()`:
   - Checks `docker` is on PATH
   - Checks `docker compose` plugin is available
   - Checks the daemon is reachable via `docker info`
3. On failure, returns a verbose actionable error pointing at:
   - The curl one-liner for `install-docker.sh`
   - The curl one-liner for `install-all.sh`
   - Manual Docker docs as a fallback

Sample error:

```
==> docker is not installed (manages the agent containers …).

  Quick install — auto-detects your OS:
    curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-docker.sh | bash

  Or install everything claws needs in one shot:
    curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-all.sh | bash

  Verify after install:
    claws doctor
```

The daemon-not-reachable error specifically calls out the three
common causes (daemon stopped, user not in docker group, config
issue) with a fix for each.

### 3. README Prerequisites section

A new section between the title and the "Install" section
documenting:
- The prereqs table (tool / required? / why)
- The one-line installer command
- The check.sh command
- Per-tool installer commands
- A pointer to `scripts/prereqs/README.md` for the full reference

---

## OS support

| OS | Status | How |
|---|---|---|
| Ubuntu 20.04+ | ✓ | `apt` + `get.docker.com` |
| Debian 11+ | ✓ | `apt` + `get.docker.com` |
| Fedora 38+ | ✓ | `dnf` + `get.docker.com` |
| RHEL / CentOS / Rocky / Alma 8+ | ✓ | `yum`/`dnf` + `get.docker.com` |
| Arch Linux | ✓ | `pacman` + `get.docker.com` |
| Alpine | partial | `apk` (no compose plugin auto) |
| macOS (Intel + Apple Silicon) | ✓ | `brew install --cask docker` |
| Windows | not directly | use WSL2 (Ubuntu) and run installer there |

---

## Reuse story

The user emphasised that these scripts should be reusable for our
other tools (sharedwatch, intent-gateway, future projects). The
scripts contain ONLY:

- OS detection (inline `detect_os` function)
- Package-manager dispatch
- The Docker official install script
- A `REPO=` constant at the top of `check.sh`, `install-all.sh`,
  and `install-docker.sh`

To adopt for another project:
1. Copy `scripts/prereqs/` into the other repo
2. Update the `REPO=` constant in those three files
3. Drop any installers the other project doesn't need

That's it. Documented in `scripts/prereqs/README.md` under
"Reuse in other tools".

---

## Decisions worth flagging

1. **`get.docker.com` over per-distro packages.** Maintained by
   Docker, includes the compose v2 plugin, supports every Linux
   distro. The alternative is to write per-distro Docker
   installation paths ourselves — much more code, easier to break
   when distros change repo URLs.

2. **Inlined OS detection, no shared lib.** Each installer
   duplicates the ~25-line detect_os function. Intentional — they
   need to work as standalone curl-pipeable files. The duplication
   is bounded (one function, well-tested) and easier to audit than
   a shared lib that has to be curl-fetched on demand.

3. **macOS via Homebrew + Docker Desktop cask.** No good
   programmatic install path for macOS Docker that doesn't go
   through Docker Desktop. The cask is the cleanest.

4. **Prereq check at dispatch time, not at command-start.** Adding
   the check inside every command would be invasive. Adding it
   once in main.go's dispatch is one place, one decision.

5. **Deny-list rather than allow-list for "needs docker" commands.**
   The list of docker-requiring commands is much longer than the
   list of docker-independent ones, so it's cheaper to maintain
   a small deny-list. Default is "needs docker" — safer to over-check
   than under-check.

6. **Compose plugin check is separate from binary check.** A docker
   install that's missing the compose plugin is a real failure
   mode (older legacy installs). We surface it with the specific
   fix rather than letting `docker compose` fail mid-command.

---

## Acceptance criteria

- A fresh Ubuntu box with NO docker installed can run
  `curl -fsSL .../install-all.sh | bash` and end up with docker +
  compose + git installed, daemon running, current user in docker
  group.
- The same box, AFTER running install-all.sh, can run
  `curl -fsSL .../install.sh | bash` and get claws installed.
- `claws list` (or any docker-using command) on a box WITHOUT
  docker prints the friendly error with the install command — not
  the Go exec.LookPath error.
- `claws version`, `claws help`, `claws update`, `claws doctor` all
  continue to work on a box without docker (the deny-list).
- `claws doctor` continues to work even when docker is broken —
  it's the diagnostic, it MUST work when other things don't.
- All five installer scripts pass `bash -n` syntax check.
- `check.sh --quiet` exits 0 on a properly-set-up box, 1 on a
  missing-prereq box.
- Each installer has `--yes` (no prompts) and `--dry-run` (no
  changes) flags.

---

## What's out of scope

- **Installing Go on the host.** Only needed for source builds;
  the binary install path (install.sh) doesn't need it. Users who
  want to build from source can read CONTRIBUTING.md.
- **Installing Node.js on the host.** Used only inside the openclaw
  runtime container, not on the host.
- **Installing the openclaw runtime image.** That's `claws image
  bootstrap`'s job (already exists).
- **Windows-native install.** WSL2 is the supported path.
- **Air-gapped install.** Users on disconnected boxes need to
  pre-stage the installer scripts and the docker packages
  themselves. The prereq scripts assume internet access.

---

## Follow-ups for later patches

- **Add `claws prereqs check` and `claws prereqs install` subcommands** —
  CLI wrappers around the bash scripts. Currently the prereqs live
  outside the CLI (you can't be in the CLI if claws isn't installed),
  but for a user who installed claws WITHOUT docker (somehow), a
  `claws prereqs install` would save them a curl.
- **Self-update the prereq scripts.** When `claws update` runs, it
  could also check whether the installer scripts at the
  raw.githubusercontent URL have changed and tell the user. Lower
  priority — the scripts are stable.
- **Air-gapped support.** A `--from-tarball=<path>` flag on
  install-docker.sh for boxes without internet. Useful for
  enterprise deployments.
- **Windows-native PowerShell installer.** Out of scope here; would
  be a separate ticket.
