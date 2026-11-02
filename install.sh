#!/usr/bin/env bash
# install.sh: Build and install repokit into ~/.local/bin
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="${HOME}/.local/bin"
TARGET_BIN="${TARGET_DIR}/repokit"
GH_REPOKIT_LINK="${TARGET_DIR}/gh-repokit"
OLD_BIN="${TARGET_DIR}/gh-standardise"

mkdir -p "$TARGET_DIR"

# Ensure Go is available
if ! command -v go >/dev/null 2>&1; then
    if [[ -x "/tmp/go/bin/go" ]]; then
        export PATH="/tmp/go/bin:$PATH"
    else
        echo -e "\033[1;31m[ERROR]\033[0m Go 1.23+ is required to build repokit."
        exit 1
    fi
fi

echo -e "\033[1;34m[INFO]\033[0m Compiling repokit from source..."
cd "$SCRIPT_DIR"
go build -ldflags="-s -w" -o "${TARGET_BIN}" ./cmd/repokit
chmod +x "${TARGET_BIN}"

# Create gh-repokit symlink
ln -sf repokit "${GH_REPOKIT_LINK}"

# Remove existing legacy symlink if present
if [[ -L "$OLD_BIN" || -f "$OLD_BIN" ]]; then
    rm -f "$OLD_BIN"
    echo -e "\033[1;34m[INFO]\033[0m Removed legacy symlink: $OLD_BIN"
fi

echo -e "\033[1;32m[SUCCESS]\033[0m Installed repokit -> $TARGET_BIN"
echo -e "\033[1;32m[SUCCESS]\033[0m Created symlink gh-repokit -> $GH_REPOKIT_LINK"

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$TARGET_DIR:"* ]]; then
    echo -e "\033[1;33m[NOTE]\033[0m $TARGET_DIR is not currently in your PATH."
    echo "Add the following line to your ~/.bashrc or ~/.zshrc:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
else
    echo "repokit is ready to use! Try running: repokit --help"
fi
