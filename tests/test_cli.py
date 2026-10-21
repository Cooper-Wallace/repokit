"""
Tests for repokit CLI tool.
"""

import os
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

# Add bin directory to sys.path so we can import helper functions directly
REPO_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(REPO_ROOT / "bin"))

# Import functions from bin/repokit
import importlib.machinery
import importlib.util

loader = importlib.machinery.SourceFileLoader("repokit", str(REPO_ROOT / "bin" / "repokit"))
spec = importlib.util.spec_from_loader("repokit", loader)
repokit = importlib.util.module_from_spec(spec)
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


if __name__ == "__main__":
    unittest.main()
