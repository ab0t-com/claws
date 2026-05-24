#!/bin/bash
# Install git hooks and gitleaks for claws development
set -e

HOOKS_DIR="$(git rev-parse --show-toplevel)/.git/hooks"
SCRIPTS_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(dirname "$SCRIPTS_DIR")"

# Install gitleaks if not present
if ! command -v gitleaks &>/dev/null; then
    echo "Installing gitleaks..."
    ARCH=$(uname -m)
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    if [ "$ARCH" = "x86_64" ]; then ARCH="x64"; fi
    if [ "$ARCH" = "aarch64" ]; then ARCH="arm64"; fi

    URL="https://github.com/gitleaks/gitleaks/releases/download/v8.24.3/gitleaks_8.24.3_${OS}_${ARCH}.tar.gz"
    mkdir -p "$HOME/.local/bin"
    curl -sL "$URL" | tar -xz -C "$HOME/.local/bin" gitleaks
    chmod +x "$HOME/.local/bin/gitleaks"
    echo "Installed gitleaks to ~/.local/bin/gitleaks"
    echo "Add to PATH: export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# Copy hooks
for hook in pre-commit pre-push commit-msg; do
    src="$REPO_DIR/scripts/hooks/$hook"
    if [ -f "$src" ]; then
        cp "$src" "$HOOKS_DIR/$hook"
        chmod +x "$HOOKS_DIR/$hook"
        echo "Installed $hook hook"
    fi
done

echo "Done. Hooks installed."
