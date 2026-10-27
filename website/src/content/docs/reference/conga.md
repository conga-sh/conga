---
title: package conga
description: Core API — App builder, containers, session accessors, extensibility, and errors.
---

Deep detail lives on [pkg.go.dev/github.com/conga-sh/conga](https://pkg.go.dev/github.com/conga-sh/conga); this page is the curated map.

## The App builder

```go
func New[T any](root ...*T) *App[T]
```

`App[T]` wraps your command struct. `New` defers instance allocation to execution; pass your own instance (`conga.New(cli)`) to inspect bound container values after execution completes — see [How to retain the root struct](/how-to/keep-root-reference/). All builder methods return `*App[T]` for chaining.

`New` is the recommended constructor. The zero value of `App[T]` is also usable: the execution entry points install the same defaults `New` sets, so `var app conga.App[CLI]` behaves like `conga.New[CLI]()`.

### Configuration methods

| Method | Purpose |
|--------|---------|
| `Name(name)` | Binary name (defaults to `os.Args[0]`) |
| `Description(desc)` | Root description |
| `Examples(examples)` | Usage examples on `--help` |
| `HelpFooter(footer)` | Footer rendered on every help screen |
| `Deprecated(reason)` | Whole-app deprecation banner |
| `Version(version)` | `--version` display string (falls back to build metadata) |
| `EnvPrefix(prefix)` | Environment tier prefix (normalized to trailing `_`) |
| `Dotenv(paths...)` | Enable `.env` loading (default `.env` + `.env.local`; [rules](/reference/dotenv/)) |
| `ConfigOptions(options...)` | Config resolution: `conga.ConfigExplicit(path)`, `conga.ConfigPaths(paths...)`, `conga.ConfigCascading()`; `ConfigExplicit` cannot be combined with paths or cascading. A `conga.Bootstrapper` may additionally contribute a document with `Bootstrap.SetConfig` |
| `RegisterDecoder(ext, decoder)` | Per-app config format decoder |
| `AllowTags(tags...)` | Allow additional struct tag keys (matched exactly) |
| `Predictor(name, p)` | Register named completion predictor (panics on misuse) |
| `Use(bindings...)` | Apply builder bindings (`network.RegisterInterface`, `timezone.RegisterLocation`, …) |
| `RegisterType[U](parser, opts...)` | Register a parser (and optional predictor) for type `U` |
| `RegisterMap[M](opts...)` | Register a `map[K]V` parser for a map type with string-compatible keys |
| `Args(args)` | Custom argument slice (defaults to `os.Args[1:]`) |
| `UsageOnError(mode)` | Error-time usage rendering mode |
| `Stdout(w)` / `Stderr(w)` | Output writers (default `os.Stdout`/`os.Stderr`) |
| `SetenvFunc(fn)` | Writer for variables loaded by [Dotenv](/reference/dotenv/) (default `os.Setenv`) |
| `ExitFunc(handler)` | Custom exit handler for `Exit` |
| `EnableWhy(options...)` / `EnableCompletion(options...)` / `EnableFeatures()` | Opt in to the `--why` report, shell completion, and struct-declared feature toggles; pass `conga.WhyOptionJSON()` to `EnableWhy` to enable `--why=json` |
| `DisableHelpFlag()` / `DisableVersionFlag()` | Remove the `--help`/`--version` flags |

### Execution methods

| Method | Purpose |
|--------|---------|
| `Run()` | Parse + execute with a signal-aware context (`SIGINT`/`SIGTERM`) |
| `RunContext(ctx)` | Execute with your context; panics on a nil context |
| `Exit(err)` | Render error messages and exit (0/1/2 contract) |

### Extensibility functions

```go
func (a *App[T]) RegisterType[U any](parser func(string) (U, error), opts ...RegistrationOption) *App[T]
func (a *App[T]) RegisterMap[M ~map[K]V, K ~string, V any](opts ...RegistrationOption) *App[T]
func WithPredictor(p predict.Predictor) RegistrationOption
```

`RegistrationOption` customizes registrations; `WithPredictor` attaches default completion to a registered type.

## Containers

| Container | Reader surface |
|-----------|----------------|
| `OptionalFlag[T]` / `RequiredFlag[T]` | `Get`, `IsSet`, `Changed`, `Source`, `SourceDetail`, `String` (`GetOr` on optional only) |
| `OptionalArg[T]` / `RequiredArg[T]` | same as flags |
| `Counter` | same as flags, value is `int` |
| `ConfigFile` | root-only config path flag; `multi:"true"` for multi-path (`Path`, `Paths`) |
| `Features[T]` | root-only feature declaration; values read through `Get`, plus `IsSet`, `Changed`, `Source`, `SourceDetail`, `String` |
| `DashArgs` / `RawArgs` | `Get []string`, `GetOr`, `Len`, `IsSet`, `String` |
| `Command[T]` | `Get() *T`, `IsSelected()` |

`HelpCommand` (with `Path() []string`), `VersionCommand` (with `JSON() bool`), and `CompletionCommand` (with `Shell() string`) are the opt-in built-in subcommand containers.

## Lifecycle interfaces

| Interface | Method | Stage |
|-----------|--------|-------|
| `Validator` | `Validate(ctx, session) error` | Cross-field command validation |
| `PreRunner` | `PreRun(ctx, session) error` | Resource initialization |
| `Runner` | `Run(ctx, session) error` | Command body (required) |
| `PostRunner` | `PostRun(ctx, session) error` | Post-success follow-up |
| `Finalizer` | `Finally(ctx, session, err) error` | Guaranteed teardown |
| `DescriptionProvider` / `ExampleProvider` | `Description()` / `Examples()` | Custom command metadata |

Value-shape interfaces (`SelfValidator`, `HelpProvider`, `TagAwareHelpProvider`, `ContextAwareTextUnmarshaler`) and the predictor hooks are catalogued in [Domain types](/reference/domain-types/#domain-type-interfaces).

## Session and hierarchy accessors

```go
func (s *Session) Root[T any]() *T
func (s *Session) Stdin() io.Reader
func (s *Session) Stdout() io.Writer
func (s *Session) Stderr() io.Writer
func (s *Session) Help() string
func (s *Session) Usage() string
func (s *Session) Version() string
func (s *Session) Why() string
func (s *Session) WhyJSON() string
func (s *Session) Path() string
func (s *Session) Name() string
func (s *Session) CandidateConfigFiles() []string
func (s *Session) LoadedConfigFiles() []string
```

`Root` returns the application struct bound to `T`: the instance passed to
`New`, or the engine-allocated instance. The type argument `T` must match the
exact root command struct type, or `Root` panics. Enclosing command structs are reached
by walking the selected `Command[T]` fields (`root.Cluster.Get()`).

`Why` and `WhyJSON` render the same value-provenance report as the built-in `--why` flag; they return an empty string when Why is not enabled, and `WhyJSON` also requires `app.EnableWhy(conga.WhyOptionJSON())`.

The App snapshots standard streams at the start of execution for the active session.

`CandidateConfigFiles`/`LoadedConfigFiles` report the configuration sources the run evaluated: file paths for file-based resolution, plus the distinct origin paths the application recorded when a [`conga.Bootstrapper`](/how-to/load-domain-files/) supplies a document with `Bootstrap.SetConfig`. Document scopes recorded with `ConfigDocument.SetScoped` are not part of the reported path.

`Help`, `Usage`, and `Version` are the formatting primitives behind the built-in `help` and `version` commands: `Help` returns the active command's help screen as a string, `Usage` returns the syntax usage summary as a string, and `Version` returns the formatted version summary line as a string, falling back to the module version (or `dev`) plus the embedded build metadata when no version string was configured.

## Domain types

`File`, `FileContent`, `Dir`, `ByteSize` — see [Domain types](/reference/domain-types/).

## Value sources

`ValueSource` constants (`SourceNone`, `SourceDefault`, `SourceConfig`, `SourceEnv`, `SourceCLI`); meanings in [About value precedence](/explanation/value-precedence/#provenance-tracking).

## Usage rendering modes

`UsageOnErrorMode`: `UsageSummary` (default), `UsageFull`, `UsageHint`, `UsageSilence`.

## Errors

Base categories (`ErrSchema`, `ErrParse`, `ErrCoerce`, `ErrValidation`, `ErrConfig`) plus the `UsageError` and `CompletionError` interfaces and `IsUsageError(err)` / `IsCompletionError(err)` classifications — catalogued in [Errors and exit codes](/reference/errors/).

## Provenance options

```go
func WhyOptionJSON() WhyOption

type WhyOption interface {
	applyWhy(settings *whySettings)
}
```

`WhyOption` is a sealed interface produced by option constructors such as `WhyOptionJSON()`. `WhyOptionJSON` enables machine-readable `--why=json` reports on `App.EnableWhy`. By default, `EnableWhy` compiles only the human-readable text table renderer, dead-code eliminating the JSON serialization pipeline unless opted in.

## Completion options

```go
func CompletionEnvPrefix(prefix string) CompletionOption

type CompletionOption interface {
	applyCompletion(settings *completionSettings)
}
```

`CompletionOption` is a sealed interface produced by option constructors such as `CompletionEnvPrefix`. Pass `CompletionEnvPrefix` to `App.EnableCompletion` to override the derived environment-variable prefix (`<APP>_`) that names the completion sideband toggles `<APP>_COMPLETION_DESCRIPTIONS` and `<APP>_COMPLETION_ALIASES`. The prefix is normalized to upper snake case; an empty prefix keeps the application-name derivation, and a prefix that normalizes to a leading digit is rejected at schema build because POSIX shells cannot assign such a variable name.

## Configuration options

The config-file authoring surface lives in the root `conga` package: the
`app.ConfigOptions` option constructors (`conga.ConfigPaths`, `conga.ConfigExplicit`,
`conga.ConfigCascading`) and the `conga.ConfigDecoder` type.
Options are declared as `conga.ConfigOption` values and folded directly into the
engine's resolution settings.

The bootstrap and programmatic document surface:

```go
type Bootstrapper interface {
	Bootstrap(ctx context.Context, b *Bootstrap) error
}

func (b *Bootstrap) Path() string

func (b *Bootstrap) Name() string

func (b *Bootstrap) LookupEnv(key string) (string, bool)

func (b *Bootstrap) Dotenv(paths ...string) error

func (b *Bootstrap) DotenvExplicit(paths ...string) error

func (b *Bootstrap) SetConfig(doc *ConfigDocument)

type ConfigDocument struct{ /* ... */ }

func (d *ConfigDocument) Set(key string, value any, origin string) *ConfigDocument

func (d *ConfigDocument) SetScoped(key string, value any, origin, scope string) *ConfigDocument

func (d *ConfigDocument) SetKey(key, entryKey string, value any, origin string) *ConfigDocument

func (d *ConfigDocument) SetKeyScoped(key, entryKey string, value any, origin, scope string) *ConfigDocument
```

A command struct implementing `Bootstrapper` runs `Bootstrap` in the bootstrap
pass: fields tagged `bootstrap:"true"` bind from `CLI > environment > default`,
the hook runs with those fields populated, and the normal binding pass then
resolves every field (including the tagged ones, now with the config tier) before
`Run`. `Bootstrap.SetConfig`
supplies a document that composes with file resolution: the document is the
highest-precedence contributor within the config tier, overlaying file-resolved
values key by key (`CLI > environment > document > config file > default`). Its
per-entry origins surface in `--why`. `SetScoped` and `SetKeyScoped` add an
optional scope within the origin; value attribution renders it as
`origin › scope` while the Config paths table lists the origin path once.
`SetKey` records one entry inside a map value and merges it key by key with
earlier writes, so a later layer overrides only the map keys it names; a later
`Set` of the same top-level key replaces the whole entry. `Bootstrap.Dotenv`
loads `.env` files through the writer configured by `App.SetenvFunc`;
`Bootstrap.DotenvExplicit` fails on a missing file. A nil document leaves the
tier inactive; errors returned from `Bootstrap` wrap `ErrConfig` and exit 1. See
[How to load domain documents](/how-to/load-domain-files/).
