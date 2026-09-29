---
title: About error reporting
description: Why pipeline failures stay values, how FatalIfError renders and exits, and the customisation points.
---

Every stage of the pipeline returns an error value; the App never terminates the process by itself. `app.FatalIfError(err)` is the single boundary where an error becomes a message on stderr and an exit status.

## Errors are values

`Run` and `RunContext` return:

- `nil` on success, including built-in requests. `--help`, `--version`, and the opt-in `--why` and `--completion` write their output to stdout and return `nil`, so `if err := app.Run(); err != nil` never fires on a valid request.
- A usage error for invalid input: unknown flags and commands, coercion failures, constraint violations, missing required input. Usage errors satisfy the `UsageError` interface, whose `CommandPath()` names the active invocation, and `conga.IsUsageError(err)` reports the classification without matching concrete types.
- A runtime error for a failure returned by the command body or a lifecycle hook.

All of them wrap one of the five base categories (`ErrSchema`, `ErrParse`, `ErrCoerce`, `ErrValidation`, `ErrConfig`). The category supports programmatic handling; the message carries the specifics: the flag or argument, the offending value, and often a closest-match suggestion.

## The FatalIfError boundary

`app.FatalIfError(err)` returns immediately when `err` is nil. Otherwise it writes `<name>: error: <message>` to the configured stderr, appends usage text for usage errors according to `UsageOnError`, and terminates:

| Failure | Output | Exit code |
|---------|--------|-----------|
| Usage error | Message plus the configured usage rendering | 2 |
| Runtime error | Message only | 1 |
| Internal completion failure | Diagnostic | 2 |

`UsageOnError` selects the rendering for usage errors: `UsageSummary` (the default: the error, a short syntax block, and a `--help` hint), `UsageFull` (the complete help screen), `UsageHint` (the hint alone), and `UsageSilence` (the message alone). Self-describing value errors (coercion failures and constraint violations whose message already names the flag and offending value) print only the message regardless of the mode. The full catalogue is in [Errors and exit codes](/reference/errors/).

When a user mistypes a flag, the default `UsageSummary` mode renders a closest-match suggestion alongside the syntax block:

```sh
hello --nmae World
```

```
hello: error: unknown flag: --nmae. Did you mean "name"?

Usage:
  hello [flags]

Run 'hello --help' for more information.
```

The process exits because a command-line tool has no caller to return to. Until `FatalIfError` is called, ordinary Go error handling stays in charge: the caller can inspect, log, or classify the error first.

## Customising the boundary

Two builder methods configure the boundary:

- `app.ExitFunc(handler)` replaces `os.Exit`, the default termination. Hosts that embed CONGA record the requested code and keep control; tests assert the code without ending the test binary — see [How to test a command](/how-to/test-a-command/).
- `app.UsageOnError(mode)` selects the usage rendering described above.

The name in `<name>: error:` is the configured `Name(...)`, or the invoked binary name when the engine can determine it.

An embedding host captures the code and continues:

```go
exitCode := 0
app := conga.New[Runner]().
	Name("batchrun").
	UsageOnError(conga.UsageSilence).
	ExitFunc(func(code int) { exitCode = code })

err := app.RunContext(ctx)
app.FatalIfError(err)
// the host decides what exitCode means for its process
```

A caller can also branch on the classification before the boundary; the error still carries the full message and the base category:

```go
if err := app.Run(); err != nil {
	if conga.IsUsageError(err) {
		// input mistake: emit telemetry, then exit 2
	}

	app.FatalIfError(err)
}
```
