package repokit

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Version is the current version of repokit.
const Version = "1.3.0"

var (
	updateMu               sync.RWMutex
	githubLatestReleaseURL = "https://api.github.com/repos/Cooper-Wallace/repokit/releases/latest"
	targetOS               = runtime.GOOS
	targetArch             = runtime.GOARCH
	executablePathFunc     = os.Executable
)

func getGitHubLatestReleaseURL() string {
	updateMu.RLock()
	defer updateMu.RUnlock()
	return githubLatestReleaseURL
}

func setGitHubLatestReleaseURL(u string) func() {
	updateMu.Lock()
	old := githubLatestReleaseURL
	githubLatestReleaseURL = u
	updateMu.Unlock()
	return func() {
		updateMu.Lock()
		githubLatestReleaseURL = old
		updateMu.Unlock()
	}
}

func getTargetPlatform() (string, string) {
	updateMu.RLock()
	defer updateMu.RUnlock()
	return targetOS, targetArch
}

func setTargetPlatform(osName, archName string) func() {
	updateMu.Lock()
	oldOS := targetOS
	oldArch := targetArch
	targetOS = osName
	targetArch = archName
	updateMu.Unlock()
	return func() {
		updateMu.Lock()
		targetOS = oldOS
		targetArch = oldArch
		updateMu.Unlock()
	}
}

// UpdateOptions specifies parameters for checking and applying self-updates.
type UpdateOptions struct {
	CheckOnly     bool
	Force         bool
	DryRun        bool
	TargetVersion string
	HTTPClient    *http.Client
	TargetPath    string
}

// UpdateInfo holds details about current, latest release versions, and download targets.
type UpdateInfo struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
	DownloadURL     string
	ChecksumURL     string
	AssetName       string
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckForUpdate queries the GitHub Releases API for the latest repokit release and compares
// it against currentVersion using ParseSemVer.
func CheckForUpdate(currentVersion string, client *http.Client) (*UpdateInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequest(http.MethodGet, getGitHubLatestReleaseURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create update request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "repokit-self-update")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query latest release: HTTP %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to decode release response: %w", err)
	}

	rawLatest := strings.TrimSpace(rel.TagName)
	cleanLatest := strings.TrimPrefix(rawLatest, "v")
	cleanCurrent := strings.TrimPrefix(strings.TrimSpace(currentVersion), "v")

	curMaj, curMin, curPatch, curOk := ParseSemVer(cleanCurrent)
	latMaj, latMin, latPatch, latOk := ParseSemVer(cleanLatest)

	updateAvailable := false
	if curOk && latOk {
		if latMaj > curMaj || (latMaj == curMaj && latMin > curMin) || (latMaj == curMaj && latMin == curMin && latPatch > curPatch) {
			updateAvailable = true
		}
	} else if cleanLatest != "" && cleanCurrent != cleanLatest {
		updateAvailable = true
	}

	osName, archName := getTargetPlatform()
	ext := ".tar.gz"
	if osName == "windows" {
		ext = ".zip"
	}
	expectedAssetName := fmt.Sprintf("repokit_%s_%s_%s%s", cleanLatest, osName, archName, ext)

	info := &UpdateInfo{
		CurrentVersion:  cleanCurrent,
		LatestVersion:   cleanLatest,
		UpdateAvailable: updateAvailable,
		AssetName:       expectedAssetName,
	}

	for _, a := range rel.Assets {
		if a.Name == expectedAssetName {
			info.DownloadURL = a.BrowserDownloadURL
		}
		if a.Name == "checksums.txt" {
			info.ChecksumURL = a.BrowserDownloadURL
		}
	}

	return info, nil
}

// ExecuteSelfUpdate performs an in-place self-update of the repokit binary according to UpdateOptions.
func ExecuteSelfUpdate(opts UpdateOptions) (*UpdateInfo, error) {
	currentVersion := opts.TargetVersion
	if currentVersion == "" {
		currentVersion = Version
	}

	info, err := CheckForUpdate(currentVersion, opts.HTTPClient)
	if err != nil {
		return nil, err
	}

	if opts.CheckOnly {
		return info, nil
	}

	if !info.UpdateAvailable && !opts.Force {
		return info, nil
	}

	if opts.DryRun {
		fmt.Printf("[dry-run] Update check for repokit:\n")
		fmt.Printf("  Current version:  %s\n", info.CurrentVersion)
		fmt.Printf("  Latest version:   %s\n", info.LatestVersion)
		fmt.Printf("  Update available: %v\n", info.UpdateAvailable)
		fmt.Printf("  Target asset:     %s\n", info.AssetName)
		fmt.Printf("  Download URL:     %s\n", info.DownloadURL)
		fmt.Printf("  Checksum URL:     %s\n", info.ChecksumURL)
		return info, nil
	}

	if info.DownloadURL == "" {
		return nil, fmt.Errorf("release asset %s download URL not found", info.AssetName)
	}
	if info.ChecksumURL == "" {
		return nil, fmt.Errorf("checksums.txt download URL not found")
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	// 1. Download checksums.txt
	reqCheck, err := http.NewRequest(http.MethodGet, info.ChecksumURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create checksum request: %w", err)
	}
	reqCheck.Header.Set("User-Agent", "repokit-self-update")
	respCheck, err := client.Do(reqCheck)
	if err != nil {
		return nil, fmt.Errorf("failed to download checksums: %w", err)
	}
	defer respCheck.Body.Close()
	if respCheck.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download checksums: HTTP %d", respCheck.StatusCode)
	}
	checksumBytes, err := io.ReadAll(respCheck.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read checksums: %w", err)
	}

	// 2. Download release asset
	reqAsset, err := http.NewRequest(http.MethodGet, info.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset request: %w", err)
	}
	reqAsset.Header.Set("User-Agent", "repokit-self-update")
	respAsset, err := client.Do(reqAsset)
	if err != nil {
		return nil, fmt.Errorf("failed to download asset: %w", err)
	}
	defer respAsset.Body.Close()
	if respAsset.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download asset: HTTP %d", respAsset.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "repokit-self-update-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, info.AssetName)
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp archive file: %w", err)
	}

	hasher := sha256.New()
	multiWriter := io.MultiWriter(archiveFile, hasher)
	if _, err := io.Copy(multiWriter, respAsset.Body); err != nil {
		_ = archiveFile.Close()
		return nil, fmt.Errorf("failed to save asset: %w", err)
	}
	if err := archiveFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close archive file: %w", err)
	}

	// 3. Verify SHA-256
	computedHash := fmt.Sprintf("%x", hasher.Sum(nil))
	expectedHash, err := parseChecksum(string(checksumBytes), info.AssetName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse checksum: %w", err)
	}
	if !strings.EqualFold(computedHash, expectedHash) {
		return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, computedHash)
	}

	// 4. Extract binary
	osName, _ := getTargetPlatform()
	targetBinaryName := "repokit"
	if osName == "windows" {
		targetBinaryName = "repokit.exe"
	}
	extractedBinaryPath := filepath.Join(tmpDir, targetBinaryName)
	if err := extractBinary(archivePath, info.AssetName, targetBinaryName, extractedBinaryPath); err != nil {
		return nil, fmt.Errorf("failed to extract binary: %w", err)
	}

	// 5. Replace current executable
	execPath := opts.TargetPath
	if execPath == "" {
		execPath, err = executablePathFunc()
		if err != nil {
			return nil, fmt.Errorf("failed to determine executable path: %w", err)
		}
	}

	if err := replaceExecutable(extractedBinaryPath, execPath); err != nil {
		return nil, fmt.Errorf("failed to replace executable: %w", err)
	}

	return info, nil
}

func parseChecksum(checksumsContent, assetName string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(checksumsContent))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hash := fields[0]
			name := strings.TrimPrefix(fields[1], "*")
			if filepath.Base(name) == assetName {
				return strings.ToLower(hash), nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading checksums failed: %w", err)
	}
	return "", fmt.Errorf("checksum for %s not found in checksums.txt", assetName)
}

func extractBinary(archivePath, assetName, targetBinaryName, destPath string) error {
	if strings.HasSuffix(assetName, ".zip") {
		return extractBinaryFromZip(archivePath, targetBinaryName, destPath)
	}
	if strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz") {
		return extractBinaryFromTarGz(archivePath, targetBinaryName, destPath)
	}
	return fmt.Errorf("unsupported archive format for asset %s", assetName)
}

func extractBinaryFromTarGz(archivePath, targetBinaryName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive failed: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader failed: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar header failed: %w", err)
		}
		if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA || header.Typeflag == 0 {
			base := filepath.Base(header.Name)
			if base == targetBinaryName || base == "repokit" || (targetBinaryName == "repokit.exe" && base == "repokit.exe") {
				out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					return fmt.Errorf("create dest file failed: %w", err)
				}
				if _, err := io.Copy(out, tr); err != nil {
					_ = out.Close()
					return fmt.Errorf("extracting binary data failed: %w", err)
				}
				if err := out.Close(); err != nil {
					return fmt.Errorf("closing dest file failed: %w", err)
				}
				return nil
			}
		}
	}
	return fmt.Errorf("binary %s not found in tar.gz archive", targetBinaryName)
}

func extractBinaryFromZip(archivePath, targetBinaryName, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip failed: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if base == targetBinaryName || base == "repokit" || (targetBinaryName == "repokit.exe" && base == "repokit.exe") {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("open file in zip failed: %w", err)
			}
			defer rc.Close()

			out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("create dest file failed: %w", err)
			}
			if _, err := io.Copy(out, rc); err != nil {
				_ = out.Close()
				return fmt.Errorf("extracting zip entry data failed: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("closing dest file failed: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("binary %s not found in zip archive", targetBinaryName)
}

func replaceExecutable(extractedBinaryPath, targetExecPath string) error {
	targetDir := filepath.Dir(targetExecPath)

	tmpTarget, err := os.CreateTemp(targetDir, ".repokit-update-*")
	if err != nil {
		return copyFileAndChmod(extractedBinaryPath, targetExecPath, 0755)
	}
	tmpName := tmpTarget.Name()
	defer func() {
		if _, err := os.Stat(tmpName); err == nil {
			_ = os.Remove(tmpName)
		}
	}()

	src, err := os.Open(extractedBinaryPath)
	if err != nil {
		_ = tmpTarget.Close()
		return fmt.Errorf("open extracted binary failed: %w", err)
	}
	defer src.Close()

	if _, err := io.Copy(tmpTarget, src); err != nil {
		_ = tmpTarget.Close()
		return fmt.Errorf("copying extracted binary failed: %w", err)
	}

	if err := tmpTarget.Chmod(0755); err != nil {
		_ = tmpTarget.Close()
		return fmt.Errorf("chmod temp binary failed: %w", err)
	}

	if err := tmpTarget.Close(); err != nil {
		return fmt.Errorf("close temp binary failed: %w", err)
	}

	if err := os.Rename(tmpName, targetExecPath); err != nil {
		if runtime.GOOS == "windows" {
			oldPath := targetExecPath + ".old"
			_ = os.Remove(oldPath)
			if renameOldErr := os.Rename(targetExecPath, oldPath); renameOldErr == nil {
				if renameErr := os.Rename(tmpName, targetExecPath); renameErr == nil {
					_ = os.Chmod(targetExecPath, 0755)
					return nil
				}
			}
		}
		return copyFileAndChmod(extractedBinaryPath, targetExecPath, 0755)
	}

	if err := os.Chmod(targetExecPath, 0755); err != nil {
		return fmt.Errorf("chmod target executable failed: %w", err)
	}

	return nil
}

func copyFileAndChmod(srcPath, dstPath string, mode os.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return dst.Chmod(mode)
}
