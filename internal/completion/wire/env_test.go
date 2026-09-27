package wire_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/completion/wire"
)

func TestShell(t *testing.T) {
	assert.Equal(t, "", wire.Shell())

	t.Setenv(wire.EnvShell, "zsh")

	assert.Equal(t, "zsh", wire.Shell())
}

func TestMergeFlags(t *testing.T) {
	enabled, explicit := wire.MergeFlags()

	assert.False(t, enabled)
	assert.False(t, explicit)

	t.Setenv(wire.EnvMergeFlags, "true")

	enabled, explicit = wire.MergeFlags()

	assert.True(t, enabled)
	assert.True(t, explicit)

	t.Setenv(wire.EnvMergeFlags, "1")

	enabled, explicit = wire.MergeFlags()

	assert.True(t, enabled)
	assert.True(t, explicit)

	t.Setenv(wire.EnvMergeFlags, "false")

	enabled, explicit = wire.MergeFlags()

	assert.False(t, enabled)
	assert.True(t, explicit)

	t.Setenv(wire.EnvMergeFlags, "garbage")

	enabled, explicit = wire.MergeFlags()

	assert.False(t, enabled)
	assert.False(t, explicit)
}

func TestDescriptionLength(t *testing.T) {
	assert.Equal(t, 80, wire.DescriptionLength())

	t.Setenv(wire.EnvDescriptionLength, "40")

	assert.Equal(t, 40, wire.DescriptionLength())

	t.Setenv(wire.EnvDescriptionLength, "0")

	assert.Equal(t, 80, wire.DescriptionLength())

	t.Setenv(wire.EnvDescriptionLength, "-5")

	assert.Equal(t, 80, wire.DescriptionLength())

	t.Setenv(wire.EnvDescriptionLength, "abc")

	assert.Equal(t, 80, wire.DescriptionLength())
}

func TestTooltip(t *testing.T) {
	assert.False(t, wire.Tooltip())

	for _, value := range []string{"true", "1", "yes", "on", "TRUE", "Yes"} {
		t.Setenv(wire.EnvTooltip, value)

		assert.True(t, wire.Tooltip(), "value %q must enable tooltips", value)
	}

	t.Setenv(wire.EnvTooltip, "garbage")

	assert.False(t, wire.Tooltip())
}

func TestColorDisabled(t *testing.T) {
	assert.False(t, wire.ColorDisabled())

	t.Setenv(wire.EnvColor, "0")

	assert.True(t, wire.ColorDisabled())

	t.Setenv(wire.EnvColor, "1")
	t.Setenv(wire.EnvNoColor, "1")

	assert.False(t, wire.ColorDisabled(), "explicit CONGA_COLOR must override NO_COLOR")

	t.Setenv(wire.EnvColor, "")
	t.Setenv(wire.EnvNoColor, "1")

	assert.True(t, wire.ColorDisabled())

	t.Setenv(wire.EnvNoColor, "")

	assert.False(t, wire.ColorDisabled(), "empty NO_COLOR must be ignored")

	t.Setenv(wire.EnvCLIColor, "0")

	assert.True(t, wire.ColorDisabled())

	t.Setenv(wire.EnvCLIColor, "1")

	assert.False(t, wire.ColorDisabled())
}

func TestZshStyleLimit(t *testing.T) {
	assert.Equal(t, 300, wire.ZshStyleLimit())

	t.Setenv(wire.EnvZshStyleLimit, "50")

	assert.Equal(t, 50, wire.ZshStyleLimit())

	t.Setenv(wire.EnvZshStyleLimit, "abc")

	assert.Equal(t, 300, wire.ZshStyleLimit())
}
