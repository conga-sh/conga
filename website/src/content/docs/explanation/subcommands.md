---
title: About subcommands and hierarchy
description: Command tree modeling with Command[T], session hierarchy inspection, and the ancestor flag reservation invariant.
---

A CONGA command tree is represented by struct composition: fields of type `conga.Command[T]` model subcommands, creating a typed hierarchy derived from Go types. This page explains the architectural design of command hierarchies, how execution state travels through the session, and why flag shadowing across ancestor chains is strictly prohibited. For procedural instructions on mounting subcommands, see [How to declare subcommands](/how-to/declare-subcommands/).

## Command tree structure

In CONGA, command nesting is expressed through fields of type `conga.Command[T]`:

```go
type App struct {
	Deploy conga.Command[DeployCommand] `name:"deploy" summary:"Deploy a service"`
	Remove conga.Command[RemoveCommand] `name:"remove" summary:"Remove a service" aliases:"rm,delete" group:"Maintenance"`
}
```

Each `Command[T]` field mounts `T` as a child command. Because `T` is an ordinary command struct, it may itself declare `Command[U]` fields, permitting hierarchies of arbitrary depth.

CONGA validates the command tree at schema construction time:
- Every leaf command must satisfy `Runner` (`Run(ctx context.Context, session *conga.Session) error`).
- Intermediate routing nodes without a `Runner` require a subcommand token during invocation; omitting one produces a usage error.
- Field names convert to kebab-case names by default, overridden by the `name` tag.

## Groups and visibility

Command listings on generated help screens reflect metadata declared on each `Command[T]` field:
- `group:"<name>"` partitions sibling commands under dedicated section headers.
- `hidden` leaves a command invocable while omitting it from help listings and shell completion.
- `aliases:"<a,b>"` registers alternate tokens that resolve to the same command node. Aliases participate in did-you-mean distance suggestions during spelling errors.

## Inspecting the hierarchy through the session

When a command executes, its lifecycle hooks receive `*conga.Session`, which provides access to the invocation path and ancestor state:

| Method | Returns |
|--------|---------|
| `session.Root[T]()` | The application struct bound to `T` (`*T`), or nil on type mismatch |
| `session.Path()` | The full invocation path string (for example `"cloudctl deploy worker"`) |
| `session.Name()` | The leaf command name (for example `"worker"`) |
| `session.Args()` | Trailing or unparsed positional arguments for the invocation |

A leaf command accesses global flags declared on the root struct via `session.Root[T]()`:

```go
func (w *WorkerCommand) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[App]()
	if root.Verbose.Get() {
		fmt.Fprintf(session.Stderr(), "invoking: %s\n", session.Path())
	}

	return nil
}
```

Enclosing intermediate command structs are reached by traversing the selected `Command[T]` fields down from the root instance:

```go
root := session.Root[App]()
deploy := root.Deploy.Get()
region := deploy.Region.Get()
```

## Forbidden flag shadowing

CONGA enforces strict ancestor flag uniqueness across the command hierarchy: **a flag, alias, or short rune declared on an ancestor command cannot be declared on any descendant command**. Violating this constraint fails schema construction with `ErrSchema` (wrapping `ErrDuplicateFlag`).

```
cloudctl deploy --force      + cloudctl remove --force   (allowed: sibling commands)
cloudctl --verbose           + cloudctl deploy --verbose (prohibited: ancestor-descendant chain)
```

### Architectural rationale

The shadowing prohibition provides deterministic, single-pass argument parsing:

1. **Unambiguous token ownership**: When `--verbose` is declared at the root command, a token sequence such as `cloudctl deploy --verbose` has exactly one owner. Permitting descendant shadowing would introduce ambiguity regarding whether `--verbose` belongs to `deploy` or configures the global application runtime before subcommand dispatch.
2. **Predictable shell completion**: Shell completion drivers can accurately determine whether a flag remains valid anywhere in the command line without evaluating backtracking rules or contextual state switches.
3. **Explicit state hierarchy**: Shared flags belong at the root command. Child commands access shared values through `session.Root[T]()` rather than duplicating flag definitions across multiple subcommands.
