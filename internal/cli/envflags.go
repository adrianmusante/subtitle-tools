package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	envVerbose = "SUBTITLE_TOOLS_VERBOSE"
	envDryRun  = "SUBTITLE_TOOLS_DRY_RUN"
	envWorkdir = "SUBTITLE_TOOLS_WORKDIR"
	// Translate tuning flags.
	envTranslateAPIKey         = "SUBTITLE_TOOLS_TRANSLATE_API_KEY"
	envTranslateModel          = "SUBTITLE_TOOLS_TRANSLATE_MODEL"
	envTranslateBaseURL        = "SUBTITLE_TOOLS_TRANSLATE_URL"
	envTranslateMaxBatchChars  = "SUBTITLE_TOOLS_TRANSLATE_MAX_BATCH_CHARS"
	envTranslateMaxWorkers     = "SUBTITLE_TOOLS_TRANSLATE_MAX_WORKERS"
	envTranslateRPS            = "SUBTITLE_TOOLS_TRANSLATE_RPS"
	envTranslateRetryMax       = "SUBTITLE_TOOLS_TRANSLATE_RETRY_MAX_ATTEMPTS"
	envTranslateRetryParseMax  = "SUBTITLE_TOOLS_TRANSLATE_RETRY_PARSE_MAX_ATTEMPTS"
	envTranslateRequestTimeout = "SUBTITLE_TOOLS_TRANSLATE_REQUEST_TIMEOUT"
)

const (
	flagApiKey           = "api-key"
	flagDryRun           = "dry-run"
	flagMaxBatchChars    = "max-batch-chars"
	flagMaxLineLen       = "max-line-len"
	flagMaxWorkers       = "max-workers"
	flagMinWordsMerge    = "min-words-merge"
	flagModel            = "model"
	flagOutput           = "output"
	flagOutputShorthand  = "o"
	flagRPS              = "rps"
	flagRequestTimeout   = "request-timeout"
	flagRetryMax         = "retry-max-attempts"
	flagRetryParseMax    = "retry-parse-max-attempts"
	flagSkipBackup       = "skip-backup"
	flagSourceLanguage   = "source-language"
	flagStripStyle       = "strip-style"
	flagTargetLanguage   = "target-language"
	flagURL              = "url"
	flagVerbose          = "verbose"
	flagVerboseShorthand = "v"
	flagWorkdir          = "workdir"
	flagWorkdirShorthand = "w"
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

func resolveDurationFlagFromEnv(cmd *cobra.Command, flagName, envKey string) error {
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
	dur, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("invalid %s=%q (expected duration, e.g. 30s): %w", envKey, v, err)
	}
	return cmd.Flags().Set(flagName, dur.String())
}
