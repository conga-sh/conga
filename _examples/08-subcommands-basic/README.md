# 08 — Subcommands basic

`conga.Command[T]` routing with command aliases, help groups, a hidden
command, and did-you-mean suggestions for typos.

## Run it

```sh
go run ./_examples/08-subcommands-basic create web-api -i redis:7 -r 3
go run ./_examples/08-subcommands-basic rm web-api -f
go run ./_examples/08-subcommands-basic --help
```

## What to look for

- `delete` is reachable as `rm`/`del`; sibling commands may share flag names.
- `doctor` is hidden from `--help` but still invocable.
- A misspelled subcommand suggests the closest match.

## Test

```sh
go test ./_examples/08-subcommands-basic
```

Related: [09 — Deep hierarchy context](../09-deep-hierarchy-context/),
[99 — Kitchen sink](../99-kitchen-sink/).
