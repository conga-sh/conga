# 19 — Shell completion

Both completion wiring styles in one CLI: self-completing domain types
(`conga.Predictable`, `conga.FileFilterable`, `conga.File`, `conga.Dir`) on
`deploy`, the predictor registry (`app.Predictor` + `predictor:"..."` tags) on
`exec`, and the full `predict` SDK playground (every constructor and combinator)
on `logs`.

## Run it

```sh
go run ./_examples/19-shell-completion completion bash
go run ./_examples/19-shell-completion __complete export deploy --cluster ''
go run ./_examples/19-shell-completion __complete export logs --bucket ''
go run ./_examples/19-shell-completion logs api --list-candidates
```

The complete scenarios, fixtures included, live in
[`testdata/completion.txt`](testdata/completion.txt).

## What to look for

- Domain types carry their own completion; plain types use named predictors.
- `__complete export` speaks the wire protocol (`docs/completion-wire-protocol.md`).
- `--list-candidates` evaluates predictors programmatically (`Invoke`, `Raw`,
  `FilterText`, `Tag`) outside the completion engine.
- `conga.CompletionEnvPrefix("DEPLOY")` names the completion toggles
  `DEPLOY_COMPLETION_*` (for example `DEPLOY_COMPLETION_DESCRIPTIONS=0`) instead
  of deriving them from the application name.

## Test

```sh
go test ./_examples/19-shell-completion
```

Related: [20 — Modular predictors](../20-modular-predictors/),
[24 — Flat completion flag](../24-flat-completion-flag/).
