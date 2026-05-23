# clawctl

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Multi-instance manager for [OpenClaw](https://github.com/openclaw/openclaw). Run multiple isolated AI agent instances on a single server, each with its own identity, credentials, channels, and workspace.

## Install

```bash
# Recommended — pinned release with checksum verification:
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | sh

# Or specify a version:
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | sh -s -- --version=v1.0.0
```

The installer downloads the latest GitHub release, verifies the SHA256 against `SHA256SUMS`, and installs to `/usr/local/bin` (or `~/.local/bin` if not writable). Pass `--help` to see all options.

## Quick Start

```bash
# 1. Build from source (or use install.sh above)
./scripts/rebuild.sh                     # build + vet + short tests
# or:  go build -o clawctl ./cmd/clawctl/

# 2. Initialize
./clawctl init

# 3. Create your first instance
./clawctl create alice

# 4. Add authentication
./clawctl auth alice codex              # OpenAI Codex OAuth
./clawctl auth alice apikey openai sk-… # or API key

# 5. Start
./clawctl start alice

# 6. Connect (SSH tunnel from your laptop)
./clawctl tunnel
# prints: ssh -N -L 18789:127.0.0.1:18789 ubuntu@<server>
```

## Commands

Run `clawctl help` for the full list, or `clawctl <command> --help` for details on any command.

### Lifecycle

```
clawctl create <name>              Create a new instance
clawctl start <name>               Start (waits for health check)
clawctl stop <name>                Stop
clawctl restart <name>             Restart
clawctl remove <name> [--purge]    Remove (--purge deletes all data)
```

### Info

```
clawctl list [--json]              List all instances
clawctl status <name> [--json]     Show instance details
clawctl health [name...] [--json]  Deep health probe
clawctl dashboard [--interval=5s]  Live refreshing view
clawctl activity [--since=2h]      Recent actions across instances
```

### Groups

Organize instances into groups with shared resources:

```
clawctl group create backend
clawctl create backend/sarah
clawctl create backend/bob
clawctl group list
```

### Manager/Worker Roles

Assign roles within a group for task-based coordination:

```
clawctl create team/lead --role=manager
clawctl create team/dev1 --role=worker --manager=lead

clawctl task create team "review PR #42" --from=lead
clawctl task list team
clawctl task claim team <id> --by=dev1
clawctl task complete team <id> --result="approved"
```

> **Note:** Tasks use filesystem `rename()` for atomic transitions and only work on local storage, not S3 FUSE mounts.

### Channels (WhatsApp, Telegram, Discord, Slack, Signal, etc.)

Connect instances to messaging platforms. Each instance can run multiple channels simultaneously. See [docs/channels.md](docs/channels.md) for the full guide.

```bash
# Quick: use the interactive wizard
clawctl channel alice telegram

# Or configure directly
clawctl exec alice config set channels.telegram.enabled true --json
clawctl exec alice config set channels.telegram.botToken '"123:ABC..."' --json
clawctl exec alice config set channels.telegram.dmPolicy '"pairing"' --json
clawctl restart alice

# WhatsApp requires QR scan
clawctl exec alice channels login --channel whatsapp

# Check channel status
clawctl exec alice channels status --probe
```

### Storage (S3)

```
clawctl storage setup --bucket=my-backup
clawctl storage sync                     # additive copy (safe)
clawctl storage sync --mirror --yes      # destructive mirror
clawctl storage mount                    # FUSE mount
clawctl storage status
```

### Proxy (Caddy)

```
clawctl proxy setup --domain=ai.example.com
clawctl proxy setup --domain=ai.example.com --dry-run
clawctl proxy status
```

Auth headers are injected by default (disable with `--no-auth`). Config is written to `/etc/caddy/conf.d/clawctl.conf` (not the main Caddyfile).

### Image & Upgrade

```bash
clawctl image list                              # list local images
clawctl image pull openclaw:v2026.3.25          # pull from registry
clawctl image pin team/sarah openclaw:v2026.3.25  # pin instance to version

clawctl upgrade team/sarah --image=openclaw:v2026.4.1  # upgrade with rollback
clawctl upgrade --all --image=openclaw:v2026.4.1       # upgrade all instances
```

Upgrade stops the instance, starts with the new image, waits for health check. If health fails within 30s, it automatically rolls back to the previous image.

### Backup & Restore

```bash
clawctl backup alice
clawctl backup alice --exclude-credentials
clawctl restore alice alice-backup-20260317.tar.gz
```

### Admin — Security & Access

```bash
# Security policy (restricts what instances can do)
clawctl policy init                    # create secure defaults
clawctl policy validate                # check all instances
clawctl policy enforce --restart       # fix all violations + restart

# Access control (restricts who can run clawctl)
clawctl access init                    # set up roles (you become admin)
clawctl access grant deploy-bot operator  # give limited access
clawctl access grant alice user        # read-only, scoped to their instance
clawctl access audit --since=1h        # view command audit log

# Token management
clawctl token rotate team/sarah        # rotate gateway auth token
clawctl token show team/sarah          # view token (truncated)

# Security audit (comprehensive check)
clawctl audit                          # runs all security checks
clawctl doctor --fix                   # auto-fix file permissions
```

### Diagnostics

```bash
clawctl init           # first-run setup
clawctl version        # versions and environment
clawctl doctor         # check Docker, image, disk, tools
clawctl doctor --fix   # auto-fix file permissions
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
| `CLAWCTL_BASE_PORT` | `18789` | Starting port for allocation |
| `CLAWCTL_SKIP_VALIDATE` | (unset) | Skip compose config validation |

## Architecture

- **Zero external Go dependencies** — standard library only
- **File-based state** — `.port-registry`, `instance.env`, `openclaw.json`
- **Docker Compose substrate** — shared template + per-instance override
- **File locking** — `flock()` on all registry/config writes
- **150+ tests** — unit + integration, all passing

## Repository Layout

```
.
├── cmd/clawctl/         Go source (all .go files, package main)
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
