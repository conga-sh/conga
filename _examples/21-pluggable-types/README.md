# 21 — Pluggable types

Pluggable type parsers in three forms: `app.Use` plugins
(`plugin/timezone.Location`, `plugin/system.Signal`), standard library types
parsed natively (`*time.Location`, `syscall.Signal`), and a foreign type wired
through `app.RegisterType` with `conga.WithPredictor` (`Percent`).

## Run it

```sh
go run ./_examples/21-pluggable-types -z Europe/Berlin -s HUP
go run ./_examples/21-pluggable-types --load 75%
go run ./_examples/21-pluggable-types --help
```

## What to look for

- Plugins and stdlib types resolve to the same underlying values
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
