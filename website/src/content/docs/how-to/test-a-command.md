---
title: How to test a command
description: Inject arguments, streams, and an exit handler, run the app in a test, and assert output and exit codes.
---

Testing a CONGA command means constructing the App in the test, replacing its inputs and outputs, and executing the same pipeline the binary uses. The engine writes through the injected pieces, so assertions read the bytes a user would see.

## Capture output and arguments

`Args` replaces the process argument vector, and `Stdout`/`Stderr` replace the streams the engine and the command write to. Build the App with all three, then run it with `RunContext(t.Context())`:

```go title="main_test.go"
package main

import (
	"bytes"
	"testing"

	"github.com/conga-sh/conga"
)

func TestHelloGreetsAliceLoudly(t *testing.T) {
	var stdoutBuffer, stderrBuffer bytes.Buffer

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--name", "Alice", "--loud"}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer)

	err := app.RunContext(t.Context())

	if err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if got, want := stdoutBuffer.String(), "HELLO, ALICE!\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if got := stderrBuffer.String(); got != "" {
		t.Errorf("stderr = %q, want no output", got)
	}
}
```

The command writes through `session.Stdout()`, which is the writer supplied to `app.Stdout(...)`, so the greeting lands in `stdoutBuffer`. `Args` keeps the test off `os.Args`, and `RunContext(t.Context())` propagates the test's cancellation instead of installing signal handlers — see [About app execution](/explanation/execution/). Streams are captured when execution starts, so they must be configured on the builder before `RunContext`.

## Assert exit codes without exiting

The default exit handler is `os.Exit`. `ExitFunc` replaces it, which lets the test observe the code that `FatalIfError` requests. Add `strings` to the import block and the second test to the file:

```go title="main_test.go"
func TestHelloUnknownFlagExitsTwo(t *testing.T) {
	var stdoutBuffer, stderrBuffer bytes.Buffer

	exitCode := 0

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--nmae", "Alice"}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code })

	err := app.RunContext(t.Context())

	if !conga.IsUsageError(err) {
		t.Fatalf("expected a usage error, got %v", err)
	}

	app.FatalIfError(err)

	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}

	if !strings.Contains(stderrBuffer.String(), "unknown flag: --nmae") {
		t.Errorf("stderr = %q, want the unknown flag message", stderrBuffer.String())
	}
}
```

`RunContext` returns the usage error without printing it; `FatalIfError` renders the message to `stderrBuffer` and calls the exit handler with `2` instead of terminating the test process — see [About error reporting](/explanation/error-reporting/). Add `UsageOnError(conga.UsageSilence)` to the builder chain when the test inspects only the message and does not need the usage block on stderr.

## Test built-in requests

Built-in requests are success paths: they render to stdout and return `nil`. Assert the rendered output and the nil error:

```go title="main_test.go"
func TestHelloHelpSucceeds(t *testing.T) {
	var stdoutBuffer, stderrBuffer bytes.Buffer

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--help"}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer)

	err := app.RunContext(t.Context())

	if err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if !strings.Contains(stdoutBuffer.String(), "Usage:\n  hello [flags]") {
		t.Errorf("stdout = %q, want the usage block", stdoutBuffer.String())
	}
}
```

## Keep the App single-use

An App executes exactly once; a second `Run` or `RunContext` call panics with `conga: App already executed`. Each test case therefore constructs a fresh App, which also means a fresh command struct for every case, and subtests do not share one instance.

## The complete test file

```go title="main_test.go"
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/conga-sh/conga"
)

func TestHelloGreetsAliceLoudly(t *testing.T) {
	var stdoutBuffer, stderrBuffer bytes.Buffer

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--name", "Alice", "--loud"}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer)

	err := app.RunContext(t.Context())

	if err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if got, want := stdoutBuffer.String(), "HELLO, ALICE!\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if got := stderrBuffer.String(); got != "" {
		t.Errorf("stderr = %q, want no output", got)
	}
}

func TestHelloHelpSucceeds(t *testing.T) {
	var stdoutBuffer, stderrBuffer bytes.Buffer

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--help"}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer)

	err := app.RunContext(t.Context())

	if err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if !strings.Contains(stdoutBuffer.String(), "Usage:\n  hello [flags]") {
		t.Errorf("stdout = %q, want the usage block", stdoutBuffer.String())
	}
}

func TestHelloUnknownFlagExitsTwo(t *testing.T) {
	var stdoutBuffer, stderrBuffer bytes.Buffer

	exitCode := 0

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--nmae", "Alice"}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code })

	err := app.RunContext(t.Context())

	if !conga.IsUsageError(err) {
		t.Fatalf("expected a usage error, got %v", err)
	}

	app.FatalIfError(err)

	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}

	if !strings.Contains(stderrBuffer.String(), "unknown flag: --nmae") {
		t.Errorf("stderr = %q, want the unknown flag message", stderrBuffer.String())
	}
}
```

## Where to go next

- [About app execution](/explanation/execution/) — `RunContext`, context propagation, and stream capture.
- [About error reporting](/explanation/error-reporting/) — classification and `FatalIfError` customisation.
- [About reading values](/explanation/reading-values/) — the container reader surface.
- [How to run the examples](/how-to/examples-cookbook/) — example 26 embeds the App in a host process with injected streams and a custom exit handler.
