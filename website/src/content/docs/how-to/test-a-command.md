---
title: How to test a command
description: Inject arguments and in-memory streams, intercept exit codes, and assert command behavior in Go unit tests.
---

Test CONGA applications in-process by constructing the `App` directly, injecting an in-memory argument slice and I/O buffers, and executing with `RunContext(t.Context())`.

## Isolate arguments and I/O streams

Inject custom arguments and memory buffers using `Args`, `Stdout`, and `Stderr`:

```go title="main_test.go"
package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Name conga.OptionalFlag[string] `default:"World" help:"Name of the person to greet"`
	Loud conga.OptionalFlag[bool]   `help:"Print greeting in uppercase"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())
	if c.Loud.Get() {
		greeting = strings.ToUpper(greeting)
	}

	fmt.Fprintln(session.Stdout(), greeting)

	return nil
}

func TestHelloGreetsUser(t *testing.T) {
	t.Parallel()

	var stdoutBuffer, stderrBuffer bytes.Buffer

	app := conga.New[CLI]().
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

## Isolate environment variables

`t.Setenv` sets a variable for the duration of the test and registers cleanup that restores the previous value:

```go title="main_test.go"
func TestNameFromEnvironment(t *testing.T) {
	t.Setenv("HELLO_NAME", "Alice")

	var stdoutBuffer bytes.Buffer

	app := conga.New[CLI]().
		EnvPrefix("HELLO").
		Args([]string{}).
		Stdout(&stdoutBuffer)

	err := app.RunContext(t.Context())
	if err != nil {
		t.Fatalf("RunContext() unexpected error: %v", err)
	}

	if got, want := stdoutBuffer.String(), "Hello, Alice!\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}
```

When the application loads `.env` files with `Dotenv`, the loaded variables land in the process environment. Chain `SetenvFunc(t.Setenv)` so every loaded variable is restored when the test completes:

```go title="main_test.go"
func TestDotenvFile(t *testing.T) {
	var stdoutBuffer bytes.Buffer

	app := conga.New[CLI]().
		EnvPrefix("HELLO").
		Dotenv("testdata/hello.env").
		SetenvFunc(t.Setenv).
		Args([]string{}).
		Stdout(&stdoutBuffer)

	err := app.RunContext(t.Context())
	if err != nil {
		t.Fatalf("RunContext() unexpected error: %v", err)
	}

	if got, want := stdoutBuffer.String(), "Hello, Alice!\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}
```

With `testdata/hello.env` containing `HELLO_NAME=Alice`, the loaded variable binds the `Name` flag and `t.Setenv` restores `HELLO_NAME` after the test.

`t.Setenv` mutates process-wide state, so tests that use it (directly or through `SetenvFunc`) cannot call `t.Parallel`.

## Assert exit codes

By default, `app.Exit` terminates the process using `os.Exit`. To assert exit codes in tests without terminating the test runner, replace the exit handler using `ExitFunc`:

```go title="main_test.go"
func TestUnknownFlagReportsUsageError(t *testing.T) {
	t.Parallel()

	var stdoutBuffer, stderrBuffer bytes.Buffer
	exitCode := 0

	app := conga.New[CLI]().
		Args([]string{"--unknown-flag"}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code })

	err := app.RunContext(t.Context())

	// Syntax, validation, and flag errors satisfy IsUsageError
	if !conga.IsUsageError(err) {
		t.Fatalf("expected usage error, got %v", err)
	}

	// Exit prints the usage diagnostic and triggers ExitFunc
	app.Exit(err)

	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2 for usage error", exitCode)
	}

	if !strings.Contains(stderrBuffer.String(), "unknown flag: --unknown-flag") {
		t.Errorf("stderr = %q, missing unknown flag message", stderrBuffer.String())
	}
}
```

`Exit` maps errors to standardized exit codes: code 2 when `conga.IsUsageError(err)` reports a usage error, and code 1 for schema-build failures and runtime failures returned from `Run`. A nil error exits with code 0.

To test error messages without dumping the full usage block to stderr, add `UsageOnError(conga.UsageSilence)` to the builder chain.

## Test built-in flags

Built-in requests (`--help`, `--version`, and `--why`) execute cleanly, writing to `Stdout` and returning `nil`:

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
		{"default", nil, "Hello, World!\n"},
		{"custom", []string{"--name", "Bob"}, "Hello, Bob!\n"},
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
