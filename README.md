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
  - Requires CI status checks (`Quality Gate / Title Validation` and `Quality Gate / Verification`) before merging.
  - Sets required approving reviews to `0` so solo developers are never blocked.
- **Client-Side Git Hooks**:
  - Installs an executable `.git/hooks/commit-msg` hook that validates commits locally before they are made.
  - Installs an executable `.git/hooks/pre-commit` hook that audits repository structure before commits.
- **Automated CI Quality Gates**:
  - Scaffolds `.github/workflows/semantic-pr.yml` to ensure PR titles strictly adhere to Conventional Commits.
- **Hybrid Security & CodeQL Quality Gates (Public & Private Repos)**:
  - **Layer 1A (Public Repositories — Native GitHub Security & CodeQL API)**: Automatically enables GitHub Secret Scanning, Push Protection, Dependabot Security Updates, zero-config CodeQL Default Setup (`PATCH /repos/{owner}/{repo}/code-scanning/default-setup`), and ruleset `code_scanning` security gate (`tool: "CodeQL"`, `security_alerts_threshold: "high_or_higher"`, `alerts_threshold: "errors"`).
  - **Layer 1B (All Repositories — Reusable CI SAST & Secret Gates)**: Enforces license-free Gitleaks CLI secret scanning (`v8.21.2`) and language SAST (`ruff --select S` Bandit rules, `go vet`, `shellcheck`, `npm audit`) inside the required `Quality Gate / Verification` check across all archetypes, with automatic graceful fallback on private personal repositories where GitHub gates native GHAS CodeQL behind Enterprise licensing.
- **Native Release Staging & Promotion Engine**:
  - Replaces external release actions with a native, zero-dependency release engine.
  - Automatically calculates SemVer increments from Conventional Commits.
  - Generates and updates `CHANGELOG.md` with candidate PRs (`release-staging` -> `main`).
  - Automatically tags releases and creates GitHub Releases upon human squash-merge to `main`.
- **Standardised Scaffolding**:
  - Scaffolds structured PR templates (`.github/pull_request_template.md`), `.gitignore`, and portfolio-ready documentation.
- **Multi-Mode Audit Engine**:
  - Audits existing repositories with offline local structure checks (`--local`, <30ms) or full remote GitHub API checks (`--remote`).
- **In-Place Binary Self-Update**:
  - Inspect and update `repokit` directly from GitHub Releases (`repokit update`) with `--check`, `--force`, and `--dry-run` modes.
  - Automatically verifies SHA-256 checksums and replaces the running binary atomically.
- **Bounded Concurrent Multi-Repo Governance**:
  - Configure standards across multiple repositories in parallel using a type-safe generic worker pool (`--concurrency <N>`, bounded 1..16, default 4).
  - Thread-safe synchronized console logging eliminates interleaved terminal output.
- **Pre-Flight Network Awareness & Offline Degradation**:
  - Probes GitHub connectivity (1500ms timeout) prior to API operations or honors `--offline` / `REPOKIT_OFFLINE=1`.
  - Automatically degrades to local standards audit and scaffolding when offline, preventing hanging timeouts.
- **Resilient Atomic File Operations**:
  - Employs temporary staging, permission preservation, and atomic file renaming (`WriteFileAtomic`) across all templates, Git hooks, and changelog updates.

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
- Go 1.23+
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

# Run offline audit with pre-flight network degradation:
repokit audit --offline

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
  ✔ GitHub Secret Scanning & Push Protection Enabled
  ✔ CodeQL Code Scanning Default Setup Configured
  ✔ Branch Ruleset Enforces CodeQL Security Gate

Local Repository Standards:
  ✔ Local Conventional Commit Hook Installed (.git/hooks/commit-msg)
  ✔ Local Pre-commit Hook Installed (.git/hooks/pre-commit)
  ✔ Pull Request Template (.github/pull_request_template.md)
  ✔ CI / Semantic PR Workflow (.github/workflows/semantic-pr.yml or ci.yml)

[SUCCESS] All repository standards are strictly satisfied!
```

---

## 2. Apply Standards to an Existing Repository
Apply GitHub API merge settings, branch protection rulesets, local hooks, and templates to a single repository or discovered repositories:

```bash
# In the target repository directory:
repokit configure

# Or specify a target path:
repokit configure /path/to/my-existing-project

# Overwrite existing PR templates or workflows:
repokit configure -f

# Discover and configure across all GitHub repositories:
repokit configure --all

# Configure across discovered repositories with bounded concurrency (e.g. 8 workers):
repokit configure --all --concurrency 8

# Configure across specific repositories:
repokit configure --repos repo1,repo2

# Configure local repositories in offline mode (skipping remote GitHub checks):
repokit configure --local --offline

# Dry-run mode (audit changes without applying):
repokit configure --dry-run

# Direct commit mode (ephemeral clone and direct commit to main without creating PR):
repokit configure --all --direct --force

# Direct commit associated with a governance release ref:
repokit configure --all --direct --force --release-ref v1.2.0

# Automatically open Pull Requests on target repositories for any updates:
repokit configure --all --pr
```

Options:
- `-f`, `--force`: Overwrite existing workflows, PR templates, and configuration files.
- `--all`: Discover and configure all repositories owned by the authenticated GitHub user.
- `--repos <r1,r2>`: Configure a specific comma-separated list of repositories.
- `--dry-run`: Inspect actions and changes that would be applied without modifying remote settings or local files.
- `--direct`, `--push-main`: Commit changes directly to `main` via ephemeral clones and bypass ruleset push restrictions (using administrative tokens), avoiding PR merge fatigue across downstream repositories.
- `--release-ref <tag>`: Associate direct governance commits with a specific release tag (e.g. `v1.2.0`) in the commit message (`chore(repokit): sync governance standards and workflows to <tag>`).
- `--concurrency <N>`: Bounded worker pool concurrency level for multi-repository configuration (1..16, defaults to 4). Concurrency of 1 executes serially.
- `--offline`: Run in offline mode, bypassing remote GitHub API calls (rulesets, permissions, repository settings) and applying local standards only.
- `--pr`: Create a pull request on branch `bot/repokit-sync` instead of pushing directly to `main`.
```

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
- **`go`**: Idiomatic Go standard library layout (`cmd/`, `internal/`), standard testing, and CI.

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

## 6. Semantic Release Staging & Promotion
Manage automated Conventional Commit release staging and promotion:

```bash
# Run release staging or promotion for current repository:
repokit release

# Specify custom target and candidate staging branches:
repokit release --target-branch main --staging-branch release-staging

# Dry-run inspection without pushing tags or opening candidate PRs:
repokit release --dry-run
```

### Native Release Staging & Promotion Lifecycle

`repokit` provides a native, zero-dependency release automation engine that replaces external release-please workflows:

- **Stage 1: Candidate Release Staging**: Push or PR merge to `main` triggers candidate release staging to the candidate staging branch (`release-staging`).
- **Stage 2: Commit Inspection & Changelog Generation**: `repokit release` inspects Conventional Commits since the last `v*` tag, bumps SemVer (`fix` -> patch, `feat` -> minor, `BREAKING CHANGE` or `!` -> major), updates `CHANGELOG.md`, and creates or updates a candidate PR from `release-staging` to `main`.
- **Stage 3: Human Review & Manual Squash-Merge**: Human review & manual squash-merge of `release-staging` PR into `main`. Direct bot auto-merging is disallowed to preserve human gatekeeping.
- **Stage 4: Release Promotion & Asset Packaging**: On merge to `main`, `repokit release` promotes the release: tags `vX.Y.Z`, creates GitHub Release with release notes, and triggers packaging/asset uploads (`package-release`).

---

## 7. In-Place Binary Self-Update
Inspect and update `repokit` directly from official GitHub Releases:

```bash
# Check if a newer version is available without downloading or replacing:
repokit update --check

# Dry-run update flow (displays target asset and checksums):
repokit update --dry-run

# Perform in-place download, SHA-256 verification, and atomic replacement:
repokit update

# Force re-installation of the latest release even if current version is up to date:
repokit update --force
```

Flags:
- `--check`: Query GitHub Releases for the latest release tag and compare against the local version without modifying files.
- `--dry-run`: Simulate the update workflow, printing release version, download URL, and checksum details without writing files.
- `--force`: Download, verify, and reinstall even if the current binary is already on the latest version.

---

## Unified Quality Gate Architecture

`repokit` enforces a **Unified Quality Gate** architecture designed to satisfy GitHub branch rulesets 100% cleanly without requiring administrative bypass permissions or maintaining separate, fragile rulesets for release automation.

Both development feature branches and automated release candidates (`release-staging` -> `main`) report against the exact same required status check contexts enforced by GitHub rulesets:

```
                      GitHub Branch Ruleset Requirements
                                      │
            ┌─────────────────────────┴─────────────────────────┐
            ▼                                                   ▼
┌───────────────────────────────────────┐   ┌───────────────────────────────────────┐
│     Quality Gate / Title Validation   │   │       Quality Gate / Verification     │
├───────────────────────────────────────┤   ├───────────────────────────────────────┤
│ • Enforces Conventional Commits 1.0.0 │   │ • Bifurcated execution based on       │
│ • Validates PR title format & scope   │   │   pull request head branch context:   │
│ • Includes first-class 'release:'     │   │   - Development Branches              │
│   type for release staging PRs        │   │   - Release Candidate Branches        │
└───────────────────────────────────────┘   └───────────────────────────────────────┘
```

### 1. `Quality Gate / Title Validation`
- Enforces strict Conventional Commit specification (1.0.0) on all Pull Request titles using `amannn/action-semantic-pull-request@v5`.
- Supports standard engineering change types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
- Includes first-class support for the `release:` type (e.g., `release: cut v1.2.0`), enabling candidate staging PRs (`release-staging` -> `main`) to pass title verification without workarounds or manual title overrides.

### 2. `Quality Gate / Verification`
The verification gate dynamically bifurcates execution according to the PR's origin branch (`github.head_ref == 'release-staging' || startsWith(github.head_ref, 'release-')`), delivering targeted validation while reporting under a single unified check context:

- **Development Pull Requests** (`feat/*`, `fix/*`, `refactor/*`, etc.):
  - Static analysis and linting (`golangci-lint`, `ruff`, `shellcheck`, `eslint`, `mvn checkstyle`).
  - Full unit, race detection, and integration testing (`go test -race`, `pytest`, `vitest`, `mvn test`).
  - Strict code coverage enforcement against project thresholds (default: 80%).
- **Release Candidate Pull Requests** (`release-staging` -> `main`):
  - Pre-merge artifact build & packaging verification:
    - **Go**: Validates GoReleaser configuration (`goreleaser check`), tests local CLI/binary build (`go build ./cmd/repokit`), and runs full test suite.
    - **Python**: Validates distribution packaging (`pip install build && python -m build`) and runs full test suite.
    - **TypeScript**: Validates production bundle compilation (`npm run build`) and test execution.
    - **Java / Bash**: Validates clean compilation and hermetic test suite pass.
  - Release changelog and metadata audit prior to human promotion.

### Why Bifurcated CI Satisfies Rulesets Without Admin Bypass
Under GitHub branch rulesets, required status checks are enforced uniformly across all PRs targeting protected branches (`main`). If release staging PRs trigger different workflows with distinct check context names (e.g. `release-check` vs `ci / test`), release PRs become unmergeable because the required development check never triggers, forcing repository administrators to manually bypass rulesets.

By unifying the check contract under **`Quality Gate / Title Validation`** and **`Quality Gate / Verification`**, `repokit` ensures:
1. **Zero Ruleset Bypasses**: 100% of pull requests—whether authored by human developers, autonomous AI agents, or the release engine—satisfy identical ruleset check contexts.
2. **Deterministic Pre-Merge Packaging**: Release candidates verify that release binaries and packages compile cleanly *before* the PR is squash-merged into `main`.
3. **Hermetic Trunk Promotion**: Eliminates broken releases on `main` while maintaining strict human-in-the-loop gatekeeping.

---

## Central Governance Propagation & Automated Sync

`repokit` provides bidirectional automated synchronization between central governance (`Cooper-Wallace/repokit`) and all downstream archetype repositories (Go, Python, TypeScript, Bash, Java).

### 1. Multi-Repository Configuration via `repokit configure`
Apply repository standards, GitHub settings, branch rulesets, workflows, and PR templates across managed repositories:

```bash
# Configure across all GitHub repositories associated with authenticated user
repokit configure --all

# Configure across specific repositories
repokit configure --repos repo1,repo2

# Dry-run mode (audit changes without applying)
repokit configure --all --dry-run

# Direct commit and push to main across all repositories without manual PR fatigue
repokit configure --all --direct --force --release-ref v1.2.0

# Automatically open Pull Requests on downstream repositories for any updates
repokit configure --all --pr
```

### 2. GitHub Actions Workflow Permissions
To allow GitHub Actions workflows in downstream repositories to automatically synchronize workflows and open Pull Requests, ensure the following repository settings are enabled:
- **Workflow permissions**: "Read and write permissions" (enables `contents: write` and `pull-requests: write`).
- **Pull Request Approvals**: Enable **"Allow GitHub Actions to create and approve pull requests"** (`can_approve_pull_request_reviews: true`) in repository Actions settings so automated sync PRs can be validated and reviewed smoothly.

### 3. Release-Driven Direct Propagation Model
Central governance follows a **Release-Driven Direct Propagation** architecture that eliminates manual PR fatigue across downstream repositories while preserving stability:

- **Release-Driven Direct Propagation (`.github/workflows/propagate.yml`)**:
  - Central governance changes and reusable workflow enhancements are released and approved via `repokit release`.
  - Published releases trigger `.github/workflows/propagate.yml`, which executes `repokit configure --all --force --direct --release-ref <vX.Y.Z>`.
  - Ephemeral cloning and direct commit to `main` updates all consuming repositories without manual PR fatigue.
  - Reusable workflows pin the stable floating `@v1` tag (`uses: Cooper-Wallace/repokit/.github/workflows/reusable-*@v1`) across all downstream archetype repositories, providing zero-churn stability and immediate non-breaking rollout.
- **Downstream Scheduled Sync (`.github/workflows/sync.yml`)**:
  - Scaffolds into all archetype repositories. Runs weekly on Mondays at 04:00 UTC (or via `workflow_dispatch`), calling the centralized reusable workflow `.github/workflows/reusable-sync.yml` in `Cooper-Wallace/repokit` to check for updates and open sync PRs automatically.

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
5. **Release Staging & Promotion**:
   - Pushes to `main` automatically stage candidate releases to `release-staging` via `repokit release`.
   - When ready for release, review the candidate PR, inspect the changelog, and squash-merge it to `main` to trigger automated tagging (`vX.Y.Z`), GitHub Release publication, and asset uploads.

---

## Documentation

Detailed architectural and operational documentation is available in [`docs/`](docs/):

- [**Quality Gates Reference Architecture**](docs/quality-gates-reference.md): Complete specifications for the 3 progressive quality gates (Automated CI & Rulesets, Agent Review Verification, and Human Review & Manual Promotion).
- [**Additive CI Runners Pattern**](docs/ci-additive-runners-pattern.md): Recommended architectural pattern for integrating project-specific CI jobs (type checking, container builds, simulation smoke tests) alongside centralized governance workflows without breaking ruleset check context matching or `repokit audit`.

---

## Testing

Run the unit test suite:
```bash
go test -v ./...
```

---

## License

This project is licensed under the [MIT License](LICENSE).
