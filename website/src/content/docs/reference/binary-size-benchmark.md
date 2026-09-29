---
title: Binary size benchmark
description: Measured ladder of a shared spec CLI across flag, pflag, cobra, kong, urfave/cli, and CONGA.
---

Sizes for a shared spec CLI built identically across frameworks. Every build is measured on `go1.27.1 linux/amd64`, `CGO_ENABLED=0`, `go build -trimpath -buildvcs=false -ldflags="-s -w"`. Numbers are a point-in-time snapshot; regenerate them with `make report` in `bench/comparison`. The conceptual reading is in [About binary size and linking](/explanation/binary-size/).

## Build ladder

Each rung adds one battery through that framework's idiomatic mechanism — viper for cobra's env and config, `altsrc` for urfave, `kong.Configuration` for kong, the opt-in builder calls for CONGA. `flag` and `pflag` stop at their natural ceiling; they do not implement env binding, required validation, or version.

| build | file bytes | rung | mechanism |
|---|---:|---|---|
| hello (fmt only) | 1,507,488 | toolchain floor | `fmt` only |
| flag (stdlib) | 1,626,272 | flags, defaults, `-h` | stdlib `flag` |
| pflag | 1,933,472 | + shorthands, slices | pflag |
| cobra | 2,551,968 | + commands, required, version, completion | stock cobra |
| cobraviper | 6,066,336 | + env, config file | viper `AutomaticEnv` + `benchctl.toml` |
| CONGA core | 2,580,640 | env, required, aliases, help, version | CONGA core |
| CONGA (full batteries) | 3,551,392 | + completion + config + why | all three opt-ins |
| urfave | 4,231,328 | full native core | urfave/cli v3 |
| urfave + config | 8,777,888 | + config file | `altsrc/v3` TOML value sources |
| kong | 4,264,096 | full native core | kong struct tags |
| kong + config | 4,518,048 | + config file | `kong.Configuration(kongtoml.Loader, ...)` |

The `cobracore` and `urfave + completion` builds are byte-identical to `cobra` and `urfave` respectively, so they are collapsed into one row each; see [About binary size and linking](/explanation/binary-size/) for why.

## Feature matrix

| feature | CONGA | cobra | cobra + viper | kong | urfave v3 | flag | pflag |
|---|---|---|---|---|---|---|---|
| flags + defaults | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| shorthand aliases | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| subcommands + alias | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| help | ✅ | ✅ | ✅ | ✅ | ✅ | 🟡 `-h` only | 🟡 `-h` only |
| version flag | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| required validation | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| native env binding | ✅ | ❌ | 🟡 viper | ✅ | ✅ | ❌ | ❌ |

✅ full · 🟡 limited · ❌ absent

Batteries:

| battery | CONGA | cobra | kong | urfave v3 |
|---|---|---|---|---|
| shell completion | ✅ `EnableCompletion()` | 🔁 default command | ❌ | 🔁 `EnableShellCompletion` |
| config file | ✅ `.Dotenv()` + decoders | 🔁 viper | 🔁 `kong.Configuration` | 🔁 altsrc/v3 |
| provenance (`--why`) | ✅ `EnableWhy()` | ❌ | ❌ | ❌ |

✅ link-time opt-in (dropped when unused) · 🔁 runtime toggle (linked either way) · ❌ not shipped

## CONGA battery ladder

Each opt-in priced against `CONGA core` (2,580,640 bytes):

| opt-in | file bytes | Δ vs CONGA core |
|---|---:|---:|
| `EnableCompletion()` | 2,990,240 | +409,600 (+15.9%) |
| `EnableWhy()` | 2,633,888 | +53,248 (+2.1%) |
| `.Dotenv()` + decoder | 3,154,080 | +573,440 (+22.2%) |
| full batteries | 3,551,392 | +970,752 (+37.6%) |

## Methodology

- **Environment:** `go1.27.1 linux/amd64`, `CGO_ENABLED=0`, `go build -trimpath -buildvcs=false -ldflags="-s -w"`. Unstripped builds are larger; expect drift across Go releases.
- **Modules:** `github.com/alecthomas/kong v1.16.1`, `github.com/alecthomas/kong-toml v0.4.0`, CONGA at the working tree (local `replace`), `github.com/spf13/cobra v1.10.2`, `github.com/spf13/pflag v1.0.10`, `github.com/spf13/viper v1.21.0`, `github.com/urfave/cli-altsrc/v3 v3.1.0`, `github.com/urfave/cli/v3 v3.13.0`.
- **Conformance:** a 15-case suite (`conformance.sh`) pins behavioural parity across the capability tiers — defaults, aliases, custom values, serve, missing-required, help, version, env binding, config-file precedence, completion, and `--why` — with each case run only on the builds that claim the capability.
- **Precedence:** kong's resolver orders configuration above the `env` tag (`kong.Configuration` overwrites the environment), so it has its own conformance case; cobra + viper, urfave + altsrc, and CONGA all give env precedence over the file, and flags precedence over both.
- **Isolation:** the harness is a nested module (`bench/comparison/go.mod`) with a local `replace` to the repository root, so competitor dependencies never enter CONGA's `go.mod` or its `depguard` boundaries.
- **Reproduce:** `cd bench/comparison && make conformance` (behaviour), `make report` (regenerate tables), `make sizes` (raw size + per-package `.text`).