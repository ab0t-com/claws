#!/bin/bash
# check.sh — claws prereq check
#
# Reports which prereqs are installed and which are missing.
# Exit 0 if all required prereqs are present, 1 otherwise.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/check.sh | bash
#   ./scripts/prereqs/check.sh
#   ./scripts/prereqs/check.sh --quiet     # exit code only, no output
#
# This script is self-contained (no source dependencies), so it works
# correctly when fetched via curl + piped to bash.

# --- POSIX bash check (same pattern as install.sh) -----------------------
if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    cat >&2 <<'EOF'
ERROR: check.sh requires bash, but was invoked under sh (often dash).

Re-run with bash:

    curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs/check.sh | bash
EOF
    exit 1
fi

set -euo pipefail

# --- Args ---------------------------------------------------------------
QUIET=0
for arg in "$@"; do
    case "$arg" in
        --quiet|-q) QUIET=1 ;;
        -h|--help)
            sed -n '2,16p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || cat <<'HELP'
check.sh — claws prereq check
Usage: check.sh [--quiet]
HELP
            exit 0
            ;;
    esac
done

# --- Colours (skipped under --quiet) ------------------------------------
if [ "$QUIET" -eq 0 ] && [ -t 1 ]; then
    BOLD="\033[1m"
    GREEN="\033[0;32m"
    YELLOW="\033[0;33m"
    RED="\033[0;31m"
    DIM="\033[0;90m"
    NC="\033[0m"
else
    BOLD=""; GREEN=""; YELLOW=""; RED=""; DIM=""; NC=""
fi

say() { [ "$QUIET" -eq 0 ] && echo -e "$1" || true; }

# When --quiet, silence ALL stdout (incl. the per-line printf for each
# prereq). stderr stays open so any real errors are still visible.
[ "$QUIET" -eq 1 ] && exec >/dev/null

REPO="ab0t-com/claws"
URL_BASE="https://raw.githubusercontent.com/${REPO}/main/scripts/prereqs"

# --- The prereqs --------------------------------------------------------
# Format: "tool|required|description|version_command"
# required: yes = mandatory, no = recommended/optional
PREREQS=(
    "bash|yes|shell required for installer scripts|bash --version | head -1"
    "curl|yes|fetches install + update artifacts|curl --version | head -1"
    "docker|yes|runs the agent containers|docker --version"
    "docker-compose|yes|orchestrates per-agent compose stacks (v2 plugin)|docker compose version"
    "git|no|needed for source builds and CONTRIBUTING|git --version"
    "tar|no|unpacks release tarballs (usually preinstalled)|tar --version | head -1"
    "sha256sum|no|verifies download checksums (usually preinstalled)|sha256sum --version 2>/dev/null | head -1 || echo present"
)

MISSING_REQUIRED=()
MISSING_OPTIONAL=()

say "${BOLD}claws prereq check${NC}\n"

# Column widths for output table
W_TOOL=18

for entry in "${PREREQS[@]}"; do
    IFS='|' read -r tool required desc version_cmd <<<"$entry"

    # 'docker-compose' is special — we check `docker compose` (plugin form)
    case "$tool" in
        docker-compose)
            if docker compose version >/dev/null 2>&1; then
                installed=1
                version="$(docker compose version --short 2>/dev/null || docker compose version 2>/dev/null | head -1)"
            else
                installed=0
                version=""
            fi
            ;;
        *)
            if command -v "$tool" >/dev/null 2>&1; then
                installed=1
                version="$(eval "$version_cmd" 2>/dev/null || echo present)"
            else
                installed=0
                version=""
            fi
            ;;
    esac

    if [ "$installed" -eq 1 ]; then
        printf "  ${GREEN}✓${NC} %-${W_TOOL}s ${DIM}%s${NC}\n" "$tool" "$version"
    else
        if [ "$required" = "yes" ]; then
            printf "  ${RED}✗${NC} %-${W_TOOL}s ${RED}MISSING${NC} ${DIM}— %s${NC}\n" "$tool" "$desc"
            MISSING_REQUIRED+=("$tool")
        else
            printf "  ${YELLOW}!${NC} %-${W_TOOL}s ${YELLOW}missing (optional)${NC} ${DIM}— %s${NC}\n" "$tool" "$desc"
            MISSING_OPTIONAL+=("$tool")
        fi
    fi
done

say ""

# --- Daemon check (docker installed but not running?) --------------------
if command -v docker >/dev/null 2>&1; then
    if ! docker info >/dev/null 2>&1; then
        say "  ${YELLOW}!${NC} docker is installed but the daemon is not reachable"
        say "    ${DIM}try: sudo systemctl start docker     (Linux)${NC}"
        say "    ${DIM}     open -a Docker                  (macOS Docker Desktop)${NC}"
        say ""
        MISSING_REQUIRED+=("docker-daemon")
    fi
fi

# --- Summary + remediation ----------------------------------------------
if [ "${#MISSING_REQUIRED[@]}" -eq 0 ]; then
    say "${GREEN}${BOLD}✓ All required prereqs are installed.${NC}"
    if [ "${#MISSING_OPTIONAL[@]}" -gt 0 ]; then
        say "  ${DIM}(optional missing: ${MISSING_OPTIONAL[*]} — install on demand)${NC}"
    fi
    say ""
    say "Next: install claws"
    say "  ${DIM}curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | bash${NC}"
    exit 0
fi

say "${RED}${BOLD}✗ Missing required prereqs: ${MISSING_REQUIRED[*]}${NC}"
say ""
say "Install everything claws needs (auto-detects your OS):"
say "  ${BOLD}curl -fsSL ${URL_BASE}/install-all.sh | bash${NC}"
say ""
say "Or install specific tools:"
for tool in "${MISSING_REQUIRED[@]}"; do
    case "$tool" in
        docker|docker-compose|docker-daemon)
            say "  ${DIM}curl -fsSL ${URL_BASE}/install-docker.sh | bash${NC}"
            ;;
        git|curl|bash)
            say "  ${DIM}curl -fsSL ${URL_BASE}/install-${tool}.sh | bash${NC}"
            ;;
    esac
done | awk '!seen[$0]++'  # dedupe (docker + docker-compose share an installer)

exit 1
