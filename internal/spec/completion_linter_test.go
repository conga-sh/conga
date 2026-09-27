package spec_test

import (
	"context"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/container"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

type TypoPredictorFlagCLI struct {
	Image mockFlag[string] `help:"Input image" predictro:"file:*.png"`
}

type TypoPredictorAltFlagCLI struct {
	Config mockFlag[string] `help:"Config path" pridictor:"file:*.json"`
}

type TypoPredictorArgCLI struct {
	Target mockArg[string] `help:"Target branch" pedictor:"branches"`
}

type EmptyPredictorFlagCLI struct {
	File mockFlag[string] `help:"Input file" predictor:""`
}

type WhitespacePredictorArgCLI struct {
	Dir mockArg[string] `help:"Directory" predictor:"   "`
}

type ContextPredictorBoolFlagCLI struct {
	Verbose mockFlag[bool] `help:"Verbose mode" predictor:"file"`
}

type ContextPredictorNegatableBoolCLI struct {
	Debug mockFlag[bool] `help:"Debug mode" negatable:"true" predictor:"file"`
}

type ContextPredictorSubcommandCLI struct {
	Subcommand mockCmd[SubcommandForPredictor] `predictor:"file" summary:"Subcommand"`
}

type SubcommandForPredictor struct {
	Name mockFlag[string] `help:"Name"`
}

func (s *SubcommandForPredictor) Run(ctx context.Context) error {
	return nil
}

type ContextPredictorSentinelCLI struct {
	_    struct{}         `predictor:"file"`
	Data mockFlag[string] `help:"Data"`
}

type ContextPredictorDashArgsCLI struct {
	Trailing mockDashArgs `help:"Trailing args" predictor:"file"`
}

type ContextPredictorRawArgsCLI struct {
	Raw mockRawArgs `help:"Raw args" predictor:"file"`
}

type ContextPredictorConfigFileCLI struct {
	Config mockConfigFile `help:"Config file" predictor:"file"`
}

type ValidBuiltinFilesCLI struct {
	SingleFile mockFlag[string]   `help:"Single file"         predictor:"file"`
	ExtFile    mockFlag[string]   `help:"Filtered file"       predictor:"file:*.png,*.jpg"`
	MultiExt   mockFlag[[]string] `help:"Multi filtered file" predictor:"file:*.json,*.yaml"`
	Directory  mockFlag[string]   `help:"Directory"           predictor:"dir"`
	Suppressed mockFlag[string]   `help:"No file fallback"    predictor:"none"`
	Off        mockFlag[string]   `help:"Off fallback"        predictor:"off"`
}

type MalformedEmptyPatternFileCLI struct {
	File mockFlag[string] `help:"File" predictor:"file:"`
}

type MalformedTrailingCommaFileCLI struct {
	File mockFlag[string] `help:"File" predictor:"file:*.png,"`
}

type MalformedLeadingCommaFileCLI struct {
	File mockFlag[string] `help:"File" predictor:"file:,*.png"`
}

type MalformedEmptyIntermediatePatternCLI struct {
	File mockFlag[string] `help:"File" predictor:"file:*.png,,*.jpg"`
}

type MalformedDirWithPatternsCLI struct {
	Dir mockFlag[string] `help:"Directory" predictor:"dir:*.tmp"`
}

type MalformedNoneWithParamsCLI struct {
	None mockFlag[string] `help:"None" predictor:"none:true"`
}

type MisspelledPrefixFilesWithColonCLI struct {
	File mockFlag[string] `help:"File" predictor:"files:*.png"`
}

type MisspelledPrefixFielsWithColonCLI struct {
	File mockFlag[string] `help:"File" predictor:"fiels:*.png"` //nolint:misspell // intentionally misspelled for test
}

type MisspelledPrefixFlieWithColonCLI struct {
	File mockFlag[string] `help:"File" predictor:"flie:*.tar.gz"`
}

type MisspelledStandaloneFilesCLI struct {
	File mockFlag[string] `help:"File" predictor:"files"`
}

type MisspelledStandaloneDirsCLI struct {
	Dir mockFlag[string] `help:"Directory" predictor:"dirs"`
}

type MisspelledStandaloneDirectoryCLI struct {
	Dir mockFlag[string] `help:"Directory" predictor:"directory"`
}

type MisspelledStandaloneDirectoriesCLI struct {
	Dir mockFlag[string] `help:"Directory" predictor:"directories"`
}

type CustomPredictorCLI struct {
	Branch mockFlag[string] `help:"Git branch" predictor:"branches"`
}

type CustomPredictorTypoCLI struct {
	Branch mockFlag[string] `help:"Git branch" predictor:"brnaches"`
}

type CustomPredictorCloseToBuiltinCLI struct {
	Image mockFlag[string] `help:"Image path" predictor:"fiel"`
}

type CustomPredictorInvalidCharsCLI struct {
	Invalid mockFlag[string] `help:"Invalid" predictor:"foo bar"`
}

type ExistingFile string

func (f ExistingFile) Predict() predict.Predictor { return predict.Files() }

type ExistingDir string

func (d ExistingDir) Predict() predict.Predictor { return predict.Dirs() }

type TypeInferenceCLI struct {
	InputFile  mockFlag[ExistingFile]   `help:"Input file"`
	InputFiles mockFlag[[]ExistingFile] `help:"Input files"`
	InputDir   mockArg[ExistingDir]     `help:"Input directory"`
	ConfigFile mockConfigFile           `help:"Configuration file"`
}

func TestStrictPredictorLinter_TypoDetection(t *testing.T) {
	t.Parallel()

	t.Run("typo in flag predictro tag suggests predictor", func(t *testing.T) {
		t.Parallel()

		var cli TypoPredictorFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct TypoPredictorFlagCLI field Image: schema error: invalid struct tag: unknown struct tag "predictro" (did you mean "predictor"?). Use conga:"allow-unknown-tags" or app.AllowTags("predictro") to permit external tags`, err.Error())
	})

	t.Run("typo in flag pridictor tag suggests predictor", func(t *testing.T) {
		t.Parallel()

		var cli TypoPredictorAltFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct TypoPredictorAltFlagCLI field Config: schema error: invalid struct tag: unknown struct tag "pridictor" (did you mean "predictor"?). Use conga:"allow-unknown-tags" or app.AllowTags("pridictor") to permit external tags`, err.Error())
	})

	t.Run("typo in positional arg pedictor tag suggests predictor", func(t *testing.T) {
		t.Parallel()

		var cli TypoPredictorArgCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct TypoPredictorArgCLI field Target: schema error: invalid struct tag: unknown struct tag "pedictor" (did you mean "predictor"?). Use conga:"allow-unknown-tags" or app.AllowTags("pedictor") to permit external tags`, err.Error())
	})

	t.Run("empty predictor tag on flag errors", func(t *testing.T) {
		t.Parallel()

		var cli EmptyPredictorFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct EmptyPredictorFlagCLI field File: schema error: invalid struct tag: predictor tag cannot be empty`, err.Error())
	})

	t.Run("whitespace predictor tag on arg errors", func(t *testing.T) {
		t.Parallel()

		var cli WhitespacePredictorArgCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct WhitespacePredictorArgCLI field Dir: schema error: invalid struct tag: predictor tag cannot be empty`, err.Error())
	})
}

func TestStrictPredictorLinter_ContextValidation(t *testing.T) {
	t.Parallel()

	t.Run("predictor tag on plain bool flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextPredictorBoolFlagCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextPredictorBoolFlagCLI field Verbose: schema error: invalid struct tag: tag "predictor" is invalid on boolean flags`, err.Error())
	})

	t.Run("predictor tag on negatable bool flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextPredictorNegatableBoolCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextPredictorNegatableBoolCLI field Debug: schema error: invalid struct tag: tag "predictor" is invalid on boolean flags`, err.Error())
	})

	t.Run("predictor tag on subcommand errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextPredictorSubcommandCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextPredictorSubcommandCLI field Subcommand: schema error: invalid struct tag: tag "predictor" is invalid on subcommand (tag "predictor" is only valid on flags and positional arguments)`, err.Error())
	})

	t.Run("predictor tag on sentinel field errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextPredictorSentinelCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextPredictorSentinelCLI field _: schema error: invalid struct tag: tag "predictor" is invalid on sentinel field (tag "predictor" is only valid on flags and positional arguments)`, err.Error())
	})

	t.Run("predictor tag on dash args errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextPredictorDashArgsCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextPredictorDashArgsCLI field Trailing: schema error: invalid struct tag: tag "predictor" is invalid on dash args (tag "predictor" is only valid on flags and positional arguments)`, err.Error())
	})

	t.Run("predictor tag on raw args errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextPredictorRawArgsCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextPredictorRawArgsCLI field Raw: schema error: invalid struct tag: tag "predictor" is invalid on raw args (tag "predictor" is only valid on flags and positional arguments)`, err.Error())
	})

	t.Run("predictor tag on config file flag errors", func(t *testing.T) {
		t.Parallel()

		var cli ContextPredictorConfigFileCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct ContextPredictorConfigFileCLI field Config: schema error: invalid struct tag: tag "predictor" is invalid on config file flag (tag "predictor" is only valid on flags and positional arguments)`, err.Error())
	})
}

func TestStrictPredictorLinter_BuiltinGrammar(t *testing.T) {
	t.Parallel()

	t.Run("valid built-in file and dir predictors build successfully", func(t *testing.T) {
		t.Parallel()

		var cli ValidBuiltinFilesCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.NoError(t, err)
		assert.Equal(t, 6, len(cmdSpec.Flags))
		assert.Equal(t, spec.PredictorKindFile, cmdSpec.Flags[0].Predictor.Kind)
		assert.Equal(t, "file", cmdSpec.Flags[0].Predictor.Name)
		assert.Zero(t, cmdSpec.Flags[0].Predictor.Patterns)

		assert.Equal(t, spec.PredictorKindFile, cmdSpec.Flags[1].Predictor.Kind)
		assert.Equal(t, []string{"*.png", "*.jpg"}, cmdSpec.Flags[1].Predictor.Patterns)

		assert.Equal(t, spec.PredictorKindFile, cmdSpec.Flags[2].Predictor.Kind)
		assert.Equal(t, []string{"*.json", "*.yaml"}, cmdSpec.Flags[2].Predictor.Patterns)

		assert.Equal(t, spec.PredictorKindDir, cmdSpec.Flags[3].Predictor.Kind)
		assert.Equal(t, spec.PredictorKindNone, cmdSpec.Flags[4].Predictor.Kind)
		assert.Equal(t, spec.PredictorKindNone, cmdSpec.Flags[5].Predictor.Kind)
	})

	t.Run("malformed file filter with empty pattern errors", func(t *testing.T) {
		t.Parallel()

		var cli MalformedEmptyPatternFileCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MalformedEmptyPatternFileCLI field File: schema error: invalid struct tag: invalid predictor syntax "file:": file filter pattern cannot be empty (expected file:*.ext)`, err.Error())
	})

	t.Run("malformed file filter with trailing comma errors", func(t *testing.T) {
		t.Parallel()

		var cli MalformedTrailingCommaFileCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MalformedTrailingCommaFileCLI field File: schema error: invalid struct tag: invalid predictor syntax "file:*.png,": trailing, leading, or empty pattern element in file filter`, err.Error())
	})

	t.Run("malformed file filter with leading comma errors", func(t *testing.T) {
		t.Parallel()

		var cli MalformedLeadingCommaFileCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MalformedLeadingCommaFileCLI field File: schema error: invalid struct tag: invalid predictor syntax "file:,*.png": trailing, leading, or empty pattern element in file filter`, err.Error())
	})

	t.Run("malformed file filter with empty intermediate pattern errors", func(t *testing.T) {
		t.Parallel()

		var cli MalformedEmptyIntermediatePatternCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MalformedEmptyIntermediatePatternCLI field File: schema error: invalid struct tag: invalid predictor syntax "file:*.png,,*.jpg": trailing, leading, or empty pattern element in file filter`, err.Error())
	})

	t.Run("malformed dir predictor with filter patterns errors", func(t *testing.T) {
		t.Parallel()

		var cli MalformedDirWithPatternsCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MalformedDirWithPatternsCLI field Dir: schema error: invalid struct tag: invalid predictor "dir:*.tmp" (directories predictor does not accept filter patterns, use predictor:"dir")`, err.Error())
	})

	t.Run("malformed none predictor with params errors", func(t *testing.T) {
		t.Parallel()

		var cli MalformedNoneWithParamsCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MalformedNoneWithParamsCLI field None: schema error: invalid struct tag: invalid predictor "none:true" (none predictor does not accept parameters, use predictor:"none")`, err.Error())
	})

	t.Run("misspelled files:*.ext prefix errors suggesting file", func(t *testing.T) {
		t.Parallel()

		var cli MisspelledPrefixFilesWithColonCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MisspelledPrefixFilesWithColonCLI field File: schema error: invalid struct tag: invalid predictor "files:*.png" (unknown predictor prefix "files", did you mean "file"?)`, err.Error())
	})

	t.Run("misspelled fiels:*.ext prefix errors suggesting file", func(t *testing.T) { //nolint:misspell // deliberate test case
		t.Parallel()

		var cli MisspelledPrefixFielsWithColonCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MisspelledPrefixFielsWithColonCLI field File: schema error: invalid struct tag: invalid predictor "fiels:*.png" (unknown predictor prefix "fiels", did you mean "file"?)`, err.Error()) //nolint:misspell // intentionally misspelled in expected test error
	})

	t.Run("misspelled flie:*.ext prefix errors suggesting file", func(t *testing.T) {
		t.Parallel()

		var cli MisspelledPrefixFlieWithColonCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MisspelledPrefixFlieWithColonCLI field File: schema error: invalid struct tag: invalid predictor "flie:*.tar.gz" (unknown predictor prefix "flie", did you mean "file"?)`, err.Error())
	})

	t.Run("misspelled standalone files predictor errors suggesting file", func(t *testing.T) {
		t.Parallel()

		var cli MisspelledStandaloneFilesCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MisspelledStandaloneFilesCLI field File: schema error: invalid struct tag: unknown predictor "files" (did you mean "file"?)`, err.Error())
	})

	t.Run("misspelled standalone dirs predictor errors suggesting dir", func(t *testing.T) {
		t.Parallel()

		var cli MisspelledStandaloneDirsCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MisspelledStandaloneDirsCLI field Dir: schema error: invalid struct tag: unknown predictor "dirs" (did you mean "dir"?)`, err.Error())
	})

	t.Run("misspelled standalone directory predictor errors suggesting dir", func(t *testing.T) {
		t.Parallel()

		var cli MisspelledStandaloneDirectoryCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MisspelledStandaloneDirectoryCLI field Dir: schema error: invalid struct tag: unknown predictor "directory" (did you mean "dir"?)`, err.Error())
	})

	t.Run("misspelled standalone directories predictor errors suggesting dir", func(t *testing.T) {
		t.Parallel()

		var cli MisspelledStandaloneDirectoriesCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct MisspelledStandaloneDirectoriesCLI field Dir: schema error: invalid struct tag: unknown predictor "directories" (did you mean "dir"?)`, err.Error())
	})
}

func TestStrictPredictorLinter_TypeInference(t *testing.T) {
	t.Parallel()

	var cli TypeInferenceCLI

	cmdSpec, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

	assert.NoError(t, err)
	assert.Equal(t, 3, len(cmdSpec.Flags))
	assert.Equal(t, 1, len(cmdSpec.Args))

	assert.Equal(t, spec.PredictorKindPredictable, cmdSpec.Flags[0].Predictor.Kind)
	assert.Equal(t, spec.PredictorKindPredictable, cmdSpec.Flags[1].Predictor.Kind)
	assert.Equal(t, spec.PredictorKindFile, cmdSpec.Flags[2].Predictor.Kind)
	assert.Equal(t, spec.PredictorKindPredictable, cmdSpec.Args[0].Predictor.Kind)
}

func TestPredictorInference_RequiresHook(t *testing.T) {
	t.Parallel()

	t.Run("nil hook leaves domains without predictor specs", func(t *testing.T) {
		t.Parallel()

		var cli TypeInferenceCLI

		cmdSpec, err := spec.Build(&cli, "app", spec.Options{ClassifyContainer: classifyTestContainer})

		assert.NoError(t, err)
		assert.Equal(t, 3, len(cmdSpec.Flags))
		assert.Equal(t, 1, len(cmdSpec.Args))
		assert.Zero(t, cmdSpec.Flags[0].Predictor)
		assert.Zero(t, cmdSpec.Flags[1].Predictor)
		assert.Equal(t, spec.PredictorKindFile, cmdSpec.Flags[2].Predictor.Kind)
		assert.Zero(t, cmdSpec.Args[0].Predictor)
	})

	t.Run("hook restores inferred predictor specs", func(t *testing.T) {
		t.Parallel()

		var cli TypeInferenceCLI

		cmdSpec, err := spec.Build(&cli, "app", spec.Options{
			ClassifyContainer: classifyTestContainer,
			InferPredictor:    spec.InferPredictorFromType,
		})

		assert.NoError(t, err)
		assert.Equal(t, spec.PredictorKindPredictable, cmdSpec.Flags[0].Predictor.Kind)
		assert.Equal(t, spec.PredictorKindPredictable, cmdSpec.Flags[1].Predictor.Kind)
		assert.Equal(t, spec.PredictorKindFile, cmdSpec.Flags[2].Predictor.Kind)
		assert.Equal(t, spec.PredictorKindPredictable, cmdSpec.Args[0].Predictor.Kind)
	})
}

func TestStrictPredictorLinter_UnregisteredPredictors(t *testing.T) {
	t.Parallel()

	t.Run("registered custom predictor passes validation", func(t *testing.T) {
		t.Parallel()

		var cli CustomPredictorCLI

		cmdSpec, err := buildSchema(&cli, "app", spec.Options{
			StrictTagValidation:  true,
			RegisteredPredictors: map[string]predict.Predictor{"branches": predict.Values("main")},
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, len(cmdSpec.Flags))
		assert.Equal(t, spec.PredictorKindCustom, cmdSpec.Flags[0].Predictor.Kind)
		assert.Equal(t, "branches", cmdSpec.Flags[0].Predictor.Name)
	})

	t.Run("unregistered custom predictor errors with suggestion", func(t *testing.T) {
		t.Parallel()

		var cli CustomPredictorTypoCLI

		_, err := buildSchema(&cli, "app", spec.Options{
			StrictTagValidation:  true,
			RegisteredPredictors: map[string]predict.Predictor{"branches": predict.Values("main")},
		})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct CustomPredictorTypoCLI field Branch: schema error: invalid struct tag: unregistered predictor "brnaches" (did you mean "branches"?). Register it with app.Predictor("brnaches", action)`, err.Error())
	})

	t.Run("unregistered custom predictor close to built-in errors suggesting built-in", func(t *testing.T) {
		t.Parallel()

		var cli CustomPredictorCloseToBuiltinCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct CustomPredictorCloseToBuiltinCLI field Image: schema error: invalid struct tag: unregistered predictor "fiel" (did you mean "file"?). Register it with app.Predictor("fiel", action)`, err.Error())
	})

	t.Run("custom predictor with invalid characters errors", func(t *testing.T) {
		t.Parallel()

		var cli CustomPredictorInvalidCharsCLI

		_, err := buildSchema(&cli, "app", spec.Options{StrictTagValidation: true})

		assert.Error(t, err)
		assert.IsError(t, err, spec.ErrSchema)
		assert.IsError(t, err, spec.ErrInvalidStructTag)
		assert.Equal(t, `struct CustomPredictorInvalidCharsCLI field Invalid: schema error: invalid struct tag: invalid predictor "foo bar" (predictor name cannot contain spaces or commas)`, err.Error())
	})
}

type mockDashArgs struct{}

func (m *mockDashArgs) testClassification() container.Classification {
	return container.Classification{Kind: container.KindDashArgs}
}

type mockRawArgs struct{}

func (m *mockRawArgs) testClassification() container.Classification {
	return container.Classification{Kind: container.KindRawArgs}
}
