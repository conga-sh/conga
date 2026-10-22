package conga_test

import (
	"context"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type bootstrapSubcommand struct {
	Profile  conga.OptionalFlag[string] `bootstrap:"true"`
	SubOther conga.OptionalFlag[string] `name:"sub-other"`
}

func (c *bootstrapSubcommand) Run(_ context.Context, _ *conga.Session) error { return nil }

type bootstrapCLI struct {
	Dir      conga.OptionalFlag[string]   `bootstrap:"true" env:"CONGA_TEST_BOOTSTRAP_DIR" name:"dir"      short:"d"`
	Taskfile conga.OptionalFlag[string]   `bootstrap:"true" default:"Taskfile.yml"         name:"taskfile"`
	Verbose  conga.OptionalFlag[bool]     `bootstrap:"true" name:"verbose"                 short:"v"`
	Tags     conga.OptionalFlag[[]string] `bootstrap:"true" name:"tag"`
	Count    conga.Counter                `bootstrap:"true" name:"count"                   short:"c"`
	Tasks    conga.OptionalArg[[]string]  `bootstrap:"true" name:"task"                    sep:"none"`
	Other    conga.OptionalFlag[string]   `name:"other"`

	Deploy conga.Command[bootstrapSubcommand]
}

func (c *bootstrapCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

func TestConfigLoader_BootstrapFlags(t *testing.T) {
	t.Setenv("CONGA_TEST_BOOTSTRAP_DIR", "env-dir-overridden")

	var (
		dir      string
		taskfile string
		verbose  bool
		tags     []string
		count    int
		tasks    []string
		other    string
		otherSet bool
	)

	app := conga.New[bootstrapCLI]().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, root *bootstrapCLI) (*conga.ConfigDocument, error) {
			dir = root.Dir.Get()
			taskfile = root.Taskfile.Get()
			verbose = root.Verbose.Get()
			tags = root.Tags.Get()
			count = root.Count.Get()
			tasks = root.Tasks.Get()
			other = root.Other.Get()
			otherSet = root.Other.IsSet()

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"--dir", "cli-dir", "-v", "--tag", "one,two", "-ccc", "--other", "hidden", "build", "LIST=a,b"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "cli-dir", dir)
	assert.Equal(t, "Taskfile.yml", taskfile)
	assert.True(t, verbose)
	assert.Equal(t, []string{"one", "two"}, tags)
	assert.Equal(t, 3, count)
	assert.Equal(t, []string{"build", "LIST=a,b"}, tasks)
	assert.Equal(t, "", other)
	assert.False(t, otherSet)
}

func TestConfigLoader_BootstrapEnvFallback(t *testing.T) {
	t.Setenv("CONGA_TEST_BOOTSTRAP_DIR", "env-dir")

	var dir string

	app := conga.New[bootstrapCLI]().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, root *bootstrapCLI) (*conga.ConfigDocument, error) {
			dir = root.Dir.Get()

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"--taskfile", "Custom.yml"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "env-dir", dir)
}

func TestConfigLoader_BootstrapSubcommand(t *testing.T) {
	t.Parallel()

	var (
		profile  string
		other    string
		otherSet bool
	)

	app := conga.New[bootstrapCLI]().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, root *bootstrapCLI) (*conga.ConfigDocument, error) {
			profile = root.Deploy.Get().Profile.Get()
			other = root.Deploy.Get().SubOther.Get()
			otherSet = root.Deploy.Get().SubOther.IsSet()

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"deploy", "--profile", "prod", "--sub-other", "hidden"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "prod", profile)
	assert.Equal(t, "", other)
	assert.False(t, otherSet)
}

func TestConfigLoader_CompletionShowsFailedBootstrapFieldUnset(t *testing.T) {
	t.Parallel()

	type HelpCLI struct {
		noopRunner

		Dir   conga.OptionalFlag[string] `bootstrap:"true" default:"default-dir" name:"dir"`
		Count conga.OptionalFlag[int]    `bootstrap:"true" name:"count"`
	}

	var (
		dir        string
		dirSet     bool
		count      int
		countIsSet bool
	)

	app := conga.New[HelpCLI]().
		EnableCompletion().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, root *HelpCLI) (*conga.ConfigDocument, error) {
			dir = root.Dir.Get()
			dirSet = root.Dir.IsSet()
			count = root.Count.Get()
			countIsSet = root.Count.IsSet()

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"__complete", "zsh", "--count", "not-a-number", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "default-dir", dir)
	assert.True(t, dirSet)
	assert.Equal(t, 0, count)
	assert.False(t, countIsSet)
}

func TestConfigLoader_CompletionUsesTypedBootstrapFieldsAndRestores(t *testing.T) {
	t.Parallel()

	root := &bootstrapCLI{}

	var seen []string

	app := conga.New(root).
		EnableCompletion().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, received *bootstrapCLI) (*conga.ConfigDocument, error) {
			seen = append(seen, received.Dir.Get())

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"__complete", "zsh", "--dir", "completion-dir", "build", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, []string{"completion-dir"}, seen)
	assert.Equal(t, "", root.Dir.Get())
	assert.False(t, root.Dir.IsSet())
}

func TestConfigLoader_CompletionRawArgsMatchRunView(t *testing.T) {
	t.Parallel()

	type RawCLI struct {
		noopRunner

		Raw conga.RawArgs `bootstrap:"true" name:"raw"`
	}

	var seen []string

	app := conga.New[RawCLI]().
		EnableCompletion().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, root *RawCLI) (*conga.ConfigDocument, error) {
			seen = root.Raw.Get()

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"__complete", "zsh", "run", "extra", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, []string{"run", "extra"}, seen)
}

func TestConfigLoader_TypedRootReceivesCallerInstance(t *testing.T) {
	t.Parallel()

	root := &bootstrapCLI{}

	var captured *bootstrapCLI

	app := conga.New(root).
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, received *bootstrapCLI) (*conga.ConfigDocument, error) {
			captured = received

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, root, captured)
}

func TestConfigLoader_BootstrapDashArgs(t *testing.T) {
	t.Parallel()

	type DashCLI struct {
		noopRunner

		Trail conga.DashArgs `bootstrap:"true" name:"trail"`
	}

	var trail []string

	app := conga.New[DashCLI]().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, root *DashCLI) (*conga.ConfigDocument, error) {
			trail = root.Trail.Get()

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"--", "one", "two"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, []string{"one", "two"}, trail)
}

func TestConfigLoader_BootstrapRawArgs(t *testing.T) {
	t.Parallel()

	type RawCLI struct {
		noopRunner

		Raw conga.RawArgs `bootstrap:"true" name:"raw"`
	}

	var raw []string

	app := conga.New[RawCLI]().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, root *RawCLI) (*conga.ConfigDocument, error) {
			raw = root.Raw.Get()

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"build", "extra"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, []string{"build", "extra"}, raw)
}

func TestConfigLoader_BootstrapArgumentPosition(t *testing.T) {
	t.Parallel()

	type ArgCLI struct {
		noopRunner

		First  conga.OptionalArg[string] `name:"first"`
		Second conga.OptionalArg[string] `bootstrap:"true" name:"second"`
	}

	var (
		first      string
		second     string
		firstIsSet bool
	)

	app := conga.New[ArgCLI]().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, root *ArgCLI) (*conga.ConfigDocument, error) {
			first = root.First.Get()
			firstIsSet = root.First.IsSet()
			second = root.Second.Get()

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"one", "two"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "", first)
	assert.False(t, firstIsSet)
	assert.Equal(t, "two", second)
}

func TestConfigLoader_CompletionLookupEnvUsesInjectedLookup(t *testing.T) {
	t.Parallel()

	var (
		compLine string
		found    bool
	)

	app := conga.New[bootstrapCLI]().
		EnableCompletion().
		ConfigOptions(conga.ConfigLoader(func(ctx conga.ConfigContext, _ *bootstrapCLI) (*conga.ConfigDocument, error) {
			compLine, found = ctx.LookupEnv("COMP_LINE")

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		}))

	line := "app --dir completion-dir"

	_, _ = app.CompleteCompLineForShell("fish", line, len(line))

	assert.True(t, found)
	assert.Equal(t, line, compLine)
}

func TestConfigLoader_CompletionSubcommandRestoresSelection(t *testing.T) {
	t.Parallel()

	root := &bootstrapCLI{}

	var profile string

	app := conga.New(root).
		EnableCompletion().
		ConfigOptions(conga.ConfigLoader(func(_ conga.ConfigContext, received *bootstrapCLI) (*conga.ConfigDocument, error) {
			profile = received.Deploy.Get().Profile.Get()

			return nil, nil //nolint:nilnil // the nil document contract keeps the tier inactive
		})).
		Args([]string{"__complete", "zsh", "deploy", "--profile", "prod", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "prod", profile)
	assert.False(t, root.Deploy.IsSelected())
}
