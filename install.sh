#!/usr/bin/env bash
# install.sh: Symlink or install repokit into ~/.local/bin
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_SRC="${SCRIPT_DIR}/bin/repokit"
TARGET_DIR="${HOME}/.local/bin"
TARGET_BIN="${TARGET_DIR}/repokit"
OLD_BIN="${TARGET_DIR}/gh-standardise"

if [[ ! -f "$BIN_SRC" ]]; then
    echo -e "\033[1;31m[ERROR]\033[0m Could not find binary source at: $BIN_SRC"
    exit 1
fi

mkdir -p "$TARGET_DIR"
chmod +x "$BIN_SRC"

# Remove existing legacy symlink if present
if [[ -L "$OLD_BIN" || -f "$OLD_BIN" ]]; then
    rm -f "$OLD_BIN"
    echo -e "\033[1;34m[INFO]\033[0m Removed legacy symlink: $OLD_BIN"
fi

ln -sf "$BIN_SRC" "$TARGET_BIN"

echo -e "\033[1;32m[SUCCESS]\033[0m Symlinked repokit -> $TARGET_BIN"

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$TARGET_DIR:"* ]]; then
    echo -e "\033[1;33m[NOTE]\033[0m $TARGET_DIR is not currently in your PATH."
    echo "Add the following line to your ~/.bashrc or ~/.zshrc:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
else
    echo "repokit is ready to use! Try running: repokit --help"
fi
