---
title: Errors and exit codes
description: Categorized base errors, the usage/runtime classification contract, and the 0/1/2 exit code contract.
---

CONGA classifies every failure it detects into five base categories, one per pipeline stage. An error returned by a command's own `Run` or lifecycle hook comes back unchanged. Match the category, and read the specifics from the message (field, token, command path, suggestions).

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

The concrete error implementations (parse, coercion, validation, config) are unexported. `IsUsageError`, `IsCompletionError`, `UsageError`, `CompletionError`, and the base category sentinels form the public classification API.

## Feature-gating and map-folding failures

Feature gating and map folding classify errors by the input source that introduced the invalid data:

| Failure | Category | Exit |
|---------|----------|:----:|
| Inactive gated flag or subcommand supplied on the command line, unknown feature key, or invalid feature value | `ErrParse` / `ErrValidation` (usage error) | 2 |
| Unknown feature key in a config file | `ErrConfig` | 1 |
| Invalid feature value in a config file | `ErrValidation` (usage error) | 2 |
| Config key naming a flag whose gating feature is inactive | `ErrConfig` | 1 |
| Environment variable naming an inactive gated flag | Ignored (the inactive flag never reaches environment binding) | – |
| Unknown or invalid feature key in the feature map environment variable | `ErrParse` (usage error) | 2 |
| Malformed map flag entry in a config file (even when overridden by CLI) | `ErrCoerce` (usage error) | 2 |
| Malformed map flag syntax in an environment variable | `ErrParse` (usage error) | 2 |
| Invalid format passed to `--why` or `--why=json` requested without `conga.WhyOptionJSON()` | `ErrValidation` (usage error) | 2 |
| Runtime error returned from `Bootstrapper.Bootstrap` | `ErrConfig` | 1 |

## Schema errors (`ErrSchema`)

Programmer errors during schema construction abort at startup before execution begins (exit code 1):
- Declaring `conga.Features[T]` or `feature:"..."` tags without calling `app.EnableFeatures()`.
- Declaring both `env` and `envprefix` on a `conga.Features` container, or a feature-field `env` tag under a combined-map container.
- Binding one environment variable through two feature fields (explicitly or through prefix derivation), or through a feature and a flag.
- Combining `conga.ConfigExplicit(...)` with `conga.ConfigPaths(...)` or `conga.ConfigCascading()`.
- Declaring the `bootstrap` tag when neither the declaring command nor an ancestor implements `conga.Bootstrapper`.
- Combining the `bootstrap` and `feature` tags on the same field.
- Declaring the `bootstrap` tag on unsupported field types (structs, pointers, interfaces).
- Inconsistent `stage` tags violating the all-or-nothing requirement on feature structs.
- Setting `enumhint:"-"` on a field without declared enums, or where the `help` tag already references `{{enum}}`.
- Ancestor flag shadowing across command-subcommand hierarchies (`ErrDuplicateFlag`).
- Built-in short flag collisions: assigning a short rune via `app.HelpShortFlag` or `app.VersionShortFlag` that collides with an application flag, or assigning the same rune to both built-in flags (`ErrDuplicateFlag`).
- Declaring `default`, `xor`, or `oneof` on a `RequiredFlag[T]`.
- Invalid group definitions: declaring `default` on a `oneof` member, two members of the same `oneof` or `xor` group sharing an `and` group, a `xor` or `and` group with fewer than two members, a `oneof` group with fewer than two members unless its sole member is a `Counter`, or an empty or whitespace group name.
- Invalid section definitions: declaring a `section` value with leading or trailing whitespace.
- CONGA tags on plain non-container fields, such as `name` or `help` on a plain struct field, instead of silently ignoring them.
- Unrecognized struct tags without calling `app.AllowTags(...)` (including `conga:"..."` directives).
- Declaring `ConfigFile` or `Features` on any command struct other than the root.

## Panics

Developer misuse of the API panics with a fixed message instead of returning an error. Schema errors are not panics, and `Run` returns them as `ErrSchema`.

| Misuse | Panic message |
|---|---|
| `conga.New` given more than one root | `conga: New accepts at most one root instance` |
| `conga.New` given a nil root | `conga: New requires a non-nil root instance` |
| `app.Predictor` with an empty name | `conga: predictor name cannot be empty` |
| `app.Predictor` with a built-in predictor's name | `conga: cannot register predictor "<name>": reserved built-in predictor name` |
| `app.Predictor` with a name already registered | `conga: predictor "<name>" is already registered` |
| `RegisterType` with a nil parser, or on a nil app | `conga: RegisterType requires a non-nil parser`, `conga: RegisterType requires a non-nil app` |
| `RegisterMap` on a nil app | `conga: RegisterMap requires a non-nil app` |
| `conga.ConfigPaths()` with no paths, or an empty path | `conga: app.ConfigOptions(conga.ConfigPaths(...)): at least one path is required`, `…: path entries must be non-empty` |
| `conga.ConfigExplicit("")` | `conga: app.ConfigOptions(conga.ConfigExplicit(...)): the explicit path cannot be empty` |
| A nil `context.Context` passed to `RunContext`, or to `Run` on `HelpCommand`, `VersionCommand`, or `CompletionCommand` | `conga: nil Context` |
| A second `Run` or `RunContext` on the same `App` | `conga: App already executed` |
| `session.Root[T]()` with a `T` other than the root command type | `conga: Session.Root[<T>] called, but root command type is <root>` |
| A method called on a nil `*ConfigDocument` | `conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)` |

A panic whose message starts with `conga: invariant violation:` is a bug in CONGA, not misuse, and is worth reporting.

## Custom error handling

Custom error handling inspects the error returned by `app.Run()` or `app.RunContext(ctx)` directly instead of delegating to `app.Exit(err)`. The returned error carries the classification. `conga.IsUsageError(err)` distinguishes input errors, `errors.AsType[conga.UsageError](err)` yields the `UsageError` for `CommandPath()` and `Usage()`, `conga.IsCompletionError(err)` identifies internal completion failures, and `errors.Is` matches the base category sentinels.

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

Self-describing value errors render only the error line regardless of the configured mode. These are errors whose message already names the flag or argument and the offending value, such as coercion failures (`unknown byte size unit "XB" in "5XB" for flag --max-memory`, `ambiguous byte size unit "KB" in "5KB": use kB (1000) or KiB (1024) for flag --max-memory`) and value-constraint violations (`"/etc/passwd" is a file, but a directory was expected for flag --dir`, `score 150 must be between 0 and 100 for flag --score`). Errors that benefit from usage context — unknown commands and flags, missing required flags/arguments, mutually exclusive or grouped flags — keep the mode's rendering.
