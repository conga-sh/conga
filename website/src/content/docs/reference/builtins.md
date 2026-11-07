---
title: Built-ins and deprecation
description: Help, version, why, completion commands, deprecation notices, and flat-CLI completion styles.
---

CONGA provides five built-in surfaces: help, version, `--why`, completion, and deprecation notices. Help and version are injected by default and disabled with builder calls; `--why` and completion are opt-in. The injected `--help` and `--version` flags are long-only unless a short rune is assigned with `app.HelpShortFlag(...)` or `app.VersionShortFlag(...)`.

## Help

Every app gets `--help` and a help screen that CONGA builds from the command schema. This `deploy` program has a description, examples, a feature, a footer, and two subcommands, one of them deprecated:

```sh
$ deploy --help
Roll out releases to a fleet.

Usage:
  deploy <command> [flags]

Examples:
  deploy rollout v1.4
  deploy rollout --region eu-west-1 v1.4

Commands:
  rollout  Roll out a release
  push     Push a release [deprecated: use rollout]

Flags:
      --feature-gates=KEY=VALUE,...  Enable or disable features (name=value)
                                     [env: DEPLOY_FEATURES]
  -v, --verbose                      Print each step
      --help                         Show help
      --version                      Show version

Features:
  canary  Canary rollouts [beta]

Docs: https://example.com/deploy
```

```sh
$ deploy push --help
WARNING: command 'deploy push' is deprecated: use rollout

Push a release

Usage:
  deploy push [flags] <release>

Arguments:
  <release>  Release to push

Flags:
      --help  Show help

Global Flags:
      --feature-gates=KEY=VALUE,...  Enable or disable features (name=value)
                                     [env: DEPLOY_FEATURES]
  -v, --verbose                      Print each step

Docs: https://example.com/deploy
```

Sections appear in this order, and a section with nothing to show is left out:

| Section | Heading | Content comes from |
|---|---|---|
| Deprecation banner | `WARNING: …` | `app.Deprecated(...)` or a command's `deprecated:"..."` tag. A screen shows the banner of every deprecated command on its path. |
| Description | none | The root: `app.Description(...)`. A subcommand: its `Description() string` method, or else its `summary:"..."` tag. |
| Usage | `Usage:` | The schema; see [Usage line](#usage-line). |
| Examples | `Examples:` | The root: `app.Examples(...)`. A subcommand: its `Examples() string` method. |
| Subcommands | `Commands:`, or the `section:"..."` headings | The command's `Command[T]` fields and their `summary` tags. |
| Arguments | `Arguments:` | The command's positional arguments. |
| Flags | `Flags:`, or the `section:"..."` headings | The command's flags, then the built-in `--help`, `--why`, and, on the root screen, `--version`. |
| Features | `Features:` | The root's `conga.Features[T]` toggles, on the root screen only. |
| Global flags | `Global Flags:` | Flags declared on ancestor commands. |
| Footer | none | `app.HelpFooter(...)`, on every screen. |

The root command takes its description from `app.Description(...)`. A root struct that implements `Description()` or `Summary()` fails schema build.

The Features section shows each feature's help text, stage (`[alpha]`, `[beta]`), integer levels, default, and environment variable. Features staged `ga` or `deprecated` are left out of it.

A boolean flag shows its default only when it is `true` (`[default: true]`).

Completion help screens add two sections. A per-shell screen such as `--completion bash --help` shows that shell's setup steps between the description and `Usage:`. The generic completion screen lists the supported shells after the arguments.

### Usage line

The usage line names the required flags and `oneof` groups first, then `[flags]` for the optional ones, then the positional arguments and any `DashArgs` or `RawArgs` collector. Each argument shows whether it is optional and whether it repeats. Here `rollout` was given a required `--image` flag and an optional list of hosts:

```text no-copy
Usage:
  deploy rollout --image=STRING [flags] [<hosts...>]
```

Brackets and `...` that you write in an argument's name are kept as written.

### Wrap width

Help text wraps to a width that CONGA picks in this order:

1. A `COLUMNS` environment variable of at least 20.
2. The width of the terminal on stdout, when it reports one of at least 20. CONGA asks the terminal on Linux, macOS, and the BSDs; on Windows and other platforms, set `COLUMNS`.
3. 100 columns, as for piped or redirected output.

Within that width:

- Widths count terminal cells, so wide CJK characters and combining marks keep the columns aligned.
- Line breaks and blank lines that you write in a description are kept, and continuation lines are indented to match.
- Long descriptions and help cells fold at spaces, with continuation lines under the help column.
- A bracketed hint such as `[env: NAME]` stays on one line where it fits.
- The help column starts at most halfway across. A label wider than that goes on its own line, with its help text below.
- The `Usage:` line, the `Examples:` block, and the footer are printed as written.

The Flags section of `COLUMNS=50 deploy --help` shows three of these rules together:

```text no-copy
Flags:
      --feature-gates=KEY=VALUE,...
                         Enable or disable
                         features (name=value)
                         [env: DEPLOY_FEATURES]
  -v, --verbose          Print each step
      --help             Show help
      --version          Show version
```

### Configurable short flags

The injected `--help` and `--version` flags render and parse as long flags. `app.HelpShortFlag('h')` and `app.VersionShortFlag('V')` assign them short runes, after which `-h` and `-V` parse to the same built-ins and the help screen renders `-h, --help` and `-V, --version`:

```go
app := conga.New[CLI]().
	HelpShortFlag('h').
	VersionShortFlag('V')
```

The zero value keeps a built-in long-only. The builders are opt-in because a short rune an application flag already declares cannot serve both: a collision between the rune and a user-declared short, or between the two built-in runes, fails schema build with `ErrDuplicateFlag` naming the flag and command. Disabling the matching built-in leaves the rune inert: it binds to nothing, and the `help` or `version` name and the rune stay free for your own flags. User-declared `--help` or `--version` flags shadow built-in rows and render long-only without inheriting the configured short rune.

The help rune applies to every command screen, and the version rune to the root screen, matching the root-only `--version` flag. Once a subcommand is selected — explicitly by name, by implicit operand routing, or by a child-only flag — `--version` is no longer a builtin there: the token is rejected as an unknown flag, or captured by the selected command's trailing `RawArgs` when its declared positionals are already full (the raw-args grammar applies identically to the named form). Use `tool --version` for the version summary. See [Implicit subcommands](/reference/struct-tags/#implicit-subcommands). Completion offers each configured rune as a shorthand candidate next to the declared short flags whenever the matching built-in is offered on the active command.

### Global help footers and top matter

Top-matter text consists of command descriptions and the deprecation banner.

`app.HelpFooter(footer)` sets a global help footer rendered at the bottom of every help screen across the application, including subcommands:

```go
app := conga.New[CLI]().
	Description("Service deployment manager").
	HelpFooter("Documentation: https://example.com/docs\nReport issues: https://example.com/issues")
```

Help footers are strictly global. Defining a `HelpFooter() string` method on a root command struct or a subcommand struct fails schema construction with `ErrSchema`.

### The `help` subcommand

Opt in to a `help [command...]` subcommand for nested commands:

```go
type App struct {
	Help conga.HelpCommand `summary:"Show help for a command"`
	...
}
```

`HelpCommand` resolves the requested path (with aliases) and returns the target's help screen; unknown paths are parse errors with suggestions. Call `Path() []string` to inspect the requested subcommand path. `app.DisableHelpFlag()` removes `--help` flag injection entirely.

The built-in command fields accept `name`, `summary`, `aliases`, `section`, and `hidden` tags. A declared `aliases:"h"` adds a routing alias, so `hello h` resolves the `help` subcommand; the same tag applies to `conga.VersionCommand` and `conga.CompletionCommand` fields.

## Version

Every root command gets `--version`. `app.Version("1.0.0")` sets the rendered string; the default is the module version (or `dev`) plus the embedded build metadata (VCS commit and date, when available). The summary line renders under the configured application name (`App.Name(...)`), falling back to the invoked binary name (`os.Args[0]`) only when unset.

The flag renders a single summary line, and `session.Version()` returns that same line. The commit and build timestamp are appended when the build info carries them and the version string does not already embed them (Go 1.27 records a pseudo-version for source builds):

```sh
$ hello --version
hello has version 1.0.0 built with go1.27.1 from c2e2ea84591f on 2026-09-25T16:51:49Z
```

Without `app.Version(...)`, the summary falls back to the build metadata. A source run with no VCS information reports `dev`:

```sh
$ hello --version
hello has version dev built with go1.27.1
```

Declaring an opt-in `conga.VersionCommand` field adds a `version` subcommand that renders the same metadata as a docker-style block with the full commit hash and OS/arch:

```sh
$ hello version
hello:
 Version:    1.0.0
 Git commit: c2e2ea84591f0ef557489d0f7f2b0b1565555950
 Built:      2026-09-25T16:51:49Z
 Go version: go1.27.1
 OS/Arch:    linux/amd64
```

`hello version --json` renders a flat machine-readable object with the same fields (`version`, `gitCommit`, `built`, `goVersion`, `os`, `arch`, `modified`). Call `JSON() bool` on the command struct to query whether JSON format was requested. The `--json` flag is engine-owned: a user-defined `--json` flag on an ancestor command cannot collide with it, and a non-boolean value is a usage error (exit 2).

`app.DisableVersionFlag()` removes the flag while keeping the version summary line available through `session.Version()` and `VersionCommand`. `app.VersionShortFlag('V')` assigns the rune rendered and parsed as `-V` (see [Configurable short flags](#configurable-short-flags)).

## The `--why` report

`app.EnableWhy()` enables `--why`, which prints a per-field provenance report — every resolved value in the active command chain with its [source](/explanation/value-precedence/): `cli`, `env`, `config (file)`, `default`, or `unset`. Configuration origins render as the file basename when it is unambiguous, or as the shortest unique directory prefix followed by `...` when basenames collide; the JSON report keeps the resolved paths verbatim. A multi-file `ConfigFile` row renders `(N files)` and lists each path as a tree sub-row (`├─ `, `└─ `), while any other slice flag with more than one item renders `(N values)` and lists each item vertically.

Map flags merge key by key across tiers: the flag row displays the key count `(N keys)` alongside the highest contributing tier (annotated with its config origin when a single file contributes), followed by tree-structured sub-rows detailing each key's value and the tier that last wrote it (annotated with its config origin). The `CONFIG PATH` column names the config-document key. Values are redacted by default; fields opt in to display with `safe:"true"`. The report machinery is linked only when `app.EnableWhy()` is called; otherwise `--why` remains an unknown flag.

When connected to an interactive terminal, the report uses colour: each source gets a marker and a colour (`● cli` cyan, `● env` yellow, `● config` green, `○ default` grey, `unset` dim). When piped, redirected, or running under `NO_COLOR=1` or `TERM=dumb`, it prints plain text with no markers, as shown on the how-to pages. Output color can also be forced with `CONGA_COLOR=1` or disabled with `CONGA_COLOR=0`.

Resolution runs through the standard binding pipeline (skipping required-flag presence checks). If configuration is malformed or flag coercion fails, `--why` fails fast with normal error diagnostics instead of printing partial tables.

By default (or with `--why=text`), the report renders as formatted human-readable tables. Pass `conga.WhyOptionJSON()` to `EnableWhy` (`app.EnableWhy(conga.WhyOptionJSON())`) to enable `--why=json`, which emits a versioned machine-readable document:

| Property | Type | Description |
|----------|------|-------------|
| `$schema` | `string` | Schema URI, `https://conga.sh/schemas/why/v1.json` |
| `schemaVersion` | `int` | Integer schema version (currently `1`) |
| `command` | `string` | Executed command path |
| `version` | `string` | Application version string; omitted when empty |
| `timestamp` | `string` | RFC 3339 timestamp of the invocation; omitted when empty |
| `workingDirectory` | `string` | Current working directory; omitted when empty |
| `redacted` | `bool` | True if one or more values were masked |
| `flags` | `[]object` | Resolved flags for the active command: `name` (without `--`), `type`, `value`, `source`, and, when present, `detail`, `env`, `configPath`, `redacted: true`, `hidden: true`, and `entries` (for maps). A redacted value is `null`. |
| `globalFlags` | `[]object` | Resolved flags from ancestor commands, in the same form; omitted when empty |
| `features` | `[]object` | Feature toggles: `name`, `value`, `source`, and `detail` when present; omitted when empty |
| `config` | `object` | Config status: registered `decoders`, evaluated `paths` (`path`, `status`, `probedFiles`), and `examples` when the report suggests where to create a file |

Requesting `--why=json` without `conga.WhyOptionJSON()` enabled on the application, or passing any value other than `text` or `json`, is a usage error (exit 2).

When `conga.WhyOptionJSON()` is enabled, the help screen renders the flag as `--why[=FORMAT]` with the accepted formats appended as an enum hint (`[enum: text, json]`); otherwise `--why` renders as a plain toggle. Completion mirrors the split: the flag-name candidates are the value-bearing forms `--why=text` and `--why=json`, and completing after `--why=` offers the enabled formats filtered by prefix.

When the root command declares `conga.Features[T]`, the report ends with a `Features:` table listing every declared feature, its resolved value, and its source. Each feature reports the tier that last wrote its key; keys no tier sets fall back to the field default (`○ default`) or render `unset`. See [Feature toggles](/reference/struct-tags/#feature-toggles-congafeaturest).

## Completion surface

`app.EnableCompletion()` enables shell completion. The completion flag, the `completion` subcommand, the `__complete` protocol, and their shell drivers are linked only when this call is present.

| Style | Surface | When |
|-------|---------|------|
| Flat flag | `--completion <shell>` | Enabled flat, single-command CLIs |
| Subcommand | `completion <shell>` via `conga.CompletionCommand` | Declare the field and enable `app.EnableCompletion()` (inspect requested shell with `Shell() string`) |
| Hidden driver | `<cmd> __complete <shell> <words...>` | Used internally by generated shell functions |

When the user presses tab, the installed shell function invokes the binary through the hidden driver with the partial words:

```sh
cloudctl __complete bash "dep"
```

The driver tokenizes the partial line, identifies the target command, flag, or argument, evaluates the applicable predictor and the built-in candidates, writes the shell-native directives to stdout, and exits 0 immediately, preserving the application's single-execution slot.

If an application implements [`conga.Bootstrapper`](/how-to/load-domain-files/), the completion driver binds fields tagged `bootstrap:"true"` from the command line, environment, and defaults, and runs `Bootstrap` before candidate generation. This ensures predictors and feature gates evaluate with access to configuration extracted from the document or `.env` files. The driver restores bootstrap and selection state before returning candidates.

With completion enabled, several elements complete without custom code:
- Flag names (long flags, short runes, aliases, and negatable forms), omitting deprecated flags.
- Subcommands, command aliases, and command sections with typo-correction suggestions.
- Enum values restricted by `enum:"..."` tags.
- Domain types with built-in path or unit predictions (`conga.File`, `conga.Dir`, `conga.ByteSize`).

## Deprecation

Whole-application deprecation prints a warning banner on every invocation (root, subcommands, and help):

```go
app := conga.New[Legacy]().
	Deprecated("migrated to newctl — see https://example.com/migration")
```

Member-level deprecation uses tags and emits a warning when the flag or subcommand is used:

```go
type App struct {
	Old   conga.OptionalFlag[bool]    `deprecated:"use --new instead" help:"Legacy switch"`
	Purge conga.Command[PurgeCommand] `deprecated:"folded into 'clean'" summary:"Purge caches"`
}
```

Deprecated members are excluded from shell completion.
