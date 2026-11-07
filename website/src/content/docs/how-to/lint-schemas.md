---
title: How to lint schemas
description: Run conga-lint to catch schema errors, tag typos, missing Run methods, and flag shadowing before the program runs, and check builder call order.
---

`conga-lint` finds the schema mistakes the program otherwise reports one at a time at startup, and leaf commands without a `Run` method, which fail only when a user invokes them. This guide lints an example `cloudctl` program with three mistakes. [conga-lint](/reference/conga-lint/) lists every check and flag.

## Add the analyzer to your module

Declare `conga-lint` as a tool dependency:

```sh
go get -tool github.com/conga-sh/conga/cmd/conga-lint@latest
```

## Run the analyzer

Run it with `go tool` over your packages. This example declares `--verbose` on both the root and `deploy`, misspells `help` as `hlep`, and gives `StatusCmd` no `Run` method:

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
	Status conga.Command[StatusCmd] `summary:"Show service status"`
}

type DeployCmd struct {
	Verbose  conga.OptionalFlag[bool] `help:"Print deploy details"` // [!code error]
	Replicas conga.OptionalFlag[int]  `default:"3" hlep:"Number of replicas"` // [!code error]
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying %d replicas\n", d.Replicas.Get())

	return nil
}

type StatusCmd struct{} // [!code error]

func main() {
	app := conga.New[CLI]().
		EnableCompletion().
		Name("cloudctl")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```sh error={2-4}
$ go tool conga-lint ./...
main.go:18:2: command "deploy": schema error: duplicate flag: --verbose (already declared by ancestor command "cli")
main.go:19:2: struct DeployCmd field Replicas: schema error: invalid struct tag: unknown struct tag "hlep" (did you mean "help"?). Use app.AllowTags("hlep") to permit external tags
main.go:28:6: leaf command "status" (struct StatusCmd) does not implement conga.Runner (implement Run(context.Context, *conga.Session) error)
```

Running the program shows only the first mistake:

```sh error={2}
$ cloudctl status
cloudctl: error: struct DeployCmd field Replicas: schema error: invalid struct tag: unknown struct tag "hlep" (did you mean "help"?). Use app.AllowTags("hlep") to permit external tags
```

Go's `./...` pattern skips directories that start with `_`, such as `_examples/`; name those paths explicitly.

:::tip[Cache results with `go vet`]
`go vet -vettool="$(go tool -n conga-lint)" ./...` runs the same analyzer and caches results between runs. `go vet` then runs only this analyzer, not its standard checks.
:::

## Check builder call order

`-builder-order` also checks the builder calls at each `conga.New` against a canonical order: identity calls such as `Name` first, then the `Enable*` and `Disable*` toggles, then behaviour, then execution. In this example `EnableCompletion` comes before `Name`:

```sh error={2}
$ go tool conga-lint -builder-order ./...
main.go:33:3: builder call order violation: Name (rank 1) called after EnableCompletion (rank 9); use -fix to reorder
main.go:18:2: command "deploy": schema error: duplicate flag: --verbose (already declared by ancestor command "cli")
main.go:19:2: struct DeployCmd field Replicas: schema error: invalid struct tag: unknown struct tag "hlep" (did you mean "help"?). Use app.AllowTags("hlep") to permit external tags
main.go:28:6: leaf command "status" (struct StatusCmd) does not implement conga.Runner (implement Run(context.Context, *conga.Session) error)
```

To reorder the calls in place, add `-fix`. It rewrites the files and prints nothing:

```sh
go tool conga-lint -builder-order -fix ./...
```

Add `-diff` to print the change without applying it. `-fix` moves calls only where the swap is safe; it never moves `Use(...)`, which runs plugin code.

With the three mistakes fixed as well, the check passes and prints nothing (exit 0):

```sh
go tool conga-lint -builder-order ./...
```

## Use your team's builder order

`-builder-order-spec` replaces the canonical order with your own: method names separated by commas, or `@path` to read them one per line from a file. Methods the list leaves out can go anywhere. On the fixed example, a spec that puts `EnableCompletion` before `Name` turns the canonical order into a violation:

```sh error={2}
$ go tool conga-lint -builder-order -builder-order-spec EnableCompletion,Name ./...
main.go:38:3: builder call order violation: EnableCompletion (rank 1) called after Name (rank 2); use -fix to reorder
```

The spec takes effect only together with `-builder-order`. A name that isn't an `App` method stops the run with exit 2 and lists the valid names.

## Know what users see

| Mistake | Without the linter | With `conga-lint` |
|---|---|---|
| A misspelled or unknown tag | Startup error on every run, exit 1 | Reported with the field and a "did you mean" hint |
| A descendant redeclares a parent's flag | Startup error on every run, exit 1 | Reported at the descendant's field |
| A leaf command has no `Run` method | Error only when a user invokes that command, exit 1 | Reported at the struct |
| Builder calls out of order | No effect | Reported with `-builder-order`; `-fix` reorders them |
| `-builder-order-spec` names an unknown method | No effect | `invalid builder order spec: unknown App method "Nmae"` and the valid names, exit 2 |
| `-builder-order-spec` without `-builder-order` | No effect | The spec is ignored and no order check runs, exit 0 |

## Where to go next

- [conga-lint](/reference/conga-lint/): every check, the discovery rules, and the flags.
- [Struct tag reference](/reference/struct-tags/): the tags the analyzer validates.
- [Errors and exit codes](/reference/errors/): what each startup error means for the exit code.
