package conga

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
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
	assert.True(t, app.setenvFunc == nil)
	assert.True(t, app.configEngine == nil)
	assert.True(t, app.whyHooks == nil)
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

func TestValidator_HasStreamsAndConfigPaths(t *testing.T) {
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

func TestValidator_FeatureCapable_HasConfigPaths(t *testing.T) {
	t.Parallel()

	app := New[featureValidatorCLI]().
		EnableFeatures().
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

	app.predictors["shell"] = predict.Values("custom-shell")

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

	assert.True(t, app.Examples("app deploy --image nginx:alpine") == app)
	assert.Equal(t, "app deploy --image nginx:alpine", app.examples)
}

func TestApp_Fluent_HelpFooter(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	assert.True(t, app.HelpFooter("Documentation: https://example.com/docs") == app)
	assert.Equal(t, "Documentation: https://example.com/docs", app.helpFooter)
}

func TestApp_Fluent_Deprecated(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	assert.True(t, app.Deprecated("use the new unified tool") == app)
	assert.Equal(t, "use the new unified tool", app.deprecated)
}

func TestApp_Fluent_Stdin(t *testing.T) {
	t.Parallel()

	app := New[struct{}]()

	assert.True(t, app.stdin == io.Reader(os.Stdin))

	assert.True(t, app.Stdin(strings.NewReader("payload")) == app)
	assert.False(t, app.stdin == io.Reader(os.Stdin))

	app.Stdin(nil)

	assert.False(t, app.stdin == io.Reader(os.Stdin))
}

func TestApp_StdoutNil_IsNoOp(t *testing.T) {
	t.Parallel()

	t.Run("defaults to os.Stdout", func(t *testing.T) {
		t.Parallel()

		app := New[struct{}]()

		assert.True(t, app.stdout == io.Writer(os.Stdout))
	})

	t.Run("sets non-nil writer", func(t *testing.T) {
		t.Parallel()

		app := New[struct{}]()

		writer := &bytes.Buffer{}

		assert.True(t, app.Stdout(writer) == app)
		assert.True(t, app.stdout == io.Writer(writer))
	})

	t.Run("nil is no-op", func(t *testing.T) {
		t.Parallel()

		app := New[struct{}]()
		writer := &bytes.Buffer{}
		app.Stdout(writer)

		app.Stdout(nil)

		assert.True(t, app.stdout == io.Writer(writer))
	})
}

func TestApp_StderrNil_IsNoOp(t *testing.T) {
	t.Parallel()

	t.Run("defaults to os.Stderr", func(t *testing.T) {
		t.Parallel()

		app := New[struct{}]()

		assert.True(t, app.stderr == io.Writer(os.Stderr))
	})

	t.Run("sets non-nil writer", func(t *testing.T) {
		t.Parallel()

		app := New[struct{}]()

		writer := &bytes.Buffer{}

		assert.True(t, app.Stderr(writer) == app)
		assert.True(t, app.stderr == io.Writer(writer))
	})

	t.Run("nil is no-op", func(t *testing.T) {
		t.Parallel()

		app := New[struct{}]()
		writer := &bytes.Buffer{}
		app.Stderr(writer)

		app.Stderr(nil)

		assert.True(t, app.stderr == io.Writer(writer))
	})
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
	assert.Equal(t, "custom validator failure", err.Error())
	assert.Equal(t, nil, errors.Unwrap(err))
	_, isUsage := errors.AsType[*UsageError](err)
	assert.False(t, isUsage)
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

			assert.Equal(t, test.want, pruned.configKeys())
		})
	}
}

type signalTestExitCodeError struct {
	code    int
	message string
	cause   error
}

func (e *signalTestExitCodeError) Error() string { return e.message }
func (e *signalTestExitCodeError) ExitCode() int { return e.code }
func (e *signalTestExitCodeError) Unwrap() error { return e.cause }

func TestApp_CaughtSignal_ExitMatrix(t *testing.T) {
	t.Parallel()

	type childCase struct {
		name           string
		script         string
		caughtSignal   syscall.Signal
		expectedCode   int
		expectedStderr string
	}

	usageErr := usageError([]*spec.Command{{Name: "app"}}, "bad usage")

	cases := []struct {
		name           string
		caughtSignal   syscall.Signal
		err            error
		expectedCode   int
		expectedStderr string
		expectedBlock  string
		memberCount    int
	}{
		{
			name:         "nil error after signal exits 0 silently",
			caughtSignal: syscall.SIGINT,
			err:          nil,
			expectedCode: 0,
		},
		{
			name:         "bare context.Canceled after SIGINT",
			caughtSignal: syscall.SIGINT,
			err:          context.Canceled,
			expectedCode: 130,
			memberCount:  1,
		},
		{
			name:         "wrapped context.Canceled after SIGTERM",
			caughtSignal: syscall.SIGTERM,
			err:          fmt.Errorf("rpc call: %w", context.Canceled),
			expectedCode: 143,
			memberCount:  1,
		},
		{
			name:           "joined context.Canceled after SIGHUP prints non-canceled member",
			caughtSignal:   syscall.SIGHUP,
			err:            errors.Join(errors.New("step failed"), context.Canceled),
			expectedCode:   129,
			expectedStderr: "app: error: step failed\n",
			memberCount:    2,
		},
		{
			name:         "joined bare and wrapped context.Canceled after SIGHUP is silent",
			caughtSignal: syscall.SIGHUP,
			err:          errors.Join(context.Canceled, fmt.Errorf("wrapped: %w", context.Canceled)),
			expectedCode: 129,
			memberCount:  2,
		},
		{
			name:         "joined finallyError with context.Canceled primary and other failure omits cancel",
			caughtSignal: syscall.SIGINT,
			err: errors.Join(
				&finallyError{
					primary:    context.Canceled,
					suppressed: []error{errors.New("cleanup failed")},
				},
				errors.New("other failure"),
			),
			expectedCode:   130,
			expectedStderr: "app: error: cleanup failed\napp: error: other failure\n",
			memberCount:    3,
		},
		{
			name:           "joined context.Canceled and author ExitCoder prints both and uses author code",
			caughtSignal:   syscall.SIGINT,
			err:            errors.Join(context.Canceled, &signalTestExitCodeError{code: 75, message: "author failed"}),
			expectedCode:   75,
			expectedStderr: "app: error: context canceled\napp: error: author failed\n",
			memberCount:    2,
		},
		{
			name:         "finallyError with context.Canceled primary and suppressed Finally errors",
			caughtSignal: syscall.SIGINT,
			err: &finallyError{
				primary:    context.Canceled,
				suppressed: []error{errors.New("close db failed"), errors.New("remove lock failed")},
			},
			expectedCode:   130,
			expectedStderr: "app: error: close db failed\napp: error: remove lock failed\n",
			memberCount:    3,
		},
		{
			name:         "nested finallyError with context.Canceled primary prints inner and outer suppressed errors",
			caughtSignal: syscall.SIGINT,
			err: &finallyError{
				primary: &finallyError{
					primary:    context.Canceled,
					suppressed: []error{errors.New("inner cleanup failed")},
				},
				suppressed: []error{errors.New("outer cleanup failed")},
			},
			expectedCode:   130,
			expectedStderr: "app: error: inner cleanup failed\napp: error: outer cleanup failed\n",
			memberCount:    3,
		},
		{
			name:           "non-canceled error after SIGINT prints and exits 130",
			caughtSignal:   syscall.SIGINT,
			err:            errors.New("rpc error: code = Canceled desc = context canceled"),
			expectedCode:   130,
			expectedStderr: "app: error: rpc error: code = Canceled desc = context canceled\n",
			memberCount:    1,
		},
		{
			name:           "author ExitCoder wins over caught signal",
			caughtSignal:   syscall.SIGINT,
			err:            &signalTestExitCodeError{code: 75, message: "author failed"},
			expectedCode:   75,
			expectedStderr: "app: error: author failed\n",
			memberCount:    1,
		},
		{
			name:           "author ExitCoder with out-of-range code 0 exits 1 and prints",
			caughtSignal:   syscall.SIGINT,
			err:            &signalTestExitCodeError{code: 0, message: "zero code"},
			expectedCode:   1,
			expectedStderr: "app: error: zero code\n",
			memberCount:    1,
		},
		{
			name:           "author ExitCoder with out-of-range code 256 wrapping context.Canceled exits 1 and prints",
			caughtSignal:   syscall.SIGINT,
			err:            &signalTestExitCodeError{code: 256, message: "wrapped cancel", cause: context.Canceled},
			expectedCode:   1,
			expectedStderr: "app: error: wrapped cancel\n",
			memberCount:    1,
		},
		{
			name:           "UsageError after caught signal prints without usage block and exits 130",
			caughtSignal:   syscall.SIGINT,
			err:            usageErr,
			expectedCode:   130,
			expectedStderr: "app: error: bad usage\n",
			expectedBlock:  "",
			memberCount:    1,
		},
		{
			name:           "RunContext cancellation without caught signal prints and exits 1",
			caughtSignal:   0,
			err:            context.Canceled,
			expectedCode:   1,
			expectedStderr: "app: error: context canceled\n",
			memberCount:    1,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			app, stderrBuffer, recordedCode := newSignalExitApp(t, testCase.caughtSignal)

			app.Exit(testCase.err)

			assert.Equal(t, testCase.expectedCode, *recordedCode)
			assert.Equal(t, testCase.expectedCode, app.ExitCode(testCase.err))
			assert.Equal(t, testCase.expectedBlock, app.UsageBlock(testCase.err))
			assert.Equal(t, testCase.expectedStderr, stderrBuffer.String())
			assert.Equal(t, testCase.memberCount, len(DiagnosticOf(testCase.err).Members()))
		})
	}

	t.Run("child exec.ExitError", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("shell signal children are Unix-specific")
		}

		childCases := []childCase{
			{
				name:           "signalled child after SIGINT is silent and exits 130",
				script:         "kill -KILL $$",
				caughtSignal:   syscall.SIGINT,
				expectedCode:   130,
				expectedStderr: "",
			},
			{
				name:           "child exiting 130 after SIGINT is silent and exits 130",
				script:         "exit 130",
				caughtSignal:   syscall.SIGINT,
				expectedCode:   130,
				expectedStderr: "",
			},
			{
				name:           "child exiting 1 after SIGINT prints and exits 130",
				script:         "exit 1",
				caughtSignal:   syscall.SIGINT,
				expectedCode:   130,
				expectedStderr: "app: error: exit status 1\n",
			},
			{
				name:           "child exiting 143 after SIGINT prints and exits 130",
				script:         "exit 143",
				caughtSignal:   syscall.SIGINT,
				expectedCode:   130,
				expectedStderr: "app: error: exit status 143\n",
			},
			{
				name:           "child exiting 7 without caught signal prints and exits 7",
				script:         "exit 7",
				caughtSignal:   0,
				expectedCode:   7,
				expectedStderr: "app: error: exit status 7\n",
			},
		}

		for _, child := range childCases {
			t.Run(child.name, func(t *testing.T) {
				t.Parallel()

				err := exec.CommandContext(t.Context(), "sh", "-c", child.script).Run()
				assert.Error(t, err)

				app, stderrBuffer, recordedCode := newSignalExitApp(t, child.caughtSignal)

				app.Exit(err)

				assert.Equal(t, child.expectedCode, *recordedCode)
				assert.Equal(t, child.expectedCode, app.ExitCode(err))
				assert.Equal(t, child.expectedStderr, stderrBuffer.String())
			})
		}

		t.Run("joined signalled child and other failure after SIGINT prints other failure and exits 130", func(t *testing.T) {
			t.Parallel()

			childErr := exec.CommandContext(t.Context(), "sh", "-c", "kill -KILL $$").Run()
			assert.Error(t, childErr)

			joined := errors.Join(childErr, errors.New("other failure"))
			app, stderrBuffer, recordedCode := newSignalExitApp(t, syscall.SIGINT)

			app.Exit(joined)

			assert.Equal(t, 130, *recordedCode)
			assert.Equal(t, 130, app.ExitCode(joined))
			assert.Equal(t, "app: error: other failure\n", stderrBuffer.String())
		})
	})
}

func newSignalExitApp(t *testing.T, sig syscall.Signal) (*App[emptyCLI], *bytes.Buffer, *int) {
	t.Helper()

	var (
		stderrBuffer bytes.Buffer
		recordedCode = -1
	)

	app := New[emptyCLI]().
		Name("app").
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { recordedCode = code })

	if sig != 0 {
		app.caughtSignal.Store(int64(sig))
	}

	return app, &stderrBuffer, &recordedCode
}
