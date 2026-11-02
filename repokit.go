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
	"strings"
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
	CommitPattern         = `^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-zA-Z0-9_.-]+\))?!?: .+$`

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
PATTERN="^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-zA-Z0-9_.-]+\))?!?: .+$"

if ! echo "$COMMIT_MSG" | grep -Eq "$PATTERN"; then
    echo -e "\033[1;31m[ERROR] Commit rejected: Message does not follow Conventional Commits.\033[0m"
    echo -e "\033[1;33mExpected:\033[0m <type>(<scope optional>): <description>"
    echo -e "\033[1;33mAllowed Types:\033[0m feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert"
    echo -e "\033[1;33mExamples:\033[0m"
    echo "  feat(laser): add pointcloud downsampling"
    echo "  fix(launch): correct topic remapping in simulation"
    echo "  docs(readme): add setup and launch instructions"
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
          requireScope: false
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
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("failed to get github user: %s", strings.TrimSpace(stderr))
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

func ApplyBranchRuleset(repoSlug string, defaultBranch string, requiredChecks []string) error {
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
		if strings.HasSuffix(repoSlug, "/repokit") || repoSlug == "repokit" {
			requiredChecks = []string{"Test & Coverage", "Validate Conventional Commit PR Title"}
		} else {
			requiredChecks = []string{"ci / Test & Coverage", "validate-pr / Validate Conventional Commit PR Title"}
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
		LogInfo("Updating existing ruleset (ID: %v)...", existingID)
		cmdArgs = []string{"gh", "api", "--method", "PUT", fmt.Sprintf("repos/%s/rulesets/%v", repoSlug, existingID), "--input", "-"}
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
	if strings.Contains(content, "validate-pr:") &&
		!strings.Contains(content, "github.event_name == 'pull_request'") &&
		!strings.Contains(content, "github.event_name == \"pull_request\"") {

		lines := strings.Split(content, "\n")
		var newLines []string
		modified := false
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
			newContent := strings.Join(newLines, "\n")
			if err := os.WriteFile(ciFile, []byte(newContent), 0644); err != nil {
				return false, err
			}
			LogSuccess("Patched existing CI workflow with PR event guard (.github/workflows/ci.yml)")
			return true, nil
		}
	}
	return false, nil
}

func ScaffoldArchetypeCI(repoRoot string, force bool) (bool, error) {
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
							stdoutDetail, _, codeDetail, _ := RunCmd([]string{"gh", "api", fmt.Sprintf("repos/%s/rulesets/%v", slug, idVal)}, "")
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
											rulesetHasStatusChecks = len(reqChecks) > 0
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
		isGovernanceHub := (fileExists(filepath.Join(repoRoot, ".github", "workflows", "reusable-python.yml")) ||
			fileExists(filepath.Join(repoRoot, ".github", "workflows", "reusable-go.yml"))) &&
			(fileExists(filepath.Join(repoRoot, "bin", "repokit")) ||
				fileExists(filepath.Join(repoRoot, "cmd", "repokit", "main.go")) ||
				fileExists(filepath.Join(repoRoot, "repokit.go")))

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

		localItems = []CheckItem{
			{Label: "Local Conventional Commit Hook Installed (.git/hooks/commit-msg)", Passed: hasHook},
			{Label: "Local Pre-commit Hook Installed (.git/hooks/pre-commit)", Passed: hasPreHook},
			{Label: "Pull Request Template (.github/pull_request_template.md)", Passed: hasPRTemplate},
			{Label: ciLabel, Passed: hasCIWorkflow},
			{Label: "CI Workflow PR Event Guard (skips PR check on main push)", Passed: hasPRGuard},
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

	fmt.Printf("%sConfiguring Repository:%s %s (%s)\n\n", Bold, Reset, slug, repoRoot)

	_ = ApplyGitHubRepoSettings(slug)
	_ = ApplyBranchRuleset(slug, "~DEFAULT_BRANCH", nil)
	_ = InstallHooks(repoRoot, true)
	_ = ScaffoldTemplates(repoRoot, force)
	_, _ = ScaffoldArchetypeCI(repoRoot, force)

	fmt.Println()
	LogSuccess("Repository configuration complete!")
	return nil
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
	_ = InstallHooks(absPath, true)

	RunCmd([]string{"git", "add", "."}, absPath)
	RunCmd([]string{"git", "commit", "-m", "chore: initial repository scaffolding"}, absPath)

	LogInfo("Pushing initial commit to main...")
	_, pushErr, pushCode, _ := RunCmd([]string{"git", "push", "-u", "origin", "main"}, absPath)
	if pushCode != 0 {
		LogWarn("Initial push: %s", strings.TrimSpace(pushErr))
	}

	_ = ApplyGitHubRepoSettings(slug)
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
