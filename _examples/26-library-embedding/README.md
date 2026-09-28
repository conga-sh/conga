# 26 — Library embedding

Embedding CONGA as a library inside a host process: the host supplies the
argument vector, injects its own stdin/stdout/stderr streams, calls
`RunContext` with its own context, consumes typed errors programmatically
(`IsUsageError`, `UsageError.CommandPath`, `ErrCoerce`, `ErrParse`), and
intercepts termination with a custom `ExitFunc` instead of `os.Exit`.

## Run it

```sh
go run ./_examples/26-library-embedding migrate-data --priority 7
go run ./_examples/26-library-embedding migrate-data --priority notanumber
go run ./_examples/26-library-embedding --help
```

## What to look for

- All CONGA output lands in the host's buffers; the host flushes them itself.
- The captured exit code reflects the failure class (2 for usage errors).
- `DisableHelpFlag()`/`DisableVersionFlag()` hand the built-in UX to the host.

## Test

```sh
go test ./_examples/26-library-embedding
```

Related: [10 — Builtins help and version](../10-builtins-help-version/),
[Reference: errors and exit codes](https://docs.conga.sh/reference/errors/).
