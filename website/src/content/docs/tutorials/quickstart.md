---
title: Quickstart
description: Build a working command-line tool with flags, metadata, generated help, and version output using CONGA.
---

In this tutorial, we will build `hello`, a command-line tool that greets a user by name. We will write a command struct with an optional flag, configure application metadata, run the program with defaults and custom flags, and inspect the generated help and version output.

## The command we will build

The command accepts an optional `--name` flag:

```sh
hello --name conga
```

```text no-copy
Hello, conga!
```

When invoked without flags, it prints a default greeting:

```sh
hello
```

```text no-copy
Hello, World!
```

## Set up the project

Create a directory for the project and initialize a Go module:

```sh
mkdir hello && cd hello
go mod init hello
go get github.com/conga-sh/conga@latest
```

## Write the program

Create `main.go`:

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
	fmt.Fprintf(session.Stdout(), "Hello, %s!\n", c.Name.Get())

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("hello").
		Description("A greeting command").
		Version("1.0.0")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

Notice the three parts of the program:
- `CLI` defines the command flags. Field names convert to kebab-case, so `Name` becomes `--name`.
- `Run` contains the command logic. `c.Name.Get()` reads the resolved flag value, and writes go to `session.Stdout()`.
- `main` configures the application name, description, and version, then executes the command.

## Run the program

Run the program with no arguments to observe the default output:

```sh
go run .
```

```text no-copy
Hello, World!
```

Because `--name` was omitted, CONGA used the `default:"World"` from the struct tag.

Pass `--name` to override the default value:

```sh
go run . --name conga
```

```text no-copy
Hello, conga!
```

The short flag `-n` produces the same result:

```sh
go run . -n Bob
```

```text no-copy
Hello, Bob!
```

## Inspect generated help

CONGA generates `--help` automatically from the struct tags and application metadata:

```sh
go run . --help
```

```text no-copy
A greeting command

Usage:
  hello [flags...]

Flags:
  -n, --name="World"  Name of the person to greet
      --help          Show help
      --version       Show version
```

The overview line comes from `Description(...)`, the usage line uses the application name from `Name(...)`, and each flag line reflects the field name, default value, and help text from the struct tags.

## Inspect version output

The `--version` flag renders the application name, the version string set by `Version(...)`, and the Go toolchain that built the binary:

```sh
go run . --version
```

```text no-copy
hello has version 1.0.0 built with go1.27.1
```

## Where to go next

**How-to guides**
- [How to declare subcommands](/how-to/declare-subcommands/): route subcommands with `Command[T]`, command aliases, and groups.
- [How to accept positional arguments](/how-to/accept-positional-arguments/): define required, optional, and trailing positional arguments.
- [How to bind environment variables](/how-to/bind-environment-variables/): map environment variables to flags with prefixes and struct tags.
- [How to manage config files](/how-to/manage-config-files/): register decoders, declare search paths, and cascade tiers.
- [How to test a command](/how-to/test-a-command/): capture streams, mock arguments, and assert exit codes.
- [How to retain the root struct](/how-to/keep-root-reference/): inspect resolved container values after execution with `conga.New(cli)`.
- [How to run the examples](/how-to/examples-cookbook/): explore runnable sample applications, including the [minimal hello](https://github.com/conga-sh/conga/tree/main/_examples/01-minimal-hello) program.

**Concepts and reference**
- [About flags and args](/explanation/flags-and-args/): container reader guarantees, trade-offs, and positional arguments.
- [About the command lifecycle](/explanation/lifecycle/): execution stages, hook order, and teardown guarantees.
- [About error reporting](/explanation/error-reporting/): usage errors, runtime errors, and `Exit`.
- [Package conga](/reference/conga/): core API reference.
