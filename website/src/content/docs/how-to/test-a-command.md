---
title: How to test a command
description: Run a CONGA command inside a Go test with its own arguments, standard streams, and environment, and check its output and exit code.
---

When a command reads from `session.Stdin()` and writes to `session.Stdout()` and `session.Stderr()` instead of `os.Stdin` and `os.Stdout`, you can run and test it inside a `go test` function. Those methods return the process streams by default; in a test, the builder's `Args`, `Stdin`, `Stdout`, and `Stderr` methods replace the command line and streams for that `App` instance.

The builder's `Stdout` and `Stderr` methods set the writers that `session.Stdout()` and `session.Stderr()` return and leave `os.Stdout` and `os.Stderr` alone. Output from `fmt.Println` or anything written directly to `os.Stdout` goes to the test binary's own stdout instead of the test's buffer.

This guide tests a `hello` program that greets a name from `--name`, `HELLO_NAME`, or standard input:

```go title="main.go" hidden={1-11}
package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Name  conga.OptionalFlag[string] `default:"World" help:"Name of the person to greet"`
	Loud  conga.OptionalFlag[bool]   `help:"Print greeting in uppercase"`
	Stdin conga.OptionalFlag[bool]   `help:"Read the name from standard input"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	name := c.Name.Get()
	if c.Stdin.Get() {
		input, err := io.ReadAll(session.Stdin()) // [!code highlight]
		if err != nil {
			return err
		}
		name = strings.TrimSpace(string(input))
	}

	greeting := fmt.Sprintf("Hello, %s!", name)
	if c.Loud.Get() {
		greeting = strings.ToUpper(greeting)
	}

	fmt.Fprintln(session.Stdout(), greeting) // [!code highlight]

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("hello").
		EnvPrefix("HELLO")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

## Pass arguments and capture output

Build the app in the test with `Args`, `Stdout`, and `Stderr`, then run it with `app.RunContext(t.Context())`:

```go title="main_test.go" hidden={1-10}
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/conga-sh/conga"
)

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

```sh
$ go test -run TestHelloGreetsUser -v
=== RUN   TestHelloGreetsUser
=== PAUSE TestHelloGreetsUser
=== CONT  TestHelloGreetsUser
--- PASS: TestHelloGreetsUser (0.00s)
PASS
ok  	example/hello	0.005s
```

To feed standard input to `session.Stdin()`, pass an `io.Reader` to `Stdin`:

```go title="main_test.go"
func TestHelloReadsStdin(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[CLI]().
		Args([]string{"--stdin"}).
		Stdin(strings.NewReader("Alice\n")). // [!code highlight]
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

Arguments and streams belong to that `App` instance, so these tests can call `t.Parallel()`.

:::caution[Always call Args in tests]
Without `Args`, the app parses `os.Args`, which holds the flags `go test` passes to the test binary, and `RunContext` returns `unknown shorthand flag: 't' in -test.testlogfile`. Pass `Args(nil)` for a run with no command-line arguments.
:::

## Run table-driven tests

An `App` executes once; calling `Run` or `RunContext` a second time on the same instance panics with `conga: App already executed`. Call `conga.New` inside each test and each `t.Run` case:

```go title="main_test.go"
func TestGreetings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"default", nil, "Hello, World!\n"},
		{"custom", []string{"--name", "Bob"}, "Hello, Bob!\n"},
		{"loud", []string{"--name", "Bob", "--loud"}, "HELLO, BOB!\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var stdoutBuffer bytes.Buffer

			app := conga.New[CLI]().
				Args(tc.args).
				Stdout(&stdoutBuffer)

			if err := app.RunContext(t.Context()); err != nil {
				t.Fatalf("RunContext() unexpected error: %v", err)
			}

			if got := stdoutBuffer.String(); got != tc.want {
				t.Errorf("stdout = %q, want %q", got, tc.want)
			}
		})
	}
}
```

## Test environment variables and .env files

`t.Setenv` sets an environment variable for the test and restores the previous value when the test finishes. Because `t.Setenv` modifies the process environment, tests that call it cannot call `t.Parallel()`:

```go title="main_test.go"
func TestNameFromEnvironment(t *testing.T) {
	t.Setenv("HELLO_NAME", "Alice")

	var stdoutBuffer bytes.Buffer

	app := conga.New[CLI]().
		EnvPrefix("HELLO").
		Args(nil).
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

When an app loads a `.env` file with `Dotenv`, it sets each variable with `os.Setenv` by default, leaving those variables set for later tests in the same binary. Pass `SetenvFunc(t.Setenv)` so each variable loaded from `testdata/hello.env` (`HELLO_NAME=Alice`) is restored when the test ends:

```go title="main_test.go" {6-7}
func TestNameFromDotenvFile(t *testing.T) {
	var stdoutBuffer bytes.Buffer

	app := conga.New[CLI]().
		EnvPrefix("HELLO").
		Dotenv("testdata/hello.env").
		SetenvFunc(t.Setenv).
		Args(nil).
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

:::caution[Variables already set outrank the file]
`Dotenv` never overwrites a variable that is already in the process environment. If the shell that runs `go test` exports `HELLO_NAME=Eve`, this test reads `Eve` and fails with `stdout = "Hello, Eve!\n", want "Hello, Alice!\n"`.
:::

## Check errors and exit codes

`app.Exit(err)` formats the error to the app's `Stderr` and calls `os.Exit`:

- Exit code `0` when `err` is `nil`.
- Exit code `2` when `conga.IsUsageError(err)` is `true` (an unknown flag, a value of the wrong type, or a value that fails `range` or `enum`).
- Exit code `1` for any other error, such as an error returned from `Run`, a missing or malformed config or `.env` file, or an invalid struct tag. See [Errors](/reference/errors/) for every error category.

In a test, `os.Exit` would stop the test binary, so pass `ExitFunc` to record the code instead:

```go title="main_test.go"
func TestUnknownFlagExitsWithUsageError(t *testing.T) {
	t.Parallel()

	var stderrBuffer bytes.Buffer
	exitCode := -1

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--unknown-flag"}).
		Stdout(new(bytes.Buffer)).
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code }) // [!code highlight]

	err := app.RunContext(t.Context())
	if !conga.IsUsageError(err) {
		t.Fatalf("expected a usage error, got %v", err)
	}

	app.Exit(err)

	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}

	if !strings.Contains(stderrBuffer.String(), "unknown flag: --unknown-flag") {
		t.Errorf("stderr = %q, want the unknown flag message", stderrBuffer.String())
	}
}
```

After `app.Exit(err)`, `stderrBuffer` holds the error and usage block a user sees:

```text no-copy error={1}
hello: error: unknown flag: --unknown-flag

Usage:
  hello [flags]

Run 'hello --help' for more information.
```

To omit the usage block on errors, add `UsageOnError(conga.UsageSilence)` so `Stderr` receives only the error line, or `UsageOnError(conga.UsageHint)` to keep the error line and the `--help` hint.

## Test --help and other built-in flags

`--help`, `--version`, and `--why` write to `Stdout` and return `nil`:

```go title="main_test.go"
func TestHelpFlagPrintsUsage(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[CLI]().
		Name("hello").
		Args([]string{"--help"}).
		Stdout(&stdoutBuffer)

	err := app.RunContext(t.Context())
	if err != nil {
		t.Fatalf("RunContext() unexpected error: %v", err)
	}

	if !strings.Contains(stdoutBuffer.String(), "Usage:\n  hello [flags]") {
		t.Errorf("stdout = %q, want the usage block", stdoutBuffer.String())
	}
}
```

## Where to go next

- [About error reporting](/explanation/error-reporting/): error categories, exit codes, and usage output.
- [How to load .env files](/how-to/load-env-files/): how `Dotenv` resolves and loads environment files.
- [About the command lifecycle](/explanation/lifecycle/): context, signals, and the order of lifecycle hooks.
