---
title: How to declare subcommands
description: Route subcommands with Command[T], assign command aliases, group commands, and reach root state.
---

This guide demonstrates how to declare nested subcommands with `Command[T]`, assign command aliases and visual groups, and reach root command state from child commands.

## Mount a subcommand

Declare a field of type `conga.Command[T]` on the enclosing command struct, where `T` is a struct implementing `Run(ctx context.Context, session *conga.Session) error`:

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a service"`
}

type DeployCmd struct {
	Replicas conga.OptionalFlag[int] `default:"3" help:"Number of replicas" short:"r"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying service (replicas: %d)\n", d.Replicas.Get())

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("cloudctl").
		Description("Deploy and inspect services").
		Version("1.0.0")

	app.FatalIfError(app.Run())
}
```

Field names convert to kebab-case, so `Deploy` becomes the `deploy` subcommand. The `summary` tag provides the description displayed in parent command listings.

Invoke the subcommand with flags:

```sh
$ go run . deploy --replicas 5
deploying service (replicas: 5)
```

Omitted flags receive their declared default value:

```sh
$ go run . deploy
deploying service (replicas: 3)
```

To accept positional arguments alongside flags, see [How to accept positional arguments](/how-to/accept-positional-arguments/).

## Add command aliases and groups

Assign command aliases and visual groups with struct tags on the `conga.Command[T]` field:

```go title="main.go" ins={5-6,18-24}
type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a service"`
	List   conga.Command[ListCmd]   `aliases:"ls,ps" group:"Inspection" summary:"List deployed services"`
}

type ListCmd struct {
	All conga.OptionalFlag[bool] `help:"Include inactive services" short:"a"`
}

func (l *ListCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "listing services")

	return nil
}
```

The alias invokes the command identically:

```sh
$ go run . ls
listing services
```

`--help` renders grouped subcommands under the assigned header:

```sh
$ go run . --help
Deploy and inspect services

Usage:
  cloudctl <command>

Commands:
  deploy  Deploy a service

Inspection Commands:
  list    List deployed services (aliases: ls, ps)

Flags:
  --help     Show help
  --version  Show version
```

## Read root state from a subcommand

Subcommands read global flags and metadata declared on the root command through `session.Root[T]()`:

```go
func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[CLI]()

	fmt.Fprintf(session.Stdout(), "command path: %s\n", session.Path())
	fmt.Fprintf(session.Stdout(), "leaf name: %s\n", session.Name())

	return nil
}
```

Flag shadowing across ancestor-descendant chains is prohibited: a flag name declared on the root command cannot be redeclared on any child subcommand.

## Where to go next

- [How to accept positional arguments](/how-to/accept-positional-arguments/) — define mandatory, optional, and trailing positional arguments.
- [About subcommands and hierarchy](/explanation/subcommands/) — hierarchy architecture and flag-shadowing rules.
- [How to keep a reference to the root struct](/how-to/keep-root-reference/) — construct and bind caller-owned root instances.
