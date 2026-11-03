---
title: How to accept positional arguments
description: Define required, optional, and trailing positional arguments, and forward raw tokens.
---

CONGA binds command-line tokens to struct fields using typed argument containers. This guide demonstrates how to define required and optional positional arguments, capture trailing arguments in a slice, and forward unparsed tokens after `--`.

Positional arguments are declared on leaf commands. A command that declares `Command[T]` subcommands is a branch and cannot also declare positional arguments; schema build rejects the mix with `ErrSchema`. Declare positionals on the leaf that executes and pass shared context through inherited flags.

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
copy src.txt
```

```text no-copy
copying src.txt to backup/
```

```sh
copy src.txt dest.txt
```

```text no-copy
copying src.txt to dest.txt
```

Declare required positional arguments before optional positional arguments in the struct. Placing an optional argument before a required argument produces a schema error (`ErrSchema`) during schema initialisation.

Positional arguments resolve exclusively from command-line tokens.

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
delete purge file1.txt file2.txt file3.txt
```

```text no-copy
mode: purge, targets: [file1.txt file2.txt file3.txt]
```

```sh
delete purge
```

```text no-copy
delete: error: missing required positional argument <files...>
```

A struct may declare at most one slice positional argument, and it must follow all scalar positional fields. Multiple slice fields in the same struct produce an `ErrSchema` error at startup.

## Forward unparsed arguments after `--`

To pass arguments directly to child processes or external tools without flag parsing, declare a `conga.DashArgs` field:

```go title="main.go"
type TestRunner struct {
	Package conga.RequiredArg[string] `help:"Package to test"`
	Extra   conga.DashArgs
}

func (t *TestRunner) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "package: %s, forwarded: %v\n", t.Package.Get(), t.Extra.Get())

	return nil
}
```

CONGA forwards all tokens appearing after `--` verbatim into `DashArgs` in their original sequence:

```sh
testrunner ./pkg/... -- -v -run TestUpload
```

```text no-copy
package: ./pkg/..., forwarded: [-v -run TestUpload]
```

To capture trailing unparsed tokens without requiring the `--` delimiter, declare a `conga.RawArgs` field instead. `RawArgs` is a leaf-only collector: the command cannot also declare subcommands, and every positional argument declared before it must be a required scalar. Use `DashArgs` when the command also branches; `--` is the explicit boundary.

### Completion for forwarded arguments

Forwarded words are opaque by default: the completion engine offers no candidates, because it cannot know whether the child process expects files, flags, or something else. Declare a predictor when the forwarded arguments have a known domain:

```go title="main.go"
type TestRunner struct {
	Package conga.RequiredArg[string] `help:"Package to test"`
	Extra   conga.DashArgs            `predictor:"file"`
}
```

The tag accepts the same predictor vocabulary as flags and positional arguments (`file`, `file:*.yml`, `dir`, `exec`, `symlink`, `none`, or a registered custom name). `RawArgs` accepts the same tag; its candidates apply from the moment raw mode begins — the first token that cannot bind after every declared required positional is filled. Once raw mode starts, host flags are no longer completed.

## Where to go next

- [About flags and args](/explanation/flags-and-args/) — typed container semantics, constraints, and separators.
- [How to declare subcommands](/how-to/declare-subcommands/) — mount subcommands on command structs with `Command[T]`.
- [Struct tag reference](/reference/struct-tags/) — reference catalog for argument metadata and validation tags.
