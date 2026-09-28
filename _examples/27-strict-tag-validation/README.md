# 27 — Strict tag validation

Opting out of strict struct-tag linting with `app.StrictTagValidation(false)`.
The schema mixes CONGA tags with third-party `validate:"..."` and `db:"..."`
tags owned by other libraries; the default strict mode would reject those
unknown keys with an `ErrSchema` error before `Run`.

## Run it

```sh
go run ./_examples/27-strict-tag-validation --email ops@example.com
go run ./_examples/27-strict-tag-validation --help
```

## What to look for

- The schema builds and runs despite the foreign tag keys.
- Third-party tags are inert: they never appear in help or the inspector.
- Prefer the narrower escapes when only a few fields are foreign:
  `app.AllowTags(...)` (see [99 — Kitchen sink](../99-kitchen-sink/)) or a
  per-field `conga:"allow-unknown-tags"` directive.

## Test

```sh
go test ./_examples/27-strict-tag-validation
```

Related: [Reference: struct tags](https://docs.conga.sh/reference/struct-tags/),
[99 — Kitchen sink](../99-kitchen-sink/).
