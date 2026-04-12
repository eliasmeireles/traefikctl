package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

const defaultServiceTemplate = `[Unit]
Description=Traefik Proxy
Documentation=https://github.com/eliasmeireles/traefikctl
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=traefik
Group=traefik
ExecStart=/usr/local/bin/traefik --configFile=/etc/traefik/traefik.yaml
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=traefikctl

# Allow binding to privileged ports (80, 443) as non-root
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

# Security settings
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=read-only
ReadWritePaths=/etc/traefik /var/log/traefik

[Install]
WantedBy=multi-user.target
`

var (
	serviceName   string
	svcLogsFollow bool
	svcLogsLines  int
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage traefikctl systemd service",
}

var serviceInstallCmd = &cobra.Command{
	Use:          "install",
	Short:        "Install systemd service",
	SilenceUsage: true,
	RunE:         runServiceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:          "uninstall",
	Short:        "Uninstall systemd service",
	SilenceUsage: true,
	RunE:         runServiceUninstall,
}

var serviceStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Check service status",
	SilenceUsage: true,
	RunE:         runServiceStatus,
}

var serviceLogsCmd = &cobra.Command{
	Use:          "logs",
	Short:        "View service logs via journalctl",
	SilenceUsage: true,
	RunE:         runServiceLogs,
}

var serviceRestartCmd = &cobra.Command{
	Use:          "restart",
	Short:        "Restart the Traefik service",
	SilenceUsage: true,
	RunE:         runServiceRestart,
}

var serviceReloadCmd = &cobra.Command{
	Use:          "reload",
	Short:        "Reload Traefik config without full restart (systemctl reload)",
	SilenceUsage: true,
	RunE:         runServiceReload,
}

func init() {
	serviceInstallCmd.Flags().StringVar(&serviceName, "name", "traefikctl", "Service name")
	serviceUninstallCmd.Flags().StringVar(&serviceName, "name", "traefikctl", "Service name")
	serviceStatusCmd.Flags().StringVar(&serviceName, "name", "traefikctl", "Service name")
	serviceLogsCmd.Flags().StringVar(&serviceName, "name", "traefikctl", "Service name")
	serviceLogsCmd.Flags().BoolVarP(&svcLogsFollow, "follow", "f", true, "Follow log output")
	serviceLogsCmd.Flags().IntVarP(&svcLogsLines, "lines", "n", 50, "Number of lines to show")

	serviceRestartCmd.Flags().StringVar(&serviceName, "name", "traefikctl", "Service name")
	serviceReloadCmd.Flags().StringVar(&serviceName, "name", "traefikctl", "Service name")
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceLogsCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
	serviceCmd.AddCommand(serviceReloadCmd)
	rootCmd.AddCommand(serviceCmd)
}

func runServiceInstall(cmd *cobra.Command, args []string) error {
	systemdPath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	if err := os.WriteFile(systemdPath, []byte(defaultServiceTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	logger.Info("Service file created: %s", systemdPath)

	if err := systemctl("daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	if err := systemctl("enable", serviceName); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	logger.Info("Service installed and enabled")
	logger.Info("Start with: sudo systemctl start %s", serviceName)
	logger.Info("View logs: sudo journalctl -u %s -f", serviceName)

	return nil
}

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	_ = systemctl("stop", serviceName)
	_ = systemctl("disable", serviceName)

	systemdPath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	if err := os.Remove(systemdPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	if err := systemctl("daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	logger.Info("Service uninstalled")
	return nil
}

func runServiceStatus(cmd *cobra.Command, args []string) error {
	output, err := exec.Command("systemctl", "status", serviceName).CombinedOutput()
	fmt.Println(string(output))
	if err != nil {
		return fmt.Errorf("service not running or not found")
	}
	return nil
}

func runServiceLogs(cmd *cobra.Command, args []string) error {
	return journalctlLogs(serviceName, svcLogsFollow, svcLogsLines)
}

func runServiceRestart(cmd *cobra.Command, args []string) error {
	if err := systemctl("restart", serviceName); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}
	logger.Info("Service '%s' restarted", serviceName)
	return nil
}

func runServiceReload(cmd *cobra.Command, args []string) error {
	if err := systemctl("reload", serviceName); err != nil {
		return fmt.Errorf("failed to reload service (not all services support reload): %w", err)
	}
	logger.Info("Service '%s' config reloaded", serviceName)
	return nil
}

func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
