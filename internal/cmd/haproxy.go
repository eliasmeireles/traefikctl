package cmd

import "github.com/spf13/cobra"

var haproxyCmd = &cobra.Command{
	Use:   "haproxy",
	Short: "HAProxy integration utilities",
	Long:  "Tools for importing and exporting HAProxy configurations.",
}

func init() {
	rootCmd.AddCommand(haproxyCmd)
}
