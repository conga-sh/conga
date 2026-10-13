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

	"github.com/conga-sh/conga/internal/completion"
	"github.com/conga-sh/conga/internal/slicex"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

func TestAppConfig_ConfigHooks_NilByDefault(t *testing.T) {
	t.Parallel()

	app := New[emptyCLI]()

	assert.True(t, app.dotenvLoader == nil)
	assert.True(t, app.configEngine == nil)
	assert.True(t, app.whyRenderer == nil)
	assert.True(t, app.defaultPredictors == nil)
	assert.True(t, app.mountCompletion == nil)
	assert.True(t, app.handleCompletionInvocation == nil)
	assert.True(t, app.completionHelp == nil)
	assert.True(t, app.renderCompletionScript == nil)
	assert.True(t, app.completionShells == nil)
}

func TestAppConfig_ConfigOptions_AccumulatesSlots(t *testing.T) {
	t.Parallel()

	app := New[emptyCLI]().
		ConfigOptions(ConfigPaths("a.json", "b.json")).
		ConfigOptions(ConfigExplicit("override.json"), ConfigCascading())

	assert.Equal(t, 3, len(app.configOptions))
	assert.Equal(t, spec.ConfigSettings{
		Explicit:  "override.json",
		Tier:      []string{"a.json", "b.json"},
		Cascading: true,
	}, app.configSettings)
}

func TestAppConfig_ConfigOptions_RepeatedPaths_LastDeclarationWins(t *testing.T) {
	t.Parallel()

	app := New[emptyCLI]().
		ConfigOptions(ConfigPaths("first.json")).
		ConfigOptions(ConfigPaths("second.json"))

	assert.Equal(t, []string{"second.json"}, app.configSettings.Tier)
}

func TestValidator_HasStreamsAndConfigContext(t *testing.T) {
	t.Parallel()

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
		ConfigOptions(ConfigPaths("test-config.json"))

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
}

func TestValidator_FeatureCapable_HasConfigContext(t *testing.T) {
	t.Parallel()

	app := New[featureValidatorCLI]().
		Args([]string{}).
		ConfigOptions(ConfigPaths("test-config.json"))

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
}

func TestApp_initDefaultPredictors_RegistersShellPredictor(t *testing.T) {
	t.Parallel()

	app := New[emptyCLI]()

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

	app := New[emptyCLI]()
	app.EnableCompletion()

	custom := predict.Values("custom-shell")
	app.predictors["shell"] = custom

	app.initDefaultPredictors()

	assert.Equal(t, []string{"custom-shell"}, app.predictors["shell"].Raw())
}

func TestApp_UsageOnError_SetsUsageSilenceMode(t *testing.T) {
	t.Parallel()

	app := New[emptyCLI]()
	assert.Equal(t, UsageSummary, app.usageMode)

	app.UsageOnError(UsageSilence)

	assert.Equal(t, UsageSilence, app.usageMode)
}

type emptyCLI struct{}

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

type featureValidatorSet struct {
	Enabled bool
}

type featureValidatorCLI struct {
	Features Features[featureValidatorSet] `flag:"feature-gates"`
}

func (c *featureValidatorCLI) Validate(_ context.Context, session *Session) error {
	candidates := session.CandidateConfigFiles()
	if len(candidates) != 1 || candidates[0] != "test-config.json" {
		return fmt.Errorf("unexpected candidate config files: %v", candidates)
	}

	return nil
}

func (c *featureValidatorCLI) Run(_ context.Context, _ *Session) error {
	return nil
}

func TestApp_Fluent_Examples(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	got := app.Examples("app deploy --image nginx:alpine")

	assert.True(t, got == app)
	assert.Equal(t, "app deploy --image nginx:alpine", app.examples)
}

func TestApp_Fluent_HelpFooter(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	got := app.HelpFooter("Documentation: https://example.com/docs")

	assert.True(t, got == app)
	assert.Equal(t, "Documentation: https://example.com/docs", app.helpFooter)
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

// TestPrunedFlag_ConfigKeys pins the configuration-key mapping against
// lookupConfig's read tiers: root-owned pruned flags claim the bare tiers,
// subcommand-owned pruned flags claim only command-scoped tiers so a valid
// top-level entry an active flag consumes is never rejected.
func TestPrunedFlag_ConfigKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prefix   string
		flagName string
		config   string
		want     []string
	}{
		{name: "root flag name only", flagName: "wait", want: []string{"wait"}},
		{name: "root flag with config key", flagName: "wait", config: "timeout", want: []string{"wait", "timeout"}},
		{name: "root flag excluded from config", flagName: "wait", config: "-", want: nil},
		{name: "subcommand flag name only", prefix: "admin", flagName: "beta", want: []string{"admin.beta"}},
		{name: "nested subcommand flag name only", prefix: "cluster.node", flagName: "wait", want: []string{"cluster.node.wait", "node.wait"}},
		{name: "subcommand flag with config key", prefix: "admin", flagName: "beta", config: "beta-key", want: []string{"admin.beta-key", "admin.beta"}},
		{name: "nested subcommand flag with config key", prefix: "cluster.node", flagName: "wait", config: "wait-key", want: []string{"cluster.node.wait-key", "cluster.node.wait", "node.wait"}},
		{name: "subcommand flag with qualified config key", prefix: "cluster.node", flagName: "wait", config: "cluster.node.wait-key", want: []string{"cluster.node.wait-key", "cluster.node.wait", "node.wait"}},
		{name: "subcommand flag excluded from config", prefix: "admin", flagName: "beta", config: "-", want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pruned := prunedFlag{
				flag:   &spec.Flag{Name: test.flagName, ConfigKey: test.config},
				prefix: test.prefix,
			}

			got := pruned.configKeys()

			assert.Equal(t, test.want, got)
		})
	}
}
