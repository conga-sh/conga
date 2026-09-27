package conga

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

func TestAppConfig_clone_CopiesAllFields(t *testing.T) {
	t.Parallel()

	app := New[appConfigCloneCLI]()
	app.name = "clone-app"
	app.description = "description"
	app.version = "v1.2.3"
	app.envPrefix = "CLONE_"
	app.configSettings = spec.ConfigSettings{Explicit: "config.json", Tier: []string{"a.json", "b.json"}, Cascading: true}
	app.configOptions = []config.Option{config.Paths("a.json", "b.json")}
	app.RegisterDecoder(".json", func([]byte) (map[string]any, error) { return map[string]any{}, nil })
	app.Dotenv(".env", ".env.local")
	app.configData["port"] = "9000"
	app.args = []string{"--port", "9000"}
	app.argsExplicitlySet = true
	app.usageMode = UsageFull
	app.stdout = &bytes.Buffer{}
	app.stderr = &bytes.Buffer{}
	app.exitFunc = func(code int) {}
	app.strictTagValidation = false
	app.allowedTags = []string{"custom-tag"}
	app.predictors["branches"] = predict.Values("main", "dev")
	app.EnableCompletion()
	app.examples = "app deploy"
	app.footer = "footer text"
	app.deprecated = "deprecated notice"

	cloned := app.clone()

	assert.Equal(t, app.name, cloned.name)
	assert.Equal(t, app.description, cloned.description)
	assert.Equal(t, app.version, cloned.version)
	assert.Equal(t, app.envPrefix, cloned.envPrefix)
	assert.Equal(t, app.configSettings, cloned.configSettings)
	assert.Equal(t, 1, len(cloned.configOptions))
	assert.NotZero(t, cloned.configRegistry.Decoder(".json"))
	assert.True(t, cloned.configEngine != nil)
	assert.True(t, cloned.dotenvLoader != nil)
	assert.True(t, cloned.dotenvOverloader != nil)
	assert.Equal(t, app.dotenvPaths, cloned.dotenvPaths)
	assert.Equal(t, app.configData, cloned.configData)
	assert.Equal(t, app.args, cloned.args)
	assert.Equal(t, app.argsExplicitlySet, cloned.argsExplicitlySet)
	assert.Equal(t, app.usageMode, cloned.usageMode)
	assert.Equal(t, app.stdout, cloned.stdout)
	assert.Equal(t, app.stderr, cloned.stderr)
	assert.NotZero(t, cloned.exitFunc)
	assert.Equal(t, app.strictTagValidation, cloned.strictTagValidation)
	assert.Equal(t, app.allowedTags, cloned.allowedTags)
	assert.Equal(t, 1, len(cloned.predictors))
	assert.NotZero(t, cloned.predictors["branches"])
	assert.True(t, cloned.defaultPredictors != nil)
	assert.True(t, cloned.mountCompletion != nil)
	assert.True(t, cloned.handleCompletionInvocation != nil)
	assert.True(t, cloned.completionHelp != nil)
	assert.True(t, cloned.renderCompletionScript != nil)
	assert.Equal(t, app.completionShells, cloned.completionShells)
	assert.Equal(t, app.examples, cloned.examples)
	assert.Equal(t, app.footer, cloned.footer)
	assert.Equal(t, app.deprecated, cloned.deprecated)
}

func TestAppConfig_clone_DeepCopiesMapsAndSlices(t *testing.T) {
	t.Parallel()

	app := New[appConfigCloneCLI]()
	app.configSettings = spec.ConfigSettings{Tier: []string{"a.json", "b.json"}}
	app.configOptions = []config.Option{config.Paths("options.json")}
	app.RegisterDecoder(".json", func([]byte) (map[string]any, error) { return map[string]any{}, nil })
	app.dotenvPaths = []string{".env"}
	app.configData["port"] = "9000"
	app.args = []string{"--port", "9000"}
	app.allowedTags = []string{"custom-tag"}
	app.predictors["branches"] = predict.Values("main")

	cloned := app.clone()
	cloned.configSettings.Tier[0] = "mutated.json"
	cloned.configOptions[0] = config.Explicit("mutated.json")
	cloned.configRegistry.Register(".toml", func([]byte) (map[string]any, error) { return map[string]any{}, nil })
	cloned.dotenvPaths[0] = "mutated.env"
	cloned.configData["port"] = "1"
	cloned.args[0] = "--host"
	cloned.allowedTags[0] = "mutated-tag"
	cloned.predictors["other"] = predict.Values("x")

	assert.Equal(t, []string{"a.json", "b.json"}, app.configSettings.Tier)
	assert.Equal(t, []string{"options.json"}, config.Apply(app.configOptions...).Tier)
	assert.NotZero(t, app.configRegistry.Decoder(".json"))
	assert.Zero(t, app.configRegistry.Decoder(".toml"))
	assert.Equal(t, []string{".env"}, app.dotenvPaths)
	assert.Equal(t, map[string]string{"port": "9000"}, app.configData)
	assert.Equal(t, []string{"--port", "9000"}, app.args)
	assert.Equal(t, []string{"custom-tag"}, app.allowedTags)
	assert.Equal(t, 1, len(app.predictors))
	assert.NotZero(t, app.predictors["branches"])
}

func TestAppConfig_ConfigHooks_NilByDefault(t *testing.T) {
	t.Parallel()

	app := New[appConfigCloneCLI]()

	assert.True(t, app.dotenvLoader == nil)
	assert.True(t, app.dotenvOverloader == nil)
	assert.True(t, app.configEngine == nil)
	assert.True(t, app.whyRenderer == nil)
	assert.True(t, app.defaultPredictors == nil)
	assert.True(t, app.mountCompletion == nil)
	assert.True(t, app.handleCompletionInvocation == nil)
	assert.True(t, app.completionHelp == nil)
	assert.True(t, app.renderCompletionScript == nil)
	assert.True(t, app.completionShells == nil)
}

func TestAppConfig_clone_ResetsDeprecationNotices(t *testing.T) {
	t.Parallel()

	app := New[appConfigCloneCLI]()
	app.deprecationNotices = []string{"WARNING: stale"}

	cloned := app.clone()

	assert.Zero(t, cloned.deprecationNotices)
	assert.Equal(t, []string{"WARNING: stale"}, app.deprecationNotices)
}

func TestAppConfig_ConfigOptions_AccumulatesSlots(t *testing.T) {
	t.Parallel()

	app := New[appConfigCloneCLI]().
		ConfigOptions(config.Paths("a.json", "b.json")).
		ConfigOptions(config.Explicit("override.json"), config.Cascading())

	assert.Equal(t, 3, len(app.configOptions))
	assert.Equal(t, spec.ConfigSettings{
		Explicit:  "override.json",
		Tier:      []string{"a.json", "b.json"},
		Cascading: true,
	}, app.configSettings)
}

func TestAppConfig_ConfigOptions_RepeatedPaths_LastDeclarationWins(t *testing.T) {
	t.Parallel()

	app := New[appConfigCloneCLI]().
		ConfigOptions(config.Paths("first.json")).
		ConfigOptions(config.Paths("second.json"))

	assert.Equal(t, []string{"second.json"}, app.configSettings.Tier)
}

func TestReloader_ErrorParityWithRunContext(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	app := New[reloaderParityCLI]().Args([]string{"--host", "localhost", "--addr", "127.0.0.1"})

	runErr := app.RunContext(ctx)
	reloader := app.buildReloadFunc()
	_, reloadErr := reloader(ctx)

	assert.Error(t, runErr)
	assert.Error(t, reloadErr)
	assert.IsError(t, runErr, ErrValidation)
	assert.IsError(t, reloadErr, ErrValidation)
	assert.Equal(t, runErr.Error(), reloadErr.Error())
}

func TestReloader_DoesNotMutateOriginalCompletionPredictors(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	app := New[reloadFeatureCLI]().
		EnableCompletion().
		Args([]string{"--host", "localhost"})

	delete(app.predictors, "shell")

	_, err := app.buildReloadFunc()(ctx)

	assert.NoError(t, err)

	_, originalHasShell := app.predictors["shell"]
	assert.False(t, originalHasShell)
}

func TestApp_ClonedWhyRenderer_DispatchesToPassedApp(t *testing.T) {
	t.Parallel()

	original := New[reloaderParityCLI]().EnableWhy()

	var originalBuffer bytes.Buffer

	original.stdout = &originalBuffer

	fresh := &App[reloaderParityCLI]{appConfig: original.clone()}

	var freshBuffer bytes.Buffer

	fresh.stdout = &freshBuffer

	cmd, err := fresh.buildSpec()
	assert.NoError(t, err)

	fresh.whyRenderer(fresh, []*spec.Command{cmd}, nil)

	assert.Equal(t, "", originalBuffer.String())
	assert.NotEqual(t, "", freshBuffer.String())
}

func TestValidator_HasStreamsAndReloaderContext(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	var (
		stdoutBuffer bytes.Buffer
		stderrBuffer bytes.Buffer
		stdinBuffer  bytes.Buffer
	)

	app := New[validatorContextCLI]().
		Args([]string{}).
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer).
		Stdin(&stdinBuffer).
		ConfigOptions(config.Paths("test-config.json"))

	runErr := app.RunContext(ctx)
	reloader := app.buildReloadFunc()
	_, reloadErr := reloader(ctx)

	assert.NoError(t, runErr)
	assert.NoError(t, reloadErr)
}

func TestApp_initDefaultPredictors_RegistersShellPredictor(t *testing.T) {
	t.Parallel()

	app := New[appConfigCloneCLI]()

	app.EnableCompletion()
	app.initDefaultPredictors()
	pred, ok := app.predictors["shell"]

	assert.True(t, ok)

	expectedRaw := slicex.Map(completion.ShellDescriptions(), func(d completion.ShellInfo) string {
		return d.Name
	})
	assert.Equal(t, expectedRaw, pred.Raw())
}

func TestApp_initDefaultPredictors_PreservesCustomShellPredictor(t *testing.T) {
	t.Parallel()

	app := New[appConfigCloneCLI]()
	app.EnableCompletion()

	custom := predict.Values("custom-shell")
	app.predictors["shell"] = custom

	app.initDefaultPredictors()

	assert.Equal(t, []string{"custom-shell"}, app.predictors["shell"].Raw())
}

func TestApp_UsageOnError_SetsUsageSilenceMode(t *testing.T) {
	t.Parallel()

	app := New[appConfigCloneCLI]()
	assert.Equal(t, UsageSummary, app.usageMode)

	app.UsageOnError(UsageSilence)

	assert.Equal(t, UsageSilence, app.usageMode)
}

type appConfigCloneCLI struct{}

type reloaderParityCLI struct {
	Port OptionalFlag[int]    `name:"port"`
	Host OptionalFlag[string] `name:"host" xor:"endpoint"`
	Addr OptionalFlag[string] `name:"addr" xor:"endpoint"`
}

func (c *reloaderParityCLI) Run() error {
	return nil
}

type reloadFeatureCLI struct {
	Host OptionalFlag[string] `name:"host"`
}

func (c *reloadFeatureCLI) Run(ctx context.Context, session *Session) error {
	return nil
}

type validatorContextCLI struct {
	Port OptionalFlag[int] `name:"port"`
}

func (c *validatorContextCLI) Validate(ctx context.Context, session *Session) error {
	candidates := session.CandidateConfigFiles()
	if len(candidates) != 1 || candidates[0] != "test-config.json" {
		return fmt.Errorf("unexpected candidate config files: %v", candidates)
	}

	if session.Stdout() == os.Stdout {
		return errors.New("stdout in validator context fell back to os.Stdout")
	}

	if session.Stderr() == os.Stderr {
		return errors.New("stderr in validator context fell back to os.Stderr")
	}

	if session.Stdin() == os.Stdin {
		return errors.New("stdin in validator context fell back to os.Stdin")
	}

	return nil
}

func (c *validatorContextCLI) Run(_ context.Context, _ *Session) error {
	return nil
}

func TestApp_Fluent_Examples(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	got := app.Examples("app deploy --image nginx:alpine")

	assert.True(t, got == app)
	assert.Equal(t, "app deploy --image nginx:alpine", app.examples)
}

func TestApp_Fluent_Footer(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	got := app.Footer("Documentation: https://example.com/docs")

	assert.True(t, got == app)
	assert.Equal(t, "Documentation: https://example.com/docs", app.footer)
}

func TestApp_Fluent_Deprecated(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	got := app.Deprecated("use the new unified tool")

	assert.True(t, got == app)
	assert.Equal(t, "use the new unified tool", app.deprecated)
}

func TestApp_Fluent_Stdin(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	assert.True(t, app.stdin == io.Reader(os.Stdin))

	got := app.Stdin(strings.NewReader("payload"))

	assert.True(t, got == app)
	assert.False(t, app.stdin == io.Reader(os.Stdin))

	app.Stdin(nil)

	assert.False(t, app.stdin == io.Reader(os.Stdin))
}

type validatorFailingCLI struct {
	Port OptionalFlag[int] `name:"port"`
}

func (c *validatorFailingCLI) Validate(_ context.Context, _ *Session) error {
	return errors.New("custom validator failure")
}

func (c *validatorFailingCLI) Run(_ context.Context, _ *Session) error {
	return nil
}

func TestValidator_FailureSurfacesFromRun(t *testing.T) {
	t.Parallel()

	app := New[validatorFailingCLI]().Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, ErrValidation)
	assert.Equal(t, "custom validator failure", err.Error())
}
