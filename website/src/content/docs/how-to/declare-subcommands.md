---
title: How to declare subcommands
description: Add subcommands with Command[T], group, alias, hide, and deprecate them, read root flags from a subcommand, and make one subcommand the default.
---

This guide builds a `cloudctl` program with `deploy`, `list`, `debug`, and `push` subcommands. [Subcommands (`Command[T]`)](/reference/struct-tags/#subcommands-commandt) lists every tag a subcommand field takes. If you're new to CONGA, [Build a multi-command CLI](/tutorials/multi-command-cli/) builds a program with subcommands step by step.

## Add a subcommand

A subcommand is a field of its parent command's struct, and its type is the container `conga.Command[...]`. The type in the brackets is the subcommand's own struct, which declares the subcommand's flags and arguments; a pointer to it implements `conga.Runner`:

```go
type Runner interface {
	Run(ctx context.Context, session *Session) error
}
```

The field name in kebab case becomes the command name, so `Deploy` is `deploy`, and `summary` is its line in the parent's help:

```go title="main.go" hidden={1-8}
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Verbose conga.OptionalFlag[bool] `help:"Print each step" short:"v"`

	Deploy conga.Command[DeployCmd] `summary:"Deploy a manifest"`
}

type DeployCmd struct {
	Replicas conga.OptionalFlag[int]   `default:"3" help:"Number of replicas" short:"r"`
	Manifest conga.RequiredArg[string] `help:"Manifest file"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying %s with %d replicas\n", d.Manifest.Get(), d.Replicas.Get())

	return nil
}

func main() {
	app := conga.New[CLI]().Name("cloudctl")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```sh
$ cloudctl --help
Usage:
  cloudctl <command> [flags]

Commands:
  deploy  Deploy a manifest

Flags:
  -v, --verbose  Print each step
      --help     Show help
      --version  Show version
```

```sh
$ cloudctl deploy web.yaml
deploying web.yaml with 3 replicas
```

`CLI` has no `Run` method, so `cloudctl` alone is a usage error:

```sh error={2}
$ cloudctl
cloudctl: error: subcommand required for "cloudctl"

Usage:
  cloudctl <command> [flags]

Commands:
  deploy  Deploy a manifest

Flags:
  -v, --verbose  Print each step
      --help     Show help
      --version  Show version
```

To run code when the user names no subcommand, implement `Runner` on the parent too. Its `Run` handles `cloudctl` alone, and `cloudctl deploy web.yaml` still runs `deploy`.

A command declares subcommands or positional arguments, not both; [How to accept positional arguments](/how-to/accept-positional-arguments/) covers positional arguments.

## Group, alias, hide, and deprecate subcommands

Tags on the `conga.Command[...]` field control how a subcommand is reached and listed:

```go title="main.go" ins={5-7}
type CLI struct {
	Verbose conga.OptionalFlag[bool] `help:"Print each step" short:"v"`

	Deploy conga.Command[DeployCmd] `summary:"Deploy a manifest"`
	List   conga.Command[ListCmd]   `aliases:"ls,ps" section:"Inspection" summary:"List running services"`
	Debug  conga.Command[DebugCmd]  `hidden:"true" summary:"Dump internal state"`
	Push   conga.Command[DeployCmd] `deprecated:"use deploy" summary:"Deploy a manifest (old name)"`
}
```

`section` puts `list` under its own heading, `hidden` leaves `debug` out of help and completion, and `deprecated` labels `push`:

```sh {7,9-10}
$ cloudctl --help
Usage:
  cloudctl <command> [flags]

Commands:
  deploy  Deploy a manifest
  push    Deploy a manifest (old name) [deprecated: use deploy]

Inspection:
  list  List running services

Flags:
  -v, --verbose  Print each step
      --help     Show help
      --version  Show version
```

`aliases` routes `ls` and `ps` to `list`, and `cloudctl debug` still runs. A deprecated command still runs, after a warning on stderr, and completion stops offering it:

```sh
$ cloudctl ls
api  worker
```

```sh
$ cloudctl push web.yaml
WARNING: command 'cloudctl push' is deprecated: use deploy
deploying web.yaml with 3 replicas
```

Shell completion offers the aliases; `--help` lists each command once, by name. `name:"..."` replaces the kebab-case name. [How to deprecate flags and commands](/how-to/deprecate-flags-and-commands/) also deprecates flags and whole applications.

## Read root flags from a subcommand

Flags on a parent apply to all its descendants, and the user can give them before or after the subcommand name. A subcommand reads them through `session.Root[CLI]()`, which returns the root struct:

```go title="main.go" ins={2-5}
func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[CLI]()
	if root.Verbose.Get() {
		fmt.Fprintf(session.Stdout(), "running %s\n", session.Path())
	}

	fmt.Fprintf(session.Stdout(), "deploying %s with %d replicas\n", d.Manifest.Get(), d.Replicas.Get())

	return nil
}
```

```sh
$ cloudctl -v deploy web.yaml
running cloudctl deploy
deploying web.yaml with 3 replicas
```

The subcommand's help lists the parent's flags under `Global Flags:`. A descendant can't redeclare a parent's flag name, alias, or short letter; sibling subcommands can share names. `session.Path()` and the other accessors are in [package conga](/reference/conga/#session-and-hierarchy-accessors).

## Make one subcommand the default

Tag a subcommand `implicit:"true"` to run it when the user names none:

```go title="main.go" {4}
type CLI struct {
	Verbose conga.OptionalFlag[bool] `help:"Print each step" short:"v"`

	Deploy conga.Command[DeployCmd] `implicit:"true" summary:"Deploy a manifest"`
	// ...
}
```

`cloudctl web.yaml` now deploys, and so does `cloudctl -r 5 web.yaml`, since `-r` belongs to `deploy`. A subcommand named on the command line still wins:

```sh
$ cloudctl web.yaml
deploying web.yaml with 3 replicas
```

```sh
$ cloudctl list
api  worker
```

Help marks the default and shows the command as optional:

```sh {3,6}
$ cloudctl --help
Usage:
  cloudctl [<command>] [flags]

Commands:
  deploy  Deploy a manifest (default)
  push    Deploy a manifest (old name) [deprecated: use deploy]

Inspection:
  list  List running services

Flags:
  -v, --verbose  Print each step
      --help     Show help
      --version  Show version
```

An operand that starts with `-` goes after `--`: `cloudctl -- -web.yaml` deploys `-web.yaml`. [Implicit subcommands](/reference/struct-tags/#implicit-subcommands) gives the full routing rules, including a parent that also implements `Run`.

## Know what users see

| What happens | When | Result |
|---|---|---|
| The user mistypes a subcommand (`cloudctl lst`) | Run | `unknown command: lst. Did you mean "list"?`, exit 2 |
| The user runs a subcommand whose type has no `Run` method | Run | `runner not implemented: command "list" does not implement conga.Runner`, exit 1 |
| A subcommand declares a flag its parent already declares | Startup | `schema error: duplicate flag: --verbose (already declared by ancestor command "cloudctl")`, exit 1 |
| Two subcommands of one parent are tagged `implicit:"true"` | Startup | `schema error: only one implicit command is allowed per command`, exit 1 |
| With `deploy` implicit, a flag only `list` declares comes before `list` (`cloudctl --format json list`) | Run | `unknown flag: --format`, exit 2; the line routes to `deploy` |
| With `deploy` implicit, the user runs `cloudctl` alone | Run | `missing required positional argument <manifest>`, exit 2 |

A missing `Run` passes startup and fails only when a user invokes that subcommand. [`conga-lint`](/reference/conga-lint/) reports it before you build.

## Where to go next

- [Subcommands (`Command[T]`)](/reference/struct-tags/#subcommands-commandt): every subcommand tag and the implicit routing rules.
- [About subcommands and hierarchy](/explanation/subcommands/): why flags are reserved down a branch and how routing works.
- [How to accept positional arguments](/how-to/accept-positional-arguments/): give a leaf command its arguments.
- [How to read resolved values after Run](/how-to/read-values-after-run/): read resolved values after `Run` returns.
