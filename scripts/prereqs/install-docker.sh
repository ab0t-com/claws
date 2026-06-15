#!/bin/bash
# install-docker.sh — install Docker Engine + Docker Compose plugin
#
# Auto-detects your OS and uses the official Docker install path:
#   Linux  → Docker's official get.docker.com convenience script
#   macOS  → Docker Desktop via Homebrew (or manual link if no brew)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-docker.sh | bash
#   ./scripts/prereqs/install-docker.sh
#   ./scripts/prereqs/install-docker.sh --dry-run
#   ./scripts/prereqs/install-docker.sh --yes      # skip the confirmation prompt
#
# Idempotent: if docker is already installed and the daemon is running,
# does nothing and exits 0.
#
# This script is self-contained for curl|bash use. It will prompt for
# sudo password if needed.

# --- POSIX bash check ---------------------------------------------------
if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    echo "ERROR: install-docker.sh requires bash. Re-run with: curl ... | bash" >&2
    exit 1
fi

set -euo pipefail

# --- Args ---------------------------------------------------------------
DRY_RUN=0
SKIP_CONFIRM=0
for arg in "$@"; do
    case "$arg" in
        --dry-run)  DRY_RUN=1 ;;
        --yes|-y)   SKIP_CONFIRM=1 ;;
        -h|--help)
            sed -n '2,16p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || cat <<'HELP'
install-docker.sh — install Docker Engine + Compose plugin (auto-detects OS)
HELP
            exit 0
            ;;
    esac
done

# --- Output helpers -----------------------------------------------------
if [ -t 1 ]; then
    BOLD="\033[1m"; GREEN="\033[0;32m"; YELLOW="\033[0;33m"; RED="\033[0;31m"; DIM="\033[0;90m"; NC="\033[0m"
else
    BOLD=""; GREEN=""; YELLOW=""; RED=""; DIM=""; NC=""
fi

step() { echo -e "\n${BOLD}==>${NC} $1"; }
ok()   { echo -e "  ${GREEN}✓${NC} $1"; }
warn() { echo -e "  ${YELLOW}!${NC} $1" >&2; }
die()  { echo -e "  ${RED}✗${NC} $1" >&2; exit 1; }
run()  { if [ "$DRY_RUN" -eq 1 ]; then echo "  [dry-run] $*"; else eval "$*"; fi; }

# --- OS detection (inlined for self-containment) ------------------------
OS_FAMILY=""    # linux | macos
OS_ID=""        # ubuntu | debian | fedora | rhel | centos | arch | alpine | macos
OS_LIKE=""     # debian | rhel | etc. (for fallback dispatch)
PKG_MGR=""      # apt | dnf | yum | pacman | apk | brew

detect_os() {
    if [ "$(uname)" = "Darwin" ]; then
        OS_FAMILY="macos"
        OS_ID="macos"
        if command -v brew >/dev/null 2>&1; then
            PKG_MGR="brew"
        fi
        return
    fi
    if [ -f /etc/os-release ]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        OS_FAMILY="linux"
        OS_ID="${ID:-unknown}"
        OS_LIKE="${ID_LIKE:-}"
    fi
    if   command -v apt-get >/dev/null 2>&1; then PKG_MGR="apt"
    elif command -v dnf     >/dev/null 2>&1; then PKG_MGR="dnf"
    elif command -v yum     >/dev/null 2>&1; then PKG_MGR="yum"
    elif command -v pacman  >/dev/null 2>&1; then PKG_MGR="pacman"
    elif command -v apk     >/dev/null 2>&1; then PKG_MGR="apk"
    fi
}

# --- sudo wrapper -------------------------------------------------------
SUDO=""
need_sudo() {
    if [ "$(id -u)" -eq 0 ]; then return; fi
    if command -v sudo >/dev/null 2>&1; then
        SUDO="sudo"
    else
        die "this install needs root, but sudo isn't installed. Re-run as root."
    fi
}

# --- Already-installed check --------------------------------------------
docker_ok() {
    command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1
}
daemon_ok() {
    docker info >/dev/null 2>&1
}

# --- Linux install path -------------------------------------------------
install_linux() {
    step "Installing Docker via Docker's official convenience script"
    echo -e "  ${DIM}This uses https://get.docker.com — maintained by Docker Inc.${NC}"
    echo -e "  ${DIM}The script auto-detects your distro and installs from Docker's repos.${NC}"

    need_sudo

    if [ "$SKIP_CONFIRM" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
        read -r -p "  Continue? [Y/n] " yn
        case "$yn" in [Nn]*) die "aborted by user" ;; esac
    fi

    if [ "$DRY_RUN" -eq 1 ]; then
        echo "  [dry-run] curl -fsSL https://get.docker.com | $SUDO sh"
    else
        # Download to tmp first so we can show progress + audit if needed.
        TMP=$(mktemp)
        trap "rm -f $TMP" EXIT
        echo -e "  ${DIM}Fetching install script...${NC}"
        curl -fsSL https://get.docker.com -o "$TMP"
        ok "fetched ($(wc -c <"$TMP") bytes)"
        echo -e "  ${DIM}Running install (may take 1-3 minutes)...${NC}"
        $SUDO sh "$TMP"
        rm -f "$TMP"
        trap - EXIT
        ok "Docker installed"
    fi

    # Start + enable daemon
    if command -v systemctl >/dev/null 2>&1; then
        step "Starting Docker daemon"
        run "$SUDO systemctl enable --now docker"
        ok "Docker daemon running"
    else
        warn "no systemctl — start the docker daemon manually"
    fi

    # Add user to docker group (so they don't need sudo for every command)
    if [ -n "${USER:-}" ] && [ "$USER" != "root" ]; then
        step "Adding $USER to the docker group"
        if id -nG "$USER" 2>/dev/null | grep -qw docker; then
            ok "$USER already in docker group"
        else
            run "$SUDO usermod -aG docker $USER"
            ok "$USER added to docker group"
            warn "log out and back in (or run 'newgrp docker') for this to take effect"
        fi
    fi
}

# --- macOS install path -------------------------------------------------
install_macos() {
    step "Installing Docker Desktop for macOS"
    if [ "$PKG_MGR" = "brew" ]; then
        echo -e "  ${DIM}Using Homebrew cask...${NC}"
        run "brew install --cask docker"
        ok "Docker Desktop installed"
        echo ""
        warn "Open Docker Desktop once from /Applications to start the daemon."
        warn "Then re-run this script to verify."
    else
        cat <<EOF
  Homebrew is not installed. To install Docker Desktop manually:

    1. Download from: https://docs.docker.com/desktop/install/mac-install/
    2. Open the downloaded .dmg
    3. Drag Docker to Applications
    4. Launch Docker from /Applications (it'll prompt for setup)
    5. Re-run this script to verify

  Or install Homebrew first and re-run this script:

    /bin/bash -c "\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
EOF
        exit 1
    fi
}

# --- main ---------------------------------------------------------------
echo -e "${BOLD}claws prereq installer — docker${NC}"

detect_os
echo -e "  ${DIM}OS:        ${OS_ID} (${OS_FAMILY})${NC}"
echo -e "  ${DIM}Package:   ${PKG_MGR:-(none detected)}${NC}"

if docker_ok && daemon_ok; then
    ok "Docker + Compose already installed and daemon is running"
    docker --version
    docker compose version
    exit 0
fi

if docker_ok && ! daemon_ok; then
    warn "Docker installed but daemon not running"
    if command -v systemctl >/dev/null 2>&1; then
        need_sudo
        step "Starting Docker daemon"
        run "$SUDO systemctl enable --now docker"
        if daemon_ok; then
            ok "Daemon now running"
            exit 0
        fi
    fi
    die "daemon failed to start — investigate: sudo journalctl -u docker -n 50"
fi

case "$OS_FAMILY" in
    linux) install_linux ;;
    macos) install_macos ;;
    *)
        die "unsupported OS: $(uname). See https://docs.docker.com/engine/install/"
        ;;
esac

# --- Final verification --------------------------------------------------
if [ "$DRY_RUN" -eq 1 ]; then
    echo -e "\n${BOLD}(dry-run — nothing was actually installed)${NC}"
    exit 0
fi

step "Verifying installation"
if docker_ok && daemon_ok; then
    ok "$(docker --version)"
    ok "$(docker compose version)"
    echo ""
    echo -e "${GREEN}${BOLD}✓ Docker ready.${NC}"
    echo ""
    echo "Next: install claws"
    echo -e "  ${DIM}curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | bash${NC}"
else
    warn "Installation completed but verification failed. You may need to:"
    warn "  1. Log out and back in (for docker group membership)"
    warn "  2. Open Docker Desktop manually (macOS)"
    warn "  3. Run: sudo systemctl start docker (Linux)"
    exit 1
fi
