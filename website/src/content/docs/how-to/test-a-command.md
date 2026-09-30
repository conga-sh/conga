---
title: How to test a command
description: Inject arguments and in-memory streams, intercept exit codes, and assert command behavior in Go unit tests.
---

Testing CONGA applications does not require spawning external processes or mutating global process state. Construct the `App` directly in your test, inject an in-memory argument slice and I/O buffers, and execute the application with `RunContext(t.Context())`.

## Isolate arguments and I/O streams

Inject custom arguments and memory buffers using `Args`, `Stdout`, and `Stderr`:

```go title="main_test.go"
package main

import (
	"bytes"
	"testing"

	"github.com/conga-sh/conga"
)

func TestHelloGreetsUser(t *testing.T) {
	t.Parallel()

	var stdoutBuffer, stderrBuffer bytes.Buffer

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--name", "Alice", "--loud"}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer)

	err := app.RunContext(t.Context())
	if err != nil {
		t.Fatalf("RunContext() unexpected error: %v", err)
	}

	if got, want := stdoutBuffer.String(), "HELLO, ALICE!\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if stderrBuffer.Len() > 0 {
		t.Errorf("stderr = %q, want empty", stderrBuffer.String())
	}
}
```

Configuring `Args` leaves `os.Args` untouched, preventing race conditions when tests run in parallel.

Streams and arguments are snapshotted when `RunContext` begins. Builder calls made after execution has started do not affect the running session.

## Assert exit codes without terminating the test runner

By default, `app.FatalIfError` terminates the process using `os.Exit`. To assert exit codes in tests without terminating the test runner, replace the exit handler using `ExitFunc`:

```go title="main_test.go"
func TestUnknownFlagReportsUsageError(t *testing.T) {
	t.Parallel()

	var stdoutBuffer, stderrBuffer bytes.Buffer
	exitCode := 0

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--unknown-flag"}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code })

	err := app.RunContext(t.Context())

	// Syntax, validation, and flag errors satisfy IsUsageError
	if !conga.IsUsageError(err) {
		t.Fatalf("expected usage error, got %v", err)
	}

	// FatalIfError prints the usage diagnostic and triggers ExitFunc
	app.FatalIfError(err)

	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2 for usage error", exitCode)
	}

	if !strings.Contains(stderrBuffer.String(), "unknown flag: --unknown-flag") {
		t.Errorf("stderr = %q, missing unknown flag message", stderrBuffer.String())
	}
}
```

`FatalIfError` maps errors to standardized exit codes: code 2 for usage and schema errors (`conga.IsUsageError(err) == true`), and code 1 for runtime failures returned from `Run`.

To test error messages without dumping the full usage block to stderr, add `UsageOnError(conga.UsageSilence)` to the builder chain.

## Test built-in flags

Built-in requests (`--help`, `--version`, and `--why`) are valid execution paths, not errors. They write to `Stdout` and return `nil`:

```go title="main_test.go"
func TestHelpFlagRendersUsage(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--help"}).
		Stdout(&stdoutBuffer)

	err := app.RunContext(t.Context())
	if err != nil {
		t.Fatalf("RunContext() error = %v", err)
	}

	if !strings.Contains(stdoutBuffer.String(), "Usage:\n  hello [flags]") {
		t.Errorf("stdout = %q, want usage block", stdoutBuffer.String())
	}
}
```

## Construct fresh App instances per test

CONGA enforces a single-execution rule: an `App` instance executes exactly once. Calling `Run` or `RunContext` a second time on the same `App` panics with `conga: App already executed`.

Always call `conga.New[T]()` inside each test or table-driven test iteration to ensure a clean schema, fresh containers, and isolated state:

```go title="main_test.go"
func TestTable(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default", nil, "hello World\n"},
		{"custom", []string{"--name", "Bob"}, "hello Bob\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdoutBuffer bytes.Buffer

			app := conga.New[CLI]().
				Args(tc.args).
				Stdout(&stdoutBuffer)

			if err := app.RunContext(t.Context()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := stdoutBuffer.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
```

## Where to go next

- [About the command lifecycle](/explanation/lifecycle/) — context propagation, signal handling, and execution lifecycles.
- [About error reporting](/explanation/error-reporting/) — error categorization, exit codes, and usage silencing.
- [About flags and args](/explanation/flags-and-args/) — inspecting container state and provenance.
