package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	mwAddName string
	mwAddType string
	mwAddFile string
	mwAddOpts []string
)

var middlewareAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a middleware to a dynamic config file",
	Long: `Add a reusable middleware to a Traefik dynamic config file.

Types and required --opt keys:
  redirect-https   scheme=https  permanent=true
  rate-limit       average=100   burst=50
  basic-auth       users=user1:hash,user2:hash
  strip-prefix     prefixes=/api,/v1

Examples:
  traefikctl middleware add --name redirect-https --type redirect-https
  traefikctl middleware add --name my-limit --type rate-limit --opt average=100 --opt burst=50`,
	SilenceUsage: true,
	RunE:         runMiddlewareAdd,
}

func init() {
	middlewareAddCmd.Flags().StringVar(&mwAddName, "name", "", "Middleware name")
	middlewareAddCmd.Flags().StringVar(&mwAddType, "type", "", "Middleware type")
	middlewareAddCmd.Flags().StringVar(&mwAddFile, "file", "", "Dynamic config file")
	middlewareAddCmd.Flags().StringArrayVar(&mwAddOpts, "opt", nil, "Type-specific options as key=value (repeatable)")

	_ = middlewareAddCmd.MarkFlagRequired("name")
	_ = middlewareAddCmd.MarkFlagRequired("type")

	middlewareCmd.AddCommand(middlewareAddCmd)
}

func runMiddlewareAdd(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(mwAddFile)
	if err != nil {
		return err
	}

	opts := parseKeyValuePairs(mwAddOpts)

	if err := addMiddleware(mwAddName, mwAddType, opts, filePath); err != nil {
		return err
	}

	logger.Info("Middleware '%s' (%s) added to %s", mwAddName, mwAddType, filePath)
	return nil
}

func parseKeyValuePairs(pairs []string) map[string]string {
	out := map[string]string{}
	for _, pair := range pairs {
		if idx := strings.Index(pair, "="); idx != -1 {
			out[pair[:idx]] = pair[idx+1:]
		}
	}
	return out
}
