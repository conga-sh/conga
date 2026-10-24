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

Built-in requests (`--help`, `help`, `--version`, `version`, and opt-in `--why` and `--completion <shell>`) write to stdout during `app.Run()` and return `nil` on clean completion.

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
	IsUsageError() bool
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

## Feature-gating and map-folding failures

Feature gating and map folding classify errors by the input tier that introduced the invalid data:

| Failure | Category | Exit |
|---------|----------|:----:|
| Inactive gated flag or subcommand supplied on the command line, unknown feature key, or invalid feature value | `ErrParse` / `ErrValidation` (usage error) | 2 |
| Unknown feature key or invalid feature value in a config file | `ErrConfig` | 1 |
| Config key naming a flag whose gating feature is inactive | `ErrConfig` | 1 |
| Environment variable naming an inactive gated flag | Ignored (the pruned flag never reaches environment binding) | – |
| Unknown or invalid feature key in the feature map environment variable | `ErrParse` (usage error) | 2 |
| Malformed map flag syntax in a config file (even when overridden by CLI) | `ErrConfig` | 1 |
| Malformed map flag syntax in an environment variable | `ErrParse` (usage error) | 2 |
| Invalid format passed to `--why` or `--why=json` requested without `conga.WhyJSON()` | `ErrValidation` (usage error) | 2 |
| Runtime error returned from a `conga.ConfigLoader` function | `ErrConfig` | 1 |

## Schema errors (`ErrSchema`)

Programmer errors during schema construction abort at startup before execution begins (exit code 1):
- Declaring `conga.Features[T]` or `feature:"..."` tags without calling `app.EnableFeatures()`.
- Declaring both `env` and `envprefix` on a `conga.Features` container, or a feature-field `env` tag under a combined-map container.
- Binding one environment variable through two feature fields (explicitly or through prefix derivation), or through a feature and a flag.
- Combining `conga.ConfigExplicit(...)` with `conga.ConfigPaths(...)` or `conga.ConfigCascading()`.
- Combining `conga.ConfigLoader(...)` with file options (`ConfigPaths`, `ConfigExplicit`, `ConfigCascading`), `RegisterDecoder`, or a root `ConfigFile` field.
- Declaring the `bootstrap` tag without a declared `conga.ConfigLoader`.
- Combining the `bootstrap` and `feature` tags on the same field.
- Declaring the `bootstrap` tag on unsupported field types (structs, pointers, interfaces).
- Inconsistent `stage` tags violating the all-or-nothing requirement on feature structs.
- Setting `enumhint:"-"` on a field without declared enums, or where the `help` tag already references `{{enum}}`.
- Ancestor flag shadowing across command-subcommand hierarchies (`ErrDuplicateFlag`).
- Declaring `ConfigFile` or `Features` on any command struct other than the root.

## Custom error handling

Custom error handling inspects the error returned by `app.Run()` or `app.RunContext(ctx)` directly instead of delegating to `app.Exit(err)`. The classification is available from the returned error: `conga.IsUsageError(err)` distinguishes input errors, `errors.AsType[conga.UsageError](err)` yields the `UsageError` for `CommandPath()` and `Usage()`, `conga.IsCompletionError(err)` identifies internal completion failures, and `errors.Is` matches the base category sentinels.

```go
if err := app.RunContext(ctx); err != nil {
	if usageErr, ok := errors.AsType[conga.UsageError](err); ok {
		fmt.Fprintf(os.Stderr, "usage error in %s: %v\n", usageErr.CommandPath(), err)
		fmt.Fprintf(os.Stderr, "%s\n", usageErr.Usage())

		os.Exit(2)
	}

	if errors.Is(err, conga.ErrConfig) {
		fmt.Fprintf(os.Stderr, "configuration failure: %v\n", err)
		
		os.Exit(78)
	}

	if conga.IsCompletionError(err) {
		fmt.Fprintf(os.Stderr, "completion failure: %v\n", err)
		
		os.Exit(70)
	}

	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
```

[About error reporting](/explanation/error-reporting/) covers the separation of error values from process termination.

## Usage rendering on errors

`UsageOnError(mode)` controls what `Exit` prints below a usage error:

| Mode | Output |
|------|--------|
| `conga.UsageSummary` (default) | Clean error, 1–2 line syntax, `--help` hint |
| `conga.UsageFull` | Clean error plus the complete help screen |
| `conga.UsageHint` | Clean error plus a single `Run '<cmd> --help'` hint |
| `conga.UsageSilence` | Only the clean error message |

Self-describing value errors render only the error line regardless of the configured mode. These are errors whose message already names the flag/argument and the offending value — coercion failures (`unknown byte size unit "XB" in "5XB" for flag --max-memory`, `ambiguous byte size unit "KB" in "5KB": use kB (1000) or KiB (1024) for flag --max-memory`) and value-constraint violations (`"/etc/passwd" is a file, but a directory was expected for flag --dir`, `score 150 must be between 0 and 100 for flag --score`). Errors that benefit from usage context — unknown commands and flags, missing required flags/arguments, mutually exclusive or grouped flags — keep the mode's rendering.
