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

func TestRoutingFlagConsumesNext(t *testing.T) {
	t.Parallel()

	t.Run("value flag on the routed chain", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "tool", Flags: []*spec.Flag{{Name: "output", Short: 'o'}}}
		tokens := tokenizer.Tokenize([]string{"--output", "value"})

		consumesNext := spec.RoutingFlagConsumesNext([]*spec.Command{root}, tokens[0])

		assert.True(t, consumesNext)
	})

	t.Run("value flag declared only by the implicit child", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true, Flags: []*spec.Flag{{Name: "format"}}}
		root := &spec.Command{Name: "tool", Subcommands: []*spec.Command{leaf}}
		tokens := tokenizer.Tokenize([]string{"--format", "json"})

		consumesNext := spec.RoutingFlagConsumesNext([]*spec.Command{root}, tokens[0])

		assert.True(t, consumesNext)
	})

	t.Run("root version builtin wins over an implicit value shadow", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true, Flags: []*spec.Flag{{Name: "version"}}}
		root := &spec.Command{Name: "tool", Subcommands: []*spec.Command{leaf}}
		tokens := tokenizer.Tokenize([]string{"--version", "script.ts"})

		consumesNext := spec.RoutingFlagConsumesNext([]*spec.Command{root}, tokens[0])

		assert.False(t, consumesNext)
	})

	t.Run("root version short wins over an implicit value shadow", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true, Flags: []*spec.Flag{{Name: "verbose", Short: 'V'}}}
		root := &spec.Command{Name: "tool", VersionShort: 'V', Subcommands: []*spec.Command{leaf}}
		tokens := tokenizer.Tokenize([]string{"-V", "script.ts"})

		consumesNext := spec.RoutingFlagConsumesNext([]*spec.Command{root}, tokens[0])

		assert.False(t, consumesNext)
	})

	t.Run("value-taking completion builtin consumes the shell", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "tool"}
		tokens := tokenizer.Tokenize([]string{"--completion", "bash"})

		consumesNext := spec.RoutingFlagConsumesNext([]*spec.Command{root}, tokens[0])

		assert.True(t, consumesNext)
	})

	t.Run("implicit flags of a sibling-entered parent stay invisible", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true, Flags: []*spec.Flag{{Name: "format"}}}
		admin := &spec.Command{Name: "admin"}
		root := &spec.Command{Name: "tool", Subcommands: []*spec.Command{leaf, admin}}
		tokens := tokenizer.Tokenize([]string{"--format", "purge"})

		consumesNext := spec.RoutingFlagConsumesNext([]*spec.Command{root, admin}, tokens[0])

		assert.False(t, consumesNext)
	})
}

func TestRoutingLeadingShortRegistered(t *testing.T) {
	t.Parallel()

	t.Run("rune declared by the implicit child", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true, Flags: []*spec.Flag{{Name: "five", Short: '5', IsBool: true}}}
		root := &spec.Command{Name: "tool", Subcommands: []*spec.Command{leaf}}
		tokens := tokenizer.Tokenize([]string{"-5"})

		registered := spec.RoutingLeadingShortRegistered([]*spec.Command{root}, tokens[0])

		assert.True(t, registered)
	})

	t.Run("builtin short rune", func(t *testing.T) {
		t.Parallel()

		root := &spec.Command{Name: "tool", HelpShort: '5'}
		tokens := tokenizer.Tokenize([]string{"-5"})

		registered := spec.RoutingLeadingShortRegistered([]*spec.Command{root}, tokens[0])

		assert.True(t, registered)
	})

	t.Run("unregistered rune", func(t *testing.T) {
		t.Parallel()

		leaf := &spec.Command{Name: "run", Implicit: true}
		root := &spec.Command{Name: "tool", Subcommands: []*spec.Command{leaf}}
		tokens := tokenizer.Tokenize([]string{"-5"})

		registered := spec.RoutingLeadingShortRegistered([]*spec.Command{root}, tokens[0])

		assert.False(t, registered)
	})
}

func TestFlagTokenResolves(t *testing.T) {
	t.Parallel()

	flags := []*spec.Flag{
		{Name: "verbose", Short: 'v', IsCounter: true},
		{Name: "quiet", Short: 'q', IsBool: true, IsNegatable: true},
		{Name: "output", Short: 'o'},
	}

	tests := []struct {
		name     string
		args     []string
		resolves bool
	}{
		{name: "declared long flag", args: []string{"--output"}, resolves: true},
		{name: "negated long flag", args: []string{"--no-quiet"}, resolves: true},
		{name: "builtin long flag", args: []string{"--help"}, resolves: true},
		{name: "unknown long flag", args: []string{"--unknown"}, resolves: false},
		{name: "repeated counter cluster", args: []string{"-vv"}, resolves: true},
		{name: "mixed boolean cluster", args: []string{"-vq"}, resolves: true},
		{name: "cluster ending in value remainder", args: []string{"-voxyz"}, resolves: true},
		{name: "builtin short in cluster", args: []string{"-vh"}, resolves: true},
		{name: "unknown rune in cluster", args: []string{"-vx"}, resolves: false},
		{name: "positional", args: []string{"deploy"}, resolves: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := &spec.Command{Name: "tool", HelpShort: 'h', Flags: flags}
			tokens := tokenizer.Tokenize(test.args)

			resolves := spec.FlagTokenResolves([]*spec.Command{root}, tokens[0])

			assert.Equal(t, test.resolves, resolves)
		})
	}
}

func TestBuiltinRequested(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		requested bool
	}{
		{name: "long help", args: []string{"--help", "bogus"}, requested: true},
		{name: "short help in cluster", args: []string{"-vh", "bogus"}, requested: true},
		{name: "help after terminator", args: []string{"--", "--help"}, requested: false},
		{name: "declared flags only", args: []string{"-v", "bogus"}, requested: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := &spec.Command{
				Name:      "tool",
				HelpShort: 'h',
				Flags:     []*spec.Flag{{Name: "verbose", Short: 'v', IsBool: true}},
			}
			tokens := tokenizer.Tokenize(test.args)

			requested := spec.BuiltinRequested([]*spec.Command{root}, tokens)

			assert.Equal(t, test.requested, requested)
		})
	}
}
