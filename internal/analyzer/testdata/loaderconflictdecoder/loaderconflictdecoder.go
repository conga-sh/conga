// Package loaderconflictdecoder covers the static loader guard for
// conga.ConfigLoader() combined with app.RegisterDecoder: with a loader there
// is no file loading, so decoders are dead weight.
package loaderconflictdecoder

import (
	"context"

	"github.com/conga-sh/conga"
)

// LoaderWithDecoder declares a programmatic loader and a format decoder.
type LoaderWithDecoder struct {
	Host conga.OptionalFlag[string] `name:"host"`
}

func (c *LoaderWithDecoder) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func loadConfig(ctx conga.ConfigContext, _ *LoaderWithDecoder) (*conga.ConfigDocument, error) {
	return nil, nil
}

func decode(data []byte) (map[string]any, error) { return nil, nil }

var _ = conga.New[LoaderWithDecoder]().
	ConfigOptions(conga.ConfigLoader(loadConfig)).
	RegisterDecoder(".json", decode) // want `conga\.ConfigLoader\(\) replaces file loading and cannot be combined with app\.RegisterDecoder`
