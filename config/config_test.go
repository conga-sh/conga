package config_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/config"
	"github.com/conga-sh/conga/internal/assertx"
)

func TestApply_NoOptions(t *testing.T) {
	t.Parallel()

	settings := config.Apply()

	assert.Equal(t, config.Settings{}, settings)
}

func TestApply_Paths_SetsTier(t *testing.T) {
	t.Parallel()

	settings := config.Apply(config.Paths("a.json", "b.json"))

	assert.Equal(t, []string{"a.json", "b.json"}, settings.Tier)
	assert.Equal(t, "", settings.Explicit)
	assert.False(t, settings.Cascading)
}

func TestApply_Explicit_SetsExplicitFile(t *testing.T) {
	t.Parallel()

	settings := config.Apply(config.Explicit("app.json"))

	assert.Equal(t, "app.json", settings.Explicit)
	assert.Equal(t, []string(nil), settings.Tier)
	assert.False(t, settings.Cascading)
}

func TestApply_Cascading_EnablesCascading(t *testing.T) {
	t.Parallel()

	settings := config.Apply(config.Cascading())

	assert.True(t, settings.Cascading)
	assert.Equal(t, "", settings.Explicit)
	assert.Equal(t, []string(nil), settings.Tier)
}

func TestApply_AllOptions_Composes(t *testing.T) {
	t.Parallel()

	settings := config.Apply(
		config.Paths("a.json", "b.json"),
		config.Explicit("override.json"),
		config.Cascading(),
	)

	assert.Equal(t, config.Settings{
		Explicit:  "override.json",
		Tier:      []string{"a.json", "b.json"},
		Cascading: true,
	}, settings)
}

func TestApply_OptionOrder_DoesNotAffectResult(t *testing.T) {
	t.Parallel()

	forward := config.Apply(
		config.Paths("a.json", "b.json"),
		config.Explicit("override.json"),
		config.Cascading(),
	)
	reverse := config.Apply(
		config.Cascading(),
		config.Explicit("override.json"),
		config.Paths("a.json", "b.json"),
	)

	assert.Equal(t, forward, reverse)
}

func TestApply_RepeatedPaths_LastDeclarationWins(t *testing.T) {
	t.Parallel()

	settings := config.Apply(
		config.Paths("first.json"),
		config.Paths("second.json", "third.json"),
	)

	assert.Equal(t, []string{"second.json", "third.json"}, settings.Tier)
}

func TestApply_RepeatedExplicit_LastDeclarationWins(t *testing.T) {
	t.Parallel()

	settings := config.Apply(
		config.Explicit("first.json"),
		config.Explicit("second.json"),
	)

	assert.Equal(t, "second.json", settings.Explicit)
}

func TestApply_RepeatedCascading_RemainsEnabled(t *testing.T) {
	t.Parallel()

	settings := config.Apply(
		config.Cascading(),
		config.Cascading(),
	)

	assert.True(t, settings.Cascading)
}

func TestApply_ReturnedTier_IsIsolatedBetweenSnapshots(t *testing.T) {
	t.Parallel()

	option := config.Paths("a.json", "b.json")
	first := config.Apply(option)
	first.Tier[0] = "mutated.json"

	second := config.Apply(option)

	assert.Equal(t, []string{"a.json", "b.json"}, second.Tier)
}

func TestPaths_ClonesInputSlice(t *testing.T) {
	t.Parallel()

	paths := []string{"a.json", "b.json"}
	option := config.Paths(paths...)
	paths[0] = "mutated.json"

	settings := config.Apply(option)

	assert.Equal(t, []string{"a.json", "b.json"}, settings.Tier)
}

func TestPaths_ZeroArguments_Panics(t *testing.T) {
	t.Parallel()

	assertx.PanicsWithValue(t, "conga: app.ConfigOptions(config.Paths(...)): at least one path is required", func() {
		config.Paths()
	})
}

func TestPaths_BlankEntry_Panics(t *testing.T) {
	t.Parallel()

	for name, blank := range map[string]string{
		"empty entry":           "",
		"whitespace-only entry": "   ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertx.PanicsWithValue(t, "conga: app.ConfigOptions(config.Paths(...)): path entries must be non-empty", func() {
				config.Paths("config.json", blank)
			})
		})
	}
}

func TestExplicit_Blank_Panics(t *testing.T) {
	t.Parallel()

	for name, blank := range map[string]string{
		"empty path":           "",
		"whitespace-only path": "   ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertx.PanicsWithValue(t, "conga: app.ConfigOptions(config.Explicit(...)): the explicit path cannot be empty", func() {
				config.Explicit(blank)
			})
		})
	}
}
