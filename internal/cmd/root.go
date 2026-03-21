package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "traefikctl",
		Short: "Traefik Control CLI and Agent",
		Long: `traefikctl is a CLI tool for managing Traefik proxy configurations.
It provides installation, configuration management, and dynamic routing
with automatic hot-reload support.`,
		Version: Version,
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "/etc/traefikctl/config.yaml", "config file path")
}

func exitWithError(msg string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	} else {
		fmt.Fprintln(os.Stderr, msg)
	}
	os.Exit(1)
}
