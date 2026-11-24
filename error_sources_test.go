package conga_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	_ "embed"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
)

//go:embed testdata/errors/stdin_read_twice.golden
var goldenErrorsStdinReadTwice string

// sourceEven is a domain type whose validator rejects odd values.
type sourceEven int

func (e *sourceEven) UnmarshalText(text []byte) error {
	parsed, err := strconv.Atoi(string(text))
	if err != nil {
		return err
	}

	*e = sourceEven(parsed)

	return nil
}

func (e sourceEven) Validate(_ context.Context) error {
	if e%2 != 0 {
		return fmt.Errorf("value %d is odd", int(e))
	}

	return nil
}

// sourceFlagsCLI declares one flag per value-failure shape, each readable from
// the command line, the environment and the configuration file.
type sourceFlagsCLI struct {
	noopRunner

	Port    conga.OptionalFlag[int]                   `env:"SRC_PORT"   name:"port"`
	Level   conga.OptionalFlag[string]                `enum:"low,high"  env:"SRC_LEVEL" name:"level"`
	Size    conga.OptionalFlag[int]                   `env:"SRC_SIZE"   name:"size"     range:"1..10"`
	Label   conga.OptionalFlag[string]                `env:"SRC_LABEL"  len:"3..5"      name:"label"`
	Zones   conga.OptionalFlag[[]string]              `env:"SRC_ZONES"  items:"2..3"    name:"zones"`
	Limits  conga.OptionalFlag[map[string]int]        `env:"SRC_LIMITS" name:"limits"`
	Evens   conga.OptionalFlag[map[string]sourceEven] `env:"SRC_EVENS"  name:"evens"`
	Single  conga.OptionalFlag[sourceEven]            `env:"SRC_SINGLE" name:"single"`
	Verbose conga.Counter                             `env:"SRC_V"      name:"v"`
}

type sourceDefaultsCLI struct {
	noopRunner

	Port  conga.OptionalFlag[int]               `default:"abc" name:"port"`
	Size  conga.OptionalFlag[int]               `default:"11"  name:"size"  range:"1..10"`
	Items conga.OptionalFlag[map[string]string] `default:"a=1" items:"2..3" name:"items"`
	Even  conga.OptionalFlag[sourceEven]        `default:"3"   name:"even"`
}

type sourceArgCLI struct {
	noopRunner

	Count conga.RequiredArg[int] `name:"<count>"`
}

type sourceRangeArgCLI struct {
	noopRunner

	Count conga.RequiredArg[int] `name:"<count>" range:"1..5"`
}

type sourceDefaultArgCLI struct {
	noopRunner

	Count conga.OptionalArg[int] `default:"abc" name:"[count]"`
}

type sourceStdinCLI struct {
	noopRunner

	First  conga.OptionalFlag[conga.FileContent] `env:"SRC_FIRST"  name:"first"`
	Second conga.OptionalFlag[conga.FileContent] `env:"SRC_SECOND" name:"second"`
}

// TestErrorSources walks every value failure through RunContext and App.Exit
// for each source that can produce it. Only the command line produces a
// *conga.UsageError. Every other source exits 1, and no value failure prints a
// usage block unless it names no flag, argument or feature.
func TestErrorSources(t *testing.T) {
	type sourceCase struct {
		name       string
		environ    map[string]string
		config     string
		run        func(t *testing.T, config string) classificationRunResult
		message    string
		causeType  string
		usage      bool
		exitCode   int
		usageShown bool
		output     string // the exact output, when the row pins it
	}

	flagsRun := func(args ...string) func(t *testing.T, config string) classificationRunResult {
		return func(t *testing.T, config string) classificationRunResult {
			t.Helper()

			app := conga.New[sourceFlagsCLI]().RegisterDecoder(".json", json.Decode).Args(args)
			if config != "" {
				app = app.ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, config)))
			}

			return runClassification(t, app)
		}
	}

	stdinRun := func(args ...string) func(t *testing.T, config string) classificationRunResult {
		return func(t *testing.T, config string) classificationRunResult {
			t.Helper()

			app := conga.New[sourceStdinCLI]().RegisterDecoder(".json", json.Decode).Args(args)
			if config != "" {
				app = app.ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, config)))
			}

			return runClassification(t, app)
		}
	}

	testCases := []sourceCase{
		// Conversion.
		{
			name:       "flag conversion from the command line",
			run:        flagsRun("--port", "abc"),
			message:    `invalid integer value "abc" for flag --port`,
			causeType:  "*coerce.numError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "flag conversion from the environment",
			environ:    map[string]string{"SRC_PORT": "abc"},
			run:        flagsRun(),
			message:    `invalid integer value "abc" for flag --port (from environment variable SRC_PORT)`,
			causeType:  "*coerce.numError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "flag conversion from the config file",
			config:     `{"port": "abc"}`,
			run:        flagsRun(),
			message:    `invalid integer value "abc" for flag --port (from config key "port")`,
			causeType:  "*coerce.numError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "flag conversion from a default",
			run: func(t *testing.T, _ string) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[sourceDefaultsCLI]().Args([]string{"--size", "5", "--items", "a=1,b=2", "--even", "2"}))
			},
			message:    `invalid integer value "abc" for flag --port (from default value)`,
			causeType:  "*coerce.numError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "counter count from the environment",
			environ:    map[string]string{"SRC_V": "-1"},
			run:        flagsRun(),
			message:    `invalid unsigned integer value "-1" for flag --v (from environment variable SRC_V)`,
			causeType:  "*intx.Error",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "counter count from the config file",
			config:     `{"v": "-1"}`,
			run:        flagsRun(),
			message:    `invalid unsigned integer value "-1" for flag --v (from config key "v")`,
			causeType:  "*intx.Error",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "map entry conversion from the command line",
			run:        flagsRun("--limits", "a=x"),
			message:    `value for key "a": invalid integer value "x" for flag --limits`,
			causeType:  "*conga.mapEntryError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "map entry conversion from the environment",
			environ:    map[string]string{"SRC_LIMITS": "a=x"},
			run:        flagsRun(),
			message:    `value for key "a": invalid integer value "x" for flag --limits (from environment variable SRC_LIMITS)`,
			causeType:  "*conga.mapEntryError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "map entry conversion from the config file",
			config:     `{"limits": {"a": "x"}}`,
			run:        flagsRun(),
			message:    `value for key "a": invalid integer value "x" for flag --limits (from config key "limits")`,
			causeType:  "*conga.mapEntryError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "argument conversion from the command line",
			run:        argRun[sourceArgCLI]("x"),
			message:    `invalid integer value "x" for argument <count>`,
			causeType:  "*coerce.numError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "argument conversion from a default",
			run:        argRun[sourceDefaultArgCLI](),
			message:    `invalid integer value "abc" for argument [count] (from default value)`,
			causeType:  "*coerce.numError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		// Constraints.
		{
			name:       "enum from the command line",
			run:        flagsRun("--level", "mid"),
			message:    `invalid value "mid" (must be one of: low, high) for flag --level`,
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "enum from the environment",
			environ:    map[string]string{"SRC_LEVEL": "mid"},
			run:        flagsRun(),
			message:    `invalid value "mid" (must be one of: low, high) for flag --level (from environment variable SRC_LEVEL)`,
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "enum from the config file",
			config:     `{"level": "mid"}`,
			run:        flagsRun(),
			message:    `invalid value "mid" (must be one of: low, high) for flag --level (from config key "level")`,
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "range from the command line",
			run:        flagsRun("--size", "11"),
			message:    `value 11 is out of range [1..10] for flag --size`,
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "range from the environment",
			environ:    map[string]string{"SRC_SIZE": "11"},
			run:        flagsRun(),
			message:    `value 11 is out of range [1..10] for flag --size (from environment variable SRC_SIZE)`,
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "range from the config file",
			config:     `{"size": 11}`,
			run:        flagsRun(),
			message:    `value 11 is out of range [1..10] for flag --size (from config key "size")`,
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "range from a default",
			run: func(t *testing.T, _ string) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[sourceDefaultsCLI]().Args([]string{"--port", "1", "--items", "a=1,b=2", "--even", "2"}))
			},
			message:    `value 11 is out of range [1..10] for flag --size (from default value)`,
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "length from the command line",
			run:        flagsRun("--label", "ab"),
			message:    `value "ab" length 2 is out of range [3..5] for flag --label`,
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "length from the environment",
			environ:    map[string]string{"SRC_LABEL": "ab"},
			run:        flagsRun(),
			message:    `value "ab" length 2 is out of range [3..5] for flag --label (from environment variable SRC_LABEL)`,
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "item count from the command line",
			run:        flagsRun("--zones", "a"),
			message:    `item count 1 is out of range [2..3] for flag --zones`,
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "item count from the config file",
			config:     `{"zones": ["a"]}`,
			run:        flagsRun(),
			message:    `item count 1 is out of range [2..3] for flag --zones (from config key "zones")`,
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "map item count from a default",
			run: func(t *testing.T, _ string) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[sourceDefaultsCLI]().Args([]string{"--port", "1", "--size", "5", "--even", "2"}))
			},
			message:    `item count 1 is out of range [2..3] for flag --items (from default value)`,
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "argument range from the command line",
			run:        argRun[sourceRangeArgCLI]("9"),
			message:    `value 9 is out of range [1..5] for argument <count>`,
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		// Validation of the converted value.
		{
			name:       "validator from the command line",
			run:        flagsRun("--single", "3"),
			message:    `value 3 is odd for flag --single`,
			causeType:  "*errors.errorString",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "validator from the environment",
			environ:    map[string]string{"SRC_SINGLE": "3"},
			run:        flagsRun(),
			message:    `value 3 is odd for flag --single (from environment variable SRC_SINGLE)`,
			causeType:  "*errors.errorString",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:       "map entry validator from the command line",
			run:        flagsRun("--evens", "a=3"),
			message:    `value 3 is odd for flag --evens entry "a"`,
			causeType:  "*errors.errorString",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "map entry validator from the config file",
			config:     `{"evens": {"a": "3"}}`,
			run:        flagsRun(),
			message:    `value 3 is odd for flag --evens entry "a" (from config key "evens")`,
			causeType:  "*errors.errorString",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "validator from a default",
			run: func(t *testing.T, _ string) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[sourceDefaultsCLI]().Args([]string{"--port", "1", "--size", "5", "--items", "a=1,b=2"}))
			},
			message:    `value 3 is odd for flag --even (from default value)`,
			causeType:  "*errors.errorString",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		// Built-in flags, which only the command line can give.
		{
			name:       "help flag with a bad boolean",
			run:        flagsRun("--help=maybe"),
			message:    `invalid boolean value "maybe", expected true or false (or 1 or 0) for flag --help`,
			causeType:  "*coerce.booleanError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:       "version flag with a bad boolean",
			run:        flagsRun("--version=maybe"),
			message:    `invalid boolean value "maybe", expected true or false (or 1 or 0) for flag --version`,
			causeType:  "*coerce.booleanError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		// Conflicts that name no flag, argument or feature keep the usage block.
		{
			name:       "stdin read twice from the command line",
			run:        stdinRun("--first", "-", "--second", "-"),
			message:    `stdin can only be read once (--first and --second both use "-")`,
			usage:      true,
			exitCode:   2,
			usageShown: true,
			output:     goldenErrorsStdinReadTwice,
		},
		{
			name:       "stdin read twice from the environment",
			environ:    map[string]string{"SRC_FIRST": "-", "SRC_SECOND": "-"},
			run:        stdinRun(),
			message:    `stdin can only be read once (--first and --second both use "-")`,
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for key, value := range testCase.environ {
				t.Setenv(key, value)
			}

			result := testCase.run(t, testCase.config)

			assert.Error(t, result.err)
			assert.Equal(t, testCase.message, result.err.Error())

			_, isUsage := errors.AsType[*conga.UsageError](result.err)
			assert.Equal(t, testCase.usage, isUsage)
			assert.Equal(t, testCase.causeType, causeTypeOf(result.err))
			assert.Equal(t, testCase.exitCode, result.exitCode)

			expectedPrefix := "app: error: " + testCase.message + "\n"

			assert.True(t, strings.HasPrefix(result.output, expectedPrefix))
			assert.Equal(t, testCase.usageShown, strings.Contains(result.output, "\nUsage:\n"))

			if testCase.output != "" {
				assert.Equal(t, testCase.output, result.output)
			} else if !testCase.usageShown {
				assert.Equal(t, expectedPrefix, result.output)
			}
		})
	}
}

// argRun returns a runner for an app of type T given args.
func argRun[T any](args ...string) func(t *testing.T, config string) classificationRunResult {
	return func(t *testing.T, _ string) classificationRunResult {
		t.Helper()

		return runClassification(t, conga.New[T]().Args(args))
	}
}

// causeTypeOf names the type of the error behind err, or "" when it has none.
func causeTypeOf(err error) string {
	cause := errors.Unwrap(err)
	if cause == nil {
		return ""
	}

	return fmt.Sprintf("%T", cause)
}
