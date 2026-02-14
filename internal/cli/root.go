package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"log/slog"

	"github.com/adrianmusante/subtitle-tools/internal/logging"
	"github.com/spf13/cobra"
)

var verbose bool

// version and commit are set at build time via -ldflags.
// If left empty, they show as "dev".
var version = ""
var commit = ""

const (
	envVerbose = "SUBTITLE_TOOLS_VERBOSE"
	envDryRun  = "SUBTITLE_TOOLS_DRY_RUN"
	envWorkdir = "SUBTITLE_TOOLS_WORKDIR"
	// Translate tuning flags.
	envTranslateAPIKey        = "SUBTITLE_TOOLS_TRANSLATE_API_KEY"
	envTranslateModel         = "SUBTITLE_TOOLS_TRANSLATE_MODEL"
	envTranslateBaseURL       = "SUBTITLE_TOOLS_TRANSLATE_URL"
	envTranslateMaxBatchChars = "SUBTITLE_TOOLS_TRANSLATE_MAX_BATCH_CHARS"
	envTranslateMaxWorkers    = "SUBTITLE_TOOLS_TRANSLATE_MAX_WORKERS"
	envTranslateRPS           = "SUBTITLE_TOOLS_TRANSLATE_RPS"
	envTranslateRetryMax      = "SUBTITLE_TOOLS_TRANSLATE_RETRY_MAX_ATTEMPTS"
	envTranslateRetryParseMax = "SUBTITLE_TOOLS_TRANSLATE_RETRY_PARSE_MAX_ATTEMPTS"
)

func parseEnvBool(key string) (bool, bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return false, false, nil
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return false, false, nil
	}

	switch strings.ToLower(v) {
	case "1", "t", "true", "y", "yes", "on":
		return true, true, nil
	case "0", "f", "false", "n", "no", "off":
		return false, true, nil
	default:
		// Try Go's bool parser too (covers True/False etc.)
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, false, fmt.Errorf("invalid %s=%q (expected true/false)", key, v)
		}
		return b, true, nil
	}
}

func envString(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", false
	}
	return v, true
}

func resolveBoolFlagFromEnv(cmd *cobra.Command, flagName, envKey string) error {
	f := cmd.Flags().Lookup(flagName)
	if f == nil {
		return nil
	}
	// If CLI flag was provided, it wins.
	if f.Changed {
		return nil
	}
	b, ok, err := parseEnvBool(envKey)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return cmd.Flags().Set(flagName, strconv.FormatBool(b))
}

func resolveStringFlagFromEnv(cmd *cobra.Command, flagName, envKey string) error {
	f := cmd.Flags().Lookup(flagName)
	if f == nil {
		return nil
	}
	// If CLI flag was provided, it wins.
	if f.Changed {
		return nil
	}
	v, ok := envString(envKey)
	if !ok {
		return nil
	}
	return cmd.Flags().Set(flagName, v)
}

func resolveIntFlagFromEnv(cmd *cobra.Command, flagName, envKey string) error {
	f := cmd.Flags().Lookup(flagName)
	if f == nil {
		return nil
	}
	// If CLI flag was provided, it wins.
	if f.Changed {
		return nil
	}
	v, ok := envString(envKey)
	if !ok {
		return nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("invalid %s=%q (expected integer): %w", envKey, v, err)
	}
	return cmd.Flags().Set(flagName, strconv.Itoa(i))
}

func resolveFloat64FlagFromEnv(cmd *cobra.Command, flagName, envKey string) error {
	f := cmd.Flags().Lookup(flagName)
	if f == nil {
		return nil
	}
	// If CLI flag was provided, it wins.
	if f.Changed {
		return nil
	}
	v, ok := envString(envKey)
	if !ok {
		return nil
	}
	fl, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fmt.Errorf("invalid %s=%q (expected float): %w", envKey, v, err)
	}
	return cmd.Flags().Set(flagName, strconv.FormatFloat(fl, 'f', -1, 64))
}

var rootCmd = &cobra.Command{
	Use:           "subtitle-tools",
	Short:         "Command-line tools for working with subtitle file",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Allow configuring verbosity via env var when the flag isn't provided.
		if err := resolveBoolFlagFromEnv(cmd, "verbose", envVerbose); err != nil {
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
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose (debug) logging")

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
