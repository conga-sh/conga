// Package loaderambiguous covers the conservative degradation of the static
// loader guard: an opaque ConfigOptions argument could hide a file-based
// option, so the analyzer stays silent instead of inventing a conflict.
package loaderambiguous

import (
	"context"

	"github.com/conga-sh/conga"
)

// LoaderWithOpaqueOptions declares a loader alongside an unclassifiable
// ConfigOptions argument.
type LoaderWithOpaqueOptions struct {
	Host conga.OptionalFlag[string] `name:"host"`
}

func (c *LoaderWithOpaqueOptions) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func loadConfig(ctx conga.ConfigContext) (*conga.ConfigDocument, error) { return nil, nil }

var opaqueOptions = []conga.ConfigOption{conga.ConfigPaths("search.json")}

var _ = conga.New[LoaderWithOpaqueOptions]().ConfigOptions(append(opaqueOptions, conga.ConfigLoader(loadConfig))...)
