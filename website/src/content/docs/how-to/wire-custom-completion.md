---
title: How to wire custom completion
description: Supply dynamic completion candidates for flags and arguments using predict SDK combinators, registries, and interfaces.
---

When flags or positional arguments accept dynamic values — such as server names, cluster regions, or API resources — implement custom completion using `package predict`. This guide explains predictor resolution precedence, static and dynamic candidate generation, and self-completing types.

## Understand predictor resolution precedence

When generating completion candidates for a field, CONGA evaluates candidate sources in strict precedence order:

1. **Field-level tag**: An explicit `predictor:"name"` tag overrides all other predictors.
2. **Tag-aware type interface**: The field type implements `conga.TagAwarePredictable`.
3. **Type interface**: The field type implements `conga.Predictable`.
4. **Type-level registration**: A predictor was attached to the type via `app.RegisterType(..., conga.WithPredictor(p))`.
5. **Built-in domain predictor**: Domain types (such as `conga.File` and `conga.Dir`) evaluate their built-in path completers.

## Register named predictors on the builder

For field-specific completion logic, register a named predictor on the application builder, then reference it using the `predictor` struct tag:

```go title="main.go"
import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

func main() {
	app := conga.New[CLI]().
		Name("deployer").
		EnableCompletion()

	app.Predictor("regions", predict.Values(
		"us-east-1",
		"us-west-2",
		"eu-west-1",
	))

	app.FatalIfError(app.Run())
}

type CLI struct {
	Region conga.RequiredFlag[string] `help:"Deployment region" predictor:"regions"`
}
```

In strict tag validation mode (`app.StrictTagValidation(true)` or `conga-lint -strict`), referencing a predictor name that was not registered with `app.Predictor` produces an `ErrSchema` error during initialisation.

## Provide candidate descriptions with predict.Item

To render descriptions alongside completion candidates in shells that support rich completions (such as zsh, fish, and nushell), return `predict.Item` slices:

```go title="main.go"
app.Predictor("environments", predict.PredictorFunc(
	func(ctx context.Context, session *conga.Session) ([]predict.Item, error) {
		return []predict.Item{
			{Value: "dev", Description: "Local development environment"},
			{Value: "staging", Description: "Pre-production verification cluster"},
			{Value: "prod", Description: "Production live cluster"},
		}, nil
	},
))
```

## Generate dynamic candidates at runtime

To query a database, read a configuration directory, or inspect an API at completion time, use `predict.Callback`:

```go title="main.go"
app.Predictor("clusters", predict.Callback(
	func(ctx context.Context, session *conga.Session) ([]predict.Item, error) {
		clusters, err := fetchActiveClusters(ctx)
		if err != nil {
			return nil, err
		}

		items := make([]predict.Item, len(clusters))
		for i, c := range clusters {
			items[i] = predict.Item{Value: c.Name, Description: c.Status}
		}

		return items, nil
	},
))
```

If the callback returns an error or takes too long, CONGA logs the failure to stderr in diagnostic mode and returns an empty candidate list to the shell without crashing the interactive prompt.

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

When completion candidates depend on metadata declared on the field (such as file extensions or resource filters), implement `conga.TagAwarePredictable`:

```go title="main.go"
type DocumentPath string

func (d DocumentPath) Predict(tags conga.FieldTags) predict.Predictor {
	allowedExt := tags.Get("ext") // reads `ext:".json"` from field tag
	if allowedExt == "" {
		return predict.Files()
	}

	return predict.Files(allowedExt)
}

type CLI struct {
	Config conga.RequiredFlag[DocumentPath] `ext:".yaml" help:"Path to configuration YAML"`
}
```

The `tags` parameter provides read-only access to all struct tags declared on the owning field, allowing a single type to adjust its completion logic based on field annotations.

## Where to go next

- [How to enable shell completion](/how-to/enable-shell-completion/) — generate completion scripts for bash, zsh, fish, powershell, elvish, and nushell.
- [Package predict reference](/reference/predict/) — constructor and combinator reference for candidate filtering and caching.
- [About the shell completion architecture](/explanation/completion-architecture/) — driver dispatch and wire framing protocol.
