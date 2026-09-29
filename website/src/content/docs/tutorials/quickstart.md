---
title: Quickstart
description: Build hello from a minimal Hello, World! program into a command with flags, session output, generated help, and version output.
---

In this tutorial, we will build `hello` step by step. We start with a minimal program that prints `Hello, World!`, add `--name` and `--loud` flags, move the output through the session, and finish with the generated help and version output. CONGA requires Go 1.27+.

## The command we will build

The finished command adds these flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `-n`, `--name` | `World` | Name of the person to greet |
| `-l`, `--loud` | `false` | Print greeting in uppercase |

The engine also supplies two flags on every root command; they are not declared in the struct:

| Flag | Meaning |
|------|---------|
| `--help` | Show help |
| `--version` | Show version |

See [Built-ins and deprecation](/reference/builtins/) for the full catalogue. The usage line of the finished command is `hello [flags]`.

## A first program

Create the project directory and module:

```sh
$ mkdir hello && cd hello
$ go mod init hello
$ go get github.com/conga-sh/conga
```

Then create `main.go`:

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct{}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Println("Hello, World!")

	return nil
}

func main() {
	app := conga.New[CLI]()

	app.FatalIfError(app.Run())
}
```

The struct is empty and `Run` prints with `fmt.Println`: a command with no input needs no fields. We run it:

```sh
$ go run .
Hello, World!
```

`conga.New[CLI]()` creates the `*conga.App[CLI]` builder and binds `CLI` as the root command. `New[T]()` lets the engine allocate the struct when execution starts; pass your own instance with `conga.New(cli)` when the host needs a reference before or after execution — see [How to keep a reference to the root struct](/how-to/keep-root-reference/).

`Run` is the method the engine calls to execute the command; implementing it makes `CLI` a `conga.Runner`. The other lifecycle hooks are optional and run in a fixed order around it; [About the lifecycle](/explanation/lifecycle/) has the full order. Its two parameters are the execution context and the session handle:

- `ctx` is the context the pipeline runs under. `app.Run()` derives one that is cancelled by Ctrl+C (`SIGINT`) or `SIGTERM`; `app.RunContext(ctx)` uses a context supplied by the developer, including its deadlines and cancellation. [About app execution](/explanation/execution/) covers both entry points.
- `session` is the handle to the running invocation: the configured standard streams, the command path and arguments, and the configuration accessors. The first version reads no input, so it uses neither parameter.

`app.Run()` is the whole pipeline: it derives the schema from the struct, parses the arguments (`os.Args[1:]` by default), resolves and validates every value, and calls `Run`. `app.FatalIfError(...)` is the process boundary: it prints a failure to stderr and exits with status `2` for usage errors and `1` for runtime failures; a successful run exits with status `0`. [About error reporting](/explanation/error-reporting/) explains that contract.

The root struct's type name is not important: `CLI` is only the Go type name and never appears on the command line. `--help` and `--version` display the invoked binary name, which is `hello` here because the directory is named `hello`.

:::tip
The engine supplies `--help` and `--version` from this first step. Run `go run . --help` at any point to render the schema as it stands.
:::

## Add the --name flag

A CONGA command is a Go struct whose exported fields describe the CLI. We add the first field:

```go title="main.go" ins={2}
type CLI struct {
	Name conga.OptionalFlag[string] `default:"World" help:"Name of the person to greet" short:"n"`
}
```

`Name` becomes `--name`: field names convert to kebab-case, so `MaxMemory` would become `--max-memory`, and the `name:"..."` tag overrides the derived name. `conga.OptionalFlag[string]` is a container, a typed field that holds the value the engine resolves for it. The tags shape the flag: `default:"World"` supplies the value when no higher-precedence source provides one, `short:"n"` adds `-n`, and `help:"..."` is the line rendered on the help screen.

In `Run`, `c.Name.Get()` returns the resolved value. Resolution is complete before `Run` is called, and `Get()` is unconditional: a missing required flag fails with a usage error first, and an unset optional flag returns the zero value, so the body needs no presence check. [About reading values](/explanation/reading-values/) has the full reader surface.

```go title="main.go" del={2} ins={3,5}
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Println("Hello, World!")
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())

	fmt.Println(greeting)

	return nil
}
```

The first run shows the `default` value; the second overrides it from the command line:

```sh
$ go run .
Hello, World!
$ go run . --name conga
Hello, conga!
```

## Add the --loud flag

A `bool` container declares a switch. We add the field:

```go title="main.go" ins={3}
type CLI struct {
	Name conga.OptionalFlag[string] `default:"World" help:"Name of the person to greet" short:"n"`
	Loud conga.OptionalFlag[bool]   `help:"Print greeting in uppercase" short:"l"`
}
```

The uppercase branch needs `strings`:

```go title="main.go" ins={4}
import (
	"context"
	"fmt"
	"strings"

	"github.com/conga-sh/conga"
)
```

```go title="main.go" ins={3-5}
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())
	if c.Loud.Get() {
		greeting = strings.ToUpper(greeting)
	}

	fmt.Println(greeting)

	return nil
}
```

Both the long and the short form work:

```sh
$ go run . --name conga --loud
HELLO, CONGA!
$ go run . -n Bob -l
HELLO, BOB!
```

The other container types — `RequiredFlag[T]`, `OptionalArg[T]`, `Counter`, and the rest — are catalogued in [About flags and args](/explanation/flags-and-args/).

## Write through the session

So far the command prints with `fmt.Println`, which writes to the process stdout. The `session` parameter exposes the output stream the engine is configured with: `session.Stdout()` returns `os.Stdout` by default, and a host or test can replace it with `app.Stdout(...)` to capture everything the command writes. We switch the final write to the session stream:

```go title="main.go" del={7} ins={8}
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())
	if c.Loud.Get() {
		greeting = strings.ToUpper(greeting)
	}

	fmt.Println(greeting)
	fmt.Fprintln(session.Stdout(), greeting)

	return nil
}
```

Writing to `os.Stdout` directly still works; the session stream is the convenience that makes output capturable in [How to test a command](/how-to/test-a-command/). The output is unchanged:

```sh
$ go run . --name conga --loud
HELLO, CONGA!
```

## Name, description, and version

`main` has built the app with no metadata so far. Three builder calls describe the application, and all of them are optional:

```go title="main.go" del={2} ins={3-6}
func main() {
	app := conga.New[CLI]()
	app := conga.New[CLI]().
		Name("hello").
		Description("Minimal single-command hello world CLI").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
```

- `Name("hello")` sets the application name. `--help`, `--version`, and error output display the invoked binary name and fall back to this value when no binary name can be derived, such as in tests and embedded hosts.
- `Description("...")` is the overview line at the top of the help screen. Without it, the help screen starts at `Usage:`.
- `Version("1.0.0")` sets the string rendered by `--version`. Without it, the summary falls back to the module version (or `dev`) plus embedded build metadata — see [Version](/reference/builtins/#version).

## Generated help

The help screen is generated from the struct and the builder calls:

```sh
$ go run . --help
Minimal single-command hello world CLI

Usage:
  hello [flags]

Flags:
  -n, --name="World"  Name of the person to greet
  -l, --loud          Print greeting in uppercase
      --help          Show help
      --version       Show version
```

The first line comes from `Description(...)`, the usage line combines the binary name with the schema, and each flag line combines a field name with its tags: `short:"n"` produces the `-n` prefix, and `default:"World"` produces the quoted value.

## Version output

`--version` renders the application name, the version set by `Version(...)`, and the toolchain that built the binary:

```sh
$ go run . --version
hello has version 1.0.0 built with go1.27.1
```

## Mistyped flags

A mistyped flag reports the closest known name:

```sh
$ go run . --nmae x
hello: error: unknown flag: --nmae. Did you mean "name"?

Usage:
  hello [flags]

Run 'hello --help' for more information.
```

The error, the usage summary, and the hint are rendered by `FatalIfError`, and the command exits with status `2`.

## The complete program

```go title="main.go"
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Name conga.OptionalFlag[string] `default:"World" help:"Name of the person to greet" short:"n"`
	Loud conga.OptionalFlag[bool]   `help:"Print greeting in uppercase" short:"l"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())
	if c.Loud.Get() {
		greeting = strings.ToUpper(greeting)
	}

	fmt.Fprintln(session.Stdout(), greeting)

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("hello").
		Description("Minimal single-command hello world CLI").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
```

## Where to go next

- [About reading values](/explanation/reading-values/) — the container reader surface and why `Get()` needs no presence check.
- [About app execution](/explanation/execution/) — `Run` versus `RunContext`, context and signal handling, and the single-execution rule.
- [About error reporting](/explanation/error-reporting/) — usage versus runtime errors, and customising `FatalIfError`.
- [How to test a command](/how-to/test-a-command/) — inject streams and arguments, then assert output and exit codes.
- [How to keep a reference to the root struct](/how-to/keep-root-reference/) — reach the root struct before and after execution with `conga.New(cli)`.
- [Subcommands](/tutorials/subcommands/) — build `cloudctl`, a multi-command CLI with positional arguments and aliases.
- [Configuration](/tutorials/configuration/) — resolve values from environment variables and config files, and inspect provenance with `--why`.
- [About flags and args](/explanation/flags-and-args/) — counters, maps, slices, and constraints.
- [How to run the examples](/how-to/examples-cookbook/) — runnable programs, including the [minimal hello](https://github.com/conga-sh/conga/tree/main/_examples/01-minimal-hello) program this tutorial derives from.
- Optional modules: [decoders](/reference/decoders/), [predictors](/reference/predict/), [plugins](/reference/plugins/), and the [config watcher](/reference/watch/).
