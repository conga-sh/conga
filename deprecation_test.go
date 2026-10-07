package conga_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
)

type DeprecatedAppWithSubCLI struct {
	noopRunner

	Deploy conga.Command[DeprecatedDeployCmd] `summary:"Deploy command"`
}

type DeprecatedDeployCmd struct {
	noopRunner
}

type DeprecatedSubcommandCLI struct {
	noopRunner

	Legacy conga.Command[DeprecatedLegacyCmd] `deprecated:"use deploy instead" summary:"Legacy command"`
}

type DeprecatedLegacyCmd struct {
	noopRunner
}

type DeprecatedFlagCLI struct {
	noopRunner

	Legacy conga.OptionalFlag[string] `default:"old" deprecated:"use --modern instead" env:"LEGACY_FLAG" name:"legacy"`
}

type DeprecationPurityCLI struct {
	noopRunner

	Legacy conga.OptionalFlag[string] `deprecated:"use --modern instead" name:"legacy"`
	Modern conga.OptionalFlag[string] `name:"modern"`
}

func TestDeprecation_ApplicationCascading(t *testing.T) {
	t.Parallel()

	t.Run("root invocation emits notice", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[DeprecatedAppWithSubCLI]().
			Name("testapp").
			Deprecated("use the new unified CLI").
			Stderr(&buffer).
			Args([]string{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "WARNING: testapp is deprecated: use the new unified CLI\n", buffer.String())
	})

	t.Run("subcommand invocation still emits root notice", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[DeprecatedAppWithSubCLI]().
			Name("testapp").
			Deprecated("use the new unified CLI").
			Stderr(&buffer).
			Args([]string{"deploy"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "WARNING: testapp is deprecated: use the new unified CLI\n", buffer.String())
	})
}

func TestDeprecation_SubcommandTag(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[DeprecatedSubcommandCLI]().
		Name("testapp").
		Stderr(&buffer).
		Args([]string{"legacy"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "WARNING: command 'testapp legacy' is deprecated: use deploy instead\n", buffer.String())
}

func TestDeprecation_ProvenanceGating(t *testing.T) {
	t.Parallel()

	t.Run("CLI source", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[DeprecatedFlagCLI]().
			Name("testapp").
			Stderr(&buffer).
			Args([]string{"--legacy", "from-cli"})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "WARNING: flag '--legacy' is deprecated: use --modern instead\n", buffer.String())
	})

	t.Run("config source", func(t *testing.T) {
		t.Parallel()

		cfgPath := filepath.Join(t.TempDir(), "config.json")
		assert.NoError(t, os.WriteFile(cfgPath, []byte(`{"legacy": "from-config"}`), 0o600))

		var buffer bytes.Buffer

		app := conga.New[DeprecatedFlagCLI]().
			Name("testapp").
			RegisterDecoder(".json", json.RawDecoder).
			Stderr(&buffer).
			ConfigOptions(conga.ConfigPaths(cfgPath)).
			Args([]string{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "WARNING: config key 'legacy' is deprecated: use --modern instead\n", buffer.String())
	})

	t.Run("default source is silent", func(t *testing.T) {
		t.Parallel()

		var buffer bytes.Buffer

		app := conga.New[DeprecatedFlagCLI]().
			Name("testapp").
			Stderr(&buffer).
			Args([]string{})

		err := app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "", buffer.String())
	})
}

func TestDeprecation_ProvenanceGating_Env(t *testing.T) {
	t.Setenv("LEGACY_FLAG", "from-env")

	var buffer bytes.Buffer

	app := conga.New[DeprecatedFlagCLI]().
		Name("testapp").
		Stderr(&buffer).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "WARNING: environment variable 'LEGACY_FLAG' is deprecated: use --modern instead\n", buffer.String())
}

func TestDeprecation_SuppressionOnError(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[DeprecatedFlagCLI]().
		Name("testapp").
		Deprecated("use the new unified CLI").
		Stderr(&buffer).
		Args([]string{"--unknown-flag"})

	err := app.Run()

	assert.Error(t, err)
	assert.Equal(t, "", buffer.String())
}

func TestDeprecation_CompletionPurity(t *testing.T) {
	t.Parallel()

	var stdoutBuffer, stderrBuffer bytes.Buffer

	app := conga.New[DeprecationPurityCLI]().
		EnableCompletion().
		Name("testapp").
		Stdout(&stdoutBuffer).
		Stderr(&stderrBuffer).
		Args([]string{"__complete", "fish", "--m"})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "", stderrBuffer.String())
	assert.Contains(t, stdoutBuffer.String(), "--modern")
}

type deprecationConfigFileFlagCLI struct {
	noopRunner

	Config conga.ConfigFile `deprecated:"use --config-path instead" name:"config"`
}

type deprecationConfigFilesFlagCLI struct {
	noopRunner

	Configs conga.ConfigFile `deprecated:"use --config-dir instead" multi:"true" name:"configs"`
}

func TestDeprecation_ConfigFileFlag(t *testing.T) {
	t.Parallel()

	t.Run("single config file flag", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "app.json")
		err := os.WriteFile(cfgPath, []byte("{}"), 0o600)
		assert.NoError(t, err)

		var buffer bytes.Buffer

		app := conga.New[deprecationConfigFileFlagCLI]().
			RegisterDecoder(".json", json.RawDecoder).
			Name("testapp").
			Stderr(&buffer).
			Args([]string{"--config", cfgPath})

		err = app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "WARNING: flag '--config' is deprecated: use --config-path instead\n", buffer.String())
	})

	t.Run("multi config files flag", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "app.json")
		err := os.WriteFile(cfgPath, []byte("{}"), 0o600)
		assert.NoError(t, err)

		var buffer bytes.Buffer

		app := conga.New[deprecationConfigFilesFlagCLI]().
			RegisterDecoder(".json", json.RawDecoder).
			Name("testapp").
			Stderr(&buffer).
			Args([]string{"--configs", cfgPath})

		err = app.Run()

		assert.NoError(t, err)
		assert.Equal(t, "WARNING: flag '--configs' is deprecated: use --config-dir instead\n", buffer.String())
	})
}

type deprecationFilterCLI struct {
	noopRunner

	ModernFlag conga.OptionalFlag[string] `name:"modern"`
	LegacyFlag conga.OptionalFlag[string] `deprecated:"use --modern instead" name:"legacy"`

	Deploy conga.Command[deprecationFilterDeployCmd] `summary:"Deploy"`
	Legacy conga.Command[deprecationFilterLegacyCmd] `deprecated:"use deploy instead" summary:"Legacy"`
}

type deprecationFilterDeployCmd struct {
	noopRunner
}

type deprecationFilterLegacyCmd struct {
	noopRunner
}

func TestDeprecation_CompletionFiltering(t *testing.T) {
	t.Parallel()

	app := conga.New[deprecationFilterCLI]().Name("testapp")

	t.Run("deprecated flags are excluded from flag completion", func(t *testing.T) {
		t.Parallel()

		invoked, _ := app.CompleteForShell("fish", []string{"--"})

		values := make(map[string]bool, len(invoked.Items))

		for _, item := range invoked.Items {
			values[item.Value] = true
		}

		assert.True(t, values["--modern"])
		assert.False(t, values["--legacy"])
	})

	t.Run("deprecated subcommands are excluded from command completion", func(t *testing.T) {
		t.Parallel()

		invoked, _ := app.CompleteForShell("fish", []string{""})

		values := make(map[string]bool, len(invoked.Items))

		for _, item := range invoked.Items {
			values[item.Value] = true
		}

		assert.True(t, values["deploy"])
		assert.False(t, values["legacy"])
	})
}

func TestDeprecation_RootWarningRendered(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer

	app := conga.New[DeprecatedAppWithSubCLI]().
		Name("testapp").
		Deprecated("use the new unified CLI").
		Stderr(&buffer).
		Args([]string{})

	err := app.Run()

	assert.NoError(t, err)
	assert.Equal(t, "WARNING: testapp is deprecated: use the new unified CLI\n", buffer.String())
}

type deprecatedReqFlagCLI struct {
	noopRunner

	Token conga.RequiredFlag[string] `deprecated:"use --new-token instead" name:"token"`
}

func TestDeprecation_RequiredFlagRejected(t *testing.T) {
	t.Parallel()

	app := conga.New[deprecatedReqFlagCLI]().
		Name("testapp").
		Args([]string{"--token", "abc"})

	err := app.Run()

	assert.Error(t, err)
	assert.IsError(t, err, conga.ErrSchema)
	assert.Equal(t, `struct deprecatedReqFlagCLI field Token: schema error: invalid struct tag: tag "deprecated" cannot be used on required flags (make the flag optional before deprecating it so callers can migrate without error)`, err.Error())
}
