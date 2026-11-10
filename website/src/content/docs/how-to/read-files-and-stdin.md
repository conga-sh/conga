---
title: How to read files and standard input
description: Accept file paths that CONGA checks before Run, read standard input and write standard output with -, take a file's contents as a flag's value, accept directories, and accept sizes such as 10MiB.
---

This guide builds a `convert` command that reads text, converts it to upper case, and writes the result. Its flags use CONGA's filesystem and size types, which check the value before `Run` starts.

## Accept a file that must exist

Give the flag the type `conga.File`, one of the [filesystem types](/reference/domain-types/#filesystem-types). The `existing` [tag](/reference/struct-tags/#domain-type-tags) makes CONGA check that the path names a file, and the `stdin` tag lets the user pass `-` to read standard input. `Reader` opens whichever the user chose:

```go title="main.go" hidden={1-10}
package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/conga-sh/conga"
)

type Convert struct {
	Input conga.RequiredFlag[conga.File] `existing:"true" stdin:"true"`
}

func (c *Convert) Run(ctx context.Context, session *conga.Session) error {
	reader, err := c.Input.Get().Reader()
	if err != nil {
		return err
	}
	defer reader.Close()

	text, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	fmt.Fprint(session.Stdout(), strings.ToUpper(string(text)))

	return nil
}

func main() {
	app := conga.New[Convert]().Name("convert")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```sh
$ convert --input notes.txt
HELLO, WORLD
```

```sh
$ echo 'from stdin' | convert --input -
FROM STDIN
```

A path that doesn't name a file is a usage error, exit 2, before `Run` starts:

```sh error={2}
$ convert --input missing.txt
convert: error: stat missing.txt: no such file or directory for flag --input
```

```sh error={2}
$ convert --input kept
convert: error: "kept" is a directory, but a file was expected for flag --input
```

The flag has no `help` tag, so help describes what it accepts, including `-`:

```text no-copy
      --input=FILE  Path to an existing file or '-' for stdin
```

Without the `stdin` tag, CONGA rejects `-` with `"-" requires the stdin or stdout tag on the field for flag --input`. `File` also expands `~` and environment variables in the path. When the app runs inside a test with its own `Stdin`, `Reader` reads that instead of the process's standard input.

## Write to a file or standard output

The `stdout` tag lets `-` mean standard output, and `Writer` creates the file or returns standard output. With `default:"-"`, the result goes to standard output unless the user names a file:

```go title="main.go" ins={3,8-12} {14}
type Convert struct {
	Input  conga.RequiredFlag[conga.File] `existing:"true" stdin:"true"`
	Output conga.OptionalFlag[conga.File] `default:"-" stdout:"true"`
}

func (c *Convert) Run(ctx context.Context, session *conga.Session) error {
	// ...
	writer, err := c.Output.Get().Writer()
	if err != nil {
		return err
	}
	defer writer.Close()

	fmt.Fprint(writer, strings.ToUpper(string(text)))

	return nil
}
```

```sh
$ convert --input notes.txt --output out.txt
```

```sh
$ cat out.txt
HELLO, WORLD
```

`Writer` creates the file, or truncates it if it exists. Closing the writer for `-` leaves standard output open, so `defer writer.Close()` is safe either way. `IsStdio` reports whether the user chose `-`, for a command that should skip a step, such as a backup, when writing to standard output.

## Take a file's contents as the value

When the command needs a file's contents rather than its path, give the flag the type `conga.FileContent`. CONGA reads the file while it reads the command line, and `Bytes` returns the contents. `-` reads standard input, with no tag needed:

```go title="main.go" ins={4,9}
type Convert struct {
	Input  conga.RequiredFlag[conga.File]        `existing:"true" stdin:"true"`
	Output conga.OptionalFlag[conga.File]        `default:"-" stdout:"true"`
	Header conga.OptionalFlag[conga.FileContent] `help:"File whose contents go first" placeholder:"FILE"`
}

func (c *Convert) Run(ctx context.Context, session *conga.Session) error {
	// ...
	fmt.Fprint(writer, string(c.Header.Get().Bytes()))
	fmt.Fprint(writer, strings.ToUpper(string(text)))

	return nil
}
```

```sh
$ convert --input notes.txt --header header.txt
# converted
HELLO, WORLD
```

```sh
$ echo "# from stdin" | convert --input notes.txt --header -
# from stdin
HELLO, WORLD
```

Because CONGA reads the file before `Run`, a missing file is a usage error, exit 2:

```sh error={2}
$ convert --input notes.txt --header missing.txt
convert: error: stat missing.txt: no such file or directory for flag --header
```

`FileContent` reads at most 16 MiB. Use `File` and `Reader` for anything larger. The `placeholder` tag replaces the default placeholder in help, `FILE-CONTENT`.

## Accept a size such as 10MiB

Give the flag the type `conga.ByteSize`. It accepts byte counts with units, and `Bytes` returns the count. A `range` tag bounds it in the same units:

```go title="main.go" ins={3,8,13-15} {9}
type Convert struct {
	// ...
	MaxSize conga.OptionalFlag[conga.ByteSize]    `default:"1MiB" help:"Largest input to accept" range:"1KiB..100MiB"`
}

func (c *Convert) Run(ctx context.Context, session *conga.Session) error {
	// ...
	limit := c.MaxSize.Get().Bytes()
	text, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return err
	}
	if int64(len(text)) > limit {
		return fmt.Errorf("input is larger than %s", c.MaxSize.Get())
	}
	// ...
}
```

Units are case-sensitive and end in `B`. `kB`, `MB`, and `GB` are powers of 1000, and `KiB`, `MiB`, and `GiB` powers of 1024. A unit without the `B` is rejected:

```sh error={2}
$ convert --input notes.txt --max-size 10M
convert: error: unknown byte size unit "M" in "10M" for flag --max-size
```

```sh error={2}
$ convert --input notes.txt --max-size 10B
convert: error: value 10B is out of range [1KiB..100MiB] for flag --max-size
```

A value prints in the unit the user typed, so the command's own error says `1KiB`. That error comes from `Run`, so it exits with status 1:

```sh error={2}
$ convert --input big.txt --max-size 1KiB
convert: error: input is larger than 1KiB
```

[Byte size parsing](/reference/domain-types/#byte-size-parsing) lists every accepted form, and the `units` and `display` tags that restrict the units or fix how a size prints.

## Accept a directory

Give the flag the type `conga.Dir`. With the `existing` tag, CONGA checks that the path names a directory. The file now also imports `os` and `path/filepath`:

```go title="main.go" ins={3,11-17}
type Convert struct {
	// ...
	KeepDir conga.OptionalFlag[conga.Dir]         `existing:"true" help:"Directory to keep a copy of the input in"`
}

func (c *Convert) Run(ctx context.Context, session *conga.Session) error {
	// ...
	if int64(len(text)) > limit {
		return fmt.Errorf("input is larger than %s", c.MaxSize.Get())
	}

	if c.KeepDir.IsSet() {
		copyPath := filepath.Join(c.KeepDir.Get().String(), "input.txt")
		if err := os.WriteFile(copyPath, text, 0o644); err != nil {
			return err
		}
	}
	// ...
}
```

```sh
$ convert --input notes.txt --keep-dir kept
HELLO, WORLD
```

```sh
$ ls kept
input.txt
```

```sh error={2}
$ convert --input notes.txt --keep-dir notes.txt
convert: error: "notes.txt" is a file, but a directory was expected for flag --keep-dir
```

`String` returns the path, with `~` and environment variables expanded.

## Know what users see

| What the user does | Result |
|---|---|
| Names a file that doesn't exist, for `File` with `existing` or for `FileContent` | `stat missing.txt: no such file or directory for flag --input`, exit 2 |
| Names a directory where a file is expected | `"kept" is a directory, but a file was expected for flag --input`, exit 2 |
| Names a file where a directory is expected | `"notes.txt" is a file, but a directory was expected for flag --keep-dir`, exit 2 |
| Names a directory that doesn't exist, for `Dir` with `existing` | `stat nowhere: no such file or directory for flag --keep-dir`, exit 2 |
| Passes `-` to a `File` flag without `stdin` or `stdout` | `"-" requires the stdin or stdout tag on the field for flag --input`, exit 2 |
| Gives a size without the `B` | `unknown byte size unit "M" in "10M" for flag --max-size`, exit 2 |
| Gives a size outside the `range` | `value 10B is out of range [1KiB..100MiB] for flag --max-size`, exit 2 |

The same checks apply when the value comes from an environment variable or a config file.

## Where to go next

- [Domain types](/reference/domain-types/): every filesystem and size type, and how each completes in the shell.
- [Domain-type tags](/reference/struct-tags/#domain-type-tags): `existing`, `stdin`, `stdout`, `glob`, `executable`, and `symlink`, and which combine.
- [How to add custom flag types](/how-to/add-custom-flag-types/): write a type of your own that checks its value the same way.
- [How to test a command](/how-to/test-a-command/): give `-` its input from a test.
