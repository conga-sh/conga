// Package featureenvconflict exercises the conga-lint mirrors of the feature
// env surface exclusivity rules: env and envprefix on one container, and a
// field env tag under a combined-map container.
package featureenvconflict

import (
	"context"

	"github.com/conga-sh/conga"
)

type modeConflictSet struct {
	Enabled bool
}

type modeConflictApp struct {
	Features conga.Features[modeConflictSet] `env:"APP_FEATURES" envprefix:"APP_"` // want `struct modeConflictApp field Features: tag "env" and tag "envprefix" cannot both be declared on a conga.Features container`
}

func (a *modeConflictApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[modeConflictApp]().EnableFeatures()

type fieldConflictSet struct {
	Enabled bool `env:"TASK_X_ENABLED" help:"Enabled"`
}

type fieldConflictApp struct {
	Features conga.Features[fieldConflictSet] `env:"APP_FEATURES" flag:"feature-gates"` // want `struct fieldConflictApp field Features: feature field Enabled declares env while the Features container declares env`
}

func (a *fieldConflictApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[fieldConflictApp]().EnableFeatures()
