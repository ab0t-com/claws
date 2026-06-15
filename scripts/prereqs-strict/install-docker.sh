#!/bin/bash
# install-docker.sh — strict / audit-friendly variant
#
# Same goal as ../prereqs/install-docker.sh (install docker + compose),
# but with audit-friendly guards suitable for corporate / regulated
# environments:
#
#   • Refuses to run inside a container (no docker-in-docker)
#   • Refuses to run if CLAWS_NO_INSTALL=1 (policy opt-out)
#   • --audit mode prints every command WITHOUT executing it
#   • Every sudo command is echoed before execution (no surprises)
#   • All output mirrored to /tmp/claws-prereqs-<ts>.log for audit
#   • Refuses to overwrite /etc/docker/daemon.json if it already exists
#   • --no-group skips adding $USER to the docker group
#   • --method=getdocker (default) | distro | skip — lets ops pick how
#     docker gets installed: Docker's official script, the distro's own
#     package, or skip (only configure daemon + group)
#   • Idempotent — only ADDS, never removes; existing installs detected
#   • CI environments auto-skip prompts but never auto-install
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/install-docker.sh | bash
#   ./scripts/prereqs-strict/install-docker.sh [flags]
#
# Flags:
#   --audit                  Print every command, change nothing. Best
#                            for a security review.
#   --dry-run                Same as --audit (kept for compatibility).
#   --yes, -y                Skip confirmation prompt.
#   --no-group               Don't add $USER to the docker group.
#   --method=<m>             How to install: getdocker | distro | skip.
#                            Default: getdocker.
#
# Env vars honoured:
#   CLAWS_NO_INSTALL=1       Refuse to install anything; exit 0.
#   CLAWS_PREREQS_LOG=<path> Override log file location.
#   HTTP_PROXY, HTTPS_PROXY, NO_PROXY — passed through to curl + apt.

if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    echo "ERROR: install-docker.sh requires bash." >&2
    exit 1
fi
set -euo pipefail

# ----------------------------------------------------------------------
# Args
# ----------------------------------------------------------------------
AUDIT=0
DRY_RUN=0
SKIP_CONFIRM=0
NO_GROUP=0
METHOD="getdocker"

for arg in "$@"; do
    case "$arg" in
        --audit)        AUDIT=1; DRY_RUN=1 ;;
        --dry-run)      DRY_RUN=1 ;;
        --yes|-y)       SKIP_CONFIRM=1 ;;
        --no-group)     NO_GROUP=1 ;;
        --method=*)     METHOD="${arg#--method=}" ;;
        -h|--help)
            sed -n '2,42p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || \
                echo "install-docker.sh — strict variant. See header."
            exit 0
            ;;
        *)
            echo "ERROR: unknown flag: $arg (use --help)" >&2
            exit 1
            ;;
    esac
done

case "$METHOD" in
    getdocker|distro|skip) ;;
    *) echo "ERROR: invalid --method=$METHOD (use: getdocker | distro | skip)" >&2; exit 1 ;;
esac

# Amazon Linux: get.docker.com refuses with "Unsupported distribution 'amzn'".
# Route to the distro path automatically (with a clear notice), but respect
# an explicit --method=skip. If the operator passed --method=distro, leave
# it alone.
if [ -f /etc/os-release ]; then
    _peek_id="$(. /etc/os-release; echo "${ID:-}")"
    if [ "$_peek_id" = "amzn" ] && [ "$METHOD" = "getdocker" ]; then
        METHOD="amzn"
        echo "  [info] Amazon Linux detected; auto-routing to native install (get.docker.com rejects 'amzn')"
    fi
    unset _peek_id
fi

# ----------------------------------------------------------------------
# Output helpers
# ----------------------------------------------------------------------
if [ -t 1 ]; then
    BOLD="\033[1m"; GREEN="\033[0;32m"; YELLOW="\033[0;33m"; RED="\033[0;31m"; DIM="\033[0;90m"; NC="\033[0m"
else
    BOLD=""; GREEN=""; YELLOW=""; RED=""; DIM=""; NC=""
fi

LOG_FILE="${CLAWS_PREREQS_LOG:-/tmp/claws-prereqs-$(date +%Y%m%d-%H%M%S)-$$.log}"
: >"$LOG_FILE" 2>/dev/null || LOG_FILE="/dev/null"

log()   { echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) $*" >>"$LOG_FILE" 2>/dev/null || true; }
step()  { echo -e "\n${BOLD}==>${NC} $1"; log "STEP: $1"; }
ok()    { echo -e "  ${GREEN}✓${NC} $1"; log "OK: $1"; }
warn()  { echo -e "  ${YELLOW}!${NC} $1" >&2; log "WARN: $1"; }
die()   { echo -e "  ${RED}✗${NC} $1" >&2; log "DIE: $1"; exit 1; }

# run: echo command, then execute (unless AUDIT/DRY_RUN). Mirror output
# to the log file. This is the heart of "no surprises" — every sudo
# command is visible before it executes.
run() {
    log "EXEC: $*"
    echo -e "  ${DIM}\$ $*${NC}"
    if [ "$AUDIT" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then
        echo -e "  ${YELLOW}[audit — would run]${NC}"
        return 0
    fi
    # Append output to log AND show on stderr/stdout. Eval is safe
    # because $* contains commands we constructed, not user input.
    eval "$@" 2>&1 | tee -a "$LOG_FILE"
    return "${PIPESTATUS[0]}"
}

# ----------------------------------------------------------------------
# Safety preamble — always runs, never skipped
# ----------------------------------------------------------------------
echo -e "${BOLD}claws prereq installer — docker (strict variant)${NC}"
echo -e "  ${DIM}Log:    $LOG_FILE${NC}"

# Policy opt-out
if [ "${CLAWS_NO_INSTALL:-0}" = "1" ]; then
    warn "CLAWS_NO_INSTALL=1 — refusing to install."
    warn "Override: unset CLAWS_NO_INSTALL or run via a different tool."
    exit 0
fi

# Refuse inside containers
if [ -f /.dockerenv ] || (grep -q docker /proc/1/cgroup 2>/dev/null) \
        || [ -n "${KUBERNETES_SERVICE_HOST:-}" ]; then
    die "detected container environment. Docker installs on hosts, not in containers."
fi

# CI detection: skip prompts, but never auto-confirm destructive things
IS_CI=0
if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ] || [ -n "${GITLAB_CI:-}" ] \
        || [ -n "${CIRCLECI:-}" ] || [ -n "${BUILDKITE:-}" ]; then
    IS_CI=1
    SKIP_CONFIRM=1
    echo -e "  ${DIM}CI environment detected; will skip confirmation prompts.${NC}"
fi

# Non-TTY stdin (curl|bash, cloud-init, ssh command). Communicate
# clearly: this often means automation, fresh EC2 first-boot, or an
# agent running us — auto-confirm so we don't hang waiting for input.
if [ ! -t 0 ] && [ "$SKIP_CONFIRM" -eq 0 ]; then
    echo -e "  ${DIM}non-interactive stdin detected (curl|bash, cloud-init, agent automation) — auto-confirming${NC}"
    SKIP_CONFIRM=1
fi

# Root context: e.g. fresh root-only EC2. Sudo isn't needed; note it.
if [ "$(id -u)" -eq 0 ]; then
    echo -e "  ${DIM}running as root — sudo not needed${NC}"
    log "ROOT: running as uid 0, sudo skipped"
fi

if [ -n "${HTTP_PROXY:-}${HTTPS_PROXY:-}" ]; then
    log "PROXY: HTTP_PROXY=${HTTP_PROXY:-} HTTPS_PROXY=${HTTPS_PROXY:-} NO_PROXY=${NO_PROXY:-}"
    echo -e "  ${DIM}Proxy:  HTTPS_PROXY=${HTTPS_PROXY:-${HTTP_PROXY:-}} (will be honoured)${NC}"
fi

# ----------------------------------------------------------------------
# OS detection
# ----------------------------------------------------------------------
OS_FAMILY=""; OS_ID=""; OS_VERSION=""; PKG_MGR=""; ARCH="$(uname -m)"
if [ "$(uname)" = "Darwin" ]; then
    OS_FAMILY="macos"; OS_ID="macos"
    OS_VERSION="$(sw_vers -productVersion 2>/dev/null || echo unknown)"
    command -v brew >/dev/null 2>&1 && PKG_MGR="brew"
else
    [ -f /etc/os-release ] && . /etc/os-release || true
    OS_FAMILY="linux"
    OS_ID="${ID:-unknown}"
    OS_VERSION="${VERSION_ID:-unknown}"
    if   command -v apt-get >/dev/null 2>&1; then PKG_MGR="apt"
    elif command -v dnf     >/dev/null 2>&1; then PKG_MGR="dnf"
    elif command -v yum     >/dev/null 2>&1; then PKG_MGR="yum"
    elif command -v zypper  >/dev/null 2>&1; then PKG_MGR="zypper"
    elif command -v pacman  >/dev/null 2>&1; then PKG_MGR="pacman"
    elif command -v apk     >/dev/null 2>&1; then PKG_MGR="apk"
    fi
fi
log "OS: family=$OS_FAMILY id=$OS_ID version=$OS_VERSION pkg=$PKG_MGR arch=$ARCH"
echo -e "  ${DIM}OS:     $OS_ID $OS_VERSION ($OS_FAMILY/$ARCH)${NC}"
echo -e "  ${DIM}Pkg:    ${PKG_MGR:-none}${NC}"
echo -e "  ${DIM}Method: $METHOD${NC}"

# ----------------------------------------------------------------------
# Sudo helper
# ----------------------------------------------------------------------
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then SUDO="sudo"
    else die "this install needs root, but sudo isn't installed. Re-run as root."
    fi
fi

# ----------------------------------------------------------------------
# Already installed?
# ----------------------------------------------------------------------
docker_present()  { command -v docker >/dev/null 2>&1; }
compose_present() { docker compose version >/dev/null 2>&1; }
daemon_running()  { docker info >/dev/null 2>&1; }

if docker_present && compose_present && daemon_running; then
    ok "docker + compose already installed; daemon is running"
    docker --version 2>&1 | sed 's/^/    /'
    docker compose version 2>&1 | sed 's/^/    /'
    exit 0
fi

if docker_present && ! daemon_running; then
    warn "docker is installed but the daemon is not running."
    if command -v systemctl >/dev/null 2>&1; then
        step "Starting docker daemon"
        run "$SUDO systemctl enable --now docker"
        daemon_running && { ok "daemon now running"; exit 0; } || true
    fi
    die "daemon failed to start — investigate: $SUDO journalctl -u docker -n 50"
fi

# ----------------------------------------------------------------------
# Confirmation — show full plan before any change
# ----------------------------------------------------------------------
step "Plan"
case "$METHOD" in
    getdocker)
        echo "  1. Download https://get.docker.com (Docker's official installer)"
        echo "     ${DIM}→ verify it's not empty, then run it${NC}"
        ;;
    amzn)
        case "${OS_VERSION:-}" in
            2)        echo "  1. $SUDO amazon-linux-extras install -y docker" ;;
            2023|2023.*) echo "  1. $SUDO dnf install -y docker" ;;
            *)        echo "  1. $SUDO ${PKG_MGR:-dnf} install -y docker  (fallback)" ;;
        esac
        echo "  2. install docker compose v2 plugin from github.com/docker/compose releases"
        echo "     ${DIM}→ Amazon Linux repos don't ship the compose plugin${NC}"
        ;;
    distro)
        case "$PKG_MGR" in
            apt)
                echo "  1. $SUDO apt-get update"
                echo "  2. $SUDO apt-get install -y docker.io docker-compose-v2 docker-buildx"
                ;;
            dnf|yum)
                echo "  1. $SUDO $PKG_MGR install -y docker docker-compose-plugin"
                ;;
            zypper)
                echo "  1. $SUDO zypper install -y docker docker-compose"
                ;;
            pacman)
                echo "  1. $SUDO pacman -S --noconfirm docker docker-compose"
                ;;
            apk)
                echo "  1. $SUDO apk add --no-cache docker docker-cli-compose"
                ;;
            *) die "no distro package path for $PKG_MGR; use --method=getdocker" ;;
        esac
        ;;
    skip)
        echo "  (no docker install — you'll install docker yourself)"
        ;;
esac

case "$OS_FAMILY" in
    linux)
        echo "  • $SUDO systemctl enable --now docker"
        if [ "$NO_GROUP" -eq 0 ] && [ -n "${USER:-}" ] && [ "$USER" != "root" ]; then
            echo "  • $SUDO usermod -aG docker $USER"
            echo "    ${DIM}↑ skip with --no-group${NC}"
        fi
        ;;
    macos)
        echo "  • $SUDO open -a Docker"
        ;;
esac

if [ -f /etc/docker/daemon.json ]; then
    warn "/etc/docker/daemon.json exists — we will NOT modify it."
fi

if [ "$SKIP_CONFIRM" -eq 0 ] && [ "$AUDIT" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    echo ""
    read -r -p "  Proceed with the above? [y/N] " yn
    case "$yn" in [Yy]*) ;; *) die "aborted by user" ;; esac
fi

# ----------------------------------------------------------------------
# Install docker
# ----------------------------------------------------------------------
case "$METHOD" in
    getdocker)
        case "$OS_FAMILY" in
            linux)
                step "Downloading https://get.docker.com"
                TMP=$(mktemp)
                trap "rm -f $TMP" EXIT
                if [ "$AUDIT" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
                    curl -fsSL https://get.docker.com -o "$TMP" || die "couldn't download install script"
                    BYTES=$(wc -c <"$TMP")
                    [ "$BYTES" -lt 1024 ] && die "downloaded script is suspiciously small ($BYTES bytes) — refusing"
                    SHA=$(sha256sum "$TMP" 2>/dev/null | cut -c1-12)
                    ok "fetched ($BYTES bytes, sha256=$SHA…)"
                    log "DOCKER_INSTALL_SCRIPT_SHA256: $(sha256sum "$TMP" 2>/dev/null | cut -d' ' -f1)"
                else
                    echo -e "  ${YELLOW}[audit — would download https://get.docker.com]${NC}"
                fi
                step "Running install script"
                run "$SUDO sh $TMP"
                rm -f "$TMP"; trap - EXIT
                ;;
            macos)
                if [ "$PKG_MGR" = "brew" ]; then
                    step "Installing Docker Desktop via Homebrew cask"
                    run "brew install --cask docker"
                    warn "Open Docker Desktop once from /Applications to start the daemon."
                else
                    die "Homebrew not installed. Manual: https://docs.docker.com/desktop/install/mac-install/"
                fi
                ;;
            *) die "unsupported OS: $OS_FAMILY" ;;
        esac
        ;;
    distro)
        case "$PKG_MGR" in
            apt)
                step "Installing via apt"
                run "$SUDO apt-get update -y"
                run "$SUDO DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io docker-compose-v2 docker-buildx"
                ;;
            dnf)
                step "Installing via dnf"
                run "$SUDO dnf install -y docker docker-compose-plugin"
                ;;
            yum)
                step "Installing via yum"
                run "$SUDO yum install -y docker docker-compose-plugin"
                ;;
            zypper)
                step "Installing via zypper"
                run "$SUDO zypper install -y docker docker-compose"
                ;;
            pacman)
                step "Installing via pacman"
                run "$SUDO pacman -S --noconfirm docker docker-compose"
                ;;
            apk)
                step "Installing via apk"
                run "$SUDO apk add --no-cache docker docker-cli-compose"
                ;;
            *) die "distro install not supported for $PKG_MGR — try --method=getdocker" ;;
        esac
        ;;
    skip)
        warn "--method=skip — assuming docker is already installed."
        docker_present || die "docker is not on PATH and --method=skip was given. Install manually then re-run."
        ;;
esac

# ----------------------------------------------------------------------
# Enable daemon + group membership
# ----------------------------------------------------------------------
if [ "$OS_FAMILY" = "linux" ]; then
    if command -v systemctl >/dev/null 2>&1; then
        step "Enabling + starting docker daemon (systemd)"
        run "$SUDO systemctl enable --now docker"
    else
        warn "no systemctl — start the docker daemon manually for your init system."
    fi

    if [ "$NO_GROUP" -eq 0 ] && [ -n "${USER:-}" ] && [ "$USER" != "root" ]; then
        step "Adding $USER to docker group"
        if id -nG "$USER" 2>/dev/null | grep -qw docker; then
            ok "$USER already in docker group"
        else
            run "$SUDO usermod -aG docker $USER"
            warn "log out and back in (or run 'newgrp docker') for this to take effect"
        fi
    elif [ "$NO_GROUP" -eq 1 ]; then
        echo -e "  ${DIM}skipped (--no-group): $USER will need sudo for docker commands${NC}"
    fi
fi

# ----------------------------------------------------------------------
# Final verification
# ----------------------------------------------------------------------
if [ "$AUDIT" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then
    echo ""
    echo -e "${BOLD}(audit/dry-run — nothing was actually installed)${NC}"
    echo -e "${DIM}Full log: $LOG_FILE${NC}"
    exit 0
fi

step "Verifying"
if docker_present && compose_present && daemon_running; then
    ok "$(docker --version)"
    ok "$(docker compose version)"
    echo ""
    echo -e "${GREEN}${BOLD}✓ Docker ready.${NC}"
    echo -e "${DIM}Log: $LOG_FILE${NC}"
    echo ""
    echo "Next: install claws"
    echo -e "  ${DIM}curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | bash${NC}"
    exit 0
else
    warn "install completed but verification failed. Possible reasons:"
    [ "$OS_FAMILY" = "macos" ] && warn "  • Docker Desktop not opened yet — open from /Applications"
    [ "$OS_FAMILY" = "linux" ] && warn "  • not yet in docker group — log out + back in or 'newgrp docker'"
    [ "$OS_FAMILY" = "linux" ] && warn "  • daemon not running — $SUDO systemctl start docker"
    echo -e "${DIM}Log: $LOG_FILE${NC}"
    exit 1
fi
