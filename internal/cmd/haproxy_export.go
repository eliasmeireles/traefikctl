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
	"gopkg.in/yaml.v3"
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

// bindToEntrypointAddress normalises an HAProxy bind address to a Traefik
// entrypoint address string. "*:80" → ":80", "10.0.0.1:5672" → "10.0.0.1:5672".
func bindToEntrypointAddress(bind string) string {
	if strings.HasPrefix(bind, "*:") {
		return ":" + bind[2:]
	}
	return bind
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
		defaultRouterKey := fe.Name + "-default"
		cfg.HTTP.Routers[defaultRouterKey] = &Router{
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

	router := &TCPRouter{
		Rule:        "HostSNI(`*`)",
		EntryPoints: []string{entrypoint},
		Service:     ls.Name,
	}
	// Only add TLS passthrough for port 443 (HTTPS passthrough scenario)
	if entrypoint == "websecure" {
		router.TLS = &TLSConf{Passthrough: true}
	}
	cfg.TCP.Routers[ls.Name] = router

	var servers []ServerAddress
	for _, srv := range ls.Servers {
		servers = append(servers, ServerAddress{Address: srv.Address})
	}

	cfg.TCP.Services[ls.Name] = &TCPService{
		LoadBalancer: &TCPLoadBalancer{Servers: servers},
	}

	return cfg
}

// mergeDynamicConfigs combines multiple DynamicConfig instances into one,
// merging all HTTP and TCP routers/services into shared maps.
func mergeDynamicConfigs(configs []*DynamicConfig) *DynamicConfig {
	merged := &DynamicConfig{}
	for _, c := range configs {
		if c.HTTP != nil {
			if merged.HTTP == nil {
				merged.HTTP = &HTTPConfig{
					Routers:  make(map[string]*Router),
					Services: make(map[string]*Service),
				}
			}
			for k, v := range c.HTTP.Routers {
				merged.HTTP.Routers[k] = v
			}
			for k, v := range c.HTTP.Services {
				merged.HTTP.Services[k] = v
			}
		}
		if c.TCP != nil {
			if merged.TCP == nil {
				merged.TCP = &TCPConfig{
					Routers:  make(map[string]*TCPRouter),
					Services: make(map[string]*TCPService),
				}
			}
			for k, v := range c.TCP.Routers {
				merged.TCP.Routers[k] = v
			}
			for k, v := range c.TCP.Services {
				merged.TCP.Services[k] = v
			}
		}
	}
	return merged
}

// ExportResult holds the outcome of an HAProxy export operation.
type ExportResult struct {
	// Warnings contains messages for skipped blocks (port conflicts, parse errors).
	Warnings []string
	// TCPEntrypoints maps entrypoint name to its Traefik address string
	// for every TCP service that requires a custom entrypoint (port ≠ 80/443).
	TCPEntrypoints map[string]string
}

// exportHAProxyToDir parses the given HAProxy config text and writes Traefik
// dynamic YAML files into outDir.
// When split is false (default), all blocks are merged into a single file named outFile.
// When split is true, one file is written per frontend/listen block.
func exportHAProxyToDir(text, outDir, outFile string, split bool) (ExportResult, error) {
	result := ExportResult{TCPEntrypoints: make(map[string]string)}

	haCfg, err := ParseHAProxyConfig(text)
	if err != nil {
		return result, err
	}

	backendMap := make(map[string]HAProxyBackend, len(haCfg.Backends))
	for _, be := range haCfg.Backends {
		backendMap[be.Name] = be
	}

	usedPorts := make(map[string]struct{})
	var configs []*DynamicConfig

	for _, fe := range haCfg.Frontends {
		port, skipped := resolveBindPort(fe.Binds, fe.Name, usedPorts, &result.Warnings)
		if skipped {
			continue
		}
		usedPorts[port] = struct{}{}

		entrypoint := entrypointNameForPort(port, fe.Name)
		dynCfg := convertHTTPFrontend(fe, backendMap, entrypoint)

		if split {
			path := filepath.Join(outDir, fe.Name+".yaml")
			if err := saveDynamicConfig(path, dynCfg); err != nil {
				return result, err
			}
		} else {
			configs = append(configs, dynCfg)
		}
	}

	for _, ls := range haCfg.Listens {
		port, skipped := resolveBindPort(ls.Binds, ls.Name, usedPorts, &result.Warnings)
		if skipped {
			continue
		}
		usedPorts[port] = struct{}{}

		entrypoint := entrypointNameForPort(port, ls.Name)

		var dynCfg *DynamicConfig
		if ls.Mode == "http" {
			fe := HAProxyFrontend{
				Name:           ls.Name,
				Binds:          ls.Binds,
				Mode:           ls.Mode,
				DefaultBackend: ls.Name + "-backend",
			}
			be := HAProxyBackend{
				Name:    ls.Name + "-backend",
				Mode:    "http",
				Balance: ls.Balance,
				Servers: ls.Servers,
			}
			dynCfg = convertHTTPFrontend(fe, map[string]HAProxyBackend{be.Name: be}, entrypoint)
		} else {
			dynCfg = convertTCPListen(ls, entrypoint)
			// Collect non-standard TCP entrypoints (not web/websecure)
			if entrypoint != "web" && entrypoint != "websecure" && len(ls.Binds) > 0 {
				result.TCPEntrypoints[entrypoint] = bindToEntrypointAddress(ls.Binds[0])
			}
		}

		if split {
			path := filepath.Join(outDir, ls.Name+".yaml")
			if err := saveDynamicConfig(path, dynCfg); err != nil {
				return result, err
			}
		} else {
			configs = append(configs, dynCfg)
		}
	}

	if !split && len(configs) > 0 {
		merged := mergeDynamicConfigs(configs)
		if err := saveDynamicConfig(filepath.Join(outDir, outFile), merged); err != nil {
			return result, err
		}
	}

	return result, nil
}

// resolveBindPort extracts the port from the first bind address.
// Returns skipped=true (and appends a warning) if port is already used or cannot be parsed.
func resolveBindPort(binds []string, name string, usedPorts map[string]struct{}, warnings *[]string) (port string, skipped bool) {
	if len(binds) == 0 {
		*warnings = append(*warnings, fmt.Sprintf("WARNING: %q has no bind address, skipping", name))
		return "", true
	}

	p, err := extractPort(binds[0])
	if err != nil {
		*warnings = append(*warnings, fmt.Sprintf("WARNING: %q — cannot parse bind port: %v, skipping", name, err))
		return "", true
	}

	if checkPortConflict(p, usedPorts) {
		*warnings = append(*warnings, fmt.Sprintf("WARNING: port %s already used, skipping %q", p, name))
		return "", true
	}

	return p, false
}

// applyTCPEntrypoints reads the Traefik static config at staticPath, merges the
// given TCP entrypoints into the entryPoints section, and writes the file back.
// Existing entrypoints are preserved; only new names are added.
func applyTCPEntrypoints(staticPath string, entrypoints map[string]string) error {
	data, err := os.ReadFile(staticPath)
	if err != nil {
		return permissionHint("read static config", staticPath, err)
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("cannot parse %s: %w", staticPath, err)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	eps, _ := cfg["entryPoints"].(map[string]interface{})
	if eps == nil {
		eps = make(map[string]interface{})
	}

	added := 0
	for name, addr := range entrypoints {
		if _, exists := eps[name]; exists {
			logger.Info("entrypoint %q already defined in %s — skipping", name, staticPath)
			continue
		}
		eps[name] = map[string]interface{}{"address": addr}
		added++
	}
	cfg["entryPoints"] = eps

	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("cannot marshal updated config: %w", err)
	}

	if err := os.WriteFile(staticPath, out, 0644); err != nil {
		return permissionHint("write static config", staticPath, err)
	}

	logger.Info("Added %d TCP entrypoint(s) to %s — restart Traefik to apply", added, staticPath)
	return nil
}

// outputFileName derives the YAML output filename from the HAProxy input file path.
// Falls back to defaultName when the input path is empty (e.g. base64 input).
func outputFileName(inputFilePath, defaultName string) string {
	if inputFilePath == "" {
		return defaultName
	}
	base := filepath.Base(inputFilePath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + ".yaml"
}

const defaultStaticConfigPath = "/etc/traefik/traefik.yaml"

var (
	haproxyExportFile             string
	haproxyExportBase64           string
	haproxyExportOutputDir        string
	haproxyExportSplit            bool
	haproxyExportApplyEntrypoints bool
)

var haproxyExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Convert an HAProxy config to Traefik dynamic YAML",
	Long: `Read an HAProxy configuration (from a file or base64-encoded string) and
generate Traefik dynamic YAML — merged into a single file by default.

Use --split to write one file per frontend/listen block instead.
The global and defaults sections of the HAProxy config are ignored.
Blocks with ports that conflict with previously processed blocks are skipped
with a warning.

Examples:
  traefikctl haproxy export --file /etc/haproxy/haproxy.cfg
  traefikctl haproxy export --base64 <base64-encoded-config>
  traefikctl haproxy export --file haproxy.cfg --output-dir /tmp/preview
  traefikctl haproxy export --file haproxy.cfg --split`,
	SilenceUsage: true,
	RunE:         runHAProxyExport,
}

func init() {
	haproxyExportCmd.Flags().StringVar(&haproxyExportFile, "file", "", "Path to HAProxy config file")
	haproxyExportCmd.Flags().StringVar(&haproxyExportBase64, "base64", "", "Base64-encoded HAProxy config")
	haproxyExportCmd.Flags().StringVar(&haproxyExportOutputDir, "output-dir", defaultDynamicDir, "Output directory for Traefik YAML files")
	haproxyExportCmd.Flags().BoolVar(&haproxyExportSplit, "split", false, "Write one YAML file per frontend/listen block instead of a single merged file")
	haproxyExportCmd.Flags().BoolVar(&haproxyExportApplyEntrypoints, "no-apply-entrypoints", false, "Skip automatic update of TCP entrypoints in "+defaultStaticConfigPath)
	haproxyCmd.AddCommand(haproxyExportCmd)
}

func runHAProxyExport(cmd *cobra.Command, args []string) error {
	text, err := readHAProxyInput(haproxyExportFile, haproxyExportBase64)
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

	outFile := outputFileName(haproxyExportFile, "haproxy-export.yaml")
	result, err := exportHAProxyToDir(text, outDir, outFile, haproxyExportSplit)
	for _, w := range result.Warnings {
		logger.Warn("%s", w)
	}
	if err != nil {
		return err
	}

	logger.Info("HAProxy export complete. Files written to %s", outDir)

	if len(result.TCPEntrypoints) > 0 {
		if !haproxyExportApplyEntrypoints {
			if err := applyTCPEntrypoints(defaultStaticConfigPath, result.TCPEntrypoints); err != nil {
				logger.Warn("Could not update %s: %v", defaultStaticConfigPath, err)
				logger.Info("NOTE: Add the following TCP entrypoints manually to your traefik.yaml:")
				fmt.Println()
				fmt.Println("  entryPoints:")
				for name, addr := range result.TCPEntrypoints {
					fmt.Printf("    %s:\n", name)
					fmt.Printf("      address: \"%s\"\n", addr)
				}
				fmt.Println()
			}
		} else {
			logger.Info("NOTE: Add the following TCP entrypoints to your traefik.yaml:")
			fmt.Println()
			fmt.Println("  entryPoints:")
			for name, addr := range result.TCPEntrypoints {
				fmt.Printf("    %s:\n", name)
				fmt.Printf("      address: \"%s\"\n", addr)
			}
			fmt.Println()
		}
	}

	return nil
}
