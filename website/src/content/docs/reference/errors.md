---
title: Errors and exit codes
description: The Diagnostic, UsageError and ExitCoder types, the from, note and hint lines Exit prints, the exit codes 0, 1, 2, and an ExitCoder's own code, and how Exit renders usage.
---

CONGA reports a command-line mistake as a `*conga.UsageError` and rejects invalid schemas before execution begins. `app.Run` and `app.RunContext` return whatever failed as is, with an untyped `nil` on success. An error returned by a command's own `Run` or lifecycle hook keeps its text, type, and identity, and `Finally` receives the same value. When `Finally` hooks also fail, the run's error remains the primary error and the `Finally` errors are suppressed alongside it (or, when stages succeeded and more than one `Finally` hook fails, the first becomes the primary). `Unwrap`, `errors.Is`, `errors.As`, and `ExitCode` see only the primary error, while `Error()`, `app.Exit`, and `conga.DiagnosticOf` surface every error. Tell the kinds apart by type with `errors.AsType`. A cause that CONGA did not write itself, such as an error returned by a `TextUnmarshaler`, a standard library error, or an error from the application, stays reachable through `errors.Is` and `errors.As`.

## Reading an error with DiagnosticOf

`conga.DiagnosticOf(err)` reads any error (a CONGA error, a hook's error, a developer wrapper around `Run`'s result, or a join) from the outside in into a `conga.Diagnostic`. It holds the pieces `app.Exit` prints, so a program that reports errors itself can print, render or log the same lines. Programs that only call `app.Exit(app.Run())` never link `conga.Diagnostic` or its marshalers into the binary. Its fields are unexported:

```go
func DiagnosticOf(err error) Diagnostic

func (d Diagnostic) Message() string
func (d Diagnostic) Origins() []string
func (d Diagnostic) Note() string
func (d Diagnostic) Hint() string
func (d Diagnostic) Members() []Diagnostic
func (d Diagnostic) Render() string
func (d Diagnostic) MarshalJSON() ([]byte, error)
func (d Diagnostic) MarshalText() ([]byte, error)
func (d Diagnostic) Format(state fmt.State, verb rune)
```

`Message()` returns the error's text, which is line 1 of what `app.Exit` prints and nothing more. `Origins()`, `Note()` and `Hint()` return the lines `app.Exit` prints below line 1 by the chain rule, agreeing with `app.Exit` even when a developer wraps `Run`'s result with `fmt.Errorf`. [Custom error handling](#custom-error-handling) shows a program that reports errors itself.

A `conga.Diagnostic` does not implement `error`. It has no `Error()` or `Unwrap()` method, so it can't be returned from a hook, passed to `app.Exit` or `app.ExitCode`, joined with `errors.Join`, or compared with `errors.Is`, and `==` between two values doesn't compile. `app.ExitCode` decides the exit code from the outermost `conga.ExitCoder` in the chain, and `app.UsageBlock` returns the usage block, so a `Diagnostic` has neither.

An error that stands for several errors (an `errors.Join` of them, or a primary error with suppressed `Finally` errors) has one member per error. `Message()` joins their first lines with newlines, `Origins()`, `Note()` and `Hint()` describe the first non-empty member, and `Members()` returns each non-empty member as its own `conga.Diagnostic`. `Members()` splits errors the way `app.Exit` does, including a primary error paired with suppressed `Finally` errors.

`DiagnosticOf(nil)` returns the zero `conga.Diagnostic`. Its `Message()` is empty, its `Members()` is nil and its `MarshalJSON` returns `null`.

## The lines below an error

`app.Exit` prints `<app>: error: ` and line 1, then indented lines in a fixed order. One `from:` line per origin comes first, then one `note:` line and one `hint:` line.

```text no-copy
tool: error: invalid unsigned integer value "0755" for flag --mode
  from: environment variable TOOL_MODE
  note: a leading zero is ambiguous
  hint: write 0o755 for octal or 755 for decimal
```

- `from:` says where the value came from. It shows `environment variable TOOL_MODE`, `config key "mode" in config file "/home/alice/.config/tool/config.yaml"`, `config key "mode" set by profile dev` for a `ConfigDocument` value, or `default value` for a `default` tag. An environment variable that a `.env` file set adds ` in .env file ".env.local"`. A command-line value has no `from:` line.
- `note:` gives more detail on the issue line 1 states, such as the accepted values or units.
- `hint:` says what to do about it, as an imperative or a "did you mean" suggestion.

Unknown flag, shorthand and command errors, `unexpected argument` errors, and the errors for a value given to a flag that takes none, quote the text the user typed (a shorthand flag's rune is in single quotes), truncated after 50 characters with a trailing `…`, as in `unknown flag "--prot"`. A "did you mean" suggestion is double-quoted too, and a flag suggestion keeps its dashes, as in `did you mean "--port"?`.

A flag group conflict has one `from:` line per member that did not come from the command line, flag first:

```text no-copy
tool: error: flags --to-file and --to-url are mutually exclusive
  from: --to-file: environment variable TOOL_TO_FILE
  from: --to-url: config key "to-url" in config file "tool.yaml"
```

Errors built with `session.Error().For(...)` use the same `from:` line format for flags (`--<flag>: <origin>`) and prefix positional arguments with `argument <NAME>: default value`. Every non-CLI, non-unset input passed to `For(...)` emits one line in the order `For` named it.

An error about a whole config file, such as an unknown field, names the file (`from: config file "tool.yaml"`), or the label of a `ConfigDocument` (`from: set by profiles.yaml › dev`). The errors from finding and loading files (`cannot read config file`, `cannot parse config file`, `multiple conflicting config files found`) and from `.env` files (`cannot read .env file`, `cannot parse .env file`) name the file in line 1 and add no `from:` line.

An error whose text is empty prints no lines at all.

### See every line of an error

`Error()` of every CONGA error is line 1 alone, so `log.Fatal(err)`, `t.Fatal(err)` and `assert.NoError(t, err)` show line 1 and leave out the `from:`, `note:` and `hint:` lines. Two ways show every line. Pass the error to `app.Exit` with `Stderr` and `ExitFunc` set, which is what a test captures, or call `conga.DiagnosticOf(err).Render()` (or format it with `%+v`).

### Supply the lines from your own errors

An error type supplies lines by implementing the structural interfaces `conga.Originer`, `conga.Noter` and `conga.Hinter`:

```go
type Originer interface{ error; Origins() []string }
type Noter    interface{ error; Note() string }
type Hinter   interface{ error; Hint() string }
```

An empty result prints no line. Implement only the methods the error has something to say for. Because the interfaces are structural, any error type implementing those method names participates. CONGA's own errors implement the same methods, so a developer's error gets the same layout.

`Exit` reads each piece by the chain rule. The outermost error in the chain that returns a non-empty value supplies it, in `errors.As` traversal order. Text added around an error on line 1 does not matter, because the lines come from the chain:

```go
return fmt.Errorf("limit: %w", byteSizeErr)
```

```text no-copy
tool: error: limit: unknown byte size unit "XB" in "5XB"
  note: accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB
```

A wrapper's own `Hint()` replaces CONGA's hint, and the note and the hint can come from different errors in one chain. At most one `note:` line and one `hint:` line print.

`errors.As` stops at the first implementer in a chain even when it returns an empty result, so `errors.AsType[conga.Hinter](err)` may stop at a wrapper whose `Hint()` is empty. CONGA's own errors never hide their cause's pieces. A CONGA error that wraps a cause returns its own `Origins()`, `Note()` or `Hint()` when it has one, and otherwise the cause's, so `errors.AsType` on the three interfaces agrees with `Exit` for them. A developer type that wraps another error should do the same, by returning the wrapped error's piece when it has none or by not implementing the method. Pass any error to `conga.DiagnosticOf(err)` to apply the chain rule from the outside in and match `Exit`.

### Joined errors

An error with `Unwrap() []error` whose text is its members' texts joined by newlines (the shape `errors.Join` returns) prints member by member. Each member gets its own `<app>: error:` prefix, lines and colors, a nested join is flattened, and a nil member is skipped. An `fmt.Errorf` with two `%w`, a join that `fmt.Errorf` wraps, and a member whose own text spans lines print as one error, with later lines as they are.

The usage block prints once, after every line, and only when the `*conga.UsageError` is the error that sets the exit code. In a join the first member with an `ExitCode()` sets the code, so `errors.Join(usageErr, exit75Err)` exits 2 with the usage block and `errors.Join(exit75Err, usageErr)` exits 75 without one.

### Colors

`Exit` colors the labels when stderr is a terminal. `error:` is bold red, `from:` and `note:` are dim, and `hint:` is bold cyan. Line 1 keeps the default foreground. These variables decide:

| Variable | Effect |
|---|---|
| `CONGA_COLOR` | A boolean. True colors a non-terminal stderr too, and false turns colors off. It outranks `NO_COLOR` and `CLICOLOR` |
| `NO_COLOR` | Any non-empty value turns colors off |
| `CLICOLOR` | `0` turns colors off |
| `TERM` | `dumb` turns colors off when none of `CONGA_COLOR`, `NO_COLOR` and `CLICOLOR` sets a preference |

`--why` uses the same rules for stdout. On Windows, CONGA turns on the console's escape-code handling when the stream is a console.

### Render plain text or print with %+v

`conga.DiagnosticOf(err).Render()` returns line 1 followed by the indented `from:`, `note:` and `hint:` lines for every non-empty member as plain text, with no `<app>: error:` prefix, no color, and no trailing newline. `conga.Diagnostic` also implements `fmt.Formatter`. `%v` and `%s` print `Message()`, `%+v` prints the same multi-line form as `Render()`, and every other verb, such as `%q`, formats `Message()` with that verb.

```go
if err := app.RunContext(t.Context()); err != nil {
	t.Fatal(conga.DiagnosticOf(err).Render())
}
```

### Log with log/slog

`conga.Diagnostic` implements `json.Marshaler` and `encoding.TextMarshaler`, so passing `conga.DiagnosticOf(err)` to `log/slog` records an object in `slog.NewJSONHandler` and a compact one-line string in `slog.NewTextHandler` from one call site, with no `encoding/json` import:

```go
slog.Error("run failed", "err", conga.DiagnosticOf(err))
```

```text no-copy
{"level":"ERROR","msg":"run failed","err":{"error":"invalid integer value \"abc\" for flag --port"}}
level=ERROR msg="run failed" err="invalid integer value \"abc\" for flag --port"
```

A value that came from the environment adds `"from"`, and the text form adds the origin in parentheses:

```text no-copy
{"level":"ERROR","msg":"run failed","err":{"error":"invalid integer value \"abc\" for flag --port","from":["environment variable APP_PORT"]}}
level=ERROR msg="run failed" err="invalid integer value \"abc\" for flag --port (from: environment variable APP_PORT)"
```

Both samples leave out the `time` attribute, which a `ReplaceAttr` function removes.

The object is `{"error": <line 1>, "from": [...], "note": "...", "hint": "..."}`, with every key but `error` left out when empty (`error` is always present for a non-nil error). An error with several members adds `"more": [...]`. The text form is `<line 1> (from: <origin>; <origin>; note: ...; hint: ...)`, with several members joined by ` | `.

Because `DiagnosticOf` walks from the outside in, wrapping `Run`'s result with `fmt.Errorf("main: %w", err)` before calling `conga.DiagnosticOf(wrapped)` preserves the outer message and any outer `Hint()` in both handlers.

### Show a usage block from your own error

`*conga.UsageError` is the only error that shows a usage block, and exit code 2 and the block go together. Return an error through `session.Error().Usage(true).Wrap(err)`, `session.Error().For(...).Wrap(err)` with two or more named inputs where at least one came from the command line, or `session.UsageErrorf("%w", err)`. All three keep the error's hint and note.

## Exit codes

`app.Exit(err)` terminates with 0, 1, 2, or an `ExitCoder`'s code:

| Code | Meaning |
|------|---------|
| **0** | Success (including a run that caught a signal under `app.Run()` and returned `nil`) |
| **1** | Any other error, such as a command runtime failure (`Runner.Run` or lifecycle hooks), a schema-build failure, a configuration or `.env` failure, an environment-, configuration-, or default-sourced value failure, a `Bootstrap` failure, or a plain `Validate` error |
| **2** | Usage error (`*conga.UsageError`): invalid command-line input such as unknown flags, parse failures, coercion failures, constraint violations, and missing required input, or an error from `session.UsageError` or `session.Error()` with a command-line input or `.Usage(true)`. Internal completion failures also exit 2, with a diagnostic on stderr |
| **129** | Non-nil error after `app.Run()` caught `SIGHUP` (128 + 1), unless an author `ExitCoder` overrides it |
| **130** | Non-nil error after `app.Run()` caught `SIGINT` (128 + 2), unless an author `ExitCoder` overrides it |
| **143** | Non-nil error after `app.Run()` caught `SIGTERM` (128 + 15), unless an author `ExitCoder` overrides it |
| **Other** | The code of the outermost `conga.ExitCoder` in the error chain |

When `app.Run()`'s own signal handler caught a signal, a non-nil error exits with `128 + signal` and prints no usage block (`app.UsageBlock` returns `""`) unless the outermost `conga.ExitCoder` is an author's own type. Both `*exec.ExitError` and `*conga.UsageError` yield to the signal code. On that path, a primary error wrapping `context.Canceled` or an `*exec.ExitError` killed by a signal or exiting `128 + signal` is silent, and in a joined primary those members are omitted while other primary members and suppressed `Finally` errors still print.

`app.ExitCode(err)` returns the status `app.Exit` exits with, and `app.UsageBlock(err)` returns the usage block `app.Exit` prints after the lines, without printing anything, so a program that reports errors itself matches `app.Exit`.

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

An error whose `Error()` is empty prints no lines. It still exits with its code, which is 1 for a plain error.

## UsageError

`*conga.UsageError` is returned for command-line parse, coercion, and validation failures, by `session.UsageError` and `session.UsageErrorf`, and by `session.Error()` with a command-line input or `.Usage(true)`. Its fields are unexported:

```go
func (e *UsageError) Error() string
func (e *UsageError) Unwrap() error
func (e *UsageError) ExitCode() int
func (e *UsageError) CommandPath() string
func (e *UsageError) Origins() []string
func (e *UsageError) Note() string
func (e *UsageError) Hint() string
```

- `CommandPath()` returns the invocation path of the command where the failure occurred (for example, `"cloudctl deploy"`), or an empty string when the error has no command.
- `ExitCode()` always returns 2.
- `Origins()`, `Note()` and `Hint()` return the lines `app.Exit` prints below line 1, by the chain rule. The failed value's own come first, then those of the error behind the message. For an unknown flag or command, `Hint()` is the suggestion (`did you mean "--port"?`). A flag group conflict returns the origin of each member that did not come from the command line.
- `Error()` returns line 1 alone.
- `Unwrap()` returns the error behind the message. For a value that did not convert, it is the error the conversion step returned. That is the error of a `TextUnmarshaler` or registered parser, or CONGA's own parse error, whose type is not part of the API. For a value a `SelfValidator` rejected, it is the validator's error. For `session.UsageErrorf` or `ErrorBuilder.Newf` it is the `%w` operand, and for `ErrorBuilder.Wrap` or `ErrorBuilder.Wrapf` it is the wrapped error. It returns nil when CONGA found the problem itself, as for an unknown flag, an `enum`, `range` or `len` violation, or a flag group conflict.

The usage block that goes with the error comes from the app, as `app.UsageBlock(err)` returns it. [Which failures are usage errors](#which-failures-are-usage-errors) lists what is not a `*conga.UsageError`.

### Which failures are usage errors

The source of a value decides the exit path. A command-line token that fails to convert or violates a constraint is a `*conga.UsageError` with exit 2. A value from an environment variable, a configuration file, or a `default` tag is not from the command line, so it fails with an ordinary error, exit 1, no usage block. So do `Bootstrap` failures, schema failures, runtime failures, and a plain error returned by `Validate`. A flag group conflict (`xor`, `oneof`, `and`) is a usage error when at least one of the supplied flags came from the command line, and an ordinary error when they all came from the environment or a config file. A `default` tag never counts as supplied. The error keeps its origin on a `from:` line:

```text no-copy
# from the command line
app: error: invalid integer value "abc" for flag --port

# from the environment
app: error: invalid integer value "abc" for flag --port
  from: environment variable APP_PORT

# from a config file
app: error: invalid integer value "abc" for flag --port
  from: config key "port" in config file "/home/alice/.config/app/config.yaml"

# from a default tag
app: error: invalid integer value "abc" for flag --port
  from: default value
```

The command-line value is a usage error with exit 2. The other three exit 1. `errors.As` and `errors.Is` reach the original cause in all four cases, so an application `TextUnmarshaler` error, a standard library `*fs.PathError`, and a sentinel such as `fs.ErrNotExist` remain inspectable.

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

`Validator.Validate` returns its error as is, like `PreRun`, `Run`, and `PostRun`:

| `Validate` returns | `Exit` |
|---|---|
| A plain error | Message, exit 1, no usage block |
| An `ExitCoder`, wrapped or not | Message, the coder's code |
| `ctx.Err()` after cancellation or a deadline, bare or wrapped | Message, exit 1 |
| `session.UsageError(message)` or `session.UsageErrorf(format, args...)` | Message and the usage block for the command, exit 2 |
| `session.Error()` with `For(...)`, `Usage(...)`, `Note(...)`, `Hint(...)` | Classified by source or `.Usage(bool)`, with `from:`, `note:`, and `hint:` lines |

Return `session.UsageError` or `session.UsageErrorf` when a check always reports a command-line mistake. `session.UsageErrorf` keeps a `%w` operand as the error's cause. On a nil session the error has no command, so `Exit` prints no usage block. A message that is empty prints no error line and only the usage block.

`Bootstrapper.Bootstrap` errors also come back with their text unchanged and no prefix. A plain error exits 1, an `ExitCoder` exits with its code, and a nested app's `*conga.UsageError` exits 2.

### Classify errors by input source with Session.Error

`session.Error()` returns a `conga.ErrorBuilder` value that classifies the error from the sources of the `conga.Input` containers passed to `For`, and attaches `from:`, `note:`, and `hint:` lines:

```go
type Input interface{ /* sealed */ }

func (s *Session) Error() ErrorBuilder

func (b ErrorBuilder) For(inputs ...Input) ErrorBuilder
func (b ErrorBuilder) Usage(usage bool) ErrorBuilder
func (b ErrorBuilder) Note(note string) ErrorBuilder
func (b ErrorBuilder) Hint(hint string) ErrorBuilder

func (b ErrorBuilder) New(message string) error
func (b ErrorBuilder) Newf(format string, args ...any) error
func (b ErrorBuilder) Wrap(err error) error
func (b ErrorBuilder) Wrapf(err error, format string, args ...any) error
```

`ErrorBuilder` is a value type, so calling `For`, `Usage`, `Note`, or `Hint` returns a new builder without mutating earlier ones.

`Input` is satisfied by pointers to pure value containers (`*OptionalFlag`, `*RequiredFlag`, `*OptionalArg`, `*RequiredArg`, `*Counter`, `*DashArgs`, and `*RawArgs`). Passing a value copy, a plain variable, `*Command`, `*ConfigFile`, or `*Features` fails to compile. `*DashArgs` and `*RawArgs` count as command-line inputs when `IsSet()` or non-empty, and as unset otherwise.

Each terminal keeps line 1 verbatim without appending a flag or argument suffix:

- `New(message)` builds an error with line 1 equal to `message` and no cause.
- `Newf(format, args...)` formats via `fmt.Errorf`, keeping any `%w` operands reachable through `errors.Is`, `errors.As`, and `errors.Unwrap`.
- `Wrap(err)` returns `nil` when `err` is `nil`. Otherwise it keeps `err.Error()` as line 1, sets `err` as the unwrapped cause, and inherits `err`'s `Origins()` (when `For` produces no `from:` line), `Note()`, and `Hint()` unless overridden on the builder.
- `Wrapf(err, format, args...)` returns `nil` when `err` is `nil`. Otherwise it formats the prefix via `fmt.Sprintf`, joins prefix and `err.Error()` with `": "` (omitting the separator when either side is empty), and sets `err` as the unwrapped cause. Use `Newf` when you need `%w` placement or multiple wrapped errors.

When `.Usage` is omitted, `ErrorBuilder` classifies the error from the inputs named in `For`:

| Configuration | Named inputs' sources | Returned type | Exit | Usage block | `from:` lines |
|---|---|---|---|---|---|
| Auto | 1 named input from `SourceCLI` | `*conga.UsageError` | 2 | None | None |
| Auto | 2 or more named inputs (at least 1 `SourceCLI`) | `*conga.UsageError` | 2 | Mode's usage block | One per non-CLI, non-unset input |
| Auto | All env, config, default, or unset, or no `For` | Unexported error | 1 | None | One per non-unset input |
| `.Usage(true)` | Any (with or without `For`) | `*conga.UsageError` | 2 | Mode's usage block | One per non-CLI, non-unset input |
| `.Usage(false)` | Any (with or without `For`) | Unexported error | 1 | None | One per non-CLI, non-unset input |

On a nil or uninitialized `*Session`, `Session.Error()` never panics, classifies from each container's own `Source()`, and omits `from:` lines and the usage block. On an initialized session, passing an `Input` that is not bound to an active command in the command chain panics.

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
| Attached value on `--why` without `conga.WhyOptionJSON()`, or invalid format passed to `--why` with it enabled | `*conga.UsageError` | 2 |
| Error returned from `Bootstrapper.Bootstrap` | Returned as is | 1, or the `ExitCoder`'s code |

## Schema errors

Programmer errors during schema construction abort at startup before execution begins (exit code 1). Each one prints its message on line 1, and many add a `note:` line with the detail and a `hint:` line with the fix:
- Declaring `conga.Features[T]` or `feature:"..."` tags without calling `app.EnableFeatures()`.
- Declaring both `env` and `envprefix` on a `conga.Features` container, or a feature-field `env` tag under a combined-map container.
- Binding one environment variable through two feature fields (explicitly or through prefix derivation), or through a feature and a flag.
- Binding a flag or feature to the environment variable the app reads a config file path from (`<PREFIX>_CONFIG`, or the `ConfigFile` field's `env` tag). An example is `struct CLI field Config: environment variable "PC_CONFIG" is bound by both the config file variable and flag --config`, with a `hint: give the flag another env tag or exclude it with env:"-"` line.
- Combining `conga.ConfigExplicit(...)` with `conga.ConfigPaths(...)` or `conga.ConfigCascading()`.
- Declaring the `bootstrap` tag when neither the declaring command nor an ancestor implements `conga.Bootstrapper`.
- Combining the `bootstrap` and `feature` tags on the same field.
- Declaring the `bootstrap` tag on unsupported field types (structs, pointers, interfaces).
- Inconsistent `stage` tags violating the all-or-nothing requirement on feature structs.
- Declaring `enum` on a path field (`conga.File`, `conga.Dir`, `conga.FileContent`, a filterable type, or a `file`, `dir`, `exec`, or `symlink` predictor tag): `struct Deploy field Manifest: tag "enum" is not supported on conga.File`, with a `hint: use tag "glob" to filter completion candidates, or a string field with tag "enum" to accept a fixed set of names` line.
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
- A flag or positional argument whose type, slice element type, or map key or value type has no parser: `struct CLI field Zone: unsupported type *time.Location`, with a `hint: register a parser with app.Use(timezone.RegisterLocation)` line. The hint names `app.RegisterType`, or the domain-package binding for `*time.Location`, `*user.User`, `*user.Group`, and `*net.Interface`.
- A leaf command that does not implement `conga.Runner`: `struct LeafCmd: runner not implemented`, with a `hint: implement conga.Runner: Run(context.Context, *conga.Session) error` line.
- A command struct implementing `Runner`, `Validator`, `PreRunner`, `PostRunner`, `Finalizer`, or `Bootstrapper` on its value type (via a value receiver, an embedded pointer mixin such as `*Base`, or an embedded interface such as `conga.Runner`): `struct Export: lifecycle hook Validate must use a pointer receiver` with `hint: change func (Export) Validate to func (*Export) Validate`, `struct Export: lifecycle hook Validate cannot be promoted from embedded pointer *Base` with `hint: embed Base by value or implement func (*Export) Validate`, or `struct Export: lifecycle hook Run cannot be promoted from embedded interface Runner` with `hint: store the dependency in a named field and implement func (*Export) Run`. Value-embedded mixins with pointer-receiver hooks stay allowed, and `SelfValidator` on domain types is unaffected.

## Panics

Developer misuse of the API panics with a fixed message instead of returning an error. `Run` returns schema errors as ordinary errors instead of panicking.

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
| `session.Error().For(...)` passed an unbound or unselected `Input` on an initialized session | `conga: Session.Error().For: input is not bound to the active command` |
| A method called on a nil `*ConfigDocument` | `conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)` |

A panic whose message starts with `conga: invariant violation:` is a bug in CONGA, not misuse, and is worth reporting.

## Custom error handling

Custom error handling reports the error returned by `app.Run()` or `app.RunContext(ctx)` itself instead of delegating to `app.Exit(err)`. `conga.DiagnosticOf(err)` returns a `conga.Diagnostic` whose `Members()` give the lines, `app.UsageBlock(err)` returns the usage block that goes with the error, and `app.ExitCode(err)` returns the exit code. The snippet uses its own prefix and no color. `UsageBlock` returns `""` when `app.Exit` prints no block. When it returns text, `app.Exit` prints a blank line before it only if it printed error lines, and a newline after it, as the snippet does:

```go
if err := app.RunContext(ctx); err != nil {
	printed := false

	for _, item := range conga.DiagnosticOf(err).Members() {
		fmt.Fprintf(os.Stderr, "Error: %s\n", item.Message())
		for _, origin := range item.Origins() {
			fmt.Fprintf(os.Stderr, "  from: %s\n", origin)
		}
		if note := item.Note(); note != "" {
			fmt.Fprintf(os.Stderr, "  note: %s\n", note)
		}
		if hint := item.Hint(); hint != "" {
			fmt.Fprintf(os.Stderr, "  hint: %s\n", hint)
		}

		printed = true
	}

	if block := app.UsageBlock(err); block != "" {
		if printed {
			fmt.Fprintln(os.Stderr)
		}

		fmt.Fprintln(os.Stderr, block)
	}

	os.Exit(app.ExitCode(err))
}
```

A failure from the environment, a config file or a default is not a `*conga.UsageError`, so `UsageBlock` returns `""` for it ([which failures are usage errors](#which-failures-are-usage-errors)). A value error from the command line also gets `""`. A nested app's `*conga.UsageError` that an outer app returns follows the outer app's `UsageOnError` mode.

[About error reporting](/explanation/error-reporting/) covers the separation of error values from process termination.

## Usage rendering on errors

`UsageOnError(mode)` controls what `Exit` prints below a usage error:

| Mode | Output |
|------|--------|
| `conga.UsageSummary` (default) | Clean error, 1–2 line syntax, `--help` hint |
| `conga.UsageFull` | Clean error plus the complete help screen |
| `conga.UsageHint` | Clean error plus a single `Run '<cmd> --help'` hint |
| `conga.UsageSilence` | The error and its `from:`, `note:` and `hint:` lines, no usage block |

Three shapes follow from the kind of failure:

- Command-line value errors name the flag or argument and the offending value, so they render no usage block in any mode. These are coercion failures (`unknown byte size unit "XB" in "5XB" for flag --max-memory`, `ambiguous byte size unit "KB" in "5KB" for flag --max-memory`), value-constraint violations (`"/etc/passwd" is a file, but a directory was expected for flag --dir`, `value 150 is out of range [0..100] for flag --score`), and single-input command-line errors from `session.Error().For(&flag)` when `.Usage(true)` is omitted.
- Errors that benefit from usage context keep the mode's rendering. These are unknown commands and flags, missing required flags and arguments, mutually exclusive or grouped flags, stdin read twice, an unknown feature on the command line, the errors from `session.UsageError`, and `session.Error()` with `.Usage(true)` or two or more named inputs where at least one came from the command line.
- A missing subcommand prints the full help of its command for `UsageSummary` and `UsageFull`, the hint for `UsageHint`, and no usage block for `UsageSilence`.

The usage block comes from the error's own command, so a `*conga.UsageError` without one (a nil session, or a zero value) prints no block. Environment-, configuration-, and default-sourced value failures are not usage errors, so they render `app: error: ...` with their `from:`, `note:` and `hint:` lines, exit 1, and no usage block. A group failure whose members all came from the environment or a config file renders the same way.
