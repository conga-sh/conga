---
title: Built-ins & Deprecation
description: Help, version, why, completion commands, deprecation notices, and flat-CLI completion styles.
---

## Help

Every app gets `--help`/`-h` and a generated help screen: description, usage line, grouped flags, positional arguments, subcommands, and examples — derived from the schema.

Opt in to a `help [command...]` subcommand for nested commands:

```go
type App struct {
	Help conga.HelpCommand `name:"help" summary:"Show help for a command"`
	...
}
```

`HelpCommand` resolves the requested path (with aliases) and returns the target's help screen; unknown paths are parse errors with suggestions. `app.DisableHelpFlag()` removes `--help` injection entirely.

## Version

Every root command gets `--version`/`-v`. `app.Version("1.0.0")` sets the rendered string; without it, the output falls back to the module version (or `dev`) plus the embedded build metadata (VCS commit and date, when available).

The flag and `session.Version()` render a single summary line. The commit and build timestamp are appended when the build info carries them and the version string does not already embed them (Go 1.27 records a pseudo-version for source builds):

```sh
$ hello --version
hello has version 1.0.0 built with go1.27.1 from c2e2ea84591f on 2026-09-25T16:51:49Z
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

`hello version --json` renders a flat machine-readable object with the same fields (`version`, `gitCommit`, `built`, `goVersion`, `os`, `arch`, `modified`). The `--json` flag is engine-owned: a user-defined `--json` flag on an ancestor command cannot collide with it, and a non-boolean value is a usage error (exit 2).

`app.DisableVersionFlag()` removes the flag while keeping the metadata available to `session.Version()` and `VersionCommand`.

## The `--why` report

`app.EnableWhy()` enables `--why`, which prints a per-field provenance report — every resolved value in the active command chain with its [origin tier](../explanation/value-precedence/) (`● cli`, `● env VAR`, `● config key`, `· default`, `○ unset`). Values are redacted by default; fields opt in to display with `safe:"true"`. Applications that never call `app.EnableWhy()` do not link the report machinery, and `--why` is an unknown flag.

## Completion surface

`app.EnableCompletion()` enables shell completion; without it the completion flag, subcommand, and `__complete` protocol do not exist and their drivers are not linked.

| Style | Surface | When |
|-------|---------|------|
| Flat flag | `--completion <shell>` | Enabled flat, single-command CLIs |
| Subcommand | `completion <shell>` via `conga.CompletionCommand` | Declare the field and enable `app.EnableCompletion()` |
| Hidden driver | `<cmd> __complete <shell> <words...>` | Used internally by generated shell functions |

## Deprecation

Whole-application deprecation prints a warning banner on every invocation (root, subcommands, and help):

```go
app := conga.New[Legacy]().
	Name("legacyctl").
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

## Usage output control

- `app.Stdout(w)` / `app.Stderr(w)` redirect help and error streams (tests, embedders).
- `app.Args(args)` parses a custom argument slice instead of `os.Args[1:]`.
- `app.ExitFunc(handler)` replaces `os.Exit` in `FatalIfError`.
- `UsageOnError(mode)` controls error-time usage rendering (`conga.UsageSilence` suppresses it) — see [Errors & Exit Codes](errors/).
