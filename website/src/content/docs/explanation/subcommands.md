---
title: About subcommands and hierarchy
description: Why CONGA builds the command tree from struct types, how routing and implicit subcommands select a command, and why ancestors reserve their flag names.
---

A CONGA command tree is built from Go types. A field of type `conga.Command[T]` mounts the struct `T` as a subcommand, and `T` can declare further `Command[U]` fields to any depth. [How to declare subcommands](/how-to/declare-subcommands/) builds one, and [Subcommands](/reference/struct-tags/#subcommands-commandt) in the struct tag reference lists the tags and rules.

## Command trees as typed graphs

```go
type App struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a service to a cluster"`
	Remove conga.Command[RemoveCmd] `summary:"Remove a service"`
}

type DeployCmd struct {
	Worker conga.Command[WorkerCmd] `summary:"Deploy a background worker"`
}
```

The tree's shape is fixed at compile time. Schema build walks the complete struct graph before parsing any command-line token, and validates every tag and every flag name along each ancestor-descendant path. A schema defect in any subcommand, selected or not, fails startup with `ErrSchema`, so a misconfigured subcommand can't ship unnoticed. A leaf without a `Run` method is the exception. CONGA checks it only for the command an invocation selects, so it fails when invoked, with `runner not implemented: command "<name>" does not implement conga.Runner` (exit 1), while its siblings keep working. `conga-lint` reports every such leaf before you build.

## Routing nodes and runnable commands

CONGA classifies a command with subcommands by whether its struct implements `Runner`.

A command without `Run` is a pure routing node. Invoking it without a subcommand produces a usage error (exit code 2) and renders the command's help screen.

A command with `Run` is an executable intermediate command. Invoked alone, it runs its own `Run`. Invoked with a subcommand token, it routes to the child.

A leaf command must implement `Runner`. [Command tree rules](/reference/struct-tags/#command-tree-rules) lists the outcome of each case.

## Implicit subcommands

Tools such as Bun run their most common operation without naming it, so `bun ./script.ts` runs `bun run ./script.ts`. In CONGA, a subcommand tagged `implicit:"true"` receives the tokens that no explicit subcommand claims. The examples in this section use this schema:

```go
type CLI struct {
	Exec    conga.Command[RunCmd] `implicit:"true" name:"run"`
	Install conga.Command[InstallCmd]
	Deploy  conga.Command[DeployCmd]
}

type RunCmd struct {
	DryRun conga.OptionalFlag[bool] `short:"n"`
	File   conga.RequiredArg[conga.File]
	Args   conga.RawArgs
}

type DeployCmd struct {
	Env conga.OptionalFlag[string]
}
```

`tool ./script.ts` runs as `tool run ./script.ts`. The routing rules follow from one priority. The implicit subcommand takes only tokens that would otherwise be unmatched, and never overrides an explicit selection. [Implicit subcommands](/reference/struct-tags/#implicit-subcommands) lists the rules.

### Named subcommands win

A token that matches a sibling's name or alias selects that sibling. Adding an implicit subcommand therefore never changes the meaning of an invocation that was valid before.

The trade-off is in error reporting. Without an implicit subcommand, `tool instal` is an unknown command and gets a "did you mean install?" suggestion. With one, `instal` is a valid operand, and the implicit subcommand runs with it.

### Routing reads left to right

Operands and subcommand names are classified in the order they appear. In `tool --env prod deploy`, `prod` is read before `deploy`. At that point `--env` belongs to no selected command, so `prod` is an unmatched operand and selects the implicit subcommand, which then rejects `--env` as an unknown flag (exit 2). Reclassifying `prod` after reading `deploy` would require backtracking, which the parser avoids for the same reason that [ancestors reserve their flag names](#deterministic-single-pass-parsing).

A flag declared only by the implicit subcommand or its implicit descendants selects that subcommand regardless of position, since no other command declares it. `tool -n` therefore reports `missing required positional argument <file>` from `run` rather than an unknown flag.

### Built-in requests stay where they are

`--help`, `--why`, and `--completion` do not select a subcommand. `tool --help` renders the parent's help screen, which lists the implicit subcommand and its siblings. After an operand or child-only flag has selected the implicit subcommand, help renders for that subcommand.

`--version` is root-only. Once a subcommand is selected, by name or implicitly, `--version` is no longer the built-in, so `tool ./script.ts --version` and `tool run ./script.ts --version` parse identically. Consistency between implicit and named selection takes priority over a version summary inside a subcommand.

### The parent's role decides the bare invocation

When no subcommand is selected, the parent's [role](#routing-nodes-and-runnable-commands) decides. A routing node enters its implicit subcommand. An executable intermediate command runs its own `Run`.

An implicit subcommand can declare an implicit subcommand of its own. CONGA descends through the chain only as far as the operands and child-only flags require, and stops at the first runnable command.

## Shared options on the root

Options for the whole tool are declared on the root struct, and each subcommand declares only its own flags. A handler reads root flags through the session, without copying them onto subcommands or holding them in package-level variables:

```go
func (w *WorkerCmd) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[App]()
	if root.Verbose.Get() {
		fmt.Fprintf(session.Stderr(), "invoking: %s\n", session.Path())
	}

	return nil
}
```

An invocation selects one path through the tree, from the root to the command that runs. CONGA binds values only along that path. Containers on commands off the path receive no values, and their lifecycle hooks don't run. A handler reaches intermediate commands from the root through `Command[T].Get()`, and `Command[T].IsSelected()` reports whether a command is on the path.

## Feature-gated subcommands

A `Command[T]` field accepts a `feature:"..."` tag. When the expression doesn't hold for a run, CONGA removes the subcommand and everything below it from that run. It doesn't route, doesn't appear in the parent's help, and isn't offered by completion. Invoking it is a usage error (exit code 2). [About feature toggles](/explanation/feature-toggles/) explains why inactive members are removed.

## Config sections follow the command tree

Config keys nest by command. The flags of `deploy worker` are read from the `deploy` section, then its `worker` section.

Environment variables are not nested. A config file holds settings for every command at once, so each command needs its own section. A run selects one path, so a prefix inherited down that path can't collide within the run. A subcommand therefore inherits its parent's prefix, and same-named flags on sibling commands read the same variable unless one sets an `env` or `envprefix` tag. [How to bind environment variables](/how-to/bind-environment-variables/) shows both.

## Ancestors reserve their flag names

A flag name, alias, or short rune declared on a command is reserved for every descendant, and redeclaring it fails schema build with `ErrSchema`. Siblings may reuse names:

```text no-copy
cloudctl deploy --force      + cloudctl remove --force   (allowed: sibling commands)
cloudctl --verbose           + cloudctl deploy --verbose (reserved: ancestor-descendant collision)
```

### Deterministic single-pass parsing

A flag is accepted anywhere on the command line, so `cloudctl --verbose deploy worker` and `cloudctl deploy worker --verbose` are equivalent. If `deploy` could declare its own `--verbose`, the second form would be ambiguous, and resolving it would need backtracking or an arbitrary precedence rule. Because every flag on the selected path has exactly one owner, parsing completes in a single pass.

### Predictable shell completion

Completion must determine which flags remain available at the cursor. With one owner per flag on the path, it can do so regardless of token order, without simulating which command each flag binds to.

### Sibling independence

The reservation applies only along ancestor-descendant paths. `deploy` and `remove` are never on the same path, so both can declare `--force`. Options shared by the whole tool belong on the root.
