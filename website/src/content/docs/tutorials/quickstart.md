---
title: Quickstart
description: Build hello, the traditional first program, and watch a command struct become flags, help, and version output.
---

In this tutorial, we will build `hello`, the traditional first command-line program. It prints a greeting, `--name` changes the person being greeted, and `--loud` switches the greeting to uppercase. CONGA requires Go 1.27+.

## The command we will build

The finished command accepts these flags:

| Flag | Default | Meaning |
|------|---------|---------|
| `-n`, `--name` | `World` | Name of the person to greet |
| `-l`, `--loud` | `false` | Print greeting in uppercase |
| `--help` | | Show help |
| `--version` | | Show version |

`--help` and `--version` are supplied by the engine; `--name` and `--loud` come from fields we declare. The usage line is `hello [flags]`.

## Create the command

Create `main.go` with the complete command:

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

The program has three parts: the command struct, the `Run` method, and the `main` function that wires the schema to the engine. We walk through each part below.

## The command struct

```go
type CLI struct {
	Name conga.OptionalFlag[string] `default:"World" help:"Name of the person to greet" short:"n"`
	Loud conga.OptionalFlag[bool]   `help:"Print greeting in uppercase" short:"l"`
}
```

A CONGA command is an ordinary Go struct whose exported fields describe the CLI. Three rules turn these two fields into the flags above:

- **The field name is the flag name.** Field names convert to kebab-case: `Name` becomes `--name`, `Loud` becomes `--loud`, and a multi-word field such as `MaxMemory` would become `--max-memory`. The struct name `CLI` never appears on the command line; the application name comes from `Name("hello")` in `main`.
- **The field type is a container.** `conga.OptionalFlag[string]` declares an optional flag whose value is a `string`, and `conga.OptionalFlag[bool]` declares a boolean flag. A container records the value the engine resolves for the field and the tier it came from. `Get()` reads that value in `Run` without a nil or zero check; [About reading values](/explanation/reading-values/) explains the guarantee behind the call.
- **The tags shape the flag.** `default:"World"` supplies the value when no other resolution tier provides one; `short:"n"` adds `-n` as a shorthand; `help:"Name of the person to greet"` is the line rendered on the help screen. The full vocabulary is in the [struct tag reference](/reference/struct-tags/).

The other container types — `RequiredFlag[T]`, `OptionalArg[T]`, `Counter`, and the rest — are catalogued in [About flags and args](/explanation/flags-and-args/).

## The Run method

```go
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())
	if c.Loud.Get() {
		greeting = strings.ToUpper(greeting)
	}

	fmt.Fprintln(session.Stdout(), greeting)

	return nil
}
```

Implementing `Run` makes `CLI` a `conga.Runner`. The engine parses the arguments, resolves every flag, and validates the result before calling this method, so the body reads values instead of tokens:

- `c.Name.Get()` returns the resolved name: the `default` value unless the command line, the environment, or a config file supplied a higher-precedence value. Required input is rejected before `Run` runs, and an unset optional container returns the zero value, so the body needs no presence check.
- `session.Stdout()` returns the configured output stream, `os.Stdout` by default. Commands and the engine both write through the session, so replacing the stream at the App captures every byte of output — the basis of [How to test a command](/how-to/test-a-command/).
- `ctx` carries cancellation and deadlines from the caller. `Run` derives a context that is cancelled by Ctrl+C (`SIGINT`) or `SIGTERM`; [About app execution](/explanation/execution/) covers both entry points.

The method signature is the `Runner` contract. The other lifecycle hooks are optional and run in a fixed order around it; [About the lifecycle](/explanation/lifecycle/) has the full order.

## The app wiring

```go
func main() {
	app := conga.New[CLI]().
		Name("hello").
		Description("Minimal single-command hello world CLI").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
```

- `conga.New[CLI]()` creates the `*conga.App[CLI]` builder and binds `CLI` as the root command. `New[T]()` lets the engine allocate the struct when execution starts; pass an instance with `conga.New(cli)` when the host needs a reference before or after execution. The builder methods are catalogued in [package conga](/reference/conga/).
- `Name("hello")` sets the application name used in help and version output, `Description(...)` is the overview at the top of the help screen, and `Version("1.0.0")` is the version string for `--version`.
- `app.Run()` runs the pipeline. `app.FatalIfError(...)` prints any failure to stderr and exits with status `2` for usage errors and `1` for runtime failures; a successful run exits with status `0`. [About error reporting](/explanation/error-reporting/) explains that boundary.

## Run the command

We run the default greeting, the long flags, and the short forms:

```sh
$ go run .
Hello, World!
$ go run . --name conga --loud
HELLO, CONGA!
$ go run . -n Bob -l
HELLO, BOB!
```

## See the generated help

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

`--version` renders the name and version from `Name(...)` and `Version(...)` together with the toolchain that built the binary:

```sh
$ go run . --version
hello has version 1.0.0 built with go1.27.1
```

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
- [Subcommands](/tutorials/subcommands/) — build `cloudctl`, a multi-command CLI with positional arguments and aliases.
- [Configuration](/tutorials/configuration/) — resolve values from environment variables and config files, and inspect provenance with `--why`.
- [About flags and args](/explanation/flags-and-args/) — counters, maps, slices, and constraints.
- [How to run the examples](/how-to/examples-cookbook/) — runnable programs, including the [minimal hello](https://github.com/conga-sh/conga/tree/main/_examples/01-minimal-hello) program this tutorial derives from.
- Optional modules: [decoders](/reference/decoders/), [predictors](/reference/predict/), [plugins](/reference/plugins/), and the [config watcher](/reference/watch/).
