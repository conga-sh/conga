---
title: conga-lint
description: The static schema analyzer — checks, discovery contract, and flags.
---

`conga-lint` mirrors CONGA's runtime schema contracts against Go AST and type information, catching schema errors, tag typos, and invalid hierarchies before binaries are built or run. [How to lint schemas](/how-to/lint-schemas/) covers installing and running it in CI.

## Checks

Each check reports the same mistake, with the same message, that the program reports at startup:

| Check | Runtime contract mirrored |
|-------|---------------------------|
| Leaf command Runner enforcement | A command struct without `Command[T]` fields must implement `Run(context.Context, *conga.Session) error`; without it, the command fails when invoked. |
| Branch/leaf separation | The runtime `ErrSchema` rule that a command declaring positional arguments cannot also declare `Command[T]` subcommands; declared built-in help, version, and completion command fields do not count as branching. |
| RawArgs boundary | The runtime `ErrSchema` rule that `RawArgs` cannot coexist with any command field (user subcommands or declared built-in help, version, and completion commands) and that every positional argument before it is a required scalar. |
| Duplicate trailing collectors | The runtime `ErrDuplicateFlag` rejection of a second `DashArgs` or `RawArgs` field on one command struct, naming the first declaration. |
| Struct tag validation | Tag syntax, `required:"true"` rejection, `help` on `Command[T]`, predictor vocabulary, `range`/`len`/`items` bounds, unregistered `predictor:"..."` names, `enumhint:"-"` usage, whitespace-padded `section` values, and unknown or undeclared `{{...}}` help variables. |
| Trailing-collector predictor | The predictor tag contract on `DashArgs`/`RawArgs`: the explicit tag only (no type inference), the same vocabulary as flags and positional arguments, and an absent tag means the `none` predictor. |
| Flag tag guards | Empty or whitespace-padded `xor`/`and`/`oneof` group names; `xor`/`oneof`/`deprecated`/`default` on `RequiredFlag`; `default` on `oneof` members; two `oneof` or `xor` members sharing an `and` group; a one-member `xor` or `and` group; and a one-member `oneof` group unless its sole member is a `Counter`. |
| Plain-field tag rejection | CONGA tags on exported non-container fields are rejected instead of silently ignored. `app.AllowTags(...)` accepts the listed keys; ambiguous AllowTags evidence suppresses the diagnostic. |
| Feature schema checks | The `conga.Features[T]` contract: root-only single declaration, feature-tag syntax on flags, subcommands, and config-file flags, `feature` rejected on `RequiredFlag`, feature-field type and tag vocabulary (including per-feature `env` tags), `stage` tag validity (`alpha`, `beta`, `ga`, `deprecated`) and the all-or-nothing stage rule, duplicate normalized feature names, integer feature non-zero `enum` levels, the exclusive `env`/`envprefix` container variable shapes, and environment variable collision validation. |
| Opt-in subsystem guards | Flags `conga.Features[T]` or `feature:"..."` tags when `app.EnableFeatures()` is missing, and completion features when `app.EnableCompletion()` is missing. |
| `Session.Root[T]` type safety | Flags calls to `session.Root[T]()` where the type argument `T` does not match the root application struct type passed to `conga.New`. |
| `ConfigExplicit` exclusivity | Flags builder calls combining `conga.ConfigExplicit(...)` with `conga.ConfigPaths(...)` or `conga.ConfigCascading()`. |
| `bootstrap` tag validation | Flags fields carrying `bootstrap` when neither the declaring command nor an ancestor implements `conga.Bootstrapper`, flags combining `bootstrap` with `feature`, and fields carrying `bootstrap` on unsupported types (structs, pointers, interfaces). |
| Synthesized feature flag shadowing | The `flag:"..."` name installed for a CLI-exposed `Features[T]` container joins the ancestor shadow map, so a descendant flag cannot redeclare it. |
| Version command flag shadowing | The built-in version command's `--json` selector joins the ancestor shadow map, so an ancestor `--json` flag is rejected with the same `ErrDuplicateFlag` diagnostic and migration hint. |
| Implicit command uniqueness | The runtime `ErrSchema` rule that a command declares at most one `implicit:"true"` subcommand; the diagnostic names the field that declared the first. |
| Implicit command gates | The runtime `ErrSchema` rule that an `implicit:"true"` subcommand can't carry a `feature` tag. Gating its parent, or a flag it declares, stays legal. |
| Ancestor flag shadowing | The runtime `ErrDuplicateFlag` rule: a flag name, alias, or short rune declared on an ancestor cannot be redeclared on any descendant command. The diagnostic names the struct and field on both sides, built by the same function as the runtime error. |
| Built-in short collisions | The runtime rejection of a user-declared `short` that claims the rune assigned with `app.HelpShortFlag(...)` (every command offering help) or `app.VersionShortFlag(...)` (root only), and of assigning the same rune to both built-ins. A disabled built-in leaves its rune free; non-constant runes suppress the check. |
| Cascading guard | The schema rejection of `conga.ConfigCascading()` when the root command lacks a `multi:"true"` `ConfigFile` flag. |
| Throwaway root detection | Lint-only: flags `conga.New(root)` when the root pointer is never referenced outside the application builder, steering code to the engine-allocating `conga.New[T]()` form. |
| Builder call order (opt-in) | With `-builder-order`, validates fluent builder invocations at `conga.New` construction sites against canonical ordering. |

## Discovery contract

Hierarchies are discovered exclusively from `conga.New` constructor sites (both `conga.New[T]()` and `conga.New(root)`) and then walked through the root type's `Command[T]` fields. Settings are reconstructed from literal builder arguments:

- `app.AllowTags("...", ...)`
- `app.Predictor("name", ...)`
- `app.HelpShortFlag('h')`, `app.VersionShortFlag('V')`
- `app.DisableHelpFlag()`, `app.DisableVersionFlag()`

Non-literal or computed arguments degrade conservatively, and the analyzer suppresses the affected diagnostic rather than inventing one. Builder-option evidence is aggregated package-wide, so checks that could turn it into a diagnostic, the cascading guard and the built-in short collisions, are skipped entirely in packages anchoring more than one `conga.New` root. A diagnostic is never attributed to an innocent sibling root. `./...` skips directories with leading underscores (such as `_examples/`), so pass directory paths or globs explicitly to lint them.

## Flags

| Flag | Effect |
|------|--------|
| `-builder-order` | Check builder invocations against the canonical order: identity and documentation first, `Enable*`/`Disable*` toggles alphabetically, behavior configuration grouped, execution methods last. |
| `-builder-order-spec` | Override the order with comma- or newline-separated method names, or `@path` to read one per line. Methods left out are unconstrained. Takes effect only with `-builder-order`; an unknown method name is a flag error (exit 2). |
| `-fix` | Apply builder-order fixes in place; barrier calls such as `Use(...)` and other unsafe swaps are never moved. |
| `-diff` | Preview builder-order fixes as a diff. |

## Output

A report run prints one line per finding, `file:line:column: message`, with the file path relative to the working directory, or absolute when the file lies outside it. It exits 0 when there are no findings, 3 when there are, and 1 when packages fail to load or analysis fails. `-json`, `-c`, and `-fix` (with or without `-diff`) use the standard `golang.org/x/tools` driver output, which prints absolute paths. With `-json` the exit code is always 0. Under `go vet -vettool`, `go vet` prints the findings and exits 1 when there are any.

## Relationship to runtime errors

Static linting complements runtime verification. The runtime schema builder still enforces every contract at startup, returning descriptive `ErrSchema` errors from `App.Run` if an invalid schema executes.
