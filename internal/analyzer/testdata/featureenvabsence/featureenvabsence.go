// Package featureenvabsence exercises the analyzer acceptance of env absence
// on feature fields under a combined-map container.
package featureenvabsence

import (
	"context"

	"github.com/conga-sh/conga"
)

type absenceSet struct {
	Excluded bool `env:"-" help:"Excluded"`
	Empty    bool `env:""  help:"Empty"`
}

type absenceApp struct {
	Features conga.Features[absenceSet] `env:"APP_FEATURES" flag:"feature-gates"`
}

func (a *absenceApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[absenceApp]().EnableFeatures()
