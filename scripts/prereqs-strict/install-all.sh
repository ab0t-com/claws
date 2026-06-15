#!/bin/bash
# install-all.sh — strict / audit-friendly universal installer
#
# Orchestrates the strict variants of install-curl.sh, install-git.sh,
# install-docker.sh. Same safety guards apply: see install-docker.sh in
# this dir for the full list.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/install-all.sh | bash
#   ./scripts/prereqs-strict/install-all.sh [flags]
#
# Flags forwarded to sub-installers:
#   --audit               Print every command; change nothing
#   --dry-run             Same as --audit (compat)
#   --yes, -y             Skip confirmation prompts
#   --no-group            Don't add $USER to docker group
#   --method=<m>          docker install method: getdocker | distro | skip
#   --required-only       Skip optional installers (e.g. git)
#
# Env: CLAWS_NO_INSTALL=1, CLAWS_PREREQS_LOG=<path>.

if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    cat >&2 <<'EOF'
ERROR: install-all.sh requires bash.
    curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/install-all.sh | bash
EOF
    exit 1
fi
set -euo pipefail

# Forward all flags to sub-installers; track our own only for the
# orchestration logic.
SKIP_OPTIONAL=0
AUDIT=0; DRY_RUN=0; SKIP_CONFIRM=0
FORWARD_FLAGS=()
for arg in "$@"; do
    case "$arg" in
        --required-only) SKIP_OPTIONAL=1 ;;
        --audit)         AUDIT=1; DRY_RUN=1; FORWARD_FLAGS+=("$arg") ;;
        --dry-run)       DRY_RUN=1; FORWARD_FLAGS+=("$arg") ;;
        --yes|-y)        SKIP_CONFIRM=1; FORWARD_FLAGS+=("$arg") ;;
        -h|--help)
            sed -n '2,21p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || true
            exit 0
            ;;
        *) FORWARD_FLAGS+=("$arg") ;;
    esac
done

if [ -t 1 ]; then
    BOLD="\033[1m"; GREEN="\033[0;32m"; YELLOW="\033[0;33m"; RED="\033[0;31m"; DIM="\033[0;90m"; NC="\033[0m"
else BOLD=""; GREEN=""; YELLOW=""; RED=""; DIM=""; NC=""; fi

REPO="ab0t-com/claws"
URL_BASE="https://raw.githubusercontent.com/${REPO}/main/scripts/prereqs-strict"
LOG_FILE="${CLAWS_PREREQS_LOG:-/tmp/claws-prereqs-$(date +%Y%m%d-%H%M%S)-$$.log}"
: >"$LOG_FILE" 2>/dev/null || LOG_FILE="/dev/null"

log()  { echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) $*" >>"$LOG_FILE" 2>/dev/null || true; }
step() { echo -e "\n${BOLD}==>${NC} $1"; log "STEP: $1"; }
ok()   { echo -e "  ${GREEN}✓${NC} $1"; log "OK: $1"; }
warn() { echo -e "  ${YELLOW}!${NC} $1" >&2; log "WARN: $1"; }
die()  { echo -e "  ${RED}✗${NC} $1" >&2; log "DIE: $1"; exit 1; }

echo -e "${BOLD}claws prereq installer — universal (strict variant)${NC}"
echo -e "  ${DIM}Log: $LOG_FILE${NC}"

# Safety preamble
if [ "${CLAWS_NO_INSTALL:-0}" = "1" ]; then warn "CLAWS_NO_INSTALL=1 — refusing to install."; exit 0; fi
if [ -f /.dockerenv ] || (grep -q docker /proc/1/cgroup 2>/dev/null); then
    die "detected container environment. Run on the host."
fi
if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ] || [ -n "${GITLAB_CI:-}" ]; then
    echo -e "  ${DIM}CI environment detected; prompts will be auto-confirmed where safe.${NC}"
fi

# OS detection
OS_FAMILY=""; OS_ID=""; PKG_MGR=""
if [ "$(uname)" = "Darwin" ]; then
    OS_FAMILY="macos"; OS_ID="macos"
    command -v brew >/dev/null 2>&1 && PKG_MGR="brew"
else
    [ -f /etc/os-release ] && . /etc/os-release || true
    OS_FAMILY="linux"; OS_ID="${ID:-unknown}"
    for m in apt-get dnf yum zypper pacman apk; do
        command -v "$m" >/dev/null 2>&1 && { PKG_MGR="${m%-get}"; break; }
    done
fi
echo -e "  ${DIM}OS:  $OS_ID ($OS_FAMILY)${NC}"
echo -e "  ${DIM}Pkg: ${PKG_MGR:-none}${NC}"

# What needs installing
NEED_CURL=0; NEED_GIT=0; NEED_DOCKER=0
command -v curl >/dev/null 2>&1 || NEED_CURL=1
command -v git  >/dev/null 2>&1 || NEED_GIT=1
if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
    NEED_DOCKER=1
fi

echo ""
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
    if ! docker info >/dev/null 2>&1; then
        warn "Docker is installed but the daemon isn't running."
        echo -e "  ${DIM}Linux:  sudo systemctl start docker${NC}"
        echo -e "  ${DIM}macOS:  open -a Docker${NC}"
        exit 1
    fi
    echo -e "${GREEN}${BOLD}✓ All prereqs already installed.${NC}"
    echo -e "${DIM}Log: $LOG_FILE${NC}"
    echo ""
    echo "Next: install claws"
    echo -e "  ${DIM}curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | bash${NC}"
    exit 0
fi

# Confirmation
if [ "$SKIP_CONFIRM" -eq 0 ] && [ "$AUDIT" -eq 0 ] && [ "$DRY_RUN" -eq 0 ]; then
    echo ""
    echo -e "${DIM}This may prompt for your sudo password.${NC}"
    read -r -p "Continue? [y/N] " yn
    case "$yn" in [Yy]*) ;; *) die "aborted by user" ;; esac
fi

# Resolve sub-installer paths (prefer local sibling, fall back to curl|bash)
SCRIPT_DIR=""
if [ -n "${BASH_SOURCE[0]:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd 2>/dev/null || true)"
fi

run_installer() {
    local name="$1"
    local local_path=""
    if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/install-${name}.sh" ]; then
        local_path="$SCRIPT_DIR/install-${name}.sh"
    fi
    if [ -n "$local_path" ]; then
        echo -e "  ${DIM}(local: $local_path)${NC}"
        bash "$local_path" "${FORWARD_FLAGS[@]}"
    else
        echo -e "  ${DIM}(fetch: ${URL_BASE}/install-${name}.sh)${NC}"
        curl -fsSL "${URL_BASE}/install-${name}.sh" | bash -s -- "${FORWARD_FLAGS[@]}"
    fi
}

[ $NEED_CURL -eq 1 ]                              && { step "[1/3] curl";    run_installer curl;   }
[ $NEED_GIT  -eq 1 ] && [ $SKIP_OPTIONAL -eq 0 ]  && { step "[2/3] git";     run_installer git;    }
[ $NEED_DOCKER -eq 1 ]                            && { step "[3/3] docker";  run_installer docker; }

echo ""
if [ "$AUDIT" -eq 1 ] || [ "$DRY_RUN" -eq 1 ]; then
    echo -e "${BOLD}(audit/dry-run — nothing was actually installed)${NC}"
else
    echo -e "${GREEN}${BOLD}✓ Prereqs installed.${NC}"
fi
echo -e "${DIM}Log: $LOG_FILE${NC}"
echo ""
echo "Next: install claws + verify"
echo -e "  ${DIM}curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | bash${NC}"
echo -e "  ${DIM}claws doctor${NC}"
