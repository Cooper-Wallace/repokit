package repokit

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestWriteFileAtomic_Basic(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sample.txt")
	data := []byte("hello atomic world")
	perm := os.FileMode(0644)

	if err := WriteFileAtomic(filePath, data, perm); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	readBack, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read back written file: %v", err)
	}

	if !bytes.Equal(readBack, data) {
		t.Fatalf("expected data %q, got %q", string(data), string(readBack))
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat written file: %v", err)
	}

	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != perm {
			t.Fatalf("expected permissions %o, got %o", perm, info.Mode().Perm())
		}
	}
}

func TestWriteFileAtomic_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping POSIX permission test on Windows")
	}

	tests := []struct {
		name string
		perm os.FileMode
	}{
		{"read-write owner only", 0600},
		{"read-only owner", 0400},
		{"executable owner and group", 0750},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			filePath := filepath.Join(tempDir, "perm-test.txt")
			data := []byte("permission test")

			if err := WriteFileAtomic(filePath, data, tc.perm); err != nil {
				t.Fatalf("WriteFileAtomic failed: %v", err)
			}

			info, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("stat failed: %v", err)
			}

			if info.Mode().Perm() != tc.perm {
				t.Fatalf("expected perm %o, got %o", tc.perm, info.Mode().Perm())
			}
		})
	}
}

func TestWriteFileAtomic_AutoCreateParentDirs(t *testing.T) {
	tempDir := t.TempDir()
	nestedPath := filepath.Join(tempDir, "a", "b", "c", "deep-file.txt")
	data := []byte("nested payload")

	if err := WriteFileAtomic(nestedPath, data, 0644); err != nil {
		t.Fatalf("WriteFileAtomic failed for nested path: %v", err)
	}

	readBack, err := os.ReadFile(nestedPath)
	if err != nil {
		t.Fatalf("failed to read nested file: %v", err)
	}

	if !bytes.Equal(readBack, data) {
		t.Fatalf("expected %q, got %q", string(data), string(readBack))
	}
}

func TestWriteFileAtomic_EmptyPath(t *testing.T) {
	err := WriteFileAtomic("", []byte("empty test"), 0644)
	if err == nil {
		t.Fatal("expected error for empty filePath, got nil")
	}
}

func TestWriteFileAtomic_CleanupOnFailure(t *testing.T) {
	tempDir := t.TempDir()

	// Case 1: Target path is an existing directory. Rename should fail, and temp file cleaned up.
	existingSubDir := filepath.Join(tempDir, "existing-dir")
	if err := os.Mkdir(existingSubDir, 0755); err != nil {
		t.Fatalf("failed to create existing directory: %v", err)
	}

	err := WriteFileAtomic(existingSubDir, []byte("should fail rename"), 0644)
	if err == nil {
		t.Fatal("expected WriteFileAtomic to fail when destination is an existing directory")
	}

	// Verify no temporary files remain in tempDir
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".repokit-tmp-") {
			t.Fatalf("temporary staging file leaked: %s", entry.Name())
		}
	}

	// Case 2: Read-only directory where temp file cannot be created
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		roDir := filepath.Join(tempDir, "ro-dir")
		if err := os.Mkdir(roDir, 0500); err != nil {
			t.Fatalf("failed to create roDir: %v", err)
		}
		defer func() {
			_ = os.Chmod(roDir, 0755)
		}()

		roTarget := filepath.Join(roDir, "file.txt")
		err = WriteFileAtomic(roTarget, []byte("cannot write"), 0644)
		if err == nil {
			t.Fatal("expected error writing into read-only directory")
		}
	}
}

func TestBackupAndReplace(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "config.json")
	initialData := []byte(`{"version": 1}`)
	updatedData := []byte(`{"version": 2}`)

	// 1. Initial write when file does not exist yet
	if err := BackupAndReplace(filePath, initialData, 0644); err != nil {
		t.Fatalf("initial BackupAndReplace failed: %v", err)
	}

	// Backup should not exist on first write
	bakPath := filePath + ".bak"
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		t.Fatalf("expected .bak not to exist on initial write, err: %v", err)
	}

	// 2. Second write when file already exists
	if err := BackupAndReplace(filePath, updatedData, 0600); err != nil {
		t.Fatalf("second BackupAndReplace failed: %v", err)
	}

	// Verify updated file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}
	if !bytes.Equal(content, updatedData) {
		t.Fatalf("expected %q, got %q", string(updatedData), string(content))
	}

	// Verify backup file content
	bakContent, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}
	if !bytes.Equal(bakContent, initialData) {
		t.Fatalf("expected backup to have %q, got %q", string(initialData), string(bakContent))
	}

	if runtime.GOOS != "windows" {
		// Verify backup preserved initial permissions (0644)
		bakInfo, err := os.Stat(bakPath)
		if err != nil {
			t.Fatalf("stat bak failed: %v", err)
		}
		if bakInfo.Mode().Perm() != 0644 {
			t.Fatalf("expected bak perm 0644, got %o", bakInfo.Mode().Perm())
		}

		// Verify target has updated permissions (0600)
		targetInfo, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("stat target failed: %v", err)
		}
		if targetInfo.Mode().Perm() != 0600 {
			t.Fatalf("expected target perm 0600, got %o", targetInfo.Mode().Perm())
		}
	}
}

func TestBackupAndReplace_Errors(t *testing.T) {
	// Empty path
	if err := BackupAndReplace("", []byte("test"), 0644); err == nil {
		t.Fatal("expected error for empty filePath")
	}

	// Directory target
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to make dir: %v", err)
	}
	if err := BackupAndReplace(subDir, []byte("data"), 0644); err == nil {
		t.Fatal("expected error when target is a directory")
	}
}

func TestWriteFileAtomic_ConcurrentDifferentFiles(t *testing.T) {
	tempDir := t.TempDir()
	const workers = 40
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			fPath := filepath.Join(tempDir, fmt.Sprintf("worker-%d.txt", id))
			payload := []byte(fmt.Sprintf("worker payload %d", id))
			if err := WriteFileAtomic(fPath, payload, 0644); err != nil {
				t.Errorf("worker %d failed: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	for i := 0; i < workers; i++ {
		fPath := filepath.Join(tempDir, fmt.Sprintf("worker-%d.txt", i))
		expected := []byte(fmt.Sprintf("worker payload %d", i))
		data, err := os.ReadFile(fPath)
		if err != nil {
			t.Fatalf("failed to read worker %d file: %v", i, err)
		}
		if !bytes.Equal(data, expected) {
			t.Fatalf("worker %d data mismatch: expected %q, got %q", i, string(expected), string(data))
		}
	}
}

func TestWriteFileAtomic_ConcurrentSameFile(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "shared-target.txt")
	const workers = 40
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			payload := []byte(fmt.Sprintf("atomic-write-payload-%03d\n", id))
			_ = WriteFileAtomic(targetPath, payload, 0644)
		}(i)
	}

	wg.Wait()

	// The file should exist and have a valid full payload (no torn writes)
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read shared target file: %v", err)
	}

	matched := false
	for i := 0; i < workers; i++ {
		expected := fmt.Sprintf("atomic-write-payload-%03d\n", i)
		if string(content) == expected {
			matched = true
			break
		}
	}

	if !matched {
		t.Fatalf("file content %q does not match any single atomic write", string(content))
	}

	// Verify no temporary files were left behind
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read dir failed: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".repokit-tmp-") {
			t.Fatalf("lingering temp file detected: %s", entry.Name())
		}
	}
}

