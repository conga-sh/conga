---
title: How to inspect value provenance
description: Let users see where each flag value came from, with --why, as a table or JSON, and print the same report from your own code.
---

A flag can get its value from the command line, an environment variable, a config file, or its default. When a user can't tell why a setting has the value it has, `--why` answers. It prints every flag's value and the source that set it, then exits without running the command.

This guide builds a small `server` program that reads a `server.yaml` file and `SERVER_*` variables.

## Add `--why` to the program

Call `EnableWhy()` on the builder:

```go title="main.go" {30}
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/yaml"
)

type Server struct {
	Host   conga.OptionalFlag[string]            `default:"127.0.0.1" help:"Listen address"`
	Port   conga.OptionalFlag[int]               `default:"8080" help:"Listen port" short:"p"`
	Token  conga.OptionalFlag[string]            `help:"API token"`
	Labels conga.OptionalFlag[map[string]string] `help:"Labels attached to metrics"`
}

func (s *Server) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "listening on %s:%d\n", s.Host.Get(), s.Port.Get())

	return nil
}

func main() {
	app := conga.New[Server]().
		Name("server").
		EnvPrefix("SERVER").
		ConfigOptions(conga.ConfigPaths(".")).
		RegisterDecoder(".yaml", yaml.Decode).
		EnableWhy()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

The working directory holds this config file:

```yaml title="server.yaml"
host: 0.0.0.0
labels:
  region: eu-west-1
  team: payments
```

A user who sets the token in the environment and the port on the command line, and adds `--why`, sees [all four sources](/reference/builtins/#the---why-report):

```sh
$ SERVER_TOKEN=hunter2 server --why --port 9003
Value provenance for "server":

  FLAG       VALUE      SOURCE                 ENV             CONFIG PATH
  --host     ██████     config (server.yaml)   SERVER_HOST     host
  --port     ██████     cli                    SERVER_PORT     port
  --token    ██████     env                    SERVER_TOKEN    token
  --labels   (2 keys)   config (server.yaml)   SERVER_LABELS   labels
             ├─ region=██████   config (server.yaml)
             └─ team=██████     config (server.yaml)

Config decoders: .yaml

Config paths:
  .   loaded: server.yaml

values are redacted — tag fields safe:"true" to display
```

The `SOURCE` column names where each value came from, and a config source names the file. The `ENV` and `CONFIG PATH` columns show where else the user could set the flag. `Config paths` lists each place the program searched and what it found. The command itself doesn't run, so `listening on …` never prints.

## Show values that are safe to print

Users paste `--why` output into bug reports, and a flag can hold a password or a token, so the report hides each value a user set on the command line, in the environment, or in a config file. That is why every value above shows as `██████`. Tag each flag whose value is safe to share with `safe:"true"`:

```go title="main.go" {2,3,5}
type Server struct {
	Host   conga.OptionalFlag[string]            `default:"127.0.0.1" help:"Listen address" safe:"true"`
	Port   conga.OptionalFlag[int]               `default:"8080" help:"Listen port" safe:"true" short:"p"`
	// [!callout /Token/] Still shown as ██████
	Token  conga.OptionalFlag[string]            `help:"API token"`
	Labels conga.OptionalFlag[map[string]string] `help:"Labels attached to metrics" safe:"true"`
}
```

The same run now shows every value except the token:

```sh
$ SERVER_TOKEN=hunter2 server --why --port 9003
Value provenance for "server":

  FLAG       VALUE      SOURCE                 ENV             CONFIG PATH
  --host     0.0.0.0    config (server.yaml)   SERVER_HOST     host
  --port     9003       cli                    SERVER_PORT     port
  --token    ██████     env                    SERVER_TOKEN    token
  --labels   (2 keys)   config (server.yaml)   SERVER_LABELS   labels
             ├─ region=eu-west-1   config (server.yaml)
             └─ team=payments      config (server.yaml)

Config decoders: .yaml

Config paths:
  .   loaded: server.yaml

values are redacted — tag fields safe:"true" to display
```

Some values print without the tag:

- A value that came from the flag's `default` tag prints, because it is already in `--help`. Keep secrets out of `default`, because a token given there appears in the report.
- Boolean flags and `conga.Counter` flags print, because they can't hold a secret. Tag one `safe:"false"` to hide it.
- The paths given to a `conga.ConfigFile` flag print.

Positional arguments don't appear in the report.

## Trace each key of a map flag

A map flag merges key by key. A key from a higher source replaces that key only, and keys from lower sources stay. The report lists each key under the flag with the source that set it. Here the user adds `team` in the environment and `tier` on the command line:

```sh {8-11}
$ SERVER_LABELS=team=core server --why --labels tier=gold
Value provenance for "server":

  FLAG       VALUE      SOURCE                 ENV             CONFIG PATH
  --host     0.0.0.0    config (server.yaml)   SERVER_HOST     host
  --port     8080       default                SERVER_PORT     port
  --token               unset                  SERVER_TOKEN    token
  --labels   (3 keys)   cli                    SERVER_LABELS   labels
             ├─ region=eu-west-1   config (server.yaml)
             ├─ team=core          env
             └─ tier=gold          cli

Config decoders: .yaml

Config paths:
  .   loaded: server.yaml
```

`team=core` from the environment replaced `team=payments` from the file, and `region` kept its value from the file. The flag row shows the highest source that contributed a key.

## Offer the report as JSON

For scripts and CI checks, pass `conga.WhyOptionJSON()` to `EnableWhy`:

```go title="main.go"
EnableWhy(conga.WhyOptionJSON())
```

Users then choose `--why=json`, which prints one JSON document on one line. Its `flags` array holds the same rows, with the config file in `detail` and a redacted value as `null`:

```sh expandable={20}
$ SERVER_TOKEN=hunter2 server --why=json --port 9003 | jq '.flags'
[
  {
    "name": "host",
    "type": "string",
    "value": "0.0.0.0",
    "source": "config",
    "detail": "server.yaml",
    "env": "SERVER_HOST",
    "configPath": "host"
  },
  {
    "name": "port",
    "type": "int",
    "value": 9003,
    "source": "cli",
    "env": "SERVER_PORT",
    "configPath": "port"
  },
  {
    "name": "token",
    "type": "string",
    "redacted": true,
    "value": null,
    "source": "env",
    "env": "SERVER_TOKEN",
    "configPath": "token"
  },
  {
    "name": "labels",
    "type": "map",
    "source": "config",
    "detail": "server.yaml",
    "env": "SERVER_LABELS",
    "configPath": "labels",
    "entries": [
      {
        "key": "region",
        "value": "eu-west-1",
        "source": "config",
        "detail": "server.yaml"
      },
      {
        "key": "team",
        "value": "payments",
        "source": "config",
        "detail": "server.yaml"
      }
    ]
  }
]
```

The rest of the document records the command, the working directory, a timestamp, and the config search. [The `--why` report](/reference/builtins/#the---why-report) lists every property. `--why` and `--why=text` still print the table, and `--help` now lists the flag as `--why[=FORMAT]`.

## Print the report from your own code

`session.Why()` returns the same table as a string, so a command can print it on its own terms. This step adds a `--debug` flag that writes the report to stderr before the server starts:

```go title="main.go" ins={6,10-12}
type Server struct {
	Host   conga.OptionalFlag[string]            `default:"127.0.0.1" help:"Listen address" safe:"true"`
	Port   conga.OptionalFlag[int]               `default:"8080" help:"Listen port" safe:"true" short:"p"`
	Token  conga.OptionalFlag[string]            `help:"API token"`
	Labels conga.OptionalFlag[map[string]string] `help:"Labels attached to metrics" safe:"true"`
	Debug  conga.OptionalFlag[bool]              `help:"Print where each setting came from" safe:"true"`
}

func (s *Server) Run(ctx context.Context, session *conga.Session) error {
	if s.Debug.Get() {
		fmt.Fprintln(session.Stderr(), session.Why())
	}

	fmt.Fprintf(session.Stdout(), "listening on %s:%d\n", s.Host.Get(), s.Port.Get())

	return nil
}
```

Unlike `--why`, the command runs after the report:

```sh
$ server --debug
Value provenance for "server":

  FLAG       VALUE      SOURCE                 ENV             CONFIG PATH
  --host     0.0.0.0    config (server.yaml)   SERVER_HOST     host
  --port     8080       default                SERVER_PORT     port
  --token               unset                  SERVER_TOKEN    token
  --labels   (2 keys)   config (server.yaml)   SERVER_LABELS   labels
             ├─ region=eu-west-1   config (server.yaml)
             └─ team=payments      config (server.yaml)
  --debug    true       cli                    SERVER_DEBUG    debug

Config decoders: .yaml

Config paths:
  .   loaded: server.yaml
listening on 0.0.0.0:8080
```

`session.WhyJSON()` returns the JSON document. Both return an empty string when the program doesn't call `EnableWhy`, and `WhyJSON` also needs `conga.WhyOptionJSON()`.

## Know what users see

| What the user runs | Result |
|---|---|
| `--why` on a program without `EnableWhy()` | Usage error, exit 2: `unknown flag: --why` |
| `--why=json` without `conga.WhyOptionJSON()` | Usage error, exit 2, naming the option to enable |
| `--why=xml` | Usage error, exit 2: `invalid value "xml" (must be one of: text, json) for --why` |
| `--why` while a config file holds a bad value | The same error a normal run gives, exit 2, and no report |
| `--why` without a required flag | The report, with the flag `unset`, exit 0 |

A bad config value stops the report, so the user fixes the value first:

```text no-copy error={1}
server: error: invalid integer value "eighty" for flag --port (from config key "port")
```

The report also lists [feature toggles](/how-to/gate-behind-features/#check-which-source-set-each-feature) and the flags of parent commands. Color and markers appear only on a terminal.

## Where to go next

- [The `--why` report](/reference/builtins/#the---why-report): every section of the table and every JSON property.
- [About value precedence](/explanation/value-precedence/): why the command line beats the environment, and the environment beats config.
- [How to manage config files](/how-to/manage-config-files/): choose where the program looks for config files.
- [How to bind environment variables](/how-to/bind-environment-variables/): name the variables in the `ENV` column.
