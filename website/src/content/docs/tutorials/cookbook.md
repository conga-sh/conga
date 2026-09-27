---
title: Examples Cookbook
description: Runnable examples from the repository's 27-example collection.
---

Every example below maps to a self-contained, runnable program in the repository's [`_examples/`](https://github.com/conga-sh/conga/tree/main/_examples) directory, each with an end-to-end test suite that also serves as executable documentation. Run one locally:

```sh
go run ./_examples/08-subcommands-basic create web-api -i redis:7 -r 3
```

## Example index

### Basics

| # | Example | Teaches |
|---|---------|---------|
| 01 | [minimal-hello](https://github.com/conga-sh/conga/tree/main/_examples/01-minimal-hello) | `OptionalFlag[T]`, defaults, short flags, `--help` |
| 02 | [flag-primitives](https://github.com/conga-sh/conga/tree/main/_examples/02-flag-primitives) | Primitive types, `time.Duration`, `Counter` (`-vvv`), aliases, flag groups, hidden flags, coercion errors |
| 03 | [negatable-and-maps](https://github.com/conga-sh/conga/tree/main/_examples/03-negatable-and-maps) | Negatable booleans, native `map[K]V` flags with `items` bounds, slice separators (`sep:"none"`, `sep:";"`) |
| 04 | [positional-args](https://github.com/conga-sh/conga/tree/main/_examples/04-positional-args) | `RequiredArg[T]`, `OptionalArg[T]`, trailing slices, `GetOr` fallbacks |
| 05 | [domain-types](https://github.com/conga-sh/conga/tree/main/_examples/05-domain-types) | `File` with `existing`/`stdin`/`stdout`/`symlink` tags, `Dir`, `ByteSize`, custom `TextUnmarshaler` + `SelfValidator` types |

### Constraints and structure

| # | Example | Teaches |
|---|---------|---------|
| 06 | [flag-constraints](https://github.com/conga-sh/conga/tree/main/_examples/06-flag-constraints) | `xor`, `and`, `range` (int, duration, byte size), `len`, `items`, `enum`; full-help rendering on usage errors |
| 07 | [dynamic-validation](https://github.com/conga-sh/conga/tree/main/_examples/07-dynamic-validation) | The `Validator` interface for cross-field checks before any hook runs |
| 08 | [subcommands-basic](https://github.com/conga-sh/conga/tree/main/_examples/08-subcommands-basic) | `Command[T]` routing, aliases, groups, hidden commands, did-you-mean suggestions |
| 09 | [deep-hierarchy-context](https://github.com/conga-sh/conga/tree/main/_examples/09-deep-hierarchy-context) | `session.Root[T]()`, root-down `Command[T].Get()` traversal, `session.Path()`, `session.Name()`; per-command `envprefix` tags |
| 10 | [builtins-help-version](https://github.com/conga-sh/conga/tree/main/_examples/10-builtins-help-version) | Opt-in `HelpCommand`/`VersionCommand` subcommands; `version --json`; `Examples`/`Footer` builders; `Description()`/`Examples()` methods; custom commands rendering through `session.Help`/`Usage`/`Version` |
| 11 | [trailing-args-forwarding](https://github.com/conga-sh/conga/tree/main/_examples/11-trailing-args-forwarding) | `DashArgs` (`--` forwarding) vs `RawArgs` (bare forwarding) |
| 12 | [lifecycle-hooks](https://github.com/conga-sh/conga/tree/main/_examples/12-lifecycle-hooks) | `PreRun → Run → PostRun → Finally`, error short-circuiting, guaranteed teardown |

### Configuration

| # | Example | Teaches |
|---|---------|---------|
| 13 | [twelve-factor-precedence](https://github.com/conga-sh/conga/tree/main/_examples/13-twelve-factor-precedence) | The 4-tier ladder, `EnvPrefix`, `env:"-"`, `placeholder`, `config:"-"` |
| 14 | [multiformat-and-dotenv](https://github.com/conga-sh/conga/tree/main/_examples/14-multiformat-and-dotenv) | `.env` + `.env.local`, JSON/TOML/YAML/HCL decoders, strict validation with typo hints |
| 15 | [cascading-searchpaths](https://github.com/conga-sh/conga/tree/main/_examples/15-cascading-searchpaths) | `app.ConfigOptions(config.Cascading())` system→user→local merging, partial cascades, explicit `-c` override |
| 16 | [live-config-watcher](https://github.com/conga-sh/conga/tree/main/_examples/16-live-config-watcher) | Daemon hot-reload with `watch.Config`/`watch.File`, SIGHUP reload via `session.Reload[T]`, graceful signal shutdown |
| 17 | [config-profiles](https://github.com/conga-sh/conga/tree/main/_examples/17-config-profiles) | `ConfigFile` with `multi:"true"` multi-file merging (base + profiles) |
| 18 | [config-searchpaths](https://github.com/conga-sh/conga/tree/main/_examples/18-config-searchpaths) | `app.ConfigOptions(config.Paths(...))` single-file mode, first existing file wins |

### Completion

| # | Example | Teaches |
|---|---------|---------|
| 19 | [shell-completion](https://github.com/conga-sh/conga/tree/main/_examples/19-shell-completion) | Completion opt-in (`app.EnableCompletion()`) in both wiring styles: domain types (`Predictable`, `FileFilterable`, `conga.File`, `conga.Dir`) on `deploy`, the registry (`app.Predictor`, `predictor:"..."`) on `exec`, and the full predict SDK on `logs` |
| 20 | [modular-predictors](https://github.com/conga-sh/conga/tree/main/_examples/20-modular-predictors) | Nested command groups with `system.*` and `network.*` modular domain types |
| 21 | [pluggable-types](https://github.com/conga-sh/conga/tree/main/_examples/21-pluggable-types) | `app.Use` plugins, stdlib types (`timezone.Location`, `syscall.Signal`), `app.RegisterType` + `WithPredictor` for foreign types |
| 24 | [flat-completion-flag](https://github.com/conga-sh/conga/tree/main/_examples/24-flat-completion-flag) | Opt-in flat CLI `--completion <shell>` flag via `app.EnableCompletion()` |
| 25 | [flat-completion-subcommand](https://github.com/conga-sh/conga/tree/main/_examples/25-flat-completion-subcommand) | Flat CLI opting into `CompletionCommand` alongside `EnableCompletion()` |

### Deprecation

| # | Example | Teaches |
|---|---------|---------|
| 22 | [app-deprecation](https://github.com/conga-sh/conga/tree/main/_examples/22-app-deprecation) | `app.Deprecated()` warning banner on every invocation |
| 23 | [member-deprecation](https://github.com/conga-sh/conga/tree/main/_examples/23-member-deprecation) | `deprecated:"..."` tags for flags and subcommands |

### Hosting and reference

| # | Example | Teaches |
|---|---------|---------|
| 26 | [library-embedding](https://github.com/conga-sh/conga/tree/main/_examples/26-library-embedding) | `App.Args`, injected writers, `RunContext`, typed errors + `IsUsageError`, custom `ExitFunc`, `UsageOnError(UsageSilence)`, `DisableHelpFlag`/`DisableVersionFlag` |
| 27 | [kitchen-sink](https://github.com/conga-sh/conga/tree/main/_examples/27-kitchen-sink) | Exhaustive reference: every type, all four decoders over nested subcommands, key remap, map encodings, provenance, schema directives |

## Example: twelve-factor precedence

Adapted from example 13. Provenance resolves as each tier is applied:

```go
type Serve struct {
	Addr conga.OptionalFlag[string] `default:":8080" env:"ADDR" placeholder:"HOST:PORT" help:"Listen address"`
}

func (s *Serve) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, s) // visualizes value + origin badge per field

	return nil
}
```

```sh
$ serve --addr :9000        # ● cli  --addr
$ CLOUDCTL_ADDR=:9000 serve # ● env CLOUDCTL_ADDR
$ serve                     # ● default :8080
```

## Example: cascading configuration

Adapted from example 15. Three tiers merge, later paths override earlier keys:

```go
app := conga.New[Deploy]().
	Name("deployctl").
	RegisterDecoder("yaml", yaml.RawDecoder).
	ConfigOptions(
		config.Paths(
			"/etc/deployctl/deployctl.yaml",
			"~/.config/deployctl/deployctl.yaml",
			"./deployctl.yaml",
		),
		config.Cascading(),
	)
```

## Exit-code contract

The examples assert CONGA's [exit-code contract](../reference/errors/#exit-codes) end to end: `0` on success (including `--help`/`--version`), `1` on a runtime failure returned from `Run` (including config read/parse failures), and `2` on input/usage errors.

## Example helpers

- `_examples/internal/inspector` — renders the resolved configuration of the active command chain with per-field origin badges (`● cli`, `● env VAR`, `● config key`, `· default`, `○ unset`), honouring `NO_COLOR`.
- `_examples/internal/testutil` — shared testscript commands (`exit-status`, `copy-fixture`) used by the example suites.
