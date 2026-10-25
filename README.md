# repokit: Enterprise Repository DevOps, Archetypes & Governance

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://github.com/Cooper-Wallace/repokit/actions/workflows/test.yml/badge.svg)](https://github.com/Cooper-Wallace/repokit/actions/workflows/test.yml)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://conventionalcommits.org)

An opinionated developer CLI to automate repository DevOps governance, branch protection, and Git hygiene standards across GitHub accounts and portfolio projects.

---

## The Philosophy

For solo developers, engineers, and researchers, pushing directly to `main` without gatekeeping or standardized commit history is a DevOps anti-pattern. While trunk-based commits offer fast prototyping, a disciplined Git history signals engineering maturity, facilitates automated changelog generation, and enables effortless cherry-picking and debugging.

`repokit` bridges this gap: it enforces workplace-grade branch protection and Conventional Commit hygiene while keeping solo workflows lightweight and frictionless.

---

## Features

- **GitHub Merge Governance**:
  - Enforces **Squash and Merge** only (disables merge commits and rebase merges).
  - Configures squash commit titles to default to **`PR_TITLE`** and bodies to **`PR_BODY`** (ensuring clean history on `main`).
  - Enables **automatic branch deletion** after pull requests are merged.
  - Permits PR branches to be updated automatically from `main`.
  - Disables GitHub **auto-merge** to guarantee a human review and manual promotion gate.
- **Branch Protection via GitHub Rulesets**:
  - Restricts direct pushes and force pushes to the default branch (`main`).
  - Requires Pull Requests for merging code with squash-only merge method.
  - Requires CI status checks (`Semantic PR Title Check / Validate Conventional Commit PR Title` and `ci / Test & Coverage`) before merging.
  - Sets required approving reviews to `0` so solo developers are never blocked.
- **Client-Side Git Hooks**:
  - Installs an executable `.git/hooks/commit-msg` hook that validates commits locally before they are made.
  - Installs an executable `.git/hooks/pre-commit` hook that audits repository structure before commits.
- **Automated CI Quality Gates**:
  - Scaffolds `.github/workflows/semantic-pr.yml` to ensure PR titles strictly adhere to Conventional Commits.
- **Standardised Scaffolding**:
  - Scaffolds structured PR templates (`.github/pull_request_template.md`), `.gitignore`, and portfolio-ready documentation.
- **Multi-Mode Audit Engine**:
  - Audits existing repositories with offline local structure checks (`--local`, <30ms) or full remote GitHub API checks (`--remote`).

---

## Ecosystem Integration

`gh-standardise` acts as **Layer 3 (Remote DevOps Governance)** within the 4-layer modular workstation stack:

- **Layer 0 (`riced-kubuntu-os`)**: Kubuntu OS, KDE Plasma 6 desktop configuration, and hardware pipelines.
- **Layer 1 (`workstation-setup`)**: Cross-platform developer toolchains (`uv`, `node`/`fnm`, `rustup`) and shell environments across Linux, macOS, and Windows.
- **Layer 2 (`antigravity-standardise`)**: AI pair programmer harness permissions, global agent directives (`AGENTS.md`), MCP servers, and autonomous workflows.
- **Layer 3 (`gh-standardise`)**: Remote GitHub repository rulesets, squash-and-merge enforcement, and semantic PR CI quality gates.

---

## Installation

### Quick Install (Symlink to `~/.local/bin`)
```bash
git clone https://github.com/Cooper-Wallace/repokit.git ~/Projects/repokit
cd ~/Projects/repokit
bash install.sh
```

Ensure `~/.local/bin` is in your `$PATH`:
```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Prerequisites
- Python 3.10+
- [GitHub CLI (`gh`)](https://cli.github.com/) authenticated via `gh auth login` with `repo` scope.

---

## Commands & Usage

### 1. Audit an Existing Repository
Check a repository against DevOps and structural standards:
```bash
# In the repository directory (runs local + remote audit):
repokit audit

# Run local offline audit only (<30ms, no GitHub API calls):
repokit audit --local

# Run remote GitHub settings audit only:
repokit audit --remote

# Or specify a target path:
repokit audit /path/to/project
```

Example audit output:
```text
Auditing Repository Standards: /home/cooper/Projects/riced-kubuntu-os

GitHub Repository: Cooper-Wallace/riced-kubuntu-os

GitHub Remote Governance Standards:
  ✔ Squash & Merge Enabled
  ✔ Merge Commits Disabled
  ✔ Rebase Commits Disabled
  ✔ Auto-Delete Head Branches on Merge
  ✔ Squash Commit Title set to PR_TITLE
  ✔ Squash Commit Message set to PR_BODY
  ✔ Auto-Merge Disabled (Human Gate)
  ✔ Default Branch Protection Ruleset Active
  ✔ Branch Ruleset Enforces Required Status Checks

Local Repository Standards:
  ✔ Local Conventional Commit Hook Installed (.git/hooks/commit-msg)
  ✔ Local Pre-commit Hook Installed (.git/hooks/pre-commit)
  ✔ Pull Request Template (.github/pull_request_template.md)
  ✔ CI / Semantic PR Workflow (.github/workflows/semantic-pr.yml or ci.yml)

[SUCCESS] All repository standards are strictly satisfied!
```

---

## 2. Apply Standards to an Existing Repository
Apply GitHub API merge settings, branch protection rulesets, local hooks, and templates:
```bash
cd /path/to/my-existing-project
repokit configure
```
* Use `-f` / `--force` to overwrite existing PR templates or workflows.

---

## 3. Solution Archetypes & Scaffolding
List available project structure archetypes:
```bash
repokit templates
```

Supported archetypes:
- **`python`**: Modern `src` layout, `pyproject.toml` (ruff + pytest), typing marker, and test suite.
- **`java`**: Maven layered architecture (`controller`, `service`, `model`), `application.yml`, and JUnit 5 tests.
- **`typescript`**: Vite + strict TypeScript app, modular components, and Vitest.
- **`bash`**: Standalone CLI utility (`bin/` script, `install.sh`, and automated smoke tests).

---

## 4. Create a Brand New Standardised Repository
Create a new repository on GitHub, scaffold the selected solution archetype, set up initial commit, push to `main`, and apply branch protection:

```bash
# Create a Python package
repokit create my-python-pkg --type python --public -d "Modern Python data processing package"

# Create a Java REST API
repokit create auth-service --type java --public -d "Spring Boot authentication service"

# Create a TypeScript frontend
repokit create dashboard-ui --type typescript --public -d "Real-time metrics frontend"

# Create a standalone Bash CLI utility
repokit create sys-monitor --type bash --public -d "System health monitor CLI"
```

---

## 5. Install Git Hooks
To quickly install or repair git hooks on any local repository:
```bash
# Install Conventional Commits commit-msg hook:
repokit hook install

# Install all hooks (commit-msg and pre-commit structural audit):
repokit hook install --all
```

---

## Recommended Solo Developer Workflow

Once your repository is standardised, your daily workflow remains fast and clean:

1. **Create a short-lived branch**:
   ```bash
   git checkout -b feat/laser-filter
   ```
2. **Make atomic commits**:
   ```bash
   git commit -m "feat(laser): add voxel grid downsampling filter"
   ```
3. **Push and open a Pull Request**:
   ```bash
   git push -u origin feat/laser-filter
   gh pr create --fill
   ```
4. **Review Quality Gates & Manually Merge**:
   - Check status of CI quality gates and semantic PR validation:
     ```bash
     gh pr checks
     ```
   - Complete the Quality Gates & Sign-Off Checklist on the PR (automated CI, agent review verification, and human review).
   - Manually squash-merge the Pull Request via the GitHub web UI once all quality gates pass. Automated merging (`gh pr merge`) is intentionally disallowed to preserve human gatekeeping.

---

## Testing

Run the unit test suite:
```bash
python3 -m unittest discover tests -v
```

---

## License

This project is licensed under the [MIT License](LICENSE).
