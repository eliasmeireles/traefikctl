package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check system configuration and permissions",
	Long: `Validate that all required directories, files, and permissions are
correctly configured for Traefik to run properly.`,
	Run: runCheck,
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

type checkResult struct {
	passed  int
	warned  int
	failed  int
	results []string
}

func (c *checkResult) pass(msg string) {
	c.passed++
	c.results = append(c.results, fmt.Sprintf("  [OK] %s", msg))
}

func (c *checkResult) warn(msg string) {
	c.warned++
	c.results = append(c.results, fmt.Sprintf("  [WARN] %s", msg))
}

func (c *checkResult) fail(msg, fix string) {
	c.failed++
	c.results = append(c.results, fmt.Sprintf("  [FAIL] %s", msg))
	c.results = append(c.results, fmt.Sprintf("         Fix: %s", fix))
}

func runCheck(cmd *cobra.Command, args []string) {
	result := &checkResult{}

	logger.Info("=== traefikctl System Check ===\n")

	checkTraefikInstalled(result)
	checkTraefikUser(result)
	checkDirectoriesExist(result)
	checkStaticConfig(result)
	checkServiceFileExists(result)

	fmt.Println()
	for _, line := range result.results {
		fmt.Println(line)
	}

	fmt.Println()
	logger.Info("=== Summary ===")
	logger.Info("Passed: %d | Warnings: %d | Failed: %d", result.passed, result.warned, result.failed)

	if result.failed > 0 {
		fmt.Println()
		logger.Error("Some checks failed. Fix the issues above and run 'traefikctl check' again.")
		os.Exit(1)
	}
}

func checkTraefikInstalled(r *checkResult) {
	if _, err := exec.LookPath("traefik"); err == nil {
		cmd := exec.Command("traefik", "version")
		if output, err := cmd.CombinedOutput(); err == nil {
			firstLine := strings.Split(strings.TrimSpace(string(output)), "\n")[0]
			r.pass(fmt.Sprintf("Traefik installed: %s", firstLine))
		} else {
			r.pass("Traefik installed")
		}
	} else {
		r.fail("Traefik is not installed", "sudo traefikctl install")
	}
}

func checkTraefikUser(r *checkResult) {
	if _, err := user.Lookup("traefik"); err == nil {
		r.pass("Traefik user 'traefik' exists")
	} else {
		r.fail(
			"Traefik user 'traefik' does not exist",
			"sudo useradd --system --no-create-home --shell /usr/sbin/nologin traefik",
		)
	}
}

func checkDirectoriesExist(r *checkResult) {
	dirs := []struct {
		path string
		desc string
	}{
		{"/etc/traefik", "Traefik config directory"},
		{"/etc/traefik/dynamic", "Dynamic config directory"},
		{"/var/log/traefik", "Log directory"},
	}

	for _, d := range dirs {
		info, err := os.Stat(d.path)
		if err != nil {
			r.fail(
				fmt.Sprintf("%s does not exist: %s", d.desc, d.path),
				fmt.Sprintf("sudo mkdir -p %s && sudo chown traefik:traefik %s", d.path, d.path),
			)
			continue
		}

		if !info.IsDir() {
			r.fail(
				fmt.Sprintf("%s is not a directory: %s", d.desc, d.path),
				fmt.Sprintf("sudo rm %s && sudo mkdir -p %s", d.path, d.path),
			)
			continue
		}

		// Check writable
		testFile := filepath.Join(d.path, ".traefikctl-check")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			if os.IsPermission(err) {
				r.warn(fmt.Sprintf("%s is not writable: %s (may need sudo)", d.desc, d.path))
			} else if strings.Contains(err.Error(), "read-only") {
				r.fail(
					fmt.Sprintf("%s is read-only: %s", d.desc, d.path),
					fmt.Sprintf("Check systemd ReadWritePaths includes %s", d.path),
				)
			} else {
				r.pass(fmt.Sprintf("%s exists: %s", d.desc, d.path))
			}
		} else {
			_ = os.Remove(testFile)
			r.pass(fmt.Sprintf("%s is writable: %s", d.desc, d.path))
		}
	}
}

func checkStaticConfig(r *checkResult) {
	staticPath := "/etc/traefik/traefik.yaml"
	if _, err := os.Stat(staticPath); err != nil {
		r.fail(
			"Static config not found",
			"sudo traefikctl config --generate",
		)
	} else {
		r.pass(fmt.Sprintf("Static config exists: %s", staticPath))
	}
}

func checkServiceFileExists(r *checkResult) {
	servicePath := "/etc/systemd/system/traefikctl.service"
	if _, err := os.Stat(servicePath); err != nil {
		r.fail(
			"Systemd service not found",
			"sudo traefikctl service install",
		)
	} else {
		r.pass(fmt.Sprintf("Service file exists: %s", servicePath))
	}
}
