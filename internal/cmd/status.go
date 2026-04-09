package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var statusServiceName string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a full system status overview",
	Long: `Display service state, installed Traefik version, and route count summary.

Example:
  traefikctl status`,
	SilenceUsage: true,
	RunE:         runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusServiceName, "name", "traefikctl", "Systemd service name")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("=== traefikctl Status ===")
	fmt.Println()

	printServiceState(statusServiceName)
	printTraefikVersion()
	printRoutesSummary()

	return nil
}

func printServiceState(name string) {
	out, err := exec.Command("systemctl", "is-active", name).Output()
	state := strings.TrimSpace(string(out))
	if err != nil || state != "active" {
		fmt.Printf("  Service %-20s  %s\n", name, state)
	} else {
		fmt.Printf("  Service %-20s  %s\n", name, state)
	}
}

func printTraefikVersion() {
	out, err := exec.Command("traefik", "version").Output()
	if err != nil {
		logger.Warn("Traefik binary not found or not in PATH")
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		fmt.Printf("  %s\n", lines[0])
	}
}

func printRoutesSummary() {
	files, err := listDynamicFiles()
	if err != nil || len(files) == 0 {
		logger.Warn("No dynamic config files found in %s", defaultDynamicDir)
		return
	}

	http, tcp := countRoutes(files)
	fmt.Printf("  Routes: %d HTTP, %d TCP  (across %d file(s))\n", http, tcp, len(files))
}

// countRoutes returns the total number of HTTP and TCP routers across the given config files.
func countRoutes(files []string) (httpCount, tcpCount int) {
	for _, f := range files {
		cfg, err := loadDynamicConfig(f)
		if err != nil {
			continue
		}
		if cfg.HTTP != nil {
			httpCount += len(cfg.HTTP.Routers)
		}
		if cfg.TCP != nil {
			tcpCount += len(cfg.TCP.Routers)
		}
	}
	return
}
