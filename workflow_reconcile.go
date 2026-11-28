package repokit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// WorkflowReconcileStatus classifies the reconciliation outcome for a consumer workflow file.
type WorkflowReconcileStatus string

const (
	WorkflowStatusOptedOut          WorkflowReconcileStatus = "opted-out"
	WorkflowStatusPruned            WorkflowReconcileStatus = "pruned"
	WorkflowStatusRequiresMigration WorkflowReconcileStatus = "requires-migration"
	WorkflowStatusIgnored           WorkflowReconcileStatus = "ignored"
)

// WorkflowMigrationFinding describes an overlapping legacy workflow that could not be safely auto-removed.
type WorkflowMigrationFinding struct {
	File                string
	Overlap             string
	RecommendedTemplate string
	Reason              string
}

// WorkflowReconcileResult summarizes the outcome of reconciling consumer workflow files.
type WorkflowReconcileResult struct {
	Pruned   []string
	OptedOut []string
	Findings []WorkflowMigrationFinding
}

var managedWorkflowBasenames = map[string]bool{
	"ci.yml":                   true,
	"release.yml":              true,
	"semantic-pr.yml":          true,
	"dependabot-automerge.yml": true,
	"sync.yml":                 true,
}

var repokitKeepPattern = regexp.MustCompile(`(?i)#\s*repokit\s*:\s*keep\b`)

type namedPattern struct {
	label string
	re    *regexp.Regexp
}

var customDeploymentPatterns = []namedPattern{
	{"actions/deploy-pages", regexp.MustCompile(`(?i)actions/deploy-pages`)},
	{"peaceiris/actions-gh-pages", regexp.MustCompile(`(?i)peaceiris/actions-gh-pages`)},
	{"helm/chart-releaser", regexp.MustCompile(`(?i)helm/chart-releaser`)},
	{"kubectl", regexp.MustCompile(`(?i)\bkubectl\b`)},
	{"helm", regexp.MustCompile(`(?im)(?:^|[\s;|&("'])helm(?:\s|$)`)},
	{"terraform", regexp.MustCompile(`(?i)\bterraform\b`)},
	{"ansible", regexp.MustCompile(`(?i)\bansible\b`)},
	{"ssh", regexp.MustCompile(`(?im)(?:^|[\s;|&("'])ssh(?:\s|$)`)},
	{"rsync", regexp.MustCompile(`(?i)\brsync\b`)},
	{"aws", regexp.MustCompile(`(?im)(?:^|[\s;|&("'])aws(?:\s|$)`)},
	{"gcloud", regexp.MustCompile(`(?i)\bgcloud\b`)},
	{"az", regexp.MustCompile(`(?im)(?:^|[\s;|&("'])az(?:\s|$)`)},
	{"flyctl", regexp.MustCompile(`(?i)\bflyctl\b`)},
	{"netlify", regexp.MustCompile(`(?i)\bnetlify\b`)},
	{"vercel", regexp.MustCompile(`(?i)\bvercel\b`)},
	{"argocd", regexp.MustCompile(`(?i)\bargocd\b`)},
	{"flux", regexp.MustCompile(`(?im)(?:^|[\s;|&("'])flux(?:\s|$)`)},
	{"pulumi", regexp.MustCompile(`(?i)\bpulumi\b`)},
}

var ciStepPatterns = []namedPattern{
	{"pytest", regexp.MustCompile(`(?i)\bpytest\b`)},
	{"ruff", regexp.MustCompile(`(?im)(?:^|[\s;|&("'])ruff(?:\s|$)`)},
	{"mypy", regexp.MustCompile(`(?i)\bmypy\b`)},
	{"go test", regexp.MustCompile(`(?i)\bgo\s+test\b`)},
	{"golangci-lint", regexp.MustCompile(`(?i)\bgolangci-lint\b`)},
	{"npm test", regexp.MustCompile(`(?i)\bnpm\s+test\b`)},
	{"pnpm test", regexp.MustCompile(`(?i)\bpnpm\s+test\b`)},
	{"yarn test", regexp.MustCompile(`(?i)\byarn\s+test\b`)},
	{"mvn test", regexp.MustCompile(`(?i)\bmvn\s+test\b`)},
	{"gradle test", regexp.MustCompile(`(?i)\bgradle\s+test\b`)},
	{"shellcheck", regexp.MustCompile(`(?i)\bshellcheck\b`)},
	{"bats", regexp.MustCompile(`(?im)(?:^|[\s;|&("'])bats(?:\s|$)`)},
}

var releaseStepPatterns = []namedPattern{
	{"softprops/action-gh-release", regexp.MustCompile(`(?i)softprops/action-gh-release`)},
	{"googleapis/release-please-action", regexp.MustCompile(`(?i)googleapis/release-please-action`)},
	{"goreleaser/goreleaser-action", regexp.MustCompile(`(?i)goreleaser/goreleaser-action`)},
	{"pypa/gh-action-pypi-publish", regexp.MustCompile(`(?i)pypa/gh-action-pypi-publish`)},
}

var securityStepPatterns = []namedPattern{
	{"github/codeql-action", regexp.MustCompile(`(?i)github/codeql-action`)},
	{"aquasecurity/trivy-action", regexp.MustCompile(`(?i)aquasecurity/trivy-action`)},
	{"gitleaks", regexp.MustCompile(`(?i)\bgitleaks\b`)},
}

var nonGHCRRegistryPatterns = []namedPattern{
	{"docker.io", regexp.MustCompile(`(?i)\bdocker\.io\b`)},
	{"gcr.io", regexp.MustCompile(`(?i)\bgcr\.io\b`)},
	{"pkg.dev", regexp.MustCompile(`(?i)\bpkg\.dev\b`)},
	{"azurecr.io", regexp.MustCompile(`(?i)\bazurecr\.io\b`)},
	{"quay.io", regexp.MustCompile(`(?i)\bquay\.io\b`)},
	{"ecr", regexp.MustCompile(`(?i)(?:dkr\.ecr\.|public\.ecr\.aws)`)},
}

func matchFirstPattern(content string, patterns []namedPattern) (bool, string) {
	for _, p := range patterns {
		if p.re.MatchString(content) {
			return true, p.label
		}
	}
	return false, ""
}

func hasCustomDeploymentSteps(content string) (bool, string) {
	return matchFirstPattern(content, customDeploymentPatterns)
}

func isGovernanceHubOrExempt(repoRoot string, slug string) bool {
	if repoRoot != "" && IsGovernanceHub(repoRoot) {
		return true
	}
	if slug == "" {
		return false
	}
	if strings.EqualFold(slug, DefaultGovernanceRepo) ||
		strings.EqualFold(slug, "Cooper-Wallace/repokit") ||
		strings.HasSuffix(strings.ToLower(slug), "/repokit") {
		return true
	}
	return IsVaultOrExemptRepo(slug)
}

func countWorkflowJobs(content string) int {
	lines := strings.Split(content, "\n")
	inJobs := false
	jobIndent := -1
	jobs := 0

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if !inJobs {
			if indent == 0 && (trimmed == "jobs:" || strings.HasPrefix(trimmed, "jobs: ") || strings.HasPrefix(trimmed, "jobs:#")) {
				inJobs = true
			}
			continue
		}

		if indent == 0 {
			break
		}
		if jobIndent == -1 {
			jobIndent = indent
		}
		if indent == jobIndent && !strings.HasPrefix(trimmed, "-") {
			if idx := strings.Index(trimmed, ":"); idx > 0 {
				jobs++
			}
		}
	}
	return jobs
}

// ReconcileConsumerWorkflows inspects non-managed workflows under .github/workflows/,
// safely pruning superseded legacy workflows and reporting overlapping workflows that require migration.
func ReconcileConsumerWorkflows(repoRoot string, slug string, dryRun bool) (*WorkflowReconcileResult, error) {
	result := &WorkflowReconcileResult{}
	if isGovernanceHubOrExempt(repoRoot, slug) || repoRoot == "" {
		return result, nil
	}

	workflowsDir := filepath.Join(repoRoot, ".github", "workflows")
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}

	var candidateNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yml" && ext != ".yaml" {
			continue
		}
		if managedWorkflowBasenames[strings.ToLower(name)] {
			continue
		}
		candidateNames = append(candidateNames, name)
	}
	sort.Strings(candidateNames)

	for _, name := range candidateNames {
		fullPath := filepath.Join(workflowsDir, name)
		relPath := filepath.ToSlash(filepath.Join(".github", "workflows", name))

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read workflow %s: %w", relPath, err)
		}
		content := string(data)

		status, finding, pruneReason := classifyWorkflowFile(relPath, content)
		switch status {
		case WorkflowStatusOptedOut:
			result.OptedOut = append(result.OptedOut, relPath)
		case WorkflowStatusPruned:
			if dryRun {
				LogInfo("[DRY RUN] Would prune legacy workflow %s (%s)", relPath, pruneReason)
			} else {
				if err := os.Remove(fullPath); err != nil {
					return nil, fmt.Errorf("failed to prune legacy workflow %s: %w", relPath, err)
				}
				result.Pruned = append(result.Pruned, relPath)
				LogInfo("Pruned legacy workflow %s (%s)", relPath, pruneReason)
			}
		case WorkflowStatusRequiresMigration:
			if finding != nil {
				result.Findings = append(result.Findings, *finding)
			}
		case WorkflowStatusIgnored:
			// Unrelated repo-specific workflow; leave untouched.
		}
	}

	return result, nil
}

func classifyWorkflowFile(relPath string, content string) (WorkflowReconcileStatus, *WorkflowMigrationFinding, string) {
	if repokitKeepPattern.MatchString(content) {
		return WorkflowStatusOptedOut, nil, ""
	}

	lower := strings.ToLower(content)
	hasCustom, customTool := hasCustomDeploymentSteps(content)
	hasCI, ciTool := matchFirstPattern(content, ciStepPatterns)
	hasRelease, _ := matchFirstPattern(content, releaseStepPatterns)
	hasSecurity, _ := matchFirstPattern(content, securityStepPatterns)
	hasDocker := strings.Contains(lower, "docker/build-push-action")
	hasGHCR := strings.Contains(lower, "ghcr.io")
	hasNonGHCR, nonGHCRReg := matchFirstPattern(content, nonGHCRRegistryPatterns)
	hasSemanticPR := strings.Contains(lower, "amannn/action-semantic-pull-request")
	hasDependabot := strings.Contains(lower, "dependabot/fetch-metadata") ||
		(strings.Contains(lower, "dependabot[bot]") && strings.Contains(lower, "gh pr merge --auto"))
	jobCount := countWorkflowJobs(content)

	// Case A — Safe Auto-Prune (never prune when custom deployment/test/release/security steps exist)
	if !hasCustom && !hasCI && !hasRelease && !hasSecurity {
		if hasDocker && hasGHCR && !hasNonGHCR && !hasSemanticPR && !hasDependabot {
			return WorkflowStatusPruned, nil, "superseded by release.yml / reusable-release-docker.yml@v1"
		}
		if hasSemanticPR && !hasDocker && !hasDependabot && jobCount <= 1 {
			return WorkflowStatusPruned, nil, "superseded by semantic-pr.yml"
		}
		if hasDependabot && !hasDocker && !hasSemanticPR && jobCount <= 1 {
			return WorkflowStatusPruned, nil, "superseded by dependabot-automerge.yml"
		}
	}

	// Case B — Requires Migration
	if hasDocker {
		var reason string
		switch {
		case hasCustom:
			reason = fmt.Sprintf("Contains docker/build-push-action alongside custom deployment step (%s) that cannot be safely auto-removed", customTool)
		case !hasGHCR:
			reason = "Targets a non-GHCR container registry or lacks ghcr.io configuration"
		case hasNonGHCR:
			reason = fmt.Sprintf("Targets non-GHCR container registry (%s) in addition to ghcr.io", nonGHCRReg)
		case hasCI:
			reason = fmt.Sprintf("Combines docker/build-push-action with CI/test steps (%s) that cannot be safely auto-removed", ciTool)
		default:
			reason = "Contains container release steps alongside other workflow logic that cannot be safely auto-removed"
		}
		return WorkflowStatusRequiresMigration, &WorkflowMigrationFinding{
			File:                relPath,
			Overlap:             "Container Release",
			RecommendedTemplate: ".github/workflows/release.yml (reusable-release-docker.yml@v1)",
			Reason:              reason,
		}, ""
	}

	if hasCI {
		return WorkflowStatusRequiresMigration, &WorkflowMigrationFinding{
			File:                relPath,
			Overlap:             "CI / Test & Lint",
			RecommendedTemplate: ".github/workflows/ci.yml",
			Reason:              "Contains custom CI/test/lint steps that cannot be safely auto-removed",
		}, ""
	}

	if hasRelease {
		return WorkflowStatusRequiresMigration, &WorkflowMigrationFinding{
			File:                relPath,
			Overlap:             "Release Automation",
			RecommendedTemplate: ".github/workflows/release.yml",
			Reason:              "Contains custom release publishing steps that overlap with repokit release automation",
		}, ""
	}

	if hasSecurity {
		return WorkflowStatusRequiresMigration, &WorkflowMigrationFinding{
			File:                relPath,
			Overlap:             "Security / CodeQL",
			RecommendedTemplate: "GitHub CodeQL Default Setup & .github/workflows/ci.yml",
			Reason:              "Contains overlapping security scanning steps",
		}, ""
	}

	if hasSemanticPR || hasDependabot {
		template := ".github/workflows/semantic-pr.yml / .github/workflows/dependabot-automerge.yml"
		if hasSemanticPR && !hasDependabot {
			template = ".github/workflows/semantic-pr.yml"
		} else if hasDependabot && !hasSemanticPR {
			template = ".github/workflows/dependabot-automerge.yml"
		}
		reason := "Contains PR/dependency governance steps mixed with additional workflow jobs"
		if hasCustom {
			reason = fmt.Sprintf("Contains PR/dependency governance steps alongside custom deployment step (%s)", customTool)
		}
		return WorkflowStatusRequiresMigration, &WorkflowMigrationFinding{
			File:                relPath,
			Overlap:             "PR / Dependency Governance",
			RecommendedTemplate: template,
			Reason:              reason,
		}, ""
	}

	// Case C — Ignored
	return WorkflowStatusIgnored, nil, ""
}

func logWorkflowReconcileFindings(slug string, res *WorkflowReconcileResult) {
	if res == nil {
		return
	}
	for _, opted := range res.OptedOut {
		LogInfo("[%s] Preserving opted-out workflow %s (# repokit: keep)", slug, opted)
	}
	for _, f := range res.Findings {
		LogWarn("[%s] Overlapping workflow %s (%s) requires migration to %s: %s", slug, f.File, f.Overlap, f.RecommendedTemplate, f.Reason)
	}
}

func buildWorkflowMigrationIssueBody(findings []WorkflowMigrationFinding, prRef string) string {
	var b strings.Builder
	b.WriteString("`repokit` detected non-standard workflow(s) in `.github/workflows/` that overlap with centralized governance templates and could not be safely auto-removed.\n")
	if prRef != "" {
		b.WriteString(fmt.Sprintf("\nRelated governance sync PR: %s\n", prRef))
	}
	b.WriteString("\n| Workflow File | Overlapping Capability | Recommended Template | Why Not Auto-Removed |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	sanitizeCell := func(s string) string {
		return strings.ReplaceAll(s, "|", "\\|")
	}
	for _, f := range findings {
		b.WriteString(fmt.Sprintf("| `%s` | %s | `%s` | %s |\n",
			sanitizeCell(f.File),
			sanitizeCell(f.Overlap),
			sanitizeCell(f.RecommendedTemplate),
			sanitizeCell(f.Reason),
		))
	}
	b.WriteString("\n### Migration Instructions\n\n")
	b.WriteString("1. Adopt the corresponding `repokit` reusable workflow template (e.g. `.github/workflows/ci.yml`, `.github/workflows/release.yml`).\n")
	b.WriteString("2. Delete the legacy workflow file once migrated, **or** isolate only genuinely repo-specific deployment steps and add `# repokit: keep` at the top of the file to silence future alerts.\n")
	return b.String()
}

// EnsureWorkflowMigrationIssue idempotently creates or retrieves an open governance-migration issue
// on a consumer repository when overlapping legacy workflows could not be safely auto-removed.
func EnsureWorkflowMigrationIssue(slug string, findings []WorkflowMigrationFinding, prRef string) (int, error) {
	if len(findings) == 0 {
		return 0, nil
	}

	stdout, stderr, code, _ := RunCmd([]string{
		"gh", "issue", "list",
		"--repo", slug,
		"--label", "governance-migration",
		"--state", "open",
		"--json", "number,title",
	}, "")
	if code != 0 {
		return 0, fmt.Errorf("failed to list open governance-migration issues on %s: %s", slug, strings.TrimSpace(stderr))
	}

	var existing []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal([]byte(stdout), &existing); err == nil && len(existing) > 0 {
		num := existing[0].Number
		LogInfo("Existing governance-migration issue #%d found on %s", num, slug)
		return num, nil
	}

	const issueTitle = "chore(ci): migrate overlapping legacy workflows to repokit templates"
	bodyText := buildWorkflowMigrationIssueBody(findings, prRef)

	args := []string{
		"gh", "issue", "create",
		"--repo", slug,
		"--title", issueTitle,
		"--label", "governance-migration,automated",
		"--body", bodyText,
	}

	out, errOut, createCode, _ := RunCmd(args, "")
	if createCode != 0 {
		errLower := strings.ToLower(errOut)
		if strings.Contains(errLower, "label") || strings.Contains(errLower, "not found") {
			_ = EnsureRepoLabels(slug, "governance-migration", "automated")
			out, errOut, createCode, _ = RunCmd(args, "")
		}
		if createCode != 0 {
			errLower = strings.ToLower(errOut)
			if strings.Contains(errLower, "label") || strings.Contains(errLower, "not found") {
				argsNoLabel := []string{
					"gh", "issue", "create",
					"--repo", slug,
					"--title", issueTitle,
					"--body", bodyText,
				}
				out, errOut, createCode, _ = RunCmd(argsNoLabel, "")
			}
		}
		if createCode != 0 {
			return 0, fmt.Errorf("failed to create governance-migration issue on %s: %s", slug, strings.TrimSpace(errOut))
		}
	}

	outTrimmed := strings.TrimSpace(out)
	var issueNum int
	re := regexp.MustCompile(`/issues/(\d+)`)
	if match := re.FindStringSubmatch(outTrimmed); len(match) > 1 {
		issueNum, _ = strconv.Atoi(match[1])
	} else if num, err := strconv.Atoi(outTrimmed); err == nil {
		issueNum = num
	}

	LogSuccess("[%s] Created governance-migration issue #%d", slug, issueNum)
	return issueNum, nil
}
