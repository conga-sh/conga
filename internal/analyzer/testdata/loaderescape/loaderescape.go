// Package loaderescape covers the conservative degradation of the static
// loader guard when the app value escapes the analyzed function: builder
// calls on an escaping app cannot be attributed, so the analyzer stays
// silent.
package loaderescape

import (
	"context"

	"github.com/conga-sh/conga"
)

// LoaderEscaping returns the app value, hiding any later builder call from
// the straight-line analysis.
type LoaderEscaping struct {
	Host conga.OptionalFlag[string] `name:"host"`
}

func (c *LoaderEscaping) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func loadConfig(ctx conga.ConfigContext, _ *LoaderEscaping) (*conga.ConfigDocument, error) {
	return nil, nil
}

func decode(data []byte) (map[string]any, error) { return nil, nil }

func escapingApp() *conga.App[LoaderEscaping] {
	return conga.New[LoaderEscaping]().
		ConfigOptions(conga.ConfigLoader(loadConfig)).
		RegisterDecoder(".json", decode)
}
