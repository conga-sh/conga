# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `conga.ConfigLoader` lets an application supply the run's configuration tier from code. `app.ConfigOptions(conga.ConfigLoader(load))` replaces file resolution with the `conga.ConfigDocument` the loader returns (built with `Set(key, value, origin)`), validated exactly like a decoded config file. `conga.ConfigContext` exposes the invocation facts at loader time (`CommandPath`, `FlagValue`, `FlagValues`, `LookupEnv`), values keep the config rank (`CLI > environment > document > default`), map flags merge key by key, per-entry origins surface in `--why`, feature gates ride the document, and loader failures wrap `ErrConfig` (exit 1). Loader applications must not also register decoders, declare file resolution options, or declare a root `ConfigFile` field; schema build rejects the combination with `ErrSchema` and `conga-lint` mirrors the guard.
- `App.EnableFeatures()` activates struct-declared feature toggles: it installs the feature resolver and validator hooks, and is required for any application declaring a `conga.Features[T]` container or a `feature:"..."` tag.
- The `--why` provenance flag accepts an optional output format selector: bare `--why` or `--why=text` renders human-readable tables. `app.EnableWhy(conga.WhyJSON())` additionally enables `--why=json`, which emits a versioned machine-readable document (`$schema`, `schemaVersion`, `command`, `version`, `timestamp`, `workingDirectory`, `flags`, `globalFlags`, `features`, `config`, `redacted`). Non-format values and `--why=json` without the option are rejected with a usage error (exit 2).
- `Session.Why()` and `Session.WhyJSON()` expose the same provenance report programmatically inside `PreRun`, `Run`, and other lifecycle hooks. They return an empty string when Why (or the JSON option, for `WhyJSON`) is not enabled on the application.

### Changed

- **Breaking**: feature toggles are now opt-in. An application declaring a `conga.Features[T]` container or a `feature:"..."` tag without calling `app.EnableFeatures()` is rejected at schema build with `ErrSchema`, so binaries that never enable features keep the feature machinery unlinked.
- **Breaking**: map flags (`map[K]V` fields, registered `RegisterMap` types, and the synthesized `conga.Features[T]` map) now merge key-by-key across all source tiers (`default` < config < env < CLI) instead of one winning tier supplying the whole map. A config file can set one key while the command line sets another, and both apply; a higher tier overrides only the keys it names. Feature keys are canonicalized to the declared name, so any accepted spelling (`gentle-force`, `gentle_force`, `gentleForce`) overrides across tiers. Scalars and slices keep winning-tier semantics. A malformed map or unknown feature key in a lower tier (e.g. config) now errors even when a higher tier also sets the flag, attributed to the tier that supplied it.
- **Breaking**: merged `predict/{network,system,timezone}` and `plugin/{network,system,timezone}` into three topical kits — `network`, `system`, and `timezone`. Each kit bundles its domain types, autocompletion predictors, and standard library bindings in one package.
- **Breaking**: renamed the standard library bindings to `network.RegisterInterface`, `system.RegisterUser`, `system.RegisterGroup`, `system.RegisterSignal`, and `timezone.RegisterLocation`. This resolves the previous `system.User` type-versus-function collision.
- **Breaking**: `SourceDetail()` for config-sourced values now reports the contributing origin instead of the config key: the config file path as resolved, or the loader label (`Set(key, value, origin)`), falling back to the matched config key only when a programmatic loader supplies no origin. A merged map container lists every contributing file in load order, separated by commas. `Source()` is unchanged, and coercion-error attribution and the deprecation warning keep naming the config key.
- Positional-argument `enum` validation now checks the coerced value's string form, matching flag validation, so normalizing `encoding.TextUnmarshaler` types compose with the `enum` tag.
- The `--why` report renders one indented sub-row per map key with the tier that last wrote it, and the Features table reports each toggle's actual source tier instead of the winning tier of the whole map.
- The `--why` report annotates every config-sourced row with the contributing file (`config (app.yaml)`), and an entry-bearing map row renders `-` for its value and source instead of repeating what its per-key sub-rows state.
- The `--why` report now binds through the normal resolution pipeline (including on the built-in `version` and `completion` commands) and fails fast on invalid configuration, coercion, or feature input instead of rendering partial diagnostic rows.
- Multi-file `ConfigFile` rows render `(N files)` with tree sub-rows listing each resolved path; the sequential `[1]`, `[2]` configuration-origin indices and the `Config files:` legend are removed in favor of directly reported origins.

### Fixed

- `conga.ConfigCascading()` now merges map flags key by key across the cascaded config files themselves: a key set only by a lower-precedence file survives instead of being silently dropped when a higher-precedence file rewrites the flag. The nearest file wins each overlapping key. Feature gates (`conga.Features[T]`) merge per key the same way, and gate-off pruning is unaffected. Scalar and slice flags keep nearest-file-wins semantics. Map flags behind a dotted `config:` namespace key now bind their nested section (previously rejected as an unknown field), and values containing the flag's `sep` survive the nested-map form without separator round-tripping. Array-valued map sections split per element regardless of the flag's `sep`, so a custom separator no longer swallows later elements. Config sections addressed at a flag excluded with `config:"-"` are now rejected like their scalar counterparts. The `--why` report attributes each merged map key to the config file that last wrote it.
