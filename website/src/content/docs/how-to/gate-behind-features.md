---
title: How to gate flags and subcommands behind features
description: Declare named features, hide flags and subcommands until a feature is on, and let users switch features from the command line, the environment, or a config file.
---

A feature is a named setting, on or off or a numbered level, that the person running your CLI changes at run time. While its feature is on, a gated flag or subcommand appears in `--help` and shell completion and accepts input, and a gated flag gets a row in `--why`.

This guide builds the HTTP server from the [feature toggles example](https://github.com/conga-sh/conga/tree/main/_examples/29-feature-toggles). It declares four features, gates two flags and one subcommand behind them, and turns them on from each source.

## Declare the features

Declare the features in a field of the root command struct whose type is the container `conga.Features[...]`. The type in the brackets is a struct, and each of its exported fields is one feature, named after the field in kebab-case (`AdminAPI` becomes `admin-api`). A `bool` field is off unless its `default` tag says otherwise:

```go title="main.go"
type experiments struct {
	HTTP3     bool `stage:"alpha" help:"Experimental HTTP/3 (QUIC) listener"`
	AdminAPI  bool `stage:"alpha" help:"Experimental admin API"`
	StrictTLS bool `stage:"beta"  default:"true" help:"Require TLS and reject weak ciphers"`
	Metrics   bool `stage:"beta"  default:"true" help:"Expose the metrics endpoint"`
}

type CLI struct {
	Features conga.Features[experiments] `flag:"feature-gates" env:"API_FEATURES" config:"features"`
}
```

The three container tags name the places a user can set features from: the `--feature-gates` flag, the `API_FEATURES` variable, and the `features:` section of a config file. Each tag is optional, so declare only the sources your users need. For the environment, each feature can have its own variable instead, as [Give each feature its own environment variable](#give-each-feature-its-own-environment-variable) shows.

## Enable features

Turn on feature support by calling `EnableFeatures()` on the application builder:

```go title="main.go"
app := conga.New[CLI]().
	Name("feature-toggles").
	EnableFeatures().
	EnableWhy()
```

The root help screen now lists every feature, its stage, and its default:

```text no-copy
Features:
  http3       Experimental HTTP/3 (QUIC) listener [alpha]
  admin-api   Experimental admin API [alpha]
  strict-tls  Require TLS and reject weak ciphers [beta] [default: true]
  metrics     Expose the metrics endpoint [beta] [default: true]
```

## Gate a flag

Add a `feature` tag naming the feature the flag depends on. Here the feature is `http3` and the flag it gates is `--quic`:

```go title="main.go" ins={5}
type CLI struct {
	Features conga.Features[experiments] `flag:"feature-gates" env:"API_FEATURES" config:"features"`

	Listen conga.OptionalFlag[string] `default:":8080" help:"Listen address"`
	QUIC   conga.OptionalFlag[bool]   `feature:"http3"  help:"Enable the QUIC/HTTP3 listener (experimental)"`
}
```

`--quic` appears in `--help` once `http3` is on. Before that, typing it gives a usage error (exit code 2):

```sh error={2}
$ feature-toggles --quic
feature-toggles: error: flag '--quic' requires feature "http3"

Usage:
  feature-toggles [flags]

Run 'feature-toggles --help' for more information.
```

Turn the feature on and the flag appears in `--help` and works:

```sh
feature-toggles --feature-gates http3=true --quic
```

The `feature` tag works on `OptionalFlag`, `Counter`, `ConfigFile`, and `Command[T]` fields. A required flag can't be gated, because it would be required while it doesn't exist. Here `QUIC` was made a `RequiredFlag`:

```text no-copy error={1} wrap
feature-toggles: error: struct CLI field QUIC: schema error: invalid struct tag: tag "feature" cannot be used on required flags (feature-gated flags must be optional)
```

## Gate a subcommand

Put the same tag on a `Command[T]` field. The subcommand and everything beneath it disappear together:

```go title="main.go"
Admin conga.Command[AdminCmd] `feature:"admin-api" summary:"Experimental admin API"`
```

```sh error={2}
$ feature-toggles admin
feature-toggles: error: command "admin" requires feature "admin-api"

Usage:
  feature-toggles admin [flags]

Run 'feature-toggles admin --help' for more information.
```

## Show a flag only while a feature is off

Prefix the feature name with `!`. The example keeps a plaintext escape hatch that exists only while strict TLS is off:

```go title="main.go"
AllowInsecure conga.OptionalFlag[bool] `feature:"!strict-tls" help:"Serve plaintext (development only)"`
```

`strict-tls` defaults to on, so `--allow-insecure` appears once a user runs with `--feature-gates strict-tls=false`. Before that, the error says which state the feature must be in:

```sh error={2}
$ feature-toggles --allow-insecure
feature-toggles: error: flag '--allow-insecure' requires feature "strict-tls" to be off

Usage:
  feature-toggles [flags]

Run 'feature-toggles --help' for more information.
```

To require several conditions, separate them with commas. All of them must hold. `feature:"http3,!strict-tls"` gates a member on `http3` being on *and* `strict-tls` being off. The [expression table](/reference/struct-tags/#feature-expressions) lists the forms.

## Turn features on

Each source changes only the features it names, and the rest keep their values from lower sources.

From the command line:

```sh
feature-toggles --feature-gates http3=true,admin-api=true
```

From the environment, using the variable named by the container's `env` tag:

```sh
API_FEATURES=strict-tls=false feature-toggles --allow-insecure
```

With `envprefix`, each feature has its own variable instead (see [Give each feature its own environment variable](#give-each-feature-its-own-environment-variable)).

From a config file, under the section named by the container's `config` tag. The application must register a decoder for the file format (see [How to manage config files](/how-to/manage-config-files/)):

```yaml title="features.yaml"
features:
  http3: true
  admin-api: true
```

```sh
feature-toggles -c features.yaml admin
```

When two sources set the same feature, the command line beats the environment, which beats the config file, which beats the `default` tag. Here the environment turns `http3` off and the command line turns it back on:

```sh
$ API_FEATURES=http3=false feature-toggles --feature-gates http3=true
features: http3=true admin-api=false strict-tls=true metrics=true
```

On the command line and in the environment, users may spell a name in kebab-case, snake_case, or camelCase, so `admin-api`, `admin_api`, and `adminApi` are the same feature. A config file takes the feature name spelled in the app's config [key casing](/how-to/manage-config-files/#choose-the-key-casing) (`admin-api` by default), like every other config key:

```yaml title="features.yaml"
features:
  admin_api: true
```

```sh error={2} wrap
$ feature-toggles -c features.yaml
feature-toggles: error: config file "features.yaml": unknown field "admin_api" in section "features" (did you mean "admin-api"?)
```

## Check which source set each feature

Run with `--why` to see each feature's value and the source that set it:

```sh hidden={3-10,18-22}
$ API_FEATURES=admin-api=true feature-toggles -c features.yaml --why
Value provenance for "feature-toggles":

  FLAG              VALUE                        SOURCE    ENV            CONFIG PATH
  --config-file     features.yaml                cli       API_CONFIG     -
  --feature-gates   admin-api=true, http3=true   env       API_FEATURES   features
  --listen          :8080                        default   API_LISTEN     listen
  --quic                                         unset     API_QUIC       quic
  --cache           128                          default   API_CACHE      cache

Features:

  FEATURE      VALUE   SOURCE
  http3        true    config (features.yaml)
  admin-api    true    env
  strict-tls   true    default
  metrics      true    default

Config decoders: .yaml, .yml

Config paths:
  features.yaml   loaded
```

`unset` marks a flag or feature that keeps its zero value, because it has no `default` and no source sets it. See [How to inspect value provenance](/how-to/inspect-value-provenance/) for the rest of the report.

## Know what users see when input is wrong

Gated members and feature names are checked against every source. Each source reports problems in its own way:

| What the user does | Command line | Environment | Config file |
|---|---|---|---|
| Uses a gated flag or command while its feature is off | Usage error, exit 2 | Ignored | Config error, exit 1 |
| Sets a gated flag in a file whose feature `--feature-gates` or the environment turned off | — | — | Ignored, the run continues |
| Names an unknown feature (`htt3=true`) | Usage error, exit 2, with a "did you mean" hint | Config error, exit 1, with a hint (list variable only) | Config error, exit 1, with a hint |
| Gives a bad value (`http3=maybe`) | Usage error, exit 2 | Config error, exit 1, naming the variable | Config error, exit 1, naming the config key |

When a gated flag has its own variable, such as `API_QUIC` from `EnvPrefix("API")`, the environment skips it while the flag is inactive, so leftover variables from a parent shell or CI runner leave unrelated runs alone. Feature variables, whether the `API_FEATURES` list or per-feature variables such as `API_FEATURE_METRICS`, exist only to set features, so the program always validates them. They come from the environment, so their failures are `ErrConfig` with exit 1.

An unknown name or a bad value in `API_FEATURES` produces:

```sh error={2}
$ API_FEATURES=htt3=true feature-toggles
feature-toggles: error: unknown feature "htt3" (from environment variable API_FEATURES) (did you mean "http3"?)
```

```sh error={2}
$ API_FEATURES=http3=maybe feature-toggles
feature-toggles: error: invalid value "maybe" for feature "http3" (from environment variable API_FEATURES): must be true or false (or 1 or 0)
```

The config-file errors name the file:

```text no-copy error={1-2} wrap
feature-toggles: error: config file "features.yaml": unknown feature "htt3" in section "features" (did you mean "http3"?)
feature-toggles: error: config file "features.yaml": flag '--quic' requires feature "http3"
```

A file that sets `--quic` is still valid when the feature is off only because the command line or the environment turned it off. With `http3: true` and `quic: true` in `features.yaml`, `--feature-gates http3=false` runs without `--quic`:

```sh
$ feature-toggles -c features.yaml --feature-gates http3=false
features: http3=false admin-api=false strict-tls=true metrics=true
```

A bad value in the file names the key it came from and exits 1:

```text no-copy error={1} wrap
feature-toggles: error: invalid value "maybe" for feature "http3" (from config key "features.http3"): must be true or false (or 1 or 0)
```

## Read a feature's value

`Features.Get()` returns the resolved struct:

```go title="main.go"
func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	if c.Features.Get().Metrics {
		// start the metrics collector
	}

	return nil
}
```

A feature can also stand alone. `metrics` gates no member and is read only in code.

## Use a feature with levels

Declare an integer field and list its levels in `enum`. `0` means off, so list only the levels above it:

```go title="main.go"
Syntax int `stage:"beta" enum:"1,2" help:"Config syntax version (0 = off)"`
```

Users pick a level with `--feature-gates syntax=2`. The help screen lists the levels, and a value outside them gives a usage error:

```text no-copy
  syntax      Config syntax version (0 = off) [beta] [levels: 1, 2]
```

```text no-copy error={1}
feature-toggles: error: invalid value "3" for feature "syntax": must be one of: 0, 1, 2
```

A gate such as `feature:"syntax"` holds at every level from 1 up. To vary behavior by level, read it in code with `c.Features.Get().Syntax`.

## Give each feature its own environment variable

The `env` tag on the container gives users one variable holding a list of pairs. To give each feature its own variable instead, replace `env` with `envprefix`:

| Container tag | What the user sets |
|---|---|
| `env:"API_FEATURES"` | `API_FEATURES=http3=true,metrics=false` |
| `envprefix:"API_FEATURE"` | `API_FEATURE_HTTP3=true API_FEATURE_METRICS=false` |

Each feature's variable is the prefix plus its name in upper snake case. A field's own `env` tag renames its variable, and `env:"-"` leaves the feature to the command line and config file:

```go title="main.go" {2,4,9}
type experiments struct {
	HTTP3     bool `env:"ENABLE_HTTP3" stage:"alpha" help:"Experimental HTTP/3 (QUIC) listener"`
	AdminAPI  bool `stage:"alpha" help:"Experimental admin API"`
	StrictTLS bool `env:"-" stage:"beta" default:"true" help:"Require TLS and reject weak ciphers"`
	Metrics   bool `stage:"beta" default:"true" help:"Expose the metrics endpoint"`
}

type CLI struct {
	Features conga.Features[experiments] `flag:"feature-gates" envprefix:"API_FEATURE" config:"features"`
}
```

Users find the variables in the help screen, next to each feature:

```text no-copy
Features:
  http3       Experimental HTTP/3 (QUIC) listener [alpha] [env: ENABLE_HTTP3]
  admin-api   Experimental admin API [alpha] [env: API_FEATURE_ADMIN_API]
  strict-tls  Require TLS and reject weak ciphers [beta] [default: true]
  metrics     Expose the metrics endpoint [beta] [default: true] [env: API_FEATURE_METRICS]
```

Switching changes which variables the program reads, so tell your users before you switch:

- `envprefix` replaces the list variable. A container uses one form at a time, so the program reads `API_FEATURE_HTTP3` in place of `API_FEATURES`.
- A field's `env` tag renames its variable. With `env:"ENABLE_HTTP3"`, the program reads `ENABLE_HTTP3` in place of `API_FEATURE_HTTP3`.
- Users set `env:"-"` features through the container's other sources, here `--feature-gates` and the `features:` config section, for example `--feature-gates strict-tls=false`.
- Features take their variables from the container's `env` or `envprefix` tag alone. `EnvPrefix("API")` gives `--listen` the variable `API_LISTEN` and leaves features as they are.

The command line still beats these variables, one feature at a time. `ENABLE_HTTP3=false feature-toggles --feature-gates http3=true` runs with `http3` on.

The application checks at startup that each variable belongs to one feature. Here `AdminAPI` was also given `env:"ENABLE_HTTP3"`:

```text no-copy error={1} wrap
feature-toggles: error: struct CLI field Features: schema error: environment variable "ENABLE_HTTP3" is bound by both feature "http3" and feature "admin-api"
```

A feature's variable can't also belong to a flag. With `env:"API_LISTEN"` on `AdminAPI` instead, the variable of `--listen`:

```text no-copy error={1} wrap
feature-toggles: error: struct CLI field Features: schema error: environment variable "API_LISTEN" is bound by both feature "admin-api" and flag --listen
```

## Mark how mature each feature is

The `stage` tag labels a feature `alpha`, `beta`, `ga`, or `deprecated`, and the label appears in the help screen's `Features:` list. Once you set `stage` on one feature you must set it on all of them.

Features staged `ga` or `deprecated` drop out of the help list but keep working, and users can still set them and see them in `--why`. [About feature toggles](/explanation/feature-toggles/#feature-maturity-stages) explains why.

## Where to go next

- [About feature toggles](/explanation/feature-toggles/): why features are typed struct fields and why sources merge feature by feature.
- [Struct tags](/reference/struct-tags/#feature-toggles-congafeaturest): every container tag, feature-field tag, and expression form.
- [About value precedence](/explanation/value-precedence/): how command line, environment, config, and defaults combine.
- [Feature toggles example](https://github.com/conga-sh/conga/tree/main/_examples/29-feature-toggles): the program used on this page.
