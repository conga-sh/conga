# 01 — Minimal hello

The smallest complete CONGA program: one struct, one `Run` method, one command.
Shows `OptionalFlag[T]` with a `default` tag, a `short` flag, and the generated
`--help` screen.

## Run it

```sh
go run ./_examples/01-minimal-hello
go run ./_examples/01-minimal-hello --name Alice -l
go run ./_examples/01-minimal-hello --help
```

## What to look for

- `Name` becomes `--name`/`-n` and falls back to `World` without a flag.
- The `Run` method writes to `session.Stdout()`, never to `os.Stdout`.

## Test

```sh
go test ./_examples/01-minimal-hello
```

Related: [Quickstart](https://docs.conga.sh/tutorials/quickstart/),
[02 — Flag primitives](../02-flag-primitives/).
