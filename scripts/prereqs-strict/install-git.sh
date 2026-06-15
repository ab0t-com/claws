#!/bin/bash
# install-git.sh — strict / audit-friendly variant
# See install-docker.sh in this dir for the full list of guards.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/install-git.sh | bash
#   ./scripts/prereqs-strict/install-git.sh [--audit] [--yes]
#
# Env: CLAWS_NO_INSTALL=1, CLAWS_PREREQS_LOG=<path>.

if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    echo "ERROR: install-git.sh requires bash." >&2
    exit 1
fi
set -euo pipefail

AUDIT=0; DRY_RUN=0; SKIP_CONFIRM=0
for arg in "$@"; do
    case "$arg" in
        --audit)    AUDIT=1; DRY_RUN=1 ;;
        --dry-run)  DRY_RUN=1 ;;
        --yes|-y)   SKIP_CONFIRM=1 ;;
        -h|--help)  sed -n '2,9p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || true; exit 0 ;;
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
    log "EXEC: $*"
    echo -e "  ${DIM}\$ $*${NC}"
    if [ "$AUDIT" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then echo -e "  ${YELLOW}[audit — would run]${NC}"; return 0; fi
    eval "$@" 2>&1 | tee -a "$LOG_FILE"
    return "${PIPESTATUS[0]}"
}

echo -e "${BOLD}claws prereq installer — git (strict)${NC}"
echo -e "  ${DIM}Log: $LOG_FILE${NC}"

# Safety preamble
if [ "${CLAWS_NO_INSTALL:-0}" = "1" ]; then warn "CLAWS_NO_INSTALL=1 — refusing to install."; exit 0; fi
if [ -f /.dockerenv ] || (grep -q docker /proc/1/cgroup 2>/dev/null); then
    warn "container environment detected. Continuing anyway (git in a container is fine)."
fi
if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ]; then SKIP_CONFIRM=1; fi
if [ ! -t 0 ] && [ "$SKIP_CONFIRM" -eq 0 ]; then
    echo -e "  ${DIM}non-interactive stdin — auto-confirming${NC}"; SKIP_CONFIRM=1
fi
[ "$(id -u)" -eq 0 ] && echo -e "  ${DIM}running as root — sudo not needed${NC}"

if command -v git >/dev/null 2>&1; then
    ok "git already installed: $(git --version)"
    exit 0
fi

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    command -v sudo >/dev/null 2>&1 && SUDO="sudo" || die "needs root, but sudo missing. Re-run as root."
fi

# OS detection
if [ "$(uname)" = "Darwin" ]; then
    step "macOS — triggering Xcode Command Line Tools installer"
    run "xcode-select --install" || true
    warn "Complete the Xcode CLT install dialog, then re-run this script."
    warn "Alternative: brew install git"
    exit 0
fi

[ -f /etc/os-release ] && . /etc/os-release || true
PKG_MGR=""
if   command -v apt-get >/dev/null 2>&1; then PKG_MGR="apt"
elif command -v dnf     >/dev/null 2>&1; then PKG_MGR="dnf"
elif command -v yum     >/dev/null 2>&1; then PKG_MGR="yum"
elif command -v zypper  >/dev/null 2>&1; then PKG_MGR="zypper"
elif command -v pacman  >/dev/null 2>&1; then PKG_MGR="pacman"
elif command -v apk     >/dev/null 2>&1; then PKG_MGR="apk"
fi
[ -z "$PKG_MGR" ] && die "no package manager detected. Install git manually: https://git-scm.com/"

echo -e "  ${DIM}OS:  ${ID:-unknown} ${VERSION_ID:-}${NC}"
echo -e "  ${DIM}Pkg: $PKG_MGR${NC}"

step "Plan"
case "$PKG_MGR" in
    apt)    echo "  $SUDO apt-get update && $SUDO apt-get install -y git" ;;
    dnf)    echo "  $SUDO dnf install -y git" ;;
    yum)    echo "  $SUDO yum install -y git" ;;
    zypper) echo "  $SUDO zypper install -y git" ;;
    pacman) echo "  $SUDO pacman -S --noconfirm git" ;;
    apk)    echo "  $SUDO apk add --no-cache git" ;;
esac

if [ "$SKIP_CONFIRM" -eq 0 ] && [ "$AUDIT" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    read -r -p "  Proceed? [y/N] " yn
    case "$yn" in [Yy]*) ;; *) die "aborted by user" ;; esac
fi

step "Installing git"
case "$PKG_MGR" in
    apt)
        run "$SUDO apt-get update -y"
        run "$SUDO DEBIAN_FRONTEND=noninteractive apt-get install -y git"
        ;;
    dnf)    run "$SUDO dnf install -y git" ;;
    yum)    run "$SUDO yum install -y git" ;;
    zypper) run "$SUDO zypper install -y git" ;;
    pacman) run "$SUDO pacman -S --noconfirm git" ;;
    apk)    run "$SUDO apk add --no-cache git" ;;
esac

if [ "$AUDIT" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then
    echo -e "\n${BOLD}(audit/dry-run — nothing was installed)${NC}"
    echo -e "${DIM}Log: $LOG_FILE${NC}"
    exit 0
fi

if command -v git >/dev/null 2>&1; then
    ok "$(git --version)"
    echo -e "${GREEN}${BOLD}✓ git ready.${NC}"
    echo -e "${DIM}Log: $LOG_FILE${NC}"
else
    die "install completed but 'git' not on PATH — investigate (log: $LOG_FILE)"
fi
