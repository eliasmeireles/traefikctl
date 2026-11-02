package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var mwListFile string
var mwListProtocol string

var middlewareListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List all configured middlewares",
	SilenceUsage: true,
	RunE:         runMiddlewareList,
}

func init() {
	middlewareListCmd.Flags().StringVar(&mwListFile, "file", "", "List only this dynamic file")
	middlewareListCmd.Flags().StringVar(&mwListProtocol, "protocol", "", "Filter by http or tcp (default: both)")
	middlewareCmd.AddCommand(middlewareListCmd)
}

func runMiddlewareList(cmd *cobra.Command, args []string) error {
	if mwListProtocol != "" && mwListProtocol != "http" && mwListProtocol != "tcp" {
		return fmt.Errorf("invalid --protocol %q: expected http or tcp", mwListProtocol)
	}
	var files []string
	if mwListFile != "" {
		files = []string{mwListFile}
	} else {
		var err error
		files, err = listDynamicFiles()
		if err != nil {
			return err
		}
	}

	total := 0

	for _, filePath := range files {
		cfg, loadErr := loadDynamicConfig(filePath)
		if loadErr != nil {
			logger.Warn("Skipping %s: %v", filePath, loadErr)
			continue
		}

		fmt.Printf("\n%s\n", filePath)
		if cfg.HTTP != nil && mwListProtocol != "tcp" {
			total += printMiddlewares("HTTP", cfg.HTTP.Middlewares)
		}
		if cfg.TCP != nil && mwListProtocol != "http" {
			total += printMiddlewares("TCP", cfg.TCP.Middlewares)
		}
	}

	fmt.Println()
	logger.Info("Total middlewares: %d", total)
	return nil
}

// printMiddlewares prints a sorted protocol-specific middleware listing.
func printMiddlewares(protocol string, middlewares map[string]*MiddlewareConfig) int {
	keys := make([]string, 0, len(middlewares))
	for name := range middlewares {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		fmt.Printf("  [%s] %-30s  %s\n", protocol, name, middlewareSummary(middlewares[name]))
	}
	return len(keys)
}

func middlewareSummary(mw *MiddlewareConfig) string {
	switch {
	case mw.IPAllowList != nil:
		return fmt.Sprintf("ip-allowlist (sourceRange=%v)", mw.IPAllowList.SourceRange)
	case mw.IPWhiteList != nil:
		return fmt.Sprintf("ip-whitelist (deprecated, sourceRange=%v)", mw.IPWhiteList.SourceRange)
	case mw.RedirectScheme != nil:
		return fmt.Sprintf("redirect-https (scheme=%s, permanent=%v)", mw.RedirectScheme.Scheme, mw.RedirectScheme.Permanent)
	case mw.RateLimit != nil:
		return fmt.Sprintf("rate-limit (avg=%d, burst=%d)", mw.RateLimit.Average, mw.RateLimit.Burst)
	case mw.BasicAuth != nil:
		return fmt.Sprintf("basic-auth (%d user(s))", len(mw.BasicAuth.Users))
	case mw.StripPrefix != nil:
		return fmt.Sprintf("strip-prefix (%v)", mw.StripPrefix.Prefixes)
	case mw.Headers != nil:
		return "headers"
	default:
		return "unknown"
	}
}
