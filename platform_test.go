package repokit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlatformIntrospection(t *testing.T) {
	osName := GetPlatformOS()
	if osName != runtime.GOOS {
		t.Fatalf("expected GetPlatformOS() %q, got %q", runtime.GOOS, osName)
	}

	archName := GetPlatformArch()
	if archName != runtime.GOARCH {
		t.Fatalf("expected GetPlatformArch() %q, got %q", runtime.GOARCH, archName)
	}

	isWin := IsWindows()
	if isWin != (runtime.GOOS == "windows") {
		t.Fatalf("IsWindows() mismatch: expected %v, got %v", runtime.GOOS == "windows", isWin)
	}

	isMac := IsMacOS()
	if isMac != (runtime.GOOS == "darwin") {
		t.Fatalf("IsMacOS() mismatch: expected %v, got %v", runtime.GOOS == "darwin", isMac)
	}

	isLin := IsLinux()
	if isLin != (runtime.GOOS == "linux") {
		t.Fatalf("IsLinux() mismatch: expected %v, got %v", runtime.GOOS == "linux", isLin)
	}
}

func TestBuildShellCommand_CrossPlatformUnits(t *testing.T) {
	// 1. Windows simulation
	winBin, winArgs := buildShellCommandForOS("windows", "echo hello")
	if winBin != "cmd.exe" {
		t.Errorf("expected winBin 'cmd.exe', got %q", winBin)
	}
	if len(winArgs) != 2 || winArgs[0] != "/c" || winArgs[1] != "echo hello" {
		t.Errorf("expected winArgs ['/c', 'echo hello'], got %v", winArgs)
	}

	// 2. Darwin (macOS) simulation
	macBin, macArgs := buildShellCommandForOS("darwin", "echo hello")
	if macBin != "sh" {
		t.Errorf("expected macBin 'sh', got %q", macBin)
	}
	if len(macArgs) != 2 || macArgs[0] != "-c" || macArgs[1] != "echo hello" {
		t.Errorf("expected macArgs ['-c', 'echo hello'], got %v", macArgs)
	}

	// 3. Linux simulation
	linBin, linArgs := buildShellCommandForOS("linux", "echo hello")
	if linBin != "sh" {
		t.Errorf("expected linBin 'sh', got %q", linBin)
	}
	if len(linArgs) != 2 || linArgs[0] != "-c" || linArgs[1] != "echo hello" {
		t.Errorf("expected linArgs ['-c', 'echo hello'], got %v", linArgs)
	}
}

func TestBuildShellCommand_Execution(t *testing.T) {
	cmdStr := "echo repokit_exec_test_token"
	bin, args := BuildShellCommand(cmdStr)
	if bin == "" {
		t.Fatal("BuildShellCommand returned empty binary")
	}

	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to execute shell command %s %v: %v", bin, args, err)
	}

	if !strings.Contains(string(out), "repokit_exec_test_token") {
		t.Fatalf("expected output to contain token, got: %s", string(out))
	}
}

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", "."},
		{".", "."},
		{"./foo/bar/../baz", "foo/baz"},
		{"foo//bar///qux", "foo/bar/qux"},
		{"foo/./bar", "foo/bar"},
		{"foo/bar/baz/..", "foo/bar"},
	}

	for _, tc := range cases {
		got := NormalizePath(tc.input)
		// Compare against expected with OS-clean path
		expectedClean := filepath.Clean(filepath.ToSlash(tc.input))
		if got != expectedClean {
			t.Errorf("NormalizePath(%q) = %q, expected %q", tc.input, got, expectedClean)
		}
	}
}

func TestEnsureExecutable(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "script.sh")

	// Create file with 0644 permissions (non-executable)
	if err := os.WriteFile(filePath, []byte("#!/bin/sh\necho test\n"), 0644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	// Windows simulation should always return nil
	if err := ensureExecutableForOS("windows", filePath); err != nil {
		t.Fatalf("ensureExecutableForOS('windows') returned error: %v", err)
	}
	if err := ensureExecutableForOS("windows", filepath.Join(tempDir, "nonexistent")); err != nil {
		t.Fatalf("ensureExecutableForOS('windows') on nonexistent returned error: %v", err)
	}

	// Linux/POSIX test
	if runtime.GOOS != "windows" {
		infoBefore, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("stat before failed: %v", err)
		}
		if infoBefore.Mode().Perm()&0111 != 0 {
			t.Fatalf("expected file to not have execute bits before EnsureExecutable: %o", infoBefore.Mode().Perm())
		}

		// Ensure executable
		if err := EnsureExecutable(filePath); err != nil {
			t.Fatalf("EnsureExecutable failed: %v", err)
		}

		infoAfter, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("stat after failed: %v", err)
		}
		if infoAfter.Mode().Perm()&0111 == 0 {
			t.Fatalf("expected execute bits to be set, got mode: %o", infoAfter.Mode().Perm())
		}

		// Idempotency check
		if err := EnsureExecutable(filePath); err != nil {
			t.Fatalf("subsequent EnsureExecutable failed: %v", err)
		}
	}

	// Error case: non-existent file on Unix
	if runtime.GOOS != "windows" {
		nonExistent := filepath.Join(tempDir, "does-not-exist")
		if err := EnsureExecutable(nonExistent); err == nil {
			t.Fatal("expected error on nonexistent file for EnsureExecutable, got nil")
		}
	}
}
