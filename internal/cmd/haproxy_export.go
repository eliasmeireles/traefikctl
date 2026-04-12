package cmd

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
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
