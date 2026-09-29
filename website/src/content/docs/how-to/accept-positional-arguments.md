---
title: How to accept positional arguments
description: Define mandatory, optional, and trailing positional arguments, and forward raw tokens.
---

Positional arguments map command-line tokens to struct fields based on their position in the argument vector. Use CONGA's typed argument containers to define required and optional arguments, capture trailing arguments in a slice, and forward unparsed tokens to external commands.

## Define required and optional positional arguments

Declare fields on the command struct using `conga.RequiredArg[T]` and `conga.OptionalArg[T]`:

```go title="main.go"
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
```

CONGA binds positional tokens to struct fields strictly in field declaration order. Required positional arguments must precede optional positional arguments. Placing an optional argument before a required argument produces a schema error (`ErrSchema`) during schema initialisation:

```go
type InvalidOrder struct {
	Optional conga.OptionalArg[string] // schema error: optional argument precedes required
	Required conga.RequiredArg[string]
}
```

Positional arguments resolve exclusively from command-line tokens. Unlike flags, positional arguments do not bind to environment variables or config files.

## Handle missing or unexpected arguments

When a required argument is omitted, CONGA returns an error wrapping `conga.ErrParse`, renders the command usage to stderr, and exits with code 2:

```sh
$ copy
copy: error: missing required positional argument <source>

Usage:
  copy <source> [<target>] [flags]

Run 'copy --help' for more information.
```

If an invocation provides more positional arguments than the command declares, CONGA rejects the extra tokens:

```sh
$ copy src.txt dest.txt extra.txt
copy: error: unexpected argument "extra.txt"
```

## Capture trailing arguments in a slice

To accept a variable number of positional arguments, declare a slice field `[]T` as the final positional argument:

```go title="main.go"
type DeleteCmd struct {
	Mode  conga.RequiredArg[string] `help:"Deletion mode"`
	Files []string                  `help:"Target files to delete"`
}

func (d *DeleteCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "mode: %s, targets: %v\n", d.Mode.Get(), d.Files)

	return nil
}
```

The engine binds initial tokens to the scalar positional fields (`Mode`) and collects all subsequent tokens into the slice:

```sh
$ delete purge file1.txt file2.txt file3.txt
mode: purge, targets: [file1.txt file2.txt file3.txt]
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
