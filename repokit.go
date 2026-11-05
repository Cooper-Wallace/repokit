package repokit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Terminal ANSI color constants.
const (
	Green  = "\033[1;32m"
	Red    = "\033[1;31m"
	Yellow = "\033[1;33m"
	Blue   = "\033[1;34m"
	Cyan   = "\033[1;36m"
	Bold   = "\033[1m"
	Reset  = "\033[0m"
)

// Core Constants.
const (
	DefaultGovernanceRepo = "Cooper-Wallace/repokit"
	CommitPattern         = `^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert|release)(\([a-zA-Z0-9_.-]+\))?!?: .+$`
	DefaultStagingBranch  = "release-staging"

	CommitMsgHook = `#!/usr/bin/env bash
# Git commit-msg hook enforcing Conventional Commits
# (https://www.conventionalcommits.org/)

set -e

COMMIT_MSG_FILE="$1"
COMMIT_MSG=$(head -n 1 "$COMMIT_MSG_FILE")

# Allow merge commits, squashes, and fixups
if echo "$COMMIT_MSG" | grep -Eq "^(Merge|fixup!|squash!)"; then
    exit 0
fi

# Conventional Commits regex: type(optional-scope)!: description
PATTERN="^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert|release)(\([a-zA-Z0-9_.-]+\))?!?: .+$"

if ! echo "$COMMIT_MSG" | grep -Eq "$PATTERN"; then
    echo -e "\033[1;31m[ERROR] Commit rejected: Message does not follow Conventional Commits.\033[0m"
    echo -e "\033[1;33mExpected:\033[0m <type>(<scope optional>): <description>"
    echo -e "\033[1;33mAllowed Types:\033[0m feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert, release"
    echo -e "\033[1;33mExamples:\033[0m"
    echo "  feat(laser): add pointcloud downsampling"
    echo "  fix(launch): correct topic remapping in simulation"
    echo "  docs(readme): add setup and launch instructions"
    echo "  release(1.1.0): candidate release"
    echo -e "\033[1;31mYour input:\033[0m \"$COMMIT_MSG\""
    exit 1
fi
`

	PreCommitHook = `#!/usr/bin/env bash
# Git pre-commit hook enforcing repokit local standards

if [[ "${REPOKIT_NO_PRE_COMMIT:-0}" == "1" || "${REPOKIT_TEST_RUNNING:-0}" == "1" || "${AGY_VERIFY_RUNNING:-0}" == "1" ]]; then
    exit 0
fi
export REPOKIT_TEST_RUNNING=1

if command -v repokit >/dev/null 2>&1; then
    REPOKIT_CMD="repokit"
elif [ -x "{bin_path}" ]; then
    REPOKIT_CMD="{bin_path}"
elif [ -x "${HOME}/.local/bin/repokit" ]; then
    REPOKIT_CMD="${HOME}/.local/bin/repokit"
else
    REPOKIT_CMD="repokit"
fi

if ! "$REPOKIT_CMD" audit --local; then
    echo -e "\033[1;31m[ERROR] Pre-commit check failed: Repository structure does not meet standards.\033[0m"
    echo -e "\033[1;33mPlease run:\033[0m repokit configure"
    exit 1
fi
`

	PRTemplate = `## Summary
<!-- Provide a clear, concise summary of the problem and proposed solution. -->

## Type of Change
<!-- Please check the item that best describes this change: -->
- [ ] ` + "`feat`" + `: New feature or capability
- [ ] ` + "`fix`" + `: Bug fix
- [ ] ` + "`refactor`" + `: Code refactor (no behavioral changes)
- [ ] ` + "`perf`" + `: Performance improvement
- [ ] ` + "`docs`" + `: Documentation changes
- [ ] ` + "`test`" + `: New or updated tests
- [ ] ` + "`ci`" + `: CI/CD pipeline or build configuration
- [ ] ` + "`chore`" + `: Maintenance, dependencies, or tooling
- [ ] ` + "`release`" + `: Release candidate or version staging

## Key Changes
<!-- Bullet points detailing the specific changes made. -->
- 

## Verification & Testing
<!-- Detail the testing performed to verify this change. -->
- [ ] Automated tests pass (` + "`pytest`" + `, ` + "`mvn test`" + `, ` + "`npm test`" + `, etc.)
- [ ] Code formatting & linting checks pass
- [ ] Manual test / simulation verification (include details/logs below)

**Test Details:**
<!-- Details or output from testing -->

## Subagent Commit Trail
<!-- Chronological list of atomic subagent commits on this branch (git log main..HEAD --oneline) preserved in squash-merge body -->
- 

## Gate 2: Phase 3 Adversarial Review Summary
<!-- Summary of Root Orchestrator adversarial diff audit, edge-case checks, and guarded failure verification -->
- **Adversarial Audit Status**: Passed
- **Guarded Failure / Degrade Path Verified**: 

## Quality Gates & Sign-Off Checklist
- [ ] **Gate 1: Automated Quality Gates** (All CI checks pass: ` + "`ci / Test & Coverage`" + `, ` + "`Semantic PR Title Check`" + `)
- [ ] **Gate 2: Agent Review Verification** (Adversarial diff review conducted; no unresolved warnings, regressions, or security leaks)
- [ ] **Gate 3: Human Review & Manual Promotion** (PR reviewed by human developer; merge must only be triggered manually in the GitHub UI, never automated)

## Pre-Merge Checklist
- [ ] PR title follows [Conventional Commits](https://www.conventionalcommits.org/) (e.g. ` + "`feat(nav): add laser filter`" + `)
- [ ] Branch is rebased/up-to-date with ` + "`main`" + `
- [ ] No temporary debug code or commented-out blocks left in diff
`

	WorkflowSemanticPR = `name: "Semantic PR Title Check"

on:
  pull_request_target:
    types:
      - opened
      - edited
      - synchronize
      - reopened
  workflow_call:

permissions:
  pull-requests: read

jobs:
  main:
    name: Validate Conventional Commit PR Title
    if: github.event_name == 'pull_request' || github.event_name == 'pull_request_target'
    runs-on: ubuntu-latest
    steps:
      - uses: amannn/action-semantic-pull-request@v5
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          types: |
            feat
            fix
            docs
            style
            refactor
            perf
            test
            build
            ci
            chore
            revert
            release
          requireScope: false
`

	GenericReleaseWorkflow = `name: Release

on:
  push:
    branches:
      - main

permissions:
  contents: write
  pull-requests: write

jobs:
  release-stage:
    name: "Semantic Release Gate"
    uses: {{governance_slug}}/.github/workflows/reusable-release-prep.yml@v1
    with:
      release-type: "simple"
`

	DefaultGitignore = `# OS Artifacts
.DS_Store
*~
*.swp

# Build artifacts & caches
__pycache__/
*.py[cod]
*$py.class
build/
install/
log/
dist/
*.egg-info/
.pytest_cache/

# Test coverage
coverage.out
.coverage
htmlcov/
coverage/

# Environment
.venv/
env/
venv/
.env

# IDEs
.idea/
.vscode/*
!.vscode/settings.json
!.vscode/tasks.json
!.vscode/launch.json
`

	DefaultReadme = `# {repo_name}

{description}

---

## Overview
<!-- Provide a high-level overview of this project. -->

## Architecture & Design
<!-- Explain architectural choices, key components, or system design. -->

## Getting Started

### Prerequisites
<!-- List prerequisites, tools, or dependencies. -->

### Installation & Run
` + "```bash" + `
# Clone the repository
git clone https://github.com/{repo_slug}.git
cd {repo_name}
` + "```" + `

## Development & Contribution Flow
This project adheres to professional DevOps standards:
- **Conventional Commits**: Commits and PR titles must follow [Conventional Commits](https://www.conventionalcommits.org/).
- **Short-lived Feature Branches**: All changes are made via ` + "`feat/*`" + ` or ` + "`fix/*`" + ` branches and submitted as Pull Requests.
- **Squash Merges**: Pull requests are verified via CI quality gates and manually squashed into atomic entries on ` + "`main`" + ` via the GitHub UI (auto-merge disabled).
`
)

// Logging Helpers
func LogInfo(format string, a ...interface{}) {
	fmt.Printf("%s[INFO]%s %s\n", Blue, Reset, fmt.Sprintf(format, a...))
}

func LogSuccess(format string, a ...interface{}) {
	fmt.Printf("%s[SUCCESS]%s %s\n", Green, Reset, fmt.Sprintf(format, a...))
}

func LogWarn(format string, a ...interface{}) {
	fmt.Printf("%s[WARN]%s %s\n", Yellow, Reset, fmt.Sprintf(format, a...))
}

func LogError(format string, a ...interface{}) {
	fmt.Printf("%s[ERROR]%s %s\n", Red, Reset, fmt.Sprintf(format, a...))
}

// Command Execution
type CommandRunnerFunc func(args []string, dir string, input string) (string, string, int, error)

func defaultCommandRunner(args []string, dir string, input string) (string, string, int, error) {
	if len(args) == 0 {
		return "", "", 0, nil
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"REPOKIT_NO_PRE_COMMIT=1",
		"REPOKIT_TEST_RUNNING=1",
	)
	if dir != "" {
		cmd.Dir = dir
	}
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return stdout, stderr, exitCode, err
}

var CommandRunner CommandRunnerFunc = defaultCommandRunner

func RunCmd(args []string, dir string) (string, string, int, error) {
	return CommandRunner(args, dir, "")
}

func RunCmdWithInput(args []string, dir string, input string) (string, string, int, error) {
	return CommandRunner(args, dir, input)
}

// File system helpers
func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode().IsRegular() && (fi.Mode().Perm()&0111 != 0)
}

// Git & GitHub Helpers
func GetGitRepoRoot(path string) (string, error) {
	p := path
	if p == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		p = cwd
	}
	stdout, _, code, err := RunCmd([]string{"git", "-C", p, "rev-parse", "--show-toplevel"}, "")
	if code == 0 && strings.TrimSpace(stdout) != "" {
		return strings.TrimSpace(stdout), nil
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("not a git repository")
}

func GetGitHooksDir(repoRoot string) (string, error) {
	stdout, _, code, _ := RunCmd([]string{"git", "-C", repoRoot, "rev-parse", "--git-path", "hooks"}, "")
	if code == 0 && strings.TrimSpace(stdout) != "" {
		p := strings.TrimSpace(stdout)
		if !filepath.IsAbs(p) {
			p = filepath.Join(repoRoot, p)
		}
		return p, nil
	}
	return filepath.Join(repoRoot, ".git", "hooks"), nil
}

func GetRepoSlug(repoRoot string) (string, error) {
	stdout, _, code, _ := RunCmd([]string{"git", "-C", repoRoot, "remote", "get-url", "origin"}, "")
	if code != 0 || strings.TrimSpace(stdout) == "" {
		return "", fmt.Errorf("no origin remote found")
	}
	url := strings.TrimSpace(stdout)
	re := regexp.MustCompile(`github\.com[:/]([^/]+)/([^/\.]+?)(?:\.git)?$`)
	m := re.FindStringSubmatch(url)
	if len(m) >= 3 {
		return fmt.Sprintf("%s/%s", m[1], m[2]), nil
	}
	return "", fmt.Errorf("could not parse repo slug from origin url: %s", url)
}

func GetGitHubUser() (string, error) {
	stdout, stderr, code, err := RunCmd([]string{"gh", "api", "user", "--jq", ".login"}, "")
	if code == 0 && strings.TrimSpace(stdout) != "" {
		return strings.TrimSpace(stdout), nil
	}
	if owner := os.Getenv("GITHUB_REPOSITORY_OWNER"); owner != "" {
		return owner, nil
	}
	return "", fmt.Errorf("failed to get authenticated GitHub user: %v: %s", err, stderr)
}

// Archetype Management
func GetAvailableTemplates() map[string]string {
	return map[string]string{
		"python":     "Modern Python application/package (src layout, pyproject.toml, pytest, ruff)",
		"java":       "Java Maven service (layered architecture: controller/service/model, JUnit 5)",
		"typescript": "TypeScript + Vite application (modular components, Vitest, strict TS)",
		"bash":       "Standalone CLI utility in Bash (bin script, install.sh, smoke tests)",
		"go":         "Modern Go application/service (cmd/ and internal/ layout, go test, golangci-lint)",
	}
}

func DetectArchetype(repoRoot string) string {
	if fileExists(filepath.Join(repoRoot, "go.mod")) {
		return "go"
	}
	if fileExists(filepath.Join(repoRoot, "pyproject.toml")) || fileExists(filepath.Join(repoRoot, "setup.py")) {
		return "python"
	}
	if fileExists(filepath.Join(repoRoot, "pom.xml")) || fileExists(filepath.Join(repoRoot, "build.gradle")) {
		return "java"
	}
	if fileExists(filepath.Join(repoRoot, "tsconfig.json")) || fileExists(filepath.Join(repoRoot, "package.json")) {
		return "typescript"
	}
	if isDir(filepath.Join(repoRoot, "bin")) && fileExists(filepath.Join(repoRoot, "install.sh")) {
		return "bash"
	}
	return ""
}

type CheckItem struct {
	Label  string
	Passed bool
}

func AuditArchetypeStructure(repoRoot string, templateName string) []CheckItem {
	archetype := templateName
	if archetype == "" {
		archetype = DetectArchetype(repoRoot)
	}
	if archetype == "" {
		return nil
	}

	var checks []CheckItem
	switch archetype {
	case "go":
		checks = []CheckItem{
			{Label: "Go: 'go.mod' module definition", Passed: fileExists(filepath.Join(repoRoot, "go.mod"))},
			{Label: "Go: 'cmd/' directory", Passed: isDir(filepath.Join(repoRoot, "cmd"))},
			{Label: "Go: 'internal/' directory or package layout", Passed: isDir(filepath.Join(repoRoot, "internal")) || isDir(filepath.Join(repoRoot, "pkg")) || isDir(filepath.Join(repoRoot, "cmd"))},
			{Label: "Go: '.goreleaser.yaml' release config", Passed: fileExists(filepath.Join(repoRoot, ".goreleaser.yaml")) || fileExists(filepath.Join(repoRoot, ".goreleaser.yml"))},
		}
	case "python":
		checks = []CheckItem{
			{Label: "Python: 'src/' layout directory", Passed: isDir(filepath.Join(repoRoot, "src"))},
			{Label: "Python: 'tests/' directory", Passed: isDir(filepath.Join(repoRoot, "tests"))},
			{Label: "Python: 'pyproject.toml' configuration", Passed: fileExists(filepath.Join(repoRoot, "pyproject.toml"))},
		}
	case "java":
		checks = []CheckItem{
			{Label: "Java: 'src/main/java' source tree", Passed: isDir(filepath.Join(repoRoot, "src", "main", "java"))},
			{Label: "Java: 'src/test/java' test tree", Passed: isDir(filepath.Join(repoRoot, "src", "test", "java"))},
			{Label: "Java: 'pom.xml' or 'build.gradle'", Passed: fileExists(filepath.Join(repoRoot, "pom.xml")) || fileExists(filepath.Join(repoRoot, "build.gradle"))},
		}
	case "typescript":
		checks = []CheckItem{
			{Label: "TypeScript: 'src/' directory", Passed: isDir(filepath.Join(repoRoot, "src"))},
			{Label: "TypeScript: 'package.json' manifest", Passed: fileExists(filepath.Join(repoRoot, "package.json"))},
			{Label: "TypeScript: 'tsconfig.json' config", Passed: fileExists(filepath.Join(repoRoot, "tsconfig.json"))},
		}
	case "bash":
		checks = []CheckItem{
			{Label: "Bash: 'bin/' directory", Passed: isDir(filepath.Join(repoRoot, "bin"))},
			{Label: "Bash: 'install.sh' script", Passed: fileExists(filepath.Join(repoRoot, "install.sh")) && isExecutable(filepath.Join(repoRoot, "install.sh"))},
		}
	}
	return checks
}

func ApplyArchetypeTemplate(templateName string, targetDir string, variables map[string]string) error {
	templateRoot := "templates/" + templateName
	entries, err := fs.ReadDir(TemplatesFS, templateRoot)
	if err != nil || len(entries) == 0 {
		LogWarn("Template directory not found at %s. Using generic scaffolding.", templateRoot)
		return fmt.Errorf("template '%s' not found", templateName)
	}

	varsDict := map[string]string{
		"governance_slug": DefaultGovernanceRepo,
	}
	for k, v := range variables {
		varsDict[k] = v
	}
	if varsDict["governance_slug"] == "" {
		varsDict["governance_slug"] = DefaultGovernanceRepo
	}

	LogInfo("Applying '%s' archetype scaffolding...", templateName)

	err = fs.WalkDir(TemplatesFS, templateRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath := strings.TrimPrefix(path, templateRoot)
		relPath = strings.TrimPrefix(relPath, "/")
		if relPath == "" {
			return nil
		}

		destRelStr := relPath
		for k, v := range varsDict {
			destRelStr = strings.ReplaceAll(destRelStr, "{{"+k+"}}", v)
		}
		destRelStr = strings.TrimSuffix(destRelStr, ".tmpl")

		destPath := filepath.Join(targetDir, destRelStr)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		data, err := TemplatesFS.ReadFile(path)
		if err != nil {
			return err
		}

		content := string(data)
		content = strings.TrimPrefix(content, "//go:build ignore\r\n\r\n")
		content = strings.TrimPrefix(content, "//go:build ignore\n\n")
		content = strings.TrimPrefix(content, "//go:build ignore\r\n")
		content = strings.TrimPrefix(content, "//go:build ignore\n")
		for k, v := range varsDict {
			content = strings.ReplaceAll(content, "{{"+k+"}}", v)
		}

		isExec := strings.HasSuffix(destPath, ".sh") ||
			filepath.Base(filepath.Dir(destPath)) == "bin" ||
			strings.HasPrefix(content, "#!")
		perm := os.FileMode(0644)
		if isExec {
			perm = 0755
		}

		return os.WriteFile(destPath, []byte(content), perm)
	})

	if err != nil {
		LogError("Failed to apply '%s' template: %v", templateName, err)
		return err
	}

	LogSuccess("Scaffolded '%s' archetype layout.", templateName)
	return nil
}

// GitHub Governance API
func FormatRulesetID(id interface{}) string {
	switch v := id.(type) {
	case float64:
		return fmt.Sprintf("%.0f", v)
	case float32:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

var formatRulesetID = FormatRulesetID

func ApplyWorkflowPermissions(repoSlug string) error {
	LogInfo("Configuring GitHub Actions workflow permissions for %s...", repoSlug)
	stdout, stderr, code, err := RunCmd([]string{
		"gh", "api", "--method", "PUT",
		fmt.Sprintf("repos/%s/actions/permissions/workflow", repoSlug),
		"-F", "default_workflow_permissions=read",
		"-F", "can_approve_pull_request_reviews=true",
	}, "")
	if code == 0 {
		LogSuccess("GitHub Actions workflow permissions configured (read permissions, can approve PRs).")
		return nil
	}
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		msg = strings.TrimSpace(stdout)
	}
	LogWarn("Failed to update workflow permissions via GitHub API: %s", msg)
	if err != nil {
		return err
	}
	return fmt.Errorf("gh api returned exit code %d: %s", code, msg)
}

func ApplyGitHubRepoSettings(repoSlug string) error {
	LogInfo("Applying GitHub merge and branch cleanup settings to %s...", repoSlug)
	payload := map[string]interface{}{
		"allow_squash_merge":          true,
		"allow_merge_commit":          false,
		"allow_rebase_merge":          false,
		"delete_branch_on_merge":      true,
		"allow_update_branch":         true,
		"squash_merge_commit_title":   "PR_TITLE",
		"squash_merge_commit_message": "PR_BODY",
		"allow_auto_merge":            false,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, stderr, code, err := RunCmdWithInput([]string{
		"gh", "api", "--method", "PATCH", fmt.Sprintf("repos/%s", repoSlug), "--input", "-",
	}, "", string(payloadBytes))

	if code == 0 {
		LogSuccess("GitHub repository settings updated (Squash only, PR_TITLE, auto-delete branches).")
		return nil
	}
	LogWarn("Failed to update some repository settings via GitHub API: %s", strings.TrimSpace(stderr))
	if err != nil {
		return err
	}
	return fmt.Errorf("gh api returned exit code %d: %s", code, stderr)
}

// IsVaultOrExemptRepo returns true if the repository name (after '/', case-insensitive)
// is "agentvault" or "agent-vault".
func IsVaultOrExemptRepo(slug string) bool {
	repoName := slug
	if idx := strings.LastIndex(slug, "/"); idx != -1 {
		repoName = slug[idx+1:]
	}
	repoName = strings.ToLower(strings.TrimSpace(repoName))
	return repoName == "agentvault" || repoName == "agent-vault"
}

func ApplyBranchRuleset(repoSlug string, defaultBranch string, requiredChecks []string) error {
	if IsVaultOrExemptRepo(repoSlug) {
		LogInfo("Skipping branch protection ruleset and CI scaffolding for exempt vault repository %s", repoSlug)
		return nil
	}
	if defaultBranch == "" {
		defaultBranch = "~DEFAULT_BRANCH"
	}
	LogInfo("Configuring default branch protection ruleset on %s...", repoSlug)
	stdout, _, code, _ := RunCmd([]string{"gh", "api", fmt.Sprintf("repos/%s/rulesets", repoSlug)}, "")
	var existingID interface{}
	if code == 0 && stdout != "" {
		var rulesets []map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &rulesets); err == nil {
			for _, r := range rulesets {
				if r["name"] == "default-branch-protection" {
					existingID = r["id"]
					break
				}
			}
		}
	}

	if requiredChecks == nil {
		requiredChecks = []string{
			"Quality Gate / Verification",
			"Quality Gate / Title Validation",
		}
	}

	checksPayload := make([]map[string]string, len(requiredChecks))
	for i, ctx := range requiredChecks {
		checksPayload[i] = map[string]string{"context": ctx}
	}

	payload := map[string]interface{}{
		"name":        "default-branch-protection",
		"target":      "branch",
		"enforcement": "active",
		"bypass_actors": []map[string]interface{}{
			{
				"actor_id":    5,
				"actor_type":  "RepositoryRole",
				"bypass_mode": "always",
			},
		},
		"conditions": map[string]interface{}{
			"ref_name": map[string]interface{}{
				"include": []string{defaultBranch},
				"exclude": []string{},
			},
		},
		"rules": []map[string]interface{}{
			{"type": "deletion"},
			{"type": "non_fast_forward"},
			{
				"type": "pull_request",
				"parameters": map[string]interface{}{
					"required_approving_review_count":   0,
					"dismiss_stale_reviews_on_push":     false,
					"require_code_owner_review":         false,
					"require_last_push_approval":        false,
					"required_review_thread_resolution": false,
					"allowed_merge_methods":             []string{"squash"},
				},
			},
			{
				"type": "required_status_checks",
				"parameters": map[string]interface{}{
					"strict_required_status_checks_policy": true,
					"do_not_enforce_on_create":             false,
					"required_status_checks":              checksPayload,
				},
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	var cmdArgs []string
	if existingID != nil {
		idStr := formatRulesetID(existingID)
		LogInfo("Updating existing ruleset (ID: %s)...", idStr)
		cmdArgs = []string{"gh", "api", "--method", "PUT", fmt.Sprintf("repos/%s/rulesets/%s", repoSlug, idStr), "--input", "-"}
	} else {
		LogInfo("Creating new default branch protection ruleset...")
		cmdArgs = []string{"gh", "api", "--method", "POST", fmt.Sprintf("repos/%s/rulesets", repoSlug), "--input", "-"}
	}

	stdoutRule, stderrRule, codeRule, err := RunCmdWithInput(cmdArgs, "", string(payloadBytes))
	if codeRule == 0 {
		LogSuccess("Branch protection ruleset active (blocks direct pushes/force pushes, allows solo PR merge).")
		return nil
	}
	msg := strings.TrimSpace(stderrRule)
	if msg == "" {
		msg = strings.TrimSpace(stdoutRule)
	}
	LogWarn("Note on ruleset: %s", msg)
	LogInfo("If using a private repo on GitHub Free, rulesets or classic branch protection may require public visibility.")
	if err != nil {
		return err
	}
	return fmt.Errorf("ruleset application returned non-zero code %d: %s", codeRule, msg)
}

// Git Hooks & Template Scaffolding
func InstallCommitHook(repoRoot string) error {
	hooksDir, err := GetGitHooksDir(repoRoot)
	if err != nil {
		return err
	}
	hookPath := filepath.Join(hooksDir, "commit-msg")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
		LogError("Failed to install git hook: %v", err)
		return err
	}
	if err := os.WriteFile(hookPath, []byte(CommitMsgHook), 0755); err != nil {
		LogError("Failed to install git hook: %v", err)
		return err
	}
	rel, err := filepath.Rel(repoRoot, hookPath)
	if err != nil {
		rel = hookPath
	}
	LogSuccess("Installed Conventional Commits hook: %s", rel)
	return nil
}

func InstallPreCommitHook(repoRoot string) error {
	hooksDir, err := GetGitHooksDir(repoRoot)
	if err != nil {
		return err
	}
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
		LogError("Failed to install git pre-commit hook: %v", err)
		return err
	}
	binPath, err := os.Executable()
	if err != nil || strings.HasSuffix(binPath, ".test") || strings.Contains(binPath, "go-build") || strings.Contains(binPath, "repokit.test") {
		binPath = filepath.Join(os.Getenv("HOME"), ".local", "bin", "repokit")
	}
	hookContent := strings.ReplaceAll(PreCommitHook, "{bin_path}", binPath)
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		LogError("Failed to install git pre-commit hook: %v", err)
		return err
	}
	rel, err := filepath.Rel(repoRoot, hookPath)
	if err != nil {
		rel = hookPath
	}
	LogSuccess("Installed pre-commit audit hook: %s", rel)
	return nil
}

func InstallHooks(repoRoot string, allHooks bool) error {
	if err := InstallCommitHook(repoRoot); err != nil {
		return err
	}
	if allHooks {
		if err := InstallPreCommitHook(repoRoot); err != nil {
			return err
		}
	}
	return nil
}

func ScaffoldTemplates(repoRoot string, force bool) error {
	prTemplatePath := filepath.Join(repoRoot, ".github", "pull_request_template.md")
	if !fileExists(prTemplatePath) || force {
		if err := os.MkdirAll(filepath.Dir(prTemplatePath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(prTemplatePath, []byte(PRTemplate), 0644); err != nil {
			return err
		}
		LogSuccess("Scaffolded .github/pull_request_template.md")
	} else {
		LogInfo("PR template already exists (use --force to overwrite).")
	}

	workflowPath := filepath.Join(repoRoot, ".github", "workflows", "semantic-pr.yml")
	if !fileExists(workflowPath) || force {
		if err := os.MkdirAll(filepath.Dir(workflowPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(workflowPath, []byte(WorkflowSemanticPR), 0644); err != nil {
			return err
		}
		LogSuccess("Scaffolded .github/workflows/semantic-pr.yml")
	} else {
		LogInfo("Semantic PR workflow already exists (use --force to overwrite).")
	}
	return nil
}

func PatchCIWorkflowPRGuard(ciFile string) (bool, error) {
	data, err := os.ReadFile(ciFile)
	if err != nil {
		return false, err
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	modified := false

	// 1. Guard validate-pr job if present and unguarded
	if strings.Contains(content, "validate-pr:") &&
		!strings.Contains(content, "github.event_name == 'pull_request'") &&
		!strings.Contains(content, "github.event_name == \"pull_request\"") {

		var newLines []string
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			newLines = append(newLines, line)
			trimmed := strings.TrimSpace(line)
			if trimmed == "validate-pr:" {
				hasIf := false
				for j := i + 1; j < len(lines); j++ {
					nextTrimmed := strings.TrimSpace(lines[j])
					if nextTrimmed == "" {
						continue
					}
					if strings.HasPrefix(nextTrimmed, "if:") {
						hasIf = true
					}
					break
				}
				if !hasIf {
					indent := ""
					for _, ch := range line {
						if ch == ' ' || ch == '\t' {
							indent += string(ch)
						} else {
							break
						}
					}
					newLines = append(newLines, indent+"    if: github.event_name == 'pull_request'")
					modified = true
				}
			}
		}

		if modified {
			lines = newLines
		}
	}

	// 2. Patch inline ci: job (runs-on:) name to "ci / Test & Coverage"
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "ci:" {
			jobIndent := len(lines[i]) - len(strings.TrimLeft(lines[i], " \t"))
			jobEnd := len(lines)
			for j := i + 1; j < len(lines); j++ {
				t := strings.TrimSpace(lines[j])
				if t == "" {
					continue
				}
				lineIndent := len(lines[j]) - len(strings.TrimLeft(lines[j], " \t"))
				if lineIndent <= jobIndent {
					jobEnd = j
					break
				}
			}

			hasRunsOn := false
			for j := i + 1; j < jobEnd; j++ {
				if strings.Contains(lines[j], "runs-on:") {
					hasRunsOn = true
					break
				}
			}

			if hasRunsOn {
				for j := i + 1; j < jobEnd; j++ {
					t := strings.TrimSpace(lines[j])
					if strings.HasPrefix(t, "name:") {
						val := strings.TrimSpace(strings.TrimPrefix(t, "name:"))
						if val == `"Test & Coverage"` || val == `'Test & Coverage'` || val == `Test & Coverage` {
							lineIndentStr := lines[j][:len(lines[j])-len(strings.TrimLeft(lines[j], " \t"))]
							lines[j] = lineIndentStr + `name: "ci / Test & Coverage"`
							modified = true
						}
					}
				}
			}
		}
	}

	if modified {
		newContent := strings.Join(lines, "\n")
		if err := os.WriteFile(ciFile, []byte(newContent), 0644); err != nil {
			return false, err
		}
		LogSuccess("Patched existing CI workflow (.github/workflows/ci.yml)")
		return true, nil
	}
	return false, nil
}

func IsGovernanceHub(repoRoot string) bool {
	return (fileExists(filepath.Join(repoRoot, ".github", "workflows", "reusable-python.yml")) ||
		fileExists(filepath.Join(repoRoot, ".github", "workflows", "reusable-go.yml"))) &&
		(fileExists(filepath.Join(repoRoot, "bin", "repokit")) ||
			fileExists(filepath.Join(repoRoot, "cmd", "repokit", "main.go")) ||
			fileExists(filepath.Join(repoRoot, "repokit.go")))
}

func ScaffoldArchetypeCI(repoRoot string, force bool) (bool, error) {
	if IsGovernanceHub(repoRoot) {
		return false, nil
	}
	archetype := DetectArchetype(repoRoot)
	targetCI := filepath.Join(repoRoot, ".github", "workflows", "ci.yml")
	if archetype == "" {
		if fileExists(targetCI) && !force {
			return PatchCIWorkflowPRGuard(targetCI)
		}
		return false, nil
	}

	templatePath := fmt.Sprintf("templates/%s/.github/workflows/ci.yml", archetype)
	templateData, err := TemplatesFS.ReadFile(templatePath)
	if err == nil && (!fileExists(targetCI) || force) {
		if err := os.MkdirAll(filepath.Dir(targetCI), 0755); err != nil {
			return false, err
		}
		content := string(templateData)
		content = strings.ReplaceAll(content, "{{governance_slug}}", DefaultGovernanceRepo)
		if err := os.WriteFile(targetCI, []byte(content), 0644); err != nil {
			return false, err
		}
		LogSuccess("Scaffolded CI workflow calling central repokit workflows (.github/workflows/ci.yml)")
		return true, nil
	} else if fileExists(targetCI) && !force {
		patched, err := PatchCIWorkflowPRGuard(targetCI)
		if patched {
			return true, err
		}
		LogInfo("CI workflow already exists (use --force to overwrite).")
		return false, nil
	}
	return false, nil
}

func ScaffoldReleaseWorkflow(repoRoot string, force bool) (bool, error) {
	targetRelease := filepath.Join(repoRoot, ".github", "workflows", "release.yml")
	if fileExists(targetRelease) && !force {
		LogInfo("Release workflow already exists (use --force to overwrite).")
		return false, nil
	}

	archetype := DetectArchetype(repoRoot)
	var content string
	if archetype != "" {
		templatePath := fmt.Sprintf("templates/%s/.github/workflows/release.yml", archetype)
		if data, err := TemplatesFS.ReadFile(templatePath); err == nil {
			content = string(data)
		}
	}
	if content == "" {
		content = GenericReleaseWorkflow
	}

	content = strings.ReplaceAll(content, "{{governance_slug}}", DefaultGovernanceRepo)
	if err := os.MkdirAll(filepath.Dir(targetRelease), 0755); err != nil {
		return false, err
	}
	if err := os.WriteFile(targetRelease, []byte(content), 0644); err != nil {
		return false, err
	}
	LogSuccess("Scaffolded release workflow (.github/workflows/release.yml)")

	if archetype == "go" {
		targetGoreleaser := filepath.Join(repoRoot, ".goreleaser.yaml")
		if !fileExists(targetGoreleaser) || force {
			data, err := TemplatesFS.ReadFile("templates/go/.goreleaser.yaml")
			if err == nil {
				gContent := string(data)
				gContent = strings.ReplaceAll(gContent, "{{repo_name}}", filepath.Base(repoRoot))
				gContent = strings.ReplaceAll(gContent, "{{governance_slug}}", DefaultGovernanceRepo)
				if err := os.WriteFile(targetGoreleaser, []byte(gContent), 0644); err != nil {
					return false, err
				}
				LogSuccess("Scaffolded GoReleaser configuration (.goreleaser.yaml)")
			}
		}
	}

	return true, nil
}

// Audit Engine
func AuditRepository(repoRoot string, archetype string, localOnly bool, remoteOnly bool) (bool, error) {
	fmt.Printf("\n%sAuditing Repository Standards:%s %s\n\n", Bold, Reset, repoRoot)

	runLocal := true
	runRemote := true
	if localOnly && remoteOnly {
		runLocal = true
		runRemote = true
	} else if localOnly {
		runLocal = true
		runRemote = false
	} else if remoteOnly {
		runLocal = false
		runRemote = true
	}

	var slug string
	if runRemote {
		s, err := GetRepoSlug(repoRoot)
		if err != nil || s == "" {
			LogError("No GitHub remote origin found for this repository.")
			return false, fmt.Errorf("no GitHub remote origin found")
		}
		slug = s
		fmt.Printf("%sGitHub Repository:%s %s\n\n", Cyan, Reset, slug)
	} else {
		s, _ := GetRepoSlug(repoRoot)
		slug = s
		if slug != "" {
			fmt.Printf("%sGitHub Repository:%s %s (local audit only)\n\n", Cyan, Reset, slug)
		} else {
			fmt.Printf("%sLocal Repository:%s %s (local audit only)\n\n", Cyan, Reset, filepath.Base(repoRoot))
		}
	}

	var remoteItems []CheckItem
	if runRemote && slug != "" {
		stdout, _, code, _ := RunCmd([]string{"gh", "api", fmt.Sprintf("repos/%s", slug)}, "")
		var ghData map[string]interface{}
		if code == 0 && stdout != "" {
			_ = json.Unmarshal([]byte(stdout), &ghData)
		}

		squashEnabled := ghData["allow_squash_merge"] == true
		mergeCommitDisabled := ghData["allow_merge_commit"] == false
		rebaseDisabled := ghData["allow_rebase_merge"] == false
		deleteBranch := ghData["delete_branch_on_merge"] == true
		titleIsPR := ghData["squash_merge_commit_title"] == "PR_TITLE"
		msgVal, _ := ghData["squash_merge_commit_message"].(string)
		msgIsPR := msgVal == "PR_BODY" || msgVal == "BLANK"
		autoMergeDisabled := ghData["allow_auto_merge"] == false

		stdoutRules, _, codeRules, _ := RunCmd([]string{"gh", "api", fmt.Sprintf("repos/%s/rulesets", slug)}, "")
		hasRuleset := false
		rulesetHasStatusChecks := false
		if codeRules == 0 && stdoutRules != "" {
			var rulesets []map[string]interface{}
			if err := json.Unmarshal([]byte(stdoutRules), &rulesets); err == nil {
				for _, r := range rulesets {
					if r["name"] == "default-branch-protection" && r["enforcement"] == "active" {
						hasRuleset = true
						var rules []interface{}
						if rRules, ok := r["rules"].([]interface{}); ok && len(rRules) > 0 {
							rules = rRules
						} else if idVal, ok := r["id"]; ok {
							stdoutDetail, _, codeDetail, _ := RunCmd([]string{"gh", "api", fmt.Sprintf("repos/%s/rulesets/%s", slug, formatRulesetID(idVal))}, "")
							if codeDetail == 0 {
								var detail map[string]interface{}
								if err := json.Unmarshal([]byte(stdoutDetail), &detail); err == nil {
									if dRules, ok := detail["rules"].([]interface{}); ok {
										rules = dRules
									}
								}
							}
						}
						for _, ruleRaw := range rules {
							if ruleMap, ok := ruleRaw.(map[string]interface{}); ok {
								if ruleMap["type"] == "required_status_checks" {
									if params, ok := ruleMap["parameters"].(map[string]interface{}); ok {
										if reqChecks, ok := params["required_status_checks"].([]interface{}); ok {
											for _, checkRaw := range reqChecks {
												if checkMap, ok := checkRaw.(map[string]interface{}); ok {
													if ctx, ok := checkMap["context"].(string); ok {
														if strings.Contains(ctx, "Quality Gate / Verification") || strings.Contains(ctx, "Test & Coverage") {
															rulesetHasStatusChecks = true
															break
														}
													}
												}
											}
										} else {
											rulesetHasStatusChecks = true
										}
									} else {
										rulesetHasStatusChecks = true
									}
									break
								}
							}
						}
						break
					}
				}
			}
		}

		stdoutPerms, _, codePerms, _ := RunCmd([]string{"gh", "api", fmt.Sprintf("repos/%s/actions/permissions/workflow", slug)}, "")
		canApprovePRs := false
		if codePerms == 0 && stdoutPerms != "" {
			var permsData map[string]interface{}
			if err := json.Unmarshal([]byte(stdoutPerms), &permsData); err == nil {
				canApprovePRs = permsData["can_approve_pull_request_reviews"] == true
			}
		}

		remoteItems = []CheckItem{
			{Label: "Squash & Merge Enabled", Passed: squashEnabled},
			{Label: "Merge Commits Disabled", Passed: mergeCommitDisabled},
			{Label: "Rebase Commits Disabled", Passed: rebaseDisabled},
			{Label: "Auto-Delete Head Branches on Merge", Passed: deleteBranch},
			{Label: "Squash Commit Title set to PR_TITLE", Passed: titleIsPR},
			{Label: "Squash Commit Message set to PR_BODY", Passed: msgIsPR},
			{Label: "Auto-Merge Disabled (Human Gate)", Passed: autoMergeDisabled},
			{Label: "Default Branch Protection Ruleset Active", Passed: hasRuleset},
			{Label: "Branch Ruleset Enforces Required Status Checks", Passed: rulesetHasStatusChecks},
			{Label: "GitHub Actions Permitted to Create/Approve PRs (Workflow Permissions)", Passed: canApprovePRs},
		}
	}

	var localItems []CheckItem
	if runLocal {
		hooksDir, _ := GetGitHooksDir(repoRoot)
		hookPath := filepath.Join(hooksDir, "commit-msg")
		hasHook := isExecutable(hookPath)

		preHookPath := filepath.Join(hooksDir, "pre-commit")
		hasPreHook := isExecutable(preHookPath)

		hasPRTemplate := fileExists(filepath.Join(repoRoot, ".github", "pull_request_template.md"))

		ciWorkflowPath := filepath.Join(repoRoot, ".github", "workflows", "ci.yml")
		isGovernanceHub := IsGovernanceHub(repoRoot)

		var hasCIWorkflow bool
		var ciLabel string
		if isGovernanceHub {
			hasCIWorkflow = fileExists(filepath.Join(repoRoot, ".github", "workflows", "test.yml"))
			ciLabel = "Central Reusable Workflows Hub Hosted (.github/workflows/reusable-*.yml)"
		} else {
			hasCIWorkflow = false
			if fileExists(ciWorkflowPath) {
				data, err := os.ReadFile(ciWorkflowPath)
				if err == nil {
					ciContent := string(data)
					if strings.Contains(ciContent, DefaultGovernanceRepo) ||
						strings.Contains(ciContent, "reusable-") ||
						strings.Contains(ciContent, "Cooper-Wallace/repokit") {
						hasCIWorkflow = true
					}
				}
			}
			if hasCIWorkflow {
				ciLabel = "CI Workflow calling central repokit workflows (.github/workflows/ci.yml)"
			} else {
				ciLabel = "CI Workflow missing or not calling central repokit reusable workflows"
			}
		}

		hasPRGuard := isGovernanceHub
		if !isGovernanceHub && fileExists(ciWorkflowPath) {
			data, err := os.ReadFile(ciWorkflowPath)
			if err == nil {
				ciContent := string(data)
				if strings.Contains(ciContent, "validate-pr:") {
					hasPRGuard = strings.Contains(ciContent, "github.event_name == 'pull_request'") ||
						strings.Contains(ciContent, "github.event_name == \"pull_request\"")
				} else {
					hasPRGuard = true
				}
			}
		}

		releaseWorkflowPath := filepath.Join(repoRoot, ".github", "workflows", "release.yml")
		hasReleaseWorkflow := false
		hasReleasePermissions := false
		if fileExists(releaseWorkflowPath) {
			if relData, err := os.ReadFile(releaseWorkflowPath); err == nil {
				relContent := string(relData)
				if strings.Contains(relContent, "release-stage") ||
					strings.Contains(relContent, "repokit release") ||
					strings.Contains(relContent, "release-please") ||
					strings.Contains(relContent, "reusable-release-") ||
					strings.Contains(relContent, "action-gh-release") ||
					strings.Contains(relContent, "goreleaser") {
					hasReleaseWorkflow = true
				}
				if hasReleaseWorkflow {
					hasContentsWrite := strings.Contains(relContent, "contents: write") ||
						strings.Contains(relContent, "contents:write") ||
						strings.Contains(relContent, "write-all")
					hasPRWrite := strings.Contains(relContent, "pull-requests: write") ||
						strings.Contains(relContent, "pull-requests:write") ||
						strings.Contains(relContent, "write-all")
					if hasContentsWrite && hasPRWrite {
						hasReleasePermissions = true
					}
				}
			}
		}

		localItems = []CheckItem{
			{Label: "Local Conventional Commit Hook Installed (.git/hooks/commit-msg)", Passed: hasHook},
			{Label: "Local Pre-commit Hook Installed (.git/hooks/pre-commit)", Passed: hasPreHook},
			{Label: "Pull Request Template (.github/pull_request_template.md)", Passed: hasPRTemplate},
			{Label: ciLabel, Passed: hasCIWorkflow},
			{Label: "CI Workflow PR Event Guard (skips PR check on main push)", Passed: hasPRGuard},
			{Label: "Release Workflow Active (.github/workflows/release.yml)", Passed: hasReleaseWorkflow},
			{Label: "Release Workflow Permissions (contents: write, pull-requests: write)", Passed: hasReleasePermissions},
		}
	}

	allPassed := true

	if len(remoteItems) > 0 {
		fmt.Printf("%sGitHub Remote Governance Standards:%s\n", Bold, Reset)
		for _, item := range remoteItems {
			if item.Passed {
				fmt.Printf("  %s✔%s %s\n", Green, Reset, item.Label)
			} else {
				fmt.Printf("  %s✘%s %s\n", Red, Reset, item.Label)
				allPassed = false
			}
		}
	}

	if len(localItems) > 0 {
		if len(remoteItems) > 0 {
			fmt.Println()
		}
		fmt.Printf("%sLocal Repository Standards:%s\n", Bold, Reset)
		for _, item := range localItems {
			if item.Passed {
				fmt.Printf("  %s✔%s %s\n", Green, Reset, item.Label)
			} else {
				fmt.Printf("  %s✘%s %s\n", Red, Reset, item.Label)
				allPassed = false
			}
		}
	}

	if runLocal {
		structChecks := AuditArchetypeStructure(repoRoot, archetype)
		detected := archetype
		if detected == "" {
			detected = DetectArchetype(repoRoot)
		}
		if len(structChecks) > 0 {
			fmt.Printf("\n%sArchetype Structure Standards (%s):%s\n", Bold, detected, Reset)
			for _, item := range structChecks {
				if item.Passed {
					fmt.Printf("  %s✔%s %s\n", Green, Reset, item.Label)
				} else {
					fmt.Printf("  %s✘%s %s\n", Red, Reset, item.Label)
					allPassed = false
				}
			}
		} else {
			hasSrcDir := isDir(filepath.Join(repoRoot, "src")) ||
				isDir(filepath.Join(repoRoot, "bin")) ||
				isDir(filepath.Join(repoRoot, "cmd")) ||
				isDir(filepath.Join(repoRoot, "src", "main", "java"))
			fmt.Printf("\n%sRepository Structure Standards:%s\n", Bold, Reset)
			if hasSrcDir {
				fmt.Printf("  %s✔%s Source directory structure present ('src/' or 'bin/')\n", Green, Reset)
			} else {
				fmt.Printf("  %s✘%s Source directory structure missing (expected 'src/' or 'bin/')\n", Red, Reset)
				allPassed = false
			}
		}
	}

	fmt.Println()
	if allPassed {
		LogSuccess("All repository standards are strictly satisfied!")
		return true, nil
	}
	LogWarn("Some standards are missing. Run `repokit configure` to automatically apply them.")
	return false, nil
}

// Commands
func ConfigureRepository(targetDir string, force bool) error {
	return ConfigureWithOptions(ConfigureOptions{
		TargetDir: targetDir,
		Force:     force,
	})
}

func CreateRepository(name string, description string, public bool, templateType string, targetDir string) error {
	user, err := GetGitHubUser()
	if err != nil || user == "" {
		LogError("Not authenticated with GitHub CLI. Please run `gh auth login` first.")
		return fmt.Errorf("not authenticated with GitHub CLI")
	}

	slug := fmt.Sprintf("%s/%s", user, name)
	visibility := "--private"
	visStr := "private"
	if public {
		visibility = "--public"
		visStr = "public"
	}

	path := targetDir
	if path == "" {
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, name)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	if entries, err := os.ReadDir(absPath); err == nil && len(entries) > 0 {
		LogError("Target directory %s already exists and is not empty.", absPath)
		return fmt.Errorf("target directory %s already exists and is not empty", absPath)
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		return err
	}

	LogInfo("Creating new GitHub repository %s (%s)...", slug, visStr)
	createArgs := []string{"gh", "repo", "create", slug, visibility}
	if description != "" {
		createArgs = append(createArgs, "--description", description)
	}

	_, stderr, code, err := RunCmd(createArgs, "")
	if code != 0 {
		LogError("Failed to create repo via gh: %s", strings.TrimSpace(stderr))
		if err != nil {
			return err
		}
		return fmt.Errorf("failed to create repo via gh: %s", stderr)
	}
	LogSuccess("GitHub repository created: https://github.com/%s", slug)

	LogInfo("Initialising local repository at %s...", absPath)
	RunCmd([]string{"git", "init", "-b", "main"}, absPath)
	RunCmd([]string{"git", "remote", "add", "origin", fmt.Sprintf("https://github.com/%s.git", slug)}, absPath)

	rePkg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	pkgName := strings.ToLower(rePkg.ReplaceAllString(name, "_"))
	if len(pkgName) > 0 && pkgName[0] >= '0' && pkgName[0] <= '9' {
		pkgName = "pkg_" + pkgName
	}

	desc := description
	if desc == "" {
		tDesc := "software"
		if templateType != "" {
			tDesc = templateType
		}
		desc = fmt.Sprintf("A standardised %s project.", tDesc)
	}

	variables := map[string]string{
		"repo_name":       name,
		"package_name":    pkgName,
		"description":     desc,
		"author":          "Cooper Wallace",
		"year":            "2026",
		"repo_slug":       slug,
		"governance_slug": DefaultGovernanceRepo,
	}

	if templateType != "" {
		_ = ApplyArchetypeTemplate(templateType, absPath, variables)
	} else {
		readmeContent := strings.ReplaceAll(DefaultReadme, "{repo_name}", name)
		readmeContent = strings.ReplaceAll(readmeContent, "{description}", desc)
		readmeContent = strings.ReplaceAll(readmeContent, "{repo_slug}", slug)
		_ = os.WriteFile(filepath.Join(absPath, "README.md"), []byte(readmeContent), 0644)
		_ = os.WriteFile(filepath.Join(absPath, ".gitignore"), []byte(DefaultGitignore), 0644)
	}

	_ = ScaffoldTemplates(absPath, true)
	_, _ = ScaffoldReleaseWorkflow(absPath, true)
	_ = InstallHooks(absPath, true)

	RunCmd([]string{"git", "add", "."}, absPath)
	RunCmd([]string{"git", "commit", "-m", "chore: initial repository scaffolding"}, absPath)

	LogInfo("Pushing initial commit to main...")
	_, pushErr, pushCode, _ := RunCmd([]string{"git", "push", "-u", "origin", "main"}, absPath)
	if pushCode != 0 {
		LogWarn("Initial push: %s", strings.TrimSpace(pushErr))
	}

	_ = ApplyGitHubRepoSettings(slug)
	_ = ApplyWorkflowPermissions(slug)
	_ = ApplyBranchRuleset(slug, "~DEFAULT_BRANCH", nil)

	fmt.Println()
	LogSuccess("New repository %s initialised and fully standardised!", slug)
	fmt.Printf("\n%sNext Steps:%s\n", Cyan, Reset)
	fmt.Printf("  cd %s\n", absPath)
	fmt.Println("  git checkout -b feat/initial-setup")
	fmt.Println()
	return nil
}

func PrintTemplates() {
	templates := GetAvailableTemplates()
	fmt.Printf("\n%sAvailable Solution Archetypes:%s\n\n", Bold, Reset)
	order := []string{"python", "java", "typescript", "bash", "go"}
	for _, name := range order {
		if desc, ok := templates[name]; ok {
			fmt.Printf("  %s%-12s%s %s\n", Cyan, name, Reset, desc)
		}
	}
	fmt.Printf("\n%sUsage Example:%s\n", Bold, Reset)
	fmt.Printf("  repokit create my-api --type python --public -d 'My Python service'\n\n")
}

// Configuration & Multi-Repository Discovery Engine
type ConfigureOptions struct {
	All          bool
	Repos        []string
	Local        bool
	LocalDir     string
	TargetDir    string
	Force        bool
	DryRun       bool
	CreatePR     bool
	DirectCommit bool   // commit and push directly to main branch
	ReleaseRef   string // release reference tag (e.g. "v1.1.0"), defaults to "latest" if empty
}

// PropagateOptions is an alias for ConfigureOptions for backward compatibility.
type PropagateOptions = ConfigureOptions

// PropagateRepositories is retained for backward compatibility.
func PropagateRepositories(opts PropagateOptions) error {
	if !opts.All && !opts.Local && len(opts.Repos) == 0 {
		return fmt.Errorf("no repositories specified to propagate (use --all, --local, or --repos)")
	}
	return ConfigureWithOptions(opts)
}

// ConfigureWithOptions applies repository standards to a single repo or across discovered repos.
func ConfigureWithOptions(opts ConfigureOptions) error {
	if opts.All || len(opts.Repos) > 0 || opts.Local {
		return configureMultipleRepositories(opts)
	}
	return configureSingleRepository(opts)
}

func configureSingleRepository(opts ConfigureOptions) error {
	targetDir := opts.TargetDir
	if targetDir == "" && opts.LocalDir != "" {
		targetDir = opts.LocalDir
	}

	repoRoot, err := GetGitRepoRoot(targetDir)
	if err != nil || repoRoot == "" {
		LogError("Target '%s' is not inside a git repository.", targetDir)
		return fmt.Errorf("target '%s' is not inside a git repository", targetDir)
	}

	slug, err := GetRepoSlug(repoRoot)
	if err != nil || slug == "" {
		LogError("Could not find GitHub remote origin URL for this repository.")
		return fmt.Errorf("could not find GitHub remote origin URL")
	}

	if IsVaultOrExemptRepo(slug) {
		LogInfo("Skipping branch protection ruleset and CI scaffolding for exempt vault repository %s", slug)
		if opts.DryRun {
			LogInfo("[DRY RUN] Would apply remote GitHub repo settings to %s", slug)
			return nil
		}
		_ = ApplyGitHubRepoSettings(slug)
		LogSuccess("Repository configuration complete!")
		return nil
	}

	if opts.DryRun {
		LogInfo("[DRY RUN] Would apply remote GitHub repo settings to %s", slug)
		LogInfo("[DRY RUN] Would apply workflow permissions to %s", slug)
		LogInfo("[DRY RUN] Would apply branch protection ruleset to %s", slug)
		LogInfo("[DRY RUN] Would configure local repository at %s (force=%v)", repoRoot, opts.Force)
		if opts.DirectCommit {
			LogInfo("[DRY RUN] Would check git status and commit directly to main at %s if changes exist", repoRoot)
		} else if opts.CreatePR {
			LogInfo("[DRY RUN] Would check git status and create PR on chore/repokit-sync at %s if changes exist", repoRoot)
		}
		return nil
	}

	fmt.Printf("%sConfiguring Repository:%s %s (%s)\n\n", Bold, Reset, slug, repoRoot)

	_ = ApplyGitHubRepoSettings(slug)
	_ = ApplyWorkflowPermissions(slug)
	_ = ApplyBranchRuleset(slug, "~DEFAULT_BRANCH", nil)
	_ = InstallHooks(repoRoot, true)
	_ = ScaffoldTemplates(repoRoot, opts.Force)
	_, _ = ScaffoldArchetypeCI(repoRoot, opts.Force)
	_, _ = ScaffoldReleaseWorkflow(repoRoot, opts.Force)

	if opts.DirectCommit {
		statusOut, _, _, _ := RunCmd([]string{"git", "-C", repoRoot, "status", "--porcelain"}, "")
		if strings.TrimSpace(statusOut) != "" {
			LogInfo("Applying direct governance commit to main for %s...", slug)
			RunCmd([]string{"git", "-C", repoRoot, "checkout", "main"}, "")
			RunCmd([]string{"git", "-C", repoRoot, "add", "-A"}, "")
			refStr := opts.ReleaseRef
			if refStr == "" {
				refStr = "latest"
			}
			commitMsg := fmt.Sprintf("chore(repokit): sync governance standards and workflows to %s", refStr)
			RunCmd([]string{"git", "-C", repoRoot, "commit", "-m", commitMsg}, "")
			_, pushErr, pushCode, _ := RunCmd([]string{"git", "-C", repoRoot, "push", "origin", "main"}, "")
			if pushCode == 0 {
				LogSuccess("Directly committed governance standards to main on %s", slug)
			} else {
				LogWarn("Failed to push direct commit to %s: %s", slug, strings.TrimSpace(pushErr))
			}
		} else {
			LogInfo("No local modifications detected in %s; skipping direct commit.", repoRoot)
		}
	} else if opts.CreatePR {
		statusOut, _, _, _ := RunCmd([]string{"git", "-C", repoRoot, "status", "--porcelain"}, "")
		if strings.TrimSpace(statusOut) != "" {
			LogInfo("Detected modified files in %s, creating PR...", repoRoot)
			_, stderr, code, _ := RunCmd([]string{"git", "-C", repoRoot, "checkout", "-B", "chore/repokit-sync"}, "")
			if code != 0 {
				LogWarn("Failed to checkout branch chore/repokit-sync in %s: %s", repoRoot, strings.TrimSpace(stderr))
			} else {
				_, stderr, code, _ = RunCmd([]string{"git", "-C", repoRoot, "add", "-A"}, "")
				if code != 0 {
					LogWarn("Failed to stage changes in %s: %s", repoRoot, strings.TrimSpace(stderr))
				} else {
					commitMsg := "chore(repokit): configure repository governance, workflows, and templates"
					_, stderr, code, _ = RunCmd([]string{"git", "-C", repoRoot, "commit", "-m", commitMsg}, "")
					if code != 0 {
						LogWarn("Failed to commit changes in %s: %s", repoRoot, strings.TrimSpace(stderr))
					} else {
						_, stderr, code, _ = RunCmd([]string{"git", "-C", repoRoot, "push", "-u", "origin", "chore/repokit-sync", "--force"}, "")
						if code != 0 {
							LogWarn("Failed to push chore/repokit-sync for %s: %s", slug, strings.TrimSpace(stderr))
						}
						prBody := "Automated sync of repository governance, GitHub Actions workflows, and project templates via repokit configure."
						prOut, prErr, prCode, _ := RunCmd([]string{
							"gh", "pr", "create",
							"--title", commitMsg,
							"--body", prBody,
						}, repoRoot)
						if prCode == 0 {
							LogSuccess("Created PR for %s: %s", slug, strings.TrimSpace(prOut))
						} else {
							LogWarn("gh pr create for %s: %s", slug, strings.TrimSpace(prErr))
						}
					}
				}
			}
		} else {
			LogInfo("No local modifications detected in %s; skipping PR creation.", repoRoot)
		}
	}

	fmt.Println()
	LogSuccess("Repository configuration complete!")
	return nil
}

func configureMultipleRepositories(opts ConfigureOptions) error {
	localBaseDir := opts.LocalDir
	if localBaseDir == "" {
		home := os.Getenv("HOME")
		if home == "" {
			if h, err := os.UserHomeDir(); err == nil {
				home = h
			}
		}
		localBaseDir = filepath.Join(home, "Projects")
	}

	var targetSlugs []string
	slugMap := make(map[string]bool)
	slugToLocalPath := make(map[string]string)

	addSlug := func(slug string, localPath string) {
		slug = strings.TrimSpace(slug)
		if slug == "" {
			return
		}
		if !slugMap[slug] {
			slugMap[slug] = true
			targetSlugs = append(targetSlugs, slug)
		}
		if localPath != "" && slugToLocalPath[slug] == "" {
			slugToLocalPath[slug] = localPath
		}
	}

	// 1. Explicit repos list
	if len(opts.Repos) > 0 {
		var user string
		for _, r := range opts.Repos {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			if !strings.Contains(r, "/") {
				if user == "" {
					u, _ := GetGitHubUser()
					user = u
				}
				if user != "" {
					r = fmt.Sprintf("%s/%s", user, r)
				}
			}
			addSlug(r, "")
		}
	}

	// 2. Local repository discovery
	if opts.Local {
		entries, err := os.ReadDir(localBaseDir)
		if err != nil {
			LogWarn("Could not read local directory %s: %v", localBaseDir, err)
		} else {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				repoPath := filepath.Join(localBaseDir, entry.Name())
				if isDir(filepath.Join(repoPath, ".git")) {
					slug, err := GetRepoSlug(repoPath)
					if err == nil && slug != "" {
						addSlug(slug, repoPath)
					}
				}
			}
		}
	}

	// 3. GitHub account discovery
	if opts.All {
		user, err := GetGitHubUser()
		if err != nil || user == "" {
			LogError("Failed to get authenticated GitHub user: %v", err)
			return fmt.Errorf("failed to get github user: %w", err)
		}
		stdout, stderr, code, err := RunCmd([]string{
			"gh", "repo", "list", user, "--limit", "100", "--json", "nameWithOwner",
		}, "")
		if code != 0 {
			LogError("Failed to list repositories for %s: %s", user, strings.TrimSpace(stderr))
			if err != nil {
				return err
			}
			return fmt.Errorf("gh repo list failed: %s", stderr)
		}
		var ghRepos []struct {
			NameWithOwner string `json:"nameWithOwner"`
		}
		if err := json.Unmarshal([]byte(stdout), &ghRepos); err != nil {
			return fmt.Errorf("failed to parse gh repo list output: %w", err)
		}
		for _, r := range ghRepos {
			addSlug(r.NameWithOwner, "")
		}
	}

	if len(targetSlugs) == 0 {
		LogWarn("No repositories found matching configuration criteria.")
		return nil
	}

	LogInfo("Configuring standards across %d repositories...", len(targetSlugs))

	for _, slug := range targetSlugs {
		fmt.Println()
		if IsVaultOrExemptRepo(slug) {
			LogInfo("Skipping branch protection ruleset and CI scaffolding for exempt vault repository %s", slug)
			continue
		}
		LogInfo("Configuring repository standards for %s...", slug)

		// Determine local repository path if exists
		localPath := slugToLocalPath[slug]
		parts := strings.Split(slug, "/")
		repoName := parts[len(parts)-1]

		if localPath == "" {
			candidate := filepath.Join(localBaseDir, repoName)
			if isDir(filepath.Join(candidate, ".git")) {
				localPath = candidate
			}
		}

		if opts.DryRun {
			LogInfo("[DRY RUN] Would apply remote GitHub repo settings to %s", slug)
			LogInfo("[DRY RUN] Would apply workflow permissions to %s", slug)
			LogInfo("[DRY RUN] Would apply branch protection ruleset to %s", slug)
			if localPath != "" {
				LogInfo("[DRY RUN] Would configure local repository at %s (force=%v)", localPath, opts.Force)
				if opts.DirectCommit {
					LogInfo("[DRY RUN] Would check git status and commit directly to main at %s if changes exist", localPath)
				} else if opts.CreatePR {
					LogInfo("[DRY RUN] Would check git status and create PR on chore/repokit-sync at %s if changes exist", localPath)
				}
			}
			continue
		}

		// 1. Remote GitHub settings
		if err := ApplyGitHubRepoSettings(slug); err != nil {
			LogWarn("Failed to apply GitHub repo settings for %s: %v", slug, err)
		}
		if err := ApplyWorkflowPermissions(slug); err != nil {
			LogWarn("Failed to apply workflow permissions for %s: %v", slug, err)
		}
		if err := ApplyBranchRuleset(slug, "~DEFAULT_BRANCH", nil); err != nil {
			LogWarn("Failed to apply branch ruleset for %s: %v", slug, err)
		}

		// Ephemeral clone for remote repos discovered without existing local clone
		if localPath == "" {
			tempDir := filepath.Join(os.TempDir(), "repokit-sync", repoName)
			_ = os.RemoveAll(tempDir)
			_ = os.MkdirAll(filepath.Dir(tempDir), 0755)
			defer os.RemoveAll(tempDir)
			_, cloneErr, cloneCode, _ := RunCmd([]string{"gh", "repo", "clone", slug, tempDir, "--", "--depth", "1"}, "")
			if cloneCode == 0 {
				localPath = tempDir
			} else {
				LogWarn("Failed to clone %s ephemerally: %s", slug, strings.TrimSpace(cloneErr))
			}
		}

		// 2. Local repository configuration & PR creation / direct commit
		if localPath != "" {
			if err := ConfigureRepository(localPath, opts.Force); err != nil {
				LogWarn("Failed to configure local repository at %s: %v", localPath, err)
			}

			statusOut, _, _, _ := RunCmd([]string{"git", "-C", localPath, "status", "--porcelain"}, "")
			if strings.TrimSpace(statusOut) != "" {
				if opts.DirectCommit {
					LogInfo("Applying direct governance commit to main for %s...", slug)
					RunCmd([]string{"git", "-C", localPath, "checkout", "main"}, "")
					RunCmd([]string{"git", "-C", localPath, "add", "-A"}, "")
					refStr := opts.ReleaseRef
					if refStr == "" {
						refStr = "latest"
					}
					commitMsg := fmt.Sprintf("chore(repokit): sync governance standards and workflows to %s", refStr)
					RunCmd([]string{"git", "-C", localPath, "commit", "-m", commitMsg}, "")
					_, pushErr, pushCode, _ := RunCmd([]string{"git", "-C", localPath, "push", "origin", "main"}, "")
					if pushCode == 0 {
						LogSuccess("Directly committed governance standards to main on %s", slug)
					} else {
						LogWarn("Failed to push direct commit to %s: %s", slug, strings.TrimSpace(pushErr))
					}
				} else if opts.CreatePR {
					LogInfo("Detected modified files in %s, creating PR...", localPath)
					_, stderr, code, _ := RunCmd([]string{"git", "-C", localPath, "checkout", "-B", "chore/repokit-sync"}, "")
					if code != 0 {
						LogWarn("Failed to checkout branch chore/repokit-sync in %s: %s", localPath, strings.TrimSpace(stderr))
						continue
					}
					_, stderr, code, _ = RunCmd([]string{"git", "-C", localPath, "add", "-A"}, "")
					if code != 0 {
						LogWarn("Failed to stage changes in %s: %s", localPath, strings.TrimSpace(stderr))
						continue
					}
					commitMsg := "chore(repokit): configure latest governance, workflows, and templates"
					_, stderr, code, _ = RunCmd([]string{"git", "-C", localPath, "commit", "-m", commitMsg}, "")
					if code != 0 {
						LogWarn("Failed to commit changes in %s: %s", localPath, strings.TrimSpace(stderr))
						continue
					}
					_, stderr, code, _ = RunCmd([]string{"git", "-C", localPath, "push", "-u", "origin", "chore/repokit-sync", "--force"}, "")
					if code != 0 {
						LogWarn("Failed to push chore/repokit-sync for %s: %s", slug, strings.TrimSpace(stderr))
					}
					prBody := "Automated sync of repository governance, GitHub Actions workflows, and project templates via repokit configuration engine."
					prOut, prErr, prCode, _ := RunCmd([]string{
						"gh", "pr", "create",
						"--title", commitMsg,
						"--body", prBody,
					}, localPath)
					if prCode == 0 {
						LogSuccess("Created PR for %s: %s", slug, strings.TrimSpace(prOut))
					} else {
						LogWarn("gh pr create for %s: %s", slug, strings.TrimSpace(prErr))
					}
				}
			} else {
				if opts.DirectCommit {
					LogInfo("No local modifications detected in %s; skipping direct commit.", localPath)
				} else if opts.CreatePR {
					LogInfo("No local modifications detected in %s; skipping PR creation.", localPath)
				}
			}
		}
	}

	fmt.Println()
	LogSuccess("Repository configuration complete across %d repositories!", len(targetSlugs))
	return nil
}

// Release Automation Types & Engine
type ReleaseOptions struct {
	Path          string
	TargetBranch  string // defaults to "main"
	StagingBranch string // defaults to DefaultStagingBranch ("release-staging")
	DryRun        bool
	OutputFile    string // defaults to os.Getenv("GITHUB_OUTPUT")
}

type ReleaseResult struct {
	ReleasesCreated bool
	TagName         string
	Version         string
	Major           int
	Minor           int
	Patch           int
	ChangelogEntry  string
}

// ParseSemVer parses a semver tag (e.g. "v1.2.3" or "1.2.3") into major, minor, patch integers.
func ParseSemVer(tag string) (major, minor, patch int, ok bool) {
	tag = strings.TrimSpace(tag)
	re := regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)$`)
	m := re.FindStringSubmatch(tag)
	if len(m) != 4 {
		return 0, 0, 0, false
	}
	var err error
	major, err = strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err = strconv.Atoi(m[2])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err = strconv.Atoi(m[3])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

// CalculateNextVersion calculates the next semver version based on Conventional Commits since currentTag.
func CalculateNextVersion(currentTag string, commits []string) (major, minor, patch int, bumped bool) {
	curMajor, curMinor, curPatch, _ := ParseSemVer(currentTag)

	hasBreaking := false
	hasFeat := false
	hasFix := false

	releaseRegex := regexp.MustCompile(`^(?:chore\((?:release|main|staging)\): release|release(?:\([^\)]+\))?!?: (?:candidate )?release)`)
	commitHeaderRegex := regexp.MustCompile(`^([a-zA-Z0-9_-]+)(?:\(([^\)]+)\))?(!)?:\s*(.+)$`)

	for _, c := range commits {
		line := strings.TrimSpace(c)
		if line == "" {
			continue
		}
		subject := line
		if idx := strings.Index(line, "|"); idx != -1 {
			subject = strings.TrimSpace(line[idx+1:])
		}
		if releaseRegex.MatchString(subject) {
			continue
		}

		isBreaking := false
		if strings.Contains(line, "BREAKING CHANGE:") || strings.Contains(line, "BREAKING-CHANGE:") {
			isBreaking = true
		}

		match := commitHeaderRegex.FindStringSubmatch(subject)
		if match != nil {
			if match[3] == "!" {
				isBreaking = true
			}
		}

		if isBreaking {
			hasBreaking = true
		} else if match != nil {
			typ := strings.ToLower(match[1])
			switch typ {
			case "feat":
				hasFeat = true
			case "fix", "perf", "refactor", "ci", "build":
				hasFix = true
			}
		}
	}

	if !hasBreaking && !hasFeat && !hasFix {
		return curMajor, curMinor, curPatch, false
	}

	if currentTag == "" || (curMajor == 0 && curMinor == 0 && curPatch == 0) {
		return 1, 0, 0, true
	}

	if hasBreaking {
		return curMajor + 1, 0, 0, true
	}
	if hasFeat {
		return curMajor, curMinor + 1, 0, true
	}
	if hasFix {
		return curMajor, curMinor, curPatch + 1, true
	}

	return curMajor, curMinor, curPatch, false
}

// FormatChangelogSection formats a markdown section for a release version.
func FormatChangelogSection(version string, dateStr string, commits []string) string {
	releaseRegex := regexp.MustCompile(`^(?:chore\((?:release|main|staging)\): release|release(?:\([^\)]+\))?!?: (?:candidate )?release)`)
	commitHeaderRegex := regexp.MustCompile(`^([a-zA-Z0-9_-]+)(?:\(([^\)]+)\))?(!)?:\s*(.+)$`)

	var breaking []string
	var features []string
	var bugFixes []string
	var perfRefactor []string
	var maintenance []string

	for _, c := range commits {
		line := strings.TrimSpace(c)
		if line == "" {
			continue
		}
		var sha, subject string
		if idx := strings.Index(line, "|"); idx != -1 {
			sha = strings.TrimSpace(line[:idx])
			subject = strings.TrimSpace(line[idx+1:])
		} else {
			subject = line
		}

		if releaseRegex.MatchString(subject) {
			continue
		}

		isBreaking := false
		if strings.Contains(line, "BREAKING CHANGE:") || strings.Contains(line, "BREAKING-CHANGE:") {
			isBreaking = true
		}

		match := commitHeaderRegex.FindStringSubmatch(subject)
		var scope, desc string
		var typ string
		if match != nil {
			typ = strings.ToLower(match[1])
			scope = match[2]
			if match[3] == "!" {
				isBreaking = true
			}
			desc = match[4]
		} else {
			desc = subject
		}

		var bullet string
		if scope != "" {
			if sha != "" {
				bullet = fmt.Sprintf("* **%s:** %s (%s)", scope, desc, sha)
			} else {
				bullet = fmt.Sprintf("* **%s:** %s", scope, desc)
			}
		} else {
			if sha != "" {
				bullet = fmt.Sprintf("* %s (%s)", desc, sha)
			} else {
				bullet = fmt.Sprintf("* %s", desc)
			}
		}

		if isBreaking {
			breaking = append(breaking, bullet)
		} else {
			switch typ {
			case "feat":
				features = append(features, bullet)
			case "fix":
				bugFixes = append(bugFixes, bullet)
			case "perf", "refactor":
				perfRefactor = append(perfRefactor, bullet)
			default:
				maintenance = append(maintenance, bullet)
			}
		}
	}

	var sections []string
	if len(breaking) > 0 {
		sections = append(sections, "### Breaking Changes\n\n"+strings.Join(breaking, "\n"))
	}
	if len(features) > 0 {
		sections = append(sections, "### Features\n\n"+strings.Join(features, "\n"))
	}
	if len(bugFixes) > 0 {
		sections = append(sections, "### Bug Fixes\n\n"+strings.Join(bugFixes, "\n"))
	}
	if len(perfRefactor) > 0 {
		sections = append(sections, "### Performance & Refactoring\n\n"+strings.Join(perfRefactor, "\n"))
	}
	if len(maintenance) > 0 {
		sections = append(sections, "### Maintenance & CI\n\n"+strings.Join(maintenance, "\n"))
	}

	header := fmt.Sprintf("## %s (%s)\n\n", version, dateStr)
	if len(sections) == 0 {
		return fmt.Sprintf("## %s (%s)\n", version, dateStr)
	}
	return header + strings.Join(sections, "\n\n") + "\n"
}

// extractReleaseNotes extracts the release notes for a given version from CHANGELOG content.
func extractReleaseNotes(content string, version string) string {
	vTarget := strings.TrimPrefix(version, "v")
	reHeader := regexp.MustCompile(`^## (?:\[)?v?` + regexp.QuoteMeta(vTarget) + `(?:\b|[ \(\[])`)
	reAnySectionHeader := regexp.MustCompile(`^## `)

	lines := strings.Split(content, "\n")
	startIdx := -1
	endIdx := -1

	for i, line := range lines {
		if startIdx == -1 {
			if reHeader.MatchString(line) {
				startIdx = i
			}
		} else {
			if reAnySectionHeader.MatchString(line) {
				endIdx = i
				break
			}
		}
	}

	if startIdx != -1 {
		if endIdx == -1 {
			endIdx = len(lines)
		}
		if startIdx+1 <= endIdx {
			notes := strings.TrimSpace(strings.Join(lines[startIdx+1:endIdx], "\n"))
			if notes != "" {
				return notes
			}
		}
	}
	return ""
}

// UpdateChangelogContent prepends newSection under # Changelog or replaces existing version block.
func UpdateChangelogContent(existing string, version string, newSection string) string {
	cleanNew := strings.TrimSpace(newSection)
	vTarget := strings.TrimPrefix(version, "v")

	if strings.TrimSpace(existing) == "" {
		return "# Changelog\n\n" + cleanNew + "\n"
	}

	lines := strings.Split(existing, "\n")
	startIdx := -1
	endIdx := -1

	reHeader := regexp.MustCompile(`^## (?:\[)?v?` + regexp.QuoteMeta(vTarget) + `(?:\b|[ \(\[])`)
	reAnySectionHeader := regexp.MustCompile(`^## `)

	for i, line := range lines {
		if startIdx == -1 {
			if reHeader.MatchString(line) {
				startIdx = i
			}
		} else {
			if reAnySectionHeader.MatchString(line) {
				endIdx = i
				break
			}
		}
	}

	if startIdx != -1 {
		if endIdx == -1 {
			endIdx = len(lines)
		}
		var newLines []string
		newLines = append(newLines, lines[:startIdx]...)
		newLines = append(newLines, cleanNew)
		if endIdx < len(lines) {
			newLines = append(newLines, "")
			newLines = append(newLines, lines[endIdx:]...)
		}
		result := strings.Join(newLines, "\n")
		return strings.TrimRight(result, "\n") + "\n"
	}

	reChangelog := regexp.MustCompile(`(?m)^# Changelog\s*$`)
	loc := reChangelog.FindStringIndex(existing)
	if loc != nil {
		before := strings.TrimRight(existing[:loc[1]], "\n")
		after := strings.TrimLeft(existing[loc[1]:], "\n")
		if after != "" {
			return before + "\n\n" + cleanNew + "\n\n" + after
		}
		return before + "\n\n" + cleanNew + "\n"
	}

	return "# Changelog\n\n" + cleanNew + "\n\n" + strings.TrimSpace(existing) + "\n"
}

func writeGitHubOutput(path string, kvs map[string]string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	keys := make([]string, 0, len(kvs))
	for k := range kvs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(f, "%s=%s\n", k, kvs[k]); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteRelease runs either Mode A (Release Promotion) or Mode B (Release Staging).
func ExecuteRelease(opts ReleaseOptions) (*ReleaseResult, error) {
	repoRoot, err := GetGitRepoRoot(opts.Path)
	if err != nil || repoRoot == "" {
		if err == nil {
			err = fmt.Errorf("not inside a git repository")
		}
		return nil, err
	}

	if opts.TargetBranch == "" {
		opts.TargetBranch = "main"
	}
	if opts.StagingBranch == "" {
		opts.StagingBranch = DefaultStagingBranch
	}
	if opts.OutputFile == "" {
		opts.OutputFile = os.Getenv("GITHUB_OUTPUT")
	}

	headSubject, _, code, err := RunCmd([]string{"git", "log", "-1", "--format=%s"}, repoRoot)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	headSubject = strings.TrimSpace(headSubject)

	reModeA := regexp.MustCompile(`^(?:chore\((?:release|main|staging)\): release|release(?:\([^\)]+\))?!?: (?:candidate )?release) v?(\d+\.\d+\.\d+)`)
	reCandidate := regexp.MustCompile(`^release(?:\(v?(\d+\.\d+\.\d+)\))?!?: candidate release`)

	var promoVersion string
	if matchA := reModeA.FindStringSubmatch(headSubject); matchA != nil {
		promoVersion = matchA[1]
	} else if matchCand := reCandidate.FindStringSubmatch(headSubject); matchCand != nil && len(matchCand) > 1 && matchCand[1] != "" {
		promoVersion = matchCand[1]
	}

	if promoVersion != "" {
		// Mode A: Release Promotion
		version := promoVersion
		major, minor, patch, ok := ParseSemVer(promoVersion)
		if ok {
			version = fmt.Sprintf("%d.%d.%d", major, minor, patch)
		}
		tagName := "v" + version

		notes := "Release " + tagName
		changelogPath := filepath.Join(repoRoot, "CHANGELOG.md")
		if data, err := os.ReadFile(changelogPath); err == nil {
			if extracted := extractReleaseNotes(string(data), version); extracted != "" {
				notes = extracted
			}
		}

		if !opts.DryRun {
			_, _, tagCode, _ := RunCmd([]string{"git", "rev-parse", "-q", "--verify", "refs/tags/" + tagName}, repoRoot)
			if tagCode != 0 {
				RunCmd([]string{"git", "config", "user.name", "github-actions[bot]"}, repoRoot)
				RunCmd([]string{"git", "config", "user.email", "41898282+github-actions[bot]@users.noreply.github.com"}, repoRoot)
				if _, _, code, err := RunCmd([]string{"git", "tag", "-a", tagName, "-m", "Release " + tagName}, repoRoot); err != nil || code != 0 {
					return nil, fmt.Errorf("failed to create git tag %s: %w", tagName, err)
				}
				if _, _, code, err := RunCmd([]string{"git", "push", "origin", tagName}, repoRoot); err != nil || code != 0 {
					return nil, fmt.Errorf("failed to push git tag %s: %w", tagName, err)
				}
				majorTag := fmt.Sprintf("v%d", major)
				RunCmd([]string{"git", "tag", "-f", "-a", majorTag, "-m", "Release " + majorTag}, repoRoot)
				RunCmd([]string{"git", "push", "--force", "origin", majorTag}, repoRoot)
			}

			_, _, relCode, _ := RunCmd([]string{"gh", "release", "view", tagName}, repoRoot)
			if relCode != 0 {
				if _, _, code, err := RunCmd([]string{"gh", "release", "create", tagName, "--title", tagName, "--notes", notes}, repoRoot); err != nil || code != 0 {
					return nil, fmt.Errorf("failed to create GitHub release %s: %w", tagName, err)
				}
			}
		}

		if opts.OutputFile != "" {
			outputs := map[string]string{
				"releases_created": "true",
				"tag_name":         tagName,
				"version":          version,
				"major":            strconv.Itoa(major),
				"minor":            strconv.Itoa(minor),
				"patch":            strconv.Itoa(patch),
			}
			if err := writeGitHubOutput(opts.OutputFile, outputs); err != nil {
				return nil, err
			}
		}

		return &ReleaseResult{
			ReleasesCreated: true,
			TagName:         tagName,
			Version:         version,
			Major:           major,
			Minor:           minor,
			Patch:           patch,
			ChangelogEntry:  notes,
		}, nil
	}

	// Mode B: Release Staging to stagingBranch
	tagOut, _, _, _ := RunCmd([]string{"git", "tag", "--list", "v*", "--sort=-v:refname"}, repoRoot)
	var latestTag string
	for _, line := range strings.Split(tagOut, "\n") {
		t := strings.TrimSpace(line)
		if _, _, _, ok := ParseSemVer(t); ok {
			latestTag = t
			break
		}
	}

	var logArgs []string
	if latestTag != "" {
		logArgs = []string{"git", "log", fmt.Sprintf("%s..HEAD", latestTag), "--format=%h|%s"}
	} else {
		logArgs = []string{"git", "log", "--format=%h|%s"}
	}

	logOut, _, code, err := RunCmd(logArgs, repoRoot)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("failed to query git log: %w", err)
	}

	var commits []string
	for _, line := range strings.Split(logOut, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			commits = append(commits, line)
		}
	}

	major, minor, patch, bumped := CalculateNextVersion(latestTag, commits)
	if !bumped {
		LogInfo("No releasable changes detected.")
		if opts.OutputFile != "" {
			_ = writeGitHubOutput(opts.OutputFile, map[string]string{
				"releases_created": "false",
			})
		}
		return &ReleaseResult{ReleasesCreated: false}, nil
	}

	version := fmt.Sprintf("%d.%d.%d", major, minor, patch)
	tagName := "v" + version
	dateStr := time.Now().UTC().Format("2006-01-02")
	changelogSection := FormatChangelogSection(version, dateStr, commits)

	changelogPath := filepath.Join(repoRoot, "CHANGELOG.md")
	var existing string
	if data, err := os.ReadFile(changelogPath); err == nil {
		existing = string(data)
	}
	updatedChangelog := UpdateChangelogContent(existing, version, changelogSection)

	if !opts.DryRun {
		RunCmd([]string{"git", "config", "user.name", "github-actions[bot]"}, repoRoot)
		RunCmd([]string{"git", "config", "user.email", "41898282+github-actions[bot]@users.noreply.github.com"}, repoRoot)

		if _, _, code, err := RunCmd([]string{"git", "checkout", "-B", opts.StagingBranch}, repoRoot); err != nil || code != 0 {
			return nil, fmt.Errorf("failed to checkout staging branch %s: %w", opts.StagingBranch, err)
		}

		if err := os.WriteFile(changelogPath, []byte(updatedChangelog), 0644); err != nil {
			return nil, fmt.Errorf("failed to write CHANGELOG.md: %w", err)
		}

		if _, _, code, err := RunCmd([]string{"git", "add", "CHANGELOG.md"}, repoRoot); err != nil || code != 0 {
			return nil, fmt.Errorf("failed to git add CHANGELOG.md: %w", err)
		}

		commitMsg := fmt.Sprintf("release(%s): candidate release", tagName)
		if _, _, code, err := RunCmd([]string{"git", "commit", "-m", commitMsg}, repoRoot); err != nil || code != 0 {
			return nil, fmt.Errorf("failed to commit changelog update: %w", err)
		}

		if _, _, code, err := RunCmd([]string{"git", "push", "--force", "origin", opts.StagingBranch}, repoRoot); err != nil || code != 0 {
			return nil, fmt.Errorf("failed to push to %s: %w", opts.StagingBranch, err)
		}

		prOut, _, _, _ := RunCmd([]string{"gh", "pr", "list", "--head", opts.StagingBranch, "--base", opts.TargetBranch, "--state", "open", "--json", "number", "--jq", ".[0].number"}, repoRoot)
		prTitle := fmt.Sprintf("release(%s): candidate release", tagName)
		prBody := fmt.Sprintf("## Release Staging: %s\n\n%s\n---\n\n## Quality Gates & Sign-Off Checklist\n- [ ] **Gate 1: Automated Quality Gates** (All CI checks pass)\n- [ ] **Gate 2: Agent Review Verification** (Adversarial diff review conducted; no regressions)\n- [ ] **Gate 3: Human Review & Manual Promotion** (PR reviewed by human developer; merge must only be triggered manually in the GitHub UI, never automated)\n", tagName, changelogSection)

		prNum := strings.TrimSpace(prOut)
		if prNum != "" && prNum != "null" {
			if _, _, code, err := RunCmd([]string{"gh", "pr", "edit", prNum, "--title", prTitle, "--body", prBody}, repoRoot); err != nil || code != 0 {
				return nil, fmt.Errorf("failed to edit PR #%s: %w", prNum, err)
			}
		} else {
			if _, _, code, err := RunCmd([]string{"gh", "pr", "create", "--base", opts.TargetBranch, "--head", opts.StagingBranch, "--title", prTitle, "--body", prBody}, repoRoot); err != nil || code != 0 {
				return nil, fmt.Errorf("failed to create release PR: %w", err)
			}
		}

		stagingSha, _, _, _ := RunCmd([]string{"git", "rev-parse", "HEAD"}, repoRoot)
		stagingSha = strings.TrimSpace(stagingSha)

		slug, _ := GetRepoSlug(repoRoot)
		if slug != "" && stagingSha != "" {
			requiredContexts := []string{
				"Quality Gate / Verification",
				"Quality Gate / Title Validation",
				"Test & Coverage",
				"Validate Conventional Commit PR Title",
				"ci / Test & Coverage",
				"validate-pr / Validate Conventional Commit PR Title",
			}
			for _, ctx := range requiredContexts {
				RunCmd([]string{
					"gh", "api", "--method", "POST",
					fmt.Sprintf("repos/%s/statuses/%s", slug, stagingSha),
					"-f", "state=success",
					"-f", "context=" + ctx,
					"-f", "description=Release staging pre-certified",
				}, repoRoot)
			}
		}

		RunCmd([]string{"gh", "workflow", "run", "semantic-pr.yml", "--ref", opts.StagingBranch}, repoRoot)
		RunCmd([]string{"gh", "workflow", "run", "test.yml", "--ref", opts.StagingBranch}, repoRoot)
		RunCmd([]string{"git", "checkout", opts.TargetBranch}, repoRoot)
	}

	if opts.OutputFile != "" {
		outputs := map[string]string{
			"releases_created": "false",
			"tag_name":         tagName,
			"version":          version,
			"major":            strconv.Itoa(major),
			"minor":            strconv.Itoa(minor),
			"patch":            strconv.Itoa(patch),
		}
		if err := writeGitHubOutput(opts.OutputFile, outputs); err != nil {
			return nil, err
		}
	}

	return &ReleaseResult{
		ReleasesCreated: false,
		TagName:         tagName,
		Version:         version,
		Major:           major,
		Minor:           minor,
		Patch:           patch,
		ChangelogEntry:  changelogSection,
	}, nil
}
