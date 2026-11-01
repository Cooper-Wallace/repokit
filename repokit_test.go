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

	// Governance hub with test.yml should pass audit
	_ = os.WriteFile(filepath.Join(target, ".github", "workflows", "test.yml"), []byte("name: Test\n"), 0644)
	passedHub, _ := AuditRepository(target, "", true, false)
	if !passedHub {
		t.Errorf("Governance hub with test.yml should pass audit")
	}
}

func TestPRTemplateContainsQualityGates(t *testing.T) {
	expectedTerms := []string{
		"## Quality Gates & Sign-Off Checklist",
		"Gate 1: Automated Quality Gates",
		"Gate 2: Agent Review Verification",
		"Gate 3: Human Review & Manual Promotion",
		"never automated",
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
			return defaultCommandRunner(args, dir, input)
		}

		passed, err := AuditRepository(tmpDir, "", false, true)
		if err != nil || !passed {
			t.Errorf("Expected remote audit to pass, got passed=%v, err=%v", passed, err)
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
