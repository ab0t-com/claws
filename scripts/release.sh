#!/bin/bash
# clawctl release — build cross-platform binaries
#
# Usage: ./scripts/release.sh [version]
#   version defaults to git tag or "dev"
#
# Output: release/ directory with binaries + checksums
#
# Supported targets:
#   linux/amd64, linux/arm64
#   darwin/amd64, darwin/arm64
#   windows/amd64
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RELEASE_DIR="$ROOT/release"

# Version: argument > git tag > "dev"
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    VERSION=$(git -C "$ROOT" describe --tags --exact-match 2>/dev/null || echo "dev")
fi

BOLD="\033[1m"
GREEN="\033[0;32m"
NC="\033[0m"

echo -e "${BOLD}clawctl release — v${VERSION}${NC}"
echo ""

# Clean previous release
rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

# Build targets
TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

LDFLAGS="-s -w -X main.Version=${VERSION}"

for target in "${TARGETS[@]}"; do
    os="${target%%/*}"
    arch="${target##*/}"

    binary="clawctl"
    if [ "$os" = "windows" ]; then
        binary="clawctl.exe"
    fi

    outdir="$RELEASE_DIR/clawctl-${VERSION}-${os}-${arch}"
    mkdir -p "$outdir"

    echo -n "  Building ${os}/${arch}... "

    GOOS="$os" GOARCH="$arch" go build \
        -ldflags "$LDFLAGS" \
        -o "$outdir/$binary" \
        "$ROOT/."

    # Include compose template and install script
    cp "$ROOT/docker-compose.yml" "$outdir/"
    cp "$ROOT/scripts/install.sh" "$outdir/" 2>/dev/null || true
    cp "$ROOT/scripts/security-audit.sh" "$outdir/" 2>/dev/null || true

    # Create tarball (or zip for windows)
    cd "$RELEASE_DIR"
    dirname="clawctl-${VERSION}-${os}-${arch}"
    if [ "$os" = "windows" ]; then
        zip -qr "${dirname}.zip" "$dirname"
    else
        tar czf "${dirname}.tar.gz" "$dirname"
    fi
    cd "$ROOT"

    echo -e "${GREEN}OK${NC}"
done

# Generate checksums
echo ""
echo -n "  Generating checksums... "
cd "$RELEASE_DIR"
sha256sum *.tar.gz *.zip 2>/dev/null > checksums-sha256.txt
cd "$ROOT"
echo -e "${GREEN}OK${NC}"

echo ""
echo -e "${BOLD}Release artifacts:${NC}"
ls -lh "$RELEASE_DIR"/*.tar.gz "$RELEASE_DIR"/*.zip 2>/dev/null | awk '{print "  " $NF " (" $5 ")"}'
echo ""
echo "  Checksums: $RELEASE_DIR/checksums-sha256.txt"
echo ""
echo -e "${BOLD}Next steps:${NC}"
echo "  1. Tag the release:  git tag v${VERSION} && git push --tags"
echo "  2. Upload to GitHub: gh release create v${VERSION} release/*.tar.gz release/*.zip release/checksums-sha256.txt"
echo ""
