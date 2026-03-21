package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	removeName string
	removeFile string
)

var resourceRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a router and its service",
	Long: `Remove a Traefik router and its associated service from a dynamic config file.

If the file has no routers/services left, it is deleted with a warning.

Examples:
  traefikctl resource remove --name my-app
  traefikctl resource remove --name postgres --file /etc/traefik/dynamic/services.yaml`,
	SilenceUsage: true,
	RunE:         runResourceRemove,
}

func init() {
	resourceRemoveCmd.Flags().StringVar(&removeName, "name", "", "Router name to remove")
	resourceRemoveCmd.Flags().StringVar(&removeFile, "file", "", "Dynamic config file (skip selection prompt)")

	_ = resourceRemoveCmd.MarkFlagRequired("name")

	resourceCmd.AddCommand(resourceRemoveCmd)
}

func runResourceRemove(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(removeFile)
	if err != nil {
		return err
	}

	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}

	found := false
	svcName := removeName + "-svc"

	// Try HTTP
	if cfg.HTTP != nil {
		if _, exists := cfg.HTTP.Routers[removeName]; exists {
			// Get service name from router before deleting
			if router := cfg.HTTP.Routers[removeName]; router != nil {
				svcName = router.Service
			}
			delete(cfg.HTTP.Routers, removeName)
			delete(cfg.HTTP.Services, svcName)
			logger.Info("Removed HTTP router '%s' and service '%s'", removeName, svcName)
			found = true
		}

		// Clean up empty HTTP section
		if len(cfg.HTTP.Routers) == 0 && len(cfg.HTTP.Services) == 0 {
			cfg.HTTP = nil
		}
	}

	// Try TCP
	if !found && cfg.TCP != nil {
		if _, exists := cfg.TCP.Routers[removeName]; exists {
			if router := cfg.TCP.Routers[removeName]; router != nil {
				svcName = router.Service
			}
			delete(cfg.TCP.Routers, removeName)
			delete(cfg.TCP.Services, svcName)
			logger.Info("Removed TCP router '%s' and service '%s'", removeName, svcName)
			found = true
		}

		if len(cfg.TCP.Routers) == 0 && len(cfg.TCP.Services) == 0 {
			cfg.TCP = nil
		}
	}

	if !found {
		return fmt.Errorf("router '%s' not found in %s", removeName, filePath)
	}

	// If config is empty, remove the file
	if cfg.HTTP == nil && cfg.TCP == nil {
		if err := os.Remove(filePath); err != nil {
			logger.Warn("Failed to remove empty file: %v", err)
		} else {
			logger.Warn("File '%s' has no routers left, file removed", filePath)
		}
		return nil
	}

	if err := saveDynamicConfig(filePath, cfg); err != nil {
		return err
	}

	logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
	return nil
}
