package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
	"github.com/eliasmeireles/traefikctl/internal/traefik"
	"github.com/eliasmeireles/traefikctl/internal/validate"
)

// defaultServiceTemplate uses systemd's LogsDirectory= directive so /var/log/traefik
// is (re)created with traefik:traefik ownership on every service start. Without
// this, a manual recreation of the directory (e.g. after rm -rf /var/log/**)
// would leave it as root:root and Traefik would crash on the first log write.
//
// It also has to survive boot ordering. An entry point may bind to one specific
// address rather than the wildcard — typically an overlay/VPN interface such as
// WireGuard, NetBird or Tailscale — and those are configured well after
// network-online.target is reached. Traefik does not retry a failed bind, it
// exits. Restart=always plus StartLimitIntervalSec=0 means systemd keeps
// retrying until the address shows up, instead of tripping the default start
// limit (5 starts within 10s) and leaving the proxy down until someone notices.
const defaultServiceTemplate = `[Unit]
Description=Traefik Proxy
Documentation=https://github.com/eliasmeireles/traefikctl
After=network-online.target
Wants=network-online.target
# Entry points bound to a late-appearing address (VPN/overlay interfaces) make
# Traefik exit at boot. Never give up restarting it.
StartLimitIntervalSec=0

[Service]
Type=simple
User=traefik
Group=traefik
ExecStart=/usr/local/bin/traefik --configFile=/etc/traefik/traefik.yaml
Restart=always
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
ReadWritePaths=/etc/traefik

# Ensure /var/log/traefik exists with traefik:traefik ownership on every start.
LogsDirectory=traefik
LogsDirectoryMode=0755

[Install]
WantedBy=multi-user.target
`

var (
	serviceName   string
	svcLogsFollow bool
	svcLogsLines  int

	svcRestartSkipValidate bool
	svcRestartNoWait       bool
	svcRestartWait         time.Duration
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

var svcReloadRestart bool

var serviceReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Explain how Traefik picks up config changes (does not actually reload)",
	Long: `Traefik has no config-reload signal: the systemd unit defines no
ExecReload=, so 'systemctl reload traefikctl' always fails. This command is
informational rather than an alias for that failing call — running it
used to look like a successful hot reload while silently doing nothing.

Dynamic config (routers, services, middlewares under /etc/traefik/dynamic/)
is picked up automatically by Traefik's file watcher — no action needed.

Static config (/etc/traefik/traefik.yaml) always requires a real restart.
Use --restart to do that now, or run 'traefikctl service restart' directly.`,
	Deprecated:   "does not reload Traefik; kept as an explanation, not an action. Use 'traefikctl service restart' for static config changes.",
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
	serviceRestartCmd.Flags().BoolVar(&svcRestartSkipValidate, "skip-validate", false, "Skip static config validation before restarting")
	serviceRestartCmd.Flags().BoolVar(&svcRestartNoWait, "no-wait", false, "Restart and return immediately, without confirming the service stayed up")
	serviceRestartCmd.Flags().DurationVar(&svcRestartWait, "wait", DefaultRestartWait, "How long to wait for the service to stabilize after restart")
	serviceReloadCmd.Flags().StringVar(&serviceName, "name", "traefikctl", "Service name")
	serviceReloadCmd.Flags().BoolVar(&svcReloadRestart, "restart", false, "Actually restart the service now (equivalent to 'service restart')")
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceLogsCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
	serviceCmd.AddCommand(serviceReloadCmd)
	rootCmd.AddCommand(serviceCmd)
}

func runServiceInstall(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	systemdPath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	installer := traefik.NewInstaller()
	if err := installer.EnsureUser(); err != nil {
		logger.Warn("Could not ensure traefik user: %v", err)
	}
	if err := installer.EnsureDirectories(); err != nil {
		logger.Warn("Could not ensure traefik directories: %v", err)
	}

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
	logger.Info("Enable log rotation (recommended): sudo traefikctl logs rotation install")

	return nil
}

func runServiceUninstall(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

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
	if err := requireRoot(); err != nil {
		return err
	}

	if !svcRestartSkipValidate {
		if err := refuseIfConfigInvalid(); err != nil {
			return err
		}
	}

	if err := systemctl("restart", serviceName); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	if svcRestartNoWait {
		logger.Warn("Service '%s' restart issued but NOT confirmed (--no-wait) — it may still be crash-looping", serviceName)
		return nil
	}

	state, err := waitHealthy(serviceName, svcRestartWait, DefaultSettle)
	if err != nil {
		logger.Error("Service '%s' did not come up healthy after restart: %v", serviceName, err)
		logger.Info("Last journal lines:")
		_ = journalctlLogs(serviceName, false, 50)
		return fmt.Errorf("service '%s' failed to stabilize after restart: %w", serviceName, err)
	}

	logger.Info("Service '%s' restarted and stable (%s/%s)", serviceName, state.ActiveState, state.SubState)
	return nil
}

// refuseIfConfigInvalid validates the static config and, if it has
// problems, prints the full report and returns an error instead of letting
// the caller restart into a crash-loop.
func refuseIfConfigInvalid() error {
	result, err := validate.StaticFile(context.Background(), defaultStaticConfigPath, validate.Options{})
	if err != nil {
		return fmt.Errorf("failed to validate %s before restart: %w", defaultStaticConfigPath, err)
	}
	if result.Skipped || result.OK() {
		return nil
	}

	fmt.Print(result.String())
	return fmt.Errorf("refusing to restart: %d problem(s) found in %s (fix them, or pass --skip-validate to restart anyway)", len(result.Errors()), defaultStaticConfigPath)
}

func runServiceReload(cmd *cobra.Command, args []string) error {
	if svcReloadRestart {
		return runServiceRestart(cmd, args)
	}

	logger.Info("Dynamic config (routers/services/middlewares) reloads automatically — no action needed.")
	logger.Info("Static config (%s) requires a real restart:", defaultStaticConfigPath)
	logger.Info("  sudo traefikctl service reload --restart   (or: sudo traefikctl service restart)")
	return nil
}

func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
