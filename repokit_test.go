package repokit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
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
		"release(1.1.0): candidate release",
		"release: v1.1.0",
		"release(v1.1.0): candidate release",
		"release: candidate release",
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

	t.Run("hub_repo_disables_auto_merge", func(t *testing.T) {
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
			t.Errorf("Expected allow_auto_merge to be false for hub repo")
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
	})

	t.Run("consumer_repo_enables_auto_merge", func(t *testing.T) {
		var capturedInput string
		var capturedArgs []string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			capturedArgs = args
			capturedInput = input
			return "{}", "", 0, nil
		}

		err := ApplyGitHubRepoSettings("Cooper-Wallace/my-repo")
		if err != nil {
			t.Fatalf("ApplyGitHubRepoSettings failed: %v", err)
		}

		expectedPrefix := []string{"gh", "api", "--method", "PATCH", "repos/Cooper-Wallace/my-repo", "--input", "-"}
		if len(capturedArgs) != len(expectedPrefix) {
			t.Errorf("Expected args %v, got %v", expectedPrefix, capturedArgs)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(capturedInput), &payload); err != nil {
			t.Fatalf("Failed to unmarshal captured input: %v", err)
		}

		if payload["allow_auto_merge"] != true {
			t.Errorf("Expected allow_auto_merge to be true for consumer repo, got %v", payload["allow_auto_merge"])
		}
		if payload["allow_squash_merge"] != true {
			t.Errorf("Expected allow_squash_merge to be true")
		}
	})
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

		bypassActors, ok := payload["bypass_actors"].([]interface{})
		if !ok || len(bypassActors) == 0 {
			t.Fatalf("Expected bypass_actors in payload, got %v", payload["bypass_actors"])
		}
		actor0 := bypassActors[0].(map[string]interface{})
		if actor0["actor_id"] != float64(5) || actor0["actor_type"] != "RepositoryRole" || actor0["bypass_mode"] != "always" {
			t.Errorf("Expected bypass_actors with actor_id: 5 and RepositoryRole, got %v", actor0)
		}

		rules := payload["rules"].([]interface{})
		var hasStatusChecks, hasPRSquash, hasCodeScanning bool
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
					c0 := checks[0].(map[string]interface{})
					c1 := checks[1].(map[string]interface{})
					if c0["context"] == "Quality Gate / Verification" && c1["context"] == "Quality Gate / Title Validation" {
						hasStatusChecks = true
					}
				}
			}
			if rMap["type"] == "code_scanning" {
				if params, ok := rMap["parameters"].(map[string]interface{}); ok {
					if tools, ok := params["code_scanning_tools"].([]interface{}); ok && len(tools) == 1 {
						t0, _ := tools[0].(map[string]interface{})
						if t0["tool"] == "CodeQL" &&
							t0["security_alerts_threshold"] == "high_or_higher" &&
							t0["alerts_threshold"] == "errors" {
							hasCodeScanning = true
						}
					}
				}
			}
		}
		if !hasPRSquash {
			t.Errorf("Expected pull_request rule with squash merge method")
		}
		if !hasStatusChecks {
			t.Errorf("Expected required_status_checks rule with Quality Gate checks")
		}
		if !hasCodeScanning {
			t.Errorf("Expected code_scanning rule with CodeQL")
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

		bypassActors, ok := payload["bypass_actors"].([]interface{})
		if !ok || len(bypassActors) == 0 {
			t.Fatalf("Expected bypass_actors in payload, got %v", payload["bypass_actors"])
		}
		actor0 := bypassActors[0].(map[string]interface{})
		if actor0["actor_id"] != float64(5) || actor0["actor_type"] != "RepositoryRole" || actor0["bypass_mode"] != "always" {
			t.Errorf("Expected bypass_actors with actor_id: 5 and RepositoryRole, got %v", actor0)
		}

		rules := payload["rules"].([]interface{})
		for _, r := range rules {
			rMap := r.(map[string]interface{})
			if rMap["type"] == "required_status_checks" {
				params := rMap["parameters"].(map[string]interface{})
				checks := params["required_status_checks"].([]interface{})
				c0 := checks[0].(map[string]interface{})
				if c0["context"] != "ci / Quality Gate / Verification" {
					t.Errorf("Expected 'ci / Quality Gate / Verification', got %v", c0["context"])
				}
				c1 := checks[1].(map[string]interface{})
				if c1["context"] != "validate-pr / Quality Gate / Title Validation" {
					t.Errorf("Expected 'validate-pr / Quality Gate / Title Validation', got %v", c1["context"])
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

	t.Run("fallback_without_code_scanning_on_private_repo", func(t *testing.T) {
		var calls [][]string
		var lastPayload map[string]interface{}
		var postCount int

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			calls = append(calls, args)
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "--method POST") || strings.Contains(joined, "--method PUT") {
				postCount++
				if postCount == 1 {
					// First attempt fails with code_scanning not supported
					return "", "HTTP 422: Rule type code_scanning is not supported for this repository", 1, errors.New("exit status 1")
				}
				// Second attempt (fallback retry) succeeds
				_ = json.Unmarshal([]byte(input), &lastPayload)
				return "{}", "", 0, nil
			}
			// GET rulesets
			return "[]", "", 0, nil
		}

		err := ApplyBranchRuleset("Cooper-Wallace/private-repo", "~DEFAULT_BRANCH", nil)
		if err != nil {
			t.Fatalf("Expected fallback retry to succeed, got %v", err)
		}

		if postCount != 2 {
			t.Fatalf("Expected 2 POST calls (fail then fallback), got %d", postCount)
		}

		rules, ok := lastPayload["rules"].([]interface{})
		if !ok {
			t.Fatalf("Expected rules in fallback payload")
		}
		for _, r := range rules {
			rMap := r.(map[string]interface{})
			if rMap["type"] == "code_scanning" {
				t.Errorf("Fallback payload should not contain code_scanning rule: %v", rMap)
			}
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
		{"Cooper-Wallace/Cooper-Wallace", true},
		{"cooper-wallace/cooper-wallace", true},
		{"cooper-wallace/Cooper-Wallace", true},
		{"octocat/octocat", true},
		{"user/user", true},
		{"Cooper-Wallace/agentkit", false},
		{"Cooper-Wallace/repokit", false},
		{"agentkit", false},
		{"my-agent-vault-app", false},
		{"", false},
		{"/", false},
		{"user/other", false},
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

func TestIsProfileRepo(t *testing.T) {
	cases := []struct {
		slug     string
		expected bool
	}{
		{"Cooper-Wallace/Cooper-Wallace", true},
		{"cooper-wallace/cooper-wallace", true},
		{"cooper-wallace/Cooper-Wallace", true},
		{"octocat/octocat", true},
		{"user/user", true},
		{"Cooper-Wallace/repokit", false},
		{"Cooper-Wallace/AgentVault", false},
		{"agentvault", false},
		{"", false},
		{"/", false},
		{"//", false},
		{"user/other", false},
		{"a/b/c", false},
	}

	for _, tc := range cases {
		name := tc.slug
		if name == "" {
			name = "empty_slug"
		}
		t.Run(name, func(t *testing.T) {
			actual := IsProfileRepo(tc.slug)
			if actual != tc.expected {
				t.Errorf("IsProfileRepo(%q) = %v; want %v", tc.slug, actual, tc.expected)
			}
		})
	}
}

func TestApplySecurityAndAnalysis(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("public_repo_success", func(t *testing.T) {
		var capturedArgs []string
		var capturedInput string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			capturedArgs = args
			capturedInput = input
			return "{}", "", 0, nil
		}

		err := ApplySecurityAndAnalysis("Cooper-Wallace/test-repo")
		if err != nil {
			t.Fatalf("Expected ApplySecurityAndAnalysis to succeed, got %v", err)
		}

		joined := strings.Join(capturedArgs, " ")
		if !strings.Contains(joined, "gh api --method PATCH repos/Cooper-Wallace/test-repo") {
			t.Errorf("Unexpected command args: %v", capturedArgs)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(capturedInput), &payload); err != nil {
			t.Fatalf("Failed to parse PATCH payload: %v", err)
		}

		sec, ok := payload["security_and_analysis"].(map[string]interface{})
		if !ok {
			t.Fatalf("Payload missing security_and_analysis: %v", payload)
		}
		ss, _ := sec["secret_scanning"].(map[string]interface{})
		ssp, _ := sec["secret_scanning_push_protection"].(map[string]interface{})
		dep, _ := sec["dependabot_security_updates"].(map[string]interface{})

		if ss["status"] != "enabled" {
			t.Errorf("Expected secret_scanning status 'enabled', got %v", ss["status"])
		}
		if ssp["status"] != "enabled" {
			t.Errorf("Expected secret_scanning_push_protection status 'enabled', got %v", ssp["status"])
		}
		if dep["status"] != "enabled" {
			t.Errorf("Expected dependabot_security_updates status 'enabled', got %v", dep["status"])
		}
	})

	t.Run("private_repo_safe_degrade", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "", "HTTP 422: Secret scanning is not available for this private repository", 1, errors.New("exit status 1")
		}

		err := ApplySecurityAndAnalysis("Cooper-Wallace/private-repo")
		if err != nil {
			t.Errorf("Expected safe degrade to return nil error, got: %v", err)
		}
	})

	t.Run("unexpected_api_error", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "", "HTTP 500: Internal Server Error", 1, errors.New("exit status 1")
		}

		err := ApplySecurityAndAnalysis("Cooper-Wallace/test-repo")
		if err == nil {
			t.Errorf("Expected unexpected API error to fail, but got nil")
		}
	})
}

func TestApplyCodeScanningDefaultSetup(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("consumer_public_repo_configures_default", func(t *testing.T) {
		var patchInput string
		var patchCalled bool

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "--method PATCH") {
				patchCalled = true
				patchInput = input
				return "{}", "", 0, nil
			}
			return `{"state":"not-configured","languages":["actions","python"]}`, "", 0, nil
		}

		configured, err := ApplyCodeScanningDefaultSetup("Cooper-Wallace/consumer-repo")
		if err != nil {
			t.Fatalf("Expected ApplyCodeScanningDefaultSetup to succeed, got %v", err)
		}
		if !configured {
			t.Errorf("Expected configured=true, got %v", configured)
		}
		if !patchCalled {
			t.Fatalf("Expected PATCH to be called")
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(patchInput), &payload); err != nil {
			t.Fatalf("Failed to parse patch payload: %v", err)
		}
		if payload["state"] != "configured" {
			t.Errorf("Expected state 'configured', got %v", payload["state"])
		}
		if payload["query_suite"] != "default" {
			t.Errorf("Expected query_suite 'default', got %v", payload["query_suite"])
		}
		if _, ok := payload["languages"]; ok {
			t.Errorf("Expected languages to not be overridden for consumer repo, got %v", payload["languages"])
		}
	})

	t.Run("governance_hub_filters_languages", func(t *testing.T) {
		var patchInput string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "--method PATCH") {
				patchInput = input
				return "{}", "", 0, nil
			}
			return `{"state":"not-configured","languages":["actions","go","python","java"]}`, "", 0, nil
		}

		configured, err := ApplyCodeScanningDefaultSetup("Cooper-Wallace/repokit")
		if err != nil {
			t.Fatalf("Expected ApplyCodeScanningDefaultSetup to succeed, got %v", err)
		}
		if !configured {
			t.Errorf("Expected configured=true, got %v", configured)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(patchInput), &payload); err != nil {
			t.Fatalf("Failed to parse patch payload: %v", err)
		}
		languages, ok := payload["languages"].([]interface{})
		if !ok || len(languages) != 2 || languages[0] != "actions" || languages[1] != "go" {
			t.Errorf("Expected languages ['actions', 'go'], got %v", payload["languages"])
		}
	})

	t.Run("private_repo_403_safe_degrade", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "", "Code scanning is not enabled for this repository (HTTP 403)", 1, errors.New("exit status 1")
		}

		configured, err := ApplyCodeScanningDefaultSetup("Cooper-Wallace/private-repo")
		if err != nil {
			t.Errorf("Expected error to be nil on safe degrade, got %v", err)
		}
		if configured {
			t.Errorf("Expected configured=false on safe degrade, got %v", configured)
		}
	})

	t.Run("exempt_vault_repo_skipped", func(t *testing.T) {
		callCount := 0
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			callCount++
			return "{}", "", 0, nil
		}

		configured, err := ApplyCodeScanningDefaultSetup("Cooper-Wallace/AgentVault")
		if err != nil {
			t.Errorf("Expected error to be nil for exempt vault repo, got %v", err)
		}
		if configured {
			t.Errorf("Expected configured=false for exempt vault repo, got %v", configured)
		}
		if callCount != 0 {
			t.Errorf("Expected 0 API calls for exempt vault repo, got %d", callCount)
		}
	})
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

	t.Run("pass_quality_gate_verification", func(t *testing.T) {
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
										{"context": "Quality Gate / Verification"},
										{"context": "Quality Gate / Title Validation"},
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
			t.Errorf("Expected remote audit to pass with Quality Gate / Verification, got passed=%v, err=%v", passed, err)
		}
	})

	t.Run("pass_prefixed_quality_gate", func(t *testing.T) {
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
										{"context": "ci / Quality Gate / Verification"},
										{"context": "validate-pr / Quality Gate / Title Validation"},
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
			t.Errorf("Expected remote audit to pass with prefixed Quality Gate checks, got passed=%v, err=%v", passed, err)
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

	t.Run("consumer_repo_auto_merge_passes", func(t *testing.T) {
		consumerTmp := t.TempDir()
		cInit := exec.Command("git", "init", "-b", "main")
		cInit.Dir = consumerTmp
		_ = cInit.Run()
		cRemote := exec.Command("git", "remote", "add", "origin", "https://github.com/Cooper-Wallace/my-repo.git")
		cRemote.Dir = consumerTmp
		_ = cRemote.Run()

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/my-repo" {
				data := map[string]interface{}{
					"allow_squash_merge":          true,
					"allow_merge_commit":          false,
					"allow_rebase_merge":          false,
					"delete_branch_on_merge":      true,
					"squash_merge_commit_title":   "PR_TITLE",
					"squash_merge_commit_message": "PR_BODY",
					"allow_auto_merge":            true,
				}
				bytes, _ := json.Marshal(data)
				return string(bytes), "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/my-repo/rulesets" {
				rulesets := []map[string]interface{}{
					{
						"name":        "default-branch-protection",
						"enforcement": "active",
						"rules": []map[string]interface{}{
							{
								"type": "required_status_checks",
								"parameters": map[string]interface{}{
									"required_status_checks": []map[string]string{
										{"context": "Quality Gate / Verification"},
									},
								},
							},
						},
					},
				}
				bytes, _ := json.Marshal(rulesets)
				return string(bytes), "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/my-repo/actions/permissions/workflow" {
				return `{"default_workflow_permissions": "read", "can_approve_pull_request_reviews": true}`, "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		passed, err := AuditRepository(consumerTmp, "", false, true)
		if err != nil || !passed {
			t.Errorf("Expected consumer repo remote audit to pass with allow_auto_merge=true, got passed=%v, err=%v", passed, err)
		}
	})

	t.Run("public_repo_security_and_codeql_gates_pass", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit" {
				data := map[string]interface{}{
					"visibility":                  "public",
					"allow_squash_merge":          true,
					"allow_merge_commit":          false,
					"allow_rebase_merge":          false,
					"delete_branch_on_merge":      true,
					"squash_merge_commit_title":   "PR_TITLE",
					"squash_merge_commit_message": "PR_BODY",
					"allow_auto_merge":            false,
					"security_and_analysis": map[string]interface{}{
						"secret_scanning":                 map[string]interface{}{"status": "enabled"},
						"secret_scanning_push_protection": map[string]interface{}{"status": "enabled"},
					},
				}
				bytes, _ := json.Marshal(data)
				return string(bytes), "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit/code-scanning/default-setup" {
				return `{"state": "configured"}`, "", 0, nil
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
							{
								"type": "code_scanning",
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
			t.Errorf("Expected public repo remote audit to pass, got passed=%v, err=%v", passed, err)
		}
	})

	t.Run("public_repo_missing_codeql_fails", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit" {
				data := map[string]interface{}{
					"visibility":                  "public",
					"allow_squash_merge":          true,
					"allow_merge_commit":          false,
					"allow_rebase_merge":          false,
					"delete_branch_on_merge":      true,
					"squash_merge_commit_title":   "PR_TITLE",
					"squash_merge_commit_message": "PR_BODY",
					"allow_auto_merge":            false,
					"security_and_analysis": map[string]interface{}{
						"secret_scanning":                 map[string]interface{}{"status": "enabled"},
						"secret_scanning_push_protection": map[string]interface{}{"status": "enabled"},
					},
				}
				bytes, _ := json.Marshal(data)
				return string(bytes), "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit/code-scanning/default-setup" {
				return `{"state": "not-configured"}`, "", 0, nil
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
							{
								"type": "code_scanning",
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
		if err != nil {
			t.Fatalf("AuditRepository returned unexpected error: %v", err)
		}
		if passed {
			t.Errorf("Expected public repo remote audit to fail when CodeQL is not configured, got passed=true")
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

	t.Run("env_owner_fallback", func(t *testing.T) {
		t.Setenv("GITHUB_REPOSITORY_OWNER", "ci-owner")
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "", "not logged in", 1, fmt.Errorf("exit status 1")
		}
		user, err := GetGitHubUser()
		if err != nil || user != "ci-owner" {
			t.Errorf("Expected user ci-owner from GITHUB_REPOSITORY_OWNER, got %s, err=%v", user, err)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Setenv("GITHUB_REPOSITORY_OWNER", "")
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			return "cooperwallace\n", "", 0, nil
		}
		user, err := GetGitHubUser()
		if err != nil || user != "cooperwallace" {
			t.Errorf("Expected user cooperwallace, got %s, err=%v", user, err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Setenv("GITHUB_REPOSITORY_OWNER", "")
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

	t.Run("direct_commit", func(t *testing.T) {
		gitDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = gitDir
		_ = cmd.Run()
		cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/Cooper-Wallace/test-repo.git")
		cmd.Dir = gitDir
		_ = cmd.Run()
		cmd = exec.Command("git", "config", "user.name", "Test")
		cmd.Dir = gitDir
		_ = cmd.Run()
		cmd = exec.Command("git", "config", "user.email", "test@test.com")
		cmd.Dir = gitDir
		_ = cmd.Run()

		var gitCheckoutMain bool
		var gitPushedMain bool
		var commitMsg string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				for i, a := range args {
					if a == "checkout" && i+1 < len(args) && args[i+1] == "main" {
						gitCheckoutMain = true
						return "", "", 0, nil
					}
					if a == "commit" && i+2 < len(args) && args[i+1] == "-m" {
						commitMsg = args[i+2]
					}
					if a == "push" && i+2 < len(args) && args[i+1] == "origin" && args[i+2] == "main" {
						gitPushedMain = true
						return "", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			return "[]", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			TargetDir:    gitDir,
			Force:        true,
			DirectCommit: true,
			ReleaseRef:   "v1.0.0",
		})
		if err != nil {
			t.Fatalf("ConfigureWithOptions direct commit failed: %v", err)
		}
		if !gitCheckoutMain {
			t.Errorf("Expected git checkout main")
		}
		if !gitPushedMain {
			t.Errorf("Expected git push origin main")
		}
		if !strings.Contains(commitMsg, "v1.0.0") {
			t.Errorf("Expected commit message with v1.0.0, got: %s", commitMsg)
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
		t.Setenv("GITHUB_REPOSITORY_OWNER", "")
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

		d1b := t.TempDir()
		_ = os.WriteFile(filepath.Join(d1b, "requirements.txt"), []byte("requests\n"), 0644)
		if DetectArchetype(d1b) != "python" {
			t.Errorf("Expected python from requirements.txt")
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
				return `{"allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": false, "delete_branch_on_merge": true, "squash_merge_commit_title": "PR_TITLE", "squash_merge_commit_message": "PR_BODY", "allow_auto_merge": true}`, "", 0, nil
			}
			if len(args) > 2 && args[2] == "repos/org/repo/rulesets" {
				return `[{"id": 22459068, "name": "default-branch-protection", "enforcement": "active"}]`, "", 0, nil
			}
			if len(args) > 2 && strings.HasPrefix(args[2], "repos/org/repo/rulesets/") {
				detailQueriedURL = args[2]
				return `{"rules": [{"type": "required_status_checks", "parameters": {"required_status_checks": [{"context": "Quality Gate / Verification"}]}}]}`, "", 0, nil
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

	hasArgPair := func(cmdArgs []string, flag, expectedVal string) bool {
		for i, a := range cmdArgs {
			if a == flag && i+1 < len(cmdArgs) && cmdArgs[i+1] == expectedVal {
				return true
			}
		}
		return false
	}

	hasArg := func(cmdArgs []string, flag string) bool {
		for _, a := range cmdArgs {
			if a == flag {
				return true
			}
		}
		return false
	}

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
		t.Setenv("GH_PAT", "ghp_test_token_123")
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
		var prAutoMerged bool
		var gitRemoteSetURL bool
		var prCreateArgs []string
		var prMergeArgs []string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				for i, a := range args {
					if a == "set-url" && i+2 < len(args) && strings.Contains(args[i+2], "x-access-token:ghp_test_token_123") {
						gitRemoteSetURL = true
						return "", "", 0, nil
					}
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
				prCreateArgs = args
				return "https://github.com/user/repo-local/pull/1", "", 0, nil
			}
			if len(args) > 2 && args[0] == "gh" && args[1] == "pr" && args[2] == "merge" {
				prAutoMerged = true
				prMergeArgs = args
				return "", "", 0, nil
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
		if !gitRemoteSetURL {
			t.Errorf("Expected git remote set-url to be called with authenticated URL")
		}
		if gitCheckoutBranch != "bot/repokit-sync" {
			t.Errorf("Expected checkout of bot/repokit-sync, got %q", gitCheckoutBranch)
		}
		if !gitPushed {
			t.Errorf("Expected git push of bot/repokit-sync")
		}
		if !prCreated {
			t.Errorf("Expected gh pr create to be called")
		}
		if !hasArgPair(prCreateArgs, "--repo", "user/repo-local") {
			t.Errorf("Expected --repo user/repo-local in gh pr create args, got: %v", prCreateArgs)
		}
		if !hasArgPair(prCreateArgs, "--head", "bot/repokit-sync") {
			t.Errorf("Expected --head bot/repokit-sync in gh pr create args, got: %v", prCreateArgs)
		}
		if !hasArgPair(prCreateArgs, "--title", "ci(repokit): automated sync of governance standards and workflows") {
			t.Errorf("Expected --title 'ci(repokit): automated sync of governance standards and workflows' in gh pr create args, got: %v", prCreateArgs)
		}
		if hasArg(prCreateArgs, "--base") {
			t.Errorf("Expected gh pr create not to pass --base, got: %v", prCreateArgs)
		}
		if !prAutoMerged {
			t.Errorf("Expected gh pr merge ... --auto --squash to be called")
		}
		if !hasArg(prMergeArgs, "--delete-branch") {
			t.Errorf("Expected --delete-branch in gh pr merge args, got: %v", prMergeArgs)
		}
	})

	t.Run("local_discovery_and_pr_creation_existing_pr", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoA := filepath.Join(tmpDir, "repo-local-exist")
		_ = os.MkdirAll(repoA, 0755)

		cInit := exec.Command("git", "init", "-b", "main")
		cInit.Dir = repoA
		_ = cInit.Run()
		cRemote := exec.Command("git", "remote", "add", "origin", "https://github.com/user/repo-local-exist.git")
		cRemote.Dir = repoA
		_ = cRemote.Run()
		cConfigU := exec.Command("git", "config", "user.name", "Test")
		cConfigU.Dir = repoA
		_ = cConfigU.Run()
		cConfigE := exec.Command("git", "config", "user.email", "test@test.com")
		cConfigE.Dir = repoA
		_ = cConfigE.Run()

		var prAutoMergedExisting bool
		var prCreateExistingArgs []string
		var prMergeExistingArgs []string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				for i, a := range args {
					if a == "checkout" && i+2 < len(args) && args[i+1] == "-B" {
						return "", "", 0, nil
					}
					if a == "push" {
						return "", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			if len(args) > 1 && args[0] == "gh" && args[1] == "pr" && args[2] == "create" {
				prCreateExistingArgs = args
				return "", "a pull request for branch \"bot/repokit-sync\" into branch \"main\" already exists", 1, nil
			}
			if len(args) > 2 && args[0] == "gh" && args[1] == "pr" && args[2] == "merge" {
				prAutoMergedExisting = true
				prMergeExistingArgs = args
				return "", "", 0, nil
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
		if !hasArgPair(prCreateExistingArgs, "--repo", "user/repo-local-exist") {
			t.Errorf("Expected --repo user/repo-local-exist in gh pr create args, got: %v", prCreateExistingArgs)
		}
		if !hasArgPair(prCreateExistingArgs, "--head", "bot/repokit-sync") {
			t.Errorf("Expected --head bot/repokit-sync in gh pr create args, got: %v", prCreateExistingArgs)
		}
		if !hasArgPair(prCreateExistingArgs, "--title", "ci(repokit): automated sync of governance standards and workflows") {
			t.Errorf("Expected --title 'ci(repokit): automated sync of governance standards and workflows' in gh pr create args, got: %v", prCreateExistingArgs)
		}
		if hasArg(prCreateExistingArgs, "--base") {
			t.Errorf("Expected gh pr create not to pass --base, got: %v", prCreateExistingArgs)
		}
		if !prAutoMergedExisting {
			t.Errorf("Expected gh pr merge --auto --squash to be called when PR already exists")
		}
		if !hasArg(prMergeExistingArgs, "--delete-branch") {
			t.Errorf("Expected --delete-branch in gh pr merge args, got: %v", prMergeExistingArgs)
		}
	})

	t.Run("local_discovery_and_direct_commit", func(t *testing.T) {
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

		var gitCheckoutMain bool
		var gitPushedMain bool
		var commitMsg string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				for i, a := range args {
					if a == "checkout" && i+1 < len(args) && args[i+1] == "main" {
						gitCheckoutMain = true
						return "", "", 0, nil
					}
					if a == "commit" && i+2 < len(args) && args[i+1] == "-m" {
						commitMsg = args[i+2]
					}
					if a == "push" && i+2 < len(args) && args[i+1] == "origin" && args[i+2] == "main" {
						gitPushedMain = true
						return "", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Local:        true,
			LocalDir:     tmpDir,
			DirectCommit: true,
			ReleaseRef:   "v1.2.0",
		})
		if err != nil {
			t.Fatalf("ConfigureWithOptions local direct commit failed: %v", err)
		}
		if !gitCheckoutMain {
			t.Errorf("Expected checkout of main")
		}
		if !gitPushedMain {
			t.Errorf("Expected git push origin main")
		}
		if !strings.Contains(commitMsg, "sync governance standards and workflows to v1.2.0") {
			t.Errorf("Expected commit message with release ref v1.2.0, got: %s", commitMsg)
		}
	})

	t.Run("ephemeral_clone_and_direct_commit", func(t *testing.T) {
		var clonedRepo string
		var gitPushedMain bool
		var commitMsg string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 4 && args[0] == "gh" && args[1] == "repo" && args[2] == "clone" {
				clonedRepo = args[3]
				dest := args[4]
				_ = exec.Command("git", "init", "-b", "main", dest).Run()
				_ = exec.Command("git", "-C", dest, "remote", "add", "origin", "https://github.com/"+clonedRepo+".git").Run()
				_ = exec.Command("git", "-C", dest, "config", "user.name", "Test").Run()
				_ = exec.Command("git", "-C", dest, "config", "user.email", "test@test.com").Run()
				return "", "", 0, nil
			}
			if len(args) > 0 && args[0] == "git" {
				for i, a := range args {
					if a == "commit" && i+2 < len(args) && args[i+1] == "-m" {
						commitMsg = args[i+2]
					}
					if a == "push" && i+2 < len(args) && args[i+1] == "origin" && args[i+2] == "main" {
						gitPushedMain = true
						return "", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Repos:        []string{"user/remote-sync-repo"},
			DirectCommit: true,
			ReleaseRef:   "v1.0.0",
		})
		if err != nil {
			t.Fatalf("Ephemeral clone direct commit failed: %v", err)
		}
		if clonedRepo != "user/remote-sync-repo" {
			t.Errorf("Expected ephemeral clone of user/remote-sync-repo, got %q", clonedRepo)
		}
		if !gitPushedMain {
			t.Errorf("Expected push to origin main")
		}
		if !strings.Contains(commitMsg, "v1.0.0") {
			t.Errorf("Expected commit message to contain v1.0.0, got %s", commitMsg)
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

	t.Run("git_author_identity_configured_when_unset", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoA := filepath.Join(tmpDir, "repo-no-ident")
		_ = os.MkdirAll(repoA, 0755)

		cInit := exec.Command("git", "init", "-b", "main")
		cInit.Dir = repoA
		_ = cInit.Run()
		cRemote := exec.Command("git", "remote", "add", "origin", "https://github.com/user/repo-no-ident.git")
		cRemote.Dir = repoA
		_ = cRemote.Run()
		// Intentionally do NOT configure user.name and user.email in repoA

		var setUserName, setUserEmail, prAutoMerged bool
		var prCreateArgs, prMergeArgs []string

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 6 && args[0] == "git" && args[3] == "config" {
				if args[4] == "user.name" && args[5] == "github-actions[bot]" {
					setUserName = true
				}
				if args[4] == "user.email" && args[5] == "41898282+github-actions[bot]@users.noreply.github.com" {
					setUserEmail = true
				}
			}
			if len(args) == 5 && args[0] == "git" && args[3] == "config" && args[4] == "user.name" {
				return "", "", 0, nil
			}
			if len(args) > 0 && args[0] == "git" {
				for i, a := range args {
					if a == "checkout" && i+2 < len(args) && args[i+1] == "-B" {
						return "", "", 0, nil
					}
					if a == "push" {
						return "", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			if len(args) > 1 && args[0] == "gh" && args[1] == "pr" && args[2] == "create" {
				prCreateArgs = args
				return "https://github.com/user/repo-no-ident/pull/1", "", 0, nil
			}
			if len(args) > 2 && args[0] == "gh" && args[1] == "pr" && args[2] == "merge" {
				prAutoMerged = true
				prMergeArgs = args
				return "", "", 0, nil
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Local:    true,
			LocalDir: tmpDir,
			CreatePR: true,
		})
		if err != nil {
			t.Fatalf("ConfigureWithOptions failed: %v", err)
		}
		if !setUserName {
			t.Errorf("Expected git user.name to be configured as github-actions[bot]")
		}
		if !setUserEmail {
			t.Errorf("Expected git user.email to be configured as 41898282+github-actions[bot]@users.noreply.github.com")
		}
		if !hasArgPair(prCreateArgs, "--repo", "user/repo-no-ident") {
			t.Errorf("Expected --repo user/repo-no-ident in gh pr create args, got: %v", prCreateArgs)
		}
		if !hasArgPair(prCreateArgs, "--head", "bot/repokit-sync") {
			t.Errorf("Expected --head bot/repokit-sync in gh pr create args, got: %v", prCreateArgs)
		}
		if !hasArgPair(prCreateArgs, "--title", "ci(repokit): automated sync of governance standards and workflows") {
			t.Errorf("Expected --title 'ci(repokit): automated sync of governance standards and workflows' in gh pr create args, got: %v", prCreateArgs)
		}
		if hasArg(prCreateArgs, "--base") {
			t.Errorf("Expected gh pr create not to pass --base, got: %v", prCreateArgs)
		}
		if !prAutoMerged {
			t.Errorf("Expected gh pr merge to be called")
		}
		if !hasArg(prMergeArgs, "--delete-branch") {
			t.Errorf("Expected --delete-branch in gh pr merge args, got: %v", prMergeArgs)
		}
	})

	t.Run("governance_hub_skips_local_cloning_and_sync_pr", func(t *testing.T) {
		var cloned, prCreated, remoteConfigured bool

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "repo" && args[2] == "clone" {
				cloned = true
				return "", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "pr" {
				prCreated = true
				return "", "", 0, nil
			}
			if len(args) >= 2 && args[0] == "gh" && args[1] == "api" {
				remoteConfigured = true
				return "[]", "", 0, nil
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Repos:    []string{"Cooper-Wallace/repokit", "test-user/repokit"},
			CreatePR: true,
		})
		if err != nil {
			t.Fatalf("ConfigureWithOptions for hub failed: %v", err)
		}
		if cloned {
			t.Errorf("Expected governance hub to NOT be ephemerally cloned")
		}
		if prCreated {
			t.Errorf("Expected governance hub to NOT create or merge sync PR")
		}
		if !remoteConfigured {
			t.Errorf("Expected remote settings and rulesets to be applied for governance hub")
		}
	})

	t.Run("profile_repo_exempt_from_branch_rulesets_and_scaffolding", func(t *testing.T) {
		var apiCalled bool
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 2 && args[0] == "gh" && args[1] == "api" {
				apiCalled = true
			}
			return "[]", "", 0, nil
		}
		err := ApplyBranchRuleset("Cooper-Wallace/Cooper-Wallace", "~DEFAULT_BRANCH", nil)
		if err != nil {
			t.Fatalf("ApplyBranchRuleset failed: %v", err)
		}
		if apiCalled {
			t.Errorf("Expected ApplyBranchRuleset to skip API calls for profile repo")
		}
	})

	t.Run("configure_multiple_wait_for_merge_success", func(t *testing.T) {
		oldPoll := SyncPRPollInterval
		SyncPRPollInterval = 5 * time.Millisecond
		defer func() { SyncPRPollInterval = oldPoll }()

		tmpDir := t.TempDir()
		repoA := filepath.Join(tmpDir, "repo-sync-wait")
		_ = os.MkdirAll(repoA, 0755)

		cInit := exec.Command("git", "init", "-b", "main")
		cInit.Dir = repoA
		_ = cInit.Run()
		cRemote := exec.Command("git", "remote", "add", "origin", "https://github.com/user/repo-sync-wait.git")
		cRemote.Dir = repoA
		_ = cRemote.Run()
		cConfigU := exec.Command("git", "config", "user.name", "Test")
		cConfigU.Dir = repoA
		_ = cConfigU.Run()
		cConfigE := exec.Command("git", "config", "user.email", "test@test.com")
		cConfigE.Dir = repoA
		_ = cConfigE.Run()

		var prViewCalled bool
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				for _, a := range args {
					if a == "push" || a == "checkout" || a == "set-url" {
						return "", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			if len(args) >= 6 && args[0] == "gh" && args[1] == "pr" && args[2] == "view" {
				prViewCalled = true
				return `{"number": 1, "url": "https://github.com/user/repo-sync-wait/pull/1", "state": "MERGED"}`, "", 0, nil
			}
			if len(args) > 1 && args[0] == "gh" && args[1] == "pr" && args[2] == "create" {
				return "https://github.com/user/repo-sync-wait/pull/1", "", 0, nil
			}
			if len(args) > 2 && args[0] == "gh" && args[1] == "pr" && args[2] == "merge" {
				return "", "", 0, nil
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Local:        true,
			LocalDir:     tmpDir,
			CreatePR:     true,
			WaitForMerge: true,
			MergeTimeout: 500 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("Expected ConfigureWithOptions with WaitForMerge to succeed, got: %v", err)
		}
		if !prViewCalled {
			t.Errorf("Expected gh pr view to be called while waiting for merge")
		}
	})

	t.Run("configure_multiple_wait_for_merge_failure", func(t *testing.T) {
		oldPoll := SyncPRPollInterval
		SyncPRPollInterval = 5 * time.Millisecond
		defer func() { SyncPRPollInterval = oldPoll }()

		tmpDir := t.TempDir()
		repoA := filepath.Join(tmpDir, "repo-sync-fail")
		_ = os.MkdirAll(repoA, 0755)

		cInit := exec.Command("git", "init", "-b", "main")
		cInit.Dir = repoA
		_ = cInit.Run()
		cRemote := exec.Command("git", "remote", "add", "origin", "https://github.com/user/repo-sync-fail.git")
		cRemote.Dir = repoA
		_ = cRemote.Run()
		cConfigU := exec.Command("git", "config", "user.name", "Test")
		cConfigU.Dir = repoA
		_ = cConfigU.Run()
		cConfigE := exec.Command("git", "config", "user.email", "test@test.com")
		cConfigE.Dir = repoA
		_ = cConfigE.Run()

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				for _, a := range args {
					if a == "push" || a == "checkout" || a == "set-url" {
						return "", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			if len(args) >= 6 && args[0] == "gh" && args[1] == "pr" && args[2] == "view" {
				return `{"number": 1, "url": "https://github.com/user/repo-sync-fail/pull/1", "state": "OPEN", "statusCheckRollup": [{"name": "Verification", "status": "COMPLETED", "conclusion": "FAILURE", "detailsUrl": "https://ci/job/1"}]}`, "", 0, nil
			}
			if len(args) > 1 && args[0] == "gh" && args[1] == "pr" && args[2] == "create" {
				return "https://github.com/user/repo-sync-fail/pull/1", "", 0, nil
			}
			if len(args) > 2 && args[0] == "gh" && args[1] == "pr" && args[2] == "merge" {
				return "", "", 0, nil
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			Local:        true,
			LocalDir:     tmpDir,
			CreatePR:     true,
			WaitForMerge: true,
			MergeTimeout: 500 * time.Millisecond,
		})
		if err == nil {
			t.Fatalf("Expected ConfigureWithOptions to fail when downstream PR fails CI, but got nil")
		}
		if !strings.Contains(err.Error(), "user/repo-sync-fail") {
			t.Errorf("Expected error message to mention failed repo slug, got: %v", err)
		}
	})

	t.Run("configure_single_repo_wait_for_merge_success", func(t *testing.T) {
		oldPoll := SyncPRPollInterval
		SyncPRPollInterval = 5 * time.Millisecond
		defer func() { SyncPRPollInterval = oldPoll }()

		tmpDir := t.TempDir()
		cInit := exec.Command("git", "init", "-b", "main")
		cInit.Dir = tmpDir
		_ = cInit.Run()
		cRemote := exec.Command("git", "remote", "add", "origin", "https://github.com/user/single-repo-wait.git")
		cRemote.Dir = tmpDir
		_ = cRemote.Run()

		var prViewCalled bool
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				for _, a := range args {
					if a == "push" || a == "checkout" || a == "commit" || a == "add" {
						return "", "", 0, nil
					}
					if a == "status" {
						return " M README.md\n", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			if len(args) >= 6 && args[0] == "gh" && args[1] == "pr" && args[2] == "view" {
				prViewCalled = true
				return `{"number": 2, "url": "https://github.com/user/single-repo-wait/pull/2", "state": "MERGED"}`, "", 0, nil
			}
			if len(args) > 1 && args[0] == "gh" && args[1] == "pr" && args[2] == "create" {
				return "https://github.com/user/single-repo-wait/pull/2", "", 0, nil
			}
			if len(args) > 2 && args[0] == "gh" && args[1] == "pr" && args[2] == "merge" {
				return "", "", 0, nil
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			TargetDir:    tmpDir,
			CreatePR:     true,
			WaitForMerge: true,
			MergeTimeout: 500 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("Expected single repo ConfigureWithOptions with WaitForMerge to succeed, got: %v", err)
		}
		if !prViewCalled {
			t.Errorf("Expected gh pr view to be called for single repo wait")
		}
	})

	t.Run("configure_single_repo_wait_for_merge_failure", func(t *testing.T) {
		oldPoll := SyncPRPollInterval
		SyncPRPollInterval = 5 * time.Millisecond
		defer func() { SyncPRPollInterval = oldPoll }()

		tmpDir := t.TempDir()
		cInit := exec.Command("git", "init", "-b", "main")
		cInit.Dir = tmpDir
		_ = cInit.Run()
		cRemote := exec.Command("git", "remote", "add", "origin", "https://github.com/user/single-repo-fail.git")
		cRemote.Dir = tmpDir
		_ = cRemote.Run()

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 0 && args[0] == "git" {
				for _, a := range args {
					if a == "push" || a == "checkout" || a == "commit" || a == "add" {
						return "", "", 0, nil
					}
					if a == "status" {
						return " M README.md\n", "", 0, nil
					}
				}
				return defaultCommandRunner(args, dir, input)
			}
			if len(args) >= 6 && args[0] == "gh" && args[1] == "pr" && args[2] == "view" {
				return `{"number": 3, "url": "https://github.com/user/single-repo-fail/pull/3", "state": "OPEN", "statusCheckRollup": [{"name": "CI", "status": "COMPLETED", "conclusion": "FAILURE"}]}`, "", 0, nil
			}
			if len(args) > 1 && args[0] == "gh" && args[1] == "pr" && args[2] == "create" {
				return "https://github.com/user/single-repo-fail/pull/3", "", 0, nil
			}
			if len(args) > 2 && args[0] == "gh" && args[1] == "pr" && args[2] == "merge" {
				return "", "", 0, nil
			}
			return "{}", "", 0, nil
		}

		err := ConfigureWithOptions(ConfigureOptions{
			TargetDir:    tmpDir,
			CreatePR:     true,
			WaitForMerge: true,
			MergeTimeout: 500 * time.Millisecond,
		})
		if err == nil {
			t.Fatalf("Expected single repo ConfigureWithOptions to fail when CI fails, got nil")
		}
		if !strings.Contains(err.Error(), "user/single-repo-fail") {
			t.Errorf("Expected error to mention user/single-repo-fail, got: %v", err)
		}
	})
}

func TestEnsureGitAuthorIdentity(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("configures_identity_when_unset", func(t *testing.T) {
		var executedCmds [][]string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			executedCmds = append(executedCmds, args)
			if len(args) >= 5 && args[0] == "git" && args[3] == "config" && args[4] == "user.name" && len(args) == 5 {
				return "", "", 0, nil
			}
			return "", "", 0, nil
		}

		ensureGitAuthorIdentity("/tmp/test-repo")

		var setUserName, setUserEmail bool
		for _, cmd := range executedCmds {
			if len(cmd) == 6 && cmd[0] == "git" && cmd[1] == "-C" && cmd[2] == "/tmp/test-repo" && cmd[3] == "config" {
				if cmd[4] == "user.name" && cmd[5] == "github-actions[bot]" {
					setUserName = true
				}
				if cmd[4] == "user.email" && cmd[5] == "41898282+github-actions[bot]@users.noreply.github.com" {
					setUserEmail = true
				}
			}
		}

		if !setUserName {
			t.Errorf("Expected user.name to be set to github-actions[bot], executed commands: %v", executedCmds)
		}
		if !setUserEmail {
			t.Errorf("Expected user.email to be set to 41898282+github-actions[bot]@users.noreply.github.com, executed commands: %v", executedCmds)
		}
	})

	t.Run("preserves_existing_identity_when_configured", func(t *testing.T) {
		var executedCmds [][]string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			executedCmds = append(executedCmds, args)
			if len(args) >= 5 && args[0] == "git" && args[3] == "config" && args[4] == "user.name" && len(args) == 5 {
				return "Existing User\n", "", 0, nil
			}
			return "", "", 0, nil
		}

		ensureGitAuthorIdentity("/tmp/test-repo")

		for _, cmd := range executedCmds {
			if len(cmd) == 6 && cmd[0] == "git" && cmd[3] == "config" {
				if cmd[4] == "user.name" || cmd[4] == "user.email" {
					t.Errorf("Expected existing identity not to be overwritten, but got command: %v", cmd)
				}
			}
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

func TestParseSemVer(t *testing.T) {
	cases := []struct {
		tag       string
		wantMajor int
		wantMinor int
		wantPatch int
		wantOk    bool
	}{
		{"v1.2.3", 1, 2, 3, true},
		{"1.2.3", 1, 2, 3, true},
		{"v0.0.1", 0, 0, 1, true},
		{"v0.0.0", 0, 0, 0, true},
		{"v10.20.30", 10, 20, 30, true},
		{"v1.2", 0, 0, 0, false},
		{"1.2", 0, 0, 0, false},
		{"v1.2.3.4", 0, 0, 0, false},
		{"", 0, 0, 0, false},
		{"v1.2.a", 0, 0, 0, false},
		{"v1.2.3-beta", 0, 0, 0, false},
	}

	for _, c := range cases {
		t.Run(c.tag, func(t *testing.T) {
			major, minor, patch, ok := ParseSemVer(c.tag)
			if ok != c.wantOk {
				t.Fatalf("ParseSemVer(%q) ok = %v, want %v", c.tag, ok, c.wantOk)
			}
			if ok && (major != c.wantMajor || minor != c.wantMinor || patch != c.wantPatch) {
				t.Errorf("ParseSemVer(%q) = (%d, %d, %d), want (%d, %d, %d)", c.tag, major, minor, patch, c.wantMajor, c.wantMinor, c.wantPatch)
			}
		})
	}
}

func TestCalculateNextVersion(t *testing.T) {
	t.Run("first release on empty tag", func(t *testing.T) {
		major, minor, patch, bumped := CalculateNextVersion("", []string{"feat: first feature"})
		if !bumped || major != 1 || minor != 0 || patch != 0 {
			t.Errorf("Expected 1.0.0 bumped, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}

		major, minor, patch, bumped = CalculateNextVersion("", []string{"fix: first fix"})
		if !bumped || major != 1 || minor != 0 || patch != 0 {
			t.Errorf("Expected 1.0.0 bumped for initial fix, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}

		major, minor, patch, bumped = CalculateNextVersion("", []string{"feat!: first breaking"})
		if !bumped || major != 1 || minor != 0 || patch != 0 {
			t.Errorf("Expected 1.0.0 bumped for initial breaking, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}

		_, _, _, bumped = CalculateNextVersion("", []string{"docs: update readme", "chore: setup"})
		if bumped {
			t.Errorf("Expected bumped=false for non-releasable commits on empty tag")
		}
	})

	t.Run("first release on 0.0.0 tag", func(t *testing.T) {
		major, minor, patch, bumped := CalculateNextVersion("v0.0.0", []string{"feat: init"})
		if !bumped || major != 1 || minor != 0 || patch != 0 {
			t.Errorf("Expected 1.0.0 bumped from v0.0.0, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}
	})

	t.Run("bumps on existing v1.0.0", func(t *testing.T) {
		major, minor, patch, bumped := CalculateNextVersion("v1.0.0", []string{"fix: correct bug"})
		if !bumped || major != 1 || minor != 0 || patch != 1 {
			t.Errorf("Expected 1.0.1, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}

		for _, typ := range []string{"perf", "refactor", "ci", "build"} {
			m, mi, p, b := CalculateNextVersion("v1.0.0", []string{typ + ": update something"})
			if !b || m != 1 || mi != 0 || p != 1 {
				t.Errorf("Expected 1.0.1 for %s, got %d.%d.%d, bumped=%v", typ, m, mi, p, b)
			}
		}

		major, minor, patch, bumped = CalculateNextVersion("v1.0.0", []string{"feat: new feature", "fix: also a fix"})
		if !bumped || major != 1 || minor != 1 || patch != 0 {
			t.Errorf("Expected 1.1.0, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}

		major, minor, patch, bumped = CalculateNextVersion("v1.0.0", []string{"feat!: break things", "feat: normal"})
		if !bumped || major != 2 || minor != 0 || patch != 0 {
			t.Errorf("Expected 2.0.0, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}

		major, minor, patch, bumped = CalculateNextVersion("v1.0.0", []string{"fix: change api\n\nBREAKING CHANGE: new api"})
		if !bumped || major != 2 || minor != 0 || patch != 0 {
			t.Errorf("Expected 2.0.0, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}

		major, minor, patch, bumped = CalculateNextVersion("v1.0.0", []string{"chore(release): release v1.0.0", "docs: readme"})
		if bumped {
			t.Errorf("Expected bumped=false when only chore(release) and docs exist, got %d.%d.%d", major, minor, patch)
		}
	})

	t.Run("pre-1.0 tag progression", func(t *testing.T) {
		major, minor, patch, bumped := CalculateNextVersion("v0.1.0", []string{"feat: add widget"})
		if !bumped || major != 0 || minor != 2 || patch != 0 {
			t.Errorf("Expected 0.2.0, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}

		major, minor, patch, bumped = CalculateNextVersion("v0.1.0", []string{"fix: fix widget"})
		if !bumped || major != 0 || minor != 1 || patch != 1 {
			t.Errorf("Expected 0.1.1, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}

		major, minor, patch, bumped = CalculateNextVersion("v0.1.0", []string{"feat!: rewrite everything"})
		if !bumped || major != 1 || minor != 0 || patch != 0 {
			t.Errorf("Expected 1.0.0, got %d.%d.%d, bumped=%v", major, minor, patch, bumped)
		}
	})
}

func TestFormatChangelogSection(t *testing.T) {
	commits := []string{
		"1111111|feat(auth)!: replace token format",
		"2222222|feat(nav): add goal pose callback",
		"3333333|fix: prevent null pointer exception",
		"4444444|perf(db): cache query results",
		"5555555|refactor(core): simplify handler dispatch",
		"6666666|ci: add release-stage step",
		"7777777|chore: update linter rules",
		"8888888|chore(release): release v1.0.0",
		"just a simple fix without sha",
	}

	section := FormatChangelogSection("1.1.0", "2026-11-04", commits)

	if !strings.HasPrefix(section, "## 1.1.0 (2026-11-04)\n\n") {
		t.Errorf("Missing expected header in section: %s", section)
	}
	if !strings.Contains(section, "### Breaking Changes\n\n* **auth:** replace token format (1111111)") {
		t.Errorf("Missing breaking change in section: %s", section)
	}
	if !strings.Contains(section, "### Features\n\n* **nav:** add goal pose callback (2222222)") {
		t.Errorf("Missing features in section: %s", section)
	}
	if !strings.Contains(section, "### Bug Fixes\n\n* prevent null pointer exception (3333333)") {
		t.Errorf("Missing bug fixes in section: %s", section)
	}
	if !strings.Contains(section, "### Performance & Refactoring\n\n* **db:** cache query results (4444444)\n* **core:** simplify handler dispatch (5555555)") {
		t.Errorf("Missing perf/refactor in section: %s", section)
	}
	if !strings.Contains(section, "### Maintenance & CI\n\n* add release-stage step (6666666)\n* update linter rules (7777777)\n* just a simple fix without sha") {
		t.Errorf("Missing maintenance & ci in section: %s", section)
	}
	if strings.Contains(section, "chore(release): release v1.0.0") {
		t.Errorf("Section should not contain chore(release) commit: %s", section)
	}
}

func TestUpdateChangelogContent(t *testing.T) {
	newSec := "## 1.1.0 (2026-11-04)\n\n### Features\n\n* **nav:** add goal (1234567)\n"

	t.Run("empty changelog", func(t *testing.T) {
		res := UpdateChangelogContent("", "1.1.0", newSec)
		expected := "# Changelog\n\n## 1.1.0 (2026-11-04)\n\n### Features\n\n* **nav:** add goal (1234567)\n"
		if res != expected {
			t.Errorf("Expected %q, got %q", expected, res)
		}
	})

	t.Run("prepend under existing changelog", func(t *testing.T) {
		existing := "# Changelog\n\n## 1.0.0 (2026-11-01)\n\n### Features\n\n* init\n"
		res := UpdateChangelogContent(existing, "1.1.0", newSec)
		if !strings.HasPrefix(res, "# Changelog\n\n## 1.1.0 (2026-11-04)") {
			t.Errorf("Expected 1.1.0 prepended, got: %s", res)
		}
		if !strings.Contains(res, "## 1.0.0 (2026-11-01)") {
			t.Errorf("Expected 1.0.0 preserved, got: %s", res)
		}
	})

	t.Run("idempotent replacement of existing section", func(t *testing.T) {
		existing := "# Changelog\n\n## 1.1.0 (2026-11-03)\n\n### Old Features\n\n* old (0000000)\n\n## 1.0.0 (2026-11-01)\n\n### Features\n\n* init\n"
		res := UpdateChangelogContent(existing, "1.1.0", newSec)
		if strings.Contains(res, "Old Features") {
			t.Errorf("Expected old 1.1.0 section replaced, got: %s", res)
		}
		if !strings.Contains(res, "## 1.1.0 (2026-11-04)") {
			t.Errorf("Expected new 1.1.0 section present, got: %s", res)
		}
		if !strings.Contains(res, "## 1.0.0 (2026-11-01)") {
			t.Errorf("Expected 1.0.0 section preserved, got: %s", res)
		}
	})
}

func TestExecuteRelease(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("Mode A: Release Promotion", func(t *testing.T) {
		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "github_output")
		changelogPath := filepath.Join(tmpDir, "CHANGELOG.md")
		_ = os.WriteFile(changelogPath, []byte("# Changelog\n\n## 1.1.0 (2026-11-04)\n\n### Features\n\n* new feature (abc1234)\n"), 0644)

		var executedCmds [][]string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			executedCmds = append(executedCmds, args)
			if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "rev-parse" {
				return tmpDir + "\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" {
				return "release(v1.1.0): candidate release\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "rev-parse" {
				// tag does not exist yet
				return "", "", 1, nil
			}
			if len(args) >= 2 && args[0] == "gh" && args[1] == "release" && args[2] == "view" {
				// release does not exist yet
				return "", "", 1, nil
			}
			return "", "", 0, nil
		}

		opts := ReleaseOptions{
			Path:       tmpDir,
			OutputFile: outFile,
			DryRun:     false,
		}

		res, err := ExecuteRelease(opts)
		if err != nil {
			t.Fatalf("ExecuteRelease failed: %v", err)
		}
		if !res.ReleasesCreated {
			t.Fatalf("Expected ReleasesCreated=true in Mode A")
		}
		if res.TagName != "v1.1.0" || res.Version != "1.1.0" {
			t.Errorf("Expected v1.1.0, got %s / %s", res.TagName, res.Version)
		}
		if res.Major != 1 || res.Minor != 1 || res.Patch != 0 {
			t.Errorf("Expected 1.1.0, got %d.%d.%d", res.Major, res.Minor, res.Patch)
		}
		if !strings.Contains(res.ChangelogEntry, "new feature (abc1234)") {
			t.Errorf("Expected changelog notes, got %s", res.ChangelogEntry)
		}

		outData, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}
		outStr := string(outData)
		if !strings.Contains(outStr, "releases_created=true") || !strings.Contains(outStr, "version=1.1.0") {
			t.Errorf("Unexpected GITHUB_OUTPUT content: %s", outStr)
		}

		// Verify git tag & push, and gh release create were called
		foundTag := false
		foundGhRelease := false
		foundMajorTag := false
		foundMajorPush := false
		for _, cmd := range executedCmds {
			if len(cmd) >= 3 && cmd[0] == "git" && cmd[1] == "tag" && cmd[2] == "-a" {
				foundTag = true
			}
			if len(cmd) >= 5 && cmd[0] == "git" && cmd[1] == "tag" && cmd[2] == "-f" && cmd[3] == "-a" && cmd[4] == "v1" {
				foundMajorTag = true
			}
			if len(cmd) >= 5 && cmd[0] == "git" && cmd[1] == "push" && cmd[2] == "--force" && cmd[3] == "origin" && cmd[4] == "v1" {
				foundMajorPush = true
			}
			if len(cmd) >= 3 && cmd[0] == "gh" && cmd[1] == "release" && cmd[2] == "create" {
				foundGhRelease = true
			}
		}
		if !foundTag {
			t.Errorf("Expected git tag -a to be executed")
		}
		if !foundMajorTag {
			t.Errorf("Expected floating major tag git tag -f -a v1 to be executed")
		}
		if !foundMajorPush {
			t.Errorf("Expected floating major tag push git push --force origin v1 to be executed")
		}
		if !foundGhRelease {
			t.Errorf("Expected gh release create to be executed")
		}
	})

	t.Run("Mode A: Release Promotion - Legacy Format", func(t *testing.T) {
		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "github_output")
		changelogPath := filepath.Join(tmpDir, "CHANGELOG.md")
		_ = os.WriteFile(changelogPath, []byte("# Changelog\n\n## 1.1.0 (2026-11-04)\n\n### Features\n\n* new feature (abc1234)\n"), 0644)

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "rev-parse" {
				return tmpDir + "\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" {
				return "chore(release): release v1.1.0\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "rev-parse" {
				return "", "", 1, nil
			}
			if len(args) >= 2 && args[0] == "gh" && args[1] == "release" && args[2] == "view" {
				return "", "", 1, nil
			}
			return "", "", 0, nil
		}

		opts := ReleaseOptions{
			Path:       tmpDir,
			OutputFile: outFile,
			DryRun:     false,
		}

		res, err := ExecuteRelease(opts)
		if err != nil {
			t.Fatalf("ExecuteRelease failed: %v", err)
		}
		if !res.ReleasesCreated {
			t.Fatalf("Expected ReleasesCreated=true in Mode A legacy")
		}
		if res.TagName != "v1.1.0" || res.Version != "1.1.0" {
			t.Errorf("Expected v1.1.0, got %s / %s", res.TagName, res.Version)
		}
	})

	t.Run("Mode B: Release Staging - New PR", func(t *testing.T) {
		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "github_output")

		var executedCmds [][]string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			executedCmds = append(executedCmds, args)
			if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "rev-parse" {
				return tmpDir + "\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && args[2] == "-1" {
				return "feat(ui): add dashboard\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "tag" && args[2] == "--list" {
				return "v1.0.0\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && strings.Contains(args[2], "..HEAD") {
				return "1234567|feat(ui): add dashboard\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "pr" && args[2] == "list" {
				return "null\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "rev-parse" && args[2] == "HEAD" {
				return "staging123\n", "", 0, nil
			}
			if len(args) >= 5 && args[0] == "git" && args[1] == "-C" && args[3] == "remote" && args[4] == "get-url" {
				return "https://github.com/Cooper-Wallace/repokit.git\n", "", 0, nil
			}
			if len(args) >= 5 && args[0] == "gh" && args[1] == "api" && args[2] == "--method" && args[3] == "POST" && strings.Contains(args[4], "statuses") {
				return "{}", "", 0, nil
			}
			return "", "", 0, nil
		}

		opts := ReleaseOptions{
			Path:          tmpDir,
			TargetBranch:  "main",
			StagingBranch: "release-staging",
			OutputFile:    outFile,
			DryRun:        false,
		}

		res, err := ExecuteRelease(opts)
		if err != nil {
			t.Fatalf("ExecuteRelease failed: %v", err)
		}
		if res.ReleasesCreated {
			t.Errorf("Expected ReleasesCreated=false in Mode B staging")
		}
		if res.TagName != "v1.1.0" || res.Version != "1.1.0" {
			t.Errorf("Expected v1.1.0, got %s / %s", res.TagName, res.Version)
		}

		outData, _ := os.ReadFile(outFile)
		if !strings.Contains(string(outData), "releases_created=false") || !strings.Contains(string(outData), "version=1.1.0") {
			t.Errorf("Unexpected GITHUB_OUTPUT content: %s", string(outData))
		}

		foundPrCreate := false
		foundCheckoutStaging := false
		foundCheckoutMain := false
		foundSemanticPR := false
		foundTestWorkflow := false
		foundCommitRelease := false
		for _, cmd := range executedCmds {
			if len(cmd) >= 4 && cmd[0] == "gh" && cmd[1] == "pr" && cmd[2] == "create" {
				foundPrCreate = true
				for i, arg := range cmd {
					if arg == "--title" && i+1 < len(cmd) {
						if cmd[i+1] != "release(v1.1.0): candidate release" {
							t.Errorf("Expected PR title 'release(v1.1.0): candidate release', got %s", cmd[i+1])
						}
					}
				}
			}
			if len(cmd) >= 3 && cmd[0] == "git" && cmd[1] == "commit" && cmd[2] == "-m" {
				foundCommitRelease = true
				if cmd[3] != "release(v1.1.0): candidate release" {
					t.Errorf("Expected commit msg 'release(v1.1.0): candidate release', got %s", cmd[3])
				}
			}
			if len(cmd) >= 4 && cmd[0] == "git" && cmd[1] == "checkout" && cmd[2] == "-B" && cmd[3] == "release-staging" {
				foundCheckoutStaging = true
			}
			if len(cmd) >= 3 && cmd[0] == "git" && cmd[1] == "checkout" && cmd[2] == "main" {
				foundCheckoutMain = true
			}
			if len(cmd) >= 5 && cmd[0] == "gh" && cmd[1] == "workflow" && cmd[2] == "run" && cmd[3] == "semantic-pr.yml" {
				foundSemanticPR = true
			}
			if len(cmd) >= 5 && cmd[0] == "gh" && cmd[1] == "workflow" && cmd[2] == "run" && cmd[3] == "test.yml" {
				foundTestWorkflow = true
			}
		}
		if !foundCheckoutStaging {
			t.Errorf("Expected git checkout -B release-staging")
		}
		if !foundCommitRelease {
			t.Errorf("Expected git commit with candidate release message")
		}
		if !foundPrCreate {
			t.Errorf("Expected gh pr create")
		}
		if !foundCheckoutMain {
			t.Errorf("Expected git checkout main back to target branch")
		}
		if !foundSemanticPR {
			t.Errorf("Expected gh workflow run semantic-pr.yml")
		}
		if !foundTestWorkflow {
			t.Errorf("Expected gh workflow run test.yml")
		}
		foundStatusCert := 0
		for _, cmd := range executedCmds {
			if len(cmd) >= 5 && cmd[0] == "gh" && cmd[1] == "api" && cmd[2] == "--method" && cmd[3] == "POST" && strings.Contains(cmd[4], "statuses") {
				foundStatusCert++
			}
		}
		if foundStatusCert != 8 {
			t.Errorf("Expected 8 pre-certified status check calls, got %d", foundStatusCert)
		}
	})

	t.Run("Mode B: Release Staging - Update Existing PR", func(t *testing.T) {
		tmpDir := t.TempDir()

		var executedCmds [][]string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			executedCmds = append(executedCmds, args)
			if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "rev-parse" {
				return tmpDir + "\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && args[2] == "-1" {
				return "fix: resolve memory leak\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "tag" && args[2] == "--list" {
				return "v1.1.0\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && strings.Contains(args[2], "..HEAD") {
				return "abcdef1|fix: resolve memory leak\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "pr" && args[2] == "list" {
				return "55\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "rev-parse" && args[2] == "HEAD" {
				return "staging456\n", "", 0, nil
			}
			if len(args) >= 5 && args[0] == "git" && args[1] == "-C" && args[3] == "remote" && args[4] == "get-url" {
				return "https://github.com/Cooper-Wallace/repokit.git\n", "", 0, nil
			}
			if len(args) >= 5 && args[0] == "gh" && args[1] == "api" && args[2] == "--method" && args[3] == "POST" && strings.Contains(args[4], "statuses") {
				return "{}", "", 0, nil
			}
			return "", "", 0, nil
		}

		opts := ReleaseOptions{
			Path:   tmpDir,
			DryRun: false,
		}

		res, err := ExecuteRelease(opts)
		if err != nil {
			t.Fatalf("ExecuteRelease failed: %v", err)
		}
		if res.TagName != "v1.1.1" {
			t.Errorf("Expected v1.1.1, got %s", res.TagName)
		}

		foundPrEdit := false
		foundStatusCertExisting := 0
		for _, cmd := range executedCmds {
			if len(cmd) >= 4 && cmd[0] == "gh" && cmd[1] == "pr" && cmd[2] == "edit" && cmd[3] == "55" {
				foundPrEdit = true
				for i, arg := range cmd {
					if arg == "--title" && i+1 < len(cmd) {
						if cmd[i+1] != "release(v1.1.1): candidate release" {
							t.Errorf("Expected PR title 'release(v1.1.1): candidate release', got %s", cmd[i+1])
						}
					}
				}
			}
			if len(cmd) >= 5 && cmd[0] == "gh" && cmd[1] == "api" && cmd[2] == "--method" && cmd[3] == "POST" && strings.Contains(cmd[4], "statuses") {
				foundStatusCertExisting++
			}
		}
		if !foundPrEdit {
			t.Errorf("Expected gh pr edit 55")
		}
		if foundStatusCertExisting != 8 {
			t.Errorf("Expected 8 pre-certified status check calls, got %d", foundStatusCertExisting)
		}
	})

	t.Run("Mode B: Release Staging - PR Edit Fallback to PATCH on GraphQL error", func(t *testing.T) {
		tmpDir := t.TempDir()

		var executedCmds [][]string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			executedCmds = append(executedCmds, args)
			if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "rev-parse" {
				return tmpDir + "\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && args[2] == "-1" {
				return "fix: resolve memory leak\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "tag" && args[2] == "--list" {
				return "v1.1.0\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && strings.Contains(args[2], "..HEAD") {
				return "abcdef1|fix: resolve memory leak\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "pr" && args[2] == "list" {
				return "55\n", "", 0, nil
			}
			if len(args) >= 4 && args[0] == "gh" && args[1] == "pr" && args[2] == "edit" {
				return "", "GraphQL: Field 'projectCards' doesn't exist on type 'PullRequest'", 1, fmt.Errorf("exit status 1")
			}
			if len(args) >= 5 && args[0] == "gh" && args[1] == "api" && args[2] == "--method" && args[3] == "PATCH" {
				return `{"number": 55}`, "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "rev-parse" && args[2] == "HEAD" {
				return "staging456\n", "", 0, nil
			}
			if len(args) >= 5 && args[0] == "git" && args[1] == "-C" && args[3] == "remote" && args[4] == "get-url" {
				return "https://github.com/Cooper-Wallace/repokit.git\n", "", 0, nil
			}
			if len(args) >= 5 && args[0] == "gh" && args[1] == "api" && args[2] == "--method" && args[3] == "POST" && strings.Contains(args[4], "statuses") {
				return "{}", "", 0, nil
			}
			return "", "", 0, nil
		}

		opts := ReleaseOptions{
			Path:   tmpDir,
			DryRun: false,
		}

		res, err := ExecuteRelease(opts)
		if err != nil {
			t.Fatalf("Expected ExecuteRelease to succeed via fallback PATCH, got: %v", err)
		}
		if res.TagName != "v1.1.1" {
			t.Errorf("Expected v1.1.1, got %s", res.TagName)
		}

		foundPatch := false
		for _, cmd := range executedCmds {
			if len(cmd) >= 5 && cmd[0] == "gh" && cmd[1] == "api" && cmd[2] == "--method" && cmd[3] == "PATCH" {
				if strings.Contains(cmd[4], "repos/Cooper-Wallace/repokit/pulls/55") {
					foundPatch = true
				}
			}
		}
		if !foundPatch {
			t.Errorf("Expected fallback gh api --method PATCH for PR 55")
		}
	})

	t.Run("Mode B: Release Staging - PR Edit and PATCH Both Fail Surfaces Stderr", func(t *testing.T) {
		tmpDir := t.TempDir()

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "rev-parse" {
				return tmpDir + "\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && args[2] == "-1" {
				return "fix: resolve memory leak\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "tag" && args[2] == "--list" {
				return "v1.1.0\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && strings.Contains(args[2], "..HEAD") {
				return "abcdef1|fix: resolve memory leak\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "pr" && args[2] == "list" {
				return "55\n", "", 0, nil
			}
			if len(args) >= 4 && args[0] == "gh" && args[1] == "pr" && args[2] == "edit" {
				return "", "GraphQL: Field 'projectCards' doesn't exist", 1, fmt.Errorf("exit status 1")
			}
			if len(args) >= 5 && args[0] == "gh" && args[1] == "api" && args[2] == "--method" && args[3] == "PATCH" {
				return "", "API rate limit exceeded", 1, fmt.Errorf("exit status 1")
			}
			if len(args) >= 5 && args[0] == "git" && args[1] == "-C" && args[3] == "remote" && args[4] == "get-url" {
				return "https://github.com/Cooper-Wallace/repokit.git\n", "", 0, nil
			}
			return "", "", 0, nil
		}

		opts := ReleaseOptions{
			Path:   tmpDir,
			DryRun: false,
		}

		_, err := ExecuteRelease(opts)
		if err == nil {
			t.Fatalf("Expected ExecuteRelease to fail when edit and patch both fail, got nil")
		}
		if !strings.Contains(err.Error(), "projectCards") {
			t.Errorf("Expected error to contain original stderr 'projectCards', got: %v", err)
		}
		if !strings.Contains(err.Error(), "API rate limit exceeded") {
			t.Errorf("Expected error to contain patch stderr 'API rate limit exceeded', got: %v", err)
		}
	})

	t.Run("Mode B: No Releasable Commits", func(t *testing.T) {
		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "github_output")

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "rev-parse" {
				return tmpDir + "\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && args[2] == "-1" {
				return "docs: update guide\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "tag" && args[2] == "--list" {
				return "v1.0.0\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && strings.Contains(args[2], "..HEAD") {
				return "1234567|docs: update guide\n", "", 0, nil
			}
			return "", "", 0, nil
		}

		opts := ReleaseOptions{
			Path:       tmpDir,
			OutputFile: outFile,
		}

		res, err := ExecuteRelease(opts)
		if err != nil {
			t.Fatalf("ExecuteRelease failed: %v", err)
		}
		if res.ReleasesCreated || res.Version != "" {
			t.Errorf("Expected no release, got %+v", res)
		}

		outData, _ := os.ReadFile(outFile)
		if !strings.Contains(string(outData), "releases_created=false") {
			t.Errorf("Expected releases_created=false in output file, got: %s", string(outData))
		}
	})

	t.Run("Dry Run Mode", func(t *testing.T) {
		tmpDir := t.TempDir()

		var executedCmds [][]string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			executedCmds = append(executedCmds, args)
			if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "rev-parse" {
				return tmpDir + "\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && args[2] == "-1" {
				return "feat: dry run feature\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "tag" && args[2] == "--list" {
				return "v1.0.0\n", "", 0, nil
			}
			if len(args) >= 3 && args[0] == "git" && args[1] == "log" && strings.Contains(args[2], "..HEAD") {
				return "1111111|feat: dry run feature\n", "", 0, nil
			}
			return "", "", 0, nil
		}

		opts := ReleaseOptions{
			Path:   tmpDir,
			DryRun: true,
		}

		res, err := ExecuteRelease(opts)
		if err != nil {
			t.Fatalf("ExecuteRelease failed: %v", err)
		}
		if res.TagName != "v1.1.0" {
			t.Errorf("Expected v1.1.0, got %s", res.TagName)
		}

		for _, cmd := range executedCmds {
			if len(cmd) >= 2 && cmd[0] == "git" && (cmd[1] == "checkout" || cmd[1] == "commit" || cmd[1] == "push") {
				t.Errorf("Mutating git command executed in dry run: %v", cmd)
			}
			if len(cmd) >= 2 && cmd[0] == "gh" {
				t.Errorf("gh command executed in dry run: %v", cmd)
			}
		}
	})
}

func TestGetReleaseNotes(t *testing.T) {
	mockChangelog := `# Changelog

## 1.2.0 (2026-11-06)

### Features

* **core:** add dynamic sync pr bodies (#25)

## 1.1.2 (2026-11-05)

### Bug Fixes

* **propagation:** configure git author identity (#22)

## 1.0.0 (2026-11-01)

### Features

* initial release
`

	tmpDir := t.TempDir()
	changelogPath := filepath.Join(tmpDir, "CHANGELOG.md")
	if err := os.WriteFile(changelogPath, []byte(mockChangelog), 0644); err != nil {
		t.Fatalf("Failed to write mock changelog: %v", err)
	}

	t.Run("extracts_exact_version", func(t *testing.T) {
		notes := GetReleaseNotes(tmpDir, "1.1.2")
		if !strings.HasPrefix(notes, "## 1.1.2 (2026-11-05)") {
			t.Errorf("Expected notes to start with header for 1.1.2, got: %s", notes)
		}
		if !strings.Contains(notes, "configure git author identity") {
			t.Errorf("Expected notes to contain commit details, got: %s", notes)
		}
		if strings.Contains(notes, "## 1.0.0") {
			t.Errorf("Notes leaked into next version section: %s", notes)
		}
	})

	t.Run("strips_v_prefix", func(t *testing.T) {
		notes := GetReleaseNotes(tmpDir, "v1.1.2")
		if !strings.HasPrefix(notes, "## 1.1.2 (2026-11-05)") {
			t.Errorf("Expected notes to start with header for 1.1.2, got: %s", notes)
		}
		if !strings.Contains(notes, "configure git author identity") {
			t.Errorf("Expected notes to contain commit details, got: %s", notes)
		}
	})

	t.Run("extracts_first_section_when_version_empty", func(t *testing.T) {
		notes := GetReleaseNotes(tmpDir, "")
		if !strings.HasPrefix(notes, "## 1.2.0 (2026-11-06)") {
			t.Errorf("Expected first section 1.2.0 when version is empty, got: %s", notes)
		}
		if !strings.Contains(notes, "add dynamic sync pr bodies") {
			t.Errorf("Expected first section content, got: %s", notes)
		}
		if strings.Contains(notes, "## 1.1.2") {
			t.Errorf("Notes leaked into next section: %s", notes)
		}
	})

	t.Run("extracts_first_section_when_version_not_found", func(t *testing.T) {
		notes := GetReleaseNotes(tmpDir, "v9.9.9")
		if !strings.HasPrefix(notes, "## 1.2.0 (2026-11-06)") {
			t.Errorf("Expected fallback to first section 1.2.0, got: %s", notes)
		}
		if !strings.Contains(notes, "add dynamic sync pr bodies") {
			t.Errorf("Expected first section content, got: %s", notes)
		}
	})

	t.Run("extracts_last_section_to_eof", func(t *testing.T) {
		notes := GetReleaseNotes(tmpDir, "v1.0.0")
		if !strings.HasPrefix(notes, "## 1.0.0 (2026-11-01)") {
			t.Errorf("Expected notes for 1.0.0, got: %s", notes)
		}
		if !strings.Contains(notes, "initial release") {
			t.Errorf("Expected content of 1.0.0, got: %s", notes)
		}
	})

	t.Run("returns_empty_when_changelog_not_found", func(t *testing.T) {
		emptyDir := t.TempDir()
		notes := GetReleaseNotes(emptyDir, "v1.0.0")
		if notes != "" {
			t.Errorf("Expected empty string when CHANGELOG.md is not found, got: %q", notes)
		}
	})

	t.Run("returns_empty_for_empty_changelog", func(t *testing.T) {
		emptyChangelogDir := t.TempDir()
		_ = os.WriteFile(filepath.Join(emptyChangelogDir, "CHANGELOG.md"), []byte("   \n\n  "), 0644)
		// Pointing directly to empty CHANGELOG.md
		notes := GetReleaseNotes(filepath.Join(emptyChangelogDir, "CHANGELOG.md"), "1.0.0")
		if notes != "" {
			t.Errorf("Expected empty string for empty changelog, got: %q", notes)
		}
	})
}

func TestBuildSyncPRBody(t *testing.T) {
	t.Run("builds_body_with_release_notes", func(t *testing.T) {
		body := buildSyncPRBody("v1.1.2")
		if !strings.Contains(body, "🤖 **Automated Governance Sync**") {
			t.Errorf("Expected header banner in PR body: %s", body)
		}
		if !strings.Contains(body, "**Merge Policy**: This PR is queued for automated squash merge") {
			t.Errorf("Expected merge policy in PR body: %s", body)
		}
		if !strings.Contains(body, "### Release Notes (`1.1.2`)") {
			t.Errorf("Expected Release Notes header in PR body, got: %s", body)
		}
		if !strings.Contains(body, "configure git author identity") {
			t.Errorf("Expected release notes content in PR body, got: %s", body)
		}
	})

	t.Run("builds_body_with_fallback_when_no_changelog", func(t *testing.T) {
		// When ReleaseRef is specified but no release notes are extracted
		// We can test the helper directly or by inspecting formatting
		body := `> 🤖 **Automated Governance Sync**
> This pull request was automatically generated by [repokit](https://github.com/Cooper-Wallace/repokit) to synchronize centralized governance standards, reusable GitHub Actions workflows, and project templates.
>
> **Merge Policy**: This PR is queued for automated squash merge once all required status checks pass.

### Propagated Version
Synchronized to repokit release ` + "`v2.0.0`."

		if !strings.Contains(body, "Synchronized to repokit release `v2.0.0`.") {
			t.Errorf("Expected propagated version text")
		}
	})
}

func TestWaitForSyncPRs(t *testing.T) {
	oldRunner := CommandRunner
	oldPollInterval := SyncPRPollInterval
	defer func() {
		CommandRunner = oldRunner
		SyncPRPollInterval = oldPollInterval
	}()
	SyncPRPollInterval = 5 * time.Millisecond

	t.Run("empty_prs_list", func(t *testing.T) {
		res, err := WaitForSyncPRs(nil, 5*time.Second)
		if err != nil {
			t.Fatalf("Expected nil error for empty prs, got: %v", err)
		}
		if len(res) != 0 {
			t.Errorf("Expected 0 prs returned, got %d", len(res))
		}
	})

	t.Run("all_prs_merged_successfully", func(t *testing.T) {
		prs := []SyncPRInfo{
			{Slug: "org/repo-a", Branch: "bot/repokit-sync"},
			{Slug: "org/repo-b", Branch: "bot/repokit-sync"},
		}
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 6 && args[0] == "gh" && args[1] == "pr" && args[2] == "view" {
				slug := args[5]
				var num int
				if slug == "org/repo-a" {
					num = 101
				} else {
					num = 102
				}
				resp := ghPRViewOutput{
					Number: num,
					URL:    fmt.Sprintf("https://github.com/%s/pull/%d", slug, num),
					State:  "MERGED",
				}
				data, _ := json.Marshal(resp)
				return string(data), "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		res, err := WaitForSyncPRs(prs, 200*time.Millisecond)
		if err != nil {
			t.Fatalf("Expected all PRs to merge cleanly, got error: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("Expected 2 PRs in result, got %d", len(res))
		}
		if res[0].PRNumber != 101 || res[0].PRURL != "https://github.com/org/repo-a/pull/101" {
			t.Errorf("Expected repo-a PR details populated, got: %+v", res[0])
		}
		if res[1].PRNumber != 102 || res[1].PRURL != "https://github.com/org/repo-b/pull/102" {
			t.Errorf("Expected repo-b PR details populated, got: %+v", res[1])
		}
	})

	t.Run("pr_check_failure_surfaces_error", func(t *testing.T) {
		failures := []struct {
			name       string
			status     string
			conclusion string
			state      string
		}{
			{name: "failure_conclusion", status: "COMPLETED", conclusion: "FAILURE"},
			{name: "cancelled_conclusion", status: "COMPLETED", conclusion: "CANCELLED"},
			{name: "timed_out_conclusion", status: "COMPLETED", conclusion: "TIMED_OUT"},
			{name: "action_required_conclusion", status: "COMPLETED", conclusion: "ACTION_REQUIRED"},
			{name: "state_error", status: "COMPLETED", state: "ERROR"},
		}

		for _, tc := range failures {
			t.Run(tc.name, func(t *testing.T) {
				prs := []SyncPRInfo{
					{Slug: "org/repo-fail", Branch: "bot/repokit-sync"},
				}
				CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
					if len(args) >= 6 && args[0] == "gh" && args[1] == "pr" && args[2] == "view" {
						resp := ghPRViewOutput{
							Number: 55,
							URL:    "https://github.com/org/repo-fail/pull/55",
							State:  "OPEN",
							StatusCheckRollup: []ghStatusCheckEntry{
								{
									Name:       "Quality Gate / Verification",
									Status:     tc.status,
									Conclusion: tc.conclusion,
									State:      tc.state,
									DetailsURL: "https://github.com/org/repo-fail/actions/runs/12345",
								},
							},
						}
						data, _ := json.Marshal(resp)
						return string(data), "", 0, nil
					}
					if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
						if args[2] == "list" {
							return "[]", "", 0, nil
						}
						if args[2] == "create" {
							return "https://github.com/org/repo-fail/issues/77\n", "", 0, nil
						}
					}
					return defaultCommandRunner(args, dir, input)
				}

				res, err := WaitForSyncPRs(prs, 200*time.Millisecond)
				if err == nil {
					t.Fatalf("Expected error when check fails, got nil")
				}
				if !strings.Contains(err.Error(), "org/repo-fail") {
					t.Errorf("Expected error to mention failed slug, got: %v", err)
				}
				if len(res) > 0 && res[0].PRNumber != 55 {
					t.Errorf("Expected PR number 55 populated, got %d", res[0].PRNumber)
				}
				if len(res) > 0 && res[0].IssueNumber != 77 {
					t.Errorf("Expected issue number 77 populated, got %d", res[0].IssueNumber)
				}
				if len(res) > 0 && res[0].IssueURL != "https://github.com/org/repo-fail/issues/77" {
					t.Errorf("Expected issue URL populated, got %s", res[0].IssueURL)
				}
				if !strings.Contains(err.Error(), "https://github.com/org/repo-fail/issues/77") {
					t.Errorf("Expected error to contain issue link, got: %v", err)
				}
			})
		}
	})

	t.Run("pr_closed_without_merging_surfaces_error", func(t *testing.T) {
		prs := []SyncPRInfo{
			{Slug: "org/repo-closed", Branch: "bot/repokit-sync"},
		}
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 6 && args[0] == "gh" && args[1] == "pr" && args[2] == "view" {
				resp := ghPRViewOutput{
					Number: 99,
					URL:    "https://github.com/org/repo-closed/pull/99",
					State:  "CLOSED",
				}
				data, _ := json.Marshal(resp)
				return string(data), "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		_, err := WaitForSyncPRs(prs, 100*time.Millisecond)
		if err == nil {
			t.Fatalf("Expected error when PR is closed, got nil")
		}
		if !strings.Contains(err.Error(), "org/repo-closed") {
			t.Errorf("Expected error to mention org/repo-closed, got: %v", err)
		}
	})

	t.Run("timeout_expires_surfaces_error", func(t *testing.T) {
		prs := []SyncPRInfo{
			{Slug: "org/repo-slow", Branch: "bot/repokit-sync"},
		}
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 6 && args[0] == "gh" && args[1] == "pr" && args[2] == "view" {
				resp := ghPRViewOutput{
					Number: 12,
					URL:    "https://github.com/org/repo-slow/pull/12",
					State:  "OPEN",
					StatusCheckRollup: []ghStatusCheckEntry{
						{
							Name:   "Quality Gate / Verification",
							Status: "IN_PROGRESS",
						},
					},
				}
				data, _ := json.Marshal(resp)
				return string(data), "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		_, err := WaitForSyncPRs(prs, 20*time.Millisecond)
		if err == nil {
			t.Fatalf("Expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "org/repo-slow") {
			t.Errorf("Expected error to mention org/repo-slow, got: %v", err)
		}
	})

	t.Run("fast_fail_when_all_prs_reach_terminal_status", func(t *testing.T) {
		prs := []SyncPRInfo{
			{Slug: "org/repo-merged", Branch: "bot/repokit-sync"},
			{Slug: "org/repo-failed", Branch: "bot/repokit-sync"},
		}
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 6 && args[0] == "gh" && args[1] == "pr" && args[2] == "view" {
				slug := args[5]
				if slug == "org/repo-merged" {
					resp := ghPRViewOutput{
						Number: 101,
						URL:    "https://github.com/org/repo-merged/pull/101",
						State:  "MERGED",
					}
					data, _ := json.Marshal(resp)
					return string(data), "", 0, nil
				}
				if slug == "org/repo-failed" {
					resp := ghPRViewOutput{
						Number: 102,
						URL:    "https://github.com/org/repo-failed/pull/102",
						State:  "OPEN",
						StatusCheckRollup: []ghStatusCheckEntry{
							{
								Name:       "CI",
								Status:     "COMPLETED",
								Conclusion: "FAILURE",
								DetailsURL: "https://github.com/org/repo-failed/actions/runs/999",
							},
						},
					}
					data, _ := json.Marshal(resp)
					return string(data), "", 0, nil
				}
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					return "https://github.com/org/repo-failed/issues/42\n", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		start := time.Now()
		// Timeout is large (10s), but fast-failing should exit in milliseconds when mergedCount + failedCount == total
		res, err := WaitForSyncPRs(prs, 10*time.Second)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatalf("Expected error due to failed PR, got nil")
		}
		if elapsed > 1*time.Second {
			t.Errorf("Expected fast-failing early exit, took too long: %v", elapsed)
		}
		if len(res) != 2 {
			t.Fatalf("Expected 2 PRs in results, got %d", len(res))
		}
		if res[0].PRNumber != 101 || res[1].PRNumber != 102 {
			t.Errorf("PR numbers not populated properly: %+v", res)
		}
		if res[1].IssueNumber != 42 {
			t.Errorf("Expected issue number 42 on failed PR, got %d", res[1].IssueNumber)
		}
		if !strings.Contains(err.Error(), "org/repo-failed (Issue: https://github.com/org/repo-failed/issues/42)") {
			t.Errorf("Expected error to summarize failed repo and issue link, got: %v", err)
		}
	})
}

func TestEnsureConsumerFailureIssue(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("creates_new_issue_when_none_exists", func(t *testing.T) {
		var createdRepo, createdTitle, createdLabel, createdBody string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					for i := 0; i < len(args); i++ {
						switch args[i] {
						case "--repo":
							createdRepo = args[i+1]
						case "--title":
							createdTitle = args[i+1]
						case "--label":
							createdLabel = args[i+1]
						case "--body":
							createdBody = args[i+1]
						}
					}
					return "https://github.com/org/test-consumer/issues/42\n", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureConsumerFailureIssue("org/test-consumer", 15, "Lint & Test", "https://github.com/org/test-consumer/actions/runs/123")
		if err != nil {
			t.Fatalf("Expected issue creation to succeed, got: %v", err)
		}
		if num != 42 {
			t.Errorf("Expected issue number 42, got %d", num)
		}
		if createdRepo != "org/test-consumer" {
			t.Errorf("Expected repo 'org/test-consumer', got %q", createdRepo)
		}
		if createdTitle != "fix(ci): governance synchronization blocked by failing CI checks" {
			t.Errorf("Unexpected issue title: %q", createdTitle)
		}
		if createdLabel != "governance-sync,automated" {
			t.Errorf("Unexpected issue labels: %q", createdLabel)
		}
		if !strings.Contains(createdBody, "PR #15") || !strings.Contains(createdBody, "Lint & Test") || !strings.Contains(createdBody, "https://github.com/org/test-consumer/actions/runs/123") {
			t.Errorf("Body missing PR, check name, or URL: %s", createdBody)
		}
		if !strings.Contains(createdBody, "Fix the issue on main. Once merged into main, open bot/* synchronization PRs will automatically update, re-test, and auto-merge.") {
			t.Errorf("Body missing remediation instruction: %s", createdBody)
		}
	})

	t.Run("idempotent_when_open_issue_exists", func(t *testing.T) {
		createCalled := false
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return `[{"number": 99, "title": "fix(ci): governance synchronization blocked by failing CI checks"}]`, "", 0, nil
				}
				if args[2] == "create" {
					createCalled = true
					return "", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureConsumerFailureIssue("org/test-consumer", 15, "Lint & Test", "")
		if err != nil {
			t.Fatalf("Expected success for existing issue, got: %v", err)
		}
		if num != 99 {
			t.Errorf("Expected existing issue number 99, got %d", num)
		}
		if createCalled {
			t.Errorf("Expected gh issue create NOT to be called when issue exists")
		}
	})

	t.Run("list_error_surfaces", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" && args[2] == "list" {
				return "", "GraphQL error: unauthorized", 1, fmt.Errorf("exit status 1")
			}
			return defaultCommandRunner(args, dir, input)
		}

		_, err := EnsureConsumerFailureIssue("org/test-consumer", 15, "Test", "")
		if err == nil {
			t.Fatalf("Expected error when gh issue list fails, got nil")
		}
		if !strings.Contains(err.Error(), "unauthorized") {
			t.Errorf("Expected error to mention unauthorized, got: %v", err)
		}
	})

	t.Run("create_error_surfaces", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					return "", "cannot create issue: permission denied", 1, fmt.Errorf("exit status 1")
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		_, err := EnsureConsumerFailureIssue("org/test-consumer", 15, "Test", "")
		if err == nil {
			t.Fatalf("Expected error when gh issue create fails, got nil")
		}
		if !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("Expected error to mention permission denied, got: %v", err)
		}
	})

	t.Run("missing_labels_provisions_and_retries", func(t *testing.T) {
		var createdLabels []string
		issueCreateCalls := 0

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					issueCreateCalls++
					if issueCreateCalls == 1 {
						return "", "could not add label \"governance-sync\": label not found", 1, fmt.Errorf("exit status 1")
					}
					return "https://github.com/org/test-consumer/issues/77\n", "", 0, nil
				}
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "label" && args[2] == "create" {
				createdLabels = append(createdLabels, args[3])
				return "", "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureConsumerFailureIssue("org/test-consumer", 20, "CI Test", "")
		if err != nil {
			t.Fatalf("Expected success after provisioning labels, got: %v", err)
		}
		if num != 77 {
			t.Errorf("Expected issue number 77, got %d", num)
		}
		if issueCreateCalls != 2 {
			t.Errorf("Expected 2 issue create attempts, got %d", issueCreateCalls)
		}
		if len(createdLabels) != 2 || createdLabels[0] != "governance-sync" || createdLabels[1] != "automated" {
			t.Errorf("Expected labels governance-sync and automated to be created, got: %v", createdLabels)
		}
	})

	t.Run("missing_labels_fallback_without_labels", func(t *testing.T) {
		issueCreateWithoutLabels := false

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					hasLabelArg := false
					for _, a := range args {
						if a == "--label" {
							hasLabelArg = true
							break
						}
					}
					if hasLabelArg {
						return "", "could not add label: not found", 1, fmt.Errorf("exit status 1")
					}
					issueCreateWithoutLabels = true
					return "https://github.com/org/test-consumer/issues/88\n", "", 0, nil
				}
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "label" && args[2] == "create" {
				return "", "label create forbidden", 1, fmt.Errorf("exit status 1")
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureConsumerFailureIssue("org/test-consumer", 20, "CI Test", "")
		if err != nil {
			t.Fatalf("Expected success via fallback without labels, got: %v", err)
		}
		if num != 88 {
			t.Errorf("Expected issue number 88, got %d", num)
		}
		if !issueCreateWithoutLabels {
			t.Errorf("Expected issue create to be attempted without --label flag")
		}
	})
}

func TestEnsureHubFailureIssue(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("creates_new_hub_issue_when_none_exists", func(t *testing.T) {
		var createdRepo, createdTitle, createdLabel, createdBody string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					for i := 0; i < len(args); i++ {
						switch args[i] {
						case "--repo":
							createdRepo = args[i+1]
						case "--title":
							createdTitle = args[i+1]
						case "--label":
							createdLabel = args[i+1]
						case "--body":
							createdBody = args[i+1]
						}
					}
					return "https://github.com/Cooper-Wallace/repokit/issues/88\n", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		failedSlugs := []string{"Cooper-Wallace/repo-1", "Cooper-Wallace/repo-2"}
		num, err := EnsureHubFailureIssue(failedSlugs, "v1.2.3")
		if err != nil {
			t.Fatalf("Expected hub issue creation to succeed, got: %v", err)
		}
		if num != 88 {
			t.Errorf("Expected issue number 88, got %d", num)
		}
		if createdRepo != "Cooper-Wallace/repokit" {
			t.Errorf("Expected hub repo 'Cooper-Wallace/repokit', got %q", createdRepo)
		}
		if createdTitle != "bug(propagation): governance deployment failed for release v1.2.3" {
			t.Errorf("Unexpected title: %q", createdTitle)
		}
		if createdLabel != "propagation-failure,automated" {
			t.Errorf("Unexpected label: %q", createdLabel)
		}
		if !strings.Contains(createdBody, "repo-1") || !strings.Contains(createdBody, "repo-2") {
			t.Errorf("Expected failed slugs in body: %s", createdBody)
		}
	})

	t.Run("idempotent_when_open_hub_issue_exists", func(t *testing.T) {
		createCalled := false
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return `[{"number": 77, "title": "bug(propagation): governance deployment failed for release v1.2.3"}]`, "", 0, nil
				}
				if args[2] == "create" {
					createCalled = true
					return "", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureHubFailureIssue([]string{"org/repo-x"}, "v1.2.3")
		if err != nil {
			t.Fatalf("Expected success for existing hub issue, got: %v", err)
		}
		if num != 77 {
			t.Errorf("Expected existing issue number 77, got %d", num)
		}
		if createCalled {
			t.Errorf("Expected gh issue create NOT to be called when hub issue exists")
		}
	})

	t.Run("default_release_ref_when_empty", func(t *testing.T) {
		var createdTitle string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					for i := 0; i < len(args); i++ {
						if args[i] == "--title" {
							createdTitle = args[i+1]
						}
					}
					return "https://github.com/Cooper-Wallace/repokit/issues/105\n", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureHubFailureIssue([]string{"org/repo-y"}, "")
		if err != nil {
			t.Fatalf("Expected success, got: %v", err)
		}
		if num != 105 {
			t.Errorf("Expected 105, got %d", num)
		}
		if createdTitle != "bug(propagation): governance deployment failed for release latest" {
			t.Errorf("Expected default release ref 'latest', got %q", createdTitle)
		}
	})

	t.Run("list_error_surfaces", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" && args[2] == "list" {
				return "", "API rate limit exceeded", 1, fmt.Errorf("exit status 1")
			}
			return defaultCommandRunner(args, dir, input)
		}

		_, err := EnsureHubFailureIssue([]string{"org/repo-z"}, "v1.0.0")
		if err == nil {
			t.Fatalf("Expected error when gh issue list fails, got nil")
		}
		if !strings.Contains(err.Error(), "rate limit") {
			t.Errorf("Expected error to mention rate limit, got: %v", err)
		}
	})

	t.Run("create_error_surfaces", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					return "", "GraphQL error: server error", 1, fmt.Errorf("exit status 1")
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		_, err := EnsureHubFailureIssue([]string{"org/repo-z"}, "v1.0.0")
		if err == nil {
			t.Fatalf("Expected error when gh issue create fails, got nil")
		}
		if !strings.Contains(err.Error(), "server error") {
			t.Errorf("Expected error to mention server error, got: %v", err)
		}
	})

	t.Run("hub_missing_labels_provisions_and_retries", func(t *testing.T) {
		var createdLabels []string
		issueCreateCalls := 0

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					issueCreateCalls++
					if issueCreateCalls == 1 {
						return "", "could not add label \"propagation-failure\": label not found", 1, fmt.Errorf("exit status 1")
					}
					return "https://github.com/Cooper-Wallace/repokit/issues/99\n", "", 0, nil
				}
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "label" && args[2] == "create" {
				createdLabels = append(createdLabels, args[3])
				return "", "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureHubFailureIssue([]string{"org/downstream-repo"}, "v2.0.0")
		if err != nil {
			t.Fatalf("Expected success after provisioning hub labels, got: %v", err)
		}
		if num != 99 {
			t.Errorf("Expected issue number 99, got %d", num)
		}
		if len(createdLabels) != 2 || createdLabels[0] != "propagation-failure" || createdLabels[1] != "automated" {
			t.Errorf("Expected hub labels propagation-failure and automated, got: %v", createdLabels)
		}
	})
}

func TestDefaultBranchVerification(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	t.Run("get_repo_default_branch_via_gh_repo_view", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 4 && args[0] == "gh" && args[1] == "repo" && args[2] == "view" {
				return `{"defaultBranchRef":{"name":"main"}}`, "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}
		branch, err := GetRepoDefaultBranch("owner/repo", "")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if branch != "main" {
			t.Errorf("Expected 'main', got %q", branch)
		}
	})

	t.Run("get_repo_default_branch_via_gh_api_fallback", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 4 && args[0] == "gh" && args[1] == "repo" && args[2] == "view" {
				return "", "view failed", 1, nil
			}
			if len(args) >= 3 && args[0] == "gh" && args[1] == "api" && args[2] == "repos/owner/repo" {
				return `{"default_branch":"master"}`, "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}
		branch, err := GetRepoDefaultBranch("owner/repo", "")
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if branch != "master" {
			t.Errorf("Expected 'master', got %q", branch)
		}
	})

	t.Run("get_repo_default_branch_via_local_git", func(t *testing.T) {
		tmpDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		_ = cmd.Run()

		CommandRunner = defaultCommandRunner
		branch, err := GetRepoDefaultBranch("", tmpDir)
		if err != nil {
			t.Fatalf("Expected no error from local git, got: %v", err)
		}
		if branch != "main" {
			t.Errorf("Expected 'main', got %q", branch)
		}
	})

	t.Run("audit_repository_fails_when_default_branch_not_main", func(t *testing.T) {
		tmpDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "master")
		cmd.Dir = tmpDir
		_ = cmd.Run()
		cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/Cooper-Wallace/repokit.git")
		cmd.Dir = tmpDir
		_ = cmd.Run()

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit" {
				data := map[string]interface{}{
					"default_branch":              "master",
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
		if err != nil {
			t.Fatalf("AuditRepository returned unexpected error: %v", err)
		}
		if passed {
			t.Errorf("Expected AuditRepository to fail when default branch is master, but passed=true")
		}
	})

	t.Run("audit_repository_passes_when_default_branch_is_main", func(t *testing.T) {
		tmpDir := t.TempDir()
		cmd := exec.Command("git", "init", "-b", "main")
		cmd.Dir = tmpDir
		_ = cmd.Run()
		cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/Cooper-Wallace/repokit.git")
		cmd.Dir = tmpDir
		_ = cmd.Run()

		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) > 2 && args[2] == "repos/Cooper-Wallace/repokit" {
				data := map[string]interface{}{
					"default_branch":              "main",
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
			t.Errorf("Expected AuditRepository to pass when default branch is main, got passed=%v, err=%v", passed, err)
		}
	})
}

func TestTemplateWorkflowValidity(t *testing.T) {
	archetypes := []string{"bash", "go", "java", "python", "typescript"}

	for _, arch := range archetypes {
		t.Run(arch+"_sync_yml", func(t *testing.T) {
			path := filepath.Join("templates", arch, ".github", "workflows", "sync.yml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", path, err)
			}
			content := string(data)
			if strings.Contains(content, "secrets:\n      inherit") || strings.Contains(content, "secrets:\n    inherit") {
				t.Errorf("%s has invalid multi-line secrets: inherit syntax", path)
			}
			if !strings.Contains(content, "secrets: inherit") {
				t.Errorf("%s missing 'secrets: inherit'", path)
			}
		})

		t.Run(arch+"_release_yml", func(t *testing.T) {
			path := filepath.Join("templates", arch, ".github", "workflows", "release.yml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", path, err)
			}
			content := string(data)
			if !strings.Contains(content, "statuses: write") {
				t.Errorf("%s missing 'statuses: write'", path)
			}
			if !strings.Contains(content, "actions: write") {
				t.Errorf("%s missing 'actions: write'", path)
			}
			if !strings.Contains(content, "contents: write") {
				t.Errorf("%s missing 'contents: write'", path)
			}
			if !strings.Contains(content, "pull-requests: write") {
				t.Errorf("%s missing 'pull-requests: write'", path)
			}
		})
	}

	t.Run("GenericReleaseWorkflow_constant", func(t *testing.T) {
		if !strings.Contains(GenericReleaseWorkflow, "statuses: write") {
			t.Errorf("GenericReleaseWorkflow missing 'statuses: write'")
		}
		if !strings.Contains(GenericReleaseWorkflow, "actions: write") {
			t.Errorf("GenericReleaseWorkflow missing 'actions: write'")
		}
	})
}

func TestConfigureMultipleRepositories_Deduplication(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo-dedup")
	_ = os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)

	apiRepoSettingsCalls := 0
	apiSecuritySettingsCalls := 0
	apiWorkflowPermsCalls := 0
	apiBranchRulesetCalls := 0

	CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
		if len(args) >= 5 && args[0] == "git" && args[1] == "-C" && args[3] == "remote" && args[4] == "get-url" {
			return "https://github.com/org/repo-dedup.git\n", "", 0, nil
		}
		if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "rev-parse" {
			return repoDir + "\n", "", 0, nil
		}
		if len(args) >= 4 && args[0] == "gh" && args[1] == "repo" && args[2] == "view" {
			return `{"defaultBranchRef": {"name": "main"}}`, "", 0, nil
		}
		if len(args) >= 3 && args[0] == "gh" && args[1] == "api" {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "code-scanning/default-setup") {
				return "{}", "", 0, nil
			}
			if strings.Contains(joined, "--method PATCH repos/org/repo-dedup") {
				if strings.Contains(input, "allow_squash_merge") {
					apiRepoSettingsCalls++
					return "{}", "", 0, nil
				}
				if strings.Contains(input, "security_and_analysis") {
					apiSecuritySettingsCalls++
					return "{}", "", 0, nil
				}
				return "{}", "", 0, nil
			}
			if strings.Contains(joined, "actions/permissions/workflow") {
				apiWorkflowPermsCalls++
				return "{}", "", 0, nil
			}
			if len(args) == 3 && args[2] == "repos/org/repo-dedup/rulesets" {
				apiBranchRulesetCalls++
				return "[]", "", 0, nil
			}
			if strings.Contains(joined, "rulesets") {
				return "{}", "", 0, nil
			}
		}
		if len(args) >= 3 && args[0] == "git" && args[1] == "-C" && args[3] == "status" {
			return "", "", 0, nil
		}
		return defaultCommandRunner(args, dir, input)
	}

	err := ConfigureWithOptions(ConfigureOptions{
		Local:    true,
		LocalDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("ConfigureWithOptions failed: %v", err)
	}

	if apiRepoSettingsCalls != 1 {
		t.Errorf("Expected ApplyGitHubRepoSettings to be called exactly 1 time, got %d", apiRepoSettingsCalls)
	}
	if apiSecuritySettingsCalls != 1 {
		t.Errorf("Expected ApplySecurityAndAnalysis to be called exactly 1 time, got %d", apiSecuritySettingsCalls)
	}
	if apiWorkflowPermsCalls != 1 {
		t.Errorf("Expected ApplyWorkflowPermissions to be called exactly 1 time, got %d", apiWorkflowPermsCalls)
	}
	if apiBranchRulesetCalls != 1 {
		t.Errorf("Expected ApplyBranchRuleset to be called exactly 1 time, got %d", apiBranchRulesetCalls)
	}
}

func TestWaitForSyncPRs_DefaultTimeout(t *testing.T) {
	res, err := WaitForSyncPRs(nil, 0)
	if err != nil {
		t.Fatalf("WaitForSyncPRs with zero timeout failed: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("Expected 0 PRs returned, got %d", len(res))
	}
}

func TestEnsureRepoLabels(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	var labelArgs [][]string
	CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
		if len(args) >= 3 && args[0] == "gh" && args[1] == "label" && args[2] == "create" {
			labelArgs = append(labelArgs, args)
			return "", "", 0, nil
		}
		return defaultCommandRunner(args, dir, input)
	}

	err := EnsureRepoLabels("org/label-repo", "label-a", "label-b")
	if err != nil {
		t.Fatalf("EnsureRepoLabels failed: %v", err)
	}
	if len(labelArgs) != 2 {
		t.Fatalf("Expected 2 label create calls, got %d", len(labelArgs))
	}
	if labelArgs[0][3] != "label-a" || labelArgs[1][3] != "label-b" {
		t.Errorf("Unexpected label arguments: %v", labelArgs)
	}
}

func TestArchetypeWorkflowPermissionsAndBotPRHydration(t *testing.T) {
	archetypes := []string{"bash", "go", "java", "python", "typescript"}

	for _, arch := range archetypes {
		t.Run("template_"+arch+"_ci_permissions", func(t *testing.T) {
			path := filepath.Join("templates", arch, ".github", "workflows", "ci.yml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", path, err)
			}
			content := string(data)
			if !strings.Contains(content, "contents: write") {
				t.Errorf("%s missing contents: write", path)
			}
			if !strings.Contains(content, "pull-requests: write") {
				t.Errorf("%s missing pull-requests: write", path)
			}
			if !strings.Contains(content, "statuses: write") {
				t.Errorf("%s missing statuses: write", path)
			}
			if !strings.Contains(content, "secrets: inherit") {
				t.Errorf("%s missing secrets: inherit", path)
			}
		})

		t.Run("reusable_"+arch+"_bot_hydration", func(t *testing.T) {
			path := filepath.Join(".github", "workflows", "reusable-"+arch+".yml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", path, err)
			}
			content := string(data)
			if !strings.Contains(content, `startswith("bot/")`) {
				t.Errorf("%s missing startswith(\"bot/\")", path)
			}
			if strings.Contains(content, `--head "bot/"`) {
				t.Errorf("%s still contains --head \"bot/\"", path)
			}
			if !strings.Contains(content, "contents: write") {
				t.Errorf("%s missing contents: write", path)
			}
			if !strings.Contains(content, "pull-requests: write") {
				t.Errorf("%s missing pull-requests: write", path)
			}
			if !strings.Contains(content, "statuses: write") {
				t.Errorf("%s missing statuses: write", path)
			}
			if !strings.Contains(content, "git rebase -X theirs origin/main") {
				t.Errorf("%s missing git rebase -X theirs origin/main fallback", path)
			}
		})
	}
}
