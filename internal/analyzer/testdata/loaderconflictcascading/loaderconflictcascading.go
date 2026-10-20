// Package loaderconflictcascading covers the static loader guard for
// conga.ConfigLoader() combined with conga.ConfigCascading() across two
// ConfigOptions calls on one fluent chain.
package loaderconflictcascading

import (
	"context"

	"github.com/conga-sh/conga"
)

// LoaderWithCascading declares a programmatic loader and cascading merge mode.
type LoaderWithCascading struct {
	Host conga.OptionalFlag[string] `name:"host"`
}

func (c *LoaderWithCascading) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func loadConfig(ctx conga.ConfigContext, _ *LoaderWithCascading) (*conga.ConfigDocument, error) {
	return nil, nil
}

var _ = conga.New[LoaderWithCascading]().
	ConfigOptions(conga.ConfigLoader(loadConfig)).
	ConfigOptions(conga.ConfigCascading()) // want `conga\.ConfigLoader\(\) supplies the configuration document programmatically and cannot be combined with conga\.ConfigPaths\(\), conga\.ConfigExplicit\(\), or conga\.ConfigCascading\(\)`
