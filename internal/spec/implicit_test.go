package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

type implicitLeafCLI struct {
	File mockReqArg[string]
}

type implicitRootCLI struct {
	Exec  mockCmd[implicitLeafCLI] `implicit:"true" name:"run"`
	Build mockCmd[implicitLeafCLI]
}

type implicitDuplicateCLI struct {
	First  mockCmd[implicitLeafCLI] `implicit:"true"`
	Second mockCmd[implicitLeafCLI] `implicit:"true"`
}

type implicitInvalidBoolCLI struct {
	Exec mockCmd[implicitLeafCLI] `implicit:"ture"`
}

func TestBuild_ImplicitCommand(t *testing.T) {
	t.Parallel()

	cmd, err := spec.Build(&implicitRootCLI{}, "tool", spec.Options{
		ClassifyContainer: classifyTestContainer,
		IsRunner:          func(any) bool { return true },
	})

	assert.NoError(t, err)
	assert.True(t, cmd.IsRunnable)
	assert.Equal(t, "run", cmd.ImplicitCommand().Name)
	assert.True(t, cmd.ImplicitCommand().Implicit)
	assert.True(t, cmd.ImplicitCommand().IsRunnable)
	assert.Equal(t, "build", cmd.Subcommands[1].Name)
	assert.False(t, cmd.Subcommands[1].Implicit)
}

func TestBuild_ImplicitCommandDuplicate(t *testing.T) {
	t.Parallel()

	_, err := spec.Build(&implicitDuplicateCLI{}, "tool", spec.Options{
		ClassifyContainer: classifyTestContainer,
	})

	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, "struct implicitDuplicateCLI field Second: schema error: only one implicit command is allowed per command; implicit command already declared by field First", err.Error())
}

func TestBuild_ImplicitCommandInvalidBool(t *testing.T) {
	t.Parallel()

	_, err := spec.Build(&implicitInvalidBoolCLI{}, "tool", spec.Options{
		ClassifyContainer: classifyTestContainer,
	})

	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, "struct implicitInvalidBoolCLI field Exec: schema error: invalid struct tag: tag \"implicit\" expects \"true\" or \"false\", got \"ture\"", err.Error())
}
