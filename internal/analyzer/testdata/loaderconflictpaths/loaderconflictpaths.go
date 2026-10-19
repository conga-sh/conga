// Package loaderconflictpaths covers the static loader guard for
// conga.ConfigLoader() combined with conga.ConfigPaths() in one
// ConfigOptions call: the loader replaces file resolution, so the paths are
// dead configuration.
package loaderconflictpaths

import (
	"context"

	"github.com/conga-sh/conga"
)

// LoaderWithPaths declares a programmatic loader and a file search tier.
type LoaderWithPaths struct {
	Host conga.OptionalFlag[string] `name:"host"`
}

func (c *LoaderWithPaths) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func loadConfig(ctx conga.ConfigContext, _ *LoaderWithPaths) (*conga.ConfigDocument, error) {
	return nil, nil
}

var _ = conga.New[LoaderWithPaths]().ConfigOptions(conga.ConfigLoader(loadConfig), conga.ConfigPaths("search.json")) // want `conga\.ConfigLoader\(\) supplies the configuration document programmatically and cannot be combined with conga\.ConfigPaths\(\), conga\.ConfigExplicit\(\), or conga\.ConfigCascading\(\)`
