# 17 — Config profiles

Multi-file merging with `conga.ConfigFile` and `multi:"true"`: repeated `-c`
flags and one comma-separated list both merge with later files winning.
`conga.ConfigCascading()` makes the fallback tier merge every existing file instead
of stopping at the first.

## Run it

```sh
go run ./_examples/17-config-profiles -c deploy/base.yaml -c deploy/prod.yaml
go run ./_examples/17-config-profiles -c deploy/base.yaml -c deploy/prod.yaml --why
go run ./_examples/17-config-profiles -c 'deploy/base.yaml,deploy/prod.yaml'
go run ./_examples/17-config-profiles
```

Create the fixture files first (`deploy/base.yaml`, `deploy/prod.yaml`); the
exact contents are in [`testdata/profiles.txt`](testdata/profiles.txt).

## What to look for

- `port: 9443` from `prod.yaml` overrides `port: 9000` from `base.yaml`.
- The config-file container reports every loaded file in order.
- Without `-c`, the declared fallback paths cascade in lowest-first order.
- `--why` lists both loaded files and names the contributing file for each value.

## Test

```sh
go test ./_examples/17-config-profiles
```

Related: [15 — Cascading search paths](../15-cascading-searchpaths/),
[99 — Kitchen sink](../99-kitchen-sink/).
