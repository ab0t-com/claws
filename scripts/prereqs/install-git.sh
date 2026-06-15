#!/bin/bash
# install-git.sh — install git (auto-detects your OS)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/install-git.sh | bash
#   ./scripts/prereqs/install-git.sh [--yes] [--dry-run]
#
# Idempotent: if git is already installed, exits 0 without changes.

if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    echo "ERROR: install-git.sh requires bash. Re-run with: curl ... | bash" >&2
    exit 1
fi

set -euo pipefail

DRY_RUN=0
SKIP_CONFIRM=0
for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=1 ;;
        --yes|-y)  SKIP_CONFIRM=1 ;;
        -h|--help) sed -n '2,8p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || true; exit 0 ;;
    esac
done

if [ ! -t 0 ] && [ "$SKIP_CONFIRM" -eq 0 ]; then
    echo "  [info] non-interactive stdin — auto-confirming"
    SKIP_CONFIRM=1
fi
[ "$(id -u)" -eq 0 ] && echo "  [info] running as root — sudo not needed"

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

SUDO=""
[ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1 && SUDO="sudo"

echo -e "${BOLD}claws prereq installer — git${NC}"

if command -v git >/dev/null 2>&1; then
    ok "git already installed: $(git --version)"
    exit 0
fi

# OS detection
if [ "$(uname)" = "Darwin" ]; then
    step "macOS — installing git via Xcode Command Line Tools"
    # Trigger CLT installer (GUI prompt); user must complete the install.
    if [ "$DRY_RUN" -eq 1 ]; then
        echo "  [dry-run] xcode-select --install"
    else
        xcode-select --install 2>/dev/null || true
        warn "If a dialog appeared, complete the Xcode CLT install and re-run this script."
        warn "Alternative: brew install git"
        exit 0
    fi
fi

[ -f /etc/os-release ] && . /etc/os-release || true
PKG_MGR=""
if   command -v apt-get >/dev/null 2>&1; then PKG_MGR="apt"
elif command -v dnf     >/dev/null 2>&1; then PKG_MGR="dnf"
elif command -v yum     >/dev/null 2>&1; then PKG_MGR="yum"
elif command -v pacman  >/dev/null 2>&1; then PKG_MGR="pacman"
elif command -v apk     >/dev/null 2>&1; then PKG_MGR="apk"
fi

[ -z "$PKG_MGR" ] && die "couldn't detect a package manager. Install git manually: https://git-scm.com/downloads"

echo -e "  ${DIM}OS:        ${ID:-unknown}${NC}"
echo -e "  ${DIM}Package:   $PKG_MGR${NC}"

if [ "$SKIP_CONFIRM" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    read -r -p "  Install git via $PKG_MGR? [Y/n] " yn
    case "$yn" in [Nn]*) die "aborted by user" ;; esac
fi

step "Installing git"
case "$PKG_MGR" in
    apt)
        run "$SUDO apt-get update -y"
        run "$SUDO DEBIAN_FRONTEND=noninteractive apt-get install -y git"
        ;;
    dnf)    run "$SUDO dnf install -y git" ;;
    yum)    run "$SUDO yum install -y git" ;;
    pacman) run "$SUDO pacman -S --noconfirm git" ;;
    apk)    run "$SUDO apk add --no-cache git" ;;
esac

if [ "$DRY_RUN" -eq 0 ]; then
    if command -v git >/dev/null 2>&1; then
        ok "$(git --version)"
        echo -e "${GREEN}${BOLD}✓ git ready.${NC}"
    else
        die "install completed but 'git' not on PATH — investigate"
    fi
else
    echo -e "\n${BOLD}(dry-run — nothing was installed)${NC}"
fi
