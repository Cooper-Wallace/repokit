#!/usr/bin/env bash
# Basic smoke test for {{repo_name}}
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${REPO_ROOT}/bin/{{repo_name}}"

chmod +x "$BIN"

echo "Testing --help output..."
"$BIN" --help | grep -q "Usage:"
echo "✔ Help check passed"

echo "Testing --version output..."
"$BIN" --version | grep -q "version"
echo "✔ Version check passed"

echo "Testing default run command..."
"$BIN" run | grep -q "Completed successfully"
echo "✔ Run check passed"

echo "All tests passed successfully!"
