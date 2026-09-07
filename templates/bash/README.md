# {{repo_name}}

{{description}}

---

## Overview
A lightweight, standalone CLI utility written in Bash.

## Installation

### Local Install (Symlink to `~/.local/bin`)
```bash
git clone https://github.com/{{repo_slug}}.git
cd {{repo_name}}
bash install.sh
```

Ensure `~/.local/bin` is in your `$PATH`:
```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Usage
```bash
# Display help
{{repo_name}} --help

# Run operation
{{repo_name}} run
```

## Testing
Run smoke tests:
```bash
bash tests/test_{{package_name}}.sh
```

## Development & Contribution Flow
- **Conventional Commits**: All commit messages must follow [Conventional Commits](https://www.conventionalcommits.org/).
- **Feature Branches**: Branch off `main` via `feat/*` or `fix/*`.
- **Pull Requests**: Changes are merged via squash-and-merge with CI status checks.
