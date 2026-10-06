# 10 — Builtins help and version

Opt-in `conga.HelpCommand` and `conga.VersionCommand` subcommands (including
`version --json`) alongside custom commands that render through
`session.Help()`, `session.Usage()`, and `session.Version()`. Also covers the
`Examples`/`HelpFooter` builders and the `Description()`/`Examples()` methods.

## Run it

```sh
go run ./_examples/10-builtins-help-version help status
go run ./_examples/10-builtins-help-version version --json
go run ./_examples/10-builtins-help-version guide
go run ./_examples/10-builtins-help-version syntax
```

## What to look for

- `help status` renders the multi-line `Description()` and `Examples()`.
- `guide`/`syntax`/`about` reproduce the built-in renderers without pseudo-error
  sentinels — they return `nil`.
- Invalid inputs still exit 2 (`UsageOnError(conga.UsageHint)`).

## Test

```sh
go test ./_examples/10-builtins-help-version
```

Related: [24 — Flat completion flag](../24-flat-completion-flag/),
[25 — Flat completion subcommand](../25-flat-completion-subcommand/).
