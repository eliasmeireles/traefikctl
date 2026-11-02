package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	updateName    string
	updateDomain  string
	updateFile    string
	updateChanges routeChanges
)

var resourceUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update a router or service",
	SilenceUsage: true,
	RunE:         runResourceUpdate,
}

func init() {
	resourceUpdateCmd.Flags().StringVar(&updateName, "name", "", "Router name to update")
	resourceUpdateCmd.Flags().Var(domainFlag{&updateDomain}, "domain", "Replacement hostnames (repeatable or comma-separated)")
	resourceUpdateCmd.Flags().StringVar(&updateFile, "file", "", "Dynamic config file")
	bindRouteChanges(resourceUpdateCmd, &updateChanges)
	_ = resourceUpdateCmd.MarkFlagRequired("name")
	resourceCmd.AddCommand(resourceUpdateCmd)
}

func runResourceUpdate(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}
	updateChanges.domains = updateDomain
	updateChanges.readChangedFlags(cmd)
	if updateChanges.address == "" && updateDomain == "" && updateChanges.service == "" && updateChanges.entrypoint == "" && !updateChanges.setPriority && !updateChanges.setMiddlewares && !updateChanges.setPassthrough {
		return fmt.Errorf("specify at least one route option to update")
	}
	filePath, err := selectDynamicFile(updateFile)
	if err != nil {
		return err
	}
	if err := updateRouter(updateName, filePath, updateChanges); err != nil {
		return err
	}
	logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
	return nil
}

// updateRouter changes a router and optionally its local backend or service reference.
func updateRouter(name, filePath string, changes routeChanges) error {
	if err := validateResourceName(name); err != nil {
		return err
	}
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}
	switch {
	case cfg.HTTP != nil && cfg.HTTP.Routers[name] != nil && changes.protocol != "tcp":
		if err := changes.validate("http", filePath); err != nil {
			return err
		}
		router := cfg.HTTP.Routers[name]
		if err := changes.applyHTTP(router); err != nil {
			return err
		}
		if changes.service != "" {
			router.Service = changes.service
		}
		if changes.address != "" {
			svc := cfg.HTTP.Services[router.Service]
			if svc == nil || svc.LoadBalancer == nil || len(svc.LoadBalancer.Servers) == 0 {
				return fmt.Errorf("service %q has no local backend; update its owning file or use --service", router.Service)
			}
			svc.LoadBalancer.Servers[0].URL = "http://" + changes.address
		}
	case cfg.TCP != nil && cfg.TCP.Routers[name] != nil:
		if err := changes.validate("tcp", filePath); err != nil {
			return err
		}
		router := cfg.TCP.Routers[name]
		if err := changes.applyTCP(router); err != nil {
			return err
		}
		if changes.service != "" {
			router.Service = changes.service
		}
		if changes.address != "" {
			svc := cfg.TCP.Services[router.Service]
			if svc == nil || svc.LoadBalancer == nil || len(svc.LoadBalancer.Servers) == 0 {
				return fmt.Errorf("service %q has no local backend; update its owning file or use --service", router.Service)
			}
			svc.LoadBalancer.Servers[0].Address = changes.address
		}
	default:
		return fmt.Errorf("router '%s' not found in %s", name, filePath)
	}
	return saveDynamicConfig(filePath, cfg)
}
