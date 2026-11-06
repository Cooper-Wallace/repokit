package repokit

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetPlatformOS returns the current runtime operating system (runtime.GOOS).
func GetPlatformOS() string {
	return runtime.GOOS
}

// GetPlatformArch returns the current runtime architecture (runtime.GOARCH).
func GetPlatformArch() string {
	return runtime.GOARCH
}

// IsWindows reports whether the runtime operating system is Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsMacOS reports whether the runtime operating system is macOS (Darwin).
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux reports whether the runtime operating system is Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// buildShellCommandForOS constructs the shell binary name and argument slice
// for executing cmdStr on a specific target operating system.
func buildShellCommandForOS(goos, cmdStr string) (string, []string) {
	if goos == "windows" {
		return "cmd.exe", []string{"/c", cmdStr}
	}
	return "sh", []string{"-c", cmdStr}
}

// BuildShellCommand returns the shell binary and argument slice to execute cmdStr
// on the current platform.
//
// On Windows: returns "cmd.exe", []string{"/c", cmdStr}
// On Unix/macOS: returns "sh", []string{"-c", cmdStr}
func BuildShellCommand(cmdStr string) (string, []string) {
	return buildShellCommandForOS(runtime.GOOS, cmdStr)
}

// NormalizePath normalizes path separators to forward slashes and returns a
// cleaned, platform-canonical path.
func NormalizePath(path string) string {
	return filepath.Clean(filepath.ToSlash(path))
}

// ensureExecutableForOS enables 0111 execute bits on POSIX systems or returns nil on Windows.
func ensureExecutableForOS(goos, path string) error {
	if goos == "windows" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	currentMode := info.Mode()
	newMode := currentMode | 0111
	if currentMode != newMode {
		return os.Chmod(path, newMode)
	}
	return nil
}

// EnsureExecutable ensures execute permissions (0111) are enabled on the target file
// on Unix and macOS. On Windows, executable state is determined by file extension,
// so this operation is a no-op that returns nil.
func EnsureExecutable(path string) error {
	return ensureExecutableForOS(runtime.GOOS, path)
}
