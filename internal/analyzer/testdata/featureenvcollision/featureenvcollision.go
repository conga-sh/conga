// Package featureenvcollision exercises the conga-lint mirror of the feature
// env collision validator: duplicate resolved names across feature fields and
// feature variables that flags also bind, including derived names.
package featureenvcollision

import (
	"context"

	"github.com/conga-sh/conga"
)

// Two feature fields declaring the same variable.
type dupFieldSet struct {
	First  bool `env:"SHARED_VAR" help:"First"`
	Second bool `env:"SHARED_VAR" help:"Second"`
}

type dupFieldApp struct {
	Features conga.Features[dupFieldSet] // want `struct dupFieldApp field Features: schema error: environment variable "SHARED_VAR" is bound by both feature "first" and feature "second"`
}

func (a *dupFieldApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[dupFieldApp]().EnableFeatures()

// An explicit field tag colliding with a sibling's derived variable.
type derivedCollisionSet struct {
	First  bool `env:"TASK_X_SECOND" help:"First"`
	Second bool `                    help:"Second"`
}

type derivedCollisionApp struct {
	Features conga.Features[derivedCollisionSet] `envprefix:"TASK_X_"` // want `struct derivedCollisionApp field Features: schema error: environment variable "TASK_X_SECOND" is bound by both feature "first" and feature "second"`
}

func (a *derivedCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[derivedCollisionApp]().EnableFeatures()

// A feature variable a flag also binds explicitly.
type flagCollisionSet struct {
	Enabled bool `env:"SHARED_FLAG_VAR" help:"Enabled"`
}

type flagCollisionApp struct {
	Features conga.Features[flagCollisionSet] `envprefix:"TASK_X_"` // want `struct flagCollisionApp field Features: schema error: environment variable "SHARED_FLAG_VAR" is bound by both feature "enabled" and flag --port`

	Port conga.OptionalFlag[int] `env:"SHARED_FLAG_VAR" help:"Port"`
}

func (a *flagCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[flagCollisionApp]().EnableFeatures()

// The combined map variable a flag also binds.
type mapFlagCollisionSet struct {
	Enabled bool
}

type mapFlagCollisionApp struct {
	Features conga.Features[mapFlagCollisionSet] `env:"SHARED_MAP_VAR"` // want `struct mapFlagCollisionApp field Features: schema error: environment variable "SHARED_MAP_VAR" is bound by both the feature map flag --features and flag --port`

	Port conga.OptionalFlag[int] `env:"SHARED_MAP_VAR" help:"Port"`
}

func (a *mapFlagCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[mapFlagCollisionApp]().EnableFeatures()

// A feature variable a subcommand flag also binds.
type subCollisionSet struct {
	Enabled bool `env:"SHARED_SUB_VAR" help:"Enabled"`
}

type subCollisionSub struct {
	Port conga.OptionalFlag[int] `env:"SHARED_SUB_VAR" help:"Port"`
}

func (s *subCollisionSub) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

type subCollisionApp struct {
	Features conga.Features[subCollisionSet] // want `struct subCollisionApp field Features: schema error: environment variable "SHARED_SUB_VAR" is bound by both feature "enabled" and flag --port`

	Sub conga.Command[subCollisionSub] `summary:"Sub"`
}

func (a *subCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[subCollisionApp]().EnableFeatures()

// A feature variable colliding with a flag's prefix-derived variable.
type derivedFlagCollisionSet struct {
	Enabled bool `env:"TASK_X_PORT" help:"Enabled"`
}

type derivedFlagCollisionApp struct {
	Features conga.Features[derivedFlagCollisionSet] // want `struct derivedFlagCollisionApp field Features: schema error: environment variable "TASK_X_PORT" is bound by both feature "enabled" and flag --port`

	Port conga.OptionalFlag[int] `help:"Port"`
}

func (a *derivedFlagCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[derivedFlagCollisionApp]().EnableFeatures().EnvPrefix("TASK_X")

// A feature variable colliding with the config-file flag's derived variable.
type configFileCollisionSet struct {
	Enabled bool `env:"TASK_X_CONFIG" help:"Enabled"`
}

type configFileCollisionApp struct {
	Features   conga.Features[configFileCollisionSet] // want `struct configFileCollisionApp field Features: schema error: environment variable "TASK_X_CONFIG" is bound by both feature "enabled" and flag --config-file`
	ConfigFile conga.ConfigFile
}

func (a *configFileCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[configFileCollisionApp]().EnableFeatures().EnvPrefix("TASK_X")

// Two independent feature-flag collision pairs: both surfaces report the pair
// whose flag is declared first, never the feature declaration order.
type orderCollisionSet struct {
	First  bool `env:"FLAG_A_VAR" help:"First"`
	Second bool `env:"FLAG_B_VAR" help:"Second"`
}

type orderCollisionApp struct {
	Features conga.Features[orderCollisionSet] // want `struct orderCollisionApp field Features: schema error: environment variable "FLAG_B_VAR" is bound by both feature "second" and flag --flag-one`

	FlagOne conga.OptionalFlag[int] `env:"FLAG_B_VAR" help:"Flag one"`
	FlagTwo conga.OptionalFlag[int] `env:"FLAG_A_VAR" help:"Flag two"`
}

func (a *orderCollisionApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[orderCollisionApp]().EnableFeatures()

// The container envprefix is trimmed before deriving per-feature names, so a
// padded prefix still collides with an explicit flag variable.
type whitespacePrefixSet struct {
	Enabled bool `help:"Enabled"`
}

type whitespacePrefixApp struct {
	Features conga.Features[whitespacePrefixSet] `envprefix:" TASK_X "` // want `struct whitespacePrefixApp field Features: schema error: environment variable "TASK_X_ENABLED" is bound by both feature "enabled" and flag --port`

	Port conga.OptionalFlag[int] `env:"TASK_X_ENABLED" help:"Port"`
}

func (a *whitespacePrefixApp) Run(ctx context.Context, session *conga.Session) error {
	return nil
}

var _ = conga.New[whitespacePrefixApp]().EnableFeatures()
