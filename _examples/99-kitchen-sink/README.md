# 99 — Kitchen sink

The exhaustive reference example and the last stop of the tour: every primitive
width, the filesystem and value domain types (`File`, `Dir`, `ByteSize`,
`time.Duration`, `url.URL`, maps), all four decoder formats over descriptive
nested subcommands, config-key remap, map encodings, deep nested sections,
`Changed()`/`Source()` provenance, feature toggles, environment binding, `.env`
loading, and the schema directives (`conga:"ignore"`, `allow-unknown-tags,recurse`,
`AllowTags`).

The [`configs/`](configs/) directory doubles as browsable documentation: each
file maps the same schema in a different format, including a `features` section.

## Run it

```sh
go run ./_examples/99-kitchen-sink --config-file configs/config.toml
go run ./_examples/99-kitchen-sink --config-file configs/config.yaml types primitives
go run ./_examples/99-kitchen-sink --config-file configs/config.toml config maps
go run ./_examples/99-kitchen-sink --config-file configs/config.toml config maps --why
go run ./_examples/99-kitchen-sink --config-file configs/config.toml advanced provenance
go run ./_examples/99-kitchen-sink --config-file configs/config.toml feature-state
go run ./_examples/99-kitchen-sink --feature-gates http3=true experimental --probe
go run ./_examples/99-kitchen-sink --why=json
go run ./_examples/99-kitchen-sink --help
```

The complete scenarios, fixtures included, live in
[`testdata/kitchen.txt`](testdata/kitchen.txt) and
[`testdata/dotenv.txt`](testdata/dotenv.txt).

## What to look for

- The same values bound from the TOML, JSON, YAML, and HCL variants of the
  schema, including `api_secret` → `--secret-key` remapping.
- Every `● config` origin detail naming the exact file that supplied the value.
- `--why` lists the loaded file and the contributing file for each value,
  and redacts the API secret and the env-var carrier.
- Feature toggles declared once on the root: `KITCHEN_SINK_FEATURE_<NAME>`
  variables derived from the container `envprefix`, a field `env` override
  (`KITCHEN_SINK_STRICT_TLS`), an `env:"-"` opt-out (metrics), and a leveled
  integer gate (`compression`) that accepts only its enumerated levels.
- Gated members (`--http3-port`, the `experimental` subcommand) are pruned from
  `--help`, `--why`, and completion until the `http3` feature is active;
  invoking them while inactive is a usage error (exit 2).
- Flag environment binding: `EnvPrefix("KITCHEN_SINK")` derives names, an `env`
  tag overrides one, and `env:"-"` removes the tier (`--temp-flag`). The same
  prefix selects the config file via `KITCHEN_SINK_CONFIG`.
- `.env` loading through `App.Dotenv()` feeding both flag and feature variables
  (see [`testdata/dotenv.txt`](testdata/dotenv.txt)); the writer is `os.Setenv`
  by default and overridable with `App.SetenvFunc` for tests.
- `--why=json` machine-readable provenance, opted in with `conga.WhyJSON()`.
- Help omits `internal` (ignored command), plain ignored fields, and inactive
  gated members.

## Test

```sh
go test ./_examples/99-kitchen-sink
```

Related: the whole tour above, starting at [01 — Minimal hello](../01-minimal-hello/).
Feature toggles in depth: [29 — Feature toggles](../29-feature-toggles/).
Environment precedence in depth: [13 — Twelve-factor precedence](../13-twelve-factor-precedence/).
Multi-format config and `.env`: [14 — Multiformat and dotenv](../14-multiformat-and-dotenv/).
