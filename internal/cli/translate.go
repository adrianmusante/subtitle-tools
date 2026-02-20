package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/adrianmusante/subtitle-tools/internal/fs"
	"github.com/adrianmusante/subtitle-tools/internal/logging"
	"github.com/adrianmusante/subtitle-tools/internal/run"
	"github.com/adrianmusante/subtitle-tools/internal/translate"
	"github.com/spf13/cobra"
)

var translateCmd = &cobra.Command{
	Use:   "translate [flags] <input-file>",
	Short: "Translate subtitles to another language using an OpenAI-compatible API",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Allow resolving some flags from env vars.
		if err := resolveBoolFlagFromEnv(cmd, flagDryRun, envDryRun); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, flagWorkdir, envWorkdir); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, flagApiKey, envTranslateAPIKey); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, flagModel, envTranslateModel); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, flagURL, envTranslateBaseURL); err != nil {
			return err
		}
		if err := resolveIntFlagFromEnv(cmd, flagMaxBatchChars, envTranslateMaxBatchChars); err != nil {
			return err
		}
		if err := resolveIntFlagFromEnv(cmd, flagMaxWorkers, envTranslateMaxWorkers); err != nil {
			return err
		}
		if err := resolveFloat64FlagFromEnv(cmd, flagRPS, envTranslateRPS); err != nil {
			return err
		}
		if err := resolveIntFlagFromEnv(cmd, flagRetryMax, envTranslateRetryMax); err != nil {
			return err
		}
		if err := resolveIntFlagFromEnv(cmd, flagRetryParseMax, envTranslateRetryParseMax); err != nil {
			return err
		}
		if err := resolveDurationFlagFromEnv(cmd, flagRequestTimeout, envTranslateRequestTimeout); err != nil {
			return err
		}

		ctx := cmd.Context()
		log := logging.FromContext(ctx)

		inputPath := args[0]
		if inputPath == "-" {
			return errors.New("stdin is not supported yet; pass a subtitle file path")
		}
		absInput, err := fs.ResolveAbsPath(inputPath)
		if err != nil {
			return err
		}
		inputPath = absInput

		outputPath, _ := cmd.Flags().GetString("output")
		if outputPath == "" {
			return errors.New("--output is required and must not exist (we never overwrite on translate)")
		}
		absOutput, err := fs.ResolveAbsPath(outputPath)
		if err != nil {
			return err
		}
		outputPath = absOutput
		if _, err := os.Stat(outputPath); err == nil {
			return errors.New("output file already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := fs.ValidatePathWritable(outputPath); err != nil {
			return fmt.Errorf("invalid --output path %s: %w", outputPath, err)
		}

		sourceLang, _ := cmd.Flags().GetString(flagSourceLanguage)
		targetLang, _ := cmd.Flags().GetString(flagTargetLanguage)
		apiKey, _ := cmd.Flags().GetString(flagApiKey)
		model, _ := cmd.Flags().GetString(flagModel)
		baseURL, _ := cmd.Flags().GetString(flagURL)
		dryRun, _ := cmd.Flags().GetBool(flagDryRun)
		workdir, _ := cmd.Flags().GetString(flagWorkdir)
		maxBatchChars, _ := cmd.Flags().GetInt(flagMaxBatchChars)
		maxWorkers, _ := cmd.Flags().GetInt(flagMaxWorkers)
		rps, _ := cmd.Flags().GetFloat64(flagRPS)
		retryMaxAttempts, _ := cmd.Flags().GetInt(flagRetryMax)
		retryParseMaxAttempts, _ := cmd.Flags().GetInt(flagRetryParseMax)
		requestTimeout, _ := cmd.Flags().GetDuration(flagRequestTimeout)

		// Normalize comma-separated api keys early so opts don't carry spaces.
		apiKey = run.NormalizeCSV(apiKey)

		if workdir != "" {
			absWorkdir, err := fs.ResolveAbsPath(workdir)
			if err != nil {
				return err
			}
			workdir = absWorkdir
		}

		runWorkdir, cleanup, err := run.NewWorkdir(workdir, "translate")
		if err != nil {
			return err
		}
		log.Debug("using workdir", "workdir", runWorkdir)
		if !dryRun { // Only defer cleanup if not dry-run, so we can inspect files afterwards.
			defer cleanup()
		}

		opts := translate.Options{
			InputPath:             inputPath,
			OutputPath:            outputPath,
			DryRun:                dryRun,
			WorkDir:               runWorkdir,
			SourceLanguage:        sourceLang,
			TargetLanguage:        targetLang,
			APIKey:                apiKey,
			Model:                 model,
			BaseURL:               baseURL,
			MaxBatchChars:         maxBatchChars,
			MaxWorkers:            maxWorkers,
			RPS:                   rps,
			RetryMaxAttempts:      retryMaxAttempts,
			RetryParseMaxAttempts: retryParseMaxAttempts,
			RequestTimeout:        requestTimeout,
		}

		safeOpts := opts
		safeOpts.APIKey = run.MaskKeys(opts.APIKey, run.CommaSeparator)
		log.Debug("translate run", "opts", safeOpts)

		res, err := translate.Run(ctx, opts)
		if err != nil {
			return err
		}

		log.Info("translated subtitles written", "path", res.WrittenPath, "batches", res.Batches)
		return nil
	},
}

func init() {
	_ = translateCmd.Flags().StringP(flagOutput, flagOutputShorthand, "", "Output file path (required; must not already exist)")
	_ = translateCmd.Flags().String(flagSourceLanguage, "", "Source language (optional; helps disambiguate the input)")
	_ = translateCmd.Flags().String(flagTargetLanguage, "", "Target language (e.g. es, es-MX, fr)")
	_ = translateCmd.Flags().String(flagApiKey, "", "API key. A comma-separated list of keys can be provided to distribute requests across multiple keys")
	_ = translateCmd.Flags().String(flagModel, "", "Model to use (e.g. gpt-5, gemini-flash-latest)")
	_ = translateCmd.Flags().String(flagURL, "", "Base URL for the API endpoint (optional; inferred from --model if omitted)")
	_ = translateCmd.Flags().Bool(flagDryRun, false, "Write output to a temporary file and do not create the final output file")
	_ = translateCmd.Flags().StringP(flagWorkdir, flagWorkdirShorthand, "", "Working directory base. If set, a unique subdirectory is created per run")
	_ = translateCmd.Flags().Int(flagMaxBatchChars, translate.DefaultMaxBatchChars, "Soft limit for the batch payload size")
	_ = translateCmd.Flags().Int(flagMaxWorkers, translate.DefaultMaxWorkers, "Number of concurrent translation workers (batches in-flight)")
	_ = translateCmd.Flags().Float64(flagRPS, translate.DefaultRequestPerSecond, "Max requests per second (0 disables rate limiting)")
	_ = translateCmd.Flags().Int(flagRetryMax, translate.DefaultRetryMaxAttempts, "Max attempts per request for retryable errors")
	_ = translateCmd.Flags().Int(flagRetryParseMax, translate.DefaultParseRetryMaxAttempts, "Max attempts per batch when the model output is invalid/unparseable (ParseTranslatedLines/mismatch)")
	_ = translateCmd.Flags().Duration(flagRequestTimeout, translate.DefaultRequestTimeout, "HTTP request timeout duration (e.g. 30s, 1m; 0 disables timeout)")

	_ = translateCmd.MarkFlagRequired(flagTargetLanguage)
	// NOTE: api-key and model can be provided via env vars, so we validate at runtime.
}
