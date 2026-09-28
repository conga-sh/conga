# CONGA Examples

Runnable examples for the CONGA CLI framework. Each numbered directory is a
self-contained, runnable program with an end-to-end testscript suite in
`testdata/` that also serves as executable documentation.

Run any example directly:

```sh
go run ./_examples/08-subcommands-basic create web-api -i redis:7 -r 3
```

Run every example's test suite:

```sh
go test ./_examples/...
```

## The tour

| # | Example | What it teaches |
|---|---------|-----------------|
| 01 | `01-minimal-hello` | Minimal single-command app: `OptionalFlag[T]`, defaults, short flags, `--help` |
| 02 | `02-flag-primitives` | Primitive types, `time.Duration`, `conga.Counter` clustering (`-vvv`), aliases, flag groups, hidden flags, coercion errors |
| 03 | `03-negatable-and-maps` | Negatable booleans (`--[no-]color`, last-flag-wins), native `map[K]V` key-value flags with `items` bounds, slice separators (`sep:"none"`, `sep:";"`) |
| 04 | `04-positional-args` | `RequiredArg[T]`, `OptionalArg[T]`, trailing positional slices, `GetOr` fallbacks |
| 05 | `05-domain-types` | `File` with `existing`/`stdin`/`stdout`/`symlink` tags, `Dir`, `ByteSize`, custom `TextUnmarshaler` + `SelfValidator` domain types (including custom map types); domain types earn file/dir/symlink completion for free |
| 06 | `06-flag-constraints` | `xor`, `and`, `conga.RequiredFlag`, `range` (int, duration, byte size), `len`, `items`, `enum`; full-help rendering on usage errors; exit code 2 |
| 07 | `07-dynamic-validation` | The `conga.Validator` interface for cross-field semantic validation before any hook runs |
| 08 | `08-subcommands-basic` | `Command[T]` routing, command aliases, command groups, hidden commands, did-you-mean suggestions |
| 09 | `09-deep-hierarchy-context` | Deep nesting with `session.Root`, root-down `Command[T].Get()` traversal, `session.Path()`, `session.Name()`; per-command `envprefix` tags (inheritance and override) |
| 10 | `10-builtins-help-version` | Opt-in `conga.HelpCommand` / `conga.VersionCommand` subcommands vs `--help` / `--version` flags; `Examples`/`Footer` builders and the `Description()`/`Examples()` subcommand methods |
| 11 | `11-trailing-args-forwarding` | `conga.DashArgs` (`--` forwarding, go-test style) vs `conga.RawArgs` (bare forwarding, docker style) |
| 12 | `12-lifecycle-hooks` | `PreRun` → `Run` → `PostRun` → `Finally` ordering, error short-circuiting, guaranteed teardown, exit code 1 on runtime errors |
| 13 | `13-twelve-factor-precedence` | The 4-tier precedence ladder (CLI > env > config > default), `EnvPrefix`, `env:"-"`, `placeholder`, `config:"-"` |
| 14 | `14-multiformat-and-dotenv` | `.env` + `.env.local` loading (expansion, quotes), JSON/TOML/YAML/HCL decoders, strict validation with typo hints |
| 15 | `15-cascading-searchpaths` | `app.ConfigOptions(config.Cascading())` system→user→local merging, partial cascades, explicit `-c` override |
| 16 | `16-live-config-watcher` | Daemon hot-reload: `watch.Config`, `watch.File`, SIGHUP reload via `session.Reload`, `session.CandidateConfigFiles`, `session.LoadedConfigFiles`, graceful signal shutdown |
| 17 | `17-config-profiles` | `conga.ConfigFile` with `multi:"true"` multi-file configuration merging (base + profiles, repeatable and comma-list forms) |
| 18 | `18-config-searchpaths` | `app.ConfigOptions(config.Paths(...))` single-file mode where first existing file wins |
| 19 | `19-shell-completion` | Shell completion in both wiring styles: self-completing domain types (`conga.Predictable`, `FileFilterable`, `conga.File`, `conga.Dir`) on `deploy`, the predictor registry (`app.Predictor`, `predictor:"..."` tags) on `exec`, and the full predict SDK playground (every constructor and combinator) on `logs` |
| 20 | `20-modular-predictors` | Nested command grouping with the modular domain types: `system.*` (`User`, `Group`, `Process`, `PID`, `Signal`, `EnvVar`, `Locale`) and `network.*` (`NetInterface`, `Host`, `Port`, `HTTPStatus`, `MIMEType`, `HTTPMethod`) |
| 21 | `21-pluggable-types` | Pluggable type parsers: `app.Use` plugins (`timezone.Location`, `system.Signal`), standard library types (`*time.Location`, `syscall.Signal`), and `app.RegisterType` + `WithPredictor` for foreign types |
| 22 | `22-app-deprecation` | Whole-command deprecation: `app.Deprecated()` emits a warning banner on every invocation (root, subcommands, `--help`) |
| 23 | `23-member-deprecation` | Member deprecation: `deprecated:"..."` struct tags for flags and subcommands, exclusion from completion |
| 24 | `24-flat-completion-flag` | Flat CLI: defaults to the `--completion <shell>` flag; `--help` omits the `Commands:` section |
| 25 | `25-flat-completion-subcommand` | Flat CLI opting into `conga.CompletionCommand`: generates `completion` subcommand and omits `--completion` flag |
| 26 | `26-library-embedding` | Embedding CONGA as a library: `App.Args`, injected `Stdout`/`Stderr` writers, `RunContext`, typed errors + `IsUsageError`, custom `Exit` handler, `UsageOnError(UsageSilence)`, `Disable{Help,Version,Why,Completion}` |
| 27 | `27-kitchen-sink` | The exhaustive reference: every type width and filesystem/value domain type, all four decoder formats over descriptive nested subcommands, config-key remap, map encodings, deep nested sections, `Changed()`/`Source()` provenance, and the schema directives (`conga:"ignore"`, `allow-unknown-tags,recurse`, `AllowTags`). The `configs/` directory doubles as browsable documentation — run it with `--config-file configs/config.toml` |

## Shared helpers

- `internal/inspector` — renders the resolved configuration of the entire
  active command chain, from root down to the leaf: a rounded-box title with
  the command path, one section per command (siblings marked `▸ selected`),
  and per-field rows with value and origin badges (`● cli`, `● env VAR`,
  `● config key`, `· default`, `○ unset`). Colour is enabled on terminals and
  honours `NO_COLOR`, `CLICOLOR=0`, and `CONGA_COLOR` (`0`/`1`). Used by every
  example except `01-minimal-hello` so you can *see* the precedence engine at
  work.
- `internal/testutil` — shared testscript commands (`exit-status` for the
  exit-code contract, `copy-fixture` for materialising package-relative
  fixture files) used by the example test suites.

## Exit-code contract

CONGA distinguishes failure classes, and the examples assert it:

| Code | Meaning |
|------|---------|
| 0 | Success, or `--help` / `--version` |
| 1 | Runtime failure returned from `Run`, and config-file read/parse failures |
| 2 | Input/usage error: unknown flags, coercion failures, constraint violations |
