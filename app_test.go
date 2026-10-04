package conga_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/internal/assertx"
	"github.com/conga-sh/conga/plugin/timezone"
	"github.com/conga-sh/conga/predict"
)

type SimpleAppCLI struct {
	noopRunner

	CacheEnabled conga.OptionalFlag[bool]  `default:"true" name:"cache" negatable:"true"`
	Port         conga.OptionalFlag[int]   `default:"8080" short:"p"`
	TagList      conga.OptionalFlag[[]int] `name:"tag"     short:"t"`
}

type InvalidSchemaTagCLI struct {
	noopRunner

	Port conga.OptionalFlag[int] `required:"true"`
}

type EnvSemanticsCLI struct {
	noopRunner

	CustomPort conga.OptionalFlag[int]    `default:"8080"      env:"CUSTOM_PORT" name:"port"`
	Token      conga.OptionalFlag[string] `default:"secret"    env:"-"           name:"token"`
	Host       conga.OptionalFlag[string] `default:"127.0.0.1" name:"host"`
}

type ZeroAppCLI struct {
	CacheEnabled conga.OptionalFlag[bool]  `default:"true" name:"cache" negatable:"true"`
	Port         conga.OptionalFlag[int]   `default:"8080" short:"p"`
	TagList      conga.OptionalFlag[[]int] `name:"tag"     short:"t"`
}

func (c *ZeroAppCLI) Run(ctx context.Context, session *conga.Session) error {
	fmt.Fprintln(session.Stdout(), "CacheEnabled:", c.CacheEnabled.Get(), "Port:", c.Port.Get(), "TagList:", c.TagList.Get())

	return nil
}

type ContextReceiverCLI struct {
	ReceivedValue string
}

func (c *ContextReceiverCLI) Run(ctx context.Context, session *conga.Session) error {
	if value, ok := ctx.Value(ctxKey{}).(string); ok {
		c.ReceivedValue = value
	}

	return nil
}

func TestApp_NegatedBoolFlag(t *testing.T) {
	t.Parallel()

	cli := new(SimpleAppCLI)
	app := conga.New(cli).
		Name("testapp").
		Description("A test application").
		Args([]string{"--no-cache"})

	err := app.Run()

	assert.NoError(t, err)
	assert.False(t, cli.CacheEnabled.Get())
	assert.Equal(t, conga.SourceCLI, cli.CacheEnabled.Source())
}

func TestApp_EnvPrefixDerivation(t *testing.T) {
	t.Setenv("DEMO_PORT", "4000")

	cli := new(SimpleAppCLI)
	app := conga.New(cli).
		EnvPrefix("DEMO").
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 4000, cli.Port.Get())
	assert.Equal(t, conga.SourceEnv, cli.Port.Source())
}

func TestApp_EnvPrefixExcluded(t *testing.T) {
	t.Setenv("DEMO_TOKEN", "injected-value")

	cli := new(EnvSemanticsCLI)
	app := conga.New(cli).
		EnvPrefix("DEMO").
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "secret", cli.Token.Get())
	assert.Equal(t, conga.SourceDefault, cli.Token.Source())
}

func TestApp_EnvExplicitOverridesPrefix(t *testing.T) {
	t.Setenv("DEMO_PORT", "9999")
	t.Setenv("DEMO_HOST", "0.0.0.0")

	cli := new(EnvSemanticsCLI)
	app := conga.New(cli).
		EnvPrefix("DEMO").
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 8080, cli.CustomPort.Get())
	assert.Equal(t, conga.SourceDefault, cli.CustomPort.Source())
	assert.Equal(t, "0.0.0.0", cli.Host.Get())
	assert.Equal(t, conga.SourceEnv, cli.Host.Source())
}

func TestApp_SliceFlags(t *testing.T) {
	t.Parallel()

	cli := new(SimpleAppCLI)
	app := conga.New(cli).
		Args([]string{"-t", "101", "--tag", "202", "-t", "303"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []int{101, 202, 303}, cli.TagList.Get())
}

func TestApp_RunContext(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(t.Context(), ctxKey{}, "injected-value")

	cli := new(ContextReceiverCLI)
	app := conga.New(cli).
		Args([]string{})

	err := app.RunContext(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "injected-value", cli.ReceivedValue)
}

func TestApp_RunContext_PropagatesSchemaError(t *testing.T) {
	t.Parallel()

	app := conga.New[InvalidSchemaTagCLI]().
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
}

func TestApp_RunContext_NilContext(t *testing.T) {
	t.Parallel()

	app := conga.New[SimpleAppCLI]().
		Args([]string{"-p", "7777"})

	var nilCtx context.Context

	assert.Panics(t, func() {
		_ = app.RunContext(nilCtx)
	})
}

func TestApp_RunContext_DoubleExecutionPanics(t *testing.T) {
	t.Parallel()

	app := conga.New[SimpleAppCLI]().
		Args([]string{"-p", "7777"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)

	assertx.PanicsWithValue(t, "conga: App already executed", func() {
		_ = app.RunContext(t.Context())
	})
}

func TestApp_Run_DoubleExecutionPanics(t *testing.T) {
	t.Parallel()

	app := conga.New[SimpleAppCLI]().
		Args([]string{"-p", "7777"})

	err := app.Run()

	assert.NoError(t, err)

	assertx.PanicsWithValue(t, "conga: App already executed", func() {
		_ = app.Run()
	})
}

func TestApp_ZeroValue(t *testing.T) {
	t.Parallel()

	t.Run("one line", func(t *testing.T) {
		t.Parallel()

		var stdoutBuffer bytes.Buffer

		err := new(conga.App[ZeroAppCLI]).
			Args([]string{"-p", "7777"}).
			Stdout(&stdoutBuffer).
			RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, stdoutBuffer.String(), "CacheEnabled: true Port: 7777 TagList: []\n")
	})

	t.Run("runs like new", func(t *testing.T) {
		t.Parallel()

		var (
			app          conga.App[ZeroAppCLI]
			stdoutBuffer bytes.Buffer
		)

		err := app.Args([]string{"-p", "7777"}).
			Stdout(&stdoutBuffer).
			RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, stdoutBuffer.String(), "CacheEnabled: true Port: 7777 TagList: []\n")
	})

	t.Run("with cli", func(t *testing.T) {
		t.Parallel()

		var (
			cli          ZeroAppCLI
			stdoutBuffer bytes.Buffer
		)

		err := conga.New(&cli).
			Args([]string{"-p", "7777"}).
			Stdout(&stdoutBuffer).
			RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, cli.Port.Get(), 7777)
		assert.Equal(t, stdoutBuffer.String(), "CacheEnabled: true Port: 7777 TagList: []\n")
	})
}

func TestApp_ZeroValue_EnablesBuiltins(t *testing.T) {
	t.Parallel()

	var (
		app          conga.App[goldenCLI]
		stdoutBuffer bytes.Buffer
	)

	configureGoldenApp(&app).Args([]string{"--help"}).Stdout(&stdoutBuffer)

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assertGolden(t, "testdata/help/root.golden", goldenHelpRoot, stdoutBuffer.String())
}

func TestApp_ZeroValue_PredictorBeforeRun(t *testing.T) {
	t.Parallel()

	var app conga.App[SimpleAppCLI]

	app.Predictor("colors", predict.Values("red", "blue")).Args([]string{"-p", "1"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
}

func TestApp_ZeroValue_StrictTagValidationDefault(t *testing.T) {
	t.Parallel()

	type ZeroValueUnknownTagCLI struct {
		noopRunner

		Port conga.OptionalFlag[int] `custom:"x" default:"8080"`
	}

	var app conga.App[ZeroValueUnknownTagCLI]

	app.Args([]string{})

	err := app.RunContext(t.Context())

	assert.IsError(t, err, conga.ErrSchema)
}

func TestApp_CompletionInvocationDoesNotConsumeExecution(t *testing.T) {
	t.Parallel()

	cli := new(SimpleAppCLI)
	app := conga.New(cli).
		EnableCompletion().
		Name("testapp").
		Stdout(io.Discard).
		Args([]string{"__complete", "bash", ""})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)

	app.Args([]string{"-p", "7777"})

	err = app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 7777, cli.Port.Get())
}

func TestApp_RunDefaultContext(t *testing.T) {
	t.Parallel()

	cli := new(SimpleAppCLI)
	app := conga.New(cli).
		Args([]string{"-p", "9999"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, 9999, cli.Port.Get())
}

func TestApp_Complete(t *testing.T) {
	t.Parallel()

	app := conga.New[SimpleAppCLI]().
		Name("testapp")

	invoked, formatted := app.CompleteForShell("fish", []string{"--ca"})

	assert.Equal(t, 1, len(invoked.Items))
	assert.Equal(t, "--cache", invoked.Items[0].Value)
	assert.Equal(t, "--cache", formatted) // fish framing omits the trailing tab
}

func TestApp_CompleteCompLine(t *testing.T) {
	t.Parallel()

	app := conga.New[SimpleAppCLI]().
		Name("testapp")

	invoked, formatted := app.CompleteCompLineForShell("fish", "testapp --ca", 12)

	assert.Equal(t, 1, len(invoked.Items))
	assert.Equal(t, "--cache", invoked.Items[0].Value)
	assert.Equal(t, "--cache", formatted) // fish framing omits the trailing tab
}

func TestApp_HiddenCompleteCommand(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[SimpleAppCLI]().
		EnableCompletion().
		Name("testapp").
		Stdout(&buffer).
		Args([]string{"__complete", "fish", "--ca"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "--cache", buffer.String()) // fish framing omits the trailing tab
}

func TestApp_HiddenCompleteCommand_EnvCompLine(t *testing.T) {
	t.Setenv("COMP_LINE", "testapp --ca")
	t.Setenv("COMP_POINT", "12")

	var buffer bytes.Buffer

	app := conga.New[SimpleAppCLI]().
		EnableCompletion().
		Name("testapp").
		Stdout(&buffer).
		Args([]string{"__complete", "bash"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "D\t\nC\t--cache", buffer.String()) // bash framing
}

// TestApp_HiddenCompleteCommand_EnvCompLineRedirectTarget verifies the
// redirect-rejection contract of the __complete dispatch
// (docs/completion-wire-protocol.md §4.1): with the cursor at a redirection
// target the bash framing carries the D line only — no C candidate lines may
// leak from the raw-argument fallback.
func TestApp_HiddenCompleteCommand_EnvCompLineRedirectTarget(t *testing.T) {
	t.Setenv("COMP_LINE", "testapp echo test > ")
	t.Setenv("COMP_POINT", strconv.Itoa(len("testapp echo test > ")))

	var buffer bytes.Buffer

	type echoSubcommand struct{ noopRunner }

	type redirectTargetCLI struct {
		Echo conga.Command[echoSubcommand] `summary:"Echo command"`
	}

	app := conga.New[redirectTargetCLI]().
		EnableCompletion().
		Name("testapp").
		Stdout(&buffer).
		Args([]string{"__complete", "bash"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "D\t", buffer.String())
}

func TestApp_Predictor_EmptyNamePanic(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		conga.New[SimpleAppCLI]().Predictor("", predict.None())
	})
}

func TestApp_Predictor_ReservedNamePanic(t *testing.T) {
	t.Parallel()

	reservedNames := []string{
		"file", "files",
		"dir", "dirs", "directory", "directories",
		"exec", "executable", "executables",
		"symlink", "symlinks",
		"none", "off",
	}

	for _, name := range reservedNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Panics(t, func() {
				conga.New[SimpleAppCLI]().Predictor(name, predict.None())
			})
		})
	}
}

func TestApp_Predictor_DuplicateRegistrationPanic(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		conga.New[SimpleAppCLI]().
			Predictor("branches", predict.Values("main")).
			Predictor("branches", predict.Values("dev"))
	})
}

func TestApp_CompletionOffByDefault_RejectsCompletionCommand(t *testing.T) {
	t.Parallel()

	t.Run("single command app rejects completion as unexpected arg", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Completion conga.CompletionCommand
			Port       conga.OptionalFlag[int] `default:"8080"`
		}

		app := conga.New[CLI]().
			Args([]string{"completion", "bash"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("multi command app rejects completion as unknown command", func(t *testing.T) {
		t.Parallel()

		type StatusSubcommand struct {
			noopRunner

			Active conga.OptionalFlag[bool]
		}

		type CLI struct {
			Status     conga.Command[StatusSubcommand] `summary:"Status command"`
			Completion conga.CompletionCommand
		}

		app := conga.New[CLI]().
			Args([]string{"completion", "bash"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})
}

func TestApp_Use_MountsRealPluginPackage(t *testing.T) {
	t.Parallel()

	type usePluginCLI struct {
		noopRunner

		Zone conga.OptionalFlag[*time.Location] `default:"UTC" short:"z"`
	}

	cli := new(usePluginCLI)
	app := conga.New(cli).
		Args([]string{"-z", "Europe/Paris"}).
		Use(timezone.Location[usePluginCLI])

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "Europe/Paris", cli.Zone.Get().String())
	assert.Equal(t, conga.SourceCLI, cli.Zone.Source())
}

func TestApp_Use_AppliesPluginsInOrderAndSkipsNil(t *testing.T) {
	t.Parallel()

	type useOrderCLI struct {
		noopRunner

		Extra conga.OptionalFlag[string] `help:"Extra value"`
	}

	var appliedOrder []string

	first := func(app *conga.App[useOrderCLI]) {
		appliedOrder = append(appliedOrder, "first")
	}

	second := func(app *conga.App[useOrderCLI]) {
		appliedOrder = append(appliedOrder, "second")
	}

	cli := new(useOrderCLI)
	app := conga.New(cli).
		Args([]string{"--extra", "set"}).
		Use(first, nil, second)

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, appliedOrder)
	assert.Equal(t, "set", cli.Extra.Get())
	assert.Equal(t, conga.SourceCLI, cli.Extra.Source())
}

func TestApp_TagValidationBuilders(t *testing.T) {
	t.Parallel()

	t.Run("unknown tag rejected by default with exact message", func(t *testing.T) {
		t.Parallel()

		type UnknownTagCLI struct {
			noopRunner

			Port conga.OptionalFlag[int] `custom:"x" default:"8080"`
		}

		app := conga.New[UnknownTagCLI]().Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct UnknownTagCLI field Port: schema error: invalid struct tag: unknown struct tag "custom". Use conga:"allow-unknown-tags" or app.AllowTags("custom") to permit external tags`, err.Error())
	})

	t.Run("AllowTags permits the registered custom tag", func(t *testing.T) {
		t.Parallel()

		type AllowedTagCLI struct {
			noopRunner

			Port conga.OptionalFlag[int] `custom:"x" default:"8080"`
		}

		cli := new(AllowedTagCLI)
		app := conga.New(cli).
			Args([]string{}).
			AllowTags("custom")

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, 8080, cli.Port.Get())
	})

	t.Run("StrictTagValidation false disables unknown tag rejection", func(t *testing.T) {
		t.Parallel()

		type LenientTagCLI struct {
			noopRunner

			Port conga.OptionalFlag[int] `custom:"x" default:"8080"`
		}

		cli := new(LenientTagCLI)
		app := conga.New(cli).
			Args([]string{}).
			StrictTagValidation(false)

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, 8080, cli.Port.Get())
	})
}

type ctxKey struct{}
