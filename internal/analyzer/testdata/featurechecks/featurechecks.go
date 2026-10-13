// Package featurechecks exercises the conga-lint mirrors of the feature schema
// contract: root-only Features, struct-parameter requirement, feature tag
// syntax, required-flag gating, feature-name collisions, and the enum/default
// validators shared with schema build.
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
	BadReq    conga.RequiredFlag[string] `feature:"="       help:"Bad required"`   // want `struct featureChecksApp field BadReq: schema error: invalid struct tag: invalid feature tag syntax "="`

	Sub conga.Command[featureChecksSub] `feature:"foo=bar" summary:"Sub"` // want `struct featureChecksApp field Sub: schema error: invalid struct tag: invalid feature tag syntax "foo=bar"`
}

func (a *featureChecksApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[featureChecksApp]()

type nonRootFeatureSub struct {
	Features conga.Features[goodFeatureSet] // want `struct nonRootFeatureSub field Features: schema error: conga.Features must be declared on the root command struct`
	Ignored  conga.Features[goodFeatureSet] `conga:"-"`
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

type badFeatureTagSet struct {
	NoEnum     int  // want `struct badFeatureTagApp field NoEnum: schema error: invalid struct tag: feature "no-enum" is an integer feature and requires an enum tag listing its non-zero levels`
	EmptyEnum  int  `enum:""`                // want `struct badFeatureTagApp field EmptyEnum: schema error: invalid struct tag: tag "enum" cannot be empty`
	BoolEnum   bool `enum:"1"`               // want `struct badFeatureTagApp field BoolEnum: schema error: invalid struct tag: tag "enum" is only valid on integer features`
	BadEnum    int  `enum:"abc"`             // want `struct badFeatureTagApp field BadEnum: schema error: invalid struct tag: feature enum value "abc": must be an integer`
	ZeroEnum   int  `enum:"0,1"`             // want `struct badFeatureTagApp field ZeroEnum: schema error: invalid struct tag: feature "zero-enum" enum value "0" is the reserved off level; list only non-zero levels`
	WidthEnum  int8 `enum:"300"`             // want `struct badFeatureTagApp field WidthEnum: schema error: invalid struct tag: feature enum value "300": is out of range for the feature's integer type`
	BadDefault int  `enum:"1,2" default:"9"` // want `struct badFeatureTagApp field BadDefault: schema error: invalid struct tag: feature "bad-default" default "9": must be one of: 0, 1, 2`
}

type badFeatureTagApp struct {
	Features conga.Features[badFeatureTagSet]
}

func (a *badFeatureTagApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[badFeatureTagApp]()

type duplicateFeatureNameSet struct {
	Name  bool
	Alias bool `name:"name"` // want `struct duplicateFeatureNameApp field Alias: schema error: duplicate flag: feature "name" \(already declared in duplicateFeatureNameSet\)`
}

type duplicateFeatureNameApp struct {
	Features conga.Features[duplicateFeatureNameSet]
}

func (a *duplicateFeatureNameApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[duplicateFeatureNameApp]()

type duplicateFeaturesApp struct {
	Features conga.Features[goodFeatureSet] `flag:"gates"`
	Again    conga.Features[goodFeatureSet] // want `struct duplicateFeaturesApp field Again: schema error: duplicate flag: Features \(already declared by gates\)`
}

func (a *duplicateFeaturesApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[duplicateFeaturesApp]()

type featureShadowSub struct {
	Gates conga.OptionalFlag[bool] `help:"Shadowed gate"` // want `command "sub": schema error: duplicate flag: --gates \(already declared by ancestor command "feature-shadow-app"\)`
}

func (s *featureShadowSub) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type featureShadowApp struct {
	Features conga.Features[goodFeatureSet] `flag:"gates"`

	Sub conga.Command[featureShadowSub] `summary:"Sub"`
}

func (a *featureShadowApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[featureShadowApp]()

type featureFlagCollisionApp struct {
	Features conga.Features[goodFeatureSet] `flag:"gates"`

	Gates conga.OptionalFlag[bool] `name:"gates" help:"Collides with the synthesized flag"` // want `already declared by ancestor`
}

func (a *featureFlagCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[featureFlagCollisionApp]()
