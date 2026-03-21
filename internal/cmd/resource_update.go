package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	updateName    string
	updateAddress string
	updateDomain  string
	updateFile    string
)

var resourceUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a router or service",
	Long: `Update a Traefik router's domain or service's address.

The router is identified by --name.

Examples:
  traefikctl resource update --name my-app --address 10.8.0.5:8080
  traefikctl resource update --name my-app --domain new.example.com
  traefikctl resource update --name my-app --address 10.8.0.5:8080 --domain new.example.com`,
	SilenceUsage: true,
	RunE:         runResourceUpdate,
}

func init() {
	resourceUpdateCmd.Flags().StringVar(&updateName, "name", "", "Router name to update")
	resourceUpdateCmd.Flags().StringVar(&updateAddress, "address", "", "New server address (ip:port)")
	resourceUpdateCmd.Flags().StringVar(&updateDomain, "domain", "", "New domain for the router rule")
	resourceUpdateCmd.Flags().StringVar(&updateFile, "file", "", "Dynamic config file (skip selection prompt)")

	_ = resourceUpdateCmd.MarkFlagRequired("name")

	resourceCmd.AddCommand(resourceUpdateCmd)
}

func runResourceUpdate(cmd *cobra.Command, args []string) error {
	if updateAddress == "" && updateDomain == "" {
		return fmt.Errorf("must specify --address or --domain (or both)")
	}

	filePath, err := selectDynamicFile(updateFile)
	if err != nil {
		return err
	}

	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}

	// Try HTTP
	if cfg.HTTP != nil {
		if router, exists := cfg.HTTP.Routers[updateName]; exists {
			if updateDomain != "" {
				old := router.Rule
				router.Rule = fmt.Sprintf("Host(`%s`)", updateDomain)
				logger.Info("Updated router '%s' rule: %s -> %s", updateName, old, router.Rule)
			}

			if updateAddress != "" {
				svcName := router.Service
				if svc, svcExists := cfg.HTTP.Services[svcName]; svcExists && svc.LoadBalancer != nil {
					if len(svc.LoadBalancer.Servers) > 0 {
						old := svc.LoadBalancer.Servers[0].URL
						svc.LoadBalancer.Servers[0].URL = fmt.Sprintf("http://%s", updateAddress)
						logger.Info("Updated service '%s' address: %s -> http://%s", svcName, old, updateAddress)
					}
				}
			}

			if err := saveDynamicConfig(filePath, cfg); err != nil {
				return err
			}

			logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
			return nil
		}
	}

	// Try TCP
	if cfg.TCP != nil {
		if _, exists := cfg.TCP.Routers[updateName]; exists {
			if updateDomain != "" {
				logger.Warn("TCP routers use HostSNI, --domain is not applicable for TCP")
			}

			if updateAddress != "" {
				router := cfg.TCP.Routers[updateName]
				svcName := router.Service
				if svc, svcExists := cfg.TCP.Services[svcName]; svcExists && svc.LoadBalancer != nil {
					if len(svc.LoadBalancer.Servers) > 0 {
						old := svc.LoadBalancer.Servers[0].Address
						svc.LoadBalancer.Servers[0].Address = updateAddress
						logger.Info("Updated TCP service '%s' address: %s -> %s", svcName, old, updateAddress)
					}
				}
			}

			if err := saveDynamicConfig(filePath, cfg); err != nil {
				return err
			}

			logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
			return nil
		}
	}

	return fmt.Errorf("router '%s' not found in %s", updateName, filePath)
}
