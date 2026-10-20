// Package loadermultianchor covers the conservative degradation of the static
// loader guard under multiple conga.New anchors: facts cannot be attributed to
// a single app, so the analyzer stays silent.
package loadermultianchor

import (
	"context"

	"github.com/conga-sh/conga"
)

// LoaderFirst is the first anchored app with a loader and a decoder.
type LoaderFirst struct {
	Host conga.OptionalFlag[string] `name:"host"`
}

func (c *LoaderFirst) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

// LoaderSecond is the second anchored app with a loader and a decoder.
type LoaderSecond struct {
	Port conga.OptionalFlag[int] `name:"port"`
}

func (c *LoaderSecond) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

func loadConfig[T any](ctx conga.ConfigContext, _ *T) (*conga.ConfigDocument, error) { return nil, nil }

func decode(data []byte) (map[string]any, error) { return nil, nil }

var _ = conga.New[LoaderFirst]().
	ConfigOptions(conga.ConfigLoader(loadConfig[LoaderFirst])).
	RegisterDecoder(".json", decode)

var _ = conga.New[LoaderSecond]().
	ConfigOptions(conga.ConfigLoader(loadConfig[LoaderSecond])).
	RegisterDecoder(".json", decode)
