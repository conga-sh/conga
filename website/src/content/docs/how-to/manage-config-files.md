---
title: How to manage config files
description: Register config formats, search for files in several places, merge them, let users name their own, and see what users get when a file is wrong.
---

A config file gives flags their values when the user leaves them off the command line and no environment variable sets them. This guide registers a format, searches three places for a file, merges every file found, lets users name their own files, and maps fields to keys.

The running example is the [cascading search paths example](https://github.com/conga-sh/conga/tree/main/_examples/15-cascading-searchpaths). Paths in the output use `/home/alice` as the home directory.

## Register the formats you accept

Register a decoder for each file extension the application reads. Decoders are separate packages, so an application links only the formats it registers:

```go title="main.go"
package main

import (
	"context"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/yaml"
)

type CLI struct {
	ConfigFile conga.ConfigFile `multi:"true" short:"c"`

	APIEndpoint conga.OptionalFlag[string]        `default:"https://api.default.io" safe:"true"`
	Timeout     conga.OptionalFlag[time.Duration] `default:"10s" help:"Request timeout" safe:"true"`
	Theme       conga.OptionalFlag[string]        `default:"dark" enum:"dark,light,solarized,system" safe:"true"`
	Telemetry   conga.OptionalFlag[bool]          `default:"true" help:"Enable telemetry reporting"`
}

func (c *CLI) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func main() {
	app := conga.New[CLI]().
		Name("cascading-searchpaths").
		EnableWhy().
		RegisterDecoder(".yaml", yaml.Decode).
		RegisterDecoder(".yml", yaml.Decode)

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

Users see the accepted formats on the config flag in `--help`:

```text no-copy
  -c, --config-file=PATH,...                   Path to config file(s) [decoders: .yaml, .yml]
```

CONGA ships decoders for JSON, TOML, YAML, and HCL in `conga/decoder/...`; see [package decoder](/reference/decoders/).

## Search for a file in several places

Add `ConfigPaths` with the places to look, highest precedence first. Each entry is a directory; the program looks there for `config.<ext>` and `<app-name>.<ext>` for every registered extension. `~` and `${VAR:-default}` expand:

```go title="main.go" ins={6}
app := conga.New[CLI]().
	Name("cascading-searchpaths").
	EnableWhy().
	RegisterDecoder(".yaml", yaml.Decode).
	RegisterDecoder(".yml", yaml.Decode).
	ConfigOptions(conga.ConfigPaths(".", "~/.config/cascading-searchpaths", "${SYSCONF_DIR:-/etc/cascading-searchpaths}"))
```

The program loads the first file it finds and stops. In this run, all three places have a file, and the one in the working directory wins:

```sh
$ cascading-searchpaths --why
  FLAG             VALUE                        SOURCE                                ENV   CONFIG PATH
  --config-file    cascading-searchpaths.yaml   config                                -     -
  --api-endpoint   https://api.local.dev        config (cascading-searchpaths.yaml)   -     api-endpoint
  --timeout        10s                          default                               -     timeout
  --theme          dark                         default                               -     theme
  --telemetry      true                         default                               -     telemetry
```

Without `ConfigPaths`, the program looks in the user's config directory, `${XDG_CONFIG_HOME:-~/.config}`; [About the config cascade](/explanation/config-cascade/) covers that default. To load one fixed file instead of searching, use `conga.ConfigExplicit(path)`; the run fails if that file is missing.

## Merge files from every place

Add `ConfigCascading()` to load every file found and merge them key by key, higher places winning. The `ConfigFile` flag must then accept several files, with `multi:"true"`, as in the struct above:

```go title="main.go" ins={3}
ConfigOptions(
	conga.ConfigPaths(".", "~/.config/cascading-searchpaths", "${SYSCONF_DIR:-/etc/cascading-searchpaths}"),
	conga.ConfigCascading(),
)
```

Each value now comes from the highest file that sets it. Here the working directory sets `api_endpoint`, the user's file sets `theme` and `timeout`, and the system file sets `telemetry`:

```sh
$ cascading-searchpaths --why
  FLAG             VALUE                   SOURCE                                                ENV   CONFIG PATH
  --config-file    (3 files)               config                                                -     -
                   ├─ /etc/cascading-searchpaths/cascading-searchpaths.yaml
                   ├─ /home/alice/.config/cascading-searchpaths/cascading-searchpaths.yaml
                   └─ cascading-searchpaths.yaml
  --api-endpoint   https://api.local.dev   config (cascading-searchpaths.yaml)                   -     api-endpoint
  --timeout        15s                     config (/home/alice/.../cascading-searchpaths.yaml)   -     timeout
  --theme          light                   config (/home/alice/.../cascading-searchpaths.yaml)   -     theme
  --telemetry      false                   config (/etc/.../cascading-searchpaths.yaml)          -     telemetry

Config decoders: .yaml, .yml

Config paths:
  .                                           loaded: cascading-searchpaths.yaml
  /home/alice/.config/cascading-searchpaths   loaded: /home/alice/.config/cascading-searchpaths/cascading-searchpaths.yaml
  /etc/cascading-searchpaths                  loaded: /etc/cascading-searchpaths/cascading-searchpaths.yaml
```

The system file also sets `timeout: 30s`, but the user's `15s` wins. Flags on the command line and environment variables still beat every file.

## Let users name their own files

The `ConfigFile` field gives users `-c` / `--config-file`. Naming a file replaces the search: the program loads only the named files.

With `multi:"true"`, users can repeat `-c`, and a later file beats an earlier one:

```sh
$ cascading-searchpaths -c team.yaml -c ci.yaml --why
  FLAG             VALUE                    SOURCE               ENV   CONFIG PATH
  --config-file    (2 files)                cli                  -     -
                   ├─ team.yaml
                   └─ ci.yaml
  --api-endpoint   https://api.default.io   default              -     api-endpoint
  --timeout        5s                       config (ci.yaml)     -     timeout
  --theme          solarized                config (team.yaml)   -     theme
  --telemetry      true                     default              -     telemetry
```

Without `multi:"true"`, the flag takes one file and its help reads `--config-file=PATH`. A named file must exist:

```sh error={2}
$ cascading-searchpaths -c nope.yaml
cascading-searchpaths: error: cannot read config file "nope.yaml": stat nope.yaml: no such file or directory
```

`ConfigFile` belongs on the root command; subcommands read the loaded files through the session (see [List the config files a run loaded](#list-the-config-files-a-run-loaded)).

## Match keys to fields

A key matches the flag's name in kebab-case, and snake_case works too: the example's files write `api_endpoint:` for `--api-endpoint`. The `CONFIG PATH` column of `--why` shows each flag's key.

To read a different key, set `config:"<key>"`. To keep a field out of config files, set `config:"-"`. This variation adds a token that users pass on the command line or in the environment only:

```go title="main.go"
Token conga.OptionalFlag[string] `config:"-" help:"API token (command line or environment only)"`
```

A file that sets it is rejected:

```sh error={2}
$ cascading-searchpaths -c token.yaml
cascading-searchpaths: error: config file "token.yaml": field "token" is excluded from config files in command "cascading-searchpaths"
```

## Nest subcommand settings

A subcommand's flags live under a section named after the subcommand. This variation adds a `server` subcommand:

```go title="main.go"
type CLI struct {
	// ...
	Server conga.Command[ServerCmd] `summary:"Run the API server"`
}

type ServerCmd struct {
	Port conga.OptionalFlag[int] `default:"8080" safe:"true" help:"Listen port"`
}
```

```yaml title="server.yaml"
server:
  port: 9000
```

```sh
$ cascading-searchpaths -c server.yaml server --why
  FLAG     VALUE   SOURCE                 ENV   CONFIG PATH
  --port   9000    config (server.yaml)   -     server.port
```

[About subcommands and hierarchy](/explanation/subcommands/#hierarchical-configuration-mapping) describes how deeper hierarchies map.

## Add a format of your own

A decoder is a function with the signature `func(data []byte) (map[string]any, error)` that turns the file into a map of keys to values; nested maps hold subcommand sections. This one reads `key = value` lines:

```go title="main.go"
func iniDecoder(data []byte) (map[string]any, error) {
	result := make(map[string]any)
	scanner := bufio.NewScanner(bytes.NewReader(data))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("malformed line: %q", line)
		}

		result[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
```

Register it like a built-in decoder, and `.ini` joins the formats listed in `--help`:

```go title="main.go"
app.RegisterDecoder(".ini", iniDecoder)
```

An error from the decoder names the file:

```sh error={2}
$ cascading-searchpaths -c bad.ini
cascading-searchpaths: error: cannot parse config file "bad.ini": malformed line: "theme light"
```

For files that mix configuration with application data, such as a Taskfile, use a [`conga.Bootstrapper`](/how-to/load-domain-files/) instead of a decoder.

## List the config files a run loaded

`session.CandidateConfigFiles()` returns the places the program searched, and `session.LoadedConfigFiles()` the files it loaded, lowest precedence first:

```go title="main.go"
func (s *ServerCmd) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "port=%d\n", s.Port.Get())
	fmt.Fprintf(session.Stdout(), "candidates=%q\n", session.CandidateConfigFiles())
	fmt.Fprintf(session.Stdout(), "loaded=%q\n", session.LoadedConfigFiles())

	return nil
}
```

```sh
$ cascading-searchpaths server
port=8080
candidates=["." "/home/alice/.config/cascading-searchpaths" "/etc/cascading-searchpaths"]
loaded=["/etc/cascading-searchpaths/cascading-searchpaths.yaml" "/home/alice/.config/cascading-searchpaths/cascading-searchpaths.yaml" "cascading-searchpaths.yaml"]
```

## Know what users see when a file is wrong

A problem with the file itself is a config error (exit 1). A bad value in a valid file is reported like the same value on the command line (exit 2), naming the key it came from:

| What the file has | Result |
|---|---|
| A `-c` file that doesn't exist | Config error, exit 1 |
| Text the decoder can't parse | Config error, exit 1, naming the file |
| An extension with no registered decoder | Config error, exit 1 |
| An unknown key (`them: dark`) | Config error, exit 1, with a "did you mean" hint |
| A key for a `config:"-"` field | Config error, exit 1 |
| A value of the wrong type (`timeout: soon`) | Usage error, exit 2, naming the key |
| A value outside `enum` or `range` (`theme: pink`) | Usage error, exit 2, naming the key |

```text no-copy error={1-4}
cascading-searchpaths: error: config file "typo.yaml": unknown field "them" in command "cascading-searchpaths" (did you mean "theme"?)
cascading-searchpaths: error: unsupported config file format ".toml" for "settings.toml"
cascading-searchpaths: error: invalid duration "soon" (valid units: ms, s, m, h) for flag --timeout (from config key "timeout")
cascading-searchpaths: error: invalid value "pink" (must be one of: dark, light, solarized, system) for flag --theme (from config key "theme")
```

## Where to go next

- [About the config cascade](/explanation/config-cascade/): search order, the XDG default, and how merging works.
- [Package decoder](/reference/decoders/): the JSON, TOML, YAML, and HCL decoders.
- [How to load domain documents](/how-to/load-domain-files/): files that mix configuration with application data.
- [How to inspect value provenance](/how-to/inspect-value-provenance/): the full `--why` report.
- [Cascading search paths example](https://github.com/conga-sh/conga/tree/main/_examples/15-cascading-searchpaths): the program used on this page.
