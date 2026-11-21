package wire_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
)

const testEnvPrefix = "TEST_"

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestShell(t *testing.T) {
	t.Run("default is empty when unset", func(t *testing.T) {
		shellName := wire.Shell()

		assert.Equal(t, "", shellName)
	})

	t.Run("shell name from environment", func(t *testing.T) {
		t.Setenv(wire.EnvShell, "zsh")

		shellName := wire.Shell()

		assert.Equal(t, "zsh", shellName)
	})
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestAliasesEnabled(t *testing.T) {
	t.Run("default is true when unset", func(t *testing.T) {
		assert.True(t, wire.AliasesEnabled(testEnvPrefix))
	})

	t.Run("shown with true", func(t *testing.T) {
		t.Setenv(testEnvPrefix+wire.CompletionAliasesSuffix, "true")

		assert.True(t, wire.AliasesEnabled(testEnvPrefix))
	})

	t.Run("suppressed with false", func(t *testing.T) {
		t.Setenv(testEnvPrefix+wire.CompletionAliasesSuffix, "false")

		assert.False(t, wire.AliasesEnabled(testEnvPrefix))
	})

	t.Run("unparseable yes keeps the default", func(t *testing.T) {
		t.Setenv(testEnvPrefix+wire.CompletionAliasesSuffix, "yes")

		assert.True(t, wire.AliasesEnabled(testEnvPrefix))
	})

	t.Run("legacy framework-prefixed name is ignored", func(t *testing.T) {
		t.Setenv("CONGA_COMPLETION_ALIASES", "0")

		assert.True(t, wire.AliasesEnabled(testEnvPrefix))
	})

	t.Run("empty prefix does not read unprefixed name", func(t *testing.T) {
		t.Setenv("COMPLETION_ALIASES", "0")

		assert.True(t, wire.AliasesEnabled(""))
	})
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestDescriptionsEnabled(t *testing.T) {
	t.Run("default is true when unset", func(t *testing.T) {
		assert.True(t, wire.DescriptionsEnabled(testEnvPrefix))
	})

	t.Run("shown with true", func(t *testing.T) {
		t.Setenv(testEnvPrefix+wire.CompletionDescriptionsSuffix, "true")

		assert.True(t, wire.DescriptionsEnabled(testEnvPrefix))
	})

	t.Run("suppressed with false", func(t *testing.T) {
		t.Setenv(testEnvPrefix+wire.CompletionDescriptionsSuffix, "false")

		assert.False(t, wire.DescriptionsEnabled(testEnvPrefix))
	})

	t.Run("unparseable yes keeps the default", func(t *testing.T) {
		t.Setenv(testEnvPrefix+wire.CompletionDescriptionsSuffix, "yes")

		assert.True(t, wire.DescriptionsEnabled(testEnvPrefix))
	})

	t.Run("legacy framework-prefixed name is ignored", func(t *testing.T) {
		t.Setenv("CONGA_COMPLETION_DESCRIPTIONS", "0")

		assert.True(t, wire.DescriptionsEnabled(testEnvPrefix))
	})

	t.Run("empty prefix does not read unprefixed name", func(t *testing.T) {
		t.Setenv("COMPLETION_DESCRIPTIONS", "0")

		assert.True(t, wire.DescriptionsEnabled(""))
	})
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestBareSpaceFlagsEnabled(t *testing.T) {
	t.Run("default is false when unset", func(t *testing.T) {
		assert.False(t, wire.BareSpaceFlagsEnabled(testEnvPrefix))
	})

	t.Run("restored with true", func(t *testing.T) {
		t.Setenv(testEnvPrefix+wire.CompletionBareSpaceFlagsSuffix, "true")

		assert.True(t, wire.BareSpaceFlagsEnabled(testEnvPrefix))
	})

	t.Run("suppressed with false", func(t *testing.T) {
		t.Setenv(testEnvPrefix+wire.CompletionBareSpaceFlagsSuffix, "false")

		assert.False(t, wire.BareSpaceFlagsEnabled(testEnvPrefix))
	})

	t.Run("unparseable yes keeps the default", func(t *testing.T) {
		t.Setenv(testEnvPrefix+wire.CompletionBareSpaceFlagsSuffix, "yes")

		assert.False(t, wire.BareSpaceFlagsEnabled(testEnvPrefix))
	})

	t.Run("legacy framework-prefixed name is ignored", func(t *testing.T) {
		t.Setenv("CONGA_COMPLETION_BARE_SPACE_FLAGS", "1")

		assert.False(t, wire.BareSpaceFlagsEnabled(testEnvPrefix))
	})

	t.Run("empty prefix does not read unprefixed name", func(t *testing.T) {
		t.Setenv("COMPLETION_BARE_SPACE_FLAGS", "1")

		assert.False(t, wire.BareSpaceFlagsEnabled(""))
	})
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestMergeFlags(t *testing.T) {
	t.Run("default is disabled and not explicit when unset", func(t *testing.T) {
		enabled, explicit := wire.MergeFlags()

		assert.False(t, enabled)
		assert.False(t, explicit)
	})

	t.Run("enabled and explicit with true", func(t *testing.T) {
		t.Setenv(wire.EnvMergeFlags, "true")

		enabled, explicit := wire.MergeFlags()

		assert.True(t, enabled)
		assert.True(t, explicit)
	})

	t.Run("disabled but explicit with false", func(t *testing.T) {
		t.Setenv(wire.EnvMergeFlags, "false")

		enabled, explicit := wire.MergeFlags()

		assert.False(t, enabled)
		assert.True(t, explicit)
	})

	t.Run("unparseable yes is not explicit", func(t *testing.T) {
		t.Setenv(wire.EnvMergeFlags, "yes")

		enabled, explicit := wire.MergeFlags()

		assert.False(t, enabled)
		assert.False(t, explicit)
	})
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestDescriptionLength(t *testing.T) {
	t.Run("default is 80 when unset", func(t *testing.T) {
		length := wire.DescriptionLength()

		assert.Equal(t, 80, length)
	})

	t.Run("custom length", func(t *testing.T) {
		t.Setenv(wire.EnvDescriptionLength, "40")

		length := wire.DescriptionLength()

		assert.Equal(t, 40, length)
	})

	t.Run("zero falls back to default", func(t *testing.T) {
		t.Setenv(wire.EnvDescriptionLength, "0")

		length := wire.DescriptionLength()

		assert.Equal(t, 80, length)
	})

	t.Run("negative falls back to default", func(t *testing.T) {
		t.Setenv(wire.EnvDescriptionLength, "-5")

		length := wire.DescriptionLength()

		assert.Equal(t, 80, length)
	})

	t.Run("unparseable value defaults to 80", func(t *testing.T) {
		t.Setenv(wire.EnvDescriptionLength, "abc")

		length := wire.DescriptionLength()

		assert.Equal(t, 80, length)
	})
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestTooltip(t *testing.T) {
	t.Run("default is false when unset", func(t *testing.T) {
		assert.False(t, wire.Tooltip())
	})

	t.Run("enabled with true", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "true")

		assert.True(t, wire.Tooltip())
	})

	t.Run("disabled with false", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "false")

		assert.False(t, wire.Tooltip())
	})

	t.Run("unparseable yes keeps the default", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "yes")

		assert.False(t, wire.Tooltip())
	})
}

func TestColorDisabled(t *testing.T) {
	cases := []struct {
		name     string
		color    string
		noColor  string
		cliColor string
		term     string
		disabled bool
	}{
		{name: "no preference keeps color"},
		{name: "CONGA_COLOR true forces color", color: "true"},
		{name: "CONGA_COLOR false disables color", color: "false", disabled: true},
		{name: "unparseable CONGA_COLOR keeps color", color: "always"},
		{name: "CONGA_COLOR true outranks NO_COLOR", color: "true", noColor: "1"},
		{name: "CONGA_COLOR false with NO_COLOR disables color", color: "false", noColor: "1", disabled: true},
		{name: "unparseable CONGA_COLOR defers to NO_COLOR", color: "always", noColor: "1", disabled: true},
		{name: "CONGA_COLOR true outranks CLICOLOR 0", color: "true", cliColor: "0"},
		{name: "CONGA_COLOR false with CLICOLOR 0 disables color", color: "false", cliColor: "0", disabled: true},
		{name: "unparseable CONGA_COLOR defers to CLICOLOR 0", color: "always", cliColor: "0", disabled: true},
		{name: "CONGA_COLOR true ignores TERM dumb", color: "true", term: "dumb"},
		{name: "TERM dumb alone keeps color", term: "dumb"},
		{name: "CLICOLOR 1 keeps color", cliColor: "1"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(wire.EnvColor, testCase.color)
			t.Setenv(wire.EnvNoColor, testCase.noColor)
			t.Setenv(wire.EnvCLIColor, testCase.cliColor)
			t.Setenv("TERM", testCase.term)

			assert.Equal(t, testCase.disabled, wire.ColorDisabled())
		})
	}
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestZshStyleLimit(t *testing.T) {
	t.Run("default is 300 when unset", func(t *testing.T) {
		limit := wire.ZshStyleLimit()

		assert.Equal(t, 300, limit)
	})

	t.Run("custom limit", func(t *testing.T) {
		t.Setenv(wire.EnvZshStyleLimit, "50")

		limit := wire.ZshStyleLimit()

		assert.Equal(t, 50, limit)
	})

	t.Run("unparseable value defaults to 300", func(t *testing.T) {
		t.Setenv(wire.EnvZshStyleLimit, "abc")

		limit := wire.ZshStyleLimit()

		assert.Equal(t, 300, limit)
	})
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestZshListSeparator(t *testing.T) {
	t.Run("default is two hyphens when unset", func(t *testing.T) {
		separator := wire.ZshListSeparator()

		assert.Equal(t, "--", separator)
	})

	t.Run("custom separator", func(t *testing.T) {
		t.Setenv(wire.EnvZshListSeparator, "=>")

		separator := wire.ZshListSeparator()

		assert.Equal(t, "=>", separator)
	})

	t.Run("empty value defaults to two hyphens", func(t *testing.T) {
		t.Setenv(wire.EnvZshListSeparator, "")

		separator := wire.ZshListSeparator()

		assert.Equal(t, "--", separator)
	})

	t.Run("colon value defaults to two hyphens", func(t *testing.T) {
		t.Setenv(wire.EnvZshListSeparator, "a:b")

		separator := wire.ZshListSeparator()

		assert.Equal(t, "--", separator)
	})

	t.Run("control character defaults to two hyphens", func(t *testing.T) {
		t.Setenv(wire.EnvZshListSeparator, "a\x1fb")

		separator := wire.ZshListSeparator()

		assert.Equal(t, "--", separator)
	})

	t.Run("oversized value defaults to two hyphens", func(t *testing.T) {
		t.Setenv(wire.EnvZshListSeparator, strings.Repeat("x", 33))

		separator := wire.ZshListSeparator()

		assert.Equal(t, "--", separator)
	})
}
