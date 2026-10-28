---
title: Built-ins and deprecation
description: Help, version, why, completion commands, deprecation notices, and flat-CLI completion styles.
---

CONGA provides five built-in surfaces: help, version, `--why`, completion, and deprecation notices. Help and version are injected by default and disabled with builder calls; `--why` and completion are opt-in. The injected `--help` and `--version` flags are long-only unless a short rune is assigned with `app.HelpShortFlag(...)` or `app.VersionShortFlag(...)`.

## Help

Every app gets `--help` and a generated help screen. CONGA derives the help output from the command schema and renders sections in a fixed order:

1. **Deprecation banner**: configured via `app.Deprecated(...)` or the member `deprecated:"..."` tag.
2. **Description**: configured via `app.Description(...)` on the root builder, or `Description() string` on subcommand structs. Subcommands fall back to their `summary:"..."` tag when no description method is defined. Root structs must not implement `Description()` or `Summary()`.
3. **Completion instructions**: per-shell setup steps rendered when inspecting a completion command (e.g. `--completion bash --help`).
4. **Usage syntax**: command invocation syntax including flags, positional arguments, and trailing arguments (`DashArgs`, `RawArgs`). Optional flags collapse into a `[flags...]` placeholder; positional arguments carry their optionality and repetition (`[<target>]`, `<file...>`), and author-supplied brackets and `...` are preserved.
5. **Examples**: usage examples configured via `app.Examples(...)` on the root builder, or `Examples() string` on subcommand structs.
6. **Subcommands**: child commands listed under their `group:"..."` section headers or the default `Commands:` heading.
7. **Positional arguments**: argument names and descriptions under `Arguments:`.
8. **Completion footer**: supported shells list rendered on generic completion help screens.
9. **Flags**: flags declared on the active command, grouped under `group:"..."` headers or the default `Flags:` heading. Built-in flags (`--help`, `--version`, `--why`) appear here.
10. **Features** (root screen only): feature toggles declared via `conga.Features[T]`, rendered under `Features:` with help text, stage (`[alpha]`, `[beta]`), integer levels, and defaults. Toggles staged `ga` or `deprecated` are omitted.
11. **Global flags**: persistent flags inherited from ancestor commands.
12. **Global help footer**: text configured via `app.HelpFooter(...)`.

### Configurable short flags

The injected `--help` and `--version` flags render and parse as long flags. `app.HelpShortFlag('h')` and `app.VersionShortFlag('V')` assign them short runes, after which `-h` and `-V` parse to the same built-ins and the help screen renders `-h, --help` and `-V, --version`:

```go
app := conga.New[CLI]().
	HelpShortFlag('h').
	VersionShortFlag('V')
```

The zero value keeps a built-in long-only. The builders are opt-in because a short rune an application flag already declares cannot serve both: a collision between the rune and a user-declared short, or between the two built-in runes, fails schema build with `ErrDuplicateFlag` naming the flag and command. Disabling the matching built-in leaves the rune inert: it binds to nothing, and the `help` or `version` name and the rune stay free for your own flags.

The help rune applies to every command screen, and the version rune to the root screen, matching the root-only `--version` flag. Completion offers each configured rune as a shorthand candidate next to the declared short flags whenever the matching built-in is offered on the active command.

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

The built-in command fields accept `name`, `summary`, `aliases`, `group`, and `hidden` tags. A declared `aliases:"h"` adds a routing alias, so `hello h` resolves the `help` subcommand; the same tag applies to `conga.VersionCommand` and `conga.CompletionCommand` fields.

## Version

Every root command gets `--version`. `app.Version("1.0.0")` sets the rendered string; the default is the module version (or `dev`) plus the embedded build metadata (VCS commit and date, when available).

The flag renders a single summary line, and `session.Version()` returns that same line. The commit and build timestamp are appended when the build info carries them and the version string does not already embed them (Go 1.27 records a pseudo-version for source builds):

```sh
hello --version
```

```text no-copy
hello has version 1.0.0 built with go1.27.1 from c2e2ea84591f on 2026-09-25T16:51:49Z
```

Without `app.Version(...)`, the summary falls back to the build metadata. A source run with no VCS information reports `dev`:

```sh
hello --version
```

```text no-copy
hello has version dev built with go1.27.1
```

Declaring an opt-in `conga.VersionCommand` field adds a `version` subcommand that renders the same metadata as a docker-style block with the full commit hash and OS/arch:

```sh
hello version
```

```text no-copy
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

`app.EnableWhy()` enables `--why`, which prints a per-field provenance report — every resolved value in the active command chain with its [origin tier](/explanation/value-precedence/) (`● cli`, `● env (VAR)`, `● config (file)`, `○ default`, `unset`). Configuration origins render as the file basename when it is unambiguous, or as the shortest unique directory prefix followed by `...` when basenames collide; the JSON report keeps the resolved paths verbatim. A multi-file `ConfigFile` row renders `(N files)` and lists each path as a tree sub-row (`├─ `, `└─ `), while any other slice flag with more than one item renders `(N values)` and lists each item vertically.

Map flags merge key by key across tiers: the flag row displays the key count `(N keys)` alongside the highest contributing tier (annotated with its config origin when a single file contributes), followed by tree-structured sub-rows detailing each key's value and the tier that last wrote it (annotated with its config origin). The `CONFIG PATH` column names the config-document key. Values are redacted by default; fields opt in to display with `safe:"true"`. The report machinery is linked only when `app.EnableWhy()` is called; otherwise `--why` remains an unknown flag.

When connected to an interactive terminal, the report uses ANSI color styling (cyan for CLI, yellow for env, green for config, dim/gray for defaults and unset); when piped, redirected, or running under `NO_COLOR=1` or `TERM=dumb`, it automatically falls back to unstyled plain text. Output color can also be forced with `CONGA_COLOR=1` or disabled with `CONGA_COLOR=0`.

Resolution runs through the standard binding pipeline (skipping required-flag presence checks). If configuration is malformed or flag coercion fails, `--why` fails fast with normal error diagnostics instead of printing partial tables.

By default (or with `--why=text`), the report renders as formatted human-readable tables. Pass `conga.WhyOptionJSON()` to `EnableWhy` (`app.EnableWhy(conga.WhyOptionJSON())`) to enable `--why=json`, which emits a versioned machine-readable document:

| Property | Type | Description |
|----------|------|-------------|
| `$schema` | `string` | Schema validation URI |
| `schemaVersion` | `int` | Integer schema version (currently `1`) |
| `command` | `string` | Executed command path |
| `version` | `string` | Application version string |
| `timestamp` | `string` | RFC 3339 timestamp of the invocation |
| `workingDirectory` | `string` | Current working directory |
| `flags` | `[]object` | Resolved flags for the active command: `name`, `type`, `value`, `source`, `detail`, `env`, `configPath`, `hidden`, `redacted`, and `entries` (for maps) |
| `globalFlags` | `[]object` | Resolved flags from ancestor commands |
| `features` | `[]object` | Feature toggles: `name`, `value`, `source`, `detail` |
| `config` | `object` | Config status: registered `decoders`, evaluated `paths` (`path`, `status`, `probedFiles`, `loaded`), and `examples` |
| `redacted` | `bool` | True if one or more sensitive fields were masked |

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
- Subcommands, command aliases, and command groups with typo-correction suggestions.
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
