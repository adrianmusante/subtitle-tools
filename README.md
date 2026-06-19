# subtitle-tools

CLI toolkit for cleaning, fixing, and translating subtitles.

## Install

```bash
go install github.com/adrianmusante/subtitle-tools/cmd/subtitle-tools@latest
```

Alternatively, download a pre-built binary from [GitHub Releases](https://github.com/adrianmusante/subtitle-tools/releases) and place it on your `PATH`.

## CLI Reference

Command-line tools for working with subtitle file.

### Top-level

General flags for `subtitle-tools`

Flags:

| Flag            | Environment variable     | Description                       | Type | Default |
|-----------------|--------------------------|-----------------------------------|------|---------|
| `-h, --help`    |                          | Show help for `subtitle-tools`    | bool | `false` |
| `-v, --verbose` | `SUBTITLE_TOOLS_VERBOSE` | Enable verbose (debug) logging    | bool | `false` |
| `--version`     |                          | Show version for `subtitle-tools` | bool | `false` |

Usage:

```text
subtitle-tools [flags]
subtitle-tools [command]
```

### fix

Fixes common issues in `.srt` files.

Fixes:
- overlaps: subtitles sharing the same time span.
- out of order: subtitles not sorted by time.
- cue index: invalid sequence of cue indices.
- line wrap: rewraps lines that may exceed typical screen width.
- style stripping: removes styling such as HTML tags (when enabled).
- HI stripping: removes hearing-impaired cues (when enabled).
- empty cues: removes subtitles with no text.
- decoration-only cues: removes cues that contain only decorative symbols (e.g. music notes) and no other text.
- deduplication: removes duplicated subtitles.
- time shifting: shifts all cue times by a specified duration (when enabled).

#### Usage:

```text
subtitle-tools fix [flags] <input-file>
```

Flags:

| Flag                | Environment variable     | Description                                                                       | Type     | Default    |
|---------------------|--------------------------|-----------------------------------------------------------------------------------|----------|------------|
| `--dry-run`         | `SUBTITLE_TOOLS_DRY_RUN` | Write output to a temporary file and do not overwrite the original                | bool     | `false`    |
| `--max-line-len`    |                          | Max line length when wrapping                                                     | int      | `70`       |
| `--min-words-merge` |                          | Minimum words to consider a line short for merging                                | int      | `3`        |
| `-o, --output`      |                          | Output file path (defaults to overwriting input)                                  | string   |            |
| `--shift-time`      |                          | Shift all cue times by the specified duration (e.g. 500ms, -2s, 1s250ms)          | duration | `0s`       |
| `--skip-backup`     |                          | Do not create a .bak backup when overwriting the input file                       | bool     | `false`    |
| `--strip-hi`        |                          | Remove hearing-impaired cues (e.g. [music])                                       | bool     | `false`    |
| `--strip-hi-mode`   |                          | HI stripping mode: safe, standard, safe-plus, standard-plus                       | string   | `standard` |
| `--strip-style`     |                          | Remove HTML/XML style tags from subtitle text                                     | bool     | `false`    |
| `-w, --workdir`     | `SUBTITLE_TOOLS_WORKDIR` | Working directory base; unique subdirectory per run                               | string   |            |

Behavior:
- If `-o/--output` is omitted, `fix` overwrites the input file.
- When overwriting the input file, a `*.bak` backup is created by default. Use `--skip-backup` to disable it.
- If `--dry-run` is set, the original file is never modified; output is written to a temporary file.
- If `-w/--workdir` is provided, a unique subdirectory is created inside it per run.
  If omitted, a system temp directory is used and deleted at the end.
- If `--strip-style` is set, all styling (e.g. HTML tags) is removed from subtitle lines.
- If `--strip-hi` is set, HI cues are removed after style stripping.
- `--strip-hi-mode safe` is conservative: strips only `[]` cues with low risk of over-cleaning.
- `--strip-hi-mode standard` (default) is more thorough: strips `[]` cues including speaker prefixes and inline cues.
- `--strip-hi-mode safe-plus` strips `[]`, `()`, and `{}` while preserving safe behavior.
- `--strip-hi-mode standard-plus` strips `[]`, `()`, and `{}` with full standard cleanup.
- All HI stripping modes preserve leading dialogue dashes (e.g. `- Thank you.`).
- Music symbols (`♪`, `♫`) are preserved when the line has content (e.g. lyrics), while empty music-only lines are removed.

Examples:

```bash
subtitle-tools fix --strip-hi input.srt
subtitle-tools fix --strip-hi --strip-hi-mode safe input.srt
subtitle-tools fix --strip-hi --strip-hi-mode standard input.srt
subtitle-tools fix --strip-hi --strip-hi-mode safe-plus input.srt
subtitle-tools fix --strip-hi --strip-hi-mode standard-plus input.srt
```

When to use each mode:
- Use `standard` (default) for most files: strips `[]` cues including speaker prefixes.
- Use `safe` when you want minimal intervention and only obvious `[]` cues removed.
- Use `safe-plus` or `standard-plus` when you also want to strip `()` and `{}` cues.
- If unsure, run with `--dry-run` first and inspect the result.

Practical cases:

| Input pattern (example)                        | Recommended mode              | Why                                                                 |
|------------------------------------------------|-------------------------------|---------------------------------------------------------------------|
| `[MUSIC]` on its own line                      | `standard` (default)          | Removes obvious HI-only cues reliably.                              |
| `JOHN: Hello there.`                           | `standard`                    | Speaker prefix is stripped, keeping only the dialogue.              |
| `(whispers) Hello` or `{door closes} Hello`    | `safe-plus`                   | Trims extended HI context while keeping dialogue.                   |
| `JOHN: ♪ (whispers) run! [door slams]`         | `standard-plus`               | Best for heavily cluttered lines when clean dialogue is preferred.  |
| Mixed style + HI: `<i>[MUSIC]</i>`             | `--strip-style` + `standard`  | First removes tags, then strips base HI cues.                       |
| Ambiguous speaker text: `MARIA: We should go.` | `safe` + `--dry-run`          | Avoids over-cleaning when speaker labels may be meaningful.         |

### translate

Translate subtitles to another language using an OpenAI-compatible API

> [!TIP]
> Run `fix` before `translate` to clean the subtitle file. This can improve translation quality and reduce errors from non-standard formatting.
> After translating, you can run `fix` again to apply line wrap to the translated text.

#### Usage:

```text
subtitle-tools translate [flags] <input-file>
```

Flags:

| Flag                         | Environment variable                                | Description                                                              | Type     | Default  |
|------------------------------|-----------------------------------------------------|--------------------------------------------------------------------------|----------|----------|
| `--api-key`                  | `SUBTITLE_TOOLS_TRANSLATE_API_KEY`                  | API key; comma-separated list distributes requests across keys           | string   |          |
| `--dry-run`                  | `SUBTITLE_TOOLS_DRY_RUN`                            | Write output to a temporary file and do not create the final output file | bool     | `false`  |
| `--max-batch-chars`          | `SUBTITLE_TOOLS_TRANSLATE_MAX_BATCH_CHARS`          | Soft limit for the batch payload size                                    | int      | `7000`   |
| `--max-workers`              | `SUBTITLE_TOOLS_TRANSLATE_MAX_WORKERS`              | Number of concurrent translation workers (batches in-flight)             | int      | `2`      |
| `--model`                    | `SUBTITLE_TOOLS_TRANSLATE_MODEL`                    | Model to use (e.g. gpt-5, gemini-flash-latest)                           | string   | required |
| `-o, --output`               |                                                     | Output file path; must not already exist                                 | string   | required |
| `--request-timeout`          | `SUBTITLE_TOOLS_TRANSLATE_REQUEST_TIMEOUT`          | HTTP request timeout duration (e.g. 30s, 1m; 0 disables timeout)         | duration | `2m30s`  |
| `--retry-max-attempts`       | `SUBTITLE_TOOLS_TRANSLATE_RETRY_MAX_ATTEMPTS`       | Max attempts per request for retryable errors                            | int      | `5`      |
| `--retry-parse-max-attempts` | `SUBTITLE_TOOLS_TRANSLATE_RETRY_PARSE_MAX_ATTEMPTS` | Max attempts per batch when model output is invalid/unparseable          | int      | `2`      |
| `--rps`                      | `SUBTITLE_TOOLS_TRANSLATE_RPS`                      | Max requests per second (0 disables rate limiting)                       | float    | `4`      |
| `--source-language`          |                                                     | Source language. If omitted, it’s auto-detected. (e.g. es, es-MX, fr)    | string   |          |
| `--target-language`          |                                                     | Target language (e.g. es, es-MX, fr)                                     | string   | required |
| `--url`                      | `SUBTITLE_TOOLS_TRANSLATE_URL`                      | Base URL for the API endpoint (inferred from --model if omitted)         | string   |          |
| `-w, --workdir`              | `SUBTITLE_TOOLS_WORKDIR`                            | Working directory base; unique subdirectory per run                      | string   |          |

### update

Download and replace the CLI with the latest version.

#### Usage:

```text
subtitle-tools update [flags]
```

Flags:

| Flag            | Environment variable            | Description                                                              | Type   | Default |
|-----------------|---------------------------------|--------------------------------------------------------------------------|--------|---------|
| `--api-key`     | `SUBTITLE_TOOLS_GITHUB_API_KEY` | GitHub API key (optional; helps avoid rate limits)                       | string |         |
| `--dry-run`     | `SUBTITLE_TOOLS_DRY_RUN`        | Download the update but do not replace the current executable            | bool   | `false` |
| `-w, --workdir` | `SUBTITLE_TOOLS_WORKDIR`        | Working directory base; unique subdirectory per run                      | string |         |

## Configuration (environment variables)

You can provide some flag values via environment variables.

Precedence:
1. CLI flag value (if explicitly provided)
2. Environment variable
3. Flag default

> **Bool values accept:** `true/false`, `1/0`, `yes/no`, `on/off`.
