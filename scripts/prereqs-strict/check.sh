#!/bin/bash
# check.sh — strict variant of prereq check
#
# Same output shape as ../prereqs/check.sh but additionally:
#   • Refuses to make any change (read-only by construction)
#   • Detects + reports container/CI/proxy environments
#   • Optional --json output for piping into other tools
#   • Logs the check timestamp + result to /tmp/claws-prereqs-<ts>.log
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/prereqs-strict/check.sh | bash
#   ./scripts/prereqs-strict/check.sh [--quiet] [--json]

if [ -z "${BASH_VERSION:-}" ]; then
    if [ -f "$0" ] && [ -r "$0" ] && command -v bash >/dev/null 2>&1; then
        exec bash "$0" "$@"
    fi
    echo "ERROR: check.sh requires bash" >&2
    exit 1
fi
set -euo pipefail

QUIET=0; JSON=0
for arg in "$@"; do
    case "$arg" in
        --quiet|-q) QUIET=1 ;;
        --json)     JSON=1 ;;
        -h|--help)  sed -n '2,14p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//' || true; exit 0 ;;
    esac
done

if [ "$JSON" -eq 1 ]; then QUIET=1; fi

if [ "$QUIET" -eq 0 ] && [ -t 1 ]; then
    BOLD="\033[1m"; GREEN="\033[0;32m"; YELLOW="\033[0;33m"; RED="\033[0;31m"; DIM="\033[0;90m"; NC="\033[0m"
else BOLD=""; GREEN=""; YELLOW=""; RED=""; DIM=""; NC=""; fi

LOG_FILE="${CLAWS_PREREQS_LOG:-/tmp/claws-prereqs-$(date +%Y%m%d-%H%M%S)-$$.log}"
: >"$LOG_FILE" 2>/dev/null || LOG_FILE="/dev/null"
log() { echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) $*" >>"$LOG_FILE" 2>/dev/null || true; }

say() { [ "$QUIET" -eq 0 ] && echo -e "$1" || true; }

REPO="ab0t-com/claws"
URL_BASE="https://raw.githubusercontent.com/${REPO}/main/scripts/prereqs-strict"

# Env signals worth surfacing
IN_CONTAINER=0
if [ -f /.dockerenv ] || (grep -q docker /proc/1/cgroup 2>/dev/null); then IN_CONTAINER=1; fi
IN_CI=0
if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ] || [ -n "${GITLAB_CI:-}" ]; then IN_CI=1; fi
HAS_PROXY=0
[ -n "${HTTP_PROXY:-}${HTTPS_PROXY:-}" ] && HAS_PROXY=1

say "${BOLD}claws prereq check (strict)${NC}"
say "  ${DIM}Log: $LOG_FILE${NC}"
[ "$IN_CONTAINER" -eq 1 ] && say "  ${DIM}Note: container environment detected${NC}"
[ "$IN_CI" -eq 1 ]         && say "  ${DIM}Note: CI environment detected${NC}"
[ "$HAS_PROXY" -eq 1 ]     && say "  ${DIM}Note: proxy detected (HTTP_PROXY/HTTPS_PROXY)${NC}"
say ""

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
PRESENT=()
declare -A VERSIONS

for entry in "${PREREQS[@]}"; do
    IFS='|' read -r tool required desc version_cmd <<<"$entry"

    case "$tool" in
        docker-compose)
            if docker compose version >/dev/null 2>&1; then
                installed=1; version="$(docker compose version --short 2>/dev/null || docker compose version 2>/dev/null | head -1)"
            else installed=0; version=""; fi ;;
        *)
            if command -v "$tool" >/dev/null 2>&1; then
                installed=1; version="$(eval "$version_cmd" 2>/dev/null || echo present)"
            else installed=0; version=""; fi ;;
    esac

    if [ "$installed" -eq 1 ]; then
        PRESENT+=("$tool")
        VERSIONS[$tool]="$version"
        say "  ${GREEN}✓${NC} $(printf '%-18s' "$tool") ${DIM}$version${NC}"
        log "PRESENT: $tool $version"
    else
        if [ "$required" = "yes" ]; then
            MISSING_REQUIRED+=("$tool")
            say "  ${RED}✗${NC} $(printf '%-18s' "$tool") ${RED}MISSING${NC} ${DIM}— $desc${NC}"
            log "MISSING REQUIRED: $tool ($desc)"
        else
            MISSING_OPTIONAL+=("$tool")
            say "  ${YELLOW}!${NC} $(printf '%-18s' "$tool") ${YELLOW}missing (optional)${NC} ${DIM}— $desc${NC}"
            log "MISSING OPTIONAL: $tool ($desc)"
        fi
    fi
done

# Daemon
DAEMON_OK=1
if command -v docker >/dev/null 2>&1; then
    if ! docker info >/dev/null 2>&1; then
        DAEMON_OK=0
        say ""
        say "  ${YELLOW}!${NC} docker is installed but the daemon isn't reachable"
        say "    ${DIM}Linux:  sudo systemctl start docker${NC}"
        say "    ${DIM}macOS:  open -a Docker${NC}"
        MISSING_REQUIRED+=("docker-daemon")
        log "DAEMON: not reachable"
    fi
fi

say ""

# JSON output
if [ "$JSON" -eq 1 ]; then
    printf '{'
    printf '"ts":"%s",' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf '"ok":%s,' "$([ ${#MISSING_REQUIRED[@]} -eq 0 ] && echo true || echo false)"
    printf '"in_container":%s,' "$([ $IN_CONTAINER -eq 1 ] && echo true || echo false)"
    printf '"in_ci":%s,' "$([ $IN_CI -eq 1 ] && echo true || echo false)"
    printf '"has_proxy":%s,' "$([ $HAS_PROXY -eq 1 ] && echo true || echo false)"
    printf '"daemon_ok":%s,' "$([ $DAEMON_OK -eq 1 ] && echo true || echo false)"
    printf '"present":['
    sep=""
    for t in "${PRESENT[@]}"; do printf '%s"%s"' "$sep" "$t"; sep=","; done
    printf '],'
    printf '"missing_required":['
    sep=""
    for t in "${MISSING_REQUIRED[@]}"; do printf '%s"%s"' "$sep" "$t"; sep=","; done
    printf '],'
    printf '"missing_optional":['
    sep=""
    for t in "${MISSING_OPTIONAL[@]}"; do printf '%s"%s"' "$sep" "$t"; sep=","; done
    printf ']'
    printf '}\n'
fi

# Exit code + remediation
if [ "${#MISSING_REQUIRED[@]}" -eq 0 ]; then
    say "${GREEN}${BOLD}✓ All required prereqs are installed.${NC}"
    [ "${#MISSING_OPTIONAL[@]}" -gt 0 ] && say "  ${DIM}(optional missing: ${MISSING_OPTIONAL[*]})${NC}"
    log "RESULT: ok"
    exit 0
fi

say "${RED}${BOLD}✗ Missing required: ${MISSING_REQUIRED[*]}${NC}"
say ""
say "Audit what an install would do (no changes):"
say "  ${BOLD}curl -fsSL ${URL_BASE}/install-all.sh | bash -s -- --audit${NC}"
say ""
say "Or install everything:"
say "  ${BOLD}curl -fsSL ${URL_BASE}/install-all.sh | bash${NC}"
log "RESULT: missing required (${MISSING_REQUIRED[*]})"
exit 1
