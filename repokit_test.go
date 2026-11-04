package repokit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestConventionalCommits(t *testing.T) {
	re := regexp.MustCompile(CommitPattern)

	valid := []string{
		"feat: add laser filter node",
		"fix: prevent null pointer exception",
		"docs: update installation instructions",
		"style: reformat according to PEP8",
		"refactor: extract math utilities",
		"perf: optimize pointcloud downsampling",
		"test: add unit tests for quaternion conversion",
		"build: update dependencies in setup.py",
		"ci: add semantic-pr workflow",
		"chore: bump version number",
		"revert: revert previous commit",
		"feat(nav): add goal pose callback",
		"fix(laser-driver)!: breaking protocol change",
		"chore(deps): update ros2 packages",
	}

	for _, msg := range valid {
		t.Run("valid/"+msg, func(t *testing.T) {
			if !re.MatchString(msg) {
				t.Errorf("Expected valid commit message to match pattern: %s", msg)
			}
		})
	}

	invalid := []string{
		"WIP",
		"fixed a bug",
		"Added new feature",
		"feat:no space after colon",
		"feature: wrong type name",
		"fix(nav):",
		"chore(): empty scope",
		"Update README.md",
	}

	for _, msg := range invalid {
		t.Run("invalid/"+msg, func(t *testing.T) {
			if re.MatchString(msg) {
				t.Errorf("Expected invalid commit message NOT to match pattern: %s", msg)
			}
		})
	}
}

func TestCommitHookExecution(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	if err := InstallCommitHook(tmpDir); err != nil {
		t.Fatalf("InstallCommitHook failed: %v", err)
	}

	hookPath := filepath.Join(tmpDir, ".git", "hooks", "commit-msg")
	if !fileExists(hookPath) {
		t.Fatalf("commit-msg hook does not exist at %s", hookPath)
	}
	if !isExecutable(hookPath) {
		t.Fatalf("commit-msg hook is not executable")
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	cmdBad := exec.Command("git", "commit", "-m", "bad commit")
	cmdBad.Dir = tmpDir
	if err := cmdBad.Run(); err == nil {
		t.Errorf("Bad commit should have been rejected by hook")
	}

	cmdGood := exec.Command("git", "commit", "-m", "feat(core): initial test commit")
	cmdGood.Dir = tmpDir
	if out, err := cmdGood.CombinedOutput(); err != nil {
		t.Errorf("Valid commit failed: %v\nOutput: %s", err, string(out))
	}
}

func TestHookInstallPreCommitAndCommitMsg(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	if err := InstallHooks(tmpDir, true); err != nil {
		t.Fatalf("InstallHooks(tmpDir, true) failed: %v", err)
	}

	commitHook := filepath.Join(tmpDir, ".git", "hooks", "commit-msg")
	preHook := filepath.Join(tmpDir, ".git", "hooks", "pre-commit")

	if !fileExists(commitHook) {
		t.Errorf("commit-msg hook should exist")
	}
	if !isExecutable(commitHook) {
		t.Errorf("commit-msg hook should be executable")
	}

	if !fileExists(preHook) {
		t.Errorf("pre-commit hook should exist")
	}
	if !isExecutable(preHook) {
		t.Errorf("pre-commit hook should be executable")
	}
}

func TestArchetypeScaffold(t *testing.T) {
	archetypes := []struct {
		name         string
		expectedFile string
	}{
		{"python", "pyproject.toml"},
		{"java", "pom.xml"},
		{"typescript", "package.json"},
		{"bash", "install.sh"},
		{"go", "go.mod"},
	}

	for _, a := range archetypes {
		t.Run(a.name, func(t *testing.T) {
			target := t.TempDir()
			vars := map[string]string{
				"repo_name":       "sample-app",
				"package_name":    "sample_app",
				"description":     "Sample description",
				"author":          "Cooper Wallace",
				"year":            "2026",
				"repo_slug":       "Cooper-Wallace/sample-app",
				"governance_slug": DefaultGovernanceRepo,
			}

			err := ApplyArchetypeTemplate(a.name, target, vars)
			if err != nil {
				t.Fatalf("ApplyArchetypeTemplate failed for %s: %v", a.name, err)
			}

			checkPath := filepath.Join(target, a.expectedFile)
			if !fileExists(checkPath) {
				t.Errorf("Expected file %s does not exist in %s", a.expectedFile, target)
			}

			ciPath := filepath.Join(target, ".github", "workflows", "ci.yml")
			if !fileExists(ciPath) {
				t.Errorf("Expected .github/workflows/ci.yml in %s", target)
			} else {
				ciContent, _ := os.ReadFile(ciPath)
				if !strings.Contains(string(ciContent), DefaultGovernanceRepo) {
					t.Errorf("ci.yml does not contain DefaultGovernanceRepo")
				}
				if strings.Contains(string(ciContent), "{{governance_slug}}") {
					t.Errorf("ci.yml still contains {{governance_slug}} placeholder")
				}
			}

			checks := AuditArchetypeStructure(target, a.name)
			for _, c := range checks {
				if !c.Passed {
					t.Errorf("Check failed for %s: %s", a.name, c.Label)
				}
			}
		})
	}
}

func TestCustomGovernanceSlug(t *testing.T) {
	target := t.TempDir()
	customSlug := "custom-org/custom-repokit"
	vars := map[string]string{
		"repo_name":       "custom-pkg",
		"package_name":    "custom_pkg",
		"description":     "Custom pkg",
		"author":          "Cooper Wallace",
		"year":            "2026",
		"repo_slug":       "custom-org/custom-pkg",
		"governance_slug": customSlug,
	}

	err := ApplyArchetypeTemplate("python", target, vars)
	if err != nil {
		t.Fatalf("ApplyArchetypeTemplate failed: %v", err)
	}

	ciPath := filepath.Join(target, ".github", "workflows", "ci.yml")
	contentBytes, err := os.ReadFile(ciPath)
	if err != nil {
		t.Fatalf("failed to read ci.yml: %v", err)
	}
	content := string(contentBytes)
	if !strings.Contains(content, customSlug) {
		t.Errorf("ci.yml does not contain custom governance slug: %s", customSlug)
	}
	if strings.Contains(content, "{{governance_slug}}") {
		t.Errorf("ci.yml still contains {{governance_slug}} placeholder")
	}
}

func TestArchetypeAuditStructure(t *testing.T) {
	archetypes := []string{"python", "java", "typescript", "bash", "go"}
	for _, arch := range archetypes {
		t.Run("detect_and_audit_"+arch, func(t *testing.T) {
			target := t.TempDir()
			vars := map[string]string{
				"repo_name":    "test-" + arch,
				"package_name": "test_" + arch,
			}
			if err := ApplyArchetypeTemplate(arch, target, vars); err != nil {
				t.Fatalf("failed to apply %s: %v", arch, err)
			}

			detected := DetectArchetype(target)
			if detected != arch {
				t.Errorf("Expected detected archetype '%s', got '%s'", arch, detected)
			}

			checks := AuditArchetypeStructure(target, "")
			if len(checks) == 0 {
				t.Fatalf("AuditArchetypeStructure returned 0 checks for %s", arch)
			}
			for _, check := range checks {
				if !check.Passed {
					t.Errorf("Structure check failed for %s: %s", arch, check.Label)
				}
			}
		})
	}
}

func TestPatchCIWorkflowPRGuard(t *testing.T) {
	target := t.TempDir()
	ciFile := filepath.Join(target, "ci.yml")
	initialContent := `name: CI

on:
  pull_request:
  push:

jobs:
  validate-pr:
    uses: Cooper-Wallace/repokit/.github/workflows/semantic-pr.yml@main

  test:
    runs-on: ubuntu-latest
    steps:
      - run: pytest
`
	if err := os.WriteFile(ciFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write test ci.yml: %v", err)
	}

	// First run: should inject guard and return true
	patched, err := PatchCIWorkflowPRGuard(ciFile)
	if err != nil {
		t.Fatalf("PatchCIWorkflowPRGuard failed: %v", err)
	}
	if !patched {
		t.Errorf("Expected first run to patch workflow, got false")
	}

	contentBytes, _ := os.ReadFile(ciFile)
	content := string(contentBytes)
	if !strings.Contains(content, "if: github.event_name == 'pull_request'") {
		t.Errorf("Patched workflow missing PR event guard: %s", content)
	}

	// Second run: should be idempotent and return false
	patched2, err := PatchCIWorkflowPRGuard(ciFile)
	if err != nil {
		t.Fatalf("Second PatchCIWorkflowPRGuard failed: %v", err)
	}
	if patched2 {
		t.Errorf("Expected second run to be idempotent (false), got true")
	}

	contentBytes2, _ := os.ReadFile(ciFile)
	if string(contentBytes2) != content {
		t.Errorf("File changed on second run")
	}

	t.Run("patch_inline_ci_job_name", func(t *testing.T) {
		inlineTarget := t.TempDir()
		inlineCI := filepath.Join(inlineTarget, "ci.yml")
		inlineContent := `name: CI

on:
  pull_request:
  push:

jobs:
  validate-pr:
    if: github.event_name == 'pull_request'
    uses: Cooper-Wallace/repokit/.github/workflows/semantic-pr.yml@main

  ci:
    name: "Test & Coverage"
    runs-on: ubuntu-latest
    steps:
      - run: pytest
`
		if err := os.WriteFile(inlineCI, []byte(inlineContent), 0644); err != nil {
			t.Fatalf("failed to write test ci.yml: %v", err)
		}

		patched, err := PatchCIWorkflowPRGuard(inlineCI)
		if err != nil {
			t.Fatalf("PatchCIWorkflowPRGuard failed: %v", err)
		}
		if !patched {
			t.Errorf("Expected patch to return true for inline ci: job name")
		}

		cb, _ := os.ReadFile(inlineCI)
		cStr := string(cb)
		if !strings.Contains(cStr, `name: "ci / Test & Coverage"`) {
			t.Errorf("Expected patched job name 'ci / Test & Coverage', got:\n%s", cStr)
		}

		// Idempotency check
		patchedAgain, err := PatchCIWorkflowPRGuard(inlineCI)
		if err != nil {
			t.Fatalf("Second PatchCIWorkflowPRGuard failed: %v", err)
		}
		if patchedAgain {
			t.Errorf("Expected second run to be idempotent (false), got true")
		}
	})

	t.Run("patch_inline_ci_job_name_unquoted", func(t *testing.T) {
		inlineTarget := t.TempDir()
		inlineCI := filepath.Join(inlineTarget, "ci.yml")
		inlineContent := `name: CI

on:
  push:

jobs:
  ci:
    name: Test & Coverage
    runs-on: ubuntu-latest
    steps:
      - run: pytest
`
		if err := os.WriteFile(inlineCI, []byte(inlineContent), 0644); err != nil {
			t.Fatalf("failed to write test ci.yml: %v", err)
		}

		patched, err := PatchCIWorkflowPRGuard(inlineCI)
		if err != nil {
			t.Fatalf("PatchCIWorkflowPRGuard failed: %v", err)
		}
		if !patched {
			t.Errorf("Expected patch to return true for unquoted inline ci: job name")
		}

		cb, _ := os.ReadFile(inlineCI)
		cStr := string(cb)
		if !strings.Contains(cStr, `name: "ci / Test & Coverage"`) {
			t.Errorf("Expected patched job name 'ci / Test & Coverage', got:\n%s", cStr)
		}
	})
}

func TestAuditLocalFast(t *testing.T) {
	target := t.TempDir()

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = target
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Audit should fail on empty repository
	passed, err := AuditRepository(target, "", true, false)
	if err != nil {
		t.Fatalf("AuditRepository returned unexpected error: %v", err)
	}
	if passed {
		t.Errorf("Empty repository should not pass audit")
	}

	// Apply required standards
	if err := InstallHooks(target, true); err != nil {
		t.Fatalf("InstallHooks failed: %v", err)
	}
	if err := ScaffoldTemplates(target, true); err != nil {
		t.Fatalf("ScaffoldTemplates failed: %v", err)
	}
	if _, err := ScaffoldReleaseWorkflow(target, true); err != nil {
		t.Fatalf("ScaffoldReleaseWorkflow failed: %v", err)
	}

	ciDir := filepath.Join(target, ".github", "workflows")
	_ = os.MkdirAll(ciDir, 0755)
	ciContent := `name: CI
jobs:
  validate-pr:
    if: github.event_name == 'pull_request'
    uses: Cooper-Wallace/repokit/.github/workflows/semantic-pr.yml@main
  ci:
    uses: Cooper-Wallace/repokit/.github/workflows/reusable-python.yml@main
`
	_ = os.WriteFile(filepath.Join(ciDir, "ci.yml"), []byte(ciContent), 0644)
	_ = os.MkdirAll(filepath.Join(target, "src"), 0755)

	passedGood, err := AuditRepository(target, "", true, false)
	if err != nil {
		t.Fatalf("AuditRepository returned error: %v", err)
	}
	if !passedGood {
		t.Errorf("Valid repository should pass audit")
	}
}

func TestAuditGovernanceHub(t *testing.T) {
	target := t.TempDir()

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = target
	_ = cmd.Run()

	_ = InstallHooks(target, true)
	_ = os.MkdirAll(filepath.Join(target, ".github", "workflows"), 0755)
	_ = os.WriteFile(filepath.Join(target, ".github", "pull_request_template.md"), []byte("## Summary\n"), 0644)
	_ = os.MkdirAll(filepath.Join(target, "cmd", "repokit"), 0755)
	_ = os.WriteFile(filepath.Join(target, "cmd", "repokit", "main.go"), []byte("package main\n"), 0644)
	_ = os.WriteFile(filepath.Join(target, "install.sh"), []byte("#!/bin/bash\n"), 0755)
	_ = os.WriteFile(filepath.Join(target, ".github", "workflows", "reusable-python.yml"), []byte("name: Reusable Python\n"), 0644)

	// Governance hub without test.yml should fail audit
	passedNoTest, _ := AuditRepository(target, "", true, false)
	if passedNoTest {
		t.Errorf("Governance hub without test.yml should fail audit")
	}

	// Governance hub with test.yml and release.yml should pass audit
	_ = os.WriteFile(filepath.Join(target, ".github", "workflows", "test.yml"), []byte("name: Test\n"), 0644)
	relContent := `name: Release
permissions:
  contents: write
  pull-requests: write
jobs:
  release-please:
    uses: Cooper-Wallace/repokit/.github/workflows/reusable-release-prep.yml@main
`
	_ = os.WriteFile(filepath.Join(target, ".github", "workflows", "release.yml"), []byte(relContent), 0644)
	passedHub, _ := AuditRepository(target, "", true, false)
	if !passedHub {
		t.Errorf("Governance hub with test.yml and release.yml should pass audit")
	}
}

func TestIsGovernanceHub(t *testing.T) {
	// Repokit root returns true
	if !IsGovernanceHub(".") {
		t.Errorf("Expected repokit root to return true for IsGovernanceHub")
	}

	// Temp non-governance repo returns false
	tempNonGov := t.TempDir()
	if IsGovernanceHub(tempNonGov) {
		t.Errorf("Expected empty temp repo to return false for IsGovernanceHub")
	}

	// Temp repo with only archetype files returns false
	_ = os.WriteFile(filepath.Join(tempNonGov, "go.mod"), []byte("module example.com/test\n"), 0644)
	if IsGovernanceHub(tempNonGov) {
		t.Errorf("Expected standard go repo to return false for IsGovernanceHub")
	}

	// Temp repo with reusable workflow but no repokit binary/files returns false
	tempPartial := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempPartial, ".github", "workflows"), 0755)
	_ = os.WriteFile(filepath.Join(tempPartial, ".github", "workflows", "reusable-python.yml"), []byte("name: Reusable Python\n"), 0644)
	if IsGovernanceHub(tempPartial) {
		t.Errorf("Expected repo with only reusable workflows to return false for IsGovernanceHub")
	}

	// Temp repo with reusable workflow and repokit.go returns true
	_ = os.WriteFile(filepath.Join(tempPartial, "repokit.go"), []byte("package main\n"), 0644)
	if !IsGovernanceHub(tempPartial) {
		t.Errorf("Expected repo with reusable workflow and repokit.go to return true for IsGovernanceHub")
	}
}

func TestPRTemplateContainsQualityGates(t *testing.T) {
	expectedTerms := []string{
		"## Quality Gates & Sign-Off Checklist",
		"Gate 1: Automated Quality Gates",
		"Gate 2: Agent Review Verification",
		"Gate 3: Human Review & Manual Promotion",
		"never automated",
		"## Subagent Commit Trail",
		"## Gate 2: Phase 3 Adversarial Review Summary",
	}

	for _, term := range expectedTerms {
		if !strings.Contains(PRTemplate, term) {
			t.Errorf("PRTemplate constant missing term: %s", term)
		}
	}

	repoRoot, err := GetGitRepoRoot(".")
	if err == nil {
		templatePath := filepath.Join(repoRoot, ".github", "pull_request_template.md")
		if fileExists(templatePath) {
			contentBytes, _ := os.ReadFile(templatePath)
			content := string(contentBytes)
			for _, term := range expectedTerms {
				if !strings.Contains(content, term) {
					t.Errorf(".github/pull_request_template.md missing term: %s", term)
				}
			}
		}
	}
}

func TestSemanticPRWorkflowPushGuard(t *testing.T) {
	guard := "if: github.event_name == 'pull_request' || github.event_name == 'pull_request_target'"
	if !strings.Contains(WorkflowSemanticPR, guard) {
		t.Errorf("WorkflowSemanticPR constant missing push guard: %s", guard)
	}
	if !strings.Contains(WorkflowSemanticPR, "workflow_call:") {
		t.Errorf("WorkflowSemanticPR constant missing workflow_call:")
	}

	repoRoot, err := GetGitRepoRoot(".")
	if err == nil {
		workflowPath := filepath.Join(repoRoot, ".github", "workflows", "semantic-pr.yml")
		if fileExists(workflowPath) {
			contentBytes, _ := os.ReadFile(workflowPath)
			content := string(contentBytes)
			if !strings.Contains(content, guard) {
				t.Errorf(".github/workflows/semantic-pr.yml missing push guard: %s", guard)
			}
		}
	}
}

func TestArchetypeCIWorkflowHasSemanticPRPushGuard(t *testing.T) {
	for _, arch := range []string{"bash", "python", "java", "typescript", "go"} {
		templatePath := fmt.Sprintf("templates/%s/.github/workflows/ci.yml", arch)
		contentBytes, err := TemplatesFS.ReadFile(templatePath)
		if err != nil {
			t.Fatalf("Could not read template %s: %v", templatePath, err)
		}
		content := string(contentBytes)
		if !strings.Contains(content, "if: github.event_name == 'pull_request'") {
			t.Errorf("CI template for %s does not guard validate-pr with if: github.event_name == 'pull_request'", arch)
		}
	}
}

func TestApplyGitHubRepoSettings(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	var capturedInput string
	var capturedArgs []string

	CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
		capturedArgs = args
		capturedInput = input
		return "{}", "", 0, nil
	}

	err := ApplyGitHubRepoSettings("Cooper-Wallace/repokit")
	if err != nil {
		t.Fatalf("ApplyGitHubRepoSettings failed: %v", err)
	}

	expectedPrefix := []string{"gh", "api", "--method", "PATCH", "repos/Cooper-Wallace/repokit", "--input", "-"}
	if len(capturedArgs) != len(expectedPrefix) {
		t.Errorf("Expected args %v, got %v", expectedPrefix, capturedArgs)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(capturedInput), &payload); err != nil {
		t.Fatalf("Failed to unmarshal captured input: %v", err)
	}

	if payload["allow_auto_merge"] != false {
		t.Errorf("Expected allow_auto_merge to be false")
	}
	if payload["allow_squash_merge"] != true {
		t.Errorf("Expected allow_squash_merge to be true")
	}
	if payload["allow_merge_commit"] != false {
		t.Errorf("Expected allow_merge_commit to be false")
	}
	if payload["allow_rebase_merge"] != false {
		t.Errorf("Expected allow_rebase_merge to be false")
	}
	if payload["squash_merge_commit_title"] != "PR_TITLE" {
		t.Errorf("Expected PR_TITLE")
	}
	if payload["squash_merge_commit_message"] != "PR_BODY" {
		t.Errorf("Expected PR_BODY")
	}
}

func TestApplyBranchRuleset(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("hub_repo_ruleset", func(t *testing.T) {
		var calls [][]string
		var capturedInput string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			calls = append(calls, args)
			if input != "" {
				capturedInput = input
			}
			return "[]", "", 0, nil
		}

		err := ApplyBranchRuleset("Cooper-Wallace/repokit", "~DEFAULT_BRANCH", nil)
		if err != nil {
			t.Fatalf("ApplyBranchRuleset failed: %v", err)
		}

		if len(calls) != 2 {
			t.Fatalf("Expected 2 gh calls, got %d", len(calls))
		}

		var payload map[string]interface{}
		_ = json.Unmarshal([]byte(capturedInput), &payload)

		rules := payload["rules"].([]interface{})
		var hasStatusChecks, hasPRSquash bool
		for _, r := range rules {
			rMap := r.(map[string]interface{})
			if rMap["type"] == "pull_request" {
				params := rMap["parameters"].(map[string]interface{})
				methods := params["allowed_merge_methods"].([]interface{})
				if len(methods) == 1 && methods[0] == "squash" {
					hasPRSquash = true
				}
			}
			if rMap["type"] == "required_status_checks" {
				params := rMap["parameters"].(map[string]interface{})
				checks := params["required_status_checks"].([]interface{})
				if len(checks) == 2 {
					hasStatusChecks = true
				}
			}
		}
		if !hasPRSquash {
			t.Errorf("Expected pull_request rule with squash merge method")
		}
		if !hasStatusChecks {
			t.Errorf("Expected required_status_checks rule with 2 checks")
		}
	})

	t.Run("consumer_repo_ruleset", func(t *testing.T) {
		var capturedInput string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if input != "" {
				capturedInput = input
			}
			return "[]", "", 0, nil
		}

		err := ApplyBranchRuleset("Cooper-Wallace/my-repo", "~DEFAULT_BRANCH", nil)
		if err != nil {
			t.Fatalf("ApplyBranchRuleset failed: %v", err)
		}

		var payload map[string]interface{}
		_ = json.Unmarshal([]byte(capturedInput), &payload)

		rules := payload["rules"].([]interface{})
		for _, r := range rules {
			rMap := r.(map[string]interface{})
			if rMap["type"] == "required_status_checks" {
				params := rMap["parameters"].(map[string]interface{})
				checks := params["required_status_checks"].([]interface{})
				c0 := checks[0].(map[string]interface{})
				if c0["context"] != "ci / Test & Coverage" {
					t.Errorf("Expected 'ci / Test & Coverage', got %v", c0["context"])
				}
			}
		}
	})

	t.Run("exempt_vault_repo", func(t *testing.T) {
		var calls [][]string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			calls = append(calls, args)
			return "[]", "", 0, nil
		}

		err := ApplyBranchRuleset("Cooper-Wallace/AgentVault", "~DEFAULT_BRANCH", nil)
		if err != nil {
			t.Fatalf("ApplyBranchRuleset should succeed for exempt vault repo: %v", err)
		}
		if len(calls) != 0 {
			t.Errorf("Expected 0 gh calls for exempt vault repo, got %d", len(calls))
		}
	})
}

func TestIsVaultOrExemptRepo(t *testing.T) {
	cases := []struct {
		slug     string
		expected bool
	}{
		{"Cooper-Wallace/AgentVault", true},
		{"Cooper-Wallace/agentvault", true},
		{"Cooper-Wallace/agent-vault", true},
		{"Cooper-Wallace/Agent-Vault", true},
		{"agentvault", true},
		{"agent-vault", true},
		{"AgentVault", true},
		{"Agent-Vault", true},
		{"Cooper-Wallace/agentkit", false},
		{"Cooper-Wallace/repokit", false},
		{"agentkit", false},
		{"my-agent-vault-app", false},
		{"", false},
	}

	for _, tc := range cases {
		name := tc.slug
		if name == "" {
			name = "empty_slug"
		}
		t.Run(name, func(t *testing.T) {
			actual := IsVaultOrExemptRepo(tc.slug)
			if actual != tc.expected {
				t.Errorf("IsVaultOrExemptRepo(%q) = %v; want %v", tc.slug, actual, tc.expected)
			}
		})
	}
}

func TestAuditRepositoryRemote(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	_ = cmd.Run()
	cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/Cooper-Wallace/repokit.git")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	t.Run("all_pass", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit" {
				data := map[string]interface{}{
					"allow_squash_merge":          true,
					"allow_merge_commit":          false,
					"allow_rebase_merge":          false,
					"delete_branch_on_merge":      true,
					"squash_merge_commit_title":   "PR_TITLE",
					"squash_merge_commit_message": "PR_BODY",
					"allow_auto_merge":            false,
				}
				bytes, _ := json.Marshal(data)
				return string(bytes), "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit/rulesets" {
				rulesets := []map[string]interface{}{
					{
						"name":        "default-branch-protection",
						"enforcement": "active",
						"rules": []map[string]interface{}{
							{
								"type": "required_status_checks",
								"parameters": map[string]interface{}{
									"required_status_checks": []map[string]string{
										{"context": "Test & Coverage"},
									},
								},
							},
						},
					},
				}
				bytes, _ := json.Marshal(rulesets)
				return string(bytes), "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit/actions/permissions/workflow" {
				return `{"default_workflow_permissions": "read", "can_approve_pull_request_reviews": true}`, "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		passed, err := AuditRepository(tmpDir, "", false, true)
		if err != nil || !passed {
			t.Errorf("Expected remote audit to pass, got passed=%v, err=%v", passed, err)
		}
	})

	t.Run("workflow_permissions_fails", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit" {
				data := map[string]interface{}{
					"allow_squash_merge":          true,
					"allow_merge_commit":          false,
					"allow_rebase_merge":          false,
					"delete_branch_on_merge":      true,
					"squash_merge_commit_title":   "PR_TITLE",
					"squash_merge_commit_message": "PR_BODY",
					"allow_auto_merge":            false,
				}
				bytes, _ := json.Marshal(data)
				return string(bytes), "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit/rulesets" {
				rulesets := []map[string]interface{}{
					{
						"name":        "default-branch-protection",
						"enforcement": "active",
						"rules": []map[string]interface{}{
							{
								"type": "required_status_checks",
								"parameters": map[string]interface{}{
									"required_status_checks": []map[string]string{
										{"context": "Test & Coverage"},
									},
								},
							},
						},
					},
				}
				bytes, _ := json.Marshal(rulesets)
				return string(bytes), "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit/actions/permissions/workflow" {
				return `{"default_workflow_permissions": "read", "can_approve_pull_request_reviews": false}`, "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		passed, _ := AuditRepository(tmpDir, "", false, true)
		if passed {
			t.Errorf("Expected remote audit to fail when can_approve_pull_request_reviews is false")
		}
	})

	t.Run("auto_merge_fails", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit" {
				data := map[string]interface{}{
					"allow_squash_merge":          true,
					"allow_merge_commit":          false,
					"allow_rebase_merge":          false,
					"delete_branch_on_merge":      true,
					"squash_merge_commit_title":   "PR_TITLE",
					"squash_merge_commit_message": "PR_BODY",
					"allow_auto_merge":            true, // FAILS
				}
				bytes, _ := json.Marshal(data)
				return string(bytes), "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		passed, _ := AuditRepository(tmpDir, "", false, true)
		if passed {
			t.Errorf("Expected remote audit to fail when allow_auto_merge is true")
		}
	})
}

func TestScaffoldArchetypeCI(t *testing.T) {
	t.Run("scaffold_new", func(t *testing.T) {
		target := t.TempDir()
		_ = os.WriteFile(filepath.Join(target, "pyproject.toml"), []byte("[project]\nname='test'\n"), 0644)
		scaffolded, err := ScaffoldArchetypeCI(target, false)
		if err != nil {
			t.Fatalf("ScaffoldArchetypeCI failed: %v", err)
		}
		if !scaffolded {
			t.Errorf("Expected ScaffoldArchetypeCI to return true")
		}
		ciPath := filepath.Join(target, ".github", "workflows", "ci.yml")
		if !fileExists(ciPath) {
			t.Fatalf("Expected .github/workflows/ci.yml to exist")
		}
		contentBytes, _ := os.ReadFile(ciPath)
		if !strings.Contains(string(contentBytes), DefaultGovernanceRepo) {
			t.Errorf("Expected ci.yml to contain DefaultGovernanceRepo")
		}
	})

	t.Run("patch_existing_no_force", func(t *testing.T) {
		target := t.TempDir()
		_ = os.WriteFile(filepath.Join(target, "pyproject.toml"), []byte("[project]\nname='test'\n"), 0644)
		ciDir := filepath.Join(target, ".github", "workflows")
		_ = os.MkdirAll(ciDir, 0755)
		ciFile := filepath.Join(ciDir, "ci.yml")
		_ = os.WriteFile(ciFile, []byte("name: CI\njobs:\n  validate-pr:\n    uses: foo\n"), 0644)

		patched, err := ScaffoldArchetypeCI(target, false)
		if err != nil || !patched {
			t.Errorf("Expected patch to succeed, got %v, err=%v", patched, err)
		}
		contentBytes, _ := os.ReadFile(ciFile)
		if !strings.Contains(string(contentBytes), "if: github.event_name == 'pull_request'") {
			t.Errorf("Expected patched ci.yml to contain PR guard")
		}
	})

	t.Run("no_archetype_patch_existing", func(t *testing.T) {
		target := t.TempDir()
		ciDir := filepath.Join(target, ".github", "workflows")
		_ = os.MkdirAll(ciDir, 0755)
		ciFile := filepath.Join(ciDir, "ci.yml")
		_ = os.WriteFile(ciFile, []byte("name: CI\njobs:\n  validate-pr:\n    uses: foo\n"), 0644)

		patched, _ := ScaffoldArchetypeCI(target, false)
		if !patched {
			t.Errorf("Expected patch without archetype to succeed")
		}
	})

	t.Run("governance_hub_skipped", func(t *testing.T) {
		target := t.TempDir()
		_ = os.MkdirAll(filepath.Join(target, ".github", "workflows"), 0755)
		_ = os.WriteFile(filepath.Join(target, ".github", "workflows", "reusable-go.yml"), []byte("name: Reusable Go\n"), 0644)
		_ = os.WriteFile(filepath.Join(target, "repokit.go"), []byte("package main\n"), 0644)
		_ = os.WriteFile(filepath.Join(target, "go.mod"), []byte("module github.com/test/repo\n"), 0644)

		scaffolded, err := ScaffoldArchetypeCI(target, false)
		if err != nil {
			t.Fatalf("ScaffoldArchetypeCI returned unexpected error: %v", err)
		}
		if scaffolded {
			t.Errorf("Expected ScaffoldArchetypeCI to return false on governance hub")
		}

		ciPath := filepath.Join(target, ".github", "workflows", "ci.yml")
		if fileExists(ciPath) {
			t.Errorf("Expected ci.yml NOT to be created on governance hub")
		}

		// Also verify with force = true
		scaffoldedForce, err := ScaffoldArchetypeCI(target, true)
		if err != nil {
			t.Fatalf("ScaffoldArchetypeCI with force returned error: %v", err)
		}
		if scaffoldedForce {
			t.Errorf("Expected ScaffoldArchetypeCI with force to return false on governance hub")
		}
		if fileExists(ciPath) {
			t.Errorf("Expected ci.yml NOT to be created on governance hub even with force")
		}

		// Also verify on repokit root itself
		scaffoldedRoot, err := ScaffoldArchetypeCI(".", false)
		if err != nil {
			t.Fatalf("ScaffoldArchetypeCI on root returned error: %v", err)
		}
		if scaffoldedRoot {
			t.Errorf("Expected ScaffoldArchetypeCI to return false on repokit root")
		}
	})
}

func TestScaffoldReleaseWorkflow(t *testing.T) {
	t.Run("scaffold_new_generic", func(t *testing.T) {
		target := t.TempDir()
		scaffolded, err := ScaffoldReleaseWorkflow(target, false)
		if err != nil {
			t.Fatalf("ScaffoldReleaseWorkflow failed: %v", err)
		}
		if !scaffolded {
			t.Errorf("Expected ScaffoldReleaseWorkflow to return true")
		}
		relPath := filepath.Join(target, ".github", "workflows", "release.yml")
		if !fileExists(relPath) {
			t.Fatalf("Expected .github/workflows/release.yml to exist")
		}
		data, _ := os.ReadFile(relPath)
		if !strings.Contains(string(data), DefaultGovernanceRepo) {
			t.Errorf("Expected release.yml to contain DefaultGovernanceRepo")
		}
		if !strings.Contains(string(data), "reusable-release-prep.yml") {
			t.Errorf("Expected release.yml to call reusable-release-prep.yml")
		}
	})

	t.Run("scaffold_go_archetype", func(t *testing.T) {
		target := t.TempDir()
		_ = os.WriteFile(filepath.Join(target, "go.mod"), []byte("module test\n"), 0644)
		scaffolded, err := ScaffoldReleaseWorkflow(target, false)
		if err != nil {
			t.Fatalf("ScaffoldReleaseWorkflow failed: %v", err)
		}
		if !scaffolded {
			t.Errorf("Expected ScaffoldReleaseWorkflow to return true")
		}
		goreleaserPath := filepath.Join(target, ".goreleaser.yaml")
		if !fileExists(goreleaserPath) {
			t.Fatalf("Expected .goreleaser.yaml to exist")
		}
		data, _ := os.ReadFile(goreleaserPath)
		if !strings.Contains(string(data), filepath.Base(target)) {
			t.Errorf("Expected .goreleaser.yaml to contain repo name")
		}
	})

	t.Run("existing_no_force", func(t *testing.T) {
		target := t.TempDir()
		relDir := filepath.Join(target, ".github", "workflows")
		_ = os.MkdirAll(relDir, 0755)
		_ = os.WriteFile(filepath.Join(relDir, "release.yml"), []byte("existing"), 0644)

		scaffolded, err := ScaffoldReleaseWorkflow(target, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if scaffolded {
			t.Errorf("Expected ScaffoldReleaseWorkflow to return false when release.yml exists without force")
		}
	})
}

func TestGetGitHubUser(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("success", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "cooperwallace\n", "", 0, nil
		}
		user, err := GetGitHubUser()
		if err != nil || user != "cooperwallace" {
			t.Errorf("Expected user cooperwallace, got %s, err=%v", user, err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "", "not logged in", 1, fmt.Errorf("exit status 1")
		}
		_, err := GetGitHubUser()
		if err == nil {
			t.Errorf("Expected error from GetGitHubUser")
		}
	})
}

func TestConfigureRepository(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("not_in_git_repo", func(t *testing.T) {
		emptyDir := t.TempDir()
		err := ConfigureRepository(emptyDir, false)
		if err == nil {
			t.Errorf("Expected error when configuring non-git repo")
		}
	})

	t.Run("git_repo_no_remote", func(t *testing.T) {
		gitDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = gitDir
		_ = cmd.Run()

		err := ConfigureRepository(gitDir, false)
		if err == nil {
			t.Errorf("Expected error when configuring repo with no remote")
		}
	})

	t.Run("valid_repo", func(t *testing.T) {
		gitDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = gitDir
		_ = cmd.Run()
		cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/Cooper-Wallace/test-repo.git")
		cmd.Dir = gitDir
		_ = cmd.Run()

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				return defaultCommandRunner(args, dir, input)
			}
			return "[]", "", 0, nil
		}

		err := ConfigureRepository(gitDir, true)
		if err != nil {
			t.Fatalf("ConfigureRepository failed: %v", err)
		}

		prTemplate := filepath.Join(gitDir, ".github", "pull_request_template.md")
		if !fileExists(prTemplate) {
			t.Errorf("Expected PR template to be scaffolded")
		}
	})

	t.Run("exempt_vault_repo", func(t *testing.T) {
		gitDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = gitDir
		_ = cmd.Run()
		cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/Cooper-Wallace/AgentVault.git")
		cmd.Dir = gitDir
		_ = cmd.Run()

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				return defaultCommandRunner(args, dir, input)
			}
			return "[]", "", 0, nil
		}

		err := ConfigureRepository(gitDir, true)
		if err != nil {
			t.Fatalf("ConfigureRepository failed: %v", err)
		}

		ciFile := filepath.Join(gitDir, ".github", "workflows", "ci.yml")
		if fileExists(ciFile) {
			t.Errorf("CI workflow should not be scaffolded for exempt vault repo")
		}
		relFile := filepath.Join(gitDir, ".github", "workflows", "release.yml")
		if fileExists(relFile) {
			t.Errorf("Release workflow should not be scaffolded for exempt vault repo")
		}
		prTemplate := filepath.Join(gitDir, ".github", "pull_request_template.md")
		if fileExists(prTemplate) {
			t.Errorf("PR template should not be scaffolded for exempt vault repo")
		}
	})
}

func TestCreateRepository(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("not_authenticated", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 1 && args[1] == "api" {
				return "", "auth error", 1, fmt.Errorf("auth error")
			}
			return "", "", 0, nil
		}
		err := CreateRepository("test-repo", "", false, "go", "")
		if err == nil {
			t.Errorf("Expected error when not authenticated")
		}
	})

	t.Run("target_dir_not_empty", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("data"), 0644)

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "testuser\n", "", 0, nil
		}
		err := CreateRepository("test-repo", "", false, "go", dir)
		if err == nil {
			t.Errorf("Expected error when target dir not empty")
		}
	})

	t.Run("successful_create_go", func(t *testing.T) {
		parentDir := t.TempDir()
		targetDir := filepath.Join(parentDir, "my-go-pkg")

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "gh" {
				if len(args) > 1 && args[1] == "api" && args[2] == "user" {
					return "testuser\n", "", 0, nil
				}
				return "{}", "", 0, nil
			}
			if len(args) > 1 && args[0] == "git" && (args[1] == "push" || args[1] == "commit") {
				return "", "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		err := CreateRepository("my-go-pkg", "Sample description", true, "go", targetDir)
		if err != nil {
			t.Fatalf("CreateRepository failed: %v", err)
		}

		if !fileExists(filepath.Join(targetDir, "go.mod")) {
			t.Errorf("Expected go.mod to be created")
		}
		if !fileExists(filepath.Join(targetDir, ".github", "pull_request_template.md")) {
			t.Errorf("Expected pull_request_template.md to be created")
		}
	})

	t.Run("successful_create_default_readme", func(t *testing.T) {
		parentDir := t.TempDir()
		targetDir := filepath.Join(parentDir, "my-default-pkg")

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "gh" {
				if len(args) > 1 && args[1] == "api" && args[2] == "user" {
					return "testuser\n", "", 0, nil
				}
				return "{}", "", 0, nil
			}
			if len(args) > 1 && args[0] == "git" && (args[1] == "push" || args[1] == "commit") {
				return "", "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		err := CreateRepository("my-default-pkg", "", false, "", targetDir)
		if err != nil {
			t.Fatalf("CreateRepository failed: %v", err)
		}

		if !fileExists(filepath.Join(targetDir, "README.md")) {
			t.Errorf("Expected README.md to be created")
		}
		if !fileExists(filepath.Join(targetDir, ".gitignore")) {
			t.Errorf("Expected .gitignore to be created")
		}
	})
}

func TestHelpers(t *testing.T) {
	// Repo slug parsing
	t.Run("repo_slug_parsing", func(t *testing.T) {
		tmpDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		_ = cmd.Run()
		cmd = exec.Command("git", "remote", "add", "origin", "git@github.com:Cooper-Wallace/repokit.git")
		cmd.Dir = tmpDir
		_ = cmd.Run()

		slug, err := GetRepoSlug(tmpDir)
		if err != nil || slug != "Cooper-Wallace/repokit" {
			t.Errorf("Expected 'Cooper-Wallace/repokit', got '%s', err: %v", slug, err)
		}
	})

	t.Run("git_repo_root", func(t *testing.T) {
		root, err := GetGitRepoRoot(".")
		if err != nil || root == "" {
			t.Errorf("GetGitRepoRoot failed: %v", err)
		}

		_, errBad := GetGitRepoRoot(os.TempDir())
		if errBad == nil {
			// In case os.TempDir() is inside a git repo, ignore; otherwise it should fail
		}
	})

	t.Run("print_templates", func(t *testing.T) {
		PrintTemplates()
	})

	t.Run("logging", func(t *testing.T) {
		LogInfo("test info: %s", "arg")
		LogSuccess("test success: %s", "arg")
		LogWarn("test warn: %s", "arg")
		LogError("test error: %s", "arg")
	})

	t.Run("run_cmd_error", func(t *testing.T) {
		_, _, code, err := RunCmd([]string{"nonexistent_binary_xyz_123"}, "")
		if err == nil || code == 0 {
			t.Errorf("Expected error running nonexistent binary")
		}
	})

	t.Run("run_cmd_empty_args", func(t *testing.T) {
		out, errOut, code, err := RunCmd([]string{}, "")
		if out != "" || errOut != "" || code != 0 || err != nil {
			t.Errorf("Expected empty result for empty args")
		}
	})
}

func TestMoreBranches(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("scaffold_templates_already_exist", func(t *testing.T) {
		dir := t.TempDir()
		_ = ScaffoldTemplates(dir, true)
		// Second call with force=false
		_ = ScaffoldTemplates(dir, false)
	})

	t.Run("scaffold_archetype_ci_already_exists", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname='test'\n"), 0644)
		_ = os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0755)
		ciContent := "name: CI\njobs:\n  validate-pr:\n    if: github.event_name == 'pull_request'\n    uses: Cooper-Wallace/repokit/.github/workflows/semantic-pr.yml@main\n"
		_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte(ciContent), 0644)

		modified, err := ScaffoldArchetypeCI(dir, false)
		if err != nil || modified {
			t.Errorf("Expected modified=false when CI already exists and guarded")
		}
	})

	t.Run("patch_ci_guard_nonexistent_file", func(t *testing.T) {
		_, err := PatchCIWorkflowPRGuard("/nonexistent/file/path/ci.yml")
		if err == nil {
			t.Errorf("Expected error patching nonexistent file")
		}
	})

	t.Run("detect_archetypes_variants", func(t *testing.T) {
		d1 := t.TempDir()
		_ = os.WriteFile(filepath.Join(d1, "setup.py"), []byte("# setup"), 0644)
		if DetectArchetype(d1) != "python" {
			t.Errorf("Expected python from setup.py")
		}

		d2 := t.TempDir()
		_ = os.WriteFile(filepath.Join(d2, "build.gradle"), []byte("// gradle"), 0644)
		if DetectArchetype(d2) != "java" {
			t.Errorf("Expected java from build.gradle")
		}

		d3 := t.TempDir()
		_ = os.WriteFile(filepath.Join(d3, "package.json"), []byte("{}"), 0644)
		if DetectArchetype(d3) != "typescript" {
			t.Errorf("Expected typescript from package.json")
		}

		d4 := t.TempDir()
		if DetectArchetype(d4) != "" {
			t.Errorf("Expected empty archetype for empty dir")
		}

		if checks := AuditArchetypeStructure(d4, "unknown"); checks != nil {
			t.Errorf("Expected nil checks for unknown archetype")
		}
	})

	t.Run("apply_nonexistent_archetype_template", func(t *testing.T) {
		dir := t.TempDir()
		err := ApplyArchetypeTemplate("nonexistent-type-xyz", dir, nil)
		if err == nil {
			t.Errorf("Expected error for nonexistent archetype")
		}
	})

	t.Run("github_repo_settings_failure", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "", "API error", 1, fmt.Errorf("API error")
		}
		err := ApplyGitHubRepoSettings("org/repo")
		if err == nil {
			t.Errorf("Expected error from ApplyGitHubRepoSettings on failure")
		}
	})

	t.Run("ruleset_update_existing_and_failure", func(t *testing.T) {
		// Existing ruleset update
		var capturedMethod string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/org/repo/rulesets" {
				return `[{"id": 42, "name": "default-branch-protection"}]`, "", 0, nil
			}
			if len(args) > 3 && args[3] == "PUT" {
				capturedMethod = "PUT"
				return "{}", "", 0, nil
			}
			return "{}", "", 0, nil
		}
		err := ApplyBranchRuleset("org/repo", "~DEFAULT_BRANCH", []string{"check-1"})
		if err != nil {
			t.Fatalf("ApplyBranchRuleset failed: %v", err)
		}
		if capturedMethod != "PUT" {
			t.Errorf("Expected PUT method for existing ruleset")
		}

		// Ruleset failure
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "", "Ruleset failure", 1, fmt.Errorf("Ruleset failure")
		}
		errFail := ApplyBranchRuleset("org/repo", "~DEFAULT_BRANCH", nil)
		if errFail == nil {
			t.Errorf("Expected error when ruleset application fails")
		}
	})

	t.Run("audit_both_local_and_remote", func(t *testing.T) {
		tmpDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		_ = cmd.Run()
		cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/Cooper-Wallace/repokit.git")
		cmd.Dir = tmpDir
		_ = cmd.Run()

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				return defaultCommandRunner(args, dir, input)
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit" {
				data := map[string]interface{}{
					"allow_squash_merge":          true,
					"allow_merge_commit":          false,
					"allow_rebase_merge":          false,
					"delete_branch_on_merge":      true,
					"squash_merge_commit_title":   "PR_TITLE",
					"squash_merge_commit_message": "PR_BODY",
					"allow_auto_merge":            false,
				}
				bytes, _ := json.Marshal(data)
				return string(bytes), "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit/rulesets" {
				rulesets := []map[string]interface{}{
					{
						"name":        "default-branch-protection",
						"enforcement": "active",
						"rules": []map[string]interface{}{
							{
								"type": "required_status_checks",
								"parameters": map[string]interface{}{
									"required_status_checks": []map[string]string{
										{"context": "Test & Coverage"},
									},
								},
							},
						},
					},
				}
				bytes, _ := json.Marshal(rulesets)
				return string(bytes), "", 0, nil
			}
			return "{}", "", 0, nil
		}

		// local_only=true and remote_only=true activates both
		_, _ = AuditRepository(tmpDir, "", true, true)
	})
}

func TestApplyWorkflowPermissions(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("success", func(t *testing.T) {
		var capturedArgs []string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			capturedArgs = args
			return "{}", "", 0, nil
		}
		err := ApplyWorkflowPermissions("org/my-repo")
		if err != nil {
			t.Fatalf("Expected ApplyWorkflowPermissions to succeed, got %v", err)
		}
		expectedEndpoint := "repos/org/my-repo/actions/permissions/workflow"
		foundEndpoint := false
		foundDefaultPerms := false
		foundCanApprove := false
		for i, a := range capturedArgs {
			if a == expectedEndpoint {
				foundEndpoint = true
			}
			if a == "-F" && i+1 < len(capturedArgs) {
				if capturedArgs[i+1] == "default_workflow_permissions=read" {
					foundDefaultPerms = true
				}
				if capturedArgs[i+1] == "can_approve_pull_request_reviews=true" {
					foundCanApprove = true
				}
			}
		}
		if !foundEndpoint {
			t.Errorf("Expected endpoint %s in args: %v", expectedEndpoint, capturedArgs)
		}
		if !foundDefaultPerms {
			t.Errorf("Expected default_workflow_permissions=read in args: %v", capturedArgs)
		}
		if !foundCanApprove {
			t.Errorf("Expected can_approve_pull_request_reviews=true in args: %v", capturedArgs)
		}
	})

	t.Run("failure", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "", "Permission denied", 1, fmt.Errorf("exit status 1")
		}
		err := ApplyWorkflowPermissions("org/my-repo")
		if err == nil {
			t.Errorf("Expected ApplyWorkflowPermissions to fail on exit code 1")
		}
	})
}

func TestRulesetIDFormatting(t *testing.T) {
	t.Run("formatRulesetID_types", func(t *testing.T) {
		largeID := float64(22459068)
		formatted := FormatRulesetID(largeID)
		if formatted != "22459068" {
			t.Errorf("FormatRulesetID(float64(22459068)) = %q, want '22459068'", formatted)
		}
		if strings.Contains(formatted, "e+") || strings.Contains(formatted, "E+") {
			t.Errorf("FormatRulesetID produced scientific notation: %s", formatted)
		}

		if got := FormatRulesetID(int64(22459068)); got != "22459068" {
			t.Errorf("FormatRulesetID(int64) = %q, want '22459068'", got)
		}
		if got := FormatRulesetID(42); got != "42" {
			t.Errorf("FormatRulesetID(int) = %q, want '42'", got)
		}
		if got := FormatRulesetID("99999"); got != "99999" {
			t.Errorf("FormatRulesetID(string) = %q, want '99999'", got)
		}
	})

	t.Run("apply_branch_ruleset_float_id", func(t *testing.T) {
		oldRunner := CommandRunner
		defer func() { CommandRunner = oldRunner }()

		var capturedURL string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/org/repo/rulesets" {
				return `[{"id": 22459068, "name": "default-branch-protection"}]`, "", 0, nil
			}
			if len(args) > 4 && args[1] == "api" && args[3] == "PUT" {
				capturedURL = args[4]
				return "{}", "", 0, nil
			}
			return "{}", "", 0, nil
		}

		err := ApplyBranchRuleset("org/repo", "~DEFAULT_BRANCH", nil)
		if err != nil {
			t.Fatalf("ApplyBranchRuleset failed: %v", err)
		}
		expectedURL := "repos/org/repo/rulesets/22459068"
		if capturedURL != expectedURL {
			t.Errorf("Expected PUT URL %q, got %q", expectedURL, capturedURL)
		}
	})

	t.Run("audit_repository_float_id", func(t *testing.T) {
		oldRunner := CommandRunner
		defer func() { CommandRunner = oldRunner }()

		tmpDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		_ = cmd.Run()
		cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/org/repo.git")
		cmd.Dir = tmpDir
		_ = cmd.Run()

		var detailQueriedURL string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/org/repo" {
				return `{"allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": false, "delete_branch_on_merge": true, "squash_merge_commit_title": "PR_TITLE", "squash_merge_commit_message": "PR_BODY", "allow_auto_merge": false}`, "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/org/repo/rulesets" {
				return `[{"id": 22459068, "name": "default-branch-protection", "enforcement": "active"}]`, "", 0, nil
			}
			if len(args) > 2 && strings.HasPrefix(args[2], "repos/org/repo/rulesets/") {
				detailQueriedURL = args[2]
				return `{"rules": [{"type": "required_status_checks", "parameters": {"required_status_checks": [{"context": "ci"}]}}]}`, "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/org/repo/actions/permissions/workflow" {
				return `{"can_approve_pull_request_reviews": true}`, "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		passed, err := AuditRepository(tmpDir, "", false, true)
		if err != nil || !passed {
			t.Errorf("Expected audit to pass, got passed=%v, err=%v", passed, err)
		}
		expectedDetailURL := "repos/org/repo/rulesets/22459068"
		if detailQueriedURL != expectedDetailURL {
			t.Errorf("Expected ruleset detail URL %q, got %q", expectedDetailURL, detailQueriedURL)
		}
	})
}

func TestConfigureWithOptions(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("backward_compatible_propagate_no_repos_specified", func(t *testing.T) {
		err := PropagateRepositories(PropagateOptions{})
		if err == nil {
			t.Errorf("Expected error when no repos specified")
		}
	})

	t.Run("single_repo_dry_run", func(t *testing.T) {
		gitDir := t.TempDir()
		cInit := exec.Command("git", "init", "-b", "main")
		cInit.Dir = gitDir
		_ = cInit.Run()
		cRemote := exec.Command("git", "remote", "add", "origin", "https://github.com/Cooper-Wallace/test-repo.git")
		cRemote.Dir = gitDir
		_ = cRemote.Run()

		err := ConfigureWithOptions(ConfigureOptions{
			TargetDir: gitDir,
			DryRun:    true,
		})
		if err != nil {
			t.Fatalf("Single repo dry run failed: %v", err)
		}
	})

	t.Run("dry_run_with_repos", func(t *testing.T) {
		mutatingCalls := 0
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			for _, a := range args {
				if a == "PUT" || a == "PATCH" || a == "POST" || a == "push" || a == "commit" {
					mutatingCalls++
				}
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Repos:  []string{"testorg/repo1", "testorg/repo2"},
			DryRun: true,
		})
		if err != nil {
			t.Fatalf("Dry run failed: %v", err)
		}
		if mutatingCalls > 0 {
			t.Errorf("Expected 0 mutating calls during dry run, got %d", mutatingCalls)
		}
	})

	t.Run("explicit_repos_execution", func(t *testing.T) {
		var executedEndpoints []string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 4 && args[1] == "api" {
				executedEndpoints = append(executedEndpoints, args[4])
			} else if len(args) > 2 && args[1] == "api" {
				executedEndpoints = append(executedEndpoints, args[2])
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Repos: []string{"org/repoA"},
		})
		if err != nil {
			t.Fatalf("ConfigureWithOptions failed: %v", err)
		}
		hasPatchRepo := false
		hasWorkflowPerms := false
		hasRuleset := false
		for _, ep := range executedEndpoints {
			if strings.Contains(ep, "repos/org/repoA") {
				hasPatchRepo = true
			}
			if strings.Contains(ep, "actions/permissions/workflow") {
				hasWorkflowPerms = true
			}
			if strings.Contains(ep, "rulesets") {
				hasRuleset = true
			}
		}
		if !hasPatchRepo || !hasWorkflowPerms || !hasRuleset {
			t.Errorf("Expected remote calls for repoA, got: %v", executedEndpoints)
		}
	})

	t.Run("local_discovery_and_pr_creation", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoA := filepath.Join(tmpDir, "repo-local")
		_ = os.MkdirAll(repoA, 0755)

		cInit := exec.Command("git", "init", "-b", "main")
		cInit.Dir = repoA
		_ = cInit.Run()
		cRemote := exec.Command("git", "remote", "add", "origin", "https://github.com/user/repo-local.git")
		cRemote.Dir = repoA
		_ = cRemote.Run()
		cConfigU := exec.Command("git", "config", "user.name", "Test")
		cConfigU.Dir = repoA
		_ = cConfigU.Run()
		cConfigE := exec.Command("git", "config", "user.email", "test@test.com")
		cConfigE.Dir = repoA
		_ = cConfigE.Run()

		var gitCheckoutBranch string
		var gitPushed bool
		var prCreated bool

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				for i, a := range args {
					if a == "checkout" && i+2 < len(args) && args[i+1] == "-B" {
						gitCheckoutBranch = args[i+2]
						return "", "", 0, nil
					}
					if a == "push" {
						gitPushed = true
						return "", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			if len(args) > 1 && args[0] == "gh" && args[1] == "pr" && args[2] == "create" {
				prCreated = true
				return "https://github.com/user/repo-local/pull/1", "", 0, nil
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Local:    true,
			LocalDir: tmpDir,
			CreatePR: true,
		})
		if err != nil {
			t.Fatalf("ConfigureWithOptions local failed: %v", err)
		}
		if gitCheckoutBranch != "chore/repokit-sync" {
			t.Errorf("Expected checkout of chore/repokit-sync, got %q", gitCheckoutBranch)
		}
		if !gitPushed {
			t.Errorf("Expected git push of chore/repokit-sync")
		}
		if !prCreated {
			t.Errorf("Expected gh pr create to be called")
		}
	})

	t.Run("all_discovery", func(t *testing.T) {
		var listedUser bool
		var configuredSlugs []string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[1] == "api" && args[2] == "user" {
				return "octocat\n", "", 0, nil
			}
			if len(args) > 2 && args[0] == "gh" && args[1] == "repo" && args[2] == "list" {
				listedUser = true
				return `[{"nameWithOwner": "octocat/repo1"}, {"nameWithOwner": "octocat/repo2"}]`, "", 0, nil
			}
			if len(args) > 2 && args[0] == "gh" && args[1] == "api" {
				for _, a := range args {
					if strings.Contains(a, "octocat/repo1") || strings.Contains(a, "octocat/repo2") {
						configuredSlugs = append(configuredSlugs, a)
					}
				}
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			All:    true,
			DryRun: false,
		})
		if err != nil {
			t.Fatalf("ConfigureWithOptions All failed: %v", err)
		}
		if !listedUser {
			t.Errorf("Expected gh repo list to be called")
		}
		if len(configuredSlugs) == 0 {
			t.Errorf("Expected configuration API calls for discovered repos")
		}
	})

	t.Run("exempt_vault_in_multiple_repos", func(t *testing.T) {
		var configuredSlugs []string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[0] == "gh" && args[1] == "api" {
				for _, a := range args {
					if strings.Contains(a, "AgentVault") {
						configuredSlugs = append(configuredSlugs, a)
					}
				}
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Repos: []string{"Cooper-Wallace/AgentVault"},
		})
		if err != nil {
			t.Fatalf("ConfigureWithOptions failed: %v", err)
		}
		if len(configuredSlugs) > 0 {
			t.Errorf("Expected 0 API calls for exempt vault repo in multiple repos, got %v", configuredSlugs)
		}
	})
}

func TestPropagateRepositories(t *testing.T) {
	TestConfigureWithOptions(t)
}

func TestArchetypeEnterpriseOperabilityAndCoverage(t *testing.T) {
	archetypes := []string{"python", "go", "typescript", "bash", "java"}

	for _, arch := range archetypes {
		t.Run(arch+"/readme_operability", func(t *testing.T) {
			readmePath := fmt.Sprintf("templates/%s/README.md", arch)
			data, err := TemplatesFS.ReadFile(readmePath)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", readmePath, err)
			}
			content := string(data)
			requiredSections := []string{
				"## Example Workflow",
				"## Configuration, Failure Modes & Safe Degrade Behavior",
				"## Known Limitations",
			}
			for _, sec := range requiredSections {
				if !strings.Contains(content, sec) {
					t.Errorf("%s missing required section: %s", readmePath, sec)
				}
			}
		})

		t.Run(arch+"/gitignore_coverage", func(t *testing.T) {
			giPath := fmt.Sprintf("templates/%s/.gitignore", arch)
			data, err := TemplatesFS.ReadFile(giPath)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", giPath, err)
			}
			content := string(data)
			requiredEntries := []string{
				"coverage.out",
				".coverage",
				"htmlcov/",
				"coverage/",
			}
			for _, entry := range requiredEntries {
				if !strings.Contains(content, entry) {
					t.Errorf("%s missing coverage entry: %s", giPath, entry)
				}
			}
		})
	}

	t.Run("default_gitignore_coverage", func(t *testing.T) {
		requiredEntries := []string{
			"coverage.out",
			".coverage",
			"htmlcov/",
			"coverage/",
		}
		for _, entry := range requiredEntries {
			if !strings.Contains(DefaultGitignore, entry) {
				t.Errorf("DefaultGitignore missing coverage entry: %s", entry)
			}
		}
	})
}
