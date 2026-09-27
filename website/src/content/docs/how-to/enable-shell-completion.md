---
title: How to enable shell completion
description: Generate completion scripts for bash, zsh, fish, powershell, elvish, and nushell.
---

CONGA ships its own completion driver supporting **bash, zsh, fish, powershell, elvish, and nushell**. Scripts are generated from the same schema that powers parsing and help. How the driver and wire protocol work is covered in [About the shell completion architecture](../explanation/completion-architecture/).

Completion is opt-in: call `app.EnableCompletion()` on the builder. Apps that skip it keep `--completion`, the `completion` subcommand, and the `__complete` handler out of the binary entirely.

```go
app := conga.New[CLI]().
	EnableCompletion().
	Name("mytool")
```

## Generate a script

With completion enabled, flat, single-command CLIs provide a `--completion <shell>` flag:

```sh
$ mytool --completion bash >> ~/.bashrc
```

Apps with subcommands get a hidden `__complete` handler used by the generated shell functions; you can additionally expose an opt-in `completion <shell>` subcommand by declaring a `conga.CompletionCommand` field:

```go
type App struct {
	Completion conga.CompletionCommand `name:"completion" summary:"Generate shell completion script"`
}
```

`mytool completion` on its own renders help, including per-shell install hints.

## What completes automatically

- **Flag names** — long, short, and aliases, including negatable `--no-` forms; deprecated flags are omitted.
- **Subcommand names**, aliases, and groups, with did-you-mean behaviour in the shell editors.
- **Enum-constrained flags and arguments** complete their allowed values.
- **Domain-typed fields** — `conga.File` completes file paths, `conga.Dir` directories, `conga.ByteSize` unit suffixes, and so on. See [Domain Types](../reference/domain-types/).
- **Registered predictors** — flags and arguments tagged `predictor:"name"` complete via the SDK. See [How to wire custom completion](wire-custom-completion/).

## Control completion

| Builder call | Effect |
|--------------|--------|
| `app.EnableCompletion()` | Enables completion support: flag, subcommand, and `__complete` handler. Off by default; apps that skip it do not link the drivers |
| `app.Predictor(name, p)` | Registers a named predictor for `predictor:"name"` tags |

Fields with built-in semantics complete automatically; for your own types and registries, continue with [How to wire custom completion](wire-custom-completion/).
