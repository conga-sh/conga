# 30 — Programmatic config loader

`app.ConfigOptions(conga.ConfigLoader(...))` supplies the configuration tier from
code instead of files. `flowctl` runs named pipelines, and its pipeline files
are mixed documents: the top-level `parallelism`, `log_level`, and `env` keys
are configuration, while `pipelines` is domain data that `Run` consumes. The
loader turns the configuration keys into a `conga.ConfigDocument`, so they bind
at the config tier (`CLI > environment > config > default`) and appear in
`--why` with the file and pipeline that supplied them.

## Why a loader

A decoder maps one file onto the flag schema. A pipeline file is not a flag
schema: `steps`, `needs`, and `desc` are domain data, and the config keys of the
selected pipeline have to overlay the file's top-level keys. The loader also
concatenates `steps` and `needs` across files — the config tiers replace slices
instead of merging them — and picks the pipeline overlay from `--pipeline`
before any flag binds.

## Run it

```sh
go run ./_examples/30-config-loader -f _examples/30-config-loader/testdata/fixtures/pipeline.yaml
go run ./_examples/30-config-loader -f _examples/30-config-loader/testdata/fixtures/pipeline.yaml --pipeline deploy
go run ./_examples/30-config-loader -f _examples/30-config-loader/testdata/fixtures/pipeline.yaml -f _examples/30-config-loader/testdata/fixtures/pipeline.local.yaml --pipeline deploy
go run ./_examples/30-config-loader -f _examples/30-config-loader/testdata/fixtures/pipeline.yaml -f _examples/30-config-loader/testdata/fixtures/pipeline.local.yaml --pipeline deploy --why
FLOWCTL_LOG_LEVEL=debug go run ./_examples/30-config-loader -f _examples/30-config-loader/testdata/fixtures/pipeline.yaml --pipeline deploy
```

The complete scenarios, fixtures included, live in
[`testdata/loader.txt`](testdata/loader.txt) and
[`testdata/env.txt`](testdata/env.txt).

## What to look for

- The file's top-level config keys bind with `● config pipeline.yaml`
  provenance; they outrank `default` tags and lose to `FLOWCTL_*` environment
  variables and CLI flags.
- Selecting a pipeline overlays its `parallelism`/`log_level`/`env` keys, and
  `--why` attributes every map entry to the file and pipeline section that last
  wrote it (`config (pipeline.local.yaml › pipeline deploy)`), while the Config
  paths table still lists each file once.
- With two files, scalar config is nearest-file-wins, while `steps` and `needs`
  concatenate (`needs` deduplicated) — a merge the config tier cannot express.
- Pipeline selection is `--pipeline` (environment fallback `FLOWCTL_PIPELINE`);
  file selection is `-f/--file` (environment fallback `FLOWCTL_FILE`).
- Broken loader input is app-authored data: exit 1 for a missing file, an
  unsupported `version`, an unknown key, or an unknown pipeline.

## Test

```sh
go test ./_examples/30-config-loader
```

Related: [13 — Twelve-factor precedence](../13-twelve-factor-precedence/),
[15 — Cascading search paths](../15-cascading-searchpaths/),
[17 — Config profiles](../17-config-profiles/).
