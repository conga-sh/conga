// Package featuretypes contains a conga.Features declaration whose inner field
// has an illegal type; conga-lint must mirror the runtime kind check.
package featuretypes

import (
	"context"

	"github.com/conga-sh/conga"
)

type badFeatureSet struct {
	Enabled bool
	Label   string // want `struct featureTypesApp field Label: schema error: invalid struct tag: feature field Label must be a bool or an integer, got string`
}

type featureTypesApp struct {
	Features conga.Features[badFeatureSet]
}

func (a *featureTypesApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[featureTypesApp]().EnableFeatures()
