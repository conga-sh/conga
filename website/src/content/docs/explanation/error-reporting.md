---
title: About error reporting
description: How execution separates error values from process termination, when developer defects panic, and why FatalIfError is an optional convenience.
---

Command execution in CONGA separates error evaluation from process termination: the execution methods `app.Run` and `app.RunContext` return command-line mistakes and runtime failures as plain error values instead of calling `os.Exit`. Callers decide how to handle failures, whether by inspecting the error directly or delegating terminal formatting and exit status codes to the optional `app.FatalIfError(err)` convenience method.

## Errors as values across the pipeline

Command-line frameworks often conflate argument parsing with process termination. In Go's standard `flag` package and many third-party libraries, calling `Parse()` on invalid input prints to standard error and calls `os.Exit`. Hardcoded process exits complicate unit testing by requiring tests to run commands inside subprocesses, and prevent using CLI commands as embedded components within servers, GUI applications, or test harnesses.

CONGA treats command execution as a value pipeline that produces standard Go errors. Calling `app.Run()` or `app.RunContext(ctx)` executes the parsing, coercion, validation, and runner stages, returning:

- `nil` on clean execution, including built-in commands. Requests for `--help`, `--version`, and opt-in diagnostics like `--why` and `--completion` render to standard output and return `nil`. In frameworks that model help as a sentinel error (such as `flag.ErrHelp`), callers must write boilerplate to intercept and suppress the pseudo-error. In CONGA, requesting help is a successful invocation, ensuring that standard Go error handling (`if err := app.Run(); err != nil`) triggers only on genuine failures.
- A usage error when command-line input violates syntax, naming, or value constraints.
- A runtime error when the active command's `Run` method or a lifecycle hook returns a failure.

## Pipeline errors versus programmer defects

CONGA divides failures into two distinct classes based on who is responsible:

- Pipeline errors (operator mistakes and execution failures): Invalid flags, unknown subcommands, value coercion failures, violated constraints, missing arguments, and failures returned by a command's `Run` method. These represent normal operating conditions. `Run` and `RunContext` return them as `error` values so the application retains full control over inspection, structured logging, and exit handling.
- Programmer defects (developer contract violations): Passing a nil `context.Context` to `RunContext`, attempting to execute an `App` instance a second time, registering duplicate or empty predictor names, or supplying invalid schema configurations. In accordance with standard Go conventions, CONGA panics immediately with a fixed descriptive message (such as `"conga: nil Context"` or `"conga: App already executed"`). Panics denote unrecoverable programming defects that require code fixes rather than runtime error handling.

## The optional FatalIfError convenience

Standalone command-line applications typically translate errors into standard error messages and an exit status code. To eliminate repetitive error-handling boilerplate, CONGA provides `app.FatalIfError(err)` as an optional convenience helper:

- When `err` is `nil`, the method returns immediately and execution continues.
- When `err` is non-nil, the method formats the error message, writes it to the configured standard error stream, prints contextual usage if appropriate, and calls the configured exit function (defaulting to `os.Exit`).

Exit codes follow POSIX and Unix shell conventions:

- Exit code 2 indicates a command usage error (unknown flags, missing arguments, invalid values) or an internal completion failure. Scripts inspect status 2 to determine that the command failed before work began because the operator supplied invalid syntax.
- Exit code 1 indicates an operational or runtime failure returned by a command body or lifecycle hook.
- Exit code 0 indicates success.

`FatalIfError` is entirely optional. Applications that embed CLI commands, collect telemetry, or format errors using structured loggers like `slog` can omit `FatalIfError` and handle the returned error value directly. When callers do use `FatalIfError`, setting `app.ExitFunc(handler)` intercepts the requested exit status code without killing the test process, allowing tests to verify exit behavior in-process.

## Self-describing value errors

When an operator supplies invalid input, the nature of the error dictates what information is helpful on standard error. CONGA distinguishes between two kinds of input failures:

1. Syntax mistakes: Unknown flags (`--nmae`), unknown subcommands, or missing required positional arguments indicate that the operator does not know the command interface. In these cases, printing a syntax usage block or `--help` hint alongside the error message guides the operator toward the correct command form.
2. Value constraint violations: Supplying `--port abc` or `--memory 100XB` represents a semantic error rather than a syntax failure. The operator knows the flag name, but supplied an unparseable or out-of-bounds value.

For value errors, the error message already names the flag, the invalid input, and the reason (`cannot parse "abc" as int for flag --port`). CONGA treats these as self-describing errors. When `FatalIfError` encounters a self-describing error, it suppresses the syntax usage block across all `UsageOnError` modes. Printing a 40-line usage screen below a value error scrolls the terminal window and buries the actual message under redundant text.

## Categorical classification without concrete coupling

CONGA categorizes every pipeline failure into five base sentinels: `ErrSchema`, `ErrParse`, `ErrCoerce`, `ErrValidation`, and `ErrConfig`.

The underlying struct implementations that format these errors remain unexported. Applications inspect and handle errors using standard Go error primitives (`errors.Is(err, conga.ErrValidation)`) or the `conga.IsUsageError(err)` predicate, rather than asserting on concrete internal types. This boundary allows the internal error representation to evolve without breaking downstream callers.

For mode options and complete exit code specifications, see [Errors and exit codes](/reference/errors/). For testing patterns, see [How to test a command](/how-to/test-a-command/).
