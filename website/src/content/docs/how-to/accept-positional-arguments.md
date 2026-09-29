---
title: How to accept positional arguments
description: Define mandatory, optional, and trailing positional arguments, and forward raw tokens.
---

CONGA binds command-line tokens to struct fields using typed argument containers. This guide demonstrates how to define required and optional positional arguments, capture trailing arguments in a slice, and forward unparsed tokens after `--`.

## Define required and optional positional arguments

Declare fields on the command struct using `conga.RequiredArg[T]` and `conga.OptionalArg[T]`:

```go title="main.go"
type CopyCmd struct {
	Source conga.RequiredArg[string] `help:"Source file path"`
	Target conga.OptionalArg[string] `help:"Destination file path"`
}

func (c *CopyCmd) Run(ctx context.Context, session *conga.Session) error {
	destination := c.Target.GetOr("backup/")

	fmt.Fprintf(session.Stdout(), "copying %s to %s\n", c.Source.Get(), destination)

	return nil
}
```

When invoked, CONGA binds tokens in declaration order. If an optional argument is omitted, `GetOr` returns the provided fallback:

```sh
$ copy src.txt
copying src.txt to backup/

$ copy src.txt dest.txt
copying src.txt to dest.txt
```

Declare required positional arguments before optional positional arguments in the struct. Placing an optional argument before a required argument produces a schema error (`ErrSchema`) during schema initialisation.

Positional arguments resolve exclusively from command-line tokens. Unlike flags, positional arguments do not bind to environment variables or config files.

## Capture trailing arguments in a slice

To accept a variable number of positional arguments, declare `conga.RequiredArg[[]T]` (or `conga.OptionalArg[[]T]`) as the final positional argument:

```go title="main.go"
type DeleteCmd struct {
	Mode  conga.RequiredArg[string]   `help:"Deletion mode"`
	Files conga.RequiredArg[[]string] `help:"Target files to delete" name:"<files...>"`
}

func (d *DeleteCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "mode: %s, targets: %v\n", d.Mode.Get(), d.Files.Get())

	return nil
}
```

The engine binds initial tokens to the scalar positional fields (`Mode`) and collects all subsequent tokens into the slice:

```sh
$ delete purge file1.txt file2.txt file3.txt
mode: purge, targets: [file1.txt file2.txt file3.txt]

$ delete purge
delete: error: missing required positional argument <files...>
```

A struct may declare at most one slice positional argument, and it must follow all scalar positional fields. Multiple slice fields in the same struct produce an `ErrSchema` error at startup.

## Forward unparsed arguments after `--`

To pass arguments directly to child processes or external tools without flag parsing, declare a `conga.DashArgs` field:

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

CONGA leaves all tokens appearing after `--` completely unparsed and preserves their order. Flags after `--` are not parsed as flags of the current application:

```sh
$ testrunner ./pkg/... -- -v -run TestUpload
package: ./pkg/..., forwarded: [-v -run TestUpload]
```

To capture trailing unparsed tokens without requiring the `--` delimiter, declare a `conga.RawArgs` field instead.

## Where to go next

- [About flags and args](/explanation/flags-and-args/) — typed container semantics, constraints, and separators.
- [How to declare subcommands](/how-to/declare-subcommands/) — mount subcommands on command structs with `Command[T]`.
- [Struct tag reference](/reference/struct-tags/) — reference catalog for argument metadata and validation tags.
