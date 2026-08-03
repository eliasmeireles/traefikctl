package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
	"github.com/eliasmeireles/traefikctl/internal/traefik"
	"github.com/eliasmeireles/traefikctl/internal/validate"
)

// setupStepNames is the single source of truth for setup's step order.
// It's a package-level value (not built inside runSetup) specifically so
// setup_test.go can assert on ordering — e.g. that "validate" always runs
// before "start" — without executing anything.
var setupStepNames = []string{
	"root-check",
	"binary",
	"user-and-directories",
	"generate-config",
	"acme",
	"service-unit",
	"validate",
	"start",
	"log-rotation",
	"summary",
}

var (
	setupYes          bool
	setupDryRun       bool
	setupVersion      string
	setupACMEEmail    string
	setupSkipRotation bool
	setupNoStart      bool
	setupUpgrade      bool
	setupForce        bool
)

var setupCmd = &cobra.Command{
	Use:     "setup",
	Aliases: []string{"bootstrap", "init"},
	Short:   "Install, configure, validate and start Traefik in one command",
	Long: `Runs every step needed to go from nothing to a working, validated,
running Traefik: install the binary, create the system user and
directories, generate a static config (never overwriting one that already
exists), install the systemd unit, validate the config, start the service
and confirm it actually stayed up, and install log rotation.

Safe to re-run: every step is idempotent and reports done/skipped. The
config is validated BEFORE the service is (re)started, so a bad
traefik.yaml is caught here instead of crash-looping afterward.`,
	SilenceUsage: true,
	RunE:         runSetup,
}

func init() {
	setupCmd.Flags().BoolVarP(&setupYes, "yes", "y", false, "Don't prompt for confirmation")
	setupCmd.Flags().BoolVar(&setupDryRun, "dry-run", false, "Print what would happen without changing anything")
	setupCmd.Flags().StringVar(&setupVersion, "version", traefik.DefaultVersion, "Traefik version to install (e.g. v3.4.0 or 'latest')")
	setupCmd.Flags().StringVar(&setupACMEEmail, "acme-email", "", "Enable Let's Encrypt ACME with this contact email")
	setupCmd.Flags().BoolVar(&setupSkipRotation, "skip-rotation", false, "Don't install log rotation")
	setupCmd.Flags().BoolVar(&setupNoStart, "no-start", false, "Do everything except starting/restarting the service")
	setupCmd.Flags().BoolVar(&setupUpgrade, "upgrade", false, "Replace an already-installed Traefik binary with --version")
	setupCmd.Flags().BoolVar(&setupForce, "force", false, "Overwrite existing config files instead of skipping them")
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	if setupDryRun {
		setupYes = true
	}

	steps := map[string]func() error{
		"root-check":           setupStepRootCheck,
		"binary":               setupStepBinary,
		"user-and-directories": setupStepUserAndDirectories,
		"generate-config":      setupStepGenerateConfig,
		"acme":                 setupStepACME,
		"service-unit":         setupStepServiceUnit,
		"validate":             setupStepValidate,
		"start":                setupStepStart,
		"log-rotation":         setupStepLogRotation,
		"summary":              setupStepSummary,
	}

	for _, name := range setupStepNames {
		fn, ok := steps[name]
		if !ok {
			continue
		}
		logger.Info("--- %s ---", name)
		if err := fn(); err != nil {
			return fmt.Errorf("setup step %q failed: %w", name, err)
		}
	}
	return nil
}

func setupStepRootCheck() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("traefikctl setup only supports Linux (this system is %s)", runtime.GOOS)
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl not found in PATH — traefikctl setup requires systemd")
	}
	if setupDryRun {
		logger.Info("(dry-run) skipping root check")
		return nil
	}
	return requireRoot()
}

func setupStepBinary() error {
	installer := traefik.NewInstaller()

	version, err := resolveTraefikVersion(installer, setupVersion)
	if err != nil {
		return err
	}

	if installer.IsInstalled() && !setupUpgrade {
		current, _ := installer.GetVersion()
		logger.Info("skipped: Traefik already installed (%s). Use --upgrade to replace it.", current)
		return nil
	}

	if setupDryRun {
		action := "install"
		if installer.IsInstalled() {
			action = "upgrade to"
		}
		logger.Info("(dry-run) would %s Traefik %s", action, version)
		return nil
	}

	if err := installer.Install(version); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	logger.Info("Traefik %s installed", version)
	return nil
}

func setupStepUserAndDirectories() error {
	if setupDryRun {
		logger.Info("(dry-run) would ensure traefik user, /etc/traefik/dynamic, /var/log/traefik and acme.json")
		return nil
	}
	return runInstallSetup(traefik.NewInstaller())
}

func setupStepGenerateConfig() error {
	if setupDryRun {
		logger.Info("(dry-run) would generate %s and an example dynamic config if missing", defaultStaticConfigPath)
		return nil
	}
	return generateConfigs(setupForce)
}

func setupStepACME() error {
	if setupACMEEmail == "" {
		logger.Info("skipped: no --acme-email given")
		return nil
	}
	if setupDryRun {
		logger.Info("(dry-run) would enable ACME for %s", setupACMEEmail)
		return nil
	}
	if err := appendACMEConfig(setupACMEEmail); err != nil {
		if isAlreadyPresentErr(err) {
			logger.Info("skipped: ACME already configured")
			return nil
		}
		return err
	}
	return nil
}

func setupStepServiceUnit() error {
	if setupDryRun {
		logger.Info("(dry-run) would write and enable the systemd unit")
		return nil
	}
	return runServiceInstall(nil, nil)
}

func setupStepValidate() error {
	if setupDryRun {
		logger.Info("(dry-run) would validate %s", defaultStaticConfigPath)
		return nil
	}

	result, err := validate.StaticFile(context.Background(), defaultStaticConfigPath, validate.Options{})
	if err != nil {
		return fmt.Errorf("failed to validate %s: %w", defaultStaticConfigPath, err)
	}
	if result.Skipped {
		logger.Warn("%s", result.SkipReason)
		return nil
	}
	if !result.OK() {
		fmt.Print(result.String())
		return fmt.Errorf("%d problem(s) found in %s — fix them before the service can start", len(result.Errors()), defaultStaticConfigPath)
	}
	logger.Info("%s: OK", defaultStaticConfigPath)
	return nil
}

func setupStepStart() error {
	if setupNoStart {
		logger.Info("skipped: --no-start")
		return nil
	}
	if setupDryRun {
		logger.Info("(dry-run) would start (or restart) and confirm the service stays up")
		return nil
	}

	name := "traefikctl"
	state, err := showUnit(name)
	action := "restart"
	if err != nil || state.ActiveState != "active" {
		action = "start"
	}

	if err := systemctl(action, name); err != nil {
		return fmt.Errorf("failed to %s service: %w", action, err)
	}

	healthy, err := waitHealthy(name, DefaultRestartWait, DefaultSettle)
	if err != nil {
		logger.Error("Service did not come up healthy: %v", err)
		logger.Info("Last journal lines:")
		_ = journalctlLogs(name, false, 50)
		return err
	}

	logger.Info("Service stable (%s/%s)", healthy.ActiveState, healthy.SubState)
	return nil
}

func setupStepLogRotation() error {
	if setupSkipRotation {
		logger.Info("skipped: --skip-rotation")
		return nil
	}
	if fileExists(rotationTimerPath) || fileExists(logrotateConfigPath) {
		logger.Info("skipped: log rotation already installed")
		return nil
	}
	if setupDryRun {
		logger.Info("(dry-run) would install the log rotation timer")
		return nil
	}

	rotationMode = "timer"
	rotationMaxSize = "100M"
	rotationKeep = 3
	rotationCompress = false
	rotationInterval = "5min"
	rotationBin = defaultBinaryPath
	return installTimer()
}

func setupStepSummary() error {
	if setupDryRun {
		logger.Info("(dry-run) done — nothing was changed")
		return nil
	}

	result := performChecks()
	printChecks(result)

	if result.failed > 0 {
		return fmt.Errorf("setup finished but %d check(s) still fail — see above", result.failed)
	}
	logger.Info("\nSetup complete.")
	return nil
}

func isAlreadyPresentErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already present")
}
