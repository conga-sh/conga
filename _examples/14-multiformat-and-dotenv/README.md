# 14 — Multiformat and dotenv

`.env` + `.env.local` loading (variable expansion, quotes, non-destruction of
real environment variables) and all four built-in decoders: JSON, TOML, YAML,
and HCL — with strict content validation and typo hints.

## Run it

```sh
go run ./_examples/14-multiformat-and-dotenv
go run ./_examples/14-multiformat-and-dotenv --why
go run ./_examples/14-multiformat-and-dotenv --help
```

The complete scenarios, fixtures included, live in
[`testdata/multiformat.txt`](testdata/multiformat.txt).

## What to look for

- `.env.local` overrides `.env`, and real OS variables are never overwritten.
- A misspelled config key suggests the closest match.
- Config read/parse failures are runtime errors (exit code 1), not usage errors.
- `--why` lists the registered decoders, the winning tier per flag, and redacts
  the credential-bearing `DATABASE_URL` and `API_KEY` fields.

## Test

```sh
go test ./_examples/14-multiformat-and-dotenv
```

Related: [13 — Twelve-factor precedence](../13-twelve-factor-precedence/),
[99 — Kitchen sink](../99-kitchen-sink/).
