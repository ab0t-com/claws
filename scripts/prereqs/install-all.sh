#!/bin/bash
# install-all.sh — universal claws prereq installer
#
# Detects your OS and installs everything claws needs:
#   - curl  (fetches install + update artifacts)
#   - git   (recommended; needed for source builds)
#   - docker engine + compose plugin (mandatory)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-all.sh | bash
#   ./scripts/prereqs/install-all.sh           # interactive
#   ./scripts/prereqs/install-all.sh --yes     # no prompts
#   ./scripts/prereqs/install-all.sh --dry-run # show what would happen
#
# Idempotent: skips anything already installed. Safe to re-run.

if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    cat >&2 <<'EOF'
ERROR: install-all.sh requires bash. Re-run with:
    curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-all.sh | bash
EOF
    exit 1
fi

set -euo pipefail

# --- Args ---------------------------------------------------------------
DRY_RUN=0
SKIP_CONFIRM=0
SKIP_OPTIONAL=0
for arg in "$@"; do
    case "$arg" in
        --dry-run)       DRY_RUN=1 ;;
        --yes|-y)        SKIP_CONFIRM=1 ;;
        --required-only) SKIP_OPTIONAL=1 ;;
        -h|--help)
            sed -n '2,17p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || true
            exit 0
            ;;
    esac
done

# Non-TTY contexts (curl|bash, cloud-init, CI, ssh command) have no
# human to answer prompts. Auto-confirm with a clear notice so the
# install proceeds on a fresh EC2 / root box without hanging.
if [ ! -t 0 ] && [ "$SKIP_CONFIRM" -eq 0 ]; then
    echo "  [info] non-interactive stdin (curl|bash / cloud-init / CI) — auto-confirming"
    SKIP_CONFIRM=1
fi
if [ "$(id -u)" -eq 0 ]; then
    echo "  [info] running as root — sudo not needed"
fi

# --- Output -------------------------------------------------------------
if [ -t 1 ]; then
    BOLD="\033[1m"; GREEN="\033[0;32m"; YELLOW="\033[0;33m"; RED="\033[0;31m"; DIM="\033[0;90m"; NC="\033[0m"
else
    BOLD=""; GREEN=""; YELLOW=""; RED=""; DIM=""; NC=""
fi

REPO="ab0t-com/claws"
URL_BASE="https://raw.githubusercontent.com/${REPO}/main/scripts/prereqs"

step()  { echo -e "\n${BOLD}==>${NC} $1"; }
ok()    { echo -e "  ${GREEN}✓${NC} $1"; }
warn()  { echo -e "  ${YELLOW}!${NC} $1" >&2; }
die()   { echo -e "  ${RED}✗${NC} $1" >&2; exit 1; }

# --- OS detection -------------------------------------------------------
detect_os() {
    OS_FAMILY=""; OS_ID=""; PKG_MGR=""
    if [ "$(uname)" = "Darwin" ]; then
        OS_FAMILY="macos"; OS_ID="macos"
        command -v brew >/dev/null 2>&1 && PKG_MGR="brew"
        return
    fi
    [ -f /etc/os-release ] && . /etc/os-release || true
    OS_FAMILY="linux"; OS_ID="${ID:-unknown}"
    if   command -v apt-get >/dev/null 2>&1; then PKG_MGR="apt"
    elif command -v dnf     >/dev/null 2>&1; then PKG_MGR="dnf"
    elif command -v yum     >/dev/null 2>&1; then PKG_MGR="yum"
    elif command -v pacman  >/dev/null 2>&1; then PKG_MGR="pacman"
    elif command -v apk     >/dev/null 2>&1; then PKG_MGR="apk"
    fi
}

detect_os

echo -e "${BOLD}claws prereq installer${NC}"
echo -e "  ${DIM}OS:        ${OS_ID} (${OS_FAMILY})${NC}"
echo -e "  ${DIM}Package:   ${PKG_MGR:-(none detected)}${NC}"
echo ""

# --- What needs installing? ---------------------------------------------
NEED_CURL=0
NEED_GIT=0
NEED_DOCKER=0

command -v curl   >/dev/null 2>&1 || NEED_CURL=1
command -v git    >/dev/null 2>&1 || NEED_GIT=1
if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
    NEED_DOCKER=1
fi

# --- Print the plan -----------------------------------------------------
echo "Plan:"
[ $NEED_CURL -eq 1 ]   && echo -e "  ${YELLOW}install${NC} curl"                  || ok "curl already installed"
if [ $SKIP_OPTIONAL -eq 1 ]; then
    echo -e "  ${DIM}skip${NC} git (--required-only)"
else
    [ $NEED_GIT -eq 1 ] && echo -e "  ${YELLOW}install${NC} git" || ok "git already installed"
fi
[ $NEED_DOCKER -eq 1 ] && echo -e "  ${YELLOW}install${NC} docker + compose plugin" || ok "docker + compose already installed"

if [ $NEED_CURL -eq 0 ] && [ $NEED_GIT -eq 0 ] && [ $NEED_DOCKER -eq 0 ]; then
    echo ""
    echo -e "${GREEN}${BOLD}✓ All prereqs already installed.${NC}"
    echo ""
    if ! docker info >/dev/null 2>&1; then
        warn "Docker is installed but the daemon isn't running."
        echo -e "  ${DIM}Linux:  sudo systemctl start docker${NC}"
        echo -e "  ${DIM}macOS:  open -a Docker${NC}"
        exit 1
    fi
    echo "Next: install claws"
    echo -e "  ${DIM}curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | bash${NC}"
    exit 0
fi

# --- Confirmation -------------------------------------------------------
if [ "$SKIP_CONFIRM" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    echo ""
    echo -e "${DIM}This may prompt for your sudo password.${NC}"
    read -r -p "Continue? [Y/n] " yn
    case "$yn" in [Nn]*) die "aborted by user" ;; esac
fi

# --- Run each sub-installer -----------------------------------------------
# We invoke them via curl|bash so this script Just Works when itself
# fetched via curl|bash. When running locally from a checkout, we use
# the sibling script path first as a fast-path.
SCRIPT_DIR=""
if [ -n "${BASH_SOURCE[0]:-}" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd 2>/dev/null || true)"
fi

run_installer() {
    local name="$1"; shift
    local args=("$@")
    [ "$SKIP_CONFIRM" -eq 1 ] && args+=("--yes")
    [ "$DRY_RUN"      -eq 1 ] && args+=("--dry-run")

    local local_path=""
    if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/install-${name}.sh" ]; then
        local_path="$SCRIPT_DIR/install-${name}.sh"
    fi

    if [ -n "$local_path" ]; then
        echo -e "${DIM}(running local copy: $local_path)${NC}"
        bash "$local_path" "${args[@]}"
    else
        echo -e "${DIM}(fetching ${URL_BASE}/install-${name}.sh)${NC}"
        curl -fsSL "${URL_BASE}/install-${name}.sh" | bash -s -- "${args[@]}"
    fi
}

if [ $NEED_CURL -eq 1 ]; then
    step "[1/3] curl"
    run_installer curl
fi

if [ $NEED_GIT -eq 1 ] && [ $SKIP_OPTIONAL -eq 0 ]; then
    step "[2/3] git"
    run_installer git
fi

if [ $NEED_DOCKER -eq 1 ]; then
    step "[3/3] docker"
    run_installer docker
fi

# --- Summary ------------------------------------------------------------
echo ""
if [ "$DRY_RUN" -eq 1 ]; then
    echo -e "${BOLD}(dry-run — nothing was actually installed)${NC}"
    exit 0
fi

echo -e "${GREEN}${BOLD}✓ Prereqs installed.${NC}"
echo ""
echo "Next: install claws"
echo -e "  ${DIM}curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | bash${NC}"
echo ""
echo "Then verify the install:"
echo -e "  ${DIM}claws doctor${NC}"
