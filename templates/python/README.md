# {{repo_name}}

{{description}}

---

## Overview
A modern Python package with `src` layout, `pyproject.toml` packaging, and strict testing.

## Getting Started

### Installation
```bash
git clone https://github.com/{{repo_slug}}.git
cd {{repo_name}}

# Create and activate virtualenv
python3 -m venv .venv
source .venv/bin/activate

# Install editable with dev dependencies
pip install -e ".[dev]"
```

### Running Tests & Linting
```bash
# Run test suite
pytest

# Run linter
ruff check .
```

## Development & Contribution Flow
- **Conventional Commits**: All commit messages must follow [Conventional Commits](https://www.conventionalcommits.org/).
- **Feature Branches**: Branch off `main` via `feat/*` or `fix/*`.
- **Pull Requests**: Changes are merged via squash-and-merge with CI status checks.
