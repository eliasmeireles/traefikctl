package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
	"github.com/eliasmeireles/traefikctl/internal/validate"
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

	healthy := printServiceState(statusServiceName)
	configOK := printConfigState()
	printTraefikVersion()
	printRoutesSummary()

	if !healthy || !configOK {
		return fmt.Errorf("one or more problems found above")
	}
	return nil
}

// printServiceState reports the unit's real state — distinguishing a
// crash-loop from healthy — instead of a single systemctl is-active poll,
// which can read "active" at the exact instant between two crashes on a
// Restart=always unit.
func printServiceState(name string) bool {
	s, err := showUnit(name)
	if err != nil {
		fmt.Printf("  Service %-20s  unknown (%v)\n", name, err)
		return false
	}

	if isCrashLooping(s) {
		fmt.Printf("  Service %-20s  CRASH-LOOPING (%s/%s, %d restarts, last exit %d)\n",
			name, s.ActiveState, s.SubState, s.NRestarts, s.ExecMainStatus)
		fmt.Printf("      -> sudo traefikctl validate\n")
		fmt.Printf("      -> traefikctl logs --service\n")
		return false
	}

	if s.ActiveState != "active" {
		fmt.Printf("  Service %-20s  %s/%s\n", name, s.ActiveState, s.SubState)
		return false
	}

	fmt.Printf("  Service %-20s  %s/%s\n", name, s.ActiveState, s.SubState)
	return true
}

// printConfigState runs the same validator used by `traefikctl validate`
// and `check`, so status never reports "active" over a config that's one
// restart away from crash-looping.
func printConfigState() bool {
	result, err := validate.StaticFile(context.Background(), defaultStaticConfigPath, validate.Options{})
	if err != nil {
		fmt.Printf("  Config  could not be validated: %v\n", err)
		return false
	}
	if result.Skipped {
		fmt.Printf("  Config  %s\n", result.SkipReason)
		return true
	}
	if !result.OK() {
		fmt.Printf("  Config  %d problem(s) — run 'traefikctl validate' for details\n", len(result.Errors()))
		return false
	}
	fmt.Printf("  Config  OK\n")
	return true
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
