# 24 — Flat completion flag

A flat CLI (no subcommands) that opts into `EnableCompletion()` and therefore
uses the `--completion <shell>` flag style. Because there are no commands, the
generated help omits the `Commands:` section entirely.

## Run it

```sh
go run ./_examples/24-flat-completion-flag --help
go run ./_examples/24-flat-completion-flag --completion bash
go run ./_examples/24-flat-completion-flag --level 9 input.css
```

## What to look for

- No `Commands:` section in `--help`, even with completion enabled.
- `--completion <shell>` prints the script for the requested shell.
- The `--completion` flag is the default flat-CLI wiring style.

## Test

```sh
go test ./_examples/24-flat-completion-flag
```

Related: [25 — Flat completion subcommand](../25-flat-completion-subcommand/),
[19 — Shell completion](../19-shell-completion/).
