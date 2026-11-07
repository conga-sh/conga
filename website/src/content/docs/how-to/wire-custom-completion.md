---
title: How to wire custom completion
description: Offer completion candidates for flags and arguments, with descriptions, values computed at Tab time, self-completing types, and tag-aware types.
---

Shell completion lists the values a flag accepts when the user presses Tab. CONGA already completes `enum` values and the `conga.File` and `conga.Dir` types; a plain `string` flag offers nothing until you give it candidates. This guide adds them: a fixed list, values with descriptions, types that complete themselves, and values computed from other flags.

This guide builds a small `deploy` command with four flags, each completed a different way. The listings are zsh's; other shells show the same candidates in their own layout. The [shell completion example](https://github.com/conga-sh/conga/tree/main/_examples/19-shell-completion) uses every predictor and combinator in `package predict`.

Completion needs `EnableCompletion()` on the builder and the script installed in the user's shell; see [How to enable shell completion](/how-to/enable-shell-completion/).

## Offer a fixed list of values

Register a named predictor with `Predictor`, and point the flag at it with a `predictor` tag:

```go title="main.go" ins={3,10}
type CLI struct {
	// ...
	Image conga.OptionalFlag[string] `help:"Container image" predictor:"images"`
}

func main() {
	app := conga.New[CLI]().
		Name("deploy").
		EnableCompletion().
		Predictor("images", predict.Values("api:1.4", "api:1.5", "worker:2.0"))
	// ...
}
```

```text no-copy
$ deploy --image <Tab>
Container image
api:1.4     api:1.5     worker:2.0
```

zsh shows the flag's `help` text above the candidates.

## Show a description with each value

Use `predict.Items` with `predict.NewItem(value, description)` in place of `predict.Values`. Shells that support descriptions, such as zsh, fish, and nushell, show them beside each value. The next section wraps this predictor in a type:

```go title="main.go"
predict.Items(
	predict.NewItem("us-east-1", "North America (N. Virginia)"),
	predict.NewItem("eu-west-1", "Europe (Ireland)"),
	predict.NewItem("ap-southeast-1", "Asia Pacific (Singapore)"),
)
```

```text no-copy
$ deploy --region <Tab>
Target region
ap-southeast-1  -- Asia Pacific (Singapore)
eu-west-1       -- Europe (Ireland)
us-east-1       -- North America (N. Virginia)
```

## Make a type complete itself

When a type always offers the same candidates, give it a `Predict` method. Every field of that type then completes with no `predictor` tag:

```go title="main.go"
type Region string

func (r Region) Predict() predict.Predictor {
	return predict.Items(
		predict.NewItem("us-east-1", "North America (N. Virginia)"),
		predict.NewItem("eu-west-1", "Europe (Ireland)"),
		predict.NewItem("ap-southeast-1", "Asia Pacific (Singapore)"),
	)
}

type CLI struct {
	Region conga.OptionalFlag[Region] `default:"us-east-1" help:"Target region"`
	// ...
}
```

CONGA calls `Predict` once, when it builds the schema, so the candidates are fixed for the run. For candidates that depend on what the user has typed, return a callback.

## Compute values when the user presses Tab

`predict.Callback` runs at completion time and can read other flags already on the command line. Here the clusters depend on `--region`:

```go title="main.go"
app.Predictor("clusters", predict.Callback(func(compCtx predict.Context) predict.Predictor {
	region := compCtx.FlagValue("region", "us-east-1")

	return predict.Values(clustersByRegion[region]...)
}))
```

```go title="main.go"
Cluster conga.OptionalFlag[string] `help:"Cluster in the region" predictor:"clusters"`
```

The second argument to `FlagValue` is the value to use when the flag is absent, so the list follows the default region until the user picks another:

```text no-copy
$ deploy --cluster <Tab>
Cluster in the region
prod-us-1     staging-us-1
```

```text no-copy
$ deploy --region eu-west-1 --cluster <Tab>
Cluster in the region
prod-eu-1
```

A callback has 250 ms (`predict.DefaultTimeout`) to answer. A slower one gives no candidates, so the prompt stays responsive. To offer something on a slow lookup, chain `.Timeout(0, fallback)`, which keeps the default deadline and returns `fallback` when it passes. When the data source fails, return `predict.None()`.

When the application implements [`conga.Bootstrapper`](/how-to/load-domain-files/), `Bootstrap` runs before callbacks, so values from a document or `.env` file can shape the candidates.

## Read field tags in a type's completion

A type can read tags from the field that uses it. List the tags in `TagSpecs`, and implement `Predict(tags conga.FieldTags)` in place of `Predict()`:

```go title="main.go"
type Manifest string

func (m Manifest) TagSpecs() []conga.TagSpec {
	return []conga.TagSpec{{Name: "ext", Kind: conga.TagKindString}}
}

func (m Manifest) Predict(tags conga.FieldTags) predict.Predictor {
	if extension := tags.String("ext"); extension != "" {
		return predict.Files(extension)
	}

	return predict.Files()
}

type CLI struct {
	// ...
	Manifest conga.OptionalFlag[Manifest] `ext:".yaml" help:"Deployment manifest"`
}
```

With `ext:".yaml"`, the listing offers directories, so the user can move through them, and YAML files only:

```text no-copy
$ deploy --manifest <Tab>
Deployment manifest
charts/
deploy.yaml   staging.yaml
```

`TagSpecs` also tells strict tag validation that `ext` belongs on this field. `FieldTags` offers `Has`, `Bool`, and `String`.

## Know what users see

| Situation | Result |
|---|---|
| A `predictor` tag names an unregistered predictor | The program fails at startup, before any completion (exit 1), with a "did you mean" hint |
| A callback takes longer than 250 ms | No candidates, or the `Timeout` fallback |
| A callback returns `predict.None()` | No candidates |

```text no-copy error={1}
deploy: error: struct CLI field Image: schema error: invalid struct tag: unregistered predictor "imgs" (did you mean "images"?). Register it with app.Predictor("imgs", action)
```

When a field has several possible sources, such as a tag and a self-completing type, the tag wins; [Which predictor a field uses](/reference/predict/#which-predictor-a-field-uses) lists the full order.

## Where to go next

- [How to enable shell completion](/how-to/enable-shell-completion/): install the completion script in each shell.
- [Package predict](/reference/predict/): every constructor, combinator, and interface.
- [Shell completion example](https://github.com/conga-sh/conga/tree/main/_examples/19-shell-completion): every predictor in one program.
