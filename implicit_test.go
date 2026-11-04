package conga_test

import (
	"bytes"
	"context"
	"errors"
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

type implicitRunnableFlagsCmd struct {
	DryRun conga.OptionalFlag[bool]   `short:"n"`
	Output conga.OptionalFlag[string] `short:"o"`
	Args   conga.RawArgs

	executed bool
	dryRun   bool
	output   string
}

func (c *implicitRunnableFlagsCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true
	c.dryRun = c.DryRun.Get()
	c.output = c.Output.Get()

	return nil
}

type implicitRunnableFlagsRoot struct {
	Verbose conga.Counter                           `short:"v"`
	Exec    conga.Command[implicitRunnableFlagsCmd] `implicit:"true" name:"run"`
	Install conga.Command[implicitInstallCmd]       `name:"install"`

	executed bool
}

func (c *implicitRunnableFlagsRoot) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

type implicitDeepRunnableMidCmd struct {
	MidFlag conga.OptionalFlag[bool]           `name:"mid-flag"`
	JSON    conga.Command[implicitDeepJSONCmd] `name:"json"`
	Exec    conga.Command[implicitDeepLeafCmd] `implicit:"true" name:"run"`

	executed bool
}

func (c *implicitDeepRunnableMidCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

type implicitDeepRunnableRootCLI struct {
	Mid conga.Command[implicitDeepRunnableMidCmd] `implicit:"true" name:"mid"`
}

type implicitVersionShortShadowCmd struct {
	DryRun  conga.OptionalFlag[bool] `short:"n"`
	Verbose conga.OptionalFlag[bool] `short:"V"`

	executed bool
}

func (c *implicitVersionShortShadowCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

type implicitVersionShortShadowRoot struct {
	Exec conga.Command[implicitVersionShortShadowCmd] `implicit:"true" name:"run"`
}

type implicitVersionLongShadowCmd struct {
	Version conga.OptionalFlag[bool]

	executed bool
}

func (c *implicitVersionLongShadowCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

type implicitVersionLongShadowRoot struct {
	Exec conga.Command[implicitVersionLongShadowCmd] `implicit:"true" name:"run"`
}

type implicitExplicitRunCmd struct {
	Format conga.OptionalFlag[string]

	executed bool
	format   string
}

func (c *implicitExplicitRunCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true
	c.format = c.Format.Get()

	return nil
}

type implicitExplicitMidCmd struct {
	Run conga.Command[implicitExplicitRunCmd] `implicit:"true" name:"run"`
}

type implicitExplicitRootCLI struct {
	Mid conga.Command[implicitExplicitMidCmd] `name:"mid"`
}

type implicitClusterValueLeafCmd struct {
	Five conga.OptionalFlag[bool] `short:"5"`

	executed bool
}

func (c *implicitClusterValueLeafCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

type implicitClusterValueMidCmd struct {
	DryRun conga.OptionalFlag[bool]                   `short:"n"`
	Output conga.OptionalFlag[string]                 `short:"o"`
	Exec   conga.Command[implicitClusterValueLeafCmd] `implicit:"true" name:"run"`

	executed bool
	dryRun   bool
	output   string
}

func (c *implicitClusterValueMidCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true
	c.dryRun = c.DryRun.Get()
	c.output = c.Output.Get()

	return nil
}

type implicitClusterValueRootCLI struct {
	Mid conga.Command[implicitClusterValueMidCmd] `implicit:"true" name:"mid"`
}

type implicitDashLeafCmd struct {
	Rest conga.DashArgs

	executed bool
	path     string
	rest     []string
}

func (c *implicitDashLeafCmd) Run(_ context.Context, session *conga.Session) error {
	c.executed = true
	c.path = session.Path()
	c.rest = c.Rest.Get()

	return nil
}

type implicitDashMidCmd struct {
	MidFlag conga.OptionalFlag[bool]           `name:"mid-flag"`
	Exec    conga.Command[implicitDashLeafCmd] `implicit:"true" name:"run"`

	executed bool
}

func (c *implicitDashMidCmd) Run(_ context.Context, _ *conga.Session) error {
	c.executed = true

	return nil
}

type implicitDashRootCLI struct {
	Rest conga.DashArgs
	Mid  conga.Command[implicitDashMidCmd] `implicit:"true" name:"mid"`

	executed bool
}

func (c *implicitDashRootCLI) Run(_ context.Context, _ *conga.Session) error {
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

func TestImplicitCommand_ChildFlagSelectsRunnableImplicit(t *testing.T) {
	t.Parallel()

	t.Run("short flag", func(t *testing.T) {
		t.Parallel()

		cli := new(implicitRunnableFlagsRoot)
		app := conga.New(cli).Name("tool").Args([]string{"-n"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Exec.Get().executed)
		assert.True(t, cli.Exec.Get().dryRun)
		assert.False(t, cli.executed)
	})

	t.Run("long flag", func(t *testing.T) {
		t.Parallel()

		cli := new(implicitRunnableFlagsRoot)
		app := conga.New(cli).Name("tool").Args([]string{"--dry-run"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Exec.Get().executed)
		assert.True(t, cli.Exec.Get().dryRun)
		assert.False(t, cli.executed)
	})
}

func TestImplicitCommand_ChildFlagMissingRequiredArgument(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).Name("tool").Args([]string{"-n"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.Equal(t, "missing required positional argument <file>", err.Error())

	var usage conga.UsageError

	assert.True(t, errors.As(err, &usage))
	assert.Equal(t, "tool run", usage.CommandPath())
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_ChildFlagWithValueSelectsImplicit(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableFlagsRoot)
	app := conga.New(cli).Name("tool").Args([]string{"-o", "report.txt"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Exec.Get().executed)
	assert.Equal(t, "report.txt", cli.Exec.Get().output)
	assert.False(t, cli.executed)
}

func TestImplicitCommand_RootFlagKeepsRunnableParent(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableFlagsRoot)
	app := conga.New(cli).Name("tool").Args([]string{"-v"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.executed)
	assert.Equal(t, 1, cli.Verbose.Get())
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_FlagBeforeExplicitNameSelectsImplicitChild(t *testing.T) {
	t.Parallel()

	t.Run("flag before the explicit name", func(t *testing.T) {
		t.Parallel()

		cli := new(implicitExplicitRootCLI)
		app := conga.New(cli).Name("tool").Args([]string{"--format", "json", "mid"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Mid.Get().Run.Get().executed)
		assert.Equal(t, "json", cli.Mid.Get().Run.Get().format)
	})

	t.Run("flag after the explicit name", func(t *testing.T) {
		t.Parallel()

		cli := new(implicitExplicitRootCLI)
		app := conga.New(cli).Name("tool").Args([]string{"mid", "--format", "json"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Mid.Get().Run.Get().executed)
		assert.Equal(t, "json", cli.Mid.Get().Run.Get().format)
	})
}

func TestImplicitCommand_ShortClusterChildFlagSelectsImplicit(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableFlagsRoot)
	app := conga.New(cli).Name("tool").Args([]string{"-vn"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Exec.Get().executed)
	assert.True(t, cli.Exec.Get().dryRun)
	assert.Equal(t, 1, cli.Verbose.Get())
	assert.False(t, cli.executed)
}

func TestImplicitCommand_SiblingFlagDoesNotSelectImplicit(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableFlagsRoot)
	app := conga.New(cli).Name("tool").Args([]string{"--frozen"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), "unknown flag: --frozen")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_ChildFlagBeforeHelpSelectsImplicit(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[implicitRunnableFlagsRoot]().
		Name("tool").
		Stdout(&stdoutBuffer).
		Args([]string{"-n", "--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "Usage:\n  tool run")
	assert.NotContains(t, stdoutBuffer.String(), "Usage:\n  tool [<command>]")
}

func TestImplicitCommand_ChildFlagBeforeVersionStaysUnknown(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).Name("tool").Version("1.0.0").Args([]string{"-n", "--version"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), "unknown flag: --version")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_VersionBeforeChildFlagStaysUnknown(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).Name("tool").Version("1.0.0").Args([]string{"--version", "-n"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), "unknown flag: --version")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_ChildFlagVersionShortInClusterStaysUnknown(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).
		Name("tool").
		Version("1.0.0").
		VersionShortFlag('V').
		Args([]string{"-Vn"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), "unknown shorthand flag: 'V' in -Vn")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_ChildFlagVersionShortStaysUnknown(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).
		Name("tool").
		Version("1.0.0").
		VersionShortFlag('V').
		Args([]string{"-n", "-V"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), "unknown flag: -V")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_ChildFlagVersionShortBindsImplicitChild(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := new(implicitVersionShortShadowRoot)
	app := conga.New(cli).
		Name("tool").
		Version("1.0.0").
		VersionShortFlag('V').
		Stdout(&stdoutBuffer).
		Args([]string{"-n", "-V"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "", stdoutBuffer.String())
	assert.True(t, cli.Exec.Get().executed)
	assert.True(t, cli.Exec.Get().Verbose.Get())
}

func TestImplicitCommand_RootVersionRendersVersion(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).Name("tool").Version("1.0.0").Stdout(&stdoutBuffer).Args([]string{"--version"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "tool has version 1.0.0")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_ConfiguredVersionShortWinsOverImplicitChild(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := new(implicitVersionShortShadowRoot)
	app := conga.New(cli).
		Name("tool").
		Version("1.0.0").
		VersionShortFlag('V').
		Stdout(&stdoutBuffer).
		Args([]string{"-V"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "tool has version 1.0.0")
	assert.False(t, cli.Exec.Get().executed)
	assert.False(t, cli.Exec.Get().Verbose.Get())
}

func TestImplicitCommand_ImplicitChildVersionLongFlagDoesNotShadowRootBuiltin(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := new(implicitVersionLongShadowRoot)
	app := conga.New(cli).
		Name("tool").
		Version("1.0.0").
		Stdout(&stdoutBuffer).
		Args([]string{"--version"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "tool has version 1.0.0")
	assert.False(t, cli.Exec.Get().executed)
	assert.False(t, cli.Exec.Get().Version.Get())
}

func TestImplicitCommand_ExplicitNameBindsImplicitChildVersionShadow(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := new(implicitVersionShortShadowRoot)
	app := conga.New(cli).
		Name("tool").
		Version("1.0.0").
		VersionShortFlag('V').
		Stdout(&stdoutBuffer).
		Args([]string{"run", "-V"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "", stdoutBuffer.String())
	assert.True(t, cli.Exec.Get().executed)
	assert.True(t, cli.Exec.Get().Verbose.Get())
}

func TestImplicitCommand_DisabledVersionStaysUnknown(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).
		Name("tool").
		Version("1.0.0").
		DisableVersionFlag().
		Args([]string{"-n", "--version"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), "unknown flag: --version")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_OperandBeforeVersionStaysUnknown(t *testing.T) {
	t.Parallel()

	cli := new(implicitDeepRootCLI)
	app := conga.New(cli).Name("tool").Version("1.0.0").Args([]string{"script.ts", "--version"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)

	var usage conga.UsageError

	assert.True(t, errors.As(err, &usage))
	assert.Equal(t, "tool mid run", usage.CommandPath())
	assert.Contains(t, err.Error(), "unknown flag: --version")
	assert.False(t, cli.Mid.Get().Run.Get().executed)
}

func TestImplicitCommand_OperandVersionForwardsToRawArgs(t *testing.T) {
	t.Parallel()

	t.Run("implicit operand", func(t *testing.T) {
		t.Parallel()

		cli := new(implicitRunnableRootCLI)
		app := conga.New(cli).Name("tool").Version("1.0.0").Args([]string{"script.ts", "--version"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Exec.Get().executed)
		assert.Equal(t, "script.ts", cli.Exec.Get().File.Get())
		assert.Equal(t, []string{"--version"}, cli.Exec.Get().Args.Get())
		assert.False(t, cli.executed)
	})

	t.Run("explicit name", func(t *testing.T) {
		t.Parallel()

		cli := new(implicitRunnableRootCLI)
		app := conga.New(cli).Name("tool").Version("1.0.0").Args([]string{"run", "script.ts", "--version"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Exec.Get().executed)
		assert.Equal(t, "script.ts", cli.Exec.Get().File.Get())
		assert.Equal(t, []string{"--version"}, cli.Exec.Get().Args.Get())
		assert.False(t, cli.executed)
	})
}

func TestImplicitCommand_ExplicitNameBeforeVersionStaysUnknown(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).Name("tool").Version("1.0.0").Args([]string{"run", "--version"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), "unknown flag: --version")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_ChildFlagBeforeWhyRendersProvenance(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := new(implicitRunnableFlagsRoot)
	app := conga.New(cli).Name("tool").EnableWhy().Stdout(&stdoutBuffer).Args([]string{"-n", "--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "Value provenance")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_HelpShortInClusterSelectsImplicit(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	cli := new(implicitRunnableFlagsRoot)
	app := conga.New(cli).Name("tool").HelpShortFlag('h').Stdout(&stdoutBuffer).Args([]string{"-hn"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "Usage:\n  tool run")
	assert.NotContains(t, stdoutBuffer.String(), "Usage:\n  tool [<command>]")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}

func TestImplicitCommand_TerminatorOperandsFollowFlagSelectedChain(t *testing.T) {
	t.Parallel()

	cli := new(implicitDashRootCLI)
	app := conga.New(cli).Name("tool").Args([]string{"--mid-flag", "--", "x"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Mid.Get().Exec.Get().executed)
	assert.Equal(t, "tool mid run", cli.Mid.Get().Exec.Get().path)
	assert.Equal(t, []string{"x"}, cli.Mid.Get().Exec.Get().rest)
	assert.False(t, cli.Mid.Get().executed)
	assert.False(t, cli.executed)
}

func TestImplicitCommand_DeepChainFlagSelectsNeededDepth(t *testing.T) {
	t.Parallel()

	t.Run("mid flag stops at the runnable intermediate", func(t *testing.T) {
		t.Parallel()

		cli := new(implicitDeepRunnableRootCLI)
		app := conga.New(cli).Name("tool").Args([]string{"--mid-flag"})

		err := app.Run()

		assert.NoError(t, err)
		assert.True(t, cli.Mid.Get().executed)
		assert.False(t, cli.Mid.Get().Exec.Get().executed)
	})

	t.Run("leaf flag descends to the implicit leaf", func(t *testing.T) {
		t.Parallel()

		cli := new(implicitDeepRunnableRootCLI)
		app := conga.New(cli).Name("tool").Args([]string{"--format", "json"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "missing required positional argument <file>", err.Error())

		var usage conga.UsageError

		assert.True(t, errors.As(err, &usage))
		assert.Equal(t, "tool mid run", usage.CommandPath())
		assert.False(t, cli.Mid.Get().executed)
	})

	t.Run("attached value descends to the implicit leaf", func(t *testing.T) {
		t.Parallel()

		cli := new(implicitDeepRunnableRootCLI)
		app := conga.New(cli).Name("tool").Args([]string{"--format=json"})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "missing required positional argument <file>", err.Error())

		var usage conga.UsageError

		assert.True(t, errors.As(err, &usage))
		assert.Equal(t, "tool mid run", usage.CommandPath())
		assert.False(t, cli.Mid.Get().executed)
	})
}

func TestImplicitCommand_ShortClusterValueSkipsFlagLikeValue(t *testing.T) {
	t.Parallel()

	cli := new(implicitClusterValueRootCLI)
	app := conga.New(cli).Name("tool").Args([]string{"-no", "-5"})

	err := app.Run()

	assert.NoError(t, err)
	assert.True(t, cli.Mid.Get().executed)
	assert.True(t, cli.Mid.Get().dryRun)
	assert.Equal(t, "-5", cli.Mid.Get().output)
	assert.False(t, cli.Mid.Get().Exec.Get().executed)
}

func TestImplicitCommand_RootHelpListsVersion(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[implicitRunnableRootCLI]().Name("tool").Version("1.0.0").Stdout(&stdoutBuffer).Args([]string{"--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "--version")
}

func TestImplicitCommand_ImplicitHelpOmitsRootVersion(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	app := conga.New[implicitRunnableRootCLI]().Name("tool").Version("1.0.0").Stdout(&stdoutBuffer).Args([]string{"-n", "--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), "Usage:\n  tool run")
	assert.NotContains(t, stdoutBuffer.String(), "--version")
}

func TestImplicitCommand_VersionBeforeExplicitNameStaysUnknown(t *testing.T) {
	t.Parallel()

	cli := new(implicitRunnableRootCLI)
	app := conga.New(cli).Name("tool").Version("1.0.0").Args([]string{"--version", "run"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), "unknown flag: --version")
	assert.False(t, cli.executed)
	assert.False(t, cli.Exec.Get().executed)
}
