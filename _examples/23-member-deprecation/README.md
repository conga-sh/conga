# 23 — Member deprecation

Member-level deprecation with the `deprecated:"..."` struct tag: provenance-aware
warnings at runtime, marked help columns, and exclusion of deprecated members
from shell completion.

## Run it

```sh
go run ./_examples/23-member-deprecation --old value
go run ./_examples/23-member-deprecation legacy
go run ./_examples/23-member-deprecation --help
go run ./_examples/23-member-deprecation __complete export --
```

## What to look for

- Using `--old` or the `legacy` subcommand warns and points at the replacement.
- Deprecated members do not appear in completion candidates.
- Non-deprecated members are unaffected.

## Test

```sh
go test ./_examples/23-member-deprecation
```

Related: [22 — App deprecation](../22-app-deprecation/),
[19 — Shell completion](../19-shell-completion/).
