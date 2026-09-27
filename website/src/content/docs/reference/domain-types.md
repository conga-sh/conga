---
title: Domain Types
description: Built-in filesystem and size types that coerce, validate, and complete themselves.
---

Types in the root `conga` package implement the `Predictable` interfaces — they carry their own completion behaviour alongside coercion and validation. Using them makes flags and arguments complete automatically in all six shells. To author your own, see [How to extend with custom types, maps & decoders](../how-to/custom-types-and-decoders/).

## Filesystem types

| Type | Coercion / validation | Completes as |
|------|----------------------|--------------|
| `conga.File` | Expands `~` and environment variables; capabilities declared through tags | File paths (or per capability tags) |
| `conga.Dir` | Expands `~` and environment variables; `existing` tag requires a directory | Directories |

`File` declares its capabilities with custom field tags:

| Tag | Meaning |
|-----|---------|
| `existing:"true"` | The path must exist and be a regular file. |
| `stdin:"true"` | `"-"` reads standard input. |
| `stdout:"true"` | `"-"` writes standard output. |
| `executable:"true"` | The path must be an executable regular file; `$PATH`-relative names resolve to their absolute location at parse time. Completes from `$PATH` executables. |
| `symlink:"true"` | The path must be a symbolic link. Completes as symlinks. |
| `glob:"<patterns>"` | Completion-only candidate filter: suggested files must match one of the comma-separated glob patterns (matched on entry names, e.g. `glob:"Makefile,Dockerfile"` or `glob:"*.yaml"`). Directories stay visible for traversal. Incompatible with `executable` and `symlink`. |

The `executable` and `symlink` capabilities are complete value kinds: they exclude each other and every other capability tag (`existing`, `stdin`, `stdout`, `glob`), so a field declares exactly one.

```go
type Convert struct {
	Input  conga.OptionalFlag[conga.File] `existing:"true" stdin:"true" help:"File to convert ('-' for stdin)"`
	Output conga.OptionalFlag[conga.File] `stdout:"true" help:"Output path or '-' for stdout"`
	Tool   conga.OptionalFlag[conga.File] `executable:"true" help:"External converter binary"`
	Recipe conga.OptionalFlag[conga.File] `glob:"*.yaml,Makefile" help:"Recipe source"`
}
```

All filesystem capability checks run during the validation phase on the final resolved value (after the configuration cascade); failures are usage errors (exit 2). Tag typos such as `existing:"yes"` fail at schema build, as do malformed `glob` patterns and glob patterns containing path separators (patterns match entry names during traversal, so `src/*.go` can never match).

## Content and I/O types

| Type | Behaviour |
|------|-----------|
| `conga.FileContent` | Reads the file into memory during parsing; accepts `-` for stdin; 16 MB limit |
| `conga.ByteSize` | Parses `"10MiB"`, `"1.5GB"`, `"500KiB"`, exact bytes; `Bytes()` returns `int64`; strict case-sensitive units |

```go
type Upload struct {
	Payload conga.FileContent `help:"Payload file ('-' for stdin)"`
	Budget  conga.ByteSize    `default:"10MiB" help:"Maximum transfer size"`
}

func (u *Upload) Run(ctx context.Context, session *conga.Session) error {
	limit := u.Budget.Bytes() // 10485760
	...
}
```

`File` streams through `Reader()` and `Writer()`, which return `io.ReadCloser` and `io.WriteCloser`. When the value is `-`, `Reader()` wraps `os.Stdin` and `Writer()` wraps `os.Stdout` — the wrappers absorb `Close()`, so callers can always `defer Close()`. `Stat()` exposes the file metadata, and `IsStdio()` reports the `-` case so commands can skip filesystem-only side effects (backups, atomic renames) and route diagnostics away from a stdout payload stream. Reading beyond these helpers is the caller's responsibility (`io.ReadAll(f.Reader())`); `FileContent` is the bounded whole-file shortcut.

```go
func (c *Convert) Run(ctx context.Context, session *conga.Session) error {
	writer, err := c.Output.Get().Writer()
	if err != nil {
		return err
	}
	defer writer.Close()

	reader, err := c.Input.Get().Reader()
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = io.Copy(writer, reader)

	return err
}
```

## Domain type interfaces

| Interface | Method | Purpose |
|-----------|--------|---------|
| `Predictable` | `Predict() predict.Predictor` | Parameterless self-completion candidates; consulted at schema build |
| `TagAwarePredictable` | `Predict(tags FieldTags) predict.Predictor` | Tag-tailored candidates; preferred when both forms are implemented |
| `HelpProvider` | `Help() string` | Parameterless default help description |
| `TagAwareHelpProvider` | `Help(tags FieldTags) string` | Tag-tailored default help; preferred when both forms are implemented |
| `TagSpecProvider` | `TagSpecs() []TagSpec` | Registers the custom field tags the type accepts |
| `ContextAwareTextUnmarshaler` | `UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error` | Decode-time access to the declared tags and configured streams |
| `SelfValidator` | `Validate(ctx context.Context) error` | Post-cascade validation of the resolved value |

Implement exactly one form of each tag-aware pair. Filesystem-derived types can additionally participate in shell-side filtering through `FileFilterable`, `DirFilterable`, and `PathFilterable`.

```go
type ContextAwareTextUnmarshaler interface {
	UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error
}

type SelfValidator interface {
	Validate(ctx context.Context) error
}

type TagSpecProvider interface {
	TagSpecs() []TagSpec
}
```
