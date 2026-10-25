# Quality Gates Reference Architecture

This document defines the 3-tier Quality Gates architecture enforced by `repokit` across all repositories. Every code change—whether authored by human engineers or autonomous AI agents—must satisfy each progressive gate before trunk promotion into `main`.

```
                  +----------------------------------------------+
                  |               Pull Request Open              |
                  +----------------------------------------------+
                                         |
                                         v
+----------------------------------------------------------------------------------+
| Gate 1: Automated CI Quality Gates (GitHub Actions)                             |
|  - Semantic PR Title Check (Conventional Commits 1.0.0)                         |
|  - Language Test & Coverage Matrix (Python, Bash, Java, TypeScript)              |
|  - Security & Secret Audits (Gitleaks, Dependency Auditing)                     |
|  - GitHub Ruleset Enforcement (strict status checks, up-to-date with main)       |
+----------------------------------------------------------------------------------+
                                         |
                                         | [PASS]
                                         v
+----------------------------------------------------------------------------------+
| Gate 2: Agent Review Gate (Local Antigravity Orchestrator)                      |
|  - Phase 3 Adversarial Diff Audit                                               |
|  - Architectural Contract & AST Sanity Verification                             |
|  - Secret Leakage & Credential Screening                                         |
|  - Conventional Commit & Working Tree Hygiene                                   |
|  - Structured Verification Report in PR Description/Comments                    |
+----------------------------------------------------------------------------------+
                                         |
                                         | [VERIFIED]
                                         v
+----------------------------------------------------------------------------------+
| Gate 3: Human Review & Manual Promotion Gate                                     |
|  - Human Reviewer Inspects Diff & Verification Checklist                         |
|  - Verification of PR Template Sign-Off Items                                    |
|  - Sole Promotion Authorization: Human Clicks "Squash and merge" in GitHub UI   |
|  - Automated Merges Strictly Prohibited (`allow_auto_merge: false`)             |
+----------------------------------------------------------------------------------+
                                         |
                                         v
                  +----------------------------------------------+
                  |      Trunk Promotion (`main` via Squash)      |
                  +----------------------------------------------+
```

---

## Gate 1: Automated CI Quality Gates (GitHub Actions)

Gate 1 is fully automated, executing in cloud-hosted GitHub Actions runners upon pull request creation, synchronization, or edits. It guarantees code correctness, static analysis compliance, test pass rates, code coverage thresholds, and commit message semantics.

### 1. Central Reusable Workflows

`repokit` provides centralized, battle-tested reusable workflows hosted in `.github/workflows/`:

#### `reusable-python.yml`
- **Matrix & Environment**: Configurable Python version runtime (default: `3.11`).
- **Dependency Resolution**: Upgrades `pip` and installs dependencies from `requirements.txt` and/or editable installs via `pyproject.toml` / `setup.py`.
- **Static Analysis & Formatting**:
  - `ruff check .` for comprehensive Python linting.
  - `ruff format --check .` for code style enforcement.
- **Unit Testing & Coverage**:
  - Executes unit test suites via `pytest`.
  - Enforces minimum code coverage thresholds using `pytest-cov` with `--cov-fail-under=${{ inputs.min-coverage }}` (default: `80%`).
  - Emits markdown coverage tables directly into `$GITHUB_STEP_SUMMARY`.
- **Zero-Tolerance Failure**: 100% test pass rate required.

#### `reusable-bash.yml`
- **Static Analysis**: Automatically identifies all executable shell scripts in `bin/*` and `*.sh` and executes `shellcheck`.
- **Automated Test Execution**:
  - Automatically identifies, permissions (`chmod +x`), and executes all test scripts matching `tests/*.sh` and `tests/test_*.sh`.
  - Zero test script failures permitted (`FAIL_COUNT == 0`).
- **Step Summary Reporting**: Renders an execution table in `$GITHUB_STEP_SUMMARY` detailing individual test script outcomes.

#### `reusable-java.yml`
- **Matrix & Environment**: Sets up Eclipse Temurin JDK distribution (default: `21`).
- **Maven Build Lifecycle**: Runs `mvn -B verify` covering:
  - Bytecode compilation (`mvn clean compile`).
  - Static analysis and Checkstyle verification.
  - JUnit 5 test suite execution (`mvn test`).
- **JaCoCo Coverage Enforcement**:
  - Generates JaCoCo coverage metrics.
  - Parses instruction metrics from `target/site/jacoco/jacoco.csv` and outputs package coverage summaries into `$GITHUB_STEP_SUMMARY`.

#### `reusable-typescript.yml`
- **Matrix & Environment**: Sets up Node.js runtime (default: `20`).
- **Dependency Resolution**: Deterministic dependency installation via `npm ci` (fallback: `npm install`).
- **Strict Type Checking**: Runs `npx tsc --noEmit` to verify type safety across the AST without generating build artifacts.
- **Test Suite & Coverage**:
  - Executes unit tests with Vitest (`npx vitest run --coverage`).
  - Enforces line coverage thresholds via `--coverage.thresholds.lines=${{ inputs.min-coverage }}` (default: `80%`).
  - Streams coverage outputs into `$GITHUB_STEP_SUMMARY`.

#### `reusable-security.yml`
- **Secret Detection**: Runs `gitleaks/gitleaks-action` with full Git history audit (`fetch-depth: 0`) to prevent credential, token, and private key leakage.
- **Audit Logging**: Appends security audit sign-offs to `$GITHUB_STEP_SUMMARY`.

#### `semantic-pr.yml`
- **Conventional Commits 1.0.0 Enforcement**: Evaluates the Pull Request title using `amannn/action-semantic-pull-request@v5`.
- **Permitted Types**:
  - `feat`: New functionality
  - `fix`: Bug fix
  - `docs`: Documentation updates
  - `style`: Formatting changes that do not affect code logic
  - `refactor`: Structural code changes without behavior changes
  - `perf`: Performance optimizations
  - `test`: Adding or correcting tests
  - `build`: Build system or external dependency modifications
  - `ci`: CI configuration and workflow files
  - `chore`: Auxiliary tooling, repo management, and housekeeping
  - `revert`: Reverting previous commits
- **Scope Optional**: Validates formats such as `feat(auth): add OAuth2 provider` or `docs: update quality gate guide`.

---

### 2. Branch Protection Ruleset Enforcement

GitHub repository rulesets configured via `repokit configure` or `repokit create` enforce strict remote gatekeeping on the default branch (`main`):

```json
{
  "name": "default-branch-protection",
  "target": "branch",
  "enforcement": "active",
  "conditions": {
    "ref_name": { "include": ["~DEFAULT_BRANCH"], "exclude": [] }
  },
  "rules": [
    { "type": "deletion" },
    { "type": "non_fast_forward" },
    {
      "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 0,
        "dismiss_stale_reviews_on_push": false,
        "require_code_owner_review": false,
        "require_last_push_approval": false,
        "required_review_thread_resolution": false,
        "allowed_merge_methods": ["squash"]
      }
    },
    {
      "type": "required_status_checks",
      "parameters": {
        "strict_required_status_checks_policy": true,
        "do_not_enforce_on_create": false,
        "required_status_checks": [
          { "context": "Semantic PR Title Check / Validate Conventional Commit PR Title" },
          { "context": "ci / Test & Coverage" }
        ]
      }
    }
  ]
}
```

#### Key Mechanics:
1. **`strict_required_status_checks_policy: true`**:
   - The head branch **must be up to date with `main`** before merging.
   - If `main` advances while a PR is under review, the PR author must rebase or update from `main`, triggering a re-run of all required checks.
2. **Hard-Blocked Merge Button**:
   - Both status check contexts (`ci / Test & Coverage` and `Semantic PR Title Check / Validate Conventional Commit PR Title`) must report `success`.
   - Any check failure or pending state hard-blocks the GitHub merge button.
3. **Squash Only**:
   - `allowed_merge_methods: ["squash"]` blocks merge commits and rebase merges, ensuring a clean, linear trunk history.

---

## Gate 2: Agent Review Gate

Gate 2 is an autonomous verification gate executed by the local Antigravity Agent Orchestrator during **Phase 3 (Adversarial Review)** before requesting human review or marking tasks complete.

### Purpose
While Gate 1 validates automated test execution and static rules, Gate 2 provides deep semantic and architectural auditing. It catches subtle bugs, edge cases, hallucinated APIs, leaked temporary artifacts, and specification drift.

### Verification Checklist
The Orchestrator conducts an adversarial review against the Git diff (`git diff main...HEAD`), validating:

1. **Regression & Correctness**:
   - Do all modified modules maintain backward compatibility?
   - Are edge cases and failure paths handled gracefully?
   - Do test suites cover both happy paths and boundary conditions?
2. **Security & Secrets Hygiene**:
   - Are there any `.env` files, API keys, credentials, or tokens in the diff?
   - Does `.gitignore` properly exclude build and runtime artifacts?
3. **AST Sanity & Clean Architecture**:
   - Are dependencies minimal and strictly necessary?
   - Are type hints and docstrings preserved and updated?
   - Is dead or commented-out code completely excised?
4. **Source Control & Commit Hygiene**:
   - Does each worker subagent produce exactly one atomic Conventional Commit?
   - Is the working tree completely clean (`git status` reports nothing untracked or modified)?

### Structured Audit Report
The Orchestrator records its audit findings directly into the PR description or as an immediate PR comment:

```markdown
### Agent Adversarial Audit Report (Phase 3)
- **Status**: [VERIFIED]
- **Target Specification**: Implements RFC-042 additive runner support.
- **Diff Analysis**: 4 files modified (+142, -18).
- **Security Check**: Clean (no credentials, tokens, or untracked state).
- **Test Execution**: 19/19 passing (100% pass rate).
- **AST / Code Quality**: Type annotations verified, formatting clean.
- **Recommendation**: Ready for Gate 3 Human Review & Manual Promotion.
```

---

## Gate 3: Human Review & Manual Promotion Gate

Gate 3 is the human-in-the-loop authorization gate. Autonomous agents are strictly prohibited from merging pull requests or promoting changes to trunk.

### 1. PR Template Sign-Off Checklist

Every repository governed by `repokit` includes `.github/pull_request_template.md`. The author and reviewer must complete the sign-off checklist:

```markdown
## Quality Gates & Sign-Off Checklist
- [ ] **Gate 1: Automated Quality Gates** (All CI checks pass: `ci / Test & Coverage`, `Semantic PR Title Check`)
- [ ] **Gate 2: Agent Review Verification** (Adversarial diff review conducted; no unresolved warnings, regressions, or security leaks)
- [ ] **Gate 3: Human Review & Manual Promotion** (PR reviewed by human developer; merge must only be triggered manually in the GitHub UI, never automated)
```

### 2. Manual Review & Diff Inspection
The human developer:
1. Inspects the pull request diff in the GitHub web interface.
2. Reads the Agent Adversarial Audit Report (Gate 2) and CI Step Summaries (Gate 1).
3. Verifies that the PR title matches Conventional Commits and accurately describes the squashed change.

### 3. Sole Promotion Authorization
- **Human Clicks "Squash and merge"**: Trunk promotion into `main` must only occur via manual human action in the GitHub web UI.
- **Auto-Merge Lockout**:
  - `allow_auto_merge` is disabled on every repository (`allow_auto_merge: false`).
  - Running `gh pr merge` from scripts or agent harnesses is strictly disallowed by developer directives.
  - The repository's squash commit title defaults to `PR_TITLE` and body to `PR_BODY`, ensuring that the human-reviewed PR title and description become the single atomic commit message on `main`.
- **Automatic Head Branch Cleanup**: Upon squash-merging, GitHub automatically deletes the temporary feature branch (`delete_branch_on_merge: true`).

---

## Quality Gate Matrix

| Quality Gate | Responsible Actor | Execution Mechanism | Enforcement Point | Failure Consequence |
|---|---|---|---|---|
| **Gate 1: CI & Ruleset** | GitHub Actions Runner | Reusable workflows & semantic PR action | GitHub Branch Protection Ruleset | Merge button hard-blocked in GitHub UI |
| **Gate 2: Agent Review** | Antigravity Orchestrator | Phase 3 Adversarial Audit | PR Description / Verification Comment | Task rejected, returned to subagent for fixes |
| **Gate 3: Human Promotion** | Human Developer | Web UI Review & Manual Promotion | GitHub "Squash and merge" Button | PR remains open until human approval |
