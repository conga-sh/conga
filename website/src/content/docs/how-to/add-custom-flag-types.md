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

```sh error={2}
$ service --ports 80
service: error: invalid port range "80", expected START-END for flag --ports
```

## Access field tags and streams during unmarshaling

To inspect the owning field's struct tags or read from the engine's configured streams during decoding, implement `conga.ContextAwareTextUnmarshaler`:

```go
type ContextAwareTextUnmarshaler interface {
	UnmarshalTextWithContext(text []byte, decodeContext conga.DecodeContext) error
}
```

To accept custom struct tags on fields using the type, implement `conga.TagSpecProvider`. CONGA validates struct tags strictly at schema build, rejecting any tags not registered by the type:

```go title="main.go"
type SecretInput struct {
	Value string
}

func (s *SecretInput) TagSpecs() []conga.TagSpec {
	return []conga.TagSpec{
		{Name: "stdin", Kind: conga.TagKindBool},
		{Name: "prefix", Kind: conga.TagKindString},
	}
}

func (s *SecretInput) UnmarshalTextWithContext(text []byte, decodeContext conga.DecodeContext) error {
	var raw string
	if string(text) == "-" {
		if !decodeContext.Tags.Bool("stdin") {
			return errors.New("reading from stdin is not enabled for this field")
		}

		data, err := io.ReadAll(decodeContext.Stdin)
		if err != nil {
			return fmt.Errorf("reading secret from stdin: %w", err)
		}

		raw = strings.TrimSpace(string(data))
	} else {
		raw = string(text)
	}

	if prefix := decodeContext.Tags.String("prefix"); prefix != "" {
		if !strings.HasPrefix(raw, prefix) {
			return fmt.Errorf("secret must start with prefix %q", prefix)
		}
	}

	s.Value = raw

	return nil
}

type CLI struct {
	Token conga.RequiredFlag[SecretInput] `help:"API token or - for stdin" stdin:"true" prefix:"sk-"`
}
```

The `decodeContext.Tags` accessor methods inspect declared custom tags:
- `Bool(key)` reports whether a boolean tag was set to `"true"`.
- `String(key)` returns the declared string tag value, or `""` if omitted.
- `Has(key)` reports whether the tag was declared on the field.

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
	app := conga.New[CLI]()

	app.RegisterType(
		parseCIDR,
		conga.WithPredictor(predict.Values("10.0.0.0/8", "192.168.0.0/16")),
	)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

The parser function signature must be `func(string) (U, error)`. The engine infers the target type `U` from the return value.

`conga.WithPredictor(p)` associates a completion provider with every field of type `U`. A field can override this default with an explicit `predictor:"..."` struct tag.

Re-registering a type overwrites the previous parser, allowing applications to replace built-in or library-provided decoders.

## Register custom map values

A `map[string]string` flag binds with no registration. Every other map type needs `app.RegisterMap`, and its value type must be one CONGA can parse: a built-in type such as `int` or `time.Duration`, or a type that implements `encoding.TextUnmarshaler`. This variation adds named port ranges to `Service`, reusing `PortRange` as the map value:

```go title="main.go" ins={3,17}
type Service struct {
	Ports  conga.RequiredFlag[PortRange]            `help:"Port range in format START-END"`
	Expose conga.OptionalFlag[map[string]PortRange] `help:"Named port ranges in NAME=START-END format"`
}

func (s *Service) Run(ctx context.Context, session *conga.Session) error {
	expose := s.Expose.Get()
	for _, name := range slices.Sorted(maps.Keys(expose)) {
		fmt.Fprintf(session.Stdout(), "%s: %d-%d\n", name, expose[name].Start, expose[name].End)
	}

	return nil
}

func main() {
	app := conga.New[Service]().
		RegisterMap[map[string]PortRange]()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

Map keys must have an underlying string type; `RegisterMap`'s type constraints enforce this at compile time.

Users pass entries as `KEY=VALUE`, repeating the flag or separating entries with commas:

```sh
$ service --ports 8000-8100 --expose web=80-90 --expose admin=9000-9010
admin: 9000-9010
web: 80-90
```

`--expose web=80-90,admin=9000-9010` gives the same map. A bad value names its key:

```sh error={2}
$ service --ports 8000-8100 --expose web=80
service: error: value for key "web": invalid port range "80", expected START-END for flag --expose
```

Without the `RegisterMap` call, the flag fails on first use with `map type map[string]main.PortRange requires registration via app.RegisterMap`.

## Where to go next

- [How to wire custom completion](/how-to/wire-custom-completion/) — build completion providers for flags and custom types with `package predict`.
- [Domain types](/reference/domain-types/) — reference catalog of built-in types including `File`, `Dir`, and `ByteSize`.
- [Struct tags](/reference/struct-tags/#domain-type-tags) — declare custom tags, validation hooks, and exclusion rules on domain types.
- [About flags and args](/explanation/flags-and-args/) — typed container semantics and constraints.
