package envx_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/envx"
)

func TestColorEnabled(t *testing.T) {
	cases := []struct {
		name     string
		color    string
		noColor  string
		cliColor string
		term     string
		pipe     bool
		enabled  bool
	}{
		{name: "terminal with no preference", term: "xterm", enabled: true},
		{name: "pipe with no preference", term: "xterm", pipe: true},
		{name: "empty NO_COLOR is ignored", term: "xterm", enabled: true},
		{name: "NO_COLOR disables on a terminal", term: "xterm", noColor: "1"},
		{name: "CLICOLOR 0 disables on a terminal", term: "xterm", cliColor: "0"},
		{name: "TERM dumb disables on a terminal", term: "dumb"},
		{name: "CONGA_COLOR true forces a pipe", color: "true", term: "xterm", pipe: true, enabled: true},
		{name: "CONGA_COLOR false disables a terminal", color: "false", term: "xterm"},
		{name: "CONGA_COLOR true outranks NO_COLOR", color: "true", noColor: "1", term: "xterm", pipe: true, enabled: true},
		{name: "CONGA_COLOR true outranks CLICOLOR 0", color: "true", cliColor: "0", term: "xterm", pipe: true, enabled: true},
		{name: "CONGA_COLOR true outranks TERM dumb", color: "true", term: "dumb", pipe: true, enabled: true},
		{name: "unparseable CONGA_COLOR follows the terminal", color: "always", term: "xterm", enabled: true},
		{name: "unparseable CONGA_COLOR follows the pipe", color: "always", term: "xterm", pipe: true},
		{name: "unparseable CONGA_COLOR defers to NO_COLOR", color: "always", noColor: "1", term: "xterm"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(envx.EnvColor, testCase.color)
			t.Setenv(envx.EnvNoColor, testCase.noColor)
			t.Setenv(envx.EnvCLIColor, testCase.cliColor)
			t.Setenv("TERM", testCase.term)

			var writer io.Writer = &bytes.Buffer{}
			if !testCase.pipe {
				writer = openTerminal(t)
			}

			assert.Equal(t, testCase.enabled, envx.ColorEnabled(writer))
		})
	}
}

func TestColorEnabled_DevNullIsNotATerminal(t *testing.T) {
	t.Setenv(envx.EnvColor, "")
	t.Setenv(envx.EnvNoColor, "")
	t.Setenv(envx.EnvCLIColor, "")
	t.Setenv("TERM", "xterm")

	null, err := os.Open(os.DevNull)
	assert.NoError(t, err)

	defer null.Close()

	assert.False(t, envx.ColorEnabled(null))
}

func TestColorEnabled_ForcedColorOnNull(t *testing.T) {
	t.Setenv(envx.EnvColor, "1")

	null, err := os.Open(os.DevNull)
	assert.NoError(t, err)

	defer null.Close()

	assert.True(t, envx.ColorEnabled(null))
}
