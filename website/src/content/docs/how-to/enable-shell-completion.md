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

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
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

type DeployCmd struct{}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	return nil
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

## Customise completion behaviour

Descriptions and alias candidates are shown by default. Two toggles suppress them, and the zsh snippet derives the same toggles from your zsh configuration so existing setups keep working.

### Zsh styles

The generated zsh script honours these styles at completion time:

| Style | Effect |
|---|---|
| `verbose` | Set to `false` (or `no`/`off`/`0`) to hide candidate descriptions. This is the standard zsh style; descriptions are shown otherwise |
| `show-aliases` | Set to `false` (or `no`/`off`/`0`) to hide alias candidates (subcommand aliases, flag aliases, and app-provided aliases such as taskfile task aliases). This is the convention established by go-task |
| `list-separator` | The separator rendered between a match and its description. Engine colours are generated with the configured separator, so both columns keep their colours |

```zsh title="~/.zshrc"
zstyle ':completion:*' verbose no             # hide descriptions
zstyle ':completion:*:*:cloudctl:*' show-aliases no  # hide aliases for cloudctl only
zstyle ':completion:*' list-separator ' · '   # colour both columns around a custom separator
```

Your own `group-name`, `list-colors`, and `list-separator` styles are respected: the engine only installs its colour defaults when you have not configured `list-colors`, and its colour patterns use your configured `list-separator`. An empty `list-colors` value, such as Oh-My-Zsh's default, counts as unconfigured and keeps engine colours.

If you migrate an application whose completion honoured these styles before, your existing zstyle lines keep working unchanged.

### Environment toggles for every shell

All six framings honour the same toggles through environment variables. Set them in your shell configuration; they apply to every CONGA-completed command.

| Variable | Effect |
|---|---|
| `CONGA_COMPLETION_DESCRIPTIONS=0` | Hide candidate descriptions (`false` also disables; unset or `1` shows them) |
| `CONGA_COMPLETION_ALIASES=0` | Hide alias candidates (`false` also disables; unset or `1` shows them) |

```sh title="bash"
export CONGA_COMPLETION_DESCRIPTIONS=0
export CONGA_COMPLETION_ALIASES=0
```

```fish title="config.fish"
set -gx CONGA_COMPLETION_DESCRIPTIONS 0
set -gx CONGA_COMPLETION_ALIASES 0
```

```powershell title="$PROFILE"
$env:CONGA_COMPLETION_DESCRIPTIONS = '0'
$env:CONGA_COMPLETION_ALIASES = '0'
```

```nu title="env.nu"
$env.CONGA_COMPLETION_DESCRIPTIONS = "0"
$env.CONGA_COMPLETION_ALIASES = "0"
```

```elvish title="rc.elv"
set-env CONGA_COMPLETION_DESCRIPTIONS 0
set-env CONGA_COMPLETION_ALIASES 0
```

In bash, descriptions only appear in the successive-tab listing, so `CONGA_COMPLETION_DESCRIPTIONS=0` affects that listing only.

If your application completes domain aliases (for example taskfile task aliases) through a custom predictor, mark those candidates with `predict.Item.WithAlias()` so the aliases toggle suppresses them too. Subcommand and flag aliases declared with the `aliases` tag are marked automatically.

## Completion at runtime

The hidden `__complete` driver, the shell-native output, and the elements that complete without custom code are documented in [Built-ins and deprecation](/reference/builtins/#completion-surface). The completion command itself supports tab-completion: typing `cloudctl completion <TAB>` suggests all supported shell names with descriptions.

## Where to go next

- [How to wire custom completion](/how-to/wire-custom-completion/) — implement dynamic completion candidates with `package predict`.
- [Package predict reference](/reference/predict/) — combinator and constructor catalog for candidate generation.
- [Built-ins and deprecation](/reference/builtins/#completion-surface) — flat flag, subcommand, and hidden driver surfaces.
