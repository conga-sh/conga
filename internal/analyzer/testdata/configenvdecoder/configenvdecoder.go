// Package configenvdecoder exercises the conga-lint mirror of the config env
// collision guard for an app without a ConfigFile: a registered decoder
// reserves <PREFIX>_CONFIG, which a flag named config derives.
package configenvdecoder

import (
	"context"

	"github.com/conga-sh/conga"
)

type decoderApp struct {
	Config conga.OptionalFlag[string] `help:"Profile name"` // want `struct decoderApp field Config: schema error: environment variable "APP_CONFIG" is bound by both the config file variable and flag --config; give the flag another env tag or exclude it with env:"-"`
}

func (a *decoderApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[decoderApp]().EnvPrefix("APP").RegisterDecoder(".json", nil)
