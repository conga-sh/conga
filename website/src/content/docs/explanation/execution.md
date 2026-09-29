---
title: About app execution
description: Run and RunContext, the context they hand to the pipeline, the single-execution rule, and stream capture.
---

`app.Run()` and `app.RunContext(ctx)` are the two entry points into the application pipeline. Both parse the command line, resolve and validate values, and call the lifecycle hooks; they differ in the context the pipeline runs under.

## Run and RunContext

| Method | Context | Use when |
|--------|---------|----------|
| `Run()` | Derived from `context.Background()` through `signal.NotifyContext` for `SIGINT` and `SIGTERM` | A standalone program's `main` function |
| `RunContext(ctx)` | The caller's context, passed through untouched | Embedding in a host process, tests, or a caller that manages cancellation and deadlines |

`Run()` is the conventional entry point for a command-line program. It listens for Ctrl+C (`os.Interrupt`) and `SIGTERM`, cancels the pipeline context when either arrives, and stops listening when execution returns. The signal-aware behaviour exists only in `Run()`.

`RunContext` leaves cancellation to the caller. The supplied context reaches every lifecycle hook (`Validate`, `PreRun`, `Run`, `PostRun`, `Finally`) and any `SelfValidator`; after cancellation, hooks observe it through `ctx.Err()`. A nil context is a programming mistake, so `RunContext` panics with `conga: nil Context`. Tests call `RunContext(t.Context())` so the test's cancellation propagates into the command.

## One execution per App

An `App` executes exactly once. A second `Run` or `RunContext` call panics with `conga: App already executed`: the builder is captured when execution starts and the pipeline has no reset. A host that serves several invocations builds one `App` per invocation. A running command that needs fresh configuration calls [`session.Reload[T]`](/reference/conga/#session-and-hierarchy-accessors), which re-evaluates the configuration cascade into a new command struct.

## Standard streams and I/O

CONGA routes I/O through `session.Stdin()`, `session.Stdout()`, and `session.Stderr()`.

By default, the engine connects these to the standard process streams (`os.Stdin`, `os.Stdout`, `os.Stderr`). When commands write through `session.Stdout()` rather than calling `fmt.Println` or writing directly to `os.Stdout`, they remain testable and embeddable. Automated tests and host applications can replace the streams on the builder (`app.Stdout(&buffer)`) and inspect output directly without hijacking global process state.

The active session snapshots streams and arguments (`os.Args[1:]` by default) when execution starts. Builder calls made after `Run` or `RunContext` begins do not affect the running command. Built-in output (`--help`, `--version`, `--why`, and shell completion scripts) also writes exclusively through these configured streams.

## Execution returns an error

Both entry points return `nil` on success, including built-in requests (`--help`, `--version`, and the opt-in `--why` and `--completion`), which render their output and finish normally. Failures come back as errors: usage errors for invalid input, and runtime errors raised by the command body. The App neither prints nor exits on its own; [`FatalIfError`](/explanation/error-reporting/) is the boundary that does. The hook order around the command body is covered in [About the lifecycle](/explanation/lifecycle/).
