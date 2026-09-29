---
title: How to extend with custom types, maps & decoders
description: Register flag parsers, map parsers, configuration formats, and self-completing domain types.
---

This page covers four extension points: custom flag types, custom map types, configuration decoders, and self-completing domain types.

## Register a custom flag type

Any type can become a flag/argument type by registering a parser for it. Parsers are plain functions from string to value:

```go
type CIDR string

func parseCIDR(raw string) (CIDR, error) {
	if _, _, err := net.ParseCIDR(raw); err != nil {
		return "", fmt.Errorf("invalid CIDR %q: %w", raw, err)
	}

	return CIDR(raw), nil
}

func main() {
	app := conga.New[CLI]().
		Name("nettool")

	app.RegisterType(parseCIDR, conga.WithPredictor(predict.Values("10.0.0.0/8", "192.168.0.0/16")))
	...
}
```

- `app.RegisterType(parser, opts...)` — `U`, the value type, is inferred from the parser. Re-registering a type overwrites the previous parser (last-write-wins), so libraries can ship defaults you then customize.
- `conga.WithPredictor(p)` attaches a completion predictor used by fields of that type, and a field can override it with a `predictor:"..."` tag.
- Panics are reserved for misuse (nil app, nil parser).
- Types implementing `encoding.TextUnmarshaler` (like all built-in [domain types](../reference/domain-types/)) coerce automatically, and may self-validate via the `SelfValidator` interface (`Validate(ctx) error`).

## Register custom map types

Typed maps (`map[K]V` flags) work natively for primitives. For custom value types register a map parser — passing a **sample map** lets Go infer the parameters:

```go
type Metadata map[string]CIDR

app.RegisterMap(map[string]CIDR(nil), conga.WithPredictor(myPredictor))
```

- Syntax is `KEY=VALUE` per occurrence; entries merge into the map.
- Only string keys are supported; non-string keys panic.
- Key and value coercion errors are typed (`ErrCoerce` chain) and identify which side failed.

## Register configuration formats

Decoders are registered per application with `app.RegisterDecoder(extension, decoder)`. A decoder returns a flat `map[string]any` whose keys match the [config keys](../explanation/value-precedence/#wiring-the-config-tier) of your flags. The built-in decoders (`decoder/json`, `yaml`, `toml`, `hcl`) follow the same contract and live in their own packages, so only registered formats are linked into your binary. See [package decoder](../reference/decoders/) for the `config.Decoder` signature and built-in parse sentinels.

## Make a type self-completing

Implement `Predictable` on any custom type used as a flag or argument type parameter. Types whose candidates depend on the field's declared custom tags implement `TagAwarePredictable` instead; the engine prefers the tag-aware form and falls back to the parameterless one. Implement exactly one form of the pair:

```go
type Predictable interface {
	Predict() predict.Predictor
}

type TagAwarePredictable interface {
	Predict(tags FieldTags) predict.Predictor
}

type Env string

func (e *Env) Predict() predict.Predictor {
	return system.EnvVars() // any predictor from the SDK
}
```

The built-in types are catalogued in [Domain Types](../reference/domain-types/); the full SDK is in [package predict](../reference/predict/), and wiring styles are covered in [How to wire custom completion](wire-custom-completion/).

## Register custom field tags

Domain types register the struct tags they accept by implementing `TagSpecProvider`, receive them at decode time via `ContextAwareTextUnmarshaler`, and validate their resolved value during the validation phase via `SelfValidator`:

```go
type ContextAwareTextUnmarshaler interface {
	UnmarshalTextWithContext(text []byte, decodeContext DecodeContext) error
}

type SelfValidator interface {
	Validate(ctx context.Context) error
}

type TagSpecProvider interface {
	TagSpecs() []TagSpec
}
```

This is how `File` registers `existing`, `stdin`, `stdout`, `executable`, `symlink`, and `glob`. Registered tag keys join the strict tag-key vocabulary, values are parsed at schema build (strict booleans, non-empty validated strings), excluded combinations are rejected at startup, and the captured tags drive validation. The same tags tailor the type's default help description (`Help(tags)` on `TagAwareHelpProvider`) and its completion candidates (`Predict(tags)` on `TagAwarePredictable`), shown on help screens and in completion candidate descriptions when no explicit `help` tag is declared. `conga.TagSpec` declares each tag's name and kind (`conga.TagKindBool` / `conga.TagKindString`).

## Struct tag policy

Strict tag linting is on by default. When your structs carry third-party tags:

- Allow specific tags globally: `app.AllowTags("prometheus", "db")`.
- Allow per field: `conga:"allow-unknown-tags"` (add `,recurse` to extend into nested command structs).
- Opt out entirely: `app.StrictTagValidation(false)`.
- Lint the schema statically in CI with [How to lint CONGA schemas](lint-schemas/).

See the [`conga` directive reference](../reference/struct-tags/#the-conga-directive-tag).
