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
		if err := resolveBoolFlagFromEnv(cmd, "dry-run", envDryRun); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, "workdir", envWorkdir); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, "api-key", envTranslateAPIKey); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, "model", envTranslateModel); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, "url", envTranslateBaseURL); err != nil {
			return err
		}
		if err := resolveIntFlagFromEnv(cmd, "max-batch-chars", envTranslateMaxBatchChars); err != nil {
			return err
		}
		if err := resolveIntFlagFromEnv(cmd, "max-workers", envTranslateMaxWorkers); err != nil {
			return err
		}
		if err := resolveFloat64FlagFromEnv(cmd, "rps", envTranslateRPS); err != nil {
			return err
		}
		if err := resolveIntFlagFromEnv(cmd, "retry-max-attempts", envTranslateRetryMax); err != nil {
			return err
		}
		if err := resolveIntFlagFromEnv(cmd, "retry-parse-max-attempts", envTranslateRetryParseMax); err != nil {
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

		targetLang, _ := cmd.Flags().GetString("target-language")
		apiKey, _ := cmd.Flags().GetString("api-key")
		model, _ := cmd.Flags().GetString("model")
		baseURL, _ := cmd.Flags().GetString("url")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		workdir, _ := cmd.Flags().GetString("workdir")
		maxBatchChars, _ := cmd.Flags().GetInt("max-batch-chars")
		maxWorkers, _ := cmd.Flags().GetInt("max-workers")
		rps, _ := cmd.Flags().GetFloat64("rps")
		retryMaxAttempts, _ := cmd.Flags().GetInt("retry-max-attempts")
		retryParseMaxAttempts, _ := cmd.Flags().GetInt("retry-parse-max-attempts")

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
			TargetLanguage:        targetLang,
			APIKey:                apiKey,
			Model:                 model,
			BaseURL:               baseURL,
			MaxBatchChars:         maxBatchChars,
			MaxWorkers:            maxWorkers,
			RPS:                   rps,
			RetryMaxAttempts:      retryMaxAttempts,
			RetryParseMaxAttempts: retryParseMaxAttempts,
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
	_ = translateCmd.Flags().StringP("output", "o", "", "Output file path (required; must not already exist)")
	_ = translateCmd.Flags().String("target-language", "", "Target language (e.g. es, es-MX, fr)")
	_ = translateCmd.Flags().String("api-key", "", "API key. A comma-separated list of keys can be provided to distribute requests across multiple keys")
	_ = translateCmd.Flags().String("model", "", "Model to use (e.g. gpt-5, gemini-flash-latest)")
	_ = translateCmd.Flags().String("url", "", "Base URL for the API endpoint (optional; inferred from --model if omitted)")
	_ = translateCmd.Flags().Bool("dry-run", false, "Write output to a temporary file and do not create the final output file")
	_ = translateCmd.Flags().StringP("workdir", "w", "", "Working directory base. If set, a unique subdirectory is created per run")
	_ = translateCmd.Flags().Int("max-batch-chars", translate.DefaultMaxBatchChars, "Soft limit for the batch payload size")
	_ = translateCmd.Flags().Int("max-workers", translate.DefaultMaxWorkers, "Number of concurrent translation workers (batches in-flight)")
	_ = translateCmd.Flags().Float64("rps", translate.DefaultRequestPerSecond, "Max requests per second (0 disables rate limiting)")
	_ = translateCmd.Flags().Int("retry-max-attempts", translate.DefaultRetryMaxAttempts, "Max attempts per request for retryable errors")
	_ = translateCmd.Flags().Int("retry-parse-max-attempts", translate.DefaultParseRetryMaxAttempts, "Max attempts per batch when the model output is invalid/unparseable (ParseTranslatedLines/mismatch)")

	_ = translateCmd.MarkFlagRequired("target-language")
	// NOTE: api-key and model can be provided via env vars, so we validate at runtime.
}
