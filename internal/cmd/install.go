package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
	"github.com/eliasmeireles/traefikctl/internal/traefik"
)

const latestVersionAlias = "latest"

var (
	checkOnly      bool
	installVersion string
	installUpgrade bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install or upgrade Traefik on the system",
	Long: `Download and install (or upgrade) the Traefik binary from GitHub releases.
Supports amd64 and arm64 architectures.
Requires root/sudo privileges.

Without --version the most recent release is fetched from the Traefik
GitHub API. Pass --version vX.Y.Z to pin a specific one. Use --upgrade
to replace an existing binary with the requested version.

This command will:
1. Download Traefik binary to /usr/local/bin/traefik
2. Create traefik system user and group (if missing)
3. Set capabilities for low port binding
4. Create required directories`,
	SilenceUsage: true,
	RunE:         runInstall,
}

func init() {
	installCmd.Flags().BoolVar(&checkOnly, "check", false, "Only check if Traefik is installed")
	installCmd.Flags().StringVar(&installVersion, "version", latestVersionAlias, "Traefik version to install (e.g. v3.4.0); defaults to the latest release")
	installCmd.Flags().BoolVar(&installUpgrade, "upgrade", false, "Replace the existing Traefik binary with the requested version")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	installer := traefik.NewInstaller()

	if checkOnly {
		return runInstallCheck(installer)
	}

	if err := requireRoot(); err != nil {
		return err
	}

	version, err := resolveTraefikVersion(installer, installVersion)
	if err != nil {
		return err
	}

	if installer.IsInstalled() && !installUpgrade {
		current, _ := installer.GetVersion()
		logger.Info("Traefik is already installed:\n%s", current)
		logger.Info("Run with --upgrade to replace it (e.g. 'sudo traefikctl install --upgrade --version %s').", version)
		return runInstallSetup(installer)
	}

	action := "Installing"
	if installer.IsInstalled() {
		action = "Upgrading to"
	}
	logger.Info("%s Traefik %s...", action, version)

	if err := installer.Install(version); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	if newVersion, err := installer.GetVersion(); err != nil {
		logger.Warn("Installation completed but failed to verify: %v", err)
	} else {
		logger.Info("Installation completed:\n%s", newVersion)
	}

	if err := runInstallSetup(installer); err != nil {
		return err
	}

	if installUpgrade {
		logger.Info("\nRestart Traefik to load the new binary: sudo traefikctl service restart")
	} else {
		logger.Info("\nNext steps:")
		logger.Info("1. Generate configs: sudo traefikctl config --generate")
		logger.Info("2. Install service: sudo traefikctl service install")
		logger.Info("3. Start service: sudo systemctl start traefikctl")
		logger.Info("4. Check setup: traefikctl check")
	}
	return nil
}

func runInstallCheck(installer *traefik.Installer) error {
	if installer.IsInstalled() {
		version, err := installer.GetVersion()
		if err != nil {
			return fmt.Errorf("failed to get version: %w", err)
		}
		logger.Info("Traefik is installed:\n%s", version)
		return nil
	}
	fmt.Println("Traefik is not installed")
	return nil
}

func runInstallSetup(installer *traefik.Installer) error {
	logger.Info("\n=== Setting up system ===")

	if err := installer.EnsureUser(); err != nil {
		return fmt.Errorf("failed to create system user: %w", err)
	}

	if err := installer.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	return nil
}

// resolveTraefikVersion expands the "latest" alias — the default when no
// --version is given — by querying the GitHub API. An empty value means the
// same thing, so an unset flag never falls back to a stale pinned release.
// Any other value is returned verbatim.
func resolveTraefikVersion(installer *traefik.Installer, requested string) (string, error) {
	if requested != "" && !strings.EqualFold(requested, latestVersionAlias) {
		return requested, nil
	}
	logger.Info("Fetching latest Traefik version...")
	v, err := installer.LatestVersion()
	if err != nil {
		return "", fmt.Errorf(
			"failed to resolve latest version: %w (pass --version vX.Y.Z to install a specific release)", err)
	}
	logger.Info("Latest Traefik version: %s", v)
	return v, nil
}
