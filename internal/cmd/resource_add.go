package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	addDomain        string
	addAddress       string
	addName          string
	addEntrypoint    string
	addType          string
	addFile          string
	addMiddlewares   []string
	addRedirectHTTPS bool
	addTLS           bool
	addCertResolver  string
)

var resourceAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new router and service",
	Long: `Add a new Traefik router and service to a dynamic configuration file.

If no file exists, one will be created automatically.

Examples:
  traefikctl resource add --domain app.example.com --address 10.8.0.2:8080 --name my-app
  traefikctl resource add --domain api.example.com --address 10.8.0.3:3000 --name my-api --entrypoint websecure
  traefikctl resource add --address 10.8.0.10:5432 --name postgres --type tcp --entrypoint postgres`,
	SilenceUsage: true,
	RunE:         runResourceAdd,
}

func init() {
	resourceAddCmd.Flags().StringVar(&addDomain, "domain", "", "Domain for the router rule (e.g., app.example.com)")
	resourceAddCmd.Flags().StringVar(&addAddress, "address", "", "Backend server address (ip:port)")
	resourceAddCmd.Flags().StringVar(&addName, "name", "", "Router and service name")
	resourceAddCmd.Flags().StringVar(&addEntrypoint, "entrypoint", "web", "Entrypoint name (default: web)")
	resourceAddCmd.Flags().StringVar(&addType, "type", "http", "Type: http or tcp")
	resourceAddCmd.Flags().StringVar(&addFile, "file", "", "Dynamic config file (skip selection prompt)")
	resourceAddCmd.Flags().StringArrayVar(&addMiddlewares, "middleware", nil, "Attach middleware(s) by name (repeatable)")
	resourceAddCmd.Flags().BoolVar(&addRedirectHTTPS, "redirect-https", false, "Add HTTP→HTTPS redirect middleware automatically")
	resourceAddCmd.Flags().BoolVar(&addTLS, "tls", false, "Enable TLS on the router")
	resourceAddCmd.Flags().StringVar(&addCertResolver, "cert-resolver", "", "Let's Encrypt cert resolver name (e.g. letsencrypt)")

	_ = resourceAddCmd.MarkFlagRequired("address")
	_ = resourceAddCmd.MarkFlagRequired("name")

	resourceCmd.AddCommand(resourceAddCmd)
}

func runResourceAdd(cmd *cobra.Command, args []string) error {
	var filePath string
	var cfg *DynamicConfig
	var err error

	// Determine file path
	files, _ := listDynamicFiles()

	if addFile != "" {
		filePath = addFile
	} else if len(files) == 0 {
		filePath = filepath.Join(defaultDynamicDir, "services.yaml")
		logger.Info("No dynamic config files found, will create: %s", filePath)
	} else if len(files) == 1 {
		filePath = files[0]
	} else {
		filePath, err = selectDynamicFile("")
		if err != nil {
			return err
		}
	}

	// Load or create config
	if _, statErr := os.Stat(filePath); os.IsNotExist(statErr) {
		cfg = &DynamicConfig{}
	} else {
		cfg, err = loadDynamicConfig(filePath)
		if err != nil {
			return err
		}
	}

	if addType == "tcp" {
		return addTCPResource(cfg, filePath)
	}

	return addHTTPResource(cfg, filePath)
}

func addHTTPResource(cfg *DynamicConfig, filePath string) error {
	if cfg.HTTP == nil {
		cfg.HTTP = &HTTPConfig{}
	}
	if cfg.HTTP.Routers == nil {
		cfg.HTTP.Routers = make(map[string]*Router)
	}
	if cfg.HTTP.Services == nil {
		cfg.HTTP.Services = make(map[string]*Service)
	}

	// Check if router already exists
	if _, exists := cfg.HTTP.Routers[addName]; exists {
		return fmt.Errorf("router '%s' already exists", addName)
	}

	// Build router rule
	rule := "PathPrefix(`/`)"
	if addDomain != "" {
		rule = fmt.Sprintf("Host(`%s`)", addDomain)
	}

	svcName := addName + "-svc"

	// Create router
	router := &Router{
		Rule:        rule,
		EntryPoints: []string{addEntrypoint},
		Service:     svcName,
		Middlewares: addMiddlewares,
	}

	if addTLS || addCertResolver != "" {
		router.TLS = &RouterTLS{}
		if addCertResolver != "" {
			router.TLS.CertResolver = addCertResolver
		}
	}

	cfg.HTTP.Routers[addName] = router

	if addRedirectHTTPS {
		const redirectMWName = "redirect-to-https"
		if cfg.HTTP.Middlewares == nil {
			cfg.HTTP.Middlewares = map[string]*MiddlewareConfig{}
		}
		cfg.HTTP.Middlewares[redirectMWName] = &MiddlewareConfig{
			RedirectScheme: &RedirectScheme{Scheme: "https", Permanent: true},
		}
		// Move the main router to websecure and create an HTTP router that redirects.
		router.EntryPoints = []string{"websecure"}
		cfg.HTTP.Routers[addName] = router

		httpRouterName := addName + "-http"
		cfg.HTTP.Routers[httpRouterName] = &Router{
			Rule:        rule,
			EntryPoints: []string{"web"},
			Service:     svcName,
			Middlewares: []string{redirectMWName},
		}
		logger.Info("Created HTTP redirect router '%s' -> websecure via middleware '%s'", httpRouterName, redirectMWName)
	}

	// Create service
	cfg.HTTP.Services[svcName] = &Service{
		LoadBalancer: &LoadBalancer{
			Servers: []ServerURL{
				{URL: fmt.Sprintf("http://%s", addAddress)},
			},
		},
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return permissionHint("create dynamic config directory", filepath.Dir(filePath), err)
	}

	if err := saveDynamicConfig(filePath, cfg); err != nil {
		return err
	}

	logger.Info("Added HTTP router '%s' -> %s", addName, addAddress)
	if addDomain != "" {
		logger.Info("  Rule: Host(`%s`)", addDomain)
	}
	logger.Info("  Service: %s", svcName)
	logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
	return nil
}

func addTCPResource(cfg *DynamicConfig, filePath string) error {
	if cfg.TCP == nil {
		cfg.TCP = &TCPConfig{}
	}
	if cfg.TCP.Routers == nil {
		cfg.TCP.Routers = make(map[string]*TCPRouter)
	}
	if cfg.TCP.Services == nil {
		cfg.TCP.Services = make(map[string]*TCPService)
	}

	// Check if router already exists
	if _, exists := cfg.TCP.Routers[addName]; exists {
		return fmt.Errorf("TCP router '%s' already exists", addName)
	}

	svcName := addName + "-svc"

	// Create router
	cfg.TCP.Routers[addName] = &TCPRouter{
		Rule:        "HostSNI(`*`)",
		EntryPoints: []string{addEntrypoint},
		Service:     svcName,
	}

	// Create service
	cfg.TCP.Services[svcName] = &TCPService{
		LoadBalancer: &TCPLoadBalancer{
			Servers: []ServerAddress{
				{Address: addAddress},
			},
		},
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return permissionHint("create dynamic config directory", filepath.Dir(filePath), err)
	}

	if err := saveDynamicConfig(filePath, cfg); err != nil {
		return err
	}

	logger.Info("Added TCP router '%s' -> %s", addName, addAddress)
	logger.Info("  Entrypoint: %s", addEntrypoint)
	logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
	return nil
}
