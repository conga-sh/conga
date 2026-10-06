---
title: Errors and exit codes
description: Categorized base errors, the usage/runtime classification contract, and the 0/1/2 exit code contract.
---

CONGA classifies every failure into five base categories, one per pipeline stage. Match the category; the message carries the specifics (field, token, command path, suggestions).

## Exit codes

`app.Exit(err)` terminates with one of three codes:

| Code | Meaning |
|------|---------|
| **0** | Success |
| **1** | Command runtime failure (`Runner.Run` or lifecycle hooks), schema-build failure (`ErrSchema`), or configuration failure (`ErrConfig`) |
| **2** | Usage error: unknown flags, coercion failures, constraint violations, missing required input — and internal completion failures with a diagnostic on stderr |

Built-in requests (`--help`, `help`, `--version`, `version`, and — when enabled with `app.EnableWhy()` / `app.EnableCompletion()` — `--why` and `--completion <shell>`) render their output to stdout during `app.Run()` and return `nil` — they are normal execution, not errors.

## Base categories

| Sentinel | Stage |
|----------|-------|
| `conga.ErrSchema` | Schema building: tag linting, duplicates, invalid order (programmer errors at startup) |
| `conga.ErrParse` | Command-line tokenization and parsing |
| `conga.ErrCoerce` | String → typed value conversion |
| `conga.ErrValidation` | Constraints and semantic checks |
| `conga.ErrConfig` | Config file and `.env` reading, parsing, and key resolution |

## Usage and completion classification

Use `conga.IsUsageError(err)` to decide whether an error is input-related (exit 2) versus a runtime failure (exit 1). Errors caused by invalid command-line input satisfy the `conga.UsageError` interface:

```go
type UsageError interface {
	error
	CommandPath() string
	Usage() string
}
```

- `CommandPath()` returns the invocation path of the command where the failure occurred (for example, `"cloudctl deploy"`).
- `Usage()` returns the syntax usage summary string for that command.

Internal completion failures satisfy the `conga.CompletionError` interface and are detected with `conga.IsCompletionError(err)`:

```go
type CompletionError interface {
	error
	IsCompletionError() bool
}
```

The concrete error implementations (parse, coercion, validation, config) are unexported; `IsUsageError`, `IsCompletionError`, `UsageError`, `CompletionError`, and the base category sentinels form the public classification API.

## Replacing app.Exit

When an application requires custom process termination, error formatting, or telemetry, inspect errors returned from `app.Run()` or `app.RunContext(ctx)` directly rather than calling `app.Exit(err)`:

```go
if err := app.RunContext(ctx); err != nil {
	if usageErr, ok := errors.AsType[conga.UsageError](err); ok {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n%s\n", err, usageErr.Usage())
		os.Exit(2)
	}

	if conga.IsCompletionError(err) {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(2)
	}

	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
```

## Usage rendering on errors

`UsageOnError(mode)` controls what `Exit` prints below a usage error:

| Mode | Output |
|------|--------|
| `conga.UsageSummary` (default) | Clean error, 1–2 line syntax, `--help` hint |
| `conga.UsageFull` | Clean error plus the complete help screen |
| `conga.UsageHint` | Clean error plus a single `Run '<cmd> --help'` hint |
| `conga.UsageSilence` | Only the clean error message |

Self-describing value errors render only the error line regardless of the configured mode. These are errors whose message already names the flag/argument and the offending value — coercion failures (`unknown byte size unit "XB" in "5XB" for flag --max-memory`) and value-constraint violations (`"/etc/passwd" is a file, but a directory was expected for flag --dir`, `score 150 must be between 0 and 100 for flag --score`). Errors that benefit from usage context — unknown commands and flags, missing required flags/arguments, mutually exclusive or grouped flags — keep the mode's rendering.
