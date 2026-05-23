#!/bin/bash
# clawctl installer
#
# Remote (release):
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/scripts/install.sh | sh -s -- --version=v1.0.0
#
# Local (running from an extracted release tarball, next to ./clawctl):
#   ./install.sh
#
# Local-dev (running from a fresh git clone with Go installed):
#   CLAWCTL_LOCAL_DEV=1 ./scripts/install.sh
#   # or just run:  ./scripts/rebuild.sh
#
# Install location:
#   --dir=/custom/path                # explicit
#   else /usr/local/bin if writable, else $HOME/.local/bin
#
# Compliance / safety:
#   - HTTPS only, fail on any HTTP error (-f)
#   - SHA256 verification of downloaded tarball against SHA256SUMS
#   - No 'curl | sudo sh' coercion: sudo only escalates when actually needed
#   - --dry-run prints the exact commands that would run
#   - Refuses to overwrite an existing newer binary unless --force
set -euo pipefail

# =====================================================================
# Repo coordinates
# =====================================================================
REPO_OWNER="ab0t-com"
REPO_NAME="claws"
REPO="${REPO_OWNER}/${REPO_NAME}"
BINARY="clawctl"

# =====================================================================
# Colors
# =====================================================================
BOLD="\033[1m"
GREEN="\033[0;32m"
YELLOW="\033[0;33m"
RED="\033[0;31m"
DIM="\033[0;90m"
NC="\033[0m"

info()  { echo -e "  ${DIM}$1${NC}"; }
ok()    { echo -e "  ${GREEN}✓${NC} $1"; }
warn()  { echo -e "  ${YELLOW}!${NC} $1" >&2; }
err()   { echo -e "  ${RED}✗${NC} $1" >&2; }
die()   { err "$1"; exit 1; }

# =====================================================================
# Flags
# =====================================================================
VERSION=""
INSTALL_DIR=""
DRY_RUN=0
FORCE=0
LOCAL_DEV="${CLAWCTL_LOCAL_DEV:-}"

for arg in "$@"; do
    case "$arg" in
        --version=*)   VERSION="${arg#--version=}" ;;
        --dir=*)       INSTALL_DIR="${arg#--dir=}" ;;
        --dry-run)     DRY_RUN=1 ;;
        --force)       FORCE=1 ;;
        --local-dev)   LOCAL_DEV=1 ;;
        -h|--help)
            sed -n '2,28p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) die "unknown flag: $arg (use --help)" ;;
    esac
done

run() {
    if [ "$DRY_RUN" -eq 1 ]; then
        echo "  [dry-run] $*"
    else
        eval "$*"
    fi
}

echo -e "${BOLD}clawctl installer${NC}"
echo ""

# =====================================================================
# Detect OS / arch (used by both remote and local-dev modes)
# =====================================================================
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *) die "unsupported architecture: $ARCH" ;;
esac
case "$OS" in
    linux|darwin) ;;
    *) die "unsupported OS: $OS — download manually: https://github.com/${REPO}/releases" ;;
esac

info "OS:   $OS"
info "Arch: $ARCH"

# =====================================================================
# Decide install dir (deferred until we know we need it)
# =====================================================================
choose_install_dir() {
    if [ -n "$INSTALL_DIR" ]; then return; fi
    if [ -w "/usr/local/bin" ] || [ "$(id -u)" -eq 0 ]; then
        INSTALL_DIR="/usr/local/bin"
    elif command -v sudo &>/dev/null && [ -d "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
        info "Will use sudo to install to /usr/local/bin"
    else
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
}

# Copy file into INSTALL_DIR, sudo if needed.
install_binary() {
    local src="$1"
    choose_install_dir

    if [ ! "$FORCE" -eq 1 ] && [ -x "$INSTALL_DIR/$BINARY" ]; then
        existing_ver=$("$INSTALL_DIR/$BINARY" version 2>/dev/null | head -1 || echo "unknown")
        warn "$INSTALL_DIR/$BINARY already exists ($existing_ver)"
        warn "Re-run with --force to overwrite."
        exit 1
    fi

    info "Installing → $INSTALL_DIR/$BINARY"
    if [ -w "$INSTALL_DIR" ]; then
        run "cp \"$src\" \"$INSTALL_DIR/$BINARY\""
        run "chmod +x \"$INSTALL_DIR/$BINARY\""
    else
        run "sudo cp \"$src\" \"$INSTALL_DIR/$BINARY\""
        run "sudo chmod +x \"$INSTALL_DIR/$BINARY\""
    fi
}

# Copy compose template + html assets to data dir.
install_data_assets() {
    local src_dir="$1"
    local DATA_DIR="$HOME/.local/share/clawctl"
    run "mkdir -p \"$DATA_DIR\""
    if [ -f "$src_dir/docker-compose.yml" ]; then
        run "cp \"$src_dir/docker-compose.yml\" \"$DATA_DIR/\""
    fi
    if [ -d "$src_dir/html" ]; then
        run "cp -r \"$src_dir/html\" \"$DATA_DIR/\""
    fi
    if [ -d "$src_dir/docs" ]; then
        run "cp -r \"$src_dir/docs\" \"$DATA_DIR/\""
    fi
}

verify_on_path() {
    if ! command -v "$BINARY" &>/dev/null; then
        echo ""
        warn "$INSTALL_DIR is not on your PATH."
        echo "  Add it:  export PATH=\"$INSTALL_DIR:\$PATH\""
        echo "  Or:      echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.bashrc"
    fi
}

# =====================================================================
# Mode resolution
# =====================================================================
SCRIPT_DIR="$(cd "$(dirname "$0")" 2>/dev/null && pwd)" || SCRIPT_DIR="$(pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." 2>/dev/null && pwd)" || REPO_ROOT=""

# --- Auto-detect local-dev: scripts/install.sh inside a git checkout with go.mod ---
if [ -z "$LOCAL_DEV" ] && [ -f "$REPO_ROOT/go.mod" ] && [ -d "$REPO_ROOT/cmd/$BINARY" ] \
        && [ "$(basename "$SCRIPT_DIR")" = "scripts" ]; then
    LOCAL_DEV=1
    info "Detected local-dev checkout at $REPO_ROOT"
fi

# --- Local-dev mode: build from source ---
if [ "$LOCAL_DEV" = "1" ]; then
    echo ""
    info "Local-dev mode — building from source."
    if ! command -v go &>/dev/null; then
        die "go not found. Install Go 1.22+ from https://go.dev/dl/"
    fi
    info "$(go version)"
    run "(cd \"$REPO_ROOT\" && go build -o \"$REPO_ROOT/$BINARY\" \"./cmd/$BINARY/\")"
    install_binary "$REPO_ROOT/$BINARY"
    install_data_assets "$REPO_ROOT"
    ok "Local-dev install complete."
    verify_on_path
    echo ""
    echo "  Next: clawctl setup"
    exit 0
fi

# --- Local-release mode: running from an extracted tarball next to ./clawctl ---
if [ -f "$SCRIPT_DIR/$BINARY" ]; then
    echo ""
    info "Found $BINARY binary next to installer."
    install_binary "$SCRIPT_DIR/$BINARY"
    install_data_assets "$SCRIPT_DIR"
    ok "Installed from local release dir."
    verify_on_path
    echo ""
    echo "  Next: clawctl setup"
    exit 0
fi

# =====================================================================
# Remote mode: fetch from GitHub releases
# =====================================================================
echo ""

if ! command -v curl &>/dev/null && ! command -v wget &>/dev/null; then
    die "neither curl nor wget found — cannot download release"
fi

fetch_url() {
    # $1: URL  $2: output file
    if command -v curl &>/dev/null; then
        curl -fsSL "$1" -o "$2"
    else
        wget -qO "$2" "$1"
    fi
}

resolve_latest_version() {
    # Use GitHub API for a stable, redirect-free lookup.
    local api="https://api.github.com/repos/${REPO}/releases/latest"
    local tmp; tmp=$(mktemp)
    if ! fetch_url "$api" "$tmp"; then
        rm -f "$tmp"
        return 1
    fi
    grep -m1 '"tag_name"' "$tmp" | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
    rm -f "$tmp"
}

if [ -z "$VERSION" ]; then
    info "Fetching latest version from github.com/${REPO}..."
    VERSION="$(resolve_latest_version || true)"
    [ -n "$VERSION" ] || die "could not determine latest version. Specify --version=vX.Y.Z"
    info "Latest: $VERSION"
fi

TARBALL="clawctl-${VERSION}-${OS}-${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
TARBALL_URL="${BASE_URL}/${TARBALL}"
CHECKSUMS_URL="${BASE_URL}/SHA256SUMS"

info "Package:   $TARBALL"
info "Checksums: SHA256SUMS"

# --- Download ---
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo -n "  Downloading tarball... "
if ! fetch_url "$TARBALL_URL" "$TMP/$TARBALL"; then
    echo -e "${RED}FAIL${NC}"
    die "download failed: $TARBALL_URL"
fi
echo -e "${GREEN}OK${NC}"

echo -n "  Downloading SHA256SUMS... "
if ! fetch_url "$CHECKSUMS_URL" "$TMP/SHA256SUMS"; then
    echo -e "${YELLOW}MISSING${NC}"
    warn "release has no SHA256SUMS — proceeding WITHOUT verification (not recommended)"
else
    echo -e "${GREEN}OK${NC}"
    echo -n "  Verifying SHA256... "
    expected=$(grep " ${TARBALL}\$" "$TMP/SHA256SUMS" | awk '{print $1}')
    if [ -z "$expected" ]; then
        echo -e "${YELLOW}NOT LISTED${NC}"
        warn "tarball not listed in SHA256SUMS — skipping verification"
    else
        actual=$(sha256sum "$TMP/$TARBALL" | awk '{print $1}')
        if [ "$expected" = "$actual" ]; then
            echo -e "${GREEN}OK${NC}"
        else
            echo -e "${RED}MISMATCH${NC}"
            err "expected: $expected"
            err "actual:   $actual"
            die "checksum verification failed"
        fi
    fi
fi

# --- Extract ---
echo -n "  Extracting... "
tar xzf "$TMP/$TARBALL" -C "$TMP"
echo -e "${GREEN}OK${NC}"

EXTRACT_DIR=$(find "$TMP" -maxdepth 1 -type d -name "clawctl-*" | head -1)
[ -n "$EXTRACT_DIR" ] || die "extracted dir not found in archive"

EXTRACTED_BIN="$EXTRACT_DIR/$BINARY"
[ -f "$EXTRACTED_BIN" ] || die "binary not found in archive: $EXTRACTED_BIN"

# --- Install ---
install_binary "$EXTRACTED_BIN"
install_data_assets "$EXTRACT_DIR"

echo ""
ok "clawctl ${VERSION} installed."
verify_on_path

echo ""
echo "  Get started:"
echo "    clawctl setup    — guided setup (recommended)"
echo "    clawctl help     — see all commands"
echo ""
