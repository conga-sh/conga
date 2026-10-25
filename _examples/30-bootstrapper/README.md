# 30 — Bootstrapping flag-selected files

`flowctl` reads pipeline files selected by `-f/--file` and runs the pipeline
named by `--pipeline`, but CONGA normally resolves configuration before it binds
flags. The `-f` value is therefore unavailable when file resolution starts. A
`conga.Bootstrapper` breaks that cycle: CONGA binds only the fields tagged
`bootstrap:"true"`, runs `Bootstrap`, then performs the normal binding pass with
the configuration tier available.

`Bootstrap` uses that early access to read the pipeline files. Their top-level
`parallelism`, `log_level`, and `env` keys become a `conga.ConfigDocument`
installed with `b.SetConfig`, while `pipelines` merges into `c.pipelines` as
domain data for `Run`. `SetConfig` contributes to the config tier; it does not
replace file-based configuration, so the hook composes with decoders and search
tiers.

## The two binding passes

`-f/--file` and `--pipeline` are the only fields tagged `bootstrap:"true"`. They
are bound twice on a normal run:

- Bootstrap pass: `CLI > environment > default`. Configuration files are not
  resolved yet, so a value supplied by a config file is unavailable here.
- Normal pass: `CLI > environment > config > default`. Every field, tagged or
  not, is bound before `Run`.

Tag only the fields needed to locate or select the external document. Keep the
hook deterministic and limited to bounded local I/O: it also runs for `--why`
and shell completion.

## Why a Bootstrapper

A decoder maps one file onto the flag schema. A pipeline file is not a flag
schema: `steps`, `needs`, and `desc` are domain data, and the config keys of the
selected pipeline overlay the file's top-level keys. `Bootstrap` also
concatenates `steps` and `needs` across files — the config tier replaces slices
instead of merging them — and picks the pipeline overlay from `--pipeline`
before the full binding pass.

## Run it

```sh
go run ./_examples/30-bootstrapper -f _examples/30-bootstrapper/testdata/fixtures/pipeline.yaml
go run ./_examples/30-bootstrapper -f _examples/30-bootstrapper/testdata/fixtures/pipeline.yaml --pipeline deploy
go run ./_examples/30-bootstrapper -f _examples/30-bootstrapper/testdata/fixtures/pipeline.yaml -f _examples/30-bootstrapper/testdata/fixtures/pipeline.local.yaml --pipeline deploy
go run ./_examples/30-bootstrapper -f _examples/30-bootstrapper/testdata/fixtures/pipeline.yaml -f _examples/30-bootstrapper/testdata/fixtures/pipeline.local.yaml --pipeline deploy --why
FLOWCTL_LOG_LEVEL=debug go run ./_examples/30-bootstrapper -f _examples/30-bootstrapper/testdata/fixtures/pipeline.yaml --pipeline deploy
```

The complete scenarios, fixtures included, live in
[`testdata/bootstrap.txt`](testdata/bootstrap.txt) and
[`testdata/env.txt`](testdata/env.txt).

## What to look for

- `-f/--file` and `--pipeline` are visible inside `Bootstrap` through the
  bootstrap pass and again in `Run` through the normal pass; environment
  fallbacks and defaults resolve in both.
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
- Broken bootstrap input is app-authored data: exit 1 for a missing file, an
  unsupported `version`, an unknown key, or an unknown pipeline.

## Test

```sh
go test ./_examples/30-bootstrapper
```

Related: [13 — Twelve-factor precedence](../13-twelve-factor-precedence/),
[15 — Cascading search paths](../15-cascading-searchpaths/),
[17 — Config profiles](../17-config-profiles/).
