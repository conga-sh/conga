package conga_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
)

type XorDefaultCLI struct {
	noopRunner

	Graceful  conga.OptionalFlag[bool] `default:"true"   name:"graceful"  xor:"drain-mode"`
	Immediate conga.OptionalFlag[bool] `name:"immediate" xor:"drain-mode"`
}

type SemanticsCLI struct {
	noopRunner

	Name conga.OptionalFlag[string] `default:"initial" env:"SEMANTICS_NAME" name:"name"`
}

func TestChangedSemantics_DefaultSource(t *testing.T) {
	t.Parallel()

	cli := new(SemanticsCLI)
	app := conga.New(cli).
		Name("app").
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.True(t, cli.Name.IsSet())
	assert.False(t, cli.Name.Changed())
	assert.Equal(t, "initial", cli.Name.Get())
	assert.Equal(t, conga.SourceDefault, cli.Name.Source())
}

func TestChangedSemantics_EnvSource(t *testing.T) {
	t.Setenv("SEMANTICS_NAME", "from_env")

	cli := new(SemanticsCLI)
	app := conga.New(cli).
		Name("app").
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.True(t, cli.Name.IsSet())
	assert.True(t, cli.Name.Changed())
	assert.Equal(t, "from_env", cli.Name.Get())
	assert.Equal(t, conga.SourceEnv, cli.Name.Source())
}

func TestChangedSemantics_CLISource(t *testing.T) {
	t.Parallel()

	cli := new(SemanticsCLI)
	app := conga.New(cli).
		Name("app").
		Args([]string{"--name", "from_cli"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.True(t, cli.Name.IsSet())
	assert.True(t, cli.Name.Changed())
	assert.Equal(t, "from_cli", cli.Name.Get())
	assert.Equal(t, conga.SourceCLI, cli.Name.Source())
}

func TestXor_DefaultDoesNotConflict(t *testing.T) {
	t.Parallel()

	cli := new(XorDefaultCLI)
	app := conga.New(cli).
		Name("app").
		Args([]string{"--immediate"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.True(t, cli.Graceful.Get())
	assert.Equal(t, conga.SourceDefault, cli.Graceful.Source())
	assert.True(t, cli.Immediate.Get())
	assert.True(t, cli.Immediate.Changed())
}

func TestXor_ExplicitConflictStillDetected(t *testing.T) {
	t.Parallel()

	t.Run("both explicit flags provided", func(t *testing.T) {
		t.Parallel()

		app := conga.New[XorDefaultCLI]().
			Name("app").
			Args([]string{"--immediate", "--graceful"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("explicit false flag still counts as provided", func(t *testing.T) {
		t.Parallel()

		app := conga.New[XorDefaultCLI]().
			Name("app").
			Args([]string{"--immediate", "--graceful=false"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestXor_EnvCountsAsChanged(t *testing.T) {
	t.Setenv("XOR_GRACEFUL", "true")

	type XorEnvCLI struct {
		noopRunner

		Graceful  conga.OptionalFlag[bool] `env:"XOR_GRACEFUL" name:"graceful"  xor:"drain-mode"`
		Immediate conga.OptionalFlag[bool] `name:"immediate"   xor:"drain-mode"`
	}

	app := conga.New[XorEnvCLI]().
		Name("app").
		Args([]string{"--immediate"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
	assert.IsError(t, err, conga.ErrValidation)
}

func TestAndGroup_EnvTriggersGroup(t *testing.T) {
	t.Setenv("RT_PASS", "hunter2")

	type AndGroupCLI struct {
		noopRunner

		User     conga.OptionalFlag[string] `and:"auth" name:"user"`
		Password conga.OptionalFlag[string] `and:"auth" env:"RT_PASS" name:"pass"`
	}

	app := conga.New[AndGroupCLI]().
		Name("app").
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be provided together")
	assert.IsError(t, err, conga.ErrValidation)
}

func TestAndGroup_DefaultDoesNotTriggerGroup(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		User conga.OptionalFlag[string] `and:"auth" default:"anon" name:"user"`
		Pass conga.OptionalFlag[string] `and:"auth" name:"pass"`
	}

	app := conga.New[CLI]().
		Name("app").
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
}
