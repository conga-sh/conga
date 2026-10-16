// Package loaderconflictconfigfile covers the static loader guard for
// conga.ConfigLoader() combined with a root conga.ConfigFile field: the
// loader replaces file loading, so the config-file flag never loads a file.
package loaderconflictconfigfile

import (
	"context"

	"github.com/conga-sh/conga"
)

// LoaderWithConfigFile declares a programmatic loader and a config-file flag
// whose resolved default name is reported in the diagnostic.
type LoaderWithConfigFile struct {
	Config conga.ConfigFile // want `the root command declares config-file flag "config-file", but conga\.ConfigLoader\(\) replaces file loading and never loads a file`
}

func (c *LoaderWithConfigFile) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func loadConfig(ctx conga.ConfigContext) (*conga.ConfigDocument, error) { return nil, nil }

var _ = conga.New[LoaderWithConfigFile]().ConfigOptions(conga.ConfigLoader(loadConfig))
