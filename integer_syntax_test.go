package conga_test

import (
	"bytes"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/internal/errlines"
)

type integerSyntaxCLI struct {
	noopRunner

	Mode   conga.OptionalFlag[uint32] `help:"File mode"`
	Offset conga.OptionalFlag[int]    `help:"Offset"`
	Count  conga.OptionalArg[int8]    `help:"Count"`
}

type integerSyntaxEnvCLI struct {
	noopRunner

	Mode conga.OptionalFlag[uint32] `env:"CONGA_TEST_INTEGER_SYNTAX_MODE" help:"File mode"`
}

type integerSyntaxConfigCLI struct {
	noopRunner

	Mode conga.OptionalFlag[uint32] `help:"File mode"`
}

type integerSyntaxEnumCLI struct {
	noopRunner

	Pick  conga.OptionalFlag[int]     `enum:"0x10,20" help:"Pick"`
	Sizes conga.OptionalFlag[[]uint8] `enum:"0b1,0o2" help:"Sizes"`
	Slot  conga.OptionalArg[uint16]   `enum:"0X1F,0"  help:"Slot"`
}

type integerSyntaxRangeCLI struct {
	noopRunner

	Width conga.OptionalFlag[int]    `help:"Width" range:"0x10..0x20"`
	Size  conga.OptionalFlag[uint64] `help:"Size"  range:"0b1..0o10"`
}

// integerSyntaxLevel is an integer type with a registered parser that reads
// level names, so its enum values are level names too.
type integerSyntaxLevel int

func (l integerSyntaxLevel) String() string {
	if l == 1 {
		return "high"
	}

	return "low"
}

type integerSyntaxRegisteredCLI struct {
	noopRunner

	Level conga.OptionalFlag[integerSyntaxLevel] `enum:"low,high" help:"Level"`
}

type integerSyntaxCounterEnvCLI struct {
	noopRunner

	Verbose conga.Counter `env:"CONGA_TEST_INTEGER_SYNTAX_VERBOSE" help:"Verbosity" short:"v"`
}

type integerSyntaxLevels struct {
	Depth int `enum:"0x2,3" help:"Depth level"`
}

type integerSyntaxFeatureCLI struct {
	noopRunner

	Features conga.Features[integerSyntaxLevels] `flag:"feature"`
}

func TestIntegerSyntax_FlagValues(t *testing.T) {
	t.Parallel()

	accepted := []struct {
		args     []string
		expected uint32
	}{
		{args: []string{"--mode", "0x1ed"}, expected: 493},
		{args: []string{"--mode", "0X1ED"}, expected: 493},
		{args: []string{"--mode", "0o755"}, expected: 493},
		{args: []string{"--mode", "0O755"}, expected: 493},
		{args: []string{"--mode", "0b111101101"}, expected: 493},
		{args: []string{"--mode", "755"}, expected: 755},
		{args: []string{"--mode", "0"}, expected: 0},
	}

	for _, testCase := range accepted {
		t.Run(testCase.args[1], func(t *testing.T) {
			t.Parallel()

			cli := new(integerSyntaxCLI)
			app := conga.New(cli).Args(testCase.args)

			err := app.RunContext(t.Context())

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, cli.Mode.Get())
		})
	}
}

func TestIntegerSyntax_NegativePrefixedFlagValue(t *testing.T) {
	t.Parallel()

	cli := new(integerSyntaxCLI)
	app := conga.New(cli).Args([]string{"--offset", "-0x10"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, -16, cli.Offset.Get())
}

func TestIntegerSyntax_FlagLeadingZero(t *testing.T) {
	t.Parallel()

	result := runClassification(t, conga.New[integerSyntaxCLI]().Args([]string{"--mode", "0755"}))

	assert.Error(t, result.err)
	assert.Equal(t, `invalid unsigned integer value "0755" for flag --mode`, result.err.Error())
	assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(result.err))
	assert.Equal(t, `write 0o755 for octal or 755 for decimal`, errlines.Hint(result.err))
	assertUsageError(t, result.err)
	assert.Equal(t, 2, result.exitCode)
}

func TestIntegerSyntax_NegativeFlagLeadingZero(t *testing.T) {
	t.Parallel()

	err := conga.New[integerSyntaxCLI]().Args([]string{"--offset", "-0755"}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid integer value "-0755" for flag --offset`, err.Error())
	assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
	assert.Equal(t, `write -0o755 for octal or -755 for decimal`, errlines.Hint(err))
	assertUsageError(t, err)
}

func TestIntegerSyntax_LeadingZeroWithNonOctalDigits(t *testing.T) {
	t.Parallel()

	err := conga.New[integerSyntaxCLI]().Args([]string{"--offset", "0089"}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid integer value "0089" for flag --offset`, err.Error())
	assert.Equal(t, `a leading zero is not allowed`, errlines.Note(err))
	assert.Equal(t, `write 89`, errlines.Hint(err))
	assertUsageError(t, err)
}

func TestIntegerSyntax_UnderscoreRejected(t *testing.T) {
	t.Parallel()

	err := conga.New[integerSyntaxCLI]().Args([]string{"--offset", "1_000"}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid integer value "1_000" for flag --offset`, err.Error())
	assertUsageError(t, err)
}

func TestIntegerSyntax_PrefixWithoutDigits(t *testing.T) {
	t.Parallel()

	err := conga.New[integerSyntaxCLI]().Args([]string{"--offset", "0x"}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid integer value "0x" for flag --offset`, err.Error())
	assertUsageError(t, err)
}

func TestIntegerSyntax_PositionalPrefixed(t *testing.T) {
	t.Parallel()

	cli := new(integerSyntaxCLI)
	app := conga.New(cli).Args([]string{"0b11"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, int8(3), cli.Count.Get())
}

func TestIntegerSyntax_PositionalLeadingZero(t *testing.T) {
	t.Parallel()

	err := conga.New[integerSyntaxCLI]().Args([]string{"07"}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid integer value "07" for argument <count>`, err.Error())
	assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
	assert.Equal(t, `write 0o7 for octal or 7 for decimal`, errlines.Hint(err))
	assertUsageError(t, err)
}

func TestIntegerSyntax_PositionalPrefixedOutOfRange(t *testing.T) {
	t.Parallel()

	err := conga.New[integerSyntaxCLI]().Args([]string{"0x80"}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `value "0x80" is out of range for int8 for argument <count>`, err.Error())
	assertUsageError(t, err)
}

func TestIntegerSyntax_EnvLeadingZero(t *testing.T) {
	t.Setenv("CONGA_TEST_INTEGER_SYNTAX_MODE", "0755")

	err := conga.New[integerSyntaxEnvCLI]().Args([]string{}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid unsigned integer value "0755" for flag --mode`, err.Error())
	assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
	assert.Equal(t, `write 0o755 for octal or 755 for decimal`, errlines.Hint(err))
	assert.Contains(t, conga.ResolveError(err).Render(), `from: environment variable CONGA_TEST_INTEGER_SYNTAX_MODE`)
	assertNotUsageError(t, err)
}

func TestIntegerSyntax_EnvPrefixed(t *testing.T) {
	t.Setenv("CONGA_TEST_INTEGER_SYNTAX_MODE", "0o755")

	cli := new(integerSyntaxEnvCLI)
	app := conga.New(cli).Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, uint32(493), cli.Mode.Get())
	assert.Equal(t, conga.SourceEnv, cli.Mode.Source())
}

func TestIntegerSyntax_ConfigStringLeadingZero(t *testing.T) {
	t.Parallel()

	app := conga.New[integerSyntaxConfigCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"mode": "0755"}`))).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid unsigned integer value "0755" for flag --mode`, err.Error())
	assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
	assert.Equal(t, `write 0o755 for octal or 755 for decimal`, errlines.Hint(err))
	assert.Contains(t, conga.ResolveError(err).Render(), `from: config key "mode"`)
	assertNotUsageError(t, err)
}

func TestIntegerSyntax_ConfigStringPrefixed(t *testing.T) {
	t.Parallel()

	cli := new(integerSyntaxConfigCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"mode": "0x1ed"}`))).
		Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, uint32(493), cli.Mode.Get())
	assert.Equal(t, conga.SourceConfig, cli.Mode.Source())
}

func TestIntegerSyntax_DefaultTag(t *testing.T) {
	t.Parallel()

	t.Run("prefixed default", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Mode conga.OptionalFlag[uint32] `default:"0o644" help:"File mode"`
		}

		cli := new(CLI)
		app := conga.New(cli).Args([]string{})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, uint32(420), cli.Mode.Get())
		assert.Equal(t, conga.SourceDefault, cli.Mode.Source())
	})

	t.Run("leading zero default", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Mode conga.OptionalFlag[uint32] `default:"0644" help:"File mode"`
		}

		result := runClassification(t, conga.New[CLI]().Args([]string{}))

		assert.Error(t, result.err)
		assert.Equal(t, `invalid unsigned integer value "0644" for flag --mode`, result.err.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(result.err))
		assert.Equal(t, `write 0o644 for octal or 644 for decimal`, errlines.Hint(result.err))
		assert.Contains(t, conga.ResolveError(result.err).Render(), `from: default value`)
		assertNotUsageError(t, result.err)
		assert.Equal(t, 1, result.exitCode)
	})
}

func TestIntegerSyntax_EnumComparesNumbers(t *testing.T) {
	t.Parallel()

	accepted := []struct {
		name     string
		args     []string
		expected int
	}{
		{name: "decimal of hex enum value", args: []string{"--pick", "16"}, expected: 16},
		{name: "hex as written", args: []string{"--pick", "0x10"}, expected: 16},
		{name: "octal of hex enum value", args: []string{"--pick", "0o20"}, expected: 16},
		{name: "hex of decimal enum value", args: []string{"--pick", "0x14"}, expected: 20},
	}

	for _, testCase := range accepted {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cli := new(integerSyntaxEnumCLI)
			app := conga.New(cli).Args(testCase.args)

			err := app.RunContext(t.Context())

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, cli.Pick.Get())
		})
	}
}

func TestIntegerSyntax_EnumRejectsOtherNumber(t *testing.T) {
	t.Parallel()

	err := conga.New[integerSyntaxEnumCLI]().Args([]string{"--pick", "0x11"}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid value "0x11" for flag --pick`, err.Error())
	assert.Equal(t, `accepted values: 0x10, 20`, errlines.Note(err))
	assertUsageError(t, err)
}

func TestIntegerSyntax_EnumRejectsLastRepeatedValue(t *testing.T) {
	t.Parallel()

	err := conga.New[integerSyntaxEnumCLI]().Args([]string{"--pick", "0x10", "--pick", "0o21"}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid value "0o21" for flag --pick`, err.Error())
	assert.Equal(t, `accepted values: 0x10, 20`, errlines.Note(err))
	assertUsageError(t, err)
}

func TestIntegerSyntax_EnumEnvSpelling(t *testing.T) {
	t.Setenv("CONGA_TEST_INTEGER_SYNTAX_PICK", "0x11")

	type CLI struct {
		noopRunner

		Pick conga.OptionalFlag[int] `enum:"0x10,20" env:"CONGA_TEST_INTEGER_SYNTAX_PICK" help:"Pick"`
	}

	err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid value "0x11" for flag --pick`, err.Error())
	assert.Equal(t, `accepted values: 0x10, 20`, errlines.Note(err))
	assert.Contains(t, conga.ResolveError(err).Render(), `from: environment variable CONGA_TEST_INTEGER_SYNTAX_PICK`)
	assertNotUsageError(t, err)
}

func TestIntegerSyntax_EnumDefaultSpelling(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Slot conga.OptionalArg[uint16] `default:"0x7" enum:"0X1F,0" help:"Slot"`
	}

	err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `invalid value "0x7" for argument <slot>`, err.Error())
	assert.Equal(t, `accepted values: 0X1F, 0`, errlines.Note(err))
	assert.Contains(t, conga.ResolveError(err).Render(), `from: default value`)
	assertNotUsageError(t, err)
}

func TestIntegerSyntax_EnumSliceElements(t *testing.T) {
	t.Parallel()

	t.Run("accepted elements", func(t *testing.T) {
		t.Parallel()

		cli := new(integerSyntaxEnumCLI)
		app := conga.New(cli).Args([]string{"--sizes", "1,0x2"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, []uint8{1, 2}, cli.Sizes.Get())
	})

	t.Run("rejected element", func(t *testing.T) {
		t.Parallel()

		err := conga.New[integerSyntaxEnumCLI]().Args([]string{"--sizes", "1, 0x3"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `invalid value "0x3" for flag --sizes`, err.Error())
		assert.Equal(t, `accepted values: 0b1, 0o2`, errlines.Note(err))
		assertUsageError(t, err)
	})
}

func TestIntegerSyntax_EnumPositional(t *testing.T) {
	t.Parallel()

	t.Run("accepted", func(t *testing.T) {
		t.Parallel()

		cli := new(integerSyntaxEnumCLI)
		app := conga.New(cli).Args([]string{"31"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, uint16(31), cli.Slot.Get())
	})

	t.Run("rejected", func(t *testing.T) {
		t.Parallel()

		err := conga.New[integerSyntaxEnumCLI]().Args([]string{"0b101"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `invalid value "0b101" for argument <slot>`, err.Error())
		assert.Equal(t, `accepted values: 0X1F, 0`, errlines.Note(err))
		assertUsageError(t, err)
	})
}

func TestIntegerSyntax_EnumHelpKeepsSpelling(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[integerSyntaxEnumCLI]().Name("app").Stdout(&buffer).Args([]string{"--help"})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Pick [enum: 0x10, 20]")
	assert.Contains(t, buffer.String(), "Sizes [enum: 0b1, 0o2]")
	assert.NotContains(t, buffer.String(), "enum: 16")
}

func TestIntegerSyntax_EnumSchemaErrors(t *testing.T) {
	t.Parallel()

	t.Run("leading zero", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Pick conga.OptionalFlag[int] `enum:"0755,8" help:"Pick"`
		}

		err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct CLI field Pick: enum value "0755" is not a valid int`, err.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
		assert.Equal(t, `write 0o755 for octal or 755 for decimal`, errlines.Hint(err))
	})

	t.Run("not an integer", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Pick conga.OptionalArg[int] `enum:"one" help:"Pick"`
		}

		err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct CLI field Pick: enum value "one" is not a valid int`, err.Error())
	})

	t.Run("out of range for the element type", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Pick conga.OptionalFlag[[]uint8] `enum:"0x100" help:"Pick"`
		}

		err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct CLI field Pick: enum value "0x100" is out of range for uint8`, err.Error())
	})
}

func TestIntegerSyntax_EnumOnRegisteredIntegerType(t *testing.T) {
	t.Parallel()

	cli := new(integerSyntaxRegisteredCLI)
	app := conga.New(cli).Args([]string{"--level", "high"})
	app.RegisterType(func(raw string) (integerSyntaxLevel, error) {
		if raw == "high" {
			return 1, nil
		}

		return 0, nil
	})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, integerSyntaxLevel(1), cli.Level.Get())
}

func TestIntegerSyntax_RangeBounds(t *testing.T) {
	t.Parallel()

	t.Run("value at the prefixed bound", func(t *testing.T) {
		t.Parallel()

		cli := new(integerSyntaxRangeCLI)
		app := conga.New(cli).Args([]string{"--width", "0x10", "--size", "8"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, 16, cli.Width.Get())
		assert.Equal(t, uint64(8), cli.Size.Get())
	})

	t.Run("value below the prefixed bound", func(t *testing.T) {
		t.Parallel()

		err := conga.New[integerSyntaxRangeCLI]().Args([]string{"--width", "15"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `value 15 is out of range [0x10..0x20] for flag --width`, err.Error())
		assertUsageError(t, err)
	})

	t.Run("value above the prefixed bound", func(t *testing.T) {
		t.Parallel()

		err := conga.New[integerSyntaxRangeCLI]().Args([]string{"--size", "9"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `value 9 is out of range [0b1..0o10] for flag --size`, err.Error())
		assertUsageError(t, err)
	})

	t.Run("leading zero bound", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Width conga.OptionalFlag[int] `help:"Width" range:"010..20"`
		}

		err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct CLI field Width: invalid integer min bound "010"`, err.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
		assert.Equal(t, `write 0o10 for octal or 10 for decimal`, errlines.Hint(err))
	})

	t.Run("leading zero unsigned max bound", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Size conga.OptionalFlag[uint] `help:"Size" range:"1..020"`
		}

		err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct CLI field Size: invalid unsigned integer max bound "020"`, err.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
		assert.Equal(t, `write 0o20 for octal or 20 for decimal`, errlines.Hint(err))
	})
}

func TestIntegerSyntax_FeatureLevels(t *testing.T) {
	t.Parallel()

	t.Run("prefixed level", func(t *testing.T) {
		t.Parallel()

		cli := new(integerSyntaxFeatureCLI)
		app := conga.New(cli).EnableFeatures().Args([]string{"--feature", "depth=0b11"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, 3, cli.Features.Get().Depth)
	})

	t.Run("prefixed enum level", func(t *testing.T) {
		t.Parallel()

		cli := new(integerSyntaxFeatureCLI)
		app := conga.New(cli).EnableFeatures().Args([]string{"--feature", "depth=2"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, 2, cli.Features.Get().Depth)
	})

	t.Run("leading zero level", func(t *testing.T) {
		t.Parallel()

		err := conga.New[integerSyntaxFeatureCLI]().EnableFeatures().Args([]string{"--feature", "depth=02"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `invalid value "02" for feature "depth"`, err.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
		assert.Equal(t, `write 0o2 for octal or 2 for decimal`, errlines.Hint(err))
		assertUsageError(t, err)
	})

	t.Run("leading zero enum level", func(t *testing.T) {
		t.Parallel()

		type levels struct {
			Depth uint8 `enum:"02" help:"Depth level"`
		}

		type CLI struct {
			noopRunner

			Features conga.Features[levels] `flag:"feature"`
		}

		err := conga.New[CLI]().EnableFeatures().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct levels field Depth: invalid feature enum value "02"`, err.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
		assert.Equal(t, `write 0o2 for octal or 2 for decimal`, errlines.Hint(err))
	})
}

func TestIntegerSyntax_CounterEnv(t *testing.T) {
	t.Run("prefixed count", func(t *testing.T) {
		t.Setenv("CONGA_TEST_INTEGER_SYNTAX_VERBOSE", "0x3")

		cli := new(integerSyntaxCounterEnvCLI)
		app := conga.New(cli).Args([]string{})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, 3, cli.Verbose.Get())
	})

	t.Run("leading zero count", func(t *testing.T) {
		t.Setenv("CONGA_TEST_INTEGER_SYNTAX_VERBOSE", "02")

		err := conga.New[integerSyntaxCounterEnvCLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `invalid unsigned integer value "02" for flag --verbose`, err.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
		assert.Equal(t, `write 0o2 for octal or 2 for decimal`, errlines.Hint(err))
		assert.Contains(t, conga.ResolveError(err).Render(), `from: environment variable CONGA_TEST_INTEGER_SYNTAX_VERBOSE`)
		assertNotUsageError(t, err)
	})
}

func TestIntegerSyntax_CountBounds(t *testing.T) {
	t.Parallel()

	t.Run("prefixed len bounds", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Region conga.OptionalFlag[string] `help:"Region" len:"0x2..0x4"`
		}

		cli := new(CLI)
		app := conga.New(cli).Args([]string{"--region", "eu"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "eu", cli.Region.Get())
	})

	t.Run("value outside prefixed len bounds", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Region conga.OptionalFlag[string] `help:"Region" len:"0x2..0x4"`
		}

		err := conga.New[CLI]().Args([]string{"--region", "europe"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `value "europe" length 6 is out of range [0x2..0x4] for flag --region`, err.Error())
		assertUsageError(t, err)
	})

	t.Run("prefixed items bound", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Hosts conga.OptionalFlag[[]string] `help:"Hosts" items:"0b10"`
		}

		err := conga.New[CLI]().Args([]string{"--hosts", "a,b,c"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `expected exactly 0b10 items, got 3 for flag --hosts`, err.Error())
		assertUsageError(t, err)
	})

	t.Run("leading zero len bound", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Region conga.OptionalFlag[string] `help:"Region" len:"010"`
		}

		err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct CLI field Region: invalid len min bound "010"`, err.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
		assert.Equal(t, `write 0o10 for octal or 10 for decimal`, errlines.Hint(err))
	})

	t.Run("leading zero items max bound", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Hosts conga.OptionalFlag[[]string] `help:"Hosts" items:"1..05"`
		}

		err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct CLI field Hosts: invalid items max bound "05"`, err.Error())
		assert.Equal(t, `a leading zero is ambiguous`, errlines.Note(err))
		assert.Equal(t, `write 0o5 for octal or 5 for decimal`, errlines.Hint(err))
	})
}

type integerSyntaxCountCLI struct {
	noopRunner

	Exact   conga.OptionalFlag[string]   `help:"Exact"   len:"0x3"`
	AtLeast conga.OptionalFlag[string]   `help:"AtLeast" len:"0o10.."`
	AtMost  conga.OptionalFlag[string]   `help:"AtMost"  len:"..0b11"`
	Between conga.OptionalFlag[string]   `help:"Between" len:"0x2..0x4"`
	Hosts   conga.OptionalFlag[[]string] `help:"Hosts"   items:"0x2"`
	Zones   conga.OptionalFlag[[]string] `help:"Zones"   items:"0b11.."`
	Peers   conga.OptionalFlag[[]string] `help:"Peers"   items:"..0o1"`
	Nodes   conga.OptionalFlag[[]string] `help:"Nodes"   items:"0x2..0x3"`
}

func TestIntegerSyntax_CountMessagesKeepBoundSpelling(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "len exact", args: []string{"--exact", "europe"}, message: `value "europe" length 6 must be exactly 0x3 for flag --exact`},
		{name: "len min", args: []string{"--at-least", "europe"}, message: `value "europe" length 6 must be >= 0o10 for flag --at-least`},
		{name: "len max", args: []string{"--at-most", "europe"}, message: `value "europe" length 6 must be <= 0b11 for flag --at-most`},
		{name: "len min and max", args: []string{"--between", "europe"}, message: `value "europe" length 6 is out of range [0x2..0x4] for flag --between`},
		{name: "items exact", args: []string{"--hosts", "a"}, message: `expected exactly 0x2 items, got 1 for flag --hosts`},
		{name: "items min", args: []string{"--zones", "a"}, message: `expected at least 0b11 items, got 1 for flag --zones`},
		{name: "items max", args: []string{"--peers", "a,b"}, message: `expected at most 0o1 items, got 2 for flag --peers`},
		{name: "items min and max", args: []string{"--nodes", "a,b,c,d"}, message: `item count 4 is out of range [0x2..0x3] for flag --nodes`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := conga.New[integerSyntaxCountCLI]().Args(testCase.args).RunContext(t.Context())

			assert.Error(t, err)
			assert.Equal(t, testCase.message, err.Error())
			assertUsageError(t, err)
		})
	}
}

func TestIntegerSyntax_SignedCountBound(t *testing.T) {
	t.Parallel()

	t.Run("negative leading zero", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Region conga.OptionalFlag[string] `help:"Region" len:"-05"`
		}

		err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct CLI field Region: invalid len min bound "-05"`, err.Error())
		assert.Equal(t, `must be non-negative integer`, errlines.Note(err))
	})

	t.Run("negative", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Hosts conga.OptionalFlag[[]string] `help:"Hosts" items:"..-1"`
		}

		err := conga.New[CLI]().Args([]string{}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct CLI field Hosts: invalid items max bound "-1"`, err.Error())
		assert.Equal(t, `must be non-negative integer`, errlines.Note(err))
	})
}

func TestIntegerSyntax_NegativePrefixedOutOfRange(t *testing.T) {
	t.Parallel()

	t.Run("flag value", func(t *testing.T) {
		t.Parallel()

		err := conga.New[integerSyntaxCLI]().Args([]string{"--offset", "-0x8000000000000001"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `value "-0x8000000000000001" is out of range for int for flag --offset`, err.Error())
		assertUsageError(t, err)
	})

	t.Run("positional", func(t *testing.T) {
		t.Parallel()

		err := conga.New[integerSyntaxCLI]().Args([]string{"-0x81"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `value "-0x81" is out of range for int8 for argument <count>`, err.Error())
		assertUsageError(t, err)
	})

	t.Run("positional beyond int64", func(t *testing.T) {
		t.Parallel()

		err := conga.New[integerSyntaxCLI]().Args([]string{"-0x8000000000000001"}).RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `value "-0x8000000000000001" is out of range for int8 for argument <count>`, err.Error())
		assertUsageError(t, err)
	})
}
