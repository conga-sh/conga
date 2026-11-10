---
title: How to read resolved values after Run
description: Pass your own root struct to conga.New to read resolved values, and where each came from, after Run returns.
---

To read resolved values in `main` after `Run` returns, pass your own root struct to `conga.New`. This guide uses a one-flag `greet` program.

## Pass your own instance to `conga.New`

Allocate the root struct and pass its pointer. CONGA resolves every value into that instance, and `session.Root[CLI]()` returns the same pointer inside lifecycle methods:

```go title="main.go" hidden={1-9} {22-23,31-32}
package main

import (
	"context"
	"fmt"
	"os"

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
	app := conga.New(cli).
		Name("greet").
		EnvPrefix("GREET")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}

	fmt.Fprintf(os.Stderr, "name=%q source=%s detail=%q changed=%t\n",
		cli.Name.Get(), cli.Name.Source(), cli.Name.SourceDetail(), cli.Name.Changed())
}
```

```sh
$ greet --name Alice
Hello, Alice!
name="Alice" source=cli detail="--name" changed=true
```

## Read where each value came from

After `Run`, each container holds its value and the source that set it. `Source` returns a `conga.ValueSource`, `SourceDetail` names the flag, variable, file, or default it came from, and `Changed` reports whether the value came from the command line, an environment variable, or a config file:

```sh
$ greet
Hello, World!
name="World" source=default detail="World" changed=false
```

```sh
$ GREET_NAME=Bob greet
Hello, Bob!
name="Bob" source=env detail="GREET_NAME" changed=true
```

[Containers](/reference/conga/#containers) lists every reader method, and [About value precedence](/explanation/value-precedence/) explains which source wins.

## Handle `--help` and `--version` runs

`--help` and `--version` print their output and return `nil` from `Run` without resolving any value, so the code after `Run` sees every container unset:

```sh
$ greet --help
Usage:
  greet [flags]

Flags:
      --name="World"  Name to greet [env: GREET_NAME]
      --help          Show help
      --version       Show version
name="" source=none detail="" changed=false
```

Check `IsSet()`, or compare `Source()` with `conga.SourceNone`, before acting on a value after `Run`. On a usage error, `app.Exit` ends the process before that code runs.

## Know what goes wrong

| Mistake | When | Result |
|---|---|---|
| `conga.New` receives a nil pointer | Startup | Panic: `conga: New requires a non-nil root instance` |
| `Run` or `RunContext` is called a second time on one `App` | Second call | Panic: `conga: App already executed` |
| The code after `Run` reads values on a `--help` or `--version` run | After `Run` | Zero values with source `none` |
| The instance passed to `conga.New` is never used outside the app | `conga-lint` | `conga.New root instance is not referenced outside the app (use conga.New[CLI]() to let the engine allocate the root)` |

The last row is only a lint finding, and the program still runs normally. When nothing outside the app reads the root, use `conga.New[CLI]()`.

## Where to go next

- [Containers](/reference/conga/#containers): `Get`, `GetOr`, `IsSet`, `Changed`, `Source`, and `SourceDetail` for every container.
- [How to test a command](/how-to/test-a-command/): run a fresh `App` per test case and assert on the resolved values.
- [About value precedence](/explanation/value-precedence/): how CONGA picks the winning source for each value.
