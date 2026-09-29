---
title: How to wire custom completion
description: Wire custom completion through the registry or self-completing types.
---

This guide demonstrates how to provide dynamic autocompletion candidates for flags and arguments using the predictor registry or self-completing types.

## Import the prediction SDK

`package predict` is a standalone SDK for generating completion candidates with zero dependency on the root framework:

```go
import "github.com/conga-sh/conga/predict"
```

## Register a named predictor

Register a named predictor on the application builder, then reference it with the `predictor` struct tag:

```go title="main.go"
app.Predictor("region", predict.Values("us-east-1", "us-west-2", "eu-central-1"))

type Deploy struct {
	Region conga.OptionalFlag[string] `help:"Target region" predictor:"region"`
}
```

When completion runs, CONGA invokes the registered predictor to supply candidates for `--region`.

## Make a type self-completing

Implement `Predictable` on a custom type used as a flag or argument type parameter. When completion candidates depend on struct tags declared on the field, implement `TagAwarePredictable` instead:

```go title="main.go"
type Predictable interface {
	Predict() predict.Predictor
}

type TagAwarePredictable interface {
	Predict(tags conga.FieldTags) predict.Predictor
}
```

Implement the interface on the type:

```go title="main.go"
type Environment string

func (e Environment) Predict() predict.Predictor {
	return predict.Values("development", "staging", "production")
}

type CLI struct {
	Env conga.OptionalFlag[Environment] `help:"Target environment"`
}
```

Fields using `Environment` complete their candidates automatically without an explicit `predictor` tag.

## Where to go next

- [How to enable shell completion](/how-to/enable-shell-completion/) — generate completion scripts for bash, zsh, fish, powershell, elvish, and nushell.
- [Package predict](/reference/predict/) — constructor and combinator reference for candidate generation.
- [Domain types](/reference/domain-types/) — reference catalog of self-completing types.
