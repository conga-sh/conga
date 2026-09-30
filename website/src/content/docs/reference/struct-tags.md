---
title: Struct tag reference
description: Every struct tag CONGA understands, per construct.
---

CONGA derives the entire CLI schema from struct fields and their tags. Tags are validated at schema initialization: unknown CONGA tags are rejected with a did-you-mean suggestion unless `conga:"allow-unknown-tags"` is set or the tag is on the allowed foreign list. Unknown tags, typos, or tags on incompatible field types fail schema building with `ErrSchema` before `main` does anything. The same checks run statically in CI — see [How to lint CONGA schemas](/how-to/lint-schemas/).

## Tag availability matrix

| Tag | Flag | Arg | Command | ConfigFile | DashArgs/RawArgs |
|-----|:----:|:---:|:-------:|:----------:|:----------------:|
| `name`        | ✅ | ✅ | ✅ | ✅ | ✅ |
| `short`       | ✅ | –  | –  | ✅ | –  |
| `help`        | ✅ | ✅ | –  | ✅ | –  |
| `summary`     | –  | –  | ✅ | –  | –  |
| `default`     | ✅ | ✅ | –  | ✅ | –  |
| `env`         | ✅ | –  | –  | ✅ | –  |
| `aliases`     | ✅ | –  | ✅ | ✅ | –  |
| `group`       | ✅ | –  | ✅ | ✅ | –  |
| `hidden`      | ✅ | –  | ✅ | ✅ | –  |
| `safe`        | ✅ | –  | –  | –  | –  |
| `placeholder` | ✅ | –  | –  | ✅ | –  |
| `multi`       | –  | –  | –  | ✅ (multi) | – |
| `negatable`   | ✅ (bool) | – | – | – | – |
| `sep`         | ✅ (slices) | ✅ (slices) | – | ✅ (multi) | – |
| `xor`         | ✅ | –  | –  | –  | –  |
| `and`         | ✅ | –  | –  | –  | –  |
| `enum`        | ✅ | ✅ | –  | –  | –  |
| `range`       | ✅ | ✅ | –  | –  | –  |
| `len`         | ✅ | ✅ | –  | –  | –  |
| `items`       | ✅ | ✅ | –  | –  | –  |
| `config`      | ✅ | –  | –  | ✅ | –  |
| `predictor`   | ✅ | ✅ | –  | –  | –  |
| `deprecated`  | ✅ | –  | ✅ | ✅ | –  |
| `envprefix`   | –  | –  | ✅ | –  | –  |
| `conga`       | ✅ | ✅ | ✅ | ✅ | ✅ |

Foreign tags (e.g. `json:"..."`, `yaml:"..."`) on schema structs are allowed and ignored. Unknown non-standard tags fail validation with a did-you-mean suggestion.

## Tag sets by construct

### Flags (`OptionalFlag[T]`, `RequiredFlag[T]`, `Counter`)

| Tag | Meaning |
|-----|---------|
| `name` | Override the flag name (default: kebab-case of the field name, e.g. `MaxRetries` → `--max-retries`). |
| `short` | Single-rune short flag (e.g. `short:"n"`). |
| `help` | Help text shown on the help screen. Custom domain types can supply a default via the parameterless `conga.HelpProvider` or the tag-tailored `conga.TagAwareHelpProvider`; an explicit `help` tag wins. |
| `default` | Default value when no other source provides one. Parsed with the same coercion as CLI input, so `default:"10MiB"` works for `conga.ByteSize`. |
| `env` | Environment variable name(s), comma-separated; `env:"-"` disables environment variable binding. The name is used verbatim: `EnvPrefix` is not prepended to an explicit `env` tag. |
| `aliases` | Comma-separated long-name aliases. |
| `group` | Help-screen grouping header: entries sharing a group render under their own titled section (ungrouped entries render first). |
| `hidden` | Hide from help and completion (presence or `hidden:"true"`). |
| `safe` | Control display of the resolved value in the `--why` provenance report; strict bool (`true`/`false` only). Boolean and counter flags default to visible — their value domain can never carry credential material; every other flag type defaults to redacted for values from the CLI, environment, or config files — declared defaults are always displayed, matching `--help`. `safe:"false"` forces redaction on a boolean or counter flag. Rejected on config-file flags and positional arguments. |
| `xor` | Mutually exclusive group name. |
| `and` | And group name whose members must all be set together. |
| `enum` | Comma-separated allowed values. |
| `range` | Numeric/duration bound (see [Bounds syntax](#bounds-syntax)). |
| `len` | Length bound for strings/slices/maps. |
| `items` | Element count bound for slices/maps. |
| `sep` | Slice separator (default `","`; `sep:"none"` disables splitting entirely, docker-run style). |
| `negatable` | Boolean only: emit `--[no-]flag` form (presence or `negatable:"true"`). |
| `placeholder` | Value placeholder in help/syntax. |
| `config` | Config-file key that feeds the flag (defaults to the flag name); `config:"-"` excludes the field from config resolution entirely. |
| `deprecated` | Optional deprecation notice/reason; hidden from completion. Only valid on optional flags (rejected on `RequiredFlag[T]`). |
| `predictor` | Completion predictor (see [Predictors](#predictors)). |
| `conga` | Directives (see [`conga` directives](#conga-directives)). |

### Positional arguments (`OptionalArg[T]`, `RequiredArg[T]`)

| Tag | Meaning |
|-----|---------|
| `name` | Argument placeholder (default: `<kebab-field-name>`). |
| `help` | Help text. |
| `default` | Default value for optional arguments. |
| `enum` | Comma-separated allowed values. |
| `range` | Numeric/duration bound. |
| `len` | Length bound. |
| `items` | Element count bound. |
| `sep` | Slice separator (default `","`). |
| `predictor` | Completion predictor. |
| `conga` | Directives. |

Arguments are pure CLI input, resolved from the command line; the `env` and `config` tags apply to flags.

### Subcommands (`Command[T]`)

| Tag | Meaning |
|-----|---------|
| `name` | Override the subcommand name (default: kebab-case of the field name). |
| `summary` | Optional 1-line synopsis shown in the parent's command list and completion. |
| `aliases` | Comma-separated command aliases. |
| `group` | Help-screen grouping header: subcommands sharing a group render under their own titled section (ungrouped commands render first) and are tagged as `<group> commands` in completion. |
| `hidden` | Hide from help and completion. |
| `envprefix` | Environment variable prefix for this command's flags. |
| `deprecated` | Optional deprecation notice/reason. |
| `conga` | Directives. |

The `envprefix` tag may also stand alone on a blank field to set the
command's environment prefix without contributing a schema element:

```go
type Server struct {
	_       struct{}                          `envprefix:"SERVER_"`
	Timeout conga.OptionalFlag[time.Duration] `help:"Request timeout"`
}
```

### Config file flags (`ConfigFile`)

| Tag | Meaning |
|-----|---------|
| `name` | Flag name. |
| `short` | Short rune. |
| `help` | Help text. |
| `env` | Environment variable name(s); `env:"-"` disables environment variable binding. |
| `aliases` | Long-name aliases. |
| `placeholder` | Value placeholder (default `PATH`). |
| `group` | Help-screen grouping header: entries sharing a group render under their own titled section (ungrouped entries render first). |
| `hidden` | Hide from help and completion. |
| `multi` | Multi-value arity: repeated `--config-file` and `sep`-split values accumulate (presence or `multi:"true"`). |
| `sep` | Separator for multi-value flags and environment variables (default `","`). |
| `config` | Config-file key that feeds the flag; `config:"-"` disables config binding. |
| `deprecated` | Optional deprecation notice/reason; hidden from completion. |
| `conga` | Directives. |

The `default` tag is not accepted on a `ConfigFile` flag: a default path would be neither loaded nor validated. Search locations are declared with [`config.Paths(...)` or `config.Explicit(...)`](/reference/config/), and `placeholder` sets the value displayed in help (default `PATH`). Without a `help` tag the flag renders `Path to configuration file` (`Path to configuration file(s)` for `multi:"true"`; the engine appends a ` (no configuration decoder registered)` suffix when no decoder is registered). The candidates evaluated for an invocation are reported by `session.CandidateConfigFiles()` and the [`--why` report](/how-to/inspect-value-provenance/). See [About the config cascade](/explanation/config-cascade/).

### Trailing args (`DashArgs`, `RawArgs`)

| Tag | Meaning |
|-----|---------|
| `name` | Placeholder name. |
| `conga` | Directives. |

## Domain-type tags

Domain types can register the custom struct tags they accept by implementing
`conga.TagSpecProvider`. Registered tags are validated at schema build (key
membership under strict tag validation, value grammar, and combination
exclusions), delivered to the type at decode time via
`conga.ContextAwareTextUnmarshaler`, and enforced during the validation phase
via `conga.SelfValidator`:

```go
// ContextAwareTextUnmarshaler receives the field's declared custom tags and
// the engine's ambient environment (the standard streams configured via
// App.Stdin/App.Stdout) at decode time. The engine prefers it over
// encoding.TextUnmarshaler; types capture the tags on the value for later
// validation. The zero DecodeContext selects no tags and process streams.
type ContextAwareTextUnmarshaler interface {
	UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error
}

// SelfValidator is implemented by custom flag/arg domain types. The engine
// passes a non-nil context derived from RunContext; implementations read
// the tag state captured during decoding.
type SelfValidator interface {
	Validate(ctx context.Context) error
}
```

A user-defined type registers and consumes tags exactly like the built-ins:

```go
type Token string

func (t *Token) TagSpecs() []conga.TagSpec {
	return []conga.TagSpec{{Name: "prefix", Kind: conga.TagKindBool}}
}

func (t *Token) UnmarshalTextWithContext(text []byte, decodeContext conga.DecodeContext) error {
	value := string(text)
	if decodeContext.Tags.Bool("prefix") {
		value = "token:" + value
	}

	*t = Token(value)

	return nil
}

func (t *Token) Validate(ctx context.Context) error { /* read captured state */ }
```

Plain `encoding.TextUnmarshaler` unmarshaling (outside the engine) decodes an
untagged, unconstrained value.

The built-in filesystem domain types register these tags:

| Type | Tag | Meaning |
|------|-----|---------|
| `File` | `existing` | The path must exist and be a regular file (mutually exclusive with `stdout`, `executable`, and `symlink`). |
| `File` | `stdin` | `"-"` reads standard input (mutually exclusive with `stdout`, `executable`, and `symlink`). |
| `File` | `stdout` | `"-"` writes standard output (mutually exclusive with `stdin`, `existing`, `executable`, and `symlink`). |
| `File` | `executable` | The path must be an executable regular file; `$PATH`-relative bare names resolve to their absolute location at parse time (mutually exclusive with `existing`, `stdin`, `stdout`, `symlink`, and `glob`). Completes from `$PATH` executables. |
| `File` | `symlink` | The path must be a symbolic link (mutually exclusive with `existing`, `stdin`, `stdout`, `executable`, and `glob`). Completes as symlinks. |
| `File` | `glob` | Completion-only candidate filter: comma-separated glob patterns (e.g. `glob:"Makefile,Dockerfile"` or `glob:"*.yaml"`) matched against entry names during traversal; directories stay visible for traversal. Mutually exclusive with `executable` and `symlink`. |
| `Dir` | `existing` | The path must exist and be a directory. |

```go
type CLI struct {
	Input  conga.OptionalFlag[conga.File] `name:"input" existing:"true" stdin:"true"`
	Output conga.OptionalFlag[conga.File] `name:"output" stdout:"true"`
	Work   conga.OptionalFlag[conga.Dir]  `name:"work" existing:"true"`
}
```

Validation runs after the full configuration cascade, so only the final
resolved value is checked: a config-provided path that is overridden on the
command line is never validated. Failures are usage errors (exit 2)
classified under `ErrValidation`. Boolean tag values are strict — a typo
such as `existing:"yes"` fails at schema build. String-valued tags
(`conga.TagKindString`) must be non-empty and are validated by the domain
type at schema build: malformed `glob` patterns and patterns containing
path separators are rejected with remediation guidance.

Domain types may also supply a default help description by implementing
`conga.HelpProvider` for a parameterless description, or
`conga.TagAwareHelpProvider` when the description is tailored to the declared
tags (the preferred form when both are implemented):

```go
type HelpProvider interface {
	Help() string
}

// tags carries the field's declared custom tag values (empty when none).
type TagAwareHelpProvider interface {
	Help(tags FieldTags) string
}
```

For example, `File` with `existing:"true" stdin:"true"` renders
`Path to an existing file or '-' for stdin` on help screens and in shell
completion candidate descriptions when the field declares no explicit `help`
tag. The static analyzer (`conga-lint`) validates tag usage for the built-in
tagged types; foreign tagged types degrade conservatively (it may miss
diagnostics, never invent them). The runtime interfaces are catalogued in
[Domain types](/reference/domain-types/).

## Subcommand methods

Subcommand structs may optionally implement these methods to enrich their help
screen (the root command sets the same values via the fluent builders
`app.Description(...)`, `app.Examples(...)`, `app.Footer(...)`, and
`app.Deprecated(...)`):

| Method | Meaning |
|--------|---------|
| `Description() string` | Multi-line help-screen overview; falls back to `summary` when empty. |
| `Examples() string` | Usage examples rendered under an `Examples:` heading with 2-space indentation. |

## `conga` directives

Set on the `conga:"..."` tag, comma-separated:

| Directive | Meaning |
|-----------|---------|
| `ignore` (or `-`) | Exclude the field from the schema entirely. |
| `recurse` | Recursively walk the field's nested struct into the schema. |
| `allow-unknown-tags` | Permit unknown foreign tags on this field without error. On any field of the root struct it also widens to the whole schema (nested recursion included when combined with `recurse`). |

## Allowed foreign tags

These tags are recognized as non-CONGA and pass validation silently:

`json`, `yaml`, `yml`, `toml`, `hcl`, `xml`, `mapstructure`, `envconfig`,
`validate`, `binding`, `gorm`, `db`, `doc`, `example`, `description`

Applications can extend this list at runtime with `app.AllowTags(...)`, which
registers additional custom struct tags to be permitted during strict struct
tag validation:

```go
app.AllowTags("protobuf", "storm")
```

To disable strict tag checking across the entire application, call `app.StrictTagValidation(false)`.

Any other unknown tag triggers a schema error with a closest-match suggestion.

## Bounds syntax

`range`, `len`, and `items` accept:

| Form | Meaning |
|------|---------|
| `"1..100"` | Inclusive min..max. |
| `"..100"` | Max only. |
| `"1.."` | Min only. |
| `"5"` | Exact value. |

- `range` applies to numeric types (`int*`, `uint*` except `uintptr`, `float*`),
  `time.Duration`, and any other domain type whose pointer implements
  `encoding.TextUnmarshaler` with an ordered projection (`Bytes() int64` or a
  numeric underlying kind). Text dialects parse every bound with their own
  `UnmarshalText`, so `ByteSize` accepts `range:"1MiB..1GiB"` alongside
  `range:"1..100"` and `range:"100ms..5s"`; pointer element types and text
  types without an orderable projection are rejected.
- `len` applies to strings, slices, and maps (e.g. `len:"3..20"`).
- `items` applies to slices and maps (e.g. `items:"1..5"`).

## Predictors

`predictor:"..."` accepts:

| Value | Meaning |
|-------|---------|
| `file` | File completion. |
| `file:*.ext` | File completion filtered by comma-separated patterns (e.g. `file:*.go,*.mod`). |
| `dir` | Directory completion. |
| `exec` / `executable` | Executable completion from `$PATH`. |
| `symlink` / `symlinks` | Symlink completion. |
| `none` / `off` | Disable completion for the field. |
| `<name>` | A custom predictor registered via `app.Predictor(name, action)`. |

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
```

```go
type Root struct {
	Deploy conga.Command[Deploy] `summary:"Deploy the application"`
	Config conga.ConfigFile      `short:"c"`
}
```

The configuration resolution itself (explicit file, search tier, cascading
merge) is declared at the application level via `app.ConfigOptions(...)`:

```go
import "github.com/conga-sh/conga/config"

app.ConfigOptions(
	config.Paths("/etc/cloudctl/cloudctl.yaml", "~/.config/cloudctl/cloudctl.yaml"),
	config.Cascading(),
)
```
