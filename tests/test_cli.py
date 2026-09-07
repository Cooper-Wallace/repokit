"""
Tests for gh-standardise CLI tool.
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

# Import functions from bin/gh-standardise
import importlib.machinery
import importlib.util

loader = importlib.machinery.SourceFileLoader("gh_standardise", str(REPO_ROOT / "bin" / "gh-standardise"))
spec = importlib.util.spec_from_loader("gh_standardise", loader)
gh_standardise = importlib.util.module_from_spec(spec)
loader.exec_module(gh_standardise)

PATTERN = gh_standardise.COMMIT_PATTERN
BIN_CMD = [sys.executable, str(REPO_ROOT / "bin" / "gh-standardise")]


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
            res = gh_standardise.apply_archetype_template("python", target, variables)
            self.assertTrue(res)
            self.assertTrue((target / "pyproject.toml").exists())
            self.assertTrue((target / "src" / "sample_pkg" / "__init__.py").exists())
            self.assertTrue((target / "src" / "sample_pkg" / "core.py").exists())
            self.assertTrue((target / "tests" / "test_core.py").exists())

            # Verify variable substitution
            content = (target / "src" / "sample_pkg" / "core.py").read_text()
            self.assertIn("sample-pkg", content)
            self.assertNotIn("{{repo_name}}", content)

            # Audit check
            checks = gh_standardise.audit_archetype_structure(target, "python")
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
            res = gh_standardise.apply_archetype_template("java", target, variables)
            self.assertTrue(res)
            self.assertTrue((target / "pom.xml").exists())
            self.assertTrue((target / "src/main/java/com/cooper/sample_api/Application.java").exists())
            self.assertTrue((target / "src/main/java/com/cooper/sample_api/controller/HealthController.java").exists())
            self.assertTrue((target / "src/test/java/com/cooper/sample_api/ApplicationTests.java").exists())

            checks = gh_standardise.audit_archetype_structure(target, "java")
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
            res = gh_standardise.apply_archetype_template("typescript", target, variables)
            self.assertTrue(res)
            self.assertTrue((target / "package.json").exists())
            self.assertTrue((target / "tsconfig.json").exists())
            self.assertTrue((target / "vite.config.ts").exists())
            self.assertTrue((target / "src/main.ts").exists())
            self.assertTrue((target / "tests/api.test.ts").exists())

            checks = gh_standardise.audit_archetype_structure(target, "typescript")
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
            res = gh_standardise.apply_archetype_template("bash", target, variables)
            self.assertTrue(res)
            self.assertTrue((target / "bin/sample-cli").exists())
            self.assertTrue((target / "install.sh").exists())
            self.assertTrue(os.access(target / "bin/sample-cli", os.X_OK))

            checks = gh_standardise.audit_archetype_structure(target, "bash")
            self.assertTrue(all(passed for _, passed in checks))


class TestCLICommands(unittest.TestCase):
    def test_help_command(self):
        res = subprocess.run([*BIN_CMD, "--help"], capture_output=True, text=True)
        self.assertEqual(res.returncode, 0)
        self.assertIn("gh-standardise", res.stdout)
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


if __name__ == "__main__":
    unittest.main()
