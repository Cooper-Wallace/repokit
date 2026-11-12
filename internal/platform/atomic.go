package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to filePath atomically by first staging the write in
// a temporary file in the same directory, setting file permissions, syncing buffers
// to disk, and renaming over the target path. If any error occurs prior to renaming,
// the temporary staging file is removed.
func WriteFileAtomic(filePath string, data []byte, perm os.FileMode) error {
	if filePath == "" {
		return errors.New("filePath cannot be empty")
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	tempFile, err := os.CreateTemp(dir, ".repokit-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in %q: %w", dir, err)
	}

	tempName := tempFile.Name()
	var success bool
	var closed bool

	defer func() {
		if !closed {
			_ = tempFile.Close()
		}
		if !success {
			_ = os.Remove(tempName)
		}
	}()

	if err := tempFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set permissions on %q: %w", tempName, err)
	}

	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write data to %q: %w", tempName, err)
	}

	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file %q: %w", tempName, err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file %q: %w", tempName, err)
	}
	closed = true

	if err := os.Rename(tempName, filePath); err != nil {
		return fmt.Errorf("failed to rename %q to %q: %w", tempName, filePath, err)
	}

	success = true
	return nil
}

// BackupAndReplace creates a backup of filePath at filePath+".bak" with the original
// file permissions if filePath exists, and then atomically replaces filePath with data.
func BackupAndReplace(filePath string, data []byte, perm os.FileMode) error {
	if filePath == "" {
		return errors.New("filePath cannot be empty")
	}

	info, err := os.Stat(filePath)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("cannot backup directory %q: target must be a regular file", filePath)
		}
		existingData, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read existing file for backup %q: %w", filePath, err)
		}
		backupPath := filePath + ".bak"
		if err := WriteFileAtomic(backupPath, existingData, info.Mode().Perm()); err != nil {
			return fmt.Errorf("failed to create backup file %q: %w", backupPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat file %q: %w", filePath, err)
	}

	return WriteFileAtomic(filePath, data, perm)
}
