# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While CONGA is pre-1.0, breaking changes increment the minor version.

## [0.9.0](https://github.com/conga-sh/conga/compare/v0.8.0...v0.9.0) (2026-11-13)


### ⚠ BREAKING CHANGES

* **features:** a feature tag on an implicit:"true" command fails schema build with ErrSchema; gate the parent or make the command explicit. Feature error messages changed: env and config unknown-feature errors and the config gated-flag error no longer start with "config
* **coerce:** --flag= on a boolean, an empty map[string]bool entry value, and an empty boolean config string no longer mean true. Pass --flag or --flag=true instead.
* **binder:** an xor, oneof, or and group failure whose supplied members all came from the environment or a config file now matches ErrConfig only, exits 1, and reports IsUsageError false instead of matching ErrValidation and exiting 2. Group error messages name non-command-line members with their source and command-line members by the typed spelling; update callers that match the old text. --why no longer fails on a group conflict.
* **errors:** environment-, configuration-, and default-sourced value failures now match ErrConfig only, exit 1, and report IsUsageError false instead of matching ErrCoerce or ErrValidation, exit 2. Update callers that match the old categories. Unexpected arguments and negated flags with a value now match ErrParse; wrong path kinds match ErrValidation; a missing Runner matches ErrSchema.
* **spec:** an app with an ancestor --json flag and a mounted VersionCommand now fails schema build; rename the ancestor flag or remove the VersionCommand field and rely on --version. An ancestor flag that redeclares a CLI-exposed Features flag likewise fails; rename one of them.
* **binder:** a Counter rejects an attached command-line value (--verbose=2, -v=2) instead of setting or adding to the count; repeat the flag (-vv), set the count through an environment variable or config file, or use OptionalFlag[int] for a level set on the command line. Negative and signed counts from the environment or config now fail instead of being accepted, and counter errors say "unsigned integer" instead of "integer".
* **config:** config files must spell each key exactly. A file that writes api_endpoint for --api-endpoint now fails with ErrConfig, exit 1. Rename the key to the one --why shows, set conga.ConfigKeyCase(conga.SnakeCase), or set config:"<key>" on the field. Flag and subcommand aliases no longer work as config keys, a literal "db.host" key must be nested, and a flag with an explicit config tag no longer reads its name.
* **config:** `conga.ConfigExplicit(path)` no longer overrides the `<PREFIX>_CONFIG` environment variable. A program that relied on it to ignore the variable now loads the file the variable names. When the variable is set, a missing `ConfigExplicit` file is no longer an error, because it isn't read.

### 🐛 Bug Fixes

* **binder:** attribute flag group errors to their source and spelling ([#192](https://github.com/conga-sh/conga/issues/192)) ([4e0a76b](https://github.com/conga-sh/conga/commit/4e0a76b81ca5facc71bad091e05a712e79640d2e)), closes [#170](https://github.com/conga-sh/conga/issues/170)
* **binder:** make counters take no command-line value and accept only unsigned counts ([#186](https://github.com/conga-sh/conga/issues/186)) ([d8c8a71](https://github.com/conga-sh/conga/commit/d8c8a71b97cd22e69b85730b7e5894b996d2e113)), closes [#149](https://github.com/conga-sh/conga/issues/149)
* claim stdin once per scalar flag, report namespace sections like command sections, and let errors.Is reach config-sourced causes ([#205](https://github.com/conga-sh/conga/issues/205)) ([199c539](https://github.com/conga-sh/conga/commit/199c539e3e8690c24a40924eb1d873646408db3d))
* clarify the and-group error, the RawArgs usage line, and stale docs ([#140](https://github.com/conga-sh/conga/issues/140)) ([ff1ab5c](https://github.com/conga-sh/conga/commit/ff1ab5c995987be364f5e3e8369fef3442723d7a)), closes [#111](https://github.com/conga-sh/conga/issues/111) [#123](https://github.com/conga-sh/conga/issues/123) [#124](https://github.com/conga-sh/conga/issues/124) [#109](https://github.com/conga-sh/conga/issues/109)
* **coerce:** reject an empty boolean value ([#193](https://github.com/conga-sh/conga/issues/193)) ([c15b852](https://github.com/conga-sh/conga/commit/c15b852072be9de92e9a6c1e96a8e703c618b722)), closes [#171](https://github.com/conga-sh/conga/issues/171)
* **coerce:** reject float32 bound misses, earlier bad occurrences, double stdin reads, and misleading byte size errors ([#187](https://github.com/conga-sh/conga/issues/187)) ([91ea375](https://github.com/conga-sh/conga/commit/91ea375c4be8afcaac29feedec307eb61d58f27b)), closes [#147](https://github.com/conga-sh/conga/issues/147) [#150](https://github.com/conga-sh/conga/issues/150) [#151](https://github.com/conga-sh/conga/issues/151) [#152](https://github.com/conga-sh/conga/issues/152)
* **completion:** handle multibyte cursors, escaped spaces and quoting in bash, zsh, fish and PowerShell ([#199](https://github.com/conga-sh/conga/issues/199)) ([7ef4480](https://github.com/conga-sh/conga/commit/7ef4480c877a1d3722980fd25e0fb0cecffd6412)), closes [#154](https://github.com/conga-sh/conga/issues/154) [#155](https://github.com/conga-sh/conga/issues/155)
* **completion:** honour LS_COLORS and quote values safely in PowerShell and nushell ([#201](https://github.com/conga-sh/conga/issues/201)) ([b2c49ae](https://github.com/conga-sh/conga/commit/b2c49ae9a85ff4c24e62ab1441a6b27a2b254628)), closes [#156](https://github.com/conga-sh/conga/issues/156) [#157](https://github.com/conga-sh/conga/issues/157)
* **completion:** keep candidate order in bash and zsh when keeporder is set ([#198](https://github.com/conga-sh/conga/issues/198)) ([f6f03c5](https://github.com/conga-sh/conga/commit/f6f03c5b03c0850cf92cf327acea3227f5846c33)), closes [#153](https://github.com/conga-sh/conga/issues/153)
* **config:** bind decoded numbers and datetimes exactly ([#184](https://github.com/conga-sh/conga/issues/184)) ([3e74bbd](https://github.com/conga-sh/conga/commit/3e74bbdd32fb2c056081f88e0bfaeb49c0365c14)), closes [#145](https://github.com/conga-sh/conga/issues/145) [#146](https://github.com/conga-sh/conga/issues/146)
* **config:** match config keys exactly ([#183](https://github.com/conga-sh/conga/issues/183)) ([69aec2d](https://github.com/conga-sh/conga/commit/69aec2df75090d1e7dc04ca1a0e4e7f1860a51ad)), closes [#142](https://github.com/conga-sh/conga/issues/142) [#143](https://github.com/conga-sh/conga/issues/143) [#182](https://github.com/conga-sh/conga/issues/182)
* **config:** rank &lt;PREFIX&gt;_CONFIG above ConfigExplicit ([#135](https://github.com/conga-sh/conga/issues/135)) ([6831b39](https://github.com/conga-sh/conga/commit/6831b3970819b5e9bdf38e6496b2f4ada911befd)), closes [#117](https://github.com/conga-sh/conga/issues/117)
* **dotenv:** keep escaped dollars literal and accept a closing quote at end of file ([#195](https://github.com/conga-sh/conga/issues/195)) ([670d26b](https://github.com/conga-sh/conga/commit/670d26b28aabfcce4fa2dfc04f507993d692bb81)), closes [#159](https://github.com/conga-sh/conga/issues/159)
* **errors:** classify value errors by source and give each failure one category ([#190](https://github.com/conga-sh/conga/issues/190)) ([0061ce5](https://github.com/conga-sh/conga/commit/0061ce5009b31dbe59b3ed3546baca18d8c164c0)), closes [#133](https://github.com/conga-sh/conga/issues/133) [#162](https://github.com/conga-sh/conga/issues/162)
* **errors:** make suggestions and schema messages accurate ([#196](https://github.com/conga-sh/conga/issues/196)) ([c00f0c9](https://github.com/conga-sh/conga/commit/c00f0c97b6fc2dcad1e314acb4bf8de4d444a275)), closes [#163](https://github.com/conga-sh/conga/issues/163) [#164](https://github.com/conga-sh/conga/issues/164) [#127](https://github.com/conga-sh/conga/issues/127)
* **features:** attribute feature errors, word gate expressions, and respect higher-tier gates ([#194](https://github.com/conga-sh/conga/issues/194)) ([d9b94fb](https://github.com/conga-sh/conga/commit/d9b94fbe634cdc82e9fa7d344fb96929fb07acca)), closes [#160](https://github.com/conga-sh/conga/issues/160) [#161](https://github.com/conga-sh/conga/issues/161)
* **file:** resolve `./name` executables in the working directory as absolute paths ([#203](https://github.com/conga-sh/conga/issues/203)) ([3f93eb4](https://github.com/conga-sh/conga/commit/3f93eb43ec5e722ecf6a1d90578250d3cafd4833)), closes [#188](https://github.com/conga-sh/conga/issues/188)
* **predict:** apply Exclude to every predictor and let a predictor's Usage win ([#138](https://github.com/conga-sh/conga/issues/138)) ([e959d51](https://github.com/conga-sh/conga/commit/e959d51cebbc11480f1e9141b188e3ba561dc48b)), closes [#116](https://github.com/conga-sh/conga/issues/116) [#114](https://github.com/conga-sh/conga/issues/114)
* **predict:** complete `..` to `../` and drop known_hosts patterns from host candidates ([#202](https://github.com/conga-sh/conga/issues/202)) ([848a966](https://github.com/conga-sh/conga/commit/848a9665bce4577cac6ed1bc59afbe99d57740b5)), closes [#158](https://github.com/conga-sh/conga/issues/158)
* restore TypeParser assignability and tighten completion and conga-lint output ([#141](https://github.com/conga-sh/conga/issues/141)) ([b4803a1](https://github.com/conga-sh/conga/commit/b4803a111334c25581fa9ed8d94dda71e10e7b52))
* **spec:** reject ancestor flags that collide with built-in and feature flags ([#189](https://github.com/conga-sh/conga/issues/189)) ([48b49bc](https://github.com/conga-sh/conga/commit/48b49bc2a4b6705e9fbbd14ae4ea1e23f2aa619f)), closes [#148](https://github.com/conga-sh/conga/issues/148)
* **why:** list skipped named config files and keep comma paths whole ([#185](https://github.com/conga-sh/conga/issues/185)) ([4ba5e50](https://github.com/conga-sh/conga/commit/4ba5e50bb5081aeef446ccf30de3f391e534aba4)), closes [#144](https://github.com/conga-sh/conga/issues/144)
* **why:** report skipped search paths and align section spacing ([#137](https://github.com/conga-sh/conga/issues/137)) ([89a8e52](https://github.com/conga-sh/conga/commit/89a8e5229f9167c878cf7faf914c2ac3d46629e9)), closes [#108](https://github.com/conga-sh/conga/issues/108) [#122](https://github.com/conga-sh/conga/issues/122)

## [0.8.0](https://github.com/conga-sh/conga/compare/v0.7.0...v0.8.0) (2026-11-10)


### ⚠ BREAKING CHANGES

* report invalid feature values from config as usage errors

### ✨ Features

* **conga-lint:** print diagnostics with paths relative to the working directory ([8be000e](https://github.com/conga-sh/conga/commit/8be000e86d21c5e8545c809c599e3a6556031196))
* name the RegisterType parser signature conga.TypeParser ([8be000e](https://github.com/conga-sh/conga/commit/8be000e86d21c5e8545c809c599e3a6556031196))


### 🐛 Bug Fixes

* **completion:** correct and complete the per-shell install instructions ([8be000e](https://github.com/conga-sh/conga/commit/8be000e86d21c5e8545c809c599e3a6556031196))
* list the accepted boolean values, including 1 and 0, in error messages ([8be000e](https://github.com/conga-sh/conga/commit/8be000e86d21c5e8545c809c599e3a6556031196))
* report invalid feature values from config as usage errors ([8be000e](https://github.com/conga-sh/conga/commit/8be000e86d21c5e8545c809c599e3a6556031196))


### 📚 Documentation

* rewrite the documentation site with tutorials, task guides, reference, and explanation pages ([8be000e](https://github.com/conga-sh/conga/commit/8be000e86d21c5e8545c809c599e3a6556031196))
* **website:** render keyboard keys with starlight-kbd ([b476607](https://github.com/conga-sh/conga/commit/b476607e6b49855f73add47614698ba4a975d98a))

## [0.7.0](https://github.com/conga-sh/conga/compare/v0.6.0...v0.7.0) (2026-11-05)


### ⚠ BREAKING CHANGES

* **envprefix:** document replacement semantics and remove blank-field prefix ([#106](https://github.com/conga-sh/conga/issues/106))

### ✨ Features

* **completion:** allow digit-leading app names for derived env prefix ([#104](https://github.com/conga-sh/conga/issues/104)) ([9520fee](https://github.com/conga-sh/conga/commit/9520feea306aef22df3c28912f55912486e686ae))


### 🐛 Bug Fixes

* **envprefix:** document replacement semantics and remove blank-field prefix ([#106](https://github.com/conga-sh/conga/issues/106)) ([848fbe8](https://github.com/conga-sh/conga/commit/848fbe82eebe15ac5b81e225cf9c58b30e84f1f5))


### 📚 Documentation

* clarify EnvPrefix does not apply to Features containers ([#107](https://github.com/conga-sh/conga/issues/107)) ([ceb3f61](https://github.com/conga-sh/conga/commit/ceb3f61384c07433126eed2bc69ec7d3868e6c84))

## [0.6.0](https://github.com/conga-sh/conga/compare/v0.5.0...v0.6.0) (2026-11-05)


### ⚠ BREAKING CHANGES

* `--` now ends subcommand routing; schema build rejects positionals mixed with subcommands and RawArgs combined with command fields. Move positionals to leaf commands or use DashArgs for `--` forwarding.

### ✨ Features

* add implicit subcommand selection by unmatched operands ([#98](https://github.com/conga-sh/conga/issues/98)) ([6d90611](https://github.com/conga-sh/conga/commit/6d90611fa09f4114d6bb2841873cdb2b27bd5a2c))
* complete trailing arguments with predictors and grammar invariants ([#95](https://github.com/conga-sh/conga/issues/95)) ([43b5f3e](https://github.com/conga-sh/conga/commit/43b5f3efedb30f65922761afb20737e74aa97a5e))


### 🐛 Bug Fixes

* **spec:** honor declared help/version flags shadowing built-in shorts ([#97](https://github.com/conga-sh/conga/issues/97)) ([965c3a1](https://github.com/conga-sh/conga/commit/965c3a109d5654f6389d1a54baf78e8fb174ba17))


### 📚 Documentation

* **website:** document implicit subcommand routing ([#103](https://github.com/conga-sh/conga/issues/103)) ([17178a3](https://github.com/conga-sh/conga/commit/17178a36fc9c852337e802e660feac7d24cb9fc5))

## [0.5.0](https://github.com/conga-sh/conga/compare/v0.4.0...v0.5.0) (2026-11-01)


### ⚠ BREAKING CHANGES

* rename group tag to section for help headings ([#91](https://github.com/conga-sh/conga/issues/91))

### ✨ Features

* rename group tag to section for help headings ([#91](https://github.com/conga-sh/conga/issues/91)) ([7c5caff](https://github.com/conga-sh/conga/commit/7c5caff32f1efd25bcca464cc410f668a77c55b7)), closes [#87](https://github.com/conga-sh/conga/issues/87)


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
* **spec:** support aliases and section tags on builtin subcommands ([#66](https://github.com/conga-sh/conga/issues/66)) ([d14d383](https://github.com/conga-sh/conga/commit/d14d383ee6b5a05b3413647dc4a9aff9542b2481))
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
