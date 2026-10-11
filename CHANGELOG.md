# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

- **Breaking**: merged `predict/{network,system,timezone}` and `plugin/{network,system,timezone}` into three topical kits — `network`, `system`, and `timezone`. Each kit bundles its domain types, autocompletion predictors, and standard library bindings in one package.
- **Breaking**: renamed the standard library bindings to `network.RegisterInterface`, `system.RegisterUser`, `system.RegisterGroup`, `system.RegisterSignal`, and `timezone.RegisterLocation`. This resolves the previous `system.User` type-versus-function collision.
- Positional-argument `enum` validation now checks the coerced value's string form, matching flag validation, so normalizing `encoding.TextUnmarshaler` types compose with the `enum` tag.

### Fixed
