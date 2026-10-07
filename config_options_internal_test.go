package conga

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/assertx"
	"github.com/conga-sh/conga/internal/spec"
)

func TestFoldConfigOptions_NoOptions(t *testing.T) {
	t.Parallel()

	settings := foldConfigOptions(nil)

	assert.Equal(t, spec.ConfigSettings{}, settings)
}

func TestFoldConfigOptions_Paths_SetsTier(t *testing.T) {
	t.Parallel()

	settings := foldConfigOptions([]ConfigOption{ConfigPaths("a.json", "b.json")})

	assert.Equal(t, []string{"a.json", "b.json"}, settings.Tier)
	assert.Equal(t, "", settings.Explicit)
	assert.False(t, settings.Cascading)
}

func TestFoldConfigOptions_Explicit_SetsExplicitFile(t *testing.T) {
	t.Parallel()

	settings := foldConfigOptions([]ConfigOption{ConfigExplicit("app.json")})

	assert.Equal(t, "app.json", settings.Explicit)
	assert.Equal(t, []string(nil), settings.Tier)
	assert.False(t, settings.Cascading)
}

func TestFoldConfigOptions_Cascading_EnablesCascading(t *testing.T) {
	t.Parallel()

	settings := foldConfigOptions([]ConfigOption{ConfigCascading()})

	assert.True(t, settings.Cascading)
	assert.Equal(t, "", settings.Explicit)
	assert.Equal(t, []string(nil), settings.Tier)
}

func TestFoldConfigOptions_AllOptions_Composes(t *testing.T) {
	t.Parallel()

	settings := foldConfigOptions([]ConfigOption{
		ConfigPaths("a.json", "b.json"),
		ConfigExplicit("override.json"),
		ConfigCascading(),
	})

	assert.Equal(t, spec.ConfigSettings{
		Explicit:  "override.json",
		Tier:      []string{"a.json", "b.json"},
		Cascading: true,
	}, settings)
}

func TestFoldConfigOptions_OptionOrder_DoesNotAffectResult(t *testing.T) {
	t.Parallel()

	forward := foldConfigOptions([]ConfigOption{
		ConfigPaths("a.json", "b.json"),
		ConfigExplicit("override.json"),
		ConfigCascading(),
	})
	reverse := foldConfigOptions([]ConfigOption{
		ConfigCascading(),
		ConfigExplicit("override.json"),
		ConfigPaths("a.json", "b.json"),
	})

	assert.Equal(t, forward, reverse)
}

func TestFoldConfigOptions_RepeatedPaths_LastDeclarationWins(t *testing.T) {
	t.Parallel()

	settings := foldConfigOptions([]ConfigOption{
		ConfigPaths("first.json"),
		ConfigPaths("second.json", "third.json"),
	})

	assert.Equal(t, []string{"second.json", "third.json"}, settings.Tier)
}

func TestFoldConfigOptions_RepeatedExplicit_LastDeclarationWins(t *testing.T) {
	t.Parallel()

	settings := foldConfigOptions([]ConfigOption{
		ConfigExplicit("first.json"),
		ConfigExplicit("second.json"),
	})

	assert.Equal(t, "second.json", settings.Explicit)
}

func TestFoldConfigOptions_RepeatedCascading_RemainsEnabled(t *testing.T) {
	t.Parallel()

	settings := foldConfigOptions([]ConfigOption{
		ConfigCascading(),
		ConfigCascading(),
	})

	assert.True(t, settings.Cascading)
}

func TestFoldConfigOptions_ReturnedTier_IsIsolatedBetweenSnapshots(t *testing.T) {
	t.Parallel()

	option := ConfigPaths("a.json", "b.json")
	first := foldConfigOptions([]ConfigOption{option})
	first.Tier[0] = "mutated.json"

	second := foldConfigOptions([]ConfigOption{option})

	assert.Equal(t, []string{"a.json", "b.json"}, second.Tier)
}

func TestConfigPaths_ClonesInputSlice(t *testing.T) {
	t.Parallel()

	paths := []string{"a.json", "b.json"}
	option := ConfigPaths(paths...)
	paths[0] = "mutated.json"

	settings := foldConfigOptions([]ConfigOption{option})

	assert.Equal(t, []string{"a.json", "b.json"}, settings.Tier)
}

func TestConfigPaths_ZeroArguments_Panics(t *testing.T) {
	t.Parallel()

	assertx.PanicsWithValue(t, "conga: app.ConfigOptions(conga.ConfigPaths(...)): at least one path is required", func() {
		ConfigPaths()
	})
}

func TestConfigPaths_BlankEntry_Panics(t *testing.T) {
	t.Parallel()

	for name, blank := range map[string]string{
		"empty entry":           "",
		"whitespace-only entry": "   ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertx.PanicsWithValue(t, "conga: app.ConfigOptions(conga.ConfigPaths(...)): path entries must be non-empty", func() {
				ConfigPaths("config.json", blank)
			})
		})
	}
}

func TestConfigExplicit_Blank_Panics(t *testing.T) {
	t.Parallel()

	for name, blank := range map[string]string{
		"empty path":           "",
		"whitespace-only path": "   ",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertx.PanicsWithValue(t, "conga: app.ConfigOptions(conga.ConfigExplicit(...)): the explicit path cannot be empty", func() {
				ConfigExplicit(blank)
			})
		})
	}
}
