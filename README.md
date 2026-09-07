# gh-standardise

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Tests](https://github.com/Cooper-Wallace/gh-standardise/actions/workflows/test.yml/badge.svg)](https://github.com/Cooper-Wallace/gh-standardise/actions/workflows/test.yml)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://conventionalcommits.org)

An opinionated developer CLI to automate repository DevOps governance, branch protection, and Git hygiene standards across GitHub accounts and portfolio projects.

---

## The Philosophy

For solo developers, engineers, and researchers, pushing directly to `main` without gatekeeping or standardized commit history is a DevOps anti-pattern. While trunk-based commits offer fast prototyping, a disciplined Git history signals engineering maturity, facilitates automated changelog generation, and enables effortless cherry-picking and debugging.

`gh-standardise` bridges this gap: it enforces workplace-grade branch protection and Conventional Commit hygiene while keeping solo workflows lightweight and frictionless.

---

## Features

- **GitHub Merge Governance**:
  - Enforces **Squash and Merge** only (disables merge commits and rebase merges).
  - Configures squash commit titles to default to **`PR_TITLE`** and bodies to **`PR_BODY`** (ensuring clean history on `main`).
  - Enables **automatic branch deletion** after pull requests are merged.
  - Permits PR branches to be updated automatically from `main`.
- **Branch Protection via GitHub Rulesets**:
  - Restricts direct pushes and force pushes to the default branch (`main`).
  - Requires Pull Requests for merging code.
  - Sets required approving reviews to `0` so solo developers are never blocked.
- **Client-Side Git Hooks**:
  - Installs an executable `.git/hooks/commit-msg` hook that validates commits locally before they are made.
- **Automated CI Quality Gates**:
  - Scaffolds `.github/workflows/semantic-pr.yml` to ensure PR titles strictly adhere to Conventional Commits.
- **Standardised Scaffolding**:
  - Scaffolds structured PR templates (`.github/pull_request_template.md`), `.gitignore`, and portfolio-ready documentation.
- **10-Point Audit Engine**:
  - Audits existing repositories and produces an instant compliance report.

---

## Installation

### Quick Install (Symlink to `~/.local/bin`)
```bash
git clone https://github.com/Cooper-Wallace/gh-standardise.git ~/Projects/gh-standardise
cd ~/Projects/gh-standardise
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
Check a repository against the 10-point standard:
```bash
# In the repository directory:
gh-standardise audit

# Or specify a target path:
gh-standardise audit /path/to/project
```

Example audit output:
```text
Auditing Repository Standards: /home/cooper/Projects/riced-kubuntu-os

GitHub Repository: Cooper-Wallace/riced-kubuntu-os

  ✔ Squash & Merge Enabled
  ✔ Merge Commits Disabled
  ✔ Rebase Commits Disabled
  ✔ Auto-Delete Head Branches on Merge
  ✔ Squash Commit Title set to PR_TITLE
  ✔ Squash Commit Message set to PR_BODY
  ✔ Default Branch Protection Ruleset Active
  ✔ Local Conventional Commit Hook Installed
  ✔ Pull Request Template (.github/pull_request_template.md)
  ✔ Semantic PR CI Workflow (.github/workflows/semantic-pr.yml)

[SUCCESS] All repository standards are strictly satisfied!
```

---

### 2. Apply Standards to an Existing Repository
Apply GitHub API merge settings, branch protection rulesets, local hooks, and templates:
```bash
cd /path/to/my-existing-project
gh-standardise configure
```
* Use `-f` / `--force` to overwrite existing PR templates or workflows.

---

### 3. Create a Brand New Standardised Repository
Create a new repository on GitHub, clone/initialise it locally, set up the initial commit, push to `main`, and apply branch protection:

```bash
# Create a private repository (default)
gh-standardise create my-robotics-pkg -d "ROS 2 navigation and perception node"

# Create a public portfolio repository
gh-standardise create pointcloud-filter --public -d "High-performance pointcloud downsampler"
```

---

### 4. Install Git Commit Hook Only
To quickly install or repair the Conventional Commits hook on any local git repository:
```bash
gh-standardise hook install
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
4. **Review & Merge**:
   ```bash
   gh pr checks
   gh pr merge --squash --delete-branch
   ```

---

## Testing

Run the unit test suite:
```bash
python3 -m unittest discover tests -v
```

---

## License

This project is licensed under the [MIT License](LICENSE).
