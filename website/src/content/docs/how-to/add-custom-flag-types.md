---
title: How to add custom flag types
description: Parse user-defined types, unmarshalers, and map containers from command-line strings.
---

CONGA supports user-defined flag types through two mechanisms: implementing Go's standard unmarshaling interfaces directly on custom types, or registering external parser functions on the application builder.

## Implement standard text unmarshaling

If you own the type, implement standard `encoding.TextUnmarshaler`. Types whose pointer receiver implements `UnmarshalText([]byte) error` are automatically supported without builder registration:

```go title="main.go"
type PortRange struct {
	Start int
	End   int
}

func (p *PortRange) UnmarshalText(text []byte) error {
	var start, end int
	if n, err := fmt.Sscanf(string(text), "%d-%d", &start, &end); err != nil || n != 2 {
		return fmt.Errorf("invalid port range %q, expected START-END", text)
	}

	p.Start = start
	p.End = end

	return nil
}

type Service struct {
	Ports conga.RequiredFlag[PortRange] `help:"Port range in format START-END"`
}
```

When a flag value cannot be parsed, CONGA wraps the returned error in `conga.ErrCoerce`, identifies the flag name, and exits with code 2:

```sh
$ service --ports 80
service: error: invalid value "80" for flag --ports: invalid port range "80", expected START-END
```

## Access field tags and streams during unmarshaling

To inspect the owning field's struct tags or read from the engine's configured streams during decoding, implement `conga.ContextAwareTextUnmarshaler`:

```go
type ContextAwareTextUnmarshaler interface {
	UnmarshalTextWithContext(text []byte, decodeContext conga.DecodeContext) error
}
```

The `decodeContext` parameter provides the field's declared tags (`Tags`) and standard streams (`Stdin`, `Stdout`):

```go title="main.go"
type SecretInput struct {
	Value string
}

func (s *SecretInput) UnmarshalTextWithContext(text []byte, decodeContext conga.DecodeContext) error {
	if string(text) == "-" {
		data, err := io.ReadAll(decodeContext.Stdin)
		if err != nil {
			return fmt.Errorf("reading secret from stdin: %w", err)
		}
		s.Value = strings.TrimSpace(string(data))
		return nil
	}

	s.Value = string(text)
	return nil
}
```

The coercion pipeline prefers `ContextAwareTextUnmarshaler` over `encoding.TextUnmarshaler`. If `UnmarshalTextWithContext` is implemented, the engine invokes it exclusively.

## Register external types with RegisterType

If you do not own the target type or want to attach type-level autocompletion, register a parser function using `app.RegisterType`:

```go title="main.go"
func parseCIDR(raw string) (net.IPNet, error) {
	_, ipnet, err := net.ParseCIDR(raw)
	if err != nil {
		return net.IPNet{}, fmt.Errorf("invalid CIDR %q: %w", raw, err)
	}

	return *ipnet, nil
}

func main() {
	app := conga.New[CLI]().
		Name("nettool")

	app.RegisterType(
		parseCIDR,
		conga.WithPredictor(predict.Values("10.0.0.0/8", "192.168.0.0/16")),
	)

	app.FatalIfError(app.Run())
}
```

The parser function signature must be `func(string) (U, error)`. The engine infers the target type `U` from the return value.

`conga.WithPredictor(p)` associates a completion provider with every field of type `U`. A field can override this default with an explicit `predictor:"..."` struct tag.

Re-registering a type overwrites the previous parser, allowing applications to replace built-in or library-provided decoders.

## Register custom map values

CONGA natively decodes primitive map types such as `map[string]string` and `map[string]int` without registration. When the map value is a custom type, register the map type with `app.RegisterMap`:

```go title="main.go"
type Router struct {
	Routes map[string]net.IPNet `help:"Routing table entries in key=cidr format"`
}

func main() {
	app := conga.New[Router]().
		Name("router")

	app.RegisterType(parseCIDR)
	app.RegisterMap(map[string]net.IPNet(nil))

	app.FatalIfError(app.Run())
}
```

Map keys must be strings. Registering a map type with a non-string key panics during application initialisation.

Supply map entries using `KEY=VALUE` syntax across repeated flags or separated by commas:

```sh
$ router --routes primary=10.0.0.0/8 --routes backup=192.168.0.0/16
```

## Where to go next

- [How to wire custom completion](/how-to/wire-custom-completion/) — build completion providers for flags and custom types with `package predict`.
- [Domain types](/reference/domain-types/) — reference catalog of built-in types including `File`, `Dir`, and `ByteSize`.
- [About flags and args](/explanation/flags-and-args/) — typed container semantics and constraints.
