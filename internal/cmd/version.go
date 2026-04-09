package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

// BuildDate is injected at build time via -ldflags.
var BuildDate = "unknown"

var versionCmd = &cobra.Command{
	Use:          "version",
	Short:        "Show traefikctl and Traefik versions",
	SilenceUsage: true,
	Run:          runVersion,
}

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Show traefikctl and Traefik versions")
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("traefikctl:\n")
	fmt.Printf("  Version:   %s\n", Version)
	fmt.Printf("  Go version: %s\n", runtime.Version())
	fmt.Printf("  Built:     %s\n", BuildDate)
	fmt.Printf("  OS/Arch:   %s/%s\n", runtime.GOOS, runtime.GOARCH)

	out, err := exec.Command("traefik", "version").Output()
	if err != nil {
		logger.Warn("Traefik binary not found or not in PATH")
		return
	}

	fmt.Printf("\nTraefik:\n%s", string(out))
}
