"""
Tests for repokit CLI tool.
"""

import json
import os
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch

# Add bin directory to sys.path so we can import helper functions directly
REPO_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(REPO_ROOT / "bin"))

# Import functions from bin/repokit
import importlib.machinery
import importlib.util

loader = importlib.machinery.SourceFileLoader("repokit", str(REPO_ROOT / "bin" / "repokit"))
spec = importlib.util.spec_from_loader("repokit", loader)
repokit = importlib.util.module_from_spec(spec)
sys.modules["repokit"] = repokit
loader.exec_module(repokit)

PATTERN = repokit.COMMIT_PATTERN
BIN_CMD = [sys.executable, str(REPO_ROOT / "bin" / "repokit")]


class TestConventionalCommits(unittest.TestCase):
    def test_valid_commit_messages(self):
        valid = [
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
        ]
        for msg in valid:
            with self.subTest(msg=msg):
                self.assertIsNotNone(re.match(PATTERN, msg), f"Should match: {msg}")

    def test_invalid_commit_messages(self):
        invalid = [
            "WIP",
            "fixed a bug",
            "Added new feature",
            "feat:no space after colon",
            "feature: wrong type name",
            "fix(nav):",
            "chore(): empty scope",
            "Update README.md",
        ]
        for msg in invalid:
            with self.subTest(msg=msg):
                self.assertIsNone(re.match(PATTERN, msg), f"Should not match: {msg}")

    def test_commit_hook_execution(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            p = Path(tmpdir)
            subprocess.run(["git", "init", "-b", "main"], cwd=p, check=True, capture_output=True)
            subprocess.run([*BIN_CMD, "hook", "install"], cwd=p, check=True, capture_output=True)

            hook = p / ".git" / "hooks" / "commit-msg"
            self.assertTrue(hook.exists())

            subprocess.run(["git", "config", "user.name", "Test User"], cwd=p, check=True)
            subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=p, check=True)

            (p / "file.txt").write_text("hello")
            subprocess.run(["git", "add", "."], cwd=p, check=True)
            res_bad = subprocess.run(["git", "commit", "-m", "bad commit"], cwd=p, capture_output=True, text=True)
            self.assertNotEqual(res_bad.returncode, 0, "Bad commit should be rejected by hook")

            res_good = subprocess.run(["git", "commit", "-m", "feat(core): initial test commit"], cwd=p, capture_output=True, text=True)
            self.assertEqual(res_good.returncode, 0, f"Valid commit should succeed: {res_good.stderr}")

    def test_hook_install_pre_commit_and_commit_msg(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            p = Path(tmpdir)
            subprocess.run(["git", "init", "-b", "main"], cwd=p, check=True, capture_output=True)
            res = subprocess.run([*BIN_CMD, "hook", "install", "--all", str(p)], check=True, capture_output=True, text=True)
            self.assertEqual(res.returncode, 0)

            commit_hook = p / ".git" / "hooks" / "commit-msg"
            pre_hook = p / ".git" / "hooks" / "pre-commit"

            self.assertTrue(commit_hook.exists(), "commit-msg hook should exist")
            self.assertTrue(pre_hook.exists(), "pre-commit hook should exist")
            self.assertTrue(os.access(commit_hook, os.X_OK), "commit-msg hook should be executable")
            self.assertTrue(os.access(pre_hook, os.X_OK), "pre-commit hook should be executable")


class TestArchetypeTemplates(unittest.TestCase):
    def test_python_scaffold(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target = Path(tmpdir)
            variables = {
                "repo_name": "sample-pkg",
                "package_name": "sample_pkg",
                "description": "Sample python package",
                "author": "Cooper Wallace",
                "year": "2026",
                "repo_slug": "Cooper-Wallace/sample-pkg",
            }
            res = repokit.apply_archetype_template("python", target, variables)
            self.assertTrue(res)
            self.assertTrue((target / "pyproject.toml").exists())
            self.assertTrue((target / "src" / "sample_pkg" / "__init__.py").exists())
            self.assertTrue((target / "src" / "sample_pkg" / "core.py").exists())
            self.assertTrue((target / "tests" / "test_core.py").exists())
            self.assertTrue((target / ".github" / "workflows" / "ci.yml").exists())

            # Verify variable substitution
            content = (target / "src" / "sample_pkg" / "core.py").read_text()
            self.assertIn("sample-pkg", content)
            self.assertNotIn("{{repo_name}}", content)

            ci_content = (target / ".github" / "workflows" / "ci.yml").read_text()
            self.assertIn(repokit.DEFAULT_GOVERNANCE_REPO, ci_content)
            self.assertNotIn("{{governance_slug}}", ci_content)
            self.assertNotIn("cooperwallace/gh-standardise", ci_content)

            # Audit check
            checks = repokit.audit_archetype_structure(target, "python")
            self.assertTrue(all(passed for _, passed in checks))

    def test_java_scaffold(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target = Path(tmpdir)
            variables = {
                "repo_name": "sample-api",
                "package_name": "sample_api",
                "description": "Sample java api",
                "author": "Cooper Wallace",
                "year": "2026",
                "repo_slug": "Cooper-Wallace/sample-api",
            }
            res = repokit.apply_archetype_template("java", target, variables)
            self.assertTrue(res)
            self.assertTrue((target / "pom.xml").exists())
            self.assertTrue((target / "src/main/java/com/cooper/sample_api/Application.java").exists())
            self.assertTrue((target / "src/main/java/com/cooper/sample_api/controller/HealthController.java").exists())
            self.assertTrue((target / "src/test/java/com/cooper/sample_api/ApplicationTests.java").exists())
            self.assertTrue((target / ".github" / "workflows" / "ci.yml").exists())

            ci_content = (target / ".github" / "workflows" / "ci.yml").read_text()
            self.assertIn(repokit.DEFAULT_GOVERNANCE_REPO, ci_content)
            self.assertNotIn("{{governance_slug}}", ci_content)

            checks = repokit.audit_archetype_structure(target, "java")
            self.assertTrue(all(passed for _, passed in checks))

    def test_typescript_scaffold(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target = Path(tmpdir)
            variables = {
                "repo_name": "sample-ui",
                "package_name": "sample_ui",
                "description": "Sample frontend",
                "author": "Cooper Wallace",
                "year": "2026",
                "repo_slug": "Cooper-Wallace/sample-ui",
            }
            res = repokit.apply_archetype_template("typescript", target, variables)
            self.assertTrue(res)
            self.assertTrue((target / "package.json").exists())
            self.assertTrue((target / "tsconfig.json").exists())
            self.assertTrue((target / "vite.config.ts").exists())
            self.assertTrue((target / "src/main.ts").exists())
            self.assertTrue((target / "tests/api.test.ts").exists())
            self.assertTrue((target / ".github" / "workflows" / "ci.yml").exists())

            ci_content = (target / ".github" / "workflows" / "ci.yml").read_text()
            self.assertIn(repokit.DEFAULT_GOVERNANCE_REPO, ci_content)
            self.assertNotIn("{{governance_slug}}", ci_content)

            checks = repokit.audit_archetype_structure(target, "typescript")
            self.assertTrue(all(passed for _, passed in checks))

    def test_bash_scaffold(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target = Path(tmpdir)
            variables = {
                "repo_name": "sample-cli",
                "package_name": "sample_cli",
                "description": "Sample bash cli",
                "author": "Cooper Wallace",
                "year": "2026",
                "repo_slug": "Cooper-Wallace/sample-cli",
            }
            res = repokit.apply_archetype_template("bash", target, variables)
            self.assertTrue(res)
            self.assertTrue((target / "bin/sample-cli").exists())
            self.assertTrue((target / "install.sh").exists())
            self.assertTrue(os.access(target / "bin/sample-cli", os.X_OK))
            self.assertTrue((target / ".github" / "workflows" / "ci.yml").exists())

            ci_content = (target / ".github" / "workflows" / "ci.yml").read_text()
            self.assertIn(repokit.DEFAULT_GOVERNANCE_REPO, ci_content)
            self.assertNotIn("{{governance_slug}}", ci_content)

            checks = repokit.audit_archetype_structure(target, "bash")
            self.assertTrue(all(passed for _, passed in checks))

    def test_custom_governance_slug(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target = Path(tmpdir)
            custom_slug = "custom-org/custom-repokit"
            variables = {
                "repo_name": "custom-pkg",
                "package_name": "custom_pkg",
                "description": "Custom pkg",
                "author": "Cooper Wallace",
                "year": "2026",
                "repo_slug": "custom-org/custom-pkg",
                "governance_slug": custom_slug,
            }
            res = repokit.apply_archetype_template("python", target, variables)
            self.assertTrue(res)
            ci_content = (target / ".github" / "workflows" / "ci.yml").read_text()
            self.assertIn(custom_slug, ci_content)
            self.assertNotIn("{{governance_slug}}", ci_content)

    def test_scaffold_archetype_ci(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target = Path(tmpdir)
            (target / "pyproject.toml").write_text("[project]\nname = 'test'\n")
            res = repokit.scaffold_archetype_ci(target)
            self.assertTrue(res)
            ci_path = target / ".github" / "workflows" / "ci.yml"
            self.assertTrue(ci_path.exists())
            ci_content = ci_path.read_text()
            self.assertIn("Cooper-Wallace/repokit", ci_content)
            self.assertNotIn("{{governance_slug}}", ci_content)


class TestCLICommands(unittest.TestCase):
    def test_help_command(self):
        res = subprocess.run([*BIN_CMD, "--help"], capture_output=True, text=True)
        self.assertEqual(res.returncode, 0)
        self.assertIn("repokit", res.stdout)
        self.assertIn("create", res.stdout)
        self.assertIn("configure", res.stdout)
        self.assertIn("audit", res.stdout)
        self.assertIn("templates", res.stdout)

    def test_templates_command(self):
        res = subprocess.run([*BIN_CMD, "templates"], capture_output=True, text=True)
        self.assertEqual(res.returncode, 0)
        self.assertIn("python", res.stdout)
        self.assertIn("java", res.stdout)
        self.assertIn("typescript", res.stdout)
        self.assertIn("bash", res.stdout)

    def test_audit_local_fast(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            p = Path(tmpdir)
            subprocess.run(["git", "init", "-b", "main"], cwd=p, check=True, capture_output=True)

            # Audit should fail on empty repository without standards
            res_bad = subprocess.run([*BIN_CMD, "audit", str(p), "--local"], capture_output=True, text=True)
            self.assertNotEqual(res_bad.returncode, 0, "Audit should fail on empty repository")

            # Apply required local standards
            subprocess.run([*BIN_CMD, "hook", "install", "--all", str(p)], check=True, capture_output=True)
            (p / ".github").mkdir(parents=True, exist_ok=True)
            (p / ".github" / "pull_request_template.md").write_text("## Summary\n")
            (p / ".github" / "workflows").mkdir(parents=True, exist_ok=True)
            (p / ".github" / "workflows" / "ci.yml").write_text(
                "name: CI\njobs:\n  ci:\n    uses: Cooper-Wallace/repokit/.github/workflows/reusable-python.yml@main\n"
            )
            (p / "src").mkdir(parents=True, exist_ok=True)

            # Audit should now pass quickly
            res_good = subprocess.run([*BIN_CMD, "audit", str(p), "--local"], capture_output=True, text=True)
            self.assertEqual(res_good.returncode, 0, f"Audit should pass for valid repository: {res_good.stderr}")
            self.assertIn("All repository standards are strictly satisfied", res_good.stdout)
            self.assertIn("CI Workflow calling central repokit workflows", res_good.stdout)

    def test_audit_ci_workflow_enforcement(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            p = Path(tmpdir)
            subprocess.run(["git", "init", "-b", "main"], cwd=p, check=True, capture_output=True)
            subprocess.run([*BIN_CMD, "hook", "install", "--all", str(p)], check=True, capture_output=True)
            (p / ".github").mkdir(parents=True, exist_ok=True)
            (p / ".github" / "pull_request_template.md").write_text("## Summary\n")
            (p / "src").mkdir(parents=True, exist_ok=True)

            # Missing ci.yml
            res_missing = subprocess.run([*BIN_CMD, "audit", str(p), "--local"], capture_output=True, text=True)
            self.assertNotEqual(res_missing.returncode, 0)
            self.assertIn("CI Workflow missing or not calling central repokit reusable workflows", res_missing.stdout)

            # ci.yml exists but doesn't call central repokit reusable workflows
            (p / ".github" / "workflows").mkdir(parents=True, exist_ok=True)
            (p / ".github" / "workflows" / "ci.yml").write_text("name: Local CI\njobs:\n  build:\n    runs-on: ubuntu-latest\n")
            res_non_compliant = subprocess.run([*BIN_CMD, "audit", str(p), "--local"], capture_output=True, text=True)
            self.assertNotEqual(res_non_compliant.returncode, 0)
            self.assertIn("CI Workflow missing or not calling central repokit reusable workflows", res_non_compliant.stdout)

            # ci.yml calling central repokit reusable workflows
            (p / ".github" / "workflows" / "ci.yml").write_text(
                "name: CI\njobs:\n  ci:\n    uses: Cooper-Wallace/repokit/.github/workflows/reusable-bash.yml@main\n"
            )
            res_compliant = subprocess.run([*BIN_CMD, "audit", str(p), "--local"], capture_output=True, text=True)
            self.assertEqual(res_compliant.returncode, 0)
            self.assertIn("CI Workflow calling central repokit workflows", res_compliant.stdout)

    def test_audit_governance_hub_self_audit(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            p = Path(tmpdir)
            subprocess.run(["git", "init", "-b", "main"], cwd=p, check=True, capture_output=True)
            subprocess.run([*BIN_CMD, "hook", "install", "--all", str(p)], check=True, capture_output=True)
            (p / ".github" / "workflows").mkdir(parents=True, exist_ok=True)
            (p / ".github" / "pull_request_template.md").write_text("## Summary\n")
            (p / "bin").mkdir(parents=True, exist_ok=True)
            (p / "bin" / "repokit").write_text("#!/usr/bin/env python3\n")
            (p / "install.sh").write_text("#!/usr/bin/env bash\n")
            (p / "install.sh").chmod(0o755)
            (p / ".github" / "workflows" / "reusable-python.yml").write_text("name: Reusable Python\n")

            # Governance hub without test.yml should fail audit
            res_no_test = subprocess.run([*BIN_CMD, "audit", str(p), "--local"], capture_output=True, text=True)
            self.assertNotEqual(res_no_test.returncode, 0)
            self.assertIn("Central Reusable Workflows Hub Hosted (.github/workflows/reusable-*.yml)", res_no_test.stdout)

            # Governance hub with test.yml should pass audit
            (p / ".github" / "workflows" / "test.yml").write_text("name: Test\n")
            res_hub = subprocess.run([*BIN_CMD, "audit", str(p), "--local"], capture_output=True, text=True)
            self.assertEqual(res_hub.returncode, 0)
            self.assertIn("Central Reusable Workflows Hub Hosted (.github/workflows/reusable-*.yml)", res_hub.stdout)
            self.assertIn("All repository standards are strictly satisfied", res_hub.stdout)


class TestGovernanceAndQualityGates(unittest.TestCase):
    @patch.object(repokit.subprocess, "run")
    def test_apply_github_repo_settings_disables_auto_merge(self, mock_run):
        mock_run.return_value = subprocess.CompletedProcess(
            args=[], returncode=0, stdout="{}", stderr=""
        )
        res = repokit.apply_github_repo_settings("Cooper-Wallace/repokit")
        self.assertTrue(res)

        mock_run.assert_called_once()
        call_args, call_kwargs = mock_run.call_args
        self.assertIn("input", call_kwargs)
        payload = json.loads(call_kwargs["input"])

        self.assertIn("allow_auto_merge", payload)
        self.assertIs(payload["allow_auto_merge"], False)
        self.assertIs(payload["allow_squash_merge"], True)
        self.assertIs(payload["allow_merge_commit"], False)
        self.assertIs(payload["allow_rebase_merge"], False)
        self.assertEqual(payload["squash_merge_commit_title"], "PR_TITLE")
        self.assertEqual(payload["squash_merge_commit_message"], "PR_BODY")

    @patch.object(repokit.subprocess, "run")
    def test_apply_branch_ruleset_enforces_squash_and_status_checks(self, mock_run):
        # 1. Hub repository scenario (Cooper-Wallace/repokit)
        mock_run.side_effect = [
            subprocess.CompletedProcess(args=[], returncode=0, stdout="[]", stderr=""),
            subprocess.CompletedProcess(args=[], returncode=0, stdout="{}", stderr=""),
        ]

        res_hub = repokit.apply_branch_ruleset("Cooper-Wallace/repokit")
        self.assertTrue(res_hub)
        self.assertEqual(mock_run.call_count, 2)
        _, post_kwargs_hub = mock_run.call_args_list[1]
        payload_hub = json.loads(post_kwargs_hub["input"])

        pr_rule = next((r for r in payload_hub["rules"] if r.get("type") == "pull_request"), None)
        self.assertIsNotNone(pr_rule, "pull_request rule must be in ruleset")
        self.assertEqual(pr_rule["parameters"].get("allowed_merge_methods"), ["squash"])

        status_rule_hub = next((r for r in payload_hub["rules"] if r.get("type") == "required_status_checks"), None)
        self.assertIsNotNone(status_rule_hub, "required_status_checks rule must be in ruleset")
        self.assertTrue(status_rule_hub["parameters"].get("strict_required_status_checks_policy"))
        self.assertFalse(status_rule_hub["parameters"].get("do_not_enforce_on_create"))

        expected_hub_contexts = [
            {"context": "Test & Coverage"},
            {"context": "Validate Conventional Commit PR Title"},
        ]
        self.assertEqual(status_rule_hub["parameters"].get("required_status_checks"), expected_hub_contexts)

        # 2. Consumer repository scenario (Cooper-Wallace/my-repo)
        mock_run.reset_mock()
        mock_run.side_effect = [
            subprocess.CompletedProcess(args=[], returncode=0, stdout="[]", stderr=""),
            subprocess.CompletedProcess(args=[], returncode=0, stdout="{}", stderr=""),
        ]

        res_consumer = repokit.apply_branch_ruleset("Cooper-Wallace/my-repo")
        self.assertTrue(res_consumer)
        self.assertEqual(mock_run.call_count, 2)
        _, post_kwargs_consumer = mock_run.call_args_list[1]
        payload_consumer = json.loads(post_kwargs_consumer["input"])

        status_rule_consumer = next((r for r in payload_consumer["rules"] if r.get("type") == "required_status_checks"), None)
        self.assertIsNotNone(status_rule_consumer)
        expected_consumer_contexts = [
            {"context": "ci / Test & Coverage"},
            {"context": "validate-pr / Validate Conventional Commit PR Title"},
        ]
        self.assertEqual(status_rule_consumer["parameters"].get("required_status_checks"), expected_consumer_contexts)

        # 3. Explicit required_checks scenario
        mock_run.reset_mock()
        mock_run.side_effect = [
            subprocess.CompletedProcess(args=[], returncode=0, stdout="[]", stderr=""),
            subprocess.CompletedProcess(args=[], returncode=0, stdout="{}", stderr=""),
        ]

        custom_checks = ["Custom / Test", "Custom / Lint"]
        res_custom = repokit.apply_branch_ruleset("Cooper-Wallace/custom-repo", required_checks=custom_checks)
        self.assertTrue(res_custom)
        self.assertEqual(mock_run.call_count, 2)
        _, post_kwargs_custom = mock_run.call_args_list[1]
        payload_custom = json.loads(post_kwargs_custom["input"])

        status_rule_custom = next((r for r in payload_custom["rules"] if r.get("type") == "required_status_checks"), None)
        self.assertIsNotNone(status_rule_custom)
        expected_custom_contexts = [
            {"context": "Custom / Test"},
            {"context": "Custom / Lint"},
        ]
        self.assertEqual(status_rule_custom["parameters"].get("required_status_checks"), expected_custom_contexts)

    @patch.object(repokit, "get_repo_slug")
    @patch.object(repokit, "run_cmd")
    def test_audit_repository_remote_auto_merge_and_ruleset(self, mock_run_cmd, mock_slug):
        mock_slug.return_value = "Cooper-Wallace/repokit"

        # Case 1: All pass (auto_merge is False, ruleset has required status checks)
        def mock_cmd_side_effect(cmd, check=True):
            if cmd == ["gh", "api", "repos/Cooper-Wallace/repokit"]:
                return subprocess.CompletedProcess(
                    args=cmd,
                    returncode=0,
                    stdout=json.dumps({
                        "allow_squash_merge": True,
                        "allow_merge_commit": False,
                        "allow_rebase_merge": False,
                        "delete_branch_on_merge": True,
                        "squash_merge_commit_title": "PR_TITLE",
                        "squash_merge_commit_message": "PR_BODY",
                        "allow_auto_merge": False,
                    }),
                    stderr="",
                )
            elif cmd == ["gh", "api", "repos/Cooper-Wallace/repokit/rulesets"]:
                return subprocess.CompletedProcess(
                    args=cmd,
                    returncode=0,
                    stdout=json.dumps([
                        {
                            "name": "default-branch-protection",
                            "enforcement": "active",
                            "rules": [
                                {
                                    "type": "required_status_checks",
                                    "parameters": {
                                        "required_status_checks": [
                                            {"context": "Test & Coverage"},
                                            {"context": "Validate Conventional Commit PR Title"},
                                        ]
                                    },
                                }
                            ],
                        }
                    ]),
                    stderr="",
                )
            return subprocess.CompletedProcess(args=cmd, returncode=0, stdout="{}", stderr="")

        mock_run_cmd.side_effect = mock_cmd_side_effect
        self.assertTrue(repokit.audit_repository(REPO_ROOT, remote_only=True))

        # Case 2: Failure if allow_auto_merge is True
        def mock_cmd_auto_merge_true(cmd, check=True):
            if cmd == ["gh", "api", "repos/Cooper-Wallace/repokit"]:
                return subprocess.CompletedProcess(
                    args=cmd,
                    returncode=0,
                    stdout=json.dumps({
                        "allow_squash_merge": True,
                        "allow_merge_commit": False,
                        "allow_rebase_merge": False,
                        "delete_branch_on_merge": True,
                        "squash_merge_commit_title": "PR_TITLE",
                        "squash_merge_commit_message": "PR_BODY",
                        "allow_auto_merge": True,
                    }),
                    stderr="",
                )
            return mock_cmd_side_effect(cmd, check=check)

        mock_run_cmd.side_effect = mock_cmd_auto_merge_true
        self.assertFalse(repokit.audit_repository(REPO_ROOT, remote_only=True))

        # Case 3: Failure if ruleset is missing required status checks
        def mock_cmd_no_checks(cmd, check=True):
            if cmd == ["gh", "api", "repos/Cooper-Wallace/repokit/rulesets"]:
                return subprocess.CompletedProcess(
                    args=cmd,
                    returncode=0,
                    stdout=json.dumps([
                        {
                            "name": "default-branch-protection",
                            "enforcement": "active",
                            "rules": [],
                        }
                    ]),
                    stderr="",
                )
            return mock_cmd_side_effect(cmd, check=check)

        mock_run_cmd.side_effect = mock_cmd_no_checks
        self.assertFalse(repokit.audit_repository(REPO_ROOT, remote_only=True))

    def test_pr_template_contains_quality_gates_checklist(self):
        # Verify in-memory PR_TEMPLATE constant
        self.assertIn("## Quality Gates & Sign-Off Checklist", repokit.PR_TEMPLATE)
        self.assertIn("Gate 1: Automated Quality Gates", repokit.PR_TEMPLATE)
        self.assertIn("Gate 2: Agent Review Verification", repokit.PR_TEMPLATE)
        self.assertIn("Gate 3: Human Review & Manual Promotion", repokit.PR_TEMPLATE)
        self.assertIn("never automated", repokit.PR_TEMPLATE)

        # Verify synchronized .github/pull_request_template.md
        template_file = REPO_ROOT / ".github" / "pull_request_template.md"
        self.assertTrue(template_file.exists(), ".github/pull_request_template.md must exist")
        content = template_file.read_text(encoding="utf-8")
        self.assertIn("## Quality Gates & Sign-Off Checklist", content)
        self.assertIn("Gate 1: Automated Quality Gates", content)
        self.assertIn("Gate 2: Agent Review Verification", content)
        self.assertIn("Gate 3: Human Review & Manual Promotion", content)
        self.assertIn("never automated", content)

    def test_semantic_pr_workflow_push_guard(self):
        # Verify in-memory WORKFLOW_SEMANTIC_PR constant
        self.assertIn(
            "if: github.event_name == 'pull_request' || github.event_name == 'pull_request_target'",
            repokit.WORKFLOW_SEMANTIC_PR,
        )
        self.assertIn("workflow_call:", repokit.WORKFLOW_SEMANTIC_PR)

        # Verify .github/workflows/semantic-pr.yml
        workflow_file = REPO_ROOT / ".github" / "workflows" / "semantic-pr.yml"
        self.assertTrue(workflow_file.exists())
        content = workflow_file.read_text(encoding="utf-8")
        self.assertIn(
            "if: github.event_name == 'pull_request' || github.event_name == 'pull_request_target'",
            content,
        )
        self.assertIn("workflow_call:", content)

    def test_archetype_ci_workflow_has_semantic_pr_push_guard(self):
        for arch in ["bash", "python", "java", "typescript"]:
            ci_template = REPO_ROOT / "templates" / arch / ".github" / "workflows" / "ci.yml"
            self.assertTrue(ci_template.exists(), f"CI template for {arch} must exist")
            content = ci_template.read_text(encoding="utf-8")
            self.assertIn(
                "if: github.event_name == 'pull_request'",
                content,
                f"CI template for {arch} must guard validate-pr with if: github.event_name == 'pull_request'",
            )

    def test_patch_ci_workflow_pr_guard(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            ci_file = Path(tmpdir) / "ci.yml"
            initial_content = (
                "name: CI\n\n"
                "on:\n  pull_request:\n  push:\n\n"
                "jobs:\n"
                "  validate-pr:\n"
                "    uses: Cooper-Wallace/repokit/.github/workflows/semantic-pr.yml@main\n\n"
                "  test:\n"
                "    runs-on: ubuntu-latest\n"
                "    steps:\n"
                "      - run: pytest\n"
            )
            ci_file.write_text(initial_content, encoding="utf-8")

            # First run: should inject guard and return True
            res = repokit.patch_ci_workflow_pr_guard(ci_file)
            self.assertTrue(res)

            patched_content = ci_file.read_text(encoding="utf-8")
            self.assertIn("validate-pr:\n    if: github.event_name == 'pull_request'\n", patched_content)
            self.assertIn("  test:\n    runs-on: ubuntu-latest\n", patched_content)
            self.assertIn("uses: Cooper-Wallace/repokit/.github/workflows/semantic-pr.yml@main", patched_content)

            # Second run: already guarded, must be idempotent and return False
            res2 = repokit.patch_ci_workflow_pr_guard(ci_file)
            self.assertFalse(res2)
            self.assertEqual(ci_file.read_text(encoding="utf-8"), patched_content)

    def test_scaffold_archetype_ci_patches_existing_workflow(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            repo_path = Path(tmpdir)
            (repo_path / "pyproject.toml").write_text("[project]\nname='foo'\n")
            ci_dir = repo_path / ".github" / "workflows"
            ci_dir.mkdir(parents=True)
            ci_file = ci_dir / "ci.yml"
            custom_content = (
                "name: Custom CI\n"
                "jobs:\n"
                "  validate-pr:\n"
                "    uses: Cooper-Wallace/repokit/.github/workflows/semantic-pr.yml@main\n"
                "  custom-job:\n"
                "    runs-on: ubuntu-latest\n"
            )
            ci_file.write_text(custom_content, encoding="utf-8")

            # Without --force, it should patch validate-pr and preserve custom-job
            modified = repokit.scaffold_archetype_ci(repo_path, force=False)
            self.assertTrue(modified)
            content = ci_file.read_text(encoding="utf-8")
            self.assertIn("if: github.event_name == 'pull_request'", content)
            self.assertIn("custom-job:", content)

    def test_audit_ci_workflow_pr_guard(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            p = Path(tmpdir)
            subprocess.run(["git", "init", "-b", "main"], cwd=p, check=True, capture_output=True)
            subprocess.run([*BIN_CMD, "hook", "install", "--all", str(p)], check=True, capture_output=True)
            (p / ".github").mkdir(parents=True, exist_ok=True)
            (p / ".github" / "pull_request_template.md").write_text("## Summary\n")
            (p / ".github" / "workflows").mkdir(parents=True, exist_ok=True)
            (p / "src").mkdir(parents=True, exist_ok=True)

            ci_file = p / ".github" / "workflows" / "ci.yml"

            # Case 1: validate-pr without guard -> audit should report False
            ci_file.write_text(
                "name: CI\njobs:\n  validate-pr:\n    uses: Cooper-Wallace/repokit/.github/workflows/semantic-pr.yml@main\n"
                "  ci:\n    uses: Cooper-Wallace/repokit/.github/workflows/reusable-python.yml@main\n"
            )
            res_no_guard = subprocess.run([*BIN_CMD, "audit", str(p), "--local"], capture_output=True, text=True)
            self.assertNotEqual(res_no_guard.returncode, 0)
            self.assertIn("CI Workflow PR Event Guard (skips PR check on main push)", res_no_guard.stdout)
            self.assertIn("✘", res_no_guard.stdout)
            self.assertFalse(repokit.audit_repository(p, local_only=True))

            # Case 2: validate-pr with guard -> audit should report True
            ci_file.write_text(
                "name: CI\njobs:\n  validate-pr:\n    if: github.event_name == 'pull_request'\n"
                "    uses: Cooper-Wallace/repokit/.github/workflows/semantic-pr.yml@main\n"
                "  ci:\n    uses: Cooper-Wallace/repokit/.github/workflows/reusable-python.yml@main\n"
            )
            res_guarded = subprocess.run([*BIN_CMD, "audit", str(p), "--local"], capture_output=True, text=True)
            self.assertEqual(res_guarded.returncode, 0)
            self.assertIn("CI Workflow PR Event Guard (skips PR check on main push)", res_guarded.stdout)
            self.assertIn("All repository standards are strictly satisfied", res_guarded.stdout)
            self.assertTrue(repokit.audit_repository(p, local_only=True))


if __name__ == "__main__":
    unittest.main()
