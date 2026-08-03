package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

const (
	defaultReleasesAPIURL = "https://api.github.com/repos/eliasmeireles/traefikctl/releases?per_page=30"
	// Asset filename matches the release workflow output (release.yml uses
	// "traefikctl-<os>-<arch>" with hyphens, not underscores).
	downloadURLPattern = "https://github.com/eliasmeireles/traefikctl/releases/download/%s/traefikctl-%s-%s"
	installPath        = "/usr/local/bin/traefikctl"
)

// stableTagRegex matches semver-shaped release tags (e.g. v0.0.4, v1.2.3).
// Pre-release tags like "v0.0.4-rc-05" are intentionally excluded so the
// default 'traefikctl update' (no --version) only ships final builds.
var stableTagRegex = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

var httpClient = &http.Client{Timeout: 60 * time.Second}

var updateVersion string

var updateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update traefikctl to the latest release",
	SilenceUsage: true,
	RunE:         runUpdate,
}

func init() {
	updateCmd.Flags().StringVar(&updateVersion, "version", "", "Specific version to install (e.g. v1.2.3)")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	version := updateVersion

	if version == "" {
		logger.Info("Fetching latest stable version...")
		v, err := fetchLatestStableVersion(defaultReleasesAPIURL)
		if err != nil {
			return fmt.Errorf("failed to fetch latest stable version: %w", err)
		}
		version = v
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	url := fmt.Sprintf(downloadURLPattern, version, goos, goarch)
	logger.Info("Downloading traefikctl %s...", version)

	tmp, err := downloadToTemp(url, filepath.Dir(installPath))
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = os.Remove(tmp) }()

	if err := os.Chmod(tmp, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := replaceBinary(tmp, installPath); err != nil {
		return permissionHint("install binary", installPath, err)
	}

	logger.Info("Update complete: traefikctl %s installed.", version)
	return nil
}

// fetchLatestStableVersion lists releases at apiURL (newest first) and
// returns the first tag_name that matches stableTagRegex and is neither a
// draft nor a pre-release. Pre-release tags (e.g. "v0.0.4-rc-05") are
// intentionally skipped so users running 'traefikctl update' without a
// --version flag never accidentally install an RC. To install one, pass
// --version explicitly.
func fetchLatestStableVersion(apiURL string) (string, error) {
	resp, err := httpClient.Get(apiURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var releases []struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		if stableTagRegex.MatchString(r.TagName) {
			return r.TagName, nil
		}
	}
	return "", fmt.Errorf("no stable release found in latest %d entries (pre-release builds are skipped — pass --version to install one)", len(releases))
}

// downloadToTemp downloads the binary at url to a temporary file and returns its path.
// The temp file is created in dir when possible so the final rename stays on the
// same filesystem (atomic and avoids EXDEV cross-device errors). If dir is not
// writable, it falls back to the OS default temp directory.
func downloadToTemp(url, dir string) (string, error) {
	resp, err := httpClient.Get(url) //nolint:gosec
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp(dir, "traefikctl-update-*")
	if err != nil {
		tmp, err = os.CreateTemp("", "traefikctl-update-*")
		if err != nil {
			return "", err
		}
	}
	defer func() { _ = tmp.Close() }()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}

	return tmp.Name(), nil
}

// replaceBinary moves src to dst atomically when possible, falling back to a
// copy when the two paths live on different filesystems (EXDEV).
func replaceBinary(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	staging, err := os.CreateTemp(filepath.Dir(dst), "traefikctl-update-*")
	if err != nil {
		return err
	}
	stagingPath := staging.Name()
	cleanup := func() { _ = os.Remove(stagingPath) }

	if _, err := io.Copy(staging, in); err != nil {
		_ = staging.Close()
		cleanup()
		return err
	}
	if err := staging.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(stagingPath, 0755); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(stagingPath, dst); err != nil {
		cleanup()
		return err
	}
	return nil
}
