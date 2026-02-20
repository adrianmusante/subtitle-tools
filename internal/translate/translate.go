package translate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adrianmusante/subtitle-tools/internal/fs"
	"github.com/adrianmusante/subtitle-tools/internal/run"
	"github.com/adrianmusante/subtitle-tools/internal/srt"
	"golang.org/x/time/rate"
)

type Options struct {
	InputPath      string
	OutputPath     string
	DryRun         bool
	WorkDir        string
	SourceLanguage string
	TargetLanguage string
	APIKey         string
	Model          string
	BaseURL        string
	RequestTimeout time.Duration

	// batching
	MaxBatchChars int // soft limit for payload size

	// execution
	MaxWorkers int     // number of concurrent batches
	RPS        float64 // requests per second (0 disables rate limiting)

	// retry
	// RetryMaxAttempts controls how many attempts are made for retryable errors.
	// Must be >= 1.
	RetryMaxAttempts int

	// RetryParseMaxAttempts controls how many attempts are made for a batch when
	// the model returns an invalid/unparseable response (e.g. ParseTranslatedLines
	// fails or the output doesn't match the requested idx set).
	// Must be >= 1.
	RetryParseMaxAttempts int
}

type Result struct {
	WrittenPath string
	Batches     int
}

const DefaultRequestTimeout = 150 * time.Second
const DefaultMaxBatchChars = 7_000
const DefaultMaxWorkers = 2
const DefaultRequestPerSecond = 4
const DefaultParseRetryMaxAttempts = 2

func Run(ctx context.Context, opts Options) (Result, error) {
	opts, err := validateAndDefaultOptions(opts)
	if err != nil {
		return Result{}, err
	}

	slog.Info("reading subtitles for translation",
		"input_path", opts.InputPath,
		"source_language", normalizeTargetLanguageLabel(opts.SourceLanguage),
		"target_language", normalizeTargetLanguageLabel(opts.TargetLanguage))

	subs, err := readSubtitles(opts.InputPath)
	if err != nil {
		return Result{}, err
	}

	retryOptions := DefaultRetryOptions()
	retryOptions.MaxAttempts = opts.RetryMaxAttempts
	client := OpenAIClient{
		BaseURL: opts.BaseURL, APIKey: opts.APIKey, Model: opts.Model,
		Timeout:      opts.RequestTimeout,
		RetryOptions: retryOptions,
	}

	batches, err := buildBatches(subs, opts.MaxBatchChars)
	if err != nil {
		return Result{}, err
	}

	translatedTexts, err := translateBatches(ctx, opts, &client, batches)
	if err != nil {
		return Result{}, err
	}

	outSubs := applyTranslations(subs, translatedTexts)

	writtenPath, err := writeOutput(opts, outSubs)
	if err != nil {
		return Result{}, err
	}

	return Result{WrittenPath: writtenPath, Batches: len(batches)}, nil
}

type batch struct {
	idxs  []int
	texts []string
}

func validateAndDefaultOptions(opts Options) (Options, error) {
	if opts.InputPath == "" {
		return Options{}, errors.New("input path is required")
	}
	if opts.WorkDir == "" {
		return Options{}, errors.New("workdir is required")
	}
	if opts.TargetLanguage == "" {
		return Options{}, errors.New("target language is required")
	}
	if opts.Model == "" {
		return Options{}, errors.New("model is required")
	}
	if opts.MaxBatchChars <= 0 {
		opts.MaxBatchChars = DefaultMaxBatchChars
	}
	if opts.MaxWorkers <= 0 {
		opts.MaxWorkers = DefaultMaxWorkers
	}
	if opts.RetryMaxAttempts <= 0 {
		opts.RetryMaxAttempts = 1 // at least one attempt
	}
	if opts.RetryParseMaxAttempts <= 0 {
		opts.RetryParseMaxAttempts = 1 // at least one attempt
	}
	if opts.RequestTimeout < 0 { //
		opts.RequestTimeout = 0 // disable timeout if negative
	}
	if opts.OutputPath == "" {
		return Options{}, errors.New("output is required")
	}
	return opts, nil
}

func readSubtitles(inputPath string) ([]*srt.Subtitle, error) {
	in, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer fs.CloseOrLog(in, inputPath)

	subs, err := srt.ReadAll(in)
	if err != nil {
		return nil, err
	}
	err = srt.ValidateSequentialIdx(subs)
	if err != nil {
		slog.Warn("invalid subtitles index; reindexing...", "err", err)
		srt.Reindex(subs)
	}
	return subs, nil
}

func buildBatches(subs []*srt.Subtitle, maxBatchChars int) ([]batch, error) {
	var batches []batch
	for start := 0; start < len(subs); {
		idxs, texts, next, err := buildBatch(subs, start, maxBatchChars)
		if err != nil {
			return nil, err
		}
		batches = append(batches, batch{idxs: idxs, texts: texts})
		start = next
	}
	return batches, nil
}

func translateBatches(
	ctx context.Context,
	opts Options,
	client *OpenAIClient,
	batches []batch,
) (map[int]string, error) {
	translatedTexts := make(map[int]string)
	var translatedMu sync.Mutex

	jobs := make(chan batch)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	limiter := newLimiter(opts.RPS)

	remaining := atomic.Int64{}
	remaining.Store(int64(len(batches)))

	parseRetry := RetryOptions{
		MaxAttempts: opts.RetryParseMaxAttempts,
		BaseDelay:   250 * time.Millisecond,
		MaxDelay:    3 * time.Second,
		Jitter:      0.2,
	}

	worker := func() {
		for b := range jobs {
			n := remaining.Add(-1)
			slog.Info("Processing batch...", "batch_size", len(b.idxs), "remaining_batches", n)
			if err := runOneBatch(ctx, limiter, client, opts.SourceLanguage, opts.TargetLanguage, b, parseRetry, &translatedMu, translatedTexts); err != nil {
				reportWorkerErrorAndCancel(cancel, errCh, err)
				return
			}
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < opts.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker()
		}()
	}

	go enqueueBatches(ctx, jobs, batches)

	wg.Wait()
	if err := firstErr(errCh); err != nil {
		return nil, err
	}
	if err := nonCanceledContextErr(ctx); err != nil {
		return nil, err
	}

	return translatedTexts, nil
}

func newLimiter(rps float64) *rate.Limiter {
	if rps <= 0 {
		return nil
	}
	return rate.NewLimiter(rate.Limit(rps), 1)
}

func enqueueBatches(ctx context.Context, jobs chan<- batch, batches []batch) {
	defer close(jobs)
	for _, b := range batches {
		select {
		case <-ctx.Done():
			return
		case jobs <- b:
		}
	}
}

func reportWorkerErrorAndCancel(cancel context.CancelFunc, errCh chan<- error, err error) {
	select {
	case errCh <- err:
	default:
	}
	cancel()
}

func firstErr(errCh <-chan error) error {
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func nonCanceledContextErr(ctx context.Context) error {
	if ctx.Err() == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return ctx.Err()
}

func runOneBatch(
	ctx context.Context,
	limiter *rate.Limiter,
	client *OpenAIClient,
	sourceLanguage string,
	targetLanguage string,
	b batch,
	parseRetry RetryOptions,
	translatedMu *sync.Mutex,
	translatedTexts map[int]string,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}
	}

	payload, err := FormatForTranslation(b.idxs, b.texts)
	if err != nil {
		return err
	}

	// Defensive defaults.
	if parseRetry.MaxAttempts <= 0 {
		parseRetry.MaxAttempts = 1
	}

	expected := make(map[int]struct{}, len(b.idxs))
	for _, id := range b.idxs {
		expected[id] = struct{}{}
	}

	// Retry only when the model response is invalid/unparseable or doesn't match
	// the expected idx set. Network/HTTP retries are handled inside TranslateBatch.

	var lastParseErr error
	for attempt := 1; attempt <= parseRetry.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		resp, err := client.TranslateBatch(ctx, sourceLanguage, targetLanguage, payload)
		if err != nil {
			return err
		}

		slog.Debug("received translation response", "request", payload, "response", resp, "batch_size", len(b.idxs), "attempt", attempt)

		parsed, err := ParseTranslatedLines(resp)
		if err != nil {
			lastParseErr = err
			if attempt < parseRetry.MaxAttempts {
				slog.Warn("invalid translation output; retrying batch", "attempt", attempt, "max_attempts", parseRetry.MaxAttempts, "err", err)
				if err := sleepWithContext(ctx, computeBackoff(attempt, parseRetry)); err != nil {
					return err
				}
				continue
			}
			return err
		}

		validated, err := validateParsedBatch(expected, b.idxs, parsed)
		if err != nil {
			lastParseErr = err
			if attempt < parseRetry.MaxAttempts {
				slog.Warn("unexpected translation output; retrying batch", "attempt", attempt, "max_attempts", parseRetry.MaxAttempts, "err", err)
				if err := sleepWithContext(ctx, computeBackoff(attempt, parseRetry)); err != nil {
					return err
				}
				continue
			}
			return err
		}

		translatedMu.Lock()
		for _, pl := range validated {
			translatedTexts[pl.Idx] = pl.Text
		}
		translatedMu.Unlock()
		return nil
	}

	if lastParseErr != nil {
		return lastParseErr
	}
	return errors.New("translation batch failed for unknown reasons")
}

func validateParsedBatch(expected map[int]struct{}, idxs []int, parsed []ParsedLine) ([]ParsedLine, error) {
	if len(parsed) != len(idxs) {
		return nil, fmt.Errorf("batch size mismatch: expected %d lines, got %d", len(idxs), len(parsed))
	}
	// Ensure all parsed entries are expected and unique.
	seen := make(map[int]struct{}, len(parsed))
	for _, pl := range parsed {
		if _, ok := expected[pl.Idx]; !ok {
			return nil, fmt.Errorf("unexpected idx in translated output: %d", pl.Idx)
		}
		if _, dup := seen[pl.Idx]; dup {
			return nil, fmt.Errorf("duplicate idx in translated output: %d", pl.Idx)
		}
		seen[pl.Idx] = struct{}{}
	}
	if len(seen) != len(expected) {
		// Missing some expected idxs.
		return nil, fmt.Errorf("translated output missing %d idxs", len(expected)-len(seen))
	}
	return parsed, nil
}

func applyTranslations(subs []*srt.Subtitle, translatedTexts map[int]string) []*srt.Subtitle {
	outSubs := make([]*srt.Subtitle, 0, len(subs))
	for _, s := range subs {
		nt := *s
		if t, ok := translatedTexts[s.Idx]; ok {
			nt.Text = t
		}
		outSubs = append(outSubs, &nt)
	}
	return outSubs
}

func writeOutput(opts Options, subs []*srt.Subtitle) (string, error) {
	tmpOutputPath, err := writeTempOutput(opts, subs)
	if err != nil {
		return "", err
	}

	outputPath := opts.OutputPath
	if opts.DryRun {
		outputPath = tmpOutputPath
	} else {
		if err := fs.RenameOrMove(tmpOutputPath, outputPath); err != nil {
			return "", err
		}
	}
	return outputPath, nil
}

func writeTempOutput(opts Options, subs []*srt.Subtitle) (string, error) {
	namer := run.NewTempNamer(opts.WorkDir, opts.InputPath)
	tmpOutputPath := namer.Step("output")

	fout, err := os.Create(tmpOutputPath)
	if err != nil {
		return "", err
	}
	defer fs.CloseOrLog(fout, tmpOutputPath)

	if err := srt.WriteAll(fout, subs); err != nil {
		return "", err
	}

	return tmpOutputPath, nil
}

func buildBatch(subs []*srt.Subtitle, start int, maxChars int) (idxs []int, texts []string, next int, err error) {
	// Rough estimate: for NDJSON each entry is a JSON object + a newline.
	// We'll compute using the same encoding used by FormatForTranslation.
	chars := 0
	for i := start; i < len(subs); i++ {
		// Always include at least one.
		idx := subs[i].Idx
		text := subs[i].Text
		enc, formatErr := FormatOneForTranslation(idx, text)
		if formatErr != nil {
			return nil, nil, start, fmt.Errorf("format translation line for idx %d: %w", idx, formatErr)
		}
		lineLen := len(enc) + 1
		if i > start && chars+lineLen > maxChars {
			return idxs, texts, i, nil
		}
		idxs = append(idxs, idx)
		texts = append(texts, text)
		chars += lineLen
	}
	return idxs, texts, len(subs), nil
}
