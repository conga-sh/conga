---
title: How to get completion from tags and types
description: Complete enum values, paths, programs, symlinks, signals, HTTP methods, time zones, and feature names from tags and types with no predictor code, and turn completion off for a field.
---

CONGA completes a flag's values from its tags and its type. This guide builds a `deploy` command with a flag for each source of candidates: an `enum` tag, a `predictor` tag, the filesystem types, and the domain types. Positional arguments complete from the same tags and types.

The program enables completion with `EnableCompletion()`, as [How to enable shell completion](/how-to/enable-shell-completion/) describes. The listings below come from zsh, and other shells offer the same candidates in their own layout.

## Offer a fixed list of values

A flag with an `enum` tag completes its values:

```go title="main.go" hidden={1-8}
package main

import (
	"context"
	"fmt"

	"github.com/conga-sh/conga"
)

type Deploy struct {
	Env conga.OptionalFlag[string] `default:"dev" enum:"dev,staging,prod" help:"Target environment"`
}

func (d *Deploy) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "deploying to", d.Env.Get())

	return nil
}

func main() {
	app := conga.New[Deploy]().Name("deploy").EnableCompletion()

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

```ansi no-copy frame="code"
$ deploy --env <Tab>
[2mTarget environment[0m
dev      prod     staging
```

## Complete file and directory paths

Fields of the filesystem types complete paths. `conga.File` completes files, its `glob` tag keeps only the names that match, `conga.Dir` completes directories, and `conga.FileContent` completes files:

```go title="main.go" ins={3-5}
type Deploy struct {
	// ...
	Manifest conga.OptionalFlag[conga.File]        `existing:"true" glob:"*.yaml,*.yml"`
	WorkDir  conga.OptionalFlag[conga.Dir]
	Notes    conga.OptionalFlag[conga.FileContent] `help:"Release notes"`
}
```

Here the working directory holds `deploy.yaml`, `values.yml`, `app.toml`, `notes.txt`, and `README.md`, the directories `cache` and `releases`, and a symlink `current` to `releases/v2`:

```ansi no-copy frame="code"
$ deploy --manifest <Tab>
[2mPath to an existing file matching '*.yaml,*.yml'[0m
[1m[34mcache/[0m       [1m[34mreleases/[0m
[36mcurrent/[2m[39m  -- -> releases/v2[0m
[33mdeploy.yaml[39m  [33mvalues.yml[39m
```

```ansi no-copy frame="code"
$ deploy --work-dir <Tab>
[2mPath to a directory[0m
[1m[34mcache/[0m     [1m[34mreleases/[0m
[36mcurrent/[2m[39m  -- -> releases/v2[0m
```

Directories stay among the candidates whatever the `glob`, so a path completes one directory at a time. CONGA colours each path by kind, following `LS_COLORS` when it is set.

The `glob` patterns filter only the candidates. `existing` checks only that the file exists, so a file outside the patterns still passes:

```sh
$ deploy --manifest notes.txt
deploying to dev
```

## Complete paths for a string flag

To complete paths for a `string` flag, give it a `predictor` tag naming a built-in predictor:

```go title="main.go" ins={3-6}
type Deploy struct {
	// ...
	Hook    conga.OptionalFlag[string] `help:"Program to run after deploying" predictor:"exec"`
	Values  conga.OptionalFlag[string] `help:"Values file" predictor:"file:*.toml"`
	Cache   conga.OptionalFlag[string] `help:"Cache directory" predictor:"dir"`
	Current conga.OptionalFlag[string] `help:"Link to the current release" predictor:"symlink"`
}
```

| `predictor` value | Completes |
|---|---|
| `file` | Files |
| `file:*.toml` | Files matching the comma-separated patterns |
| `dir` | Directories |
| `exec` | Programs on the `PATH` |
| `symlink` | Symbolic links |

```ansi no-copy frame="code"
$ deploy --values <Tab>
[2mValues file[0m
[33mapp.toml[39m
[1m[34mcache/[0m     [1m[34mreleases/[0m
[36mcurrent/[2m[39m  -- -> releases/v2[0m
```

```ansi no-copy frame="code"
$ deploy --hook gre<Tab>
[2mProgram to run after deploying[0m
[32mgrep[39m       [32mgresource[39m
```

```ansi no-copy frame="code"
$ deploy --current <Tab>
[2mLink to the current release[0m
[36mcurrent/[2m[39m  -- -> releases/v2[0m
```

The tag changes only what completes. The value stays a `string`, and CONGA accepts a path that doesn't exist. To check that it exists, use `conga.File` or `conga.Dir` with `existing:"true"`.

## Turn completion off for a field

`predictor:"none"` stops a field from completing, even when its type would. Here `--output` is a `conga.File`, which would otherwise complete paths:

```go title="main.go" ins={3}
type Deploy struct {
	// ...
	Output conga.OptionalFlag[conga.File] `predictor:"none"`
}
```

```ansi no-copy frame="code"
$ deploy --output <Tab>
[2mPath to a file[0m
```

## Complete signals, HTTP methods, and time zones

Types from the `system` and `network` packages complete their own values, each with a description. `*time.Location` completes zone names once `Use(timezone.RegisterLocation)` registers the type on the builder:

```go title="main.go" ins={4,7-9,14-16} {22}
import (
	"context"
	"fmt"
	"time"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/network"
	"github.com/conga-sh/conga/system"
	"github.com/conga-sh/conga/timezone"
)

type Deploy struct {
	// ...
	Signal conga.OptionalFlag[system.Signal]
	Method conga.OptionalFlag[network.HTTPMethod]
	Zone   conga.OptionalFlag[*time.Location] `help:"Time zone"`
}

// ...

func main() {
	app := conga.New[Deploy]().Name("deploy").EnableCompletion().Use(timezone.RegisterLocation)

	// ...
}
```

```ansi no-copy frame="code"
$ deploy --signal SIGT<Tab>
[2mSignal name or number (e.g. SIGTERM, 9)[0m
[33mSIGTERM[2m[39m  -- Termination signal (graceful shutdown request)[0m
[33mSIGTRAP[2m[39m  -- Trace/breakpoint trap[0m
[35mSIGTSTP[2m[39m  -- Stop typed at terminal (Ctrl+Z)[0m
[35mSIGTTIN[2m[39m  -- Terminal input for background process[0m
[35mSIGTTOU[2m[39m  -- Terminal output for background process[0m
```

```ansi no-copy frame="code"
$ deploy --method <Tab>
[2mHTTP method (GET, POST, PUT, DELETE, ...)[0m
CONNECT[2m  -- Establish two-way tunnel to destination server[0m
[31mDELETE[2m[39m   -- Remove specified resource[0m
[32mGET[2m[39m      -- Retrieve resource representation[0m
HEAD[2m     -- Retrieve response headers without body[0m
OPTIONS[2m  -- Describe communication options for target resource[0m
[35mPATCH[2m[39m    -- Apply partial modifications to resource[0m
[34mPOST[2m[39m     -- Submit data for processing or resource creation[0m
[33mPUT[2m[39m      -- Replace resource state completely[0m
TRACE[2m    -- Perform message loop-back test along resource path[0m
```

```ansi no-copy frame="code"
$ deploy --zone Europe/L<Tab>
[2mTime zone[0m
Europe/Lisbon[2m  -- Portugal (mainland)[0m
Europe/London
```

Without `Use(timezone.RegisterLocation)`, `--zone` completes nothing, and the flag fails when the user gives it a value:

```sh error={2}
$ deploy --zone Europe/London
deploy: error: unsupported type *time.Location for flag --zone
```

[How to use the network, system, and timezone packages](/how-to/use-domain-packages/) covers the types themselves.

## Complete feature names

A `conga.Features` field declares the program's feature gates from a struct of `bool` fields, and its flag completes the feature names, then the values after `=`. Features need `EnableFeatures()` on the builder:

```go title="main.go" ins={1-4,8} {14}
type Gates struct {
	Canary   bool `help:"Send a share of traffic to the new version"`
	Rollback bool `help:"Roll back automatically on errors"`
}

type Deploy struct {
	// ...
	Features conga.Features[Gates] `flag:"feature-gates"`
}

// ...

func main() {
	app := conga.New[Deploy]().Name("deploy").EnableCompletion().EnableFeatures().Use(timezone.RegisterLocation)

	// ...
}
```

```ansi no-copy frame="code"
$ deploy --feature-gates <Tab>
[2mEnable or disable features (name=value)[0m
canary=[2m    -- Send a share of traffic to the new version[0m
rollback=[2m  -- Roll back automatically on errors[0m
```

```ansi no-copy frame="code"
$ deploy --feature-gates canary=<Tab>
[2mEnable or disable features (name=value)[0m
false  true
```

## Know what users see

| Field | What completion offers |
|---|---|
| `string` with no `enum` or `predictor` tag | No candidates |
| `map[string]string` | No candidates. zsh shows the hint `<key>=<value>` |
| `predictor:"none"` | No candidates, whatever the type |
| `conga.File` with a `glob` tag | Matching files, plus directories and symlinks. A file outside the patterns still passes |
| A `predictor` tag naming a path predictor | Paths. A path that doesn't exist still passes |
| `*time.Location` without `Use(timezone.RegisterLocation)` | No candidates. A value for the flag fails with exit 2 |

## Where to go next

- [How to compute and shape completion candidates](/how-to/compute-completion-candidates/): compute candidates in Go, from loaded data or an API, with `app.Predictor` and the `predict` package.
- [Predictors](/reference/struct-tags/#predictors): every `predictor` tag value.
- [Domain types](/reference/domain-types/): what each filesystem and domain type completes as.
