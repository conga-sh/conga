package conga_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
)

var (
	// What App.Exit prints for a session.UsageError in each UsageOnErrorMode,
	// with a message and with an empty one.
	//go:embed testdata/errors/session_usage_default.golden
	goldenErrorsSessionUsageDefault string

	//go:embed testdata/errors/session_usage_empty_default.golden
	goldenErrorsSessionUsageEmptyDefault string

	//go:embed testdata/errors/session_usage_empty_full.golden
	goldenErrorsSessionUsageEmptyFull string

	//go:embed testdata/errors/session_usage_empty_hint.golden
	goldenErrorsSessionUsageEmptyHint string

	//go:embed testdata/errors/session_usage_full.golden
	goldenErrorsSessionUsageFull string

	//go:embed testdata/errors/session_usage_hint.golden
	goldenErrorsSessionUsageHint string

	// What App.Exit prints for a missing subcommand and an unknown flag in each
	// UsageOnErrorMode.
	//go:embed testdata/errors/shape_subcommand_required_help.golden
	goldenErrorsShapeSubcommandRequiredHelp string

	//go:embed testdata/errors/shape_subcommand_required_hint.golden
	goldenErrorsShapeSubcommandRequiredHint string

	//go:embed testdata/errors/shape_unknown_flag_full.golden
	goldenErrorsShapeUnknownFlagFull string

	//go:embed testdata/errors/shape_unknown_flag_hint.golden
	goldenErrorsShapeUnknownFlagHint string

	//go:embed testdata/errors/shape_unknown_flag_summary.golden
	goldenErrorsShapeUnknownFlagSummary string
)

// exitCodeError is an application error that chooses its own exit code.
type exitCodeError struct {
	code    int
	message string
	cause   error
}

func (e *exitCodeError) Error() string { return e.message }

func (e *exitCodeError) ExitCode() int { return e.code }

func (e *exitCodeError) Unwrap() error { return e.cause }

// exitResult is what App.Exit wrote to stderr and the code it asked for.
type exitResult struct {
	stderr string
	code   int
}

// exitFor runs App.Exit for err on an app named "app" and reports its stderr and
// exit code. It also checks that App.ExitCode returns the code App.Exit used.
func exitFor(t *testing.T, err error) exitResult {
	t.Helper()

	var (
		stderrBuffer bytes.Buffer
		exitCode     = -1
	)

	app := conga.New[noopRunner]().
		Name("app").
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code })

	app.Exit(err)

	assert.Equal(t, exitCode, app.ExitCode(err), "App.ExitCode disagrees with App.Exit")

	return exitResult{stderr: stderrBuffer.String(), code: exitCode}
}

func TestApp_Exit_ExitCoder(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		err    error
		stderr string
		code   int
	}{
		{name: "nil error", err: nil, stderr: "", code: 0},
		{name: "plain error", err: errors.New("boom"), stderr: "app: error: boom\n", code: 1},
		{name: "empty plain error prints nothing", err: errors.New(""), stderr: "", code: 1},
		{name: "custom code", err: &exitCodeError{code: 42, message: "custom failure"}, stderr: "app: error: custom failure\n", code: 42},
		{name: "lowest valid code", err: &exitCodeError{code: 1, message: "low"}, stderr: "app: error: low\n", code: 1},
		{name: "highest valid code", err: &exitCodeError{code: 255, message: "high"}, stderr: "app: error: high\n", code: 255},
		{name: "code 0 becomes 1", err: &exitCodeError{code: 0, message: "zero"}, stderr: "app: error: zero\n", code: 1},
		{name: "code -1 becomes 1", err: &exitCodeError{code: -1, message: "negative"}, stderr: "app: error: negative\n", code: 1},
		{name: "code 256 becomes 1", err: &exitCodeError{code: 256, message: "too large"}, stderr: "app: error: too large\n", code: 1},
		{name: "wrapped coder", err: fmt.Errorf("deploy: %w", &exitCodeError{code: 9, message: "inner"}), stderr: "app: error: deploy: inner\n", code: 9},
		{name: "empty message coder prints nothing", err: &exitCodeError{code: 5}, stderr: "", code: 5},
		{name: "outermost coder wins", err: &exitCodeError{code: 7, message: "outer", cause: &exitCodeError{code: 9, message: "inner"}}, stderr: "app: error: outer\n", code: 7},
		{name: "hand-built exec error without process state", err: &exec.ExitError{}, stderr: "app: error: <nil>\n", code: 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := exitFor(t, testCase.err)

			assert.Equal(t, testCase.stderr, result.stderr)
			assert.Equal(t, testCase.code, result.code)
		})
	}
}

func TestApp_ExitCode_PrintsNothing(t *testing.T) {
	t.Parallel()

	var stderrBuffer bytes.Buffer

	exited := false

	app := conga.New[noopRunner]().
		Name("app").
		Stderr(&stderrBuffer).
		ExitFunc(func(int) { exited = true })

	code := app.ExitCode(&exitCodeError{code: 42, message: "custom failure"})

	assert.Equal(t, 42, code)
	assert.Equal(t, "", stderrBuffer.String())
	assert.False(t, exited)
}

func TestApp_Exit_UsageErrorWithoutCommand(t *testing.T) {
	t.Parallel()

	t.Run("nil session prints the message and no usage block", func(t *testing.T) {
		t.Parallel()

		var session *conga.Session

		result := exitFor(t, session.UsageError("bad input"))

		assert.Equal(t, "app: error: bad input\n", result.stderr)
		assert.Equal(t, 2, result.code)
	})

	t.Run("zero value prints nothing and exits 2", func(t *testing.T) {
		t.Parallel()

		result := exitFor(t, &conga.UsageError{})

		assert.Equal(t, "", result.stderr)
		assert.Equal(t, 2, result.code)
	})
}

func TestApp_Exit_ChildProcess(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shell children are Unix-specific")
	}

	run := func(script string) error {
		return exec.CommandContext(t.Context(), "sh", "-c", script).Run()
	}

	t.Run("child exit code passes through", func(t *testing.T) {
		t.Parallel()

		err := run("exit 7")
		assert.Error(t, err)

		result := exitFor(t, err)

		assert.Equal(t, "app: error: exit status 7\n", result.stderr)
		assert.Equal(t, 7, result.code)
	})

	t.Run("wrapped child exit code passes through", func(t *testing.T) {
		t.Parallel()

		err := run("exit 7")
		assert.Error(t, err)

		result := exitFor(t, fmt.Errorf("run tool: %w", err))

		assert.Equal(t, "app: error: run tool: exit status 7\n", result.stderr)
		assert.Equal(t, 7, result.code)
	})

	t.Run("author wrapper keeps its own code", func(t *testing.T) {
		t.Parallel()

		err := run("exit 7")
		assert.Error(t, err)

		result := exitFor(t, &exitCodeError{code: 9, message: "tool failed", cause: err})

		assert.Equal(t, "app: error: tool failed\n", result.stderr)
		assert.Equal(t, 9, result.code)
	})

	t.Run("child killed by SIGSEGV exits 139", func(t *testing.T) {
		t.Parallel()

		err := run("kill -SEGV $$")
		assert.Error(t, err)

		result := exitFor(t, err)

		// The platform may append "(core dumped)" to the signal description.
		assert.Contains(t, result.stderr, "app: error: signal: segmentation fault")
		assert.Equal(t, 139, result.code)
	})

	t.Run("child killed by SIGKILL exits 137", func(t *testing.T) {
		t.Parallel()

		err := run("kill -KILL $$")
		assert.Error(t, err)

		result := exitFor(t, err)

		assert.Equal(t, "app: error: signal: killed\n", result.stderr)
		assert.Equal(t, 137, result.code)
	})

	t.Run("author wrapper over a killed child keeps its own code", func(t *testing.T) {
		t.Parallel()

		err := run("kill -KILL $$")
		assert.Error(t, err)

		result := exitFor(t, &exitCodeError{code: 9, message: "tool crashed", cause: err})

		assert.Equal(t, "app: error: tool crashed\n", result.stderr)
		assert.Equal(t, 9, result.code)
	})
}

// sessionUsageCLI is a root with one leaf whose Validate hook fails the way the
// --fail flag asks.
type sessionUsageCLI struct {
	Deploy conga.Command[sessionUsageDeploy] `summary:"Deploy a release"`
}

type sessionUsageDeploy struct {
	Fail conga.OptionalFlag[string] `help:"How Validate fails" name:"fail"`
}

var errSessionUsageBoom = errors.New("backend refused")

func (c *sessionUsageDeploy) Validate(_ context.Context, session *conga.Session) error {
	switch c.Fail.Get() {
	case "usage":
		return session.UsageError("--target is required with --env=staging")
	case "empty":
		return session.UsageError("")
	case "usagef":
		return session.UsageErrorf("cannot deploy: %w", errSessionUsageBoom)
	case "plain":
		return errSessionUsageBoom
	case "wrapped-plain":
		return fmt.Errorf("deploy check: %w", errSessionUsageBoom)
	case "coder":
		return &exitCodeError{code: 7, message: "deploy refused"}
	case "coder-over-usage":
		return &exitCodeError{code: 9, message: "deploy wrapped", cause: session.UsageError("--target is required with --env=staging")}
	case "wrapped-coder":
		return fmt.Errorf("deploy check: %w", &exitCodeError{code: 7, message: "deploy refused"})
	default:
		return nil
	}
}

func (c *sessionUsageDeploy) Run(_ context.Context, _ *conga.Session) error { return nil }

// runSessionUsage runs the deploy leaf with the given --fail mode and usage mode,
// then maps the error through App.Exit.
func runSessionUsage(t *testing.T, fail string, mode conga.UsageOnErrorMode) (exitResult, error) {
	t.Helper()

	var (
		stderrBuffer bytes.Buffer
		exitCode     = -1
	)

	app := conga.New[sessionUsageCLI]().
		Name("app").
		UsageOnError(mode).
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code }).
		Args([]string{"deploy", "--fail", fail})

	err := app.RunContext(t.Context())
	assert.Error(t, err)

	app.Exit(err)

	return exitResult{stderr: stderrBuffer.String(), code: exitCode}, err
}

func TestSession_UsageError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		fail   string
		mode   conga.UsageOnErrorMode
		stderr string
	}{
		{name: "summary", fail: "usage", mode: conga.UsageSummary, stderr: goldenErrorsSessionUsageDefault},
		{name: "full", fail: "usage", mode: conga.UsageFull, stderr: goldenErrorsSessionUsageFull},
		{name: "hint", fail: "usage", mode: conga.UsageHint, stderr: goldenErrorsSessionUsageHint},
		{name: "silence", fail: "usage", mode: conga.UsageSilence, stderr: "app: error: --target is required with --env=staging\n"},
		{name: "empty message summary", fail: "empty", mode: conga.UsageSummary, stderr: goldenErrorsSessionUsageEmptyDefault},
		{name: "empty message full", fail: "empty", mode: conga.UsageFull, stderr: goldenErrorsSessionUsageEmptyFull},
		{name: "empty message hint", fail: "empty", mode: conga.UsageHint, stderr: goldenErrorsSessionUsageEmptyHint},
		{name: "empty message silence", fail: "empty", mode: conga.UsageSilence, stderr: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := runSessionUsage(t, testCase.fail, testCase.mode)

			usage := assertUsageError(t, err)
			assert.Equal(t, "app deploy", usage.CommandPath())
			assert.Equal(t, 2, result.code)
			assert.Equal(t, testCase.stderr, result.stderr)
		})
	}
}

func TestSession_UsageErrorf(t *testing.T) {
	t.Parallel()

	result, err := runSessionUsage(t, "usagef", conga.UsageSilence)

	usage := assertUsageError(t, err)
	assert.Equal(t, "cannot deploy: backend refused", err.Error())
	assert.Equal(t, "app deploy", usage.CommandPath())
	assert.IsError(t, err, errSessionUsageBoom)
	assert.Equal(t, errSessionUsageBoom, errors.Unwrap(err))
	assert.Equal(t, "app: error: cannot deploy: backend refused\n", result.stderr)
	assert.Equal(t, 2, result.code)
}

func TestSession_UsageErrorf_WithoutWrappedOperand(t *testing.T) {
	t.Parallel()

	var session *conga.Session

	err := session.UsageErrorf("bad value %d", 7)

	usage := assertUsageError(t, err)
	assert.Equal(t, "bad value 7", err.Error())
	assert.Equal(t, "", usage.CommandPath())
	assert.Equal(t, nil, errors.Unwrap(err))
}

func TestValidate_Errors(t *testing.T) {
	t.Parallel()

	t.Run("plain error exits 1 without usage and is returned untouched", func(t *testing.T) {
		t.Parallel()

		result, err := runSessionUsage(t, "plain", conga.UsageFull)

		assertNotUsageError(t, err)
		assert.Equal(t, errSessionUsageBoom, err)
		assert.Equal(t, "app: error: backend refused\n", result.stderr)
		assert.Equal(t, 1, result.code)
	})

	t.Run("wrapped plain error exits 1 without usage", func(t *testing.T) {
		t.Parallel()

		result, err := runSessionUsage(t, "wrapped-plain", conga.UsageFull)

		assertNotUsageError(t, err)
		assert.IsError(t, err, errSessionUsageBoom)
		assert.Equal(t, "app: error: deploy check: backend refused\n", result.stderr)
		assert.Equal(t, 1, result.code)
	})

	t.Run("ExitCoder exits with its code", func(t *testing.T) {
		t.Parallel()

		result, err := runSessionUsage(t, "coder", conga.UsageFull)

		assertNotUsageError(t, err)
		assert.Equal(t, "app: error: deploy refused\n", result.stderr)
		assert.Equal(t, 7, result.code)
	})

	t.Run("an author code wrapped around a usage error wins and prints no usage block", func(t *testing.T) {
		t.Parallel()

		result, err := runSessionUsage(t, "coder-over-usage", conga.UsageFull)

		usage := assertUsageError(t, errors.Unwrap(err))
		assert.Equal(t, "app deploy", usage.CommandPath())
		assert.Equal(t, "app: error: deploy wrapped\n", result.stderr)
		assert.Equal(t, 9, result.code)
	})

	t.Run("wrapped ExitCoder exits with its code", func(t *testing.T) {
		t.Parallel()

		result, err := runSessionUsage(t, "wrapped-coder", conga.UsageFull)

		assertNotUsageError(t, err)
		assert.Equal(t, "app: error: deploy check: deploy refused\n", result.stderr)
		assert.Equal(t, 7, result.code)
	})
}

// wrapContextKey carries, in the run context, whether cancelledValidateCLI wraps
// the context's error.
type wrapContextKey struct{}

// cancelledValidateCLI fails Validate with the context's own error, wrapped
// when the run context says so.
type cancelledValidateCLI struct{}

func (c *cancelledValidateCLI) Validate(ctx context.Context, _ *conga.Session) error {
	if wrap, _ := ctx.Value(wrapContextKey{}).(bool); wrap {
		return fmt.Errorf("validate: %w", ctx.Err())
	}

	return ctx.Err()
}

func (c *cancelledValidateCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

func TestValidate_ContextErrors(t *testing.T) {
	t.Parallel()

	cancelled := func(t *testing.T) context.Context {
		t.Helper()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		return ctx
	}

	expired := func(t *testing.T) context.Context {
		t.Helper()

		ctx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 1))
		t.Cleanup(cancel)

		return ctx
	}

	testCases := []struct {
		name    string
		context func(t *testing.T) context.Context
		wrap    bool
		cause   error
		message string
	}{
		{name: "cancellation", context: cancelled, cause: context.Canceled, message: "app: error: context canceled\n"},
		{name: "wrapped cancellation", context: cancelled, wrap: true, cause: context.Canceled, message: "app: error: validate: context canceled\n"},
		{name: "deadline", context: expired, cause: context.DeadlineExceeded, message: "app: error: context deadline exceeded\n"},
		{name: "wrapped deadline", context: expired, wrap: true, cause: context.DeadlineExceeded, message: "app: error: validate: context deadline exceeded\n"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var (
				stderrBuffer bytes.Buffer
				exitCode     = -1
			)

			app := conga.New[cancelledValidateCLI]().
				Name("app").
				Stderr(&stderrBuffer).
				ExitFunc(func(code int) { exitCode = code }).
				Args([]string{})

			err := app.RunContext(context.WithValue(testCase.context(t), wrapContextKey{}, testCase.wrap))

			assertNotUsageError(t, err)
			assert.IsError(t, err, testCase.cause)

			app.Exit(err)

			assert.Equal(t, 1, exitCode)
			assert.Equal(t, testCase.message, stderrBuffer.String())
		})
	}
}

// bootstrapErrorContextKey carries, in the run context, the error
// bootstrapFailureCLI returns from Bootstrap.
type bootstrapErrorContextKey struct{}

// bootstrapFailureCLI fails Bootstrap with the error the run context carries.
type bootstrapFailureCLI struct{}

func (c *bootstrapFailureCLI) Bootstrap(ctx context.Context, _ *conga.Bootstrap) error {
	err, _ := ctx.Value(bootstrapErrorContextKey{}).(error)

	return err
}

func (c *bootstrapFailureCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

func TestBootstrap_Errors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		err    error
		stderr string
		code   int
	}{
		{name: "plain error", err: errors.New("cannot reach registry"), stderr: "app: error: cannot reach registry\n", code: 1},
		{name: "ExitCoder", err: &exitCodeError{code: 4, message: "registry unavailable"}, stderr: "app: error: registry unavailable\n", code: 4},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var (
				stderrBuffer bytes.Buffer
				exitCode     = -1
			)

			app := conga.New[bootstrapFailureCLI]().
				Name("app").
				Stderr(&stderrBuffer).
				ExitFunc(func(code int) { exitCode = code }).
				Args([]string{})

			err := app.RunContext(context.WithValue(t.Context(), bootstrapErrorContextKey{}, testCase.err))

			assertNotUsageError(t, err)
			assert.Equal(t, testCase.err, err)

			app.Exit(err)

			assert.Equal(t, testCase.code, exitCode)
			assert.Equal(t, testCase.stderr, stderrBuffer.String())
		})
	}
}

func TestBootstrap_NestedAppUsageErrorExitsTwo(t *testing.T) {
	t.Parallel()

	nested := conga.New[noopRunner]().Args([]string{"--bogus"}).RunContext(t.Context())

	var (
		stderrBuffer bytes.Buffer
		exitCode     = -1
	)

	app := conga.New[bootstrapFailureCLI]().
		Name("app").
		UsageOnError(conga.UsageSilence).
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code }).
		Args([]string{})

	err := app.RunContext(context.WithValue(t.Context(), bootstrapErrorContextKey{}, nested))

	assertUsageError(t, err)

	app.Exit(err)

	assert.Equal(t, 2, exitCode)
	assert.Equal(t, "app: error: unknown flag: --bogus\n", stderrBuffer.String())
}

// shapeCLI has a required-subcommand root and a typed flag, so one app yields
// a value error and a subcommand-required error.
type shapeCLI struct {
	Port    conga.OptionalFlag[int]           `name:"port"`
	Cluster conga.Command[SubcommandChildCLI] `name:"cluster"`
}

func TestApp_Exit_UsageShapesByMode(t *testing.T) {
	t.Parallel()

	modes := []struct {
		name        string
		mode        conga.UsageOnErrorMode
		unknownFlag string
	}{
		{name: "summary", mode: conga.UsageSummary, unknownFlag: goldenErrorsShapeUnknownFlagSummary},
		{name: "full", mode: conga.UsageFull, unknownFlag: goldenErrorsShapeUnknownFlagFull},
		{name: "hint", mode: conga.UsageHint, unknownFlag: goldenErrorsShapeUnknownFlagHint},
		{name: "silence", mode: conga.UsageSilence, unknownFlag: "app: error: unknown flag: --bogus\n"},
	}

	exitWith := func(t *testing.T, mode conga.UsageOnErrorMode, args ...string) exitResult {
		t.Helper()

		var (
			stderrBuffer bytes.Buffer
			exitCode     = -1
		)

		app := conga.New[shapeCLI]().
			Name("app").
			UsageOnError(mode).
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code }).
			Args(args)

		err := app.RunContext(t.Context())

		assertUsageError(t, err)

		app.Exit(err)

		return exitResult{stderr: stderrBuffer.String(), code: exitCode}
	}

	for _, testCase := range modes {
		t.Run("value error prints the line alone in "+testCase.name, func(t *testing.T) {
			t.Parallel()

			result := exitWith(t, testCase.mode, "--port", "abc", "cluster")

			assert.Equal(t, "app: error: invalid integer value \"abc\" for flag --port\n", result.stderr)
			assert.Equal(t, 2, result.code)
		})

		t.Run("unknown flag follows the mode in "+testCase.name, func(t *testing.T) {
			t.Parallel()

			result := exitWith(t, testCase.mode, "--bogus")

			assert.Equal(t, testCase.unknownFlag, result.stderr)
			assert.Equal(t, 2, result.code)
		})
	}

	t.Run("subcommand required prints full help for summary and full", func(t *testing.T) {
		t.Parallel()

		for _, mode := range []conga.UsageOnErrorMode{conga.UsageSummary, conga.UsageFull} {
			result := exitWith(t, mode)

			assert.Equal(t, goldenErrorsShapeSubcommandRequiredHelp, result.stderr)
			assert.Equal(t, 2, result.code)
		}
	})

	t.Run("subcommand required prints the hint for hint mode", func(t *testing.T) {
		t.Parallel()

		result := exitWith(t, conga.UsageHint)

		assert.Equal(t, goldenErrorsShapeSubcommandRequiredHint, result.stderr)
		assert.Equal(t, 2, result.code)
	})

	t.Run("subcommand required prints the line alone for silence", func(t *testing.T) {
		t.Parallel()

		result := exitWith(t, conga.UsageSilence)

		assert.Equal(t, "app: error: subcommand required for \"app\"\n", result.stderr)
		assert.Equal(t, 2, result.code)
	})
}
