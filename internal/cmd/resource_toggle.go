package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

const disabledDir = "/etc/traefikctl/disabled"

var (
	enableName  string
	disableName string
	disableFile string
)

var resourceEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Re-enable a previously disabled router",
	Long: `Restore a disabled router back to the active dynamic config file.

Example:
  traefikctl resource enable --name my-app`,
	SilenceUsage: true,
	RunE:         runResourceEnable,
}

var resourceDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Temporarily disable a router without removing it",
	Long: `Remove a router from active config and save it to /etc/traefikctl/disabled/.
Restore it with: traefikctl resource enable --name <name>

Example:
  traefikctl resource disable --name my-app`,
	SilenceUsage: true,
	RunE:         runResourceDisable,
}

func init() {
	resourceEnableCmd.Flags().StringVar(&enableName, "name", "", "Router name")
	resourceDisableCmd.Flags().StringVar(&disableName, "name", "", "Router name")
	resourceDisableCmd.Flags().StringVar(&disableFile, "file", "", "Dynamic config file (skip selection prompt)")

	_ = resourceEnableCmd.MarkFlagRequired("name")
	_ = resourceDisableCmd.MarkFlagRequired("name")

	resourceCmd.AddCommand(resourceEnableCmd)
	resourceCmd.AddCommand(resourceDisableCmd)
}

func runResourceDisable(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(disableFile)
	if err != nil {
		return err
	}

	if err := disableRouter(disableName, filePath, disabledDir); err != nil {
		return err
	}

	logger.Info("Router '%s' disabled (saved to %s/%s.yaml)", disableName, disabledDir, disableName)
	logger.Info("Re-enable with: traefikctl resource enable --name %s", disableName)
	return nil
}

func runResourceEnable(cmd *cobra.Command, args []string) error {
	files, err := listDynamicFiles()
	if err != nil {
		return err
	}

	var targetFile string
	if len(files) == 1 {
		targetFile = files[0]
	} else if len(files) > 1 {
		targetFile, err = selectDynamicFile("")
		if err != nil {
			return err
		}
	} else {
		// No dynamic files exist yet; write the restored router to the default location.
		targetFile = filepath.Join(defaultDynamicDir, "services.yaml")
	}

	if err := enableRouter(enableName, targetFile, disabledDir); err != nil {
		return err
	}

	logger.Info("Router '%s' re-enabled in %s", enableName, targetFile)
	return nil
}

// disableRouter moves a router+service from activeFile into dDir/<name>.yaml.
func disableRouter(name, activeFile, dDir string) error {
	cfg, err := loadDynamicConfig(activeFile)
	if err != nil {
		return err
	}

	snapshot := &DynamicConfig{}
	found := false

	if cfg.HTTP != nil {
		if _, ok := cfg.HTTP.Routers[name]; ok {
			snapshot.HTTP = &HTTPConfig{
				Routers:  map[string]*Router{name: cfg.HTTP.Routers[name]},
				Services: map[string]*Service{},
			}
			svcName := cfg.HTTP.Routers[name].Service
			if svc, ok := cfg.HTTP.Services[svcName]; ok {
				snapshot.HTTP.Services[svcName] = svc
				delete(cfg.HTTP.Services, svcName)
			}
			delete(cfg.HTTP.Routers, name)
			if len(cfg.HTTP.Routers) == 0 && len(cfg.HTTP.Services) == 0 {
				cfg.HTTP = nil
			}
			found = true
		}
	}

	if !found && cfg.TCP != nil {
		if _, ok := cfg.TCP.Routers[name]; ok {
			snapshot.TCP = &TCPConfig{
				Routers:  map[string]*TCPRouter{name: cfg.TCP.Routers[name]},
				Services: map[string]*TCPService{},
			}
			svcName := cfg.TCP.Routers[name].Service
			if svc, ok := cfg.TCP.Services[svcName]; ok {
				snapshot.TCP.Services[svcName] = svc
				delete(cfg.TCP.Services, svcName)
			}
			delete(cfg.TCP.Routers, name)
			if len(cfg.TCP.Routers) == 0 && len(cfg.TCP.Services) == 0 {
				cfg.TCP = nil
			}
			found = true
		}
	}

	if !found {
		return fmt.Errorf("router '%s' not found", name)
	}

	if err := os.MkdirAll(dDir, 0755); err != nil {
		return fmt.Errorf("failed to create disabled dir: %w", err)
	}

	disabledPath := filepath.Join(dDir, name+".yaml")
	if err := saveDynamicConfig(disabledPath, snapshot); err != nil {
		return err
	}

	if cfg.HTTP == nil && cfg.TCP == nil {
		return os.Remove(activeFile)
	}

	return saveDynamicConfig(activeFile, cfg)
}

// enableRouter restores a disabled router from dDir/<name>.yaml into targetFile.
func enableRouter(name, targetFile, dDir string) error {
	disabledPath := filepath.Join(dDir, name+".yaml")

	snapshot, err := loadDynamicConfig(disabledPath)
	if err != nil {
		return fmt.Errorf("disabled snapshot not found for '%s': %w", name, err)
	}

	cfg, err := loadDynamicConfig(targetFile)
	if err != nil {
		return err
	}

	if snapshot.HTTP != nil {
		if cfg.HTTP == nil {
			cfg.HTTP = &HTTPConfig{Routers: map[string]*Router{}, Services: map[string]*Service{}}
		}
		for k, v := range snapshot.HTTP.Routers {
			cfg.HTTP.Routers[k] = v
		}
		for k, v := range snapshot.HTTP.Services {
			cfg.HTTP.Services[k] = v
		}
	}

	if snapshot.TCP != nil {
		if cfg.TCP == nil {
			cfg.TCP = &TCPConfig{Routers: map[string]*TCPRouter{}, Services: map[string]*TCPService{}}
		}
		for k, v := range snapshot.TCP.Routers {
			cfg.TCP.Routers[k] = v
		}
		for k, v := range snapshot.TCP.Services {
			cfg.TCP.Services[k] = v
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	if err := saveDynamicConfig(targetFile, cfg); err != nil {
		return err
	}

	return os.Remove(disabledPath)
}
