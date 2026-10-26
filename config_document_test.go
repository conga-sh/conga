package conga_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/assertx"
)

// documentBootstrapper is embedded by test CLIs to implement conga.Bootstrapper.
// The zero value supplies no document (the tier stays inactive); tests set doc,
// err, or onBootstrap before constructing the app.
type documentBootstrapper struct {
	doc         *conga.ConfigDocument
	err         error
	onBootstrap func(handle *conga.Bootstrap) error
}

func (c *documentBootstrapper) Bootstrap(_ context.Context, handle *conga.Bootstrap) error {
	if c.onBootstrap != nil {
		return c.onBootstrap(handle)
	}

	if c.err != nil {
		return c.err
	}

	if c.doc != nil {
		handle.SetConfig(c.doc)
	}

	return nil
}

type documentBasicCLI struct {
	noopRunner
	documentBootstrapper

	Host conga.OptionalFlag[string] `default:"localhost"`
	Port conga.OptionalFlag[int]    `default:"8000"`
}

type documentSliceCLI struct {
	noopRunner
	documentBootstrapper

	Tags conga.OptionalFlag[[]string] `default:"default"`
}

type documentCounterCLI struct {
	noopRunner
	documentBootstrapper

	Verbosity conga.Counter `short:"v"`
}

type documentMapCLI struct {
	noopRunner
	documentBootstrapper

	Headers conga.OptionalFlag[map[string]string]
}

type documentSessionCLI struct {
	documentBootstrapper

	Host conga.OptionalFlag[string]
	Port conga.OptionalFlag[int]

	Loaded    []string
	Candidate []string
}

func (c *documentSessionCLI) Run(_ context.Context, session *conga.Session) error {
	c.Loaded = session.LoadedConfigFiles()
	c.Candidate = session.CandidateConfigFiles()

	return nil
}

type documentExcludedCLI struct {
	noopRunner
	documentBootstrapper

	Secret conga.OptionalFlag[string] `config:"-"`
}

type documentNamespaceCLI struct {
	noopRunner
	documentBootstrapper

	Port conga.OptionalFlag[int] `config:"network.port" default:"80"`
}

type documentEnumCLI struct {
	noopRunner
	documentBootstrapper

	Mode conga.OptionalFlag[string] `default:"dev" enum:"dev,prod"`
}

type documentDeploySubcommand struct {
	Replicas conga.OptionalFlag[int] `default:"1"`
}

func (s *documentDeploySubcommand) Run(_ context.Context, _ *conga.Session) error { return nil }

type documentSubcommandCLI struct {
	noopRunner
	documentBootstrapper

	Deploy conga.Command[documentDeploySubcommand]
}

type documentExperiments struct {
	ForceAll bool `name:"force-all"`
}

type documentFeaturesCLI struct {
	noopRunner
	documentBootstrapper

	Features conga.Features[documentExperiments] `config:"experiments" env:"-"`

	ForceAll conga.OptionalFlag[bool] `feature:"force-all"`
}

func documentWith(entries ...func(document *conga.ConfigDocument)) *conga.ConfigDocument {
	document := &conga.ConfigDocument{}
	for _, entry := range entries {
		entry(document)
	}

	return document
}

// bootstrapApp builds the app around a caller-owned root so tests can install
// their document, error, or hook on the CLI before the run. The extra cli
// reference keeps conga-lint's single-use-root check from treating the shared
// helper parameter as a throwaway root.
func bootstrapApp[T any](cli *T) *conga.App[T] {
	_ = cli

	return conga.New(cli)
}

func TestBootstrapper_DocumentOverridesDefault(t *testing.T) {
	t.Parallel()

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("host", "document.example.com", "embedded")
	})}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "document.example.com", cli.Host.Get())
	assert.Equal(t, conga.SourceConfig, cli.Host.Source())
	assert.Equal(t, 8000, cli.Port.Get())
	assert.Equal(t, conga.SourceDefault, cli.Port.Source())
}

func TestBootstrapper_EnvOverridesDocument(t *testing.T) {
	t.Setenv("CONGA_TEST_LOADER_HOST", "env.example.com")

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("host", "document.example.com", "embedded")
	})}

	app := bootstrapApp(cli).
		EnvPrefix("CONGA_TEST_LOADER").
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "env.example.com", cli.Host.Get())
	assert.Equal(t, conga.SourceEnv, cli.Host.Source())
}

func TestBootstrapper_CLIOverridesDocument(t *testing.T) {
	t.Parallel()

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("host", "document.example.com", "embedded")
	})}

	app := bootstrapApp(cli).
		Args([]string{"--host", "cli.example.com"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "cli.example.com", cli.Host.Get())
	assert.Equal(t, conga.SourceCLI, cli.Host.Source())
}

func TestBootstrapper_SliceFlagFromDocument(t *testing.T) {
	t.Parallel()

	cli := &documentSliceCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("tags", []string{"alpha", "beta"}, "embedded")
	})}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"alpha", "beta"}, cli.Tags.Get())
	assert.Equal(t, conga.SourceConfig, cli.Tags.Source())
}

func TestBootstrapper_SliceFlagCLIOverridesDocument(t *testing.T) {
	t.Parallel()

	cli := &documentSliceCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("tags", []string{"alpha", "beta"}, "embedded")
	})}

	app := bootstrapApp(cli).
		Args([]string{"--tags", "cli-one", "--tags", "cli-two"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"cli-one", "cli-two"}, cli.Tags.Get())
	assert.Equal(t, conga.SourceCLI, cli.Tags.Source())
}

func TestBootstrapper_CounterFromDocument(t *testing.T) {
	t.Parallel()

	cli := &documentCounterCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("verbosity", "2", "embedded")
	})}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 2, cli.Verbosity.Get())
	assert.Equal(t, conga.SourceConfig, cli.Verbosity.Source())
}

func TestBootstrapper_MapFlagMergesAcrossTiers(t *testing.T) {
	t.Parallel()

	cli := &documentMapCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("headers", map[string]string{"region": "eu", "shared": "document"}, "taskrc")
	})}

	app := bootstrapApp(cli).
		Args([]string{"--headers", "shared=cli", "--headers", "team=core"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"region": "eu", "shared": "cli", "team": "core"}, cli.Headers.Get())
	assert.Equal(t, conga.SourceCLI, cli.Headers.Source())
}

func TestBootstrapper_MapFlagNestedAnySection(t *testing.T) {
	t.Parallel()

	cli := &documentMapCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("headers", map[string]any{"Region": "eu", "team": 3}, "taskrc")
	})}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"Region": "eu", "team": "3"}, cli.Headers.Get())
	assert.Equal(t, conga.SourceConfig, cli.Headers.Source())
}

func TestBootstrapper_NilDocumentLeavesTierInactive(t *testing.T) {
	t.Parallel()

	cli := new(documentBasicCLI)

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", cli.Host.Get())
	assert.Equal(t, conga.SourceDefault, cli.Host.Source())
}

func TestBootstrapper_EmptyDocumentLeavesDefaults(t *testing.T) {
	t.Parallel()

	cli := &documentBasicCLI{doc: &conga.ConfigDocument{}}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "localhost", cli.Host.Get())
	assert.Equal(t, conga.SourceDefault, cli.Host.Source())
}

func TestBootstrapper_ErrorClassifiedUnderErrConfig(t *testing.T) {
	t.Parallel()

	var stderrBuffer bytes.Buffer

	exitCode := -1

	cli := &documentBasicCLI{err: errors.New("taskrc unreadable")}

	app := bootstrapApp(cli).
		ExitFunc(func(code int) { exitCode = code }).
		Stderr(&stderrBuffer).
		Args([]string{})

	err := app.Run()
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.False(t, conga.IsUsageError(err))
	assert.Equal(t, "bootstrap failed: taskrc unreadable", err.Error())

	app.Exit(err)

	assert.Equal(t, 1, exitCode)
}

func TestBootstrapper_UnknownKeyFailsWithSuggestion(t *testing.T) {
	t.Parallel()

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("hots", "typo.example.com", "taskrc")
	})}

	app := bootstrapApp(cli).
		Name("app").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `config file "taskrc"`)
	assert.Contains(t, err.Error(), `unknown field "hots" in command "app"`)
	assert.Contains(t, err.Error(), `did you mean "host"?`)
}

func TestBootstrapper_ExcludedFlagRejected(t *testing.T) {
	t.Parallel()

	cli := &documentExcludedCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("secret", "leaked", "taskrc")
	})}

	app := bootstrapApp(cli).
		Name("app").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `field "secret" is excluded from config files in command "app"`)
}

func TestBootstrapper_SubcommandSection(t *testing.T) {
	t.Parallel()

	cli := &documentSubcommandCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("deploy", map[string]any{"replicas": 3}, "taskrc")
	})}

	app := bootstrapApp(cli).
		Args([]string{"deploy"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 3, cli.Deploy.Get().Replicas.Get())
	assert.Equal(t, conga.SourceConfig, cli.Deploy.Get().Replicas.Source())
}

func TestBootstrapper_NamespaceSection(t *testing.T) {
	t.Parallel()

	cli := &documentNamespaceCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("network", map[string]any{"port": 9090}, "taskrc")
	})}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9090, cli.Port.Get())
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
}

func TestBootstrapper_EnumViolationMatchesFileMessageShape(t *testing.T) {
	t.Parallel()

	cli := &documentEnumCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("mode", "staging", "taskrc")
	})}

	app := bootstrapApp(cli).
		Name("app").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Equal(t, `invalid value "staging" (must be one of: dev, prod) for flag --mode`, err.Error())
}

func TestBootstrapper_RangeViolationMatchesFileMessageShape(t *testing.T) {
	t.Parallel()

	type RangeCLI struct {
		noopRunner
		documentBootstrapper

		Workers conga.OptionalFlag[int] `default:"2" range:"1..8"`
	}

	cli := &RangeCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("workers", 64, "taskrc")
	})}

	app := bootstrapApp(cli).
		Name("app").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Contains(t, err.Error(), "for flag --workers")
}

func TestBootstrapper_CoercionViolationAttributesToConfig(t *testing.T) {
	t.Parallel()

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("port", "not-a-number", "taskrc")
	})}

	app := bootstrapApp(cli).
		Name("app").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.Contains(t, err.Error(), `for flag --port (from config key "port")`)
}

func TestBootstrapper_WhyReportsOrigins(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := &documentMapCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("headers", map[string]string{"region": "eu"}, "taskrc")
	})}

	app := bootstrapApp(cli).
		EnableWhy().
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	output := stdoutBuffer.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "--headers")
	assert.Contains(t, output, "config (taskrc)")
	assert.Contains(t, output, "loaded")
	assert.NotContains(t, output, "loaded: taskrc")
}

// TestBootstrapper_SourceDetailReportsOrigin pins that a document-supplied
// scalar records the application-supplied origin label as its source detail,
// matching the map-flag behavior.
func TestBootstrapper_SourceDetailReportsOrigin(t *testing.T) {
	t.Parallel()

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("port", 7100, "profile dev")
	})}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
	assert.Equal(t, "profile dev", cli.Port.SourceDetail())
}

// TestBootstrapper_SourceDetailFallsBackToConfigKey pins the documented
// fallback: when the document supplies no origin for a key, the source detail
// reports the matched configuration key.
func TestBootstrapper_SourceDetailFallsBackToConfigKey(t *testing.T) {
	t.Parallel()

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("port", 7100, "")
	})}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, conga.SourceConfig, cli.Port.Source())
	assert.Equal(t, "port", cli.Port.SourceDetail())
}

func TestBootstrapper_ScopedOriginSeparatesDetailFromConfigPath(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.SetScoped("port", 9200, "pipeline.local.yaml", "pipeline deploy")
	})}

	app := bootstrapApp(cli).
		Name("app").
		EnableWhy().
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "pipeline.local.yaml › pipeline deploy", cli.Port.SourceDetail())

	rows, configPaths, found := strings.Cut(stdoutBuffer.String(), "Config paths:")

	assert.True(t, found)
	assert.Contains(t, rows, "config (pipeline.local.yaml › pipeline deploy)")
	assert.Contains(t, configPaths, "pipeline.local.yaml")
	assert.Contains(t, configPaths, "loaded")
	assert.NotContains(t, configPaths, "pipeline deploy")
}

func TestBootstrapper_SetKeyScopedSeparatesDetailFromConfigPath(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := &documentMapCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.SetKeyScoped("headers", "region", "eu", "taskrc", "profile dev")
	})}

	app := bootstrapApp(cli).
		Name("app").
		EnableWhy().
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)

	rows, configPaths, found := strings.Cut(stdoutBuffer.String(), "Config paths:")

	assert.True(t, found)
	assert.Contains(t, rows, "region=██████")
	assert.Contains(t, rows, "config (taskrc › profile dev)")
	assert.Contains(t, configPaths, "taskrc")
	assert.NotContains(t, configPaths, "profile dev")
}

func TestBootstrapper_SessionAccessorsExcludeScope(t *testing.T) {
	t.Parallel()

	cli := &documentSessionCLI{doc: documentWith(
		func(document *conga.ConfigDocument) {
			document.SetScoped("host", "document.example.com", "taskrc", "profile dev")
		},
		func(document *conga.ConfigDocument) {
			document.SetScoped("port", 9200, "taskrc", "profile prod")
		},
	)}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"taskrc"}, cli.Loaded)
	assert.Equal(t, []string{"taskrc"}, cli.Candidate)
}

func TestBootstrapper_WhyAttributesAppError(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := &documentBasicCLI{err: errors.New("taskrc unreadable")}

	app := bootstrapApp(cli).
		Name("app").
		EnableWhy().
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "taskrc unreadable")
}

func TestBootstrapper_WhyValidationFailureReportsUniqueCandidate(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("hots", "typo.example.com", "taskrc")
	})}

	app := bootstrapApp(cli).
		Name("app").
		EnableWhy().
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `unknown field "hots"`)
}

func TestBootstrapper_SessionAccessorsReportOrigins(t *testing.T) {
	t.Parallel()

	cli := &documentSessionCLI{doc: documentWith(
		func(document *conga.ConfigDocument) {
			document.Set("host", "document.example.com", "taskrc")
		},
		func(document *conga.ConfigDocument) {
			document.Set("port", 9200, "embedded")
		},
	)}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"taskrc", "embedded"}, cli.Loaded)
	assert.Equal(t, []string{"taskrc", "embedded"}, cli.Candidate)
}

func TestBootstrapper_FeatureGateFromDocument(t *testing.T) {
	t.Parallel()

	cli := &documentFeaturesCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("experiments", map[string]string{"force-all": "true"}, "taskrc")
	})}

	app := bootstrapApp(cli).
		EnableFeatures().
		Args([]string{"--force-all"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Features.Get().ForceAll)
	assert.Equal(t, conga.SourceConfig, cli.Features.Source())
	assert.True(t, cli.ForceAll.Get())
	assert.Equal(t, conga.SourceCLI, cli.ForceAll.Source())
}

func TestBootstrapper_FeatureGateOffPrunesFlag(t *testing.T) {
	t.Parallel()

	cli := &documentFeaturesCLI{doc: &conga.ConfigDocument{}}

	app := bootstrapApp(cli).
		EnableFeatures().
		Args([]string{"--force-all"})

	err := app.Run()

	assert.Error(t, err)
	assert.True(t, conga.IsUsageError(err))
}

func TestBootstrapper_FeatureEnvChannelExcluded(t *testing.T) {
	t.Setenv("CONGA_TEST_LOADER_FEATURES", "force-all=true")

	cli := &documentFeaturesCLI{doc: &conga.ConfigDocument{}}

	app := bootstrapApp(cli).
		EnableFeatures().
		EnvPrefix("CONGA_TEST_LOADER").
		Args([]string{"--force-all"})

	err := app.Run()

	assert.Error(t, err)
	assert.True(t, conga.IsUsageError(err))
}

func TestBootstrapper_FeatureInvalidValueFailsUnderErrConfig(t *testing.T) {
	t.Parallel()

	cli := &documentFeaturesCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("experiments", map[string]string{"force-all": "maybe"}, "taskrc")
	})}

	app := bootstrapApp(cli).
		EnableFeatures().
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
}

func TestBootstrapper_CompletionReflectsDocumentGate(t *testing.T) {
	t.Parallel()

	cli := &documentFeaturesCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("experiments", map[string]string{"force-all": "true"}, "taskrc")
	})}

	app := bootstrapApp(cli).
		EnableFeatures().
		EnableCompletion()

	invoked, _ := app.CompleteForShell("fish", []string{"--"})

	values := make(map[string]bool, len(invoked.Items))

	for _, item := range invoked.Items {
		values[item.Value] = true
	}

	assert.True(t, values["--force-all"])
}

func TestBootstrapper_CompletionPrunesDisabledGate(t *testing.T) {
	t.Parallel()

	cli := &documentFeaturesCLI{doc: &conga.ConfigDocument{}}

	app := bootstrapApp(cli).
		EnableFeatures().
		EnableCompletion()

	invoked, _ := app.CompleteForShell("fish", []string{"--"})

	values := make(map[string]bool, len(invoked.Items))

	for _, item := range invoked.Items {
		values[item.Value] = true
	}

	assert.False(t, values["--force-all"])
}

func TestBootstrapper_CompletionInvokesBootstrapOnce(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	cli := &documentFeaturesCLI{}
	cli.onBootstrap = func(*conga.Bootstrap) error {
		invocations.Add(1)

		return nil
	}

	app := bootstrapApp(cli).
		EnableFeatures().
		EnableCompletion()

	app.CompleteForShell("fish", []string{"--"})

	assert.Equal(t, int32(1), invocations.Load())
}

func TestBootstrapper_RunsOncePerRun(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	cli := &documentFeaturesCLI{}
	cli.onBootstrap = func(*conga.Bootstrap) error {
		invocations.Add(1)

		return nil
	}

	app := bootstrapApp(cli).
		EnableFeatures().
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, int32(1), invocations.Load())
}

func TestBootstrapper_RunsOnceAcrossWhy(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	var stdoutBuffer bytes.Buffer

	cli := &documentFeaturesCLI{}
	cli.onBootstrap = func(*conga.Bootstrap) error {
		invocations.Add(1)

		return nil
	}

	app := bootstrapApp(cli).
		EnableFeatures().
		EnableWhy().
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, int32(1), invocations.Load())
}

func TestBootstrapper_RunsForHelpWhenFeaturesEnabled(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	var stdoutBuffer bytes.Buffer

	cli := &documentFeaturesCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.Set("experiments", map[string]string{"force-all": "true"}, "taskrc")
	})}
	cli.onBootstrap = func(handle *conga.Bootstrap) error {
		invocations.Add(1)
		handle.SetConfig(cli.doc)

		return nil
	}

	app := bootstrapApp(cli).
		EnableFeatures().
		Stdout(&stdoutBuffer).
		Args([]string{"--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, int32(1), invocations.Load())
	assert.Contains(t, stdoutBuffer.String(), "--force-all")
}

func TestBootstrapper_HelpWithoutFeaturesSkipsBootstrap(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	var stdoutBuffer bytes.Buffer

	cli := &documentBasicCLI{}
	cli.onBootstrap = func(*conga.Bootstrap) error {
		invocations.Add(1)

		return nil
	}

	app := bootstrapApp(cli).
		Stdout(&stdoutBuffer).
		Args([]string{"--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, int32(0), invocations.Load())
}

func TestBootstrapper_VersionSkipsBootstrap(t *testing.T) {
	t.Parallel()

	var invocations atomic.Int32

	cli := &documentFeaturesCLI{}
	cli.onBootstrap = func(*conga.Bootstrap) error {
		invocations.Add(1)

		return nil
	}

	app := bootstrapApp(cli).
		EnableFeatures().
		Args([]string{"--version"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, int32(0), invocations.Load())
}

func TestConfigDocument_SetOnNilPanics(t *testing.T) {
	t.Parallel()

	var document *conga.ConfigDocument

	assertx.PanicsWithValue(t, "conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)", func() {
		document.Set("host", "value", "origin")
	})
}

func TestConfigDocument_RepeatedSetLastValueWins(t *testing.T) {
	t.Parallel()

	cli := &documentBasicCLI{doc: documentWith(
		func(document *conga.ConfigDocument) {
			document.Set("host", "first.example.com", "first")
		},
		func(document *conga.ConfigDocument) {
			document.Set("host", "second.example.com", "second")
		},
	)}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "second.example.com", cli.Host.Get())
}

func TestConfigDocument_SetKeyRecordsMapEntry(t *testing.T) {
	t.Parallel()

	cli := &documentMapCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.SetKey("headers", "region", "eu", "taskrc")
	})}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"region": "eu"}, cli.Headers.Get())
	assert.Equal(t, conga.SourceConfig, cli.Headers.Source())
}

func TestConfigDocument_SetKeyAcceptsTypedValue(t *testing.T) {
	t.Parallel()

	cli := &documentFeaturesCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.SetKey("experiments", "force-all", true, "taskrc")
	})}

	app := bootstrapApp(cli).
		EnableFeatures().
		Args([]string{"--force-all"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Features.Get().ForceAll)
	assert.True(t, cli.ForceAll.Get())
}

func TestConfigDocument_SetKeyOverlaysEarlierSet(t *testing.T) {
	t.Parallel()

	cli := &documentMapCLI{doc: documentWith(
		func(document *conga.ConfigDocument) {
			document.Set("headers", map[string]string{"region": "eu", "shared": "base"}, "base")
		},
		func(document *conga.ConfigDocument) {
			document.SetKey("headers", "shared", "overlay", "workspace")
		},
	)}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"region": "eu", "shared": "overlay"}, cli.Headers.Get())
}

func TestConfigDocument_SetReplacesEarlierSetKeyOverlays(t *testing.T) {
	t.Parallel()

	cli := &documentMapCLI{doc: documentWith(
		func(document *conga.ConfigDocument) {
			document.SetKey("headers", "region", "eu", "overlay")
		},
		func(document *conga.ConfigDocument) {
			document.Set("headers", map[string]string{"team": "core"}, "replacement")
		},
	)}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"team": "core"}, cli.Headers.Get())
}

func TestConfigDocument_RepeatedSetKeyLastValueWins(t *testing.T) {
	t.Parallel()

	cli := &documentMapCLI{doc: documentWith(
		func(document *conga.ConfigDocument) {
			document.SetKey("headers", "region", "eu", "first")
		},
		func(document *conga.ConfigDocument) {
			document.SetKey("headers", "region", "us", "second")
		},
	)}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"region": "us"}, cli.Headers.Get())
}

func TestConfigDocument_SetKeyOnNilPanics(t *testing.T) {
	t.Parallel()

	var document *conga.ConfigDocument

	assertx.PanicsWithValue(t, "conga: nil ConfigDocument; use the zero value (var document conga.ConfigDocument)", func() {
		document.SetKey("headers", "region", "eu", "origin")
	})
}

func TestConfigDocument_SetKeyUnknownKeyFailsUnderErrConfig(t *testing.T) {
	t.Parallel()

	cli := &documentMapCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.SetKey("header", "region", "eu", "taskrc")
	})}

	app := bootstrapApp(cli).
		Name("app").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Contains(t, err.Error(), `unknown section "header" in command "app"`)
	assert.Contains(t, err.Error(), `did you mean "headers"?`)
}

func TestBootstrapper_ContextCommandPath(t *testing.T) {
	t.Parallel()

	var commandPath string

	cli := &documentSubcommandCLI{}
	cli.onBootstrap = func(handle *conga.Bootstrap) error {
		commandPath = handle.Path()

		return nil
	}

	app := bootstrapApp(cli).
		Name("app").
		Args([]string{"deploy"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "app deploy", commandPath)
}

func TestBootstrapper_ContextLookupEnv(t *testing.T) {
	t.Setenv("CONGA_TEST_LOADER_CONTEXT_KEY", "context-value")

	var (
		value   string
		found   bool
		missing bool
	)

	cli := &documentBasicCLI{}
	cli.onBootstrap = func(handle *conga.Bootstrap) error {
		value, found = handle.LookupEnv("CONGA_TEST_LOADER_CONTEXT_KEY")
		_, missing = handle.LookupEnv("CONGA_TEST_LOADER_CONTEXT_MISSING")

		return nil
	}

	app := bootstrapApp(cli).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "context-value", value)
	assert.True(t, found)
	assert.False(t, missing)
}

func TestConfigDocument_SetKeyOnScalarFlagReportsNotAMap(t *testing.T) {
	t.Parallel()

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.SetKey("port", "http", 8080, "taskrc")
	})}

	app := bootstrapApp(cli).
		Name("app").
		Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrConfig)
	assert.Equal(t, `config file "taskrc": field "port" in command "app" is not a map`, err.Error())
}

func TestBootstrapper_ScopedOriginWithSlashAndCollidingBasenames(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
		document.SetScoped("host", "prod.example.com", "/srv/prod/pipelines/pipeline.yaml", "jobs/deploy")
		document.SetScoped("port", 9200, "/srv/dev/pipelines/pipeline.yaml", "jobs/release")
	})}

	app := bootstrapApp(cli).
		Name("app").
		EnableWhy().
		Stdout(&stdoutBuffer).
		Args([]string{"--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "/srv/prod/pipelines/pipeline.yaml › jobs/deploy", cli.Host.SourceDetail())
	assert.Equal(t, "/srv/dev/pipelines/pipeline.yaml › jobs/release", cli.Port.SourceDetail())

	output := stdoutBuffer.String()

	assert.Contains(t, output, "config (/srv/prod/.../pipeline.yaml › jobs/deploy)")
	assert.Contains(t, output, "config (/srv/dev/.../pipeline.yaml › jobs/release)")
	assert.NotContains(t, output, "config (deploy)")
	assert.NotContains(t, output, "config (release)")
	assert.Contains(t, output, "/srv/prod/pipelines/pipeline.yaml")
	assert.Contains(t, output, "/srv/dev/pipelines/pipeline.yaml")
}
