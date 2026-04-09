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
)

var resourceCopyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Clone an existing router to a new name and optional domain",
	Long: `Copy a router and its service to a new name.
The original is not modified. Optionally point the copy at a different domain.

Examples:
  traefikctl resource copy --from my-app --name my-app-staging --domain staging.example.com
  traefikctl resource copy --from my-app --name my-app-v2`,
	SilenceUsage: true,
	RunE:         runResourceCopy,
}

func init() {
	resourceCopyCmd.Flags().StringVar(&copyFrom, "from", "", "Source router name")
	resourceCopyCmd.Flags().StringVar(&copyName, "name", "", "Destination router name")
	resourceCopyCmd.Flags().StringVar(&copyDomain, "domain", "", "New domain for the copy (optional)")
	resourceCopyCmd.Flags().StringVar(&copySrcFile, "file", "", "Source dynamic config file")
	resourceCopyCmd.Flags().StringVar(&copyDstFile, "dest", "", "Destination file (defaults to same as source)")

	_ = resourceCopyCmd.MarkFlagRequired("from")
	_ = resourceCopyCmd.MarkFlagRequired("name")

	resourceCmd.AddCommand(resourceCopyCmd)
}

func runResourceCopy(cmd *cobra.Command, args []string) error {
	srcFile, err := selectDynamicFile(copySrcFile)
	if err != nil {
		return err
	}

	dstFile := srcFile
	if copyDstFile != "" {
		dstFile = copyDstFile
	}

	if err := copyRouter(copyFrom, copyName, copyDomain, srcFile, dstFile); err != nil {
		return err
	}

	logger.Info("Router '%s' copied to '%s'", copyFrom, copyName)
	if copyDomain != "" {
		logger.Info("  New rule: Host(`%s`)", copyDomain)
	}
	logger.Info("Config saved: %s (Traefik will auto-reload)", dstFile)
	return nil
}

// copyRouter duplicates a router and its associated service under a new name.
// If newDomain is non-empty, the copy gets Host(`newDomain`) as its rule.
// Returns an error if the destination router name already exists.
func copyRouter(srcName, dstName, newDomain, srcFile, dstFile string) error {
	src, err := loadDynamicConfig(srcFile)
	if err != nil {
		return err
	}

	if src.HTTP == nil {
		return fmt.Errorf("router '%s' not found", srcName)
	}

	srcRouter, ok := src.HTTP.Routers[srcName]
	if !ok {
		return fmt.Errorf("router '%s' not found", srcName)
	}

	var dst *DynamicConfig
	if srcFile == dstFile {
		dst = src
	} else if _, statErr := os.Stat(dstFile); os.IsNotExist(statErr) {
		dst = &DynamicConfig{}
	} else {
		dst, err = loadDynamicConfig(dstFile)
		if err != nil {
			return err
		}
	}

	if dst.HTTP == nil {
		dst.HTTP = &HTTPConfig{Routers: map[string]*Router{}, Services: map[string]*Service{}}
	}

	if _, exists := dst.HTTP.Routers[dstName]; exists {
		return fmt.Errorf("router '%s' already exists in destination", dstName)
	}

	newSvcName := dstName + "-svc"

	rule := srcRouter.Rule
	if newDomain != "" {
		rule = fmt.Sprintf("Host(`%s`)", newDomain)
	}

	dst.HTTP.Routers[dstName] = &Router{
		Rule:        rule,
		EntryPoints: append([]string{}, srcRouter.EntryPoints...),
		Service:     newSvcName,
		Priority:    srcRouter.Priority,
	}

	newSvc := &Service{LoadBalancer: &LoadBalancer{}}
	if srcSvc := src.HTTP.Services[srcRouter.Service]; srcSvc != nil && srcSvc.LoadBalancer != nil {
		newSvc.LoadBalancer.Servers = append([]ServerURL{}, srcSvc.LoadBalancer.Servers...)
	}
	dst.HTTP.Services[newSvcName] = newSvc

	if err := os.MkdirAll(filepath.Dir(dstFile), 0755); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}

	return saveDynamicConfig(dstFile, dst)
}
