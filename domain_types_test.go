package conga_test

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
)

// emptyPathCLI declares one File, Dir, and FileContent flag per capability tag
// shape, so each empty-path row targets exactly one field.
type emptyPathCLI struct {
	noopRunner

	File       conga.OptionalFlag[conga.File]        `config:"file"     env:"EMPTY_PATH_FILE"    name:"file"`
	Stdout     conga.OptionalFlag[conga.File]        `name:"stdout"     stdout:"true"`
	Stdin      conga.OptionalFlag[conga.File]        `name:"stdin"      stdin:"true"`
	Existing   conga.OptionalFlag[conga.File]        `existing:"true"   name:"existing"`
	Executable conga.OptionalFlag[conga.File]        `executable:"true" name:"executable"`
	Symlink    conga.OptionalFlag[conga.File]        `name:"symlink"    symlink:"true"`
	Glob       conga.OptionalFlag[conga.File]        `glob:"*.txt"      name:"glob"`
	Dir        conga.OptionalFlag[conga.Dir]         `config:"dir"      env:"EMPTY_PATH_DIR"     name:"dir"`
	ExistDir   conga.OptionalFlag[conga.Dir]         `existing:"true"   name:"exist-dir"`
	Content    conga.OptionalFlag[conga.FileContent] `config:"content"  env:"EMPTY_PATH_CONTENT" name:"content"`
}

type emptyPathRequiredDirCLI struct {
	noopRunner

	OutDir conga.RequiredFlag[conga.Dir] `name:"out-dir"`
}

type emptyPathOptionalArgCLI struct {
	noopRunner

	Source conga.OptionalArg[conga.File] `name:"[src]"`
}

type emptyPathRequiredArgCLI struct {
	noopRunner

	Destination conga.RequiredArg[conga.Dir] `name:"<dst>"`
}

type emptyPathDefaultCLI struct {
	noopRunner

	File    conga.OptionalFlag[conga.File]        `default:"" name:"file"`
	Dir     conga.OptionalFlag[conga.Dir]         `default:"" name:"dir"`
	Content conga.OptionalFlag[conga.FileContent] `default:"" name:"content"`
}

// emptyPathSources reports the source of every emptyPathCLI field, so a
// rejected value is shown to leave all fields unset.
func emptyPathSources(cli *emptyPathCLI) []conga.ValueSource {
	return []conga.ValueSource{
		cli.File.Source(), cli.Stdout.Source(), cli.Stdin.Source(), cli.Existing.Source(),
		cli.Executable.Source(), cli.Symlink.Source(), cli.Glob.Source(), cli.Dir.Source(),
		cli.ExistDir.Source(), cli.Content.Source(),
	}
}

func TestEmptyPathCommandLine(t *testing.T) {
	t.Parallel()

	flags := []string{"file", "stdout", "stdin", "existing", "executable", "symlink", "glob", "dir", "exist-dir", "content"}

	// Each spelling hands the field a value that is empty or expands to blank
	// text: an attached empty value, a separate empty token, whitespace, and an
	// unset environment variable.
	spellings := []struct {
		name string
		args func(flag string) []string
	}{
		{name: "attached empty", args: func(flag string) []string { return []string{"--" + flag + "="} }},
		{name: "separate empty", args: func(flag string) []string { return []string{"--" + flag, ""} }},
		{name: "whitespace", args: func(flag string) []string { return []string{"--" + flag, "   "} }},
		{name: "unset variable", args: func(flag string) []string { return []string{"--" + flag, "$CONGA_EMPTY_PATH_UNSET_VARIABLE"} }},
	}

	for _, flag := range flags {
		for _, spelling := range spellings {
			t.Run(flag+" "+spelling.name, func(t *testing.T) {
				t.Parallel()

				cli := new(emptyPathCLI)
				app := conga.New(cli).Args(spelling.args(flag))

				result := runClassification(t, app)

				assert.Equal(t, "empty path for flag --"+flag, result.err.Error())
				assert.IsError(t, result.err, conga.ErrCoerce)
				assert.False(t, errors.Is(result.err, conga.ErrValidation))
				assert.False(t, errors.Is(result.err, conga.ErrConfig))
				assert.True(t, conga.IsUsageError(result.err))
				assert.Equal(t, 2, result.exitCode)
				assert.Equal(t, "app: error: empty path for flag --"+flag+"\n", result.output)
				assert.Equal(t, make([]conga.ValueSource, 10), emptyPathSources(cli))
			})
		}
	}
}

func TestEmptyPathRequiredDirFlag(t *testing.T) {
	t.Parallel()

	cli := new(emptyPathRequiredDirCLI)
	app := conga.New(cli).Args([]string{"--out-dir="})

	result := runClassification(t, app)

	assert.Equal(t, "empty path for flag --out-dir", result.err.Error())
	assert.IsError(t, result.err, conga.ErrCoerce)
	assert.True(t, conga.IsUsageError(result.err))
	assert.Equal(t, 2, result.exitCode)
	assert.Equal(t, conga.SourceNone, cli.OutDir.Source())
}

func TestEmptyPathPositional(t *testing.T) {
	t.Parallel()

	t.Run("optional file argument", func(t *testing.T) {
		t.Parallel()

		cli := new(emptyPathOptionalArgCLI)
		app := conga.New(cli).Args([]string{""})

		result := runClassification(t, app)

		assert.Equal(t, "empty path for argument [src]", result.err.Error())
		assert.IsError(t, result.err, conga.ErrCoerce)
		assert.True(t, conga.IsUsageError(result.err))
		assert.Equal(t, 2, result.exitCode)
		assert.Equal(t, conga.SourceNone, cli.Source.Source())
	})

	t.Run("required directory argument", func(t *testing.T) {
		t.Parallel()

		cli := new(emptyPathRequiredArgCLI)
		app := conga.New(cli).Args([]string{""})

		result := runClassification(t, app)

		assert.Equal(t, "empty path for argument <dst>", result.err.Error())
		assert.IsError(t, result.err, conga.ErrCoerce)
		assert.True(t, conga.IsUsageError(result.err))
		assert.Equal(t, 2, result.exitCode)
		assert.Equal(t, conga.SourceNone, cli.Destination.Source())
	})
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestEmptyPathEnvironmentIsUnset(t *testing.T) {
	variables := []string{"EMPTY_PATH_FILE", "EMPTY_PATH_DIR", "EMPTY_PATH_CONTENT"}

	for _, variable := range variables {
		t.Run(variable, func(t *testing.T) {
			t.Setenv(variable, "")

			cli := new(emptyPathCLI)
			app := conga.New(cli).Args([]string{})

			err := app.RunContext(t.Context())

			assert.NoError(t, err)
			assert.Equal(t, make([]conga.ValueSource, 10), emptyPathSources(cli))
		})
	}
}

func TestEmptyPathConfig(t *testing.T) {
	t.Parallel()

	// The config tier treats an empty string as absent for non-string target
	// kinds, so "" reaches the unmarshaler only for FileContent; File and Dir
	// are reached through blank text and an unset variable.
	values := []struct {
		name  string
		key   string
		value string
	}{
		{name: "content empty", key: "content", value: ""},
		{name: "content whitespace", key: "content", value: "   "},
		{name: "file whitespace", key: "file", value: "   "},
		{name: "file unset variable", key: "file", value: "$CONGA_EMPTY_PATH_UNSET_VARIABLE"},
		{name: "dir whitespace", key: "dir", value: "   "},
		{name: "dir unset variable", key: "dir", value: "$CONGA_EMPTY_PATH_UNSET_VARIABLE"},
	}

	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			t.Parallel()

			cli := new(emptyPathCLI)
			app := conga.New(cli).
				RegisterDecoder(".json", json.Decode).
				ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"`+value.key+`": "`+value.value+`"}`))).
				Args([]string{})

			result := runClassification(t, app)

			message := `empty path for flag --` + value.key + ` (from config key "` + value.key + `")`
			assert.Equal(t, message, result.err.Error())
			assert.IsError(t, result.err, conga.ErrConfig)
			assert.False(t, errors.Is(result.err, conga.ErrCoerce))
			assert.False(t, conga.IsUsageError(result.err))
			assert.Equal(t, 1, result.exitCode)
			assert.Equal(t, "app: error: "+message+"\n", result.output)
			assert.Equal(t, make([]conga.ValueSource, 10), emptyPathSources(cli))
		})
	}
}

func TestEmptyPathConfigEmptyStringIsAbsent(t *testing.T) {
	t.Parallel()

	keys := []string{"file", "dir"}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			cli := new(emptyPathCLI)
			app := conga.New(cli).
				RegisterDecoder(".json", json.Decode).
				ConfigOptions(conga.ConfigExplicit(writeConfigFile(t, `{"`+key+`": ""}`))).
				Args([]string{})

			err := app.RunContext(t.Context())

			assert.NoError(t, err)
			assert.Equal(t, make([]conga.ValueSource, 10), emptyPathSources(cli))
		})
	}
}

func TestEmptyPathDefaultIsNoDefault(t *testing.T) {
	t.Parallel()

	cli := new(emptyPathDefaultCLI)
	app := conga.New(cli).Args([]string{})

	err := app.RunContext(t.Context())

	assert.NoError(t, err)
	assert.Equal(t, conga.SourceNone, cli.File.Source())
	assert.Equal(t, conga.SourceNone, cli.Dir.Source())
	assert.Equal(t, conga.SourceNone, cli.Content.Source())
}

func TestEmptyPathUnmarshalText(t *testing.T) {
	t.Parallel()

	t.Run("File", func(t *testing.T) {
		t.Parallel()

		var file conga.File

		err := file.UnmarshalText([]byte(""))

		assert.Equal(t, "empty path", err.Error())
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, "", file.String())
	})

	t.Run("Dir", func(t *testing.T) {
		t.Parallel()

		var dir conga.Dir

		err := dir.UnmarshalText([]byte(""))

		assert.Equal(t, "empty path", err.Error())
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, "", dir.String())
	})

	t.Run("FileContent", func(t *testing.T) {
		t.Parallel()

		var content conga.FileContent

		err := content.UnmarshalText([]byte(""))

		assert.Equal(t, "empty path", err.Error())
		assert.IsError(t, err, conga.ErrCoerce)
		assert.Equal(t, "", content.String())
	})
}

func TestFileFailedUnmarshalLeavesValueUnchanged(t *testing.T) {
	t.Parallel()

	var stdoutBuffer bytes.Buffer

	populated := conga.DecodeContext{
		Tags:   conga.FieldTags{"stdin": "true", "stdout": "true"},
		Stdin:  strings.NewReader("configured stdin"),
		Stdout: &stdoutBuffer,
	}

	var file conga.File
	assert.NoError(t, file.UnmarshalTextWithContext([]byte("-"), populated))

	failing := conga.DecodeContext{
		Tags:   conga.FieldTags{"executable": "true"},
		Stdin:  strings.NewReader("other stdin"),
		Stdout: io.Discard,
	}

	err := file.UnmarshalTextWithContext([]byte("conga-no-such-executable"), failing)

	assert.IsError(t, err, exec.ErrNotFound)
	assert.Equal(t, "-", file.String())
	assert.True(t, file.IsStdio())
	assert.NoError(t, file.Validate(t.Context()))

	reader, err := file.Reader()
	assert.NoError(t, err)
	input, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, "configured stdin", string(input))

	writer, err := file.Writer()
	assert.NoError(t, err)
	_, err = io.WriteString(writer, "configured stdout")
	assert.NoError(t, err)
	assert.Equal(t, "configured stdout", stdoutBuffer.String())
}
