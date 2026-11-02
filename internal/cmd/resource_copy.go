package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	copyFrom    string
	copyName    string
	copyDomain  string
	copySrcFile string
	copyDstFile string
	copyChanges routeChanges
)

var resourceCopyCmd = &cobra.Command{
	Use:          "copy",
	Short:        "Clone an HTTP or TCP router with optional changes",
	SilenceUsage: true,
	RunE:         runResourceCopy,
}

func init() {
	resourceCopyCmd.Flags().StringVar(&copyFrom, "from", "", "Source router name")
	resourceCopyCmd.Flags().StringVar(&copyName, "name", "", "Destination router name")
	resourceCopyCmd.Flags().Var(domainFlag{&copyDomain}, "domain", "Replacement hostnames (repeatable or comma-separated)")
	resourceCopyCmd.Flags().StringVar(&copySrcFile, "file", "", "Source dynamic config file")
	resourceCopyCmd.Flags().StringVar(&copyDstFile, "dest", "", "Destination file (defaults to source)")
	bindRouteChanges(resourceCopyCmd, &copyChanges)
	_ = resourceCopyCmd.MarkFlagRequired("from")
	_ = resourceCopyCmd.MarkFlagRequired("name")
	resourceCmd.AddCommand(resourceCopyCmd)
}

func runResourceCopy(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}
	srcFile, err := selectDynamicFile(copySrcFile)
	if err != nil {
		return err
	}
	dstFile := copyDstFile
	if dstFile == "" {
		dstFile = srcFile
	}
	copyChanges.domains = copyDomain
	copyChanges.readChangedFlags(cmd)
	if err := copyRouterWithChanges(copyFrom, copyName, srcFile, dstFile, copyChanges); err != nil {
		return err
	}
	logger.Info("Router '%s' copied to '%s' in %s", copyFrom, copyName, dstFile)
	return nil
}

// copyRouter clones a route with optional replacement hostnames.
func copyRouter(srcName, dstName, domains, srcFile, dstFile string) error {
	return copyRouterWithChanges(srcName, dstName, srcFile, dstFile, routeChanges{domains: domains})
}

// copyRouterWithChanges clones a route, preserving TLS, middleware and external services.
func copyRouterWithChanges(srcName, dstName, srcFile, dstFile string, changes routeChanges) error {
	if err := validateResourceName(srcName); err != nil {
		return err
	}
	if err := validateResourceName(dstName); err != nil {
		return err
	}
	src, err := loadDynamicConfig(srcFile)
	if err != nil {
		return err
	}
	dst, err := loadDynamicConfig(dstFile)
	if err != nil {
		return err
	}
	switch {
	case src.HTTP != nil && src.HTTP.Routers[srcName] != nil && changes.protocol != "tcp":
		err = copyHTTPRouter(src, dst, srcName, dstName, dstFile, changes)
	case src.TCP != nil && src.TCP.Routers[srcName] != nil:
		err = copyTCPRouter(src, dst, srcName, dstName, dstFile, changes)
	default:
		return fmt.Errorf("router '%s' not found", srcName)
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dstFile), 0755); err != nil {
		return err
	}
	return saveDynamicConfig(dstFile, dst)
}
