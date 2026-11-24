# 07 — Dynamic validation

The `conga.Validator` interface: pure, side-effect-free cross-field semantic
validation that runs before any lifecycle hook.

## Run it

```sh
go run ./_examples/07-dynamic-validation
go run ./_examples/07-dynamic-validation --start-date 2026-03-01 --end-date 2026-01-01
go run ./_examples/07-dynamic-validation --max-retries 3 --backoff 0s
```

## What to look for

- Date ordering and retry/backoff consistency are checked in `Validate`.
- `Validate` reports mistakes with `session.UsageErrorf`, so failures exit with code 2 and a descriptive message. A plain error would exit with code 1 and print no usage block.
- Validation happens before `PreRun`/`Run`; nothing observable has executed yet.

## Test

```sh
go test ./_examples/07-dynamic-validation
```

Related: [06 — Flag constraints](../06-flag-constraints/),
[12 — Lifecycle hooks](../12-lifecycle-hooks/).
