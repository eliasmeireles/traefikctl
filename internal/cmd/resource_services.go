package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// neighboringConfigs reads the dynamic provider files beside the selected file.
func neighboringConfigs(filePath string) ([]*DynamicConfig, error) {
	entries, err := os.ReadDir(filepath.Dir(filePath))
	if err != nil {
		return nil, err
	}
	var configs []*DynamicConfig
	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if entry.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		cfg, err := loadDynamicConfig(filepath.Join(filepath.Dir(filePath), entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("cannot inspect %s: %w", entry.Name(), err)
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

// validateExistingService verifies references in the matching protocol namespace.
func validateExistingService(name, protocol, filePath string) error {
	if err := validateResourceName(name); err != nil {
		return err
	}
	configs, err := neighboringConfigs(filePath)
	if err != nil {
		return err
	}
	for _, cfg := range configs {
		if protocol == "http" && cfg.HTTP != nil && cfg.HTTP.Services[name] != nil {
			return nil
		}
		if protocol == "tcp" && cfg.TCP != nil && cfg.TCP.Services[name] != nil {
			return nil
		}
	}
	return fmt.Errorf("%s service %q not found in dynamic files in %s", protocol, name, filepath.Dir(filePath))
}

// resourceServiceName chooses an existing service or the generated local service.
func resourceServiceName(name, address, existing, protocol, filePath string) (string, error) {
	if (address == "") == (existing == "") {
		return "", fmt.Errorf("specify exactly one of --address or --service (mutually exclusive)")
	}
	if existing != "" {
		if err := validateExistingService(existing, protocol, filePath); err != nil {
			return "", err
		}
		return existing, nil
	}
	return name + "-svc", nil
}
