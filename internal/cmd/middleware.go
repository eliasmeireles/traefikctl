package cmd

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var middlewareCmd = &cobra.Command{
	Use:   "middleware",
	Short: "Manage Traefik middlewares",
	Long: `Manage reusable Traefik middlewares (rate limiting, auth, redirects, headers).

Supported types: redirect-https, basic-auth, rate-limit, strip-prefix, ip-allowlist.
TCP supports ip-allowlist. ip-whitelist is a deprecated HTTP legacy alias.`,
}

func init() {
	rootCmd.AddCommand(middlewareCmd)
}

// addMiddleware creates or updates a named middleware in the config file.
// Supported types and their opts keys:
//
//	redirect-https: scheme (default "https"), permanent ("true"/"false")
//	rate-limit:     average (int), burst (int)
//	basic-auth:     users (comma-separated htpasswd entries)
//	strip-prefix:   prefixes (comma-separated path prefixes)
func addMiddleware(name, mwType string, opts map[string]string, filePath string) error {
	return addMiddlewareForProtocol(name, mwType, "http", opts, filePath)
}

// addMiddlewareForProtocol writes a middleware in its protocol namespace.
func addMiddlewareForProtocol(name, mwType, protocol string, opts map[string]string, filePath string) error {
	if err := validateResourceName(name); err != nil {
		return err
	}
	if protocol != "http" && protocol != "tcp" {
		return fmt.Errorf("invalid --protocol %q: expected http or tcp", protocol)
	}
	if protocol == "tcp" && mwType != "ip-allowlist" {
		return fmt.Errorf("TCP supports only ip-allowlist; middleware type %q is HTTP-only or unsupported", mwType)
	}
	mw, err := buildMiddleware(mwType, opts)
	if err != nil {
		return err
	}
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}
	if protocol == "tcp" {
		if cfg.TCP == nil {
			cfg.TCP = &TCPConfig{}
		}
		if cfg.TCP.Middlewares == nil {
			cfg.TCP.Middlewares = map[string]*MiddlewareConfig{}
		}
		cfg.TCP.Middlewares[name] = mw
	} else {
		if cfg.HTTP == nil {
			cfg.HTTP = &HTTPConfig{}
		}
		if cfg.HTTP.Middlewares == nil {
			cfg.HTTP.Middlewares = map[string]*MiddlewareConfig{}
		}
		cfg.HTTP.Middlewares[name] = mw
	}
	return saveDynamicConfig(filePath, cfg)
}

func buildMiddleware(mwType string, opts map[string]string) (*MiddlewareConfig, error) {
	switch mwType {
	case "ip-allowlist", "ip-whitelist":
		ranges := strings.Split(opts["sourcerange"], ",")
		if opts["sourcerange"] == "" {
			return nil, fmt.Errorf("%s requires --opt sourcerange with comma-separated IPs or CIDRs", mwType)
		}
		for i, raw := range ranges {
			source := strings.TrimSpace(raw)
			ranges[i] = source
			if net.ParseIP(source) != nil {
				continue
			}
			if _, _, err := net.ParseCIDR(source); err != nil {
				return nil, fmt.Errorf("invalid source range %q: expected an IP address or CIDR", source)
			}
		}
		allow := &IPAllowList{SourceRange: ranges}
		if mwType == "ip-whitelist" {
			return &MiddlewareConfig{IPWhiteList: allow}, nil
		}
		return &MiddlewareConfig{IPAllowList: allow}, nil
	case "redirect-https":
		scheme := opts["scheme"]
		if scheme == "" {
			scheme = "https"
		}
		return &MiddlewareConfig{
			RedirectScheme: &RedirectScheme{
				Scheme:    scheme,
				Permanent: opts["permanent"] == "true",
			},
		}, nil

	case "rate-limit":
		avg, _ := strconv.Atoi(opts["average"])
		burst, _ := strconv.Atoi(opts["burst"])
		if avg == 0 {
			avg = 100
		}
		if burst == 0 {
			burst = 50
		}
		return &MiddlewareConfig{RateLimit: &RateLimit{Average: avg, Burst: burst}}, nil

	case "basic-auth":
		users := splitCSV(opts["users"])
		if len(users) == 0 {
			return nil, fmt.Errorf("basic-auth requires --opt users=user1:hash,user2:hash")
		}
		return &MiddlewareConfig{BasicAuth: &BasicAuth{Users: users}}, nil

	case "strip-prefix":
		prefixes := splitCSV(opts["prefixes"])
		if len(prefixes) == 0 {
			return nil, fmt.Errorf("strip-prefix requires --opt prefixes=/api,/v1")
		}
		return &MiddlewareConfig{StripPrefix: &StripPrefix{Prefixes: prefixes}}, nil

	default:
		return nil, fmt.Errorf("unsupported middleware type '%s'. Supported: redirect-https, rate-limit, basic-auth, strip-prefix, ip-allowlist, ip-whitelist", mwType)
	}
}

// removeMiddleware deletes a named middleware from the config file.
func removeMiddleware(name, filePath string) error {
	return removeMiddlewareForProtocol(name, "", filePath)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
