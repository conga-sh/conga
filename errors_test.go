package conga_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/internal/errlines"
	"github.com/conga-sh/conga/internal/help"
	"github.com/conga-sh/conga/internal/spec"
	"github.com/conga-sh/conga/predict"
)

type TextUnmarshalerType struct {
	Value string
}

func (t *TextUnmarshalerType) UnmarshalText(text []byte) error {
	value := string(text)
	if value == "fail" {
		return errors.New("custom unmarshal failure")
	}

	t.Value = value

	return nil
}

type CustomTypeCLI struct {
	noopRunner

	Custom conga.OptionalFlag[TextUnmarshalerType]  `name:"custom"`
	Ptr    conga.OptionalFlag[*TextUnmarshalerType] `name:"ptr"`
	URL    conga.OptionalFlag[*url.URL]             `name:"url"`
}

type SubcommandParentCLI struct {
	Cluster conga.Command[SubcommandChildCLI] `name:"cluster"`
}

type SubcommandChildCLI struct {
	Action conga.OptionalFlag[string] `name:"action"`
}

func (s *SubcommandChildCLI) Run(_ context.Context, _ *conga.Session) error {
	return nil
}

type SliceEnumCLI struct {
	noopRunner

	Formats conga.OptionalFlag[[]string] `enum:"json,yaml,table" name:"format"`
}

// upperEnum normalizes its input to upper case, so enum membership must be
// checked against the coerced value on both flags and positional arguments.
type upperEnum string

func (u *upperEnum) UnmarshalText(text []byte) error {
	*u = upperEnum(strings.ToUpper(strings.TrimSpace(string(text))))

	return nil
}

type NormalizingEnumCLI struct {
	noopRunner

	Method conga.OptionalFlag[upperEnum] `enum:"GET,POST" name:"method"`
	Verb   conga.OptionalArg[upperEnum]  `enum:"GET,POST" name:"[verb]"`
}

type PathKindCLI struct {
	noopRunner

	File conga.OptionalFlag[conga.File] `existing:"true" name:"file"`
}

type ExecutableCLI struct {
	noopRunner

	Prog conga.OptionalFlag[conga.File] `executable:"true" name:"prog"`
}

type SymlinkCLI struct {
	noopRunner

	Hook conga.OptionalFlag[conga.File] `name:"hook" symlink:"true"`
}

type FileTagExclusionCLI struct {
	noopRunner

	Input conga.OptionalFlag[conga.File] `executable:"true" existing:"true" name:"input"`
}

type FileStdioExclusionCLI struct {
	noopRunner

	Output conga.OptionalFlag[conga.File] `name:"output" stdout:"true" symlink:"true"`
}

type CounterErrorCLI struct {
	noopRunner

	Count conga.Counter `name:"count"`
}

type UnsupportedTypeCLI struct {
	noopRunner

	Channel conga.OptionalFlag[chan int] `name:"ch"`
}

type ByteSizeErrorCLI struct {
	noopRunner

	MaxMemory conga.OptionalFlag[conga.ByteSize] `name:"max-memory"`
}

type MissingRequiredFlagCLI struct {
	noopRunner

	Port conga.RequiredFlag[int] `name:"port"`
}

type MissingOneOfFlagCLI struct {
	noopRunner

	JSON conga.OptionalFlag[bool] `name:"json" oneof:"format"`
	YAML conga.OptionalFlag[bool] `name:"yaml" oneof:"format"`
}

func TestErrorTypesByFailure(t *testing.T) {
	t.Parallel()

	t.Run("schema", func(t *testing.T) {
		t.Parallel()

		_, err := spec.Build(42, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "target must be a pointer to a struct, got int", err.Error())
	})

	t.Run("unknown flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[XorDefaultCLI]().Args([]string{"--bogus"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
		assert.Equal(t, "unknown flag: --bogus", err.Error())
	})

	t.Run("flag requires a value", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().Args([]string{"--api-key", "k", "--port"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
		assert.Equal(t, "flag requires an argument: --port", err.Error())
	})

	t.Run("mutually exclusive flags", func(t *testing.T) {
		t.Parallel()

		app := conga.New[XorDefaultCLI]().Args([]string{"--immediate", "--graceful"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
		assert.Equal(t, "flags --graceful and --immediate are mutually exclusive", err.Error())
	})

	t.Run("missing required flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().Args([]string{"file.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
		assert.Equal(t, "missing required flag: --api-key", err.Error())
	})

	t.Run("invalid enum value", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().Args([]string{"--api-key", "k", "--format", "invalid", "file.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
	})

	t.Run("invalid value for the flag type", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().Args([]string{"--api-key", "k", "-p", "not-a-number", "file.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
		assert.Equal(t, `invalid integer value "not-a-number" for flag --port`, err.Error())
	})

	t.Run("config read failure", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().
			ConfigOptions(conga.ConfigExplicit(filepath.Join(t.TempDir(), "missing.json"))).
			Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assertNotUsageError(t, err)
		assert.IsError(t, err, fs.ErrNotExist)
	})
}

func TestErrorPathNotFound(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "missing.txt")

	decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"existing": "true"}}

	var file conga.File

	err := file.UnmarshalTextWithContext([]byte(missingPath), decodeContext)
	assert.NoError(t, err)

	err = file.Validate(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, fs.ErrNotExist)
}

func TestErrorPathWrongKind(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("x"), 0o600)
	assert.NoError(t, err)

	var file conga.File

	decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"existing": "true"}}

	err = file.UnmarshalTextWithContext([]byte(tmpDir), decodeContext)
	assert.NoError(t, err)

	err = file.Validate(t.Context())

	assert.Error(t, err)
	assertNotUsageError(t, err)
	assert.Equal(t, fmt.Sprintf("%q is a directory, but a file was expected", tmpDir), err.Error())
}

func TestValidationErrorPathKindRender(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	app := conga.New[PathKindCLI]().Args([]string{"--file", tmpDir})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Equal(t, fmt.Sprintf("%q is a directory, but a file was expected for flag --file", tmpDir), err.Error())
}

func TestValidationErrorExecutableRender(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("execute-bit check is not enforced on Windows")
	}

	progPath := filepath.Join(t.TempDir(), "prog")
	assert.NoError(t, os.WriteFile(progPath, []byte("#!/bin/sh\n"), 0o644))

	app := conga.New[ExecutableCLI]().Args([]string{"--prog", progPath})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Equal(t, fmt.Sprintf("%q is not executable for flag --prog", progPath), err.Error())
}

func TestValidationErrorSymlinkRender(t *testing.T) {
	t.Parallel()

	hookPath := filepath.Join(t.TempDir(), "hook.sh")
	assert.NoError(t, os.WriteFile(hookPath, []byte("#!/bin/sh\n"), 0o755))

	app := conga.New[SymlinkCLI]().Args([]string{"--hook", hookPath})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Equal(t, fmt.Sprintf("%q is not a symbolic link for flag --hook", hookPath), err.Error())
}

func TestSchemaErrorFileTagExclusionExistingExecutable(t *testing.T) {
	t.Parallel()

	err := conga.New[FileTagExclusionCLI]().RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `struct FileTagExclusionCLI field Input: tag "existing" cannot be combined with tag "executable"`, err.Error())
}

func TestSchemaErrorFileTagExclusionSymlinkStdout(t *testing.T) {
	t.Parallel()

	err := conga.New[FileStdioExclusionCLI]().RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, `struct FileStdioExclusionCLI field Output: tag "stdout" cannot be combined with tag "symlink"`, err.Error())
}

func TestCoercionErrorCounterRender(t *testing.T) {
	t.Parallel()

	app := conga.New[CounterErrorCLI]().
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"count": "abc"}`))).
		Args([]string{})
	err := app.RunContext(t.Context())

	assertNotUsageError(t, err)
	assert.Equal(t, `invalid unsigned integer value "abc" for flag --count`, err.Error())
	assert.Contains(t, fmt.Sprintf("%+v", err), `from: config key "count"`)
}

func TestSchemaErrorUnsupportedType(t *testing.T) {
	t.Parallel()

	app := conga.New[UnsupportedTypeCLI]().Args(nil)
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.Equal(t, "struct UnsupportedTypeCLI field Channel: unsupported type chan int", err.Error())
	assert.Equal(t, "register a parser with app.RegisterType", errlines.Hint(err))
}

func TestCoercionErrorByteSizeRender(t *testing.T) {
	t.Parallel()

	app := conga.New[ByteSizeErrorCLI]().Args([]string{"--max-memory", "10ZB"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Equal(t, `unknown byte size unit "ZB" in "10ZB" for flag --max-memory`, err.Error())
	assert.Equal(t, `accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB`, errlines.Note(err))
}

func TestCoercionErrorByteSizeAmbiguousUnitRender(t *testing.T) {
	t.Parallel()

	app := conga.New[ByteSizeErrorCLI]().Args([]string{"--max-memory", "10KB"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Equal(t, `ambiguous byte size unit "KB" in "10KB" for flag --max-memory`, err.Error())
	assert.Equal(t, `use kB (1000) or KiB (1024)`, errlines.Hint(err))
}

// classificationCLI declares one field per value-failure shape exercised by
// TestErrorClassificationBySource: CLI, environment, configuration, maps,
// path capabilities, and file-content coercion.
type classificationCLI struct {
	noopRunner

	Port    conga.OptionalFlag[int]               `env:"APP_PORT"    name:"port"`
	Mode    conga.OptionalFlag[string]            `enum:"dev,prod"   env:"APP_MODE"   name:"mode"`
	Workers conga.OptionalFlag[int]               `name:"workers"    range:"1..8"`
	In      conga.OptionalFlag[conga.File]        `env:"APP_IN"      existing:"true"  name:"in"`
	Exe     conga.OptionalFlag[conga.File]        `executable:"true" name:"exe"`
	Body    conga.OptionalFlag[conga.FileContent] `name:"body"`
	Labels  conga.OptionalFlag[map[string]string] `env:"APP_LABELS"  name:"labels"`
	Counts  conga.OptionalFlag[map[string]int]    `name:"counts"`
	Verbose conga.OptionalFlag[bool]              `name:"verbose"    negatable:"true"`
}

// classificationXorCLI, classificationOneOfCLI, and classificationAndCLI each
// declare one flag group whose members read from the environment and the
// configuration file, so the group error's type follows the highest source
// among the supplied members.
type classificationXorCLI struct {
	noopRunner

	Fast conga.OptionalFlag[bool] `env:"APP_FAST" name:"fast" xor:"speed"`
	Slow conga.OptionalFlag[bool] `env:"APP_SLOW" name:"slow" xor:"speed"`
}

type classificationOneOfCLI struct {
	noopRunner

	JSON conga.OptionalFlag[bool] `env:"APP_JSON" name:"json" oneof:"format"`
	YAML conga.OptionalFlag[bool] `env:"APP_YAML" name:"yaml" oneof:"format"`
}

type classificationAndCLI struct {
	noopRunner

	User conga.OptionalFlag[string] `and:"credentials" env:"APP_USER" name:"user"`
	Pass conga.OptionalFlag[string] `and:"credentials" env:"APP_PASS" name:"pass"`
}

type classificationFeatureSet struct {
	Level int `enum:"1,2"`
}

type classificationFeaturesCLI struct {
	noopRunner

	Features conga.Features[classificationFeatureSet] `config:"experiments" env:"APP_FEATURES" flag:"features"`
}

type classificationNoRunnerCLI struct {
	Flag conga.OptionalFlag[string] `name:"flag"`
}

type configKeySubcommand struct{}

func (c *configKeySubcommand) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// configKeyAncestorCLI declares every attributed value on the root and routes
// to a subcommand, so config keys must be reported for the declaring command.
type configKeyAncestorCLI struct {
	Port   conga.OptionalFlag[int]            `name:"port"`
	In     conga.OptionalFlag[conga.File]     `existing:"true" name:"in"`
	Counts conga.OptionalFlag[map[string]int] `name:"counts"`

	Deploy conga.Command[configKeySubcommand] `summary:"Deploy"`
}

// classificationRunResult is one failed invocation captured through App.Exit,
// mirroring how an end user observes the failure.
type classificationRunResult struct {
	err        error
	output     string
	exitCode   int
	usageShown bool
}

// runClassification executes app and routes its error through App.Exit, so the
// table asserts the same exit code and stderr rendering a user sees.
func runClassification[T any](t *testing.T, app *conga.App[T]) classificationRunResult {
	t.Helper()

	var (
		buffer   bytes.Buffer
		exitCode = -1
	)

	app.Name("app").
		Stderr(&buffer).
		ExitFunc(func(code int) { exitCode = code })

	err := app.RunContext(t.Context())

	app.Exit(err)

	output := buffer.String()

	return classificationRunResult{
		err:        err,
		output:     output,
		exitCode:   exitCode,
		usageShown: strings.Contains(output, "\nUsage:\n"),
	}
}

func classificationApp() *conga.App[classificationCLI] {
	return conga.New[classificationCLI]().
		RegisterDecoder(".json", json.Decode).
		Args([]string{})
}

// TestErrorClassificationBySource is the single table that #162 asks for: one
// subtest per failure shape, asserting the exact message, the error type,
// the usage classification, the exit code through App.Exit, and whether the
// usage block renders.
//
//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorClassificationBySource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Log("path-kind, executable-bit, and $PATH messages are Unix-specific; those rows are skipped")
	}

	missingPath := filepath.Join(t.TempDir(), "missing.txt")
	noExecPath := filepath.Join(t.TempDir(), "noexec")

	assert.NoError(t, os.WriteFile(noExecPath, []byte("#!/bin/sh\n"), 0o644))

	dirPath := t.TempDir()

	testCases := []struct {
		name          string
		run           func(t *testing.T) classificationRunResult
		message       string
		from          []string // a prefix of each origin line, in order
		note          string
		hint          string
		causeType     string
		usage         bool
		exitCode      int
		usageShown    bool
		skipOnWindows bool
	}{
		{
			name: "default positional range constraint",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				type CLI struct {
					noopRunner

					Count conga.OptionalArg[int] `default:"5000" name:"[count]" range:"1..1000"`
				}

				return runClassification(t, conga.New[CLI]().Args([]string{}))
			},
			message:    `value 5000 is out of range [1..1000] for argument [count]`,
			from:       []string{`default value`},
			causeType:  "",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "command-line wrong path kind",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().Args([]string{"--in", dirPath}))
			},
			message:    fmt.Sprintf("%q is a directory, but a file was expected for flag --in", dirPath),
			causeType:  "*conga.pathKindError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name: "configuration wrong path kind",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().
					ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"in": "`+dirPath+`"}`))).
					Args([]string{}))
			},
			message:    fmt.Sprintf("%q is a directory, but a file was expected for flag --in", dirPath),
			from:       []string{`config key "in"`},
			causeType:  "*conga.pathKindError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "command-line existing path missing",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().Args([]string{"--in", missingPath}))
			},
			message:    fmt.Sprintf("stat %s: no such file or directory for flag --in", missingPath),
			causeType:  "*fs.PathError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name: "environment existing path missing",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Setenv("APP_IN", missingPath)

				return runClassification(t, classificationApp())
			},
			message:    fmt.Sprintf("stat %s: no such file or directory for flag --in", missingPath),
			from:       []string{`environment variable APP_IN`},
			causeType:  "*fs.PathError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name:          "command-line executable bit missing",
			skipOnWindows: true,
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().Args([]string{"--exe", noExecPath}))
			},
			message:    fmt.Sprintf("%q is not executable for flag --exe", noExecPath),
			causeType:  "*conga.executableError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:          "command-line executable lookup miss",
			skipOnWindows: true,
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().Args([]string{"--exe", "nosuchtool"}))
			},
			message:    `exec: "nosuchtool": executable file not found in $PATH for flag --exe`,
			causeType:  "*exec.Error",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name:          "command-line executable absolute path missing",
			skipOnWindows: true,
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().Args([]string{"--exe", missingPath}))
			},
			message:    fmt.Sprintf("stat %s: no such file or directory for flag --exe", missingPath),
			causeType:  "*fs.PathError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name: "command-line file content missing",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().Args([]string{"--body", missingPath}))
			},
			message:    fmt.Sprintf("stat %s: no such file or directory for flag --body", missingPath),
			causeType:  "*fs.PathError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name: "command-line file content wrong kind",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().Args([]string{"--body", dirPath}))
			},
			message:    fmt.Sprintf("%q is a directory, but a file was expected for flag --body", dirPath),
			causeType:  "*conga.pathKindError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name: "unexpected argument",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().Args([]string{"extra"}))
			},
			message:    `unexpected argument "extra"`,
			causeType:  "",
			usage:      true,
			exitCode:   2,
			usageShown: true,
		},
		{
			name: "environment map entry malformed",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Setenv("APP_LABELS", "novalue")

				return runClassification(t, classificationApp())
			},
			message:    `invalid key-value pair "novalue" for flag --labels`,
			from:       []string{`environment variable APP_LABELS`},
			note:       `expected KEY=VALUE`,
			causeType:  "*coerce.mapPairError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "negated flag with value",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, classificationApp().Args([]string{"--no-verbose=true"}))
			},
			message:    "negated flag does not take a value: --no-verbose=true",
			causeType:  "",
			usage:      true,
			exitCode:   2,
			usageShown: true,
		},
		{
			name: "configuration feature invalid value",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationFeaturesCLI]().
					EnableFeatures().
					RegisterDecoder(".json", json.Decode).
					ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"experiments": {"level": 7}}`))).
					Args([]string{}))
			},
			message:    `invalid value "7" for feature "level"`,
			from:       []string{`config key "experiments.level"`},
			note:       `accepted values: 0, 1, 2`,
			causeType:  "*spec.featureValueError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "environment feature invalid value",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Setenv("APP_FEATURES", "level=7")

				return runClassification(t, conga.New[classificationFeaturesCLI]().
					EnableFeatures().
					Args([]string{}))
			},
			message:    `invalid value "7" for feature "level"`,
			from:       []string{`environment variable APP_FEATURES`},
			note:       `accepted values: 0, 1, 2`,
			causeType:  "*spec.featureValueError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "configuration feature unknown name",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Chdir(t.TempDir())
				assert.NoError(t, os.WriteFile("config.json", []byte(`{"experiments": {"bogus": true}}`), 0o600))

				return runClassification(t, conga.New[classificationFeaturesCLI]().
					EnableFeatures().
					RegisterDecoder(".json", json.Decode).
					ConfigOptions(conga.ConfigExplicit("config.json")).
					Args([]string{}))
			},
			message:    `unknown feature "bogus" in section "experiments"`,
			from:       []string{`config file "`},
			causeType:  "",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "environment feature unknown name",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Setenv("APP_FEATURES", "bogus=true")

				return runClassification(t, conga.New[classificationFeaturesCLI]().
					EnableFeatures().
					Args([]string{}))
			},
			message:    `unknown feature "bogus"`,
			from:       []string{`environment variable APP_FEATURES`},
			causeType:  "",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "command-line feature invalid value",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationFeaturesCLI]().
					EnableFeatures().
					Args([]string{"--features", "level=7"}))
			},
			message:    `invalid value "7" for feature "level"`,
			note:       `accepted values: 0, 1, 2`,
			causeType:  "*spec.featureValueError",
			usage:      true,
			exitCode:   2,
			usageShown: false,
		},
		{
			name: "command-line feature unknown name",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationFeaturesCLI]().
					EnableFeatures().
					Args([]string{"--features", "bogus=true"}))
			},
			message:    `unknown feature "bogus"`,
			causeType:  "",
			usage:      true,
			exitCode:   2,
			usageShown: true,
		},
		{
			name:          "default-sourced path capability failure",
			skipOnWindows: true,
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationDefaultFileCLI]().Args([]string{}))
			},
			message:    `stat /nonexistent-conga-test/missing.txt: no such file or directory for flag --in`,
			from:       []string{`default value`},
			causeType:  "*fs.PathError",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "xor group conflict between command-line members",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationXorCLI]().Args([]string{"--fast", "--slow"}))
			},
			message:    `flags --fast and --slow are mutually exclusive`,
			causeType:  "",
			usage:      true,
			exitCode:   2,
			usageShown: true,
		},
		{
			name: "xor group conflict between environment members",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Setenv("APP_FAST", "true")
				t.Setenv("APP_SLOW", "true")

				return runClassification(t, conga.New[classificationXorCLI]().Args([]string{}))
			},
			message:    `flags --fast and --slow are mutually exclusive`,
			from:       []string{`--fast: environment variable APP_FAST`, `--slow: environment variable APP_SLOW`},
			causeType:  "",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "xor group conflict between configuration members",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationXorCLI]().
					RegisterDecoder(".json", json.Decode).
					ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"fast": true, "slow": true}`))).
					Args([]string{}))
			},
			message:    `flags --fast and --slow are mutually exclusive`,
			from:       []string{`--fast: config key "fast"`, `--slow: config key "slow"`},
			causeType:  "",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "xor group conflict with one command-line member",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Setenv("APP_SLOW", "true")

				return runClassification(t, conga.New[classificationXorCLI]().Args([]string{"--fast"}))
			},
			message:    `flags --fast and --slow are mutually exclusive`,
			from:       []string{`--slow: environment variable APP_SLOW`},
			causeType:  "",
			usage:      true,
			exitCode:   2,
			usageShown: true,
		},
		{
			name: "oneof group conflict between command-line members",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationOneOfCLI]().Args([]string{"--json", "--yaml"}))
			},
			message:    `flags --json and --yaml are mutually exclusive`,
			causeType:  "",
			usage:      true,
			exitCode:   2,
			usageShown: true,
		},
		{
			name: "oneof group conflict between environment members",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Setenv("APP_JSON", "true")
				t.Setenv("APP_YAML", "true")

				return runClassification(t, conga.New[classificationOneOfCLI]().Args([]string{}))
			},
			message:    `flags --json and --yaml are mutually exclusive`,
			from:       []string{`--json: environment variable APP_JSON`, `--yaml: environment variable APP_YAML`},
			causeType:  "",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "oneof group conflict between configuration members",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationOneOfCLI]().
					RegisterDecoder(".json", json.Decode).
					ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"json": true, "yaml": true}`))).
					Args([]string{}))
			},
			message:    `flags --json and --yaml are mutually exclusive`,
			from:       []string{`--json: config key "json"`, `--yaml: config key "yaml"`},
			causeType:  "",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "oneof group conflict with one command-line member",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Setenv("APP_YAML", "true")

				return runClassification(t, conga.New[classificationOneOfCLI]().Args([]string{"--json"}))
			},
			message:    `flags --json and --yaml are mutually exclusive`,
			from:       []string{`--yaml: environment variable APP_YAML`},
			causeType:  "",
			usage:      true,
			exitCode:   2,
			usageShown: true,
		},
		{
			name: "and group with a command-line member",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationAndCLI]().Args([]string{"--user", "ada"}))
			},
			message:    `flag --user must be given together with --pass`,
			causeType:  "",
			usage:      true,
			exitCode:   2,
			usageShown: true,
		},
		{
			name: "and group with an environment member",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				t.Setenv("APP_USER", "ada")

				return runClassification(t, conga.New[classificationAndCLI]().Args([]string{}))
			},
			message:    `flag --user must be given together with --pass`,
			from:       []string{`--user: environment variable APP_USER`},
			causeType:  "",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
		{
			name: "leaf without runner",
			run: func(t *testing.T) classificationRunResult {
				t.Helper()

				return runClassification(t, conga.New[classificationNoRunnerCLI]().Args([]string{}))
			},
			message:    `struct classificationNoRunnerCLI: runner not implemented`,
			hint:       `implement conga.Runner: Run(context.Context, *conga.Session) error`,
			causeType:  "",
			usage:      false,
			exitCode:   1,
			usageShown: false,
		},
	}

	//nolint:paralleltest // subtests mutate the process environment with t.Setenv
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.skipOnWindows && runtime.GOOS == "windows" {
				t.Skip("Unix-specific path and executable diagnostics")
			}

			result := testCase.run(t)

			assert.Error(t, result.err)
			assert.Equal(t, testCase.message, result.err.Error())
			_, isUsage := errors.AsType[*conga.UsageError](result.err)
			assert.Equal(t, testCase.usage, isUsage)
			assert.Equal(t, testCase.exitCode, result.exitCode)
			assert.Equal(t, testCase.causeType, causeTypeOf(result.err))
			assert.Equal(t, testCase.note, errlines.Note(result.err))
			assert.Equal(t, testCase.hint, errlines.Hint(result.err))

			origins := errlines.Origins(result.err)

			assert.Equal(t, len(testCase.from), len(origins))

			expectedPrefix := "app: error: " + testCase.message + "\n"

			for index, from := range testCase.from {
				assert.True(t, strings.HasPrefix(origins[index], from))

				expectedPrefix += "  from: " + origins[index] + "\n"
			}

			if testCase.note != "" {
				expectedPrefix += "  note: " + testCase.note + "\n"
			}

			if testCase.hint != "" {
				expectedPrefix += "  hint: " + testCase.hint + "\n"
			}

			assert.True(t, strings.HasPrefix(result.output, expectedPrefix))
			assert.Equal(t, testCase.usageShown, strings.Contains(result.output, "\nUsage:\n"))

			if !testCase.usageShown {
				assert.Equal(t, expectedPrefix, result.output)
			}
		})
	}
}

func TestConfigKeyPath_AncestorFlagOnSubcommand(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing.txt")

	newApp := func(t *testing.T, config string) *conga.App[configKeyAncestorCLI] {
		t.Helper()

		return conga.New[configKeyAncestorCLI]().
			RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, config))).
			Args([]string{"deploy"})
	}

	t.Run("coercion", func(t *testing.T) {
		t.Parallel()

		err := newApp(t, `{"port": "abc"}`).Run()

		assertNotUsageError(t, err)
		assert.Equal(t, `invalid integer value "abc" for flag --port`, err.Error())
		assert.Contains(t, fmt.Sprintf("%+v", err), `from: config key "port"`)
	})

	t.Run("self validation", func(t *testing.T) {
		t.Parallel()

		err := newApp(t, fmt.Sprintf(`{"in": %q}`, missing)).Run()

		assertNotUsageError(t, err)
		assert.Equal(t, fmt.Sprintf("stat %s: no such file or directory for flag --in", missing), err.Error())
		assert.Contains(t, fmt.Sprintf("%+v", err), `from: config key "in" in config file "`)
	})

	t.Run("map entry", func(t *testing.T) {
		t.Parallel()

		err := newApp(t, `{"counts": {"a": "abc"}}`).Run()

		assertNotUsageError(t, err)
		assert.Equal(t, `value for key "a": invalid integer value "abc" for flag --counts`, err.Error())
		assert.Contains(t, fmt.Sprintf("%+v", err), `from: config key "counts"`)
	})
}

type classificationDefaultFileCLI struct {
	noopRunner

	In conga.OptionalFlag[conga.File] `default:"/nonexistent-conga-test/missing.txt" existing:"true" name:"in"`
}

// TestErrorClassificationDefaultCoercion pins the default-value origin suffix
// on both coercion paths, for a flag and for a positional argument.
func TestErrorClassificationDefaultCoercion(t *testing.T) {
	t.Parallel()

	t.Run("flag", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Port conga.OptionalFlag[int] `default:"abc" name:"port"`
		}

		result := runClassification(t, conga.New[CLI]().Args([]string{}))

		assert.Error(t, result.err)
		assert.Equal(t, `invalid integer value "abc" for flag --port`, result.err.Error())
		assert.Contains(t, fmt.Sprintf("%+v", result.err), `from: default value`)
		assertNotUsageError(t, result.err)
		assert.Equal(t, 1, result.exitCode)
	})

	t.Run("positional argument", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Count conga.OptionalArg[int] `default:"abc" name:"[count]"`
		}

		result := runClassification(t, conga.New[CLI]().Args([]string{}))

		assert.Error(t, result.err)
		assert.Equal(t, `invalid integer value "abc" for argument [count]`, result.err.Error())
		assert.Contains(t, fmt.Sprintf("%+v", result.err), `from: default value`)
		assertNotUsageError(t, result.err)
		assert.Equal(t, 1, result.exitCode)
	})

	t.Run("positional argument self-validation", func(t *testing.T) {
		t.Parallel()

		if runtime.GOOS == "windows" {
			t.Skip("path diagnostics are Unix-specific")
		}

		type CLI struct {
			noopRunner

			In conga.OptionalArg[conga.File] `default:"/nonexistent-conga-test/missing.txt" existing:"true" name:"[in]"`
		}

		result := runClassification(t, conga.New[CLI]().Args([]string{}))

		assert.Error(t, result.err)
		assert.Equal(t, `stat /nonexistent-conga-test/missing.txt: no such file or directory for argument [in]`, result.err.Error())
		assert.Contains(t, fmt.Sprintf("%+v", result.err), `from: default value`)
		assertNotUsageError(t, result.err)
		assert.Equal(t, 1, result.exitCode)
	})
}

type classificationUnmarshalError struct {
	raw string
}

func (e *classificationUnmarshalError) Error() string {
	return "custom config unmarshal failure"
}

type classificationUnmarshaler struct{}

func (classificationUnmarshaler) UnmarshalText(text []byte) error {
	return &classificationUnmarshalError{raw: string(text)}
}

// TestErrorClassificationCauseReachable pins that errors.As and errors.Is reach
// the application's TextUnmarshaler error and stdlib causes such as
// *fs.PathError, for command-line and config-sourced values alike.
func TestErrorClassificationCauseReachable(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Custom conga.OptionalFlag[classificationUnmarshaler] `name:"custom"`
	}

	t.Run("TextUnmarshaler error", func(t *testing.T) {
		t.Parallel()

		result := runClassification(t, conga.New[CLI]().
			RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"custom": "value"}`))).
			Args([]string{}))

		assert.Error(t, result.err)
		assert.Equal(t, `custom config unmarshal failure for flag --custom`, result.err.Error())
		assert.Contains(t, fmt.Sprintf("%+v", result.err), `from: config key "custom"`)
		assertNotUsageError(t, result.err)

		var unmarshalErr *classificationUnmarshalError
		assert.True(t, errors.As(result.err, &unmarshalErr))
		assert.Equal(t, "value", unmarshalErr.raw)
	})

	t.Run("stdlib path error", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing.txt")

		result := runClassification(t, classificationApp().
			ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"in": "`+missing+`"}`))).
			Args([]string{}))

		assert.Error(t, result.err)
		assertNotUsageError(t, result.err)
		assert.IsError(t, result.err, fs.ErrNotExist)

		var pathErr *fs.PathError
		assert.True(t, errors.As(result.err, &pathErr))
		assert.Equal(t, missing, pathErr.Path)
	})

	t.Run("CONGA's own integer error is the cause and reaches strconv.ErrSyntax", func(t *testing.T) {
		t.Parallel()

		result := runClassification(t, classificationApp().Args([]string{"--port", "abc"}))

		assertUsageError(t, result.err)
		assert.Equal(t, `invalid integer value "abc" for flag --port`, result.err.Error())
		assert.Equal(t, `invalid integer value "abc"`, errors.Unwrap(errors.Unwrap(result.err)).Error())

		assert.IsError(t, result.err, strconv.ErrSyntax)
	})

	t.Run("command-line TextUnmarshaler error", func(t *testing.T) {
		t.Parallel()

		result := runClassification(t, conga.New[CLI]().Args([]string{"--custom", "value"}))

		usage := assertUsageError(t, result.err)
		assert.Equal(t, "custom config unmarshal failure for flag --custom", result.err.Error())
		assert.Equal(t, 2, result.exitCode)
		assert.Equal(t, "app", usage.CommandPath())

		var unmarshalErr *classificationUnmarshalError
		assert.True(t, errors.As(result.err, &unmarshalErr))
		assert.Equal(t, "value", unmarshalErr.raw)
	})
}

// TestErrorClassificationExecutableBoundary pins decision 4's conversion-time
// rule against the post-coercion check it borders: a bare executable name is
// looked up on $PATH while converting and aborts before --why renders; an
// absolute missing path converts cleanly and its post-coercion File.Validate
// check is skipped when --why renders the report.
func TestErrorClassificationExecutableBoundary(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("$PATH lookup and execute-bit diagnostics are Unix-specific")
	}

	t.Run("bare name fails during conversion and aborts --why", func(t *testing.T) {
		t.Parallel()

		var stdoutBuffer bytes.Buffer

		app := conga.New[classificationCLI]().
			Name("app").
			EnableWhy().
			Stdout(&stdoutBuffer).
			Args([]string{"--exe", "nosuchtool", "--why"})

		err := app.Run()

		assert.Error(t, err)
		assertUsageError(t, err)
		assert.Equal(t, `exec: "nosuchtool": executable file not found in $PATH for flag --exe`, err.Error())
		assert.Equal(t, "", stdoutBuffer.String())
	})

	t.Run("absolute missing path converts and --why skips validation", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "nosuchtool")

		var stdoutBuffer bytes.Buffer

		app := conga.New[classificationCLI]().
			Name("app").
			EnableWhy().
			Stdout(&stdoutBuffer).
			Args([]string{"--exe", missing, "--why"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, stdoutBuffer.String(), `Value provenance for "app":`)
		assert.Contains(t, stdoutBuffer.String(), "--exe")
	})
}

// The two ./nosuchtool tests change the working directory, so they don't
// run in parallel.
func TestErrorClassificationExecutableDotSlashMissingConvertsForWhy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("$PATH lookup and execute-bit diagnostics are Unix-specific")
	}

	t.Chdir(t.TempDir())

	var stdoutBuffer bytes.Buffer

	app := conga.New[classificationCLI]().
		Name("app").
		EnableWhy().
		Stdout(&stdoutBuffer).
		Args([]string{"--exe", "./nosuchtool", "--why"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, stdoutBuffer.String(), `Value provenance for "app":`)
	assert.Contains(t, stdoutBuffer.String(), "--exe")
}

func TestErrorClassificationExecutableDotSlashMissingFailsValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("$PATH lookup and execute-bit diagnostics are Unix-specific")
	}

	workDir := t.TempDir()
	t.Chdir(workDir)

	app := conga.New[classificationCLI]().
		Name("app").
		Args([]string{"--exe", "./nosuchtool"})

	err := app.Run()

	assertUsageError(t, err)
	assert.Equal(t, fmt.Sprintf("stat %s: no such file or directory for flag --exe", filepath.Join(workDir, "nosuchtool")), err.Error())
}

// hookFailureContextKey carries, in the run context, the failure hookFailureCLI
// produces.
type hookFailureContextKey struct{}

// hookFailure names the lifecycle stage that fails with err, and the error a
// failing Finally returns.
type hookFailure struct {
	stage       string
	err         error
	finallyErr  error
	finallySeen *error
}

// hookFailureCLI fails the stage its run context names.
type hookFailureCLI struct{}

func (c *hookFailureCLI) PreRun(ctx context.Context, _ *conga.Session) error {
	if failure := c.failure(ctx); failure.stage == "PreRun" {
		return failure.err
	}

	return nil
}

func (c *hookFailureCLI) Run(ctx context.Context, _ *conga.Session) error {
	if failure := c.failure(ctx); failure.stage == "Run" {
		return failure.err
	}

	return nil
}

func (c *hookFailureCLI) PostRun(ctx context.Context, _ *conga.Session) error {
	if failure := c.failure(ctx); failure.stage == "PostRun" {
		return failure.err
	}

	return nil
}

func (c *hookFailureCLI) Finally(ctx context.Context, _ *conga.Session, err error) error {
	failure := c.failure(ctx)

	if failure.finallySeen != nil {
		*failure.finallySeen = err
	}

	return failure.finallyErr
}

func (c *hookFailureCLI) failure(ctx context.Context) hookFailure {
	failure, _ := ctx.Value(hookFailureContextKey{}).(hookFailure)

	return failure
}

type hookPassThroughError struct{ message string }

func (e *hookPassThroughError) Error() string { return e.message }

func TestHookErrorsPassThrough(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"PreRun", "Run", "PostRun"} {
		t.Run(stage+" error keeps its text behind the Error", func(t *testing.T) {
			t.Parallel()

			hookErr := &hookPassThroughError{message: stage + " failed"}

			err := conga.New[hookFailureCLI]().
				Args([]string{}).
				RunContext(context.WithValue(t.Context(), hookFailureContextKey{}, hookFailure{stage: stage, err: hookErr}))

			assert.IsError(t, err, hookErr)
			assert.Equal(t, stage+" failed", err.Error())
			assertNotUsageError(t, err)
			assert.False(t, spec.IsSchemaError(err))
		})
	}

	t.Run("a failing Finally joins with the hook error", func(t *testing.T) {
		t.Parallel()

		var (
			hookErr    = &hookPassThroughError{message: "Run failed"}
			finallyErr = &hookPassThroughError{message: "Finally failed"}
			seen       error
		)

		err := conga.New[hookFailureCLI]().
			Args([]string{}).
			RunContext(context.WithValue(t.Context(), hookFailureContextKey{}, hookFailure{stage: "Run", err: hookErr, finallyErr: finallyErr, finallySeen: &seen}))

		assert.True(t, errors.Is(err, hookErr))
		assert.True(t, errors.Is(err, finallyErr))
		assert.Equal(t, "Run failed\nFinally failed", err.Error())
		assert.True(t, seen == hookErr) //nolint:errorlint // identity, not errors.Is, is the property under test
	})

	t.Run("the standard library error from Run is not a usage error", func(t *testing.T) {
		t.Parallel()

		err := conga.New[failingCLI]().Args([]string{}).RunContext(t.Context())

		assert.IsError(t, err, fs.ErrPermission)
		assert.Equal(t, fs.ErrPermission.Error(), err.Error())
		assertNotUsageError(t, err)
	})
}

func TestDomainTypeErrorsAreRootErrors(t *testing.T) {
	t.Parallel()

	var size conga.ByteSize

	err := size.UnmarshalText([]byte("10ZB"))
	assertNotUsageError(t, err)
	assert.Equal(t, nil, errors.Unwrap(err))
	assert.Equal(t, `unknown byte size unit "ZB" in "10ZB"`, err.Error())
	assert.Equal(t, `accepted units: B, kB, MB, GB, TB, PB, EB, KiB, MiB, GiB, TiB, PiB, EiB`, errlines.Note(err))
}

func TestUsageError_NotReturnedForSchemaOrPlainErrors(t *testing.T) {
	t.Parallel()

	schemaErr := conga.New[classificationNoRunnerCLI]().Args([]string{}).RunContext(t.Context())
	assert.True(t, spec.IsSchemaError(schemaErr))
	assertNotUsageError(t, schemaErr)

	err := errors.New("database connection refused")
	assertNotUsageError(t, err)
}

func TestUsageError_EndToEnd(t *testing.T) {
	t.Parallel()

	t.Run("unknown flag parses as usage error", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner
		}

		app := conga.New[CLI]().Args([]string{"--bogus"})

		err := app.Run()

		assert.Error(t, err)
		assertUsageError(t, err)
	})

	t.Run("coercion failure is a usage error", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Port conga.OptionalFlag[int] `name:"port"`
		}

		app := conga.New[CLI]().Args([]string{"--port", "abc"})

		err := app.Run()

		assert.Error(t, err)
		assertUsageError(t, err)
	})

	t.Run("missing required flag is a usage error", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Token conga.RequiredFlag[string] `name:"token"`
		}

		app := conga.New[CLI]().Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assertUsageError(t, err)
	})
}

func TestErrNoRunner(t *testing.T) {
	t.Parallel()

	t.Run("flat command without Run", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			Flag conga.OptionalFlag[string] `name:"flag"`
		}

		app := conga.New[CLI]().Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assertNotUsageError(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct CLI: runner not implemented`, err.Error())
		assert.Equal(t, `implement conga.Runner: Run(context.Context, *conga.Session) error`, errlines.Hint(err))
	})

	t.Run("subcommand without Run", func(t *testing.T) {
		t.Parallel()

		type ChildCmd struct {
			Flag conga.OptionalFlag[string] `name:"flag"`
		}

		type ParentCLI struct {
			Subcommand conga.Command[ChildCmd] `name:"sub"`
		}

		app := conga.New[ParentCLI]().Args([]string{"sub"})

		err := app.Run()

		assert.Error(t, err)
		assertNotUsageError(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct ChildCmd: runner not implemented`, err.Error())
		assert.Equal(t, `implement conga.Runner: Run(context.Context, *conga.Session) error`, errlines.Hint(err))
	})

	t.Run("nested subcommand without Run", func(t *testing.T) {
		t.Parallel()

		type LeafCmd struct {
			Flag conga.OptionalFlag[string] `name:"flag"`
		}

		type MidCmd struct {
			Leaf conga.Command[LeafCmd] `name:"leaf"`
		}

		type RootCLI struct {
			Mid conga.Command[MidCmd] `name:"mid"`
		}

		app := conga.New[RootCLI]().Args([]string{"mid", "leaf"})

		err := app.Run()

		assert.Error(t, err)
		assertNotUsageError(t, err)
		assert.True(t, spec.IsSchemaError(err))
		assert.Equal(t, `struct LeafCmd: runner not implemented`, err.Error())
		assert.Equal(t, `implement conga.Runner: Run(context.Context, *conga.Session) error`, errlines.Hint(err))
	})
}

func TestTextUnmarshalerCoerceWrapping_Issue06(t *testing.T) {
	t.Parallel()

	app := conga.New[CustomTypeCLI]().Args([]string{"--custom", "fail"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertUsageError(t, err)
}

func TestSubcommandTypoSuggestion_Issue10(t *testing.T) {
	t.Parallel()

	app := conga.New[SubcommandParentCLI]().Args([]string{"clustr"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Equal(t, `unknown command: clustr`, err.Error())
	assert.Equal(t, `did you mean cluster?`, errlines.Hint(err))
}

func TestSubcommandRequired_Issue11(t *testing.T) {
	t.Parallel()

	app := conga.New[SubcommandParentCLI]().Name("clustermgr").Args([]string{})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assertUsageError(t, err)
	assert.Contains(t, err.Error(), `subcommand required for "clustermgr"`)
}

func TestSliceEnumValidation_Issue03(t *testing.T) {
	t.Parallel()

	t.Run("valid slice enum elements", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceEnumCLI]().Args([]string{"--format", "json,yaml"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("invalid slice enum element", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceEnumCLI]().Args([]string{"--format", "json,xml"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
	})
}

func TestNormalizingEnumValidation(t *testing.T) {
	t.Parallel()

	t.Run("flag accepts normalized value", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NormalizingEnumCLI]().Args([]string{"--method", "get"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("argument accepts normalized value", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NormalizingEnumCLI]().Args([]string{"get"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("flag rejects value outside enum", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NormalizingEnumCLI]().Args([]string{"--method", "patch"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
	})

	t.Run("argument rejects value outside enum", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NormalizingEnumCLI]().Args([]string{"patch"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
	})
}

func TestExit_OutputsAndExits(t *testing.T) {
	t.Parallel()

	t.Run("nil error exits with status 0 and no output", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode = -1
		)

		app := conga.New[SubcommandParentCLI]().
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		app.Exit(nil)

		assert.Equal(t, 0, exitCode)
		assert.Equal(t, "", buffer.String())
	})

	t.Run("calling Exit before Run outputs error and exits with status 1", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[SubcommandParentCLI]().
			Name("clustermgr").
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		app.Exit(errors.New("premature error"))

		assert.Equal(t, 1, exitCode)
		assert.Equal(t, "clustermgr: error: premature error\n", buffer.String())
	})

	t.Run("usage error prints error, usage syntax, and hint and exits with status 2", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[SubcommandParentCLI]().
			Name("clustermgr").
			Args([]string{"clustr"}).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, goldenErrorsUsageDefault, buffer.String())
	})

	t.Run("silenced usage prints only the error message", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[SubcommandParentCLI]().
			Name("clustermgr").
			Args([]string{"clustr"}).
			UsageOnError(conga.UsageSilence).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, goldenErrorsUsageSilence, buffer.String())
	})

	t.Run("usage full mode prints error and full help screen", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[SubcommandParentCLI]().
			Name("clustermgr").
			Args([]string{"clustr"}).
			UsageOnError(conga.UsageFull).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, goldenErrorsUsageFull, buffer.String())
	})

	t.Run("subcommand required error outputs error message and exits with status 2", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[SubcommandParentCLI]().
			Name("clustermgr").
			Args([]string{}).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)
		assertUsageError(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, goldenErrorsSubcommandRequired, buffer.String())
	})

	t.Run("usage hint mode prints error and only help hint", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[SubcommandParentCLI]().
			Name("clustermgr").
			Args([]string{"clustr"}).
			UsageOnError(conga.UsageHint).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, goldenErrorsUsageHint, buffer.String())
	})

	t.Run("runtime error from command execution exits with status 1 and suppresses usage", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[failingCLI]().
			Name("app").
			Args([]string{}).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 1, exitCode)
		assert.Equal(t, "app: error: permission denied\n", buffer.String())
	})

	t.Run("coercion error prints message only and exits with status 2", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[FullServerCLI]().
			Name("clustermgr").
			Args([]string{"--port", "abc"}).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, goldenErrorsInvalidValue, buffer.String())
	})
}

func TestExit_ValueErrorsRenderWithoutUsage(t *testing.T) {
	t.Parallel()

	t.Run("domain-type validation error renders message only in default usage mode", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		tmpDir := t.TempDir()

		app := conga.New[PathKindCLI]().
			Name("app").
			Args([]string{"--file", tmpDir}).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)
		assertUsageError(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, fmt.Sprintf("app: error: %q is a directory, but a file was expected for flag --file\n", tmpDir), buffer.String())
	})

	t.Run("domain-type validation error renders message only in full usage mode", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		tmpDir := t.TempDir()

		app := conga.New[PathKindCLI]().
			Name("app").
			Args([]string{"--file", tmpDir}).
			UsageOnError(conga.UsageFull).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, fmt.Sprintf("app: error: %q is a directory, but a file was expected for flag --file\n", tmpDir), buffer.String())
	})

	t.Run("custom validatable constraint error renders message only", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[CustomTypesCLI]().
			Name("app").
			Args([]string{"--score", "150"}).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, "app: error: score 150 must be between 0 and 100 for flag --score\n", buffer.String())
	})

	t.Run("missing required flag error still renders usage", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[MissingRequiredFlagCLI]().
			Name("app").
			Args([]string{}).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, goldenErrorsMissingRequiredFlag, buffer.String())
	})

	t.Run("missing oneof flag error still renders usage", func(t *testing.T) {
		t.Parallel()

		var (
			buffer   bytes.Buffer
			exitCode int
		)

		app := conga.New[MissingOneOfFlagCLI]().
			Name("app").
			Args([]string{}).
			Stderr(&buffer).
			ExitFunc(func(code int) {
				exitCode = code
			})

		err := app.RunContext(t.Context())
		assert.Error(t, err)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assert.Equal(t, goldenErrorsMissingOneOfFlag, buffer.String())
	})
}

func TestHelpAndUsageFormatting(t *testing.T) {
	t.Parallel()

	var app SubcommandParentCLI

	cmd, err := spec.Build(&app, "clustermgr", conga.SpecOptionsWithContainers(spec.Options{}))
	assert.NoError(t, err)

	usage := help.Usage([]*spec.Command{cmd})
	assert.Contains(t, usage, "Usage:\n  clustermgr <command>")
	assert.Equal(t, "Run 'clustermgr --help' for more information.", help.Hint([]*spec.Command{cmd}))
	assert.Contains(t, help.Format([]*spec.Command{cmd}), "Commands:\n  cluster")
}

func TestOptionTerminatorLookahead_Issue02(t *testing.T) {
	t.Parallel()

	type OptionTerminatorCLI struct {
		noopRunner

		Name  conga.OptionalFlag[string]  `name:"name"`
		Short conga.OptionalFlag[string]  `name:"output"     short:"o"`
		Files conga.RequiredArg[[]string] `name:"<files...>"`
	}

	t.Run("long value flag before double dash", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OptionTerminatorCLI]().Args([]string{"--name", "--", "file1.txt", "file2.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
		assert.Contains(t, err.Error(), "flag requires an argument: --name")
	})

	t.Run("short value flag before double dash", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OptionTerminatorCLI]().Args([]string{"-o", "--", "file1.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
		assert.Contains(t, err.Error(), "flag requires an argument: -o")
	})

	t.Run("attached value flag before double dash", func(t *testing.T) {
		t.Parallel()

		cli := new(OptionTerminatorCLI)
		app := conga.New(cli).Args([]string{"--name=myname", "--", "file1.txt", "file2.txt"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "myname", cli.Name.Get())
		assert.Equal(t, []string{"file1.txt", "file2.txt"}, cli.Files.Get())
	})
}

func TestPointerTargetCoercion_Issue07(t *testing.T) {
	t.Parallel()

	t.Run("pointer targets coerce successfully", func(t *testing.T) {
		t.Parallel()

		cli := new(CustomTypeCLI)
		app := conga.New(cli).Args([]string{
			"--url", "https://example.com/api/v1?query=test",
			"--ptr", "custom-pointer-val",
		})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.NotZero(t, cli.URL.Get())
		assert.Equal(t, "https", cli.URL.Get().Scheme)
		assert.Equal(t, "example.com", cli.URL.Get().Host)
		assert.Equal(t, "/api/v1", cli.URL.Get().Path)
		assert.NotZero(t, cli.Ptr.Get())
		assert.Equal(t, "custom-pointer-val", cli.Ptr.Get().Value)
	})

	t.Run("invalid URL pointer fails coercion", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CustomTypeCLI]().Args([]string{"--url", "http://[invalid-host-bracket"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
	})

	t.Run("failing pointer TextUnmarshaler fails coercion", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CustomTypeCLI]().Args([]string{"--ptr", "fail"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
	})
}

func TestJSONArrayFlattening_Issue09(t *testing.T) {
	t.Parallel()

	cfgPath := filepath.Join(t.TempDir(), "config.json")

	err := os.WriteFile(cfgPath, []byte(`{"tags": ["production", "web-tier", "frontend"], "nums": [10, 20, 30]}`), 0o600)
	assert.NoError(t, err)

	type JSONArrayCLI struct {
		noopRunner

		Tags conga.OptionalFlag[[]string] `config:"tags" name:"tag"`
		Nums conga.OptionalFlag[[]int]    `config:"nums" name:"num"`
	}

	cli := new(JSONArrayCLI)
	app := conga.New(cli).
		RegisterDecoder(".json", json.Decode).
		ConfigOptions(conga.ConfigExplicit(cfgPath)).
		Args([]string{})

	assert.NoError(t, app.Run())
	assert.Equal(t, []string{"production", "web-tier", "frontend"}, cli.Tags.Get())
	assert.Equal(t, conga.SourceConfig, cli.Tags.Source())
	assert.Equal(t, []int{10, 20, 30}, cli.Nums.Get())
}

func TestSchemaFlagCollisions_Issue05(t *testing.T) {
	t.Parallel()

	t.Run("duplicate long flag name", func(t *testing.T) {
		t.Parallel()

		type DuplicateLongCLI struct {
			Port    conga.OptionalFlag[int] `name:"port"`
			AltPort conga.OptionalFlag[int] `name:"port"`
		}

		var cli DuplicateLongCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct DuplicateLongCLI field AltPort: duplicate flag --port", err.Error())
		assert.Equal(t, "already declared by field Port", errlines.Note(err))
	})

	t.Run("duplicate short flag rune", func(t *testing.T) {
		t.Parallel()

		type DuplicateShortCLI struct {
			Port    conga.OptionalFlag[int]    `short:"p"`
			Profile conga.OptionalFlag[string] `short:"p"`
		}

		var cli DuplicateShortCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct DuplicateShortCLI field Profile: duplicate short flag -p", err.Error())
		assert.Equal(t, "already declared by field Port", errlines.Note(err))
	})

	t.Run("duplicate flag alias", func(t *testing.T) {
		t.Parallel()

		type DuplicateAliasCLI struct {
			Verbose conga.OptionalFlag[bool] `aliases:"v,debug" name:"verbose"`
			Trace   conga.OptionalFlag[bool] `aliases:"debug"   name:"trace"`
		}

		var cli DuplicateAliasCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct DuplicateAliasCLI field Trace: duplicate flag alias --debug", err.Error())
		assert.Equal(t, "already declared by field Verbose", errlines.Note(err))
	})

	t.Run("duplicate subcommand name and alias", func(t *testing.T) {
		t.Parallel()

		type DummySubcommand struct{}

		type DuplicateSubCLI struct {
			Subcommand1 conga.Command[DummySubcommand] `name:"deploy"`
			Subcommand2 conga.Command[DummySubcommand] `name:"deploy"`
		}

		var cli DuplicateSubCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, `struct DuplicateSubCLI field Subcommand2: duplicate subcommand "deploy"`, err.Error())
		assert.Equal(t, `already declared by field Subcommand1`, errlines.Note(err))
	})

	t.Run("sibling subcommands flag sharing is allowed", func(t *testing.T) {
		t.Parallel()

		type Leaf1 struct {
			Port conga.OptionalFlag[int] `name:"port" short:"p"`
		}

		type Leaf2 struct {
			Port conga.OptionalFlag[int] `name:"port" short:"p"`
		}

		type SiblingCLI struct {
			Cmd1 conga.Command[Leaf1] `name:"cmd1"`
			Cmd2 conga.Command[Leaf2] `name:"cmd2"`
		}

		var cli SiblingCLI

		cmd, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.NoError(t, err)
		assert.Equal(t, 2, len(cmd.Subcommands))
	})

	t.Run("depth-2 sibling subcommands flag sharing is allowed", func(t *testing.T) {
		t.Parallel()

		type Leaf1 struct {
			Port conga.OptionalFlag[int] `name:"port" short:"p"`
		}

		type Leaf2 struct {
			Port conga.OptionalFlag[int] `name:"port" short:"p"`
		}

		type Mid1 struct {
			Leaf conga.Command[Leaf1] `name:"leaf"`
		}

		type Mid2 struct {
			Leaf conga.Command[Leaf2] `name:"leaf"`
		}

		type Depth2SiblingCLI struct {
			Mid1 conga.Command[Mid1] `name:"mid1"`
			Mid2 conga.Command[Mid2] `name:"mid2"`
		}

		var cli Depth2SiblingCLI

		cmd, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.NoError(t, err)
		assert.Equal(t, 2, len(cmd.Subcommands))
	})

	t.Run("depth-2 ancestor flag shadowing is rejected with correct message", func(t *testing.T) {
		t.Parallel()

		type ChildCmd struct {
			Verbose conga.OptionalFlag[bool] `name:"verbose"`
		}

		type MidCmd struct {
			Child conga.Command[ChildCmd] `name:"child"`
		}

		type Depth2ShadowCLI struct {
			Verbose conga.OptionalFlag[bool] `name:"verbose"`
			Mid     conga.Command[MidCmd]    `name:"mid"`
		}

		var cli Depth2ShadowCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, `struct ChildCmd field Verbose: duplicate flag --verbose`, err.Error())
		assert.Equal(t, `already declared by struct Depth2ShadowCLI field Verbose on an ancestor command`, errlines.Note(err))
	})

	t.Run("subcommand flag shadowing ancestor is rejected", func(t *testing.T) {
		t.Parallel()

		type ServerCmd struct {
			Port conga.RequiredFlag[int] `help:"Server port"`
		}

		type ShadowCLI struct {
			Port   conga.OptionalFlag[int]  `default:"8080"   help:"Root port" short:"p"`
			Server conga.Command[ServerCmd] `summary:"Server"`
		}

		var cli ShadowCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, `struct ServerCmd field Port: duplicate flag --port`, err.Error())
		assert.Equal(t, `already declared by struct ShadowCLI field Port on an ancestor command`, errlines.Note(err))
	})

	t.Run("subcommand short flag shadowing ancestor is rejected", func(t *testing.T) {
		t.Parallel()

		type ServerCmd struct {
			Verbose conga.OptionalFlag[bool] `help:"Server verbose" short:"v"`
		}

		type ShadowShortCLI struct {
			Verbose conga.OptionalFlag[bool] `help:"Root verbose" short:"v"`
			Server  conga.Command[ServerCmd] `summary:"Server"`
		}

		var cli ShadowShortCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, `struct ServerCmd field Verbose: duplicate flag --verbose`, err.Error())
		assert.Equal(t, `already declared by struct ShadowShortCLI field Verbose on an ancestor command`, errlines.Note(err))
	})

	t.Run("parent flag and subcommand with same name coexists", func(t *testing.T) {
		t.Parallel()

		type ConfigSubcommand struct {
			noopRunner

			Key conga.RequiredArg[string] `name:"<key>"`
		}

		type CoexistCLI struct {
			ConfigFile conga.OptionalFlag[string]      `name:"config" short:"c"`
			Config     conga.Command[ConfigSubcommand] `name:"config"`
		}

		cli := new(CoexistCLI)
		app := conga.New(cli).Args([]string{"--config", "app.yaml", "config", "mykey"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "app.yaml", cli.ConfigFile.Get())
		assert.True(t, cli.Config.IsSelected())
		assert.Equal(t, "mykey", cli.Config.Get().Key.Get())
	})

	t.Run("flag argument named same as subcommand does not steal subcommand", func(t *testing.T) {
		t.Parallel()

		type RunSubcommand struct {
			noopRunner

			Task conga.RequiredArg[string] `name:"<task>"`
		}

		type DisambiguateCLI struct {
			Config conga.OptionalFlag[string]   `name:"config" short:"c"`
			Run    conga.Command[RunSubcommand] `name:"run"`
		}

		cli := new(DisambiguateCLI)
		app := conga.New(cli).Args([]string{"--config", "run", "run", "mytask"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "run", cli.Config.Get())
		assert.True(t, cli.Run.IsSelected())
		assert.Equal(t, "mytask", cli.Run.Get().Task.Get())
	})

	t.Run("built-in help short colliding with a user short is rejected", func(t *testing.T) {
		t.Parallel()

		type BuiltinHelpShortCLI struct {
			noopRunner

			Host conga.OptionalFlag[string] `help:"Host" short:"h"`
		}

		var cli BuiltinHelpShortCLI

		app := conga.New(&cli).Name("app").HelpShortFlag('h').Args([]string{})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct BuiltinHelpShortCLI field Host: built-in --help short flag -h collides with --host`, err.Error())
		assert.Equal(t, `assign a different rune to one of them`, errlines.Hint(err))
	})

	t.Run("built-in version short colliding with a user short is rejected", func(t *testing.T) {
		t.Parallel()

		type BuiltinVersionShortCLI struct {
			noopRunner

			Verbose conga.OptionalFlag[bool] `help:"Verbose logging" short:"V"`
		}

		var cli BuiltinVersionShortCLI

		app := conga.New(&cli).Name("app").VersionShortFlag('V').Args([]string{})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct BuiltinVersionShortCLI field Verbose: built-in --version short flag -V collides with --verbose`, err.Error())
		assert.Equal(t, `assign a different rune to one of them`, errlines.Hint(err))
	})

	t.Run("built-in help and version shorts colliding with each other is rejected", func(t *testing.T) {
		t.Parallel()

		type BuiltinShortPairCLI struct {
			noopRunner
		}

		var cli BuiltinShortPairCLI

		app := conga.New(&cli).Name("app").HelpShortFlag('h').VersionShortFlag('h').Args([]string{})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, "built-in --help and --version both claim short flag -h", err.Error())
		assert.Equal(t, "assign a different rune to one of them", errlines.Hint(err))
	})

	t.Run("built-in help short colliding on a subcommand is rejected", func(t *testing.T) {
		t.Parallel()

		type DeployCmd struct {
			noopRunner

			Host conga.OptionalFlag[string] `help:"Host" short:"h"`
		}

		type SubcommandShortCLI struct {
			Deploy conga.Command[DeployCmd] `summary:"Deploy"`
		}

		var cli SubcommandShortCLI

		app := conga.New(&cli).Name("app").HelpShortFlag('h').Args([]string{})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.Equal(t, `struct DeployCmd field Host: built-in --help short flag -h collides with --host`, err.Error())
		assert.Equal(t, `assign a different rune to one of them`, errlines.Hint(err))
	})

	t.Run("built-in short rune is free when the built-in is disabled", func(t *testing.T) {
		t.Parallel()

		type DisabledHelpShortCLI struct {
			noopRunner

			Host conga.OptionalFlag[string] `help:"Host" short:"h"`
		}

		var cli DisabledHelpShortCLI

		app := conga.New(&cli).Name("app").DisableHelpFlag().HelpShortFlag('h').Args([]string{})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})
}

func TestNegativeNumbersAndNumericFlags_Issue01(t *testing.T) {
	t.Parallel()

	t.Run("positional negative integer and float", func(t *testing.T) {
		t.Parallel()

		type CalcCLI struct {
			noopRunner

			IntValue   conga.RequiredArg[int]     `name:"int-val"`
			FloatValue conga.RequiredArg[float64] `name:"float-val"`
		}

		cli := new(CalcCLI)
		app := conga.New(cli).Args([]string{"-5", "-3.14"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, -5, cli.IntValue.Get())
		assert.Equal(t, -3.14, cli.FloatValue.Get())
	})

	t.Run("positional slice of negative numbers", func(t *testing.T) {
		t.Parallel()

		type SliceCLI struct {
			noopRunner

			Numbers conga.RequiredArg[[]int] `name:"numbers"`
		}

		cli := new(SliceCLI)
		app := conga.New(cli).Args([]string{"-5", "-10", "-15"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, []int{-5, -10, -15}, cli.Numbers.Get())
	})

	t.Run("flag negative value with long flag separate argument", func(t *testing.T) {
		t.Parallel()

		type OffsetCLI struct {
			noopRunner

			Offset  conga.OptionalFlag[int]     `name:"offset"  short:"o"`
			Scale   conga.OptionalFlag[float64] `name:"scale"   short:"s"`
			Verbose conga.OptionalFlag[bool]    `name:"verbose" short:"v"`
		}

		cli := new(OffsetCLI)
		app := conga.New(cli).Args([]string{"--offset", "-10", "--scale", "-0.5"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, -10, cli.Offset.Get())
		assert.Equal(t, -0.5, cli.Scale.Get())
	})

	t.Run("flag negative value with short flag separate argument", func(t *testing.T) {
		t.Parallel()

		type OffsetCLI struct {
			noopRunner

			Offset  conga.OptionalFlag[int]     `name:"offset"  short:"o"`
			Scale   conga.OptionalFlag[float64] `name:"scale"   short:"s"`
			Verbose conga.OptionalFlag[bool]    `name:"verbose" short:"v"`
		}

		cli := new(OffsetCLI)
		app := conga.New(cli).Args([]string{"-o", "-25", "-s", "-1.25", "-v"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, -25, cli.Offset.Get())
		assert.Equal(t, -1.25, cli.Scale.Get())
		assert.True(t, cli.Verbose.Get())
	})

	t.Run("flag negative value with attached equals syntax", func(t *testing.T) {
		t.Parallel()

		type OffsetCLI struct {
			noopRunner

			Offset  conga.OptionalFlag[int]     `name:"offset"  short:"o"`
			Scale   conga.OptionalFlag[float64] `name:"scale"   short:"s"`
			Verbose conga.OptionalFlag[bool]    `name:"verbose" short:"v"`
		}

		cli := new(OffsetCLI)
		app := conga.New(cli).Args([]string{"--offset=-99", "-s=-4.5"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, -99, cli.Offset.Get())
		assert.Equal(t, -4.5, cli.Scale.Get())
	})

	t.Run("explicit numeric short flag rune is recognized as flag", func(t *testing.T) {
		t.Parallel()

		type NetCLI struct {
			noopRunner

			IPv4 conga.OptionalFlag[bool] `name:"ipv4" short:"4"`
			IPv6 conga.OptionalFlag[bool] `name:"ipv6" short:"6"`
			Port conga.OptionalFlag[int]  `name:"port" short:"p"`
		}

		cli := new(NetCLI)
		app := conga.New(cli).Args([]string{"-6", "-p", "8080"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.True(t, cli.IPv6.Get())
		assert.False(t, cli.IPv4.Get())
		assert.Equal(t, 8080, cli.Port.Get())
	})

	t.Run("long flag requires argument error when followed by non-numeric flag", func(t *testing.T) {
		t.Parallel()

		type TestCLI struct {
			noopRunner

			Offset  conga.OptionalFlag[int]  `name:"offset"  short:"o"`
			Verbose conga.OptionalFlag[bool] `name:"verbose" short:"v"`
		}

		app := conga.New[TestCLI]().Args([]string{"--offset", "-v"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
	})

	t.Run("short flag requires argument error when followed by non-numeric flag", func(t *testing.T) {
		t.Parallel()

		type TestCLI struct {
			noopRunner

			Offset  conga.OptionalFlag[int]  `name:"offset"  short:"o"`
			Verbose conga.OptionalFlag[bool] `name:"verbose" short:"v"`
		}

		app := conga.New[TestCLI]().Args([]string{"-o", "-v"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
	})

	t.Run("unexpected negative number when no positional args declared", func(t *testing.T) {
		t.Parallel()

		type NoPosCLI struct {
			noopRunner

			Verbose conga.OptionalFlag[bool] `name:"verbose" short:"v"`
		}

		app := conga.New[NoPosCLI]().Args([]string{"-5"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assertUsageError(t, err)
	})

	t.Run("scientific notation and large negative numbers", func(t *testing.T) {
		t.Parallel()

		type AdvancedCalcCLI struct {
			noopRunner

			ExpectedValue conga.RequiredArg[float64] `name:"exp-val"`
			IntValue      conga.RequiredArg[int]     `name:"int-val"`
		}

		cli := new(AdvancedCalcCLI)
		app := conga.New(cli).Args([]string{"-1e-3", "-99999"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, -0.001, cli.ExpectedValue.Get())
		assert.Equal(t, -99999, cli.IntValue.Get())
	})
}

func TestInvalidArgOrder_Issue08(t *testing.T) {
	t.Parallel()

	t.Run("slice arg followed by required scalar arg fails schema", func(t *testing.T) {
		t.Parallel()

		type SliceBeforeScalarCLI struct {
			Files  conga.RequiredArg[[]string] `name:"<files...>"`
			Output conga.RequiredArg[string]   `name:"<output>"`
		}

		var cli SliceBeforeScalarCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct SliceBeforeScalarCLI field Output: slice argument <files...> must be the last positional argument", err.Error())
	})

	t.Run("slice arg followed by optional scalar arg fails schema", func(t *testing.T) {
		t.Parallel()

		type SliceBeforeOptionalCLI struct {
			Files  conga.RequiredArg[[]string] `name:"<files...>"`
			Target conga.OptionalArg[string]   `name:"[target]"`
		}

		var cli SliceBeforeOptionalCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct SliceBeforeOptionalCLI field Target: slice argument <files...> must be the last positional argument", err.Error())
	})

	t.Run("multiple slice positional args fail schema", func(t *testing.T) {
		t.Parallel()

		type MultipleSlicesCLI struct {
			Files conga.RequiredArg[[]string] `name:"<files...>"`
			Ports conga.RequiredArg[[]int]    `name:"<ports...>"`
		}

		var cli MultipleSlicesCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct MultipleSlicesCLI field Ports: slice argument <files...> must be the last positional argument", err.Error())
	})

	t.Run("optional slice followed by required scalar fails schema", func(t *testing.T) {
		t.Parallel()

		type OptionalSliceBeforeScalarCLI struct {
			Items conga.OptionalArg[[]string] `name:"[items...]"`
			Dest  conga.RequiredArg[string]   `name:"<dest>"`
		}

		var cli OptionalSliceBeforeScalarCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct OptionalSliceBeforeScalarCLI field Dest: slice argument [items...] must be the last positional argument", err.Error())
	})

	t.Run("interspersed flag between slice and scalar positional arg fails schema", func(t *testing.T) {
		t.Parallel()

		type InterspersedCLI struct {
			Files   conga.RequiredArg[[]string] `name:"<files...>"`
			Verbose conga.OptionalFlag[bool]    `short:"v"`
			Dest    conga.RequiredArg[string]   `name:"<dest>"`
		}

		var cli InterspersedCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct InterspersedCLI field Dest: slice argument <files...> must be the last positional argument", err.Error())
	})

	t.Run("valid ordering with scalar args followed by trailing slice succeeds", func(t *testing.T) {
		t.Parallel()

		type ValidArgsCLI struct {
			noopRunner

			Env     conga.RequiredArg[string]   `name:"<env>"`
			Config  conga.OptionalArg[string]   `name:"[config]"`
			Sources conga.OptionalArg[[]string] `name:"<sources...>"`
			Verbose conga.OptionalFlag[bool]    `short:"v"`
		}

		cli := new(ValidArgsCLI)
		app := conga.New(cli).
			Name("deploy").
			Args([]string{"production", "config.yaml", "src1.go", "src2.go", "src3.go", "-v"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "production", cli.Env.Get())
		assert.Equal(t, "config.yaml", cli.Config.Get())
		assert.Equal(t, []string{"src1.go", "src2.go", "src3.go"}, cli.Sources.Get())
		assert.True(t, cli.Verbose.Get())
	})

	t.Run("subcommand invalid arg order fails isolated in subcommand", func(t *testing.T) {
		t.Parallel()

		type InvalidSubcommand struct {
			Files  conga.RequiredArg[[]string] `name:"<files...>"`
			Target conga.RequiredArg[string]   `name:"<target>"`
		}

		type ParentCLI struct {
			Subcommand conga.Command[InvalidSubcommand] `name:"sub"`
		}

		var cli ParentCLI

		_, err := spec.Build(&cli, "parent", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct InvalidSubcommand field Target: slice argument <files...> must be the last positional argument", err.Error())
	})

	t.Run("optional arg without default preceding required arg fails schema", func(t *testing.T) {
		t.Parallel()

		type OptionalBeforeRequiredCLI struct {
			Opt conga.OptionalArg[string] `name:"[opt]"`
			Req conga.RequiredArg[string] `name:"<req>"`
		}

		var cli OptionalBeforeRequiredCLI

		_, err := spec.Build(&cli, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.Equal(t, "struct OptionalBeforeRequiredCLI field Req: optional positional argument cannot precede required positional argument <req>", err.Error())
	})
}

func TestErrorCommandAndDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("FriendlyTypeName mappings", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "type", conga.FriendlyTypeName(nil))
		assert.Equal(t, "duration", conga.FriendlyTypeName(reflect.TypeFor[time.Duration]()))
		assert.Equal(t, "URL", conga.FriendlyTypeName(reflect.TypeFor[*url.URL]()))
		assert.Equal(t, "URL", conga.FriendlyTypeName(reflect.TypeFor[url.URL]()))
		assert.Equal(t, "boolean", conga.FriendlyTypeName(reflect.TypeFor[bool]()))
		assert.Equal(t, "integer", conga.FriendlyTypeName(reflect.TypeFor[int]()))
		assert.Equal(t, "integer", conga.FriendlyTypeName(reflect.TypeFor[int64]()))
		assert.Equal(t, "unsigned integer", conga.FriendlyTypeName(reflect.TypeFor[uint]()))
		assert.Equal(t, "unsigned integer", conga.FriendlyTypeName(reflect.TypeFor[uint32]()))
		assert.Equal(t, "float", conga.FriendlyTypeName(reflect.TypeFor[float64]()))
		assert.Equal(t, "string", conga.FriendlyTypeName(reflect.TypeFor[string]()))
		assert.Equal(t, "struct {}", conga.FriendlyTypeName(reflect.TypeFor[struct{}]()))
	})

	t.Run("TruncateForDisplay", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "short", conga.TruncateForDisplay("short"))
		assert.Equal(t, strings.Repeat("a", 50)+"…", conga.TruncateForDisplay(strings.Repeat("a", 60)))
	})
}

func TestErrFileSizeLimitExceeded(t *testing.T) {
	t.Parallel()

	t.Run("dotenv exceeding size limit is not a usage error", func(t *testing.T) {
		t.Parallel()

		envFile := filepath.Join(t.TempDir(), "large.env")
		file, err := os.Create(envFile)
		assert.NoError(t, err)

		err = file.Truncate(16*1024*1024 + 1)
		assert.NoError(t, err)
		assert.NoError(t, file.Close())

		type CLI struct {
			noopRunner
		}

		app := conga.New[CLI]().Dotenv(envFile).Args([]string{})
		err = app.RunContext(t.Context())

		assert.Error(t, err)
		assertNotUsageError(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum limit")
	})
}

func TestErrorCommandAndChain_EndToEnd(t *testing.T) {
	t.Parallel()

	type leafCLI struct {
		noopRunner

		Port conga.OptionalFlag[int] `name:"port" range:"1..10"`
	}

	type nestedCLI struct {
		noopRunner

		Sub conga.Command[leafCLI] `name:"sub"`
	}

	t.Run("--why renders the report for the invocation path", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[nestedCLI]().
			Name("app").
			Stdout(&buffer).
			EnableWhy().
			Args([]string{"sub", "--why"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), `Value provenance for "app sub":`)
	})

	t.Run("--completion zsh renders the zsh script", func(t *testing.T) {
		t.Parallel()

		type flatCLI struct {
			noopRunner
		}

		var buffer bytes.Buffer

		app := conga.New[flatCLI]().
			EnableCompletion().
			Name("app").
			Stdout(&buffer).
			Args([]string{"--completion", "zsh"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "#compdef app")
	})

	t.Run("ValidationError satisfies UsageError with the invocation path", func(t *testing.T) {
		t.Parallel()

		app := conga.New[nestedCLI]().
			Name("app").
			Args([]string{"sub", "--port", "99"})

		err := app.Run()

		assert.Error(t, err)
		assertUsageError(t, err)

		usage := assertUsageError(t, err)
		assert.Equal(t, "app sub", usage.CommandPath())
		assert.Contains(t, usage.Usage(), "app sub [flags]")
	})

	t.Run("parse error exposes Usage syntax", func(t *testing.T) {
		t.Parallel()

		app := conga.New[nestedCLI]().
			Name("app").
			Args([]string{"sub", "--unknown-flag"})

		err := app.Run()

		assert.Error(t, err)

		usage := assertUsageError(t, err)
		assert.Contains(t, usage.Usage(), "Usage:\n  app sub [flags]")
	})

	t.Run("coercion error exposes Usage syntax", func(t *testing.T) {
		t.Parallel()

		app := conga.New[nestedCLI]().
			Name("app").
			Args([]string{"sub", "--port", "invalid-number"})

		err := app.Run()

		assert.Error(t, err)

		usage := assertUsageError(t, err)
		assert.Contains(t, usage.Usage(), "Usage:\n  app sub [flags]")
	})

	t.Run("validation error exposes Usage syntax", func(t *testing.T) {
		t.Parallel()

		app := conga.New[nestedCLI]().
			Name("app").
			Args([]string{"sub", "--port", "99"})

		err := app.Run()

		assert.Error(t, err)

		usage := assertUsageError(t, err)
		assert.Contains(t, usage.Usage(), "Usage:\n  app sub [flags]")
	})

	t.Run("completion failure is an ExitCoder and not a usage error", func(t *testing.T) {
		t.Parallel()

		booming := predict.Callback(func(_ predict.Context) predict.Predictor {
			panic("completion boom")
		})

		type panicCLI struct {
			noopRunner

			Kind conga.OptionalFlag[string] `predictor:"boom"`
		}

		app := conga.New[panicCLI]().
			EnableCompletion().
			Name("app").
			Predictor("boom", booming).
			Args([]string{"__complete", "bash", "--kind", ""})

		err := app.Run()

		assert.Error(t, err)

		// The internal completion failure is an ExitCoder reporting 2, not a usage error.
		coder, isCoder := errors.AsType[conga.ExitCoder](err)

		assert.True(t, isCoder)
		assert.Equal(t, 2, coder.ExitCode())
		assert.Equal(t, "internal completion failure: completion boom", err.Error())
		assertNotUsageError(t, err)
	})
}

type failingCLI struct{}

func (f *failingCLI) Run(ctx context.Context, session *conga.Session) error {
	return fs.ErrPermission
}

type anonymousLeafCLI struct {
	Inline conga.Command[struct {
		Verbose conga.OptionalFlag[bool] `help:"Verbose output"`
	}] `name:"inline"`
}

// TestErrNoRunner_AnonymousStruct pins the owner of an anonymous command
// struct, which has no name of its own: the message names it by the path of
// the field that declares it.
func TestErrNoRunner_AnonymousStruct(t *testing.T) {
	t.Parallel()

	app := conga.New[anonymousLeafCLI]().
		Args([]string{"inline"})

	err := app.Run()

	assert.Error(t, err)
	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct anonymousLeafCLI.Inline: runner not implemented`, err.Error())
	assert.Equal(t, `implement conga.Runner: Run(context.Context, *conga.Session) error`, errlines.Hint(err))
}

type anonymousTagCLI struct {
	Deploy conga.Command[struct {
		failingCLI

		Port conga.OptionalFlag[int] `hlep:"Port"`
	}]
}

// TestSchemaError_AnonymousSubcommandOwner pins the owner of a field on an
// anonymous subcommand struct: the struct is named by its declaring field.
func TestSchemaError_AnonymousSubcommandOwner(t *testing.T) {
	t.Parallel()

	app := conga.New[anonymousTagCLI]().
		Args([]string{"deploy"})

	err := app.Run()

	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct anonymousTagCLI.Deploy field Port: unknown struct tag "hlep"`, err.Error())
	assert.Equal(t, `did you mean "help"? Use app.AllowTags("hlep") to permit external tags`, errlines.Hint(err))
}

type nestedAnonymousCLI struct {
	Deploy conga.Command[struct {
		Push conga.Command[struct {
			failingCLI

			Port conga.OptionalFlag[int] `hlep:"Port"`
		}]
	}]
}

// TestSchemaError_NestedAnonymousSubcommandOwner pins the owner path through
// two anonymous command structs.
func TestSchemaError_NestedAnonymousSubcommandOwner(t *testing.T) {
	t.Parallel()

	app := conga.New[nestedAnonymousCLI]().
		Args([]string{"deploy", "push"})

	err := app.Run()

	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct nestedAnonymousCLI.Deploy.Push field Port: unknown struct tag "hlep"`, err.Error())
	assert.Equal(t, `did you mean "help"? Use app.AllowTags("hlep") to permit external tags`, errlines.Hint(err))
}

// anonymousRootCLI is an alias, so the root struct type has no name.
type anonymousRootCLI = struct {
	Port conga.OptionalFlag[int] `hlep:"Port"`
}

// TestSchemaError_AnonymousRootOwner pins the owner of a field on an anonymous
// root command struct.
func TestSchemaError_AnonymousRootOwner(t *testing.T) {
	t.Parallel()

	app := conga.New[anonymousRootCLI]()

	err := app.Run()

	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, `struct (root) field Port: unknown struct tag "hlep"`, err.Error())
	assert.Equal(t, `did you mean "help"? Use app.AllowTags("hlep") to permit external tags`, errlines.Hint(err))
}

type anonymousShadowCLI struct {
	Verbose conga.OptionalFlag[bool] `short:"v"`
	Deploy  conga.Command[struct {
		failingCLI

		Loud conga.OptionalFlag[bool] `short:"v"`
	}]
}

// TestSchemaError_AnonymousShadowOwner pins both owners of a short rune that an
// anonymous subcommand struct redeclares from its parent.
func TestSchemaError_AnonymousShadowOwner(t *testing.T) {
	t.Parallel()

	app := conga.New[anonymousShadowCLI]().
		Args([]string{"deploy"})

	err := app.Run()

	assert.True(t, spec.IsSchemaError(err))
	assert.Equal(t, "struct anonymousShadowCLI.Deploy field Loud: duplicate short flag -v", err.Error())
	assert.Equal(t, "already declared by struct anonymousShadowCLI field Verbose on an ancestor command", errlines.Note(err))
}
