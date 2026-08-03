package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

const (
	logrotateConfigPath = "/etc/logrotate.d/traefikctl"
	rotationServiceUnit = "traefikctl-logs-rotate.service"
	rotationTimerUnit   = "traefikctl-logs-rotate.timer"
	rotationServicePath = "/etc/systemd/system/" + rotationServiceUnit
	rotationTimerPath   = "/etc/systemd/system/" + rotationTimerUnit
	defaultBinaryPath   = "/usr/local/bin/traefikctl"
)

var (
	rotationMode     string
	rotationMaxSize  string
	rotationKeep     int
	rotationCompress bool
	rotationInterval string
	rotationBin      string
)

var logsRotationCmd = &cobra.Command{
	Use:   "rotation",
	Short: "Manage automatic Traefik log rotation",
	Long: `Install, uninstall and inspect automatic log rotation.

Two modes are supported:

  --mode=timer      A systemd timer (default) that runs
                    'traefikctl logs rotate' on a fixed interval.
                    No external dependency.

  --mode=logrotate  An /etc/logrotate.d/traefikctl drop-in that
                    relies on the system's logrotate daemon
                    (apt install logrotate). Uses copytruncate.`,
}

var logsRotationInstallCmd = &cobra.Command{
	Use:          "install",
	Short:        "Install automatic log rotation",
	SilenceUsage: true,
	RunE:         runRotationInstall,
}

var logsRotationUninstallCmd = &cobra.Command{
	Use:          "uninstall",
	Short:        "Remove timer and/or logrotate config installed by traefikctl",
	SilenceUsage: true,
	RunE:         runRotationUninstall,
}

var logsRotationStatusCmd = &cobra.Command{
	Use:          "status",
	Short:        "Show current rotation setup",
	SilenceUsage: true,
	RunE:         runRotationStatus,
}

func init() {
	logsRotationInstallCmd.Flags().StringVar(&rotationMode, "mode", "timer", "Rotation mode: timer | logrotate")
	logsRotationInstallCmd.Flags().StringVar(&rotationMaxSize, "max-size", "100M", "Max size before rotation")
	logsRotationInstallCmd.Flags().IntVar(&rotationKeep, "keep", 0, "Number of rotated copies to keep (0 = just truncate)")
	logsRotationInstallCmd.Flags().BoolVar(&rotationCompress, "compress", false, "Gzip rotated copies (used with --keep > 0)")
	logsRotationInstallCmd.Flags().StringVar(&rotationInterval, "interval", "5min", "Timer interval for --mode=timer (systemd OnUnitActiveSec)")
	logsRotationInstallCmd.Flags().StringVar(&rotationBin, "binary", defaultBinaryPath, "Absolute path to the traefikctl binary the timer should call")

	logsRotationCmd.AddCommand(logsRotationInstallCmd)
	logsRotationCmd.AddCommand(logsRotationUninstallCmd)
	logsRotationCmd.AddCommand(logsRotationStatusCmd)
	logsCmd.AddCommand(logsRotationCmd)
}

const timerServiceTpl = `[Unit]
Description=Traefik log rotation (managed by traefikctl)
Documentation=https://github.com/eliasmeireles/traefikctl

[Service]
Type=oneshot
ExecStart=%s logs rotate --max-size=%s --keep=%d%s
Nice=10
IOSchedulingClass=best-effort
IOSchedulingPriority=7
`

const timerTpl = `[Unit]
Description=Run Traefik log rotation periodically (managed by traefikctl)

[Timer]
OnBootSec=2min
OnUnitActiveSec=%s
Unit=%s
AccuracySec=30s
Persistent=true

[Install]
WantedBy=timers.target
`

const logrotateTpl = `# Managed by traefikctl. Do not edit by hand.
/var/log/traefik/traefik.log /var/log/traefik/access.log {
    size %s
    rotate %d
    %smissingok
    notifempty
    copytruncate
    su traefik traefik
}
`

func runRotationInstall(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	if _, err := parseSize(rotationMaxSize); err != nil {
		return fmt.Errorf("invalid --max-size %q: %w", rotationMaxSize, err)
	}
	if rotationKeep < 0 {
		return fmt.Errorf("--keep must be >= 0")
	}

	switch rotationMode {
	case "timer":
		return installTimer()
	case "logrotate":
		return installLogrotate()
	default:
		return fmt.Errorf("invalid --mode %q (expected: timer | logrotate)", rotationMode)
	}
}

func installTimer() error {
	if !strings.HasPrefix(rotationBin, "/") {
		return fmt.Errorf("--binary must be an absolute path, got %q", rotationBin)
	}

	compressFlag := ""
	if rotationCompress {
		compressFlag = " --compress"
	}

	svc := fmt.Sprintf(timerServiceTpl, rotationBin, rotationMaxSize, rotationKeep, compressFlag)
	if err := os.WriteFile(rotationServicePath, []byte(svc), 0o644); err != nil {
		return permissionHint("write rotation service unit", rotationServicePath, err)
	}

	timer := fmt.Sprintf(timerTpl, rotationInterval, rotationServiceUnit)
	if err := os.WriteFile(rotationTimerPath, []byte(timer), 0o644); err != nil {
		return permissionHint("write rotation timer unit", rotationTimerPath, err)
	}

	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := systemctl("enable", "--now", rotationTimerUnit); err != nil {
		return fmt.Errorf("enable %s: %w", rotationTimerUnit, err)
	}

	logger.Info("Rotation timer installed (interval=%s, max-size=%s, keep=%d, compress=%t)",
		rotationInterval, rotationMaxSize, rotationKeep, rotationCompress)
	logger.Info("Inspect: systemctl list-timers %s", rotationTimerUnit)
	logger.Info("Run now: sudo systemctl start %s", rotationServiceUnit)
	return nil
}

func installLogrotate() error {
	if _, err := exec.LookPath("logrotate"); err != nil {
		return fmt.Errorf("logrotate not found in PATH — install it (e.g. 'apt install logrotate') or use --mode=timer")
	}

	compressLine := ""
	if rotationCompress {
		compressLine = "compress\n    "
	}

	cfg := fmt.Sprintf(logrotateTpl, rotationMaxSize, rotationKeep, compressLine)
	if err := os.WriteFile(logrotateConfigPath, []byte(cfg), 0o644); err != nil {
		return permissionHint("write logrotate config", logrotateConfigPath, err)
	}

	logger.Info("Logrotate config installed at %s (size=%s, rotate=%d, compress=%t)",
		logrotateConfigPath, rotationMaxSize, rotationKeep, rotationCompress)
	logger.Info("Logrotate runs hourly via the system's logrotate cron/timer.")
	logger.Info("Test once: sudo logrotate -d %s    (debug, no changes)", logrotateConfigPath)
	logger.Info("Force run: sudo logrotate -f %s", logrotateConfigPath)
	return nil
}

func runRotationUninstall(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	var firstErr error

	_ = systemctl("disable", "--now", rotationTimerUnit)
	if err := os.Remove(rotationTimerPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		firstErr = err
	}
	if err := os.Remove(rotationServicePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		if firstErr == nil {
			firstErr = err
		}
	}
	_ = systemctl("daemon-reload")

	if err := os.Remove(logrotateConfigPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		if firstErr == nil {
			firstErr = err
		}
	}

	if firstErr != nil {
		return firstErr
	}
	logger.Info("Rotation uninstalled (timer units and logrotate config removed)")
	return nil
}

func runRotationStatus(cmd *cobra.Command, args []string) error {
	timer := fileExists(rotationTimerPath)
	svc := fileExists(rotationServicePath)
	lr := fileExists(logrotateConfigPath)

	fmt.Printf("Timer mode:      %s (%s, %s)\n", boolTag(timer && svc), rotationTimerPath, rotationServicePath)
	fmt.Printf("Logrotate mode:  %s (%s)\n", boolTag(lr), logrotateConfigPath)

	if timer && svc {
		fmt.Println()
		out, _ := exec.Command("systemctl", "list-timers", rotationTimerUnit, "--no-pager").CombinedOutput()
		fmt.Print(string(out))
	}
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func boolTag(b bool) string {
	if b {
		return "INSTALLED"
	}
	return "not installed"
}
