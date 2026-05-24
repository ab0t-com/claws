# claws

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Multi-instance manager for [OpenClaw](https://github.com/openclaw/openclaw). Run multiple isolated AI agent instances on a single server, each with its own identity, credentials, channels, and workspace.

## Install

```bash
# Recommended — pinned release with checksum verification:
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | bash

# Or specify a version:
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | bash -s -- --version=v1.0.0
```

The installer downloads the latest GitHub release, verifies the SHA256 against `SHA256SUMS`, and installs to `/usr/local/bin` (or `~/.local/bin` if not writable). Pass `--help` to see all options.

## Quick Start

```bash
# 1. Build from source (or use install.sh above)
./scripts/rebuild.sh                     # build + vet + short tests
# or:  go build -o claws ./cmd/claws/

# 2. Initialize
./claws init

# 3. Create your first instance
./claws create alice

# 4. Add authentication
./claws auth alice codex              # OpenAI Codex OAuth
./claws auth alice apikey openai sk-… # or API key

# 5. Start
./claws start alice

# 6. Connect (SSH tunnel from your laptop)
./claws tunnel
# prints: ssh -N -L 18789:127.0.0.1:18789 ubuntu@<server>
```

## Commands

Run `claws help` for the full list, or `claws <command> --help` for details on any command.

### Lifecycle

```
claws create <name>              Create a new instance
claws start <name>               Start (waits for health check)
claws stop <name>                Stop
claws restart <name>             Restart
claws remove <name> [--purge]    Remove (--purge deletes all data)
```

### Info

```
claws list [--json]              List all instances
claws status <name> [--json]     Show instance details
claws health [name...] [--json]  Deep health probe
claws dashboard [--interval=5s]  Live refreshing view
claws activity [--since=2h]      Recent actions across instances
```

### Groups

Organize instances into groups with shared resources:

```
claws group create backend
claws create backend/sarah
claws create backend/bob
claws group list
```

### Manager/Worker Roles

Assign roles within a group for task-based coordination:

```
claws create team/lead --role=manager
claws create team/dev1 --role=worker --manager=lead

claws task create team "review PR #42" --from=lead
claws task list team
claws task claim team <id> --by=dev1
claws task complete team <id> --result="approved"
```

> **Note:** Tasks use filesystem `rename()` for atomic transitions and only work on local storage, not S3 FUSE mounts.

### Channels (WhatsApp, Telegram, Discord, Slack, Signal, etc.)

Connect instances to messaging platforms. Each instance can run multiple channels simultaneously. See [docs/channels.md](docs/channels.md) for the full guide.

```bash
# Quick: use the interactive wizard
claws channel alice telegram

# Or configure directly
claws exec alice config set channels.telegram.enabled true --json
claws exec alice config set channels.telegram.botToken '"123:ABC..."' --json
claws exec alice config set channels.telegram.dmPolicy '"pairing"' --json
claws restart alice

# WhatsApp requires QR scan
claws exec alice channels login --channel whatsapp

# Check channel status
claws exec alice channels status --probe
```

### Storage (S3)

```
claws storage setup --bucket=my-backup
claws storage sync                     # additive copy (safe)
claws storage sync --mirror --yes      # destructive mirror
claws storage mount                    # FUSE mount
claws storage status
```

### Proxy (Caddy)

```
claws proxy setup --domain=ai.example.com
claws proxy setup --domain=ai.example.com --dry-run
claws proxy status
```

Auth headers are injected by default (disable with `--no-auth`). Config is written to `/etc/caddy/conf.d/claws.conf` (not the main Caddyfile).

### Image & Upgrade

```bash
claws image list                              # list local images
claws image pull openclaw:v2026.3.25          # pull from registry
claws image pin team/sarah openclaw:v2026.3.25  # pin instance to version

claws upgrade team/sarah --image=openclaw:v2026.4.1  # upgrade with rollback
claws upgrade --all --image=openclaw:v2026.4.1       # upgrade all instances
```

Upgrade stops the instance, starts with the new image, waits for health check. If health fails within 30s, it automatically rolls back to the previous image.

### Backup & Restore

```bash
claws backup alice
claws backup alice --exclude-credentials
claws restore alice alice-backup-20260317.tar.gz
```

### Admin — Security & Access

```bash
# Security policy (restricts what instances can do)
claws policy init                    # create secure defaults
claws policy validate                # check all instances
claws policy enforce --restart       # fix all violations + restart

# Access control (restricts who can run claws)
claws access init                    # set up roles (you become admin)
claws access grant deploy-bot operator  # give limited access
claws access grant alice user        # read-only, scoped to their instance
claws access audit --since=1h        # view command audit log

# Token management
claws token rotate team/sarah        # rotate gateway auth token
claws token show team/sarah          # view token (truncated)

# Security audit (comprehensive check)
claws audit                          # runs all security checks
claws doctor --fix                   # auto-fix file permissions
```

### Diagnostics

```bash
claws init           # first-run setup
claws version        # versions and environment
claws doctor         # check Docker, image, disk, tools
claws doctor --fix   # auto-fix file permissions
```

## Configuration

Instances are configured via layered JSON merge:

1. **Global defaults** — `~/.openclaw/defaults.json`
2. **Group defaults** — `~/.openclaw/<group>/defaults.json`
3. **Template** — `--from=<instance>` copies another instance's config
4. **Instance skeleton** — port, token, auth (always wins)

## Requirements

- Go 1.22+ (build only)
- Docker with Compose v2
- OpenClaw Docker image (`openclaw:local` or set `OPENCLAW_IMAGE`)

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENCLAW_ROOT` | `~/.openclaw` | Root directory for all instances |
| `OPENCLAW_IMAGE` | `openclaw:local` | Docker image to use |
| `CLAWS_BASE_PORT` | `18789` | Starting port for allocation |
| `CLAWS_SKIP_VALIDATE` | (unset) | Skip compose config validation |

## Architecture

- **Zero external Go dependencies** — standard library only
- **File-based state** — `.port-registry`, `instance.env`, `openclaw.json`
- **Docker Compose substrate** — shared template + per-instance override
- **File locking** — `flock()` on all registry/config writes
- **150+ tests** — unit + integration, all passing

## Repository Layout

```
.
├── cmd/claws/         Go source (all .go files, package main)
├── html/                Static UI / landing pages bundled in releases
├── docs/                Markdown documentation (channels, runtimes)
├── scripts/
│   ├── install.sh       End-user installer (remote + local-dev + local-release)
│   ├── rebuild.sh       Local-dev inner-loop build (+ vet + short tests)
│   ├── release.sh       Cross-compile linux/darwin × amd64/arm64 → release/
│   ├── install-hooks.sh Installs gitleaks + git hooks
│   ├── security-audit.sh Deployment hardening check (bundled in releases)
│   └── hooks/           pre-commit / pre-push / commit-msg (gitleaks)
├── docker-compose.yml   Substrate template for instance containers
├── LICENSE              MIT
├── .gitleaks.toml       Secret-scanning rules
└── .gitignore
```

Release tarballs are produced by `scripts/release.sh` and contain the binary, `docker-compose.yml`, `install.sh`, `security-audit.sh`, `LICENSE`, `README.md`, `html/`, `docs/`, and a per-target `MANIFEST.txt` of SHA256 sums.

## Contributing

```bash
# Install git hooks + gitleaks (secret-scanning at commit time)
./scripts/install-hooks.sh

# Inner-loop:
./scripts/rebuild.sh             # build + vet + short tests
./scripts/rebuild.sh --quick     # build only
./scripts/rebuild.sh --race      # build + tests with -race
```

## License

[MIT](LICENSE) © 2026 ab0t.com
