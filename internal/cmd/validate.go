package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
	"github.com/eliasmeireles/traefikctl/internal/validate"
)

var (
	validateFile  string
	validateQuiet bool
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a Traefik static configuration file",
	Long: `Runs the static config through Traefik's own config decoder
(via 'traefik healthcheck'), reporting every rejected field in one pass.

Traefik's own parser only ever reports the FIRST unknown field it finds,
then exits — so fixing a broken traefik.yaml by hand normally means one
field per restart. This strips each rejected field from a scratch copy and
retries until Traefik accepts it (or nothing more can be removed), so you
see every problem at once.`,
	SilenceUsage: true,
	RunE:         runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateFile, "file", defaultStaticConfigPath, "Path to the static config file to validate")
	validateCmd.Flags().BoolVarP(&validateQuiet, "quiet", "q", false, "Only print output when problems are found")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	return runValidateFile(validateFile, validateQuiet)
}

// runValidateFile validates path and returns a non-nil error (after
// printing the report) when problems were found, so cobra exits non-zero.
func runValidateFile(path string, quiet bool) error {
	result, err := validate.StaticFile(context.Background(), path, validate.Options{})
	if err != nil {
		return fmt.Errorf("failed to validate %s: %w", path, err)
	}

	if result.Skipped {
		logger.Warn("%s: %s", path, result.SkipReason)
		return nil
	}

	if result.OK() {
		if !quiet {
			logger.Info("%s: OK", path)
		}
		return nil
	}

	fmt.Print(result.String())
	return fmt.Errorf("%d problem(s) found in %s", len(result.Errors()), path)
}
