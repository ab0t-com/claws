#!/bin/bash
# claws rebuild — fast local-dev rebuild
#
# Usage:
#   ./scripts/rebuild.sh             # build + vet + short tests
#   ./scripts/rebuild.sh --quick     # build only, skip vet & tests
#   ./scripts/rebuild.sh --race      # build + tests with -race
#
# Output: ./claws binary at repo root, ready to run.
#
# This is the inner-loop script used while iterating. For cross-platform
# release artifacts, use ./scripts/release.sh instead.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

BOLD="\033[1m"
GREEN="\033[0;32m"
YELLOW="\033[0;33m"
RED="\033[0;31m"
DIM="\033[0;90m"
NC="\033[0m"

QUICK=0
RACE=0
for arg in "$@"; do
    case "$arg" in
        --quick) QUICK=1 ;;
        --race)  RACE=1 ;;
        -h|--help)
            sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
    esac
done

# --- Discover a usable go ---
if ! command -v go &>/dev/null; then
    # Fallback for openclaw dev hosts that vendor go under ~/.tools
    CANDIDATE="$HOME/.openclaw/team/sarah/workspace/.tools/go/bin"
    if [ -x "$CANDIDATE/go" ]; then
        export PATH="$CANDIDATE:$PATH"
    else
        echo -e "${RED}go not found on PATH${NC}" >&2
        echo "  Install Go 1.22+ from https://go.dev/dl/ or set PATH appropriately." >&2
        exit 1
    fi
fi

echo -e "${BOLD}claws rebuild${NC}  ${DIM}($(go version | awk '{print $3}'))${NC}"

# --- Version stamp from git when available ---
if VERSION=$(git -C "$ROOT" describe --tags --dirty --always 2>/dev/null); then
    : # use it
else
    VERSION="dev-$(date +%Y%m%d-%H%M%S)"
fi
LDFLAGS="-X main.Version=${VERSION}"

# --- Build ---
echo -e "  ${DIM}building ./cmd/claws/ → ./claws${NC}"
go build -ldflags "$LDFLAGS" -o "$ROOT/claws" ./cmd/claws/
SIZE=$(du -h "$ROOT/claws" | awk '{print $1}')
echo -e "  ${GREEN}✓${NC} claws ${VERSION} (${SIZE})"

if [ "$QUICK" -eq 1 ]; then
    echo -e "  ${DIM}--quick: skipping vet & tests${NC}"
    exit 0
fi

# --- Vet ---
echo -e "  ${DIM}go vet ./cmd/claws/...${NC}"
if ! go vet ./cmd/claws/...; then
    echo -e "  ${RED}✗ vet failed${NC}" >&2
    exit 1
fi
echo -e "  ${GREEN}✓${NC} vet clean"

# --- Tests (short by default) ---
# The integration suite has ~150+ tests; even -short leaves dozens of
# fast integration tests running. 60s isn't enough across the whole package.
TEST_FLAGS="-short -timeout=600s"
if [ "$RACE" -eq 1 ]; then
    TEST_FLAGS="-race -short -timeout=900s"
fi
echo -e "  ${DIM}go test ${TEST_FLAGS} ./cmd/claws/...${NC}"
if go test $TEST_FLAGS ./cmd/claws/... 2>&1 | tail -5; then
    echo -e "  ${GREEN}✓${NC} tests pass"
else
    echo -e "  ${YELLOW}!${NC} some tests failed — see output above"
    exit 1
fi

echo ""
echo -e "  ${GREEN}rebuild complete${NC}  →  ./claws"
