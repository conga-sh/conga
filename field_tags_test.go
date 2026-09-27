package conga_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/decoder/json"
)

type FileExistingCLI struct {
	noopRunner

	Input conga.OptionalFlag[conga.File] `existing:"true" name:"input"`
}

type FileStdinCLI struct {
	noopRunner

	Input conga.OptionalFlag[conga.File] `existing:"true" name:"input" stdin:"true"`
}

type FileStdoutCLI struct {
	noopRunner

	Output conga.OptionalFlag[conga.File] `name:"output" stdout:"true"`
}

type FileStdinStdoutCLI struct {
	noopRunner

	Stream conga.OptionalFlag[conga.File] `name:"stream" stdin:"true" stdout:"true"`
}

type FileExistingStdoutCLI struct {
	noopRunner

	File conga.OptionalFlag[conga.File] `existing:"true" name:"file" stdout:"true"`
}

type FileBadTagValueCLI struct {
	noopRunner

	Input conga.OptionalFlag[conga.File] `existing:"yes" name:"input"`
}

type FileUnknownTagCLI struct {
	noopRunner

	Name conga.OptionalFlag[string] `existing:"true" name:"name"`
}

type FileSliceCLI struct {
	noopRunner

	Inputs conga.OptionalFlag[[]conga.File] `existing:"true" name:"input" sep:","`
}

type FileArgCLI struct {
	noopRunner

	Input conga.RequiredArg[conga.File] `existing:"true"`
}

type FileGlobCLI struct {
	noopRunner

	Input conga.OptionalFlag[conga.File] `glob:"Makefile,Dockerfile" name:"input"`
}

type FileGlobExecutableCLI struct {
	noopRunner

	Tool conga.OptionalFlag[conga.File] `executable:"true" glob:"*.sh" name:"tool"`
}

type FileGlobSymlinkCLI struct {
	noopRunner

	Link conga.OptionalFlag[conga.File] `glob:"*.conf" name:"link" symlink:"true"`
}

type FileGlobEmptyCLI struct {
	noopRunner

	Input conga.OptionalFlag[conga.File] `glob:"" name:"input"`
}

type FileGlobMalformedCLI struct {
	noopRunner

	Input conga.OptionalFlag[conga.File] `glob:"[" name:"input"`
}

type FileGlobPathSeparatorCLI struct {
	noopRunner

	Input conga.OptionalFlag[conga.File] `glob:"src/*.go" name:"input"`
}

type DirExistingCLI struct {
	noopRunner

	WorkDir conga.OptionalFlag[conga.Dir] `existing:"true" name:"workdir"`
}

type FileReloadCLI struct {
	Input conga.OptionalFlag[conga.File] `existing:"true" name:"input"`
}

// Run reloads the configuration through session.Reload, propagating the
// re-validation outcome of the freshly resolved values.
func (c *FileReloadCLI) Run(ctx context.Context, session *conga.Session) error {
	_, err := session.Reload[FileReloadCLI](ctx)

	return err
}

func TestFileExistingValidation(t *testing.T) {
	t.Parallel()

	t.Run("existing file passes", func(t *testing.T) {
		t.Parallel()

		testFile := writeFileForTest(t, "input.txt")
		cli := new(FileExistingCLI)
		app := conga.New(cli).Name("app").Args([]string{"--input", testFile})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)

		inputFile := cli.Input.Get()
		assert.Equal(t, testFile, inputFile.String())
	})

	t.Run("missing file fails as usage validation error", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing.txt")
		app := conga.New[FileExistingCLI]().Name("app").Args([]string{"--input", missing})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.True(t, conga.IsUsageError(err))
		assert.Contains(t, err.Error(), " for flag --input")
	})

	t.Run("help wins over validation failure", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing.txt")
		app := conga.New[FileExistingCLI]().Name("app").Args([]string{"--input", missing, "--help"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("stdin dash accepted with stdin tag", func(t *testing.T) {
		t.Parallel()

		cli := new(FileStdinCLI)
		app := conga.New(cli).Name("app").Args([]string{"--input", "-"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)

		inputFile := cli.Input.Get()
		assert.Equal(t, "-", inputFile.String())
		assert.True(t, cli.Input.Get().IsStdio())
	})

	t.Run("stdout dash accepted with stdout tag", func(t *testing.T) {
		t.Parallel()

		cli := new(FileStdoutCLI)
		app := conga.New(cli).Name("app").Args([]string{"--output", "-"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)

		outputFile := cli.Output.Get()
		assert.Equal(t, "-", outputFile.String())
	})

	t.Run("dash rejected without capability tags", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileExistingCLI]().Name("app").Args([]string{"--input", "-"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, `"-" requires the stdin or stdout tag on the field for flag --input`, err.Error())
	})

	t.Run("config value overridden by valid CLI value passes", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "config-missing.txt")
		testFile := writeFileForTest(t, "override.txt")
		cli := new(FileExistingCLI)
		app := conga.New(cli).
			Name("app").
			RegisterDecoder(".json", json.RawDecoder).
			ConfigOptions(config.Explicit(writeConfigFile(t, `{"input": "`+missing+`"}`))).
			Args([]string{"--input", testFile})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)

		inputFile := cli.Input.Get()
		assert.Equal(t, testFile, inputFile.String())
	})

	t.Run("invalid config value fails validation", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "config-missing.txt")
		app := conga.New[FileExistingCLI]().
			Name("app").
			RegisterDecoder(".json", json.RawDecoder).
			ConfigOptions(config.Explicit(writeConfigFile(t, `{"input": "`+missing+`"}`))).
			Args([]string{})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Contains(t, err.Error(), " for flag --input")
	})

	t.Run("slice elements validated with index context", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "slice-missing.txt")
		testFile := writeFileForTest(t, "slice-present.txt")
		app := conga.New[FileSliceCLI]().
			Name("app").
			Args([]string{"--input", testFile + "," + missing})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Contains(t, err.Error(), " for flag --input")
	})
}

func TestFileArgExistingValidation(t *testing.T) {
	t.Parallel()

	t.Run("existing file passes", func(t *testing.T) {
		t.Parallel()

		testFile := writeFileForTest(t, "arg.txt")
		cli := new(FileArgCLI)
		app := conga.New(cli).Name("app").Args([]string{testFile})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)

		inputFile := cli.Input.Get()
		assert.Equal(t, testFile, inputFile.String())
	})

	t.Run("missing file fails", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "missing.txt")
		app := conga.New[FileArgCLI]().Name("app").Args([]string{missing})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Contains(t, err.Error(), " for argument <input>")
	})
}

func TestDirExistingValidation(t *testing.T) {
	t.Parallel()

	t.Run("existing directory passes", func(t *testing.T) {
		t.Parallel()

		app := conga.New[DirExistingCLI]().Name("app").Args([]string{"--workdir", t.TempDir()})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
	})

	t.Run("regular file fails", func(t *testing.T) {
		t.Parallel()

		testFile := writeFileForTest(t, "not-a-dir.txt")
		app := conga.New[DirExistingCLI]().Name("app").Args([]string{"--workdir", testFile})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Contains(t, err.Error(), "is a file, but a directory was expected")
	})
}

func TestFileTagSchemaErrors(t *testing.T) {
	t.Parallel()

	t.Run("stdin and stdout are mutually exclusive", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileStdinStdoutCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct FileStdinStdoutCLI field Stream: schema error: invalid struct tag: tag "stdin" cannot be combined with tag "stdout"`, err.Error())
	})

	t.Run("existing and stdout are contradictory", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileExistingStdoutCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct FileExistingStdoutCLI field File: schema error: invalid struct tag: tag "existing" cannot be combined with tag "stdout"`, err.Error())
	})

	t.Run("invalid boolean tag value rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileBadTagValueCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct FileBadTagValueCLI field Input: schema error: invalid struct tag: tag "existing" expects "true" or "false", got "yes"`, err.Error())
	})

	t.Run("domain tags rejected on unregistered types", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileUnknownTagCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Contains(t, err.Error(), `unknown struct tag "existing"`)
	})
}

func TestFileGlobTag(t *testing.T) {
	t.Parallel()

	t.Run("non-matching value is accepted (completion-only tag)", func(t *testing.T) {
		t.Parallel()

		testFile := writeFileForTest(t, "input.txt")
		cli := new(FileGlobCLI)
		app := conga.New(cli).Name("app").Args([]string{"--input", testFile})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)

		inputFile := cli.Input.Get()
		assert.Equal(t, testFile, inputFile.String())
	})

	t.Run("help mentions the declared glob patterns", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[FileGlobCLI]().Name("app").Stdout(&buffer).Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "Path to a file matching 'Makefile,Dockerfile'")
	})
}

func TestFileGlobSchemaErrors(t *testing.T) {
	t.Parallel()

	t.Run("glob and executable are mutually exclusive", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileGlobExecutableCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct FileGlobExecutableCLI field Tool: schema error: invalid struct tag: tag "executable" cannot be combined with tag "glob"`, err.Error())
	})

	t.Run("glob and symlink are mutually exclusive", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileGlobSymlinkCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct FileGlobSymlinkCLI field Link: schema error: invalid struct tag: tag "symlink" cannot be combined with tag "glob"`, err.Error())
	})

	t.Run("empty glob value rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileGlobEmptyCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct FileGlobEmptyCLI field Input: schema error: invalid struct tag: tag "glob" cannot be empty`, err.Error())
	})

	t.Run("malformed glob pattern rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileGlobMalformedCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct FileGlobMalformedCLI field Input: schema error: invalid struct tag: glob pattern "[" is malformed: syntax error in pattern`, err.Error())
	})

	t.Run("path separator in glob pattern rejected", func(t *testing.T) {
		t.Parallel()

		app := conga.New[FileGlobPathSeparatorCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct FileGlobPathSeparatorCLI field Input: schema error: invalid struct tag: glob pattern "src/*.go" must not contain path separators (patterns match file names during completion traversal)`, err.Error())
	})
}

func TestFileExistingReloadParity(t *testing.T) {
	t.Parallel()

	t.Run("reloaded config values re-validated", func(t *testing.T) {
		t.Parallel()

		testFile := writeFileForTest(t, "reload.txt")
		app := conga.New[FileReloadCLI]().
			Name("app").
			RegisterDecoder(".json", json.RawDecoder).
			ConfigOptions(config.Explicit(writeConfigFile(t, `{"input": "`+testFile+`"}`))).
			Args([]string{})

		err := app.Run()

		assert.NoError(t, err)
	})

	t.Run("reloaded config values failing validation fail reload", func(t *testing.T) {
		t.Parallel()

		missing := filepath.Join(t.TempDir(), "reload-missing.txt")
		app := conga.New[FileReloadCLI]().
			Name("app").
			RegisterDecoder(".json", json.RawDecoder).
			ConfigOptions(config.Explicit(writeConfigFile(t, `{"input": "`+missing+`"}`))).
			Args([]string{})

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Contains(t, err.Error(), " for flag --input")
	})
}

func TestFileTagAwareHelp(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Plain   conga.OptionalFlag[conga.File] `name:"plain"`
		Strict  conga.OptionalFlag[conga.File] `existing:"true" name:"strict"`
		Stdin   conga.OptionalFlag[conga.File] `existing:"true" name:"from"    stdin:"true"`
		Stdout  conga.OptionalFlag[conga.File] `name:"to"       stdout:"true"`
		DirPath conga.OptionalFlag[conga.Dir]  `existing:"true" name:"dirpath"`
	}

	var buffer bytes.Buffer

	app := conga.New[CLI]().Name("app").Stdout(&buffer).Args([]string{"--help"})

	err := app.Run()

	assert.NoError(t, err)

	output := buffer.String()
	assert.Contains(t, output, "Path to a file")
	assert.Contains(t, output, "Path to an existing file")
	assert.Contains(t, output, "Path to an existing file or '-' for stdin")
	assert.Contains(t, output, "Path to a file or '-' for stdout")
	assert.Contains(t, output, "Path to an existing directory")
}

// TaggedToken demonstrates a user-defined domain type that registers its own
// field tags, consumes them at decode time via UnmarshalTextWithContext, and
// validates the captured state.
type TaggedToken string

func (t *TaggedToken) TagSpecs() []conga.TagSpec {
	return []conga.TagSpec{
		{Name: "prefix", Kind: conga.TagKindBool, Excludes: []string{"raw"}},
		{Name: "raw", Kind: conga.TagKindBool},
	}
}

func (t *TaggedToken) UnmarshalTextWithContext(text []byte, decodeContext conga.DecodeContext) error {
	value := string(text)
	if decodeContext.Tags.Bool("prefix") {
		value = "token:" + value
	}

	*t = TaggedToken(value)

	return nil
}

func (t *TaggedToken) Validate(ctx context.Context) error {
	if strings.HasPrefix(string(*t), "token:") {
		return nil
	}

	return errors.New("token is missing the declared prefix")
}

func (t *TaggedToken) Help(tags conga.FieldTags) string {
	if tags.Bool("prefix") {
		return "API token (prefixed automatically)"
	}

	return "API token"
}

type TaggedTokenCLI struct {
	noopRunner

	Token conga.OptionalFlag[TaggedToken] `name:"token" prefix:"true"`
}

type TaggedTokenRawCLI struct {
	noopRunner

	Token conga.OptionalFlag[TaggedToken] `name:"token"`
}

type TaggedTokenExcludedCLI struct {
	noopRunner

	Token conga.OptionalFlag[TaggedToken] `name:"token" prefix:"true" raw:"true"`
}

type TaggedTokenBadValueCLI struct {
	noopRunner

	Token conga.OptionalFlag[TaggedToken] `name:"token" prefix:"yes"`
}

// PlainHookToken demonstrates a user-defined domain type whose string tag
// hook rejects values with a plain error, without referencing CONGA's
// internal schema sentinels.
type PlainHookToken string

func (p *PlainHookToken) TagSpecs() []conga.TagSpec {
	return []conga.TagSpec{
		{Name: "tier", Kind: conga.TagKindString, Validate: func(value string) error {
			if value == "gold" || value == "silver" {
				return nil
			}

			return errors.New("tier must be gold or silver")
		}},
	}
}

func (p *PlainHookToken) UnmarshalTextWithContext(text []byte, _ conga.DecodeContext) error {
	*p = PlainHookToken(text)

	return nil
}

type PlainHookTokenCLI struct {
	noopRunner

	Token conga.OptionalFlag[PlainHookToken] `name:"token" tier:"bronze"`
}

func TestUserDefinedTaggedDomainType(t *testing.T) {
	t.Parallel()

	t.Run("tags are captured at parse time and validated", func(t *testing.T) {
		t.Parallel()

		cli := new(TaggedTokenCLI)
		app := conga.New(cli).Name("app").Args([]string{"--token", "abc123"})

		err := app.RunContext(t.Context())

		assert.NoError(t, err)
		assert.Equal(t, "token:abc123", string(cli.Token.Get()))
	})

	t.Run("untagged parse leaves the value unconstrained and invalid", func(t *testing.T) {
		t.Parallel()

		app := conga.New[TaggedTokenRawCLI]().Name("app").Args([]string{"--token", "abc123"})

		err := app.RunContext(t.Context())

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrValidation)
		assert.Equal(t, `token is missing the declared prefix for flag --token`, err.Error())
	})

	t.Run("excluded tag combination rejected at schema build", func(t *testing.T) {
		t.Parallel()

		app := conga.New[TaggedTokenExcludedCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct TaggedTokenExcludedCLI field Token: schema error: invalid struct tag: tag "prefix" cannot be combined with tag "raw"`, err.Error())
	})

	t.Run("invalid boolean tag value rejected at schema build", func(t *testing.T) {
		t.Parallel()

		app := conga.New[TaggedTokenBadValueCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct TaggedTokenBadValueCLI field Token: schema error: invalid struct tag: tag "prefix" expects "true" or "false", got "yes"`, err.Error())
	})

	t.Run("string tag hook plain error is classified as a schema error", func(t *testing.T) {
		t.Parallel()

		app := conga.New[PlainHookTokenCLI]()

		err := app.Run()

		assert.Error(t, err)
		assert.IsError(t, err, conga.ErrSchema)
		assert.Equal(t, `struct PlainHookTokenCLI field Token: schema error: invalid struct tag: tag "tier": tier must be gold or silver`, err.Error())
	})

	t.Run("help description is composed from the declared tags", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[TaggedTokenCLI]().Name("app").Stdout(&buffer).Args([]string{"--help"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Contains(t, buffer.String(), "API token (prefixed automatically)")
	})
}

func TestDomainTypeArgDefaultHelp(t *testing.T) {
	t.Parallel()

	type CLI struct {
		noopRunner

		Limit conga.RequiredArg[conga.ByteSize] `name:"limit"`
	}

	var buffer bytes.Buffer

	app := conga.New[CLI]().Name("app").Stdout(&buffer).Args([]string{"--help"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Byte size (e.g., 10MiB, 1.5GB, 500KiB)")
}

func TestFlagCandidateDescriptionFromDomainType(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[FileStdinCLI]().EnableCompletion().Name("app").Stdout(&buffer).Args([]string{"__complete", "export", "--"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Contains(t, buffer.String(), "Path to an existing file or '-' for stdin")
}

func writeFileForTest(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	assert.NoError(t, os.WriteFile(path, []byte("payload"), 0600))

	return path
}
