# 16 — Help variables

A `help` tag may interpolate a field's declared constraints with `{{...}}`
variables instead of restating them in prose. This example contrasts both
renderings of the same constraint.

## Run it

```sh
go run ./_examples/16-help-variables --help
```

## What to look for

- `--automatic` uses plain prose; the engine appends the declared bound as
  `[1..65535]`.
- `--inlined` uses `{{range}}`, so the bound appears once in the prose and the
  automatic hint is suppressed.
- `{{enum}}`, `{{items}}`, `{{default}}`, and `{{env}}` inline the matching
  `enum`, `items`, `default`, and `env` tags.
- Referencing `{{default}}` moves the default out of the value slot.
- A literal `{{` is written by doubling the opening braces (`{{{{`).
- Referencing a variable the field does not declare fails schema build.

See [Reference: struct tags](https://docs.conga.sh/reference/struct-tags/#help-variables)
for the full help-variable reference.

## Test

```sh
go test ./_examples/16-help-variables
```

Related: [06 — Flag constraints](../06-flag-constraints/),
[02 — Flag primitives](../02-flag-primitives/).
