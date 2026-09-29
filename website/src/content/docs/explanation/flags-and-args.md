---
title: About flags and args
description: Typed containers for flags, positional arguments, counters, maps, slices, and raw forwarding.
---

CONGA models every piece of command-line input as an explicit, typed container field on your command struct.

## Flag containers

| Container | Meaning | Typical tags |
|-----------|---------|--------------|
| `conga.OptionalFlag[T]` | Optional flag with a zero-value fallback | `default`, `env`, `config` |
| `conga.RequiredFlag[T]` | Mandatory flag; missing value is a usage error (exit 2) | `env`, `config` |
| `conga.Counter` | Repeatable counting flag (`-v`, `-vv`, `-vvv`) | `default`, `env` |
| `conga.ConfigFile` | Root-only `--config-file` path flag with explicit loading semantics; `multi:"true"` enables repeatable multi-file values | `default`, `env`, `multi`, `sep` |

```go
type Serve struct {
	Addr    conga.OptionalFlag[string] `default:":8080" help:"Listen address" short:"a"`
	Token   conga.RequiredFlag[string] `help:"Auth token" env:"SERVE_TOKEN"`
	Verbose conga.Counter              `help:"Increase log verbosity" short:"v"`
}
```

`Counter` counts clustered repeats: `-vvv` resolves to `3`. Its `Get()` returns `int`.

## Argument containers

| Container | Meaning |
|-----------|---------|
| `conga.OptionalArg[T]` | Optional positional argument |
| `conga.RequiredArg[T]` | Mandatory positional argument |
| `[]T` (plain slice) | Trailing slice of positional arguments |
| `conga.DashArgs` | Captures everything after `--` (go-test style forwarding) |
| `conga.RawArgs` | Captures every trailing token as-is (docker-style forwarding) |

```go
type Copy struct {
	Source conga.RequiredArg[string] `help:"Source path"`
	Target conga.RequiredArg[string] `help:"Target path"`
	Extra  conga.DashArgs            `help:"Arguments forwarded verbatim after --"`
}
```

Positional arguments are declared in field order. Required arguments come before optional ones; violating that order is a schema error (`ErrSchema`) reported at startup.

## Reading values

Every flag and argument container exposes `Get()`, `GetOr()`, `IsSet()`, `Changed()`, `Source()`, `SourceDetail()`, and `String()`. The guarantees behind `Get()` and the provenance methods are covered in [About reading values](/explanation/reading-values/); the method-per-container matrix lives in [package conga](/reference/conga/#containers).

## Slices, separators, and maps

Slices (`OptionalFlag[[]string]`, `OptionalArg[[]string]`, plain slice args) split values on commas by default:

```go
type Query struct {
	// --tag a,b,c OR --tag a --tag b --tag c
	Tags conga.OptionalFlag[[]string] `help:"Labels" sep:","`
	// sep:"none" disables splitting entirely (docker-run style)
	Env  conga.OptionalFlag[[]string] `help:"Raw env pairs" sep:"none"`
	// custom separator
	Ids  conga.OptionalFlag[[]int]    `help:"Record ids" sep:";"`
}
```

Native Go maps are supported directly as flag types:

```go
type Labels struct {
	// --label key=value --label other=value2
	Labels map[string]string `help:"Key-value labels"`
}
```

For custom map value types, register a parser with `app.RegisterMap` — see [How to add custom types and decoders](/how-to/custom-types-and-decoders/).

## Negatable booleans

Bool flags accept `--no-<name>` to set them to `false` explicitly:

```go
type Build struct {
	Color conga.OptionalFlag[bool] `default:"true" help:"Colour output" negatable`
}
```

```sh
app build --no-color
```

The last occurrence wins: `--color --no-color --color` resolves to `true`.

## Value constraints

Constraints are declared on the container and enforced after coercion, before any lifecycle hook runs:

| Tag | Applies to | Syntax |
|-----|-----------|--------|
| `xor:"a,b"` | Flags | At most one of the named flags may be set (`ErrValidation`) |
| `and:"a,b"` | Flags | All named flags must be provided together (`ErrValidation`) |
| `enum:"a,b,c"` | Scalars & slices | Value must be one of the listed options (`ErrValidation`) |
| `range:"1..65535"` | Ordered scalars | Inclusive bounds; `min..`, `..max`, or an exact value (`ErrValidation`) |
| `len:"1..8"` | Strings & slices | Length bounds in the same syntax (`ErrValidation`) |
| `items:"0..100"` | Slices & maps | Per-item or per-entry bounds (`ErrValidation`) |

```go
type Deploy struct {
	Port    conga.OptionalFlag[int]    `default:"8080" range:"1..65535" help:"Listen port"`
	Level   conga.OptionalFlag[string] `default:"info" enum:"debug,info,warn,error" help:"Log level"`
	Workers conga.OptionalFlag[int]    `default:"4" range:"1..64" help:"Worker count"`
}
```

## Help-screen ergonomics

- `help:"..."` — description line.
- `placeholder:"HOST"` — value placeholder in usage lines (otherwise derived from the type).
- `group:"Server"` — visual grouping on the help screen.
- `hidden` — functional but omitted from help and completion.
- `aliases:"listen,bind"` — extra long names.
- `deprecated:"use --addr instead"` — emits a warning on use and hides the flag from completion.
- `safe:"true"` — opt in to showing the resolved value in the [`--why` provenance report](/reference/builtins/#the---why-report); values are redacted by default, keeping secrets out of the report.
