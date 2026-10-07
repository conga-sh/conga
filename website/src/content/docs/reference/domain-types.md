---
title: Domain types
description: Built-in filesystem and size types that coerce, validate, and complete themselves.
---

Types in the root `conga` package implement the `Predictable` interfaces — they carry their own completion behaviour alongside coercion and validation. Using them makes flags and arguments complete automatically in all six shells. To author custom flag types, see [How to add custom flag types](/how-to/add-custom-flag-types/).

## Filesystem types

| Type | Coercion / validation | Completes as |
|------|----------------------|--------------|
| `conga.File` | Expands `~` and environment variables; capability tags constrain the path | File paths (or per capability tags) |
| `conga.Dir` | Expands `~` and environment variables; `existing` requires a directory | Directories |

`File` accepts the capability tags `existing`, `stdin`, `stdout`, `executable`, `symlink`, and `glob`; the [struct tag reference](/reference/struct-tags/#domain-type-tags) defines each tag, its mutual exclusions, and the schema-build rules.

```go
type Convert struct {
	Input  conga.OptionalFlag[conga.File] `existing:"true" stdin:"true"`
	Output conga.OptionalFlag[conga.File] `stdout:"true"`
	Tool   conga.OptionalFlag[conga.File] `executable:"true"`
	Recipe conga.OptionalFlag[conga.File] `glob:"*.yaml,Makefile"`
}
```

All filesystem capability checks run during the validation phase on the final resolved value (after the configuration cascade); failures are usage errors (exit 2).

## Content and I/O types

| Type | Behaviour |
|------|-----------|
| `conga.FileContent` | Reads the file into memory during parsing; accepts `-` for stdin; 16 MB limit |
| `conga.ByteSize` | Parses `"10MiB"`, `"1.5GB"`, `"500KiB"`, and exact bytes; `Bytes()` returns `int64`; strict case-sensitive units. Renders in the unit family it was parsed in (echo), exact bytes included, or the canonical IEC model for constructed values |

```go
type Upload struct {
	Payload conga.FileContent
	Budget  conga.ByteSize `default:"10MiB"`
}

func (u *Upload) Run(ctx context.Context, session *conga.Session) error {
	limit := u.Budget.Bytes() // 10485760
	...
}
```

### Byte size formatting

`ByteSize` remembers the unit family a value was parsed in, so it renders back in that family (a value decoded from `"1.5GB"` prints `1500MB`); a bare count or `B`-suffixed input is exact bytes and prints as exact bytes (`"1048576B"` prints `1048576B`); values built with `conga.NewByteSize` use the canonical IEC integer model. Text input is non-negative: negative values exist only programmatically, render with a leading `-`, and do not parse back. Three struct tags tune acceptance and display:

| Tag | Values | Effect |
|-----|--------|--------|
| `units` | `binary`, `decimal`, `both` (default) | Restricts the accepted unit families; a parsed unit outside the allowed family is a coercion error. Exact byte counts (`B` suffix or bare) are family-neutral and accepted by both. |
| `display` | `binary`, `decimal`, `input` (default) | Forces the render family; `input` echoes the entered unit family. |
| `precision` | `0`–`3` (default `0`) | Emits exact fractions up to that many decimals (e.g. `1.5GB`); fractions are emitted only when they round-trip exactly. |

`ByteSize` is not comparable: `==` between two values and `ByteSize`-keyed maps are compile errors, because the render family and precision are not part of a size's identity. Compare with `Equal`, order with `Compare`, and calculate with `Bytes()`.

```go
type Cache struct {
	Memory conga.ByteSize `units:"binary" display:"input" precision:"1"`
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

## Default help text

An explicit `help:"..."` tag replaces the type-derived description. Without one, these types describe themselves in the help screen:

| Field type | Declared tags | Default help |
|------------|---------------|--------------|
| `conga.File` | – | `Path to a file` |
| `conga.File` | `existing:"true"` | `Path to an existing file` |
| `conga.File` | `stdin:"true"` | `Path to a file or '-' for stdin` |
| `conga.File` | `stdout:"true"` | `Path to a file or '-' for stdout` |
| `conga.File` | `executable:"true"` | `Path to an executable` |
| `conga.File` | `symlink:"true"` | `Path to a symbolic link` |
| `conga.File` | `glob:"<patterns>"` | `Path to a file matching '<patterns>'` |
| `conga.Dir` | – | `Path to a directory` |
| `conga.Dir` | `existing:"true"` | `Path to an existing directory` |
| `conga.FileContent` | – | `Path to a file (or '-' for stdin)` |
| `conga.ByteSize` | – | `Byte size (e.g., 10MiB, 1.5GB, 500KiB)` |
| `conga.ByteSize` | `units:"binary"` | `Byte size in binary units (e.g., 10KiB, 512MiB)` |
| `conga.ByteSize` | `units:"decimal"` | `Byte size in decimal units (e.g., 10kB, 512MB)` |

`time.Duration` renders `Duration (e.g., 300ms, 2m45s, 1h)`. The container types delegate to their value type, so `OptionalFlag[conga.File]` and `RequiredArg[conga.Dir]` use the descriptions above. After the description, the engine appends `[default: ...]`, `[env: ...]`, constraint hints such as `[1..65535]` or `[items: 2..3]`, and `[enum: ...]` where the field declares them. Referencing the matching `{{...}}` variable in the field's `help` tag suppresses that automatic hint (see [Help variables](/reference/struct-tags/#help-variables)).

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
