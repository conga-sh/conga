---
title: Built-ins and deprecation
description: Help, version, why, completion commands, deprecation notices, and flat-CLI completion styles.
---

CONGA has five built-ins:

- Help and version, on by default. `app.DisableHelpFlag()` and `app.DisableVersionFlag()` remove their flags.
- `--why` and shell completion, opt-in through `app.EnableWhy()` and `app.EnableCompletion()`.
- Deprecation notices, from `app.Deprecated(...)` and `deprecated` tags.

The injected `--help` and `--version` flags are long-only unless a short rune is assigned with `app.HelpShortFlag(...)` or `app.VersionShortFlag(...)`.

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
2. The width of the terminal on stdout, when it reports one of at least 20. CONGA asks the terminal on Linux, macOS, and the BSDs. On Windows and other platforms, set `COLUMNS`.
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

The zero value keeps a built-in long-only. A collision between the rune and a user-declared short, or between the two built-in runes, fails schema build with `ErrDuplicateFlag` naming the flag and command. Disabling the matching built-in leaves the rune inert. It binds to nothing, and the `help` or `version` name and the rune stay free for your own flags. User-declared `--help` or `--version` flags shadow built-in rows and render long-only without inheriting the configured short rune.

The help rune applies to every command screen, and the version rune to the root screen, matching the root-only `--version` flag. Once a subcommand is selected, `--version` is no longer a built-in ([`--version` after selection](/reference/struct-tags/#--version-after-selection)). Completion offers each configured rune as a shorthand candidate next to the declared short flags whenever the matching built-in is offered on the active command.

### Help footer

`app.HelpFooter(footer)` sets a global help footer rendered at the bottom of every help screen across the application, including subcommands:

```go
app := conga.New[CLI]().
	Description("Service deployment manager").
	HelpFooter("Documentation: https://example.com/docs\nReport issues: https://example.com/issues")
```

The footer is set only through `app.HelpFooter`. A `HelpFooter() string` method on a root command struct or a subcommand struct fails schema build with `ErrSchema`.

### The `help` subcommand

Opt in to a `help [command...]` subcommand for nested commands:

```go
type App struct {
	Help conga.HelpCommand `summary:"Show help for a command"`
	...
}
```

`HelpCommand` resolves the requested path (with aliases) and returns the target's help screen, and an unknown path is a parse error with suggestions. Call `Path() []string` to inspect the requested subcommand path. `app.DisableHelpFlag()` removes `--help` flag injection entirely.

The built-in command fields accept `name`, `summary`, `aliases`, `section`, and `hidden` tags. A declared `aliases:"h"` adds a routing alias, so `hello h` resolves the `help` subcommand. The same tag applies to `conga.VersionCommand` and `conga.CompletionCommand` fields.

## Version

Every root command gets `--version`. `app.Version("1.0.0")` sets the rendered string. The default is the module version (or `dev`) plus the embedded build metadata (VCS commit and date, when available). The summary line renders under the configured application name (`App.Name(...)`), falling back to the invoked binary name (`os.Args[0]`) only when unset.

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

`hello version --json` renders a flat machine-readable object with the same fields (`version`, `gitCommit`, `built`, `goVersion`, `os`, `arch`, `modified`). Call `JSON() bool` on the command struct to query whether JSON format was requested. The `--json` flag is engine-owned, so a user-defined `--json` flag on an ancestor command cannot collide with it, and a non-boolean value is a usage error (exit 2).

`app.DisableVersionFlag()` removes the flag while keeping the version summary line available through `session.Version()` and `VersionCommand`. `app.VersionShortFlag('V')` assigns the rune rendered and parsed as `-V` (see [Configurable short flags](#configurable-short-flags)).

## The `--why` report

`app.EnableWhy()` enables `--why`, which prints a per-field provenance report. It lists every resolved value in the active command chain with its [source](/explanation/value-precedence/), one of `cli`, `env`, `config (file)`, `default`, or `unset`. Configuration origins render as the file basename when it is unambiguous, or as the shortest unique directory prefix followed by `...` when basenames collide, and the JSON report keeps the resolved paths verbatim. A multi-file `ConfigFile` row renders `(N files)` and lists each path as a tree sub-row (`├─ `, `└─ `), while any other slice flag with more than one item renders `(N values)` and lists each item vertically.

A map flag's row shows `(N keys)` and the highest source that contributed, with its config file when only one file contributed. A sub-row for each key shows its value and the source that last wrote it, with its config file. The `CONFIG PATH` column names the config-document key. The report code is linked only when `app.EnableWhy()` is called, and otherwise `--why` is an unknown flag.

The text report has these parts, in this order, and leaves out any part with nothing to show:

1. Flag table: each flag of the active command, with its value (`██████` for a flag without `safe:"true"`), source, environment variable, and config key.
2. Global flags: the same table for flags declared on parent commands.
3. Features: every declared feature toggle, its value, and its source.
4. Config decoders: the registered config file extensions.
5. Config paths: each place the program searched and what it found there (`loaded: <file>`, `directory does not exist`), with the files it looked for. When a bootstrap document records scopes with `SetScoped`, the origin path appears once and the flag's source shows the scope.
6. Config file suggestions: where the user could create a config file, when none was loaded.
7. A redaction note, when any value is shown as `██████`.

A value prints as `██████` unless one of these makes it visible:

| Value | Shown |
|---|---|
| Any flag tagged `safe:"true"` | Always |
| A value from the flag's `default` tag | Always, whatever `safe` says |
| A boolean or `conga.Counter` flag | Unless tagged `safe:"false"` |
| The paths of a `conga.ConfigFile` flag | Always |
| Feature toggles | Always |
| Any other flag, set from the command line, the environment, or a config file | No |

A map flag applies the rule key by key. Keys from the default show, and keys set elsewhere print as `KEY=██████`. Unset flags have no value to show. Positional arguments are not part of the report. In JSON, a hidden value is `null` with `"redacted": true`.

When connected to an interactive terminal, the report uses color, and each source gets a marker and a color (`● cli` cyan, `● env` yellow, `● config` green, `○ default` grey, `unset` dim). When piped, redirected, or running under `NO_COLOR=1` or `TERM=dumb`, it prints plain text with no markers, as shown on the how-to pages. Output color can also be forced with `CONGA_COLOR=1` or disabled with `CONGA_COLOR=0`.

`--why` binds values as a normal run does, except that a missing required flag is not an error. Malformed configuration or a value that fails to parse produces the normal error, and no report.

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

When `conga.WhyOptionJSON()` is enabled, the help screen renders the flag as `--why[=FORMAT]` with the accepted formats appended as an enum hint (`[enum: text, json]`). Otherwise `--why` renders as a plain toggle. Completion mirrors the split, and the flag-name candidates are the value-bearing forms `--why=text` and `--why=json`, and completing after `--why=` offers the enabled formats filtered by prefix.

When the root command declares `conga.Features[T]`, the report ends with a `Features:` table listing every declared feature, its resolved value, and its source. Each feature reports the source that last wrote its key. Keys no source sets fall back to the field default (`○ default`) or render `unset`. See [Feature toggles](/reference/struct-tags/#feature-toggles-congafeaturest).

## Completion

`app.EnableCompletion()` enables shell completion. The completion flag, the `completion` subcommand, the `__complete` protocol, and their shell drivers are linked only when this call is present.

| Style | Invocation | When |
|-------|---------|------|
| Flag | `--completion <shell>` | A root command without subcommands |
| Subcommand | `completion <shell>`, listed last under `Commands:` | A root command with subcommands, or any root command that declares a `conga.CompletionCommand` field (which also suppresses the flag). The field accepts the `name`, `aliases`, `section`, `summary`, and `hidden` tags, and its `Shell() string` method returns the requested shell. Declaring `conga.CompletionCommand` without `app.EnableCompletion()` fails schema build with `ErrSchema` |
| Hidden driver | `<cmd> __complete <shell> <words...>` | Called by the generated shell functions; `export` in place of a shell name prints JSON (see [Candidates as JSON](#candidates-as-json)) |

Invoked without a shell argument, the flag or subcommand prints the supported shells and, when the active shell is detected, the command to load completion into the current session (exit 0). Invoked with `<shell> --help`, it prints the installation steps for that shell, adjusted to the environment. With `HOMEBREW_PREFIX` set, the zsh steps write to `$HOMEBREW_PREFIX/share/zsh/site-functions`, and on macOS the bash steps warn when the running Bash is older than 4.4. An unknown shell name is a usage error (exit 2).

When the user presses `Tab`, the installed shell function invokes the binary through the hidden driver with the partial words:

```sh
cloudctl __complete bash "dep"
```

The driver prints the candidates for the last word in the shell's format and exits 0. A completion invocation does not count as the app's one execution.

If an application implements [`conga.Bootstrapper`](/how-to/load-domain-documents/), the completion driver binds fields tagged `bootstrap:"true"` from the command line, environment, and defaults, and runs `Bootstrap` before candidate generation. Predictors and feature gates therefore see the values `Bootstrap` loads from `.env` files or a document. The driver restores bootstrap and selection state before returning candidates.

With completion enabled, several elements complete without custom code:
- Flag names (long flags, short runes, aliases, and negatable forms), omitting deprecated flags.
- Subcommands, command aliases, and command sections with typo-correction suggestions.
- Enum values restricted by `enum:"..."` tags.
- Paths for the `conga.File`, `conga.FileContent`, and `conga.Dir` domain types.
- The shell names after `completion` or `--completion`.

Once a member of an `xor` or `oneof` group is on the command line, its sibling members are no longer offered.

### Candidates as JSON

`<cmd> __complete export <words...>` prints the candidates for the last word as one JSON object, with no shell script involved. The last word is the one being completed, so pass `''` to complete an empty word. For the `deploy` program in [How to compute and shape completion candidates](/how-to/compute-completion-candidates/):

```sh wrap
$ deploy __complete export --pin ''
{"candidates":[{"value":"api:","display":"api:","tag":"values"},{"value":"web:","display":"web:","tag":"values"},{"value":"worker:","display":"worker:","tag":"values"}],"directives":{"keeporder":false,"messages":[],"noprefix":false,"nospace":true},"usage":"Deploy a service at one of its tags (service:tag)"}
```

`candidates` holds each value with its display text, its tag, and, when set, its description and alias marker. `directives` holds the flags the shell scripts act on, and `usage` is the `help` text of the flag or argument being completed. [The completion wire protocol](https://github.com/conga-sh/conga/blob/main/docs/completion-wire-protocol.md) defines every field.

### Flags at a bare space

At a bare space, the driver offers subcommands and positional arguments and withholds flags until the user types a `-` prefix. `cloudctl <Tab>` completes subcommands, and `cloudctl -<Tab>` completes flags. If a command defines no subcommands, positional arguments, or `RawArgs`, flags are offered at a bare space. Set the `bare-space-flags` zstyle or `<APP>_COMPLETION_BARE_SPACE_FLAGS=1` to include flags at a bare space unconditionally.

### User toggles

The generated scripts read three per-application environment variables. The default prefix is the application name in `UPPER_SNAKE_CASE` (`cloudctl` reads `CLOUDCTL_COMPLETION_*`). A derived prefix that starts with a digit gains a leading underscore (`7z` reads `_7Z_COMPLETION_*`). Pass `conga.CompletionEnvPrefix(prefix)` to `app.EnableCompletion` to override the prefix. An explicit prefix that normalizes to a leading digit fails schema build with `ErrSchema`.

| Variable | Effect |
|---|---|
| `<APP>_COMPLETION_DESCRIPTIONS=0` | Hide candidate descriptions (`false` also hides them; unset or `1` shows them) |
| `<APP>_COMPLETION_ALIASES=0` | Hide alias candidates (`false` also hides them; unset or `1` shows them) |
| `<APP>_COMPLETION_BARE_SPACE_FLAGS=1` | Show flag candidates at a bare space (`true` also shows them; unset or `0` keeps them for the `-` prefix) |

In bash, descriptions appear only in the listing on the second `Tab`, so `<APP>_COMPLETION_DESCRIPTIONS=0` affects that listing only.

Alias candidates are subcommand and flag aliases declared with the `aliases` tag, plus candidates marked with `predict.Item.WithAlias()`.

The zsh script also reads these `zstyle` keys:

| Style | Effect |
|---|---|
| `verbose` | `false`, `no`, `off`, or `0` hides candidate descriptions (the standard zsh style) |
| `show-aliases` | `false`, `no`, `off`, or `0` hides alias candidates (the go-task convention) |
| `bare-space-flags` | `yes`, `true`, `1`, or `on` shows flag candidates at a bare space |
| `list-separator` | The separator between a candidate and its description; the engine's colors follow it |

The engine installs its own colors only when `list-colors` is unset or empty (as in Oh My Zsh's default), and it leaves the user's `group-name` and `list-separator` styles in place.

### Binary names and paths

The generated script binds completion registration to the name of the binary that generated it (`cctl` emits `#compdef cctl`). Generate the script from the installed binary so the registered completer name matches.

In bash, zsh, fish, PowerShell, and Nushell, a path-qualified invocation (`./cloudctl <Tab>`, `/usr/local/bin/cloudctl <Tab>`) dispatches to the registered completer by basename. Elvish keys completers on the literal command word, so path-qualified invocations fall back to file completion there. In zsh, a named directory registered with `hash -d` (or `AUTO_NAME_DIRS`) resolves through the completer (`~dev/cloudctl <Tab>`).

### File and directory candidates

The filesystem predictors in `package predict` (`Files`, `Dirs`, `FilesGlob`, `FilesFiltered`, `PathsFiltered`, `DirsGlob`, `DirsFiltered`, `Symlinks`, `Executables`) evaluate candidate paths in-process rather than delegating to shell file completion, producing identical candidates across all six shells. Dotfiles are offered only when the typed prefix begins with `.`.

Because zsh's `_files` helper is not invoked, zsh-specific file-completion settings do not apply to these candidates. These include the `file-sort` and `file-patterns` styles, `cdpath`, and `~name` expansion in file values (tracked in [#85](https://github.com/conga-sh/conga/issues/85)).

## Deprecation

Whole-application deprecation prints a warning banner on every invocation (root, subcommands, and help):

```go
app := conga.New[Legacy]().
	Deprecated("migrated to newctl — see https://example.com/migration")
```

Member-level deprecation uses tags and emits a warning when the flag or subcommand is used:

```go
type App struct {
	Old   conga.OptionalFlag[bool] `deprecated:"use --new instead" help:"Legacy switch"`
	Purge conga.Command[PurgeCmd]  `deprecated:"folded into 'clean'" summary:"Purge caches"`
}
```

Deprecated members are excluded from shell completion.
