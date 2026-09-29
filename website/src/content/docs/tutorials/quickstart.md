---
title: Quickstart
description: Build hello from a working flag-driven command into a CLI with switches, metadata, generated help, and version output.
---

In this tutorial, we will build `hello` step by step. We start with a working command that accepts a `--name` flag with a default, add a `--loud` boolean switch, configure application metadata, and inspect the generated help and version output. CONGA requires Go 1.27+.

## The command we will build

The finished command accepts `--name` and `--loud` flags:

```sh
hello --name conga --loud
```

```
HELLO, CONGA!
```

We will build it in three stages:
1. Start with `--name` and default greeting output.
2. Add `--loud` to toggle uppercase output.
3. Configure application metadata and inspect `--help` and `--version`.

| Flag | Default | Meaning |
|------|---------|---------|
| `-n`, `--name` | `World` | Name of the person to greet |
| `-l`, `--loud` | `false` | Print greeting in uppercase |

## A first program

Create the project directory and module:

```sh
mkdir hello && cd hello
go mod init hello
go get github.com/conga-sh/conga
```

Then create `main.go`:

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Name conga.OptionalFlag[string] `default:"World" help:"Name of the person to greet" short:"n"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())

	fmt.Fprintln(session.Stdout(), greeting)

	return nil
}

func main() {
	app := conga.New[CLI]()

	app.FatalIfError(app.Run())
}
```

Run the program with the default value:

```sh
go run .
```

```
Hello, World!
```

Override the flag from the command line:

```sh
go run . --name conga
```

```
Hello, conga!
```

`conga.New[CLI]()` creates the application builder and registers `CLI` as the root command. `Run` executes the command body; implementing it makes `CLI` a `conga.Runner`.

`Name` becomes `--name`: field names convert to kebab-case. `conga.OptionalFlag[string]` is a typed container that stores the resolved flag value. Struct tags shape the flag: `default:"World"` supplies the fallback value, `short:"n"` adds `-n`, and `help:"..."` provides the flag description for the help screen.

In `Run`, `c.Name.Get()` returns the resolved value. Resolution finishes before `Run` starts, so the value is ready to use without presence checks.

Commands write to `session.Stdout()` rather than `os.Stdout`. In the terminal this prints normally, but routing writes through the session allows automated tests and host programs to capture output directly.

`app.Run()` executes the command line pipeline: it parses arguments, resolves flag values, and calls `Run`. `app.FatalIfError` prints any error to stderr and exits with a non-zero status code.

:::tip
The engine supplies `--help` and `--version` from this first step. Run `go run . --help` at any point to render the schema as it stands.
:::

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

	fmt.Fprintln(session.Stdout(), greeting)

	return nil
}
```

Both the long and the short form work:

```sh
go run . --name conga --loud
```

```
HELLO, CONGA!
```

```sh
go run . -n Bob -l
```

```
HELLO, BOB!
```

## Name, description, and version

We can describe the application by chaining builder calls on `conga.New[CLI]()`:

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

These builder calls set application metadata:

- `Name("hello")` sets the command name shown in usage lines and error messages.
- `Description("...")` provides the overview line at the top of the help screen.
- `Version("1.0.0")` sets the version string displayed by `--version`.

## Generated help

The help screen is generated from the struct and the builder calls:

```sh
go run . --help
```

```
Minimal single-command hello world CLI

Usage:
  hello [flags]

Flags:
  -n, --name="World"  Name of the person to greet
  -l, --loud          Print greeting in uppercase
      --help          Show help
      --version       Show version
```

The first line comes from `Description(...)`, the usage line combines the binary name with the schema, and each flag line combines a field name with its tags.

## Version output

`--version` renders the application name, the version set by `Version(...)`, and the toolchain that built the binary:

```sh
go run . --version
```

```
hello has version 1.0.0 built with go1.27.1
```

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

**Next tutorials**
- [Subcommands](/tutorials/subcommands/): build a multi-command CLI with positional arguments and command aliases.
- [Configuration](/tutorials/configuration/): load values from environment variables and configuration files.

**How-to guides**
- [How to test a command](/how-to/test-a-command/): capture streams, mock arguments, and assert exit codes.
- [How to keep a reference to the root struct](/how-to/keep-root-reference/): pass pre-initialized structs using `conga.New(cli)`.
- [How to run the examples](/how-to/examples-cookbook/): explore runnable sample applications, including the [minimal hello](https://github.com/conga-sh/conga/tree/main/_examples/01-minimal-hello) program.

**Concepts and reference**
- [About reading values](/explanation/reading-values/): container reader guarantees.
- [About app execution](/explanation/execution/): context propagation, standard streams, and lifecycle execution.
- [About error reporting](/explanation/error-reporting/): usage errors, runtime errors, and `FatalIfError`.
- [About flags and args](/explanation/flags-and-args/): counters, slices, maps, and positional arguments.
- [Package conga](/reference/conga/): core API reference.
