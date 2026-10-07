---
title: package predict
description: The decoupled completion SDK — predictors, combinators, items, context, directives.
---

Deep detail lives on [pkg.go.dev/github.com/conga-sh/conga/predict](https://pkg.go.dev/github.com/conga-sh/conga/predict); wiring guidance is in [How to wire custom completion](/how-to/wire-custom-completion/).

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
	// …style and tag fields; use NewItem for the common case
}

func NewItem(value, description string) Item
func (i Item) WithDisplay(display string) Item
func (i Item) WithDescription(description string) Item
func (i Item) WithStyle(style string) Item
func (i Item) WithTag(tag string) Item
```

## Constructors

| Constructor | Purpose |
|-------------|---------|
| `Values(values ...string)` | Static candidates |
| `Map(items map[string]string)` | Sorted map candidates |
| `KeyValue(valuesByKey map[string][]string)` | `KEY=VALUE` segmented completion |
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
}

func (c Context) Arg(index int) string
func (c Context) FlagValue(name string, fallback string) string
func (c Context) FlagValues(name string) []string
func (c Context) Getenv(key string) string
func (c Context) HasFlag(name string) bool
func (c Context) LastArg() string
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

`Directive` constants instruct shells on post-completion behaviour (e.g. `DirectiveNoFileComp` to disable filename fallback). Set per predictor or per `Result`.

## Interfaces for custom types

| Interface | Method |
|-----------|--------|
| `Predictable` | `Predict() Predictor` |
| `TagAwarePredictable` | `Predict(tags FieldTags) Predictor` |
| `FileFilterable` / `DirFilterable` / `PathFilterable` | Filesystem filtering hooks |

## Constants

`DefaultTimeout = 250 * time.Millisecond` — callback deadline applied by dynamic predictors; `Timeout(0)` reapplies it and a negative duration disables enforcement.

## Subpackages

- [`predict/network`](https://pkg.go.dev/github.com/conga-sh/conga/predict/network) — `Host`, `Port`, `HTTPMethod`, `HTTPStatus`, `MIMEType`, `NetInterface` + predictors
- [`predict/system`](https://pkg.go.dev/github.com/conga-sh/conga/predict/system) — `User`, `Group`, `Signal`, `PID`, `Process`, `EnvVar`, `Locale` + predictors
- [`predict/timezone`](https://pkg.go.dev/github.com/conga-sh/conga/predict/timezone) — IANA `Timezones()` predictor

Coercion-time validation failures of these domain types carry no per-type
sentinels: the engine classifies them under `ErrCoerce`, and the rendered error
names the flag and the reason. The system-query methods report absent input via
`ErrEmptyHost`, `ErrEmptyInterface`, `ErrEmptyUser`, or `ErrEmptyGroup`, plus
the wrapped OS/network error otherwise.
