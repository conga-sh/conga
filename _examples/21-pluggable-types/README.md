# 21 — Pluggable types

Pluggable type parsers in two forms: standard library types (`*time.Location`,
`syscall.Signal`) wired via `app.Use` bindings (`timezone.RegisterLocation`,
`system.RegisterSignal`), and a foreign type wired via `app.RegisterType` with
`conga.WithPredictor` (`Percent`).

## Run it

```sh
go run ./_examples/21-pluggable-types -z Europe/Berlin -s HUP
go run ./_examples/21-pluggable-types --load 75%
go run ./_examples/21-pluggable-types --help
```

## What to look for

- The `syscall.Signal` parser accepts both POSIX signal names and numeric values
  (`--signal TERM` ≈ `--signal 15`).
- `parsePercent` is the only custom parser; completion for it comes from
  `WithPredictor`.
- Failed parses are coercion errors at exit code 2.

## Test

```sh
go test ./_examples/21-pluggable-types
```

Related: [20 — Modular predictors](../20-modular-predictors/),
[05 — Domain types](../05-domain-types/).
