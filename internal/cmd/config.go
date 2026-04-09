package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
	"github.com/eliasmeireles/traefikctl/internal/traefik"
)

var (
	cfgGenerate  bool
	cfgView      bool
	cfgForce     bool
	cfgClean     bool
	cfgACME      bool
	cfgACMEEmail string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Traefik configuration",
	Long: `Generate or view Traefik configuration files.

  --generate  Create static config (/etc/traefik/traefik.yaml) and example
              dynamic config (/etc/traefik/dynamic/example.yaml).
              Existing files are never overwritten unless --force is used.

  --view      Show all current configurations.

  --clean     Used with --view to strip comments and empty lines.

  --force     Used with --generate to overwrite existing files.`,
	SilenceUsage: true,
	RunE:         runConfig,
}

func init() {
	configCmd.Flags().BoolVar(&cfgGenerate, "generate", false, "Generate Traefik config files")
	configCmd.Flags().BoolVar(&cfgView, "view", false, "View current configurations")
	configCmd.Flags().BoolVar(&cfgClean, "clean", false, "Strip comments and empty lines (use with --view)")
	configCmd.Flags().BoolVar(&cfgForce, "force", false, "Overwrite existing files (use with --generate)")
	configCmd.Flags().BoolVar(&cfgACME, "acme", false, "Append Let's Encrypt ACME config to traefik.yaml")
	configCmd.Flags().StringVar(&cfgACMEEmail, "acme-email", "", "Email for Let's Encrypt (required with --acme)")
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	if cfgACME {
		return appendACMEConfig(cfgACMEEmail)
	}

	if cfgGenerate && cfgView {
		return fmt.Errorf("cannot use --generate and --view together")
	}

	if !cfgGenerate && !cfgView {
		return fmt.Errorf("must specify either --generate or --view")
	}

	if cfgGenerate {
		return generateConfigs()
	}

	return viewConfigs()
}

func generateConfigs() error {
	staticPath := "/etc/traefik/traefik.yaml"
	dynamicDir := "/etc/traefik/dynamic"
	examplePath := filepath.Join(dynamicDir, "example.yaml")

	// Create directories
	if err := os.MkdirAll(dynamicDir, 0755); err != nil {
		return permissionHint("create config directory", dynamicDir, err)
	}

	// Write static config
	if err := writeConfigFile(staticPath, traefik.DefaultStaticConfig, "Static config"); err != nil {
		return err
	}

	// Write example dynamic config
	if err := writeConfigFile(examplePath, traefik.DefaultDynamicExample, "Example dynamic config"); err != nil {
		return err
	}

	logger.Info("\nNext steps:")
	logger.Info("1. Edit /etc/traefik/traefik.yaml to customize entrypoints")
	logger.Info("2. Add dynamic configs to /etc/traefik/dynamic/")
	logger.Info("3. Start the service: sudo systemctl start traefikctl")

	return nil
}

func writeConfigFile(path, content, desc string) error {
	_, err := os.Stat(path)
	exists := err == nil

	if exists && !cfgForce {
		logger.Info("%s already exists: %s (skipped, use --force to overwrite)", desc, path)
		return nil
	}

	if exists && cfgForce {
		logger.Info("Overwriting %s: %s", desc, path)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return permissionHint("write "+desc, path, err)
	}

	logger.Info("%s generated: %s", desc, path)
	return nil
}

func viewConfigs() error {
	staticPath := "/etc/traefik/traefik.yaml"
	dynamicDir := "/etc/traefik/dynamic"

	fmt.Printf("=== Static Config: %s ===\n\n", staticPath)
	if err := printConfigFile(staticPath, cfgClean); err != nil {
		fmt.Printf("  (not found: %s)\n", staticPath)
	}

	fmt.Printf("\n=== Dynamic Configs: %s ===\n", dynamicDir)

	entries, err := os.ReadDir(dynamicDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("  (directory not found: %s)\n", dynamicDir)
		} else {
			fmt.Printf("  (cannot read: %v)\n", err)
		}
		return nil
	}

	found := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		found = true
		filePath := filepath.Join(dynamicDir, entry.Name())
		fmt.Printf("\n--- %s ---\n\n", filePath)
		if err := printConfigFile(filePath, cfgClean); err != nil {
			fmt.Printf("  (cannot read: %v)\n", err)
		}
	}

	if !found {
		fmt.Println("  (no dynamic config files found)")
	}

	return nil
}

func printConfigFile(path string, clean bool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if !clean {
		fmt.Print(string(content))
		return nil
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	hasContent := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if idx := strings.Index(line, " #"); idx != -1 {
			line = strings.TrimRight(line[:idx], " ")
		}

		fmt.Println(line)
		hasContent = true
	}

	if !hasContent {
		fmt.Println("  (no active configuration)")
	}

	return scanner.Err()
}

// appendACMEConfig appends a Let's Encrypt ACME resolver block to the static Traefik config
// and ensures the acme.json storage file exists.
func appendACMEConfig(email string) error {
	if email == "" {
		return fmt.Errorf("--acme-email is required with --acme")
	}

	staticPath := "/etc/traefik/traefik.yaml"
	acmePath := "/etc/traefik/acme.json"

	f, err := os.OpenFile(staticPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return permissionHint("append ACME config to", staticPath, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(fmt.Sprintf(traefik.DefaultACMEConfig, email)); err != nil {
		return fmt.Errorf("failed to write ACME config: %w", err)
	}

	if _, statErr := os.Stat(acmePath); os.IsNotExist(statErr) {
		if createErr := os.WriteFile(acmePath, []byte("{}"), 0600); createErr != nil {
			logger.Warn("Failed to create %s: %v", acmePath, createErr)
		} else {
			logger.Info("Created %s (Traefik will write certificates here)", acmePath)
		}
	}

	logger.Info("ACME config appended to %s", staticPath)
	logger.Info("Restart Traefik: sudo traefikctl service restart")
	return nil
}

func permissionHint(action, path string, err error) error {
	if os.IsPermission(err) {
		return fmt.Errorf("permission denied: cannot %s at %s\n  Run with sudo: sudo traefikctl config --generate", action, path)
	}
	return fmt.Errorf("failed to %s: %w", action, err)
}
