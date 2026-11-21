package envx_test

import (
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/envx"
)

const testVariable = "CONGA_ENVX_TEST_SWITCH"

func TestEnvBool(t *testing.T) {
	t.Run("unset is not ok", func(t *testing.T) {
		value, ok := envx.EnvBool(testVariable)

		assert.False(t, value)
		assert.False(t, ok)
	})

	cases := []struct {
		raw   string
		value bool
		ok    bool
	}{
		{raw: ""},
		{raw: "1", value: true, ok: true},
		{raw: "t", value: true, ok: true},
		{raw: "T", value: true, ok: true},
		{raw: "TRUE", value: true, ok: true},
		{raw: "true", value: true, ok: true},
		{raw: "True", value: true, ok: true},
		{raw: "0", ok: true},
		{raw: "f", ok: true},
		{raw: "F", ok: true},
		{raw: "FALSE", ok: true},
		{raw: "false", ok: true},
		{raw: "False", ok: true},
		{raw: "on"},
		{raw: "off"},
		{raw: "yes"},
		{raw: "no"},
		{raw: "maybe"},
		{raw: "always"},
		{raw: " 1"},
	}

	for _, testCase := range cases {
		t.Run(strconv.Quote(testCase.raw), func(t *testing.T) {
			t.Setenv(testVariable, testCase.raw)

			value, ok := envx.EnvBool(testVariable)

			assert.Equal(t, testCase.value, value)
			assert.Equal(t, testCase.ok, ok)
		})
	}
}

func TestColorPreference(t *testing.T) {
	cases := []struct {
		name     string
		color    string
		noColor  string
		cliColor string
		expected envx.Color
	}{
		{name: "no preference", expected: envx.ColorDefault},
		{name: "CONGA_COLOR true forces color", color: "true", expected: envx.ColorOn},
		{name: "CONGA_COLOR 0 disables color", color: "0", expected: envx.ColorOff},
		{name: "CONGA_COLOR true outranks NO_COLOR", color: "true", noColor: "1", expected: envx.ColorOn},
		{name: "CONGA_COLOR 1 outranks CLICOLOR 0", color: "1", cliColor: "0", expected: envx.ColorOn},
		{name: "unrecognised CONGA_COLOR states no preference", color: "yes", expected: envx.ColorDefault},
		{name: "unrecognised CONGA_COLOR defers to NO_COLOR", color: "always", noColor: "1", expected: envx.ColorOff},
		{name: "NO_COLOR disables color", noColor: "1", expected: envx.ColorOff},
		{name: "CLICOLOR 0 disables color", cliColor: "0", expected: envx.ColorOff},
		{name: "CLICOLOR 1 states no preference", cliColor: "1", expected: envx.ColorDefault},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(envx.EnvColor, testCase.color)
			t.Setenv(envx.EnvNoColor, testCase.noColor)
			t.Setenv(envx.EnvCLIColor, testCase.cliColor)

			assert.Equal(t, testCase.expected, envx.ColorPreference())
		})
	}
}
