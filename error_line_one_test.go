package conga_test

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/decoder/json"
	"github.com/conga-sh/conga/network"
	"github.com/conga-sh/conga/system"
)

// lineOneCLI declares one flag per built-in parser whose failure carries
// pieces below its first line.
type lineOneCLI struct {
	noopRunner

	Enabled conga.OptionalFlag[bool]               `env:"L1_ENABLED" name:"enabled"`
	When    conga.OptionalFlag[time.Time]          `env:"L1_WHEN"    name:"when"`
	Site    conga.OptionalFlag[*url.URL]           `env:"L1_SITE"    name:"site"`
	Labels  conga.OptionalFlag[map[string]string]  `env:"L1_LABELS"  name:"labels"`
	Port    conga.OptionalFlag[network.Port]       `env:"L1_PORT"    name:"port"`
	Status  conga.OptionalFlag[network.HTTPStatus] `env:"L1_STATUS"  name:"status"`
	Process conga.OptionalFlag[system.PID]         `env:"L1_PID"     name:"pid"`
}

// lineOneFeatures declares the feature the feature-value case sets.
type lineOneFeatures struct {
	Level int `enum:"1,2"`
}

// lineOneFeatureCLI gates a flag behind a feature.
type lineOneFeatureCLI struct {
	noopRunner

	Features conga.Features[lineOneFeatures] `env:"L1_FEATURES" flag:"feature-gates"`
}

// lineOneSchemaCLI declares a tag the schema build rejects with a note and a
// hint.
type lineOneSchemaCLI struct {
	noopRunner

	Port conga.OptionalFlag[int] `bogus:"x" name:"port"`
}

// lineOneConfigCLI declares one flag a strict config document can misspell.
type lineOneConfigCLI struct {
	noopRunner

	Port conga.OptionalFlag[int] `name:"port"`
}

func TestErrorLineOne_EveryTypeWithPieces(t *testing.T) {
	configPath := writeConfigFile(t, `{"prot": 1}`)
	missingEnv := filepath.Join(t.TempDir(), "missing.env")

	cases := []struct {
		name     string
		environ  map[string]string
		run      func() error
		expected string
	}{
		{
			name:     "a usage error",
			run:      func() error { return conga.New[outputCLI]().Args([]string{"--level", "mid"}).RunContext(t.Context()) },
			expected: `invalid value "mid" for flag --level`,
		},
		{
			name:     "a byte size",
			environ:  map[string]string{"TOOL_LIMIT": "5XB"},
			run:      func() error { return conga.New[outputCLI]().Args(nil).RunContext(t.Context()) },
			expected: `unknown byte size unit "XB" in "5XB" for flag --limit`,
		},
		{
			name:     "a duration",
			environ:  map[string]string{"TOOL_TIMEOUT": "30"},
			run:      func() error { return conga.New[outputCLI]().Args(nil).RunContext(t.Context()) },
			expected: `invalid duration "30" for flag --timeout`,
		},
		{
			name:     "an integer with a leading zero",
			environ:  map[string]string{"TOOL_MODE": "0755"},
			run:      func() error { return conga.New[outputCLI]().Args(nil).RunContext(t.Context()) },
			expected: `invalid unsigned integer value "0755" for flag --mode`,
		},
		{
			name:     "a boolean",
			environ:  map[string]string{"L1_ENABLED": "maybe"},
			run:      func() error { return conga.New[lineOneCLI]().Args(nil).RunContext(t.Context()) },
			expected: `invalid boolean value "maybe" for flag --enabled`,
		},
		{
			name:     "a time",
			environ:  map[string]string{"L1_WHEN": "noon"},
			run:      func() error { return conga.New[lineOneCLI]().Args(nil).RunContext(t.Context()) },
			expected: `invalid time "noon" for flag --when`,
		},
		{
			name:     "a URL",
			environ:  map[string]string{"L1_SITE": "example.com"},
			run:      func() error { return conga.New[lineOneCLI]().Args(nil).RunContext(t.Context()) },
			expected: `invalid URL "example.com" for flag --site`,
		},
		{
			name:     "a map pair",
			environ:  map[string]string{"L1_LABELS": "novalue"},
			run:      func() error { return conga.New[lineOneCLI]().Args(nil).RunContext(t.Context()) },
			expected: `invalid key-value pair "novalue" for flag --labels`,
		},
		{
			name:     "a network port",
			environ:  map[string]string{"L1_PORT": "0"},
			run:      func() error { return conga.New[lineOneCLI]().Args(nil).RunContext(t.Context()) },
			expected: `invalid port 0 for flag --port`,
		},
		{
			name:     "an HTTP status",
			environ:  map[string]string{"L1_STATUS": "700"},
			run:      func() error { return conga.New[lineOneCLI]().Args(nil).RunContext(t.Context()) },
			expected: `invalid HTTP status 700 for flag --status`,
		},
		{
			name:     "a process ID",
			environ:  map[string]string{"L1_PID": "0"},
			run:      func() error { return conga.New[lineOneCLI]().Args(nil).RunContext(t.Context()) },
			expected: `invalid PID 0 for flag --pid`,
		},
		{
			name: "a feature value",
			run: func() error {
				return conga.New[lineOneFeatureCLI]().EnableFeatures().Args([]string{"--feature-gates", "level=01"}).RunContext(t.Context())
			},
			expected: `invalid value "01" for feature "level"`,
		},
		{
			name:     "a schema",
			run:      func() error { return conga.New[lineOneSchemaCLI]().Args(nil).RunContext(t.Context()) },
			expected: `struct lineOneSchemaCLI field Port: unknown struct tag "bogus"`,
		},
		{
			name: "a strict config document",
			run: func() error {
				return conga.New[lineOneConfigCLI]().
					Name("app").
					RegisterDecoder(".json", json.Decode).
					ConfigOptions(conga.ConfigExplicit(configPath)).
					Args(nil).
					RunContext(t.Context())
			},
			expected: `unknown field "prot" in command "app"`,
		},
		{
			name: "a .env file",
			run: func() error {
				return conga.New[lineOneConfigCLI]().Dotenv(missingEnv).Args(nil).RunContext(t.Context())
			},
			expected: fmt.Sprintf("cannot read .env file %q: open %s: no such file or directory", missingEnv, missingEnv),
		},
		{
			name: "a missing config decoder",
			run: func() error {
				return conga.New[configFlagCLI]().Args([]string{"--config", "x.json"}).RunContext(t.Context())
			},
			expected: `flag --config requires a registered config decoder`,
		},
		{
			name: "an unknown shell",
			run: func() error {
				return conga.New[outputSubcommandCLI]().EnableCompletion().Args([]string{"completion", "bogus"}).RunContext(t.Context())
			},
			expected: `unknown shell "bogus"`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for name, value := range testCase.environ {
				t.Setenv(name, value)
			}

			err := testCase.run()

			assert.Error(t, err)
			assert.Equal(t, testCase.expected, err.Error())
			assert.False(t, strings.Contains(err.Error(), "\n"))

			resolved := conga.DiagnosticOf(err)

			assert.Equal(t, testCase.expected, resolved.Message())

			// The pieces exist, so their absence from line 1 means something. A
			// .env read failure names its file on line 1 and has none.
			hasPieces := resolved.Note() != "" || resolved.Hint() != "" || len(resolved.Origins()) > 0

			assert.Equal(t, testCase.name != "a .env file", hasPieces)
		})
	}
}
