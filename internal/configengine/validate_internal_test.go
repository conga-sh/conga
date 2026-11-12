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

func TestValidateCommandConfig_ExcludedFlagByName(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "token", ConfigKey: "-", Aliases: []string{"auth-token"}}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"token": "secret"}, cmd, dest, "config.yaml")

	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Equal(t, `config file "config.yaml": field "token" is excluded from config files in command "app"`, err.Error())
}

// Aliases are not config keys, so an excluded flag's alias is simply unknown.
func TestValidateCommandConfig_ExcludedFlagAliasIsUnknown(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "token", ConfigKey: "-", Aliases: []string{"auth-token"}}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"auth-token": "secret"}, cmd, dest, "config.yaml")

	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Equal(t, `config file "config.yaml": unknown field "auth-token" in command "app"`, err.Error())
}

func TestValidateCommandConfig_UnknownSection(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "port"}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"cache": map[string]any{"ttl": "30"}}, cmd, dest, "config.yaml")

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
	err := validateCommandConfig(map[string]any{"cache": map[string]any{"port": "80"}}, cmd, dest, "config.yaml")

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

	err := validateCommandConfig(map[string]any{"tags": map[string]any{"shared": "sys", "a": "1"}}, cmd, dest, "sys.toml")
	assert.NoError(t, err)

	err = validateCommandConfig(map[string]any{"tags": map[string]any{"shared": "local", "b": "2"}}, cmd, dest, "local.toml")
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

	err := validateCommandConfig(map[string]any{
		"remote": map[string]any{
			"pairs": map[string]any{"shared": "low", "onlyLow": "1"},
		},
		"tags": map[string]any{"b": "2"},
	}, cmd, dest, "low.toml")
	assert.NoError(t, err)

	err = validateCommandConfig(map[string]any{
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

	err := validateCommandConfig(map[string]any{"tags": "a=1,b=2"}, cmd, dest, "app.toml")
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

	err := validateCommandConfig(map[string]any{"tags": []any{"a=1", "b=2"}}, cmd, dest, "app.toml")
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

	err := validateCommandConfig(map[string]any{"tags": map[string]any{}}, cmd, dest, "app.toml")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(dest.Map("tags")))
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

	err := validateCommandConfig(map[string]any{
		"remote": map[string]any{
			"insecure": true,
			"pairs":    map[string]any{"a": "1"},
		},
	}, cmd, dest, "remote.yaml")

	assert.NoError(t, err)

	scalarValue, hasScalar := dest.ScalarValue("remote.insecure")

	assert.True(t, hasScalar)
	assert.Equal(t, "true", scalarValue.Raw)

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

	err := validateCommandConfig(map[string]any{
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

	err := validateCommandConfig(map[string]any{
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

	err := validateCommandConfig(map[string]any{"tags": map[string]any{"a": "1"}}, cmd, dest, "app.toml")

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

	err := validateCommandConfig(map[string]any{"tags": []any{"a=1", "b=2"}}, cmd, dest, "app.toml")

	assert.NoError(t, err)

	entries := dest.Map("tags")

	assert.Equal(t, 2, len(entries))
	assert.Equal(t, configdata.Value{Raw: "1", Origin: "app.toml"}, entries["a"])
	assert.Equal(t, configdata.Value{Raw: "2", Origin: "app.toml"}, entries["b"])
}

func TestValidateDocument_ScalarStoresOrigin(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "host"}},
	}

	dest := configdata.New()
	err := ValidateDocument(map[string]any{"host": "doc.example.com"}, cmd, dest, "embedded defaults", "")

	assert.NoError(t, err)

	value, exists := dest.ScalarValue("host")

	assert.True(t, exists)
	assert.Equal(t, configdata.Value{Raw: "doc.example.com", Origin: "embedded defaults"}, value)
}

func TestValidateDocument_StringMapSectionStoresOrigin(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "headers", IsMap: true}},
	}

	dest := configdata.New()
	err := ValidateDocument(map[string]any{"headers": map[string]string{"Region": "eu"}}, cmd, dest, "taskrc", "")

	assert.NoError(t, err)

	entries := dest.Map("headers")

	assert.Equal(t, 1, len(entries))
	assert.Equal(t, configdata.Value{Raw: "eu", Origin: "taskrc"}, entries["Region"])
}

func TestValidateDocument_UnknownKeyAttributesOrigin(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "host"}},
	}

	err := ValidateDocument(map[string]any{"hots": "x"}, cmd, configdata.New(), "taskrc", "")

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Contains(t, err.Error(), `config file "taskrc"`)
	assert.Contains(t, err.Error(), `unknown field "hots" in command "app"`)
}

func TestValidateDocument_StoresOriginAndScope(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "host"}, {Name: "headers", IsMap: true}},
	}

	dest := configdata.New()
	err := ValidateDocument(map[string]any{
		"host":    "doc.example.com",
		"headers": map[string]any{"Region": "eu"},
	}, cmd, dest, "pipeline.yaml", "jobs/deploy")

	assert.NoError(t, err)

	scalar, exists := dest.ScalarValue("host")

	assert.True(t, exists)
	assert.Equal(t, configdata.Value{Raw: "doc.example.com", Origin: "pipeline.yaml", Scope: "jobs/deploy"}, scalar)
	assert.Equal(t, "pipeline.yaml › jobs/deploy", scalar.SourceDetail())
	assert.Equal(t, configdata.Value{Raw: "eu", Origin: "pipeline.yaml", Scope: "jobs/deploy"}, dest.Map("headers")["Region"])
}

func TestValidateDocument_EmptyOriginIgnoresScope(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "host"}},
	}

	dest := configdata.New()
	err := ValidateDocument(map[string]any{"host": "localhost"}, cmd, dest, "", "jobs/deploy")

	assert.NoError(t, err)

	value, exists := dest.ScalarValue("host")

	assert.True(t, exists)
	assert.Equal(t, configdata.Value{Raw: "localhost"}, value)
}

func TestHandleNestedSection_ScalarFlagReportsNotAMap(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "port"}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"port": map[string]any{"http": 8080}}, cmd, dest, "taskrc")

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrConfig)
	assert.Equal(t, `config file "taskrc": field "port" in command "app" is not a map`, err.Error())
	assert.NotContains(t, err.Error(), "did you mean")
}

func TestValidateCommandConfig_OtherSpellingsRejected(t *testing.T) {
	t.Parallel()

	messages := map[string]string{
		"dry_run": `config file "config.yaml": unknown field "dry_run" in command "app" (did you mean "dry-run"?)`,
		"dryRun":  `config file "config.yaml": unknown field "dryRun" in command "app" (did you mean "dry-run"?)`,
		"Dry-Run": `config file "config.yaml": unknown field "Dry-Run" in command "app" (did you mean "dry-run"?)`,
		"n":       `config file "config.yaml": unknown field "n" in command "app"`,
	}

	for key, message := range messages {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			cmd := &spec.Command{
				Name:  "app",
				Flags: []*spec.Flag{{Name: "dry-run", Aliases: []string{"n"}}},
			}

			dest := configdata.New()
			err := validateCommandConfig(map[string]any{key: "true"}, cmd, dest, "config.yaml")

			assert.IsError(t, err, sentinel.ErrConfigUnknownField)
			assert.Equal(t, message, err.Error())
		})
	}
}

func TestValidateCommandConfig_FlagNameBinds(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "dry-run"}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"dry-run": "true"}, cmd, dest, "config.yaml")

	assert.NoError(t, err)

	value, exists := dest.ScalarValue("dry-run")

	assert.True(t, exists)
	assert.Equal(t, configdata.Value{Raw: "true", Origin: "config.yaml"}, value)
}

// With every other spelling rejected, the first error for a document with
// several bad keys must not depend on map iteration order.
func TestValidateCommandConfig_FirstErrorIsStable(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "dry-run"}},
	}

	for range 50 {
		dest := configdata.New()
		err := validateCommandConfig(map[string]any{"dry-run": "true", "dry_run": "false", "dryRun": "x"}, cmd, dest, "config.yaml")

		assert.Equal(t, `config file "config.yaml": unknown field "dryRun" in command "app" (did you mean "dry-run"?)`, err.Error())
	}
}

func TestValidateCommandConfig_SubcommandAliasSectionRejected(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Subcommands: []*spec.Command{{
			Name:    "deploy",
			Aliases: []string{"d"},
			Flags:   []*spec.Flag{{Name: "region"}},
		}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"d": map[string]any{"region": "eu"}}, cmd, dest, "config.yaml")

	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Equal(t, `config file "config.yaml": unknown section "d" in command "app"`, err.Error())
}

func TestValidateCommandConfig_SubcommandSectionOtherSpellingRejected(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name: "app",
		Subcommands: []*spec.Command{{
			Name:  "deploy-app",
			Flags: []*spec.Flag{{Name: "region"}},
		}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"deploy_app": map[string]any{"region": "eu"}}, cmd, dest, "config.yaml")

	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Equal(t, `config file "config.yaml": unknown section "deploy_app" in command "app" (did you mean "deploy-app"?)`, err.Error())
}

func TestValidateNamespaceConfig_RootFlagNameRejectedInSection(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "host", ConfigKey: "db.host"}, {Name: "port"}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"db": map[string]any{"host": "h", "port": 99}}, cmd, dest, "config.toml")

	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Equal(t, `config file "config.toml": unknown field "port" in namespace "db" (did you mean "host"?)`, err.Error())
}

func TestValidateNamespaceConfig_ExcludedRootFlagReportedUnknown(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "host", ConfigKey: "db.host"}, {Name: "port", ConfigKey: "-"}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"db": map[string]any{"port": 99}}, cmd, dest, "config.toml")

	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Equal(t, `config file "config.toml": unknown field "port" in namespace "db" (did you mean "host"?)`, err.Error())
}

func TestValidateNamespaceConfig_SuggestsKeyInSection(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "host", ConfigKey: "db.host"}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"db": map[string]any{"hots": "h"}}, cmd, dest, "config.toml")

	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Equal(t, `config file "config.toml": unknown field "hots" in namespace "db" (did you mean "host"?)`, err.Error())
}

func TestValidateCommandConfig_DottedTagOnlyMatchesNestedSections(t *testing.T) {
	t.Parallel()

	messages := map[string]string{
		"db.host": `config file "config.toml": unknown field "db.host" in command "app"`,
		"dbhost":  `config file "config.toml": unknown field "dbhost" in command "app"`,
		"host":    `config file "config.toml": unknown field "host" in command "app"`,
	}

	for key, message := range messages {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			cmd := &spec.Command{
				Name:  "app",
				Flags: []*spec.Flag{{Name: "host", ConfigKey: "db.host"}},
			}

			dest := configdata.New()
			err := validateCommandConfig(map[string]any{key: "x"}, cmd, dest, "config.toml")

			assert.IsError(t, err, sentinel.ErrConfigUnknownField)
			assert.Equal(t, message, err.Error())
		})
	}
}

func TestValidateCommandConfig_DottedTagBindsAsNestedSections(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "host", ConfigKey: "db.host"}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"db": map[string]any{"host": "h"}}, cmd, dest, "config.toml")

	assert.NoError(t, err)

	value, exists := dest.ScalarValue("db.host")

	assert.True(t, exists)
	assert.Equal(t, configdata.Value{Raw: "h", Origin: "config.toml"}, value)
}

func TestValidateCommandConfig_ConfigTagReplacesFlagName(t *testing.T) {
	t.Parallel()

	messages := map[string]string{
		"api-endpoint": `config file "config.toml": unknown field "api-endpoint" in command "app" (did you mean "endpoint"?)`,
		"Endpoint":     `config file "config.toml": unknown field "Endpoint" in command "app" (did you mean "endpoint"?)`,
	}

	for key, message := range messages {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			cmd := &spec.Command{
				Name:  "app",
				Flags: []*spec.Flag{{Name: "api-endpoint", ConfigKey: "endpoint"}},
			}

			dest := configdata.New()
			err := validateCommandConfig(map[string]any{key: "x"}, cmd, dest, "config.toml")

			assert.IsError(t, err, sentinel.ErrConfigUnknownField)
			assert.Equal(t, message, err.Error())
		})
	}
}

func TestValidateCommandConfig_ConfigTagBinds(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:  "app",
		Flags: []*spec.Flag{{Name: "api-endpoint", ConfigKey: "endpoint"}},
	}

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"endpoint": "x"}, cmd, dest, "config.toml")

	assert.NoError(t, err)

	value, exists := dest.ScalarValue("endpoint")

	assert.True(t, exists)
	assert.Equal(t, configdata.Value{Raw: "x", Origin: "config.toml"}, value)
}

func featureMapCommand(keyCase spec.KeyCase) *spec.Command {
	featureFlag := &spec.Flag{Name: "feature-gates", ConfigKey: "experiments", IsMap: true}

	return &spec.Command{
		Name:           "app",
		Flags:          []*spec.Flag{featureFlag},
		FeatureFlag:    featureFlag,
		FeatureFields:  []*spec.FeatureField{{Name: "gentle-force"}},
		ConfigSettings: spec.ConfigSettings{KeyCase: keyCase},
	}
}

func TestValidateCommandConfig_FeatureKeyOtherSpellingRejected(t *testing.T) {
	t.Parallel()

	documents := map[string]any{
		"nested map": map[string]any{"gentle_force": true},
		"flat list":  []any{"gentle_force=true"},
		"flat text":  "gentle_force=true",
	}

	for name, section := range documents {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dest := configdata.New()
			err := validateCommandConfig(map[string]any{"experiments": section}, featureMapCommand(spec.KeyCaseKebab), dest, "config.yaml")

			assert.IsError(t, err, sentinel.ErrConfigUnknownField)
			assert.Equal(t, `config file "config.yaml": unknown field "gentle_force" in section "experiments" (did you mean "gentle-force"?)`, err.Error())
		})
	}
}

func TestValidateCommandConfig_FeatureKeyExactNameBinds(t *testing.T) {
	t.Parallel()

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"experiments": map[string]any{"gentle-force": true}}, featureMapCommand(spec.KeyCaseKebab), dest, "config.yaml")

	assert.NoError(t, err)

	entries := dest.Map("experiments")

	assert.Equal(t, map[string]configdata.Value{"gentle-force": {Raw: "true", Origin: "config.yaml"}}, entries)
}

func TestValidateCommandConfig_KeyCaseSpellsDerivedKeys(t *testing.T) {
	t.Parallel()

	cases := map[spec.KeyCase]map[string]any{
		spec.KeyCaseKebab: {"dry-run": "true", "deploy-app": map[string]any{"max-retries": "3"}},
		spec.KeyCaseSnake: {"dry_run": "true", "deploy_app": map[string]any{"max_retries": "3"}},
		spec.KeyCaseCamel: {"dryRun": "true", "deployApp": map[string]any{"maxRetries": "3"}},
	}

	for keyCase, document := range cases {
		t.Run(keyCase.Apply("key-case"), func(t *testing.T) {
			t.Parallel()

			cmd := &spec.Command{
				Name:  "app",
				Flags: []*spec.Flag{{Name: "dry-run"}},
				Subcommands: []*spec.Command{{
					Name:  "deploy-app",
					Flags: []*spec.Flag{{Name: "max-retries"}},
				}},
				ConfigSettings: spec.ConfigSettings{KeyCase: keyCase},
			}

			dest := configdata.New()
			err := validateCommandConfig(document, cmd, dest, "config.yaml")

			assert.NoError(t, err)

			dryRun, dryRunExists := dest.ScalarValue("dry-run")
			retries, retriesExists := dest.ScalarValue("deploy-app.max-retries")

			assert.True(t, dryRunExists)
			assert.Equal(t, configdata.Value{Raw: "true", Origin: "config.yaml"}, dryRun)
			assert.True(t, retriesExists)
			assert.Equal(t, configdata.Value{Raw: "3", Origin: "config.yaml"}, retries)
		})
	}
}

func TestValidateCommandConfig_KeyCaseRejectsOtherCasings(t *testing.T) {
	t.Parallel()

	messages := map[string]string{
		"dry-run": `config file "config.yaml": unknown field "dry-run" in command "app" (did you mean "dry_run"?)`,
		"dryRun":  `config file "config.yaml": unknown field "dryRun" in command "app" (did you mean "dry_run"?)`,
	}

	for key, message := range messages {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			cmd := &spec.Command{
				Name:           "app",
				Flags:          []*spec.Flag{{Name: "dry-run"}},
				ConfigSettings: spec.ConfigSettings{KeyCase: spec.KeyCaseSnake},
			}

			dest := configdata.New()
			err := validateCommandConfig(map[string]any{key: "true"}, cmd, dest, "config.yaml")

			assert.IsError(t, err, sentinel.ErrConfigUnknownField)
			assert.Equal(t, message, err.Error())
		})
	}
}

func TestValidateCommandConfig_KeyCaseLeavesConfigTagAsWritten(t *testing.T) {
	t.Parallel()

	cmd := &spec.Command{
		Name:           "app",
		Flags:          []*spec.Flag{{Name: "api-endpoint", ConfigKey: "api-url"}},
		ConfigSettings: spec.ConfigSettings{KeyCase: spec.KeyCaseSnake},
	}

	dest := configdata.New()
	errTag := validateCommandConfig(map[string]any{"api-url": "x"}, cmd, dest, "config.yaml")
	errCased := validateCommandConfig(map[string]any{"api_url": "x"}, cmd, dest, "config.yaml")

	assert.NoError(t, errTag)
	assert.IsError(t, errCased, sentinel.ErrConfigUnknownField)
	assert.Equal(t, `config file "config.yaml": unknown field "api_url" in command "app" (did you mean "api-url"?)`, errCased.Error())
}

func TestValidateCommandConfig_KeyCaseSpellsFeatureKeys(t *testing.T) {
	t.Parallel()

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"experiments": map[string]any{"gentle_force": true}}, featureMapCommand(spec.KeyCaseSnake), dest, "config.yaml")

	assert.NoError(t, err)

	entries := dest.Map("experiments")

	assert.Equal(t, map[string]configdata.Value{"gentle-force": {Raw: "true", Origin: "config.yaml"}}, entries)
}

func TestValidateCommandConfig_KeyCaseRejectsKebabFeatureKey(t *testing.T) {
	t.Parallel()

	dest := configdata.New()
	err := validateCommandConfig(map[string]any{"experiments": map[string]any{"gentle-force": true}}, featureMapCommand(spec.KeyCaseCamel), dest, "config.yaml")

	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Equal(t, `config file "config.yaml": unknown field "gentle-force" in section "experiments" (did you mean "gentleForce"?)`, err.Error())
}
