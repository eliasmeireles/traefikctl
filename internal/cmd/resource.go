package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const defaultDynamicDir = "/etc/traefik/dynamic"

var resourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "Manage Traefik routers and services (add, remove, update)",
	Long: `Manage dynamic Traefik configurations interactively.

Subcommands:
  add      Add a new HTTP router and service
  remove   Remove a router and its service
  update   Update a service address or router domain`,
}

func init() {
	rootCmd.AddCommand(resourceCmd)
}

// DynamicConfig represents a Traefik dynamic configuration file.
type DynamicConfig struct {
	HTTP *HTTPConfig `yaml:"http,omitempty"`
	TCP  *TCPConfig  `yaml:"tcp,omitempty"`
}

type HTTPConfig struct {
	Routers     map[string]*Router           `yaml:"routers,omitempty"`
	Services    map[string]*Service          `yaml:"services,omitempty"`
	Middlewares map[string]*MiddlewareConfig `yaml:"middlewares,omitempty"`
}

type TCPConfig struct {
	Routers  map[string]*TCPRouter  `yaml:"routers,omitempty"`
	Services map[string]*TCPService `yaml:"services,omitempty"`
}

// RouterTLS configures TLS for an HTTP router.
type RouterTLS struct {
	CertResolver string `yaml:"certResolver,omitempty"`
}

type Router struct {
	Rule        string     `yaml:"rule"`
	EntryPoints []string   `yaml:"entryPoints"`
	Service     string     `yaml:"service"`
	Middlewares []string   `yaml:"middlewares,omitempty"`
	TLS         *RouterTLS `yaml:"tls,omitempty"`
	Priority    int        `yaml:"priority,omitempty"`
}

type Service struct {
	LoadBalancer *LoadBalancer `yaml:"loadBalancer"`
}

type LoadBalancer struct {
	Servers []ServerURL `yaml:"servers"`
}

type ServerURL struct {
	URL string `yaml:"url"`
}

type TCPRouter struct {
	Rule        string   `yaml:"rule"`
	EntryPoints []string `yaml:"entryPoints"`
	Service     string   `yaml:"service"`
	TLS         *TLSConf `yaml:"tls,omitempty"`
}

type TLSConf struct {
	Passthrough bool `yaml:"passthrough,omitempty"`
}

type TCPService struct {
	LoadBalancer *TCPLoadBalancer `yaml:"loadBalancer"`
}

type TCPLoadBalancer struct {
	Servers []ServerAddress `yaml:"servers"`
}

type ServerAddress struct {
	Address string `yaml:"address"`
}

// MiddlewareConfig mirrors Traefik v3 dynamic config middleware structure.
type MiddlewareConfig struct {
	RedirectScheme *RedirectScheme `yaml:"redirectScheme,omitempty"`
	BasicAuth      *BasicAuth      `yaml:"basicAuth,omitempty"`
	RateLimit      *RateLimit      `yaml:"rateLimit,omitempty"`
	StripPrefix    *StripPrefix    `yaml:"stripPrefix,omitempty"`
	Headers        *Headers        `yaml:"headers,omitempty"`
}

type RedirectScheme struct {
	Scheme    string `yaml:"scheme"`
	Permanent bool   `yaml:"permanent,omitempty"`
}

type BasicAuth struct {
	Users []string `yaml:"users"`
}

type RateLimit struct {
	Average int `yaml:"average"`
	Burst   int `yaml:"burst"`
}

type StripPrefix struct {
	Prefixes []string `yaml:"prefixes"`
}

type Headers struct {
	CustomRequestHeaders  map[string]string `yaml:"customRequestHeaders,omitempty"`
	CustomResponseHeaders map[string]string `yaml:"customResponseHeaders,omitempty"`
}

// listDynamicFiles returns YAML files in the dynamic config directory.
func listDynamicFiles() ([]string, error) {
	entries, err := os.ReadDir(defaultDynamicDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read dynamic config directory %s: %w", defaultDynamicDir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(defaultDynamicDir, entry.Name()))
		}
	}

	return files, nil
}

// selectDynamicFile prompts the user to select a file, or returns the one specified by flag.
func selectDynamicFile(flagValue string) (string, error) {
	if flagValue != "" {
		if _, err := os.Stat(flagValue); err != nil {
			return "", fmt.Errorf("file not found: %s", flagValue)
		}
		return flagValue, nil
	}

	files, err := listDynamicFiles()
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no dynamic config files found in %s", defaultDynamicDir)
	}

	if len(files) == 1 {
		return files[0], nil
	}

	fmt.Println("Multiple dynamic config files found:")
	for i, f := range files {
		fmt.Printf("  %d) %s\n", i+1, f)
	}

	idx, err := promptSelect("Select the file", len(files))
	if err != nil {
		return "", err
	}

	return files[idx], nil
}

// promptSelect displays a prompt and reads a 1-based index from stdin.
func promptSelect(prompt string, max int) (int, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [1-%d]: ", prompt, max)

	input, err := reader.ReadString('\n')
	if err != nil {
		return -1, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > max {
		return -1, fmt.Errorf("invalid selection: %s", input)
	}

	return num - 1, nil
}

// loadDynamicConfig loads a Traefik dynamic config from a YAML file.
// Returns an empty config when the file does not exist.
func loadDynamicConfig(path string) (*DynamicConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &DynamicConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var cfg DynamicConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	return &cfg, nil
}

// saveDynamicConfig writes a Traefik dynamic config to a YAML file.
func saveDynamicConfig(path string, cfg *DynamicConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return permissionHint("write dynamic config", path, err)
	}

	return nil
}
