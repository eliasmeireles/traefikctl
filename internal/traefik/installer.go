package traefik

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

const (
	DefaultVersion   = "v3.3.5"
	BinaryPath       = "/usr/local/bin/traefik"
	DownloadURLBase  = "https://github.com/traefik/traefik/releases/download"
)

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

func (i *Installer) Install(version string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("automatic installation is only supported on Linux")
	}

	if version == "" {
		version = DefaultVersion
	}

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "arm64" {
		arch = "arm64"
	} else {
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
	}

	// Set ownership
	for _, dir := range dirs {
		if err := runCommand("chown", "-R", "traefik:traefik", dir); err != nil {
			logger.Warn("Failed to set ownership on %s: %v", dir, err)
		}
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
