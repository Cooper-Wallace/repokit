package repokit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestWorkflow(t *testing.T, repoRoot, filename, content string) string {
	t.Helper()
	fullPath := filepath.Join(repoRoot, ".github", "workflows", filename)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("failed to create workflows dir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", filename, err)
	}
	return fullPath
}

func TestReconcileConsumerWorkflows_SkipsManagedWorkflows(t *testing.T) {
	repoRoot := t.TempDir()
	managed := []string{
		"ci.yml",
		"release.yml",
		"semantic-pr.yml",
		"dependabot-automerge.yml",
		"sync.yml",
		"CI.yml",
	}
	for _, name := range managed {
		writeTestWorkflow(t, repoRoot, name, `
name: Managed
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@v5
        with:
          tags: ghcr.io/org/app:latest
      - run: pytest
`)
	}

	res, err := ReconcileConsumerWorkflows(repoRoot, "Cooper-Wallace/consumer-app", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Pruned) != 0 || len(res.OptedOut) != 0 || len(res.Findings) != 0 {
		t.Fatalf("expected managed workflows to be skipped, got %+v", res)
	}
	for _, name := range managed {
		if _, err := os.Stat(filepath.Join(repoRoot, ".github", "workflows", name)); err != nil {
			t.Errorf("managed workflow %s should still exist: %v", name, err)
		}
	}
}

func TestReconcileConsumerWorkflows_SkipsHubAndExemptRepos(t *testing.T) {
	legacyGHCR := `
name: Deploy
on: push
jobs:
  docker:
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@v5
        with:
          tags: ghcr.io/cooper-wallace/app:latest
`
	t.Run("hub_slug", func(t *testing.T) {
		repoRoot := t.TempDir()
		p := writeTestWorkflow(t, repoRoot, "deploy.yml", legacyGHCR)
		res, err := ReconcileConsumerWorkflows(repoRoot, DefaultGovernanceRepo, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Pruned) != 0 || len(res.Findings) != 0 {
			t.Fatalf("expected hub repo to be skipped, got %+v", res)
		}
		if _, err := os.Stat(p); err != nil {
			t.Errorf("file should not be removed on hub repo: %v", err)
		}
	})

	t.Run("vault_and_profile_slug", func(t *testing.T) {
		for _, slug := range []string{"Cooper-Wallace/AgentVault", "Cooper-Wallace/Cooper-Wallace"} {
			repoRoot := t.TempDir()
			p := writeTestWorkflow(t, repoRoot, "deploy.yml", legacyGHCR)
			res, err := ReconcileConsumerWorkflows(repoRoot, slug, false)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", slug, err)
			}
			if len(res.Pruned) != 0 || len(res.Findings) != 0 {
				t.Fatalf("expected exempt repo %s to be skipped, got %+v", slug, res)
			}
			if _, err := os.Stat(p); err != nil {
				t.Errorf("file should not be removed on exempt repo %s: %v", slug, err)
			}
		}
	})

	t.Run("local_governance_hub_markers", func(t *testing.T) {
		repoRoot := t.TempDir()
		writeTestWorkflow(t, repoRoot, "reusable-python.yml", "on:\n  workflow_call:\n")
		if err := os.WriteFile(filepath.Join(repoRoot, "repokit.go"), []byte("package repokit\n"), 0644); err != nil {
			t.Fatal(err)
		}
		p := writeTestWorkflow(t, repoRoot, "deploy.yml", legacyGHCR)
		res, err := ReconcileConsumerWorkflows(repoRoot, "", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Pruned) != 0 || len(res.Findings) != 0 {
			t.Fatalf("expected local governance hub to be skipped, got %+v", res)
		}
		if _, err := os.Stat(p); err != nil {
			t.Errorf("file should not be removed on local governance hub: %v", err)
		}
	})

	t.Run("missing_workflows_dir", func(t *testing.T) {
		repoRoot := t.TempDir()
		res, err := ReconcileConsumerWorkflows(repoRoot, "org/clean-repo", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Pruned) != 0 || len(res.OptedOut) != 0 || len(res.Findings) != 0 {
			t.Fatalf("expected empty result, got %+v", res)
		}
	})
}

func TestReconcileConsumerWorkflows_AutoPrunesLegacyWorkflows(t *testing.T) {
	augursDeploy := `name: Build and Push Docker Image
on:
  push:
    branches: [main]
    tags: ['v*']
jobs:
  build-and-push:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/metadata-action@v5
        with:
          images: ghcr.io/${{ github.repository }}
      - uses: docker/build-push-action@v5
        with:
          push: true
`

	t.Run("legacy_ghcr_deploy_dry_run_preserves_then_prunes", func(t *testing.T) {
		repoRoot := t.TempDir()
		deployPath := writeTestWorkflow(t, repoRoot, "deploy.yml", augursDeploy)

		dryRes, err := ReconcileConsumerWorkflows(repoRoot, "Cooper-Wallace/augurs-saber", true)
		if err != nil {
			t.Fatalf("dryRun failed: %v", err)
		}
		if len(dryRes.Pruned) != 0 || len(dryRes.Findings) != 0 {
			t.Fatalf("expected empty Pruned/Findings on dryRun, got %+v", dryRes)
		}
		if _, err := os.Stat(deployPath); err != nil {
			t.Fatalf("deploy.yml should be preserved during dryRun: %v", err)
		}

		res, err := ReconcileConsumerWorkflows(repoRoot, "Cooper-Wallace/augurs-saber", false)
		if err != nil {
			t.Fatalf("reconcile failed: %v", err)
		}
		if len(res.Pruned) != 1 || res.Pruned[0] != ".github/workflows/deploy.yml" {
			t.Fatalf("expected [.github/workflows/deploy.yml] pruned, got %+v", res.Pruned)
		}
		if len(res.Findings) != 0 {
			t.Fatalf("expected no findings, got %+v", res.Findings)
		}
		if _, err := os.Stat(deployPath); !os.IsNotExist(err) {
			t.Fatalf("expected deploy.yml to be deleted, stat err=%v", err)
		}
	})

	t.Run("legacy_semantic_pr_and_dependabot_auto_pruned", func(t *testing.T) {
		repoRoot := t.TempDir()
		semPath := writeTestWorkflow(t, repoRoot, "semantic-pr-old.yml", `name: Semantic PR
on:
  pull_request_target:
jobs:
  semantic:
    runs-on: ubuntu-latest
    steps:
      - uses: amannn/action-semantic-pull-request@v5
`)
		depPath := writeTestWorkflow(t, repoRoot, "dependabot-auto.yml", `name: Dependabot Auto Merge
on: pull_request_target
jobs:
  automerge:
    runs-on: ubuntu-latest
    steps:
      - uses: dependabot/fetch-metadata@v2
      - run: gh pr merge --auto --squash "$PR_URL"
`)
		depBotPath := writeTestWorkflow(t, repoRoot, "dependabot-bot-merge.yaml", `name: Auto Merge Dependabot
on: pull_request_target
jobs:
  merge:
    if: github.actor == 'dependabot[bot]'
    runs-on: ubuntu-latest
    steps:
      - run: gh pr merge --auto --squash "$PR_URL"
`)

		res, err := ReconcileConsumerWorkflows(repoRoot, "org/app", false)
		if err != nil {
			t.Fatalf("reconcile failed: %v", err)
		}
		expected := []string{
			".github/workflows/dependabot-auto.yml",
			".github/workflows/dependabot-bot-merge.yaml",
			".github/workflows/semantic-pr-old.yml",
		}
		if len(res.Pruned) != len(expected) {
			t.Fatalf("expected %v pruned, got %v", expected, res.Pruned)
		}
		for i, exp := range expected {
			if res.Pruned[i] != exp {
				t.Errorf("pruned[%d] = %q, want %q", i, res.Pruned[i], exp)
			}
		}
		for _, p := range []string{semPath, depPath, depBotPath} {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("expected %s to be deleted, err=%v", p, err)
			}
		}
	})
}

func TestReconcileConsumerWorkflows_OptOutKeep(t *testing.T) {
	repoRoot := t.TempDir()
	keepDeploy := writeTestWorkflow(t, repoRoot, "deploy.yml", `# repokit: keep
name: Custom Deploy
on: push
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@v5
        with:
          tags: ghcr.io/org/app:latest
`)
	keepTest := writeTestWorkflow(t, repoRoot, "legacy-ci.yaml", `name: Legacy CI
#repokit:keep
on: pull_request
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: pytest
`)

	res, err := ReconcileConsumerWorkflows(repoRoot, "org/app", false)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if len(res.Pruned) != 0 {
		t.Fatalf("expected no pruned files, got %v", res.Pruned)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("expected no findings for opted-out files, got %+v", res.Findings)
	}
	if len(res.OptedOut) != 2 ||
		res.OptedOut[0] != ".github/workflows/deploy.yml" ||
		res.OptedOut[1] != ".github/workflows/legacy-ci.yaml" {
		t.Fatalf("unexpected OptedOut: %v", res.OptedOut)
	}
	for _, p := range []string{keepDeploy, keepTest} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("opted-out file %s must be preserved: %v", p, err)
		}
	}
}

func TestReconcileConsumerWorkflows_RequiresMigrationFindings(t *testing.T) {
	repoRoot := t.TempDir()

	writeTestWorkflow(t, repoRoot, "codeql-legacy.yml", `name: CodeQL
on: push
jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: github/codeql-action/analyze@v3
`)
	writeTestWorkflow(t, repoRoot, "custom-release.yml", `name: Release
on:
  push:
    tags: ['v*']
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: softprops/action-gh-release@v2
`)
	writeTestWorkflow(t, repoRoot, "deploy-dockerhub.yml", `name: Docker Hub
on: push
jobs:
  docker:
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@v5
        with:
          tags: docker.io/myorg/app:latest
`)
	writeTestWorkflow(t, repoRoot, "deploy.yml", `name: Build and Deploy to K8s
on: push
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@v5
        with:
          tags: ghcr.io/org/app:latest
      - run: kubectl apply -f k8s/ && helm upgrade --install app ./chart
`)
	writeTestWorkflow(t, repoRoot, "pr-multi.yml", `name: PR Checks
on: pull_request_target
jobs:
  semantic:
    runs-on: ubuntu-latest
    steps:
      - uses: amannn/action-semantic-pull-request@v5
  notify:
    runs-on: ubuntu-latest
    steps:
      - run: echo "checking PR"
`)
	writeTestWorkflow(t, repoRoot, "test-legacy.yml", `name: Python Checks
on: pull_request
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: ruff check .
      - run: pytest -q
`)

	res, err := ReconcileConsumerWorkflows(repoRoot, "org/app", false)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if len(res.Pruned) != 0 {
		t.Fatalf("expected 0 pruned files, got %v", res.Pruned)
	}
	if len(res.Findings) != 6 {
		t.Fatalf("expected 6 findings, got %d: %+v", len(res.Findings), res.Findings)
	}

	byFile := make(map[string]WorkflowMigrationFinding)
	for _, f := range res.Findings {
		byFile[f.File] = f
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(f.File))); err != nil {
			t.Errorf("finding file %s should NOT be deleted: %v", f.File, err)
		}
	}

	if f := byFile[".github/workflows/deploy.yml"]; f.Overlap != "Container Release" ||
		!strings.Contains(f.RecommendedTemplate, "reusable-release-docker.yml@v1") ||
		!strings.Contains(f.Reason, "kubectl") {
		t.Errorf("unexpected finding for deploy.yml: %+v", f)
	}
	if f := byFile[".github/workflows/deploy-dockerhub.yml"]; f.Overlap != "Container Release" {
		t.Errorf("unexpected finding for deploy-dockerhub.yml: %+v", f)
	}
	if f := byFile[".github/workflows/test-legacy.yml"]; f.Overlap != "CI / Test & Lint" ||
		f.RecommendedTemplate != ".github/workflows/ci.yml" {
		t.Errorf("unexpected finding for test-legacy.yml: %+v", f)
	}
	if f := byFile[".github/workflows/custom-release.yml"]; f.Overlap != "Release Automation" ||
		f.RecommendedTemplate != ".github/workflows/release.yml" {
		t.Errorf("unexpected finding for custom-release.yml: %+v", f)
	}
	if f := byFile[".github/workflows/codeql-legacy.yml"]; f.Overlap != "Security / CodeQL" {
		t.Errorf("unexpected finding for codeql-legacy.yml: %+v", f)
	}
	if f := byFile[".github/workflows/pr-multi.yml"]; f.Overlap != "PR / Dependency Governance" ||
		f.RecommendedTemplate != ".github/workflows/semantic-pr.yml" {
		t.Errorf("unexpected finding for pr-multi.yml: %+v", f)
	}
}

func TestReconcileConsumerWorkflows_IgnoresUnrelatedWorkflows(t *testing.T) {
	repoRoot := t.TempDir()
	nightlyPath := writeTestWorkflow(t, repoRoot, "nightly-scrape.yml", `name: Nightly Data Scrape
on:
  schedule:
    - cron: '0 3 * * *'
jobs:
  scrape:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: python scripts/scrape.py --target augurs-bazaar
`)

	res, err := ReconcileConsumerWorkflows(repoRoot, "Cooper-Wallace/augurs-bazaar", false)
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if len(res.Pruned) != 0 || len(res.OptedOut) != 0 || len(res.Findings) != 0 {
		t.Fatalf("expected unrelated workflow to be ignored cleanly, got %+v", res)
	}
	if _, err := os.Stat(nightlyPath); err != nil {
		t.Errorf("unrelated workflow should remain untouched: %v", err)
	}
}

func TestHasCustomDeploymentSteps_AvoidsSubstringFalsePositives(t *testing.T) {
	if has, tool := hasCustomDeploymentSteps("repo: Cooper-Wallace/augurs-bazaar\nrun: python main.py"); has {
		t.Errorf("expected no custom deployment step match on 'augurs-bazaar', got tool=%q", tool)
	}
	if has, tool := hasCustomDeploymentSteps("run: influx query 'from(bucket:\"x\")'"); has {
		t.Errorf("expected no custom deployment step match on 'influx', got tool=%q", tool)
	}
	for _, tc := range []struct {
		snippet string
		want    string
	}{
		{"run: kubectl apply -f k8s/", "kubectl"},
		{"run: helm upgrade --install web ./chart", "helm"},
		{"run: aws ecr get-login-password", "aws"},
		{"run: az aks get-credentials -g rg -n cluster", "az"},
		{"uses: actions/deploy-pages@v4", "actions/deploy-pages"},
	} {
		has, tool := hasCustomDeploymentSteps(tc.snippet)
		if !has || tool != tc.want {
			t.Errorf("hasCustomDeploymentSteps(%q) = (%v, %q), want (true, %q)", tc.snippet, has, tool, tc.want)
		}
	}
}

func TestEnsureWorkflowMigrationIssue(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	sampleFindings := []WorkflowMigrationFinding{
		{
			File:                ".github/workflows/deploy.yml",
			Overlap:             "Container Release",
			RecommendedTemplate: ".github/workflows/release.yml (reusable-release-docker.yml@v1)",
			Reason:              "Contains docker/build-push-action alongside custom deployment step (kubectl)",
		},
		{
			File:                ".github/workflows/test-legacy.yml",
			Overlap:             "CI / Test & Lint",
			RecommendedTemplate: ".github/workflows/ci.yml",
			Reason:              "Contains custom CI/test/lint steps that cannot be safely auto-removed",
		},
	}

	t.Run("empty_findings_noop", func(t *testing.T) {
		called := false
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			called = true
			return "", "", 0, nil
		}
		num, err := EnsureWorkflowMigrationIssue("org/app", nil, "")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if num != 0 {
			t.Errorf("expected 0, got %d", num)
		}
		if called {
			t.Errorf("expected no command calls for empty findings")
		}
	})

	t.Run("creates_new_issue_with_markdown_table_and_pr_ref", func(t *testing.T) {
		var createdRepo, createdTitle, createdLabel, createdBody string
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					for i := 0; i < len(args)-1; i++ {
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
					return "https://github.com/org/app/issues/88\n", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureWorkflowMigrationIssue("org/app", sampleFindings, "https://github.com/org/app/pull/12")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if num != 88 {
			t.Errorf("expected issue number 88, got %d", num)
		}
		if createdRepo != "org/app" {
			t.Errorf("repo = %q, want org/app", createdRepo)
		}
		if createdTitle != "chore(ci): migrate overlapping legacy workflows to repokit templates" {
			t.Errorf("unexpected title: %q", createdTitle)
		}
		if createdLabel != "governance-migration,automated" {
			t.Errorf("unexpected labels: %q", createdLabel)
		}
		for _, needle := range []string{
			"https://github.com/org/app/pull/12",
			"Workflow File | Overlapping Capability | Recommended Template | Why Not Auto-Removed",
			".github/workflows/deploy.yml",
			"Container Release",
			".github/workflows/test-legacy.yml",
			"# repokit: keep",
		} {
			if !strings.Contains(createdBody, needle) {
				t.Errorf("issue body missing %q:\n%s", needle, createdBody)
			}
		}
	})

	t.Run("idempotent_when_open_issue_exists", func(t *testing.T) {
		createCalled := false
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return `[{"number": 55, "title": "chore(ci): migrate overlapping legacy workflows to repokit templates"}]`, "", 0, nil
				}
				if args[2] == "create" {
					createCalled = true
					return "", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureWorkflowMigrationIssue("org/app", sampleFindings, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if num != 55 {
			t.Errorf("expected 55, got %d", num)
		}
		if createCalled {
			t.Errorf("expected gh issue create not to be called when open issue exists")
		}
	})

	t.Run("missing_labels_provisions_and_retries", func(t *testing.T) {
		var createdLabels []string
		createAttempts := 0
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" {
				if args[2] == "list" {
					return "[]", "", 0, nil
				}
				if args[2] == "create" {
					createAttempts++
					if createAttempts == 1 {
						return "", "label \"governance-migration\" not found", 1, fmt.Errorf("exit status 1")
					}
					return "https://github.com/org/app/issues/91\n", "", 0, nil
				}
			}
			if len(args) >= 4 && args[0] == "gh" && args[1] == "label" && args[2] == "create" {
				createdLabels = append(createdLabels, args[3])
				return "", "", 0, nil
			}
			return defaultCommandRunner(args, dir, input)
		}

		num, err := EnsureWorkflowMigrationIssue("org/app", sampleFindings, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if num != 91 {
			t.Errorf("expected 91, got %d", num)
		}
		if createAttempts != 2 {
			t.Errorf("expected 2 issue create attempts, got %d", createAttempts)
		}
		if len(createdLabels) != 2 || createdLabels[0] != "governance-migration" || createdLabels[1] != "automated" {
			t.Errorf("expected governance-migration and automated labels created, got %v", createdLabels)
		}
	})

	t.Run("list_error_surfaces", func(t *testing.T) {
		CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
			if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" && args[2] == "list" {
				return "", "API error: forbidden", 1, fmt.Errorf("exit status 1")
			}
			return defaultCommandRunner(args, dir, input)
		}

		_, err := EnsureWorkflowMigrationIssue("org/app", sampleFindings, "")
		if err == nil || !strings.Contains(err.Error(), "forbidden") {
			t.Errorf("expected forbidden error to surface, got: %v", err)
		}
	})
}

func TestConfigureSingleRepository_ReconcilesAndRaisesMigrationIssue(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	repoDir := t.TempDir()
	_ = exec.Command("git", "init", "-b", "main", repoDir).Run()
	_ = exec.Command("git", "-C", repoDir, "remote", "add", "origin", "https://github.com/org/reconcile-app.git").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Test").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()

	legacyDeploy := writeTestWorkflow(t, repoDir, "deploy.yml", `name: Docker
on: push
jobs:
  docker:
    runs-on: ubuntu-latest
    steps:
      - uses: docker/build-push-action@v5
        with:
          tags: ghcr.io/org/reconcile-app:latest
`)
	legacyTest := writeTestWorkflow(t, repoDir, "test-legacy.yml", `name: Legacy Tests
on: pull_request
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: pytest
`)
	_ = exec.Command("git", "-C", repoDir, "add", "-A").Run()
	_ = exec.Command("git", "-C", repoDir, "commit", "-m", "chore: initial workflows").Run()

	var issueCreated bool
	var issueBody string
	CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
		if len(args) > 0 && args[0] == "git" {
			for _, a := range args {
				if a == "push" || a == "checkout" || a == "set-url" {
					return "", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}
		if len(args) >= 3 && args[0] == "gh" && args[1] == "pr" && args[2] == "create" {
			return "https://github.com/org/reconcile-app/pull/30\n", "", 0, nil
		}
		if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" && args[2] == "list" {
			return "[]", "", 0, nil
		}
		if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" && args[2] == "create" {
			issueCreated = true
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--body" {
					issueBody = args[i+1]
				}
			}
			return "https://github.com/org/reconcile-app/issues/31\n", "", 0, nil
		}
		return "{}", "", 0, nil
	}

	err := ConfigureWithOptions(ConfigureOptions{
		TargetDir: repoDir,
		CreatePR:  true,
	})
	if err != nil {
		t.Fatalf("ConfigureWithOptions failed: %v", err)
	}

	if _, err := os.Stat(legacyDeploy); !os.IsNotExist(err) {
		t.Errorf("expected legacy deploy.yml to be auto-pruned, stat err=%v", err)
	}
	if _, err := os.Stat(legacyTest); err != nil {
		t.Errorf("expected test-legacy.yml to remain on disk: %v", err)
	}
	if !issueCreated {
		t.Fatalf("expected migration issue to be created")
	}
	if !strings.Contains(issueBody, "https://github.com/org/reconcile-app/pull/30") {
		t.Errorf("expected migration issue body to reference sync PR URL, got:\n%s", issueBody)
	}
	if !strings.Contains(issueBody, ".github/workflows/test-legacy.yml") {
		t.Errorf("expected migration issue body to list test-legacy.yml, got:\n%s", issueBody)
	}
}

func TestConfigureMultipleRepositories_ReconcilesAndRaisesMigrationIssue(t *testing.T) {
	oldRunner := CommandRunner
	defer func() { CommandRunner = oldRunner }()

	baseDir := t.TempDir()
	repoDir := filepath.Join(baseDir, "multi-reconcile")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = exec.Command("git", "init", "-b", "main", repoDir).Run()
	_ = exec.Command("git", "-C", repoDir, "remote", "add", "origin", "https://github.com/org/multi-reconcile.git").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.name", "Test").Run()
	_ = exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()

	legacySemantic := writeTestWorkflow(t, repoDir, "semantic-pr-old.yml", `name: Semantic PR
on: pull_request_target
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: amannn/action-semantic-pull-request@v5
`)
	customRelease := writeTestWorkflow(t, repoDir, "custom-release.yml", `name: Custom Release
on:
  push:
    tags: ['v*']
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: softprops/action-gh-release@v2
`)
	_ = exec.Command("git", "-C", repoDir, "add", "-A").Run()
	_ = exec.Command("git", "-C", repoDir, "commit", "-m", "chore: initial workflows").Run()

	var issueCreated bool
	var issueBody string
	CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
		if len(args) > 0 && args[0] == "git" {
			for _, a := range args {
				if a == "push" || a == "checkout" || a == "set-url" {
					return "", "", 0, nil
				}
			}
			return defaultCommandRunner(args, dir, input)
		}
		if len(args) >= 3 && args[0] == "gh" && args[1] == "pr" && args[2] == "create" {
			return "https://github.com/org/multi-reconcile/pull/44\n", "", 0, nil
		}
		if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" && args[2] == "list" {
			return "[]", "", 0, nil
		}
		if len(args) >= 3 && args[0] == "gh" && args[1] == "issue" && args[2] == "create" {
			issueCreated = true
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "--body" {
					issueBody = args[i+1]
				}
			}
			return "https://github.com/org/multi-reconcile/issues/45\n", "", 0, nil
		}
		return "{}", "", 0, nil
	}

	err := ConfigureWithOptions(ConfigureOptions{
		Local:    true,
		LocalDir: baseDir,
		CreatePR: true,
	})
	if err != nil {
		t.Fatalf("ConfigureWithOptions failed: %v", err)
	}

	if _, err := os.Stat(legacySemantic); !os.IsNotExist(err) {
		t.Errorf("expected legacy semantic-pr-old.yml to be auto-pruned, stat err=%v", err)
	}
	if _, err := os.Stat(customRelease); err != nil {
		t.Errorf("expected custom-release.yml to remain on disk: %v", err)
	}
	if !issueCreated {
		t.Fatalf("expected migration issue to be created")
	}
	if !strings.Contains(issueBody, "https://github.com/org/multi-reconcile/pull/44") {
		t.Errorf("expected migration issue body to reference sync PR URL, got:\n%s", issueBody)
	}
	if !strings.Contains(issueBody, ".github/workflows/custom-release.yml") {
		t.Errorf("expected migration issue body to list custom-release.yml, got:\n%s", issueBody)
	}
}

