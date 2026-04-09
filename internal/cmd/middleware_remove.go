package cmd

import (
	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	mwRemoveName string
	mwRemoveFile string
)

var middlewareRemoveCmd = &cobra.Command{
	Use:          "remove",
	Short:        "Remove a middleware",
	SilenceUsage: true,
	RunE:         runMiddlewareRemove,
}

func init() {
	middlewareRemoveCmd.Flags().StringVar(&mwRemoveName, "name", "", "Middleware name")
	middlewareRemoveCmd.Flags().StringVar(&mwRemoveFile, "file", "", "Dynamic config file")
	_ = middlewareRemoveCmd.MarkFlagRequired("name")

	middlewareCmd.AddCommand(middlewareRemoveCmd)
}

func runMiddlewareRemove(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(mwRemoveFile)
	if err != nil {
		return err
	}

	if err := removeMiddleware(mwRemoveName, filePath); err != nil {
		return err
	}

	logger.Info("Middleware '%s' removed from %s", mwRemoveName, filePath)
	return nil
}
