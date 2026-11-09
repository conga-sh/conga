---
title: How to customize help screens
description: Add descriptions, examples, and footers, mark deprecated commands, group flags into sections, write constraints into help text, and print help from your own command.
---

CONGA builds every help screen from your command structs. This guide adds the text around them, marks a deprecated command, assigns short flags to the built-ins, groups flags and commands into sections, and controls the hints that follow each flag.

This guide builds a small `cloudctl` program with `deploy` and `rollout` subcommands. The [built-ins example](https://github.com/conga-sh/conga/tree/main/_examples/10-builtins-help-version) is a fuller program with custom help, usage, and version commands. [Built-ins and deprecation](/reference/builtins/#help) lists every section of a help screen and where its content comes from.

## Describe the application

Set the root command's description, examples, and footer on the builder:

```go title="main.go" hidden={1-8}
package main

import (
	"context"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Token   conga.OptionalFlag[string] `help:"API access token"`
	Profile conga.OptionalFlag[string] `help:"Credentials profile"`
	Verbose conga.OptionalFlag[bool]   `help:"Print each step" short:"v"`

	Deploy  conga.Command[DeployCmd]  `summary:"Deploy workloads to clusters"`
	Rollout conga.Command[RolloutCmd] `summary:"Roll out a release"`
}

func main() {
	app := conga.New[CLI]().
		Name("cloudctl").
		Description("Manage cloud infrastructure and deployments.").
		Examples("cloudctl deploy --replicas 5\ncloudctl rollout v1.4").
		HelpFooter("Documentation: https://example.com/docs")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

The description opens the screen, the examples follow the usage line, and the footer closes it:

```sh
$ cloudctl --help
Manage cloud infrastructure and deployments.

Usage:
  cloudctl <command> [flags]

Examples:
  cloudctl deploy --replicas 5
  cloudctl rollout v1.4

Commands:
  deploy   Deploy workloads to clusters
  rollout  Roll out a release

Flags:
      --token=STRING    API access token
      --profile=STRING  Credentials profile
  -v, --verbose         Print each step
      --help            Show help
      --version         Show version

Documentation: https://example.com/docs
```

The footer appears on every help screen, including each subcommand's. When the root declares [feature toggles](/how-to/gate-behind-features/), the root screen also lists them under `Features:`.

## Describe a subcommand

A subcommand's `summary` tag is its line in the parent's `Commands:` list. Its own screen opens with the same text:

```sh
$ cloudctl deploy --help
Deploy workloads to clusters

Usage:
  cloudctl deploy [flags]
```

For a longer description and examples, implement `Description()` and `Examples()` on the subcommand struct. Unlike the `summary` tag, they're Go code, so they can build the text when the program starts: `Examples()` can check `runtime.GOOS` and show Windows paths on Windows, for instance. CONGA calls them once, before it reads flags, so the text can't use the command's own flag values:

```go title="deploy.go"
type DeployCmd struct {
	Replicas conga.OptionalFlag[int]    `default:"3" help:"Number of replicas"`
	Port     conga.OptionalFlag[int]    `default:"8080" help:"Listen port" range:"1..65535"`
	Region   conga.OptionalFlag[string] `enum:"us-east-1,us-west-1,us-west-2,eu-west-1,eu-central-1" help:"Target region"`
}

func (d *DeployCmd) Description() string {
	return "Deploy application workloads to target clusters.\n\nEach replica runs on its own node."
}

func (d *DeployCmd) Examples() string {
	return "cloudctl deploy --replicas 5\ncloudctl deploy --region eu-west-1"
}
```

The description replaces the summary on the subcommand's screen, and the parent keeps showing the summary. Line breaks in the description are kept:

```sh
$ cloudctl deploy --help
Deploy application workloads to target clusters.

Each replica runs on its own node.

Usage:
  cloudctl deploy [flags]

Examples:
  cloudctl deploy --replicas 5
  cloudctl deploy --region eu-west-1

Flags:
      --replicas=3     Number of replicas
      --port=8080      Listen port [1..65535]
      --region=STRING  Target region
                       [enum: us-east-1, us-west-1, us-west-2, eu-west-1, eu-central-1]
      --help           Show help

Global Flags:
      --token=STRING    API access token
      --profile=STRING  Credentials profile
  -v, --verbose         Print each step

Documentation: https://example.com/docs
```

:::caution[Root command]
The root command takes its description from `app.Description(...)` and its examples from `app.Examples(...)`. A root struct that implements `Description()`, `Summary()`, or `Examples()` fails at startup.
:::

## Mark a subcommand deprecated

Add a `deprecated` tag with the reason. Here a `push` command gives way to `rollout`:

```go title="cli.go" ins={4}
type CLI struct {
	// ...
	Rollout conga.Command[RolloutCmd] `summary:"Roll out a release"`
	Push    conga.Command[PushCmd]    `deprecated:"use rollout" summary:"Push a release"`
}
```

The parent's list marks the command:

```text no-copy
Commands:
  deploy   Deploy workloads to clusters
  rollout  Roll out a release
  push     Push a release [deprecated: use rollout]
```

The command's own help screen opens with a warning:

```sh
$ cloudctl push --help
WARNING: command 'cloudctl push' is deprecated: use rollout

Push a release
```

The command still runs, and prints the same warning to stderr first:

```sh
$ cloudctl push v1.4
WARNING: command 'cloudctl push' is deprecated: use rollout
```

`app.Deprecated(...)` marks the whole application the same way.

## Give help and version short flags

`--help` and `--version` are long-only, so they leave every short rune to your own flags. To give them short flags, call `HelpShortFlag` and `VersionShortFlag`:

```go title="main.go" ins={3-4}
app := conga.New[CLI]().
	// ...
	HelpShortFlag('h').
	VersionShortFlag('V')
```

`-h` and `-V` then work like the long flags, and help shows them:

```text no-copy
Flags:
      --token=STRING    API access token
      --profile=STRING  Credentials profile
  -v, --verbose         Print each step
  -h, --help            Show help
  -V, --version         Show version
```

`-h` appears on every command and `-V` on the root, as `--version` does. A rune that one of your flags already uses fails at startup; see [Know what users see](#know-what-users-see). When you disable a built-in with `app.DisableHelpFlag()` or `app.DisableVersionFlag()`, its rune stays free for your own flags.

## Group flags and commands into sections

Give flags or commands a `section` tag. Each section gets its own heading, with the tag's text as written:

```go title="cli.go" {2-3,6-8}
type CLI struct {
	Token   conga.OptionalFlag[string] `help:"API access token" section:"Authentication Flags"`
	Profile conga.OptionalFlag[string] `help:"Credentials profile" section:"Authentication Flags"`
	Verbose conga.OptionalFlag[bool]   `help:"Print each step" short:"v"`

	Deploy  conga.Command[DeployCmd]  `section:"Workload Commands" summary:"Deploy workloads to clusters"`
	Rollout conga.Command[RolloutCmd] `section:"Workload Commands" summary:"Roll out a release"`
	Push    conga.Command[PushCmd]    `deprecated:"use rollout" section:"Workload Commands" summary:"Push a release"`
}
```

Untagged flags stay under `Flags:`, and each section follows in the order its first field is declared:

```sh
$ cloudctl --help
Manage cloud infrastructure and deployments.

Usage:
  cloudctl <command> [flags]

Examples:
  cloudctl deploy --replicas 5
  cloudctl rollout v1.4

Workload Commands:
  deploy   Deploy workloads to clusters
  rollout  Roll out a release
  push     Push a release [deprecated: use rollout]

Flags:
  -v, --verbose  Print each step
  -h, --help     Show help
  -V, --version  Show version

Authentication Flags:
      --token=STRING    API access token
      --profile=STRING  Credentials profile

Documentation: https://example.com/docs
```

## Write constraints into the help text

CONGA appends each flag's constraints to its help: `--port` above shows `8080` in its value slot and `[1..65535]` after the text. To phrase them yourself, put help variables in the `help` tag:

```go title="deploy.go" {2}
type DeployCmd struct {
	Port conga.OptionalFlag[int] `default:"8080" help:"Listen port (default {{default}}, allowed {{range}})" range:"1..65535"`
	// ...
}
```

Each variable you use replaces the matching hint, and the rest stay:

```text no-copy
Flags:
      --replicas=3     Number of replicas
      --port=INT       Listen port (default 8080, allowed 1..65535)
      --region=STRING  Target region
                       [enum: us-east-1, us-west-1, us-west-2, eu-west-1, eu-central-1]
  -h, --help           Show help
```

| Variable | Replaces |
|---|---|
| `{{default}}` | The `[default: …]` hint, and the default in the value slot (`--port=8080` becomes `--port=INT`) |
| `{{range}}` | The `[min..max]` hint |
| `{{enum}}` | The `[enum: …]` hint |
| `{{len}}`, `{{items}}` | The `[len: …]` and `[items: …]` hints |
| `{{env}}` | The `[env: …]` hint |

A variable needs the matching tag: `{{range}}` on a field without `range` fails at startup. Write `{{{{` for a literal `{{`. [Help variables](/reference/struct-tags/#help-variables) lists where each variable is available.

## Hide a long list of allowed values

A long `enum` crowds the screen, as `--region` does above. Add `enumhint:"-"` to leave the list out of help:

```go title="deploy.go" {2}
type DeployCmd struct {
	Region conga.OptionalFlag[string] `enum:"us-east-1,us-west-1,us-west-2,eu-west-1,eu-central-1" enumhint:"-" help:"Target region"`
	// ...
}
```

```text no-copy
Flags:
      --replicas=3     Number of replicas
      --port=INT       Listen port (default 8080, allowed 1..65535)
      --region=STRING  Target region
  -h, --help           Show help
```

The flag still accepts only the listed values, and shell completion still offers them:

```sh error={2}
$ cloudctl deploy --region mars
cloudctl: error: invalid value "mars" (must be one of: us-east-1, us-west-1, us-west-2, eu-west-1, eu-central-1) for flag --region
```

`enumhint` accepts only `-`. Feature toggles hide the list for you, because the `Features:` section already shows every toggle.

## Check help at a narrow width

Help wraps to the terminal's width. Set `COLUMNS` to see a screen as a narrow terminal shows it:

```sh
$ COLUMNS=60 cloudctl deploy --help
Deploy application workloads to target clusters.

Each replica runs on its own node.

Usage:
  cloudctl deploy [flags]

Examples:
  cloudctl deploy --replicas 5
  cloudctl deploy --region eu-west-1

Flags:
      --replicas=3     Number of replicas
      --port=INT       Listen port (default 8080, allowed
                       1..65535)
      --region=STRING  Target region
  -h, --help           Show help

Global Flags:
  -v, --verbose  Print each step

Authentication Flags:
      --token=STRING    API access token
      --profile=STRING  Credentials profile

Documentation: https://example.com/docs
```

Long help text folds at spaces, under the help column. [Wrap width](/reference/builtins/#wrap-width) gives the full rules, including the width used when output is piped.

## Know what users see

Each of these mistakes stops the program at startup, before it reads any input, with exit status 1:

| Mistake | Message |
|---|---|
| A short flag uses the rune given to `HelpShortFlag` or `VersionShortFlag` | `built-in --help short flag -h collides with --verbose; assign a different rune to one of them` |
| A help variable CONGA doesn't define, such as `{{min}}` | `tag "help" references unknown variable {{min}}; supported variables are …` |
| A help variable without its matching tag | `tag "help" references {{range}}, but the field provides no value for that variable` |
| `enumhint:"-"` on a field without `enum` | `tag "enumhint" suppresses the enum hint, but the field declares no enum values` |

Each full message names the struct and field:

```text no-copy error={1-2}
cloudctl: error: struct CLI field Verbose: schema error: duplicate flag: built-in --help short flag -h collides with --verbose; assign a different rune to one of them
cloudctl: error: struct DeployCmd field Port: schema error: invalid struct tag: tag "help" references unknown variable {{min}}; supported variables are {{range}}, {{len}}, {{items}}, {{enum}}, {{default}}, {{env}}
```

[`conga-lint`](/reference/conga-lint/) reports the same mistakes before you build.

## Print help from your own command

`session.Help()` returns the active command's help screen, and `session.Usage()` its usage line, as strings. Neither writes anything, so a command can page, reformat, or route them. This `guide` command prints its own usage string, quoted to show its exact content:

```go title="guide.go"
type GuideCmd struct{}

func (g *GuideCmd) Run(_ context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "%q\n", session.Usage())

	return nil
}
```

```sh
$ cloudctl guide
"Usage:\n  cloudctl guide [flags]"
```

The usage string includes its `Usage:` heading, as on the help screen.

## Where to go next

- [Built-ins and deprecation](/reference/builtins/#help): every help section, the usage line, and wrapping rules.
- [Help variables](/reference/struct-tags/#help-variables): the full variable catalogue.
- [How to gate features](/how-to/gate-behind-features/): the `Features:` section.
- [Built-ins example](https://github.com/conga-sh/conga/tree/main/_examples/10-builtins-help-version): custom help, usage, and version commands.
