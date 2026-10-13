// Package featurechecks exercises the conga-lint mirrors of the feature schema
// contract: root-only Features, struct-parameter requirement, feature tag
// syntax, and required-flag gating.
package featurechecks

import (
	"context"

	"github.com/conga-sh/conga"
)

type goodFeatureSet struct {
	Enabled bool
}

type featureChecksSub struct{}

func (s *featureChecksSub) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type featureChecksApp struct {
	Features conga.Features[goodFeatureSet]

	BadSyntax conga.OptionalFlag[string] `feature:"="       help:"Bad syntax"`     // want `struct featureChecksApp field BadSyntax: schema error: invalid struct tag: invalid feature tag syntax "="`
	Req       conga.RequiredFlag[string] `feature:"enabled" help:"Required gated"` // want `struct featureChecksApp field Req: schema error: invalid struct tag: tag "feature" cannot be used on required flags \(feature-gated flags must be optional\)`

	Sub conga.Command[featureChecksSub] `feature:"foo=bar" summary:"Sub"` // want `struct featureChecksApp field Sub: schema error: invalid struct tag: invalid feature tag syntax "foo=bar"`
}

func (a *featureChecksApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[featureChecksApp]()

type nonRootFeatureSub struct {
	Features conga.Features[goodFeatureSet] // want `struct nonRootFeatureSub field Features: schema error: conga.Features must be declared on the root command struct`
}

func (s *nonRootFeatureSub) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type nonRootFeatureApp struct {
	Sub conga.Command[nonRootFeatureSub] `summary:"Sub"`
}

func (a *nonRootFeatureApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[nonRootFeatureApp]()

type nonStructFeatureApp struct {
	Features conga.Features[int] // want `struct nonStructFeatureApp field Features: schema error: conga.Features must be parameterized by a struct type`
}

func (a *nonStructFeatureApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[nonStructFeatureApp]()
