package conga_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/internal/errlines"
)

// originCLI declares scalar, map and feature values that every origin
// source can supply.
type originCLI struct {
	noopRunner

	Config conga.ConfigFile `multi:"true" name:"config-file"`

	Port    conga.OptionalFlag[int]               `env:"TOOL_PORT"              name:"port"`
	Limits  conga.OptionalFlag[map[string]int]    `env:"TOOL_LIMITS"            name:"limits"`
	Input   conga.OptionalFlag[conga.File]        `env:"TOOL_INPUT"             existing:"true" name:"input"`
	Retries conga.OptionalFlag[int]               `default:"abc"                name:"retries"`
	Token   conga.OptionalFlag[conga.FileContent] `default:"/nonexistent/token" name:"token"`

	Count conga.OptionalArg[int] `default:"abc" name:"[count]"`
}

// originFeatureSet declares the features the origin tests set.
type originFeatureSet struct {
	Level int `enum:"1,2"`
}

// originFeatureCLI gates a flag behind a feature.
type originFeatureCLI struct {
	noopRunner

	Features conga.Features[originFeatureSet] `config:"experiments" env:"TOOL_FEATURES" flag:"feature-gates"`
}

// originBootstrapCLI binds a bootstrap flag from the .env file its bootstrap
// phase loads.
type originBootstrapCLI struct {
	noopRunner

	Retry conga.OptionalFlag[int] `bootstrap:"true" env:"TOOL_RETRY" name:"retry"`
}

func (c *originBootstrapCLI) Bootstrap(_ context.Context, b *conga.Bootstrap) error {
	return b.Dotenv(".env")
}

func originApp(path string) *conga.App[originCLI] {
	app := conga.New[originCLI]().Name("app").RegisterDecoder(".json", json.Decode)

	if path != "" {
		app = app.ConfigOptions(conga.ConfigExplicit(path))
	}

	return app
}

// tokenFile writes a token file and returns its path.
func tokenFile(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "token")

	assert.NoError(t, os.WriteFile(path, []byte("secret"), 0o600))

	return path
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOrigins_ConfigFileMatrix(t *testing.T) {
	t.Run("--config-file", func(t *testing.T) {
		path := writeConfigFile(t, `{"port": "abc"}`)

		err := originApp("").Args([]string{"--config-file", path, "--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, `invalid integer value "abc" for flag --port`, err.Error())
		assert.Equal(t, []string{`config key "port" in config file "` + path + `"`}, errlines.Origins(err))
	})

	t.Run("the config file environment variable", func(t *testing.T) {
		path := writeConfigFile(t, `{"port": "abc"}`)

		t.Setenv("TOOL_CONFIG", path)

		err := originApp("").EnvPrefix("TOOL_").Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{`config key "port" in config file "` + path + `"`}, errlines.Origins(err))
	})

	t.Run("ConfigExplicit", func(t *testing.T) {
		path := writeConfigFile(t, `{"port": "abc"}`)

		err := originApp(path).Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{`config key "port" in config file "` + path + `"`}, errlines.Origins(err))
	})

	t.Run("the XDG tier", func(t *testing.T) {
		directory := t.TempDir()

		t.Setenv("XDG_CONFIG_HOME", directory)

		assert.NoError(t, os.MkdirAll(filepath.Join(directory, "tool"), 0o700))

		path := filepath.Join(directory, "tool", "config.json")

		assert.NoError(t, os.WriteFile(path, []byte(`{"port": "abc"}`), 0o600))

		err := conga.New[originCLI]().Name("tool").RegisterDecoder(".json", json.Decode).
			Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{`config key "port" in config file "` + path + `"`}, errlines.Origins(err))
	})

	t.Run("search paths", func(t *testing.T) {
		path := writeConfigFile(t, `{"port": "abc"}`)

		err := conga.New[originCLI]().RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigPaths(path)).
			Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{`config key "port" in config file "` + path + `"`}, errlines.Origins(err))
	})

	t.Run("cascading names the file that holds the bad value", func(t *testing.T) {
		goodPath := writeConfigFile(t, `{"port": 1}`)
		badPath := writeConfigFile(t, `{"port": "abc"}`)

		err := conga.New[originCLI]().RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigPaths(badPath, goodPath), conga.ConfigCascading()).
			Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{`config key "port" in config file "` + badPath + `"`}, errlines.Origins(err))
	})

	t.Run("a relative path is shown as typed", func(t *testing.T) {
		t.Chdir(t.TempDir())

		assert.NoError(t, os.WriteFile("tool.json", []byte(`{"port": "abc"}`), 0o600))

		err := originApp("tool.json").Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{`config key "port" in config file "tool.json"`}, errlines.Origins(err))
	})
}

func TestErrorOrigins_MapEntryFromConfigFile(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, `{"limits": {"a": "x"}}`)

	err := originApp(path).Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

	assert.Equal(t, `value for key "a": invalid integer value "x" for flag --limits`, err.Error())
	assert.Equal(t, []string{`config key "limits" in config file "` + path + `"`}, errlines.Origins(err))
}

func TestErrorOrigins_Defaults(t *testing.T) {
	t.Parallel()

	t.Run("a static default that does not parse", func(t *testing.T) {
		t.Parallel()

		err := originApp("").Args([]string{"--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, `invalid integer value "abc" for flag --retries`, err.Error())
		assert.Equal(t, []string{"default value"}, errlines.Origins(err))
	})

	t.Run("an environment-dependent default", func(t *testing.T) {
		t.Parallel()

		err := originApp("").Args([]string{"--retries", "1", "5"}).RunContext(t.Context())

		assert.Equal(t, "stat /nonexistent/token: no such file or directory for flag --token", err.Error())
		assert.Equal(t, []string{"default value"}, errlines.Origins(err))
	})

	t.Run("a positional argument default", func(t *testing.T) {
		t.Parallel()

		err := originApp("").Args([]string{"--retries", "1", "--token", tokenFile(t)}).RunContext(t.Context())

		assert.Equal(t, "invalid integer value \"abc\" for argument [count]", err.Error())
		assert.Equal(t, []string{"default value"}, errlines.Origins(err))
	})
}

func TestErrorOrigins_Documents(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		build   func(document *conga.ConfigDocument)
		origins []string
	}{
		{
			name:    "a document label",
			build:   func(document *conga.ConfigDocument) { document.Set("port", "abc", "profile dev") },
			origins: []string{`config key "port" set by profile dev`},
		},
		{
			name:    "a scoped document value",
			build:   func(document *conga.ConfigDocument) { document.SetScoped("port", "abc", "profiles.yaml", "dev") },
			origins: []string{`config key "port" set by profiles.yaml › dev`},
		},
		{
			name:    "a document value with an empty origin",
			build:   func(document *conga.ConfigDocument) { document.Set("port", "abc", "") },
			origins: []string{`config key "port"`},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cli := &documentBasicCLI{doc: documentWith(testCase.build)}

			err := bootstrapApp(cli).Name("app").Args([]string{}).RunContext(t.Context())

			assert.Equal(t, `invalid integer value "abc" for flag --port`, err.Error())
			assert.Equal(t, testCase.origins, errlines.Origins(err))
		})
	}

	t.Run("a document with an unknown key names the label alone", func(t *testing.T) {
		t.Parallel()

		cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) { document.Set("prot", "1", "profile dev") })}

		err := bootstrapApp(cli).Name("app").Args([]string{}).RunContext(t.Context())

		assert.Equal(t, `unknown field "prot" in command "app"`, err.Error())
		assert.Equal(t, []string{"set by profile dev"}, errlines.Origins(err))
		assert.Equal(t, `did you mean "port"?`, errlines.Hint(err))
	})

	t.Run("a scoped document with an unknown key names the label and scope", func(t *testing.T) {
		t.Parallel()

		cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) {
			document.SetScoped("prot", "1", "profiles.yaml", "dev")
		})}

		err := bootstrapApp(cli).Name("app").Args([]string{}).RunContext(t.Context())

		assert.Equal(t, []string{"set by profiles.yaml › dev"}, errlines.Origins(err))
	})

	t.Run("a document with an empty origin and an unknown key has no origin line", func(t *testing.T) {
		t.Parallel()

		cli := &documentBasicCLI{doc: documentWith(func(document *conga.ConfigDocument) { document.Set("prot", "1", "") })}

		err := bootstrapApp(cli).Name("app").Args([]string{}).RunContext(t.Context())

		assert.Zero(t, errlines.Origins(err))
	})
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOrigins_WholeFileErrors(t *testing.T) {
	t.Run("an unknown field names the config file", func(t *testing.T) {
		path := writeConfigFile(t, `{"prot": 1}`)

		err := originApp(path).Args([]string{"5"}).RunContext(t.Context())

		assert.Equal(t, `unknown field "prot" in command "app"`, err.Error())
		assert.Equal(t, []string{`config file "` + path + `"`}, errlines.Origins(err))
		assert.Equal(t, `did you mean "port"?`, errlines.Hint(err))
	})

	t.Run("cannot parse names the file in line one and adds no origin line", func(t *testing.T) {
		path := writeConfigFile(t, `{not json`)

		err := originApp(path).Args([]string{"5"}).RunContext(t.Context())

		assert.Contains(t, err.Error(), `cannot parse config file "`+path+`"`)
		assert.Zero(t, errlines.Origins(err))
	})

	t.Run("cannot read names the file in line one and adds no origin line", func(t *testing.T) {
		err := originApp(filepath.Join(t.TempDir(), "missing.json")).Args([]string{"5"}).RunContext(t.Context())

		assert.Contains(t, err.Error(), "cannot read config file")
		assert.Zero(t, errlines.Origins(err))
	})

	t.Run("an unsupported format names the file in line one and adds no origin line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tool.ini")

		assert.NoError(t, os.WriteFile(path, []byte("x"), 0o600))

		err := originApp(path).Args([]string{"5"}).RunContext(t.Context())

		assert.Contains(t, err.Error(), "unsupported config file format")
		assert.Zero(t, errlines.Origins(err))
	})

	t.Run("conflicting config files name the directory in line one and add no origin line", func(t *testing.T) {
		directory := t.TempDir()

		t.Setenv("XDG_CONFIG_HOME", directory)

		assert.NoError(t, os.MkdirAll(filepath.Join(directory, "tool"), 0o700))
		assert.NoError(t, os.WriteFile(filepath.Join(directory, "tool", "config.json"), []byte(`{}`), 0o600))
		assert.NoError(t, os.WriteFile(filepath.Join(directory, "tool", "tool.json"), []byte(`{}`), 0o600))

		err := conga.New[originCLI]().Name("tool").RegisterDecoder(".json", json.Decode).Args([]string{"5"}).RunContext(t.Context())

		assert.Contains(t, err.Error(), "multiple conflicting config files found")
		assert.Zero(t, errlines.Origins(err))
	})
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOrigins_Features(t *testing.T) {
	t.Run("an invalid feature value from the config file names the file", func(t *testing.T) {
		path := writeConfigFile(t, `{"experiments": {"level": "9"}}`)

		err := conga.New[originFeatureCLI]().EnableFeatures().RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).Args([]string{}).RunContext(t.Context())

		assert.Equal(t, `invalid value "9" for feature "level"`, err.Error())
		assert.Equal(t, []string{`config key "experiments.level" in config file "` + path + `"`}, errlines.Origins(err))
		assert.Equal(t, "accepted values: 0, 1, 2", errlines.Note(err))
	})

	t.Run("a feature value 01 reads the integer note and hint", func(t *testing.T) {
		err := conga.New[originFeatureCLI]().EnableFeatures().Args([]string{"--feature-gates", "level=01"}).RunContext(t.Context())

		assert.Equal(t, `invalid value "01" for feature "level"`, err.Error())
		assert.Equal(t, "a leading zero is ambiguous", errlines.Note(err))
		assert.Equal(t, "write 0o1 for octal or 1 for decimal", errlines.Hint(err))
	})

	t.Run("an unknown feature suggestion is quoted", func(t *testing.T) {
		err := conga.New[originFeatureCLI]().EnableFeatures().Args([]string{"--feature-gates", "levle=1"}).RunContext(t.Context()) //nolint:misspell // intentionally misspelled feature name

		assert.Equal(t, `unknown feature "levle"`, err.Error()) //nolint:misspell // intentionally misspelled feature name
		assert.Equal(t, `did you mean "level"?`, errlines.Hint(err))
	})

	t.Run("an invalid feature value from a .env file names the file", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), ".env.local")

		assert.NoError(t, os.WriteFile(envPath, []byte("TOOL_FEATURES=level=9\n"), 0o600))

		err := conga.New[originFeatureCLI]().EnableFeatures().Dotenv(envPath).SetenvFunc(t.Setenv).
			Args([]string{}).RunContext(t.Context())

		assert.Equal(t, `invalid value "9" for feature "level"`, err.Error())
		assert.Equal(t, []string{`environment variable TOOL_FEATURES in .env file "` + envPath + `"`}, errlines.Origins(err))
	})

	t.Run("an unknown feature from a .env file names the file", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), ".env.local")

		assert.NoError(t, os.WriteFile(envPath, []byte("TOOL_FEATURES=levle=1\n"), 0o600)) //nolint:misspell // intentionally misspelled feature name

		err := conga.New[originFeatureCLI]().EnableFeatures().Dotenv(envPath).SetenvFunc(t.Setenv).
			Args([]string{}).RunContext(t.Context())

		assert.Equal(t, `unknown feature "levle"`, err.Error()) //nolint:misspell // intentionally misspelled feature name
		assert.Equal(t, []string{`environment variable TOOL_FEATURES in .env file "` + envPath + `"`}, errlines.Origins(err))
	})
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOrigins_Dotenv(t *testing.T) {
	t.Run("a bad value from App.Dotenv names the file", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), ".env.local")

		assert.NoError(t, os.WriteFile(envPath, []byte("TOOL_PORT=abc\n"), 0o600))

		err := originApp("").Dotenv(envPath).SetenvFunc(t.Setenv).Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{`environment variable TOOL_PORT in .env file "` + envPath + `"`}, errlines.Origins(err))
	})

	t.Run("a later .env file that overrides an earlier one is the named file", func(t *testing.T) {
		directory := t.TempDir()
		first := filepath.Join(directory, ".env")
		second := filepath.Join(directory, ".env.local")

		assert.NoError(t, os.WriteFile(first, []byte("TOOL_PORT=1\n"), 0o600))
		assert.NoError(t, os.WriteFile(second, []byte("TOOL_PORT=abc\n"), 0o600))

		err := originApp("").Dotenv(first, second).SetenvFunc(t.Setenv).Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{`environment variable TOOL_PORT in .env file "` + second + `"`}, errlines.Origins(err))
	})

	t.Run("a variable the shell already exported names no file", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), ".env")

		assert.NoError(t, os.WriteFile(envPath, []byte("TOOL_PORT=1\n"), 0o600))

		t.Setenv("TOOL_PORT", "abc")

		err := originApp("").Dotenv(envPath).SetenvFunc(t.Setenv).Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{"environment variable TOOL_PORT"}, errlines.Origins(err))
	})

	t.Run("a map flag from a .env file names the file", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), ".env")

		assert.NoError(t, os.WriteFile(envPath, []byte("TOOL_LIMITS=a=x\n"), 0o600))

		err := originApp("").Dotenv(envPath).SetenvFunc(t.Setenv).Args([]string{"--retries", "1", "--token", "x", "5"}).RunContext(t.Context())

		assert.Equal(t, []string{`environment variable TOOL_LIMITS in .env file "` + envPath + `"`}, errlines.Origins(err))
	})

	t.Run("a bootstrap flag from a .env file names the file", func(t *testing.T) {
		t.Chdir(t.TempDir())

		assert.NoError(t, os.WriteFile(".env", []byte("TOOL_RETRY=abc\n"), 0o600))

		err := conga.New[originBootstrapCLI]().Name("app").SetenvFunc(t.Setenv).Args([]string{}).RunContext(t.Context())

		assert.Equal(t, `invalid integer value "abc" for flag --retry`, err.Error())
		assert.Equal(t, []string{`environment variable TOOL_RETRY in .env file ".env"`}, errlines.Origins(err))
	})

	t.Run("an xor conflict from a .env file names the file", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), ".env")

		assert.NoError(t, os.WriteFile(envPath, []byte("TOOL_TO_FILE=out.txt\n"), 0o600))

		err := conga.New[outputGroupCLI]().Dotenv(envPath).SetenvFunc(t.Setenv).Args([]string{"--to-url", "https://example.com"}).RunContext(t.Context())

		assert.Equal(t, []string{`--to-file: environment variable TOOL_TO_FILE in .env file "` + envPath + `"`}, errlines.Origins(err))
	})

	t.Run("cannot read and cannot parse name the file in line one and add no origin line", func(t *testing.T) {
		directory := t.TempDir()
		missing := filepath.Join(directory, "missing.env")
		broken := filepath.Join(directory, "broken.env")

		assert.NoError(t, os.WriteFile(broken, []byte("='unterminated\n"), 0o600))

		missingErr := originApp("").Dotenv(missing).SetenvFunc(t.Setenv).Args([]string{"5"}).RunContext(t.Context())
		brokenErr := originApp("").Dotenv(broken).SetenvFunc(t.Setenv).Args([]string{"5"}).RunContext(t.Context())

		assert.Contains(t, missingErr.Error(), `cannot read .env file "`+missing+`"`)
		assert.Zero(t, errlines.Origins(missingErr))
		assert.Contains(t, brokenErr.Error(), `cannot parse .env file "`+broken+`"`)
		assert.Zero(t, errlines.Origins(brokenErr))
	})
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOrigins_PostBindConstraints(t *testing.T) {
	type rangeCLI struct {
		noopRunner

		Size conga.OptionalFlag[int] `env:"TOOL_SIZE" name:"size" range:"1..10"`
	}

	t.Run("a range violation from a config file names the file", func(t *testing.T) {
		path := writeConfigFile(t, `{"size": 11}`)

		err := conga.New[rangeCLI]().RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).Args([]string{}).RunContext(t.Context())

		assert.Equal(t, "value 11 is out of range [1..10] for flag --size", err.Error())
		assert.Equal(t, []string{`config key "size" in config file "` + path + `"`}, errlines.Origins(err))
	})

	t.Run("a range violation from a .env file names the file", func(t *testing.T) {
		envPath := filepath.Join(t.TempDir(), ".env")

		assert.NoError(t, os.WriteFile(envPath, []byte("TOOL_SIZE=11\n"), 0o600))

		err := conga.New[rangeCLI]().Dotenv(envPath).SetenvFunc(t.Setenv).Args([]string{}).RunContext(t.Context())

		assert.Equal(t, []string{`environment variable TOOL_SIZE in .env file "` + envPath + `"`}, errlines.Origins(err))
	})
}

//nolint:paralleltest // subtests mutate the process environment with t.Setenv
func TestErrorOrigins_SelfValidator(t *testing.T) {
	type existingCLI struct {
		noopRunner

		Input conga.OptionalFlag[conga.File] `env:"TOOL_INPUT" existing:"true" name:"input"`
	}

	t.Run("a config value names the file", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.txt")
		path := writeConfigFile(t, `{"input": "`+missing+`"}`)

		err := conga.New[existingCLI]().RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).Args([]string{}).RunContext(t.Context())

		assert.Equal(t, []string{`config key "input" in config file "` + path + `"`}, errlines.Origins(err))
	})

	t.Run("a .env value names the .env file", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.txt")
		envPath := filepath.Join(t.TempDir(), ".env")

		assert.NoError(t, os.WriteFile(envPath, []byte("TOOL_INPUT="+missing+"\n"), 0o600))

		err := conga.New[existingCLI]().Dotenv(envPath).SetenvFunc(t.Setenv).Args([]string{}).RunContext(t.Context())

		assert.Equal(t, []string{`environment variable TOOL_INPUT in .env file "` + envPath + `"`}, errlines.Origins(err))
	})
}

func TestErrorOrigins_PositionalArgumentCommandLineNamesNoOrigin(t *testing.T) {
	t.Parallel()

	err := originApp("").Args([]string{"--retries", "1", "--token", tokenFile(t), "abc"}).RunContext(t.Context())

	assert.Equal(t, `invalid integer value "abc" for argument [count]`, err.Error())
	assert.Zero(t, errlines.Origins(err))
}

func TestErrorOutput_CompletionBadShellKeepsTheNote(t *testing.T) {
	t.Parallel()

	err := conga.New[outputSubcommandCLI]().EnableCompletion().Args([]string{"completion", "bogus"}).RunContext(t.Context())

	usage := assertUsageError(t, err)

	assert.Equal(t, `unknown shell "bogus"`, err.Error())
	assert.Equal(t, "supported shells: bash, elvish, fish, nushell, powershell, zsh", usage.Note())
}

// wholeMapSelfValidator is a map type with a whole-map SelfValidator.
type wholeMapSelfValidator map[string]string

func (m wholeMapSelfValidator) Validate(_ context.Context) error {
	return errors.New("map policy rejected")
}

func TestErrorOrigins_MapFlagConfigAndDocument(t *testing.T) {
	t.Parallel()

	type mapGroupCLI struct {
		noopRunner

		Labels conga.OptionalFlag[map[string]int] `name:"labels" xor:"target"`
		Target conga.OptionalFlag[string]         `name:"target" xor:"target"`
	}

	type mapDocumentGroupCLI struct {
		noopRunner
		documentBootstrapper

		Labels conga.OptionalFlag[map[string]int] `name:"labels" xor:"target"`
		Target conga.OptionalFlag[string]         `name:"target" xor:"target"`
	}

	t.Run("xor group conflict with a map flag from a config file names the file", func(t *testing.T) {
		t.Parallel()

		path := writeConfigFile(t, `{"labels": {"env": 1}}`)

		err := conga.New[mapGroupCLI]().RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).
			Args([]string{"--target", "prod"}).RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, []string{`--labels: config key "labels" in config file "` + path + `"`}, errlines.Origins(err))
	})

	t.Run("xor group conflict with a map flag from a scoped ConfigDocument names the document", func(t *testing.T) {
		t.Parallel()

		var document conga.ConfigDocument
		document.SetScoped("labels", map[string]any{"env": 1}, "profiles.yaml", "dev")

		cli := &mapDocumentGroupCLI{doc: &document}

		err := bootstrapApp(cli).Args([]string{"--target", "prod"}).RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, []string{`--labels: config key "labels" set by profiles.yaml › dev`}, errlines.Origins(err))
	})

	t.Run("xor group conflict with a map flag merged across cascading config files drops the file name", func(t *testing.T) {
		t.Parallel()

		first := writeConfigFile(t, `{"labels": {"a": 1}}`)
		second := writeConfigFile(t, `{"labels": {"b": 2}}`)

		type cascadingMapGroupCLI struct {
			noopRunner

			Config conga.ConfigFile                   `multi:"true"  name:"config-file"`
			Labels conga.OptionalFlag[map[string]int] `name:"labels" xor:"target"`
			Target conga.OptionalFlag[string]         `name:"target" xor:"target"`
		}

		err := conga.New[cascadingMapGroupCLI]().RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigPaths(first, second), conga.ConfigCascading()).
			Args([]string{"--target", "prod"}).RunContext(t.Context())

		assertUsageError(t, err)
		assert.Equal(t, []string{`--labels: config key "labels"`}, errlines.Origins(err))
	})

	t.Run("whole-map SelfValidator from a single config file names the file", func(t *testing.T) {
		t.Parallel()

		type wholeMapCLI struct {
			noopRunner

			Limits conga.OptionalFlag[wholeMapSelfValidator] `name:"limits"`
		}

		path := writeConfigFile(t, `{"limits": {"cpu": "2"}}`)

		err := conga.New[wholeMapCLI]().RegisterDecoder(".json", json.Decode).
			ConfigOptions(conga.ConfigExplicit(path)).Args([]string{}).RunContext(t.Context())

		assert.Equal(t, `map policy rejected for flag --limits`, err.Error())
		assert.Equal(t, []string{`config key "limits" in config file "` + path + `"`}, errlines.Origins(err))
	})

	t.Run("map[string]conga.File from a ConfigDocument names the document", func(t *testing.T) {
		t.Parallel()

		type fileMapCLI struct {
			noopRunner
			documentBootstrapper

			Files conga.OptionalFlag[map[string]conga.File] `existing:"true" name:"files"`
		}

		missing := filepath.Join(t.TempDir(), "missing.txt")

		var document conga.ConfigDocument
		document.Set("files", map[string]any{"main": missing}, "profile dev")

		cli := &fileMapCLI{doc: &document}

		err := bootstrapApp(cli).Args([]string{}).RunContext(t.Context())

		assert.Equal(t, []string{`config key "files" set by profile dev`}, errlines.Origins(err))
	})
}
