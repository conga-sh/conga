---
title: About error reporting
description: How execution separates error values from process termination, when developer defects panic, and why Exit is an optional convenience.
---

Command execution in CONGA separates error evaluation from process termination: the execution methods `app.Run` and `app.RunContext` return command-line mistakes and runtime failures as plain error values instead of calling `os.Exit`. Callers decide how to handle failures, whether by inspecting the error directly or delegating terminal formatting and exit status codes to the optional `app.Exit(err)` convenience method.

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

## The optional Exit convenience

Standalone command-line applications typically translate errors into standard error messages and an exit status code. To eliminate repetitive error-handling boilerplate, CONGA provides `app.Exit(err)` as an optional convenience helper:

```go
if err := app.Run(); err != nil {
	app.Exit(err)
}
```

The method maps the result to a POSIX status:

- `nil`: exits with status 0 and writes nothing.
- A usage error: writes the diagnostic and contextual usage, then exits with status 2. Scripts inspect status 2 to determine that the command failed before work began because the operator supplied invalid syntax.
- A schema-build, configuration, or runtime failure: writes the error and exits with status 1.

`Exit` terminates through the configured exit function (`app.ExitFunc`, default `os.Exit`); it returns only when that function returns without terminating. All `PostRun` and `Finally` hooks complete inside `app.Run`, so the teardown work is already done by the time `Exit` runs. [Errors and exit codes](/reference/errors/#exit-codes) is the full contract.

`Exit` is optional. Applications that embed CLI commands, collect telemetry, or format errors using structured loggers like `slog` can omit `Exit` and handle the returned error value directly. When callers do use `Exit`, setting `app.ExitFunc(handler)` intercepts the requested exit status code without killing the test process, allowing tests to verify exit behaviour in-process.

## Self-describing value errors

When an operator supplies invalid input, the nature of the error dictates what information helps on standard error. Unknown flags, unknown subcommands, and missing required positional arguments are interface-discovery mistakes, so `Exit` pairs them with a syntax usage block or `--help` hint. Coercion and constraint failures already name the flag and the offending value, so they render without the usage block: printing a 40-line usage screen below a value error scrolls the terminal window and buries the actual message under redundant text. [Errors and exit codes](/reference/errors/#usage-rendering-on-errors) lists which failures count as self-describing.

## Categorical classification without concrete coupling

CONGA categorizes every pipeline failure into a small set of base sentinels and exposes classification through standard Go error primitives (`errors.Is`) and the `conga.IsUsageError(err)` predicate, rather than exporting the underlying struct implementations. This boundary allows the internal error representation to evolve without breaking downstream callers. [Errors and exit codes](/reference/errors/) catalogs the sentinels and the `UsageError` interface.

For testing patterns, see [How to test a command](/how-to/test-a-command/).
