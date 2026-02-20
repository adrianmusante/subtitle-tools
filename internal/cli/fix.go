package cli

import (
	"context"
	"errors"

	"github.com/adrianmusante/subtitle-tools/internal/fix"
	"github.com/adrianmusante/subtitle-tools/internal/fs"
	"github.com/adrianmusante/subtitle-tools/internal/logging"
	"github.com/adrianmusante/subtitle-tools/internal/run"
	"github.com/spf13/cobra"
)

var fixCmd = &cobra.Command{
	Use:   "fix [flags] <input-file>",
	Short: "Fix common issues in subtitle files (overlaps, out-of-order cues, etc.)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Allow resolving some flags from env vars.
		if err := resolveBoolFlagFromEnv(cmd, flagDryRun, envDryRun); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, flagWorkdir, envWorkdir); err != nil {
			return err
		}

		ctx := cmd.Context()
		log := logging.FromContext(ctx)

		inputPath := args[0]

		outputPath, _ := cmd.Flags().GetString(flagOutput)
		dryRun, _ := cmd.Flags().GetBool(flagDryRun)
		workdir, _ := cmd.Flags().GetString(flagWorkdir)
		skipBackup, _ := cmd.Flags().GetBool(flagSkipBackup)

		minWords, _ := cmd.Flags().GetInt(flagMinWordsMerge)
		maxLineLen, _ := cmd.Flags().GetInt(flagMaxLineLen)
		stripStyle, _ := cmd.Flags().GetBool(flagStripStyle)

		if inputPath == "-" {
			return errors.New("stdin is not supported yet; pass a subtitle file path")
		}

		absInput, err := fs.ResolveAbsPath(inputPath)
		if err != nil {
			return err
		}
		inputPath = absInput

		if outputPath == "" {
			outputPath = inputPath
		} else {
			absOut, err := fs.ResolveAbsPath(outputPath)
			if err != nil {
				return err
			}
			outputPath = absOut
		}

		// Temporarily disabled: failing to write the result is less costly than pre‑validating write access.
		//if err := run.ValidatePathWritable(outputPath); err != nil {
		//	return fmt.Errorf("invalid --output path %s: %w", outputPath, err)
		//}

		if workdir != "" {
			absWorkdir, err := fs.ResolveAbsPath(workdir)
			if err != nil {
				return err
			}
			workdir = absWorkdir
		}

		runWorkdir, cleanup, err := run.NewWorkdir(workdir, "fix")
		if err != nil {
			return err
		}
		log.Debug("using workdir", "workdir", runWorkdir)
		if !dryRun { // Only defer cleanup if not dry-run, so we can inspect files afterwards.
			defer cleanup()
		}

		opts := fix.Options{
			InputPath:      inputPath,
			OutputPath:     outputPath,
			DryRun:         dryRun,
			WorkDir:        runWorkdir,
			MaxLineLength:  maxLineLen,
			MinWordsMerge:  minWords,
			StripStyle:     stripStyle,
			BackupExt:      ".bak",
			CreateBackup:   !dryRun && !skipBackup,
			SkipTranslator: true,
		}

		log.Debug("running fix", "opts", opts)

		result, err := fix.Run(ctx, opts)
		if err != nil {
			return err
		}

		log.Info("fixed subtitles written", "path", result.WrittenPath)

		return nil
	},
}

func init() {
	fixCmd.Flags().StringP(flagOutput, flagOutputShorthand, "", "Output file path (optional; defaults to overwriting input)")
	fixCmd.Flags().Bool(flagDryRun, false, "Write output to a temporary file and do not overwrite the original")
	fixCmd.Flags().Bool(flagSkipBackup, false, "Do not create a .bak backup when overwriting the input file")
	fixCmd.Flags().StringP(flagWorkdir, flagWorkdirShorthand, "", "Working directory base. If set, a unique subdirectory is created per run")

	fixCmd.Flags().Int(flagMinWordsMerge, fix.DefaultMinWordsForMerging, "Minimum words to consider a line 'short' for merging")
	fixCmd.Flags().Int(flagMaxLineLen, fix.DefaultMaxLineLength, "Max line length when wrapping")
	fixCmd.Flags().Bool(flagStripStyle, false, "Remove HTML/XML style tags from subtitle text")
}

// for tests / future hooking
func fixContext() context.Context { return context.Background() }

var _ = fixContext
