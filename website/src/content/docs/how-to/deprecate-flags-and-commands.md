---
title: How to deprecate flags and commands
description: Rename a flag or a subcommand while the old name keeps working with a warning, reject the old and new flags together, and deprecate a whole application.
---

A `deprecated` tag marks a flag or a subcommand for removal. The old name keeps working, help marks it with the reason, every run that uses it prints a warning to stderr, and shell completion stops offering it. This guide renames a flag and a subcommand of a `cloudctl` program, then deprecates the whole application. [Flags](/reference/struct-tags/#flags-optionalflagt-requiredflagt-counter) and [Subcommands](/reference/struct-tags/#subcommands-commandt) list where the tag applies.

The program starts with one `deploy` subcommand, whose `--count` flag sets the number of replicas:

```go title="main.go" hidden={1-8}
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a manifest"`
}

type DeployCmd struct {
	Count    conga.OptionalFlag[int]   `default:"3" help:"Number of replicas"`
	Manifest conga.RequiredArg[string] `help:"Manifest file"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "deploying %s with %d replicas\n", d.Manifest.Get(), d.Count.Get())

	return nil
}

func main() {
	app := conga.New[CLI]().Name("cloudctl").EnvPrefix("CLOUDCTL")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```sh
$ cloudctl deploy --count 2 app.yaml
deploying app.yaml with 2 replicas
```

## Rename a flag

Add the new flag, and give the old one a `deprecated` tag with the reason. Move the `default` to the new flag, so the old one holds a value only when a user sets it:

```go title="main.go" ins={2} {3}
type DeployCmd struct {
	Replicas conga.OptionalFlag[int]   `default:"3" help:"Number of replicas"`
	Count    conga.OptionalFlag[int]   `deprecated:"use --replicas" help:"Number of replicas"`
	Manifest conga.RequiredArg[string] `help:"Manifest file"`
}
```

The two flags are separate fields, and a value given to `--count` stays in `Count`. Read the old flag in `Run` when the user set it. `Changed` reports whether a value came from the command line, the environment, or a config file rather than from a default:

```go title="main.go" ins={2-5} {7}
func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	replicas := d.Replicas.Get()
	if d.Count.Changed() {
		replicas = d.Count.Get()
	}

	fmt.Fprintf(session.Stdout(), "deploying %s with %d replicas\n", d.Manifest.Get(), replicas)

	return nil
}
```

Old invocations still work, after a warning on stderr:

```sh
$ cloudctl deploy --count 2 app.yaml
WARNING: flag '--count' is deprecated: use --replicas
deploying app.yaml with 2 replicas
```

Help marks the old flag with the reason:

```sh
$ cloudctl deploy --help
Deploy a manifest

Usage:
  cloudctl deploy [flags] <manifest>

Arguments:
  <manifest>  Manifest file

Flags:
      --replicas=3  Number of replicas [env: CLOUDCTL_REPLICAS]
      --count=INT   Number of replicas [env: CLOUDCTL_COUNT] [deprecated: use --replicas]
      --help        Show help
```

Shell completion offers `--replicas` and leaves out `--count`, even when the user has typed `--co`.

The warning names the source that set the old flag. A user who exports `CLOUDCTL_COUNT` sees the variable:

```sh
$ CLOUDCTL_COUNT=2 cloudctl deploy app.yaml
WARNING: environment variable 'CLOUDCTL_COUNT' is deprecated: use --replicas
deploying app.yaml with 2 replicas
```

A config file that sets `count` under `deploy:` gives `WARNING: config key 'count' is deprecated: use --replicas`. The key appears without its section.

## Reject the old and new flags together

As written, `--replicas 4 --count 2` deploys 2 replicas, because `Run` prefers the old flag whenever it is set. To make that an error, put both flags in one `xor` group:

```go title="main.go" {2-3}
type DeployCmd struct {
	Replicas conga.OptionalFlag[int]   `default:"3" help:"Number of replicas" xor:"replicas"`
	Count    conga.OptionalFlag[int]   `deprecated:"use --replicas" help:"Number of replicas" xor:"replicas"`
	Manifest conga.RequiredArg[string] `help:"Manifest file"`
}
```

```sh error={2}
$ cloudctl deploy --replicas 4 --count 2 app.yaml
cloudctl: error: flags --replicas and --count are mutually exclusive

Usage:
  cloudctl deploy [flags] <manifest>

Run 'cloudctl deploy --help' for more information.
```

The default on `--replicas` doesn't count as given, so `--count` alone still works. A value from the environment or a config file does count. A user who exports `CLOUDCTL_REPLICAS=4` and runs an old script that passes `--count 2` gets the same error.

## Rename a subcommand

Keep the old subcommand as a second field of the same type, so both names run the same code, and give it a `deprecated` tag. Here `push` was the old name for `deploy`:

```go title="main.go" ins={3}
type CLI struct {
	Deploy conga.Command[DeployCmd] `summary:"Deploy a manifest"`
	Push   conga.Command[DeployCmd] `deprecated:"use deploy" summary:"Deploy a manifest (old name)"`
}
```

The parent's help marks the old name:

```sh
$ cloudctl --help
Usage:
  cloudctl <command>

Commands:
  deploy  Deploy a manifest
  push    Deploy a manifest (old name) [deprecated: use deploy]

Flags:
      --help     Show help
      --version  Show version
```

The old command's own help screen opens with the warning:

```sh
$ cloudctl push --help
WARNING: command 'cloudctl push' is deprecated: use deploy

Deploy a manifest (old name)

Usage:
  cloudctl push [flags] <manifest>

Arguments:
  <manifest>  Manifest file

Flags:
      --replicas=3  Number of replicas [env: CLOUDCTL_REPLICAS]
      --count=INT   Number of replicas [env: CLOUDCTL_COUNT] [deprecated: use --replicas]
      --help        Show help
```

The command still runs, and prints one warning for each deprecated name it uses:

```sh
$ cloudctl push --count 2 app.yaml
WARNING: command 'cloudctl push' is deprecated: use deploy
WARNING: flag '--count' is deprecated: use --replicas
deploying app.yaml with 2 replicas
```

Shell completion offers `deploy` and leaves out `push`. [How to declare subcommands](/how-to/declare-subcommands/#group-alias-hide-and-deprecate-subcommands) compares `deprecated` with `hidden`, which leaves a command out of help as well.

## Deprecate the whole application

Call `Deprecated` on the app with the reason. A `Deprecated` method on the root struct fails at startup:

```go title="main.go" {2}
func main() {
	app := conga.New[CLI]().Name("cloudctl").EnvPrefix("CLOUDCTL").Deprecated("install cloudctl2")

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

Every command prints the warning to stderr first:

```sh
$ cloudctl deploy app.yaml
WARNING: cloudctl is deprecated: install cloudctl2
deploying app.yaml with 3 replicas
```

`--help` prints it to stdout, above the help screen:

```sh
$ cloudctl --help
WARNING: cloudctl is deprecated: install cloudctl2

Usage:
  cloudctl <command>

Commands:
  deploy  Deploy a manifest
  push    Deploy a manifest (old name) [deprecated: use deploy]

Flags:
      --help     Show help
      --version  Show version
```

`--version` prints the version line alone.

## Rename a required flag

Suppose `--count` had been a `conga.RequiredFlag[...]`. A `deprecated` tag on it fails at startup, because users can't stop passing a flag the program requires:

```sh error={2}
$ cloudctl deploy app.yaml
cloudctl: error: struct DeployCmd field Count: schema error: invalid struct tag: tag "deprecated" cannot be used on required flags (make the flag optional before deprecating it so callers can migrate without error)
```

Make both flags optional and put them in one `oneof` group, so the user must give exactly one of them. A `oneof` member can't have a `default`:

```go title="main.go" {2-3}
type DeployCmd struct {
	Replicas conga.OptionalFlag[int]   `help:"Number of replicas" oneof:"replicas"`
	Count    conga.OptionalFlag[int]   `deprecated:"use --replicas" help:"Number of replicas" oneof:"replicas"`
	Manifest conga.RequiredArg[string] `help:"Manifest file"`
}
```

Leaving out both is a usage error. The usage line still lists `--count` as one of the choices:

```sh error={2}
$ cloudctl deploy app.yaml
cloudctl: error: missing required flag: one of --replicas, --count

Usage:
  cloudctl deploy (--replicas=INT | --count=INT) <manifest>

Run 'cloudctl deploy --help' for more information.
```

`--count 2` alone deploys 2 replicas after the warning, as before.

A `deprecated` tag on a positional argument fails at startup with `tag "deprecated" is invalid on positional argument`.

## Know what users see

Here the app also registered a YAML config decoder, so `cloudctl` reads `~/.config/cloudctl/config.yaml`.

| What the user does | Result |
|---|---|
| Gives `--count 2` | `WARNING: flag '--count' is deprecated: use --replicas` on stderr, then deploys 2 replicas |
| Sets `CLOUDCTL_COUNT=2` | `WARNING: environment variable 'CLOUDCTL_COUNT' is deprecated: use --replicas`, then deploys 2 replicas |
| Sets `count: 2` under `deploy:` in the config file | `WARNING: config key 'count' is deprecated: use --replicas`, then deploys 2 replicas |
| Gives `--replicas 4 --count 2` | `flags --replicas and --count are mutually exclusive`, then usage, exit 2 |
| Sets `CLOUDCTL_REPLICAS=4` and gives `--count 2` | The same error, exit 2 |
| Gives neither flag | Deploys 3 replicas, no warning |
| Runs `push` | `WARNING: command 'cloudctl push' is deprecated: use deploy` on stderr, then runs |
| Runs `push --help` | The warning above the help screen, on stdout |
| Runs any command of a deprecated app | `WARNING: cloudctl is deprecated: install cloudctl2` on stderr, then runs |
| Runs `--help` on a deprecated app | The warning above the help screen, on stdout |
| Runs `--version` on a deprecated app | The version line, no warning |

## Where to go next

- [Flags](/reference/struct-tags/#flags-optionalflagt-requiredflagt-counter): the `deprecated` and `xor` tags with every other flag tag.
- [Built-ins and deprecation](/reference/builtins/#help): where the deprecation banner sits on a help screen.
- [Example 22](https://github.com/conga-sh/conga/tree/main/_examples/22-app-deprecation) and [example 23](https://github.com/conga-sh/conga/tree/main/_examples/23-member-deprecation): runnable programs that deprecate an app, a flag, and a subcommand.
