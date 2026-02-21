package cli

import (
	"github.com/adrianmusante/subtitle-tools/internal/fs"
	"github.com/adrianmusante/subtitle-tools/internal/logging"
	"github.com/adrianmusante/subtitle-tools/internal/run"
	"github.com/adrianmusante/subtitle-tools/internal/update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Download and replace the CLI with the latest version from GitHub releases",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := resolveBoolFlagFromEnv(cmd, flagDryRun, envDryRun); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, flagWorkdir, envWorkdir); err != nil {
			return err
		}
		if err := resolveStringFlagFromEnv(cmd, flagApiKey, envGithubAPIKey); err != nil {
			return err
		}

		apiKey, _ := cmd.Flags().GetString(flagApiKey)
		workdir, _ := cmd.Flags().GetString(flagWorkdir)
		dryRun, _ := cmd.Flags().GetBool(flagDryRun)
		ctx := cmd.Context()
		log := logging.FromContext(ctx)

		if workdir != "" {
			absWorkdir, err := fs.ResolveAbsPath(workdir)
			if err != nil {
				return err
			}
			workdir = absWorkdir
		}

		runWorkdir, cleanup, err := run.NewWorkdir(workdir, "update")
		if err != nil {
			return err
		}
		log.Debug("using workdir", "workdir", runWorkdir)
		if !dryRun { // Only defer cleanup if not dry-run, so we can inspect files afterwards.
			defer cleanup()
		}

		res, err := update.Run(ctx, update.Options{
			APIKey:         apiKey,
			CurrentVersion: version,
			DryRun:         dryRun,
			WorkDir:        runWorkdir,
		})
		if err != nil {
			return err
		}
		if res.Updated {
			log.Info("updated subtitle-tools", "version", res.Version, "asset", res.AssetName, "path", res.ExePath)
			return nil
		}

		log.Info("already up to date", "version", res.Version)
		return nil
	},
}

func init() {
	updateCmd.Flags().Bool(flagDryRun, false, "Download the update to a temporary file but do not replace the current executable")
	updateCmd.Flags().StringP(flagWorkdir, flagWorkdirShorthand, "", "Working directory base. If set, a unique subdirectory is created per run")
	updateCmd.Flags().String(flagApiKey, "", "GitHub API key (optional; helps avoid rate limits)")
}
