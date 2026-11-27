package conga_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/internal/assertx"
	"github.com/conga-sh/conga/internal/errlines"
	"github.com/conga-sh/conga/internal/spec"
)

// Compile-time checks that all seven pure value containers satisfy conga.Input.
var (
	_ conga.Input = (*conga.OptionalFlag[string])(nil)
	_ conga.Input = (*conga.RequiredFlag[string])(nil)
	_ conga.Input = (*conga.OptionalArg[string])(nil)
	_ conga.Input = (*conga.RequiredArg[string])(nil)
	_ conga.Input = (*conga.Counter)(nil)
	_ conga.Input = (*conga.DashArgs)(nil)
	_ conga.Input = (*conga.RawArgs)(nil)
)

type builderCauseError struct {
	message string
	origins []string
	note    string
	hint    string
}

func (e builderCauseError) Error() string     { return e.message }
func (e builderCauseError) Origins() []string { return e.origins }
func (e builderCauseError) Note() string      { return e.note }
func (e builderCauseError) Hint() string      { return e.hint }

var (
	errBuilderSentinelA = errors.New("sentinel a")
	errBuilderSentinelB = errors.New("sentinel b")
	errBuilderWrapInner = &builderCauseError{
		message: "dial tcp: timeout",
		origins: []string{"inner origin"},
		note:    "inner note",
		hint:    "inner hint",
	}
)

type errorBuilderMixedDateCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderMixedDateCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since, &c.Until).
		Note("date range must be non-empty").
		Hint("pick an --until on or after --since").
		New("--until must not be before --since")
}

type errorBuilderSingleDateCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderSingleDateCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since).
		Note("accepted format: YYYY-MM-DD").
		Hint("pass a calendar date").
		Newf("invalid date %q for --since", c.Since.Get())
}

type errorBuilderDuplicateInputCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderDuplicateInputCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().For(&c.Since, &c.Since).New("bad date")
}

type errorBuilderEnvDefaultDateCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderEnvDefaultDateCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since, &c.Until).
		Hint("pick an --until on or after --since").
		New("--until must not be before --since")
}

type errorBuilderDefaultsOnlyCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-02-01" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-01" help:"Last day"`
}

func (c *errorBuilderDefaultsOnlyCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since, &c.Until).
		New("--until must not be before --since")
}

type errorBuilderCLIUnsetPairCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `help:"First day"`
	Until conga.OptionalFlag[string] `help:"Last day"`
}

func (c *errorBuilderCLIUnsetPairCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since, &c.Until).
		New("--until is required with --since")
}

type errorBuilderSingleCLIArgCLI struct {
	noopRunner

	File conga.RequiredArg[string] `help:"Input file" name:"FILE"`
}

func (c *errorBuilderSingleCLIArgCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.File).
		New("unsupported file extension")
}

type errorBuilderDirectValidateCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `help:"First day"`
}

func (c *errorBuilderDirectValidateCLI) validateDirect(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since).
		Note("cli note").
		New("bad cli")
}

type errorBuilderConfigDateCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderConfigDateCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since, &c.Until).
		New("--until must not be before --since")
}

type errorBuilderDocumentDateCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderDocumentDateCLI) Bootstrap(_ context.Context, handle *conga.Bootstrap) error {
	var document conga.ConfigDocument
	document.Set("since", "2026-02-01", "profile dev")
	document.Set("token", "abc", "profile dev")
	handle.SetConfig(&document)

	return nil
}

func (c *errorBuilderDocumentDateCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since, &c.Until).
		New("--until must not be before --since")
}

type errorBuilderUnsetLabelsCLI struct {
	noopRunner

	Token  conga.RequiredFlag[string]            `env:"EXPORT_TOKEN"  help:"API token"`
	Labels conga.OptionalFlag[map[string]string] `env:"EXPORT_LABELS" help:"Labels"`
}

func (c *errorBuilderUnsetLabelsCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Labels).
		New("labels are required in strict mode")
}

type errorBuilderMapLabelsCLI struct {
	noopRunner

	Token  conga.RequiredFlag[string]            `env:"EXPORT_TOKEN"  help:"API token"`
	Labels conga.OptionalFlag[map[string]string] `env:"EXPORT_LABELS" help:"Labels"`
}

func (c *errorBuilderMapLabelsCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Labels).
		New("invalid label set")
}

type errorBuilderCounterEnvCLI struct {
	noopRunner

	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN" help:"API token"`
	Count conga.Counter              `env:"EXPORT_COUNT" help:"Verbosity" short:"v"`
}

func (c *errorBuilderCounterEnvCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Count).
		New("verbosity too high")
}

type errorBuilderArgDefaultCLI struct {
	noopRunner

	File   conga.OptionalArg[string]  `default:"input.json" help:"Input file"    name:"FILE"`
	Format conga.OptionalFlag[string] `default:"json"       env:"CONVERT_FORMAT" help:"Output format"`
	Dash   conga.DashArgs             `help:"Extra args"`
}

func (c *errorBuilderArgDefaultCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.File, &c.Format).
		New("file extension does not match --format")
}

type errorBuilderDashSetCLI struct {
	noopRunner

	File   conga.OptionalArg[string]  `default:"input.json" help:"Input file"    name:"FILE"`
	Format conga.OptionalFlag[string] `default:"json"       env:"CONVERT_FORMAT" help:"Output format"`
	Dash   conga.DashArgs             `help:"Extra args"`
}

func (c *errorBuilderDashSetCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Dash, &c.Format).
		New("cannot pass trailing args with default format")
}

type errorBuilderDashUnsetCLI struct {
	noopRunner

	File   conga.OptionalArg[string]  `default:"input.json" help:"Input file"    name:"FILE"`
	Format conga.OptionalFlag[string] `default:"json"       env:"CONVERT_FORMAT" help:"Output format"`
	Dash   conga.DashArgs             `help:"Extra args"`
}

func (c *errorBuilderDashUnsetCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Dash).
		New("trailing args required")
}

type errorBuilderRawSetCLI struct {
	noopRunner

	Target conga.RequiredArg[string]  `help:"Target"   name:"TARGET"`
	Format conga.OptionalFlag[string] `default:"json"  help:"Output format"`
	Raw    conga.RawArgs              `help:"Raw args"`
}

func (c *errorBuilderRawSetCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Raw, &c.Format).
		New("unsupported raw command")
}

type errorBuilderRawUnsetCLI struct {
	noopRunner

	Target conga.RequiredArg[string]  `help:"Target"   name:"TARGET"`
	Format conga.OptionalFlag[string] `default:"json"  help:"Output format"`
	Raw    conga.RawArgs              `help:"Raw args"`
}

func (c *errorBuilderRawUnsetCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Raw).
		New("command arguments missing")
}

type errorBuilderNoForCLI struct {
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN" help:"API token"`
}

func (c *errorBuilderNoForCLI) Run(_ context.Context, session *conga.Session) error {
	return session.Error().
		Note("session token expired").
		Hint("run export login").
		New("not logged in")
}

type errorBuilderAncestorRoot struct {
	Global conga.OptionalFlag[string]          `default:"root-default"  env:"ROOT_GLOBAL" help:"Global"`
	First  conga.Command[errorBuilderFirstCmd] `summary:"First sibling"`
}

type errorBuilderFirstCmd struct {
	noopRunner

	Local conga.OptionalFlag[string] `default:"local" help:"Local"`
}

func (c *errorBuilderFirstCmd) Validate(_ context.Context, session *conga.Session) error {
	parent := session.Root[errorBuilderAncestorRoot]()

	return session.Error().
		For(&parent.Global, &c.Local).
		New("global and local disagree")
}

type errorBuilderDotenvCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"CONGA_TEST_ERRBUILDER_DOTENV_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"CONGA_TEST_ERRBUILDER_DOTENV_UNTIL" help:"Last day"`
}

func (c *errorBuilderDotenvCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since, &c.Until).
		Hint("pick an --until on or after --since").
		New("--until must not be before --since")
}

type errorBuilderUsageTrueValidateNoForCLI struct {
	noopRunner

	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN" help:"API token"`
}

func (c *errorBuilderUsageTrueValidateNoForCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().Usage(true).Hint("pass --since").New("missing date range")
}

type errorBuilderUsageTrueSingleCLICLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderUsageTrueSingleCLICLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().For(&c.Since).Usage(true).New("invalid --since")
}

type errorBuilderUsageTrueRunDefaultCLI struct {
	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderUsageTrueRunDefaultCLI) Run(_ context.Context, session *conga.Session) error {
	return session.Error().For(&c.Since).Usage(true).New("bad default date")
}

type errorBuilderUsageFalseValidateCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderUsageFalseValidateCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().For(&c.Since, &c.Until).Usage(false).New("endpoint unreachable")
}

type errorBuilderUsageFalseRunCLI struct {
	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderUsageFalseRunCLI) Run(_ context.Context, session *conga.Session) error {
	return session.Error().For(&c.Since, &c.Until).Usage(false).New("connection refused")
}

type errorBuilderNewfCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderNewfCLI) Validate(_ context.Context, session *conga.Session) error {
	causeA := builderCauseError{message: "bad start", note: "note a", hint: "hint a"}
	causeB := builderCauseError{message: "bad end"}

	return errors.Join(
		session.Error().For(&c.Since).Newf("%w while checking start", errBuilderSentinelA),
		session.Error().For(&c.Since).Newf("checking %w in middle", errBuilderSentinelB),
		session.Error().For(&c.Since).Newf("checking end: %w", causeA),
		session.Error().For(&c.Until).Newf("both %w and %w", errBuilderSentinelA, causeB),
	)
}

type errorBuilderWrapNilCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`

	ran     bool
	wrapped error
}

func (c *errorBuilderWrapNilCLI) Validate(_ context.Context, session *conga.Session) error {
	c.ran = true
	c.wrapped = session.Error().For(&c.Since).Note("ignored").Wrap(nil)

	return c.wrapped
}

type errorBuilderWrapCLICLI struct {
	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderWrapCLICLI) Run(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Since).
		Hint("override hint").
		Wrap(errBuilderWrapInner)
}

type errorBuilderWrapDefaultCLI struct {
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderWrapDefaultCLI) Run(_ context.Context, session *conga.Session) error {
	return session.Error().
		For(&c.Until).
		Wrap(errBuilderWrapInner)
}

type errorBuilderWrapfCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`

	inner      *builderCauseError
	emptyInner error
	nilResult  error
}

func (c *errorBuilderWrapfCLI) Validate(_ context.Context, session *conga.Session) error {
	c.nilResult = session.Error().For(&c.Since).Wrapf(nil, "connect %s", "host")

	return errors.Join(
		session.Error().For(&c.Since).Wrapf(c.inner, "connect %s", "api"),
		session.Error().For(&c.Until).Wrapf(c.inner, ""),
		session.Error().For(&c.Until).Wrapf(c.emptyInner, "only prefix %d", 1),
	)
}

type errorBuilderJoinValidateCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
}

func (c *errorBuilderJoinValidateCLI) Validate(_ context.Context, session *conga.Session) error {
	envErr := session.Error().For(&c.Since).New("bad env since")
	cliErr := session.Error().For(&c.Until).New("bad cli until")

	return errors.Join(envErr, cliErr)
}

type errorBuilderLocalUnboundCLI struct {
	noopRunner

	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN" help:"API token"`
}

func (c *errorBuilderLocalUnboundCLI) Validate(_ context.Context, session *conga.Session) error {
	var localFlag conga.OptionalFlag[string]

	return session.Error().For(&localFlag).New("unreachable")
}

type errorBuilderSiblingUnboundRoot struct {
	First  conga.Command[errorBuilderSiblingUnboundFirst]  `summary:"First sibling"`
	Second conga.Command[errorBuilderSiblingUnboundSecond] `summary:"Second sibling"`
}

type errorBuilderSiblingUnboundFirst struct {
	noopRunner

	Local conga.OptionalFlag[string] `default:"local" help:"Local"`
}

func (c *errorBuilderSiblingUnboundFirst) Validate(_ context.Context, session *conga.Session) error {
	sibling := session.Root[errorBuilderSiblingUnboundRoot]().Second.Get()

	return session.Error().For(&sibling.Local).New("unreachable")
}

type errorBuilderSiblingUnboundSecond struct {
	noopRunner

	Local conga.OptionalFlag[string] `default:"local" help:"Local"`
}

type errorBuilderNilInputCLI struct {
	noopRunner

	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN" help:"API token"`
}

func (c *errorBuilderNilInputCLI) Validate(_ context.Context, session *conga.Session) error {
	return session.Error().For(nil).New("unreachable")
}

type errorBuilderReuseCLI struct {
	noopRunner

	Since conga.OptionalFlag[string] `default:"2026-01-01" env:"EXPORT_SINCE" help:"First day"`
	Until conga.OptionalFlag[string] `default:"2026-01-31" env:"EXPORT_UNTIL" help:"Last day"`
	Token conga.RequiredFlag[string] `env:"EXPORT_TOKEN"   help:"API token"`
	Count conga.Counter              `env:"EXPORT_COUNT"   help:"Verbosity"   short:"v"`
}

func (c *errorBuilderReuseCLI) Validate(_ context.Context, session *conga.Session) error {
	base := session.Error().For(&c.Since).Note("shared note")
	first := base.For(&c.Until).Hint("fix until").New("first error")
	second := base.For(&c.Count).New("second error")

	return errors.Join(first, second)
}

func TestInputInterface_NonInputTypesExcluded(t *testing.T) {
	t.Parallel()

	inputInterface := reflect.TypeFor[conga.Input]()

	nonInputTypes := []struct {
		name       string
		targetType reflect.Type
	}{
		{name: "value copy of OptionalFlag", targetType: reflect.TypeFor[conga.OptionalFlag[string]]()},
		{name: "value copy of RequiredFlag", targetType: reflect.TypeFor[conga.RequiredFlag[string]]()},
		{name: "value copy of OptionalArg", targetType: reflect.TypeFor[conga.OptionalArg[string]]()},
		{name: "value copy of RequiredArg", targetType: reflect.TypeFor[conga.RequiredArg[string]]()},
		{name: "value copy of Counter", targetType: reflect.TypeFor[conga.Counter]()},
		{name: "value copy of DashArgs", targetType: reflect.TypeFor[conga.DashArgs]()},
		{name: "value copy of RawArgs", targetType: reflect.TypeFor[conga.RawArgs]()},
		{name: "plain string pointer", targetType: reflect.TypeFor[*string]()},
		{name: "Command pointer", targetType: reflect.TypeFor[*conga.Command[errorBuilderArgDefaultCLI]]()},
		{name: "ConfigFile pointer", targetType: reflect.TypeFor[*conga.ConfigFile]()},
		{name: "Features pointer", targetType: reflect.TypeFor[*conga.Features[struct{ Alpha bool }]]()},
	}

	for _, testCase := range nonInputTypes {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.False(t, testCase.targetType.Implements(inputInterface))
		})
	}
}

func TestSessionError_SourceClassificationAndOrigins(t *testing.T) {
	t.Parallel()

	t.Run("CLI and default mixed exits 2 with usage block and default origin", func(t *testing.T) {
		t.Parallel()

		var stderrBuffer bytes.Buffer

		exitCode := -1

		app := conga.New[errorBuilderMixedDateCLI]().
			Name("export").
			Args([]string{"--token", "abc", "--since", "2026-02-01"}).
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code })

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, "--until must not be before --since", err.Error())
		assert.Equal(t, 2, app.ExitCode(err))
		assert.NotEqual(t, "", app.UsageBlock(err))

		diagnostic := conga.DiagnosticOf(err)
		assert.Equal(t, []string{"--until: default value"}, diagnostic.Origins())
		assert.Equal(t, "date range must be non-empty", diagnostic.Note())
		assert.Equal(t, "pick an --until on or after --since", diagnostic.Hint())

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Contains(t, stderrBuffer.String(), "export: error: --until must not be before --since\n")
		assert.Contains(t, stderrBuffer.String(), "  from: --until: default value\n")
		assert.Contains(t, stderrBuffer.String(), "  note: date range must be non-empty\n")
		assert.Contains(t, stderrBuffer.String(), "  hint: pick an --until on or after --since\n")
		assert.Contains(t, stderrBuffer.String(), "\nUsage:\n  export --token=STRING [flags]")
		assert.NotContains(t, stderrBuffer.String(), "from: --since")
	})

	t.Run("single CLI input exits 2 with line only and no usage block", func(t *testing.T) {
		t.Parallel()

		var stderrBuffer bytes.Buffer

		exitCode := -1

		app := conga.New[errorBuilderSingleDateCLI]().
			Name("export").
			Args([]string{"--token", "abc", "--since", "2026/02/01"}).
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code })

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, `invalid date "2026/02/01" for --since`, err.Error())
		assert.Equal(t, 2, app.ExitCode(err))
		assert.Equal(t, "", app.UsageBlock(err))

		diagnostic := conga.DiagnosticOf(err)
		assert.Zero(t, diagnostic.Origins())
		assert.Equal(t, "accepted format: YYYY-MM-DD", diagnostic.Note())
		assert.Equal(t, "pass a calendar date", diagnostic.Hint())

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, "export: error: invalid date \"2026/02/01\" for --since\n  note: accepted format: YYYY-MM-DD\n  hint: pass a calendar date\n", stderrBuffer.String())
		assert.NotContains(t, stderrBuffer.String(), "Usage:")
	})

	t.Run("duplicate same CLI input still counts as single input line-only", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderDuplicateInputCLI]().Name("export").Args([]string{"--token", "abc", "--since", "bad"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, "bad date", err.Error())
		assert.Equal(t, "", app.UsageBlock(err))
	})

	t.Run("defaults-only conflict exits 1 with both default origins and no usage block", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderDefaultsOnlyCLI]().Name("export").Args(nil)

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, "--until must not be before --since", err.Error())
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Equal(t, "", app.UsageBlock(err))
		assert.Equal(t, []string{
			"--since: default value",
			"--until: default value",
		}, conga.DiagnosticOf(err).Origins())
	})

	t.Run("two-input CLI and unset pair shows usage block with no from lines", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderCLIUnsetPairCLI]().Name("export").Args([]string{"--since", "2026-02-01"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, "--until is required with --since", err.Error())
		assert.Equal(t, 2, app.ExitCode(err))
		assert.NotEqual(t, "", app.UsageBlock(err))
		assert.Zero(t, conga.DiagnosticOf(err).Origins())
	})

	t.Run("single CLI positional argument exits 2 with line only and no usage block", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderSingleCLIArgCLI]().Name("convert").Args([]string{"bad.txt"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, "unsupported file extension", err.Error())
		assert.Equal(t, 2, app.ExitCode(err))
		assert.Equal(t, "", app.UsageBlock(err))
		assert.Zero(t, conga.DiagnosticOf(err).Origins())
	})

	t.Run("config file input names config file and key and exits 1", func(t *testing.T) {
		t.Parallel()

		configPath := filepath.Join(t.TempDir(), "export.json")
		assert.NoError(t, os.WriteFile(configPath, []byte(`{"since":"2026-02-01","token":"abc"}`), 0o600))

		app := conga.New[errorBuilderConfigDateCLI]().
			Name("export").
			RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Args(nil)

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, 1, app.ExitCode(err))

		diagnostic := conga.DiagnosticOf(err)
		assert.Equal(t, []string{
			`--since: config key "since" in config file "` + configPath + `"`,
			"--until: default value",
		}, diagnostic.Origins())
	})

	t.Run("ConfigDocument input names document origin label and exits 1", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderDocumentDateCLI]().Name("export").Args(nil)

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, 1, app.ExitCode(err))

		diagnostic := conga.DiagnosticOf(err)
		assert.Equal(t, []string{
			`--since: config key "since" set by profile dev`,
			"--until: default value",
		}, diagnostic.Origins())
	})

	t.Run("unset inputs omit from lines and exit 1", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderUnsetLabelsCLI]().Name("export").Args([]string{"--token", "abc"})

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, "labels are required in strict mode", err.Error())
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Zero(t, conga.DiagnosticOf(err).Origins())
	})

	t.Run("map flag via flagOrigins reports contributing config origin", func(t *testing.T) {
		t.Parallel()

		configPath := filepath.Join(t.TempDir(), "export.json")
		assert.NoError(t, os.WriteFile(configPath, []byte(`{"token":"abc","labels":{"env":"prod"}}`), 0o600))

		app := conga.New[errorBuilderMapLabelsCLI]().
			Name("export").
			RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Args(nil)

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Equal(t, []string{`--labels: config key "labels" in config file "` + configPath + `"`}, conga.DiagnosticOf(err).Origins())
	})

	t.Run("map flag mixed CLI and config classifies as CLI", func(t *testing.T) {
		t.Parallel()

		configPath := filepath.Join(t.TempDir(), "export.json")
		assert.NoError(t, os.WriteFile(configPath, []byte(`{"token":"abc","labels":{"env":"prod"}}`), 0o600))

		app := conga.New[errorBuilderMapLabelsCLI]().
			Name("export").
			RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(configPath)).
			Args([]string{"--labels", "tier=web"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, 2, app.ExitCode(err))
		assert.Equal(t, "", app.UsageBlock(err))
		assert.Zero(t, conga.DiagnosticOf(err).Origins())
	})

	t.Run("DashArgs set classifies as CLI without from line", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderDashSetCLI]().Name("convert").Args([]string{"--", "-v"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, 2, app.ExitCode(err))
		assert.NotEqual(t, "", app.UsageBlock(err))
		assert.Equal(t, []string{"--format: default value"}, conga.DiagnosticOf(err).Origins())
	})

	t.Run("DashArgs unset counts as unset", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderDashUnsetCLI]().Name("convert").Args(nil)

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Zero(t, conga.DiagnosticOf(err).Origins())
	})

	t.Run("RawArgs set classifies as CLI without from line", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderRawSetCLI]().Name("exec").Args([]string{"web", "ls", "-la"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, 2, app.ExitCode(err))
		assert.NotEqual(t, "", app.UsageBlock(err))
		assert.Equal(t, []string{"--format: default value"}, conga.DiagnosticOf(err).Origins())
	})

	t.Run("RawArgs unset counts as unset", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderRawUnsetCLI]().Name("exec").Args([]string{"web"})

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Zero(t, conga.DiagnosticOf(err).Origins())
	})

	t.Run("no For defaults to exit 1 with no from lines", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderNoForCLI]().Name("export").Args([]string{"--token", "abc"})

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, "not logged in", err.Error())
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Equal(t, "", app.UsageBlock(err))

		diagnostic := conga.DiagnosticOf(err)
		assert.Zero(t, diagnostic.Origins())
		assert.Equal(t, "session token expired", diagnostic.Note())
		assert.Equal(t, "run export login", diagnostic.Hint())
	})

	t.Run("ancestor flag passed to For resolves on active chain", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderAncestorRoot]().Name("tool").Args([]string{"first", "--local", "x"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, "global and local disagree", err.Error())
		assert.Equal(t, []string{"--global: default value"}, conga.DiagnosticOf(err).Origins())
	})
}

func TestSessionError_EnvironmentOrigins(t *testing.T) {
	t.Run("environment and default inputs exit 1 with both from lines and no usage block", func(t *testing.T) {
		t.Setenv("EXPORT_SINCE", "2026-02-01")
		t.Setenv("EXPORT_TOKEN", "abc")

		var stderrBuffer bytes.Buffer

		exitCode := -1

		app := conga.New[errorBuilderEnvDefaultDateCLI]().
			Name("export").
			Args(nil).
			Stderr(&stderrBuffer).
			ExitFunc(func(code int) { exitCode = code })

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, "--until must not be before --since", err.Error())
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Equal(t, "", app.UsageBlock(err))

		diagnostic := conga.DiagnosticOf(err)
		assert.Equal(t, []string{
			"--since: environment variable EXPORT_SINCE",
			"--until: default value",
		}, diagnostic.Origins())
		assert.Equal(t, "pick an --until on or after --since", diagnostic.Hint())

		app.Exit(err)

		assert.Equal(t, 1, exitCode)
		assert.Equal(t, "export: error: --until must not be before --since\n  from: --since: environment variable EXPORT_SINCE\n  from: --until: default value\n  hint: pick an --until on or after --since\n", stderrBuffer.String())
		assert.NotContains(t, stderrBuffer.String(), "Usage:")
	})

	t.Run("dotenv input names dotenv file in from line and exits 1", func(t *testing.T) {
		dotenvPath := filepath.Join(t.TempDir(), ".env")
		assert.NoError(t, os.WriteFile(dotenvPath, []byte("CONGA_TEST_ERRBUILDER_DOTENV_SINCE=2026-02-01\n"), 0o600))

		capturedEnv := map[string]string{}

		app := conga.New[errorBuilderDotenvCLI]().
			Name("export").
			Dotenv(dotenvPath).
			SetenvFunc(func(key, value string) {
				capturedEnv[key] = value
				t.Setenv(key, value)
			}).
			Args(nil)

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, "--until must not be before --since", err.Error())
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Equal(t, "2026-02-01", capturedEnv["CONGA_TEST_ERRBUILDER_DOTENV_SINCE"])

		diagnostic := conga.DiagnosticOf(err)
		assert.Equal(t, []string{
			"--since: environment variable CONGA_TEST_ERRBUILDER_DOTENV_SINCE in .env file \"" + dotenvPath + "\"",
			"--until: default value",
		}, diagnostic.Origins())
	})

	t.Run("Counter flag from environment reports from line and exits 1", func(t *testing.T) {
		t.Setenv("EXPORT_COUNT", "3")
		t.Setenv("EXPORT_TOKEN", "abc")

		app := conga.New[errorBuilderCounterEnvCLI]().Name("export").Args(nil)

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Equal(t, []string{"--count: environment variable EXPORT_COUNT"}, conga.DiagnosticOf(err).Origins())
	})

	t.Run("positional argument default alongside environment flag", func(t *testing.T) {
		t.Setenv("CONVERT_FORMAT", "toml")

		app := conga.New[errorBuilderArgDefaultCLI]().Name("convert").Args(nil)

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, "file extension does not match --format", err.Error())
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Equal(t, []string{
			"argument FILE: default value",
			"--format: environment variable CONVERT_FORMAT",
		}, conga.DiagnosticOf(err).Origins())
	})
}

func TestSessionError_UsageOverrideFromValidateAndRun(t *testing.T) {
	t.Parallel()

	t.Run("Usage(true) without For forces UsageError and usage block in Validate", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderUsageTrueValidateNoForCLI]().Name("export").Args([]string{"--token", "abc"})

		err := app.RunContext(t.Context())

		usageErr := assertUsageError(t, err)
		assert.Equal(t, "missing date range", usageErr.Error())
		assert.Equal(t, 2, app.ExitCode(err))
		assert.NotEqual(t, "", app.UsageBlock(err))
		assert.Equal(t, "pass --since", conga.DiagnosticOf(err).Hint())
	})

	t.Run("Usage(true) overrides single CLI input line-only shape", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderUsageTrueSingleCLICLI]().Name("export").Args([]string{"--token", "abc", "--since", "bad"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, 2, app.ExitCode(err))
		assert.NotEqual(t, "", app.UsageBlock(err))
	})

	t.Run("Usage(true) with non-CLI For keeps from lines and shows usage block", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderUsageTrueRunDefaultCLI]().Name("export").Args([]string{"--token", "abc"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, 2, app.ExitCode(err))
		assert.NotEqual(t, "", app.UsageBlock(err))
		assert.Equal(t, []string{"--since: default value"}, conga.DiagnosticOf(err).Origins())
	})

	t.Run("Usage(false) on CLI input stays exit 1 from Validate", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderUsageFalseValidateCLI]().Name("export").Args([]string{"--token", "abc", "--since", "2026-01-01"})

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, "endpoint unreachable", err.Error())
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Equal(t, "", app.UsageBlock(err))
		assert.Equal(t, []string{"--until: default value"}, conga.DiagnosticOf(err).Origins())
	})

	t.Run("Usage(false) on CLI input stays exit 1 from Run", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderUsageFalseRunCLI]().Name("export").Args([]string{"--token", "abc", "--since", "2026-01-01"})

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.Equal(t, "connection refused", err.Error())
		assert.Equal(t, 1, app.ExitCode(err))
		assert.Equal(t, "", app.UsageBlock(err))
		assert.Equal(t, []string{"--until: default value"}, conga.DiagnosticOf(err).Origins())
	})
}

func TestSessionError_Terminals(t *testing.T) {
	t.Parallel()

	t.Run("Newf supports %w at start, middle, end, and multiple %w", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderNewfCLI]().Name("export").Args([]string{"--token", "abc", "--since", "2026-01-01"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, errBuilderSentinelA)
		assert.IsError(t, err, errBuilderSentinelB)

		members := conga.DiagnosticOf(err).Members()
		assert.Equal(t, 4, len(members))
		assert.Equal(t, "sentinel a while checking start", members[0].Message())
		assert.Equal(t, "checking sentinel b in middle", members[1].Message())
		assert.Equal(t, "checking end: bad start", members[2].Message())
		assert.Equal(t, "note a", members[2].Note())
		assert.Equal(t, "hint a", members[2].Hint())
		assert.Equal(t, "both sentinel a and bad end", members[3].Message())
	})

	t.Run("Wrap returns untyped nil for nil error", func(t *testing.T) {
		t.Parallel()

		cli := new(errorBuilderWrapNilCLI)
		app := conga.New(cli).Name("export").Args([]string{"--token", "abc", "--since", "2026-01-01"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.True(t, cli.ran)
		assert.True(t, cli.wrapped == nil)
	})

	t.Run("Wrap preserves cause and inherits inner pieces unless overridden", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderWrapCLICLI]().Name("export").Args([]string{"--token", "abc", "--since", "2026-01-01"})

		err := app.RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, "dial tcp: timeout", err.Error())
		assert.IsError(t, err, errBuilderWrapInner)

		diagnostic := conga.DiagnosticOf(err)
		assert.Equal(t, []string{"inner origin"}, diagnostic.Origins())
		assert.Equal(t, "inner note", diagnostic.Note())
		assert.Equal(t, "override hint", diagnostic.Hint())
	})

	t.Run("Wrap non-CLI input replaces inner origins with For origin", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderWrapDefaultCLI]().Name("export").Args([]string{"--token", "abc"})

		err := app.RunContext(t.Context())

		assertNotUsageError(t, err)
		assert.IsError(t, err, errBuilderWrapInner)

		diagnostic := conga.DiagnosticOf(err)
		assert.Equal(t, []string{"--until: default value"}, diagnostic.Origins())
		assert.Equal(t, "inner note", diagnostic.Note())
		assert.Equal(t, "inner hint", diagnostic.Hint())
	})

	t.Run("Wrapf handles nil, formatted prefix, empty prefix, and empty inner message", func(t *testing.T) {
		t.Parallel()

		cli := &errorBuilderWrapfCLI{
			inner:      &builderCauseError{message: "connection refused", note: "inner note"},
			emptyInner: errors.New(""),
		}
		app := conga.New(cli).Name("export").Args([]string{"--token", "abc", "--since", "2026-01-01"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.True(t, cli.nilResult == nil)
		assert.IsError(t, err, cli.inner)
		assert.IsError(t, err, cli.emptyInner)

		members := conga.DiagnosticOf(err).Members()
		assert.Equal(t, 3, len(members))
		assert.Equal(t, "connect api: connection refused", members[0].Message())
		assert.Equal(t, "inner note", members[0].Note())
		assert.Equal(t, "connection refused", members[1].Message())
		assert.Equal(t, "only prefix 1", members[2].Message())
	})
}

func TestSessionError_JoinPreservedThroughValidate(t *testing.T) {
	t.Setenv("EXPORT_SINCE", "2026-02-01")

	var stderrBuffer bytes.Buffer

	exitCode := -1

	app := conga.New[errorBuilderJoinValidateCLI]().
		Name("export").
		Args([]string{"--token", "abc", "--until", "2026-01-01"}).
		Stderr(&stderrBuffer).
		ExitFunc(func(code int) { exitCode = code })

	err := app.RunContext(t.Context())

	assertUsageError(t, err)
	assert.Equal(t, 2, app.ExitCode(err))

	members := conga.DiagnosticOf(err).Members()
	assert.Equal(t, 2, len(members))
	assert.Equal(t, "bad env since", members[0].Message())
	assert.Equal(t, []string{"--since: environment variable EXPORT_SINCE"}, members[0].Origins())
	assert.Equal(t, "bad cli until", members[1].Message())
	assert.Zero(t, members[1].Origins())

	app.Exit(err)

	assert.Equal(t, 2, exitCode)
	assert.Contains(t, stderrBuffer.String(), "export: error: bad env since\n  from: --since: environment variable EXPORT_SINCE\n")
	assert.Contains(t, stderrBuffer.String(), "export: error: bad cli until\n")
}

func TestSessionError_UnboundInputPanics(t *testing.T) {
	t.Parallel()

	t.Run("local unbound flag panics with exact message", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderLocalUnboundCLI]().Name("export").Args([]string{"--token", "abc"})

		assertx.PanicsWithValue(t, "conga: Session.Error().For: input is not bound to the active command", func() {
			_ = app.RunContext(t.Context())
		})
	})

	t.Run("unselected sibling command field panics with exact message", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderSiblingUnboundRoot]().Name("tool").Args([]string{"first"})

		assertx.PanicsWithValue(t, "conga: Session.Error().For: input is not bound to the active command", func() {
			_ = app.RunContext(t.Context())
		})
	})

	t.Run("nil Input panics on initialized session", func(t *testing.T) {
		t.Parallel()

		app := conga.New[errorBuilderNilInputCLI]().Name("export").Args([]string{"--token", "abc"})

		assertx.PanicsWithValue(t, "conga: Session.Error().For: input is not bound to the active command", func() {
			_ = app.RunContext(t.Context())
		})
	})
}

func TestSessionError_NilAndUninitializedSessions(t *testing.T) {
	t.Parallel()

	t.Run("nil session classifies by Source without panicking or from lines", func(t *testing.T) {
		t.Parallel()

		var (
			session *conga.Session
			since   conga.OptionalFlag[string]
		)

		err := session.Error().For(&since).Note("hint detail").New("missing")

		assertNotUsageError(t, err)
		assert.Equal(t, "missing", err.Error())
		assert.Zero(t, conga.DiagnosticOf(err).Origins())
		assert.Equal(t, "hint detail", conga.DiagnosticOf(err).Note())
	})

	t.Run("SourceCLI-bound flag passed to nil session classifies as UsageError without usage block", func(t *testing.T) {
		t.Parallel()

		cmd := new(errorBuilderDirectValidateCLI)
		app := conga.New(cmd).Name("export").Args([]string{"--since", "2026-02-01"})

		bindErr := app.RunContext(t.Context())
		assert.NoError(t, bindErr)

		err := cmd.validateDirect(t.Context(), nil)

		usageErr := assertUsageError(t, err)
		assert.Equal(t, "bad cli", usageErr.Error())
		assert.Equal(t, 2, app.ExitCode(err))
		assert.Equal(t, "", app.UsageBlock(err))
		assert.Zero(t, conga.DiagnosticOf(err).Origins())
		assert.Equal(t, "cli note", conga.DiagnosticOf(err).Note())
	})

	t.Run("zero Session{} and zero ErrorBuilder do not panic", func(t *testing.T) {
		t.Parallel()

		var (
			zeroBuilder  conga.ErrorBuilder
			emptySession conga.Session
			dash         conga.DashArgs
		)

		errFromZeroBuilder := zeroBuilder.For(&dash).New("zero builder")
		errFromEmptySession := emptySession.Error().For(&dash).Usage(true).New("empty session")

		assertNotUsageError(t, errFromZeroBuilder)
		assert.Equal(t, "zero builder", errFromZeroBuilder.Error())

		usageErr := assertUsageError(t, errFromEmptySession)
		assert.Equal(t, "empty session", usageErr.Error())
		assert.Equal(t, "", usageErr.CommandPath())
		assert.Equal(t, "", conga.New[struct{}]().UsageBlock(usageErr))
	})
}

func TestSessionError_BuilderReuse(t *testing.T) {
	t.Parallel()

	app := conga.New[errorBuilderReuseCLI]().Name("export").Args([]string{"--token", "abc"})

	err := app.RunContext(t.Context())

	assertNotUsageError(t, err)

	members := conga.DiagnosticOf(err).Members()
	assert.Equal(t, 2, len(members))
	assert.Equal(t, []string{"--since: default value", "--until: default value"}, members[0].Origins())
	assert.Equal(t, "shared note", members[0].Note())
	assert.Equal(t, "fix until", members[0].Hint())
	assert.Equal(t, []string{"--since: default value"}, members[1].Origins())
	assert.Equal(t, "shared note", members[1].Note())
	assert.Equal(t, "", members[1].Hint())
}

// --- Value-receiver lifecycle hook schema-build tests ---

type valueReceiverRunnerCmd struct{}

func (v valueReceiverRunnerCmd) Run(_ context.Context, _ *conga.Session) error { return nil }

type valueReceiverValidatorCmd struct {
	noopRunner
}

func (v valueReceiverValidatorCmd) Validate(_ context.Context, _ *conga.Session) error {
	return nil
}

type valueReceiverPreRunnerCmd struct {
	noopRunner
}

func (v valueReceiverPreRunnerCmd) PreRun(_ context.Context, _ *conga.Session) error { return nil }

type valueReceiverPostRunnerCmd struct {
	noopRunner
}

func (v valueReceiverPostRunnerCmd) PostRun(_ context.Context, _ *conga.Session) error { return nil }

type valueReceiverFinalizerCmd struct {
	noopRunner
}

func (v valueReceiverFinalizerCmd) Finally(_ context.Context, _ *conga.Session, _ error) error {
	return nil
}

type valueReceiverBootstrapperCmd struct {
	noopRunner
}

func (v valueReceiverBootstrapperCmd) Bootstrap(_ context.Context, _ *conga.Bootstrap) error {
	return nil
}

type pointerMixinBase struct {
	called bool
}

func (b *pointerMixinBase) Validate(_ context.Context, _ *conga.Session) error {
	b.called = true

	return nil
}

func (b *pointerMixinBase) Run(_ context.Context, _ *conga.Session) error { return nil }

type embeddedPointerMixinCmd struct {
	*pointerMixinBase
}

type embeddedPointerMid struct {
	*pointerMixinBase
}

type nestedEmbeddedPointerMixinCmd struct {
	embeddedPointerMid
}

type valueEmbeddedMid struct {
	pointerMixinBase
}

type embeddedPointerToValueEmbeddedCmd struct {
	*valueEmbeddedMid
}

type unnamedPointerTarget = struct {
	pointerMixinBase
}

type unnamedEmbeddedPointerCmd struct {
	*unnamedPointerTarget
}

type embeddedInterfaceCmd struct {
	conga.Runner
}

type unnamedInterfaceTarget = interface {
	Run(ctx context.Context, session *conga.Session) error
}

type unnamedEmbeddedInterfaceCmd struct {
	unnamedInterfaceTarget
}

type valueMixinBase struct{}

func (b valueMixinBase) Validate(_ context.Context, _ *conga.Session) error { return nil }

type embeddedValueMixinCmd struct {
	noopRunner

	valueMixinBase
}

type embeddedPointerToValueMixinCmd struct {
	noopRunner

	*valueMixinBase
}

type shadowDirectValueOverPtrMixinCmd struct {
	*pointerMixinBase
}

func (s shadowDirectValueOverPtrMixinCmd) Run(_ context.Context, _ *conga.Session) error {
	_ = s.pointerMixinBase

	return nil
}

type shadowDepth1ValueOverDepth2PtrCmd struct {
	embeddedPointerMid
	valueMixinBase
}

func (s *shadowDepth1ValueOverDepth2PtrCmd) Run(_ context.Context, _ *conga.Session) error {
	_ = s.embeddedPointerMid

	return nil
}

type valueEmbeddedMixinCmd struct {
	pointerMixinBase
}

type nestedValueEmbeddedMixinCmd struct {
	valueEmbeddedMid
}

func TestSchema_ValueReceiverLifecycleHooks(t *testing.T) {
	t.Parallel()

	const wantNote = "a value receiver, an embedded pointer, or an embedded interface provides it"

	testCases := []struct {
		name     string
		run      func() error
		wantMsg  string
		wantHint string
	}{
		{
			name:     "Runner with value receiver",
			run:      func() error { return conga.New[valueReceiverRunnerCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct valueReceiverRunnerCmd: lifecycle hook Run is callable on a value of valueReceiverRunnerCmd",
			wantHint: "declare Run on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "Validator with value receiver",
			run:      func() error { return conga.New[valueReceiverValidatorCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct valueReceiverValidatorCmd: lifecycle hook Validate is callable on a value of valueReceiverValidatorCmd",
			wantHint: "declare Validate on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "PreRunner with value receiver",
			run:      func() error { return conga.New[valueReceiverPreRunnerCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct valueReceiverPreRunnerCmd: lifecycle hook PreRun is callable on a value of valueReceiverPreRunnerCmd",
			wantHint: "declare PreRun on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "PostRunner with value receiver",
			run:      func() error { return conga.New[valueReceiverPostRunnerCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct valueReceiverPostRunnerCmd: lifecycle hook PostRun is callable on a value of valueReceiverPostRunnerCmd",
			wantHint: "declare PostRun on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "Finalizer with value receiver",
			run:      func() error { return conga.New[valueReceiverFinalizerCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct valueReceiverFinalizerCmd: lifecycle hook Finally is callable on a value of valueReceiverFinalizerCmd",
			wantHint: "declare Finally on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "Bootstrapper with value receiver",
			run:      func() error { return conga.New[valueReceiverBootstrapperCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct valueReceiverBootstrapperCmd: lifecycle hook Bootstrap is callable on a value of valueReceiverBootstrapperCmd",
			wantHint: "declare Bootstrap on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "embedded value mixin with value receiver",
			run:      func() error { return conga.New[embeddedValueMixinCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct embeddedValueMixinCmd: lifecycle hook Validate is callable on a value of embeddedValueMixinCmd",
			wantHint: "declare Validate on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "embedded pointer to value mixin is rejected",
			run:      func() error { return conga.New[embeddedPointerToValueMixinCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct embeddedPointerToValueMixinCmd: lifecycle hook Validate is callable on a value of embeddedPointerToValueMixinCmd",
			wantHint: "declare Validate on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "embedded pointer mixin with pointer receiver is rejected",
			run:      func() error { return conga.New[embeddedPointerMixinCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct embeddedPointerMixinCmd: lifecycle hook Run is callable on a value of embeddedPointerMixinCmd",
			wantHint: "declare Run on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "nested embedded pointer mixin with pointer receiver is rejected",
			run:      func() error { return conga.New[nestedEmbeddedPointerMixinCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct nestedEmbeddedPointerMixinCmd: lifecycle hook Run is callable on a value of nestedEmbeddedPointerMixinCmd",
			wantHint: "declare Run on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "embedded pointer to value-embedded mixin is rejected",
			run:      func() error { return conga.New[embeddedPointerToValueEmbeddedCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct embeddedPointerToValueEmbeddedCmd: lifecycle hook Run is callable on a value of embeddedPointerToValueEmbeddedCmd",
			wantHint: "declare Run on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "unnamed embedded pointer struct is rejected",
			run:      func() error { return conga.New[unnamedEmbeddedPointerCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct unnamedEmbeddedPointerCmd: lifecycle hook Run is callable on a value of unnamedEmbeddedPointerCmd",
			wantHint: "declare Run on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "embedded lifecycle interface is rejected",
			run:      func() error { return conga.New[embeddedInterfaceCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct embeddedInterfaceCmd: lifecycle hook Run is callable on a value of embeddedInterfaceCmd",
			wantHint: "declare Run on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "unnamed embedded lifecycle interface is rejected",
			run:      func() error { return conga.New[unnamedEmbeddedInterfaceCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct unnamedEmbeddedInterfaceCmd: lifecycle hook Run is callable on a value of unnamedEmbeddedInterfaceCmd",
			wantHint: "declare Run on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "direct value receiver alongside embedded pointer mixin is rejected",
			run:      func() error { return conga.New[shadowDirectValueOverPtrMixinCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct shadowDirectValueOverPtrMixinCmd: lifecycle hook Run is callable on a value of shadowDirectValueOverPtrMixinCmd",
			wantHint: "declare Run on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
		{
			name:     "depth-1 value mixin alongside depth-2 embedded pointer mixin is rejected",
			run:      func() error { return conga.New[shadowDepth1ValueOverDepth2PtrCmd]().Args(nil).RunContext(t.Context()) },
			wantMsg:  "struct shadowDepth1ValueOverDepth2PtrCmd: lifecycle hook Validate is callable on a value of shadowDepth1ValueOverDepth2PtrCmd",
			wantHint: "declare Validate on a pointer receiver, or embed the mixin by value. Run conga-lint for the exact cause",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.run()

			assert.Error(t, err)
			assert.True(t, spec.IsSchemaError(err))
			assert.Equal(t, testCase.wantMsg, err.Error())
			assert.Equal(t, wantNote, errlines.Note(err))
			assert.Equal(t, testCase.wantHint, errlines.Hint(err))
		})
	}

	t.Run("value-embedded mixin with pointer-receiver hooks is allowed", func(t *testing.T) {
		t.Parallel()

		cmd := new(valueEmbeddedMixinCmd)
		app := conga.New(cmd).Args(nil)

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.True(t, cmd.called)
	})

	t.Run("nested value-embedded mixin with pointer-receiver hooks is allowed", func(t *testing.T) {
		t.Parallel()

		cmd := new(nestedValueEmbeddedMixinCmd)
		app := conga.New(cmd).Args(nil)

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.True(t, cmd.called)
	})

	t.Run("value-receiver SelfValidator on domain type stays allowed", func(t *testing.T) {
		t.Parallel()

		type domainSelfValidatorCLI struct {
			noopRunner

			Score conga.OptionalFlag[SelfValidatingScore]
		}

		app := conga.New[domainSelfValidatorCLI]().Args([]string{"--score", "42"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})
}
