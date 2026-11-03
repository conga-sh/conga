package conga_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type implicitRunCmd struct {
	DryRun conga.OptionalFlag[bool] `help:"Print what would run" short:"n"`
	File   conga.RequiredArg[string]
	Args   conga.RawArgs

	executed bool
	path     string
}

func (c *implicitRunCmd) Run(_ context.Context, session *conga.Session) error {
	c.executed = true
	c.path = session.Path()

	return nil
}

type implicitInstallCmd struct {
	Frozen conga.OptionalFlag[bool] `help:"Skip network access"`

	executed bool
}

func (c *implicitInstallCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

type implicitRootCLI struct {
	Exec    conga.Command[implicitRunCmd]     `implicit:"true"                name:"run" summary:"Execute a file"`
	Install conga.Command[implicitInstallCmd] `summary:"Install dependencies"`
}

type implicitRunnableRootCLI struct {
	Exec    conga.Command[implicitRunCmd]     `implicit:"true"                name:"run" summary:"Execute a file"`
	Install conga.Command[implicitInstallCmd] `summary:"Install dependencies"`

	executed bool
	path     string
}

func (c *implicitRunnableRootCLI) Run(_ context.Context, session *conga.Session) error {
	c.executed = true
	c.path = session.Path()

	return nil
}

type implicitShortClusterCLI struct {
	Verbose conga.Counter                     `help:"Increase verbosity" short:"v"`
	Output  conga.OptionalFlag[string]        `short:"o"`
	Exec    conga.Command[implicitInstallCmd] `implicit:"true"           name:"run"`
}

type implicitDeepJSONCmd struct {
	executed bool
}

func (c *implicitDeepJSONCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

type implicitDeepLeafCmd struct {
	Format conga.OptionalFlag[string]
	File   conga.RequiredArg[string]

	executed bool
	format   string
	file     string
}

func (c *implicitDeepLeafCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true
	c.format = c.Format.Get()
	c.file = c.File.Get()

	return nil
}

type implicitDeepMidCmd struct {
	JSON conga.Command[implicitDeepJSONCmd] `name:"json"`
	Run  conga.Command[implicitDeepLeafCmd] `implicit:"true" name:"run"`
}

type implicitDeepRootCLI struct {
	Mid conga.Command[implicitDeepMidCmd] `implicit:"true" name:"mid"`
}

type implicitNumericRunCmd struct {
	Value conga.RequiredArg[int]

	executed bool
	value    int
}

func (c *implicitNumericRunCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true
	c.value = c.Value.Get()

	return nil
}

type implicitNumericRootCLI struct {
	Exec conga.Command[implicitNumericRunCmd] `implicit:"true" name:"run"`

	executed bool
}

func (c *implicitNumericRootCLI) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

type implicitNumericShortRootCLI struct {
	Level conga.OptionalFlag[bool]             `short:"5"`
	Exec  conga.Command[implicitNumericRunCmd] `implicit:"true" name:"run"`

	executed bool
}

func (c *implicitNumericShortRootCLI) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

func TestImplicitCommand_OperandRoutesToImplicit(t *testing.T) {
	t.Parallel()

	cli := new(implicitRootCLI)
	app := conga.New(cli).Name("tool").Args([]string{"script.ts", "one", "two"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Exec.Get().executed)
	assert.Equal(t, "script.ts", cli.Exec.Get().File.Get())
	assert.Equal(t, []string{"one", "two"}, cli.Exec.Get().Args.Get())
	assert.Equal(t, "tool run", cli.Exec.Get().path)
	assert.False(t, cli.Install.Get().executed)
}

func TestImplicitCommand_ExplicitSiblingWins(t *testing.T) {
	t.Parallel()

	cli := new(implicitRootCLI)
	app := conga.New(cli).Args([]string{"install"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Install.Get().executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_ExplicitNameIsEquivalent(t *testing.T) {
	t.Parallel()

	cli := new(implicitRootCLI)
	app := conga.New(cli).Args([]string{"run", "script.ts"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Exec.Get().executed)
	assert.Equal(t, "script.ts", cli.Exec.Get().File.Get())
}

func TestImplicitCommand_ChildFlagBindsBeforeOperand(t *testing.T) {
	t.Parallel()

	cli := new(implicitRootCLI)
	app := conga.New(cli).Args([]string{"--dry-run", "script.ts"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Exec.Get().DryRun.Get())
	assert.Equal(t, "script.ts", cli.Exec.Get().File.Get())
}

func TestImplicitCommand_ChildFlagOnExplicitSiblingIsUnknown(t *testing.T) {
	t.Parallel()

	cli := new(implicitRootCLI)
	app := conga.New(cli).Args([]string{"--dry-run", "install"})

	err := app.Run()

	assert.IsError(t, err, conga.ErrParse)
	assert.True(t, conga.IsUsageError(err))
	assert.False(t, cli.Install.Get().executed)
}

func TestImplicitCommand_RunnableParentWinsAtEndOfTokens(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).Name("tool").Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.executed)
	assert.Equal(t, "tool", cli.path)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_TerminatorRoutesToImplicit(t *testing.T) {
	t.Parallel()

	cli := new(implicitRootCLI)
	app := conga.New(cli).Args([]string{"--", "script.ts", "-x"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Exec.Get().executed)
	assert.Equal(t, "script.ts", cli.Exec.Get().File.Get())
	assert.Equal(t, []string{"-x"}, cli.Exec.Get().Args.Get())
}

func TestImplicitCommand_HelpStaysAtParentAndMarksDefault(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[implicitRootCLI]().Name("tool").Stdout(&stdoutBuffer).Args([]string{"--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "Usage:\n  tool [<command>]")
	assert.Contains(t, stdoutBuffer.String(), "run      Execute a file (default)")
	assert.Contains(t, stdoutBuffer.String(), "install  Install dependencies")
}

func TestImplicitCommand_HelpAfterOperandShowsImplicitChild(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[implicitRootCLI]().Name("tool").Stdout(&stdoutBuffer).Args([]string{"script.ts", "--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "Usage:\n  tool run")
}

func TestImplicitCommand_TerminatorRoutesRunnableParentOperand(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).Name("tool").Args([]string{"--", "script.ts"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Exec.Get().executed)
	assert.Equal(t, "script.ts", cli.Exec.Get().File.Get())
	assert.Equal(t, "tool run", cli.Exec.Get().path)
	assert.False(t, cli.executed)
}

func TestImplicitCommand_BuiltinBeforeTerminatorKeepsParentHelp(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[implicitRunnableRootCLI]().Name("tool").Stdout(&stdoutBuffer).Args([]string{"--help", "--", "script.ts"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "Usage:\n  tool [<command>]")
	assert.NotContains(t, stdoutBuffer.String(), "Usage:\n  tool run")
}

func TestImplicitCommand_ShortClusterBuiltinRequestsParentHelp(t *testing.T) {
	t.Parallel()

	t.Run("bool and counter runes reach help", func(t *testing.T) {
		t.Parallel()

		var stdoutBuffer bytes.Buffer

		app := conga.New[implicitShortClusterCLI]().
			Name("tool").
			HelpShortFlag('h').
			Stdout(&stdoutBuffer).
			Args([]string{"-vvh"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, stdoutBuffer.String(), "Usage:\n  tool [<command>]")
		assert.NotContains(t, stdoutBuffer.String(), "Usage:\n  tool run")
	})

	t.Run("value-taking rune consumes help rune", func(t *testing.T) {
		t.Parallel()

		var stdoutBuffer bytes.Buffer

		cli := new(implicitShortClusterCLI)
		app := conga.New(cli).
			Name("tool").
			HelpShortFlag('h').
			Stdout(&stdoutBuffer).
			Args([]string{"-oh"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "h", cli.Output.Get())
		assert.True(t, cli.Exec.Get().executed)
		assert.Equal(t, "", stdoutBuffer.String())
	})
}

func TestImplicitCommand_DeepChainFlagValueBeforeOperand(t *testing.T) {
	t.Parallel()

	cli := new(implicitDeepRootCLI)
	app := conga.New(cli).Name("tool").Args([]string{"--format", "json", "script.ts"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Mid.Get().Run.Get().executed)
	assert.Equal(t, "json", cli.Mid.Get().Run.Get().format)
	assert.Equal(t, "script.ts", cli.Mid.Get().Run.Get().file)
	assert.False(t, cli.Mid.Get().JSON.Get().executed)
}

func TestImplicitCommand_NegativeNumberOperandRoutesToImplicit(t *testing.T) {
	t.Parallel()

	cli := new(implicitNumericRootCLI)
	app := conga.New(cli).Name("tool").Args([]string{"-5"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Exec.Get().executed)
	assert.Equal(t, -5, cli.Exec.Get().value)
	assert.False(t, cli.executed)
}

func TestImplicitCommand_RegisteredNegativeShortBindsAsFlag(t *testing.T) {
	t.Parallel()

	cli := new(implicitNumericShortRootCLI)
	app := conga.New(cli).Name("tool").Args([]string{"-5"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.executed)
	assert.True(t, cli.Level.Get())
	assert.False(t, cli.Exec.Get().executed)
}
