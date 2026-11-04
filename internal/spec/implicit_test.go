package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/internal/tokenizer"
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

func TestTerminalImplicitLevels(t *testing.T) {
	t.Parallel()

	t.Run("no implicit child", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "tool", IsRunnable: true}

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, nil, false)

		assert.Equal(t, 0, levels)
	})

	t.Run("runnable parent stays put", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true}
		root := &spec.Command{Name: "tool", IsRunnable: true, Subcommands: []*spec.Command{leaf}}

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, nil, false)

		assert.Equal(t, 0, levels)
	})

	t.Run("builtin request keeps the parent when no flag selects a child", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true}
		root := &spec.Command{Name: "tool", IsRunnable: true, Subcommands: []*spec.Command{leaf}}
		tokens := tokenizer.Tokenize([]string{"--help"})

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, tokens, false)

		assert.Equal(t, 0, levels)
	})

	t.Run("child-only flag selects the implicit child", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{
			Name:     "run",
			Implicit: true,
			Flags:    []*spec.Flag{{Name: "child", IsBool: true}},
		}
		root := &spec.Command{Name: "tool", IsRunnable: true, Subcommands: []*spec.Command{leaf}}
		tokens := tokenizer.Tokenize([]string{"--child"})

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, tokens, false)

		assert.Equal(t, 1, levels)
	})

	t.Run("builtin request cannot stop a deeper selected child", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{
			Name:     "run",
			Implicit: true,
			Flags:    []*spec.Flag{{Name: "child", IsBool: true}},
		}
		root := &spec.Command{Name: "tool", IsRunnable: true, Subcommands: []*spec.Command{leaf}}
		tokens := tokenizer.Tokenize([]string{"--child", "--help"})

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, tokens, false)

		assert.Equal(t, 1, levels)
	})

	t.Run("root-only version descends past a non-runnable intermediate", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true, IsRunnable: true}
		mid := &spec.Command{
			Name:        "mid",
			Implicit:    true,
			Flags:       []*spec.Flag{{Name: "mid-flag", IsBool: true}},
			Subcommands: []*spec.Command{leaf},
		}
		root := &spec.Command{Name: "tool", Subcommands: []*spec.Command{mid}}
		tokens := tokenizer.Tokenize([]string{"--mid-flag", "--version"})

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, tokens, false)

		assert.Equal(t, 2, levels)
	})

	t.Run("help stops at a non-runnable intermediate", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true, IsRunnable: true}
		mid := &spec.Command{
			Name:        "mid",
			Implicit:    true,
			Flags:       []*spec.Flag{{Name: "mid-flag", IsBool: true}},
			Subcommands: []*spec.Command{leaf},
		}
		root := &spec.Command{Name: "tool", Subcommands: []*spec.Command{mid}}
		tokens := tokenizer.Tokenize([]string{"--mid-flag", "--help"})

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, tokens, false)

		assert.Equal(t, 1, levels)
	})

	t.Run("version before terminator operands forwards past the branch", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true, IsRunnable: true}
		mid := &spec.Command{
			Name:        "mid",
			Implicit:    true,
			Flags:       []*spec.Flag{{Name: "mid-flag", IsBool: true}},
			Subcommands: []*spec.Command{leaf},
		}
		root := &spec.Command{Name: "tool", Subcommands: []*spec.Command{mid}}
		tokens := tokenizer.Tokenize([]string{"--mid-flag", "--version", "--", "x"})

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, tokens, true)

		assert.Equal(t, 2, levels)
	})

	t.Run("NoCLI long help shadow does not mask the builtin", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true}
		root := &spec.Command{
			Name:        "tool",
			Flags:       []*spec.Flag{{Name: "help", NoCLI: true}},
			Subcommands: []*spec.Command{leaf},
		}
		tokens := tokenizer.Tokenize([]string{"--help"})

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, tokens, false)

		assert.Equal(t, 0, levels)
	})

	t.Run("NoCLI short help shadow does not mask the builtin", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true}
		root := &spec.Command{
			Name:      "tool",
			HelpShort: 'h',
			Flags: []*spec.Flag{
				{Name: "help", NoCLI: true, Short: 'h', IsBool: true},
			},
			Subcommands: []*spec.Command{leaf},
		}
		tokens := tokenizer.Tokenize([]string{"-h"})

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, tokens, false)

		assert.Equal(t, 0, levels)
	})

	t.Run("non-runnable parent enters the implicit child", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true}
		root := &spec.Command{Name: "tool", Subcommands: []*spec.Command{leaf}}

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, nil, false)

		assert.Equal(t, 1, levels)
	})

	t.Run("terminator operands forward to the implicit child", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true}
		root := &spec.Command{Name: "tool", IsRunnable: true, Subcommands: []*spec.Command{leaf}}

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, nil, true)

		assert.Equal(t, 1, levels)
	})

	t.Run("dash args keep terminator operands on the parent", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true}
		root := &spec.Command{
			Name:       "tool",
			IsRunnable: true,
			DashArgs:   &spec.DashArgs{},
			Subcommands: []*spec.Command{
				leaf,
			},
		}

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, nil, true)

		assert.Equal(t, 0, levels)
	})

	t.Run("non-runnable parent with dash args forwards terminator operands", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true}
		root := &spec.Command{
			Name:     "tool",
			DashArgs: &spec.DashArgs{},
			Subcommands: []*spec.Command{
				leaf,
			},
		}

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, nil, true)

		assert.Equal(t, 1, levels)
	})

	t.Run("nested chain enters both selected levels", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{
			Name:     "run",
			Implicit: true,
			Flags:    []*spec.Flag{{Name: "deep", IsBool: true}},
		}
		mid := &spec.Command{Name: "mid", Implicit: true, Subcommands: []*spec.Command{leaf}}
		root := &spec.Command{Name: "tool", IsRunnable: true, Subcommands: []*spec.Command{mid}}
		tokens := tokenizer.Tokenize([]string{"--deep"})

		levels := spec.TerminalImplicitLevels([]*spec.Command{root}, tokens, false)

		assert.Equal(t, 2, levels)
	})
}
