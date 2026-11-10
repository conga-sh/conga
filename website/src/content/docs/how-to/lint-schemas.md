---
title: How to lint schemas
description: Run conga-lint to catch schema errors, tag typos, missing Run methods, and flag shadowing before the program runs, allow tags that other code reads, and check builder call order.
---

`conga-lint` finds the schema mistakes the program otherwise reports one at a time at startup, and leaf commands without a `Run` method, which fail only when a user invokes them. This guide lints an example `cloudctl` program with three mistakes, then allows tag keys that other code reads.

## Add the analyzer to your module

Declare `conga-lint` as a [tool dependency](/reference/conga-lint/):

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

```sh error={2-4} wrap
$ go tool conga-lint ./...
main.go:18:2: command "deploy": schema error: duplicate flag: --verbose (already declared by ancestor command "cli")
main.go:19:2: struct DeployCmd field Replicas: schema error: invalid struct tag: unknown struct tag "hlep" (did you mean "help"?). Use app.AllowTags("hlep") to permit external tags
main.go:28:6: leaf command "status" (struct StatusCmd) does not implement conga.Runner (implement Run(context.Context, *conga.Session) error)
```

Running the program shows only the first mistake:

```sh error={2} wrap
$ cloudctl status
cloudctl: error: struct DeployCmd field Replicas: schema error: invalid struct tag: unknown struct tag "hlep" (did you mean "help"?). Use app.AllowTags("hlep") to permit external tags
```

Go's `./...` pattern skips directories that start with `_`, such as `_examples/`, so name those paths explicitly.

:::tip[Cache results with `go vet`]
`go vet -vettool="$(go tool -n conga-lint)" ./...` runs the same analyzer and caches results between runs. `go vet` then runs only this analyzer, not its standard checks.
:::

## Check builder call order

`-builder-order` also checks the builder calls at each `conga.New` against a canonical order. Identity calls such as `Name` come first, then the `Enable*` and `Disable*` toggles, then behavior, then execution. In this example `EnableCompletion` comes before `Name`:

```sh error={2} wrap
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

Add `-diff` to print the change without applying it. `-fix` moves calls only where the swap is safe, and never moves `Use(...)`, which runs plugin code.

With the three mistakes fixed as well, the check passes and prints nothing (exit 0):

```sh
go tool conga-lint -builder-order ./...
```

## Use your team's builder order

`-builder-order-spec` replaces the canonical order with your own. Give it method names separated by commas, or `@path` to read them one per line from a file. Methods the list leaves out can go anywhere. On the fixed example, a spec that puts `EnableCompletion` before `Name` turns the canonical order into a violation:

```sh error={2}
$ go tool conga-lint -builder-order -builder-order-spec EnableCompletion,Name ./...
main.go:38:3: builder call order violation: EnableCompletion (rank 1) called after Name (rank 2); use -fix to reorder
```

The spec takes effect only together with `-builder-order`. A name that isn't an `App` method stops the run with exit 2 and lists the valid names.

## Keep tags that other code reads

CONGA rejects tag keys it doesn't recognize on flag, argument, and subcommand fields, so a misspelled tag fails at startup instead of being ignored. It accepts the keys that common libraries read, such as `json`, `yaml`, `validate`, and `db`. Any other key looks the same to CONGA as a misspelling, so it fails too. The `AllowTags` builder method lists the extra keys CONGA accepts. [Allowed foreign tags](/reference/struct-tags/#allowed-foreign-tags) lists the default keys.

In this variation of the fixed example, `DeployCmd` is also stored with the MongoDB driver, which reads `bson`, and written to a deploy history by the program's own encoder, which reads `history`:

```go title="main.go" {2} ins={3}
type DeployCmd struct {
	Replicas conga.OptionalFlag[int]    `bson:"replicas" default:"3" help:"Number of replicas" history:"replicas"`
	Token    conga.OptionalFlag[string] `help:"API token" history:"-"`
}
```

The analyzer reports the first unknown key on each field:

```sh error={2-3} wrap
$ go tool conga-lint ./...
main.go:18:2: struct DeployCmd field Replicas: schema error: invalid struct tag: unknown struct tag "bson" (did you mean "xor"?). Use app.AllowTags("bson") to permit external tags
main.go:19:2: struct DeployCmd field Token: schema error: invalid struct tag: unknown struct tag "history". Use app.AllowTags("history") to permit external tags
```

The program fails at startup with the first of them, exit 1:

```sh error={2} wrap
$ cloudctl deploy
cloudctl: error: struct DeployCmd field Replicas: schema error: invalid struct tag: unknown struct tag "bson" (did you mean "xor"?). Use app.AllowTags("bson") to permit external tags
```

List both keys in `AllowTags`:

```go title="main.go" {4} ins={5}
func main() {
	app := conga.New[CLI]().
		Name("cloudctl").
		EnableCompletion().
		AllowTags("bson", "history")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

The analyzer now prints nothing (exit 0), and the program runs:

```sh
$ cloudctl deploy
deploying 3 replicas
```

The analyzer reads the keys from string literals in the `AllowTags` call. When the call takes a variable, such as `AllowTags(tags...)`, it can't tell which keys are allowed, so it reports no unknown tags for that app.

`AllowTags` matches keys exactly, so a misspelled `histroy` still fails. The hint names the misspelled key rather than `history`:

```sh error={2} wrap
$ go tool conga-lint ./...
main.go:18:2: struct DeployCmd field Replicas: schema error: invalid struct tag: unknown struct tag "histroy". Use app.AllowTags("histroy") to permit external tags
```

CONGA checks plain fields, which aren't containers, only for its own tags, so a `Note string` field tagged `history:"note"` needs no `AllowTags`.

## Know what users see

| Mistake | Without the linter | With `conga-lint` |
|---|---|---|
| A misspelled or unknown tag | Startup error on every run, exit 1 | Reported with the field and a "did you mean" hint |
| A tag key that other code reads, outside the built-in list | Startup error on every run, exit 1 | Reported unless the key is listed in `AllowTags` |
| A key listed in `AllowTags` | Accepted | Accepted when the call uses string literals |
| A misspelling of a key listed in `AllowTags` | Startup error on every run, exit 1 | Reported, with a hint that names the misspelled key |
| A descendant redeclares a parent's flag | Startup error on every run, exit 1 | Reported at the descendant's field |
| A leaf command has no `Run` method | Error only when a user invokes that command, exit 1 | Reported at the struct |
| Builder calls out of order | No effect | Reported with `-builder-order`, and `-fix` reorders them |
| `-builder-order-spec` names an unknown method | No effect | `invalid builder order spec: unknown App method "Nmae"` and the valid names, exit 2 |
| `-builder-order-spec` without `-builder-order` | No effect | The spec is ignored and no order check runs, exit 0 |

## Where to go next

- [conga-lint](/reference/conga-lint/): every check, the discovery rules, and the flags.
- [Struct tags](/reference/struct-tags/): the tags the analyzer validates.
- [Errors and exit codes](/reference/errors/): what each startup error means for the exit code.
