package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/eliasmeireles/traefikctl/internal/logger"
	"github.com/spf13/cobra"
)

// readHAProxyInput returns raw HAProxy config text from either a file path
// or a base64-encoded string. filePath takes precedence when both are provided.
func readHAProxyInput(filePath, b64 string) (string, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("cannot read HAProxy config file %s: %w", filePath, err)
		}
		return string(data), nil
	}
	if b64 != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return "", fmt.Errorf("invalid base64 input: %w", err)
		}
		return string(data), nil
	}
	return "", fmt.Errorf("provide either --file or --base64")
}

// extractPort parses the port from an HAProxy bind address (e.g. "*:80", "10.0.0.1:443").
func extractPort(bind string) (string, error) {
	idx := strings.LastIndex(bind, ":")
	if idx < 0 {
		return "", fmt.Errorf("cannot determine port from bind address %q", bind)
	}
	return bind[idx+1:], nil
}

// checkPortConflict reports whether the given port is already registered.
func checkPortConflict(port string, used map[string]struct{}) bool {
	_, exists := used[port]
	return exists
}

var aclHostRe = regexp.MustCompile(`(?i)hdr\(host\)\s+-i\s+(\S+)`)

// entrypointNameForPort maps common ports to Traefik standard entrypoint names.
func entrypointNameForPort(port, frontendName string) string {
	switch port {
	case "80":
		return "web"
	case "443":
		return "websecure"
	default:
		return frontendName
	}
}

// convertHTTPFrontend converts an HAProxy HTTP frontend (and its referenced backends)
// into a Traefik DynamicConfig with HTTP routers and services.
func convertHTTPFrontend(fe HAProxyFrontend, backends map[string]HAProxyBackend, entrypoint string) *DynamicConfig {
	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers:  make(map[string]*Router),
			Services: make(map[string]*Service),
		},
	}

	aclHost := make(map[string]string)
	for _, acl := range fe.ACLs {
		m := aclHostRe.FindStringSubmatch(acl.Condition)
		if len(m) > 1 {
			aclHost[acl.Name] = m[1]
		}
	}

	for _, ub := range fe.UseBackends {
		host, ok := aclHost[ub.ACLName]
		if !ok {
			continue
		}
		cfg.HTTP.Routers[ub.Backend] = &Router{
			Rule:        fmt.Sprintf("Host(`%s`)", host),
			EntryPoints: []string{entrypoint},
			Service:     ub.Backend,
			Priority:    10,
		}
		if be, found := backends[ub.Backend]; found {
			cfg.HTTP.Services[ub.Backend] = buildHTTPService(be)
		}
	}

	if fe.DefaultBackend != "" {
		cfg.HTTP.Routers[fe.DefaultBackend] = &Router{
			Rule:        "PathPrefix(`/`)",
			EntryPoints: []string{entrypoint},
			Service:     fe.DefaultBackend,
			Priority:    1,
		}
		if be, found := backends[fe.DefaultBackend]; found {
			cfg.HTTP.Services[fe.DefaultBackend] = buildHTTPService(be)
		}
	}

	return cfg
}

func buildHTTPService(be HAProxyBackend) *Service {
	var servers []ServerURL
	for _, srv := range be.Servers {
		servers = append(servers, ServerURL{URL: "http://" + srv.Address})
	}
	return &Service{LoadBalancer: &LoadBalancer{Servers: servers}}
}

// convertTCPListen converts an HAProxy listen block (TCP mode) into a Traefik
// DynamicConfig with a TCP router and service.
func convertTCPListen(ls HAProxyListen, entrypoint string) *DynamicConfig {
	cfg := &DynamicConfig{
		TCP: &TCPConfig{
			Routers:  make(map[string]*TCPRouter),
			Services: make(map[string]*TCPService),
		},
	}

	cfg.TCP.Routers[ls.Name] = &TCPRouter{
		Rule:        "HostSNI(`*`)",
		EntryPoints: []string{entrypoint},
		Service:     ls.Name,
		TLS:         &TLSConf{Passthrough: true},
	}

	var servers []ServerAddress
	for _, srv := range ls.Servers {
		servers = append(servers, ServerAddress{Address: srv.Address})
	}

	cfg.TCP.Services[ls.Name] = &TCPService{
		LoadBalancer: &TCPLoadBalancer{Servers: servers},
	}

	return cfg
}

var (
	haproxyExportFile      string
	haproxyExportBase64    string
	haproxyExportOutputDir string
)

var haproxyExportCmd = &cobra.Command{
	Use:          "export",
	Short:        "Convert an HAProxy config to Traefik dynamic YAML files",
	SilenceUsage: true,
	RunE:         runHAProxyExport,
}

func init() {
	haproxyExportCmd.Flags().StringVar(&haproxyExportFile, "file", "", "Path to HAProxy config file")
	haproxyExportCmd.Flags().StringVar(&haproxyExportBase64, "base64", "", "Base64-encoded HAProxy config")
	haproxyExportCmd.Flags().StringVar(&haproxyExportOutputDir, "output-dir", defaultDynamicDir, "Output directory for Traefik YAML files")
	haproxyCmd.AddCommand(haproxyExportCmd)
}

func runHAProxyExport(cmd *cobra.Command, args []string) error {
	_, err := readHAProxyInput(haproxyExportFile, haproxyExportBase64)
	if err != nil {
		return err
	}

	outDir := haproxyExportOutputDir
	if !filepath.IsAbs(outDir) {
		return fmt.Errorf("output-dir must be an absolute path, got: %s", outDir)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return permissionHint("create output directory", outDir, err)
	}

	logger.Info("HAProxy export complete. Files written to %s", outDir)
	return nil
}
