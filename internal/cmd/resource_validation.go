package cmd

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var resourceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
var hostnameLabelPattern = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// domainFlag accepts repeated flags and comma-separated hostnames.
type domainFlag struct{ value *string }

func (f domainFlag) String() string { return *f.value }
func (f domainFlag) Type() string   { return "domains" }
func (f domainFlag) Set(value string) error {
	if *f.value != "" {
		*f.value += ","
	}
	*f.value += value
	return nil
}

// validateResourceName rejects empty names and provider-qualified references.
func validateResourceName(name string) error {
	if !resourceNamePattern.MatchString(name) {
		return fmt.Errorf("invalid name %q: use letters, digits, dots, underscores or hyphens, starting with a letter or digit", name)
	}
	return nil
}

// hostRule builds a validated Host or HostSNI disjunction.
func hostRule(domains, protocol string) (string, error) {
	if domains == "" {
		if protocol == "tcp" {
			return "HostSNI(`*`)", nil
		}
		return "PathPrefix(`/`)", nil
	}
	matcher := "Host"
	if protocol == "tcp" {
		matcher = "HostSNI"
	}
	var rules []string
	for _, raw := range strings.Split(domains, ",") {
		host := strings.TrimSpace(raw)
		if host == "*" && protocol == "tcp" {
			rules = append(rules, "HostSNI(`*`)")
			continue
		}
		if len(host) == 0 || len(host) > 253 || net.ParseIP(host) != nil {
			return "", fmt.Errorf("invalid domain %q: expected a hostname without scheme, port or path", host)
		}
		for _, label := range strings.Split(host, ".") {
			if !hostnameLabelPattern.MatchString(label) {
				return "", fmt.Errorf("invalid domain %q: expected a hostname without scheme, port or path", host)
			}
		}
		rules = append(rules, fmt.Sprintf("%s(`%s`)", matcher, host))
	}
	return strings.Join(rules, " || "), nil
}

// validatePassthrough requires explicit SNI and an entrypoint configured for TLS.
// websecure is the conventional TLS entrypoint when no static config exists.
func validatePassthrough(protocol, domains string, entrypoints []string, enabled bool, staticPath string) error {
	if !enabled {
		return nil
	}
	if protocol != "tcp" {
		return fmt.Errorf("--passthrough is valid only for TCP routers")
	}
	if domains == "" {
		return fmt.Errorf("--passthrough requires --domain with a hostname or '*'")
	}
	if _, err := hostRule(domains, "tcp"); err != nil {
		return err
	}
	var cfg struct {
		EntryPoints map[string]struct {
			Address string `yaml:"address"`
			HTTP    struct {
				TLS *yaml.Node `yaml:"tls"`
			} `yaml:"http"`
		} `yaml:"entryPoints"`
	}
	data, err := os.ReadFile(staticPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot verify TLS entrypoints: %w", err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("cannot parse TLS entrypoints: %w", err)
		}
	}
	if len(entrypoints) == 0 {
		return fmt.Errorf("--passthrough requires a TLS entrypoint")
	}
	for _, name := range entrypoints {
		ep, exists := cfg.EntryPoints[name]
		conventional := os.IsNotExist(err) && name == "websecure"
		_, port, _ := net.SplitHostPort(strings.TrimSuffix(ep.Address, "/tcp"))
		if !conventional && (!exists || (port != "443" && ep.HTTP.TLS == nil)) {
			return fmt.Errorf("--passthrough requires a TLS entrypoint; %q must listen on port 443 or configure http.tls in %s", name, staticPath)
		}
	}
	return nil
}

// flattenCSV accepts both repeated options and comma-separated option values.
func flattenCSV(values []string) []string {
	var result []string
	for _, value := range values {
		result = append(result, splitCSV(value)...)
	}
	return result
}
