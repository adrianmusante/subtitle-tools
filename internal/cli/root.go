package cli

import (
	"log/slog"
	"os"

	"github.com/adrianmusante/subtitle-tools/internal/logging"
	"github.com/spf13/cobra"
)

var verbose bool

// version and commit are set at build time via -ldflags.
// If left empty, they show as "dev".
var version = ""
var commit = ""

var rootCmd = &cobra.Command{
	Use:           "subtitle-tools",
	Short:         "Command-line tools for working with subtitle file",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Allow configuring verbosity via env var when the flag isn't provided.
		if err := resolveBoolFlagFromEnv(cmd, flagVerbose, envVerbose); err != nil {
			return err
		}

		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		logger := logging.New(os.Stderr, level)
		slog.SetDefault(logger)
		cmd.SetContext(logging.WithLogger(cmd.Context(), logger))
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand was passed, cobra will show help.
		return cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already formatted errors; keep it simple.
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, flagVerbose, flagVerboseShorthand, false, "Enable verbose (debug) logging")

	v := version
	if v == "" {
		v = "dev"
	}
	if commit != "" {
		rootCmd.Version = v + " (" + commit + ")"
	} else {
		rootCmd.Version = v
	}
	// Enable Cobra's built-in --version flag. This prints Version and exits.
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(translateCmd)
}
