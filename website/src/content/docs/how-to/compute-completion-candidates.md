---
title: How to compute and shape completion candidates
description: Complete names from a file the program loads, combine predictors, leave out values already given, complete values in parts, read field tags in a type's completion, and fall back when a lookup is slow.
---

A predictor callback is a Go function that CONGA calls when the shell asks for a field's candidates, so they can come from a file the program loads, an API, or what the user has already typed. This guide builds a `deploy` command that completes from its project file, as a task runner completes the tasks in its Taskfile. The tutorial [Add shell completion to a CLI](/tutorials/complete-a-cli/) covers `enum` tags, a type that implements `conga.Predictable`, and a callback registered by name.

The program enables completion with `EnableCompletion()`, as [How to enable shell completion](/how-to/enable-shell-completion/) describes. The listings below come from zsh, and other shells offer the same candidates in their own layout.

## Choose where the candidates come from

| Candidates | Source |
|---|---|
| A fixed list for one field | An `enum` tag, or `predict.Values` registered with `app.Predictor` and named in a `predictor` tag |
| The same candidates for every field of a type | The type implements `conga.Predictable`. Its `Predict` can return a `predict.Callback` to compute them when the shell asks |
| Candidates that depend on the field's tags | The type implements `conga.TagAwarePredictable` ([below](#read-field-tags-in-a-types-completion)) |
| Candidates for a type from another package | `app.RegisterType` with `conga.WithPredictor` ([How to add custom flag types](/how-to/add-custom-flag-types/)) |
| Candidates computed from the command line or loaded data | A `predict.Callback` registered with `app.Predictor` |

When a field has several sources, a `predictor` tag wins. [Which predictor a field uses](/reference/predict/#which-predictor-a-field-uses) lists the full order.

## Complete names from a project file

`deploy` deploys the services named on its command line. It reads them from a project file, `deploy.yaml`, which also sets its flags' defaults, lists each service's tags, and lists the labels a deployment may carry:

```yaml title="deploy.yaml"
region: eu-west-1
replicas: 2
cluster: prod-eu-1

services:
  api:
    description: Public API
    tags: ["1.4", "1.5"]
  web:
    description: Website
    tags: ["1.26", "1.27"]
  worker:
    description: Background jobs
    tags: ["2.0"]

labels:
  team: [payments, platform]
  tier: [frontend, backend]
```

`Bootstrap` reads the file named by `-f`. It hands `region`, `replicas`, and `cluster` to CONGA with `SetConfig`, so they set those flags as a config file would, and keeps the services and labels in an unexported field. `Run` deploys each service at its last tag or at the tag `--pin` names, and checks every tag and label against the file. A callback registered as `services` turns the loaded services into candidates:

```go title="main.go" hidden={49-75,79-120}
package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

type service struct {
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

type project struct {
	Region   string              `yaml:"region"`
	Replicas int                 `yaml:"replicas"`
	Cluster  string              `yaml:"cluster"`
	Services map[string]service  `yaml:"services"`
	Labels   map[string][]string `yaml:"labels"`
}

type Manifest string

type Deploy struct {
	File     conga.OptionalFlag[string]            `bootstrap:"true" default:"deploy.yaml" help:"Project file" short:"f"`
	Region   conga.OptionalFlag[string]            `default:"us-east-1" help:"Region to deploy to"`
	Replicas conga.OptionalFlag[int]               `default:"1" help:"Copies of each service"`
	Cluster  conga.OptionalFlag[string]            `default:"local" help:"Cluster to deploy to"`
	Pin      conga.OptionalFlag[[]string]          `help:"Deploy a service at one of its tags (service:tag)"`
	Label    conga.OptionalFlag[map[string]string] `help:"Label the deployment (key=value)"`
	Manifest conga.OptionalFlag[Manifest]          `help:"Extra manifest to apply"`
	Services conga.RequiredArg[[]string]           `help:"Services to deploy, or all" predictor:"services"`

	project project
}

func (d *Deploy) Bootstrap(_ context.Context, handle *conga.Bootstrap) error {
	path := d.File.Get()

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) && !d.File.Changed() {
		return fmt.Errorf("no %s in this directory (name a project file with -f)", path)
	}
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, &d.project); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	document := &conga.ConfigDocument{}
	if d.project.Region != "" {
		document.Set("region", d.project.Region, path)
	}
	if d.project.Replicas != 0 {
		document.Set("replicas", d.project.Replicas, path)
	}
	if d.project.Cluster != "" {
		document.Set("cluster", d.project.Cluster, path)
	}
	handle.SetConfig(document)

	return nil
}

func (d *Deploy) Run(_ context.Context, session *conga.Session) error {
	names := d.Services.Get()
	if slices.Contains(names, "all") {
		names = slices.Sorted(maps.Keys(d.project.Services))
	}

	pins := make(map[string]string)
	for _, pin := range d.Pin.Get() {
		name, tag, _ := strings.Cut(pin, ":")
		if !slices.Contains(d.project.Services[name].Tags, tag) {
			return fmt.Errorf("%s has no tag %q for service %q", d.File.Get(), tag, name)
		}
		pins[name] = tag
	}

	var images []string
	for _, name := range names {
		entry, ok := d.project.Services[name]
		if !ok {
			return fmt.Errorf("%s has no service %q", d.File.Get(), name)
		}
		images = append(images, name+":"+cmp.Or(pins[name], entry.Tags[len(entry.Tags)-1]))
	}

	var labels []string
	for _, key := range slices.Sorted(maps.Keys(d.Label.Get())) {
		value := d.Label.Get()[key]
		if !slices.Contains(d.project.Labels[key], value) {
			return fmt.Errorf("%s allows no label %s=%s", d.File.Get(), key, value)
		}
		labels = append(labels, key+"="+value)
	}

	stdout := session.Stdout()
	fmt.Fprintf(stdout, "deploying %s to %s in %s (replicas: %d)\n", strings.Join(images, ", "), d.Cluster.Get(), d.Region.Get(), d.Replicas.Get())
	if len(labels) > 0 {
		fmt.Fprintf(stdout, "labels: %s\n", strings.Join(labels, ", "))
	}
	if d.Manifest.IsSet() {
		fmt.Fprintf(stdout, "applying %s\n", d.Manifest.Get())
	}

	return nil
}

func (d *Deploy) serviceNames(predict.Context) predict.Predictor {
	descriptions := make(map[string]string, len(d.project.Services))
	for name, entry := range d.project.Services {
		descriptions[name] = entry.Description
	}

	// [!callout /Map/] Sorted candidates, each with its description.
	return predict.Map(descriptions)
}

func main() {
	deploy := &Deploy{}

	app := conga.New(deploy).
		Name("deploy").
		EnableCompletion().
		Predictor("services", predict.Callback(deploy.serviceNames))

	if err := app.Run(); err != nil {
		app.Exit(err)
	}
}
```

`main` creates the `Deploy` and passes it to `conga.New`, so the callback, a method on that `Deploy`, reads the project that `Bootstrap` stored on it. With `conga.New[Deploy]()`, CONGA would create the root during `Run`, after the predictor was registered, and the callback would have no way to reach it.

During completion, CONGA runs `Bootstrap` after it binds `-f` from the command line, the environment, or its default, and before it calls the callback. The candidates follow the file the user names:

```ansi no-copy frame="code"
$ deploy <Tab>
api[2m     -- Public API[0m
web[2m     -- Website[0m
worker[2m  -- Background jobs[0m
```

```ansi no-copy frame="code"
$ deploy -f staging.yaml <Tab>
db[2m  -- Database[0m
```

```sh
$ deploy api web
deploying api:1.5, web:1.27 to prod-eu-1 in eu-west-1 (replicas: 2)
$ deploy -f staging.yaml db
deploying db:16.4 to staging-us-1 in us-west-2 (replicas: 1)
```

When `Bootstrap` fails during completion, such as on a missing project file, the shell shows no error, and the callback sees no loaded data. The error appears on the real run. A callback prints nothing either, because the shell reads the program's standard output as the completion result. [How to load domain documents](/how-to/load-domain-documents/) covers reading the file in `Bootstrap`.

A callback can also read the file itself instead of using what `Bootstrap` stored. It then needs no instance created in `main`:

```go title="main.go"
func serviceNames(predictContext predict.Context) predict.Predictor {
	path := predictContext.FlagValue("file", "deploy.yaml")

	data, err := os.ReadFile(path)
	if err != nil {
		return predict.None()
	}

	var file project
	if err := yaml.Unmarshal(data, &file); err != nil {
		return predict.None()
	}

	descriptions := make(map[string]string, len(file.Services))
	for name, entry := range file.Services {
		descriptions[name] = entry.Description
	}

	return predict.Map(descriptions)
}
```

That works with `conga.New[Deploy]()`, registered as `predict.Callback(serviceNames)`, and puts the read inside the callback's deadline. `FlagValue` returns the path only when the user types `-f` or `--file`, so the callback repeats the `deploy.yaml` default by hand, and must change with the field's `default` tag.

## Add fixed values to computed ones

`predict.Batch` joins the candidates of several predictors, keeping the first of any duplicate value. `Run` already accepts `all`, and the batch offers it next to the loaded services:

```go title="main.go" {7-10}
func (d *Deploy) serviceNames(predict.Context) predict.Predictor {
	descriptions := make(map[string]string, len(d.project.Services))
	for name, entry := range d.project.Services {
		descriptions[name] = entry.Description
	}

	return predict.Batch(
		predict.Items(predict.NewItem("all", "Every service in the file")),
		predict.Map(descriptions),
	)
}
```

```ansi no-copy frame="code"
$ deploy <Tab>
all[2m     -- Every service in the file[0m
api[2m     -- Public API[0m
web[2m     -- Website[0m
worker[2m  -- Background jobs[0m
```

```sh
$ deploy all
deploying api:1.5, web:1.27, worker:2.0 to prod-eu-1 in eu-west-1 (replicas: 2)
```

## Leave out values already given

`predict.Context` holds the positional arguments typed before the cursor in `Args`. To stop offering a service the user has already named, remove them with `Exclude`:

```go title="main.go" {1,10}
func (d *Deploy) serviceNames(predictContext predict.Context) predict.Predictor {
	descriptions := make(map[string]string, len(d.project.Services))
	for name, entry := range d.project.Services {
		descriptions[name] = entry.Description
	}

	return predict.Batch(
		predict.Items(predict.NewItem("all", "Every service in the file")),
		predict.Map(descriptions),
	).Exclude(predictContext.Args...)
}
```

```ansi no-copy frame="code"
$ deploy api <Tab>
all[2m     -- Every service in the file[0m
web[2m     -- Website[0m
worker[2m  -- Background jobs[0m
```

`FlagValue` and `FlagValues` read earlier flags the same way. `Args` and the flag accessors see only what was typed on the command line, not values from the environment, a config file, or a `default` tag.

## Complete a value in parts

`predict.MultiParts` splits the value at a delimiter and calls a function for the part under the cursor, with the finished parts in `Parts`. `--pin web:1.26` deploys a service at one of its tags, so the first part completes from the service names and the second from that service's tags:

```go title="main.go" {3,22} ins={7-18,23}
type Deploy struct {
	// ...
	Pin      conga.OptionalFlag[[]string]          `help:"Deploy a service at one of its tags (service:tag)" predictor:"pins"`
	// ...
}

func (d *Deploy) pins(predictContext predict.Context) predict.Predictor {
	if len(predictContext.Parts) == 0 {
		var names []string
		for _, name := range slices.Sorted(maps.Keys(d.project.Services)) {
			names = append(names, name+":")
		}

		// [!callout /NoSpace/] Keeps the cursor after the `:`, ready for the tag.
		return predict.Values(names...).NoSpace()
	}

	return predict.Values(d.project.Services[predictContext.Parts[0]].Tags...)
}

// ...

		Predictor("services", predict.Callback(deploy.serviceNames)).
		Predictor("pins", predict.MultiParts(":", deploy.pins))
```

```ansi no-copy frame="code"
$ deploy --pin <Tab>
[2mDeploy a service at one of its tags (service:tag)[0m
api:     web:     worker:
```

```ansi no-copy frame="code"
$ deploy --pin web:1.2<Tab>
[2mDeploy a service at one of its tags (service:tag)[0m
1.26  1.27
```

For `key=value` values, `predict.KeyValue` takes the keys and the values of each. A `map[string]string` flag offers no candidates of its own, so `--label` names a callback that returns the labels the file allows:

```go title="main.go" {3,13} ins={7-9,14}
type Deploy struct {
	// ...
	Label    conga.OptionalFlag[map[string]string] `help:"Label the deployment (key=value)" predictor:"labels"`
	// ...
}

func (d *Deploy) labels(predict.Context) predict.Predictor {
	return predict.KeyValue(d.project.Labels)
}

// ...

		Predictor("pins", predict.MultiParts(":", deploy.pins)).
		Predictor("labels", predict.Callback(deploy.labels))
```

```ansi no-copy frame="code"
$ deploy --label <Tab>
[2mLabel the deployment (key=value)[0m
team=  tier=
```

```ansi no-copy frame="code"
$ deploy --label tier=<Tab>
[2mLabel the deployment (key=value)[0m
backend   frontend
```

```sh
$ deploy --pin web:1.26 --label tier=frontend web
deploying web:1.26 to prod-eu-1 in eu-west-1 (replicas: 2)
labels: tier=frontend
```

## Read field tags in a type's completion

A type that implements `conga.TagAwarePredictable` receives the tags of the field that uses it. The type must also implement `conga.TagSpecProvider` to declare those tags. `Manifest` reads an `ext` tag and completes files with that extension:

```go title="main.go" ins={3-9} {13}
type Manifest string

func (m Manifest) TagSpecs() []conga.TagSpec {
	return []conga.TagSpec{{Name: "ext", Kind: conga.TagKindString}}
}

func (m Manifest) Predict(tags conga.FieldTags) predict.Predictor {
	return predict.Files(tags.String("ext"))
}

type Deploy struct {
	// ...
	Manifest conga.OptionalFlag[Manifest]          `ext:".yaml" help:"Extra manifest to apply"`
	// ...
}
```

```ansi no-copy frame="code"
$ deploy --manifest manifests/<Tab>
[2mExtra manifest to apply[0m
[33mcanary.yaml[39m    [33mrollback.yaml[39m
```

`manifests/` also holds a `README.md`, which the `ext` tag leaves out:

```sh
$ deploy --manifest manifests/canary.yaml api
deploying api:1.5 to prod-eu-1 in eu-west-1 (replicas: 2)
applying manifests/canary.yaml
```

Without `TagSpecs`, CONGA rejects the tag, and the program fails at startup:

```sh error={2} wrap
$ deploy api
deploy: error: struct Deploy field Manifest: schema error: invalid struct tag: unknown struct tag "ext" (did you mean "env"?). Use app.AllowTags("ext") to permit external tags
```

`FieldTags` also offers `Has` and `Bool`.

## Fall back when a lookup is slow

CONGA gives a callback 250 milliseconds (`predict.DefaultTimeout`), and returns no candidates after that. `Timeout` sets another deadline, and its second argument is the predictor to use when the deadline passes. The project file sets `--cluster` to `prod-eu-1`, and the other clusters come from an API, which returns one name per line. The callback keeps the last answer in the user's cache directory for the fallback. `clustersURL` stands for your own API:

```go title="main.go" {3,50} ins={9-46,51}
type Deploy struct {
	// ...
	Cluster  conga.OptionalFlag[string]            `default:"local" help:"Cluster to deploy to" predictor:"clusters"`
	// ...
}

// ...

const clustersURL = "https://deploy.example.com/clusters"

func clusterCache() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}

	return filepath.Join(dir, "deploy", "clusters")
}

func clusters(predictContext predict.Context) predict.Predictor {
	response, err := http.Get(clustersURL)
	if err != nil {
		return cachedClusters(predictContext)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK {
		return cachedClusters(predictContext)
	}

	if err := os.MkdirAll(filepath.Dir(clusterCache()), 0o755); err == nil {
		_ = os.WriteFile(clusterCache(), data, 0o600)
	}

	return predict.Values(strings.Fields(string(data))...)
}

func cachedClusters(predict.Context) predict.Predictor {
	data, err := os.ReadFile(clusterCache())
	if err != nil {
		return predict.None()
	}

	return predict.Values(strings.Fields(string(data))...)
}

// ...

		Predictor("labels", predict.Callback(deploy.labels)).
		Predictor("clusters", predict.Callback(clusters).Timeout(time.Second, predict.Callback(cachedClusters)))
```

The file now also imports `io`, `net/http`, `path/filepath`, and `time`.

When the API answers within a second, the candidates come from it and refresh the cache. When it is slow or unreachable, the shell offers the cached list after at most a second:

```ansi no-copy frame="code"
$ deploy --cluster <Tab>
[2mCluster to deploy to[0m
prod-eu-1     prod-us-1     staging-us-1
```

```sh
$ deploy --cluster prod-us-1 api
deploying api:1.5 to prod-us-1 in eu-west-1 (replicas: 2)
```

## Know what users see

| Situation | Result |
|---|---|
| A `predictor` tag names an unregistered predictor | The program fails at startup with exit 1, and the message suggests the closest registered name |
| A field carries a tag its type doesn't declare in `TagSpecs` | The program fails at startup with exit 1 |
| No `deploy.yaml` in the directory, and no `-f` | The run fails with exit 1 and names `-f` as the fix. Completion shows no error and offers only `all` |
| `-f` names a file that doesn't exist | The run fails with exit 1 with the `open` error. Completion shows no error and offers only `all` |
| A callback passes its deadline | No candidates, or the `Timeout` fallback |
| A callback returns `predict.None()` | No candidates |
| A value comes from the environment, a config file, or a `default` tag | `Args`, `FlagValue`, and `FlagValues` don't see it |
| The user types a value the candidates don't include | Completion doesn't check it. `deploy` checks services, tags, and labels against the file in `Run` |

```sh error={2}
$ deploy --pin web:1.28 web
deploy: error: deploy.yaml has no tag "1.28" for service "web"
```

```sh error={2} wrap
$ deploy api
deploy: error: struct Deploy field Cluster: schema error: invalid struct tag: unregistered predictor "cluster" (did you mean "clusters"?). Register it with app.Predictor("cluster", action)
```

## Where to go next

- [How to get completion from tags and types](/how-to/complete-from-tags-and-types/): the paths, programs, signals, time zones, and other values CONGA completes with no code.
- [How to load domain documents](/how-to/load-domain-documents/): read a project file in `Bootstrap`, and record its settings with origins.
- [Package predict](/reference/predict/): every constructor, combinator, and interface.
- [Shell completion example](https://github.com/conga-sh/conga/tree/main/_examples/19-shell-completion): every predictor and combinator in one program.
