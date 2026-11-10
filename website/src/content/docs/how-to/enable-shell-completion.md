---
title: How to enable shell completion
description: Enable shell completion for bash, zsh, fish, PowerShell, Elvish, and Nushell, customize the completion command, and install the shell scripts.
---

This guide adds completion to a `cloudctl` CLI with `deploy` and `status` subcommands, then to a flat `minify` CLI. The interactive listings use zsh; other shells render the same candidates in their native layout. To compute candidates in Go, see [How to compute and shape completion candidates](/how-to/compute-completion-candidates/).

## Enable completion on the builder

Call `EnableCompletion()` on the application builder:

```go title="main.go" hidden={1-9} {37}
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a service"`
	Status conga.Command[StatusCmd] `summary:"Show service status"`
}

type DeployCmd struct {
	Env     conga.OptionalFlag[string] `default:"staging" enum:"staging,prod" help:"Target environment"`
	Service conga.RequiredArg[string]  `help:"Service to deploy"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying %s to %s\n", d.Service.Get(), d.Env.Get())

	return nil
}

type StatusCmd struct{}

func (s *StatusCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "all services healthy")

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("cloudctl").
		EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

Because `CLI` declares subcommands, CONGA mounts a `completion <shell>` subcommand, listed last under `Commands:`:

```sh {8}
$ cloudctl --help
Usage:
  cloudctl <command>

Commands:
  deploy      Deploy a service
  status      Show service status
  completion  Generate shell completion scripts

Flags:
      --help     Show help
      --version  Show version
```

A root command without subcommands gets a `--completion <shell>` flag instead; see [Enable completion in a flat CLI](#enable-completion-in-a-flat-cli).

## Customize the completion subcommand

Declare a `conga.CompletionCommand` field on the root struct to customize the subcommand with `name`, `aliases`, `section`, `summary`, or `hidden` tags:

```go title="main.go" ins={5}
type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a service"`
	Status conga.Command[StatusCmd] `summary:"Show service status"`

	Completion conga.CompletionCommand `aliases:"comp" section:"Setup" summary:"Print the shell completion script"`
}
```

```sh {9-10}
$ cloudctl --help
Usage:
  cloudctl <command>

Commands:
  deploy  Deploy a service
  status  Show service status

Setup:
  completion  Print the shell completion script

Flags:
      --help     Show help
      --version  Show version
```

`cloudctl comp zsh` now resolves to the `completion` subcommand. Setting `hidden:"true"` keeps the command executable while omitting it from `--help` and completion candidates.

## Enable completion in a flat CLI

A root command without `conga.Command[T]` fields gets a `--completion <shell>` flag in place of the subcommand. This `minify` program takes one positional argument:

```go title="main.go" hidden={1-8} {25}
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Level  conga.OptionalFlag[int]    `default:"1" help:"Minification level" range:"1..9" short:"l"`
	Output conga.OptionalFlag[string] `help:"Output file path" short:"o"`
	Input  conga.RequiredArg[string]  `help:"Input file to minify"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "minifying %s at level %d\n", c.Input.Get(), c.Level.Get())

	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("minify").
		EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```sh {11}
$ minify --help
Usage:
  minify [flags] <input>

Arguments:
  <input>  Input file to minify

Flags:
  -l, --level=1           Minification level [1..9]
  -o, --output=STRING     Output file path
      --completion=SHELL  Generate shell completion script
      --help              Show help
      --version           Show version
```

The flag takes the same shell names and prints the same setup screens, in flag syntax. Users of a flat CLI type `--completion <shell>` wherever the install commands below use `completion <shell>`:

```sh
$ minify --completion
Generate shell completion scripts

Usage:
  minify --completion [shell]

Supported shells:
  bash        Bourne Again SHell
  elvish      Elvish shell
  fish        Friendly interactive shell
  nushell     Nushell shell
  powershell  PowerShell / pwsh
  zsh         Z shell

Enable completions for your current zsh shell now:

	source <(minify --completion zsh)

For persistent setups, run 'minify --completion zsh --help'.

Flags:
      --help  Show help
```

To offer a `completion` subcommand instead, declare a `conga.CompletionCommand` field. CONGA drops the `--completion` flag, and the field takes the tags from the previous section:

```go title="main.go" ins={6}
type CLI struct {
	Level  conga.OptionalFlag[int]    `default:"1" help:"Minification level" range:"1..9" short:"l"`
	Output conga.OptionalFlag[string] `help:"Output file path" short:"o"`
	Input  conga.RequiredArg[string]  `help:"Input file to minify"`

	Completion conga.CompletionCommand `summary:"Generate shell completion scripts"`
}
```

```sh {5-6}
$ minify --help
Usage:
  minify [flags] <input>

Commands:
  completion  Generate shell completion scripts

Arguments:
  <input>  Input file to minify

Flags:
  -l, --level=1        Minification level [1..9]
  -o, --output=STRING  Output file path
      --help           Show help
      --version        Show version
```

The command name now shadows a positional value spelled `completion`: `minify completion` prints the setup screen, and `minify -- completion` or `minify ./completion` minifies a file of that name. The [flat completion flag](https://github.com/conga-sh/conga/tree/main/_examples/24-flat-completion-flag) and [flat completion subcommand](https://github.com/conga-sh/conga/tree/main/_examples/25-flat-completion-subcommand) examples show both forms.

## Load or install the completion script

Without a shell name, `cloudctl completion` lists the shells and the setup command for the detected shell:

```sh
$ cloudctl completion
Generate shell completion scripts

Usage:
  cloudctl completion [<shell>]

Arguments:
  [<shell>]  Target shell name

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

`cloudctl completion <shell> --help` prints the setup steps for that shell, adjusted to the environment: with `HOMEBREW_PREFIX` set, the zsh steps write to `$HOMEBREW_PREFIX/share/zsh/site-functions`, and on macOS the bash steps warn when the running Bash is older than 4.4.

:::code-tabs{sync="shell"}
```sh label="Bash"
# Current session
source <(cloudctl completion bash)

# Every new session
echo 'source <(cloudctl completion bash)' >> ~/.bashrc

# Or, with bash-completion installed, write a lazy-loaded completion file
mkdir -p ~/.local/share/bash-completion/completions
cloudctl completion bash > ~/.local/share/bash-completion/completions/cloudctl
```
```sh label="Zsh"
# Ensure compinit is enabled
echo 'autoload -U compinit; compinit' >> ~/.zshrc

# Current session
source <(cloudctl completion zsh)

# Every new session
echo 'source <(cloudctl completion zsh)' >> ~/.zshrc

# Or write a completion file into fpath (add fpath=(~/.zfunc $fpath) before compinit)
mkdir -p ~/.zfunc
cloudctl completion zsh > ~/.zfunc/_cloudctl

# Or, with Homebrew, write to the site-functions directory already in fpath
cloudctl completion zsh > "$HOMEBREW_PREFIX/share/zsh/site-functions/_cloudctl"
```
```sh label="Fish"
# Current session
cloudctl completion fish | source

# Every new session
mkdir -p ~/.config/fish/completions
cloudctl completion fish > ~/.config/fish/completions/cloudctl.fish
```
```powershell label="PowerShell"
# Current session
cloudctl completion powershell | Out-String | Invoke-Expression

# Every new session
if (!(Test-Path $PROFILE)) { New-Item -ItemType File -Path $PROFILE -Force | Out-Null }
Add-Content -Path $PROFILE -Value 'cloudctl completion powershell | Out-String | Invoke-Expression'

# If script execution is disabled
Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
```
```sh label="Elvish"
# Current session
eval (cloudctl completion elvish | slurp)

# Every new session
mkdir -p ~/.config/elvish
echo 'eval (cloudctl completion elvish | slurp)' >> ~/.config/elvish/rc.elv
```
```nu label="Nushell"
# Every new session (Nushell 0.102+ autoloads this directory)
mkdir ($nu.default-config-dir | path join "autoload")
cloudctl completion nushell | save -f ($nu.default-config-dir | path join "autoload/cloudctl.nu")

# Current session, after saving the file
source ($nu.default-config-dir | path join "autoload/cloudctl.nu")
```
:::

## Verify completion output

With the script loaded, pressing `Tab` after the command name completes subcommands and their summaries:

```text no-copy
$ cloudctl <Tab>
completion  -- Generate shell completion scripts
deploy      -- Deploy a service
status      -- Show service status
```

The `enum` tag on `--env` supplies its values:

```text no-copy
$ cloudctl deploy --env <Tab>
Target environment
prod     staging
```

At a bare space, `deploy` offers its positional argument, and flags appear once the user types `-`:

```text no-copy
$ cloudctl deploy <Tab>
<service>: Service to deploy
```

```text no-copy
$ cloudctl deploy --<Tab>
--env   -- Target environment
--help  -- Show help
```

[Flags at a bare space](/reference/builtins/#flags-at-a-bare-space) lists when flags appear without the `-`.

## Override the environment variable prefix

Generated shell scripts read `<APP>_COMPLETION_DESCRIPTIONS`, `<APP>_COMPLETION_ALIASES`, and `<APP>_COMPLETION_BARE_SPACE_FLAGS`, where `<APP>` defaults to the application name in `UPPER_SNAKE_CASE` (`CLOUDCTL_COMPLETION_*`). Pass `conga.CompletionEnvPrefix` to `EnableCompletion` to override the prefix:

```go title="main.go" {4}
	app := conga.New[CLI]().
		Name("cloudctl").
		EnableCompletion(
			conga.CompletionEnvPrefix("CCTL"),
		)
```

The scripts read these variables from the shell's environment at each `Tab`, so users export them in their shell configuration; an assignment typed in front of the command (`CCTL_COMPLETION_DESCRIPTIONS=0 cloudctl <Tab>`) is part of the line being completed and has no effect. With `CCTL_COMPLETION_DESCRIPTIONS=0` exported (or `zstyle ':completion:*' verbose no` in zsh), candidates lose their descriptions:

```text no-copy
$ export CCTL_COMPLETION_DESCRIPTIONS=0
$ cloudctl <Tab>
completion  deploy      status
```

See [User toggles](/reference/builtins/#user-toggles) for the full variable and `zstyle` reference.

## Know what users see

| Invocation | Result |
|---|---|
| `cloudctl completion myshell` | Usage error listing the supported shells (exit 2) |
| `--completion zsh` on a CLI that has a `completion` subcommand | Usage error, `unknown flag: --completion` (exit 2) |
| `./cloudctl <Tab>` or `/usr/local/bin/cloudctl <Tab>` | Dispatches to the `cloudctl` completer in bash, zsh, fish, PowerShell, and Nushell |
| `./cloudctl <Tab>` in Elvish | Falls back to file completion; Elvish keys completers on the literal command word |
| `cctl <Tab>` using a script generated from `cloudctl` | Falls back to file completion until the user regenerates the script from `cctl` |

```text no-copy error={1}
cloudctl: error: unknown shell "myshell", supported shells: bash, elvish, fish, nushell, powershell, zsh
```

## Where to go next

- [How to compute and shape completion candidates](/how-to/compute-completion-candidates/): compute candidates from loaded data or an API, combine predictors, and complete values in parts.
- [Completion](/reference/builtins/#completion): flag vs. subcommand rules, user toggles, binary names, and file candidates.
- [Package predict](/reference/predict/): predictor constructors, combinators, and interfaces.
