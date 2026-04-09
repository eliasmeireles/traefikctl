package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	backendAddName    string
	backendAddAddress string
	backendAddFile    string
	backendRemoveName    string
	backendRemoveAddress string
	backendRemoveFile    string
)

var resourceBackendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Manage backend servers for an existing service",
}

var resourceBackendAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a backend server to an existing service",
	Long: `Add a new backend server to an existing HTTP service for load balancing.

Example:
  traefikctl resource backend add --name my-app --address 10.0.0.5:8080`,
	SilenceUsage: true,
	RunE:         runBackendAdd,
}

var resourceBackendRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a backend server from a service",
	Long: `Remove one backend server from an existing HTTP service.
The last server cannot be removed — use 'resource remove' to delete the whole route.

Example:
  traefikctl resource backend remove --name my-app --address 10.0.0.5:8080`,
	SilenceUsage: true,
	RunE:         runBackendRemove,
}

func init() {
	resourceBackendAddCmd.Flags().StringVar(&backendAddName, "name", "", "Router name")
	resourceBackendAddCmd.Flags().StringVar(&backendAddAddress, "address", "", "Backend address (ip:port)")
	resourceBackendAddCmd.Flags().StringVar(&backendAddFile, "file", "", "Dynamic config file")
	_ = resourceBackendAddCmd.MarkFlagRequired("name")
	_ = resourceBackendAddCmd.MarkFlagRequired("address")

	resourceBackendRemoveCmd.Flags().StringVar(&backendRemoveName, "name", "", "Router name")
	resourceBackendRemoveCmd.Flags().StringVar(&backendRemoveAddress, "address", "", "Backend address to remove (ip:port)")
	resourceBackendRemoveCmd.Flags().StringVar(&backendRemoveFile, "file", "", "Dynamic config file")
	_ = resourceBackendRemoveCmd.MarkFlagRequired("name")
	_ = resourceBackendRemoveCmd.MarkFlagRequired("address")

	resourceBackendCmd.AddCommand(resourceBackendAddCmd)
	resourceBackendCmd.AddCommand(resourceBackendRemoveCmd)
	resourceCmd.AddCommand(resourceBackendCmd)
}

func runBackendAdd(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(backendAddFile)
	if err != nil {
		return err
	}

	if err := addBackendServer(backendAddName, backendAddAddress, filePath); err != nil {
		return err
	}

	logger.Info("Backend http://%s added to '%s'", backendAddAddress, backendAddName)
	logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
	return nil
}

func runBackendRemove(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(backendRemoveFile)
	if err != nil {
		return err
	}

	if err := removeBackendServer(backendRemoveName, backendRemoveAddress, filePath); err != nil {
		return err
	}

	logger.Info("Backend %s removed from '%s'", backendRemoveAddress, backendRemoveName)
	logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
	return nil
}

// addBackendServer appends a new URL to the load balancer of the service attached to routerName.
func addBackendServer(routerName, address, filePath string) error {
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}

	if cfg.HTTP == nil {
		return fmt.Errorf("router '%s' not found", routerName)
	}

	router, ok := cfg.HTTP.Routers[routerName]
	if !ok {
		return fmt.Errorf("router '%s' not found", routerName)
	}

	svc, ok := cfg.HTTP.Services[router.Service]
	if !ok || svc.LoadBalancer == nil {
		return fmt.Errorf("service '%s' not found or has no load balancer", router.Service)
	}

	newURL := fmt.Sprintf("http://%s", address)
	for _, s := range svc.LoadBalancer.Servers {
		if s.URL == newURL {
			return fmt.Errorf("backend %s already exists in service '%s'", newURL, router.Service)
		}
	}

	svc.LoadBalancer.Servers = append(svc.LoadBalancer.Servers, ServerURL{URL: newURL})
	return saveDynamicConfig(filePath, cfg)
}

// removeBackendServer removes a URL from the load balancer of the service attached to routerName.
// Returns an error if it would leave zero servers.
func removeBackendServer(routerName, address, filePath string) error {
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}

	if cfg.HTTP == nil {
		return fmt.Errorf("router '%s' not found", routerName)
	}

	router, ok := cfg.HTTP.Routers[routerName]
	if !ok {
		return fmt.Errorf("router '%s' not found", routerName)
	}

	svc, ok := cfg.HTTP.Services[router.Service]
	if !ok || svc.LoadBalancer == nil {
		return fmt.Errorf("service '%s' not found", router.Service)
	}

	target := fmt.Sprintf("http://%s", address)
	if strings.HasPrefix(address, "http") {
		target = address
	}

	filtered := make([]ServerURL, 0, len(svc.LoadBalancer.Servers))
	for _, s := range svc.LoadBalancer.Servers {
		if s.URL != target {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == len(svc.LoadBalancer.Servers) {
		return fmt.Errorf("backend %s not found in service '%s'", target, router.Service)
	}

	if len(filtered) == 0 {
		return fmt.Errorf("cannot remove the last server — use 'resource remove --name %s' to delete the whole route", routerName)
	}

	svc.LoadBalancer.Servers = filtered
	return saveDynamicConfig(filePath, cfg)
}
