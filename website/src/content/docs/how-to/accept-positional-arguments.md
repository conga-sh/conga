---
title: How to accept positional arguments
description: Accept mandatory, optional, and trailing positional arguments, and forward raw tokens.
---

This guide demonstrates how to define mandatory and optional positional arguments, capture trailing arguments in a slice, and forward unparsed tokens after `--`.

## Define required and optional arguments

Declare fields on your command struct using CONGA's typed argument containers:

- `conga.RequiredArg[T]` declares a mandatory argument matched by position.
- `conga.OptionalArg[T]` declares an optional argument that falls back to its zero value when omitted.

```go title="main.go"
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CopyCmd struct {
	Source conga.RequiredArg[string] `help:"Source file path"`
	Target conga.OptionalArg[string] `help:"Destination file path"`
}

func (c *CopyCmd) Run(ctx context.Context, session *conga.Session) error {
	destination := "backup/"
	if c.Target.IsSet() {
		destination = c.Target.Get()
	}

	fmt.Fprintf(session.Stdout(), "copying %s to %s\n", c.Source.Get(), destination)

	return nil
}

func main() {
	app := conga.New[CopyCmd]().
		Name("copy").
		Description("File copy command")

	app.FatalIfError(app.Run())
}
```

Invoke the command with both arguments:

```sh
$ go run . src.txt dest.txt
copying src.txt to dest.txt
```

When the optional argument is omitted, `IsSet()` reports `false`:

```sh
$ go run . src.txt
copying src.txt to backup/
```

## Enforce declaration order

Declare required positional arguments before optional positional arguments in the struct. Placing an optional argument before a required argument produces a schema error (`ErrSchema`) at application startup:

```go
type InvalidOrder struct {
	Optional conga.OptionalArg[string] // schema error: optional argument precedes required
	Required conga.RequiredArg[string]
}
```

When a required argument is omitted during invocation, CONGA reports the error and displays command usage:

```sh
$ go run .
copy: error: missing required positional argument <source>

Usage:
  copy <source> [<target>] [flags]

Run 'copy --help' for more information.
```

## Capture trailing arguments in a slice

Use a plain slice `[]T` to capture a variable number of positional arguments following any scalar positional arguments:

```go title="main.go"
type DeleteCmd struct {
	Mode  conga.RequiredArg[string] `help:"Deletion mode"`
	Files []string                  `help:"Target files"`
}

func (d *DeleteCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "mode: %s, targets: %v\n", d.Mode.Get(), d.Files)

	return nil
}
```

All tokens provided after `<mode>` collect into the `Files` slice:

```sh
$ go run . purge file1.txt file2.txt file3.txt
mode: purge, targets: [file1.txt file2.txt file3.txt]
```

## Forward trailing arguments after `--`

Use `conga.DashArgs` to capture all tokens passed after the `--` terminator:

```go title="main.go"
type TestRunner struct {
	Package conga.RequiredArg[string] `help:"Package to test"`
	Extra   conga.DashArgs            `help:"Arguments forwarded verbatim after --"`
}

func (t *TestRunner) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "package: %s, forwarded: %v\n", t.Package.Get(), t.Extra.Get())

	return nil
}
```

CONGA leaves all tokens after `--` unparsed and preserves their exact order:

```sh
$ go run . ./pkg/... -- -v -run TestUpload
package: ./pkg/..., forwarded: [-v -run TestUpload]
```

To capture unparsed tokens without requiring `--`, declare `conga.RawArgs` instead.

## Where to go next

- [About flags and args](/explanation/flags-and-args/) — typed container semantics, constraints, and separators.
- [How to declare subcommands](/how-to/declare-subcommands/) — mount subcommands on command structs with `Command[T]`.
- [Struct tag reference](/reference/struct-tags/) — reference catalog for argument metadata and validation tags.
