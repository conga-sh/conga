# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

- **Breaking**: map flags (`map[K]V` fields, registered `RegisterMap` types, and the synthesized `conga.Features[T]` map) now merge key-by-key across all source tiers (`default` < config < env < CLI) instead of one winning tier supplying the whole map. A config file can set one key while the command line sets another, and both apply; a higher tier overrides only the keys it names. Scalars and slices keep winning-tier semantics. A malformed map or unknown feature key in a lower tier (e.g. config) now errors even when a higher tier also sets the flag, attributed to the tier that supplied it.
- **Breaking**: merged `predict/{network,system,timezone}` and `plugin/{network,system,timezone}` into three topical kits — `network`, `system`, and `timezone`. Each kit bundles its domain types, autocompletion predictors, and standard library bindings in one package.
- **Breaking**: renamed the standard library bindings to `network.RegisterInterface`, `system.RegisterUser`, `system.RegisterGroup`, `system.RegisterSignal`, and `timezone.RegisterLocation`. This resolves the previous `system.User` type-versus-function collision.
- Positional-argument `enum` validation now checks the coerced value's string form, matching flag validation, so normalizing `encoding.TextUnmarshaler` types compose with the `enum` tag.
- The `--why` report renders one indented sub-row per map key with the tier that last wrote it, and the Features table reports each toggle's actual source tier instead of the winning tier of the whole map.

### Fixed
