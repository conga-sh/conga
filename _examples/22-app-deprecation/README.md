# 22 — App deprecation

Whole-command deprecation: `app.Deprecated("...")` emits a warning banner on
every invocation of the binary — root, subcommands, and `--help` — while the
commands keep working normally.

## Run it

```sh
go run ./_examples/22-app-deprecation
go run ./_examples/22-app-deprecation deploy -f
go run ./_examples/22-app-deprecation --help
```

## What to look for

- The banner names the replacement guidance on every invocation.
- Deprecation does not change exit codes or command behavior.

## Test

```sh
go test ./_examples/22-app-deprecation
```

Related: [23 — Member deprecation](../23-member-deprecation/),
[Reference: builtins & deprecation](https://docs.conga.sh/reference/builtins/).
