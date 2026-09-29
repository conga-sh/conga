---
title: How to lint CONGA schemas
description: Catch leaf Runner violations, struct tag typos, and ancestor flag shadowing in CI with conga-lint.
---

`conga-lint` is a static analysis tool that mirrors CONGA's runtime schema contracts against Go AST and type information. It catches schema errors, tag typos, and invalid hierarchies in CI before binaries are built or run.

## Install and run the analyzer

Declare the analyzer as a tool dependency of your Go module, then invoke it over your packages:

```sh
$ go get -tool github.com/conga-sh/conga/cmd/conga-lint
$ go tool conga-lint ./...
```

Inside the CONGA repository, the `lint:schema` task integrates the check into the test pipeline:

```sh
$ task lint:schema
```

Note: Go's `./...` pattern skips directories with leading underscores (such as `_examples/`). To lint examples, pass directory paths or globs explicitly to the tool.

## Checks performed by conga-lint

The analyzer delegates directly to the exported validators in `internal/spec`, ensuring static checks produce identical diagnostics to runtime validation:

| Check | Runtime contract mirrored |
|-------|---------------------------|
| Leaf command Runner enforcement | The bind-time `noRunnerError`: any command struct without `Command[T]` fields must implement `Run(context.Context, *conga.Session) error`. |
| Struct tag validation | Tag syntax, `required:"true"` rejection, `help` on `Command[T]`, directive validity, predictor vocabulary, `range`/`len`/`items` bounds, and unregistered `predictor:"..."` names. |
| Ancestor flag shadowing | The runtime `ErrDuplicateFlag` rule: a flag name, alias, or short rune declared on an ancestor cannot be redeclared on any descendant command. |
| Cascading guard | The schema rejection of `config.Cascading()` when the root command lacks a `multi:"true"` `ConfigFile` flag. |
| Throwaway root detection | Lint-only check: flags `conga.New(root)` when the root pointer is never referenced outside the application builder, steering code to the engine-allocating `conga.New[T]()` form. |
| Builder call order (opt-in) | With `-builder-order`, validates fluent builder invocations at `conga.New` construction sites against canonical ordering. |

## The static discovery contract

The analyzer identifies command hierarchies starting exclusively from `conga.New` constructor sites (both `conga.New[T]()` and `conga.New(root)`), then recursively walks the `Command[T]` fields of the root type.

Settings are reconstructed from literal builder arguments:
- `app.StrictTagValidation(true)` / `app.StrictTagValidation(false)`
- `app.AllowTags("...", ...)`
- `app.Predictor("name", ...)`

If builder arguments are non-literal or computed dynamically at runtime, the analyzer degrades conservatively to runtime defaults to prevent false positives. To enforce strict validation across all scanned packages regardless of detected builder calls, pass the `-strict` flag:

```sh
$ go tool conga-lint -strict ./...
```

## Validate builder call ordering

The optional `-builder-order` flag checks builder method chains against CONGA's canonical ordering conventions: identity and documentation methods first, `Enable*`/`Disable*` toggles alphabetically, behavior configuration grouped, and execution methods last.

```sh
# Report builder ordering violations
$ go tool conga-lint -builder-order ./...

# Automatically fix violations in place
$ go tool conga-lint -builder-order -fix ./...

# Preview suggested fixes as a diff
$ go tool conga-lint -builder-order -fix -diff ./...
```

Custom ordering rules can be supplied via `-builder-order-spec`, which accepts a comma-separated list of method names or `@path` to read a specification file. Safe refactoring guards ensure barrier calls (such as `Use(...)`) are never reordered.

## Relationship to runtime errors

Static linting does not replace runtime verification. While `conga-lint` catches defects at compile time, CONGA's runtime schema builder still enforces every contract at startup, returning descriptive `ErrSchema` errors from `App.Run` if an invalid schema executes.

## Where to go next

- [Struct tag reference](/reference/struct-tags/) — reference catalog for all supported struct tags and syntax rules.
- [Errors and exit codes](/reference/errors/) — base error categories and exit code mappings.
- [How to declare subcommands](/how-to/declare-subcommands/) — command hierarchy routing and flag shadowing rules.
