---
title: How to declare subcommands
description: Route nested subcommands with Command[T], assign aliases and sections, set a default subcommand, and reach root state.
---

Commands in CONGA compose hierarchically by declaring struct fields of type `conga.Command[T]`. This guide explains how to define subcommands, configure routing tags, access root state, and prevent ancestor flag shadowing.

## Mount a subcommand

Declare a field of type `conga.Command[T]` on the enclosing struct, where `T` is a struct implementing the `Runner` interface:

```go title="main.go"
type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a service to a cluster"`
}

type DeployCmd struct {
	Replicas conga.OptionalFlag[int] `default:"3" help:"Number of replicas" short:"r"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying service with %d replicas\n", d.Replicas.Get())

	return nil
}
```

Field names convert to kebab-case by default (`Deploy` becomes the `deploy` subcommand). The `summary` tag defines the short text displayed in parent command listings.

Leaf commands must implement `Run(ctx context.Context, session *conga.Session) error`. Defining a leaf command struct without a `Run` method produces an `ErrSchema` error during schema initialization.

An enclosing parent command may also implement `Run`, executing when the application is invoked without a subcommand; [About subcommands and hierarchy](/explanation/subcommands/#routing-nodes-versus-runnable-commands) covers routing versus executable nodes.

## Configure aliases, sections, and visibility

Use struct tags on the `conga.Command[T]` field to control how subcommands route and display in `--help`:

```go title="main.go"
type CLI struct {
	Verbose conga.OptionalFlag[bool] `short:"v" help:"Enable verbose output"`

	Deploy conga.Command[DeployCmd] `summary:"Deploy a service"`
	List   conga.Command[ListCmd]   `aliases:"ls,ps" section:"Inspection" summary:"List active services"`
	Debug  conga.Command[DebugCmd]  `hidden:"true" summary:"Internal debugging command"`
}
```

- `name:"..."`: Overrides the kebab-case field name with a custom command token.
- `aliases:"ls,ps"`: Registers comma-separated alternatives that route to the same command.
- `section:"Inspection"`: Collects related commands under a dedicated section header in parent help screens.
- `hidden:"true"`: Keeps the command executable while suppressing it from help listings and completion suggestions.
- `deprecated:"..."`: Marks the command as deprecated, emitting a warning at runtime, rendering a `[deprecated: <reason>]` banner on its help screen, and suppressing it from shell completion.

Help screens organize subcommands by their declared sections:

```sh
$ cloudctl --help
Usage:
  cloudctl <command> [flags]

Commands:
  deploy  Deploy a service

Inspection Commands:
  list    List active services (aliases: ls, ps)

Flags:
  -v, --verbose  Enable verbose output
      --help     Show help
      --version  Show version
```

## Make one subcommand the default

To let operators drop the subcommand name, so `tool ./script.ts` runs exactly like `tool run ./script.ts`, tag that subcommand `implicit:"true"`:

```go title="main.go"
type CLI struct {
	Exec    conga.Command[RunCmd]     `implicit:"true" name:"run" summary:"Execute a file"`
	Install conga.Command[InstallCmd] `summary:"Install dependencies"`
}

type RunCmd struct {
	DryRun conga.OptionalFlag[bool] `help:"Print what would run" short:"n"`
	File   conga.RequiredArg[conga.File]
	Args   conga.RawArgs
}
```

Each of these invocations runs `run`:

```sh
tool ./script.ts
tool run ./script.ts
tool -n ./script.ts
```

`tool install` still runs `install`, because a subcommand named on the command line always wins. Help marks the default and shows the command token as optional:

```text no-copy
Usage:
  tool [<command>]

Commands:
  run      Execute a file (default)
  install  Install dependencies
```

Declare at most one implicit subcommand per command; a second one fails schema build with `ErrSchema`, and `conga-lint` reports it before then.

Adjust for these cases:

- If an operand begins with a dash, put it after `--`: `tool -- -e.ts` runs `run` with the file `-e.ts`.
- If a subcommand flag takes a value, write it after the subcommand name: `tool deploy --env prod`, not `tool --env prod deploy`. In the second form, `prod` is an operand and selects the implicit subcommand.
- If a bare `tool` should do something other than enter the implicit subcommand, implement `Run` on the parent. A parent without `Run` enters its implicit subcommand when the invocation names no subcommand; a parent with `Run` runs itself unless an operand or a flag declared only by the implicit subcommand selects it.

The full routing rules, including how `--help` and `--version` behave, are in the [struct tag reference](/reference/struct-tags/#implicit-subcommands). [About subcommands and hierarchy](/explanation/subcommands/#implicit-subcommands) explains why routing works this way.

## Access root state and execution context

Subcommands read global flags and metadata declared on the root command struct through `session.Root[T]()`:

```go title="main.go"
type CLI struct {
	Verbose conga.OptionalFlag[bool] `help:"Enable verbose output" short:"v"`
	Deploy  conga.Command[DeployCmd] `summary:"Deploy a service"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[CLI]()

	if root.Verbose.Get() {
		fmt.Fprintf(session.Stdout(), "running %s (leaf: %s)\n", session.Path(), session.Name())
	}

	return nil
}
```

`session.Root[T]()` returns the typed root command struct instance. The full accessor catalog (`Path()`, `Name()`, and the stream accessors) is in [package conga](/reference/conga/#session-and-hierarchy-accessors); navigate down the hierarchy from the root instance by calling `Get()` on a command field (for example, `root.Deploy.Get()`).

## Share flags across subcommands

Ancestor flags remain available to all descendant subcommands. Flag names, aliases, and short runes are reserved throughout each command branch; declare shared options once on the parent command:

```go
type Root struct {
	Verbose conga.OptionalFlag[bool] `short:"v"`
	Child   conga.Command[Child]
}

type Child struct {
	Verbose conga.OptionalFlag[bool] `short:"v"` // [!code error] Schema error: duplicate flag -v
}
```

Attempting to register a descendant flag that collides with an ancestor flag fails at startup with `conga.ErrSchema`; [About subcommands and hierarchy](/explanation/subcommands/#the-ancestor-flag-reservation-invariant) explains the invariant. Sibling subcommands may declare identical flag names.

## Where to go next

- [How to accept positional arguments](/how-to/accept-positional-arguments/) — define required, optional, and trailing positional arguments.
- [About subcommands and hierarchy](/explanation/subcommands/) — hierarchy architecture and flag-shadowing rules.
- [How to retain the root struct](/how-to/keep-root-reference/) — inspect resolved container values after execution.
- [Example 32 — implicit command](https://github.com/conga-sh/conga/tree/main/_examples/32-implicit-command) — a runnable default subcommand with an explicit sibling.
