package spec_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/spec"
)

type mockFlag[T any] struct {
	//nolint:unused // read reflectively by spec.extractGenericType
	value T
}

func (m *mockFlag[T]) testClassification() container.Classification {
	return container.Classification{Kind: container.KindFlag}
}

type mockReqFlag[T any] struct {
	//nolint:unused // read reflectively by spec.extractGenericType
	value T
}

func (m *mockReqFlag[T]) testClassification() container.Classification {
	return container.Classification{Kind: container.KindFlag, Required: true}
}

type mockConfigFile struct{}

func (m *mockConfigFile) testClassification() container.Classification {
	return container.Classification{Kind: container.KindConfigFile}
}

type mockMultiConfigFile struct{}

func (m *mockMultiConfigFile) testClassification() container.Classification {
	return container.Classification{Kind: container.KindConfigFile}
}

type mockCounter struct {
	//nolint:unused // read reflectively by spec.extractGenericType
	value int
}

func (m *mockCounter) testClassification() container.Classification {
	return container.Classification{Kind: container.KindCounter}
}

type mockArg[T any] struct {
	//nolint:unused // read reflectively by spec.extractGenericType
	value    T
	required bool
}

func (m *mockArg[T]) testClassification() container.Classification {
	return container.Classification{Kind: container.KindArg, Required: m.required}
}

type mockCmd[T any] struct {
	inner T
}

func (m *mockCmd[T]) testClassification() container.Classification {
	return container.Classification{Kind: container.KindCommand, Target: &m.inner}
}

type mockFeatures[T any] struct {
	inner T
}

func (m *mockFeatures[T]) testClassification() container.Classification {
	return container.Classification{Kind: container.KindFeatures, Target: &m.inner}
}

// classifiedMock is implemented by the test container doubles to surface the
// engine classification their real counterparts declare.
type classifiedMock interface {
	testClassification() container.Classification
}

// classifyTestContainer is the test-side container classifier wiring the mock
// containers into the schema builder.
func classifyTestContainer(fieldAddress any) container.Classification {
	classified, ok := fieldAddress.(classifiedMock)
	if !ok {
		return container.Classification{}
	}

	return classified.testClassification()
}

// buildSchema builds a schema with the test container classifier and
// type-based predictor inference attached, matching the completion-enabled
// production path the spec tests exercise. Tests that assert the bare default
// (no inference without a hook) call spec.Build directly.
func buildSchema(target any, name string, opts spec.Options) (*spec.Command, error) {
	opts.ClassifyContainer = classifyTestContainer
	opts.InferPredictor = spec.InferPredictorFromType

	return spec.Build(target, name, opts)
}

type SchemaTestCLI struct {
	_ struct{} `envprefix:"TESTAPP_"`

	ServerPort mockFlag[int]       `default:"8080"    help:"Port"      short:"p"`
	DebugMode  mockFlag[bool]      `help:"Debug mode" short:"d"`
	Format     mockFlag[string]    `default:"json"    enum:"json,yaml"`
	SecretKey  mockReqFlag[string] `name:"key"`

	Config mockArg[string]   `name:"[config]"`
	Files  mockArg[[]string] `name:"<files...>"`

	Subcommand Subcommand `aliases:"s" help:"Subcommand" name:"sub"`
}

type Subcommand struct {
	Inner mockCmd[LeafCommand] `summary:"Leaf"`
}

type LeafCommand struct {
	Value mockArg[string]
}

func (l *LeafCommand) Run(ctx context.Context) error {
	return nil
}

type WhitespaceSubcommand struct{}

func (w *WhitespaceSubcommand) Description() string {
	return "\n  detailed description  \n"
}

func (w *WhitespaceSubcommand) Examples() string {
	return "\n  myapp deploy\n"
}

func (w *WhitespaceSubcommand) Run(ctx context.Context) error {
	return nil
}

type whitespaceOnlyDescCmd struct{}

func (w *whitespaceOnlyDescCmd) Description() string {
	return "   \n\t  \n"
}

func (w *whitespaceOnlyDescCmd) Run(ctx context.Context) error {
	return nil
}

func TestBuildSchema(t *testing.T) {
	t.Parallel()

	t.Run("valid schema", func(t *testing.T) {
		t.Parallel()

		var cli SchemaTestCLI

		cmdSpec, err := buildSchema(&cli, "myapp", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.Equal(t, "myapp", cmdSpec.Name)
		assert.Equal(t, "TESTAPP_", cmdSpec.EnvPrefix)
		assert.Equal(t, 4, len(cmdSpec.Flags))
		assert.Equal(t, 2, len(cmdSpec.Args))
		assert.False(t, cmdSpec.Args[0].IsSlice)
		assert.True(t, cmdSpec.Args[1].IsSlice)
		assert.Zero(t, cmdSpec.Subcommands)
	})

	t.Run("invalid non-struct target fails", func(t *testing.T) {
		t.Parallel()

		_, err := buildSchema("not-a-struct", "app", spec.Options{})

		assert.Error(t, err)
	})

	t.Run("duplicate flag names reject", func(t *testing.T) {
		t.Parallel()

		type DuplicateFlagCLI struct {
			Port1 mockFlag[int] `name:"port"`
			Port2 mockFlag[int] `name:"port"`
		}

		var cli DuplicateFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrDuplicateFlag)
	})

	t.Run("duplicate short flag reject", func(t *testing.T) {
		t.Parallel()

		type DuplicateShortFlagCLI struct {
			Port1 mockFlag[int] `short:"p"`
			Port2 mockFlag[int] `short:"p"`
		}

		var cli DuplicateShortFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrDuplicateFlag)
	})

	t.Run("duplicate subcommand names reject", func(t *testing.T) {
		t.Parallel()

		type DuplicateSubCLI struct {
			Subcommand1 mockCmd[LeafCommand] `name:"run"`
			Subcommand2 mockCmd[LeafCommand] `name:"run"`
		}

		var cli DuplicateSubCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrDuplicateFlag)
	})

	t.Run("child command flags collision with parent flags", func(t *testing.T) {
		t.Parallel()

		type ChildCLI struct {
			Port mockFlag[int]
		}

		type ParentCLI struct {
			Port  mockFlag[int]
			Child mockCmd[ChildCLI]
		}

		var cli ParentCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrDuplicateFlag)
	})

	t.Run("built-in commands recognized", func(t *testing.T) {
		t.Parallel()

		var cli BuiltinTestCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 3, len(cmdSpec.Subcommands))

		helpCmd := cmdSpec.FindSubcommand("help")
		assert.True(t, helpCmd.IsHelpCmd)
		assert.Equal(t, 1, len(helpCmd.Args))

		versionCmd := cmdSpec.FindSubcommand("version")
		assert.True(t, versionCmd.IsVersionCmd)

		completionCmd := cmdSpec.FindSubcommand("completion")
		assert.True(t, completionCmd.IsCompletionCmd)
	})

	t.Run("declared completion command without completion enabled rejected", func(t *testing.T) {
		t.Parallel()

		var cli BuiltinTestCLI

		_, err := buildSchema(&cli, "app", spec.Options{DisableCompletion: true})

		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct BuiltinTestCLI field Completion: schema error: completion command declared without completion enabled; call app.EnableCompletion() when constructing the app, or remove the field", err.Error())
	})

	t.Run("range on int with invalid bound rejected at build time", func(t *testing.T) {
		t.Parallel()

		type InvalidRangeBoundCLI struct {
			Port mockFlag[int] `range:"abc..10"`
		}

		var cli InvalidRangeBoundCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct InvalidRangeBoundCLI field Port: schema error: invalid range syntax: invalid integer min bound \"abc\"", err.Error())
	})

	t.Run("range on int with min greater than max rejected at build time", func(t *testing.T) {
		t.Parallel()

		type InvertedRangeCLI struct {
			Port mockFlag[int] `range:"10..1"`
		}

		var cli InvertedRangeCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct InvertedRangeCLI field Port: schema error: invalid range syntax: min bound \"10\" cannot be greater than max bound \"1\"", err.Error())
	})

	t.Run("range on duration with invalid bound rejected at build time", func(t *testing.T) {
		t.Parallel()

		type InvalidDurationRangeCLI struct {
			Timeout mockFlag[time.Duration] `range:"1s..invalid"`
		}

		var cli InvalidDurationRangeCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct InvalidDurationRangeCLI field Timeout: schema error: invalid range syntax: invalid duration max bound \"invalid\"", err.Error())
	})

	t.Run("range on string flag rejected at build time", func(t *testing.T) {
		t.Parallel()

		type StringRangeCLI struct {
			Name mockFlag[string] `range:"1..10"`
		}

		var cli StringRangeCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct StringRangeCLI field Name: schema error: invalid range syntax: range tag is not supported on type string", err.Error())
	})

	t.Run("len with invalid bound rejected at build time", func(t *testing.T) {
		t.Parallel()

		type InvalidLenCLI struct {
			Name mockFlag[string] `len:"abc..10"`
		}

		var cli InvalidLenCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct InvalidLenCLI field Name: schema error: invalid range syntax: invalid len min bound \"abc\" (must be non-negative integer)", err.Error())
	})

	t.Run("items with negative bound rejected at build time", func(t *testing.T) {
		t.Parallel()

		type InvalidItemsCLI struct {
			Items mockFlag[[]string] `items:"-1..5"`
		}

		var cli InvalidItemsCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct InvalidItemsCLI field Items: schema error: invalid range syntax: invalid items min bound \"-1\" (must be non-negative integer)", err.Error())
	})
}

func TestUintRangeValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid uint range accepted at build time", func(t *testing.T) {
		t.Parallel()

		type ValidUintRangeCLI struct {
			Port mockFlag[uint] `range:"1..65535"`
		}

		var cli ValidUintRangeCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
	})

	t.Run("valid range on slice of uint8 accepted at build time", func(t *testing.T) {
		t.Parallel()

		type ValidUintSliceRangeCLI struct {
			Octets mockFlag[[]uint8] `range:"0..255"`
		}

		var cli ValidUintSliceRangeCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
	})

	t.Run("inverted uint range rejected with exact message", func(t *testing.T) {
		t.Parallel()

		type InvertedUintRangeCLI struct {
			Port mockFlag[uint] `range:"100..1"`
		}

		var cli InvertedUintRangeCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct InvertedUintRangeCLI field Port: schema error: invalid range syntax: min bound \"100\" cannot be greater than max bound \"1\"", err.Error())
	})

	t.Run("non-numeric uint min bound rejected with exact message", func(t *testing.T) {
		t.Parallel()

		type InvalidUintBoundCLI struct {
			Port mockFlag[uint] `range:"abc..10"`
		}

		var cli InvalidUintBoundCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct InvalidUintBoundCLI field Port: schema error: invalid range syntax: invalid unsigned integer min bound \"abc\"", err.Error())
	})

	t.Run("negative uint bound rejected with exact message", func(t *testing.T) {
		t.Parallel()

		type NegativeUintBoundCLI struct {
			Port mockFlag[uint] `range:"-5..10"`
		}

		var cli NegativeUintBoundCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct NegativeUintBoundCLI field Port: schema error: invalid range syntax: invalid unsigned integer min bound \"-5\"", err.Error())
	})

	t.Run("uint64 overflow bound rejected with exact message", func(t *testing.T) {
		t.Parallel()

		type OverflowUintBoundCLI struct {
			Port mockFlag[uint64] `range:"1..99999999999999999999999"`
		}

		var cli OverflowUintBoundCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidRangeSyntax)
		assert.IsError(t, err, spec.ErrSchema)
		assert.Equal(t, "struct OverflowUintBoundCLI field Port: schema error: invalid range syntax: invalid unsigned integer max bound \"99999999999999999999999\"", err.Error())
	})
}

type mockHelpCmd struct{}

func (m *mockHelpCmd) testClassification() container.Classification {
	return container.Classification{Kind: container.KindHelpCommand, Target: m}
}

type mockVersionCmd struct{}

func (m *mockVersionCmd) testClassification() container.Classification {
	return container.Classification{Kind: container.KindVersionCommand, Target: m}
}

type mockCompletionCmd struct{}

func (m *mockCompletionCmd) testClassification() container.Classification {
	return container.Classification{Kind: container.KindCompletionCommand, Target: m}
}

type BuiltinTestCLI struct {
	Help       mockHelpCmd       `summary:"Help screen"`
	Version    mockVersionCmd    `summary:"Show version"`
	Completion mockCompletionCmd `summary:"Shell completion"`
}

func TestFindSubcommandAndFlag(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "root",
		Flags: []*spec.Flag{
			{Name: "port", Short: 'p', Aliases: []string{"pt"}},
			{Name: "host"},
		},
		Subcommands: []*spec.Command{
			{Name: "deploy", Aliases: []string{"dp"}},
			{Name: "build"},
		},
	}

	t.Run("find subcommand by name and alias", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "deploy", cmd.FindSubcommand("deploy").Name)
		assert.Equal(t, "deploy", cmd.FindSubcommand("dp").Name)
		assert.Equal(t, "build", cmd.FindSubcommand("build").Name)
		assert.Equal(t, (*spec.Command)(nil), cmd.FindSubcommand("nonexistent"))
	})

	t.Run("find flag by name, alias, and short", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "port", cmd.FindFlag("port").Name)
		assert.Equal(t, "port", cmd.FindFlag("pt").Name)
		assert.Equal(t, 'p', cmd.FindShortFlag('p').Short)
		assert.Equal(t, (*spec.Flag)(nil), cmd.FindFlag("missing"))
		assert.Equal(t, (*spec.Flag)(nil), cmd.FindShortFlag('x'))
	})
}

func TestFindFlagInChain(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "root",
		Flags: []*spec.Flag{
			{Name: "global", Short: 'g'},
			{Name: "shadowed", Help: "root"},
		},
	}
	subcommand := &spec.Command{
		Name: "sub",
		Flags: []*spec.Flag{
			{Name: "local", Short: 'l'},
			{Name: "shadowed", Help: "sub"},
		},
	}
	chain := []*spec.Command{root, subcommand}

	t.Run("finds leaf flag first on collision", func(t *testing.T) {
		t.Parallel()

		flag := spec.FindFlagInChain(chain, "shadowed")
		assert.Equal(t, "sub", flag.Help)
	})

	t.Run("finds parent flag", func(t *testing.T) {
		t.Parallel()

		flag := spec.FindFlagInChain(chain, "global")
		assert.Equal(t, "global", flag.Name)

		shortFlag := spec.FindShortFlagInChain(chain, 'g')
		assert.Equal(t, "global", shortFlag.Name)
	})

	t.Run("synthesizes help flag when not declared", func(t *testing.T) {
		t.Parallel()

		helpFlag := spec.FindFlagInChain(chain, "help")
		assert.Equal(t, "help", helpFlag.Name)
		assert.Equal(t, spec.DefaultHelpFlagHelp, helpFlag.Help)
	})

	t.Run("declared-only lookup finds declared flags", func(t *testing.T) {
		t.Parallel()

		flag := spec.FindDeclaredFlagInChain(chain, "global")
		assert.Equal(t, "global", flag.Name)
	})

	t.Run("declared-only lookup skips builtin help fallback", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, (*spec.Flag)(nil), spec.FindDeclaredFlagInChain(chain, "help"))
	})
}

func TestNewCompletionCommand(t *testing.T) {
	t.Parallel()

	cmd := spec.NewCompletionCommand("completion", spec.DefaultCompletionCommandHelp, nil, []string{"comp"}, false, []string{"bash", "zsh"})

	assert.Equal(t, "completion", cmd.Name)
	assert.Equal(t, []string{"comp"}, cmd.Aliases)
	assert.Equal(t, spec.DefaultCompletionCommandHelp, cmd.Summary)
	assert.True(t, cmd.IsCompletionCmd)
	assert.Equal(t, 1, len(cmd.Args))
	assert.Equal(t, "<shell>", cmd.Args[0].Name)
	assert.False(t, cmd.Args[0].Required)
	assert.Equal(t, []string{"bash", "zsh"}, cmd.Args[0].Enum)
}

func TestSubcommand_SummaryTag(t *testing.T) {
	t.Parallel()

	t.Run("optional when omitted", func(t *testing.T) {
		t.Parallel()

		type SubCmd struct{}

		type RootCLI struct {
			Deploy mockCmd[SubCmd]
		}

		var cli RootCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmd.Subcommands))
		assert.Equal(t, "", cmd.Subcommands[0].Summary)
	})

	t.Run("valid single-line summary is populated", func(t *testing.T) {
		t.Parallel()

		type SubCmd struct{}

		type RootCLI struct {
			Deploy mockCmd[SubCmd] `summary:"Deploy the service"`
		}

		var cli RootCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmd.Subcommands))
		assert.Equal(t, "Deploy the service", cmd.Subcommands[0].Summary)
	})

	t.Run("empty summary is rejected", func(t *testing.T) {
		t.Parallel()

		type SubCmd struct{}

		type RootCLI struct {
			Deploy mockCmd[SubCmd] `summary:""`
		}

		var cli RootCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, "struct RootCLI field Deploy: schema error: invalid struct tag: tag 'summary' cannot be empty", err.Error())
	})

	t.Run("whitespace-only summary is rejected", func(t *testing.T) {
		t.Parallel()

		type SubCmd struct{}

		type RootCLI struct {
			Deploy mockCmd[SubCmd] `summary:"   "`
		}

		var cli RootCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, "struct RootCLI field Deploy: schema error: invalid struct tag: tag 'summary' cannot be empty", err.Error())
	})

	t.Run("newline in summary is rejected", func(t *testing.T) {
		t.Parallel()

		type SubCmd struct{}

		type RootCLI struct {
			Deploy mockCmd[SubCmd] `summary:"line1\nline2"`
		}

		var cli RootCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, "struct RootCLI field Deploy: schema error: invalid struct tag: tag 'summary' must be a single line without newlines", err.Error())
	})

	t.Run("help tag on subcommand is rejected", func(t *testing.T) {
		t.Parallel()

		type SubCmd struct{}

		type RootCLI struct {
			Deploy mockCmd[SubCmd] `help:"Deploy service"`
		}

		var cli RootCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, "struct RootCLI field Deploy: schema error: invalid struct tag: tag 'help' is not supported on Command[T]; use 'summary' for the 1-line synopsis", err.Error())
	})

	t.Run("builtin command defaults to standard summary", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Help mockHelpCmd
		}

		var cli RootCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmd.Subcommands))
		assert.Equal(t, spec.DefaultHelpCommandHelp, cmd.Subcommands[0].Summary)
	})

	t.Run("builtin command custom summary overrides default", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Help mockHelpCmd `summary:"Custom help synopsis"`
		}

		var cli RootCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmd.Subcommands))
		assert.Equal(t, "Custom help synopsis", cmd.Subcommands[0].Summary)
	})

	t.Run("builtin command empty summary is rejected", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Help mockHelpCmd `summary:"  "`
		}

		var cli RootCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, "struct RootCLI field Help: schema error: invalid struct tag: tag 'summary' cannot be empty", err.Error())
	})

	t.Run("builtin command newline summary is rejected", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Help mockHelpCmd `summary:"line1\nline2"`
		}

		var cli RootCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, "struct RootCLI field Help: schema error: invalid struct tag: tag 'summary' must be a single line without newlines", err.Error())
	})

	t.Run("builtin command help tag is rejected", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Help mockHelpCmd `help:"Help text"`
		}

		var cli RootCLI

		_, err := buildSchema(&cli, "app", spec.Options{})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, "struct RootCLI field Help: schema error: invalid struct tag: tag 'help' is not supported on built-in command fields; use 'summary' for the 1-line synopsis", err.Error())
	})

	t.Run("subcommand with conga ignore permits help tag", func(t *testing.T) {
		t.Parallel()

		type SubCmd struct{}

		type RootCLI struct {
			Deploy mockCmd[SubCmd] `conga:"ignore" help:"Deploy service"`
		}

		var cli RootCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Zero(t, cmd.Subcommands)
	})

	t.Run("builtin command with conga ignore permits help tag", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Help mockHelpCmd `conga:"ignore" help:"Help text"`
		}

		var cli RootCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Zero(t, cmd.Subcommands)
	})
}

func TestEnvPrefix_SubcommandAndBuiltin(t *testing.T) {
	t.Parallel()

	t.Run("subcommand envprefix configures child without mutating root", func(t *testing.T) {
		t.Parallel()

		type SubCmd struct {
			Port mockFlag[int]
		}

		type RootCLI struct {
			_      struct{}        `envprefix:"ROOT_"`
			Deploy mockCmd[SubCmd] `envprefix:"DEPLOY_"`
		}

		var cli RootCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.Equal(t, "ROOT_", cmd.EnvPrefix)
		assert.Equal(t, 1, len(cmd.Subcommands))
		assert.Equal(t, "DEPLOY_", cmd.Subcommands[0].EnvPrefix)
	})

	t.Run("builtin command envprefix is rejected with clarified error", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Help mockHelpCmd `envprefix:"APP_"`
		}

		var cli RootCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct RootCLI field Help: schema error: invalid struct tag: tag "envprefix" is invalid on help command (tag "envprefix" is only valid on subcommands and sentinel fields)`, err.Error())
	})

	t.Run("builtin command deprecated tag is rejected with clarified error", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Help mockHelpCmd `deprecated:"legacy"`
		}

		var cli RootCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct RootCLI field Help: schema error: invalid struct tag: tag "deprecated" is invalid on help command (tag "deprecated" is only valid on subcommands, flags, and config file flags)`, err.Error())
	})
}

func TestEnvPrefix_FlagDerivation(t *testing.T) {
	t.Parallel()

	type CLI struct {
		Config mockConfigFile
		Port   mockFlag[int] `env:"PORT"`
		Host   mockFlag[string]
		Secret mockFlag[string] `env:"-"`
	}

	var cli CLI

	cmd, err := buildSchema(&cli, "app", spec.Options{
		EnvPrefix: "APP",
	})

	assert.NoError(t, err)
	assert.Equal(t, "APP_", cmd.EnvPrefix)

	assert.Equal(t, []string{"APP_CONFIG"}, cmd.ConfigFileFlag.EnvVars)

	portFlag := cmd.Flags[1]
	assert.Equal(t, "port", portFlag.Name)
	assert.Equal(t, []string{"PORT"}, portFlag.EnvVars)
	assert.False(t, portFlag.EnvExcluded)

	hostFlag := cmd.Flags[2]
	assert.Equal(t, "host", hostFlag.Name)
	assert.Equal(t, []string{"APP_HOST"}, hostFlag.EnvVars)
	assert.False(t, hostFlag.EnvExcluded)

	secretFlag := cmd.Flags[3]
	assert.Equal(t, "secret", secretFlag.Name)
	assert.Zero(t, secretFlag.EnvVars)
	assert.True(t, secretFlag.EnvExcluded)
}

func TestSubcommand_MetadataWhitespaceTrimmed(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Subcommand mockCmd[WhitespaceSubcommand] `summary:"Sub command"`
	}

	var cli RootCLI

	cmd, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, 1, len(cmd.Subcommands))
	assert.Equal(t, "detailed description", cmd.Subcommands[0].Description)
	assert.Equal(t, "myapp deploy", cmd.Subcommands[0].Examples)
}

func TestSubcommand_WhitespaceOnlyDescriptionIgnored(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Subcommand mockCmd[whitespaceOnlyDescCmd] `summary:"Fallback summary"`
	}

	var cli RootCLI

	cmd, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, 1, len(cmd.Subcommands))
	assert.Equal(t, "", cmd.Subcommands[0].Description)
	assert.Equal(t, "Fallback summary", cmd.Subcommands[0].Summary)
}

func TestFlag_DeprecatedRequiredRejected(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Key mockReqFlag[string] `deprecated:"use --token instead"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct RootCLI field Key: schema error: invalid struct tag: tag "deprecated" cannot be used on required flags (make the flag optional before deprecating it so callers can migrate without error)`, err.Error())
}

func TestFlag_HelpVariablesExpanded(t *testing.T) {
	t.Parallel()

	t.Run("range variable expanded", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Port mockFlag[int] `help:"Port ({{range}})" range:"1..10"`
		}

		var cli RootCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, "Port (1..10)", cmd.Flags[0].Help)
		assert.True(t, cmd.Flags[0].HelpVars.Range)
		assert.False(t, cmd.Flags[0].HelpVars.Enum)
	})

	t.Run("enum variable expanded", func(t *testing.T) {
		t.Parallel()

		type RootCLI struct {
			Env mockFlag[string] `enum:"dev,prod" help:"Env ({{enum}})"`
		}

		var cli RootCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.Equal(t, "Env (dev, prod)", cmd.Flags[0].Help)
		assert.True(t, cmd.Flags[0].HelpVars.Enum)
		assert.False(t, cmd.Flags[0].HelpVars.Range)
	})
}

func TestFlag_HelpUndeclaredVariableRejected(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Port mockFlag[int] `help:"Port ({{range}})"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct RootCLI field Port: schema error: invalid struct tag: tag "help" references {{range}}, but the field provides no value for that variable`, err.Error())
}

func TestArg_HelpEnvVariableRejected(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		_ struct{} `envprefix:"APP_"`

		Target mockArg[string] `help:"Target ({{env}})"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct RootCLI field Target: schema error: invalid struct tag: tag "help" references {{env}}, but the field provides no value for that variable`, err.Error())
}

func TestFlag_HelpEnvVariableDerivedFromPrefix(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		_ struct{} `envprefix:"APP_"`

		Port mockFlag[int] `help:"Port from {{env}}"`
	}

	var cli RootCLI

	cmd, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, "Port from APP_PORT", cmd.Flags[0].Help)
	assert.True(t, cmd.Flags[0].HelpVars.Env)
}

func TestFlag_HelpUnknownVariableRejected(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Port mockFlag[int] `help:"Port ({{rnage}})" range:"1..10"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct RootCLI field Port: schema error: invalid struct tag: tag "help" references unknown variable {{rnage}}; supported variables are {{range}}, {{len}}, {{items}}, {{enum}}, {{default}}, {{env}}`, err.Error())
}

func TestArg_HelpUnknownVariableRejected(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Channel mockArg[string] `enum:"slack,email" help:"Channel ({{enums}})"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct RootCLI field Channel: schema error: invalid struct tag: tag "help" references unknown variable {{enums}}; supported variables are {{range}}, {{len}}, {{items}}, {{enum}}, {{default}}, {{env}}`, err.Error())
}

func TestConfigFile_HelpEnvVariableExpanded(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Config mockConfigFile `env:"APP_CONFIG" help:"Load ({{env}})"`
	}

	var cli RootCLI

	cmd, err := buildSchema(&cli, "app", spec.Options{ConfigFilePatterns: []string{"*.yaml"}})

	assert.NoError(t, err)
	assert.Equal(t, "Load (APP_CONFIG)", cmd.ConfigFileFlag.Help)
	assert.True(t, cmd.ConfigFileFlag.HelpVars.Env)
}

func TestConfigFile_HelpEnvVariableRejected(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Config mockConfigFile `help:"Load ({{env}})"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct RootCLI field Config: schema error: invalid struct tag: tag "help" references {{env}}, but the field provides no value for that variable`, err.Error())
}

func TestConfigFile_HelpUnknownVariableRejected(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Config mockConfigFile `help:"Config ({{rnage}})"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct RootCLI field Config: schema error: invalid struct tag: tag "help" references unknown variable {{rnage}}; supported variables are {{range}}, {{len}}, {{items}}, {{enum}}, {{default}}, {{env}}`, err.Error())
}

func TestFlag_EmptyConstraintTagRejected(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		newCLI   func() any
		expected string
	}{
		{
			name: "empty range",
			newCLI: func() any {
				type CLI struct {
					Port mockFlag[int] `range:""`
				}

				return &CLI{}
			},
			expected: `struct CLI field Port: schema error: invalid struct tag: tag "range" cannot be empty`,
		},
		{
			name: "empty len",
			newCLI: func() any {
				type CLI struct {
					Port mockFlag[string] `len:""`
				}

				return &CLI{}
			},
			expected: `struct CLI field Port: schema error: invalid struct tag: tag "len" cannot be empty`,
		},
		{
			name: "empty items",
			newCLI: func() any {
				type CLI struct {
					Port mockFlag[[]string] `items:""`
				}

				return &CLI{}
			},
			expected: `struct CLI field Port: schema error: invalid struct tag: tag "items" cannot be empty`,
		},
		{
			name: "empty enum",
			newCLI: func() any {
				type CLI struct {
					Port mockFlag[string] `enum:""`
				}

				return &CLI{}
			},
			expected: `struct CLI field Port: schema error: invalid struct tag: tag "enum" cannot be empty`,
		},
		{
			name: "comma-only enum",
			newCLI: func() any {
				type CLI struct {
					Port mockFlag[string] `enum:","`
				}

				return &CLI{}
			},
			expected: `struct CLI field Port: schema error: invalid struct tag: tag "enum" cannot be empty`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildSchema(testCase.newCLI(), "app", spec.Options{})

			assert.Error(t, err)
			assert.IsError(t, err, spec.ErrSchema)
			assert.IsError(t, err, spec.ErrInvalidStructTag)
			assert.Equal(t, testCase.expected, err.Error())
		})
	}
}

func TestRootCommand_NamePreservedWithoutKebab(t *testing.T) {
	t.Parallel()

	type RootCLI struct{}

	var cli RootCLI

	cmd, err := buildSchema(&cli, "my_custom_tool", spec.Options{})

	assert.NoError(t, err)
	assert.Equal(t, "my_custom_tool", cmd.Name)
}

type summaryMethodCmd struct{}

func (summaryMethodCmd) Summary() string { return "unused" }

type deprecatedMethodCmd struct{}

func (deprecatedMethodCmd) Deprecated() string { return "unused" }

type helpFooterMethodCmd struct{}

func (helpFooterMethodCmd) HelpFooter() string { return "unused" }

type rootSummaryCLI struct{}

func (rootSummaryCLI) Summary() string { return "unused" }

type rootDescriptionCLI struct{}

func (rootDescriptionCLI) Description() string { return "unused" }

type rootHelpFooterCLI struct{}

func (rootHelpFooterCLI) HelpFooter() string { return "unused" }

type rootDeprecatedCLI struct{}

func (rootDeprecatedCLI) Deprecated() string { return "unused" }

type rootExamplesCLI struct{}

func (rootExamplesCLI) Examples() string { return "unused" }

func TestSubcommand_MethodRejection_Summary(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Subcommand mockCmd[summaryMethodCmd] `summary:"Sub command"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `schema error: subcommand struct summaryMethodCmd must not implement Summary(); declare the 1-line synopsis via the summary:"..." struct tag`, err.Error())
}

func TestSubcommand_MethodRejection_Deprecated(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Subcommand mockCmd[deprecatedMethodCmd] `summary:"Sub command"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `schema error: subcommand struct deprecatedMethodCmd must not implement Deprecated(); declare the deprecation notice via the deprecated:"..." struct tag`, err.Error())
}

func TestSubcommand_MethodRejection_HelpFooter(t *testing.T) {
	t.Parallel()

	type RootCLI struct {
		Subcommand mockCmd[helpFooterMethodCmd] `summary:"Sub command"`
	}

	var cli RootCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `schema error: subcommand struct helpFooterMethodCmd must not implement HelpFooter(); help footers are global and must be configured via app.HelpFooter()`, err.Error())
}

func TestRootCommand_MethodRejection_Summary(t *testing.T) {
	t.Parallel()

	var cli rootSummaryCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `schema error: root command struct rootSummaryCLI must not implement Summary(); configure application description via app.Description()`, err.Error())
}

func TestRootCommand_MethodRejection_Description(t *testing.T) {
	t.Parallel()

	var cli rootDescriptionCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `schema error: root command struct rootDescriptionCLI must not implement Description(); configure application description via app.Description()`, err.Error())
}

func TestRootCommand_MethodRejection_HelpFooter(t *testing.T) {
	t.Parallel()

	var cli rootHelpFooterCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `schema error: root command struct rootHelpFooterCLI must not implement HelpFooter(); configure global help footer via app.HelpFooter()`, err.Error())
}

func TestRootCommand_MethodRejection_Deprecated(t *testing.T) {
	t.Parallel()

	var cli rootDeprecatedCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `schema error: root command struct rootDeprecatedCLI must not implement Deprecated(); use app.Deprecated()`, err.Error())
}

func TestRootCommand_MethodRejection_Examples(t *testing.T) {
	t.Parallel()

	var cli rootExamplesCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Equal(t, `schema error: root command struct rootExamplesCLI must not implement Examples(); configure root examples via app.Examples()`, err.Error())
}

type featureSet struct {
	GentleForce  bool
	Experimental bool
	V2           bool
}

type featureFlagCLI struct {
	Features mockFeatures[featureSet]

	ForceAll mockFlag[bool]         `feature:"gentle-force" help:"Forces all tasks"`
	Sub      mockCmd[featureSubCLI] `feature:"experimental" name:"lab"`
}

type featureSubCLI struct {
	Beta mockFlag[string] `feature:"v2"`
}

type featureRequiredCLI struct {
	Features mockFeatures[featureSet]

	Req mockReqFlag[string] `feature:"preview"`
}

type undeclaredFeatureCLI struct {
	Force mockFlag[bool] `feature:"missing"`
}

type featureValueCLI struct {
	Features mockFeatures[featureValueSet]

	Mode mockFlag[string] `feature:"parallel=3"`
}

type featureValueSet struct {
	Parallel int `enum:"1,2"`
}

type featureUnsignedSet struct {
	Workers uint `enum:"1,2" help:"Workers"`
}

type featureUnsignedCLI struct {
	Features mockFeatures[featureUnsignedSet]

	Force mockFlag[bool] `feature:"workers"`
}

type featureNegativeEnumSet struct {
	Level uint `enum:"-1" help:"Level"`
}

type featureWidthSet struct {
	Level int8 `enum:"300" help:"Level"`
}

type featureNoEnumSet struct {
	Level int `help:"Level"`
}

type featureNegativeDefaultSet struct {
	Level uint `default:"-1" help:"Level"`
}

type featureWidthDefaultSet struct {
	Level int8 `default:"300" help:"Level"`
}

type featureSchemaCLI[T any] struct {
	Features mockFeatures[T]
}

type featureDeclSet struct {
	GentleForce bool `default:"true" help:"Force all"`
	Parallel    int  `enum:"1,2"     help:"Workers"   name:"parallelism"`
}

type featureDeclCLI struct {
	Features mockFeatures[featureDeclSet] `config:"experiments" envprefix:"APP_X_" flag:"feature-gates"`

	Force mockFlag[bool] `feature:"gentle-force"`
}

func TestFeaturesContainer_Parsing(t *testing.T) {
	t.Parallel()

	var cli featureDeclCLI

	cmd, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)

	assert.Equal(t, []string{"gentle-force", "parallelism"}, cmd.FeatureNames())

	gentle := cmd.FindFeature("gentle-force")
	assert.NotZero(t, gentle)
	assert.False(t, gentle.IsInt)
	assert.Equal(t, "true", gentle.Default)

	parallel := cmd.FindFeature("parallelism")
	assert.NotZero(t, parallel)
	assert.True(t, parallel.IsInt)
	assert.Equal(t, []string{"1", "2"}, parallel.Enum)

	assert.NotZero(t, cmd.FeatureFlag)
	assert.Equal(t, "feature-gates", cmd.FeatureFlag.Name)
	assert.True(t, cmd.FeatureFlag.IsMap)
	assert.Equal(t, "experiments", cmd.FeatureFlag.ConfigKey)
	assert.Equal(t, []string{"APP_X_FEATURE_GATES"}, cmd.FeatureFlag.EnvVars)
	assert.Equal(t, []string{"gentle-force", "parallelism"}, cmd.FeatureFlag.Enum)
}

type featureDuplicateCLI struct {
	First  mockFeatures[struct{}]       `flag:"gates-a"`
	Second mockFeatures[featureDeclSet] `flag:"gates-b"`
}

type featurePredictorCLI struct {
	Features mockFeatures[featureDeclSet] `flag:"gates"`
}

func TestFeaturesContainer_DuplicateDeclaration_EmptyInnerStruct(t *testing.T) {
	t.Parallel()

	var cli featureDuplicateCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrDuplicateFlag)
	assert.Equal(t, "struct featureDuplicateCLI field Second: schema error: duplicate flag: Features (already declared by gates-a)", err.Error())
}

type featureNoCLINameAfterCLI struct {
	Features mockFeatures[featureDeclSet]

	Legacy mockFlag[string] `help:"Legacy features flag" name:"features"`
}

type featureNoCLINameBeforeCLI struct {
	Legacy mockFlag[string] `help:"Legacy features flag" name:"features"`

	Features mockFeatures[featureDeclSet]
}

type featureTaggedCollisionCLI struct {
	Features mockFeatures[featureDeclSet] `flag:"gates"`

	Gates mockFlag[string] `help:"User gates" name:"gates"`
}

// hasRoutableFlag reports whether cmd declares a CLI-routable flag with the
// given name (ignoring the hidden NoCLI synthesized feature flag).
func hasRoutableFlag(cmd *spec.Command, name string) bool {
	for _, flag := range cmd.Flags {
		if flag.Name == name && !flag.NoCLI {
			return true
		}
	}

	return false
}

func TestFeaturesContainer_NoCLINameSharedWithUserFlag(t *testing.T) {
	t.Parallel()

	t.Run("user flag declared after the container", func(t *testing.T) {
		t.Parallel()

		var cli featureNoCLINameAfterCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.NotZero(t, cmd.FeatureFlag)
		assert.True(t, cmd.FeatureFlag.NoCLI)
		assert.True(t, hasRoutableFlag(cmd, "features"))
	})

	t.Run("user flag declared before the container", func(t *testing.T) {
		t.Parallel()

		var cli featureNoCLINameBeforeCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{})

		assert.NoError(t, err)
		assert.NotZero(t, cmd.FeatureFlag)
		assert.True(t, cmd.FeatureFlag.NoCLI)
		assert.True(t, hasRoutableFlag(cmd, "features"))
	})
}

func TestFeaturesContainer_TaggedNameCollidesWithUserFlag(t *testing.T) {
	t.Parallel()

	var cli featureTaggedCollisionCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrDuplicateFlag)
}

func TestFeaturesContainer_PredictorReceivesFlagName(t *testing.T) {
	t.Parallel()

	capture := func(name string, values map[string][]string) *spec.PredictorSpec {
		return &spec.PredictorSpec{Kind: spec.PredictorKindCustom, Name: name}
	}

	t.Run("explicit flag name", func(t *testing.T) {
		t.Parallel()

		var cli featureDeclCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{FeaturePredictor: capture})

		assert.NoError(t, err)
		assert.Equal(t, "feature-gates", cmd.FeatureFlag.Predictor.Name)
	})

	t.Run("derived flag name", func(t *testing.T) {
		t.Parallel()

		var cli featurePredictorCLI

		cmd, err := buildSchema(&cli, "app", spec.Options{FeaturePredictor: capture})

		assert.NoError(t, err)
		assert.Equal(t, "gates", cmd.FeatureFlag.Predictor.Name)
	})
}

type featureSubcommandTagCLI struct {
	Lab mockCmd[LeafCommand] `feature:"="`
}

func TestSubcommandFeatureTag_InvalidSyntax(t *testing.T) {
	t.Parallel()

	var cli featureSubcommandTagCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, `struct featureSubcommandTagCLI field Lab: schema error: invalid struct tag: invalid feature tag syntax "="`, err.Error())
}

func TestFeaturesContainer_UnsignedField(t *testing.T) {
	t.Parallel()

	var cli featureUnsignedCLI

	cmd, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)

	workers := cmd.FindFeature("workers")
	assert.NotZero(t, workers)
	assert.True(t, workers.IsInt)
	assert.True(t, workers.IsUnsigned)
	assert.Equal(t, strconv.IntSize, workers.BitSize)
}

type featureInvalidTypeSet struct {
	Label string `help:"Label"`
}

type featureInvalidTypeCLI struct {
	Features mockFeatures[featureInvalidTypeSet]
}

func TestFeaturesContainer_InvalidFieldType(t *testing.T) {
	t.Parallel()

	var cli featureInvalidTypeCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Equal(t, "struct featureInvalidTypeCLI field Features: schema error: invalid struct tag: feature field Label must be a bool or an integer, got string", err.Error())
}

func TestFeatureTag_UnsignedComparison(t *testing.T) {
	t.Parallel()

	var cli featureUnsignedCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)
}

func TestFeatureTag_NegativeUnsignedEnum(t *testing.T) {
	t.Parallel()

	var cli featureSchemaCLI[featureNegativeEnumSet]

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Contains(t, err.Error(), `feature enum value "-1": must be a non-negative integer`)
}

func TestFeatureTag_OutOfWidthEnum(t *testing.T) {
	t.Parallel()

	var cli featureSchemaCLI[featureWidthSet]

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Contains(t, err.Error(), `feature enum value "300": is out of range for the feature's integer type`)
}

func TestFeatureTag_NegativeUnsignedDefault(t *testing.T) {
	t.Parallel()

	var cli featureSchemaCLI[featureNegativeDefaultSet]

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Contains(t, err.Error(), `feature "level" default "-1": must be a non-negative integer`)
}

func TestFeatureTag_OutOfWidthDefault(t *testing.T) {
	t.Parallel()

	var cli featureSchemaCLI[featureWidthDefaultSet]

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Contains(t, err.Error(), `feature "level" default "300": is out of range for the feature's integer type`)
}

func TestFeatureTag_IntegerWithoutEnum(t *testing.T) {
	t.Parallel()

	var cli featureSchemaCLI[featureNoEnumSet]

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Contains(t, err.Error(), `feature "level" is an integer feature and requires an enum tag listing its non-zero levels`)
}

func TestFeatureTag_Parsing(t *testing.T) {
	t.Parallel()

	var cli featureFlagCLI

	cmd, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)

	flag := cmd.FindFlag("force-all")
	assert.NotZero(t, flag)
	assert.Equal(t, "gentle-force", flag.Feature)

	subcommand := cmd.FindSubcommand("lab")
	assert.NotZero(t, subcommand)
	assert.Equal(t, "experimental", subcommand.Feature)

	subcommandFlag := subcommand.FindFlag("beta")
	assert.NotZero(t, subcommandFlag)
	assert.Equal(t, "v2", subcommandFlag.Feature)

	assert.Equal(t, []string{"gentle-force", "experimental", "v2"}, cmd.FeatureNames())
}

type featureNormalizedCLI struct {
	Features mockFeatures[featureSet]

	ForceAll mockFlag[bool] `feature:"gentle-force, experimental" help:"Forces all"`
}

func TestFeatureTag_ExpressionNormalized(t *testing.T) {
	t.Parallel()

	var cli featureNormalizedCLI

	cmd, err := buildSchema(&cli, "app", spec.Options{})

	assert.NoError(t, err)

	flag := cmd.FindFlag("force-all")
	assert.NotZero(t, flag)
	assert.Equal(t, "gentle-force,experimental", flag.Feature)
}

func TestFeatureTag_RequiredFlagRejection(t *testing.T) {
	t.Parallel()

	var cli featureRequiredCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
}

func TestFeatureTag_UndeclaredReference(t *testing.T) {
	t.Parallel()

	var cli undeclaredFeatureCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrSchema)
	assert.Contains(t, err.Error(), `references undeclared feature "missing"`)
}

func TestFeatureTag_ValueComparisonSyntaxRejected(t *testing.T) {
	t.Parallel()

	var cli featureValueCLI

	_, err := buildSchema(&cli, "app", spec.Options{})

	assert.Error(t, err)
	assert.IsError(t, err, spec.ErrInvalidStructTag)
	assert.Contains(t, err.Error(), `invalid feature tag syntax "parallel=3"`)
}

func TestMatchFeature(t *testing.T) {
	t.Parallel()

	active := map[string]bool{
		"gentle-force": true,
		"preview":      true,
		"version":      true,
		"parallel":     true,
		"stage":        false,
		"disabled":     false,
	}

	t.Run("truthiness", func(t *testing.T) {
		t.Parallel()

		assert.True(t, spec.MatchFeature("gentle-force", active))
		assert.True(t, spec.MatchFeature("preview", active))
		assert.True(t, spec.MatchFeature("version", active))
		assert.False(t, spec.MatchFeature("disabled", active))
		assert.False(t, spec.MatchFeature("missing", active))
	})

	t.Run("integer levels", func(t *testing.T) {
		t.Parallel()

		// Integer features are on at any non-zero level and off at zero.
		assert.True(t, spec.MatchFeature("parallel", active))
		assert.False(t, spec.MatchFeature("stage", active))
	})

	t.Run("negation", func(t *testing.T) {
		t.Parallel()

		assert.True(t, spec.MatchFeature("!missing", active))
		assert.True(t, spec.MatchFeature("!disabled", active))
		assert.False(t, spec.MatchFeature("!preview", active))
	})

	t.Run("AND of multiple terms", func(t *testing.T) {
		t.Parallel()

		assert.True(t, spec.MatchFeature("gentle-force,preview,!disabled", active))
		assert.False(t, spec.MatchFeature("gentle-force,missing", active))
	})

	t.Run("empty expression", func(t *testing.T) {
		t.Parallel()

		assert.True(t, spec.MatchFeature("", active))
	})
}

func TestFeatureTag_InvalidSyntaxRejection(t *testing.T) {
	t.Parallel()

	invalidTags := []string{"=", "!", "foo=", "=bar", ",,", "foo!= ", " ! ", "foo,,bar"}

	for _, invalidTag := range invalidTags {
		t.Run(invalidTag, func(t *testing.T) {
			t.Parallel()

			err := spec.ValidateFeatureExpression(invalidTag)
			assert.Error(t, err)
			assert.IsError(t, err, spec.ErrInvalidStructTag)
		})
	}
}
