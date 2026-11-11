---
title: How to load domain documents
description: Read Taskfile-, compose-, and pipeline-style documents with conga.Bootstrapper, keep their application data, and feed their settings to flags.
---

A domain document, such as a Taskfile, a `docker-compose.yml`, or a pipeline manifest, holds application data (tasks, steps, services) next to settings that belong to flags (`parallelism`, `log_level`). To read one, implement `conga.Bootstrapper`:

```go
func (c *CLI) Bootstrap(ctx context.Context, handle *conga.Bootstrap) error
```

CONGA calls `Bootstrap` once per run, after it binds the flags tagged `bootstrap:"true"` and before it reads config files. To load a domain document, the method reads the file and hands its settings to CONGA with `handle.SetConfig`. Those settings rank like a config file, below the command line and environment and above defaults.

`Bootstrap` also runs for `--why` and shell completion, so keep it to quick local reads. During completion its errors stay quiet, and the real run reports them.

This guide builds a small pipeline runner, in which `-f` names a YAML file and `--pipeline` picks a pipeline in it. The [bootstrapper example](https://github.com/conga-sh/conga/tree/main/_examples/30-bootstrapper) is a fuller version that reads several files.

## Choose a Bootstrapper or a decoder

A config decoder suits a file that holds only settings, at a path known before flags are read. Register it with `app.RegisterDecoder` and locate the file with `conga.ConfigPaths`, `conga.ConfigExplicit`, or `--config-file` (see [How to manage config files](/how-to/manage-config-files/)). Every top-level key must then match a flag.

Use a `Bootstrapper` when the document needs more than that:

- Its path or section comes from a flag, and config files are located before any flag is read. Here, `-f` names the file and `--pipeline deploy` picks a section.
- It mixes settings with application data that matches no flag, such as each pipeline's `steps`.
- It needs its own merge rules, such as joining `steps` from several files where the config source would replace the whole list.

`Bootstrap` can also load `.env` files from a location a flag chooses, as [How to load .env files](/how-to/load-env-files/#load-a-env-from-a-directory-the-user-picks) shows.

## Tag the fields that locate the document

Tag the flags that `Bootstrap` reads to find the document with `bootstrap:"true"`:

```go title="main.go"
type CLI struct {
	File     conga.OptionalFlag[string] `bootstrap:"true" default:"pipeline.yaml" help:"Pipeline file" safe:"true" short:"f"`
	Pipeline conga.OptionalFlag[string] `bootstrap:"true" help:"Pipeline to run" safe:"true"`

	Parallelism conga.OptionalFlag[int]    `default:"1" help:"Parallel steps" safe:"true"`
	LogLevel    conga.OptionalFlag[string] `default:"warn" enum:"debug,info,warn,error" help:"Log level" safe:"true"`

	pipelines map[string]pipeline
}
```

CONGA binds the tagged fields from the command line, the environment, and their defaults, then calls `Bootstrap`. After it returns, CONGA binds every field again, now with the document's settings in place. CONGA leaves unexported fields alone, so `pipelines` can hold the application data for `Run`. [Bootstrap fields](/reference/struct-tags/#bootstrap-fields) lists which types and fields the tag accepts.

## Map the document to Go types

The document sets defaults at the top level and lets each pipeline override them. `parallelism` and `log_level` set flags, and `steps` is application data:

```yaml title="pipeline.yaml"
parallelism: 4
log_level: info

pipelines:
  build:
    steps:
      - go build ./...
  deploy:
    parallelism: 1
    log_level: debug
    steps:
      - ./deploy.sh
```

```go title="main.go"
type pipelineFile struct {
	Parallelism int                 `yaml:"parallelism"`
	LogLevel    string              `yaml:"log_level"`
	Pipelines   map[string]pipeline `yaml:"pipelines"`
}

type pipeline struct {
	Parallelism int      `yaml:"parallelism"`
	LogLevel    string   `yaml:"log_level"`
	Steps       []string `yaml:"steps"`
}
```

## Read the document in Bootstrap

`Bootstrap` reads the file and records its settings in a `conga.ConfigDocument`, first the top-level values and then the selected pipeline's, which replace them:

```go title="main.go"
func (c *CLI) Bootstrap(_ context.Context, handle *conga.Bootstrap) error {
	// [!callout /File/] Already resolved, since bootstrap fields bind before `Bootstrap` runs.
	path := c.File.Get()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var file pipelineFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	// Keep the application data for Run.
	c.pipelines = file.Pipelines

	document := &conga.ConfigDocument{}
	// The top-level settings, with no scope.
	setSettings(document, path, "", file.Parallelism, file.LogLevel)

	if name := c.Pipeline.Get(); name != "" {
		selected, ok := file.Pipelines[name]
		if !ok {
			return fmt.Errorf("%s has no pipeline %q", path, name)
		}

		// [!callout /setSettings/] The selected pipeline's settings. Written later, so they replace the top-level ones.
		setSettings(document, path, "pipeline "+name, selected.Parallelism, selected.LogLevel)
	}

	// Hand the settings to CONGA.
	handle.SetConfig(document)

	return nil
}

func setSettings(document *conga.ConfigDocument, origin, scope string, parallelism int, logLevel string) {
	if parallelism != 0 {
		document.SetScoped("parallelism", parallelism, origin, scope)
	}

	if logLevel != "" {
		document.SetScoped("log-level", logLevel, origin, scope)
	}
}
```

`Run` uses the resolved flags and the kept pipelines:

```go title="main.go"
func (c *CLI) Run(_ context.Context, session *conga.Session) error {
	fmt.Fprintf(session.Stdout(), "parallelism=%d log-level=%s\n", c.Parallelism.Get(), c.LogLevel.Get())

	if name := c.Pipeline.Get(); name != "" {
		fmt.Fprintf(session.Stdout(), "pipeline %s: steps=%v\n", name, c.pipelines[name].Steps)
	}

	return nil
}
```

Without `--pipeline`, the top-level settings apply. With `--pipeline deploy`, the pipeline's own settings win:

```sh
$ bootstrapper
parallelism=4 log-level=info
$ bootstrapper --pipeline deploy
parallelism=1 log-level=debug
pipeline deploy: steps=[./deploy.sh]
```

## Let the default file be missing

With the code so far, a run in a directory without `pipeline.yaml` fails, though the user named no file. To make the default file optional, return from `Bootstrap` early when the file doesn't exist and nothing asked for it:

```go title="main.go" ins={5-7}
func (c *CLI) Bootstrap(_ context.Context, handle *conga.Bootstrap) error {
	path := c.File.Get()

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !c.File.Changed() && !c.Pipeline.IsSet() {
		return nil
	}
	if err != nil {
		return err
	}
```

`Changed` reports whether the command line, the environment, or a config file set `-f`, so a file the user names must still exist. A pipeline can't be selected without the file, so the check also requires `--pipeline` to be unset. A `Bootstrap` that returns before `SetConfig` contributes no settings, and every flag keeps its default. In a directory without `pipeline.yaml`:

```sh error={4,6}
$ bootstrapper
parallelism=1 log-level=warn
$ bootstrapper -f pipeline.yaml
bootstrapper: error: bootstrap failed: open pipeline.yaml: no such file or directory
$ bootstrapper --pipeline deploy
bootstrapper: error: bootstrap failed: open pipeline.yaml: no such file or directory
```

## Record settings with origins

Each `ConfigDocument` write takes an origin, a label you choose (here the file path), and `--why` shows it:

| Method | Records |
|---|---|
| `Set(key, value, origin)` | A whole value for one key: a scalar, a slice, or a nested map. `key` matches the flag's `config:"..."` tag exactly, or, when it has no tag, the flag name spelled in the app's key casing. |
| `SetKey(key, entryKey, value, origin)` | One entry of a map flag, merged with earlier entries. |
| `SetScoped(...)`, `SetKeyScoped(...)` | The same writes with a scope, shown as `pipeline.yaml › pipeline deploy`. |

```sh
$ bootstrapper --pipeline deploy --why
Value provenance for "bootstrapper":

  FLAG            VALUE           SOURCE                                     ENV                CONFIG PATH
  --file          pipeline.yaml   default                                    FLOW_FILE          file
  --pipeline      deploy          cli                                        FLOW_PIPELINE      pipeline
  --parallelism   1               config (pipeline.yaml › pipeline deploy)   FLOW_PARALLELISM   parallelism
  --log-level     debug           config (pipeline.yaml › pipeline deploy)   FLOW_LOG_LEVEL     log-level

Config paths:
  pipeline.yaml   loaded
```

A later write to the same key replaces the earlier one, and a `Set` replaces any entries written with `SetKey`. Leaving out `SetConfig`, or passing it `nil`, contributes no settings. [The conga reference](/reference/conga/) has the signatures.

## Keep the command line and environment in charge

Document settings rank like a config file, so the command line and environment variables beat them. With `EnvPrefix("FLOW")`:

```sh
$ FLOW_LOG_LEVEL=error bootstrapper --pipeline deploy
parallelism=1 log-level=error
pipeline deploy: steps=[./deploy.sh]
$ bootstrapper --pipeline deploy --parallelism 8
parallelism=8 log-level=debug
pipeline deploy: steps=[./deploy.sh]
```

## Accept several files

To let users pass several files, make `-f` a list and write each file's settings in the order given. A later file then beats an earlier one, as with repeated `-c` for [config files](/how-to/manage-config-files/#let-users-name-their-own-files). The [bootstrapper example](https://github.com/conga-sh/conga/tree/main/_examples/30-bootstrapper) does this, and also merges the pipelines across files, joining their `steps`.

## Combine with config files

A document combines with config files key by key. It wins the keys it sets, and file keys it leaves alone keep their values. `Bootstrap` therefore works alongside `conga.ConfigPaths`, `conga.ConfigExplicit`, `conga.ConfigCascading`, registered decoders, and a root `conga.ConfigFile` flag. A `Bootstrap` that only loads `.env` files skips `SetConfig`.

## Know what users see when a document is wrong

Any error that `Bootstrap` returns is a config error (`ErrConfig`, exit 1) with a `bootstrap failed:` prefix. CONGA checks the settings themselves the way it checks a config file:

| Situation | Result |
|---|---|
| The file named by `-f` or `FLOW_FILE` is missing | `bootstrap failed: …`, exit 1 |
| The default `pipeline.yaml` is missing | Flags keep their defaults with the check from [Let the default file be missing](#let-the-default-file-be-missing). Without it, or with `--pipeline` set, `bootstrap failed: …`, exit 1 |
| `--pipeline` names a pipeline the file lacks | `bootstrap failed: …`, exit 1 |
| The file is not valid YAML | `bootstrap failed: …`, exit 1 |
| `Set` names a key that matches no flag, or a `config:"-"` field | Config error, exit 1, with a "did you mean" hint for typos |
| A value outside `enum` or `range` (`log_level: loud`) | Usage error, exit 2, naming the key |

```text no-copy error={1-4} wrap
bootstrapper: error: bootstrap failed: open nope.yaml: no such file or directory
bootstrapper: error: bootstrap failed: pipeline.yaml has no pipeline "release"
bootstrapper: error: bootstrap failed: parse broken.yaml: yaml: line 1: did not find expected ',' or ']'
bootstrapper: error: invalid value "loud" (must be one of: debug, info, warn, error) for flag --log-level (from config key "log-level")
```

## Where to go next

- [How to load .env files](/how-to/load-env-files/): `.env` files located by a flag.
- [How to manage config files](/how-to/manage-config-files/): settings-only files and decoders.
- [About value precedence](/explanation/value-precedence/): where document settings sit among the sources.
- [How to inspect value provenance](/how-to/inspect-value-provenance/): the full `--why` report.
- [Bootstrapper example](https://github.com/conga-sh/conga/tree/main/_examples/30-bootstrapper): a fuller version that reads several files.
