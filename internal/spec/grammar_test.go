package spec_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/spec"
)

type grammarLeafCLI struct {
	Verbose mockFlag[bool] `name:"verbose"`
}

type rawWithSubcommandCLI struct {
	Deploy mockCmd[grammarLeafCLI]
	Raw    mockRawArgs
}

type rawWithBuiltinHelpCLI struct {
	Help mockHelpCmd `summary:"Show help"`
	Raw  mockRawArgs
}

type rawWithOptionalArgCLI struct {
	Target mockArg[string]
	Raw    mockRawArgs
}

type rawWithSliceArgCLI struct {
	Targets mockArg[[]string]
	Raw     mockRawArgs
}

type rawWithRequiredArgCLI struct {
	Target mockReqArg[string]
	Raw    mockRawArgs
}

type dashWithSubcommandCLI struct {
	Deploy mockCmd[grammarLeafCLI]
	Dash   mockDashArgs
}

type dashWithOptionalArgCLI struct {
	Target mockArg[string]
	Dash   mockDashArgs
}

type rawOnlyCLI struct {
	Raw mockRawArgs
}

type rawWithFlagCLI struct {
	Verbose mockFlag[bool] `name:"verbose"`
	Raw     mockRawArgs
}

type leafArgsCLI struct {
	Target mockReqArg[string]
}

type branchOnlyCLI struct {
	Deploy mockCmd[grammarLeafCLI]
}

type mixedBranchCLI struct {
	Target mockReqArg[string]
	Deploy mockCmd[grammarLeafCLI]
}

type duplicateRawArgsCLI struct {
	First  mockRawArgs
	Second mockRawArgs
}

type duplicateDashArgsCLI struct {
	First  mockDashArgs
	Second mockDashArgs
}

type bothCollectorsCLI struct {
	Dash mockDashArgs
	Raw  mockRawArgs
}

func TestValidateBranchGrammar(t *testing.T) {
	t.Parallel()

	t.Run("no conflict", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateBranchGrammar(spec.BranchGrammar{})

		assert.NoError(t, err)
	})

	t.Run("positional only", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateBranchGrammar(spec.BranchGrammar{PositionalField: "Target"})

		assert.NoError(t, err)
	})

	t.Run("subcommand only", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateBranchGrammar(spec.BranchGrammar{SubcommandField: "Deploy"})

		assert.NoError(t, err)
	})

	t.Run("both errors", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateBranchGrammar(spec.BranchGrammar{PositionalField: "Target", SubcommandField: "Deploy"})

		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `schema error: positional argument Target cannot be combined with subcommand Deploy; a command is either a branch (subcommands) or a leaf (positional arguments): move Target into a leaf subcommand or pass the value through a flag`, err.Error())
	})
}

func TestBuild_BranchGrammar(t *testing.T) {
	t.Parallel()

	t.Run("positional arguments alone build", func(t *testing.T) {
		t.Parallel()

		var cli leafArgsCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmdSpec.Args))
		assert.Equal(t, 0, len(cmdSpec.Subcommands))
	})

	t.Run("subcommands alone build", func(t *testing.T) {
		t.Parallel()

		var cli branchOnlyCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 0, len(cmdSpec.Args))
		assert.Equal(t, 1, len(cmdSpec.Subcommands))
	})

	t.Run("mixed command errors", func(t *testing.T) {
		t.Parallel()

		var cli mixedBranchCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `struct mixedBranchCLI field Deploy: schema error: positional argument Target cannot be combined with subcommand Deploy; a command is either a branch (subcommands) or a leaf (positional arguments): move Target into a leaf subcommand or pass the value through a flag`, err.Error())
	})
}

func TestValidateTrailingGrammar(t *testing.T) {
	t.Parallel()

	t.Run("no raw args", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateTrailingGrammar(spec.TrailingGrammar{})

		assert.NoError(t, err)
	})

	t.Run("required scalar prefix", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateTrailingGrammar(spec.TrailingGrammar{
			HasRawArgs: true,
			Args:       []spec.TrailingArgGrammar{{Name: "<target>", Required: true}},
		})

		assert.NoError(t, err)
	})

	t.Run("command field error", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateTrailingGrammar(spec.TrailingGrammar{HasRawArgs: true, CommandField: "Deploy"})

		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `schema error: raw args cannot be combined with command field Deploy (declared built-in help, version, and completion command fields also count); a command is either a branch (subcommands) or a leaf (positional/raw arguments): use DashArgs for '--' forwarding or declare RawArgs on a leaf command`, err.Error())
	})

	t.Run("optional argument errors", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateTrailingGrammar(spec.TrailingGrammar{
			HasRawArgs: true,
			Args:       []spec.TrailingArgGrammar{{Name: "<target>"}},
		})

		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `schema error: raw args requires a deterministic positional prefix: positional argument <target> must be a required scalar; declare only required scalar positional arguments before RawArgs, or use DashArgs for '--' forwarding`, err.Error())
	})

	t.Run("slice argument errors", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateTrailingGrammar(spec.TrailingGrammar{
			HasRawArgs: true,
			Args:       []spec.TrailingArgGrammar{{Name: "<targets>", Required: true, IsSlice: true}},
		})

		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `schema error: raw args requires a deterministic positional prefix: positional argument <targets> must be a required scalar; declare only required scalar positional arguments before RawArgs, or use DashArgs for '--' forwarding`, err.Error())
	})

	t.Run("map argument errors", func(t *testing.T) {
		t.Parallel()

		err := spec.ValidateTrailingGrammar(spec.TrailingGrammar{
			HasRawArgs: true,
			Args:       []spec.TrailingArgGrammar{{Name: "<labels>", Required: true, IsMap: true}},
		})

		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `schema error: raw args requires a deterministic positional prefix: positional argument <labels> must be a required scalar; declare only required scalar positional arguments before RawArgs, or use DashArgs for '--' forwarding`, err.Error())
	})
}

func TestBuild_RawArgsGrammar(t *testing.T) {
	t.Parallel()

	t.Run("raw args alone builds", func(t *testing.T) {
		t.Parallel()

		var cli rawOnlyCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.True(t, cmdSpec.RawArgs != nil)
	})

	t.Run("raw args with a flag builds", func(t *testing.T) {
		t.Parallel()

		var cli rawWithFlagCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.True(t, len(cmdSpec.Flags) > 0)
		assert.True(t, cmdSpec.RawArgs != nil)
	})

	t.Run("required scalar prefix builds", func(t *testing.T) {
		t.Parallel()

		var cli rawWithRequiredArgCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmdSpec.Args))
		assert.True(t, cmdSpec.RawArgs != nil)
	})

	t.Run("raw args with subcommands errors", func(t *testing.T) {
		t.Parallel()

		var cli rawWithSubcommandCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `struct rawWithSubcommandCLI field Raw: schema error: raw args cannot be combined with command field Deploy (declared built-in help, version, and completion command fields also count); a command is either a branch (subcommands) or a leaf (positional/raw arguments): use DashArgs for '--' forwarding or declare RawArgs on a leaf command`, err.Error())
	})

	t.Run("raw args with built-in help errors", func(t *testing.T) {
		t.Parallel()

		var cli rawWithBuiltinHelpCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `struct rawWithBuiltinHelpCLI field Raw: schema error: raw args cannot be combined with command field Help (declared built-in help, version, and completion command fields also count); a command is either a branch (subcommands) or a leaf (positional/raw arguments): use DashArgs for '--' forwarding or declare RawArgs on a leaf command`, err.Error())
	})

	t.Run("optional positional before raw args errors", func(t *testing.T) {
		t.Parallel()

		var cli rawWithOptionalArgCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `struct rawWithOptionalArgCLI field Raw: schema error: raw args requires a deterministic positional prefix: positional argument <target> must be a required scalar; declare only required scalar positional arguments before RawArgs, or use DashArgs for '--' forwarding`, err.Error())
	})

	t.Run("slice positional before raw args errors", func(t *testing.T) {
		t.Parallel()

		var cli rawWithSliceArgCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, `struct rawWithSliceArgCLI field Raw: schema error: raw args requires a deterministic positional prefix: positional argument <targets> must be a required scalar; declare only required scalar positional arguments before RawArgs, or use DashArgs for '--' forwarding`, err.Error())
	})

	t.Run("dash args with subcommands builds", func(t *testing.T) {
		t.Parallel()

		var cli dashWithSubcommandCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmdSpec.Subcommands))
		assert.True(t, cmdSpec.DashArgs != nil)
	})

	t.Run("dash args with optional positional builds", func(t *testing.T) {
		t.Parallel()

		var cli dashWithOptionalArgCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmdSpec.Args))
		assert.True(t, cmdSpec.DashArgs != nil)
	})
}

func TestBuild_TrailingCollectorUniqueness(t *testing.T) {
	t.Parallel()

	t.Run("duplicate raw args errors", func(t *testing.T) {
		t.Parallel()

		var cli duplicateRawArgsCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrDuplicateDeclaration)
		assert.Equal(t, `struct duplicateRawArgsCLI field Second: schema error: duplicate RawArgs field (already declared by field First)`, err.Error())
	})

	t.Run("duplicate dash args errors", func(t *testing.T) {
		t.Parallel()

		var cli duplicateDashArgsCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrDuplicateDeclaration)
		assert.Equal(t, `struct duplicateDashArgsCLI field Second: schema error: duplicate DashArgs field (already declared by field First)`, err.Error())
	})

	t.Run("one dash and one raw collector builds", func(t *testing.T) {
		t.Parallel()

		var cli bothCollectorsCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.True(t, cmdSpec.DashArgs != nil)
		assert.True(t, cmdSpec.RawArgs != nil)
	})
}
