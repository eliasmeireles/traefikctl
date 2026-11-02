package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// routeChanges holds optional edits shared by update and copy.
type routeChanges struct {
	domains        string
	address        string
	service        string
	protocol       string
	entrypoint     string
	middlewares    []string
	passthrough    bool
	priority       int
	setPassthrough bool
	setPriority    bool
	setMiddlewares bool
}

// bindRouteChanges installs the common route editing flags.
func bindRouteChanges(cmd *cobra.Command, changes *routeChanges) {
	cmd.Flags().StringVar(&changes.address, "address", "", "Backend address (mutually exclusive with --service)")
	cmd.Flags().StringVar(&changes.service, "service", "", "Existing service name")
	cmd.Flags().StringVar(&changes.protocol, "type", "", "Expected router type: http or tcp")
	cmd.Flags().StringVar(&changes.entrypoint, "entrypoint", "", "Replacement entrypoints (comma-separated)")
	cmd.Flags().StringArrayVar(&changes.middlewares, "middleware", nil, "Replacement middleware names (repeatable or comma-separated)")
	cmd.Flags().BoolVar(&changes.passthrough, "passthrough", false, "Forward TCP TLS without termination")
	cmd.Flags().IntVar(&changes.priority, "priority", 0, "Router priority")
}

// readChangedFlags distinguishes explicit zero and false from omitted options.
func (c *routeChanges) readChangedFlags(cmd *cobra.Command) {
	c.setPassthrough = cmd.Flags().Changed("passthrough")
	c.setPriority = cmd.Flags().Changed("priority")
	c.setMiddlewares = cmd.Flags().Changed("middleware")
}

// validate checks common edit constraints before any file is written.
func (c routeChanges) validate(protocol, filePath string) error {
	if c.protocol != "" && c.protocol != protocol {
		return fmt.Errorf("router type is %s, not %s", protocol, c.protocol)
	}
	if c.address != "" && c.service != "" {
		return fmt.Errorf("--address and --service are mutually exclusive")
	}
	if c.service != "" {
		return validateExistingService(c.service, protocol, filePath)
	}
	return nil
}

// applyTCP applies router-only edits and validates the resulting TLS settings.
func (c routeChanges) applyTCP(router *TCPRouter) error {
	if c.domains != "" {
		rule, err := hostRule(c.domains, "tcp")
		if err != nil {
			return err
		}
		router.Rule = rule
		if c.domains != "*" && router.TLS == nil {
			router.TLS = &TLSConf{}
		}
	}
	if c.entrypoint != "" {
		router.EntryPoints = splitCSV(c.entrypoint)
	}
	if c.setMiddlewares {
		router.Middlewares = flattenCSV(c.middlewares)
	}
	if c.setPriority {
		router.Priority = c.priority
	}
	if c.setPassthrough {
		if router.TLS == nil && c.passthrough {
			router.TLS = &TLSConf{}
		}
		if router.TLS != nil {
			router.TLS.Passthrough = c.passthrough
		}
	}
	if router.TLS != nil && router.TLS.Passthrough {
		domains := c.domains
		if domains == "" && strings.Contains(router.Rule, "HostSNI(`") {
			domains = "*"
		}
		return validatePassthrough("tcp", domains, router.EntryPoints, true, defaultStaticConfigPath)
	}
	return nil
}

// applyHTTP applies router-only edits and rejects TCP-only flags.
func (c routeChanges) applyHTTP(router *Router) error {
	if c.passthrough {
		return fmt.Errorf("--passthrough is valid only for TCP routers")
	}
	if c.domains != "" {
		rule, err := hostRule(c.domains, "http")
		if err != nil {
			return err
		}
		router.Rule = rule
	}
	if c.entrypoint != "" {
		router.EntryPoints = splitCSV(c.entrypoint)
	}
	if c.setMiddlewares {
		router.Middlewares = flattenCSV(c.middlewares)
	}
	if c.setPriority {
		router.Priority = c.priority
	}
	return nil
}
