// Package loaderclean covers the static loader guard for a satisfying app:
// conga.ConfigLoader() alone replaces file resolution and must not be
// flagged.
package loaderclean

import (
	"context"

	"github.com/conga-sh/conga"
)

// LoaderOnly declares a programmatic configuration loader without any
// file-based configuration mechanism.
type LoaderOnly struct {
	Host conga.OptionalFlag[string] `name:"host"`
}

func (c *LoaderOnly) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func loadConfig(ctx conga.ConfigContext, _ *LoaderOnly) (*conga.ConfigDocument, error) {
	return nil, nil
}

var _ = conga.New[LoaderOnly]().ConfigOptions(conga.ConfigLoader(loadConfig))
