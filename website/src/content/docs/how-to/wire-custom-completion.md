---
title: How to wire custom completion
description: Wire custom completion through the registry or self-completing types.
---

`package predict` is a standalone SDK for generating completion candidates. Library authors can import it on its own to ship predictors for their types.

```go
import "github.com/conga-sh/conga/predict"
```

## Two wiring styles

**Registry style** — register a named predictor on the app and reference it from struct tags:

```go
app.Predictor("region", predict.Values("us-east-1", "us-west-2", "eu-central-1"))

type Deploy struct {
	Region conga.OptionalFlag[string] `predictor:"region" help:"Target region"`
}
```

**Self-completing style** — implement `Predictable`, or `TagAwarePredictable` when candidates depend on the field's declared tags, on the type itself. This is how the built-in domain types work — see [Domain Types](../reference/domain-types/) and [How to extend with custom types, maps & decoders](custom-types-and-decoders/).

Both styles resolve to the same predictor values; the [package predict reference](../reference/predict/) catalogues every constructor, combinator, context helper, and directive.
