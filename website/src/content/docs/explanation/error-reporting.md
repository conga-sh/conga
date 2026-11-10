---
title: About error reporting
description: How execution separates error values from process termination, when developer defects panic, and why Exit is an optional convenience.
---

Command execution in CONGA separates error evaluation from process termination. The execution methods `app.Run` and `app.RunContext` return command-line mistakes and runtime failures as plain error values instead of calling `os.Exit`. Callers decide how to handle failures, whether by inspecting the error directly or delegating terminal formatting and exit status codes to the optional `app.Exit(err)` convenience method.

## Errors as values across the pipeline

Many command-line libraries end the process when parsing fails. The standard library's `flag.Parse` prints the error and calls `os.Exit` on invalid input. A test of such a program has to run it as a subprocess, and its commands can't be embedded in a server, a GUI application, or a test harness.

In CONGA, `app.Run()` and `app.RunContext(ctx)` parse, coerce, validate, and run the command, then return an ordinary Go error:

- `nil` on clean execution, including built-in commands. Requests for `--help`, `--version`, and opt-in diagnostics like `--why` and `--completion` render to standard output and return `nil`. A help request succeeds, so `if err != nil` handles only real failures.
- A usage error when command-line input violates syntax, naming, or value constraints.
- A runtime error when the active command's `Run` method or a lifecycle hook returns a failure.

## Pipeline errors versus programmer defects

CONGA splits failures by who has to fix them:

- Pipeline errors are user mistakes and execution failures, such as invalid flags, unknown subcommands, values that fail to parse, violated constraints, missing arguments, and failures returned by a command's `Run` method. They are normal conditions for a CLI, so `Run` and `RunContext` return them as `error` values, and the application decides how to inspect, log, and exit.
- Programmer defects are developer contract violations, such as passing a nil `context.Context` to `RunContext`, attempting to execute an `App` instance a second time, registering duplicate or empty predictor names, or misusing a builder (`ConfigPaths` with no paths). CONGA panics on these with a fixed message, such as `"conga: nil Context"` or `"conga: App already executed"`, as Go code conventionally does for programmer errors. A panic marks a defect that needs a code change, not an error to handle at run time. Schema errors are not panics. Schema build returns them from `Run` as `ErrSchema`. [Panics](/reference/errors/#panics) lists every panic and its message.

## The optional Exit convenience

Standalone command-line applications translate errors into standard error messages and an exit status code. `app.Exit(err)` accepts the value returned by `app.Run` or `app.RunContext(ctx)` and maps it to a POSIX status. A usage error exits with status 2, so a script can tell that the command failed before any work began because its command line was wrong. [Exit codes](/reference/errors/#exit-codes) lists every status.

`Exit` terminates through the configured exit function (`app.ExitFunc`, default `os.Exit`). It returns only when that function returns without terminating. All `PostRun` and `Finally` hooks complete inside `app.Run`, so the teardown work is already done by the time `Exit` runs.

`Exit` is optional. Applications that embed CLI commands, collect telemetry, or format errors using structured loggers like `slog` can omit `Exit` and handle the returned error value directly. When callers do use `Exit`, setting `app.ExitFunc(handler)` intercepts the requested exit status code without killing the test process, allowing tests to verify exit behavior in-process.

## Self-describing value errors

`Exit` matches the help it prints to the kind of mistake. Unknown flags, unknown subcommands, and missing required positional arguments are interface-discovery mistakes, so `Exit` pairs them with a syntax usage block or `--help` hint. Coercion and constraint failures already name the flag and the offending value, so they render without the usage block. Printing a 40-line usage screen below a value error scrolls the terminal window and buries the actual message under redundant text. [Errors and exit codes](/reference/errors/#usage-rendering-on-errors) lists which failures count as self-describing.

## Categorical classification without concrete coupling

Every pipeline failure wraps one of five sentinel errors, such as `ErrParse` or `ErrConfig`, and `conga.IsUsageError(err)` reports whether it is a usage error. The concrete error types stay unexported, so their representation can change without breaking callers. [Errors and exit codes](/reference/errors/) catalogs the sentinels and the `UsageError` interface.

For testing patterns, see [How to test a command](/how-to/test-a-command/).
