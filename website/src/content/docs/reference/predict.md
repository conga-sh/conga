---
title: package predict
description: The decoupled completion SDK — predictors, combinators, items, context, directives.
---

Complete package API documentation is on [pkg.go.dev/github.com/conga-sh/conga/predict](https://pkg.go.dev/github.com/conga-sh/conga/predict); wiring guidance is in [How to wire custom completion](/how-to/wire-custom-completion/).

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

`WithAlias` marks a candidate as an alternative spelling of another candidate, such as a taskfile task alias. Alias candidates are shown by default and suppressed when the user disables them through the `show-aliases` zstyle or the application-prefixed `<APP>_COMPLETION_ALIASES` sideband. Subcommand and flag aliases declared with the `aliases` tag carry the marker automatically. See [How to enable shell completion](/how-to/enable-shell-completion/#customise-completion-behaviour).

## Constructors

| Constructor | Purpose |
|-------------|---------|
| `Values(values ...string)` | Static candidates |
| `Map(items map[string]string)` | Sorted map candidates |
| `KeyValue(valuesByKey map[string][]string)` | `KEY=VALUE` segmented completion |
| `KeyValueDescribed(valuesByKey map[string][]string, descriptionByKey map[string]string)` | `KEY=VALUE` segmented completion with per-key descriptions |
| `Items(items ...Item)` | Explicit item list |
| `Callback(fn func(Context) Predictor)` | Lazy evaluation |
| `MultiParts(delimiter string, fn func(Context) Predictor)` | Multi-segment values |
| `Batch(predictors ...Predictor)` | Aggregate predictors |
| `None()` | Return an empty candidate set |

## Filesystem constructors

`Files(extensions...)`, `FilesFiltered(filter)`, `FilesGlob(patterns...)`, `Dirs()`, `DirsFiltered(filter)`, `DirsGlob(patterns...)`, `PathsFiltered(filter)`, `Symlinks()`, `Executables()`, `FileLines(filePath)`, and `FileContents(filePath, parser)`. Filters are `func(path string, entry os.DirEntry) bool`. `FilesGlob` matches file base names against `filepath.Match` patterns while keeping directories visible for traversal (a wildcard-free pattern such as `"Makefile"` matches that literal name); `DirsGlob` matches directory base names against the same patterns. `DirsFiltered` is the predicate equivalent for directories.

## Combinators on `Predictor`

| Combinator | Purpose |
|------------|---------|
| `Prefix(s)` / `Suffix(s)` | Decorate every candidate |
| `Exclude(values ...string)` | Remove by value or display |
| `FilterText(toComplete)` | Pre-filter by query |
| `Style(s)` / `Tag(t)` | Default display style / group |
| `Usage(msg)` / `Usagef(format, a...)` | ActiveHelp / tooltip hints |
| `NoSpace()` | Suppress the trailing space after insertion |
| `KeepOrder()` | Preserve candidate order |
| `Timeout(d, alternative...)` | Deadline with optional fallback (panics with >1 alternative) |
| `Raw()` | Underlying candidate strings |
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

`predict.Context` is a plain struct distinct from `context.Context`; the driver manages deadlines through `Timeout` and `predict.DefaultTimeout`.

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

`Directive` constants instruct shells on post-completion behaviour (e.g. `DirectiveNoFileCompletion` to disable filename fallback). Set per predictor or per `Result`.

## Interfaces for custom types

| Interface | Method |
|-----------|--------|
| `Predictable` | `Predict() Predictor` |
| `TagAwarePredictable` | `Predict(tags FieldTags) Predictor` |
| `FileFilterable` / `DirFilterable` / `PathFilterable` | Filesystem filtering hooks |

## Constants

`DefaultTimeout = 250 * time.Millisecond` — callback deadline applied by dynamic predictors; `Timeout(0)` reapplies it and a negative duration disables enforcement.

## Domain types

The `network`, `system`, and `timezone` packages provide domain types and predictors. See [Domain types](/reference/domain-types/) for the type inventory, the standard library bindings, and the system-query sentinels.
