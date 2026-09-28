# 25 — Flat completion subcommand

A flat CLI that opts into the `conga.CompletionCommand` subcommand instead of
the `--completion` flag. When the subcommand is present, the flag is suppressed
from help in favor of `completion <shell>`.

## Run it

```sh
go run ./_examples/25-flat-completion-subcommand --help
go run ./_examples/25-flat-completion-subcommand completion zsh
go run ./_examples/25-flat-completion-subcommand build --dry-run
```

## What to look for

- `completion <shell>` appears under `Commands:`; `--completion` does not.
- Task arguments still complete through the subcommand form.

## Test

```sh
go test ./_examples/25-flat-completion-subcommand
```

Related: [24 — Flat completion flag](../24-flat-completion-flag/),
[19 — Shell completion](../19-shell-completion/).
