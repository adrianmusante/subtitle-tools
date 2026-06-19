# Fix CLI regression cases

Each case lives in `cases/<name>/` and contains:

- `case.json`: CLI args and expected target file.
- `input.srt`: input subtitle fixture.
- `expected.srt`: expected output after running `fix`.

`case.json` schema:

```json
{
  "args": ["--strip-hi", "--strip-hi-mode", "safe", "{{input}}"],
  "expected_target": "input"
}
```

Placeholders supported in `args`:

- `{{input}}`: temp copy of `input.srt`
- `{{output}}`: temp output path (for `-o/--output` cases)
- `{{workdir}}`: temp workdir path

To update expected files after intentional behavior changes:

```bash
UPDATE_FIX_GOLDEN=1 go test ./internal/cli -run TestFixCLI_RegressionCases
```

