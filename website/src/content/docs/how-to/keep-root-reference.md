---
title: How to keep a reference to the root struct
description: Bind a caller-owned instance with conga.New(cli) to seed fields before execution and read resolved values after it.
---

`conga.New[T]()` lets the engine allocate the root command struct at execution time and exposes it to hooks through `session.Root[T]()`. When the surrounding program needs the same struct outside the lifecycle — to seed a dependency before execution or read resolved values after it — construct the instance first and pass it to `conga.New(cli)`.

## Bind an instance you own

```go title="main.go"
package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Name   conga.OptionalFlag[string] `default:"World" help:"Name of the person to greet"`
	Logger *slog.Logger               `conga:"-"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	c.Logger.Info("greeting", "name", c.Name.Get())

	return nil
}

func main() {
	cli := &CLI{Logger: slog.New(slog.DiscardHandler)}

	app := conga.New(cli).Name("hello")

	app.FatalIfError(app.Run())

	fmt.Println("greeted", cli.Name.Get(), "from", cli.Name.Source())
}
```

`conga.New(cli)` binds the instance: the engine resolves flag values into its containers, `Run` executes on that pointer, and `session.Root[CLI]()` inside any hook returns it. Fields excluded from the schema with `conga:"-"` keep the value seeded before `Run`, which is how a host injects dependencies such as a logger; flags and positional arguments are resolved by the engine from the [resolution tiers](/explanation/value-precedence/).

```sh
go run .
```

```
greeted World from default
```

```sh
go run . --name conga
```

```
greeted conga from cli
```

:::note
Built-in requests such as `--help` and `--version` render and return before value binding, so containers stay unset after those invocations.
:::

## Reach the root from a hook

When no host code needs the struct, keep `conga.New[T]()` and read the engine-allocated instance from the session. `session.Root[T]()` is the entry point for nested command hooks, where the receiver is the subcommand struct rather than the root:

```go
func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	root := session.Root[CLI]()
	fmt.Fprintf(session.Stdout(), "deploying %s to %s\n", d.Service.Get(), root.Region.Get())

	return nil
}
```

The bound instance is the same one hooks execute on, whether it was caller-owned or engine-allocated.

## Inspect resolved values after execution

After `Run` returns, every container holds its resolved winner, so a host can log or assert it with the [reader surface](/explanation/reading-values/): `Get()` for the value, `Source()` and `SourceDetail()` for the tier that supplied it, `Changed()` to distinguish input from defaults.

```go
app.FatalIfError(app.Run())

if cli.Name.Changed() {
	fmt.Println("name was supplied by", cli.Name.SourceDetail())
}
```

## Guards and lint

- At most one root instance can be bound, and it must be non-nil. `conga.New(nil)` panics with `conga: New requires a non-nil root instance`; `conga.New(first, second)` panics with `conga: New accepts at most one root instance`.
- `conga-lint` reports a bound root that is never referenced outside the app and steers to the engine-allocating form; reading, seeding, or capturing the instance suppresses the diagnostic — see [How to lint CONGA schemas](/how-to/lint-schemas/).

## Where to go next

- [About reading values](/explanation/reading-values/) — the container methods and the guarantees behind `Get()`.
- [How to test a command](/how-to/test-a-command/) — construct an App per test case and assert output and exit codes.
- [About app execution](/explanation/execution/) — the single-execution rule and `session.Reload[T]` for fresh configuration.
- [How to run the examples](/how-to/examples-cookbook/) — example 26 embeds the App in a host process with injected streams, args, and a custom exit handler.
