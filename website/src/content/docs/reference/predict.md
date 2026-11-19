---
title: package predict
description: The decoupled completion SDK — predictors, combinators, items, context, directives.
---

Complete package API documentation is on [pkg.go.dev/github.com/conga-sh/conga/predict](https://pkg.go.dev/github.com/conga-sh/conga/predict), and task guidance is in [How to compute and shape completion candidates](/how-to/compute-completion-candidates/).

## Core types

```go
type Predictor struct { /* opaque — immutable value, use combinators */ }

type Result struct {
	Items     []Item
	Directive Directive
	Usage     string
}

type Item struct {
	Value       string
	Display     string
	Description string
	// …style, tag, and alias fields; use NewItem for the common case
}

func NewItem(value, description string) Item
func (i Item) WithDisplay(display string) Item
func (i Item) WithDescription(description string) Item
func (i Item) WithStyle(style string) Item
func (i Item) WithTag(tag string) Item
func (i Item) WithAlias() Item
```

`WithAlias` marks a candidate as an alternative spelling of another candidate, such as a taskfile task alias. Alias candidates are shown by default and suppressed when the user disables them through the `show-aliases` zstyle or the application-prefixed `<APP>_COMPLETION_ALIASES` variable. Subcommand and flag aliases declared with the `aliases` tag carry the marker automatically. See [User toggles](/reference/builtins/#user-toggles).

## Constructors

| Constructor | Purpose |
|-------------|---------|
| `Values(values ...string)` | Static candidates |
| `Map(items map[string]string)` | Sorted map candidates |
| `KeyValue(valuesByKey map[string][]string)` | `KEY=VALUE` segmented completion |
| `KeyValueDescribed(valuesByKey map[string][]string, descriptionByKey map[string]string)` | `KEY=VALUE` segmented completion with per-key descriptions |
| `Items(items ...Item)` | Explicit item list |
| `Callback(fn func(Context) Predictor)` | Lazy evaluation |
| `MultiParts(delimiter string, fn func(Context) Predictor)` | Multi-segment values. When every candidate ends with the delimiter (`web:`), the shell adds no space after it |
| `Batch(predictors ...Predictor)` | Aggregate predictors |
| `None()` | Return an empty candidate set |

## Filesystem constructors

`Files(extensions...)`, `FilesFiltered(filter)`, `FilesGlob(patterns...)`, `Dirs()`, `DirsFiltered(filter)`, `DirsGlob(patterns...)`, `PathsFiltered(filter)`, `Symlinks()`, `Executables()`, `FileLines(filePath)`, and `FileContents(filePath, parser)`. Filters are `func(path string, entry os.DirEntry) bool`. `FilesGlob` matches file base names against `filepath.Match` patterns while keeping directories visible for traversal (a wildcard-free pattern such as `"Makefile"` matches that literal name), and `DirsGlob` matches directory base names against the same patterns. `DirsFiltered` is the predicate equivalent for directories.

Hidden entries are offered only when the typed name starts with `.`. A typed `..` offers `../`, also after a directory prefix (`src/..` offers `src/../`). A typed `.` offers the hidden entries and nothing else, so in a directory without hidden entries it offers no candidates and the shell doesn't fall back to its own file completion.

## Combinators on `Predictor`

| Combinator | Purpose |
|------------|---------|
| `Prefix(s)` / `Suffix(s)` | Decorate every candidate. On a `MultiParts` predictor they also decorate the partial candidates, so call `Suffix` on the predictor the last segment returns to decorate finished values only |
| `Include(values ...string)` | Keep only candidates whose value, the text the shell inserts after `Prefix`/`Suffix`, matches one of `values`. On a `MultiParts` predictor it also keeps a partial candidate that ends with the delimiter and starts a value in every `Include` list (`api-gateway:` for `api-gateway:latest`). A `MultiParts` reached through a callback, a batch, or a `Timeout` alternative matches whole values only. Applies to whatever the predictor produces, callbacks and batches included. Each call narrows further, so `Include()` with no values keeps nothing. Candidates keep the predictor's order |
| `Exclude(values ...string)` | Remove candidates whose value, the text the shell inserts after `Prefix`/`Suffix`, matches one of `values`. Applies to whatever the predictor produces, callbacks and batches included. On a `MultiParts` predictor the value is the whole text built so far, so to remove one segment, call `Exclude` on the predictor the segment function returns |
| `FilterText(toComplete)` | Pre-filter by query |
| `Style(s)` / `Tag(t)` | Default display style / group |
| `Usage(msg)` / `Usagef(format, a...)` | ActiveHelp / tooltip hint, used when the evaluated result has none of its own (a callback's or batch member's hint wins) |
| `NoSpace()` | Suppress the trailing space after insertion |
| `KeepOrder()` | Ask the shell to keep the candidate order. bash 4.4 or later and zsh honor it. zsh keeps the order within each tag group and lists candidates that have a description before those that don't. PowerShell and nushell keep the order anyway. fish and elvish always sort |
| `Timeout(d, alternative...)` | Deadline with optional fallback (panics with >1 alternative). The fallback keeps its own modifiers, and this predictor's modifiers apply on top |
| `Raw()` | Underlying static candidate strings, narrowed by `Include` and `Exclude` |
| `Invoke(ctx)` | Evaluate to a `Result` |

## Context

```go
type Context struct {
	ToComplete      string
	Args            []string
	Parts           []string
	ParentParts     []string
	Flags           map[string]string
	FlagMultiValues map[string][]string
	Dir             string
	Env             map[string]string
	EnvLookup       func(key string) (string, bool)
}

func (c Context) Arg(index int) string
func (c Context) FlagValue(name string, fallback string) string
func (c Context) FlagValues(name string) []string
func (c Context) Getenv(key string) string
func (c Context) HasFlag(name string) bool
func (c Context) LastArg() string
func (c Context) WithFlagValues(flagValues map[string][]string) Context
```

`predict.Context` is a plain struct distinct from `context.Context`, and the driver manages deadlines through `Timeout` and `predict.DefaultTimeout`.

```go
app.Predictor("stack", predict.Callback(func(c predict.Context) predict.Predictor {
	region := c.FlagValue("region", "us-east-1")
	stacks, err := lookupStacks(region) // your data source
	if err != nil {
		return predict.None()
	}

	return predict.Values(stacks...)
}))
```

## Directives

`Directive` constants instruct shells on post-completion behavior (e.g. `DirectiveNoFileCompletion` to disable filename fallback). Set per predictor or per `Result`.

## Interfaces for custom types

| Interface | Method |
|-----------|--------|
| `Predictable` | `Predict() Predictor` |
| `TagAwarePredictable` | `Predict(tags FieldTags) Predictor` |
| `FileFilterable` / `DirFilterable` / `PathFilterable` | Filesystem filtering hooks |

`Predict` runs when the shell asks for the candidates of a field of the type, and it sees no parsed values. Return a `Callback` for candidates that depend on them. Implement either `Predictable` or `TagAwarePredictable`, not both.

### Which predictor a field uses

CONGA takes a field's predictor from the first source that applies:

1. An explicit `predictor:"<name>"` tag: a name registered with `app.Predictor`, or a built-in (`file`, `dir`, `exec`, `symlink`, `none`, `off`).
2. A predictor registered for the field's type with `app.RegisterType(..., conga.WithPredictor(p))`.
3. `FileFilterable`, then `DirFilterable`, then `PathFilterable` on the field's type.
4. `TagAwarePredictable` on the field's type (for example `conga.File`).
5. `Predictable` on the field's type (for example `conga.Dir`).
6. `true` and `false` for a bool flag, after `=`.

With none of these, the field offers no candidates. `predictor:"none"` also offers none.

An `enum` tag narrows the chosen predictor to the listed values with `Include`, so the candidates keep their descriptions and the predictor's order. A candidate stays when the text the shell inserts equals an `enum` value. A type that completes a value in segments with `MultiParts` offers the partial candidates that lead to an `enum` value, then the `enum` value. With `enum:"api-gateway:latest"`, the first segment offers `api-gateway:` and the second offers `api-gateway:latest`. When the predictor never produces an `enum` value's last segment, its partial candidate leads to no candidates. An `enum` value the predictor doesn't produce isn't offered, although validation accepts it. Validation compares the parsed value's text form, so a type that rewrites what the user types (`get` to `GET`) accepts spellings that completion doesn't offer. A field with an `enum` and no predictor source offers the `enum` values. Map fields and path fields (`conga.File`, `conga.Dir`, `conga.FileContent`, filterable types, and `file`, `dir`, `exec`, or `symlink` predictor tags) can't declare an `enum`.

For a slice field, the element type decides, and an `enum` narrows each element's candidates.

A `file`, `dir`, `exec` or `symlink` tag on a map field completes the value after the entry's first `=`, and a custom predictor tag completes the whole entry. For a map field without a `predictor` tag, steps 3 to 5 apply to the map type first. When the map type has no predictor, steps 2 to 5 apply to the value type, and a `bool` value offers `true` and `false`. A value-type predictor completes the text after the entry's first `=` and puts `key=` in front of each candidate. Before the `=`, it offers no candidates.

## Constants

`DefaultTimeout = 250 * time.Millisecond` is the callback deadline that dynamic predictors apply. `Timeout(0)` reapplies it, and a negative duration disables enforcement.

## Domain types

The `network`, `system`, and `timezone` packages provide domain types and predictors. See [Domain types](/reference/domain-types/) for the type inventory, the standard library bindings, and the system-query sentinels.
