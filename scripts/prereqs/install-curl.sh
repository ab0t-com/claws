#!/bin/bash
# install-curl.sh — install curl (auto-detects your OS)
#
# Usage:
#   ./scripts/prereqs/install-curl.sh [--yes] [--dry-run]
#
# Note: if curl is already missing, you can't curl this script — so the
# remote curl|bash form here is mostly for completeness. The expected
# usage path is "checked out from the claws repo" or "downloaded via
# wget" before running.

if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    echo "ERROR: install-curl.sh requires bash." >&2
    exit 1
fi

set -euo pipefail

DRY_RUN=0
SKIP_CONFIRM=0
for arg in "$@"; do
    case "$arg" in
        --dry-run) DRY_RUN=1 ;;
        --yes|-y)  SKIP_CONFIRM=1 ;;
        -h|--help) sed -n '2,11p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || true; exit 0 ;;
    esac
done

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

echo -e "${BOLD}claws prereq installer — curl${NC}"

if command -v curl >/dev/null 2>&1; then
    ok "curl already installed: $(curl --version | head -1)"
    exit 0
fi

if [ "$(uname)" = "Darwin" ]; then
    ok "curl ships with macOS by default. If it's truly missing, reinstall Xcode CLT: xcode-select --install"
    exit 0
fi

[ -f /etc/os-release ] && . /etc/os-release || true
PKG_MGR=""
if   command -v apt-get >/dev/null 2>&1; then PKG_MGR="apt"
elif command -v dnf     >/dev/null 2>&1; then PKG_MGR="dnf"
elif command -v yum     >/dev/null 2>&1; then PKG_MGR="yum"
elif command -v pacman  >/dev/null 2>&1; then PKG_MGR="pacman"
elif command -v apk     >/dev/null 2>&1; then PKG_MGR="apk"
fi

[ -z "$PKG_MGR" ] && die "couldn't detect a package manager. Install curl manually: https://curl.se/"

echo -e "  ${DIM}OS:        ${ID:-unknown}${NC}"
echo -e "  ${DIM}Package:   $PKG_MGR${NC}"

if [ "$SKIP_CONFIRM" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    read -r -p "  Install curl via $PKG_MGR? [Y/n] " yn
    case "$yn" in [Nn]*) die "aborted by user" ;; esac
fi

step "Installing curl"
case "$PKG_MGR" in
    apt)
        run "$SUDO apt-get update -y"
        run "$SUDO DEBIAN_FRONTEND=noninteractive apt-get install -y curl"
        ;;
    dnf)    run "$SUDO dnf install -y curl" ;;
    yum)    run "$SUDO yum install -y curl" ;;
    pacman) run "$SUDO pacman -S --noconfirm curl" ;;
    apk)    run "$SUDO apk add --no-cache curl" ;;
esac

if [ "$DRY_RUN" -eq 0 ]; then
    command -v curl >/dev/null 2>&1 && ok "$(curl --version | head -1)" && echo -e "${GREEN}${BOLD}✓ curl ready.${NC}" || die "install completed but 'curl' not on PATH"
else
    echo -e "\n${BOLD}(dry-run — nothing was installed)${NC}"
fi
