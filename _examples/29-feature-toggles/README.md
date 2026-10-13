# 29 — Feature toggles

Gate flags and whole subcommands behind named features with the `feature:"..."`
struct tag. Features are declared once on the root struct through a
`conga.Features[T]` field; each field of `T` becomes a toggle named after the
field (kebab-cased). A feature is switched on by a struct default (`default`
tag), a configuration section (`config` tag), an environment variable
(`envprefix`/`env` tags), or the CLI map flag (`flag` tag); the CLI outranks the
environment, which outranks configuration, which outranks the default. Inactive
members are invisible in `--help`, `--why`, and shell completion, and invoking
them is a usage error.

The example is an HTTP API server whose gates each hide a capability that would
not ship unconditionally: an experimental HTTP/3 listener, the experimental
admin API, and a plaintext escape hatch that strict TLS retires.

## Run it

```sh
go run ./_examples/29-feature-toggles
go run ./_examples/29-feature-toggles --help
go run ./_examples/29-feature-toggles --http3                              # exit 2: gated flag
go run ./_examples/29-feature-toggles --feature-gates http3=true --http3
SERVE_FEATURES=strict-tls=false go run ./_examples/29-feature-toggles --allow-insecure
SERVE_FEATURES=admin-api=true go run ./_examples/29-feature-toggles admin
```

Configuration activation uses the `features` section:

```sh
printf 'features:\n  http3: true\n  admin-api: true\n' > features.yaml
go run ./_examples/29-feature-toggles -c features.yaml admin
```

## What to look for

- By default `--http3` and `--allow-insecure` are absent from `--help`, as is
  the `admin` command. `strict-tls` and `metrics` are declared with
  `default:"true"`, so strict TLS is on out of the box.
- `--http3` declares `feature:"http3"`: an experimental transport stays hidden
  until an operator opts in.
- `--allow-insecure` declares `feature:"!strict-tls"`: the legacy plaintext
  escape hatch exists only while strict TLS is disabled, and disappears the
  moment the default-on security feature is in force.
- `admin` declares `feature:"admin-api"`: the whole experimental subcommand is
  pruned until the feature is enabled.
- `metrics` has no gated member; it is declared as an ordinary feature field and
  consumed through `Features.Get()`, while an environment variable can switch
  the default back off.
- Inactive gated flags and commands are pruned from `--why` and `__complete`
  candidates; the gated members still exist as struct fields (shown unset by
  the inspector) but are never bound.
- A config key that names an inactive gated flag fails with exit code 1, and an
  unknown feature name in the section fails with a did-you-mean hint.

The complete scenarios live in [`testdata/`](testdata/):
[`baseline.txt`](testdata/baseline.txt) (default posture and enforcement),
[`env.txt`](testdata/env.txt) (environment activation),
[`config.txt`](testdata/config.txt) (configuration activation and validation),
and [`completion.txt`](testdata/completion.txt) (completion pruning).

## Test

```sh
go test ./_examples/29-feature-toggles
```

Related: [13 — Twelve-factor precedence](../13-twelve-factor-precedence/),
[14 — Multiformat and dotenv](../14-multiformat-and-dotenv/),
[19 — Shell completion](../19-shell-completion/),
[23 — Member deprecation](../23-member-deprecation/).
