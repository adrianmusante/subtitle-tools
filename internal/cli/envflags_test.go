package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveBoolFlagFromEnv_FlagTakesPrecedence(t *testing.T) {
	cmd := &cobra.Command{Use: "t", RunE: func(cmd *cobra.Command, args []string) error { return nil }}
	cmd.Flags().Bool("dry-run", false, "")
	_ = cmd.Flags().Set("dry-run", "true")

	t.Setenv(envDryRun, "false")

	if err := resolveBoolFlagFromEnv(cmd, "dry-run", envDryRun); err != nil {
		t.Fatalf("resolveBoolFlagFromEnv: %v", err)
	}

	got, _ := cmd.Flags().GetBool("dry-run")
	if got != true {
		t.Fatalf("expected dry-run=true from flag, got %v", got)
	}
}

func TestResolveBoolFlagFromEnv_UsesEnvWhenFlagMissing(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Bool("dry-run", false, "")

	t.Setenv(envDryRun, "true")

	if err := resolveBoolFlagFromEnv(cmd, "dry-run", envDryRun); err != nil {
		t.Fatalf("resolveBoolFlagFromEnv: %v", err)
	}

	got, _ := cmd.Flags().GetBool("dry-run")
	if got != true {
		t.Fatalf("expected dry-run=true from env, got %v", got)
	}
}

func TestResolveStringFlagFromEnv_FlagTakesPrecedence(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().String("workdir", "", "")
	_ = cmd.Flags().Set("workdir", "/from-flag")

	t.Setenv(envWorkdir, "/from-env")

	if err := resolveStringFlagFromEnv(cmd, "workdir", envWorkdir); err != nil {
		t.Fatalf("resolveStringFlagFromEnv: %v", err)
	}

	got, _ := cmd.Flags().GetString("workdir")
	if got != "/from-flag" {
		t.Fatalf("expected workdir=/from-flag, got %q", got)
	}
}

func TestResolveStringFlagFromEnv_UsesEnvWhenFlagMissing(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().String("workdir", "", "")

	t.Setenv(envWorkdir, "/from-env")

	if err := resolveStringFlagFromEnv(cmd, "workdir", envWorkdir); err != nil {
		t.Fatalf("resolveStringFlagFromEnv: %v", err)
	}

	got, _ := cmd.Flags().GetString("workdir")
	if got != "/from-env" {
		t.Fatalf("expected workdir=/from-env, got %q", got)
	}
}

func TestResolveBoolFlagFromEnv_InvalidValueErrors(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Bool("dry-run", false, "")

	t.Setenv(envDryRun, "nope")

	if err := resolveBoolFlagFromEnv(cmd, "dry-run", envDryRun); err == nil {
		t.Fatalf("expected error for invalid env bool")
	}
}

func TestResolveIntFlagFromEnv_FlagTakesPrecedence(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Int("max-workers", 0, "")
	_ = cmd.Flags().Set("max-workers", "7")

	t.Setenv(envTranslateMaxWorkers, "3")

	if err := resolveIntFlagFromEnv(cmd, "max-workers", envTranslateMaxWorkers); err != nil {
		t.Fatalf("resolveIntFlagFromEnv: %v", err)
	}

	got, _ := cmd.Flags().GetInt("max-workers")
	if got != 7 {
		t.Fatalf("expected max-workers=7 from flag, got %v", got)
	}
}

func TestResolveIntFlagFromEnv_UsesEnvWhenFlagMissing(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Int("max-workers", 0, "")

	t.Setenv(envTranslateMaxWorkers, "3")

	if err := resolveIntFlagFromEnv(cmd, "max-workers", envTranslateMaxWorkers); err != nil {
		t.Fatalf("resolveIntFlagFromEnv: %v", err)
	}

	got, _ := cmd.Flags().GetInt("max-workers")
	if got != 3 {
		t.Fatalf("expected max-workers=3 from env, got %v", got)
	}
}

func TestResolveIntFlagFromEnv_InvalidValueErrors(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Int("max-workers", 0, "")

	t.Setenv(envTranslateMaxWorkers, "nope")

	if err := resolveIntFlagFromEnv(cmd, "max-workers", envTranslateMaxWorkers); err == nil {
		t.Fatalf("expected error for invalid env int")
	}
}

func TestResolveFloat64FlagFromEnv_FlagTakesPrecedence(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Float64("rps", 0, "")
	_ = cmd.Flags().Set("rps", "1.25")

	t.Setenv(envTranslateRPS, "0.5")

	if err := resolveFloat64FlagFromEnv(cmd, "rps", envTranslateRPS); err != nil {
		t.Fatalf("resolveFloat64FlagFromEnv: %v", err)
	}

	got, _ := cmd.Flags().GetFloat64("rps")
	if got != 1.25 {
		t.Fatalf("expected rps=1.25 from flag, got %v", got)
	}
}

func TestResolveFloat64FlagFromEnv_UsesEnvWhenFlagMissing(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Float64("rps", 0, "")

	t.Setenv(envTranslateRPS, "0.5")

	if err := resolveFloat64FlagFromEnv(cmd, "rps", envTranslateRPS); err != nil {
		t.Fatalf("resolveFloat64FlagFromEnv: %v", err)
	}

	got, _ := cmd.Flags().GetFloat64("rps")
	if got != 0.5 {
		t.Fatalf("expected rps=0.5 from env, got %v", got)
	}
}

func TestResolveFloat64FlagFromEnv_InvalidValueErrors(t *testing.T) {
	cmd := &cobra.Command{Use: "t"}
	cmd.Flags().Float64("rps", 0, "")

	t.Setenv(envTranslateRPS, "nope")

	if err := resolveFloat64FlagFromEnv(cmd, "rps", envTranslateRPS); err == nil {
		t.Fatalf("expected error for invalid env float")
	}
}

func TestTranslateCmd_RunE_ResolvesEnvVars(t *testing.T) {
	// Minimal smoke check that translate's RunE resolves env vars BEFORE reading flags.
	// We don't want to execute the full translate.Run (would require an API), so we
	// set args that fail early at --output validation.

	t.Setenv(envTranslateAPIKey, "k")
	t.Setenv(envTranslateModel, "m")
	t.Setenv(envTranslateBaseURL, "http://example")
	t.Setenv(envTranslateMaxBatchChars, "123")
	t.Setenv(envTranslateMaxWorkers, "4")
	t.Setenv(envTranslateRPS, "0.7")

	cmd := &cobra.Command{
		Use:           "translate",
		Args:          cobra.ExactArgs(1),
		RunE:          translateCmd.RunE,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"in.srt"})

	cmd.Flags().StringP("output", "o", "", "")
	cmd.Flags().String("target-language", "", "")
	cmd.Flags().String("api-key", "", "")
	cmd.Flags().String("model", "", "")
	cmd.Flags().String("url", "", "")
	cmd.Flags().Bool("dry-run", false, "")
	cmd.Flags().String("workdir", "", "")
	cmd.Flags().Int("max-batch-chars", 1, "")
	cmd.Flags().Int("max-workers", 1, "")
	cmd.Flags().Float64("rps", 0, "")

	// We expect an error because --output is missing, but env resolution should not error.
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error")
	}
	if err.Error() != "--output is required and must not exist (we never overwrite on translate)" {
		// If this message changes, the important part is that we didn't error out due to missing api-key/model.
		t.Fatalf("unexpected error: %v", err)
	}
}
