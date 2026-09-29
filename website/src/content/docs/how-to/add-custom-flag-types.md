---
title: How to add custom flag types
description: Parse user-defined types and map containers from command-line strings.
---

This guide demonstrates how to register custom parsing functions for user-defined scalar types and map containers in CONGA.

## Register a custom scalar parser

Register a custom parsing function with `app.RegisterType(parser, opts...)` on the application builder. Parsers convert raw string input to the target value type:

```go title="main.go"
package main

import (
	"context"
	"fmt"
	"net"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

type CIDR string

func parseCIDR(raw string) (CIDR, error) {
	if _, _, err := net.ParseCIDR(raw); err != nil {
		return "", fmt.Errorf("invalid CIDR %q: %w", raw, err)
	}

	return CIDR(raw), nil
}

type CLI struct {
	Network conga.RequiredFlag[CIDR] `help:"Target network CIDR" short:"n"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "network: %s\n", c.Network.Get())

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("nettool")

	app.RegisterType(parseCIDR, conga.WithPredictor(predict.Values("10.0.0.0/8", "192.168.0.0/16")))

	app.FatalIfError(app.Run())
}
```

The target type `U` is inferred from the parser function signature `func(string) (U, error)`.

`conga.WithPredictor(p)` attaches completion candidates used by fields of that type. A field can override the type predictor with an explicit `predictor:"..."` struct tag.

Re-registering a type overwrites the previous parser, allowing applications to override defaults provided by imported libraries.

## Register a custom map parser

Primitive map types (`map[string]string`, `map[string]int`) parse automatically. When the map value is a custom type, register the map parser by passing a nil instance of the map type to `app.RegisterMap`:

```go title="main.go"
type RouteMap map[string]CIDR

type Router struct {
	Routes map[string]CIDR `help:"Routing table entries in key=cidr format"`
}

func main() {
	app := conga.New[Router]().
		Name("router")

	app.RegisterType(parseCIDR)
	app.RegisterMap(map[string]CIDR(nil))

	app.FatalIfError(app.Run())
}
```

Map entries use `KEY=VALUE` syntax per occurrence on the command line:

```sh
$ go run . --routes primary=10.0.0.0/8 --routes backup=192.168.0.0/16
```

Only string map keys are supported. Non-string map keys produce a panic at registration time.

## Handle coercion failures

When a parser returns a non-nil error, CONGA wraps it in `conga.ErrCoerce`. The error reports the field name, the supplied raw string, and the underlying error message from the parser function:

```sh
$ go run . --network invalid
nettool: error: invalid value "invalid" for flag --network: invalid CIDR "invalid": invalid CIDR address: invalid
```

## Where to go next

- [How to wire custom completion](/how-to/wire-custom-completion/) — build completion logic for flags and custom types with `package predict`.
- [Domain types](/reference/domain-types/) — reference catalog of built-in types such as `File`, `Dir`, and `ByteSize`.
- [About flags and args](/explanation/flags-and-args/) — typed container semantics and constraints.
