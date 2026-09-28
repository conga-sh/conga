---
title: Quickstart
description: Build a complete CLI in one struct — flags, help, environment fallbacks, and subcommands generated from the schema.
---

In this tutorial, we will build `hello`, a small CLI that greets people by name and can also deploy a service. Along the way, we will see how a plain Go struct becomes flags, help output, environment fallbacks, and subcommand routing. Every step ends in a runnable program.

## 1. Create the module

```sh
mkdir hello && cd hello
go mod init example.com/hello
go get github.com/conga-sh/conga
```

## 2. Define the command

Write `main.go`. Every application is a plain Go struct — field types and struct tags form the CLI schema:

```go
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
		Version("1.0.0").
		EnableWhy()

	app.FatalIfError(app.Run())
}
```

Run it:

```sh
$ go run . --name conga --loud
HELLO, CONGA!
```

`Name` becomes `--name`/`-n` with a default of `World`; `Loud` becomes `--loud`/`-l`. `Run` is the command body: it reads resolved values with `Get()` and writes to the session's configured output.

## 3. Use the generated help

Every application gets `--help` and `--version` for free; `app.EnableWhy()` adds the `--why` flag, which prints where each value came from (CLI, env, config, or default):

```sh
$ go run . --help
$ go run . --version
hello has version 1.0.0 built with go1.27.1
$ go run . --why
```

The version line appends the commit and build timestamp when the binary carries them.

The help screen is derived from the struct, and typos get suggestions: `go run . --nmae x` answers `unknown flag --nmae (did you mean --name?)`.

## 4. Add an environment fallback

Give the app an environment namespace so `--name` falls back to `$HELLO_NAME`:

```go
app := conga.New[CLI]().
	Name("hello").
	EnvPrefix("HELLO").
	Version("1.0.0")
```

```sh
$ HELLO_NAME=conga go run .
Hello, conga!
```

The resolution ladder — CLI > environment > config file > default — is explained in [About value precedence](../explanation/value-precedence/).

## 5. Add a subcommand

Add a command field to the root struct and a command struct with its own `Run` method:

```go
type CLI struct {
	Name   conga.OptionalFlag[string]   `default:"World" help:"Name of the person to greet" short:"n"`
	Loud   conga.OptionalFlag[bool]     `help:"Print greeting in uppercase" short:"l"`
	Deploy conga.Command[DeployCommand] `name:"deploy" summary:"Deploy a service"`
}

type DeployCommand struct {
	Service  conga.RequiredArg[string] `help:"Service name"`
	Replicas conga.OptionalFlag[int]   `default:"3" help:"Number of replicas"`
}

func (d *DeployCommand) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying %s (replicas: %d)\n", d.Service.Get(), d.Replicas.Get())

	return nil
}
```

```sh
$ go run . deploy web-api --replicas 5
deploying web-api (replicas: 5)
```

## 6. The complete program

```go
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Name   conga.OptionalFlag[string]   `default:"World" help:"Name of the person to greet" short:"n"`
	Loud   conga.OptionalFlag[bool]     `help:"Print greeting in uppercase" short:"l"`
	Deploy conga.Command[DeployCommand] `name:"deploy" summary:"Deploy a service"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	greeting := fmt.Sprintf("Hello, %s!", c.Name.Get())
	if c.Loud.Get() {
		greeting = strings.ToUpper(greeting)
	}

	fmt.Fprintln(session.Stdout(), greeting)

	return nil
}

type DeployCommand struct {
	Service  conga.RequiredArg[string] `help:"Service name"`
	Replicas conga.OptionalFlag[int]   `default:"3" help:"Number of replicas"`
}

func (d *DeployCommand) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying %s (replicas: %d)\n", d.Service.Get(), d.Replicas.Get())

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("hello").
		Description("Minimal single-command hello world CLI").
		EnvPrefix("HELLO").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
```

## Where to go next

- [About flags & args](../explanation/flags-and-args/) — the full declarative surface: counters, maps, slices, constraints.
- [How to manage config files](../how-to/manage-config-files/) — add JSON/TOML/YAML/HCL configuration.
- [How to enable shell completion](../how-to/enable-shell-completion/) — completion for all six shells.
- [Examples Cookbook](cookbook/) — 29 runnable programs to copy from, including the [minimal hello](https://github.com/conga-sh/conga/tree/main/_examples/01-minimal-hello) this tutorial is based on.
