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

	BadSyntax conga.OptionalFlag[string] `feature:"="       help:"Bad syntax"`     // want `struct featureChecksApp field BadSyntax: invalid feature tag syntax "="`
	Req       conga.RequiredFlag[string] `feature:"enabled" help:"Required gated"` // want `struct featureChecksApp field Req: tag "feature" cannot be used on required flags`
	BadReq    conga.RequiredFlag[string] `feature:"="       help:"Bad required"`   // want `struct featureChecksApp field BadReq: invalid feature tag syntax "="`

	Sub conga.Command[featureChecksSub] `feature:"foo=bar" summary:"Sub"` // want `struct featureChecksApp field Sub: invalid feature tag syntax "foo=bar"`
}

func (a *featureChecksApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[featureChecksApp]()

type nonRootFeatureSub struct {
	Features conga.Features[goodFeatureSet] // want `struct nonRootFeatureSub field Features: conga.Features must be declared on the root command struct`
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
	Features conga.Features[int] // want `struct nonStructFeatureApp field Features: conga.Features must be parameterized by a struct type`
}

func (a *nonStructFeatureApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[nonStructFeatureApp]()

type badFeatureTagSet struct {
	NoEnum     int  // want `struct badFeatureTagSet field NoEnum: feature "no-enum" is an integer feature and requires an enum tag listing its non-zero levels`
	EmptyEnum  int  `enum:""`                // want `struct badFeatureTagSet field EmptyEnum: tag "enum" cannot be empty`
	BoolEnum   bool `enum:"1"`               // want `struct badFeatureTagSet field BoolEnum: tag "enum" is only valid on integer features`
	BadEnum    int  `enum:"abc"`             // want `struct badFeatureTagSet field BadEnum: feature enum value "abc"`
	ZeroEnum   int  `enum:"0,1"`             // want `struct badFeatureTagSet field ZeroEnum: feature "zero-enum" enum value "0" is the reserved off level`
	WidthEnum  int8 `enum:"300"`             // want `struct badFeatureTagSet field WidthEnum: feature enum value "300"`
	BadDefault int  `enum:"1,2" default:"9"` // want `struct badFeatureTagSet field BadDefault: feature "bad-default" default "9"`
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
	Alias bool `name:"name"` // want `struct duplicateFeatureNameSet field Alias: duplicate feature "name"`
}

type duplicateFeatureNameApp struct {
	Features conga.Features[duplicateFeatureNameSet]
}

func (a *duplicateFeatureNameApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[duplicateFeatureNameApp]()

type anonymousDuplicateFeatureApp struct {
	Features conga.Features[struct {
		Name  bool
		Alias bool `name:"name"` // want `struct anonymousDuplicateFeatureApp\.Features field Alias: duplicate feature "name"`
	}]
}

func (a *anonymousDuplicateFeatureApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[anonymousDuplicateFeatureApp]()

type duplicateFeaturesApp struct {
	Features conga.Features[goodFeatureSet] `flag:"gates"`
	Again    conga.Features[goodFeatureSet] // want `struct duplicateFeaturesApp field Again: duplicate Features container`
}

func (a *duplicateFeaturesApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[duplicateFeaturesApp]()

type featureShadowSub struct {
	Gates conga.OptionalFlag[bool] `help:"Shadowed gate"` // want `struct featureShadowSub field Gates: duplicate flag --gates`
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

	Gates conga.OptionalFlag[bool] `name:"gates" help:"Collides with the synthesized flag"` // want `struct featureFlagCollisionApp field Gates: duplicate flag --gates`
}

func (a *featureFlagCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[featureFlagCollisionApp]()

// featureFlagCollisionOrderApp declares the user flag before the Features
// container, so the synthesized flag collides second. The runtime aborts
// inside processFeaturesField before the group census, so the one-member xor
// group must not be reported.
type featureFlagCollisionOrderApp struct {
	Gates    conga.OptionalFlag[bool]       `name:"gates" help:"Collides with the synthesized flag"`
	Solo     conga.OptionalFlag[bool]       `name:"solo" xor:"choice" help:"Solo"`
	Features conga.Features[goodFeatureSet] `flag:"gates"` // want `struct featureFlagCollisionOrderApp field Features: duplicate flag --gates`
}

func (a *featureFlagCollisionOrderApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[featureFlagCollisionOrderApp]()

type badStageSet struct {
	Level bool `stage:"stable" help:"Level"` // want `struct badStageSet field Level: invalid value "stable" for tag "stage"`
}

type badStageApp struct {
	Features conga.Features[badStageSet]
}

func (a *badStageApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[badStageApp]()

type emptyStageSet struct {
	Level bool `stage:"" help:"Level"` // want `struct emptyStageSet field Level: tag "stage" cannot be empty`
}

type emptyStageApp struct {
	Features conga.Features[emptyStageSet]
}

func (a *emptyStageApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[emptyStageApp]()

type partialStageSet struct {
	First  bool `help:"First" stage:"alpha"`
	Second bool `help:"Second"` // want `struct partialStageSet field Second: feature field second does not declare the stage tag`
}

type partialStageApp struct {
	Features conga.Features[partialStageSet]
}

func (a *partialStageApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[partialStageApp]()

type enumHintChecksApp struct {
	MissingEnum conga.OptionalFlag[string] `enumhint:"-" help:"Format"`                             // want `struct enumHintChecksApp field MissingEnum: tag "enumhint" suppresses the enum hint, but the field declares no enum values`
	Redundant   conga.OptionalFlag[string] `enum:"json,yaml" enumhint:"-" help:"Format ({{enum}})"` // want `struct enumHintChecksApp field Redundant: tag "enumhint" is redundant: the help tag already references \{\{enum\}\}`
	BadValue    conga.OptionalFlag[string] `enum:"json,yaml" enumhint:"true" help:"Format"`         // want `struct enumHintChecksApp field BadValue: tag "enumhint" must be "-"`
}

func (a *enumHintChecksApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[enumHintChecksApp]()

type enumHintFeaturesApp struct {
	Features conga.Features[goodFeatureSet] `enumhint:"-" help:"Gates ({{enum}})"` // want `struct enumHintFeaturesApp field Features: tag "enumhint" is redundant: the help tag already references \{\{enum\}\}`
}

func (a *enumHintFeaturesApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[enumHintFeaturesApp]()

type defaultVarFeaturesApp struct {
	Features conga.Features[goodFeatureSet] `help:"Gates {{default}}"` // want `struct defaultVarFeaturesApp field Features: tag "help" references \{\{default\}\}, but the field provides no value for that variable`
}

func (a *defaultVarFeaturesApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[defaultVarFeaturesApp]()
