# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While CONGA is pre-1.0, breaking changes increment the minor version.

## [0.4.1](https://github.com/conga-sh/conga/compare/v0.4.0...v0.4.1) (2026-10-31)


### 📚 Documentation

* **website:** document flag grouping, built-in short flags, and help wrapping ([2d443b4](https://github.com/conga-sh/conga/commit/2d443b438697375695f6f8b8a9a6d674186a6dd6))

## [0.4.0](https://github.com/conga-sh/conga/compare/v0.3.0...v0.4.0) (2026-10-30)


### ⚠ BREAKING CHANGES

* `default` and `xor` tags on `RequiredFlag` are now schema errors. Use `oneof` groups with optional flags instead.
* **api:** WhyJSON() is now WhyOptionJSON() and RegisterTypeOption is now RegistrationOption; update callers, signatures, and docs accordingly.
* **completion:** CONGA_COMPLETION_ALIASES and CONGA_COMPLETION_DESCRIPTIONS are replaced by the application-prefixed variants; update shell configuration accordingly.

### ✨ Features

* **completion:** app-prefixed sideband environment variables ([#82](https://github.com/conga-sh/conga/issues/82)) ([4b57717](https://github.com/conga-sh/conga/commit/4b57717c5bf2d9e63307a0ad19b810c6d17f3dd6)), closes [#81](https://github.com/conga-sh/conga/issues/81)
* **completion:** suppress bare-space flags uniformly across shells ([#86](https://github.com/conga-sh/conga/issues/86)) ([bcd58b5](https://github.com/conga-sh/conga/commit/bcd58b531b3db74d364dbacb092cdcb520b83cf4))
* help wrapping, configurable built-in shorts, oneof groups, and directive removal ([#88](https://github.com/conga-sh/conga/issues/88)) ([81e2ad6](https://github.com/conga-sh/conga/commit/81e2ad6e24f5f58d9daf4d616d659aeab5adf7e6))


### ♻️ Code Refactoring

* **api:** rename WhyJSON and RegisterTypeOption ([#84](https://github.com/conga-sh/conga/issues/84)) ([c6122f0](https://github.com/conga-sh/conga/commit/c6122f0d62dbcbe291dd22a668de358594cd2c5b))

## [0.3.0](https://github.com/conga-sh/conga/compare/v0.2.0...v0.3.0) (2026-10-26)


### ⚠ BREAKING CHANGES

* replace ConfigLoader with Bootstrapper lifecycle hook ([#70](https://github.com/conga-sh/conga/issues/70))

### ✨ Features

* **completion:** add alias and description suppression sidebands ([#76](https://github.com/conga-sh/conga/issues/76)) ([759eba1](https://github.com/conga-sh/conga/commit/759eba1c64efd1b6ed5deb6250fed999d9dc78ea))
* replace ConfigLoader with Bootstrapper lifecycle hook ([#70](https://github.com/conga-sh/conga/issues/70)) ([9b34068](https://github.com/conga-sh/conga/commit/9b34068b03fbbdb96604aae557bb759cf3930f86))


### 🐛 Bug Fixes

* **completion:** honor zsh list-separator in list-colors patterns ([#77](https://github.com/conga-sh/conga/issues/77)) ([6a0ae76](https://github.com/conga-sh/conga/commit/6a0ae769f26f815c49830eceb9cf4ac938984ad6))


### 📚 Documentation

* **website:** reframe bootstrap docs as domain document loading ([#78](https://github.com/conga-sh/conga/issues/78)) ([ee9fb06](https://github.com/conga-sh/conga/commit/ee9fb062d098a3e94e2d5969672bd9d6d824f804))

## [0.2.0](https://github.com/conga-sh/conga/compare/v0.1.1...v0.2.0) (2026-10-24)


### ⚠ BREAKING CHANGES

* remove StrictTagValidation and allow-unknown-tags directive ([#58](https://github.com/conga-sh/conga/issues/58))
* `{{env}}` on a Features container now substitutes only the combined-map `env` variable; schemas that relied on it to list `envprefix`-derived or field `env` names now fail at schema build.

### ✨ Features

* **dotenv:** add App.SetenvFunc to override the env writer ([#55](https://github.com/conga-sh/conga/issues/55)) ([6a81466](https://github.com/conga-sh/conga/commit/6a8146642c08e19a6dd348de6922f79f56e09d6b))
* **features:** support per-feature environment variables ([#53](https://github.com/conga-sh/conga/issues/53)) ([97db132](https://github.com/conga-sh/conga/commit/97db132fd5845f38c31bc9c28822412acfaaf389))
* show per-feature env vars in help and fold per-feature env map tier ([#57](https://github.com/conga-sh/conga/issues/57)) ([2074cf8](https://github.com/conga-sh/conga/commit/2074cf8f1850ac4fe5857954340f1739269fcfa9))
* **spec:** show registered decoders in config-file flag help ([#59](https://github.com/conga-sh/conga/issues/59)) ([4bfa372](https://github.com/conga-sh/conga/commit/4bfa372f2b9a953e507579b32c55c5d75b09f828))
* **spec:** support aliases and group tags on builtin subcommands ([#66](https://github.com/conga-sh/conga/issues/66)) ([d14d383](https://github.com/conga-sh/conga/commit/d14d383ee6b5a05b3413647dc4a9aff9542b2481))
* **why:** advertise --why formats only when JSON is enabled ([#61](https://github.com/conga-sh/conga/issues/61)) ([5d54755](https://github.com/conga-sh/conga/commit/5d54755e314e6b113e94b04db7be6d27adeb9c2a))


### 🐛 Bug Fixes

* **spec:** inherit builtin flag visibility on synthesized commands ([#64](https://github.com/conga-sh/conga/issues/64)) ([a4681e7](https://github.com/conga-sh/conga/commit/a4681e796083e25e5610a56e66d776d6a88320fa))


### 📚 Documentation

* **website:** document --why flag help and completion rendering ([#67](https://github.com/conga-sh/conga/issues/67)) ([2c61bb4](https://github.com/conga-sh/conga/commit/2c61bb47f03ed8b419a13156fb3d4fe2159366c9))


### ♻️ Code Refactoring

* remove StrictTagValidation and allow-unknown-tags directive ([#58](https://github.com/conga-sh/conga/issues/58)) ([3bdcb51](https://github.com/conga-sh/conga/commit/3bdcb510fd23b1ebf98a11545a8fbf15c7184d51))

## [0.1.1](https://github.com/conga-sh/conga/compare/v0.1.0...v0.1.1) (2026-10-20)


### 🐛 Bug Fixes

* **zsh:** escape colon in completion values for _describe ([#47](https://github.com/conga-sh/conga/issues/47)) ([be89bb8](https://github.com/conga-sh/conga/commit/be89bb8e4c691b779f019cd39ca130fac8ca468f))

## [0.1.0] - 2026-10-20

CONGA v0.1.0 is the initial public release, featuring declarative struct-based CLI modeling, programmatic configuration loading, struct-declared feature toggles, multi-tier map merging, and provenance reporting with `--why`.

### Added

- **Programmatic Configuration (`conga.ConfigLoader`)**:
  - `app.ConfigOptions(conga.ConfigLoader(load))` supplies the run's configuration tier directly from code via `conga.ConfigDocument` (`Set`, `SetKey`, `SetScoped`, `SetKeyScoped`).
  - `ConfigLoaderFunc[T]` exposes the typed root (`root *T`), allowing loaders to inspect typed fields rather than parsing raw string flags.
  - Failures wrap `ErrConfig` (exit 1). Mutually exclusive with file decoders or root `ConfigFile` flags (enforced by `ErrSchema` and `conga-lint`).
- **Bootstrap Struct Tag (`bootstrap:"true"`)**:
  - Marks flags, positional arguments, `DashArgs`, and `RawArgs` for binding before the configuration loader runs.
  - Follows standard resolution (`CLI > env > default`) and supports scalar, slice-of-scalar, and map-of-scalar types.
- **Scoped Document Origins**:
  - `ConfigDocument.SetScoped` and `SetKeyScoped` record scopes within document origins (rendered as `origin › scope` in `--why` value attribution).
- **Struct-Declared Feature Toggles**:
  - `App.EnableFeatures()` activates struct-declared feature toggles for applications using `conga.Features[T]` or `feature:"..."` tags.
- **Provenance Reports (`--why` & `--why=json`)**:
  - Bare `--why` or `--why=text` renders human-readable attribution tables.
  - `app.EnableWhy(conga.WhyJSON())` enables `--why=json`, emitting a versioned schema-backed provenance document.
  - Programmatic access via `Session.Why()` and `Session.WhyJSON()` inside lifecycle hooks.

### Changed

- **Breaking**: Feature toggles are now opt-in. Declaring `conga.Features[T]` or `feature:"..."` without calling `app.EnableFeatures()` is rejected at schema build with `ErrSchema`.
- **Breaking**: Map flags (`map[K]V`, registered `RegisterMap` types, and `conga.Features[T]`) now merge key-by-key across all source tiers (`default < config < env < CLI`) instead of a single winning tier supplying the entire map. Lower-tier malformed maps or unknown keys now error even when overridden by higher tiers.
- **Breaking**: Merged `predict/{network,system,timezone}` and `plugin/{network,system,timezone}` into three topical kits: `network`, `system`, and `timezone`.
- **Breaking**: Renamed standard library bindings to `network.RegisterInterface`, `system.RegisterUser`, `system.RegisterGroup`, `system.RegisterSignal`, and `timezone.RegisterLocation`.
- **Breaking**: `conga.ConfigLoaderFunc[T]` is now generic and receives the typed root `root *T`. `conga.ConfigContext` dropped `FlagValue`/`FlagValues` in favor of `bootstrap:"true"` fields.
- **Breaking**: `SourceDetail()` for config-sourced values now reports the contributing origin (resolved file path or loader label) instead of the config key. Merged maps list all contributing files in load order.
- Positional argument `enum` validation now validates against the coerced string form, matching flag behavior.
- `--why` report enhancements:
  - Indented sub-rows per map key with tier attribution and feature toggle sources.
  - Direct file origin annotations (`config (app.yaml)`) and key count summaries (`(N keys)`).
  - Fast-fail validation on invalid config, coercion, or feature inputs.
  - Multi-file `ConfigFile` rows render `(N files)` with tree sub-rows.

### Fixed

- Empty environment variables (`FOO=`) are now treated as unset across all flag types, allowing lower-precedence defaults or config documents to apply.
- Slices with `sep:"none"` now preserve string elements verbatim, including leading and trailing whitespace.
- Shell completion executes declared `ConfigLoader` instances with bound `bootstrap` fields before candidate generation.
- `conga.ConfigCascading()` merges map flags key-by-key across cascaded config files. Nested dotted map keys (`config:`) and array-valued sections now bind without separator splitting issues.

[Unreleased]: https://github.com/conga-sh/conga/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/conga-sh/conga/releases/tag/v0.1.0
