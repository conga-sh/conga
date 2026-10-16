# 13 — Twelve-factor precedence

The four-tier resolution ladder — CLI flag > environment variable > config file
> struct default — plus `EnvPrefix`, `env:"-"`, `placeholder`, `config:"-"`,
the built-in conventional XDG tier, and the `--why` provenance report.

## Run it

```sh
go run ./_examples/13-twelve-factor-precedence
PORT=9001 go run ./_examples/13-twelve-factor-precedence
go run ./_examples/13-twelve-factor-precedence -p 9003 --why
```

The complete scenarios, fixtures included, live in
[`testdata/precedence.txt`](testdata/precedence.txt).

## What to look for

- Each value's origin badge (`● cli`, `● env PORT`, `● config server.toml`,
  `· default`) in the inspector output.
- The built-in tier probes `${XDG_CONFIG_HOME:-~/.config}/server-precedence`.
- `config:"-"` fields are rejected inside config files (exit 1).

## Test

```sh
go test ./_examples/13-twelve-factor-precedence
```

Related: [15 — Cascading search paths](../15-cascading-searchpaths/),
[18 — Config search paths](../18-config-searchpaths/).
