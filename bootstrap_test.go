package conga_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type bootstrapSubcommand struct {
	Profile  conga.OptionalFlag[string] `bootstrap:"true"`
	SubOther conga.OptionalFlag[string] `name:"sub-other"`
}

func (c *bootstrapSubcommand) Run(_ context.Context, _ *conga.Session) error { return nil }

// bootstrapObservation records bootstrapCLI's bootstrap-time state for
// assertions after the run.
type bootstrapObservation struct {
	dir         string
	taskfile    string
	verbose     bool
	tags        []string
	count       int
	tasks       []string
	other       string
	otherSet    bool
	profile     string
	subOther    string
	subOtherSet bool
	self        *bootstrapCLI
	compLine    string
	compLineOK  bool
}

type bootstrapCLI struct {
	obs bootstrapObservation

	Dir      conga.OptionalFlag[string]   `bootstrap:"true" env:"CONGA_TEST_BOOTSTRAP_DIR" name:"dir"      short:"d"`
	Taskfile conga.OptionalFlag[string]   `bootstrap:"true" default:"Taskfile.yml"         name:"taskfile"`
	Verbose  conga.OptionalFlag[bool]     `bootstrap:"true" name:"verbose"                 short:"v"`
	Tags     conga.OptionalFlag[[]string] `bootstrap:"true" name:"tag"`
	Count    conga.Counter                `bootstrap:"true" name:"count"                   short:"c"`
	Tasks    conga.OptionalArg[[]string]  `bootstrap:"true" name:"task"                    sep:"none"`
	Other    conga.OptionalFlag[string]   `name:"other"`

	Deploy conga.Command[bootstrapSubcommand]
}

func (c *bootstrapCLI) Bootstrap(_ context.Context, handle *conga.Bootstrap) error {
	c.obs = bootstrapObservation{
		dir:         c.Dir.Get(),
		taskfile:    c.Taskfile.Get(),
		verbose:     c.Verbose.Get(),
		tags:        c.Tags.Get(),
		count:       c.Count.Get(),
		tasks:       c.Tasks.Get(),
		other:       c.Other.Get(),
		otherSet:    c.Other.IsSet(),
		profile:     c.Deploy.Get().Profile.Get(),
		subOther:    c.Deploy.Get().SubOther.Get(),
		subOtherSet: c.Deploy.Get().SubOther.IsSet(),
		self:        c,
	}
	c.obs.compLine, c.obs.compLineOK = handle.LookupEnv("COMP_LINE")

	return nil
}

func (c *bootstrapCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

func TestBootstrapper_BootstrapFlags(t *testing.T) {
	t.Setenv("CONGA_TEST_BOOTSTRAP_DIR", "env-dir-overridden")

	root := &bootstrapCLI{}

	app := conga.New(root).
		Args([]string{"--dir", "cli-dir", "-v", "--tag", "one,two", "-ccc", "--other", "hidden", "build", "LIST=a,b"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "cli-dir", root.obs.dir)
	assert.Equal(t, "Taskfile.yml", root.obs.taskfile)
	assert.True(t, root.obs.verbose)
	assert.Equal(t, []string{"one", "two"}, root.obs.tags)
	assert.Equal(t, 3, root.obs.count)
	assert.Equal(t, []string{"build", "LIST=a,b"}, root.obs.tasks)
	assert.Equal(t, "", root.obs.other)
	assert.False(t, root.obs.otherSet)
}

func TestBootstrapper_BootstrapEnvFallback(t *testing.T) {
	t.Setenv("CONGA_TEST_BOOTSTRAP_DIR", "env-dir")

	root := &bootstrapCLI{}

	app := conga.New(root).
		Args([]string{"--taskfile", "Custom.yml"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "env-dir", root.obs.dir)
}

func TestBootstrapper_BootstrapSubcommand(t *testing.T) {
	t.Parallel()

	root := &bootstrapCLI{}

	app := conga.New(root).
		Args([]string{"deploy", "--profile", "prod", "--sub-other", "hidden"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "prod", root.obs.profile)
	assert.Equal(t, "", root.obs.subOther)
	assert.False(t, root.obs.subOtherSet)
}

type helpBootstrapCLI struct {
	noopRunner

	Dir   conga.OptionalFlag[string] `bootstrap:"true" default:"default-dir" name:"dir"`
	Count conga.OptionalFlag[int]    `bootstrap:"true" name:"count"`

	obs struct {
		dir        string
		dirSet     bool
		count      int
		countIsSet bool
	}
}

func (c *helpBootstrapCLI) Bootstrap(_ context.Context, _ *conga.Bootstrap) error {
	c.obs.dir = c.Dir.Get()
	c.obs.dirSet = c.Dir.IsSet()
	c.obs.count = c.Count.Get()
	c.obs.countIsSet = c.Count.IsSet()

	return nil
}

func TestBootstrapper_CompletionShowsFailedBootstrapFieldUnset(t *testing.T) {
	t.Parallel()

	root := &helpBootstrapCLI{}

	app := conga.New(root).
		EnableCompletion().
		Args([]string{"__complete", "zsh", "--count", "not-a-number", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "default-dir", root.obs.dir)
	assert.True(t, root.obs.dirSet)
	assert.Equal(t, 0, root.obs.count)
	assert.False(t, root.obs.countIsSet)
}

func TestBootstrapper_CompletionUsesTypedBootstrapFieldsAndRestores(t *testing.T) {
	t.Parallel()

	root := &bootstrapCLI{}

	app := conga.New(root).
		EnableCompletion().
		Args([]string{"__complete", "zsh", "--dir", "completion-dir", "build", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "completion-dir", root.obs.dir)
	assert.Equal(t, "", root.Dir.Get())
	assert.False(t, root.Dir.IsSet())
}

type rawBootstrapCLI struct {
	noopRunner

	Raw         conga.RawArgs `bootstrap:"true" name:"raw"`
	observedRaw []string
}

func (c *rawBootstrapCLI) Bootstrap(_ context.Context, _ *conga.Bootstrap) error {
	c.observedRaw = c.Raw.Get()

	return nil
}

func TestBootstrapper_CompletionRawArgsMatchRunView(t *testing.T) {
	t.Parallel()

	root := &rawBootstrapCLI{}

	app := conga.New(root).
		EnableCompletion().
		Args([]string{"__complete", "zsh", "run", "extra", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, []string{"run", "extra"}, root.observedRaw)
}

func TestBootstrapper_TypedRootReceivesCallerInstance(t *testing.T) {
	t.Parallel()

	root := &bootstrapCLI{}

	app := conga.New(root).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, root, root.obs.self)
}

type dashBootstrapCLI struct {
	noopRunner

	Trail         conga.DashArgs `bootstrap:"true" name:"trail"`
	observedTrail []string
}

func (c *dashBootstrapCLI) Bootstrap(_ context.Context, _ *conga.Bootstrap) error {
	c.observedTrail = c.Trail.Get()

	return nil
}

func TestBootstrapper_BootstrapDashArgs(t *testing.T) {
	t.Parallel()

	root := &dashBootstrapCLI{}

	app := conga.New(root).
		Args([]string{"--", "one", "two"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, []string{"one", "two"}, root.observedTrail)
}

func TestBootstrapper_BootstrapRawArgs(t *testing.T) {
	t.Parallel()

	root := &rawBootstrapCLI{}

	app := conga.New(root).
		Args([]string{"build", "extra"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, []string{"build", "extra"}, root.observedRaw)
}

type argBootstrapCLI struct {
	noopRunner

	First  conga.OptionalArg[string] `name:"first"`
	Second conga.OptionalArg[string] `bootstrap:"true" name:"second"`

	observedFirst    string
	observedFirstSet bool
	observedSecond   string
}

func (c *argBootstrapCLI) Bootstrap(_ context.Context, _ *conga.Bootstrap) error {
	c.observedFirst = c.First.Get()
	c.observedFirstSet = c.First.IsSet()
	c.observedSecond = c.Second.Get()

	return nil
}

func TestBootstrapper_BootstrapArgumentPosition(t *testing.T) {
	t.Parallel()

	root := &argBootstrapCLI{}

	app := conga.New(root).
		Args([]string{"one", "two"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "", root.observedFirst)
	assert.False(t, root.observedFirstSet)
	assert.Equal(t, "two", root.observedSecond)
}

func TestBootstrapper_CompletionLookupEnvUsesInjectedLookup(t *testing.T) {
	t.Parallel()

	root := &bootstrapCLI{}

	app := conga.New(root).
		EnableCompletion()

	line := "app --dir completion-dir"

	_, _ = app.CompleteCompLineForShell("fish", line, len(line))

	assert.True(t, root.obs.compLineOK)
	assert.Equal(t, line, root.obs.compLine)
}

type dotenvFeatures struct {
	ForceAll bool `name:"force-all"`
}

type dotenvCLI struct {
	noopRunner

	Dir conga.OptionalFlag[string] `bootstrap:"true" default:"." name:"dir"`

	Features conga.Features[dotenvFeatures] `config:"experiments" env:"CONGA_TEST_DOTENV_FEATURES" flag:"feature-gates"`
	ForceAll conga.OptionalFlag[bool]       `feature:"force-all"`

	Host conga.OptionalFlag[string] `env:"CONGA_TEST_DOTENV_HOST"`
}

func (c *dotenvCLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	return b.Dotenv(filepath.Join(c.Dir.Get(), ".env"))
}

// TestBootstrapper_DotenvFeatureGateAttribution pins the issue #68 acceptance
// path: a bootstrap flag selects the directory, .env there enables a gated
// feature, and --why attributes the feature and the env-sourced flag to that
// .env file.
//
//nolint:paralleltest // t.Chdir and t.Setenv cannot be used with t.Parallel
func TestBootstrapper_DotenvFeatureGateAttribution(t *testing.T) {
	t.Chdir(t.TempDir())

	err := os.WriteFile(".env", []byte("CONGA_TEST_DOTENV_FEATURES=force-all=true\nCONGA_TEST_DOTENV_HOST=from-dotenv\n"), 0o600)
	assert.NoError(t, err)

	var stdout bytes.Buffer

	cli := &dotenvCLI{}

	app := conga.New(cli).
		EnableFeatures().
		EnableWhy().
		SetenvFunc(t.Setenv).
		Stdout(&stdout).
		Args([]string{"--force-all", "--why"})

	err = app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Features.Get().ForceAll)
	assert.Equal(t, "from-dotenv", cli.Host.Get())
	assert.Equal(t, conga.SourceEnv, cli.Host.Source())

	output := stdout.String()

	assert.Contains(t, output, "--force-all")
	assert.Contains(t, output, "env (.env)")
}

type dotenvPrecedenceCLI struct {
	noopRunner

	Dir conga.OptionalFlag[string] `bootstrap:"true" default:"." name:"dir"`
}

func (c *dotenvPrecedenceCLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	return b.Dotenv(filepath.Join(c.Dir.Get(), ".env"))
}

// TestBootstrapper_DotenvPrecedence pins the shared-origins rule: a host
// environment variable is never overwritten, while a file loaded by
// Bootstrap.Dotenv overrides a key an earlier App.Dotenv file set.
//

func TestBootstrapper_DotenvPrecedence(t *testing.T) {
	t.Setenv("CONGA_TEST_PRECEDENCE_HOST", "host-value")

	dir := t.TempDir()
	t.Chdir(dir)

	err := os.WriteFile(".env", []byte("CONGA_TEST_PRECEDENCE_KEY=from-cwd\nCONGA_TEST_PRECEDENCE_HOST=from-cwd\n"), 0o600)
	assert.NoError(t, err)

	sub := filepath.Join(dir, "sub")
	err = os.MkdirAll(sub, 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(sub, ".env"), []byte("CONGA_TEST_PRECEDENCE_KEY=from-sub\n"), 0o600)
	assert.NoError(t, err)

	cli := &dotenvPrecedenceCLI{}

	app := bootstrapApp(cli).
		Dotenv().
		SetenvFunc(t.Setenv).
		Args([]string{"--dir", "sub"})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "from-sub", os.Getenv("CONGA_TEST_PRECEDENCE_KEY"))
	assert.Equal(t, "host-value", os.Getenv("CONGA_TEST_PRECEDENCE_HOST"))
}

type dotenvErrorCLI struct {
	noopRunner

	load       func(*conga.Bootstrap) error
	err        error
	classified bool
}

func (c *dotenvErrorCLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	c.err = c.load(b)
	c.classified = errors.Is(c.err, conga.ErrConfig)

	return c.err
}

// TestBootstrapper_DotenvErrorsClassifiedUnderErrConfig pins that both dotenv
// methods classify their failures under ErrConfig at the call boundary, so a
// Bootstrap hook can branch on errors.Is(err, conga.ErrConfig) before returning.
func TestBootstrapper_DotenvErrorsClassifiedUnderErrConfig(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing.env")

	malformed := filepath.Join(t.TempDir(), ".env")
	err := os.WriteFile(malformed, []byte("BROKEN LINE\n"), 0o600)
	assert.NoError(t, err)

	tests := []struct {
		name string
		load func(*conga.Bootstrap) error
	}{
		{name: "dotenv malformed", load: func(b *conga.Bootstrap) error { return b.Dotenv(malformed) }},
		{name: "dotenv explicit missing", load: func(b *conga.Bootstrap) error { return b.DotenvExplicit(missing) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cli := &dotenvErrorCLI{load: test.load}

			app := bootstrapApp(cli).Args([]string{})

			err := app.Run()

			assert.Error(t, err)
			assert.True(t, cli.classified)
			assert.IsError(t, err, conga.ErrConfig)
		})
	}
}

type bootstrapCWDDotenvCLI struct {
	noopRunner
}

func (c *bootstrapCWDDotenvCLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	return b.Dotenv()
}

// TestBootstrapper_DotenvDefaultsToCWDHierarchy pins the documented zero-path
// behavior: an argument-less Bootstrap.Dotenv probes .env then .env.local in
// the current working directory, so the local file wins.
//
//nolint:paralleltest // t.Chdir and t.Setenv cannot be used with t.Parallel
func TestBootstrapper_DotenvDefaultsToCWDHierarchy(t *testing.T) {
	t.Chdir(t.TempDir())

	err := os.WriteFile(".env", []byte("CONGA_TEST_CWD_DOTENV=from-base\n"), 0o600)
	assert.NoError(t, err)

	err = os.WriteFile(".env.local", []byte("CONGA_TEST_CWD_DOTENV=from-local\n"), 0o600)
	assert.NoError(t, err)

	cli := &bootstrapCWDDotenvCLI{}

	app := bootstrapApp(cli).
		SetenvFunc(t.Setenv).
		Args([]string{})

	err = app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "from-local", os.Getenv("CONGA_TEST_CWD_DOTENV"))
}

func TestBootstrapper_CompletionSubcommandRestoresSelection(t *testing.T) {
	t.Parallel()

	root := &bootstrapCLI{}

	app := conga.New(root).
		EnableCompletion().
		Args([]string{"__complete", "zsh", "deploy", "--profile", "prod", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, "prod", root.obs.profile)
	assert.False(t, root.Deploy.IsSelected())
}
