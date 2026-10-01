---
title: Struct tag reference
description: Every struct tag CONGA understands, per construct.
---

CONGA derives the CLI schema from struct fields and their tags. The engine validates tags at schema initialization: unknown tags, typos, or tags on incompatible field types fail schema construction with `ErrSchema` (see [How to lint CONGA schemas](/how-to/lint-schemas/)). Recognized foreign tags (such as `json` or `yaml`) pass validation silently.

## Flags (`OptionalFlag[T]`, `RequiredFlag[T]`, `Counter`)

| Tag | Optional | Required | Counter | Meaning |
|-----|:--------:|:--------:|:-------:|---------|
| `aliases` | ✅ | ✅ | – | Comma-separated long-name aliases. |
| `and` | ✅ | ✅ | – | Required co-occurrence group name whose members must all be set together. |
| `config` | ✅ | ✅ | – | Config file key feeding the flag (default: flag name). `config:"-"` excludes the field from config files. |
| `conga` | ✅ | ✅ | ✅ | Directives (see [`conga` directives](#conga-directives)). |
| `default` | ✅ | – | – | Default value when no other source provides one. Coerced with the same parser as CLI input. |
| `deprecated` | ✅ | – | ✅ | Deprecation notice; suppresses flag from completion. |
| `enum` | ✅ | ✅ | – | Comma-separated allowed values. |
| `env` | ✅ | ✅ | – | Comma-separated environment variable names. `env:"-"` disables environment binding. Explicit names ignore `envprefix`. |
| `group` | ✅ | ✅ | ✅ | Help-screen grouping header. Flags sharing a group render under their own titled section. |
| `help` | ✅ | ✅ | ✅ | Help text displayed on the help screen. Supports [help variables](#help-variables). |
| `hidden` | ✅ | ✅ | ✅ | Suppress from help and completion (`hidden` or `hidden:"true"`). |
| `items` | ✅ | ✅ | – | Element count bound for slices or maps (see [Bounds syntax](#bounds-syntax)). |
| `len` | ✅ | ✅ | – | Length bound for strings, slices, or maps. |
| `name` | ✅ | ✅ | ✅ | Flag name override (default: kebab-case of field name, e.g. `MaxRetries` → `--max-retries`). |
| `negatable` | ✅ (bool) | – | – | Boolean flags only: enables `--[no-]flag` syntax (`negatable` or `negatable:"true"`). |
| `placeholder` | ✅ | ✅ | – | Argument placeholder displayed in syntax and help. |
| `predictor` | ✅ | ✅ | – | Completion predictor identifier (see [Predictors](#predictors)). |
| `range` | ✅ | ✅ | – | Numeric or duration bounds (see [Bounds syntax](#bounds-syntax)). |
| `safe` | ✅ | ✅ | ✅ | Controls value display in `--why` provenance (`true` or `false`). Boolean and counter flags default to `true`; other flags default to `false`. Declared defaults are always visible. |
| `sep` | ✅ (slices) | ✅ (slices) | – | Slice separator (default `","`). `sep:"none"` disables splitting, preserving the unparsed string. |
| `short` | ✅ | ✅ | ✅ | Single-rune short flag (e.g. `short:"n"`). |
| `xor` | ✅ | ✅ | – | Mutually exclusive group name. |

## Positional arguments (`OptionalArg[T]`, `RequiredArg[T]`)

Positional arguments bind to command-line input.

| Tag | Optional | Required | Meaning |
|-----|:--------:|:--------:|---------|
| `conga` | ✅ | ✅ | Directives (see [`conga` directives](#conga-directives)). |
| `default` | ✅ | – | Default value for optional arguments. |
| `enum` | ✅ | ✅ | Comma-separated allowed values, rendered in help as `[enum: …]`. |
| `help` | ✅ | ✅ | Help text. Supports [help variables](#help-variables). |
| `items` | ✅ | ✅ | Element count bound for slices or maps. |
| `len` | ✅ | ✅ | Length bound for strings, slices, or maps. |
| `name` | ✅ | ✅ | Argument placeholder in usage syntax (default: `<kebab-field-name>`). |
| `predictor` | ✅ | ✅ | Completion predictor identifier (see [Predictors](#predictors)). |
| `range` | ✅ | ✅ | Numeric or duration bounds. |
| `sep` | ✅ (slices) | ✅ (slices) | Slice separator (default `","`). |

## Help variables

CONGA appends declared constraints after each field's help text: `range:"1..65535"` renders `[1..65535]`, `enum:"dev,prod"` renders `[enum: dev, prod]`, and `default:"8080"` displays in the flag's value slot. A `help` tag on a flag, positional argument, or config-file flag can inline those values using `{{...}}` variables, suppressing the matching automatic hint so the constraint appears once.

| Variable | Substituted text | Supported fields |
|----------|------------------|------------------|
| `{{range}}` | The `range` tag's bounds without brackets (e.g. `1..65535`, `>= 5`). | Flags and positional arguments with a `range` tag. |
| `{{len}}` | The `len` tag's bounds without brackets. | Flags and positional arguments with a `len` tag. |
| `{{items}}` | The `items` tag's bounds without brackets. | Flags and positional arguments with an `items` tag. |
| `{{enum}}` | The `enum` values joined with `, `. | Flags and positional arguments with an `enum` tag. |
| `{{default}}` | The `default` tag's value. | Flags and positional arguments with a `default` tag. |
| `{{env}}` | The `env` variable names joined with `, `. | Flags and config-file flags that bind environment variables (via `env` tag or `envprefix`). Positional arguments never bind environment variables. |

```go
type CLI struct {
	Port conga.OptionalFlag[int]      `range:"1..65535" help:"Port number ({{range}})"`
	Env  conga.OptionalFlag[string]   `enum:"dev,staging,prod" help:"Environment ({{enum}})"`
	Tags conga.OptionalFlag[[]string] `items:"1..5" help:"Tags ({{items}} allowed)"`
}
```

The help column renders `Port number (1..65535)`, `Environment (dev, staging, prod)`, and `Tags (1..5 allowed)`.

Suppression applies per hint: referencing `{{range}}` leaves `[len: …]`, `[items: …]`, `[enum: …]`, `[default: …]`, and `[env: …]` in place. On flags, referencing `{{default}}` also removes the default from the value slot (`--port=8080` becomes `--port=INT`), so the default appears only where the help text places it. On positional arguments, `{{default}}` inlines the declared default into the prose; arguments never render value-slot defaults or automatic bracket hints. Config-file flags support only `{{env}}`.

Referencing an undeclared variable fails schema build with `ErrSchema`: `{{range}}` without a `range` tag, any variable other than `{{env}}` on a config-file flag, or `{{env}}` on a positional argument. Unknown names fail the same way and list the six supported names. Empty constraint tags (`range:""`, `len:""`, `items:""`, and `enum:""`) are rejected at schema build unconditionally. Bare `{` and shell-style `${...}` sequences copy verbatim; double the opening braces (`{{{{`) to render a literal `{{`.

## Subcommands (`Command[T]`)

| Tag | Meaning |
|-----|---------|
| `aliases` | Comma-separated command aliases. |
| `conga` | Directives (see [`conga` directives](#conga-directives)). |
| `deprecated` | Deprecation notice displayed in help. |
| `envprefix` | Environment variable prefix for this command's flags. |
| `group` | Help-screen grouping header. Subcommands sharing a group render under their own titled section and are tagged as `<group> commands` in completion. |
| `hidden` | Suppress from help and completion (`hidden` or `hidden:"true"`). |
| `name` | Subcommand token override (default: kebab-case field name). |
| `summary` | One-line synopsis displayed in parent command listings and shell completion. |

The `envprefix` tag may stand alone on a blank field to set the command's environment prefix without contributing a schema element:

```go
type Server struct {
	_       struct{}                          `envprefix:"SERVER_"`
	Timeout conga.OptionalFlag[time.Duration] `help:"Request timeout"`
}
```

## Config file flags (`ConfigFile`)

| Tag | Meaning |
|-----|---------|
| `aliases` | Comma-separated long-name aliases. |
| `config` | Configuration file key feeding the flag. `config:"-"` disables configuration binding. |
| `conga` | Directives (see [`conga` directives](#conga-directives)). |
| `deprecated` | Deprecation notice; suppresses flag from completion. |
| `env` | Comma-separated environment variable names. `env:"-"` disables environment binding. |
| `group` | Help-screen grouping header. Flags sharing a group render under their own titled section. |
| `help` | Help text (default: `Path to configuration file`, or `Path to configuration file(s)` when `multi:"true"`). |
| `hidden` | Suppress from help and completion (`hidden` or `hidden:"true"`). |
| `multi` | Enable multi-value arity for repeated flags and slice values (`multi` or `multi:"true"`). |
| `name` | Flag name (default: `--config`). |
| `placeholder` | Value placeholder in help (default: `PATH`). |
| `sep` | Multi-value flags only: value separator for repeated flags and environment variables (default `","`). |
| `short` | Single-rune short flag. |

The `default` tag is rejected on `ConfigFile` flags. Declare default search paths using [`config.Paths(...)` or `config.Explicit(...)`](/reference/config/).

## Trailing arguments (`DashArgs`, `RawArgs`)

`DashArgs` and `RawArgs` accept:

| Tag | Meaning |
|-----|---------|
| `conga` | Directives (see [`conga` directives](#conga-directives)). |
| `name` | Placeholder name in usage syntax. |

## Domain-type tags

Domain types can register custom tags they accept by implementing `conga.TagSpecProvider`. To create custom tagged types, see [How to add custom flag types](/how-to/add-custom-flag-types/).

The built-in filesystem domain types register these tags:

### `conga.File`

| Tag | Meaning |
|-----|---------|
| `executable` | Path must be an executable regular file. Bare names resolve via `$PATH` to their absolute location at parse time. Completes from `$PATH`. Mutually exclusive with `existing`, `stdin`, `stdout`, `symlink`, and `glob`. |
| `existing` | Path must exist and be a regular file. Mutually exclusive with `stdout`, `executable`, and `symlink`. |
| `glob` | Comma-separated glob patterns filtering completion candidates (e.g. `glob:"*.yaml,Makefile"`). Patterns match entry names; malformed patterns and patterns containing path separators fail at schema build. Mutually exclusive with `executable` and `symlink`. |
| `stdin` | `"-"` reads standard input. Mutually exclusive with `stdout`, `executable`, and `symlink`. |
| `stdout` | `"-"` writes standard output. Mutually exclusive with `stdin`, `existing`, `executable`, and `symlink`. |
| `symlink` | Path must be a symbolic link. Completes as symlinks. Mutually exclusive with `existing`, `stdin`, `stdout`, `executable`, and `glob`. |

The `executable` and `symlink` capabilities are complete value kinds: they exclude each other and every other capability tag, so a field declares exactly one.

### `conga.Dir`

| Tag | Meaning |
|-----|---------|
| `existing` | Path must exist and be a directory. |

```go
type CLI struct {
	Input  conga.OptionalFlag[conga.File] `existing:"true" stdin:"true"`
	Output conga.OptionalFlag[conga.File] `stdout:"true"`
	Work   conga.OptionalFlag[conga.Dir]  `existing:"true"`
}
```

Tag values are strict booleans (`"true"` or `"false"`). Runtime validation behavior and the full interface catalog are in [Domain types](/reference/domain-types/).

## `conga` directives

Set on the `conga:"..."` tag, comma-separated:

| Directive | Meaning |
|-----------|---------|
| `ignore` (or `-`) | Exclude the field from the schema. |
| `recurse` | Recursively inspect nested struct fields into the schema. |
| `allow-unknown-tags` | Permit unknown foreign tags on this field without error. On the root struct, permits unknown tags across the entire schema. |

## Allowed foreign tags

These tags are recognized as external and pass validation silently:

`json`, `yaml`, `yml`, `toml`, `hcl`, `xml`, `mapstructure`, `envconfig`, `validate`, `binding`, `gorm`, `db`, `doc`, `example`, `description`

Register additional allowed tags at runtime with `app.AllowTags(...)`:

```go
app.AllowTags("protobuf", "storm")
```

`app.StrictTagValidation(false)` disables strict tag checking across the application. Any other unknown tag triggers a schema error with a closest-match suggestion.

## Bounds syntax

The `range`, `len`, and `items` tags accept these formats:

| Form | Meaning |
|------|---------|
| `"1..100"` | Inclusive minimum and maximum. |
| `"..100"` | Maximum only. |
| `"1.."` | Minimum only. |
| `"5"` | Exact value. |

Applicability rules:

| Tag | Applicable types | Validation |
|-----|------------------|------------|
| `range` | Numeric types (`int*`, `uint*` except `uintptr`, `float*`), `time.Duration`, and domain types implementing `encoding.TextUnmarshaler` with an ordered projection (`Bytes() int64` or numeric underlying kind). | Bounds parse using the type's own unmarshaler (e.g. `ByteSize` accepts `range:"1MiB..1GiB"`). Pointer element types and unprojected types are rejected. |
| `len` | Strings, slices, and maps. | Asserts character count on strings or entry count on slices and maps. |
| `items` | Slices and maps. | Asserts element count. |

## Predictors

The `predictor:"..."` tag sets completion behavior:

| Value | Meaning |
|-------|---------|
| `file` | File completion. |
| `file:*.ext` | File completion filtered by comma-separated patterns (e.g. `file:*.go,*.mod`). |
| `dir` | Directory completion. |
| `exec` / `executable` | Executable completion from `$PATH`. |
| `symlink` / `symlinks` | Symlink completion. |
| `none` / `off` | Disable completion for the field. |
| `<name>` | Custom predictor registered via `app.Predictor(name, action)`. |

## Examples

```go
type Deploy struct {
	Replicas conga.RequiredFlag[int]           `range:"1..100" help:"Number of replicas"`
	Image    conga.RequiredFlag[string]        `placeholder:"IMAGE" help:"Container image"`
	Tag      conga.OptionalFlag[string]        `enum:"latest,stable,canary" default:"latest"`
	Files    conga.OptionalFlag[[]string]      `sep:";" items:"1..5" predictor:"file"`
	Verbose  conga.Counter                     `short:"v" help:"Increase verbosity"`
	Timeout  conga.OptionalFlag[time.Duration] `range:"100ms..5s" default:"1s"`
	Color    conga.OptionalFlag[bool]          `negatable:"true" default:"true"`
	Region   conga.OptionalFlag[string]        `env:"AWS_REGION" config:"region"`
	DryRun   conga.OptionalFlag[bool]          `xor:"mode"`
	Force    conga.OptionalFlag[bool]          `xor:"mode"`
}

type Root struct {
	Deploy conga.Command[Deploy] `summary:"Deploy the application"`
	Config conga.ConfigFile      `short:"c"`
}
```
