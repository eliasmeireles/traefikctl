package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var resourceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured routers and services",
	Long: `List all Traefik routers and their backend services from every dynamic config file.

Example:
  traefikctl resource list`,
	SilenceUsage: true,
	RunE:         runResourceList,
}

func init() {
	resourceCmd.AddCommand(resourceListCmd)
}

func runResourceList(cmd *cobra.Command, args []string) error {
	files, err := listDynamicFiles()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		logger.Info("No dynamic config files found in %s", defaultDynamicDir)
		return nil
	}

	total := 0

	for _, filePath := range files {
		cfg, err := loadDynamicConfig(filePath)
		if err != nil {
			logger.Warn("Skipping %s: %v", filePath, err)
			continue
		}

		fmt.Printf("\n%s\n%s\n", filePath, strings.Repeat("-", len(filePath)))

		if cfg.HTTP != nil && len(cfg.HTTP.Routers) > 0 {
			for _, name := range sortedRouterKeys(cfg.HTTP.Routers) {
				router := cfg.HTTP.Routers[name]
				backend := backendURL(cfg.HTTP.Services[router.Service])
				fmt.Printf("  [HTTP] %-25s  %-50s  -> %s\n", name, router.Rule, backend)
				total++
			}
		}

		if cfg.TCP != nil && len(cfg.TCP.Routers) > 0 {
			for _, name := range sortedTCPRouterKeys(cfg.TCP.Routers) {
				router := cfg.TCP.Routers[name]
				backend := backendAddress(cfg.TCP.Services[router.Service])
				fmt.Printf("  [TCP]  %-25s  %-50s  -> %s\n", name, router.Rule, backend)
				total++
			}
		}
	}

	fmt.Println()
	logger.Info("Total routes: %d", total)

	return nil
}

func backendURL(svc *Service) string {
	if svc == nil || svc.LoadBalancer == nil || len(svc.LoadBalancer.Servers) == 0 {
		return "<no backend>"
	}

	urls := make([]string, len(svc.LoadBalancer.Servers))
	for i, s := range svc.LoadBalancer.Servers {
		urls[i] = s.URL
	}

	return strings.Join(urls, ", ")
}

func backendAddress(svc *TCPService) string {
	if svc == nil || svc.LoadBalancer == nil || len(svc.LoadBalancer.Servers) == 0 {
		return "<no backend>"
	}

	addrs := make([]string, len(svc.LoadBalancer.Servers))
	for i, s := range svc.LoadBalancer.Servers {
		addrs[i] = s.Address
	}

	return strings.Join(addrs, ", ")
}

func sortedRouterKeys(m map[string]*Router) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func sortedTCPRouterKeys(m map[string]*TCPRouter) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}
