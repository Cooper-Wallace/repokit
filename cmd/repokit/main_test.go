package main

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Cooper-Wallace/repokit"
)

func TestCLICommands(t *testing.T) {
	// help
	if code := run([]string{"help"}); code != 0 {
		t.Errorf("Expected 0 for help, got %d", code)
	}
	if code := run([]string{"-h"}); code != 0 {
		t.Errorf("Expected 0 for -h, got %d", code)
	}
	if code := run([]string{"--help"}); code != 0 {
		t.Errorf("Expected 0 for --help, got %d", code)
	}
	if code := run([]string{}); code != 1 {
		t.Errorf("Expected 1 for empty args, got %d", code)
	}

	// version
	if code := run([]string{"version"}); code != 0 {
		t.Errorf("Expected 0 for version, got %d", code)
	}
	if code := run([]string{"-v"}); code != 0 {
		t.Errorf("Expected 0 for -v, got %d", code)
	}
	if code := run([]string{"--version"}); code != 0 {
		t.Errorf("Expected 0 for --version, got %d", code)
	}

	// templates
	if code := run([]string{"templates"}); code != 0 {
		t.Errorf("Expected 0 for templates, got %d", code)
	}

	// unknown command
	if code := run([]string{"unknown-xyz"}); code != 1 {
		t.Errorf("Expected 1 for unknown command, got %d", code)
	}

	// hook errors
	if code := run([]string{"hook"}); code != 1 {
		t.Errorf("Expected 1 for hook without install, got %d", code)
	}
	if code := run([]string{"hook", "not-install"}); code != 1 {
		t.Errorf("Expected 1 for hook not-install, got %d", code)
	}

	// create without name
	if code := run([]string{"create"}); code != 1 {
		t.Errorf("Expected 1 for create without name, got %d", code)
	}
}

func TestCLIGitOperations(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// hook install
	if code := run([]string{"hook", "install", tmpDir}); code != 0 {
		t.Errorf("Expected 0 for hook install, got %d", code)
	}
	if code := run([]string{"hook", "install", "--all", tmpDir}); code != 0 {
		t.Errorf("Expected 0 for hook install --all, got %d", code)
	}

	// audit non-compliant repo
	if code := run([]string{"audit", "--local", tmpDir}); code != 1 {
		t.Errorf("Expected 1 for audit on non-compliant repo, got %d", code)
	}

	// audit not in git repo
	nonGit := filepath.Join(tmpDir, "does-not-exist")
	if code := run([]string{"audit", "--local", nonGit}); code != 1 {
		t.Errorf("Expected 1 for audit on non-git dir, got %d", code)
	}
	if code := run([]string{"hook", "install", nonGit}); code != 1 {
		t.Errorf("Expected 1 for hook install on non-git dir, got %d", code)
	}

	// configure not in git repo
	if code := run([]string{"configure", nonGit}); code != 1 {
		t.Errorf("Expected 1 for configure on non-git dir, got %d", code)
	}

	// create with mock
	oldRunner := repokit.CommandRunner
	defer func() { repokit.CommandRunner = oldRunner }()
	repokit.CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
		if len(args) > 1 && args[1] == "api" && args[2] == "user" {
			return "testuser\n", "", 0, nil
		}
		if len(args) > 1 && args[0] == "git" && args[1] == "push" {
			return "", "", 0, nil
		}
		return "{}", "", 0, nil
	}

	targetNew := filepath.Join(tmpDir, "new-cli-repo")
	if code := run([]string{"create", "new-cli-repo", "--description=Test desc", "--type=go", "--public", "--path=" + targetNew}); code != 0 {
		t.Errorf("Expected 0 for create, got %d", code)
	}

	targetNew2 := filepath.Join(tmpDir, "new-cli-repo-2")
	if code := run([]string{"create", "new-cli-repo-2", "-d", "Test desc 2", "-t", "bash", "--path", targetNew2}); code != 0 {
		t.Errorf("Expected 0 for create, got %d", code)
	}

	// configure with force
	_ = run([]string{"configure", "-f", tmpDir})

	// audit with type and flags
	_ = run([]string{"audit", "-t=go", "--local", tmpDir})
	_ = run([]string{"audit", "-t", "go", "--remote", tmpDir})
}

func TestConfigureDiscoveryCLI(t *testing.T) {
	oldRunner := repokit.CommandRunner
	defer func() { repokit.CommandRunner = oldRunner }()

	repokit.CommandRunner = func(args []string, dir string, input string) (string, string, int, error) {
		if len(args) > 2 && args[1] == "api" && args[2] == "user" {
			return "testuser\n", "", 0, nil
		}
		if len(args) > 2 && args[0] == "gh" && args[1] == "repo" && args[2] == "list" {
			return `[{"nameWithOwner": "testuser/repo1"}]`, "", 0, nil
		}
		return "{}", "", 0, nil
	}

	tmpDir := t.TempDir()

	// Error when propagate is invoked (removed command)
	if code := run([]string{"propagate"}); code != 1 {
		t.Errorf("Expected 1 for removed propagate command, got %d", code)
	}

	// Flag combinations with configure
	if code := run([]string{"configure", "--dry-run", "--repos", "r1,r2"}); code != 0 {
		t.Errorf("Expected 0 for configure --dry-run --repos r1,r2, got %d", code)
	}

	if code := run([]string{"configure", "--dry-run", "--all"}); code != 0 {
		t.Errorf("Expected 0 for configure --dry-run --all, got %d", code)
	}

	if code := run([]string{"configure", "--dry-run", "--local", "--path", tmpDir}); code != 0 {
		t.Errorf("Expected 0 for configure --dry-run --local --path <dir>, got %d", code)
	}

	if code := run([]string{"configure", "--repos=r1,r2", "--path=" + tmpDir, "-f", "--pr", "--dry-run"}); code != 0 {
		t.Errorf("Expected 0 for configure with inline flags, got %d", code)
	}
}
