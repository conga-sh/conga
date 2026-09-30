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

- **Flat applications (no subcommands)**: Automatically gain a `--completion <shell>` flag. The flag appears under `Flags:` in `--help`:
  ```
  Flags:
        --completion=SHELL  Generate shell completion script
        --help              Show help
        --version           Show version
  ```
- **Hierarchical applications (with subcommands)**: Can expose a dedicated `completion` command by declaring a `conga.CompletionCommand` field on the root command struct.

## Expose the completion subcommand

For applications with subcommands, declare a `conga.CompletionCommand` field on the root struct:

```go title="main.go"
type CLI struct {
	Completion conga.CompletionCommand  `summary:"Generate shell completion scripts"`
	Deploy     conga.Command[DeployCmd] `summary:"Deploy a service"`
}
```

When declared, CONGA promotes `completion` into the `Commands:` section of `--help` and suppresses the `--completion` flag:

```sh
cloudctl --help
```

```text no-copy
Usage:
  cloudctl <command> [flags]

Commands:
  completion  Generate shell completion scripts
  deploy      Deploy a service

Flags:
      --help     Show help
      --version  Show version
```

You can customize the completion command field using standard command struct tags:
- `summary:"..."`: Overrides the synopsis displayed in the parent command listing.
- `aliases:"comp,complete"`: Registers alternative command tokens that route to completion.
- `group:"..."`: Organizes the command under a custom section header in `--help`.
- `hidden:"true"`: Keeps completion executable while hiding it from `--help` and shell suggestions.

## Inspect completion help and active shell hints

Running `cloudctl completion` without arguments or with `--help` displays the completion help screen:

```sh
cloudctl completion
```

```text no-copy
Generate shell completion scripts

Usage:
  cloudctl completion <shell>

Arguments:
  <shell>  Target shell name

Supported shells:
  bash        Bourne Again SHell
  elvish      Elvish shell
  fish        Friendly interactive shell
  nushell     Nushell shell
  powershell  PowerShell / pwsh
  zsh         Z shell

Enable completions for your current zsh shell now:

	source <(cloudctl completion zsh)

For persistent setups, run 'cloudctl completion zsh --help'.

Flags:
      --help  Show help
```

Invoking `completion` without a shell argument exits with status 0. The output lists all six supported shells and inspects your environment to detect your running shell. If detected, CONGA prints the one-line command to load completions into your active session immediately.

For flat applications using the flag style, run `cloudctl --completion` or `cloudctl --completion --help` for the equivalent screen with flag syntax (`cloudctl --completion [shell]`).

If an unknown shell name is passed (such as `cloudctl completion myshell`), CONGA returns a validation error with exit status 2.

## View per-shell setup instructions

To view step-by-step installation and configuration instructions for a specific shell, pass `--help` with the shell name:

```sh
cloudctl completion bash --help
```

```text no-copy
Generate the shell completion script for the bash shell.

To load completions in your current shell session, run:

	source <(cloudctl completion bash)

To load completions in every new session, append the following to ~/.bashrc:

	echo 'source <(cloudctl completion bash)' >> ~/.bashrc

You will need to start a new shell for the setup to take effect.

Usage:
  cloudctl completion bash

Flags:
      --help  Show help
```

Per-shell help outputs:
- **Session command**: Loads completion functions into the current terminal session without writing files.
- **Persistent setup**: Shows the config file or directory to update (such as `~/.bashrc`, `~/.zshrc`, fish completion directories, or PowerShell profiles).
- **Environment notes**: Surfaces platform prerequisites, such as requiring Bash 4+ with `bash-completion@2` on macOS when running the default legacy Bash 3.2, or initializing Zsh `compdef` and `fpath`.

Flat applications use flag syntax: `cloudctl --completion <shell> --help` (e.g. `cloudctl --completion zsh --help`).

## Install completion scripts per shell

Run `cloudctl completion <shell>` to generate the completion script for your shell. For flat applications, replace `completion <shell>` with `--completion <shell>`.

### Bash

```sh
# Load in current session
source <(cloudctl completion bash)

# Persist for future sessions
cloudctl completion bash > /etc/bash_completion.d/cloudctl
```

### Zsh

```sh
# Ensure completion directory is in $fpath and reload
cloudctl completion zsh > "${fpath[1]}/_cloudctl"
```

### Fish

```sh
# Install to user completions directory
cloudctl completion fish > ~/.config/fish/completions/cloudctl.fish
```

### PowerShell

```sh
# Add to current profile
cloudctl completion powershell | Out-String | Invoke-Expression
```

### Elvish

```sh
# Evaluate completion definition
eval (cloudctl completion elvish | slurp)
```

### Nushell

```sh
# Save and register completion script
cloudctl completion nushell | save -f ~/.config/nushell/completions/cloudctl.nu
```

## Completion at runtime

The hidden `__complete` driver, the shell-native output, and the elements that complete without custom code are documented in [Built-ins and deprecation](/reference/builtins/#completion-surface). The completion command itself supports tab-completion: typing `cloudctl completion <TAB>` suggests all supported shell names with descriptions.

## Where to go next

- [How to wire custom completion](/how-to/wire-custom-completion/) — implement dynamic completion candidates with `package predict`.
- [Package predict reference](/reference/predict/) — combinator and constructor catalog for candidate generation.
- [Built-ins and deprecation](/reference/builtins/#completion-surface) — flat flag, subcommand, and hidden driver surfaces.
