package configengine

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/internal/sentinel"
	"github.com/conga-sh/conga/internal/spec"
)

func TestFormatConfigValue_DeterministicMapSorting(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"z": 10,
		"a": 1,
		"m": 5,
		"b": 2,
		"k": 3,
		"c": 4,
	}

	expected := "a=1,b=2,c=4,k=3,m=5,z=10"
	for range 50 {
		assert.Equal(t, expected, formatConfigValue(input))
	}
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

	dest := make(map[string]string)
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

	dest := make(map[string]string)
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

	dest := make(map[string]string)
	err := validateCommandConfig("", map[string]any{"cache": map[string]any{"port": "80"}}, cmd, dest, "config.yaml")

	assert.Error(t, err)
	assert.IsError(t, err, sentinel.ErrConfigUnknownField)
	assert.Contains(t, err.Error(), `unknown field "port" in namespace "cache"`)
}
