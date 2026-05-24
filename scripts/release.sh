#!/bin/bash
# claws release — build cross-platform binaries
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
# Note: Windows is not supported. claws uses Unix-specific syscalls
# (flock, statfs) and manages Linux Docker containers. WSL2 users on
# Windows should use the linux/amd64 build.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RELEASE_DIR="$ROOT/release"
SRC_PKG="./cmd/claws/"

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

echo -e "${BOLD}claws release — ${VERSION}${NC}"
echo -e "${DIM}go: $(go version | awk '{print $3}')${NC}"
echo ""

# Make sure the release dir exists. We DON'T wipe it — older versions
# need to remain reachable from main so `claws update --version=vX.Y.Z`
# and `install.sh --version=vX.Y.Z` work for users who want to pin or
# downgrade. Prior versions wiped this dir on every cut, which deleted
# v1.6.4 binaries when v1.6.7 shipped — broke updates for anyone on
# v1.6.5 / v1.6.6 until restored. Each new build overwrites only its
# own per-target output dirs / tarballs and rebuilds SHA256SUMS to
# cover everything in the dir.
mkdir -p "$RELEASE_DIR"

# Remove just this version's stale outputs (in case of a rebuild),
# leaving older versions alone. Per-target dirs follow the pattern
# claws-<VERSION>-<os>-<arch>/ and tarballs claws-<VERSION>-<os>-<arch>.tar.gz.
# Uses find -delete (no rm -rf — banned per project rules) so the
# blast radius is bounded to the named pattern.
for dir in "$RELEASE_DIR/claws-${VERSION}-"*/; do
    [ -d "$dir" ] || continue
    find "$dir" -depth -delete
done
find "$RELEASE_DIR" -maxdepth 1 -name "claws-${VERSION}-*.tar.gz" -delete 2>/dev/null || true

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

    binary="claws"
    dirname="claws-${VERSION}-${os}-${arch}"
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
    if [ -d "$ROOT/templates" ]; then
        cp -r "$ROOT/templates" "$outdir/"
    fi

    # ---- Generate per-target MANIFEST.txt ----
    {
        echo "claws release manifest"
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

# Top-level checksums file
echo ""
echo -n "  Generating SHA256SUMS... "
( cd "$RELEASE_DIR" && sha256sum *.tar.gz > SHA256SUMS )
echo -e "${GREEN}OK${NC}"

# VERSION file — install.sh reads this from main to resolve "latest"
echo "$VERSION" > "$RELEASE_DIR/VERSION"
echo "  Wrote $RELEASE_DIR/VERSION → $VERSION"

echo ""
echo -e "${BOLD}Release artifacts:${NC}"
ls -lh "$RELEASE_DIR"/*.tar.gz 2>/dev/null \
    | awk '{print "  " $NF " (" $5 ")"}'
echo ""
echo "  Checksums: $RELEASE_DIR/SHA256SUMS"
echo "  Version:   $RELEASE_DIR/VERSION → $VERSION"
echo ""
echo -e "${BOLD}Next:${NC} git add release/ && git commit && git push && git tag $VERSION && git push origin $VERSION"
echo ""
