#!/usr/bin/env bash
# install.sh: Symlink or install {{repo_name}} into ~/.local/bin
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_SRC="${SCRIPT_DIR}/bin/{{repo_name}}"
TARGET_DIR="${HOME}/.local/bin"
TARGET_BIN="${TARGET_DIR}/{{repo_name}}"

if [[ ! -f "$BIN_SRC" ]]; then
    echo -e "\033[1;31m[ERROR]\033[0m Could not find binary source at: $BIN_SRC"
    exit 1
fi

mkdir -p "$TARGET_DIR"
chmod +x "$BIN_SRC"
ln -sf "$BIN_SRC" "$TARGET_BIN"

echo -e "\033[1;32m[SUCCESS]\033[0m Symlinked {{repo_name}} -> $TARGET_BIN"

if [[ ":$PATH:" != *":$TARGET_DIR:"* ]]; then
    echo -e "\033[1;33m[NOTE]\033[0m $TARGET_DIR is not currently in your PATH."
    echo 'Add `export PATH="$HOME/.local/bin:$PATH"` to your ~/.bashrc or ~/.zshrc'
else
    echo "{{repo_name}} is ready to use! Run: {{repo_name}} --help"
fi
