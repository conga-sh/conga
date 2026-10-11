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
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
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

func TestErrorCategories(t *testing.T) {
	t.Parallel()

	t.Run("schema", func(t *testing.T) {
		t.Parallel()

		_, err := spec.Build(42, "app", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
	})

	t.Run("parse unknown flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[XorDefaultCLI]().Name("app").Args([]string{"--bogus"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
		assert.Equal(t, "unknown flag: --bogus", err.Error())
	})

	t.Run("parse flag requires value", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().Name("server").Args([]string{"--api-key", "k", "--port"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
		assert.Equal(t, "flag requires an argument: --port", err.Error())
	})

	t.Run("validation mutually exclusive", func(t *testing.T) {
		t.Parallel()

		app := conga.New[XorDefaultCLI]().Name("app").Args([]string{"--immediate", "--graceful"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("validation missing required flag", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().Name("server").Args([]string{"file.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, "missing required flag: --api-key", err.Error())
	})

	t.Run("validation invalid enum value", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().Name("server").Args([]string{"--api-key", "k", "--format", "invalid", "file.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("coerce invalid value", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().Name("server").Args([]string{"--api-key", "k", "-p", "not-a-number", "file.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, `invalid integer value "not-a-number" for flag --port`, err.Error())
	})

	t.Run("config read failure", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FullServerCLI]().
			ConfigOptions(conga.ConfigExplicit(filepath.Join(t.TempDir(), "missing.json"))).
			Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrConfig)
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

	filePath := filepath.Join(tmpDir, "file.txt")
	err := os.WriteFile(filePath, []byte("x"), 0o600)
	assert.NoError(t, err)

	var file conga.File

	decodeContext := conga.DecodeContext{Tags: conga.FieldTags{"existing": "true"}}

	err = file.UnmarshalTextWithContext([]byte(tmpDir), decodeContext)
	assert.NoError(t, err)

	err = file.Validate(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, fmt.Sprintf("%q is a directory, but a file was expected", tmpDir), err.Error())
}

func TestValidationErrorPathKindRender(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	app := conga.New[PathKindCLI]().Name("app").Args([]string{"--file", tmpDir})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
	assert.Equal(t, fmt.Sprintf("%q is a directory, but a file was expected for flag --file", tmpDir), err.Error())
}

func TestValidationErrorExecutableRender(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("execute-bit check is not enforced on Windows")
	}

	tmpDir := t.TempDir()
	progPath := filepath.Join(tmpDir, "prog")
	assert.NoError(t, os.WriteFile(progPath, []byte("#!/bin/sh\n"), 0o644))

	app := conga.New[ExecutableCLI]().Name("app").Args([]string{"--prog", progPath})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
	assert.Equal(t, fmt.Sprintf("%q is not executable for flag --prog", progPath), err.Error())
}

func TestValidationErrorSymlinkRender(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, "hook.sh")
	assert.NoError(t, os.WriteFile(hookPath, []byte("#!/bin/sh\n"), 0o755))

	app := conga.New[SymlinkCLI]().Name("app").Args([]string{"--hook", hookPath})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrValidation)
	assert.True(t, conga.IsUsageError(err))
	assert.Equal(t, fmt.Sprintf("%q is not a symbolic link for flag --hook", hookPath), err.Error())
}

func TestSchemaErrorFileTagExclusionExistingExecutable(t *testing.T) {
	t.Parallel()

	app := conga.New[FileTagExclusionCLI]().Name("app")
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, `struct FileTagExclusionCLI field Input: schema error: invalid struct tag: tag "existing" cannot be combined with tag "executable"`, err.Error())
}

func TestSchemaErrorFileTagExclusionSymlinkStdout(t *testing.T) {
	t.Parallel()

	app := conga.New[FileStdioExclusionCLI]().Name("app")
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, `struct FileStdioExclusionCLI field Output: schema error: invalid struct tag: tag "stdout" cannot be combined with tag "symlink"`, err.Error())
}

func TestCoercionErrorCounterRender(t *testing.T) {
	t.Parallel()

	app := conga.New[CounterErrorCLI]().Name("app").Args([]string{"--count=abc"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, `invalid integer value "abc" for flag --count`, err.Error())
}

func TestCoercionErrorUnsupportedTypeRender(t *testing.T) {
	t.Parallel()

	app := conga.New[UnsupportedTypeCLI]().Name("app").Args([]string{"--ch", "x"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, "unsupported type chan int for flag --ch", err.Error())
}

func TestCoercionErrorByteSizeRender(t *testing.T) {
	t.Parallel()

	app := conga.New[ByteSizeErrorCLI]().Name("app").Args([]string{"--max-memory", "10ZB"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, `unknown byte size unit "ZB" in "10ZB" for flag --max-memory`, err.Error())
}

func TestCoercionErrorByteSizeAmbiguousUnitRender(t *testing.T) {
	t.Parallel()

	app := conga.New[ByteSizeErrorCLI]().Name("app").Args([]string{"--max-memory", "10KB"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, `ambiguous byte size unit "KB" in "10KB": use kB (1000) or KiB (1024) for flag --max-memory`, err.Error())
}

func TestHookErrorsPassThrough(t *testing.T) {
	t.Parallel()

	app := conga.New[failingCLI]().Name("app").Args([]string{})
	err := app.RunContext(t.Context())
	assert.IsError(t, err, fs.ErrPermission)
	assert.Equal(t, fs.ErrPermission.Error(), err.Error())
	assert.False(t, conga.IsUsageError(err))

	for _, sentinel := range []error{conga.ErrSchema, conga.ErrParse, conga.ErrCoerce, conga.ErrValidation, conga.ErrConfig} {
		assert.False(t, errors.Is(err, sentinel))
	}
}

func TestDomainTypeErrorsSelfCategorize(t *testing.T) {
	t.Parallel()

	var size conga.ByteSize

	err := size.UnmarshalText([]byte("10ZB"))
	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.Equal(t, `unknown byte size unit "ZB" in "10ZB"`, err.Error())
}

func TestIsUsageError(t *testing.T) {
	t.Parallel()

	assert.False(t, conga.IsUsageError(nil))

	assert.False(t, conga.IsUsageError(conga.ErrSchema))

	err := errors.New("database connection refused")
	assert.False(t, conga.IsUsageError(err))
}

func TestIsUsageError_EndToEnd(t *testing.T) {
	t.Parallel()

	t.Run("unknown flag parses as usage error", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner
		}

		app := conga.New[CLI]().Name("app").Args([]string{"--bogus"})

		err := app.Run()

		assert.Error(t, err)
		assert.True(t, conga.IsUsageError(err))
		assert.IsError(t, err, conga.ErrParse)
	})

	t.Run("coercion failure classifies as usage error", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Port conga.OptionalFlag[int] `name:"port"`
		}

		app := conga.New[CLI]().Name("app").Args([]string{"--port", "abc"})

		err := app.Run()

		assert.Error(t, err)
		assert.True(t, conga.IsUsageError(err))
		assert.IsError(t, err, conga.ErrCoerce)
	})

	t.Run("missing required flag classifies as usage error", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			noopRunner

			Token conga.RequiredFlag[string] `name:"token"`
		}

		app := conga.New[CLI]().Name("app").Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.True(t, conga.IsUsageError(err))
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestErrNoRunner(t *testing.T) {
	t.Parallel()

	t.Run("flat command without Run", func(t *testing.T) {
		t.Parallel()

		type CLI struct {
			Flag conga.OptionalFlag[string] `name:"flag"`
		}

		app := conga.New[CLI]().Name("app").Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.False(t, conga.IsUsageError(err))
		assert.Equal(t, `runner not implemented: command "app" does not implement conga.Runner (implement Run(context.Context, *conga.Session) error)`, err.Error())
	})

	t.Run("subcommand without Run", func(t *testing.T) {
		t.Parallel()

		type ChildCmd struct {
			Flag conga.OptionalFlag[string] `name:"flag"`
		}

		type ParentCLI struct {
			Subcommand conga.Command[ChildCmd] `name:"sub"`
		}

		app := conga.New[ParentCLI]().Name("app").Args([]string{"sub"})

		err := app.Run()

		assert.Error(t, err)
		assert.False(t, conga.IsUsageError(err))
		assert.Equal(t, `runner not implemented: command "sub" does not implement conga.Runner (implement Run(context.Context, *conga.Session) error)`, err.Error())
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

		app := conga.New[RootCLI]().Name("app").Args([]string{"mid", "leaf"})

		err := app.Run()

		assert.Error(t, err)
		assert.False(t, conga.IsUsageError(err))
		assert.Equal(t, `runner not implemented: command "leaf" does not implement conga.Runner (implement Run(context.Context, *conga.Session) error)`, err.Error())
	})
}

func TestTextUnmarshalerCoerceWrapping_Issue06(t *testing.T) {
	t.Parallel()

	app := conga.New[CustomTypeCLI]().Name("app").Args([]string{"--custom", "fail"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(err))
}

func TestSubcommandTypoSuggestion_Issue10(t *testing.T) {
	t.Parallel()

	app := conga.New[SubcommandParentCLI]().Name("clustermgr").Args([]string{"clustr"})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `Did you mean "cluster"?`)
}

func TestSubcommandRequired_Issue11(t *testing.T) {
	t.Parallel()

	app := conga.New[SubcommandParentCLI]().Name("clustermgr").Args([]string{})
	err := app.RunContext(t.Context())

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrParse)
	assert.Contains(t, err.Error(), `subcommand required for "clustermgr"`)
}

func TestSliceEnumValidation_Issue03(t *testing.T) {
	t.Parallel()

	t.Run("valid slice enum elements", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceEnumCLI]().Name("app").Args([]string{"--format", "json,yaml"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("invalid slice enum element", func(t *testing.T) {
		t.Parallel()

		app := conga.New[SliceEnumCLI]().Name("app").Args([]string{"--format", "json,xml"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})
}

func TestNormalizingEnumValidation(t *testing.T) {
	t.Parallel()

	t.Run("flag accepts normalized value", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NormalizingEnumCLI]().Name("app").Args([]string{"--method", "get"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("argument accepts normalized value", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NormalizingEnumCLI]().Name("app").Args([]string{"get"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("flag rejects value outside enum", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NormalizingEnumCLI]().Name("app").Args([]string{"--method", "patch"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("argument rejects value outside enum", func(t *testing.T) {
		t.Parallel()

		app := conga.New[NormalizingEnumCLI]().Name("app").Args([]string{"patch"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
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
		assertGolden(t, "testdata/errors/usage-default.golden", goldenErrorsUsageDefault, buffer.String())
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
		assertGolden(t, "testdata/errors/usage-silence.golden", goldenErrorsUsageSilence, buffer.String())
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
		assertGolden(t, "testdata/errors/usage-full.golden", goldenErrorsUsageFull, buffer.String())
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
		assert.IsError(t, err, conga.ErrParse)

		app.Exit(err)

		assert.Equal(t, 2, exitCode)
		assertGolden(t, "testdata/errors/subcommand-required.golden", goldenErrorsSubcommandRequired, buffer.String())
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
		assertGolden(t, "testdata/errors/usage-hint.golden", goldenErrorsUsageHint, buffer.String())
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
		assertGolden(t, "testdata/errors/invalid-value.golden", goldenErrorsInvalidValue, buffer.String())
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
		assert.True(t, conga.IsUsageError(err))

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
		assertGolden(t, "testdata/errors/missing-required-flag.golden", goldenErrorsMissingRequiredFlag, buffer.String())
	})
}

func TestHelpAndUsageFormatting(t *testing.T) {
	t.Parallel()

	var app SubcommandParentCLI

	cmd, err := spec.Build(&app, "clustermgr", conga.SpecOptionsWithContainers(spec.Options{}))
	assert.NoError(t, err)

	usage := help.Usage([]*spec.Command{cmd})
	assert.Contains(t, usage, "Usage:\n  clustermgr <command>")

	hint := help.Hint([]*spec.Command{cmd})
	assert.Equal(t, "Run 'clustermgr --help' for more information.", hint)

	formatted := help.Format([]*spec.Command{cmd})
	assert.Contains(t, formatted, "Commands:\n  cluster")
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

		app := conga.New[OptionTerminatorCLI]().Name("app").Args([]string{"--name", "--", "file1.txt", "file2.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
		assert.Contains(t, err.Error(), "flag requires an argument: --name")
	})

	t.Run("short value flag before double dash", func(t *testing.T) {
		t.Parallel()

		app := conga.New[OptionTerminatorCLI]().Name("app").Args([]string{"-o", "--", "file1.txt"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
		assert.Contains(t, err.Error(), "flag requires an argument: -o")
	})

	t.Run("attached value flag before double dash", func(t *testing.T) {
		t.Parallel()

		cli := new(OptionTerminatorCLI)
		app := conga.New(cli).Name("app").Args([]string{"--name=myname", "--", "file1.txt", "file2.txt"})
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
		app := conga.New(cli).Name("app").Args([]string{
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

		app := conga.New[CustomTypeCLI]().Name("app").Args([]string{"--url", "http://[invalid-host-bracket"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.True(t, conga.IsUsageError(err))
	})

	t.Run("failing pointer TextUnmarshaler fails coercion", func(t *testing.T) {
		t.Parallel()

		app := conga.New[CustomTypeCLI]().Name("app").Args([]string{"--ptr", "fail"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrCoerce)
		assert.True(t, conga.IsUsageError(err))
	})
}

func TestJSONArrayFlattening_Issue09(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfgJSON := `{"tags": ["production", "web-tier", "frontend"], "nums": [10, 20, 30]}`
	err := os.WriteFile(cfgPath, []byte(cfgJSON), 0o600)
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct DuplicateLongCLI field AltPort: schema error: duplicate flag: --port (already declared by field Port)", err.Error())
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct DuplicateShortCLI field Profile: schema error: duplicate flag: short flag -p (already declared by field Port)", err.Error())
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct DuplicateAliasCLI field Trace: schema error: duplicate flag: alias --debug (already declared by field Verbose)", err.Error())
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct DuplicateSubCLI field Subcommand2: schema error: duplicate flag: subcommand \"deploy\" (already declared by field Subcommand1)", err.Error())
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `command "child": schema error: duplicate flag: --verbose (already declared by ancestor command "app")`, err.Error())
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
		assert.IsError(t, err, conga.ErrSchema)
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
		assert.IsError(t, err, conga.ErrSchema)
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
		app := conga.New(cli).Name("app").Args([]string{"--config", "app.yaml", "config", "mykey"})
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
		app := conga.New(cli).Name("app").Args([]string{"--config", "run", "run", "mytask"})
		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "run", cli.Config.Get())
		assert.True(t, cli.Run.IsSelected())
		assert.Equal(t, "mytask", cli.Run.Get().Task.Get())
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
		app := conga.New(cli).Name("calc").Args([]string{"-5", "-3.14"})
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
		app := conga.New(cli).Name("math").Args([]string{"-5", "-10", "-15"})
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
		app := conga.New(cli).Name("app").Args([]string{"--offset", "-10", "--scale", "-0.5"})
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
		app := conga.New(cli).Name("app").Args([]string{"-o", "-25", "-s", "-1.25", "-v"})
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
		app := conga.New(cli).Name("app").Args([]string{"--offset=-99", "-s=-4.5"})
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
		app := conga.New(cli).Name("net").Args([]string{"-6", "-p", "8080"})
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

		app := conga.New[TestCLI]().Name("app").Args([]string{"--offset", "-v"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})

	t.Run("short flag requires argument error when followed by non-numeric flag", func(t *testing.T) {
		t.Parallel()

		type TestCLI struct {
			noopRunner

			Offset  conga.OptionalFlag[int]  `name:"offset"  short:"o"`
			Verbose conga.OptionalFlag[bool] `name:"verbose" short:"v"`
		}

		app := conga.New[TestCLI]().Name("app").Args([]string{"-o", "-v"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrParse)
	})

	t.Run("unexpected negative number when no positional args declared", func(t *testing.T) {
		t.Parallel()

		type NoPosCLI struct {
			noopRunner

			Verbose conga.OptionalFlag[bool] `name:"verbose" short:"v"`
		}

		app := conga.New[NoPosCLI]().Name("app").Args([]string{"-5"})
		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
	})

	t.Run("scientific notation and large negative numbers", func(t *testing.T) {
		t.Parallel()

		type AdvancedCalcCLI struct {
			noopRunner

			ExpectedValue conga.RequiredArg[float64] `name:"exp-val"`
			IntValue      conga.RequiredArg[int]     `name:"int-val"`
		}

		cli := new(AdvancedCalcCLI)
		app := conga.New(cli).Name("calc").Args([]string{"-1e-3", "-99999"})
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct SliceBeforeScalarCLI field Output: schema error: invalid positional argument order: slice argument <files...> must be the last positional argument", err.Error())
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct SliceBeforeOptionalCLI field Target: schema error: invalid positional argument order: slice argument <files...> must be the last positional argument", err.Error())
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct MultipleSlicesCLI field Ports: schema error: invalid positional argument order: slice argument <files...> must be the last positional argument", err.Error())
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct OptionalSliceBeforeScalarCLI field Dest: schema error: invalid positional argument order: slice argument [items...] must be the last positional argument", err.Error())
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
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, "struct InterspersedCLI field Dest: schema error: invalid positional argument order: slice argument <files...> must be the last positional argument", err.Error())
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
			RootArg    conga.RequiredArg[string]        `name:"<root-arg>"`
			Subcommand conga.Command[InvalidSubcommand] `name:"sub"`
		}

		var cli ParentCLI

		_, err := spec.Build(&cli, "parent", conga.SpecOptionsWithContainers(spec.Options{}))
		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
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
		assert.IsError(t, err, conga.ErrSchema)
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

	t.Run("dotenv exceeding size limit classifies as config error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		envFile := filepath.Join(dir, "large.env")
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
		assert.IsError(t, err, conga.ErrConfig)
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
		assert.IsError(t, err, conga.ErrValidation)
		assert.True(t, conga.IsUsageError(err))

		var usage conga.UsageError

		assert.True(t, errors.As(err, &usage))
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

		var usage conga.UsageError

		assert.True(t, errors.As(err, &usage))
		assert.Contains(t, usage.Usage(), "Usage:\n  app sub [flags]")
	})

	t.Run("coercion error exposes Usage syntax", func(t *testing.T) {
		t.Parallel()

		app := conga.New[nestedCLI]().
			Name("app").
			Args([]string{"sub", "--port", "invalid-number"})

		err := app.Run()

		assert.Error(t, err)

		var usage conga.UsageError

		assert.True(t, errors.As(err, &usage))
		assert.Contains(t, usage.Usage(), "Usage:\n  app sub [flags]")
	})

	t.Run("validation error exposes Usage syntax", func(t *testing.T) {
		t.Parallel()

		app := conga.New[nestedCLI]().
			Name("app").
			Args([]string{"sub", "--port", "99"})

		err := app.Run()

		assert.Error(t, err)

		var usage conga.UsageError

		assert.True(t, errors.As(err, &usage))
		assert.Contains(t, usage.Usage(), "Usage:\n  app sub [flags]")
	})

	t.Run("CompletionError classification and decoupling from UsageError", func(t *testing.T) {
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

		// Must classify as CompletionError
		assert.True(t, conga.IsCompletionError(err))

		var compErr conga.CompletionError

		assert.True(t, errors.As(err, &compErr))
		assert.True(t, compErr.IsCompletionError())

		// Must NOT classify as UsageError
		assert.False(t, conga.IsUsageError(err))

		var usageErr conga.UsageError

		assert.False(t, errors.As(err, &usageErr))
	})
}

type failingCLI struct{}

func (f *failingCLI) Run(ctx context.Context, session *conga.Session) error {
	return fs.ErrPermission
}
