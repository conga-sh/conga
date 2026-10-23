# 27 — Strict tag validation

Registering third-party struct-tag keys with `app.AllowTags(...)`.
The schema mixes CONGA tags with third-party `validate:"..."` and `db:"..."`
tags owned by other libraries; tag validation rejects unknown keys with an
`ErrSchema` error before `Run` unless the app registers them.

## Run it

```sh
go run ./_examples/27-strict-tag-validation --email ops@example.com
go run ./_examples/27-strict-tag-validation --help
```

## What to look for

- The schema builds and runs once the foreign tag keys are registered.
- Registered keys are matched exactly: a misspelling of a registered key is
  still rejected with an `ErrSchema` error.
- Third-party tags are inert: they never appear in help or the inspector.

## Test

```sh
go test ./_examples/27-strict-tag-validation
```

Related: [Reference: struct tags](https://docs.conga.sh/reference/struct-tags/),
[99 — Kitchen sink](../99-kitchen-sink/).
