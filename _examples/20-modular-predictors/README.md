# 20 — Modular predictors

Nested command groups using the modular completion domain types from
`system` (`User`, `Group`, `Process`, `PID`, `Signal`, `EnvVar`,
`Locale`) and `network` (`NetInterface`, `Host`, `Port`, `HTTPStatus`,
`MIMEType`, `HTTPMethod`). No predictor wiring is needed: the types complete
themselves.

## Run it

```sh
go run ./_examples/20-modular-predictors system kill -s SIGKILL 42
go run ./_examples/20-modular-predictors network probe --resolve example.com
go run ./_examples/20-modular-predictors network serve -p 9090
go run ./_examples/20-modular-predictors --help
```

## What to look for

- Type methods beyond completion: `PID.Int()`, `Signal.Signal()`,
  `HTTPStatus.Code()`/`IsSuccess()`, `Port.IsPrivileged()`,
  `MIMEType.Type()`/`Subtype()`, `User.Lookup()`, `EnvVar.Value()`.
- Aliased subcommands (`p`, `s`, `k`, `i`) and nested groups.

## Test

```sh
go test ./_examples/20-modular-predictors
```

Related: [19 — Shell completion](../19-shell-completion/),
[21 — Pluggable types](../21-pluggable-types/).
