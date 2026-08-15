package traefik

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

const (
	// No Traefik version is pinned anywhere: callers resolve "latest" from
	// the GitHub API, so a new release is picked up without a code change.
	BinaryPath      = "/usr/local/bin/traefik"
	ACMEStorePath   = "/etc/traefik/acme.json"
	DownloadURLBase = "https://github.com/traefik/traefik/releases/download"
	LatestAPIURL    = "https://api.github.com/repos/traefik/traefik/releases/latest"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type Installer struct{}

func NewInstaller() *Installer {
	return &Installer{}
}

func (i *Installer) IsInstalled() bool {
	_, err := exec.LookPath("traefik")
	return err == nil
}

func (i *Installer) GetVersion() (string, error) {
	cmd := exec.Command("traefik", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get Traefik version: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// LatestVersion queries the Traefik GitHub API and returns the tag_name of
// the latest release (e.g. "v3.4.0").
func (i *Installer) LatestVersion() (string, error) {
	return fetchLatestVersion(LatestAPIURL)
}

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

func (i *Installer) Install(version string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("automatic installation is only supported on Linux")
	}

	if version == "" {
		return fmt.Errorf("no Traefik version resolved — expected a tag such as v3.7.10")
	}

	arch := runtime.GOARCH
	switch arch {
	case "amd64", "arm64":
		// supported
	default:
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	logger.Info("Downloading Traefik %s for linux/%s...", version, arch)

	tarball := fmt.Sprintf("traefik_%s_linux_%s.tar.gz", version, arch)
	url := fmt.Sprintf("%s/%s/%s", DownloadURLBase, version, tarball)
	tmpFile := fmt.Sprintf("/tmp/%s", tarball)

	// Download
	if err := runCommand("wget", "-q", "-O", tmpFile, url); err != nil {
		return fmt.Errorf("failed to download Traefik: %w", err)
	}

	// Extract
	if err := runCommand("tar", "-xzf", tmpFile, "-C", "/tmp/", "traefik"); err != nil {
		return fmt.Errorf("failed to extract Traefik: %w", err)
	}

	// Move binary
	if err := runCommand("mv", "/tmp/traefik", BinaryPath); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(BinaryPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Allow binding to low ports without root
	if err := runCommand("setcap", "cap_net_bind_service=+ep", BinaryPath); err != nil {
		logger.Warn("Failed to set capabilities (may need sudo): %v", err)
	}

	// Cleanup
	_ = os.Remove(tmpFile)

	logger.Info("Traefik installed at %s", BinaryPath)
	return nil
}

func (i *Installer) EnsureUser() error {
	// Check if user exists
	checkCmd := exec.Command("id", "-u", "traefik")
	if checkCmd.Run() == nil {
		logger.Info("Traefik user already exists")
		return nil
	}

	logger.Info("Creating traefik system user...")

	groupCmd := exec.Command("groupadd", "--system", "traefik")
	if err := groupCmd.Run(); err != nil {
		// Check if group already exists
		checkGroup := exec.Command("getent", "group", "traefik")
		if checkGroup.Run() != nil {
			return fmt.Errorf("failed to create traefik group: %w", err)
		}
	}

	userCmd := exec.Command("useradd",
		"--system",
		"--gid", "traefik",
		"--no-create-home",
		"--shell", "/usr/sbin/nologin",
		"--comment", "Traefik system user",
		"traefik")

	if err := userCmd.Run(); err != nil {
		checkUser := exec.Command("id", "-u", "traefik")
		if checkUser.Run() != nil {
			return fmt.Errorf("failed to create traefik user: %w", err)
		}
	}

	logger.Info("Traefik user and group created")
	return nil
}

func (i *Installer) EnsureDirectories() error {
	dirs := []string{
		"/etc/traefik/dynamic",
		"/var/log/traefik",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		if err := os.Chmod(dir, 0755); err != nil {
			logger.Warn("Failed to set mode on %s: %v", dir, err)
		}
		if err := runCommand("chown", "-R", "traefik:traefik", dir); err != nil {
			logger.Warn("Failed to set ownership on %s: %v", dir, err)
		}
	}

	return i.ensureACMEStore(ACMEStorePath)
}

// ensureACMEStore pre-creates the ACME key store.
//
// Traefik runs as the traefik user, but /etc/traefik itself stays root-owned,
// so Traefik cannot create acme.json on its own. Any certificatesResolvers
// entry then dies at startup with "unable to get ACME account: open
// /etc/traefik/acme.json: permission denied" and the resolver is silently
// dropped from the list — TLS issuance never works and nothing says why.
//
// An existing store is never truncated: it holds issued certificates and
// account keys. Only ownership and mode are corrected, and Traefik refuses to
// use the file unless it is 0600.
func (i *Installer) ensureACMEStore(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			logger.Warn("Failed to create %s: %v", path, err)
			return nil
		}
		_ = f.Close()
	}

	if err := os.Chmod(path, 0600); err != nil {
		logger.Warn("Failed to set mode on %s: %v", path, err)
	}
	if err := runCommand("chown", "traefik:traefik", path); err != nil {
		logger.Warn("Failed to set ownership on %s: %v", path, err)
	}

	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}
