#!/bin/bash
# install-curl.sh — strict / audit-friendly variant
# See install-docker.sh in this dir for the full list of guards.
#
# Usage:
#   ./scripts/prereqs-strict/install-curl.sh [--audit] [--yes]
#
# Note: if curl is missing you usually can't curl this script.
# Provided for completeness and for local checked-out use.

if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    echo "ERROR: install-curl.sh requires bash." >&2
    exit 1
fi
set -euo pipefail

AUDIT=0; DRY_RUN=0; SKIP_CONFIRM=0
for arg in "$@"; do
    case "$arg" in
        --audit)    AUDIT=1; DRY_RUN=1 ;;
        --dry-run)  DRY_RUN=1 ;;
        --yes|-y)   SKIP_CONFIRM=1 ;;
        -h|--help)  sed -n '2,10p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || true; exit 0 ;;
    esac
done

if [ -t 1 ]; then
    BOLD="\033[1m"; GREEN="\033[0;32m"; YELLOW="\033[0;33m"; RED="\033[0;31m"; DIM="\033[0;90m"; NC="\033[0m"
else BOLD=""; GREEN=""; YELLOW=""; RED=""; DIM=""; NC=""; fi

LOG_FILE="${CLAWS_PREREQS_LOG:-/tmp/claws-prereqs-$(date +%Y%m%d-%H%M%S)-$$.log}"
: >"$LOG_FILE" 2>/dev/null || LOG_FILE="/dev/null"
log()  { echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) $*" >>"$LOG_FILE" 2>/dev/null || true; }
step() { echo -e "\n${BOLD}==>${NC} $1"; log "STEP: $1"; }
ok()   { echo -e "  ${GREEN}✓${NC} $1"; log "OK: $1"; }
warn() { echo -e "  ${YELLOW}!${NC} $1" >&2; log "WARN: $1"; }
die()  { echo -e "  ${RED}✗${NC} $1" >&2; log "DIE: $1"; exit 1; }
run()  {
    log "EXEC: $*"; echo -e "  ${DIM}\$ $*${NC}"
    if [ "$AUDIT" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then echo -e "  ${YELLOW}[audit — would run]${NC}"; return 0; fi
    eval "$@" 2>&1 | tee -a "$LOG_FILE"; return "${PIPESTATUS[0]}"
}

echo -e "${BOLD}claws prereq installer — curl (strict)${NC}"
echo -e "  ${DIM}Log: $LOG_FILE${NC}"

if [ "${CLAWS_NO_INSTALL:-0}" = "1" ]; then warn "CLAWS_NO_INSTALL=1 — refusing to install."; exit 0; fi
if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ]; then SKIP_CONFIRM=1; fi

if command -v curl >/dev/null 2>&1; then
    ok "curl already installed: $(curl --version | head -1)"
    exit 0
fi

if [ "$(uname)" = "Darwin" ]; then
    ok "curl ships with macOS by default — reinstall Xcode CLT if truly missing"
    exit 0
fi

SUDO=""
[ "$(id -u)" -ne 0 ] && { command -v sudo >/dev/null 2>&1 && SUDO="sudo" || die "needs root, no sudo"; }

[ -f /etc/os-release ] && . /etc/os-release || true
PKG_MGR=""
if   command -v apt-get >/dev/null 2>&1; then PKG_MGR="apt"
elif command -v dnf     >/dev/null 2>&1; then PKG_MGR="dnf"
elif command -v yum     >/dev/null 2>&1; then PKG_MGR="yum"
elif command -v zypper  >/dev/null 2>&1; then PKG_MGR="zypper"
elif command -v pacman  >/dev/null 2>&1; then PKG_MGR="pacman"
elif command -v apk     >/dev/null 2>&1; then PKG_MGR="apk"
fi
[ -z "$PKG_MGR" ] && die "no package manager detected. Install curl manually: https://curl.se/"

echo -e "  ${DIM}OS:  ${ID:-unknown} ${VERSION_ID:-}${NC}"
echo -e "  ${DIM}Pkg: $PKG_MGR${NC}"

step "Plan"
case "$PKG_MGR" in
    apt)    echo "  $SUDO apt-get update && $SUDO apt-get install -y curl" ;;
    dnf)    echo "  $SUDO dnf install -y curl" ;;
    yum)    echo "  $SUDO yum install -y curl" ;;
    zypper) echo "  $SUDO zypper install -y curl" ;;
    pacman) echo "  $SUDO pacman -S --noconfirm curl" ;;
    apk)    echo "  $SUDO apk add --no-cache curl" ;;
esac

if [ "$SKIP_CONFIRM" -eq 0 ] && [ "$AUDIT" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    read -r -p "  Proceed? [y/N] " yn
    case "$yn" in [Yy]*) ;; *) die "aborted by user" ;; esac
fi

step "Installing curl"
case "$PKG_MGR" in
    apt)    run "$SUDO apt-get update -y"; run "$SUDO DEBIAN_FRONTEND=noninteractive apt-get install -y curl" ;;
    dnf)    run "$SUDO dnf install -y curl" ;;
    yum)    run "$SUDO yum install -y curl" ;;
    zypper) run "$SUDO zypper install -y curl" ;;
    pacman) run "$SUDO pacman -S --noconfirm curl" ;;
    apk)    run "$SUDO apk add --no-cache curl" ;;
esac

if [ "$AUDIT" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then
    echo -e "\n${BOLD}(audit/dry-run — nothing was installed)${NC}"
    echo -e "${DIM}Log: $LOG_FILE${NC}"
    exit 0
fi

if command -v curl >/dev/null 2>&1; then
    ok "$(curl --version | head -1)"
    echo -e "${GREEN}${BOLD}✓ curl ready.${NC}"
    echo -e "${DIM}Log: $LOG_FILE${NC}"
else
    die "install completed but 'curl' not on PATH — investigate (log: $LOG_FILE)"
fi
