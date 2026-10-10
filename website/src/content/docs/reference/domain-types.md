---
title: Domain types
description: Built-in and topical domain types that coerce, validate, and complete themselves.
---

Types in the root `conga` package and the topical `network` and `system` packages implement `Predictable` interfaces — they carry their own completion behaviour alongside coercion and validation. Using them makes flags and arguments complete automatically in all six shells. To author custom flag types, see [How to add custom flag types](/how-to/add-custom-flag-types/).

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
| `conga.ByteSize` | Parses `"10MiB"`, `"1.5GB"`, `"500KiB"`, and exact bytes; `Bytes()` returns `int64`; strict case-sensitive units. See [Byte size parsing](#byte-size-parsing) and [Byte size rendering](#byte-size-rendering) |

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

### Byte size parsing

`ByteSize` input is strict and case-sensitive. It accepts exact byte counts, SI metric units ending in `B`, and IEC binary units:

| Form | Examples | Meaning |
|------|----------|---------|
| Exact bytes | `1024`, `500B` | Byte counts |
| Decimal | `kB`, `MB`, `GB`, `TB`, `PB` | Powers of 1000 |
| Binary | `KiB`, `MiB`, `GiB`, `TiB`, `PiB` | Powers of 1024 |

The numeric prefix is digits with an optional fraction whose contribution resolves exactly to bytes (`1.5GB`, `0.5GiB`); a sub-byte remainder such as `0.1GiB` is rejected. Naked unit suffixes without `B` (`10G`, `512Mi`) and lowercase casings (`mb`, `kib`) are rejected to prevent ambiguity. `kB` is always 1000 bytes and `KiB` is always 1024; `KB` is rejected as ambiguous because it denotes 1000 bytes in SI usage and 1024 bytes in legacy usage. Text input must be non-negative.

### Byte size rendering

A value renders in its display family: a `display` tag of `binary` or `decimal` forces that family, and `input`, the default, uses the family of the unit the value was decoded from. Rendering picks the largest unit of that family that divides the byte count exactly, or exact bytes when no unit divides it:

| Input | `display` tag | Output |
|-------|---------------|--------|
| `1.5GB` | `input` (default) | `1500MB` |
| `0.5GiB` | `input` | `512MiB` |
| `1024` | `input` | `1024B` |
| `1.000000001GB` | `input` | `1000000001B` |
| `1.5GB` | `binary` | `1500000000B` |
| `1024kB` | `binary` | `1000KiB` |
| `1000KiB` | `decimal` | `1024kB` |
| `1073741824` | `binary` | `1GiB` |
| zero value | any | `0B` |

### Byte size tags

Two struct tags tune acceptance and display:

| Tag | Values | Effect |
|-----|--------|--------|
| `units` | `binary`, `decimal`, `both` (default) | Restricts the accepted unit families; a parsed unit outside the allowed family is a coercion error. Exact byte counts (`B` suffix or bare) are family-neutral and accepted by both. |
| `display` | `binary`, `decimal`, `input` (default) | Forces the render family; `input` echoes the entered unit family. |

An unrecognized tag value fails at schema build, and range bounds parse under the same tags (see [Bounds syntax](/reference/struct-tags/#bounds-syntax)).

```go
type Cache struct {
	Memory conga.ByteSize `units:"binary" display:"input"`
}
```

### Byte size comparison

Compare `ByteSize` values with `Equal`, order with `Compare`, and retrieve raw magnitudes with `Bytes()`. The type is non-comparable via Go's `==` operator to preserve unit-family independence.

### File streaming

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

## Topical domain packages

The domain packages (`network`, `system`, `timezone`) bundle value types, autocompletion predictors, and standard library bindings. Declaring a field with a domain type such as `network.Host` or `system.User` enables automatic coercion and autocompletion. When a field uses standard library types (`*net.Interface`, `*time.Location`), register the corresponding binding with `app.Use(...)`.

| Package | Domain types | Standard library binding |
|---------|--------------|--------------------------|
| `network` | `Host`, `Port`, `HTTPMethod`, `HTTPStatus`, `MIMEType`, `NetInterface` | `network.RegisterInterface` (`*net.Interface`) |
| `system` | `User`, `Group`, `Signal`, `PID`, `Process`, `EnvVar`, `Locale` | `system.RegisterUser` (`*user.User`), `system.RegisterGroup` (`*user.Group`), `system.RegisterSignal` (`syscall.Signal`) |
| `timezone` | – | `timezone.RegisterLocation` (`*time.Location`) |

Each package also exports its predictors as standalone constructors (`network.Hosts()`, `network.NetInterfaces()`, `system.Signals()`, `system.Users()`, `timezone.Timezones()`) for fields declared with plain strings.

```go
type Deploy struct {
	Host   conga.OptionalFlag[network.Host]       `help:"Target host"`
	Port   conga.OptionalFlag[network.Port]       `help:"Target port"`
	User   conga.OptionalFlag[system.User]        `help:"Run as user"`
	Method conga.OptionalFlag[network.HTTPMethod] `help:"HTTP method"`
}

type Legacy struct {
	Iface conga.OptionalFlag[*net.Interface] `help:"Network interface"`
	Zone  conga.OptionalFlag[*time.Location] `help:"Server timezone"`
}

app := conga.New[Legacy]().
	Use(network.RegisterInterface, timezone.RegisterLocation)
```

The engine classifies all domain-type coercion failures under `conga.ErrCoerce`, reporting the flag name and failure reason. Methods that query host state (`network.Host.LookupIP`, `network.NetInterface.Interface`, `system.User.Lookup`, `system.Group.Lookup`) return a descriptive error naming the failed lookup for blank input, or the wrapped OS/network error otherwise.

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
| `network.Host` | – | `Host (hostname, domain, or IP address)` |
| `network.HTTPMethod` | – | `HTTP method (GET, POST, PUT, DELETE, ...)` |
| `network.HTTPStatus` | – | `HTTP status code (100-599, e.g. 200, 404)` |
| `network.MIMEType` | – | `MIME type (e.g. application/json)` |
| `network.NetInterface` | – | `Network interface (e.g. eth0, lo)` |
| `network.Port` | – | `Port (1-65535, or a service name like "http")` |
| `system.EnvVar` | – | `Environment variable name (e.g. PATH)` |
| `system.Group` | – | `System group name` |
| `system.Locale` | – | `Locale identifier (e.g. en_US.UTF-8)` |
| `system.PID` | – | `Process ID (a positive integer)` |
| `system.Process` | – | `Process executable name (e.g. postgres)` |
| `system.Signal` | – | `Signal name or number (e.g. SIGTERM, 9)` |
| `system.User` | – | `System username` |

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
