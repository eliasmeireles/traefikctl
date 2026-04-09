package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var versionCmd = &cobra.Command{
	Use:          "version",
	Short:        "Show traefikctl and Traefik versions",
	SilenceUsage: true,
	Run:          runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("traefikctl: %s\n", Version)

	out, err := exec.Command("traefik", "version").Output()
	if err != nil {
		logger.Warn("Traefik binary not found or not in PATH")
		return
	}

	fmt.Printf("\nTraefik:\n%s", string(out))
}
