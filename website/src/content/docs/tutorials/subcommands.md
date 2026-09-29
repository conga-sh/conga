---
title: Subcommands
description: Build cloudctl, a multi-command CLI — route deploy and list, add positional arguments and aliases, and inspect the generated command tree.
---

This tutorial builds `cloudctl`, a CLI with two subcommands: `deploy` takes a service name and an optional replica count, and `list` prints the deployed services. It covers `conga.Command[T]` fields, positional arguments, and the generated command tree.

## Set up the module

```sh
mkdir cloudctl && cd cloudctl
go mod init example.com/cloudctl
go get github.com/conga-sh/conga
```

## Declare the first subcommand

A root struct field of type `conga.Command[T]` mounts a subcommand handled by `T`. Create `main.go`:

```go title="main.go" {15}
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
	Service  conga.RequiredArg[string] `help:"Service name"`
	Replicas conga.OptionalFlag[int]   `default:"3" help:"Number of replicas" short:"r"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying %s (replicas: %d)\n", d.Service.Get(), d.Replicas.Get())

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

The `Deploy` field mounts a subcommand: `conga.Command[DeployCmd]` routes the `deploy` argument to `DeployCmd`, the same way the root struct describes the root command. Field names convert to kebab-case, so `Deploy` becomes `deploy`, and `summary:"..."` is the one-line description in the parent's command list.

`DeployCmd` is an ordinary CONGA struct, with the same rules as the root:

- `Service` is a `RequiredArg[string]`, a positional argument. It has no flag name — it is matched by position — and CONGA refuses to run the command when it is missing.
- `Replicas` is a regular flag with a default and a short form, using the same tag vocabulary as the [Quickstart](../quickstart/) flags.

Run it with and without the flag, then without the argument:

```sh
$ go run . deploy web-api --replicas 5
deploying web-api (replicas: 5)
$ go run . deploy web-api
deploying web-api (replicas: 3)
$ go run . deploy
cloudctl: error: missing required positional argument <service>

Usage:
  cloudctl deploy <service> [flags]

Run 'cloudctl deploy --help' for more information.
```

The argument is validated before `Run` executes, so the body can read it directly with `Get()`.

## Inspect the command tree

`--help` renders the tree from the root:

```sh
$ go run . --help
Deploy and inspect services

Usage:
  cloudctl <command>

Commands:
  deploy  Deploy a service

Flags:
      --help     Show help
      --version  Show version
```

Each command has its own help screen, derived from its own struct:

```sh
$ go run . deploy --help
Deploy a service

Usage:
  cloudctl deploy <service> [flags]

Arguments:
  <service>  Service name

Flags:
  -r, --replicas=3  Number of replicas
      --help        Show help
```

The `Arguments:` section lists positional arguments separately from flags. The root `CLI` struct defines no `Run` method, so `cloudctl` with no subcommand reports that a command is required.

## Add a second command

`aliases` adds alternate names for a subcommand. Add the `List` field:

```go title="main.go" ins={3}
type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a service"`
	List   conga.Command[ListCmd]   `aliases:"ls" summary:"List deployed services"`
}
```

Then define the command struct and its `Run` method:

```go title="main.go"
type ListCmd struct {
	All conga.OptionalFlag[bool] `help:"Include stopped services" short:"a"`
}

func (l *ListCmd) Run(ctx context.Context, session *conga.Session) error {
	services := []string{"web-api", "worker"}
	if l.All.Get() {
		services = append(services, "cron")
	}

	for _, service := range services {
		fmt.Fprintln(session.Stdout(), service)
	}

	return nil
}
```

`aliases:"ls"` adds a second name for the same command; `--all` is a regular flag:

```sh
$ go run . list
web-api
worker
$ go run . ls -a
web-api
worker
cron
```

## The complete program

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a service"`
	List   conga.Command[ListCmd]   `aliases:"ls" summary:"List deployed services"`
}

type DeployCmd struct {
	Service  conga.RequiredArg[string] `help:"Service name"`
	Replicas conga.OptionalFlag[int]   `default:"3" help:"Number of replicas" short:"r"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying %s (replicas: %d)\n", d.Service.Get(), d.Replicas.Get())

	return nil
}

type ListCmd struct {
	All conga.OptionalFlag[bool] `help:"Include stopped services" short:"a"`
}

func (l *ListCmd) Run(ctx context.Context, session *conga.Session) error {
	services := []string{"web-api", "worker"}
	if l.All.Get() {
		services = append(services, "cron")
	}

	for _, service := range services {
		fmt.Fprintln(session.Stdout(), service)
	}

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

## Where to go next

- [Configuration](configuration/) — give flags environment and config-file fallbacks, and trace them with `--why`.
- [About subcommands](../explanation/subcommands/) — nesting, command groups, hidden commands, and global flags.
- [Struct tag reference](../reference/struct-tags/) — every command tag: `aliases`, `group`, `hidden`, `deprecated`, `envprefix`.
- [Examples Cookbook](cookbook/) — example 08 for aliases and groups, example 09 for deep hierarchies and `session.Root[T]()`.
