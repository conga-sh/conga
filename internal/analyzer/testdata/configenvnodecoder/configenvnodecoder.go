// Package configenvnodecoder checks that an app with neither a ConfigFile nor
// a registered decoder reserves no config variable: a flag named config keeps
// <PREFIX>_CONFIG.
package configenvnodecoder

import (
	"context"

	"github.com/conga-sh/conga"
)

type plainApp struct {
	Config conga.OptionalFlag[string] `help:"Profile name"`
}

func (a *plainApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[plainApp]().EnvPrefix("APP")
