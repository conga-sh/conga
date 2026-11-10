---
title: How to accept positional arguments
description: Declare required and optional positional arguments, collect a variable number into a slice, forward the words after -- with DashArgs, and pass everything through with RawArgs.
---

This guide builds a `mirror` command that copies directories to a target and forwards options to rsync. If you're new to CONGA, [Build a multi-command CLI](/tutorials/multi-command-cli/) builds a command with a positional argument step by step.

## Declare required and optional arguments

CONGA builds a command's positional arguments from the fields of its struct, as it does flags. The type of each argument field is a container: `conga.RequiredArg[...]` for a required argument, `conga.OptionalArg[...]` for an optional one, with the type of the argument's value in the brackets. CONGA binds the words on the command line to the fields in declaration order, and `GetOr` gives an optional argument's fallback:

```go title="main.go" hidden={1-8}
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type Mirror struct {
	Source conga.RequiredArg[string] `help:"Directory to copy"`
	Target conga.OptionalArg[string] `help:"Destination directory"`
}

func (m *Mirror) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "copying %s to %s\n", m.Source.Get(), m.Target.GetOr("backup/"))

	return nil
}

func main() {
	app := conga.New[Mirror]().Name("mirror")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```sh
$ mirror src/
copying src/ to backup/
```

```sh
$ mirror src/ /mnt/backup
copying src/ to /mnt/backup
```

The argument names in help come from the field names:

```sh
$ mirror --help
Usage:
  mirror <source> [<target>]

Arguments:
  <source>    Directory to copy
  [<target>]  Destination directory

Flags:
      --help     Show help
      --version  Show version
```

Required arguments come before optional ones ([Positional arguments](/reference/struct-tags/#positional-arguments-optionalargt-requiredargt)). Positional arguments bind from the command line only.

## Collect any number of arguments in a slice

Declare a slice as the last positional argument, and CONGA collects every remaining word into it. A `RequiredArg[[]T]` needs at least one word; an `OptionalArg[[]T]` can be empty:

```go title="main.go" {2-3}
type Mirror struct {
	Target  conga.RequiredArg[string]   `help:"Destination directory"`
	Sources conga.RequiredArg[[]string] `help:"Directories to copy"`
}

func (m *Mirror) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "copying %v to %s\n", m.Sources.Get(), m.Target.Get())

	return nil
}
```

```sh
$ mirror /mnt/backup src/ docs/ photos/
copying [src/ docs/ photos/] to /mnt/backup
```

```sh error={2}
$ mirror /mnt/backup
mirror: error: missing required positional argument <sources>

Usage:
  mirror <target> <sources...>

Run 'mirror --help' for more information.
```

## Forward the words after `--` with `DashArgs`

A `conga.DashArgs` [field](/reference/struct-tags/#trailing-arguments-dashargs-rawargs) receives every word after `--`, unparsed, so the command can hand them to another program:

```go title="main.go" ins={4,9}
type Mirror struct {
	Target  conga.RequiredArg[string]   `help:"Destination directory"`
	Sources conga.RequiredArg[[]string] `help:"Directories to copy"`
	Rsync   conga.DashArgs
}

func (m *Mirror) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "copying %v to %s\n", m.Sources.Get(), m.Target.Get())
	fmt.Fprintf(session.Stdout(), "rsync options: %v\n", m.Rsync.Get())

	return nil
}
```

```sh
$ mirror /mnt/backup src/ docs/ -- --delete --dry-run
copying [src/ docs/] to /mnt/backup
rsync options: [--delete --dry-run]
```

Before the `--`, CONGA parses words as usual, so a forwarded option there is an unknown flag:

```sh error={2}
$ mirror /mnt/backup src/ --delete
mirror: error: unknown flag: --delete

Usage:
  mirror <target> <sources...> [-- <args...>]

Run 'mirror --help' for more information.
```

`DashArgs` also works on a command with subcommands, because the `--` marks where forwarding starts.

## Complete the forwarded words

To complete the words after `--`, give the `DashArgs` field a `predictor` tag. It takes the same values as on a flag: a built-in such as `file` or `dir`, or a name registered with `app.Predictor`:

```go title="main.go" {4,11-12}
type Mirror struct {
	Target  conga.RequiredArg[string]   `help:"Destination directory"`
	Sources conga.RequiredArg[[]string] `help:"Directories to copy"`
	Rsync   conga.DashArgs              `predictor:"rsync-options"`
}

// ...

func main() {
	app := conga.New[Mirror]().Name("mirror").
		EnableCompletion().
		Predictor("rsync-options", predict.Values("--checksum", "--delete", "--dry-run"))
	// ...
}
```

```text no-copy
$ mirror /mnt/backup src/ -- --<Tab>
--checksum  --delete    --dry-run
```

## Pass everything through with `RawArgs`

A `conga.RawArgs` field collects every word after the required positional arguments, flags included, without a `--`. It suits a wrapper whose own arguments end early, such as one target followed by the other program's arguments:

```go title="main.go"
type Mirror struct {
	Target conga.RequiredArg[string] `help:"Destination directory"`
	Rsync  conga.RawArgs
}

func (m *Mirror) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "target %s, rsync arguments: %v\n", m.Target.Get(), m.Rsync.Get())

	return nil
}
```

```sh
$ mirror /mnt/backup src/ --delete --dry-run
target /mnt/backup, rsync arguments: [src/ --delete --dry-run]
```

Help shows the collected words after the required arguments:

```sh {3}
$ mirror --help
Usage:
  mirror <target> [command] [args...]

Arguments:
  <target>  Destination directory

Flags:
      --help     Show help
      --version  Show version
```

Collection starts after the last required positional, so words up to that point are still parsed: `mirror /mnt/backup --help` prints help, and `mirror /mnt/backup src/ --help` forwards `--help`. Only required scalar arguments can come before `RawArgs`, and the command can't have subcommands; use `DashArgs` when you need either.

## Know what users see

| What happens | When | Result |
|---|---|---|
| The user leaves out a required argument | Run | `missing required positional argument <source>`, exit 2 |
| The user gives more words than the arguments take | Run | `unexpected argument "c"`, exit 2 |
| The user puts a forwarded option before `--` | Run | `unknown flag: --delete`, exit 2 |
| An `OptionalArg` is declared before a `RequiredArg` | Startup | `schema error: invalid positional argument order: optional positional argument cannot precede required positional argument <source>`, exit 1 |
| A slice argument is followed by another positional argument | Startup | `schema error: invalid positional argument order: slice argument <sources> must be the last positional argument`, exit 1 |
| A slice or optional argument comes before `RawArgs` | Startup | `schema error: raw args requires a deterministic positional prefix`, exit 1 |
| A command declares both positional arguments and subcommands | Startup | `schema error: positional argument Target cannot be combined with subcommand Check`, exit 1 |
| A `DashArgs` or `RawArgs` field carries a `help` tag | Startup | `schema error: invalid struct tag: tag "help" is invalid on dash args`, exit 1 |

## Where to go next

- [Positional arguments](/reference/struct-tags/#positional-arguments-optionalargt-requiredargt): every tag a positional argument takes.
- [Trailing arguments](/reference/struct-tags/#trailing-arguments-dashargs-rawargs): the `DashArgs` and `RawArgs` rules and the tags they accept.
- [How to declare subcommands](/how-to/declare-subcommands/): split a program into commands, each with its own arguments.
- [About flags and args](/explanation/flags-and-args/): why positional arguments bind only from the command line.
