---
title: About subcommands and hierarchy
description: Command tree modeling with Command[T], routing versus runnable nodes, session state traversal, and the ancestor flag reservation invariant.
---

Command-line tools often grow beyond single-purpose utilities into structured suites of operations. CONGA models these multi-command hierarchies through static struct composition: declaring a field of type `conga.Command[T]` mounts `T` as a subcommand, creating a typed command tree derived directly from Go types.

For step-by-step setup instructions, see [How to declare subcommands](/how-to/declare-subcommands/). For tag syntax, see the [struct tag reference](/reference/struct-tags/).

## Command trees as typed graphs

CONGA defines the entire command hierarchy as a static graph of Go types. The root application struct declares its immediate subcommands as `conga.Command[T]` fields. Each child struct `T` can declare further `Command[U]` fields, nesting commands to any depth:

```go
type App struct {
	Deploy conga.Command[DeployCommand] `summary:"Deploy a service to a cluster"`
	Remove conga.Command[RemoveCommand] `summary:"Remove a service"`
}

type DeployCommand struct {
	Worker conga.Command[WorkerCommand] `summary:"Deploy a background worker"`
}
```

Because command nesting is expressed through ordinary struct fields, the shape of the command tree is known at compile time and validated during application startup. Before the engine processes a single command-line token, it inspects the complete struct graph:
- It confirms that every leaf command implements the `Runner` interface (`Run(ctx context.Context, session *conga.Session) error`).
- It verifies that names, aliases, and short flags are unique across ancestor-descendant paths.
- It validates all struct tags, detecting typos and unsupported options immediately.

A defect anywhere in the tree fails startup with `ErrSchema`, ensuring that misconfigured subcommands cannot escape into production.

## Routing nodes versus runnable commands

Command trees serve two operational roles: organizing related operations under shared categories, and executing concrete actions. CONGA distinguishes these roles by inspecting whether command structs implement the `Runner` interface.

### Pure routing nodes
Large tools frequently introduce category commands—such as `cloudctl service` or `git remote`—that exist solely to group related child commands. When a command struct does not implement `Runner`, the engine classifies it as a pure routing node.

Invoking a routing node without a subcommand produces a usage error (exit code 2) and renders the command's help screen, directing the operator to choose a valid child command.

### Executable intermediate commands
Some commands perform an action on their own while still exposing specialized subcommands. For example, a status command might report an overview when invoked directly, yet offer subcommands for detailed component diagnostics.

When an intermediate command struct implements `Runner` and also declares `Command[T]` fields, CONGA permits both modes:
- Invoking the command token alone executes the parent command's `Run` method.
- Supplying a child command token routes execution directly to the child.

### Leaf commands
Terminal nodes in the command hierarchy must implement `Runner`. Defining a leaf command struct without a `Run` method produces an `ErrSchema` error during schema initialization.

## State isolation and branch activation

In multi-command tools, subcommands frequently need access to shared runtime options—such as a verbose flag, an API token, or a database connection pool—declared at the root of the application.

Rather than scattering copies of shared options across subcommands or maintaining package-level global variables, CONGA keeps global options on the root struct and provides typed access to them during execution.

### Scoped command state
Each command struct in CONGA declares only the flags and arguments relevant to its own scope. Shared options stay at the root command, where they belong conceptually.

When a leaf or intermediate command needs root state, it queries the execution session using `session.Root[T]()`:

```go
func (w *WorkerCommand) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[App]()
	if root.Verbose.Get() {
		fmt.Fprintf(session.Stderr(), "invoking: %s\n", session.Path())
	}

	return nil
}
```

`session.Root[T]()` returns a typed pointer to the root application struct (`*T`), giving handlers compiler-verified access to root flags without global variables. To inspect intermediate commands along the path, handlers navigate downward from the root using `Command[T].Get()`.

### Branch activation
An invocation selects exactly one path through the command tree, from the root application to the target leaf. The engine populates and binds flags only along this active chain. Unselected sibling branches remain untouched: their containers receive no values, and their lifecycle hooks never execute.

Handlers can check whether a specific subcommand was selected on the active path by calling `cmd.IsSelected()`.

## Hierarchical configuration mapping

The command tree structure maps directly onto external config files. When an application registers a configuration decoder (such as JSON, YAML, or TOML), the engine routes configuration keys to subcommands using dot-delimited paths or nested configuration tables.

For example, a flag declared on a `WorkerCommand` mounted under a `DeployCommand` maps to:
- `deploy.worker.concurrency` in flat key-value formats.
- A nested table `[deploy.worker]` with key `concurrency` in TOML.

This alignment ensures that command line flags, environment variable prefixes, and config files follow the identical structural hierarchy.

## The ancestor flag reservation invariant

CONGA enforces a strict grammar invariant across the entire command hierarchy: **no descendant command may declare a flag name, alias, or short rune already declared by any of its ancestors**.

Attempting to register a descendant flag that collides with an ancestor flag fails schema construction with `ErrSchema` (wrapping `ErrDuplicateFlag`).

```text no-copy
cloudctl deploy --force      + cloudctl remove --force   (allowed: sibling commands)
cloudctl --verbose           + cloudctl deploy --verbose (prohibited: ancestor-descendant collision)
```

This prohibition is not an arbitrary restriction. It enables several fundamental architectural guarantees:

### Deterministic single-pass parsing
Command-line operators expect flags to work regardless of where they appear on the command line. An operator might run `cloudctl --verbose deploy worker` or `cloudctl deploy worker --verbose`.

If descendant commands could shadow ancestor flags, parsing would become ambiguous. The parser could not determine whether `--verbose` belongs to the global application runtime or configures the `deploy` subcommand without backtracking or applying arbitrary precedence heuristics. By guaranteeing that every flag token across an active branch belongs to exactly one command, CONGA achieves deterministic, single-pass parsing without backtracking.

### Predictable shell completion
Shell completion engines must determine valid options at any cursor position. When flags are unique across ancestor chains, the completion engine can immediately identify which flags remain unassigned, regardless of token order, without needing to simulate contextual command state switches.

### Sibling independence
The shadowing prohibition applies exclusively along ancestor-descendant lines. Sibling subcommands do not share an ancestor relationship and remain completely independent. Two sibling commands—such as `deploy` and `remove`—may both declare a `--force` flag without collision. Shared options belong on the root command; command-specific options remain encapsulated within their respective subcommands.
