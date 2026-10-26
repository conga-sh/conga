package wire_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
)

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
		enabled := wire.AliasesEnabled()

		assert.True(t, enabled)
	})

	t.Run("suppressed with 0", func(t *testing.T) {
		t.Setenv(wire.EnvAliases, "0")

		enabled := wire.AliasesEnabled()

		assert.False(t, enabled)
	})

	t.Run("suppressed with false", func(t *testing.T) {
		t.Setenv(wire.EnvAliases, "false")

		enabled := wire.AliasesEnabled()

		assert.False(t, enabled)
	})

	t.Run("enabled with 1", func(t *testing.T) {
		t.Setenv(wire.EnvAliases, "1")

		enabled := wire.AliasesEnabled()

		assert.True(t, enabled)
	})

	t.Run("unparseable value defaults to true", func(t *testing.T) {
		t.Setenv(wire.EnvAliases, "garbage")

		enabled := wire.AliasesEnabled()

		assert.True(t, enabled)
	})
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestDescriptionsEnabled(t *testing.T) {
	t.Run("default is true when unset", func(t *testing.T) {
		enabled := wire.DescriptionsEnabled()

		assert.True(t, enabled)
	})

	t.Run("suppressed with 0", func(t *testing.T) {
		t.Setenv(wire.EnvDescriptions, "0")

		enabled := wire.DescriptionsEnabled()

		assert.False(t, enabled)
	})

	t.Run("suppressed with false", func(t *testing.T) {
		t.Setenv(wire.EnvDescriptions, "false")

		enabled := wire.DescriptionsEnabled()

		assert.False(t, enabled)
	})

	t.Run("enabled with 1", func(t *testing.T) {
		t.Setenv(wire.EnvDescriptions, "1")

		enabled := wire.DescriptionsEnabled()

		assert.True(t, enabled)
	})

	t.Run("unparseable value defaults to true", func(t *testing.T) {
		t.Setenv(wire.EnvDescriptions, "garbage")

		enabled := wire.DescriptionsEnabled()

		assert.True(t, enabled)
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

	t.Run("enabled and explicit with 1", func(t *testing.T) {
		t.Setenv(wire.EnvMergeFlags, "1")

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

	t.Run("unparseable value defaults to disabled", func(t *testing.T) {
		t.Setenv(wire.EnvMergeFlags, "garbage")

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
		enabled := wire.Tooltip()

		assert.False(t, enabled)
	})

	t.Run("enabled with true", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "true")

		enabled := wire.Tooltip()

		assert.True(t, enabled)
	})

	t.Run("enabled with 1", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "1")

		enabled := wire.Tooltip()

		assert.True(t, enabled)
	})

	t.Run("enabled with yes", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "yes")

		enabled := wire.Tooltip()

		assert.True(t, enabled)
	})

	t.Run("enabled with on", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "on")

		enabled := wire.Tooltip()

		assert.True(t, enabled)
	})

	t.Run("enabled case-insensitively with TRUE", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "TRUE")

		enabled := wire.Tooltip()

		assert.True(t, enabled)
	})

	t.Run("enabled case-insensitively with Yes", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "Yes")

		enabled := wire.Tooltip()

		assert.True(t, enabled)
	})

	t.Run("unparseable value defaults to false", func(t *testing.T) {
		t.Setenv(wire.EnvTooltip, "garbage")

		enabled := wire.Tooltip()

		assert.False(t, enabled)
	})
}

//nolint:paralleltest // t.Setenv cannot be used with t.Parallel
func TestColorDisabled(t *testing.T) {
	t.Run("default is false when unset", func(t *testing.T) {
		disabled := wire.ColorDisabled()

		assert.False(t, disabled)
	})

	t.Run("disabled with CONGA_COLOR 0", func(t *testing.T) {
		t.Setenv(wire.EnvColor, "0")

		disabled := wire.ColorDisabled()

		assert.True(t, disabled)
	})

	t.Run("explicit CONGA_COLOR overrides NO_COLOR", func(t *testing.T) {
		t.Setenv(wire.EnvColor, "1")
		t.Setenv(wire.EnvNoColor, "1")

		disabled := wire.ColorDisabled()

		assert.False(t, disabled)
	})

	t.Run("NO_COLOR disables when CONGA_COLOR is empty", func(t *testing.T) {
		t.Setenv(wire.EnvColor, "")
		t.Setenv(wire.EnvNoColor, "1")

		disabled := wire.ColorDisabled()

		assert.True(t, disabled)
	})

	t.Run("empty NO_COLOR is ignored", func(t *testing.T) {
		t.Setenv(wire.EnvColor, "")
		t.Setenv(wire.EnvNoColor, "")

		disabled := wire.ColorDisabled()

		assert.False(t, disabled)
	})

	t.Run("disabled with CONGA_CLI_COLOR 0", func(t *testing.T) {
		t.Setenv(wire.EnvCLIColor, "0")

		disabled := wire.ColorDisabled()

		assert.True(t, disabled)
	})

	t.Run("enabled with CONGA_CLI_COLOR 1", func(t *testing.T) {
		t.Setenv(wire.EnvCLIColor, "1")

		disabled := wire.ColorDisabled()

		assert.False(t, disabled)
	})
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
