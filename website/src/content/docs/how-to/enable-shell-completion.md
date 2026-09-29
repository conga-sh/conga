---
title: How to enable shell completion
description: Generate and install shell completion scripts for bash, zsh, fish, powershell, elvish, and nushell.
---

CONGA generates shell completion scripts for bash, zsh, fish, powershell, elvish, and nushell. Completion support is opt-in at link time: the driver hooks and internal completion handlers link into the binary only when `app.EnableCompletion()` is called.

## Enable completion on the builder

Call `EnableCompletion()` when constructing the application:

```go title="main.go"
func main() {
	app := conga.New[CLI]().
		Name("cloudctl").
		EnableCompletion()

	app.FatalIfError(app.Run())
}
```

Calling `app.EnableCompletion()` installs the hidden `__complete` handler used by shell completion scripts and provides completion endpoints:

- **Flat applications (no subcommands)**: Automatically gain a `--completion <shell>` flag.
- **Hierarchical applications (with subcommands)**: Can expose a dedicated `completion` command by declaring a `conga.CompletionCommand` field on the root command struct.

## Expose the completion subcommand

For applications with subcommands, declare a `conga.CompletionCommand` field on the root struct:

```go title="main.go"
type CLI struct {
	Completion conga.CompletionCommand `summary:"Generate shell completion scripts"`
	Deploy     conga.Command[DeployCmd] `summary:"Deploy a service"`
}
```

When declared, running `cloudctl completion <shell>` prints the shell script directly to stdout. Running `cloudctl completion` without arguments displays help and per-shell installation instructions.

## Install completion scripts per shell

Install the generated completion scripts into your active shell configuration:

### Bash

```sh
# Load in current session
$ source <(cloudctl completion bash)

# Persist for future sessions
$ cloudctl completion bash > /etc/bash_completion.d/cloudctl
```

### Zsh

```sh
# Ensure completion directory is in $fpath and reload
$ cloudctl completion zsh > "${fpath[1]}/_cloudctl"
```

### Fish

```sh
# Install to user completions directory
$ cloudctl completion fish > ~/.config/fish/completions/cloudctl.fish
```

### PowerShell

```sh
# Add to current profile
$ cloudctl completion powershell | Out-String | Invoke-Expression
```

### Elvish

```sh
# Evaluate completion definition
$ eval (cloudctl completion elvish | slurp)
```

### Nushell

```sh
# Save and register completion script
$ cloudctl completion nushell | save -f ~/.config/nushell/completions/cloudctl.nu
```

## Understand completion runtime behaviour

When a user presses the tab key, the installed shell function invokes the application binary using the hidden `__complete` sub-protocol:

```sh
$ cloudctl __complete bash "cloudctl" "dep"
```

During completion execution:
1. CONGA tokenises the partial command line and identifies the target command, flag, or argument.
2. The engine evaluates candidate predictors, matching flags, subcommands, and allowed enum values.
3. Candidate directives are written directly to stdout in the shell's native completion format.
4. The process exits with status 0.
5. The application's `Run` method is never executed, and the single-execution slot is not consumed.

## What completes automatically

When completion is enabled, the following elements complete without custom code:

- **Flag names**: Long flags (`--replicas`), short runes (`-r`), aliases, and negatable forms (`--no-cache`). Deprecated flags are omitted from suggestions.
- **Subcommands**: Command names, aliases, and command groups with typo-correction suggestions.
- **Enum values**: Flags and arguments restricted by `enum:"..."` tags complete their allowed choices.
- **Domain types**: Built-in types provide context-aware predictions. `conga.File` completes filesystem files, `conga.Dir` completes directories, and `conga.ByteSize` completes unit suffixes.

## Where to go next

- [How to wire custom completion](/how-to/wire-custom-completion/) — implement dynamic completion candidates with `package predict`.
- [About the shell completion architecture](/explanation/completion-architecture/) — driver dispatch, wire framing, and zero-allocation completion.
- [Package predict reference](/reference/predict/) — combinator and constructor catalog for candidate generation.
