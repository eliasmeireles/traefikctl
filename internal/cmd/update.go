package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

const (
	defaultGitHubAPIURL = "https://api.github.com/repos/eliasmeireles/traefikctl/releases/latest"
	// Asset filename matches the release workflow output (release.yml uses
	// "traefikctl-<os>-<arch>" with hyphens, not underscores).
	downloadURLPattern = "https://github.com/eliasmeireles/traefikctl/releases/download/%s/traefikctl-%s-%s"
	installPath        = "/usr/local/bin/traefikctl"
)

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
	version := updateVersion

	if version == "" {
		logger.Info("Fetching latest version...")
		v, err := fetchLatestVersion(defaultGitHubAPIURL)
		if err != nil {
			return fmt.Errorf("failed to fetch latest version: %w", err)
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

// fetchLatestVersion queries the GitHub releases API at apiURL and returns the tag_name of the latest release.
func fetchLatestVersion(apiURL string) (string, error) {
	resp, err := httpClient.Get(apiURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if payload.TagName == "" {
		return "", fmt.Errorf("tag_name missing in response")
	}

	return payload.TagName, nil
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
