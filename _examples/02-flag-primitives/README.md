# 02 — Flag primitives

Primitive flag types (`string`, `bool`, `int`, `float64`, `time.Duration`),
`conga.Counter` (`-vvv` clustering), flag aliases, flag groups, hidden flags, and
coercion errors.

## Run it

```sh
go run ./_examples/02-flag-primitives -vvv --dbg
go run ./_examples/02-flag-primitives --timeout 5s --port 9090
go run ./_examples/02-flag-primitives --help
```

## What to look for

- `-vvv` clusters into `Verbosity == 3`.
- Aliases (`--dbg`) and hidden flags (`--secret-key` is absent from `--help`).
- A malformed `--timeout` fails with a coercion error and exit code 2.

## Test

```sh
go test ./_examples/02-flag-primitives
```

Related: [03 — Negatable and maps](../03-negatable-and-maps/),
[99 — Kitchen sink](../99-kitchen-sink/).
