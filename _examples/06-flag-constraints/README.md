# 06 — Flag constraints

Declarative tag-based constraints: mutual exclusion (`xor`), co-requisites
(`and`), `conga.RequiredFlag`, numeric `range` (int, duration, byte size),
`len`, `items`, and `enum` — plus full-help rendering on usage errors.

## Run it

```sh
go run ./_examples/06-flag-constraints -k secret --json -u admin --password hunter2 -z a -z b
go run ./_examples/06-flag-constraints -k secret --json --yaml
```

## What to look for

- Constraint violations exit with code 2 and render full help
  (`UsageOnError(conga.UsageFull)`).
- `range` works across `time.Duration` and `conga.ByteSize`, not just integers.
- `RequiredFlag.Source()` reports where the required value came from.

## Test

```sh
go test ./_examples/06-flag-constraints
```

Related: [07 — Dynamic validation](../07-dynamic-validation/),
[99 — Kitchen sink](../99-kitchen-sink/).
