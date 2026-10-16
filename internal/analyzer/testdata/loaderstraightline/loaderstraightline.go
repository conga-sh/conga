// Package loaderstraightline covers the static loader guard for facts
// declared across straight-line builder statements on the app variable, not
// on one fluent chain.
package loaderstraightline

import (
	"context"

	"github.com/conga-sh/conga"
)

// LoaderStraightLine declares the loader in one statement and the decoder in
// the next.
type LoaderStraightLine struct {
	Host conga.OptionalFlag[string] `name:"host"`
}

func (c *LoaderStraightLine) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func loadConfig(ctx conga.ConfigContext) (*conga.ConfigDocument, error) { return nil, nil }

func decode(data []byte) (map[string]any, error) { return nil, nil }

func straightLineApp() {
	app := conga.New[LoaderStraightLine]().ConfigOptions(conga.ConfigLoader(loadConfig))

	app.RegisterDecoder(".json", decode) // want `conga\.ConfigLoader\(\) replaces file loading and cannot be combined with app\.RegisterDecoder`
}
