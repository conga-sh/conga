---
title: About error reporting
description: How execution separates error values from process termination, when developer defects panic, and why Exit is an optional convenience.
---

Command execution in CONGA separates error evaluation from process termination. The execution methods `app.Run` and `app.RunContext` return command-line mistakes and runtime failures as error values instead of calling `os.Exit`. Callers decide how to handle failures, whether by inspecting the error directly or delegating terminal formatting and exit status codes to the optional `app.Exit(err)` convenience method.

## Errors as values across the pipeline

Many command-line libraries end the process when parsing fails. The standard library's `flag.Parse` prints the error and calls `os.Exit` on invalid input. A test of such a program has to run it as a subprocess, and its commands can't be embedded in a server, a GUI application, or a test harness.

In CONGA, `app.Run()` and `app.RunContext(ctx)` parse, coerce, validate, and run the command, then return whatever failed as a Go `error`:

- `nil` on clean execution, including built-in commands. Requests for `--help`, `--version`, and opt-in diagnostics like `--why` and `--completion` render to standard output and return `nil`. A help request succeeds, so `if err != nil` handles only real failures.
- A `*conga.UsageError` when command-line input violates syntax, naming, or value constraints, or when a hook returns `session.UsageError` or `session.Error()` with a command-line input or `.Usage(true)`.
- An ordinary error when a value from an environment variable, a configuration file, or a default tag fails to coerce or validate, or when `session.Error()` receives only non-CLI inputs or `.Usage(false)`. It exits with status 1 and names each value's origin on a `from:` line ([which failures are usage errors](/reference/errors/#which-failures-are-usage-errors)).
- The error a lifecycle hook returns, as is. A `Validate`, `Bootstrap`, `PreRun`, `Run`, or `PostRun` error keeps its text, type, and identity, and `Finally` receives the same value. A `Validate` or `Bootstrap` failure comes back before any `Finally` hook runs. When `Finally` hooks that do run also fail, the run's error remains the primary error and the `Finally` errors are suppressed alongside it (or, when stages succeeded and more than one `Finally` hook fails, the first becomes the primary). All of them print through `app.Exit` and appear in `conga.DiagnosticOf(err).Members()`, while `errors.Is`, `errors.As`, and `app.ExitCode` inspect only the primary error.

## Pipeline errors versus programmer defects

CONGA splits failures by who has to fix them:

- Pipeline errors are user mistakes and execution failures, such as invalid flags, unknown subcommands, values that fail to parse, violated constraints, missing arguments, and failures returned by a command's `Run` method. They are normal conditions for a CLI, so `Run` and `RunContext` return them as `error` values, and the application decides how to inspect, log, and exit.
- Programmer defects are developer contract violations, such as passing a nil `context.Context` to `RunContext`, attempting to execute an `App` instance a second time, registering duplicate or empty predictor names, or misusing a builder (`ConfigPaths` with no paths). CONGA panics on these with a fixed message, such as `"conga: nil Context"` or `"conga: App already executed"`, as Go code conventionally does for programmer errors. A panic marks a defect that needs a code change, not an error to handle at run time. Schema build returns schema errors from `Run` as ordinary errors. [Panics](/reference/errors/#panics) lists every panic and its message.

## The optional Exit convenience

Standalone command-line applications translate errors into standard error messages and an exit status code. `app.Exit(err)` accepts the value returned by `app.Run` or `app.RunContext(ctx)` and maps it to a POSIX status. `Exit` takes the outermost `conga.ExitCoder` in the error chain and exits with its code (or `128` plus the signal number when `app.Run` caught a signal and no author `ExitCoder` overrides it). A `*conga.UsageError` reports 2, so a script can tell that the command failed before any work began because its command line was wrong. An error that is not an `ExitCoder` exits with status 1. [Exit codes](/reference/errors/#exit-codes) lists every status.

`Exit` terminates through the configured exit function (`app.ExitFunc`, default `os.Exit`). It returns only when that function returns without terminating. All `PostRun` and `Finally` hooks complete inside `app.Run`, so the teardown work is already done by the time `Exit` runs.

`Exit` is optional. Applications that embed CLI commands, collect telemetry, or format errors using structured loggers like `slog` can omit `Exit` and handle the returned error value directly. Passing any error to `conga.DiagnosticOf(err)` returns an opt-in `conga.Diagnostic` that reads origins, note, and hint by the chain rule and implements `json.Marshaler`, `encoding.TextMarshaler`, and `fmt.Formatter`, so `slog`'s JSON handler records the pieces as separate keys and its text handler writes a compact line. `app.UsageBlock(err)` and `app.ExitCode(err)` return the usage block and exit status `Exit` uses. Programs that never call `DiagnosticOf` do not link it into the binary. When callers do use `Exit`, setting `app.ExitFunc(handler)` intercepts the requested exit status code without killing the test process, allowing tests to verify exit behavior in-process.

## Value errors print no usage block

`Exit` matches the help it prints to the kind of mistake. Unknown flags, unknown subcommands, and missing required positional arguments are interface-discovery mistakes, so `Exit` pairs them with a syntax usage block or `--help` hint. Command-line coercion and constraint failures already name the flag and the offending value, so they render without the usage block. Single-input command-line errors from `session.Error().For(&flag)` follow the same shape, while multi-input errors from `session.Error().For(&a, &b)` print the usage block because they involve a relationship between inputs. Environment-, configuration-, and default-sourced value failures exit with status 1 and print the error line with its `from:`, `note:` and `hint:` lines, and no usage block. Printing a 40-line usage screen below a value error scrolls the terminal window and buries the actual message under redundant text. [Errors and exit codes](/reference/errors/#usage-rendering-on-errors) lists which failures render without a usage block.

## Telling failures apart by type

CONGA reports the kind of a failure through its type. `errors.AsType[*conga.UsageError](err)` reports a command-line mistake and gives access to its `CommandPath`. `app.UsageBlock(err)` returns the usage block `Exit` prints for it, and `errors.AsType[conga.ExitCoder](err)` finds the exit code. A cause that CONGA did not write itself, such as an error returned by a `TextUnmarshaler`, a standard library error, or an error from the application, stays reachable through `errors.Is` and `errors.As`. The other concrete error types stay unexported, so their representation can change without breaking callers. [Errors and exit codes](/reference/errors/) catalogs the types.

For testing patterns, see [How to test a command](/how-to/test-a-command/).
