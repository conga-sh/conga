---
title: Quickstart
description: Build hello, a single-command CLI, and see a Go struct become flags, help, and version output.
---

This tutorial builds `hello`, a single-command CLI. It covers the command struct, the flag tags that shape its schema, and the generated help and version output.

## Set up the module

CONGA requires Go 1.27+. The `go get` command below installs the core module; format decoders, predictors, plugins, and the config watcher live in separate modules and are linked only when imported and registered.

```sh
mkdir hello && cd hello
go mod init example.com/hello
go get github.com/conga-sh/conga
```

## Define the command

The root command is a Go struct: its exported fields define the CLI schema, and its `Run` method executes the command. Create `main.go`:

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

CONGA builds the command schema from the exported fields and their tags. Field names convert to kebab-case (`MaxMemory` becomes `--max-memory`), so `Name` and `Loud` become `--name` and `--loud`:

- `default:"World"` supplies the value when no other tier sets one.
- `short:"n"` adds `-n` as a shorthand for `--name`.
- `help:"..."` is the description in the help screen.

`Run` executes the command after the engine has parsed the arguments and resolved every value; it reads those values through `Get()`. Output goes to `session.Stdout()`, which is `os.Stdout` by default and replaceable with `app.Stdout(w)`, so tests and embedders capture output by replacing the stream. `ctx` propagates cancellation from the caller; `session` exposes the configured streams, `Path()`, `Args()`, and the config-file accessors. See [About the lifecycle](../explanation/lifecycle/) for the full hook order.

`app.FatalIfError(app.Run())` prints any failure to stderr and exits with status 2 for usage errors and 1 for runtime failures.

## Run the command

Run without flags, then with both flags:

```sh
$ go run .
Hello, World!
$ go run . --name conga --loud
HELLO, CONGA!
```

## See the generated help

The help screen is generated from the command schema:

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

`--version` renders the name and version supplied by the `Name(...)` and `Version(...)` calls:

```sh
$ go run . --version
hello has version 1.0.0 built with go1.27.1
```

An unknown flag reports the closest known name:

```sh
$ go run . --nmae x
hello: error: unknown flag: --nmae. Did you mean "name"?

Usage:
  hello [flags]

Run 'hello --help' for more information.
```

## Where to go next

- [Subcommands](subcommands/) — build `cloudctl`, a multi-command CLI with positional arguments and aliases.
- [Configuration](configuration/) — resolve values from environment variables and config files, and inspect provenance with `--why`.
- [About flags & args](../explanation/flags-and-args/) — counters, maps, slices, constraints, and reading values.
- [Examples Cookbook](cookbook/) — runnable programs, including the [minimal hello](https://github.com/conga-sh/conga/tree/main/_examples/01-minimal-hello) program this tutorial derives from.
- Optional modules: [decoders](../reference/decoders/), [predictors](../reference/predict/), [plugins](../reference/plugins/), and the [config watcher](../reference/watch/).
