package configengine

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/configdata"
	"github.com/conga-sh/conga/internal/sentinel"
	"github.com/conga-sh/conga/internal/spec"
)

func TestFormatConfigValue_SliceFormatting(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "a,b,c", formatConfigValue([]string{"a", "b", "c"}))
	assert.Equal(t, "1,2,3", formatConfigValue([]any{1, 2, 3}))
	assert.Equal(t, "hello", formatConfigValue("hello"))
}

func TestValidateCommandConfig_ExcludedFlagByAlias(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{
				Name:      "token",
				ConfigKey: "-",
				Aliases:   []string{"auth-token"},
			},
		},
	}

	dest := configdata.New()
	err := validateCommandConfig("", map[string]any{"auth-token": "secret"}, cmd, dest, "config.yaml")
	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Contains(t, err.Error(), `field "auth-token" is excluded from config files in command "app"`)

	errPrimary := validateCommandConfig("", map[string]any{"token": "secret"}, cmd, dest, "config.yaml")
	assert.Error(t, errPrimary)
	assert.IsError(t, errPrimary, sentinel.ErrConfigUnknownField)
	assert.Contains(t, errPrimary.Error(), `field "token" is excluded from config files in command "app"`)
}

func TestValidateCommandConfig_UnknownSection(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "port"}},
	}

	dest := configdata.New()
	err := validateCommandConfig("", map[string]any{"cache": map[string]any{"ttl": "30"}}, cmd, dest, "config.yaml")

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Contains(t, err.Error(), `unknown section "cache" in command "app"`)
}

func TestValidateCommandConfig_UnknownFieldInNamespace(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "ttl", ConfigKey: "cache.ttl"}},
	}

	dest := configdata.New()
	err := validateCommandConfig("", map[string]any{"cache": map[string]any{"port": "80"}}, cmd, dest, "config.yaml")

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Contains(t, err.Error(), `unknown field "port" in namespace "cache"`)
}

func TestValidateCommandConfig_MapMergesAcrossFiles(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "tags", IsMap: true}},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{"tags": map[string]any{"shared": "sys", "a": "1"}}, cmd, dest, "sys.toml")
	assert.NoError(t, err)

	err = validateCommandConfig("", map[string]any{"tags": map[string]any{"shared": "local", "b": "2"}}, cmd, dest, "local.toml")
	assert.NoError(t, err)

	entries := dest.Map("tags")

	assert.Equal(t, 3, len(entries))
	assert.Equal(t, configdata.Value{Raw: "1", Origin: "sys.toml"}, entries["a"])
	assert.Equal(t, configdata.Value{Raw: "local", Origin: "local.toml"}, entries["shared"])
	assert.Equal(t, configdata.Value{Raw: "2", Origin: "local.toml"}, entries["b"])
}

func TestValidateCommandConfig_MapSharedAcrossAllAliasKeys(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "pairs", ConfigKey: "remote.pairs", IsMap: true},
			{Name: "tags", ConfigKey: "tags", IsMap: true},
		},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{
		"pairs": map[string]any{"shared": "low", "onlyLow": "1"},
		"tags":  map[string]any{"b": "2"},
	}, cmd, dest, "low.toml")
	assert.NoError(t, err)

	err = validateCommandConfig("", map[string]any{
		"remote": map[string]any{
			"pairs": map[string]any{"shared": "high", "onlyHigh": "2"},
		},
	}, cmd, dest, "high.toml")
	assert.NoError(t, err)

	for _, aliasKey := range []string{"remote.pairs", "pairs"} {
		entries := dest.Map(aliasKey)

		assert.Equal(t, 3, len(entries), aliasKey)
		assert.Equal(t, configdata.Value{Raw: "1", Origin: "low.toml"}, entries["onlyLow"], aliasKey)
		assert.Equal(t, configdata.Value{Raw: "high", Origin: "high.toml"}, entries["shared"], aliasKey)
		assert.Equal(t, configdata.Value{Raw: "2", Origin: "high.toml"}, entries["onlyHigh"], aliasKey)
	}

	tags := dest.Map("tags")

	assert.Equal(t, 1, len(tags))
	assert.Equal(t, configdata.Value{Raw: "2", Origin: "low.toml"}, tags["b"])
}

func TestValidateCommandConfig_FlatStringFormStoresEntries(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "tags", IsMap: true}},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{"tags": "a=1,b=2"}, cmd, dest, "app.toml")
	assert.NoError(t, err)

	entries := dest.Map("tags")

	assert.Equal(t, 2, len(entries))
	assert.Equal(t, configdata.Value{Raw: "1", Origin: "app.toml"}, entries["a"])
	assert.Equal(t, configdata.Value{Raw: "2", Origin: "app.toml"}, entries["b"])
}

func TestValidateCommandConfig_ListFormStoresEntries(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "tags", IsMap: true}},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{"tags": []any{"a=1", "b=2"}}, cmd, dest, "app.toml")
	assert.NoError(t, err)

	entries := dest.Map("tags")

	assert.Equal(t, 2, len(entries))
	assert.Equal(t, configdata.Value{Raw: "1", Origin: "app.toml"}, entries["a"])
	assert.Equal(t, configdata.Value{Raw: "2", Origin: "app.toml"}, entries["b"])
}

func TestValidateCommandConfig_EmptyMapSectionStoresEmptyMap(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "tags", IsMap: true}},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{"tags": map[string]any{}}, cmd, dest, "app.toml")
	assert.NoError(t, err)

	entries := dest.Map("tags")

	assert.Equal(t, 0, len(entries))
	assert.True(t, dest.HasKey("tags"))
}

func TestValidateNamespaceConfig_MapFlagAtDottedConfigKey(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Flags: []*spec.Flag{
			{Name: "insecure", ConfigKey: "remote.insecure"},
			{Name: "pairs", ConfigKey: "remote.pairs", IsMap: true},
		},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{
		"remote": map[string]any{
			"insecure": true,
			"pairs":    map[string]any{"a": "1"},
		},
	}, cmd, dest, "remote.yaml")

	assert.NoError(t, err)

	scalarValue, hasScalar := dest.Scalar("remote.insecure")

	assert.True(t, hasScalar)
	assert.Equal(t, "true", scalarValue)

	entries := dest.Map("remote.pairs")

	assert.Equal(t, 1, len(entries))
	assert.Equal(t, configdata.Value{Raw: "1", Origin: "remote.yaml"}, entries["a"])
}

func TestValidateNamespaceConfig_NonMapFlagWithNestedValueStillErrors(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "tags", ConfigKey: "remote.tags"}},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{
		"remote": map[string]any{
			"tags": map[string]any{"a": "1"},
		},
	}, cmd, dest, "remote.yaml")

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Contains(t, err.Error(), `unknown field "a" in namespace "remote.tags"`)
}

func TestValidateCommandConfig_SubcommandNamespaceErrorLabel(t *testing.T) {
	t.Parallel()

	root := &spec.Command{
		Name: "app",
		Subcommands: []*spec.Command{
			{
				Name:  "deploy",
				Flags: []*spec.Flag{{Name: "port", ConfigKey: "cache.port"}},
			},
		},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{
		"deploy": map[string]any{
			"cache": map[string]any{"bogus": "1"},
		},
	}, root, dest, "app.toml")

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Contains(t, err.Error(), `unknown field "bogus" in namespace "deploy.cache"`)
}

func TestValidateCommandConfig_ExcludedMapSectionRejected(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "tags", ConfigKey: "-", IsMap: true}},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{"tags": map[string]any{"a": "1"}}, cmd, dest, "app.toml")

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Contains(t, err.Error(), `field "tags" is excluded from config files in command "app"`)
	assert.False(t, dest.HasKey("tags"))
}

func TestValidateCommandConfig_ArrayFormSplitsPerElement(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "tags", IsMap: true, Sep: ";"}},
	}

	dest := configdata.New()

	err := validateCommandConfig("", map[string]any{"tags": []any{"a=1", "b=2"}}, cmd, dest, "app.toml")

	assert.NoError(t, err)

	entries := dest.Map("tags")

	assert.Equal(t, 2, len(entries))
	assert.Equal(t, configdata.Value{Raw: "1", Origin: "app.toml"}, entries["a"])
	assert.Equal(t, configdata.Value{Raw: "2", Origin: "app.toml"}, entries["b"])
}
