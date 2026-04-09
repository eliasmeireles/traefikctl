package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var middlewareCmd = &cobra.Command{
	Use:   "middleware",
	Short: "Manage Traefik middlewares",
	Long: `Manage reusable Traefik middlewares (rate limiting, auth, redirects, headers).

Supported types: redirect-https, basic-auth, rate-limit, strip-prefix`,
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
	cfg, err := loadOrCreateHTTPConfig(filePath)
	if err != nil {
		return err
	}

	if cfg.HTTP.Middlewares == nil {
		cfg.HTTP.Middlewares = map[string]*MiddlewareConfig{}
	}

	mw, err := buildMiddleware(mwType, opts)
	if err != nil {
		return err
	}

	cfg.HTTP.Middlewares[name] = mw
	return saveDynamicConfig(filePath, cfg)
}

func buildMiddleware(mwType string, opts map[string]string) (*MiddlewareConfig, error) {
	switch mwType {
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
		return nil, fmt.Errorf("unsupported middleware type '%s'. Supported: redirect-https, rate-limit, basic-auth, strip-prefix", mwType)
	}
}

// removeMiddleware deletes a named middleware from the config file.
func removeMiddleware(name, filePath string) error {
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}

	if cfg.HTTP == nil || cfg.HTTP.Middlewares == nil {
		return fmt.Errorf("middleware '%s' not found", name)
	}

	if _, ok := cfg.HTTP.Middlewares[name]; !ok {
		return fmt.Errorf("middleware '%s' not found", name)
	}

	delete(cfg.HTTP.Middlewares, name)

	if len(cfg.HTTP.Middlewares) == 0 {
		cfg.HTTP.Middlewares = nil
	}

	return saveDynamicConfig(filePath, cfg)
}

// loadOrCreateHTTPConfig loads the config at path, ensuring HTTP section exists.
func loadOrCreateHTTPConfig(filePath string) (*DynamicConfig, error) {
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return nil, err
	}

	if cfg.HTTP == nil {
		cfg.HTTP = &HTTPConfig{
			Routers:  map[string]*Router{},
			Services: map[string]*Service{},
		}
	}

	return cfg, nil
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
