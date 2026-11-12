---
title: Errors and exit codes
description: Categorized base errors, the usage/runtime classification contract, and the 0/1/2 exit code contract.
---

CONGA classifies every failure it detects into exactly one of five base categories. An error returned by a command's own `Run` or lifecycle hook comes back unchanged. Match the category, and read the specifics from the message (field, token, command path, suggestions).

## Exit codes

`app.Exit(err)` terminates with one of three codes:

| Code | Meaning |
|------|---------|
| **0** | Success |
| **1** | Command runtime failure (`Runner.Run` or lifecycle hooks), schema-build failure (`ErrSchema`), or configuration failure (`ErrConfig`, including environment-, configuration-, and default-sourced value failures) |
| **2** | Usage error: invalid command-line input — unknown flags, parse failures, coercion failures, constraint violations, missing required input — and internal completion failures with a diagnostic on stderr |

Built-in requests (`--help`, `help`, `--version`, `version`, and opt-in `--why` and `--completion <shell>`) write to stdout during `app.Run()` and return `nil` on clean completion.

## Base categories

| Sentinel | Stage |
|----------|-------|
| `conga.ErrSchema` | Schema building: tag linting, duplicates, invalid order (programmer errors at startup) |
| `conga.ErrParse` | Command-line tokenization and parsing |
| `conga.ErrCoerce` | Command-line string → typed value conversion |
| `conga.ErrValidation` | Command-line constraints and semantic checks |
| `conga.ErrConfig` | Config file and `.env` reading, parsing, and key resolution, plus conversion and constraint failures for environment-, configuration-, and default-sourced values |

## Which category a value failure gets

Two rules decide the category of a value failure.

**The source decides the exit path.** A command-line token is a usage error: `ErrCoerce` when conversion fails, `ErrValidation` when a constraint fails, exit 2. A value from an environment variable, a configuration file, or a `default` tag classifies under `ErrConfig` only: exit 1, no usage block, and `errors.Is(err, conga.ErrCoerce)` and `errors.Is(err, conga.ErrValidation)` both report false. The message keeps its origin:

```text no-copy
app: error: invalid integer value "abc" for flag --port
app: error: invalid integer value "abc" for flag --port (from environment variable APP_PORT)
app: error: invalid integer value "abc" for flag --port (from config key "port")
app: error: invalid integer value "abc" for flag --port (from default value)
```

The first is a usage error with exit 2. The other three are `ErrConfig` with exit 1. `errors.As` still reaches the original cause, so an application `TextUnmarshaler` error or a stdlib `*fs.PathError` remains inspectable.

**The stage decides `ErrCoerce` versus `ErrValidation`.** A failure while converting the text is `ErrCoerce`; a check on the converted value is `ErrValidation`. For example, a `File` field with `executable:"true"`:

```sh error={2}
$ app --exe nosuchtool
app: error: exec: "nosuchtool": executable file not found in $PATH for flag --exe
```

```sh error={2}
$ app --exe /usr/local/bin/nosuchtool
app: error: stat /usr/local/bin/nosuchtool: no such file or directory for flag --exe
```

The bare name is resolved on `$PATH` during conversion, so the failure is `ErrCoerce`. The absolute path converts cleanly and fails `File.Validate`, so it is `ErrValidation`. Both exit 2 because the value came from the command line. A wrong path kind on a converted value — a directory where a file was expected — is `ErrValidation`. The same check inside `FileContent` runs while converting its text, so for a command-line value it stays `ErrCoerce`.

For a command-line value, conversion-time failures stay under `ErrCoerce` even when they mean "not found". `--why` makes the difference visible: a conversion-time `$PATH` miss aborts before the report renders, while the post-coercion `File.Validate` error is skipped when `--why` renders the report.

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
| Invalid feature value in a config file | `ErrConfig` | 1 |
| Config key naming a flag whose gating feature is inactive | `ErrConfig` | 1 |
| Config key naming a flag whose gating feature the command line or environment turned off | Ignored | – |
| Environment variable naming an inactive gated flag | Ignored (the inactive flag never reaches environment binding) | – |
| Unknown feature key in the feature map environment variable | `ErrConfig` | 1 |
| Invalid feature value in the feature map environment variable | `ErrConfig` | 1 |
| Malformed map flag entry in a config file (even when overridden by CLI) | `ErrConfig` | 1 |
| Malformed map flag syntax in an environment variable | `ErrConfig` | 1 |
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
- Ancestor flag shadowing across command-subcommand hierarchies (`ErrDuplicateFlag`), including the built-in version command's `--json` and a CLI-exposed `Features[T]` flag.
- Built-in short flag collisions: assigning a short rune via `app.HelpShortFlag` or `app.VersionShortFlag` that collides with an application flag, or assigning the same rune to both built-in flags (`ErrDuplicateFlag`).
- Declaring `default`, `xor`, or `oneof` on a `RequiredFlag[T]`.
- Invalid group definitions: declaring `default` on a `oneof` member, two members of the same `oneof` or `xor` group sharing an `and` group, a `xor` or `and` group with fewer than two members, a `oneof` group with fewer than two members unless its sole member is a `Counter`, or an empty or whitespace group name.
- Invalid section definitions: declaring a `section` value with leading or trailing whitespace.
- CONGA tags on plain non-container fields, such as `name` or `help` on a plain struct field, instead of silently ignoring them.
- Unrecognized struct tags without calling `app.AllowTags(...)` (including `conga:"..."` directives).
- Declaring `ConfigFile` or `Features` on any command struct other than the root.
- A leaf command that does not implement `conga.Runner`: `schema error: runner not implemented: command "leaf" does not implement conga.Runner (implement Run(context.Context, *conga.Session) error)`.

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

Custom error handling inspects the error returned by `app.Run()` or `app.RunContext(ctx)` directly instead of delegating to `app.Exit(err)`. The returned error carries the classification. `conga.IsUsageError(err)` distinguishes command-line input errors, `errors.AsType[conga.UsageError](err)` yields the `UsageError` for `CommandPath()` and `Usage()`, `conga.IsCompletionError(err)` identifies internal completion failures, and `errors.Is` matches the base category sentinels.

An environment-, configuration-, or default-sourced coercion or constraint failure still implements `conga.UsageError`, but its `IsUsageError()` reports false and it classifies under `ErrConfig`. Test the classification through `conga.IsUsageError(err)`, not through the type assertion alone:

```go
if err := app.RunContext(ctx); err != nil {
	if conga.IsUsageError(err) {
		usageErr, _ := errors.AsType[conga.UsageError](err)
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

Self-describing command-line value errors render only the error line regardless of the configured mode. Environment-, configuration-, and default-sourced value failures are not usage errors, so they always render as a single `app: error: ...` line with exit 1. Self-describing command-line value errors are those whose message already names the flag or argument and the offending value, such as coercion failures (`unknown byte size unit "XB" in "5XB" (accepted: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB) for flag --max-memory`, `ambiguous byte size unit "KB" in "5KB": use kB (1000) or KiB (1024) for flag --max-memory`) and value-constraint violations (`"/etc/passwd" is a file, but a directory was expected for flag --dir`, `score 150 must be between 0 and 100 for flag --score`). Errors that benefit from usage context — unknown commands and flags, missing required flags/arguments, mutually exclusive or grouped flags — keep the mode's rendering. A group failure whose members all came from the environment or a config file classifies under `ErrConfig` instead and renders as the single error line with exit 1.
