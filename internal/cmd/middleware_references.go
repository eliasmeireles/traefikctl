package cmd

import (
	"fmt"
	"sort"
	"strings"
)

// removeMiddlewareForProtocol refuses to remove middleware referenced by routers.
func removeMiddlewareForProtocol(name, protocol, filePath string) error {
	if err := validateResourceName(name); err != nil {
		return err
	}
	if protocol != "" && protocol != "http" && protocol != "tcp" {
		return fmt.Errorf("invalid --protocol %q: expected http or tcp", protocol)
	}
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}
	httpFound := cfg.HTTP != nil && cfg.HTTP.Middlewares[name] != nil
	tcpFound := cfg.TCP != nil && cfg.TCP.Middlewares[name] != nil
	if protocol == "" {
		if httpFound && tcpFound {
			return fmt.Errorf("middleware %q exists in HTTP and TCP; specify --protocol", name)
		}
		protocol = "http"
		if tcpFound {
			protocol = "tcp"
		}
	}
	if (protocol == "http" && !httpFound) || (protocol == "tcp" && !tcpFound) {
		return fmt.Errorf("%s middleware '%s' not found", protocol, name)
	}
	references, err := middlewareReferences(name, protocol, filePath)
	if err != nil {
		return err
	}
	if len(references) > 0 {
		return fmt.Errorf("middleware %q is referenced by routers: %s", name, strings.Join(references, ", "))
	}
	if protocol == "tcp" {
		delete(cfg.TCP.Middlewares, name)
	} else {
		delete(cfg.HTTP.Middlewares, name)
	}
	return saveDynamicConfig(filePath, cfg)
}

// middlewareReferences includes file-provider references from neighboring files.
func middlewareReferences(name, protocol, filePath string) ([]string, error) {
	configs, err := neighboringConfigs(filePath)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, cfg := range configs {
		if protocol == "http" && cfg.HTTP != nil {
			for routerName, router := range cfg.HTTP.Routers {
				if referencesMiddleware(router.Middlewares, name) {
					names = append(names, routerName)
				}
			}
		}
		if protocol == "tcp" && cfg.TCP != nil {
			for routerName, router := range cfg.TCP.Routers {
				if referencesMiddleware(router.Middlewares, name) {
					names = append(names, routerName)
				}
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

// referencesMiddleware matches local and explicit file-provider middleware names.
func referencesMiddleware(middlewares []string, name string) bool {
	for _, reference := range middlewares {
		if reference == name || reference == name+"@file" {
			return true
		}
	}
	return false
}
