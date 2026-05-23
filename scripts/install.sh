#!/bin/bash
# clawctl installer — download and install the latest release
#
# Usage:
#   curl -sL https://get.clawctl.dev | sh
#   curl -sL https://get.clawctl.dev | sh -s -- --version=v1.0.0
#
# Or run locally after extracting a release:
#   ./install.sh
#
# Installs to /usr/local/bin (with sudo) or ~/.local/bin (without)
set -euo pipefail

BOLD="\033[1m"
GREEN="\033[0;32m"
YELLOW="\033[0;33m"
RED="\033[0;31m"
NC="\033[0m"

VERSION=""
INSTALL_DIR=""

for arg in "$@"; do
    case "$arg" in
        --version=*) VERSION="${arg#--version=}" ;;
        --dir=*) INSTALL_DIR="${arg#--dir=}" ;;
    esac
done

echo -e "${BOLD}clawctl installer${NC}"
echo ""

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)
        echo -e "${RED}Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

case "$OS" in
    linux|darwin) ;;
    *)
        echo -e "${RED}Unsupported OS: $OS${NC}"
        echo "  Download manually from: https://github.com/openclaw/clawctl/releases"
        exit 1
        ;;
esac

echo "  OS:   $OS"
echo "  Arch: $ARCH"

# --- Local install mode (running from extracted release dir) ---
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$SCRIPT_DIR/clawctl" ]; then
    echo ""
    echo "  Found clawctl binary in current directory."

    # Determine install location
    if [ -z "$INSTALL_DIR" ]; then
        if [ -w "/usr/local/bin" ]; then
            INSTALL_DIR="/usr/local/bin"
        else
            INSTALL_DIR="$HOME/.local/bin"
            mkdir -p "$INSTALL_DIR"
        fi
    fi

    cp "$SCRIPT_DIR/clawctl" "$INSTALL_DIR/clawctl"
    chmod +x "$INSTALL_DIR/clawctl"

    # Copy compose template to a discoverable location
    if [ -f "$SCRIPT_DIR/docker-compose.yml" ]; then
        DATA_DIR="$HOME/.local/share/clawctl"
        mkdir -p "$DATA_DIR"
        cp "$SCRIPT_DIR/docker-compose.yml" "$DATA_DIR/"
        echo "  Compose template: $DATA_DIR/docker-compose.yml"
    fi

    echo -e "  ${GREEN}Installed to $INSTALL_DIR/clawctl${NC}"
    echo ""

    # Verify it's on PATH
    if ! command -v clawctl &>/dev/null; then
        echo -e "  ${YELLOW}NOTE:${NC} $INSTALL_DIR is not in your PATH."
        echo "  Add it:  export PATH=\"$INSTALL_DIR:\$PATH\""
        echo "  Or add to ~/.bashrc:  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.bashrc"
    fi

    echo ""
    echo "  Next: clawctl setup"
    exit 0
fi

# --- Remote install mode (download from GitHub) ---
REPO="openclaw/clawctl"

if [ -z "$VERSION" ]; then
    echo -n "  Fetching latest version... "
    if command -v curl &>/dev/null; then
        VERSION=$(curl -sI "https://github.com/$REPO/releases/latest" | grep -i "^location:" | sed 's|.*/tag/||' | tr -d '\r\n')
    elif command -v wget &>/dev/null; then
        VERSION=$(wget -qS --max-redirect=0 "https://github.com/$REPO/releases/latest" 2>&1 | grep "Location:" | sed 's|.*/tag/||' | tr -d '\r\n')
    else
        echo -e "${RED}Neither curl nor wget found${NC}"
        exit 1
    fi

    if [ -z "$VERSION" ]; then
        echo -e "${RED}Could not determine latest version${NC}"
        echo "  Specify manually: $0 --version=v1.0.0"
        exit 1
    fi
    echo "$VERSION"
fi

TARBALL="clawctl-${VERSION}-${OS}-${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/${VERSION}/${TARBALL}"

echo "  Version: $VERSION"
echo "  Package: $TARBALL"
echo ""

# Download
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

echo -n "  Downloading... "
if command -v curl &>/dev/null; then
    curl -sL "$URL" -o "$TMP/$TARBALL"
elif command -v wget &>/dev/null; then
    wget -q "$URL" -O "$TMP/$TARBALL"
fi

if [ ! -s "$TMP/$TARBALL" ]; then
    echo -e "${RED}Download failed${NC}"
    echo "  URL: $URL"
    echo "  Check: https://github.com/$REPO/releases"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Extract
echo -n "  Extracting... "
tar xzf "$TMP/$TARBALL" -C "$TMP"
echo -e "${GREEN}OK${NC}"

# Find the binary in the extracted dir
EXTRACTED=$(find "$TMP" -name "clawctl" -type f | head -1)
if [ -z "$EXTRACTED" ]; then
    echo -e "${RED}Binary not found in archive${NC}"
    exit 1
fi

# Install
if [ -z "$INSTALL_DIR" ]; then
    if [ -w "/usr/local/bin" ] || [ "$(id -u)" -eq 0 ]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
fi

echo -n "  Installing to $INSTALL_DIR... "
if [ -w "$INSTALL_DIR" ]; then
    cp "$EXTRACTED" "$INSTALL_DIR/clawctl"
    chmod +x "$INSTALL_DIR/clawctl"
else
    sudo cp "$EXTRACTED" "$INSTALL_DIR/clawctl"
    sudo chmod +x "$INSTALL_DIR/clawctl"
fi
echo -e "${GREEN}OK${NC}"

# Copy compose template
COMPOSE_SRC="$(dirname "$EXTRACTED")/docker-compose.yml"
if [ -f "$COMPOSE_SRC" ]; then
    DATA_DIR="$HOME/.local/share/clawctl"
    mkdir -p "$DATA_DIR"
    cp "$COMPOSE_SRC" "$DATA_DIR/"
fi

echo ""
echo -e "  ${GREEN}clawctl $VERSION installed successfully.${NC}"
echo ""

# Check PATH
if ! command -v clawctl &>/dev/null; then
    echo -e "  ${YELLOW}NOTE:${NC} $INSTALL_DIR is not in your PATH."
    echo "  Run:  export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
fi

echo "  Get started:"
echo "    clawctl setup    — guided setup (recommended)"
echo "    clawctl help     — see all commands"
echo ""
