---
title: conga-lint
description: The static schema analyzer — checks, discovery contract, and flags.
---

`conga-lint` mirrors CONGA's runtime schema contracts against Go AST and type information, catching schema errors, tag typos, and invalid hierarchies before binaries are built or run. [How to lint schemas](/how-to/lint-schemas/) covers installing and running it in CI.

## Checks

The analyzer delegates to the exported validators in `internal/spec`, so static diagnostics match runtime validation:

| Check | Runtime contract mirrored |
|-------|---------------------------|
| Leaf command Runner enforcement | The bind-time `noRunnerError`: any command struct without `Command[T]` fields must implement `Run(context.Context, *conga.Session) error`. |
| Struct tag validation | Tag syntax, `required:"true"` rejection, `help` on `Command[T]`, directive validity, predictor vocabulary, `range`/`len`/`items` bounds, unregistered `predictor:"..."` names, `enumhint:"-"` usage, and unknown or undeclared `{{...}}` help variables. |
| Feature schema checks | The `conga.Features[T]` contract: root-only single declaration, feature-tag syntax on flags, subcommands, and config-file flags, `feature` rejected on `RequiredFlag`, feature-field type and tag vocabulary (including per-feature `env` tags), `stage` tag validity (`alpha`, `beta`, `ga`, `deprecated`) and the all-or-nothing stage rule, duplicate normalized feature names, integer feature non-zero `enum` levels, the exclusive `env`/`envprefix` container surfaces, and environment variable collision validation. |
| Opt-in subsystem guards | Flags `conga.Features[T]` or `feature:"..."` tags when `app.EnableFeatures()` is missing, and completion features when `app.EnableCompletion()` is missing. |
| `Session.Root[T]` type safety | Flags calls to `session.Root[T]()` where the type argument `T` does not match the root application struct type passed to `conga.New`. |
| `ConfigExplicit` exclusivity | Flags builder calls combining `conga.ConfigExplicit(...)` with `conga.ConfigPaths(...)` or `conga.ConfigCascading()`. |
| `ConfigLoader` exclusivity | Flags builder calls combining `conga.ConfigLoader(...)` with file options (`ConfigPaths`, `ConfigExplicit`, `ConfigCascading`), `RegisterDecoder`, or a root `ConfigFile` field. |
| `bootstrap` tag validation | Flags fields carrying `bootstrap` when `conga.ConfigLoader` is not declared, flags combining `bootstrap` with `feature`, and flags `bootstrap` on unsupported types (structs, pointers, interfaces). |
| Synthesized feature flag shadowing | The `flag:"..."` name installed for a CLI-exposed `Features[T]` container joins the ancestor shadow map, so a descendant flag cannot redeclare it. |
| Ancestor flag shadowing | The runtime `ErrDuplicateFlag` rule: a flag name, alias, or short rune declared on an ancestor cannot be redeclared on any descendant command. |
| Cascading guard | The schema rejection of `conga.ConfigCascading()` when the root command lacks a `multi:"true"` `ConfigFile` flag. |
| Throwaway root detection | Lint-only: flags `conga.New(root)` when the root pointer is never referenced outside the application builder, steering code to the engine-allocating `conga.New[T]()` form. |
| Builder call order (opt-in) | With `-builder-order`, validates fluent builder invocations at `conga.New` construction sites against canonical ordering. |

## Discovery contract

Hierarchies are discovered exclusively from `conga.New` constructor sites (both `conga.New[T]()` and `conga.New(root)`) and then walked through the root type's `Command[T]` fields. Settings are reconstructed from literal builder arguments:

- `app.StrictTagValidation(true)` / `app.StrictTagValidation(false)`
- `app.AllowTags("...", ...)`
- `app.Predictor("name", ...)`

Non-literal or computed arguments degrade conservatively to the runtime defaults. `./...` skips directories with leading underscores (such as `_examples/`); pass directory paths or globs explicitly to lint them.

## Flags

| Flag | Effect |
|------|--------|
| `-strict` | Force strict tag validation for every scanned package, regardless of detected builder calls. |
| `-builder-order` | Check builder invocations against the canonical order: identity and documentation first, `Enable*`/`Disable*` toggles alphabetically, behavior configuration grouped, execution methods last. |
| `-builder-order-spec` | Override the order with comma- or newline-separated method names, or `@path` to read one per line. |
| `-fix` | Apply builder-order fixes in place; barrier calls such as `Use(...)` and other unsafe swaps are never moved. |
| `-diff` | Preview builder-order fixes as a diff. |

## Relationship to runtime errors

Static linting complements runtime verification: the runtime schema builder still enforces every contract at startup, returning descriptive `ErrSchema` errors from `App.Run` if an invalid schema executes.
