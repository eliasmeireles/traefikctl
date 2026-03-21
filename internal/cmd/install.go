package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
	"github.com/eliasmeireles/traefikctl/internal/traefik"
)

var (
	checkOnly      bool
	installVersion string
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Traefik on the system",
	Long: `Download and install Traefik binary from GitHub releases.
Supports amd64 and arm64 architectures.
Requires root/sudo privileges.

This command will:
1. Download Traefik binary to /usr/local/bin/traefik
2. Create traefik system user and group
3. Set capabilities for low port binding
4. Create required directories`,
	SilenceUsage: true,
	RunE:         runInstall,
}

func init() {
	installCmd.Flags().BoolVar(&checkOnly, "check", false, "Only check if Traefik is installed")
	installCmd.Flags().StringVar(&installVersion, "version", traefik.DefaultVersion, "Traefik version to install")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	installer := traefik.NewInstaller()

	if checkOnly {
		if installer.IsInstalled() {
			version, err := installer.GetVersion()
			if err != nil {
				return fmt.Errorf("failed to get version: %w", err)
			}
			logger.Info("Traefik is installed:\n%s", version)
		} else {
			fmt.Println("Traefik is not installed")
		}
		return nil
	}

	if installer.IsInstalled() {
		version, _ := installer.GetVersion()
		logger.Info("Traefik is already installed:\n%s", version)
	} else {
		logger.Info("Installing Traefik %s...", installVersion)
		if err := installer.Install(installVersion); err != nil {
			return fmt.Errorf("installation failed: %w", err)
		}

		version, err := installer.GetVersion()
		if err != nil {
			logger.Warn("Installation completed but failed to verify: %v", err)
		} else {
			logger.Info("Installation completed:\n%s", version)
		}
	}

	logger.Info("\n=== Setting up system ===")

	if err := installer.EnsureUser(); err != nil {
		logger.Error("Failed to create system user: %v", err)
	}

	if err := installer.EnsureDirectories(); err != nil {
		logger.Error("Failed to create directories: %v", err)
	}

	logger.Info("\nNext steps:")
	logger.Info("1. Generate configs: sudo traefikctl config --generate")
	logger.Info("2. Install service: sudo traefikctl service install")
	logger.Info("3. Start service: sudo systemctl start traefikctl")
	logger.Info("4. Check setup: traefikctl check")

	return nil
}
