---
title: How to retain the root struct
description: Bind an existing instance with conga.New to inspect resolved container values after execution completes.
---

CONGA supports two root initialization patterns: engine-allocated (`conga.New[T]()`) and caller-owned (`conga.New(cli)`). Pass an existing instance pointer to `conga.New` to inspect resolved container values after `Run` completes.

## Bind a caller-owned instance

To retain access to the root struct outside the execution lifecycle, construct the instance and pass its pointer to `conga.New`:

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Name conga.OptionalFlag[string] `default:"World" help:"Name to greet"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "Hello, %s!\n", c.Name.Get())

	return nil
}

func main() {
	cli := &CLI{}

	app := conga.New(cli)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	if cli.Name.Changed() {
		fmt.Printf("greeted %s (source: %s)\n", cli.Name.Get(), cli.Name.SourceDetail())
	}
}
```

When `conga.New(cli)` is called:
- The engine binds directly to the provided instance pointer instead of allocating a fresh struct.
- Flags and arguments resolve into the instance's typed containers during the resolution phase.
- Lifecycle hooks (`PreRun`, `Run`, `PostRun`) execute using the bound instance, and `session.Root[CLI]()` returns the same pointer. The type argument must match the root struct type exactly; passing a different type panics at runtime, and `conga-lint` verifies this statically.

## Enforce initialization invariants

CONGA validates root instances at startup: passing a nil pointer (`conga.New[*CLI](nil)`) or multiple instances panics immediately. Calling `Run()` or `RunContext()` a second time on the same `App` also panics, because each application executes once. The full contract is documented in [Package conga](/reference/conga/#the-app-builder).

Built-in requests such as `--help` and `--version` render their output and return before value binding runs, so containers remain in their unset initial state after those invocations.

## Inspect resolved values after execution

After `app.Run()` returns, containers on the bound instance hold their final winning values:

```go
if err := app.Run(); err != nil {
	app.Exit(err)
}

value := cli.Name.Get()
tier := cli.Name.Source()
origin := cli.Name.SourceDetail()
isCustom := cli.Name.Changed()
```

The reader surface and the meaning of each method are catalogued in the [package conga container table](/reference/conga/#containers); [About value precedence](/explanation/value-precedence/#provenance-tracking) explains provenance.

If the root instance is only needed inside lifecycle hooks and never outside `app.Run()`, prefer the engine-allocating form `conga.New[T]()`. The static analyzer `conga-lint` detects throwaway instances passed to `conga.New(cli)` that are never referenced outside the App builder chain.

## Where to go next

- [About flags and args](/explanation/flags-and-args/) — container methods, trade-offs, and reader guarantees.
- [How to test a command](/how-to/test-a-command/) — construct isolated App instances per test case with captured streams.
