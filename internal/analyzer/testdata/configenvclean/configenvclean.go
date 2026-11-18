// Package configenvclean checks shapes the config env collision mirror must
// not report: a ConfigFile excluded from the environment, a ConfigFile whose
// derived variable rests on an undetermined prefix, and a decoder
// registration that cannot be attributed to one anchor.
package configenvclean

import (
	"context"
	"os"

	"github.com/conga-sh/conga"
)

// A ConfigFile with env:"-" reserves nothing.
type excludedApp struct {
	ConfigFile conga.ConfigFile        `env:"-"          help:"Config file"`
	Port       conga.OptionalFlag[int] `env:"APP_CONFIG" help:"Port"`
}

func (a *excludedApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[excludedApp]()

// The ConfigFile derives its variable from a non-literal prefix.
type derivedApp struct {
	ConfigFile conga.ConfigFile        `help:"Config file"`
	Port       conga.OptionalFlag[int] `env:"APP_CONFIG" help:"Port"`
}

func (a *derivedApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[derivedApp]().EnvPrefix(os.Getenv("PREFIX"))

// Two anchors leave the decoder registration unattributed.
type decoderApp struct {
	Config conga.OptionalFlag[string] `env:"APP_CONFIG" help:"Profile name"`
}

func (a *decoderApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[decoderApp]().RegisterDecoder(".json", nil)
