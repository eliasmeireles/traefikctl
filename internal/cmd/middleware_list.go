package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var middlewareListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List all configured middlewares",
	SilenceUsage: true,
	RunE:         runMiddlewareList,
}

func init() {
	middlewareCmd.AddCommand(middlewareListCmd)
}

func runMiddlewareList(cmd *cobra.Command, args []string) error {
	files, err := listDynamicFiles()
	if err != nil {
		return err
	}

	total := 0

	for _, filePath := range files {
		cfg, loadErr := loadDynamicConfig(filePath)
		if loadErr != nil {
			logger.Warn("Skipping %s: %v", filePath, loadErr)
			continue
		}

		if cfg.HTTP == nil || len(cfg.HTTP.Middlewares) == 0 {
			continue
		}

		fmt.Printf("\n%s\n", filePath)

		keys := make([]string, 0, len(cfg.HTTP.Middlewares))
		for k := range cfg.HTTP.Middlewares {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, name := range keys {
			fmt.Printf("  %-30s  %s\n", name, middlewareSummary(cfg.HTTP.Middlewares[name]))
			total++
		}
	}

	fmt.Println()
	logger.Info("Total middlewares: %d", total)
	return nil
}

func middlewareSummary(mw *MiddlewareConfig) string {
	switch {
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
