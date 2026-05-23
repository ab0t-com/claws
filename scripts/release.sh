#!/bin/bash
# clawctl release — build cross-platform binaries
#
# Usage: ./scripts/release.sh [version]
#   version defaults to git tag or "dev-<sha>"
#
# Output: release/ directory with per-target tarballs/zips,
#         per-target MANIFEST.txt, and SHA256SUMS.
#
# Supported targets:
#   linux/amd64, linux/arm64
#   darwin/amd64, darwin/arm64
#
# Note: Windows is not supported. clawctl uses Unix-specific syscalls
# (flock, statfs) and manages Linux Docker containers. WSL2 users on
# Windows should use the linux/amd64 build.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RELEASE_DIR="$ROOT/release"
SRC_PKG="./cmd/clawctl/"

# --- Discover a usable go ---
if ! command -v go &>/dev/null; then
    CANDIDATE="$HOME/.openclaw/team/sarah/workspace/.tools/go/bin"
    if [ -x "$CANDIDATE/go" ]; then
        export PATH="$CANDIDATE:$PATH"
    else
        echo "go not found on PATH" >&2; exit 1
    fi
fi

# Version: argument > git tag > git describe
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    VERSION=$(git -C "$ROOT" describe --tags --exact-match 2>/dev/null \
        || git -C "$ROOT" describe --tags --always --dirty 2>/dev/null \
        || echo "dev")
fi

BOLD="\033[1m"
GREEN="\033[0;32m"
DIM="\033[0;90m"
NC="\033[0m"

echo -e "${BOLD}clawctl release — ${VERSION}${NC}"
echo -e "${DIM}go: $(go version | awk '{print $3}')${NC}"
echo ""

# Clean previous release
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

# Build targets (Unix-only — see header note about Windows)
TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

# Git commit for traceability
GIT_COMMIT=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS="-s -w -X main.Version=${VERSION}"

for target in "${TARGETS[@]}"; do
    os="${target%%/*}"
    arch="${target##*/}"

    binary="clawctl"
    dirname="clawctl-${VERSION}-${os}-${arch}"
    outdir="$RELEASE_DIR/$dirname"
    mkdir -p "$outdir"

    echo -n "  Building ${os}/${arch}... "

    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build \
        -trimpath \
        -ldflags "$LDFLAGS" \
        -o "$outdir/$binary" \
        "$ROOT/$SRC_PKG"

    # Bundle assets the binary expects to find next to itself or in cwd
    cp "$ROOT/docker-compose.yml"     "$outdir/"
    cp "$ROOT/scripts/install.sh"     "$outdir/"
    cp "$ROOT/scripts/security-audit.sh" "$outdir/" 2>/dev/null || true
    cp "$ROOT/LICENSE"                "$outdir/"
    cp "$ROOT/README.md"              "$outdir/" 2>/dev/null || true
    # Optional UI/static assets
    if [ -d "$ROOT/html" ]; then
        cp -r "$ROOT/html" "$outdir/"
    fi
    if [ -d "$ROOT/docs" ]; then
        cp -r "$ROOT/docs" "$outdir/"
    fi

    # ---- Generate per-target MANIFEST.txt ----
    {
        echo "clawctl release manifest"
        echo "========================"
        echo "version:    ${VERSION}"
        echo "target:     ${os}/${arch}"
        echo "git-commit: ${GIT_COMMIT}"
        echo "build-date: ${BUILD_DATE}"
        echo "go-version: $(go version | awk '{print $3}')"
        echo ""
        echo "files (sha256):"
        ( cd "$outdir" && find . -type f ! -name 'MANIFEST.txt' -print0 \
            | xargs -0 sha256sum \
            | sed 's|  \./|  |' \
            | sort -k2 )
    } > "$outdir/MANIFEST.txt"

    # ---- Package ----
    ( cd "$RELEASE_DIR" && tar czf "${dirname}.tar.gz" "$dirname" )

    echo -e "${GREEN}OK${NC}"
done

# Top-level checksums file (canonical name for tooling)
echo ""
echo -n "  Generating SHA256SUMS... "
( cd "$RELEASE_DIR" && sha256sum *.tar.gz > SHA256SUMS && cp SHA256SUMS checksums-sha256.txt )
echo -e "${GREEN}OK${NC}"

echo ""
echo -e "${BOLD}Release artifacts:${NC}"
ls -lh "$RELEASE_DIR"/*.tar.gz 2>/dev/null \
    | awk '{print "  " $NF " (" $5 ")"}'
echo ""
echo "  Checksums: $RELEASE_DIR/SHA256SUMS"
echo ""
echo -e "${BOLD}Next steps:${NC}"
echo "  1. Inspect a manifest:   tar tzf release/clawctl-${VERSION}-linux-amd64.tar.gz | head"
echo "  2. Tag the release:      git tag ${VERSION} && git push --tags"
echo "  3. Upload to GitHub:     gh release create ${VERSION} \\"
echo "                             release/*.tar.gz release/SHA256SUMS"
echo ""
