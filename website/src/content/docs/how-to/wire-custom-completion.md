---
title: How to wire custom completion
description: Supply dynamic completion candidates for flags and arguments using predict SDK combinators, registries, and interfaces.
---

When flags or positional arguments accept dynamic values — such as server names, cluster regions, or API resources — implement custom completion using `package predict`. This guide explains predictor resolution precedence, static and dynamic candidate generation, and self-completing types.

## Predictor resolution precedence

When generating completion candidates for a field, CONGA evaluates candidate sources in strict precedence:

1. Field-level tag: an explicit `predictor:"name"` tag overrides all other sources.
2. Type-level registration: the field type was registered with `app.RegisterType(..., conga.WithPredictor(p))`.
3. Filesystem filter interfaces: the field type implements `predict.FileFilterable`, `predict.DirFilterable`, or `predict.PathFilterable`.
4. Tag-aware type interface: the field type implements `conga.TagAwarePredictable` (including built-in `conga.File`).
5. Parameterless type interface: the field type implements `conga.Predictable` (including built-in `conga.Dir`).

## Register named predictors on the builder

For field-specific completion logic, register a named predictor on the application builder, then reference it using the `predictor` struct tag:

```go title="main.go"
package main

import (
	"context"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

type CLI struct {
	Region conga.RequiredFlag[string] `help:"Deployment region" predictor:"regions"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func main() {
	app := conga.New[CLI]().
		EnableCompletion()

	app.Predictor("regions", predict.Values(
		"us-east-1",
		"us-west-2",
		"eu-west-1",
	))

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

In strict tag validation mode (`app.StrictTagValidation(true)` or `conga-lint -strict`), referencing a predictor name that was not registered with `app.Predictor` produces an `ErrSchema` error during initialization.

## Provide candidate descriptions with predict.Item

To render descriptions alongside completion candidates in shells that support rich completions (such as zsh, fish, and nushell), return `predict.Item` values through `predict.Items`:

```go title="main.go"
app.Predictor("environments", predict.Items(
	predict.NewItem("dev", "Local development environment"),
	predict.NewItem("staging", "Pre-production verification cluster"),
	predict.NewItem("prod", "Production live cluster"),
))
```

## Generate dynamic candidates at runtime

To query a database, read a configuration directory, or inspect an API at completion time, use `predict.Callback`:

```go title="main.go"
app.Predictor("clusters", predict.Callback(func(compCtx predict.Context) predict.Predictor {
	region := compCtx.FlagValue("region", "us-east-1")

	clusters, err := lookupClusters(region)
	if err != nil {
		return predict.None()
	}

	items := make([]predict.Item, len(clusters))
	for index, cluster := range clusters {
		items[index] = predict.NewItem(cluster.Name, cluster.Status)
	}

	return predict.Items(items...)
}))
```

`predict.Callback` takes no error return: when the data source fails, return `predict.None()`. Dynamic callbacks run under a 250 ms deadline (`predict.DefaultTimeout`); on overrun, CONGA returns an empty candidate set instead of blocking the interactive prompt. Pass a fallback predictor to `Timeout` to return candidates on overrun.

When an application uses [`conga.ConfigLoader`](/how-to/load-domain-files/), CONGA binds fields tagged `bootstrap:"true"` (such as `--file`, `--dir`, or `--profile`) from the command line and executes the loader before evaluating completion callbacks. This allows dynamic predictors to inspect configuration extracted from the file and return context-aware candidates.

## Make a type self-completing

To associate completion directly with a Go type across the entire application, implement `conga.Predictable`:

```go title="main.go"
type ClusterID string

func (c ClusterID) Predict() predict.Predictor {
	return predict.Values("alpha", "beta", "gamma")
}

type CLI struct {
	Target conga.RequiredFlag[ClusterID] `help:"Target cluster identifier"`
}
```

Fields using `ClusterID` complete automatically without requiring an explicit `predictor` tag.

## Inspect struct tags with TagAwarePredictable

When completion candidates depend on metadata declared on the field (such as file extensions or resource filters), implement `conga.TagAwarePredictable` and register the tags the type accepts with `conga.TagSpecProvider`:

```go title="main.go"
type DocumentPath string

func (d DocumentPath) TagSpecs() []conga.TagSpec {
	return []conga.TagSpec{{Name: "ext", Kind: conga.TagKindString}}
}

func (d DocumentPath) Predict(tags conga.FieldTags) predict.Predictor {
	allowedExtension := tags.String("ext") // reads `ext:".yaml"` from the field tag
	if allowedExtension == "" {
		return predict.Files()
	}

	return predict.Files(allowedExtension)
}

type CLI struct {
	Config conga.RequiredFlag[DocumentPath] `ext:".yaml" help:"Path to configuration YAML"`
}
```

The `tags` parameter provides read-only access to the tags registered by the type: `Has(key)`, `Bool(key)`, and `String(key)`. A type declaring custom tags without a `TagSpecProvider` fails strict tag validation at schema build.

## Where to go next

- [How to enable shell completion](/how-to/enable-shell-completion/) — generate completion scripts for bash, zsh, fish, powershell, elvish, and nushell.
- [Package predict reference](/reference/predict/) — constructor and combinator reference for candidate filtering and caching.
