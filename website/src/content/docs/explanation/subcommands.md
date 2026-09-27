---
title: About subcommands & hierarchy
description: Route subcommands with Command[T], inspect the active chain through the session, and keep flags unique per hierarchy.
---

## Declaring subcommands

A subcommand is a struct field of type `conga.Command[T]` where `T` is another command struct implementing `Run(ctx context.Context, session *conga.Session) error`:

```go
type App struct {
	Deploy conga.Command[DeployCommand] `name:"deploy" summary:"Deploy a service"`
	Remove conga.Command[RemoveCommand] `name:"remove" summary:"Remove a service" aliases:"rm,delete" group:"Maintenance"`
}

type DeployCommand struct {
	Service  conga.RequiredArg[string] `help:"Service name"`
	Replicas conga.OptionalFlag[int]   `default:"3" help:"Number of replicas"`
}

func (d *DeployCommand) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying %s\n", d.Service.Get())

	return nil
}

func main() {
	app := conga.New[App]().Name("cloudctl").Description("Cloud management tool")
	app.FatalIfError(app.Run())
}
```

```sh
$ cloudctl deploy web-api
deploying web-api
$ cloudctl rm web-api        # via alias
```

Nested hierarchies are arbitrary in depth: a subcommand struct may itself declare `Command[T]` fields.

## Groups and visibility

- `group:"Maintenance"` groups sibling commands under a heading in help output.
- `hidden` hides a command from listings and completion while keeping it invocable.
- Command aliases participate in did-you-mean suggestions: `cloudctl deplou` suggests `deploy`.

## Inspecting the active chain through the session

Commands and lifecycle hooks receive everything about the hierarchy through the `*conga.Session` passed alongside `context.Context`:

| Method | Returns |
|--------|---------|
| `session.Root[T]()` | The application struct bound to `T` (`*T`), nil on type mismatch |
| `session.Path()` | Full invocation path, e.g. `"cloudctl deploy worker"` |
| `session.Name()` | Leaf command name, e.g. `"worker"` |

```go
func (w *WorkerCommand) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[App]() // access global flags declared on the root struct
	if root.Verbose.Get() {
		fmt.Fprintf(session.Stderr(), "path: %s\n", session.Path())
	}

	return nil
}
```

Enclosing command structs are reached by walking the selected `Command[T]` fields from the root:

```go
root := session.Root[App]()
deploy := root.Deploy.Get()
region := deploy.Region.Get()
```

## Global and persistent flags

Declare flags on the root struct and read them in subcommands via `session.Root[T]()`. Per-command environment prefixes are available through the `envprefix` command tag.

## Flag shadowing is forbidden

A flag name, alias, or short rune declared on an ancestor command is reserved across its whole descendant chain; redeclaring one fails schema building with `ErrSchema`. Sibling commands may freely reuse identical names — `deploy --force` and `remove --force` are independent.

```
✅ cloudctl deploy --force      + cloudctl remove --force   (siblings)
❌ cloudctl --verbose           + cloudctl deploy --verbose (ancestor/descendant)
```

The rule keeps parsing unambiguous in one pass: with `--verbose` bound at the root and also possible at a leaf, a token like `cloudctl deploy --verbose` would have two legitimate owners.
