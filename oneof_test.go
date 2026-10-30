package conga_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
)

type OneOfCLI struct {
	noopRunner

	JSON conga.OptionalFlag[bool] `name:"json" oneof:"format"`
	YAML conga.OptionalFlag[bool] `name:"yaml" oneof:"format"`
}

type OneOfEnvCLI struct {
	noopRunner

	JSON conga.OptionalFlag[bool] `env:"CONGA_TEST_ONEOF_JSON" name:"json" oneof:"format"`
	YAML conga.OptionalFlag[bool] `env:"CONGA_TEST_ONEOF_YAML" name:"yaml" oneof:"format"`
}

type OneOfConfigCLI struct {
	noopRunner

	JSON conga.OptionalFlag[bool] `config:"json" name:"json" oneof:"format"`
	YAML conga.OptionalFlag[bool] `config:"yaml" name:"yaml" oneof:"format"`
}

type OneOfSingleCLI struct {
	noopRunner

	JSON conga.OptionalFlag[bool] `name:"json" oneof:"format"`
}

type OneOfCounterCLI struct {
	noopRunner

	JSON conga.Counter `name:"json" oneof:"format"`
	YAML conga.Counter `name:"yaml" oneof:"format"`
}

type OneOfHiddenCLI struct {
	noopRunner

	JSON conga.OptionalFlag[bool] `name:"json"   oneof:"format"`
	YAML conga.OptionalFlag[bool] `name:"yaml"   oneof:"format"`
	Text conga.OptionalFlag[bool] `hidden:"true" name:"text"    oneof:"format"`
}

type OneOfRequiredCLI struct {
	noopRunner

	Key  conga.RequiredFlag[string] `name:"key"`
	JSON conga.OptionalFlag[bool]   `name:"json" oneof:"format"`
	YAML conga.OptionalFlag[bool]   `name:"yaml" oneof:"format"`
}

type OneOfSubCLI struct{}

func (*OneOfSubCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type OneOfRootCLI struct {
	JSON conga.OptionalFlag[bool]   `name:"json" oneof:"format"`
	YAML conga.OptionalFlag[bool]   `name:"yaml" oneof:"format"`
	Sub  conga.Command[OneOfSubCLI] `name:"sub"`
}

type OneOfScopedSubCLI struct {
	LeafJSON conga.OptionalFlag[bool] `name:"leaf-json" oneof:"choice"`
	LeafYAML conga.OptionalFlag[bool] `name:"leaf-yaml" oneof:"choice"`
}

func (*OneOfScopedSubCLI) Run(_ context.Context, _ *conga.Session) error { return nil }

type OneOfScopedRootCLI struct {
	RootJSON conga.OptionalFlag[bool]         `name:"root-json" oneof:"choice"`
	RootYAML conga.OptionalFlag[bool]         `name:"root-yaml" oneof:"choice"`
	Sub      conga.Command[OneOfScopedSubCLI] `name:"sub"`
}

type OneOfThreeCLI struct {
	noopRunner

	JSON conga.OptionalFlag[bool] `name:"json" oneof:"format"`
	YAML conga.OptionalFlag[bool] `name:"yaml" oneof:"format"`
	Text conga.OptionalFlag[bool] `name:"text" oneof:"format"`
}

type oneOfFeatureSet struct {
	JSON bool `help:"Enable the JSON format"`
	YAML bool `help:"Enable the YAML format"`
}

type OneOfGatedCLI struct {
	noopRunner

	Features conga.Features[oneOfFeatureSet] `flag:"feature-gates"`

	JSON conga.OptionalFlag[bool] `name:"json"    oneof:"format"`
	YAML conga.OptionalFlag[bool] `feature:"yaml" name:"yaml"    oneof:"format"`
}

type OneOfAllGatedCLI struct {
	noopRunner

	Features conga.Features[oneOfFeatureSet] `flag:"feature-gates"`

	JSON conga.OptionalFlag[bool] `feature:"json" name:"json" oneof:"format"`
	YAML conga.OptionalFlag[bool] `feature:"yaml" name:"yaml" oneof:"format"`
}

func TestOneOf_MissingGroup(t *testing.T) {
	t.Parallel()

	app := conga.New[OneOfCLI]().Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "missing required flag: one of --json, --yaml", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
}

func TestOneOf_OneSupplied(t *testing.T) {
	t.Parallel()

	cli := new(OneOfCLI)
	app := conga.New(cli).Args([]string{"--json"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.True(t, cli.JSON.IsSet())
	assert.True(t, cli.JSON.Changed())
	assert.False(t, cli.YAML.IsSet())
	assert.False(t, cli.YAML.Changed())
}

func TestOneOf_Conflict(t *testing.T) {
	t.Parallel()

	app := conga.New[OneOfCLI]().Args([]string{"--json", "--yaml"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --json and --yaml are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
}

func TestOneOf_ThreeMemberConflict(t *testing.T) {
	t.Parallel()

	app := conga.New[OneOfThreeCLI]().Args([]string{"--json", "--yaml"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --json and --yaml are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestOneOf_GatedMembers(t *testing.T) {
	t.Parallel()

	t.Run("pruned member leaves a single-member group", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OneOfGatedCLI]().
			EnableFeatures().
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "missing required flag: --json", err.Error())
	})

	t.Run("enabling the gate restores the alternation", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OneOfGatedCLI]().
			EnableFeatures().
			Args([]string{"--feature-gates", "yaml=true"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "missing required flag: one of --json, --yaml", err.Error())
	})

	t.Run("all members pruned skips the group", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OneOfAllGatedCLI]().
			EnableFeatures().
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})
}

func TestOneOf_SingleMemberUsesRequiredMessage(t *testing.T) {
	t.Parallel()

	app := conga.New[OneOfSingleCLI]().Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "missing required flag: --json", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestOneOf_CounterMember(t *testing.T) {
	t.Parallel()

	cli := new(OneOfCounterCLI)
	app := conga.New(cli).Args([]string{"--json"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, 1, cli.JSON.Get())
	assert.True(t, cli.JSON.Changed())
	assert.False(t, cli.YAML.IsSet())
}

func TestOneOf_EnvCounts(t *testing.T) {
	t.Setenv("CONGA_TEST_ONEOF_JSON", "true")

	cli := new(OneOfEnvCLI)
	app := conga.New(cli).Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.True(t, cli.JSON.Changed())
	assert.False(t, cli.YAML.Changed())
}

func TestOneOf_EnvAndCLIConflict(t *testing.T) {
	t.Setenv("CONGA_TEST_ONEOF_YAML", "true")

	app := conga.New[OneOfEnvCLI]().Args([]string{"--json"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --json and --yaml are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestOneOf_ConfigCounts(t *testing.T) {
	t.Parallel()

	cli := new(OneOfConfigCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"json": true}`))).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.True(t, cli.JSON.Changed())
	assert.False(t, cli.YAML.Changed())
}

func TestOneOf_HiddenMemberSatisfiesGroup(t *testing.T) {
	t.Parallel()

	app := conga.New[OneOfHiddenCLI]().Args([]string{"--text"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
}

func TestOneOf_MissingNamesHiddenMembers(t *testing.T) {
	t.Parallel()

	app := conga.New[OneOfHiddenCLI]().Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "missing required flag: one of --json, --yaml, --text", err.Error())
}

func TestOneOf_AncestorGroupEnforced(t *testing.T) {
	t.Parallel()

	app := conga.New[OneOfRootCLI]().Args([]string{"sub"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "missing required flag: one of --json, --yaml", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}

func TestOneOf_AncestorGroupSatisfied(t *testing.T) {
	t.Parallel()

	app := conga.New[OneOfRootCLI]().Args([]string{"--json", "sub"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
}

func TestOneOf_GroupsScopedPerCommand(t *testing.T) {
	t.Parallel()

	t.Run("one member per command satisfies both groups", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OneOfScopedRootCLI]().Args([]string{"--root-json", "sub", "--leaf-yaml"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("leaf group missing is reported independently", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OneOfScopedRootCLI]().Args([]string{"--root-json", "sub"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "missing required flag: one of --leaf-json, --leaf-yaml", err.Error())
	})

	t.Run("root group conflict does not involve leaf members", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OneOfScopedRootCLI]().Args([]string{"--root-json", "--root-yaml", "sub", "--leaf-json"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "flags --root-json and --root-yaml are mutually exclusive", err.Error())
	})

	t.Run("leaf group conflict does not involve root members", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OneOfScopedRootCLI]().Args([]string{"--root-json", "sub", "--leaf-json", "--leaf-yaml"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "flags --leaf-json and --leaf-yaml are mutually exclusive", err.Error())
	})
}

func TestOneOf_RequiredFlagErrorPrecedesGroupError(t *testing.T) {
	t.Parallel()

	app := conga.New[OneOfRequiredCLI]().Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "missing required flag: --key", err.Error())
}

func TestOneOf_WhySkipsMissingGroup(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[OneOfCLI]().
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--why"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Value provenance")
	assert.Contains(t, buffer.String(), "--json")
	assert.Contains(t, buffer.String(), "--yaml")
}

func TestOneOf_WhyReportsConflict(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[OneOfCLI]().
		EnableWhy().
		Stdout(&buffer).
		Args([]string{"--why", "--json", "--yaml"})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "flags --json and --yaml are mutually exclusive", err.Error())
	assert.IsError(t, err, conga.ErrValidation)
}
