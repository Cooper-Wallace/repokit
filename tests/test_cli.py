"""
Tests for gh-standardise CLI tool.
"""

import re
import subprocess
import tempfile
import unittest
from pathlib import Path

# Pattern used by commit-msg hook
PATTERN = r"^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-zA-Z0-9_.-]+\))?!?: .+$"


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
            # Initialize git repo
            subprocess.run(["git", "init", "-b", "main"], cwd=p, check=True, capture_output=True)
            subprocess.run(["gh-standardise", "hook", "install"], cwd=p, check=True, capture_output=True)

            # Check hook exists and is executable
            hook = p / ".git" / "hooks" / "commit-msg"
            self.assertTrue(hook.exists())

            # Configure git author for tests
            subprocess.run(["git", "config", "user.name", "Test User"], cwd=p, check=True)
            subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=p, check=True)

            # Test invalid commit rejection
            (p / "file.txt").write_text("hello")
            subprocess.run(["git", "add", "."], cwd=p, check=True)
            res_bad = subprocess.run(["git", "commit", "-m", "bad commit"], cwd=p, capture_output=True, text=True)
            self.assertNotEqual(res_bad.returncode, 0, "Bad commit should be rejected by hook")

            # Test valid commit acceptance
            res_good = subprocess.run(["git", "commit", "-m", "feat(core): initial test commit"], cwd=p, capture_output=True, text=True)
            self.assertEqual(res_good.returncode, 0, f"Valid commit should succeed: {res_good.stderr}")


class TestCLIHelp(unittest.TestCase):
    def test_help_command(self):
        res = subprocess.run(["gh-standardise", "--help"], capture_output=True, text=True)
        self.assertEqual(res.returncode, 0)
        self.assertIn("gh-standardise", res.stdout)
        self.assertIn("create", res.stdout)
        self.assertIn("configure", res.stdout)
        self.assertIn("audit", res.stdout)


if __name__ == "__main__":
    unittest.main()
