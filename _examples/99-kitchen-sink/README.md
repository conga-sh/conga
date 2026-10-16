# 99 — Kitchen sink

The exhaustive reference example and the last stop of the tour: every primitive
width, the filesystem and value domain types (`File`, `Dir`, `ByteSize`,
`time.Duration`, `url.URL`, maps), all four decoder formats over descriptive
nested subcommands, config-key remap, map encodings, deep nested sections,
`Changed()`/`Source()` provenance, and the schema directives (`conga:"ignore"`,
`allow-unknown-tags,recurse`, `AllowTags`).

The [`configs/`](configs/) directory doubles as browsable documentation: each
file maps the same schema in a different format.

## Run it

```sh
go run ./_examples/99-kitchen-sink --config-file configs/config.toml
go run ./_examples/99-kitchen-sink --config-file configs/config.yaml types primitives
go run ./_examples/99-kitchen-sink --config-file configs/config.toml config maps
go run ./_examples/99-kitchen-sink --config-file configs/config.toml config maps --why
go run ./_examples/99-kitchen-sink --config-file configs/config.toml advanced provenance
go run ./_examples/99-kitchen-sink --help
```

The complete scenarios, fixtures included, live in
[`testdata/kitchen.txt`](testdata/kitchen.txt).

## What to look for

- The same values bound from the TOML, JSON, YAML, and HCL variants of the
  schema, including `api_secret` → `--secret-key` remapping.
- Every `● config` origin detail naming the exact key that supplied the value.
- `--why` lists the loaded file, shows per-key file origins for map values, and
  redacts the API secret and the env-var carrier.
- Help omits `internal` (ignored command) and plain ignored fields.

## Test

```sh
go test ./_examples/99-kitchen-sink
```

Related: the whole tour above, starting at [01 — Minimal hello](../01-minimal-hello/).
