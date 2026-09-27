---
title: package conga
description: Core API — App builder, containers, session accessors, extensibility, and errors.
---

Deep detail lives on [pkg.go.dev/github.com/conga-sh/conga](https://pkg.go.dev/github.com/conga-sh/conga); this page is the curated map.

## The App builder

```go
func New[T any](root ...*T) *App[T]
```

`App[T]` wraps your command struct. `New` defers instance allocation to execution; pass your own instance (`conga.New(cli)`) to keep a handle for pre-run seeding and post-run inspection. All builder methods return `*App[T]` for chaining.

### Configuration methods

| Method | Purpose |
|--------|---------|
| `Name(name)` | Binary name (defaults to `os.Args[0]`) |
| `Description(desc)` | Root description |
| `Examples(examples)` | Usage examples on `--help` |
| `Footer(footer)` | Footer rendered on every help screen |
| `Deprecated(reason)` | Whole-app deprecation banner |
| `Version(version)` | `--version` display string (falls back to build metadata) |
| `EnvPrefix(prefix)` | Environment tier prefix (normalized to trailing `_`) |
| `Dotenv(paths...)` | Enable `.env` loading (default `.env` + `.env.local`) |
| `ConfigOptions(options...)` | Config resolution: `config.Explicit(path)`, `config.Paths(paths...)`, `config.Cascading()` |
| `RegisterDecoder(ext, decoder)` | Per-app config format decoder |
| `StrictTagValidation(strict)` | Toggle tag linting (default on) |
| `AllowTags(tags...)` | Whitelist extra struct tags |
| `Predictor(name, p)` | Register named completion predictor (panics on misuse) |
| `Use(plugins...)` | Apply builder plugins |
| `RegisterType[U](parser, opts...)` | Register a parser (and optional predictor) for type `U` |
| `RegisterMap[M](sample, opts...)` | Register a `map[K]V` parser from a sample map |
| `Args(args)` | Custom argument slice (defaults to `os.Args[1:]`) |
| `UsageOnError(mode)` | Error-time usage rendering mode |
| `Stdout(w)` / `Stderr(w)` | Output writers (default `os.Stdout`/`os.Stderr`) |
| `ExitFunc(handler)` | Custom exit handler for `FatalIfError` |
| `EnableWhy()` / `EnableCompletion()` | Opt in to the `--why` report and shell completion |
| `DisableHelpFlag()` / `DisableVersionFlag()` | Remove the `--help`/`--version` flags |

### Execution methods

| Method | Purpose |
|--------|---------|
| `Run()` | Parse + execute with a signal-aware context (`SIGINT`/`SIGTERM`) |
| `RunContext(ctx)` | Execute with your context; panics on a nil context |
| `FatalIfError(err)` | Render error messages and exit (0/1/2 contract) |

### Extensibility functions

```go
func (a *App[T]) RegisterType[U any](parser func(string) (U, error), opts ...RegisterTypeOption) *App[T]
func (a *App[T]) RegisterMap[M ~map[K]V, K comparable, V any](sample M, opts ...RegisterTypeOption) *App[T]
func WithPredictor(p predict.Predictor) RegisterTypeOption
```

`RegisterTypeOption` customizes registrations; `WithPredictor` attaches default completion to a registered type.

## Containers

| Container | Reader surface |
|-----------|----------------|
| `OptionalFlag[T]` / `RequiredFlag[T]` | `Get`, `IsSet`, `Changed`, `Source`, `SourceDetail`, `String` (`GetOr` on optional only) |
| `OptionalArg[T]` / `RequiredArg[T]` | same as flags |
| `Counter` | same as flags, value is `int` |
| `ConfigFile` | root-only config path flag; `multi:"true"` for multi-path (`Path`, `Paths`) |
| `DashArgs` / `RawArgs` | `Get []string`, `GetOr`, `Len`, `IsSet`, `String` |
| `Command[T]` | `Get() *T`, `IsSelected()` |

`HelpCommand` (with `Path []string`), `VersionCommand` (with `JSON bool`), and `CompletionCommand` (with `Shell string`) are the opt-in built-in subcommand containers.

## Lifecycle interfaces

| Interface | Method | Stage |
|-----------|--------|-------|
| `Validator` | `Validate(ctx, session) error` | Cross-field command validation |
| `SelfValidator` | `Validate(ctx) error` | Custom-type self-validation after the cascade (reads tag state captured at decode time) |
| `PreRunner` | `PreRun(ctx, session) error` | Resource initialization |
| `Runner` | `Run(ctx, session) error` | Command body (required) |
| `PostRunner` | `PostRun(ctx, session) error` | Post-success follow-up |
| `Finalizer` | `Finally(ctx, session, err) error` | Guaranteed teardown |
| `HelpProvider` | `Help() string` | Parameterless default help for a value format |
| `TagAwareHelpProvider` | `Help(tags) string` | Tag-tailored default help; preferred when both forms are implemented |
| `DescriptionProvider` / `ExampleProvider` | `Description()` / `Examples()` | Custom command metadata |
| `ContextAwareTextUnmarshaler` | `UnmarshalTextWithContext(text, decodeContext) error` | Domain types needing field tags and/or the configured streams at decode time |

## Session & hierarchy accessors

```go
func (s *Session) Root[T any]() *T
func (s *Session) Reload[T any](ctx context.Context) (*T, error)
func (s *Session) Stdin() io.Reader
func (s *Session) Stdout() io.Writer
func (s *Session) Stderr() io.Writer
func (s *Session) Help()
func (s *Session) Usage()
func (s *Session) Version()
func (s *Session) Path() string
func (s *Session) Name() string
func (s *Session) Args() []string
func (s *Session) CandidateConfigFiles() []string
func (s *Session) LoadedConfigFiles() []string
```

`Root` returns the application struct bound to `T`: the instance passed to
`New`, or the engine-allocated instance. Enclosing command structs are reached
by walking the selected `Command[T]` fields (`root.Cluster.Get()`).

The standard streams are captured when the App starts executing; builder
changes afterwards do not affect the active session. `Reload` re-evaluates
with the current configuration.

`Help`, `Usage`, and `Version` are the same rendering primitives the built-in
`help` and `version` commands use, so a custom command can emit
framework-formatted help, syntax, and version output without pseudo-errors:
`Help` writes the active command's help screen to stdout, `Usage` writes the
syntax summary to stderr, and `Version` writes the version summary to stdout,
falling back to the module version (or `dev`) plus the embedded build metadata
when no version string was configured.

## Domain types

`File`, `FileContent`, `Dir`, `ByteSize` — see [Domain Types](domain-types/).

## Value sources

`ValueSource` constants: `SourceNone`, `SourceDefault`, `SourceConfig`, `SourceEnv`, `SourceCLI`.

## Usage rendering modes

`UsageOnErrorMode`: `UsageSummary` (default), `UsageFull`, `UsageHint`, `UsageSilence`.

## Errors

Base categories (`ErrSchema`, `ErrParse`, `ErrCoerce`, `ErrValidation`, `ErrConfig`) plus the `UsageError` interface and `IsUsageError(err)` classification — catalogued in [Errors & Exit Codes](errors/).

## Configuration package

The config-file authoring surface lives in the `github.com/conga-sh/conga/config`
package: the `app.ConfigOptions` option constructors (`config.Paths`, `config.Explicit`,
`config.Cascading`) and the `config.Decoder` type. See [package config](config/).
