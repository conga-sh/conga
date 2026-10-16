package spec

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestValidateConfigSettings_LoaderWithFileOptions(t *testing.T) {
	t.Parallel()

	root := &Command{
		Name: "app",
		ConfigSettings: ConfigSettings{
			Loader: true,
			Tier:   []string{"a.json"},
		},
	}

	err := validateConfigSettings(root, Options{})

	assert.Error(t, err)
	assert.IsError(t, err, ErrSchema)
	assert.Equal(t, "schema error: conga.ConfigLoader() supplies the configuration document programmatically and cannot be combined with conga.ConfigPaths(), conga.ConfigExplicit(), or conga.ConfigCascading()", err.Error())
}

func TestValidateConfigSettings_LoaderWithExplicit(t *testing.T) {
	t.Parallel()

	root := &Command{
		Name: "app",
		ConfigSettings: ConfigSettings{
			Loader:   true,
			Explicit: "a.json",
		},
	}

	err := validateConfigSettings(root, Options{})

	assert.Error(t, err)
	assert.IsError(t, err, ErrSchema)
	assert.Equal(t, "schema error: conga.ConfigLoader() supplies the configuration document programmatically and cannot be combined with conga.ConfigPaths(), conga.ConfigExplicit(), or conga.ConfigCascading()", err.Error())
}

func TestValidateConfigSettings_LoaderWithCascading(t *testing.T) {
	t.Parallel()

	root := &Command{
		Name: "app",
		ConfigSettings: ConfigSettings{
			Loader:    true,
			Cascading: true,
		},
	}

	err := validateConfigSettings(root, Options{})

	assert.Error(t, err)
	assert.IsError(t, err, ErrSchema)
	assert.Equal(t, "schema error: conga.ConfigLoader() supplies the configuration document programmatically and cannot be combined with conga.ConfigPaths(), conga.ConfigExplicit(), or conga.ConfigCascading()", err.Error())
}

func TestValidateConfigSettings_LoaderWithDecoder(t *testing.T) {
	t.Parallel()

	root := &Command{
		Name:           "app",
		ConfigSettings: ConfigSettings{Loader: true},
	}

	err := validateConfigSettings(root, Options{ConfigFilePatterns: []string{"*.json"}})

	assert.Error(t, err)
	assert.IsError(t, err, ErrSchema)
	assert.Equal(t, "schema error: conga.ConfigLoader() replaces file loading and cannot be combined with app.RegisterDecoder (remove the decoder registrations or the loader)", err.Error())
}

func TestValidateConfigSettings_LoaderWithConfigFileFlag(t *testing.T) {
	t.Parallel()

	root := &Command{
		Name:           "app",
		ConfigSettings: ConfigSettings{Loader: true},
		ConfigFileFlag: &Flag{Name: "config-file"},
	}

	err := validateConfigSettings(root, Options{})

	assert.Error(t, err)
	assert.IsError(t, err, ErrSchema)
	assert.Equal(t, `schema error: the root command declares config-file flag "config-file", but conga.ConfigLoader() replaces file loading and never loads a file (remove the conga.ConfigFile field or the loader)`, err.Error())
}

func TestValidateConfigSettings_LoaderAloneAccepted(t *testing.T) {
	t.Parallel()

	root := &Command{
		Name:           "app",
		ConfigSettings: ConfigSettings{Loader: true},
	}

	assert.NoError(t, validateConfigSettings(root, Options{}))
}
