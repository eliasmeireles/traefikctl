package cmd

import (
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if v, _ := cmd.Flags().GetBool("version"); v {
				runVersion(cmd, args)
				return nil
			}
			return cmd.Help()
		},
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "/etc/traefikctl/config.yaml", "config file path")
}
