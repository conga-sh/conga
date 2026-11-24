---
title: Errors and exit codes
description: The UsageError and ExitCoder types, the exit codes 0, 1, 2, and an ExitCoder's own code, and how Exit renders usage.
---

CONGA reports a command-line mistake as a `*conga.UsageError` and rejects invalid schemas before execution begins. Every other failure comes back as an ordinary error, and an error returned by a command's own `Run` or lifecycle hook comes back as the hook returned it. When a `Finally` hook also fails, the hook's error and the `Finally` errors come back joined. Tell the kinds apart by type with `errors.AsType`, and read the specifics from the message (field, token, command path, suggestions). A cause that CONGA did not write itself, such as an error returned by a `TextUnmarshaler`, a standard library error, or an error from the application, stays reachable through `errors.Is` and `errors.As`.

## Exit codes

`app.Exit(err)` terminates with 0, 1, 2, or an `ExitCoder`'s code:

| Code | Meaning |
|------|---------|
| **0** | Success |
| **1** | Any other error, such as a command runtime failure (`Runner.Run` or lifecycle hooks), a schema-build failure, a configuration or `.env` failure, an environment-, configuration-, or default-sourced value failure, a `Bootstrap` failure, or a plain `Validate` error |
| **2** | Usage error (`*conga.UsageError`): invalid command-line input such as unknown flags, parse failures, coercion failures, constraint violations, and missing required input, or an error from `session.UsageError`. Internal completion failures also exit 2, with a diagnostic on stderr |
| **Other** | The code of the outermost `conga.ExitCoder` in the error chain |

Built-in requests (`--help`, `help`, `--version`, `version`, and opt-in `--why` and `--completion <shell>`) write to stdout during `app.Run()` and return `nil` on clean completion.

## ExitCoder

An error chooses its own exit code by implementing `conga.ExitCoder`:

```go
type ExitCoder interface {
	error
	ExitCode() int
}
```

The interface is structural, so an application error needs no CONGA import to satisfy it. `Exit` looks up the outermost `ExitCoder` in the chain with one `errors.AsType` call. A wrapper with its own `ExitCode` method therefore wins over an error it wraps.

| Code reported | Exit status |
|---|---|
| 1 to 255 | That code |
| 0, a negative code, or a code above 255 | 1 |

`*exec.ExitError` is an `ExitCoder`, so returning the error from a child process passes the child's exit code through. A child killed by a signal exits with 128 plus the signal number, as a shell reports it (`signal: killed` exits 137, `signal: segmentation fault` exits 139), and `Exit` still prints the message. On Windows an `*exec.ExitError`'s code passes through unchanged, so a crash code such as `0xC0000005` stays as it is. A hand-built `*exec.ExitError` with no process state has no code and exits 1.

An error whose `Error()` is empty prints no error line. It still exits with its code, which is 1 for a plain error.

## UsageError

`*conga.UsageError` is returned for command-line parse, coercion, and validation failures, and by `session.UsageError` and `session.UsageErrorf`. Its fields are unexported:

```go
func (e *UsageError) Error() string
func (e *UsageError) Unwrap() error
func (e *UsageError) ExitCode() int
func (e *UsageError) CommandPath() string
func (e *UsageError) Usage() string
```

- `CommandPath()` returns the invocation path of the command where the failure occurred (for example, `"cloudctl deploy"`), or an empty string when the error has no command.
- `Usage()` returns the syntax usage summary string for that command.
- `ExitCode()` always returns 2.
- `Unwrap()` returns the error behind the message. For a value that did not convert, it is the error the conversion step returned. That is the error of a `TextUnmarshaler` or registered parser, or CONGA's own parse error, whose type is not part of the API. For a value a `SelfValidator` rejected, it is the validator's error. For `session.UsageErrorf` it is the `%w` operand. It returns nil when CONGA found the problem itself, as for an unknown flag, an `enum`, `range` or `len` violation, or a flag group conflict.

[Which failures are usage errors](#which-failures-are-usage-errors) lists what is not a `*conga.UsageError`.

### Which failures are usage errors

The source of a value decides the exit path. A command-line token that fails to convert or violates a constraint is a `*conga.UsageError` with exit 2. A value from an environment variable, a configuration file, or a `default` tag is not from the command line, so it fails with an ordinary error, exit 1, no usage block. So do `Bootstrap` failures, schema failures, runtime failures, and a plain error returned by `Validate`. A flag group conflict (`xor`, `oneof`, `and`) is a usage error when at least one of the supplied flags came from the command line, and an ordinary error when they all came from the environment or a config file. A `default` tag never counts as supplied. The message keeps its origin:

```text no-copy
app: error: invalid integer value "abc" for flag --port
app: error: invalid integer value "abc" for flag --port (from environment variable APP_PORT)
app: error: invalid integer value "abc" for flag --port (from config key "port")
app: error: invalid integer value "abc" for flag --port (from default value)
```

The first is a usage error with exit 2. The other three exit 1. `errors.As` and `errors.Is` reach the original cause in all four cases, so an application `TextUnmarshaler` error, a standard library `*fs.PathError`, and a sentinel such as `fs.ErrNotExist` remain inspectable.

Domain types that fail while converting the text (a `File` with `executable:"true"`, for example) follow the same rule. A bare name is resolved on `$PATH` during conversion, and an absolute path converts cleanly and fails `File.Validate`:

```sh error={2}
$ app --exe nosuchtool
app: error: exec: "nosuchtool": executable file not found in $PATH for flag --exe
```

```sh error={2}
$ app --exe /usr/local/bin/nosuchtool
app: error: stat /usr/local/bin/nosuchtool: no such file or directory for flag --exe
```

Both exit 2 because the value came from the command line. `--why` makes the difference between the two stages visible. A conversion-time `$PATH` miss aborts before the report renders, while the post-coercion `File.Validate` error is skipped when `--why` renders the report.

## Validate and Bootstrap errors

`Validator.Validate` returns its error unchanged, like `PreRun`, `Run`, and `PostRun`:

| `Validate` returns | `Exit` |
|---|---|
| A plain error | Message, exit 1, no usage block |
| An `ExitCoder`, wrapped or not | Message, the coder's code |
| `ctx.Err()` after cancellation or a deadline, bare or wrapped | Message, exit 1 |
| `session.UsageError(message)` or `session.UsageErrorf(format, args...)` | Message and the usage block for the command, exit 2 |

CONGA can't see which values a check read, so it can't tell a command-line mistake from a config one. Return `session.UsageError` when the cause is the command line. `session.UsageErrorf` keeps a `%w` operand as the error's cause. On a nil session the error has no command, so `Exit` prints no usage block. A message that is empty prints no error line and only the usage block.

`Bootstrapper.Bootstrap` errors also come back unchanged, with no prefix. A plain error exits 1, an `ExitCoder` exits with its code, and a nested app's `*conga.UsageError` exits 2.

## Feature-gating and map-folding failures

Feature gating and map folding report errors by the input source that introduced the invalid data:

| Failure | Error | Exit |
|---------|-------|:----:|
| Inactive gated flag or subcommand supplied on the command line, unknown feature key, or invalid feature value | `*conga.UsageError` | 2 |
| Unknown feature key in a config file | Ordinary error | 1 |
| Invalid feature value in a config file | Ordinary error | 1 |
| Config key naming a flag whose gating feature is inactive | Ordinary error | 1 |
| Config key naming a flag whose gating feature the command line or environment turned off | Ignored | – |
| Environment variable naming an inactive gated flag | Ignored (the inactive flag never reaches environment binding) | – |
| Unknown feature key in the feature map environment variable | Ordinary error | 1 |
| Invalid feature value in the feature map environment variable | Ordinary error | 1 |
| Malformed map flag entry in a config file (even when overridden by CLI) | Ordinary error | 1 |
| Malformed map flag syntax in an environment variable | Ordinary error | 1 |
| Invalid format passed to `--why` or `--why=json` requested without `conga.WhyOptionJSON()` | `*conga.UsageError` | 2 |
| Error returned from `Bootstrapper.Bootstrap` | Returned unchanged | 1, or the `ExitCoder`'s code |

## Schema errors

Programmer errors during schema construction abort at startup before execution begins (exit code 1):
- Declaring `conga.Features[T]` or `feature:"..."` tags without calling `app.EnableFeatures()`.
- Declaring both `env` and `envprefix` on a `conga.Features` container, or a feature-field `env` tag under a combined-map container.
- Binding one environment variable through two feature fields (explicitly or through prefix derivation), or through a feature and a flag.
- Binding a flag or feature to the environment variable the app reads a config file path from (`<PREFIX>_CONFIG`, or the `ConfigFile` field's `env` tag): `struct CLI field Config: environment variable "PC_CONFIG" is bound by both the config file variable and flag --config (hint: give the flag another env tag or exclude it with env:"-")`.
- Combining `conga.ConfigExplicit(...)` with `conga.ConfigPaths(...)` or `conga.ConfigCascading()`.
- Declaring the `bootstrap` tag when neither the declaring command nor an ancestor implements `conga.Bootstrapper`.
- Combining the `bootstrap` and `feature` tags on the same field.
- Declaring the `bootstrap` tag on unsupported field types (structs, pointers, interfaces).
- Inconsistent `stage` tags violating the all-or-nothing requirement on feature structs.
- Declaring `enum` on a path field (`conga.File`, `conga.Dir`, `conga.FileContent`, a filterable type, or a `file`, `dir`, `exec`, or `symlink` predictor tag): `struct Deploy field Manifest: tag "enum" is not supported on conga.File (hint: use tag "glob" to filter completion candidates, or a string field with tag "enum" to accept a fixed set of names)`.
- Setting `enumhint:"-"` on a field without declared enums, or where the `help` tag already references `{{enum}}`.
- Ancestor flag shadowing across command-subcommand hierarchies, including the built-in version command's `--json` and a CLI-exposed `Features[T]` flag.
- Built-in short flag collisions: assigning a short rune via `app.HelpShortFlag` or `app.VersionShortFlag` that collides with an application flag, or assigning the same rune to both built-in flags.
- Declaring `default`, `xor`, or `oneof` on a `RequiredFlag[T]`.
- Invalid group definitions: declaring `default` on a `oneof` member, two members of the same `oneof` or `xor` group sharing an `and` group, a `xor` or `and` group with fewer than two members, a `oneof` group with fewer than two members unless its sole member is a `Counter`, or an empty or whitespace group name.
- Invalid section definitions: declaring a `section` value with leading or trailing whitespace.
- CONGA tags on plain non-container fields, such as `name` or `help` on a plain struct field, instead of silently ignoring them.
- Unrecognized struct tags without calling `app.AllowTags(...)` (including `conga:"..."` directives).
- A boolean tag (`hidden`, `negatable`, `bootstrap`, `implicit`, `safe`, `multi`, or a domain type's boolean tag such as `existing`) with a value other than empty, `true`, or `false`: `struct CLI field Debug: tag "hidden" expects "true" or "false", got "yes"`.
- Declaring `ConfigFile` or `Features` on any command struct other than the root.
- A flag or positional argument whose type, slice element type, or map key or value type has no parser: `struct CLI field Zone: unsupported type *time.Location (hint: register a parser with app.Use(timezone.RegisterLocation))`. The hint names `app.RegisterType`, or the domain-package binding for `*time.Location`, `*user.User`, `*user.Group`, and `*net.Interface`.
- A leaf command that does not implement `conga.Runner`: `struct LeafCmd: runner not implemented (hint: implement conga.Runner: Run(context.Context, *conga.Session) error)`.

## Panics

Developer misuse of the API panics with a fixed message instead of returning an error. Schema errors are not panics, and `Run` returns them directly.

| Misuse | Panic message |
|---|---|
| `conga.New` given more than one root | `conga: New accepts at most one root instance` |
| `conga.New` given a nil root | `conga: New requires a non-nil root instance` |
| `app.Predictor` with an empty name | `conga: predictor name cannot be empty` |
| `app.Predictor` with a built-in predictor's name | `conga: cannot register predictor "<name>": reserved built-in predictor name` |
| `app.Predictor` with a name already registered | `conga: predictor "<name>" is already registered` |
| `RegisterType` with a nil parser, or on a nil app | `conga: RegisterType requires a non-nil parser`, `conga: RegisterType requires a non-nil app` |
| `RegisterType` for a map or slice type | `conga: RegisterType does not accept map type <M>; maps bind entry by entry, so register the key or value type instead`, `conga: RegisterType does not accept slice type <S>; slices bind element by element, so register the element type instead` |
| `RegisterType` for a slice or map type that implements `encoding.TextUnmarshaler` | `conga: RegisterType does not accept <T>; it implements encoding.TextUnmarshaler, so it binds as one value without registration` |
| `conga.ConfigPaths()` with no paths, or an empty path | `conga: app.ConfigOptions(conga.ConfigPaths(...)): at least one path is required`, `…: path entries must be non-empty` |
| `conga.ConfigExplicit("")` | `conga: app.ConfigOptions(conga.ConfigExplicit(...)): the explicit path cannot be empty` |
| A nil `context.Context` passed to `RunContext`, or to `Run` on `HelpCommand`, `VersionCommand`, or `CompletionCommand` | `conga: nil Context` |
| A second `Run` or `RunContext` on the same `App` | `conga: App already executed` |
| `session.Root[T]()` with a `T` other than the root command type | `conga: Session.Root[<T>] called, but root command type is <root>` |
| A method called on a nil `*ConfigDocument` | `conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)` |

A panic whose message starts with `conga: invariant violation:` is a bug in CONGA, not misuse, and is worth reporting.

## Custom error handling

Custom error handling reports the error returned by `app.Run()` or `app.RunContext(ctx)` itself instead of delegating to `app.Exit(err)`. `app.ExitCode(err)` returns the code `app.Exit` would exit with, so the program exits the same way. `App.Exit` prints a usage block only when the outermost `ExitCoder` in the error is the `*conga.UsageError`, and the same lookup gives `CommandPath()` and `Usage()`:

```go
if err := app.RunContext(ctx); err != nil {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)

	if coder, ok := errors.AsType[conga.ExitCoder](err); ok {
		if usageErr, isUsage := coder.(*conga.UsageError); isUsage {
			fmt.Fprintf(os.Stderr, "usage for %s:\n%s\n", usageErr.CommandPath(), usageErr.Usage())
		}
	}

	os.Exit(app.ExitCode(err))
}
```

A failure from the environment, a config file or a default is not a `*conga.UsageError`, so the snippet prints no usage block for it ([which failures are usage errors](#which-failures-are-usage-errors)).

[About error reporting](/explanation/error-reporting/) covers the separation of error values from process termination.

## Usage rendering on errors

`UsageOnError(mode)` controls what `Exit` prints below a usage error:

| Mode | Output |
|------|--------|
| `conga.UsageSummary` (default) | Clean error, 1–2 line syntax, `--help` hint |
| `conga.UsageFull` | Clean error plus the complete help screen |
| `conga.UsageHint` | Clean error plus a single `Run '<cmd> --help'` hint |
| `conga.UsageSilence` | Only the clean error message |

Three shapes follow from the kind of failure:

- Command-line value errors name the flag or argument and the offending value, so they render only the error line regardless of the configured mode. These are coercion failures (`unknown byte size unit "XB" in "5XB" (accepted: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB) for flag --max-memory`, `ambiguous byte size unit "KB" in "5KB": use kB (1000) or KiB (1024) for flag --max-memory`) and value-constraint violations (`"/etc/passwd" is a file, but a directory was expected for flag --dir`, `value 150 is out of range [0..100] for flag --score`).
- Errors that benefit from usage context keep the mode's rendering. These are unknown commands and flags, missing required flags and arguments, mutually exclusive or grouped flags, stdin read twice, an unknown feature on the command line, and the errors from `session.UsageError`.
- A missing subcommand prints the full help of its command for `UsageSummary` and `UsageFull`, the hint for `UsageHint`, and the error line alone for `UsageSilence`.

The usage block comes from the error's own command, so a `*conga.UsageError` without one (a nil session, or a zero value) prints no block. Environment-, configuration-, and default-sourced value failures are not usage errors, so they always render as a single `app: error: ...` line with exit 1. A group failure whose members all came from the environment or a config file renders the same way.
