package repokit

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestTarGzArchive(binaryName string, content []byte) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{
		Name: binaryName,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(content); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gzw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func createTestZipArchive(binaryName string, content []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	f, err := zw.Create(binaryName)
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(content); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestCheckForUpdate_UpdateAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		osName, archName := getTargetPlatform()
		assetName := fmt.Sprintf("repokit_1.3.0_%s_%s.tar.gz", osName, archName)
		resp := githubRelease{
			TagName: "v1.3.0",
			Assets: []githubAsset{
				{
					Name:               assetName,
					BrowserDownloadURL: "http://example.com/" + assetName,
				},
				{
					Name:               "checksums.txt",
					BrowserDownloadURL: "http://example.com/checksums.txt",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	info, err := CheckForUpdate("1.2.0", server.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.UpdateAvailable {
		t.Errorf("expected update to be available")
	}
	if info.CurrentVersion != "1.2.0" {
		t.Errorf("expected CurrentVersion '1.2.0', got '%s'", info.CurrentVersion)
	}
	if info.LatestVersion != "1.3.0" {
		t.Errorf("expected LatestVersion '1.3.0', got '%s'", info.LatestVersion)
	}
	if !strings.HasPrefix(info.DownloadURL, "http://example.com/repokit_1.3.0_") {
		t.Errorf("unexpected DownloadURL: %s", info.DownloadURL)
	}
	if info.ChecksumURL != "http://example.com/checksums.txt" {
		t.Errorf("unexpected ChecksumURL: %s", info.ChecksumURL)
	}
}

func TestCheckForUpdate_UpdateNotAvailable_SameVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := githubRelease{
			TagName: "v1.3.0",
			Assets:  []githubAsset{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	info, err := CheckForUpdate("1.3.0", server.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UpdateAvailable {
		t.Errorf("expected update not available for same version")
	}
}

func TestCheckForUpdate_UpdateNotAvailable_CurrentNewer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := githubRelease{
			TagName: "v1.3.0",
			Assets:  []githubAsset{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	info, err := CheckForUpdate("1.4.0", server.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UpdateAvailable {
		t.Errorf("expected update not available when current is newer")
	}
}

func TestCheckForUpdate_PlatformAssetResolution(t *testing.T) {
	resetPlatform := setTargetPlatform("windows", "amd64")
	defer resetPlatform()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := githubRelease{
			TagName: "v1.3.0",
			Assets: []githubAsset{
				{
					Name:               "repokit_1.3.0_windows_amd64.zip",
					BrowserDownloadURL: "http://example.com/repokit_1.3.0_windows_amd64.zip",
				},
				{
					Name:               "checksums.txt",
					BrowserDownloadURL: "http://example.com/checksums.txt",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	info, err := CheckForUpdate("1.0.0", server.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.AssetName != "repokit_1.3.0_windows_amd64.zip" {
		t.Errorf("expected asset name repokit_1.3.0_windows_amd64.zip, got %s", info.AssetName)
	}
	if info.DownloadURL != "http://example.com/repokit_1.3.0_windows_amd64.zip" {
		t.Errorf("expected matching download url, got %s", info.DownloadURL)
	}
}

func TestCheckForUpdate_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	_, err := CheckForUpdate("1.0.0", server.Client())
	if err == nil {
		t.Fatalf("expected error on HTTP 404, got nil")
	}
}

func TestCheckForUpdate_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	_, err := CheckForUpdate("1.0.0", server.Client())
	if err == nil {
		t.Fatalf("expected error on invalid JSON, got nil")
	}
}

func TestExecuteSelfUpdate_CheckOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := githubRelease{
			TagName: "v1.4.0",
			Assets:  []githubAsset{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	opts := UpdateOptions{
		CheckOnly:     true,
		TargetVersion: "1.3.0",
		HTTPClient:    server.Client(),
	}

	info, err := ExecuteSelfUpdate(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.UpdateAvailable {
		t.Errorf("expected update to be available")
	}
}

func TestExecuteSelfUpdate_NoUpdateAvailable_NoForce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := githubRelease{
			TagName: "v1.3.0",
			Assets:  []githubAsset{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	opts := UpdateOptions{
		TargetVersion: "1.3.0",
		Force:         false,
		HTTPClient:    server.Client(),
	}

	info, err := ExecuteSelfUpdate(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.UpdateAvailable {
		t.Errorf("expected no update available")
	}
}

func TestExecuteSelfUpdate_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	targetBinary := filepath.Join(tmpDir, "repokit")
	if err := os.WriteFile(targetBinary, []byte("original-binary"), 0755); err != nil {
		t.Fatalf("failed to create target binary: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		osName, archName := getTargetPlatform()
		assetName := fmt.Sprintf("repokit_1.4.0_%s_%s.tar.gz", osName, archName)
		resp := githubRelease{
			TagName: "v1.4.0",
			Assets: []githubAsset{
				{
					Name:               assetName,
					BrowserDownloadURL: "http://example.com/" + assetName,
				},
				{
					Name:               "checksums.txt",
					BrowserDownloadURL: "http://example.com/checksums.txt",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	opts := UpdateOptions{
		DryRun:        true,
		TargetVersion: "1.3.0",
		TargetPath:    targetBinary,
		HTTPClient:    server.Client(),
	}

	info, err := ExecuteSelfUpdate(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.UpdateAvailable {
		t.Errorf("expected update to be available")
	}

	// Verify target was not modified
	data, err := os.ReadFile(targetBinary)
	if err != nil {
		t.Fatalf("failed to read target: %v", err)
	}
	if string(data) != "original-binary" {
		t.Errorf("target was modified during dry-run")
	}
}

func TestExecuteSelfUpdate_SuccessfulUpdate_TarGz(t *testing.T) {
	resetPlatform := setTargetPlatform("linux", "amd64")
	defer resetPlatform()

	tmpDir := t.TempDir()
	targetBinary := filepath.Join(tmpDir, "repokit")
	if err := os.WriteFile(targetBinary, []byte("old-binary-content"), 0755); err != nil {
		t.Fatalf("failed to create target binary: %v", err)
	}

	newBinaryContent := []byte("#!/bin/sh\necho 'repokit v1.4.0'\n")
	archiveBytes, err := createTestTarGzArchive("repokit", newBinaryContent)
	if err != nil {
		t.Fatalf("failed to create tar.gz: %v", err)
	}

	sum := sha256.Sum256(archiveBytes)
	assetName := "repokit_1.4.0_linux_amd64.tar.gz"
	checksumsContent := fmt.Sprintf("%x  %s\n", sum, assetName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			resp := githubRelease{
				TagName: "v1.4.0",
				Assets: []githubAsset{
					{
						Name:               assetName,
						BrowserDownloadURL: "/download/" + assetName,
					},
					{
						Name:               "checksums.txt",
						BrowserDownloadURL: "/download/checksums.txt",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/download/" + assetName:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archiveBytes)
		case "/download/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksumsContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Update asset URLs to point to mock server
	cleanup := setGitHubLatestReleaseURL(server.URL + "/releases/latest")
	defer cleanup()

	// Intercept relative download URLs
	serverHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			resp := githubRelease{
				TagName: "v1.4.0",
				Assets: []githubAsset{
					{
						Name:               assetName,
						BrowserDownloadURL: server.URL + "/download/" + assetName,
					},
					{
						Name:               "checksums.txt",
						BrowserDownloadURL: server.URL + "/download/checksums.txt",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/download/" + assetName:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archiveBytes)
		case "/download/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksumsContent))
		}
	})
	server.Config.Handler = serverHandler

	opts := UpdateOptions{
		TargetVersion: "1.3.0",
		TargetPath:    targetBinary,
		HTTPClient:    server.Client(),
	}

	info, err := ExecuteSelfUpdate(opts)
	if err != nil {
		t.Fatalf("ExecuteSelfUpdate failed: %v", err)
	}

	if !info.UpdateAvailable {
		t.Errorf("expected update available to be true")
	}

	// Verify target executable was replaced
	data, err := os.ReadFile(targetBinary)
	if err != nil {
		t.Fatalf("failed to read updated target: %v", err)
	}
	if string(data) != string(newBinaryContent) {
		t.Errorf("expected updated content %q, got %q", string(newBinaryContent), string(data))
	}

	// Verify permissions are 0755
	fi, err := os.Stat(targetBinary)
	if err != nil {
		t.Fatalf("failed to stat target: %v", err)
	}
	if fi.Mode().Perm() != 0755 {
		t.Errorf("expected mode 0755, got %o", fi.Mode().Perm())
	}
}

func TestExecuteSelfUpdate_SuccessfulUpdate_Zip(t *testing.T) {
	resetPlatform := setTargetPlatform("windows", "amd64")
	defer resetPlatform()

	tmpDir := t.TempDir()
	targetBinary := filepath.Join(tmpDir, "repokit.exe")
	if err := os.WriteFile(targetBinary, []byte("old-exe-content"), 0755); err != nil {
		t.Fatalf("failed to create target binary: %v", err)
	}

	newBinaryContent := []byte("MZ-new-windows-binary")
	archiveBytes, err := createTestZipArchive("repokit.exe", newBinaryContent)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}

	sum := sha256.Sum256(archiveBytes)
	assetName := "repokit_1.4.0_windows_amd64.zip"
	checksumsContent := fmt.Sprintf("%x  %s\n", sum, assetName)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			resp := githubRelease{
				TagName: "v1.4.0",
				Assets: []githubAsset{
					{
						Name:               assetName,
						BrowserDownloadURL: server.URL + "/download/" + assetName,
					},
					{
						Name:               "checksums.txt",
						BrowserDownloadURL: server.URL + "/download/checksums.txt",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/download/" + assetName:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archiveBytes)
		case "/download/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksumsContent))
		}
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL + "/releases/latest")
	defer cleanup()

	opts := UpdateOptions{
		TargetVersion: "1.3.0",
		TargetPath:    targetBinary,
		HTTPClient:    server.Client(),
	}

	info, err := ExecuteSelfUpdate(opts)
	if err != nil {
		t.Fatalf("ExecuteSelfUpdate failed: %v", err)
	}

	if !info.UpdateAvailable {
		t.Errorf("expected update to be available")
	}

	data, err := os.ReadFile(targetBinary)
	if err != nil {
		t.Fatalf("failed to read updated target: %v", err)
	}
	if string(data) != string(newBinaryContent) {
		t.Errorf("expected updated content %q, got %q", string(newBinaryContent), string(data))
	}
}

func TestExecuteSelfUpdate_ForceUpdate(t *testing.T) {
	resetPlatform := setTargetPlatform("linux", "amd64")
	defer resetPlatform()

	tmpDir := t.TempDir()
	targetBinary := filepath.Join(tmpDir, "repokit")
	if err := os.WriteFile(targetBinary, []byte("current-binary"), 0755); err != nil {
		t.Fatalf("failed to create target binary: %v", err)
	}

	newBinaryContent := []byte("forced-reinstalled-binary")
	archiveBytes, err := createTestTarGzArchive("repokit", newBinaryContent)
	if err != nil {
		t.Fatalf("failed to create tar.gz: %v", err)
	}

	sum := sha256.Sum256(archiveBytes)
	assetName := "repokit_1.3.0_linux_amd64.tar.gz"
	checksumsContent := fmt.Sprintf("%x  %s\n", sum, assetName)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			resp := githubRelease{
				TagName: "v1.3.0",
				Assets: []githubAsset{
					{
						Name:               assetName,
						BrowserDownloadURL: server.URL + "/download/" + assetName,
					},
					{
						Name:               "checksums.txt",
						BrowserDownloadURL: server.URL + "/download/checksums.txt",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/download/" + assetName:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archiveBytes)
		case "/download/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksumsContent))
		}
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL + "/releases/latest")
	defer cleanup()

	opts := UpdateOptions{
		TargetVersion: "1.3.0", // Same version!
		Force:         true,
		TargetPath:    targetBinary,
		HTTPClient:    server.Client(),
	}

	info, err := ExecuteSelfUpdate(opts)
	if err != nil {
		t.Fatalf("forced ExecuteSelfUpdate failed: %v", err)
	}
	if info.UpdateAvailable {
		t.Errorf("expected UpdateAvailable to be false for identical versions")
	}

	data, err := os.ReadFile(targetBinary)
	if err != nil {
		t.Fatalf("failed to read binary: %v", err)
	}
	if string(data) != string(newBinaryContent) {
		t.Errorf("expected forced update content %q, got %q", string(newBinaryContent), string(data))
	}
}

func TestExecuteSelfUpdate_ChecksumMismatchRejection(t *testing.T) {
	resetPlatform := setTargetPlatform("linux", "amd64")
	defer resetPlatform()

	tmpDir := t.TempDir()
	targetBinary := filepath.Join(tmpDir, "repokit")
	origContent := []byte("original-unmodified-binary")
	if err := os.WriteFile(targetBinary, origContent, 0755); err != nil {
		t.Fatalf("failed to create target binary: %v", err)
	}

	archiveBytes, err := createTestTarGzArchive("repokit", []byte("tampered-content"))
	if err != nil {
		t.Fatalf("failed to create tar.gz: %v", err)
	}

	assetName := "repokit_1.4.0_linux_amd64.tar.gz"
	badChecksumsContent := fmt.Sprintf("0000000000000000000000000000000000000000000000000000000000000000  %s\n", assetName)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			resp := githubRelease{
				TagName: "v1.4.0",
				Assets: []githubAsset{
					{
						Name:               assetName,
						BrowserDownloadURL: server.URL + "/download/" + assetName,
					},
					{
						Name:               "checksums.txt",
						BrowserDownloadURL: server.URL + "/download/checksums.txt",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/download/" + assetName:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archiveBytes)
		case "/download/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(badChecksumsContent))
		}
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL + "/releases/latest")
	defer cleanup()

	opts := UpdateOptions{
		TargetVersion: "1.3.0",
		TargetPath:    targetBinary,
		HTTPClient:    server.Client(),
	}

	_, err = ExecuteSelfUpdate(opts)
	if err == nil {
		t.Fatalf("expected error due to checksum mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected 'checksum mismatch' in error, got %v", err)
	}

	// Verify target was unmodified
	data, _ := os.ReadFile(targetBinary)
	if string(data) != string(origContent) {
		t.Errorf("target file was altered despite checksum mismatch failure")
	}
}

func TestExecuteSelfUpdate_CorruptedAssetRejection(t *testing.T) {
	resetPlatform := setTargetPlatform("linux", "amd64")
	defer resetPlatform()

	tmpDir := t.TempDir()
	targetBinary := filepath.Join(tmpDir, "repokit")
	origContent := []byte("original-unmodified-binary")
	if err := os.WriteFile(targetBinary, origContent, 0755); err != nil {
		t.Fatalf("failed to create target binary: %v", err)
	}

	corruptedArchiveBytes := []byte("this is not a valid gzip or tar archive")
	sum := sha256.Sum256(corruptedArchiveBytes)
	assetName := "repokit_1.4.0_linux_amd64.tar.gz"
	checksumsContent := fmt.Sprintf("%x  %s\n", sum, assetName)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			resp := githubRelease{
				TagName: "v1.4.0",
				Assets: []githubAsset{
					{
						Name:               assetName,
						BrowserDownloadURL: server.URL + "/download/" + assetName,
					},
					{
						Name:               "checksums.txt",
						BrowserDownloadURL: server.URL + "/download/checksums.txt",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/download/" + assetName:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(corruptedArchiveBytes)
		case "/download/checksums.txt":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(checksumsContent))
		}
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL + "/releases/latest")
	defer cleanup()

	opts := UpdateOptions{
		TargetVersion: "1.3.0",
		TargetPath:    targetBinary,
		HTTPClient:    server.Client(),
	}

	_, err := ExecuteSelfUpdate(opts)
	if err == nil {
		t.Fatalf("expected error due to corrupted archive, got nil")
	}

	// Verify target was unmodified
	data, _ := os.ReadFile(targetBinary)
	if string(data) != string(origContent) {
		t.Errorf("target file was altered despite corrupted archive failure")
	}
}

func TestExecuteSelfUpdate_MissingAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := githubRelease{
			TagName: "v1.4.0",
			Assets: []githubAsset{
				{
					Name:               "checksums.txt",
					BrowserDownloadURL: "http://example.com/checksums.txt",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	opts := UpdateOptions{
		TargetVersion: "1.3.0",
		HTTPClient:    server.Client(),
	}

	_, err := ExecuteSelfUpdate(opts)
	if err == nil {
		t.Fatalf("expected error when asset is missing, got nil")
	}
}

func TestExecuteSelfUpdate_MissingChecksums(t *testing.T) {
	resetPlatform := setTargetPlatform("linux", "amd64")
	defer resetPlatform()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := githubRelease{
			TagName: "v1.4.0",
			Assets: []githubAsset{
				{
					Name:               "repokit_1.4.0_linux_amd64.tar.gz",
					BrowserDownloadURL: "http://example.com/asset.tar.gz",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cleanup := setGitHubLatestReleaseURL(server.URL)
	defer cleanup()

	opts := UpdateOptions{
		TargetVersion: "1.3.0",
		HTTPClient:    server.Client(),
	}

	_, err := ExecuteSelfUpdate(opts)
	if err == nil {
		t.Fatalf("expected error when checksums.txt is missing, got nil")
	}
}
