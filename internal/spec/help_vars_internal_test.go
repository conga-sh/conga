package spec

import (
	"reflect"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestInterpolateHelp(t *testing.T) {
	t.Parallel()

	values := helpVarValues{
		rangeText:   "1..10",
		lenText:     "3..20",
		itemsText:   "2..3",
		enumText:    "dev, prod",
		defaultText: "8080",
		envText:     "APP_PORT",
	}

	testCases := []struct {
		name       string
		helpText   string
		expected   string
		referenced helpVars
	}{
		{
			name:       "range variable",
			helpText:   "Port ({{range}})",
			expected:   "Port (1..10)",
			referenced: helpVars{Range: true},
		},
		{
			name:       "len variable",
			helpText:   "User ({{len}})",
			expected:   "User (3..20)",
			referenced: helpVars{Len: true},
		},
		{
			name:       "items variable",
			helpText:   "Tags ({{items}})",
			expected:   "Tags (2..3)",
			referenced: helpVars{Items: true},
		},
		{
			name:       "enum variable",
			helpText:   "Env ({{enum}})",
			expected:   "Env (dev, prod)",
			referenced: helpVars{Enum: true},
		},
		{
			name:       "default variable",
			helpText:   "Port defaults to {{default}}",
			expected:   "Port defaults to 8080",
			referenced: helpVars{Default: true},
		},
		{
			name:       "env variable",
			helpText:   "Reads {{env}}",
			expected:   "Reads APP_PORT",
			referenced: helpVars{Env: true},
		},
		{
			name:       "mixed variables",
			helpText:   "Port ({{range}}) from {{env}}, env ({{enum}})",
			expected:   "Port (1..10) from APP_PORT, env (dev, prod)",
			referenced: helpVars{Range: true, Enum: true, Env: true},
		},
		{
			name:       "repeated reference",
			helpText:   "{{range}} then {{range}}",
			expected:   "1..10 then 1..10",
			referenced: helpVars{Range: true},
		},
		{
			name:       "whitespace inside braces trimmed",
			helpText:   "Port ({{ range }})",
			expected:   "Port (1..10)",
			referenced: helpVars{Range: true},
		},
		{
			name:       "escaped opening braces",
			helpText:   "{{{{range}}",
			expected:   "{{range}}",
			referenced: helpVars{},
		},
		{
			name:       "escaped braces alongside reference",
			helpText:   "Use {{{{ for literal, {{range}} for bounds",
			expected:   "Use {{ for literal, 1..10 for bounds",
			referenced: helpVars{Range: true},
		},
		{
			name:       "shell prose copied verbatim",
			helpText:   "Expands ${HOME} in the path",
			expected:   "Expands ${HOME} in the path",
			referenced: helpVars{},
		},
		{
			name:       "dollar copied verbatim",
			helpText:   "Cost $$5",
			expected:   "Cost $$5",
			referenced: helpVars{},
		},
		{
			name:       "unterminated reference copied verbatim",
			helpText:   "{{range",
			expected:   "{{range",
			referenced: helpVars{},
		},
		{
			name:       "single brace copied verbatim",
			helpText:   "Use {x} notation",
			expected:   "Use {x} notation",
			referenced: helpVars{},
		},
		{
			name:       "no reference returns input unchanged",
			helpText:   "Plain help text",
			expected:   "Plain help text",
			referenced: helpVars{},
		},
		{
			name:       "empty help text",
			helpText:   "",
			expected:   "",
			referenced: helpVars{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			expanded, referenced, err := interpolateHelp(testCase.helpText, values)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expected, expanded)
			assert.Equal(t, testCase.referenced, referenced)
		})
	}
}

func TestInterpolateHelp_UnknownVariable(t *testing.T) {
	t.Parallel()

	expanded, referenced, err := interpolateHelp("Port ({{rnage}})", helpVarValues{rangeText: "1..10"})

	assert.Error(t, err)
	assert.IsError(t, err, ErrInvalidStructTag)
	assert.Equal(t, `schema error: invalid struct tag: tag "help" references unknown variable {{rnage}}; supported variables are {{range}}, {{len}}, {{items}}, {{enum}}, {{default}}, {{env}}`, err.Error())
	assert.Equal(t, "", expanded)
	assert.Equal(t, helpVars{}, referenced)
}

// TestInterpolateHelp_MalformedReferenceRejected verifies that any terminated
// {{...}} reference is a variable reference, so a malformed name is an error
// rather than silent literal text.
func TestInterpolateHelp_MalformedReferenceRejected(t *testing.T) {
	t.Parallel()

	for _, helpText := range []string{"Port ({{ra-nge}})", "Port ({{default:8080}})", "Port ({{}})"} {
		_, _, err := interpolateHelp(helpText, helpVarValues{rangeText: "1..10"})

		assert.Error(t, err)
		assert.IsError(t, err, ErrInvalidStructTag)
	}
}

// TestInterpolateHelp_RecordsEmptyReference verifies that a reference to a
// variable with no declared value is still recorded, so the availability check
// can tell it apart from an unreferenced variable.
func TestInterpolateHelp_RecordsEmptyReference(t *testing.T) {
	t.Parallel()

	expanded, referenced, err := interpolateHelp("Port ({{range}})", helpVarValues{})

	assert.NoError(t, err)
	assert.Equal(t, "Port ()", expanded)
	assert.Equal(t, helpVars{Range: true}, referenced)
}

func TestValidateHelpVariables(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		helpText string
		declared helpVars
		expected string
	}{
		{
			name:     "every reference declared",
			helpText: "Port ({{range}}) env ({{enum}})",
			declared: helpVars{Range: true, Enum: true},
		},
		{
			name:     "no references",
			helpText: "Plain help",
			declared: helpVars{},
		},
		{
			name:     "declared but unreferenced is fine",
			helpText: "Plain help",
			declared: helpVars{Range: true, Env: true},
		},
		{
			name:     "undeclared range",
			helpText: "Port ({{range}})",
			declared: helpVars{},
			expected: `schema error: invalid struct tag: tag "help" references {{range}}, but the field provides no value for that variable`,
		},
		{
			name:     "undeclared env",
			helpText: "Reads {{env}}",
			declared: helpVars{},
			expected: `schema error: invalid struct tag: tag "help" references {{env}}, but the field provides no value for that variable`,
		},
		{
			name:     "fixed priority order decides the reported variable",
			helpText: "{{range}} {{env}}",
			declared: helpVars{},
			expected: `schema error: invalid struct tag: tag "help" references {{range}}, but the field provides no value for that variable`,
		},
		{
			name:     "unknown variable",
			helpText: "{{rnage}}",
			declared: helpVars{},
			expected: `schema error: invalid struct tag: tag "help" references unknown variable {{rnage}}; supported variables are {{range}}, {{len}}, {{items}}, {{enum}}, {{default}}, {{env}}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := validateHelpVariables(testCase.helpText, testCase.declared)

			if testCase.expected == "" {
				assert.NoError(t, err)

				return
			}

			assert.Error(t, err)
			assert.IsError(t, err, ErrInvalidStructTag)
			assert.Equal(t, testCase.expected, err.Error())
		})
	}
}

func TestHelpVarValuesFor(t *testing.T) {
	t.Parallel()

	t.Run("all values populated", func(t *testing.T) {
		t.Parallel()

		valueRange := &Range{HasMin: true, HasMax: true, MinStr: "1", MaxStr: "10"}
		lengthRange := &Range{Exact: true, MinStr: "5"}
		itemsRange := &Range{HasMin: true, MinStr: "2"}

		values := helpVarValuesFor(valueRange, lengthRange, itemsRange, []string{"dev", "prod"}, "8080", []string{"APP_PORT", "PORT"})

		assert.Equal(t, helpVarValues{
			rangeText:   "1..10",
			lenText:     "5",
			itemsText:   ">= 2",
			enumText:    "dev, prod",
			defaultText: "8080",
			envText:     "APP_PORT, PORT",
		}, values)
	})

	t.Run("absent values stay empty", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, helpVarValues{}, helpVarValuesFor(nil, nil, nil, nil, "", nil))
	})
}

func TestHelpVarAvailability(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		tag            reflect.StructTag
		role           string
		envPrefix      string
		envPrefixKnown bool
		expected       helpVars
	}{
		{
			name:           "constraint tags declared",
			tag:            `range:"1..10" len:"2..4" items:"1..3" enum:"a,b" default:"x"`,
			role:           "flag",
			envPrefixKnown: true,
			expected:       helpVars{Range: true, Len: true, Items: true, Enum: true, Default: true},
		},
		{
			name:           "nothing declared",
			tag:            `help:"plain"`,
			role:           "flag",
			envPrefixKnown: true,
			expected:       helpVars{},
		},
		{
			name:           "empty constraint tags stay unavailable",
			tag:            `range:"" len:"" items:"" enum:""`,
			role:           "flag",
			envPrefixKnown: true,
			expected:       helpVars{},
		},
		{
			name:           "comma-only enum stays unavailable",
			tag:            `enum:","`,
			role:           "flag",
			envPrefixKnown: true,
			expected:       helpVars{},
		},
		{
			name:           "explicit empty default stays available",
			tag:            `default:""`,
			role:           "flag",
			envPrefixKnown: true,
			expected:       helpVars{Default: true},
		},
		{
			name:           "config file flag only exposes env",
			tag:            `range:"1..5" enum:"a,b" default:"x"`,
			role:           "config file flag",
			envPrefix:      "APP_",
			envPrefixKnown: true,
			expected:       helpVars{Env: true},
		},
		{
			name:           "env tag declares env",
			tag:            `env:"APP_PORT"`,
			role:           "flag",
			envPrefixKnown: true,
			expected:       helpVars{Env: true},
		},
		{
			name:           "env prefix declares env",
			tag:            `help:"plain"`,
			role:           "flag",
			envPrefix:      "APP_",
			envPrefixKnown: true,
			expected:       helpVars{Env: true},
		},
		{
			name:           "unresolved env prefix declares env",
			tag:            `help:"plain"`,
			role:           "flag",
			envPrefixKnown: false,
			expected:       helpVars{Env: true},
		},
		{
			name:           "config file flag binds env",
			tag:            `help:"plain"`,
			role:           "config file flag",
			envPrefix:      "APP_",
			envPrefixKnown: true,
			expected:       helpVars{Env: true},
		},
		{
			name:           "positional argument never binds env",
			tag:            `help:"plain"`,
			role:           "positional argument",
			envPrefix:      "APP_",
			envPrefixKnown: true,
			expected:       helpVars{},
		},
		{
			name:           "features container binds env through explicit names",
			tag:            `env:"APP_FEATURES"`,
			role:           "features",
			envPrefixKnown: true,
			expected:       helpVars{Enum: true, Env: true},
		},
		{
			name:           "features container ignores the application env prefix",
			tag:            `help:"plain"`,
			role:           "features",
			envPrefix:      "APP_",
			envPrefixKnown: true,
			expected:       helpVars{Enum: true},
		},
		{
			name:           "features container derives env from its envprefix tag",
			tag:            `envprefix:"APP_X_"`,
			role:           "features",
			envPrefixKnown: true,
			expected:       helpVars{Enum: true, Env: true},
		},
		{
			name:           "features container with excluded combined env keeps envprefix",
			tag:            `env:"-" envprefix:"APP_X_"`,
			role:           "features",
			envPrefixKnown: true,
			expected:       helpVars{Enum: true, Env: true},
		},
		{
			name:           "features container with excluded env",
			tag:            `env:"-"`,
			role:           "features",
			envPrefix:      "APP_",
			envPrefixKnown: true,
			expected:       helpVars{Enum: true},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, helpVarAvailability(testCase.tag, testCase.role, testCase.envPrefix, testCase.envPrefixKnown))
		})
	}
}

// TestEnvVarsDeclared_MatchesResolveFlagEnvVars pins the analyzer-facing
// predicate to the runtime resolver it mirrors, so the two cannot drift.
func TestEnvVarsDeclared_MatchesResolveFlagEnvVars(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		tag       reflect.StructTag
		envPrefix string
	}{
		{name: "named env", tag: `env:"APP_PORT"`, envPrefix: ""},
		{name: "named env with prefix", tag: `env:"APP_PORT"`, envPrefix: "APP_"},
		{name: "disabled env", tag: `env:"-"`, envPrefix: "APP_"},
		{name: "empty env element", tag: `env:","`, envPrefix: ""},
		{name: "empty env element with prefix", tag: `env:","`, envPrefix: "APP_"},
		{name: "absent env with prefix", tag: `help:"plain"`, envPrefix: "APP_"},
		{name: "absent env without prefix", tag: `help:"plain"`, envPrefix: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			envVars, _ := ResolveFlagEnvVars(testCase.tag, "port", testCase.envPrefix)

			assert.Equal(t, len(envVars) > 0, envVarsDeclared(testCase.tag, "flag", testCase.envPrefix, true))
		})
	}
}

func TestRange_HintText(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		valueRange *Range
		expected   string
	}{
		{name: "nil range", valueRange: nil, expected: ""},
		{name: "no bounds", valueRange: &Range{}, expected: ""},
		{name: "exact value", valueRange: &Range{Exact: true, MinStr: "5"}, expected: "5"},
		{name: "min and max", valueRange: &Range{HasMin: true, HasMax: true, MinStr: "1", MaxStr: "10"}, expected: "1..10"},
		{name: "min only", valueRange: &Range{HasMin: true, MinStr: "1"}, expected: ">= 1"},
		{name: "max only", valueRange: &Range{HasMax: true, MaxStr: "10"}, expected: "<= 10"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.expected, testCase.valueRange.HintText())
		})
	}
}
