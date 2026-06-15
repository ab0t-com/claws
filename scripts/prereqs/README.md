# claws prereq installers

A folder of self-contained installer scripts that bring a fresh machine
to the state claws needs. Each script is a single bash file with no
source dependencies, so they all work correctly when fetched via
`curl … | bash`.

Designed to be **reusable** across our other tools (sharedwatch,
intent-gateway, future projects): if you ship a CLI that needs
docker, just point users at `install-docker.sh` and you've solved
the bootstrap problem.

---

## The prereqs

| Tool | Required | Why |
|---|---|---|
| **bash** | yes | The installer scripts themselves |
| **curl** | yes | Fetches install + update artifacts; `claws update` uses it |
| **docker** (engine + daemon) | **yes** | Every agent runs in a container |
| **docker compose** (v2 plugin) | **yes** | Per-agent compose orchestration |
| **git** | recommended | Needed for source builds + CONTRIBUTING workflow |
| **tar, gzip, sha256sum** | yes (usually preinstalled) | Unpacking + verifying release tarballs |

Most modern Linux distros have `bash`, `tar`, `gzip`, and `sha256sum`
preinstalled. **Docker is the one that's almost always missing on a
fresh box.**

---

## Quick start

### One command — install everything

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-all.sh | bash
```

Detects your OS (Ubuntu/Debian/Fedora/RHEL/Arch/Alpine/macOS), tells
you what it's about to do, asks for confirmation, then runs each
sub-installer in turn. Idempotent — skips anything already installed.

### Just check what's missing

```bash
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/check.sh | bash
```

Read-only. Reports installed/missing for each prereq, exits 0 if
everything required is present.

### Install one specific tool

```bash
# docker engine + compose plugin (via Docker's official get.docker.com)
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-docker.sh | bash

# git
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-git.sh | bash

# curl (chicken/egg: you need curl to download this — usually you
# install curl via your package manager directly)
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-curl.sh | bash
```

---

## OS support

| OS | Path |
|---|---|
| Ubuntu 20.04+ | `apt-get` + `get.docker.com` |
| Debian 11+ | `apt-get` + `get.docker.com` |
| Fedora 38+ | `dnf` + `get.docker.com` |
| RHEL / CentOS / Rocky / Alma 8+ | `yum`/`dnf` + `get.docker.com` |
| **Amazon Linux 2** | `amazon-linux-extras install docker` + compose plugin from GitHub |
| **Amazon Linux 2023** | `dnf install docker` + compose plugin from GitHub |
| Arch Linux | `pacman` + `get.docker.com` |
| Alpine | `apk` (docker only, no compose plugin auto) |
| macOS (Intel + Apple Silicon) | `brew install --cask docker` |

Amazon Linux gets a dedicated install path because Docker's `get.docker.com` script rejects it with `ERROR: Unsupported distribution 'amzn'`. The script auto-detects `amzn` in `/etc/os-release` and routes to the native package install — no `--method=` flag needed. The compose v2 plugin isn't in Amazon Linux's repos, so it's installed from Docker's GitHub releases.

For Windows: use WSL2 (Ubuntu) and run the installer there.

---

## Common flags (all scripts)

| Flag | What it does |
|---|---|
| `--yes` / `-y` | Skip all confirmation prompts |
| `--dry-run` | Print what would happen, don't change anything |
| `--help` / `-h` | Show usage |

Plus `install-all.sh` has `--required-only` to skip optional installers
(e.g. git).

---

## What the installers actually do

### `install-docker.sh`

On Linux: runs Docker's official `get.docker.com` convenience script —
the one Docker recommends for non-production / first-install. It
auto-detects the distro and installs from Docker's own repos.

Then:
- Enables the docker daemon via systemd (`sudo systemctl enable --now docker`)
- Adds the current user to the `docker` group so they don't need
  sudo for every command (requires log-out / `newgrp docker` to take
  effect)

On macOS: `brew install --cask docker` (or manual link if Homebrew
isn't installed). Reminds you to open Docker Desktop once to start
the daemon.

The Docker Compose v2 plugin is included with modern Docker installs —
no separate step needed.

### `install-git.sh` / `install-curl.sh`

`apt-get install <tool>` / `dnf install <tool>` / `pacman -S <tool>` /
`apk add <tool>`. On macOS: triggers Xcode Command Line Tools, or
suggests `brew install <tool>`.

### `install-all.sh`

Universal orchestrator. Checks what's missing, prints a plan, asks
for confirmation, then runs each sub-installer in sequence. Prefers
local sibling scripts when running from a repo checkout, falls back
to curl-fetching them from `raw.githubusercontent.com` when run as
a one-liner.

### `check.sh`

Read-only diagnostic. Useful in CI / scripts to gate on prereqs being
present:

```bash
if ! curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/check.sh | bash --quiet; then
    echo "missing prereqs; bailing"
    exit 1
fi
```

---

## Reuse in other tools

These scripts have nothing claws-specific in them. To adopt for
another project:

1. Copy `scripts/prereqs/` into your repo
2. Update the `REPO=` constant near the top of `check.sh`,
   `install-all.sh`, and `install-docker.sh` to point at your repo
3. Drop any installers you don't need
4. Point your installer one-liner at your raw.githubusercontent.com
   path

The OS detection block (inline `detect_os` function in each script)
is reusable as-is.

---

## What we explicitly DO NOT install

- **Go** — only needed for source builds. The standard `install.sh`
  fetches a prebuilt binary; users don't need Go to run claws.
- **Node.js** — only used INSIDE the openclaw runtime container, not
  on the host.
- **systemd, sudo, tar, gzip** — preinstalled on every supported
  platform. If your box is missing these, the installer scripts
  will warn but not try to install them.

---

## Failure modes

- **"docker installed but daemon isn't running"** — Linux:
  `sudo systemctl start docker`. macOS: open Docker Desktop from
  `/Applications`.
- **"Permission denied" on docker commands after install** — Log
  out and back in (or `newgrp docker`) for group membership to
  take effect.
- **`get.docker.com` unreachable** — Docker's CDN might be
  temporarily down; the script fails with a clear curl error.
  Fall back to manual install: https://docs.docker.com/engine/install/
- **Unknown distro** — The script prints the OS it detected and
  exits. Open an issue with `/etc/os-release` contents and we'll
  add support.

---

## See also

- [`scripts/install.sh`](../install.sh) — claws binary installer
  (assumes prereqs are present)
- [`tickets/prereq-installer-2026-05-24/`](../../tickets/prereq-installer-2026-05-24/ticket.md)
  — design ticket
- `claws doctor` — in-CLI diagnostic that runs after install
