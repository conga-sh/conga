package conga_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type docClaimsValidateCLI struct{}

func (c *docClaimsValidateCLI) Validate(_ context.Context, _ *conga.Session) error {
	return errors.New("invalid state")
}

func (c *docClaimsValidateCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

func (c *docClaimsValidateCLI) Finally(_ context.Context, session *conga.Session, _ error) error {
	_, err := fmt.Fprintln(session.Stdout(), "finalized")

	return err
}

type docClaimsRequiredDeprecatedCLI struct {
	noopRunner

	Token conga.RequiredFlag[string] `deprecated:"gone" help:"Token"`
}

type docClaimsVersionDeprecatedCLI struct {
	noopRunner

	Version conga.VersionCommand `deprecated:"gone" summary:"Version"`
}

type docClaimsCompletionDeprecatedCLI struct {
	noopRunner

	Completion conga.CompletionCommand `deprecated:"gone" summary:"Completion"`
}

type docClaimsEnvPrefixWorkerCLI struct {
	noopRunner

	Concurrency conga.OptionalFlag[int]    `default:"4"              help:"Worker concurrency"`
	Token       conga.OptionalFlag[string] `env:"DOC_EXPLICIT_TOKEN" help:"Worker token"`
}

type docClaimsEnvPrefixCLI struct {
	Worker conga.Command[docClaimsEnvPrefixWorkerCLI] `envprefix:"DOC_WORKER" summary:"Background worker"`
}

func TestDocClaims_DeprecatedAppWarnsOnlyWhenLifecycleRuns(t *testing.T) {
	t.Parallel()

	t.Run("run that reaches the lifecycle warns", func(t *testing.T) {
		t.Parallel()

		var stderrBuffer bytes.Buffer

		app := conga.New[DeprecatedAppWithSubCLI]().
			Name("testapp").
			Deprecated("use the new CLI").
			Stderr(&stderrBuffer).
			Args([]string{})

		assert.NoError(t, app.Run())
		assert.Contains(t, stderrBuffer.String(), "WARNING: testapp is deprecated: use the new CLI")
	})

	for _, arg := range []string{"--help", "--version"} {
		t.Run(arg+" prints no warning", func(t *testing.T) {
			t.Parallel()

			var stdoutBuffer, stderrBuffer bytes.Buffer

			app := conga.New[DeprecatedAppWithSubCLI]().
				Name("testapp").
				Deprecated("use the new CLI").
				Stdout(&stdoutBuffer).
				Stderr(&stderrBuffer).
				Args([]string{arg})

			assert.NoError(t, app.Run())
			assert.NotEqual(t, "", stdoutBuffer.String())
			assert.Equal(t, "", stderrBuffer.String())
		})
	}
}

func TestDocClaims_ValidateFailureSkipsFinally(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[docClaimsValidateCLI]().Stdout(&buffer).Args([]string{})

	err := app.Run()

	assert.Error(t, err)
	assert.True(t, conga.IsUsageError(err))
	assert.Equal(t, "", buffer.String())
}

func TestDocClaims_DeprecatedTagRestrictions(t *testing.T) {
	t.Parallel()

	t.Run("required flag", func(t *testing.T) {
		t.Parallel()

		err := conga.New[docClaimsRequiredDeprecatedCLI]().Args([]string{"--token", "x"}).Run()

		assert.IsError(t, err, conga.ErrSchema)
	})

	t.Run("version command", func(t *testing.T) {
		t.Parallel()

		err := conga.New[docClaimsVersionDeprecatedCLI]().Args([]string{}).Run()

		assert.IsError(t, err, conga.ErrSchema)
	})

	t.Run("completion command", func(t *testing.T) {
		t.Parallel()

		err := conga.New[docClaimsCompletionDeprecatedCLI]().EnableCompletion().Args([]string{}).Run()

		assert.IsError(t, err, conga.ErrSchema)
	})
}

func TestDocClaims_SubcommandEnvPrefixReplacesAppPrefix(t *testing.T) {
	t.Setenv("DOC_WORKER_CONCURRENCY", "7")
	t.Setenv("CONGA_TEST_DOC_APP_DOC_WORKER_CONCURRENCY", "9")

	cli := new(docClaimsEnvPrefixCLI)

	err := conga.New(cli).
		EnvPrefix("CONGA_TEST_DOC_APP").
		Args([]string{"worker"}).
		Run()

	assert.NoError(t, err)
	assert.Equal(t, 7, cli.Worker.Get().Concurrency.Get())
}

func TestDocClaims_SubcommandEnvPrefixHelpHint(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	err := conga.New[docClaimsEnvPrefixCLI]().
		EnvPrefix("CONGA_TEST_DOC_APP").
		Stdout(&buffer).
		Args([]string{"worker", "--help"}).
		Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "[env: DOC_WORKER_CONCURRENCY]")
	assert.NotContains(t, buffer.String(), "CONGA_TEST_DOC_APP_DOC_WORKER_CONCURRENCY")
}

func TestDocClaims_EnvTagOverridesSubcommandEnvPrefix(t *testing.T) {
	t.Setenv("DOC_EXPLICIT_TOKEN", "explicit")
	t.Setenv("DOC_WORKER_TOKEN", "derived")

	cli := new(docClaimsEnvPrefixCLI)

	err := conga.New(cli).
		EnvPrefix("CONGA_TEST_DOC_APP").
		Args([]string{"worker"}).
		Run()

	assert.NoError(t, err)
	assert.Equal(t, "explicit", cli.Worker.Get().Token.Get())
}
