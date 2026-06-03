package install

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shubhangtiwari/aidlc/aidlc/internal/contract"
)

const (
	DefaultUpgradeRepository = "shubhangtiwari/aidlc"
	defaultGitHubAPIBaseURL  = "https://api.github.com"
)

type UpgradeRequest struct {
	Options        contract.UpgradeOptions
	CurrentVersion string
	GoOS           string
	GoArch         string
	ExecutablePath string
	APIBaseURL     string
	HTTPClient     *http.Client
}

type UpgradeResult struct {
	ReleaseTag  string
	Version     string
	AssetName   string
	Destination string
	DryRun      bool
	Installed   bool
	Skipped     bool
}

type releaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

type releaseResponse struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

func Upgrade(ctx context.Context, req UpgradeRequest) (UpgradeResult, error) {
	plan, checksumsURL, assetURL, platform, err := planUpgrade(ctx, req)
	if err != nil {
		return UpgradeResult{}, err
	}
	result := UpgradeResult{
		ReleaseTag:  plan.ReleaseTag,
		Version:     plan.Version,
		AssetName:   plan.AssetName,
		Destination: plan.Destination,
		DryRun:      req.Options.DryRun,
	}
	if req.Options.DryRun {
		return result, nil
	}
	if shouldSkipUpgrade(req, plan.Version) {
		result.Skipped = true
		return result, nil
	}

	client := req.httpClient()
	checksumsData, err := download(ctx, client, checksumsURL, ChecksumsAssetName)
	if err != nil {
		return result, err
	}
	checksums, err := ParseChecksums(bytes.NewReader(checksumsData))
	if err != nil {
		return result, err
	}

	archiveData, err := download(ctx, client, assetURL, plan.AssetName)
	if err != nil {
		return result, err
	}
	if err := VerifyChecksum(bytes.NewReader(archiveData), plan.AssetName, checksums); err != nil {
		return result, err
	}
	binary, err := extractBinary(archiveData, platform)
	if err != nil {
		return result, err
	}
	if err := replaceBinary(plan.Destination, binary, platform); err != nil {
		return result, err
	}
	result.Installed = true
	return result, nil
}

func planUpgrade(ctx context.Context, req UpgradeRequest) (UpgradeResult, string, string, ReleasePlatform, error) {
	platform, err := req.platform()
	if err != nil {
		return UpgradeResult{}, "", "", ReleasePlatform{}, err
	}
	release, err := fetchRelease(ctx, req)
	if err != nil {
		return UpgradeResult{}, "", "", ReleasePlatform{}, err
	}
	version, err := normalizeReleaseVersion(release.TagName)
	if err != nil {
		return UpgradeResult{}, "", "", ReleasePlatform{}, err
	}
	asset, ok := findAsset(release.Assets, platform.ArchiveName)
	if !ok {
		return UpgradeResult{}, "", "", ReleasePlatform{}, fmt.Errorf("release %s missing asset %s", release.TagName, platform.ArchiveName)
	}
	checksums, ok := findAsset(release.Assets, ChecksumsAssetName)
	if !ok {
		return UpgradeResult{}, "", "", ReleasePlatform{}, fmt.Errorf("release %s missing asset %s", release.TagName, ChecksumsAssetName)
	}
	destination, err := req.destination(platform)
	if err != nil {
		return UpgradeResult{}, "", "", ReleasePlatform{}, err
	}

	return UpgradeResult{
		ReleaseTag:  release.TagName,
		Version:     version,
		AssetName:   asset.Name,
		Destination: destination,
	}, checksums.DownloadURL, asset.DownloadURL, platform, nil
}

func fetchRelease(ctx context.Context, req UpgradeRequest) (releaseResponse, error) {
	repo := strings.TrimSpace(req.Options.Repository)
	if repo == "" {
		repo = DefaultUpgradeRepository
	}
	if !validRepository(repo) {
		return releaseResponse{}, &UsageError{Message: fmt.Sprintf("invalid repository %q: expected owner/repo", repo)}
	}
	selector := strings.TrimSpace(req.Options.Version.Value)
	if selector == "" {
		selector = "latest"
	}
	var path string
	if selector == "latest" {
		path = fmt.Sprintf("/repos/%s/releases/latest", repo)
	} else {
		tag, err := releaseTagForSelector(selector)
		if err != nil {
			return releaseResponse{}, err
		}
		path = fmt.Sprintf("/repos/%s/releases/tags/%s", repo, url.PathEscape(tag))
	}

	base := strings.TrimRight(req.APIBaseURL, "/")
	if base == "" {
		base = defaultGitHubAPIBaseURL
	}
	data, err := download(ctx, req.httpClient(), base+path, "release metadata")
	if err != nil {
		return releaseResponse{}, err
	}
	var release releaseResponse
	if err := json.Unmarshal(data, &release); err != nil {
		return releaseResponse{}, fmt.Errorf("parse release metadata: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return releaseResponse{}, fmt.Errorf("release metadata missing tag_name")
	}
	return release, nil
}

func download(ctx context.Context, client *http.Client, rawURL, label string) ([]byte, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("%s download URL is empty", label)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", label, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s: %s", label, response.Status)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	return data, nil
}

func extractBinary(data []byte, platform ReleasePlatform) ([]byte, error) {
	if platform.OS == "windows" {
		return extractZipBinary(data, platform.BinaryName)
	}
	return extractTarGzipBinary(data, platform.BinaryName)
}

func extractTarGzipBinary(data []byte, binaryName string) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open tar.gz: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar.gz: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binaryName {
			continue
		}
		return io.ReadAll(tarReader)
	}
	return nil, fmt.Errorf("archive missing %s", binaryName)
}

func extractZipBinary(data []byte, binaryName string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || filepath.Base(file.Name) != binaryName {
			continue
		}
		open, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", binaryName, err)
		}
		data, readErr := io.ReadAll(open)
		closeErr := open.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", binaryName, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", binaryName, closeErr)
		}
		return data, nil
	}
	return nil, fmt.Errorf("archive missing %s", binaryName)
}

func replaceBinary(destination string, binary []byte, platform ReleasePlatform) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create install directory: %w", err)
	}
	staged, err := os.CreateTemp(filepath.Dir(destination), ".aidlc-upgrade-*")
	if err != nil {
		return fmt.Errorf("stage binary: %w", err)
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath)

	if _, err := staged.Write(binary); err != nil {
		staged.Close()
		return fmt.Errorf("write staged binary: %w", err)
	}
	if err := staged.Chmod(0o755); err != nil {
		staged.Close()
		return fmt.Errorf("chmod staged binary: %w", err)
	}
	if err := staged.Close(); err != nil {
		return fmt.Errorf("close staged binary: %w", err)
	}
	if platform.OS == "windows" {
		return replaceBinaryWindows(destination, stagedPath)
	}
	if err := os.Rename(stagedPath, destination); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

func replaceBinaryWindows(destination, stagedPath string) error {
	backupPath := ""
	if _, err := os.Stat(destination); err == nil {
		backup, err := os.CreateTemp(filepath.Dir(destination), filepath.Base(destination)+".backup-*")
		if err != nil {
			return fmt.Errorf("stage existing binary backup: %w", err)
		}
		backupPath = backup.Name()
		if err := backup.Close(); err != nil {
			return fmt.Errorf("close existing binary backup placeholder: %w", err)
		}
		if err := os.Remove(backupPath); err != nil {
			return fmt.Errorf("prepare existing binary backup: %w", err)
		}
		if err := os.Rename(destination, backupPath); err != nil {
			return fmt.Errorf("move existing binary aside: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect existing binary: %w", err)
	}

	if err := os.Rename(stagedPath, destination); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, destination)
		}
		return fmt.Errorf("replace binary: %w", err)
	}
	if backupPath != "" {
		_ = os.Remove(backupPath)
	}
	return nil
}

func normalizeReleaseVersion(tag string) (string, error) {
	trimmed := strings.TrimSpace(tag)
	if strings.HasPrefix(trimmed, "aidlc/") {
		trimmed = strings.TrimPrefix(trimmed, "aidlc/")
	}
	if err := validateVersion(trimmed); err != nil {
		return "", fmt.Errorf("unsupported release tag %q: %w", tag, err)
	}
	return trimmed, nil
}

func releaseTagForSelector(selector string) (string, error) {
	if selector == "latest" {
		return selector, nil
	}
	if strings.HasPrefix(selector, "aidlc/") {
		if err := validateVersion(strings.TrimPrefix(selector, "aidlc/")); err != nil {
			return "", err
		}
		return selector, nil
	}
	if err := validateVersion(selector); err != nil {
		return "", err
	}
	return "aidlc/" + selector, nil
}

func validateVersion(version string) error {
	if !strings.HasPrefix(version, "v") {
		return &UsageError{Message: "version must be latest, vX.Y.Z, or aidlc/vX.Y.Z"}
	}
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 {
		return &UsageError{Message: "version must be latest, vX.Y.Z, or aidlc/vX.Y.Z"}
	}
	for _, part := range parts {
		if part == "" {
			return &UsageError{Message: "version must be latest, vX.Y.Z, or aidlc/vX.Y.Z"}
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return &UsageError{Message: "version must be latest, vX.Y.Z, or aidlc/vX.Y.Z"}
			}
		}
	}
	return nil
}

func shouldSkipUpgrade(req UpgradeRequest, targetVersion string) bool {
	if req.Options.Version.Explicit {
		return false
	}
	current := strings.TrimSpace(req.CurrentVersion)
	return current != "" && current == targetVersion
}

func findAsset(assets []releaseAsset, name string) (releaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func validRepository(repo string) bool {
	parts := strings.Split(repo, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

func (r UpgradeRequest) httpClient() *http.Client {
	if r.HTTPClient != nil {
		return r.HTTPClient
	}
	return http.DefaultClient
}

func (r UpgradeRequest) platform() (ReleasePlatform, error) {
	goos := r.GoOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := r.GoArch
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return ReleasePlatformFor(goos, goarch)
}

func (r UpgradeRequest) destination(platform ReleasePlatform) (string, error) {
	installDir := strings.TrimSpace(r.Options.InstallDir)
	if installDir == "" {
		executable := r.ExecutablePath
		if executable == "" {
			var err error
			executable, err = os.Executable()
			if err != nil {
				return "", fmt.Errorf("resolve executable path: %w", err)
			}
		}
		installDir = filepath.Dir(executable)
	}
	return filepath.Join(installDir, platform.BinaryName), nil
}
