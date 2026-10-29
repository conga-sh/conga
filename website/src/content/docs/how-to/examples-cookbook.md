---
title: How to run the examples
description: Run the repository's 31 self-contained example programs and find the one covering a given feature.
---

The CONGA repository provides 31 self-contained example programs under [`_examples/`](https://github.com/conga-sh/conga/tree/main/_examples). Each example implements a focused architecture pattern, accompanied by a dedicated README and an automated end-to-end test suite.

## Run an example locally

Execute any example directly from the repository root:

```sh
go run ./_examples/08-subcommands-basic create web-api -i redis:7 -r 3
```

```text no-copy
╭───────────────────────────────────────────────────────────╮
│  ⚙  service-mgr create                                    │
╰───────────────────────────────────────────────────────────╯

    FIELD              TYPE      VALUE     SOURCE

  service-mgr · root
    -v, --verbose      bool      false     ○ unset
    create             command   —         ▸ selected
    delete (rm, del)   command   —         —
    list               command   —         —
    doctor             command   —         —

  create · active
    <name>             string    web-api   ● cli
    -i, --image        string    redis:7   ● cli --image
    -r, --replicas     int       3         ● cli --replicas
```

To run the automated test suite across all examples:

```sh
go test ./_examples/...
```

## Example index

### Basics

| # | Example | Demonstrates |
|---|---------|--------------|
| 01 | [minimal-hello](https://github.com/conga-sh/conga/tree/main/_examples/01-minimal-hello) | `OptionalFlag[T]`, default values, short flags, and `--help`. |
| 02 | [flag-primitives](https://github.com/conga-sh/conga/tree/main/_examples/02-flag-primitives) | Primitive types, `time.Duration`, `Counter` (`-vvv`), aliases, flag groups, hidden flags, and coercion errors. |
| 03 | [negatable-and-maps](https://github.com/conga-sh/conga/tree/main/_examples/03-negatable-and-maps) | Negatable booleans, native `map[K]V` flags with `items` bounds, and slice separators (`sep:"none"`, `sep:";"`). |
| 04 | [positional-args](https://github.com/conga-sh/conga/tree/main/_examples/04-positional-args) | `RequiredArg[T]`, `OptionalArg[T]`, trailing slices, and `GetOr` fallbacks. |
| 05 | [domain-types](https://github.com/conga-sh/conga/tree/main/_examples/05-domain-types) | `File` with `existing`/`stdin`/`stdout`/`symlink`/`glob` tags, `Dir`, `ByteSize`, `FileContent`, and custom `TextUnmarshaler` types. |

### Constraints and structure

| # | Example | Demonstrates |
|---|---------|--------------|
| 06 | [flag-constraints](https://github.com/conga-sh/conga/tree/main/_examples/06-flag-constraints) | `xor`, `and`, `range` (int, duration, byte size), `len`, `items`, and `enum`. |
| 31 | [oneof-groups](https://github.com/conga-sh/conga/tree/main/_examples/31-oneof-groups) | Exactly-one `oneof:"group"` groups: missing and duplicate selections both exit 2, and the usage synopsis renders the alternation. |
| 16 | [help-variables](https://github.com/conga-sh/conga/tree/main/_examples/16-help-variables) | `{{...}}` help variables that inline a declared constraint instead of the automatic bracket hint, including `{{default}}` value-slot suppression. |
| 07 | [dynamic-validation](https://github.com/conga-sh/conga/tree/main/_examples/07-dynamic-validation) | The `Validator` interface for cross-field semantic validation before hooks execute. |
| 08 | [subcommands-basic](https://github.com/conga-sh/conga/tree/main/_examples/08-subcommands-basic) | `Command[T]` routing, aliases, visual groups, hidden commands, and typo suggestions. |
| 09 | [deep-hierarchy-context](https://github.com/conga-sh/conga/tree/main/_examples/09-deep-hierarchy-context) | `session.Root[T]()`, root-down `Command[T].Get()` traversal, `session.Path()`, `session.Name()`, and `envprefix` tags. |
| 10 | [builtins-help-version](https://github.com/conga-sh/conga/tree/main/_examples/10-builtins-help-version) | Opt-in `HelpCommand`/`VersionCommand` subcommands, `version --json`, `Examples`/`HelpFooter` builders, and custom commands rendering through session helpers. |
| 11 | [trailing-args-forwarding](https://github.com/conga-sh/conga/tree/main/_examples/11-trailing-args-forwarding) | `DashArgs` (`--` forwarding) versus `RawArgs` (bare forwarding). |
| 12 | [lifecycle-hooks](https://github.com/conga-sh/conga/tree/main/_examples/12-lifecycle-hooks) | `PreRun -> Run -> PostRun -> Finally` sequence, error short-circuiting, and guaranteed teardown. |
| 27 | [strict-tag-validation](https://github.com/conga-sh/conga/tree/main/_examples/27-strict-tag-validation) | `app.AllowTags(...)` for schemas carrying foreign struct tags. |
| 29 | [feature-toggles](https://github.com/conga-sh/conga/tree/main/_examples/29-feature-toggles) | `conga.Features[T]` gating flags and whole subcommands through `feature:"..."` expressions, activated from the CLI, environment, or a config section. |

### Configuration

| # | Example | Demonstrates |
|---|---------|--------------|
| 13 | [twelve-factor-precedence](https://github.com/conga-sh/conga/tree/main/_examples/13-twelve-factor-precedence) | The four-tier resolution ladder, `EnvPrefix`, `env:"-"`, and `config:"-"`. |
| 14 | [multiformat-and-dotenv](https://github.com/conga-sh/conga/tree/main/_examples/14-multiformat-and-dotenv) | `.env` and `.env.local` files, JSON/TOML/YAML/HCL decoders, and strict validation. |
| 15 | [cascading-searchpaths](https://github.com/conga-sh/conga/tree/main/_examples/15-cascading-searchpaths) | `app.ConfigOptions(conga.ConfigCascading())` system-to-local merging and explicit `--config-file` overrides. |
| 17 | [config-profiles](https://github.com/conga-sh/conga/tree/main/_examples/17-config-profiles) | `ConfigFile` with `multi:"true"` multi-file merging (base plus profile pattern). |
| 18 | [config-searchpaths](https://github.com/conga-sh/conga/tree/main/_examples/18-config-searchpaths) | `app.ConfigOptions(conga.ConfigPaths(...))` single-file search mode where the first existing file wins. |
| 28 | [explicit-config-file](https://github.com/conga-sh/conga/tree/main/_examples/28-explicit-config-file) | `app.ConfigOptions(conga.ConfigExplicit(...))` fatal-if-missing programmatic target. |
| 30 | [bootstrapper](https://github.com/conga-sh/conga/tree/main/_examples/30-bootstrapper) | `conga.Bootstrapper` breaks the flag-before-config cycle: `-f`/`--pipeline` bind in an early bootstrap pass, `Bootstrap` reads a flag-selected pipeline document, keeps its domain data, and contributes config keys with `Bootstrap.SetConfig` (per-key `--why` origins). |

### Completion

| # | Example | Demonstrates |
|---|---------|--------------|
| 19 | [shell-completion](https://github.com/conga-sh/conga/tree/main/_examples/19-shell-completion) | Opt-in completion via `app.EnableCompletion()`, domain types, predictor registry, and predict SDK. |
| 20 | [modular-predictors](https://github.com/conga-sh/conga/tree/main/_examples/20-modular-predictors) | Command groups using `system.*` and `network.*` domain types. |
| 21 | [pluggable-types](https://github.com/conga-sh/conga/tree/main/_examples/21-pluggable-types) | `app.Use` type bindings (`timezone.RegisterLocation`, `system.RegisterSignal`) for standard library types (`*time.Location`, `syscall.Signal`), and `app.RegisterType`. |
| 24 | [flat-completion-flag](https://github.com/conga-sh/conga/tree/main/_examples/24-flat-completion-flag) | Flat CLI `--completion <shell>` flag via `app.EnableCompletion()`. |
| 25 | [flat-completion-subcommand](https://github.com/conga-sh/conga/tree/main/_examples/25-flat-completion-subcommand) | Flat CLI opting into `CompletionCommand` alongside `EnableCompletion()`. |

### Deprecation and embedding

| # | Example | Demonstrates |
|---|---------|--------------|
| 22 | [app-deprecation](https://github.com/conga-sh/conga/tree/main/_examples/22-app-deprecation) | `app.Deprecated()` warning banner rendered on every invocation. |
| 23 | [member-deprecation](https://github.com/conga-sh/conga/tree/main/_examples/23-member-deprecation) | `deprecated:"..."` struct tags for flags and subcommands. |
| 26 | [library-embedding](https://github.com/conga-sh/conga/tree/main/_examples/26-library-embedding) | `App.Args`, injected `Stdin`/`Stdout`/`Stderr` streams, `RunContext`, `IsUsageError`, and custom `ExitFunc`. |
| 99 | [kitchen-sink](https://github.com/conga-sh/conga/tree/main/_examples/99-kitchen-sink) | Exhaustive reference: all primitive types, filesystem types, four decoders, map encodings, and provenance. |

## Exit code conventions

All example suites assert CONGA's [exit code contract](/reference/errors/#exit-codes): 0 for success (including built-in `--help` and `--version`), 2 for usage errors, and 1 for schema, configuration, and runtime failures.

## Where to go next

- [How to test a command](/how-to/test-a-command/) — test commands in memory using injected arguments and streams.
- [How to declare subcommands](/how-to/declare-subcommands/) — organize hierarchical command trees.
- [About value precedence](/explanation/value-precedence/) — deterministic four-tier resolution ladder.
