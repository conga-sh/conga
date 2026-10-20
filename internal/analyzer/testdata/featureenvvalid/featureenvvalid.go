// Package featureenvvalid exercises the conga-lint acceptance of per-feature
// environment variables: field env tags, container envprefix derivation, and
// an application prefix that never reaches features.
package featureenvvalid

import (
	"context"

	"github.com/conga-sh/conga"
)

type fieldEnvSet struct {
	GentleForce bool `env:"TASK_X_GENTLE_FORCE" help:"Forces all dependent tasks"`
	Excluded    bool `env:"-"                   help:"CLI and config only"`
	Derived     bool `                          help:"Derives from the container prefix"`
}

type fieldEnvSub struct {
	Name conga.OptionalFlag[string] `help:"Sub name"`
}

func (s *fieldEnvSub) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type fieldEnvApp struct {
	Features conga.Features[fieldEnvSet] `envprefix:"TASK_X" flag:"feature-gates" help:"Gates via {{env}}"`

	Port conga.OptionalFlag[int] `help:"Port"`

	Sub conga.Command[fieldEnvSub] `summary:"Sub"`
}

func (a *fieldEnvApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[fieldEnvApp]().EnableFeatures().EnvPrefix("OTHER")
