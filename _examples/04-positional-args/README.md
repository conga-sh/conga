# 04 — Positional args

`RequiredArg[T]`, `OptionalArg[T]`, a trailing positional slice, custom
placeholder names, and `GetOr` fallbacks.

## Run it

```sh
go run ./_examples/04-positional-args slack "deploy finished"
go run ./_examples/04-positional-args email "build failed" oncall pager
go run ./_examples/04-positional-args --help
```

## What to look for

- `<channel>` is required; `[message]` and `[recipients...]` are optional.
- The trailing slice greedily collects every remaining token.
- A missing required argument is a usage error (exit code 2).

## Test

```sh
go test ./_examples/04-positional-args
```

Related: [11 — Trailing args forwarding](../11-trailing-args-forwarding/),
[99 — Kitchen sink](../99-kitchen-sink/).
