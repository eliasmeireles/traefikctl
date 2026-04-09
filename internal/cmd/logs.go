package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

const (
	traefikLogPath    = "/var/log/traefik/traefik.log"
	traefikAccessPath = "/var/log/traefik/access.log"
)

var (
	logsFollow  bool
	logsLines   int
	logsAccess  bool
	logsService bool
	logsName    string
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View Traefik logs",
	Long: `View Traefik application or access logs.

By default reads from /var/log/traefik/traefik.log.
Use --service to read from the systemd journal instead.

Examples:
  traefikctl logs
  traefikctl logs --follow
  traefikctl logs --lines 100
  traefikctl logs --access
  traefikctl logs --service --follow`,
	SilenceUsage: true,
	RunE:         runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", true, "Follow log output")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 50, "Number of lines to show")
	logsCmd.Flags().BoolVar(&logsAccess, "access", false, "Show access log instead of application log")
	logsCmd.Flags().BoolVar(&logsService, "service", false, "Read from systemd journal")
	logsCmd.Flags().StringVar(&logsName, "name", "traefikctl", "Systemd service name (used with --service)")

	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	if logsService {
		return journalctlLogs(logsName, logsFollow, logsLines)
	}

	logFile := traefikLogPath
	if logsAccess {
		logFile = traefikAccessPath
	}

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		logger.Warn("Log file not found: %s", logFile)
		logger.Info("Tip: use --service to read from the systemd journal")
		return nil
	}

	return tailFile(logFile, logsFollow, logsLines)
}

// journalctlLogs streams or displays service logs via journalctl.
func journalctlLogs(serviceName string, follow bool, lines int) error {
	args := []string{"-u", serviceName, "--no-pager", "-n", strconv.Itoa(lines)}
	if follow {
		args = append(args, "-f")
	}

	c := exec.Command("journalctl", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		return fmt.Errorf("journalctl failed: %w", err)
	}

	return nil
}

// tailFile reads the last N lines from a file, optionally following it.
func tailFile(path string, follow bool, lines int) error {
	args := []string{"-n", strconv.Itoa(lines)}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, path)

	c := exec.Command("tail", args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Run(); err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	return nil
}
