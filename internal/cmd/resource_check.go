package cmd

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	checkName    string
	checkFile    string
	checkTimeout int
)

var resourceCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if backend servers are reachable",
	Long: `Test connectivity to all configured backend servers.

For HTTP services, performs an HTTP GET to the backend URL.
For TCP services, attempts a TCP dial to the address.

Examples:
  traefikctl resource check
  traefikctl resource check --name my-app
  traefikctl resource check --timeout 5`,
	SilenceUsage: true,
	RunE:         runResourceCheck,
}

func init() {
	resourceCheckCmd.Flags().StringVar(&checkName, "name", "", "Check only the specified router name")
	resourceCheckCmd.Flags().StringVar(&checkFile, "file", "", "Check only this dynamic config file")
	resourceCheckCmd.Flags().IntVar(&checkTimeout, "timeout", 3, "Connection timeout in seconds")

	resourceCmd.AddCommand(resourceCheckCmd)
}

func runResourceCheck(cmd *cobra.Command, args []string) error {
	files, err := listDynamicFiles()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		logger.Info("No dynamic config files found in %s", defaultDynamicDir)
		return nil
	}

	if checkFile != "" {
		files = []string{checkFile}
	}

	timeout := time.Duration(checkTimeout) * time.Second
	passed, failed := 0, 0

	for _, filePath := range files {
		cfg, loadErr := loadDynamicConfig(filePath)
		if loadErr != nil {
			logger.Warn("Skipping %s: %v", filePath, loadErr)
			continue
		}

		if cfg.HTTP != nil {
			for _, name := range sortedRouterKeys(cfg.HTTP.Routers) {
				if checkName != "" && name != checkName {
					continue
				}

				svc := cfg.HTTP.Services[cfg.HTTP.Routers[name].Service]
				if svc == nil || svc.LoadBalancer == nil || len(svc.LoadBalancer.Servers) == 0 {
					fmt.Printf("  [SKIP] %s: no backend configured\n", name)
					continue
				}

				for _, server := range svc.LoadBalancer.Servers {
					if pingErr := pingHTTP(server.URL, timeout); pingErr != nil {
						fmt.Printf("  [FAIL] %-25s -> %-35s  %v\n", name, server.URL, pingErr)
						failed++
					} else {
						fmt.Printf("  [OK]   %-25s -> %s\n", name, server.URL)
						passed++
					}
				}
			}
		}

		if cfg.TCP != nil {
			for _, name := range sortedTCPRouterKeys(cfg.TCP.Routers) {
				if checkName != "" && name != checkName {
					continue
				}

				svc := cfg.TCP.Services[cfg.TCP.Routers[name].Service]
				if svc == nil || svc.LoadBalancer == nil || len(svc.LoadBalancer.Servers) == 0 {
					fmt.Printf("  [SKIP] %s: no backend configured\n", name)
					continue
				}

				for _, server := range svc.LoadBalancer.Servers {
					if dialErr := dialTCP(server.Address, timeout); dialErr != nil {
						fmt.Printf("  [FAIL] %-25s -> %-35s  %v\n", name, server.Address, dialErr)
						failed++
					} else {
						fmt.Printf("  [OK]   %-25s -> %s\n", name, server.Address)
						passed++
					}
				}
			}
		}
	}

	fmt.Println()
	logger.Info("Results: %d reachable, %d unreachable", passed, failed)

	if failed > 0 {
		return fmt.Errorf("%d backend(s) unreachable — check if the service is running", failed)
	}

	return nil
}

func pingHTTP(rawURL string, timeout time.Duration) error {
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(rawURL) //nolint:noctx
	if err != nil {
		msg := err.Error()
		if idx := strings.LastIndex(msg, ": "); idx != -1 {
			msg = msg[idx+2:]
		}

		return fmt.Errorf("%s", msg)
	}
	defer func() { _ = resp.Body.Close() }()

	return nil
}

func dialTCP(address string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return fmt.Errorf("TCP dial failed: %w", err)
	}
	_ = conn.Close()

	return nil
}
