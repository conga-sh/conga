---
title: How to add custom flag types
description: Decode flag, environment, and config values into any type that implements encoding.TextUnmarshaler, declare tags that let each field customize a type, register a TypeParser with RegisterType, and use custom types as map values.
---

This guide builds a `relay` command with four flags, each of a type outside CONGA's built-in parsing: a port range, a route name checked against a pattern, a CIDR network, and a map of named port ranges.

## Implement `encoding.TextUnmarshaler`

Implement `encoding.TextUnmarshaler`, one of the [interfaces CONGA decodes](/reference/domain-types/#domain-type-interfaces), with a pointer receiver:

```go
type TextUnmarshaler interface {
	UnmarshalText(text []byte) error
}
```

CONGA decodes every type that implements it, including standard library types such as `time.Time` and `netip.Prefix`. Here `*PortRange` implements it:

```go title="main.go" hidden={1-8}
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type PortRange struct {
	Start int
	End   int
}

func (p *PortRange) UnmarshalText(text []byte) error {
	var start, end int
	if n, err := fmt.Sscanf(string(text), "%d-%d", &start, &end); err != nil || n != 2 {
		return fmt.Errorf("invalid port range %q, expected START-END", text)
	}

	p.Start, p.End = start, end

	return nil
}

type Relay struct {
	Ports conga.RequiredFlag[PortRange] `help:"Port range to listen on (START-END)"`
}

func (r *Relay) Run(ctx context.Context, session *conga.Session) error {
	ports := r.Ports.Get()
	fmt.Fprintf(session.Stdout(), "listening on ports %d-%d\n", ports.Start, ports.End)

	return nil
}

func main() {
	app := conga.New[Relay]().
		Name("relay").
		EnvPrefix("RELAY")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```sh
$ relay --ports 8000-8100
listening on ports 8000-8100
```

The default help placeholder comes from the type name:

```sh {6}
$ relay --help
Usage:
  relay --ports=PORT-RANGE

Flags:
      --ports=PORT-RANGE  Port range to listen on (START-END) [env: RELAY_PORTS]
      --help              Show help
      --version           Show version
```

A command-line error from `UnmarshalText` is a usage error (`ErrCoerce`, exit 2):

```sh error={2}
$ relay --ports 80
relay: error: invalid port range "80", expected START-END for flag --ports
```

## Declare custom field tags with `TagSpecProvider`

Tags let one type serve fields with different rules, each declared next to its field. A route name and a tenant ID can both be of type `Slug`, each with its own `pattern`. The built-in `conga.File` declares `existing` and `glob` the same way.

Implement `conga.TagSpecProvider` to declare the tags:

```go
type TagSpecProvider interface {
	TagSpecs() []TagSpec
}
```

Each `conga.TagSpec` names a tag and its kind, and can carry a `Validate` function that CONGA runs on the tag's value at startup.

Implement `conga.ContextAwareTextUnmarshaler` to read the tag values:

```go
type ContextAwareTextUnmarshaler interface {
	UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error
}
```

CONGA passes `UnmarshalTextWithContext` a `conga.DecodeContext` with the field's tag values in `Tags`, and the session's `Stdin` and `Stdout` for a type that reads or writes them.

:::caution
CONGA calls the unmarshaler for every occurrence of a repeated flag, and twice for a `bootstrap` field, once in each binding pass. An unmarshaler that reads a file or `Stdin` reads it each time. CONGA can't tell that a custom type does this, so keep the unmarshaler free of side effects and do the reading in `Run`, as `conga.File` does with `Reader`.
:::

Here `*Slug` implements both. `TagSpecs` declares a `pattern` tag whose `Validate` compiles it at startup, and `UnmarshalTextWithContext` matches each value against it:

```go title="main.go" ins={27}
type Slug string

func (s *Slug) TagSpecs() []conga.TagSpec {
	return []conga.TagSpec{{
		Name: "pattern",
		Kind: conga.TagKindString,
		Validate: func(value string) error {
			_, err := regexp.Compile(value)

			return err
		},
	}}
}

func (s *Slug) UnmarshalTextWithContext(text []byte, decodeContext conga.DecodeContext) error {
	if pattern := decodeContext.Tags.String("pattern"); pattern != "" && !regexp.MustCompile(pattern).Match(text) {
		return fmt.Errorf("%q does not match %s", text, pattern)
	}

	*s = Slug(text)

	return nil
}

type Relay struct {
	Ports conga.RequiredFlag[PortRange] `help:"Port range to listen on (START-END)"`
	Route conga.RequiredFlag[Slug]      `help:"Route name" pattern:"^[a-z][a-z0-9-]*$"`
}
```

`Validate` has already compiled the pattern by the time `UnmarshalTextWithContext` runs, so `regexp.MustCompile` can't panic there. `Run` now also prints the route:

```sh
$ relay --ports 8000-8100 --route checkout-api
listening on ports 8000-8100
route checkout-api
```

```sh error={2}
$ relay --ports 8000-8100 --route Checkout_API
relay: error: "Checkout_API" does not match ^[a-z][a-z0-9-]*$ for flag --route
```

A pattern that doesn't compile stops the program at startup, before it reads any flag (exit 1). Here the field was given `pattern:"^[a-z"`:

```text no-copy error={1} wrap
relay: error: struct Relay field Route: schema error: invalid struct tag: tag "pattern": error parsing regexp: missing closing ]: `[a-z`
```

A misspelled tag stops it the same way:

```text no-copy error={1} wrap
relay: error: struct Relay field Route: schema error: invalid struct tag: unknown struct tag "patern" (did you mean "pattern"?). Use app.AllowTags("patern") to permit external tags
```

## Register a parser with `RegisterType`

When a type from another package, such as `net.IPNet`, implements none of these interfaces, register a `conga.TypeParser` for it with `RegisterType`. A `TypeParser[U]` is a `func(raw string) (U, error)`, so `parseCIDR` below is a `TypeParser[net.IPNet]`. `conga.WithPredictor` attaches completion candidates to every field of that type:

```go title="main.go" ins={3,6-13,19-20}
type Relay struct {
	// ...
	Allow conga.OptionalFlag[net.IPNet] `help:"Network allowed to connect"`
}

func parseCIDR(raw string) (net.IPNet, error) {
	_, network, err := net.ParseCIDR(raw)
	if err != nil {
		return net.IPNet{}, fmt.Errorf("invalid CIDR %q", raw)
	}

	return *network, nil
}

func main() {
	app := conga.New[Relay]().
		Name("relay").
		EnvPrefix("RELAY").
		EnableCompletion().
		RegisterType(parseCIDR, conga.WithPredictor(predict.Values("10.0.0.0/8", "192.168.0.0/16")))
	// ...
}
```

`Run` prints `allowing <network>` when the flag is set:

```sh
$ relay --ports 8000-8100 --route checkout-api --allow 10.1.0.0/16
listening on ports 8000-8100
route checkout-api
allowing 10.1.0.0/16
```

```sh error={2}
$ relay --ports 8000-8100 --route checkout-api --allow 10.1.0.0
relay: error: invalid CIDR "10.1.0.0" for flag --allow
```

With the completion script loaded:

```text no-copy
$ relay --allow <Tab>
Network allowed to connect
10.0.0.0/8      192.168.0.0/16
```

## Register a map type with `RegisterMap`

`map[string]string` binds natively. To support other map types, register each with `RegisterMap`. The key's underlying type must be `string`, and the value type must implement `encoding.TextUnmarshaler` or be one CONGA parses natively. Here the value type is `PortRange`:

```go title="main.go" ins={3,9-12,20}
type Relay struct {
	// ...
	Expose conga.OptionalFlag[map[string]PortRange] `help:"Named port ranges to expose (NAME=START-END)"`
}

func (r *Relay) Run(ctx context.Context, session *conga.Session) error {
	// ...

	expose := r.Expose.Get()
	for _, name := range slices.Sorted(maps.Keys(expose)) {
		fmt.Fprintf(session.Stdout(), "exposing %s on %d-%d\n", name, expose[name].Start, expose[name].End)
	}

	return nil
}

func main() {
	app := conga.New[Relay]().
		// ...
		RegisterMap[map[string]PortRange]()
	// ...
}
```

Users repeat the flag:

```sh
$ relay --ports 8000-8100 --route checkout-api --expose web=80-90 --expose admin=9000-9010
listening on ports 8000-8100
route checkout-api
exposing admin on 9000-9010
exposing web on 80-90
```

Or separate entries with commas:

```sh
$ relay --ports 8000-8100 --route checkout-api --expose web=80-90,admin=9000-9010
listening on ports 8000-8100
route checkout-api
exposing admin on 9000-9010
exposing web on 80-90
```

A bad value names its key:

```sh error={2}
$ relay --ports 8000-8100 --route checkout-api --expose web=80
relay: error: value for key "web": invalid port range "80", expected START-END for flag --expose
```

A registered `TypeParser` decodes scalar and slice fields. Map values decode through the value type's unmarshaler or built-in parsing.

## Know what users see

| Mistake | When it shows | Result |
|---|---|---|
| A value the parser or the type's unmarshaler rejects, from the command line | When the value is bound | Usage error naming the flag, exit 2 |
| The same rejection from the environment or a config file | When the value is bound | `ErrConfig`, exit 1, naming the flag and source |
| A misspelled custom tag, a tag value its `Validate` rejects, or a custom tag on a field of another type | Startup, on every run | `schema error: invalid struct tag`, exit 1 |
| A field of a type with no parser, unmarshaler, or built-in support | Only when a user sets the flag | `unsupported type net.IPNet for flag --allow`, exit 2 |
| A map flag of an unregistered map type | Only when a user sets the flag | `map type map[string]main.PortRange requires registration via app.RegisterMap`, exit 2 |

The last two pass startup and every run that leaves the flag unset. Run the flag once in a test to catch a missing registration.

## Where to go next

- [Domain type interfaces](/reference/domain-types/#domain-type-interfaces): every interface, the decoding order, and the `TagSpec` and `DecodeContext` fields.
- [How to compute and shape completion candidates](/how-to/compute-completion-candidates/): choose where a type's candidates come from, and read field tags in its completion.
- [How to declare flags](/how-to/declare-flags/): the scalar, list, and map flags CONGA parses without a custom type.
- [How to read files and standard input](/how-to/read-files-and-stdin/): use the built-in `File`, `Dir`, `FileContent`, and `ByteSize` types.
- [Domain types](/reference/domain-types/): the built-in `File`, `Dir`, `FileContent`, and `ByteSize` types.
